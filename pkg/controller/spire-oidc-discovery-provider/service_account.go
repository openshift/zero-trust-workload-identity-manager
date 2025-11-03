package spire_oidc_discovery_provider

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/openshift/zero-trust-workload-identity-manager/api/v1alpha1"
	"github.com/openshift/zero-trust-workload-identity-manager/pkg/controller/status"
	"github.com/openshift/zero-trust-workload-identity-manager/pkg/controller/utils"
	"github.com/openshift/zero-trust-workload-identity-manager/pkg/operator/assets"
)

// Constants for status conditions are defined in controller.go

// reconcileServiceAccount reconciles the Spire OIDC Discovery Provider ServiceAccount
func (r *SpireOidcDiscoveryProviderReconciler) reconcileServiceAccount(ctx context.Context, oidc *v1alpha1.SpireOIDCDiscoveryProvider, statusMgr *status.Manager, createOnlyMode bool) error {
	sa := getSpireOIDCDiscoveryProviderServiceAccount()

	if err := controllerutil.SetControllerReference(oidc, sa, r.scheme); err != nil {
		r.log.Error(err, "failed to set controller reference on service account")
		statusMgr.AddCondition(ServiceAccountAvailable, v1alpha1.ReasonFailed,
			fmt.Sprintf("Failed to set owner reference on ServiceAccount: %v", err),
			metav1.ConditionFalse)
		return err
	}

	if err := r.createOrUpdateResource(ctx, sa, createOnlyMode); err != nil {
		statusMgr.AddCondition(ServiceAccountAvailable, v1alpha1.ReasonFailed,
			fmt.Sprintf("Failed to create ServiceAccount: %v", err),
			metav1.ConditionFalse)
		return err
	}

	statusMgr.AddCondition(ServiceAccountAvailable, v1alpha1.ReasonReady,
		"All ServiceAccount resources available",
		metav1.ConditionTrue)
	return nil
}

// getSpireOIDCDiscoveryProviderServiceAccount returns the Spire OIDC Discovery Provider ServiceAccount with proper labels
func getSpireOIDCDiscoveryProviderServiceAccount() *corev1.ServiceAccount {
	sa := utils.DecodeServiceAccountObjBytes(assets.MustAsset(utils.SpireOIDCDiscoveryProviderServiceAccountAssetName))
	sa.Labels = utils.SpireOIDCDiscoveryProviderLabels(sa.Labels)
	return sa
}

// createOrUpdateResource is a helper method to create or update a resource
func (r *SpireOidcDiscoveryProviderReconciler) createOrUpdateResource(ctx context.Context, obj client.Object, createOnlyMode bool) error {
	// Try to create first
	err := r.ctrlClient.Create(ctx, obj)
	if err == nil {
		r.log.Info("Created resource", "kind", obj.GetObjectKind().GroupVersionKind().Kind, "name", obj.GetName())
		return nil
	}

	if !kerrors.IsAlreadyExists(err) {
		r.log.Error(err, "Failed to create resource", "kind", obj.GetObjectKind().GroupVersionKind().Kind, "name", obj.GetName())
		return err
	}

	// Resource already exists
	if createOnlyMode {
		r.log.Info("Skipping update due to create-only mode", "kind", obj.GetObjectKind().GroupVersionKind().Kind, "name", obj.GetName())
		return nil
	}

	// For cluster-scoped resources (no namespace), we don't update them after initial creation
	// to avoid conflicts with manual modifications
	if obj.GetNamespace() == "" {
		r.log.Info("Skipping update of cluster-scoped resource", "kind", obj.GetObjectKind().GroupVersionKind().Kind, "name", obj.GetName())
		return nil
	}

	r.log.Info("Resource already exists", "kind", obj.GetObjectKind().GroupVersionKind().Kind, "name", obj.GetName())
	return nil
}
