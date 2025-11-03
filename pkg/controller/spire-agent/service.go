package spire_agent

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/openshift/zero-trust-workload-identity-manager/api/v1alpha1"
	"github.com/openshift/zero-trust-workload-identity-manager/pkg/controller/status"
	"github.com/openshift/zero-trust-workload-identity-manager/pkg/controller/utils"
	"github.com/openshift/zero-trust-workload-identity-manager/pkg/operator/assets"
)

// Constants for status conditions are defined in controller.go

// reconcileService reconciles the Spire Agent Service
func (r *SpireAgentReconciler) reconcileService(ctx context.Context, agent *v1alpha1.SpireAgent, statusMgr *status.Manager, createOnlyMode bool) error {
	svc := getSpireAgentService()

	if err := controllerutil.SetControllerReference(agent, svc, r.scheme); err != nil {
		r.log.Error(err, "failed to set controller reference on service")
		statusMgr.AddCondition(ServiceAvailable, v1alpha1.ReasonFailed,
			fmt.Sprintf("Failed to set owner reference on Service: %v", err),
			metav1.ConditionFalse)
		return err
	}

	if err := r.createOrUpdateResource(ctx, svc, createOnlyMode); err != nil {
		statusMgr.AddCondition(ServiceAvailable, v1alpha1.ReasonFailed,
			fmt.Sprintf("Failed to create Service: %v", err),
			metav1.ConditionFalse)
		return err
	}

	// Success status is set after all Services are created
	return nil
}

// getSpireAgentService returns the Spire Agent Service with proper labels and selectors
func getSpireAgentService() *corev1.Service {
	svc := utils.DecodeServiceObjBytes(assets.MustAsset(utils.SpireAgentServiceAssetName))
	svc.Labels = utils.SpireAgentLabels(svc.Labels)
	svc.Spec.Selector = map[string]string{
		"app.kubernetes.io/name":     "spire-agent",
		"app.kubernetes.io/instance": utils.StandardInstance,
	}
	return svc
}
