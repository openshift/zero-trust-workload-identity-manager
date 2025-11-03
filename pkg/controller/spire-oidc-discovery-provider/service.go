package spire_oidc_discovery_provider

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

// reconcileService reconciles the Spire OIDC Discovery Provider Service
func (r *SpireOidcDiscoveryProviderReconciler) reconcileService(ctx context.Context, oidc *v1alpha1.SpireOIDCDiscoveryProvider, statusMgr *status.Manager, createOnlyMode bool) error {
	svc := getSpireOIDCDiscoveryProviderService()

	if err := controllerutil.SetControllerReference(oidc, svc, r.scheme); err != nil {
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

	statusMgr.AddCondition(ServiceAvailable, v1alpha1.ReasonReady,
		"All Service resources available",
		metav1.ConditionTrue)
	return nil
}

// getSpireOIDCDiscoveryProviderService returns the Spire OIDC Discovery Provider Service with proper labels and selectors
func getSpireOIDCDiscoveryProviderService() *corev1.Service {
	svc := utils.DecodeServiceObjBytes(assets.MustAsset(utils.SpireOIDCDiscoveryProviderServiceAssetName))
	svc.Labels = utils.SpireOIDCDiscoveryProviderLabels(svc.Labels)
	svc.Spec.Selector = map[string]string{
		"app.kubernetes.io/name":     "spiffe-oidc-discovery-provider",
		"app.kubernetes.io/instance": utils.StandardInstance,
	}
	return svc
}
