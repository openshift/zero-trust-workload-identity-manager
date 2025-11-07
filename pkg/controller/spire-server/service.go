package spire_server

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/openshift/zero-trust-workload-identity-manager/api/v1alpha1"
	"github.com/openshift/zero-trust-workload-identity-manager/pkg/controller/status"
	"github.com/openshift/zero-trust-workload-identity-manager/pkg/controller/utils"
	"github.com/openshift/zero-trust-workload-identity-manager/pkg/operator/assets"
)

const (
	SpireServerServiceReady = "SpireServerServiceReady"
)

// reconcileService reconciles all Services (spire-server and controller-manager)
func (r *SpireServerReconciler) reconcileService(ctx context.Context, server *v1alpha1.SpireServer, statusMgr *status.Manager, createOnlyMode bool) error {
	// Spire Server Service
	if err := r.reconcileSpireServerService(ctx, server, statusMgr, createOnlyMode); err != nil {
		return err
	}

	// Controller Manager Webhook Service
	if err := r.reconcileSpireControllerManagerService(ctx, server, statusMgr, createOnlyMode); err != nil {
		return err
	}

	statusMgr.AddCondition(ServiceAvailable, v1alpha1.ReasonReady,
		"All Service resources available",
		metav1.ConditionTrue)

	return nil
}

// reconcileSpireServerService reconciles the Spire Server Service
func (r *SpireServerReconciler) reconcileSpireServerService(ctx context.Context, server *v1alpha1.SpireServer, statusMgr *status.Manager, createOnlyMode bool) error {
	desired := getSpireServerService(server.Spec.Labels)

	if err := controllerutil.SetControllerReference(server, desired, r.scheme); err != nil {
		r.log.Error(err, "failed to set controller reference on service")
		statusMgr.AddCondition(ServiceAvailable, v1alpha1.ReasonFailed,
			fmt.Sprintf("Failed to set owner reference on Service: %v", err),
			metav1.ConditionFalse)
		return err
	}

	// Get existing resource (from cache)
	existing := &corev1.Service{}
	err := r.ctrlClient.Get(ctx, types.NamespacedName{Name: desired.Name, Namespace: desired.Namespace}, existing)

	if err != nil {
		if !kerrors.IsNotFound(err) {
			// Unexpected error
			r.log.Error(err, "failed to get service")
			statusMgr.AddCondition(ServiceAvailable, v1alpha1.ReasonFailed,
				fmt.Sprintf("Failed to get Service: %v", err),
				metav1.ConditionFalse)
			return err
		}

		// Resource doesn't exist, create it
		if err := r.ctrlClient.Create(ctx, desired); err != nil {
			r.log.Error(err, "failed to create service")
			statusMgr.AddCondition(ServiceAvailable, v1alpha1.ReasonFailed,
				fmt.Sprintf("Failed to create Service: %v", err),
				metav1.ConditionFalse)
			return err
		}

		r.log.Info("Created Service", "name", desired.Name, "namespace", desired.Namespace)
		return nil
	}

	// Resource exists, check if we need to update
	if createOnlyMode {
		r.log.V(1).Info("Service exists, skipping update due to create-only mode", "name", desired.Name)
		return nil
	}

	// Check if update is needed
	if !utils.ResourceNeedsUpdate(existing, desired) {
		r.log.V(1).Info("Service is up to date", "name", desired.Name)
		return nil
	}

	// Update the resource
	desired.ResourceVersion = existing.ResourceVersion
	if err := r.ctrlClient.Update(ctx, desired); err != nil {
		r.log.Error(err, "failed to update service")
		statusMgr.AddCondition(ServiceAvailable, v1alpha1.ReasonFailed,
			fmt.Sprintf("Failed to update Service: %v", err),
			metav1.ConditionFalse)
		return err
	}

	r.log.Info("Updated Service", "name", desired.Name, "namespace", desired.Namespace)
	return nil
}

