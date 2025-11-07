package spire_agent

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

// Constants for status conditions are defined in controller.go

// reconcileService reconciles the Service resource required for agents
func (r *SpireAgentReconciler) reconcileService(ctx context.Context, agent *v1alpha1.SpireAgent, statusMgr *status.Manager, createOnlyMode bool) error {
	err := r.reconcileAgentService(ctx, agent, statusMgr, createOnlyMode)
	if err != nil {
		return err
	}
	statusMgr.AddCondition(ServiceAvailable, v1alpha1.ReasonReady,
		"All Service resources available",
		metav1.ConditionTrue)
	return nil
}

// reconcileAgentService reconciles the Spire Agent Service
func (r *SpireAgentReconciler) reconcileAgentService(ctx context.Context, agent *v1alpha1.SpireAgent, statusMgr *status.Manager, createOnlyMode bool) error {
	desired := getSpireAgentService(agent.Spec.Labels)

	if err := controllerutil.SetControllerReference(agent, desired, r.scheme); err != nil {
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

// getSpireAgentService returns the Spire Agent Service with proper labels and selectors
func getSpireAgentService(customLabels map[string]string) *corev1.Service {
	svc := utils.DecodeServiceObjBytes(assets.MustAsset(utils.SpireAgentServiceAssetName))
	svc.Labels = utils.SpireAgentLabels(customLabels)
	svc.Spec.Selector = map[string]string{
		"app.kubernetes.io/name":     "spire-agent",
		"app.kubernetes.io/instance": utils.StandardInstance,
	}
	return svc
}
