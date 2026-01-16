package spire_oidc_discovery_provider

import (
	"context"
	"fmt"

	rbacv1 "k8s.io/api/rbac/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/openshift/zero-trust-workload-identity-manager/api/v1alpha1"
	"github.com/openshift/zero-trust-workload-identity-manager/pkg/controller/status"
	"github.com/openshift/zero-trust-workload-identity-manager/pkg/controller/utils"
	"github.com/openshift/zero-trust-workload-identity-manager/pkg/operator/assets"
)

// reconcileExternalCertRBAC reconciles RBAC resources for router access to external certificate secret
func (r *SpireOidcDiscoveryProviderReconciler) reconcileExternalCertRBAC(ctx context.Context, oidc *v1alpha1.SpireOIDCDiscoveryProvider, statusMgr *status.Manager, createOnlyMode bool) error {
	// Only create RBAC if externalSecretRef is configured
	if oidc.Spec.ExternalSecretRef == "" {
		return nil
	}

	// Reconcile Role
	if err := r.reconcileExternalCertRole(ctx, oidc, statusMgr, createOnlyMode); err != nil {
		return err
	}

	// Reconcile RoleBinding
	if err := r.reconcileExternalCertRoleBinding(ctx, oidc, statusMgr, createOnlyMode); err != nil {
		return err
	}

	statusMgr.AddCondition(RBACAvailable, v1alpha1.ReasonReady,
		"RBAC resources available",
		metav1.ConditionTrue)

	return nil
}

// reconcileExternalCertRole reconciles the Role for router to read external certificate secret
func (r *SpireOidcDiscoveryProviderReconciler) reconcileExternalCertRole(ctx context.Context, oidc *v1alpha1.SpireOIDCDiscoveryProvider, statusMgr *status.Manager, createOnlyMode bool) error {
	desired := getExternalCertRole(oidc.Spec.Labels)

	// Set the specific secret name in resourceNames
	desired.Rules[0].ResourceNames = []string{oidc.Spec.ExternalSecretRef}

	if err := controllerutil.SetControllerReference(oidc, desired, r.scheme); err != nil {
		r.log.Error(err, "failed to set controller reference on external cert role")
		statusMgr.AddCondition(RBACAvailable, v1alpha1.ReasonFailed,
			fmt.Sprintf("Failed to set owner reference on external cert Role: %v", err),
			metav1.ConditionFalse)
		return err
	}

	// Get existing resource (from cache)
	existing := &rbacv1.Role{}
	err := r.ctrlClient.Get(ctx, types.NamespacedName{Name: desired.Name, Namespace: desired.Namespace}, existing)

	if err != nil {
		if !kerrors.IsNotFound(err) {
			// Unexpected error
			r.log.Error(err, "failed to get external cert role")
			statusMgr.AddCondition(RBACAvailable, v1alpha1.ReasonFailed,
				fmt.Sprintf("Failed to get external cert Role: %v", err),
				metav1.ConditionFalse)
			return err
		}

		// Resource doesn't exist, create it
		if err := r.ctrlClient.Create(ctx, desired); err != nil {
			r.log.Error(err, "failed to create external cert role")
			statusMgr.AddCondition(RBACAvailable, v1alpha1.ReasonFailed,
				fmt.Sprintf("Failed to create external cert Role: %v", err),
				metav1.ConditionFalse)
			return err
		}

		r.log.Info("Created external cert Role", "name", desired.Name, "namespace", desired.Namespace)
		return nil
	}

	// Resource exists, check if we need to update
	if createOnlyMode {
		r.log.V(1).Info("External cert Role exists, skipping update due to create-only mode", "name", desired.Name)
		return nil
	}

	// Check if update is needed
	if !utils.ResourceNeedsUpdate(existing, desired) {
		r.log.V(1).Info("External cert Role is up to date", "name", desired.Name)
		return nil
	}

	// Update the resource
	desired.ResourceVersion = existing.ResourceVersion
	if err := r.ctrlClient.Update(ctx, desired); err != nil {
		r.log.Error(err, "failed to update external cert role")
		statusMgr.AddCondition(RBACAvailable, v1alpha1.ReasonFailed,
			fmt.Sprintf("Failed to update external cert Role: %v", err),
			metav1.ConditionFalse)
		return err
	}

	r.log.Info("Updated external cert Role", "name", desired.Name, "namespace", desired.Namespace)
	return nil
}

