package spire_server

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

// Constants for status conditions are defined in controller.go

// reconcileRBAC reconciles all RBAC resources (spire-server, bundle, and controller-manager)
func (r *SpireServerReconciler) reconcileRBAC(ctx context.Context, server *v1alpha1.SpireServer, statusMgr *status.Manager, createOnlyMode bool) error {
	// Spire Server RBAC
	if err := r.reconcileClusterRole(ctx, server, statusMgr, createOnlyMode); err != nil {
		return err
	}

	if err := r.reconcileClusterRoleBinding(ctx, server, statusMgr, createOnlyMode); err != nil {
		return err
	}

	// Spire Bundle RBAC
	if err := r.reconcileSpireBundleRole(ctx, server, statusMgr, createOnlyMode); err != nil {
		return err
	}

	if err := r.reconcileSpireBundleRoleBinding(ctx, server, statusMgr, createOnlyMode); err != nil {
		return err
	}

	// Controller Manager RBAC
	if err := r.reconcileControllerManagerClusterRole(ctx, server, statusMgr, createOnlyMode); err != nil {
		return err
	}

	if err := r.reconcileControllerManagerClusterRoleBinding(ctx, server, statusMgr, createOnlyMode); err != nil {
		return err
	}

	if err := r.reconcileLeaderElectionRole(ctx, server, statusMgr, createOnlyMode); err != nil {
		return err
	}

	if err := r.reconcileLeaderElectionRoleBinding(ctx, server, statusMgr, createOnlyMode); err != nil {
		return err
	}

	statusMgr.AddCondition(RBACAvailable, v1alpha1.ReasonReady,
		"All RBAC resources available",
		metav1.ConditionTrue)

	return nil
}

// reconcileClusterRole reconciles the Spire Server ClusterRole
func (r *SpireServerReconciler) reconcileClusterRole(ctx context.Context, server *v1alpha1.SpireServer, statusMgr *status.Manager, createOnlyMode bool) error {
	desired := getSpireServerClusterRole(server.Spec.Labels)

	if err := controllerutil.SetControllerReference(server, desired, r.scheme); err != nil {
		r.log.Error(err, "failed to set controller reference on cluster role")
		statusMgr.AddCondition(RBACAvailable, v1alpha1.ReasonFailed,
			fmt.Sprintf("Failed to set owner reference on ClusterRole: %v", err),
			metav1.ConditionFalse)
		return err
	}

	// Get existing resource (from cache)
	existing := &rbacv1.ClusterRole{}
	err := r.ctrlClient.Get(ctx, types.NamespacedName{Name: desired.Name}, existing)

	if err != nil {
		if !kerrors.IsNotFound(err) {
			// Unexpected error
			r.log.Error(err, "failed to get cluster role")
			statusMgr.AddCondition(RBACAvailable, v1alpha1.ReasonFailed,
				fmt.Sprintf("Failed to get ClusterRole: %v", err),
				metav1.ConditionFalse)
			return err
		}

		// Resource doesn't exist, create it
		if err := r.ctrlClient.Create(ctx, desired); err != nil {
			r.log.Error(err, "failed to create cluster role")
			statusMgr.AddCondition(RBACAvailable, v1alpha1.ReasonFailed,
				fmt.Sprintf("Failed to create ClusterRole: %v", err),
				metav1.ConditionFalse)
			return err
		}

		r.log.Info("Created ClusterRole", "name", desired.Name)
		return nil
	}

	// Resource exists, check if we need to update
	if createOnlyMode {
		r.log.V(1).Info("ClusterRole exists, skipping update due to create-only mode", "name", desired.Name)
		return nil
	}

	// Check if update is needed
	if !utils.ResourceNeedsUpdate(existing, desired) {
		r.log.V(1).Info("ClusterRole is up to date", "name", desired.Name)
		return nil
	}

	// Update the resource
	desired.ResourceVersion = existing.ResourceVersion
	if err := r.ctrlClient.Update(ctx, desired); err != nil {
		r.log.Error(err, "failed to update cluster role")
		statusMgr.AddCondition(RBACAvailable, v1alpha1.ReasonFailed,
			fmt.Sprintf("Failed to update ClusterRole: %v", err),
			metav1.ConditionFalse)
		return err
	}

	r.log.Info("Updated ClusterRole", "name", desired.Name)
	return nil
}