// reconcileSpireControllerManagerService reconciles the Controller Manager webhook Service
func (r *SpireServerReconciler) reconcileSpireControllerManagerService(ctx context.Context, server *v1alpha1.SpireServer, statusMgr *status.Manager, createOnlyMode bool) error {
	desired := getSpireControllerManagerWebhookService(server.Spec.Labels)

	if err := controllerutil.SetControllerReference(server, desired, r.scheme); err != nil {
		r.log.Error(err, "failed to set controller reference on controller manager service")
		statusMgr.AddCondition(ServiceAvailable, v1alpha1.ReasonFailed,
			fmt.Sprintf("Failed to set owner reference on Controller Manager Service: %v", err),
			metav1.ConditionFalse)
		return err
	}

	// Get existing resource (from cache)
	existing := &corev1.Service{}
	err := r.ctrlClient.Get(ctx, types.NamespacedName{Name: desired.Name, Namespace: desired.Namespace}, existing)

	if err != nil {
		if !kerrors.IsNotFound(err) {
			// Unexpected error
			r.log.Error(err, "failed to get controller manager service")
			statusMgr.AddCondition(ServiceAvailable, v1alpha1.ReasonFailed,
				fmt.Sprintf("Failed to get Controller Manager Service: %v", err),
				metav1.ConditionFalse)
			return err
		}

		// Resource doesn't exist, create it
		if err := r.ctrlClient.Create(ctx, desired); err != nil {
			r.log.Error(err, "failed to create controller manager service")
			statusMgr.AddCondition(ServiceAvailable, v1alpha1.ReasonFailed,
				fmt.Sprintf("Failed to create Controller Manager Service: %v", err),
				metav1.ConditionFalse)
			return err
		}

		r.log.Info("Created Service", "name", desired.Name, "namespace", desired.Namespace)
		return nil
	}

	// Resource exists, check if we need to update
	if createOnlyMode {
		r.log.V(1).Info("Service exists, skipping update due to create-only mode", "name", desired.Name)
		return nil
	}

	// Check if update is needed
	if !utils.ResourceNeedsUpdate(existing, desired) {
		r.log.V(1).Info("Service is up to date", "name", desired.Name)
		return nil
	}

	// Update the resource
	desired.ResourceVersion = existing.ResourceVersion
	if err := r.ctrlClient.Update(ctx, desired); err != nil {
		r.log.Error(err, "failed to update controller manager service")
		statusMgr.AddCondition(ServiceAvailable, v1alpha1.ReasonFailed,
			fmt.Sprintf("Failed to update Controller Manager Service: %v", err),
			metav1.ConditionFalse)
		return err
	}

	r.log.Info("Updated Service", "name", desired.Name, "namespace", desired.Namespace)
	return nil
}

// getSpireServerService returns the Spire Server Service with proper labels and selectors
func getSpireServerService(customLabels map[string]string) *corev1.Service {
	svc := utils.DecodeServiceObjBytes(assets.MustAsset(utils.SpireServerServiceAssetName))
	svc.Labels = utils.SpireServerLabels(customLabels)
	svc.Spec.Selector = map[string]string{
		"app.kubernetes.io/name":     "spire-server",
		"app.kubernetes.io/instance": utils.StandardInstance,
	}
	return svc
}

// getSpireControllerManagerWebhookService returns the Controller Manager Service with proper labels and selectors
func getSpireControllerManagerWebhookService(customLabels map[string]string) *corev1.Service {
	svc := utils.DecodeServiceObjBytes(assets.MustAsset(utils.SpireControllerMangerWebhookServiceAssetName))
	svc.Labels = utils.SpireControllerManagerLabels(customLabels)
	svc.Spec.Selector = map[string]string{
		"app.kubernetes.io/name":     "spire-controller-manager",
		"app.kubernetes.io/instance": utils.StandardInstance,
	}
	return svc
}
