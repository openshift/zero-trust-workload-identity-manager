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

// reconcileServiceAccount reconciles the Spire OIDC Discovery Provider ServiceAccount
func (r *SpireOidcDiscoveryProviderReconciler) reconcileServiceAccount(ctx context.Context, oidc *v1alpha1.SpireOIDCDiscoveryProvider, statusMgr *status.Manager, createOnlyMode bool) error {
	desired := getSpireOIDCDiscoveryProviderServiceAccount(oidc.Spec.Labels)

	if err := controllerutil.SetControllerReference(oidc, desired, r.scheme); err != nil {
		r.log.Error(err, "failed to set controller reference on service account")
		statusMgr.AddCondition(ServiceAccountAvailable, v1alpha1.ReasonFailed,
			fmt.Sprintf("Failed to set owner reference on ServiceAccount: %v", err),
			metav1.ConditionFalse)
		return err
	}

	// Get existing resource (from cache)
	existing := &corev1.ServiceAccount{}
	err := r.ctrlClient.Get(ctx, types.NamespacedName{Name: desired.Name, Namespace: desired.Namespace}, existing)

	if err != nil {
		if !kerrors.IsNotFound(err) {
			// Unexpected error
			r.log.Error(err, "failed to get service account")
			statusMgr.AddCondition(ServiceAccountAvailable, v1alpha1.ReasonFailed,
				fmt.Sprintf("Failed to get ServiceAccount: %v", err),
				metav1.ConditionFalse)
			return err
		}

		// Resource doesn't exist, create it
		if err := r.ctrlClient.Create(ctx, desired); err != nil {
			r.log.Error(err, "failed to create service account")
			statusMgr.AddCondition(ServiceAccountAvailable, v1alpha1.ReasonFailed,
				fmt.Sprintf("Failed to create ServiceAccount: %v", err),
				metav1.ConditionFalse)
			return err
		}

		r.log.Info("Created ServiceAccount", "name", desired.Name, "namespace", desired.Namespace)
		statusMgr.AddCondition(ServiceAccountAvailable, v1alpha1.ReasonReady,
			"All ServiceAccount resources available",
			metav1.ConditionTrue)
		return nil
	}

	// Resource exists, check if we need to update
	if createOnlyMode {
		r.log.V(1).Info("ServiceAccount exists, skipping update due to create-only mode", "name", desired.Name)
		statusMgr.AddCondition(ServiceAccountAvailable, v1alpha1.ReasonReady,
			"All ServiceAccount resources available",
			metav1.ConditionTrue)
		return nil
	}

	// Check if update is needed
	if !utils.ResourceNeedsUpdate(existing, desired) {
		r.log.V(1).Info("ServiceAccount is up to date", "name", desired.Name)
		statusMgr.AddCondition(ServiceAccountAvailable, v1alpha1.ReasonReady,
			"All ServiceAccount resources available",
			metav1.ConditionTrue)
		return nil
	}

	// Update the resource
	desired.ResourceVersion = existing.ResourceVersion
	if err := r.ctrlClient.Update(ctx, desired); err != nil {
		r.log.Error(err, "failed to update service account")
		statusMgr.AddCondition(ServiceAccountAvailable, v1alpha1.ReasonFailed,
			fmt.Sprintf("Failed to update ServiceAccount: %v", err),
			metav1.ConditionFalse)
		return err
	}

	r.log.Info("Updated ServiceAccount", "name", desired.Name, "namespace", desired.Namespace)
	statusMgr.AddCondition(ServiceAccountAvailable, v1alpha1.ReasonReady,
		"All ServiceAccount resources available",
		metav1.ConditionTrue)
	return nil
}

// getSpireOIDCDiscoveryProviderServiceAccount returns the Spire OIDC Discovery Provider ServiceAccount with proper labels
func getSpireOIDCDiscoveryProviderServiceAccount(customLabels map[string]string) *corev1.ServiceAccount {
	sa := utils.DecodeServiceAccountObjBytes(assets.MustAsset(utils.SpireOIDCDiscoveryProviderServiceAccountAssetName))
	sa.Labels = utils.SpireOIDCDiscoveryProviderLabels(customLabels)
	sa.Namespace = utils.GetOperatorNamespace()
	return sa
}