// reconcileClusterRoleBinding reconciles the Spire Server ClusterRoleBinding
func (r *SpireServerReconciler) reconcileClusterRoleBinding(ctx context.Context, server *v1alpha1.SpireServer, statusMgr *status.Manager, createOnlyMode bool) error {
	desired := getSpireServerClusterRoleBinding(server.Spec.Labels)

	if err := controllerutil.SetControllerReference(server, desired, r.scheme); err != nil {
		r.log.Error(err, "failed to set controller reference on cluster role binding")
		statusMgr.AddCondition(RBACAvailable, v1alpha1.ReasonFailed,
			fmt.Sprintf("Failed to set owner reference on ClusterRoleBinding: %v", err),
			metav1.ConditionFalse)
		return err
	}

	// Get existing resource (from cache)
	existing := &rbacv1.ClusterRoleBinding{}
	err := r.ctrlClient.Get(ctx, types.NamespacedName{Name: desired.Name}, existing)

	if err != nil {
		if !kerrors.IsNotFound(err) {
			// Unexpected error
			r.log.Error(err, "failed to get cluster role binding")
			statusMgr.AddCondition(RBACAvailable, v1alpha1.ReasonFailed,
				fmt.Sprintf("Failed to get ClusterRoleBinding: %v", err),
				metav1.ConditionFalse)
			return err
		}

		// Resource doesn't exist, create it
		if err := r.ctrlClient.Create(ctx, desired); err != nil {
			r.log.Error(err, "failed to create cluster role binding")
			statusMgr.AddCondition(RBACAvailable, v1alpha1.ReasonFailed,
				fmt.Sprintf("Failed to create ClusterRoleBinding: %v", err),
				metav1.ConditionFalse)
			return err
		}

		r.log.Info("Created ClusterRoleBinding", "name", desired.Name)
		return nil
	}

	// Resource exists, check if we need to update
	if createOnlyMode {
		r.log.V(1).Info("ClusterRoleBinding exists, skipping update due to create-only mode", "name", desired.Name)
		return nil
	}

	// Check if update is needed
	if !utils.ResourceNeedsUpdate(existing, desired) {
		r.log.V(1).Info("ClusterRoleBinding is up to date", "name", desired.Name)
		return nil
	}

	// Update the resource
	desired.ResourceVersion = existing.ResourceVersion
	if err := r.ctrlClient.Update(ctx, desired); err != nil {
		r.log.Error(err, "failed to update cluster role binding")
		statusMgr.AddCondition(RBACAvailable, v1alpha1.ReasonFailed,
			fmt.Sprintf("Failed to update ClusterRoleBinding: %v", err),
			metav1.ConditionFalse)
		return err
	}

	r.log.Info("Updated ClusterRoleBinding", "name", desired.Name)
	return nil
}