// reconcileExternalCertRoleBinding reconciles the RoleBinding for router to read external certificate secret
func (r *SpireOidcDiscoveryProviderReconciler) reconcileExternalCertRoleBinding(ctx context.Context, oidc *v1alpha1.SpireOIDCDiscoveryProvider, statusMgr *status.Manager, createOnlyMode bool) error {
	desired := getExternalCertRoleBinding(oidc.Spec.Labels)

	if err := controllerutil.SetControllerReference(oidc, desired, r.scheme); err != nil {
		r.log.Error(err, "failed to set controller reference on external cert role binding")
		statusMgr.AddCondition(RBACAvailable, v1alpha1.ReasonFailed,
			fmt.Sprintf("Failed to set owner reference on external cert RoleBinding: %v", err),
			metav1.ConditionFalse)
		return err
	}

	// Get existing resource (from cache)
	existing := &rbacv1.RoleBinding{}
	err := r.ctrlClient.Get(ctx, types.NamespacedName{Name: desired.Name, Namespace: desired.Namespace}, existing)

	if err != nil {
		if !kerrors.IsNotFound(err) {
			// Unexpected error
			r.log.Error(err, "failed to get external cert role binding")
			statusMgr.AddCondition(RBACAvailable, v1alpha1.ReasonFailed,
				fmt.Sprintf("Failed to get external cert RoleBinding: %v", err),
				metav1.ConditionFalse)
			return err
		}

		// Resource doesn't exist, create it
		if err := r.ctrlClient.Create(ctx, desired); err != nil {
			r.log.Error(err, "failed to create external cert role binding")
			statusMgr.AddCondition(RBACAvailable, v1alpha1.ReasonFailed,
				fmt.Sprintf("Failed to create external cert RoleBinding: %v", err),
				metav1.ConditionFalse)
			return err
		}

		r.log.Info("Created external cert RoleBinding", "name", desired.Name, "namespace", desired.Namespace)
		return nil
	}

	// Resource exists, check if we need to update
	if createOnlyMode {
		r.log.V(1).Info("External cert RoleBinding exists, skipping update due to create-only mode", "name", desired.Name)
		return nil
	}

	// Check if update is needed
	if !utils.ResourceNeedsUpdate(existing, desired) {
		r.log.V(1).Info("External cert RoleBinding is up to date", "name", desired.Name)
		return nil
	}

	// Update the resource
	desired.ResourceVersion = existing.ResourceVersion
	if err := r.ctrlClient.Update(ctx, desired); err != nil {
		r.log.Error(err, "failed to update external cert role binding")
		statusMgr.AddCondition(RBACAvailable, v1alpha1.ReasonFailed,
			fmt.Sprintf("Failed to update external cert RoleBinding: %v", err),
			metav1.ConditionFalse)
		return err
	}

	r.log.Info("Updated external cert RoleBinding", "name", desired.Name, "namespace", desired.Namespace)
	return nil
}

// Resource getter functions

func getExternalCertRole(customLabels map[string]string) *rbacv1.Role {
	role := utils.DecodeRoleObjBytes(assets.MustAsset(utils.SpireOIDCExternalCertRoleAssetName))
	role.Labels = utils.SpireOIDCDiscoveryProviderLabels(customLabels)
	role.Namespace = utils.GetOperatorNamespace()
	return role
}

func getExternalCertRoleBinding(customLabels map[string]string) *rbacv1.RoleBinding {
	rb := utils.DecodeRoleBindingObjBytes(assets.MustAsset(utils.SpireOIDCExternalCertRoleBindingAssetName))
	rb.Labels = utils.SpireOIDCDiscoveryProviderLabels(customLabels)
	rb.Namespace = utils.GetOperatorNamespace()
	// Note: subjects namespace (openshift-ingress) is already set in the template
	return rb
}
