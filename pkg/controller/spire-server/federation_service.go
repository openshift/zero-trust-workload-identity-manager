package spire_server

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/openshift/zero-trust-workload-identity-manager/api/v1alpha1"
	"github.com/openshift/zero-trust-workload-identity-manager/pkg/controller/status"
	"github.com/openshift/zero-trust-workload-identity-manager/pkg/controller/utils"
)

const (
	federationServiceName = "spire-server-federation" //nolint:unused
)

// ensureFederationService creates or updates the federation Service
func (r *SpireServerReconciler) ensureFederationService(ctx context.Context, server *v1alpha1.SpireServer, statusMgr *status.Manager, createOnlyMode bool) error {
	if server.Spec.Federation == nil {
		// No federation configured, ensure service is deleted if it exists
		return r.deleteFederationServiceIfExists(ctx)
	}

	federationSvc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      federationServiceName,
			Namespace: utils.OperatorNamespace,
			Labels:    utils.SpireServerLabels(server.Spec.Labels),
		},
		Spec: corev1.ServiceSpec{
			Type: corev1.ServiceTypeClusterIP,
			Ports: []corev1.ServicePort{
				{
					Name:       "federation",
					Port:       server.Spec.Federation.BundleEndpoint.Port,
					Protocol:   corev1.ProtocolTCP,
					TargetPort: intstr.FromInt(int(server.Spec.Federation.BundleEndpoint.Port)),
				},
			},
			Selector: map[string]string{
				"app.kubernetes.io/name":     "spire-server",
				"app.kubernetes.io/instance": utils.StandardInstance,
			},
		},
	}

	// Set owner reference
	if err := controllerutil.SetControllerReference(server, federationSvc, r.scheme); err != nil {
		statusMgr.AddCondition(FederationServiceReady, "FederationServiceFailed",
			fmt.Sprintf("Failed to set controller reference: %v", err),
			metav1.ConditionFalse)
		return fmt.Errorf("failed to set controller reference on federation service: %w", err)
	}

	var existingService corev1.Service
	err := r.ctrlClient.Get(ctx, types.NamespacedName{Name: federationSvc.Name, Namespace: federationSvc.Namespace}, &existingService)
	if err != nil && kerrors.IsNotFound(err) {
		// Service doesn't exist, create it
		if err = r.ctrlClient.Create(ctx, federationSvc); err != nil {
			statusMgr.AddCondition(FederationServiceReady, "FederationServiceCreationFailed",
				fmt.Sprintf("Failed to create federation service: %v", err),
				metav1.ConditionFalse)
			return fmt.Errorf("failed to create federation service: %w", err)
		}
		r.log.Info("Created federation service", "name", federationSvc.Name)
		statusMgr.AddCondition(FederationServiceReady, "FederationServiceCreated",
			"Federation service created successfully",
			metav1.ConditionTrue)
		return nil
	} else if err != nil {
		statusMgr.AddCondition(FederationServiceReady, "FederationServiceFailed",
			fmt.Sprintf("Failed to get federation service: %v", err),
			metav1.ConditionFalse)
		return fmt.Errorf("failed to get federation service: %w", err)
	}

	// Service exists
	if createOnlyMode {
		r.log.Info("Skipping federation service update due to create-only mode")
		statusMgr.AddCondition(FederationServiceReady, "FederationServiceReady",
			"Federation service is ready",
			metav1.ConditionTrue)
		return nil
	}

	// Update the service if needed
	existingService.Spec.Ports = federationSvc.Spec.Ports
	existingService.Spec.Selector = federationSvc.Spec.Selector
	existingService.Labels = federationSvc.Labels

	if err = r.ctrlClient.Update(ctx, &existingService); err != nil {
		statusMgr.AddCondition(FederationServiceReady, "FederationServiceUpdateFailed",
			fmt.Sprintf("Failed to update federation service: %v", err),
			metav1.ConditionFalse)
		return fmt.Errorf("failed to update federation service: %w", err)
	}
	r.log.Info("Updated federation service", "name", federationSvc.Name)
	statusMgr.AddCondition(FederationServiceReady, "FederationServiceUpdated",
		"Federation service updated successfully",
		metav1.ConditionTrue)
	return nil
}

// deleteFederationServiceIfExists deletes the federation Service if it exists
func (r *SpireServerReconciler) deleteFederationServiceIfExists(ctx context.Context) error {
	service := &corev1.Service{}
	err := r.ctrlClient.Get(ctx, types.NamespacedName{
		Name:      federationServiceName,
		Namespace: utils.OperatorNamespace,
	}, service)

	if err != nil {
		if kerrors.IsNotFound(err) {
			// Service doesn't exist, nothing to do
			return nil
		}
		return fmt.Errorf("failed to get federation service: %w", err)
	}

	// Service exists, delete it
	if err := r.ctrlClient.Delete(ctx, service); err != nil {
		return fmt.Errorf("failed to delete federation service: %w", err)
	}

	r.log.Info("Deleted federation service", "name", federationServiceName)
	return nil
}