// reconcileSpireBundleRole reconciles the Spire Bundle Role
func (r *SpireServerReconciler) reconcileSpireBundleRole(ctx context.Context, server *v1alpha1.SpireServer, statusMgr *status.Manager, createOnlyMode bool) error {
	desired := getSpireBundleRole(server.Spec.Labels)

	if err := controllerutil.SetControllerReference(server, desired, r.scheme); err != nil {
		r.log.Error(err, "failed to set controller reference on spire-bundle role")
		statusMgr.AddCondition(RBACAvailable, v1alpha1.ReasonFailed,
			fmt.Sprintf("Failed to set owner reference on Bundle Role: %v", err),
			metav1.ConditionFalse)
		return err
	}

	// Get existing resource (from cache)
	existing := &rbacv1.Role{}
	err := r.ctrlClient.Get(ctx, types.NamespacedName{Name: desired.Name, Namespace: desired.Namespace}, existing)

	if err != nil {
		if !kerrors.IsNotFound(err) {
			// Unexpected error
			r.log.Error(err, "failed to get spire-bundle role")
			statusMgr.AddCondition(RBACAvailable, v1alpha1.ReasonFailed,
				fmt.Sprintf("Failed to get Bundle Role: %v", err),
				metav1.ConditionFalse)
			return err
		}

		// Resource doesn't exist, create it
		if err := r.ctrlClient.Create(ctx, desired); err != nil {
			r.log.Error(err, "failed to create spire-bundle role")
			statusMgr.AddCondition(RBACAvailable, v1alpha1.ReasonFailed,
				fmt.Sprintf("Failed to create Bundle Role: %v", err),
				metav1.ConditionFalse)
			return err
		}

		r.log.Info("Created Role", "name", desired.Name, "namespace", desired.Namespace)
		return nil
	}

	// Resource exists, check if we need to update
	if createOnlyMode {
		r.log.V(1).Info("Role exists, skipping update due to create-only mode", "name", desired.Name)
		return nil
	}

	// Check if update is needed
	if !utils.ResourceNeedsUpdate(existing, desired) {
		r.log.V(1).Info("Role is up to date", "name", desired.Name)
		return nil
	}

	// Update the resource
	desired.ResourceVersion = existing.ResourceVersion
	if err := r.ctrlClient.Update(ctx, desired); err != nil {
		r.log.Error(err, "failed to update spire-bundle role")
		statusMgr.AddCondition(RBACAvailable, v1alpha1.ReasonFailed,
			fmt.Sprintf("Failed to update Bundle Role: %v", err),
			metav1.ConditionFalse)
		return err
	}

	r.log.Info("Updated Role", "name", desired.Name, "namespace", desired.Namespace)
	return nil
}

// reconcileSpireBundleRoleBinding reconciles the Spire Bundle RoleBinding
func (r *SpireServerReconciler) reconcileSpireBundleRoleBinding(ctx context.Context, server *v1alpha1.SpireServer, statusMgr *status.Manager, createOnlyMode bool) error {
	desired := getSpireBundleRoleBinding(server.Spec.Labels)

	if err := controllerutil.SetControllerReference(server, desired, r.scheme); err != nil {
		r.log.Error(err, "failed to set controller reference on spire-bundle role binding")
		statusMgr.AddCondition(RBACAvailable, v1alpha1.ReasonFailed,
			fmt.Sprintf("Failed to set owner reference on Bundle RoleBinding: %v", err),
			metav1.ConditionFalse)
		return err
	}

	// Get existing resource (from cache)
	existing := &rbacv1.RoleBinding{}
	err := r.ctrlClient.Get(ctx, types.NamespacedName{Name: desired.Name, Namespace: desired.Namespace}, existing)

	if err != nil {
		if !kerrors.IsNotFound(err) {
			// Unexpected error
			r.log.Error(err, "failed to get spire-bundle role binding")
			statusMgr.AddCondition(RBACAvailable, v1alpha1.ReasonFailed,
				fmt.Sprintf("Failed to get Bundle RoleBinding: %v", err),
				metav1.ConditionFalse)
			return err
		}

		// Resource doesn't exist, create it
		if err := r.ctrlClient.Create(ctx, desired); err != nil {
			r.log.Error(err, "failed to create spire-bundle role binding")
			statusMgr.AddCondition(RBACAvailable, v1alpha1.ReasonFailed,
				fmt.Sprintf("Failed to create Bundle RoleBinding: %v", err),
				metav1.ConditionFalse)
			return err
		}

		r.log.Info("Created RoleBinding", "name", desired.Name, "namespace", desired.Namespace)
		return nil
	}

	// Resource exists, check if we need to update
	if createOnlyMode {
		r.log.V(1).Info("RoleBinding exists, skipping update due to create-only mode", "name", desired.Name)
		return nil
	}

	// Check if update is needed
	if !utils.ResourceNeedsUpdate(existing, desired) {
		r.log.V(1).Info("RoleBinding is up to date", "name", desired.Name)
		return nil
	}

	// Update the resource
	desired.ResourceVersion = existing.ResourceVersion
	if err := r.ctrlClient.Update(ctx, desired); err != nil {
		r.log.Error(err, "failed to update spire-bundle role binding")
		statusMgr.AddCondition(RBACAvailable, v1alpha1.ReasonFailed,
			fmt.Sprintf("Failed to update Bundle RoleBinding: %v", err),
			metav1.ConditionFalse)
		return err
	}

	r.log.Info("Updated RoleBinding", "name", desired.Name, "namespace", desired.Namespace)
	return nil
}

