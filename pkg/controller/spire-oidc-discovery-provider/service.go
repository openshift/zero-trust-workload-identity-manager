package spire_oidc_discovery_provider

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

// reconcileService reconciles the Spire OIDC Discovery Provider Service
func (r *SpireOidcDiscoveryProviderReconciler) reconcileService(ctx context.Context, oidc *v1alpha1.SpireOIDCDiscoveryProvider, statusMgr *status.Manager, createOnlyMode bool) error {
	desired := getSpireOIDCDiscoveryProviderService(oidc.Spec.Labels)

	if err := controllerutil.SetControllerReference(oidc, desired, r.scheme); err != nil {
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
		statusMgr.AddCondition(ServiceAvailable, v1alpha1.ReasonReady,
			"All Service resources available",
			metav1.ConditionTrue)
		return nil
	}

	// Resource exists, check if we need to update
	if createOnlyMode {
		r.log.V(1).Info("Service exists, skipping update due to create-only mode", "name", desired.Name)
		statusMgr.AddCondition(ServiceAvailable, v1alpha1.ReasonReady,
			"All Service resources available",
			metav1.ConditionTrue)
		return nil
	}

	// Preserve Kubernetes-managed fields from existing resource BEFORE comparison
	desired.ResourceVersion = existing.ResourceVersion
	desired.Spec.ClusterIP = existing.Spec.ClusterIP
	desired.Spec.ClusterIPs = existing.Spec.ClusterIPs
	desired.Spec.IPFamilies = existing.Spec.IPFamilies
	desired.Spec.IPFamilyPolicy = existing.Spec.IPFamilyPolicy
	desired.Spec.InternalTrafficPolicy = existing.Spec.InternalTrafficPolicy
	desired.Spec.SessionAffinity = existing.Spec.SessionAffinity
	if existing.Spec.HealthCheckNodePort != 0 {
		desired.Spec.HealthCheckNodePort = existing.Spec.HealthCheckNodePort
	}

	// Normalize ports - set default protocol to TCP if not specified
	for i := range desired.Spec.Ports {
		if desired.Spec.Ports[i].Protocol == "" {
			desired.Spec.Ports[i].Protocol = corev1.ProtocolTCP
		}
	}

	// Check if update is needed
	if !utils.ResourceNeedsUpdate(existing, desired) {
		r.log.V(1).Info("Service is up to date", "name", desired.Name)
		statusMgr.AddCondition(ServiceAvailable, v1alpha1.ReasonReady,
			"All Service resources available",
			metav1.ConditionTrue)
		return nil
	}

	// Update the resource
	if err := r.ctrlClient.Update(ctx, desired); err != nil {
		r.log.Error(err, "failed to update service")
		statusMgr.AddCondition(ServiceAvailable, v1alpha1.ReasonFailed,
			fmt.Sprintf("Failed to update Service: %v", err),
			metav1.ConditionFalse)
		return err
	}

	r.log.Info("Updated Service", "name", desired.Name, "namespace", desired.Namespace)
	statusMgr.AddCondition(ServiceAvailable, v1alpha1.ReasonReady,
		"All Service resources available",
		metav1.ConditionTrue)
	return nil
}

// getSpireOIDCDiscoveryProviderService returns the Spire OIDC Discovery Provider Service with proper labels and selectors
func getSpireOIDCDiscoveryProviderService(customLabels map[string]string) *corev1.Service {
	svc := utils.DecodeServiceObjBytes(assets.MustAsset(utils.SpireOIDCDiscoveryProviderServiceAssetName))
	svc.Labels = utils.SpireOIDCDiscoveryProviderLabels(customLabels)
	svc.Namespace = utils.GetOperatorNamespace()
	svc.Spec.Selector = map[string]string{
		"app.kubernetes.io/name":     "spiffe-oidc-discovery-provider",
		"app.kubernetes.io/instance": utils.StandardInstance,
	}
	return svc
}
