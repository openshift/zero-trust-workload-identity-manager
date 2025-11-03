package spire_server

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

const (
	SpireServerServiceReady = "SpireServerServiceReady"
)

// reconcileService reconciles the Spire Server Service
func (r *SpireServerReconciler) reconcileService(ctx context.Context, server *v1alpha1.SpireServer, statusMgr *status.Manager, createOnlyMode bool) error {
	svc := getSpireServerService()

	if err := controllerutil.SetControllerReference(server, svc, r.scheme); err != nil {
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

	// Success status is set after all Services are created (including Controller Manager Service)
	return nil
}

// getSpireServerService returns the Spire Server Service with proper labels and selectors
func getSpireServerService() *corev1.Service {
	svc := utils.DecodeServiceObjBytes(assets.MustAsset(utils.SpireServerServiceAssetName))
	svc.Labels = utils.SpireServerLabels(svc.Labels)
	svc.Spec.Selector = map[string]string{
		"app.kubernetes.io/name":     "spire-server",
		"app.kubernetes.io/instance": utils.StandardInstance,
	}
	return svc
}