// reconcileControllerManagerClusterRole reconciles the Controller Manager ClusterRole
func (r *SpireServerReconciler) reconcileControllerManagerClusterRole(ctx context.Context, server *v1alpha1.SpireServer, statusMgr *status.Manager, createOnlyMode bool) error {
	desired := getSpireControllerManagerClusterRole(server.Spec.Labels)

	if err := controllerutil.SetControllerReference(server, desired, r.scheme); err != nil {
		r.log.Error(err, "failed to set controller reference on controller manager cluster role")
		statusMgr.AddCondition(RBACAvailable, v1alpha1.ReasonFailed,
			fmt.Sprintf("Failed to set owner reference on Controller Manager ClusterRole: %v", err),
			metav1.ConditionFalse)
		return err
	}

	// Get existing resource (from cache)
	existing := &rbacv1.ClusterRole{}
	err := r.ctrlClient.Get(ctx, types.NamespacedName{Name: desired.Name}, existing)

	if err != nil {
		if !kerrors.IsNotFound(err) {
			// Unexpected error
			r.log.Error(err, "failed to get controller manager cluster role")
			statusMgr.AddCondition(RBACAvailable, v1alpha1.ReasonFailed,
				fmt.Sprintf("Failed to get Controller Manager ClusterRole: %v", err),
				metav1.ConditionFalse)
			return err
		}

		// Resource doesn't exist, create it
		if err := r.ctrlClient.Create(ctx, desired); err != nil {
			r.log.Error(err, "failed to create controller manager cluster role")
			statusMgr.AddCondition(RBACAvailable, v1alpha1.ReasonFailed,
				fmt.Sprintf("Failed to create Controller Manager ClusterRole: %v", err),
				metav1.ConditionFalse)
			return err
		}

		r.log.Info("Created ClusterRole", "name", desired.Name)
		return nil
	}

	// Resource exists, check if we need to update
	if createOnlyMode {
		r.log.V(1).Info("ClusterRole exists, skipping update due to create-only mode", "name", desired.Name)
		return nil
	}

	// Check if update is needed
	if !utils.ResourceNeedsUpdate(existing, desired) {
		r.log.V(1).Info("ClusterRole is up to date", "name", desired.Name)
		return nil
	}

	// Update the resource
	desired.ResourceVersion = existing.ResourceVersion
	if err := r.ctrlClient.Update(ctx, desired); err != nil {
		r.log.Error(err, "failed to update controller manager cluster role")
		statusMgr.AddCondition(RBACAvailable, v1alpha1.ReasonFailed,
			fmt.Sprintf("Failed to update Controller Manager ClusterRole: %v", err),
			metav1.ConditionFalse)
		return err
	}

	r.log.Info("Updated ClusterRole", "name", desired.Name)
	return nil
}

// reconcileControllerManagerClusterRoleBinding reconciles the Controller Manager ClusterRoleBinding
func (r *SpireServerReconciler) reconcileControllerManagerClusterRoleBinding(ctx context.Context, server *v1alpha1.SpireServer, statusMgr *status.Manager, createOnlyMode bool) error {
	desired := getSpireControllerManagerClusterRoleBinding(server.Spec.Labels)

	if err := controllerutil.SetControllerReference(server, desired, r.scheme); err != nil {
		r.log.Error(err, "failed to set controller reference on controller manager cluster role binding")
		statusMgr.AddCondition(RBACAvailable, v1alpha1.ReasonFailed,
			fmt.Sprintf("Failed to set owner reference on Controller Manager ClusterRoleBinding: %v", err),
			metav1.ConditionFalse)
		return err
	}

	// Get existing resource (from cache)
	existing := &rbacv1.ClusterRoleBinding{}
	err := r.ctrlClient.Get(ctx, types.NamespacedName{Name: desired.Name}, existing)

	if err != nil {
		if !kerrors.IsNotFound(err) {
			// Unexpected error
			r.log.Error(err, "failed to get controller manager cluster role binding")
			statusMgr.AddCondition(RBACAvailable, v1alpha1.ReasonFailed,
				fmt.Sprintf("Failed to get Controller Manager ClusterRoleBinding: %v", err),
				metav1.ConditionFalse)
			return err
		}

		// Resource doesn't exist, create it
		if err := r.ctrlClient.Create(ctx, desired); err != nil {
			r.log.Error(err, "failed to create controller manager cluster role binding")
			statusMgr.AddCondition(RBACAvailable, v1alpha1.ReasonFailed,
				fmt.Sprintf("Failed to create Controller Manager ClusterRoleBinding: %v", err),
				metav1.ConditionFalse)
			return err
		}

		r.log.Info("Created ClusterRoleBinding", "name", desired.Name)
		return nil
	}

	// Resource exists, check if we need to update
	if createOnlyMode {
		r.log.V(1).Info("ClusterRoleBinding exists, skipping update due to create-only mode", "name", desired.Name)
		return nil
	}

	// Check if update is needed
	if !utils.ResourceNeedsUpdate(existing, desired) {
		r.log.V(1).Info("ClusterRoleBinding is up to date", "name", desired.Name)
		return nil
	}

	// Update the resource
	desired.ResourceVersion = existing.ResourceVersion
	if err := r.ctrlClient.Update(ctx, desired); err != nil {
		r.log.Error(err, "failed to update controller manager cluster role binding")
		statusMgr.AddCondition(RBACAvailable, v1alpha1.ReasonFailed,
			fmt.Sprintf("Failed to update Controller Manager ClusterRoleBinding: %v", err),
			metav1.ConditionFalse)
		return err
	}

	r.log.Info("Updated ClusterRoleBinding", "name", desired.Name)
	return nil
}

// reconcileLeaderElectionRole reconciles the Leader Election Role
func (r *SpireServerReconciler) reconcileLeaderElectionRole(ctx context.Context, server *v1alpha1.SpireServer, statusMgr *status.Manager, createOnlyMode bool) error {
	desired := getSpireControllerManagerLeaderElectionRole(server.Spec.Labels)

	if err := controllerutil.SetControllerReference(server, desired, r.scheme); err != nil {
		r.log.Error(err, "failed to set controller reference on leader election role")
		statusMgr.AddCondition(RBACAvailable, v1alpha1.ReasonFailed,
			fmt.Sprintf("Failed to set owner reference on Leader Election Role: %v", err),
			metav1.ConditionFalse)
		return err
	}

	// Get existing resource (from cache)
	existing := &rbacv1.Role{}
	err := r.ctrlClient.Get(ctx, types.NamespacedName{Name: desired.Name, Namespace: desired.Namespace}, existing)

	if err != nil {
		if !kerrors.IsNotFound(err) {
			// Unexpected error
			r.log.Error(err, "failed to get leader election role")
			statusMgr.AddCondition(RBACAvailable, v1alpha1.ReasonFailed,
				fmt.Sprintf("Failed to get Leader Election Role: %v", err),
				metav1.ConditionFalse)
			return err
		}

		// Resource doesn't exist, create it
		if err := r.ctrlClient.Create(ctx, desired); err != nil {
			r.log.Error(err, "failed to create leader election role")
			statusMgr.AddCondition(RBACAvailable, v1alpha1.ReasonFailed,
				fmt.Sprintf("Failed to create Leader Election Role: %v", err),
				metav1.ConditionFalse)
			return err
		}

		r.log.Info("Created Role", "name", desired.Name, "namespace", desired.Namespace)
		return nil
	}

	// Resource exists, check if we need to update
	if createOnlyMode {
		r.log.V(1).Info("Role exists, skipping update due to create-only mode", "name", desired.Name)
		return nil
	}

	// Check if update is needed
	if !utils.ResourceNeedsUpdate(existing, desired) {
		r.log.V(1).Info("Role is up to date", "name", desired.Name)
		return nil
	}

	// Update the resource
	desired.ResourceVersion = existing.ResourceVersion
	if err := r.ctrlClient.Update(ctx, desired); err != nil {
		r.log.Error(err, "failed to update leader election role")
		statusMgr.AddCondition(RBACAvailable, v1alpha1.ReasonFailed,
			fmt.Sprintf("Failed to update Leader Election Role: %v", err),
			metav1.ConditionFalse)
		return err
	}

	r.log.Info("Updated Role", "name", desired.Name, "namespace", desired.Namespace)
	return nil
}

// reconcileLeaderElectionRoleBinding reconciles the Leader Election RoleBinding
func (r *SpireServerReconciler) reconcileLeaderElectionRoleBinding(ctx context.Context, server *v1alpha1.SpireServer, statusMgr *status.Manager, createOnlyMode bool) error {
	desired := getSpireControllerManagerLeaderElectionRoleBinding(server.Spec.Labels)

	if err := controllerutil.SetControllerReference(server, desired, r.scheme); err != nil {
		r.log.Error(err, "failed to set controller reference on leader election role binding")
		statusMgr.AddCondition(RBACAvailable, v1alpha1.ReasonFailed,
			fmt.Sprintf("Failed to set owner reference on Leader Election RoleBinding: %v", err),
			metav1.ConditionFalse)
		return err
	}

	// Get existing resource (from cache)
	existing := &rbacv1.RoleBinding{}
	err := r.ctrlClient.Get(ctx, types.NamespacedName{Name: desired.Name, Namespace: desired.Namespace}, existing)

	if err != nil {
		if !kerrors.IsNotFound(err) {
			// Unexpected error
			r.log.Error(err, "failed to get leader election role binding")
			statusMgr.AddCondition(RBACAvailable, v1alpha1.ReasonFailed,
				fmt.Sprintf("Failed to get Leader Election RoleBinding: %v", err),
				metav1.ConditionFalse)
			return err
		}

		// Resource doesn't exist, create it
		if err := r.ctrlClient.Create(ctx, desired); err != nil {
			r.log.Error(err, "failed to create leader election role binding")
			statusMgr.AddCondition(RBACAvailable, v1alpha1.ReasonFailed,
				fmt.Sprintf("Failed to create Leader Election RoleBinding: %v", err),
				metav1.ConditionFalse)
			return err
		}

		r.log.Info("Created RoleBinding", "name", desired.Name, "namespace", desired.Namespace)
		return nil
	}

	// Resource exists, check if we need to update
	if createOnlyMode {
		r.log.V(1).Info("RoleBinding exists, skipping update due to create-only mode", "name", desired.Name)
		return nil
	}

	// Check if update is needed
	if !utils.ResourceNeedsUpdate(existing, desired) {
		r.log.V(1).Info("RoleBinding is up to date", "name", desired.Name)
		return nil
	}

	// Update the resource
	desired.ResourceVersion = existing.ResourceVersion
	if err := r.ctrlClient.Update(ctx, desired); err != nil {
		r.log.Error(err, "failed to update leader election role binding")
		statusMgr.AddCondition(RBACAvailable, v1alpha1.ReasonFailed,
			fmt.Sprintf("Failed to update Leader Election RoleBinding: %v", err),
			metav1.ConditionFalse)
		return err
	}

	r.log.Info("Updated RoleBinding", "name", desired.Name, "namespace", desired.Namespace)
	return nil
}

// Resource getter functions

func getSpireServerClusterRole(customLabels map[string]string) *rbacv1.ClusterRole {
	cr := utils.DecodeClusterRoleObjBytes(assets.MustAsset(utils.SpireServerClusterRoleAssetName))
	cr.Labels = utils.SpireServerLabels(customLabels)
	return cr
}

func getSpireServerClusterRoleBinding(customLabels map[string]string) *rbacv1.ClusterRoleBinding {
	crb := utils.DecodeClusterRoleBindingObjBytes(assets.MustAsset(utils.SpireServerClusterRoleBindingAssetName))
	crb.Labels = utils.SpireServerLabels(customLabels)
	// Update the subject namespace
	for i := range crb.Subjects {
		crb.Subjects[i].Namespace = utils.GetOperatorNamespace()
	}
	return crb
}

func getSpireBundleRole(customLabels map[string]string) *rbacv1.Role {
	role := utils.DecodeRoleObjBytes(assets.MustAsset(utils.SpireBundleRoleAssetName))
	role.Labels = utils.SpireServerLabels(customLabels)
	role.Namespace = utils.GetOperatorNamespace()
	return role
}

func getSpireBundleRoleBinding(customLabels map[string]string) *rbacv1.RoleBinding {
	rb := utils.DecodeRoleBindingObjBytes(assets.MustAsset(utils.SpireBundleRoleBindingAssetName))
	rb.Labels = utils.SpireServerLabels(customLabels)
	rb.Namespace = utils.GetOperatorNamespace()
	// Update the subject namespace
	for i := range rb.Subjects {
		rb.Subjects[i].Namespace = utils.GetOperatorNamespace()
	}
	return rb
}

func getSpireControllerManagerClusterRole(customLabels map[string]string) *rbacv1.ClusterRole {
	cr := utils.DecodeClusterRoleObjBytes(assets.MustAsset(utils.SpireControllerManagerClusterRoleAssetName))
	cr.Labels = utils.SpireControllerManagerLabels(customLabels)
	return cr
}

func getSpireControllerManagerClusterRoleBinding(customLabels map[string]string) *rbacv1.ClusterRoleBinding {
	crb := utils.DecodeClusterRoleBindingObjBytes(assets.MustAsset(utils.SpireControllerManagerClusterRoleBindingAssetName))
	crb.Labels = utils.SpireControllerManagerLabels(customLabels)
	// Update the subject namespace
	for i := range crb.Subjects {
		crb.Subjects[i].Namespace = utils.GetOperatorNamespace()
	}
	return crb
}

func getSpireControllerManagerLeaderElectionRole(customLabels map[string]string) *rbacv1.Role {
	role := utils.DecodeRoleObjBytes(assets.MustAsset(utils.SpireControllerManagerLeaderElectionRoleAssetName))
	role.Labels = utils.SpireControllerManagerLabels(customLabels)
	role.Namespace = utils.GetOperatorNamespace()
	return role
}

func getSpireControllerManagerLeaderElectionRoleBinding(customLabels map[string]string) *rbacv1.RoleBinding {
	rb := utils.DecodeRoleBindingObjBytes(assets.MustAsset(utils.SpireControllerManagerLeaderElectionRoleBindingAssetName))
	rb.Labels = utils.SpireControllerManagerLabels(customLabels)
	rb.Namespace = utils.GetOperatorNamespace()
	// Update the subject namespace
	for i := range rb.Subjects {
		rb.Subjects[i].Namespace = utils.GetOperatorNamespace()
	}
	return rb
}
