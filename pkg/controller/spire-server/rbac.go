package spire_server

import (
	"context"
	"fmt"

	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/openshift/zero-trust-workload-identity-manager/api/v1alpha1"
	"github.com/openshift/zero-trust-workload-identity-manager/pkg/controller/status"
	"github.com/openshift/zero-trust-workload-identity-manager/pkg/controller/utils"
	"github.com/openshift/zero-trust-workload-identity-manager/pkg/operator/assets"
)

// Constants for status conditions are defined in controller.go

// reconcileRBAC reconciles all Spire Server RBAC resources
func (r *SpireServerReconciler) reconcileRBAC(ctx context.Context, server *v1alpha1.SpireServer, statusMgr *status.Manager, createOnlyMode bool) error {
	// ClusterRole
	cr := getSpireServerClusterRole()
	if err := controllerutil.SetControllerReference(server, cr, r.scheme); err != nil {
		r.log.Error(err, "failed to set controller reference on cluster role")
		statusMgr.AddCondition(RBACAvailable, v1alpha1.ReasonFailed,
			fmt.Sprintf("Failed to set owner reference on ClusterRole: %v", err),
			metav1.ConditionFalse)
		return err
	}
	if err := r.createOrUpdateResource(ctx, cr, createOnlyMode); err != nil {
		statusMgr.AddCondition(RBACAvailable, v1alpha1.ReasonFailed,
			fmt.Sprintf("Failed to create ClusterRole: %v", err),
			metav1.ConditionFalse)
		return err
	}

	// ClusterRoleBinding
	crb := getSpireServerClusterRoleBinding()
	if err := controllerutil.SetControllerReference(server, crb, r.scheme); err != nil {
		r.log.Error(err, "failed to set controller reference on cluster role binding")
		statusMgr.AddCondition(RBACAvailable, v1alpha1.ReasonFailed,
			fmt.Sprintf("Failed to set owner reference on ClusterRoleBinding: %v", err),
			metav1.ConditionFalse)
		return err
	}
	if err := r.createOrUpdateResource(ctx, crb, createOnlyMode); err != nil {
		statusMgr.AddCondition(RBACAvailable, v1alpha1.ReasonFailed,
			fmt.Sprintf("Failed to create ClusterRoleBinding: %v", err),
			metav1.ConditionFalse)
		return err
	}

	statusMgr.AddCondition(RBACAvailable, v1alpha1.ReasonReady,
		"All RBAC resources available",
		metav1.ConditionTrue)
	return nil
}

// reconcileSpireBundleRBAC reconciles Spire Bundle RBAC resources
func (r *SpireServerReconciler) reconcileSpireBundleRBAC(ctx context.Context, server *v1alpha1.SpireServer, statusMgr *status.Manager, createOnlyMode bool) error {
	// Role
	role := getSpireBundleRole()
	if err := controllerutil.SetControllerReference(server, role, r.scheme); err != nil {
		r.log.Error(err, "failed to set controller reference on spire-bundle role")
		statusMgr.AddCondition(RBACAvailable, v1alpha1.ReasonFailed,
			fmt.Sprintf("Failed to set owner reference on Bundle Role: %v", err),
			metav1.ConditionFalse)
		return err
	}
	if err := r.createOrUpdateResource(ctx, role, createOnlyMode); err != nil {
		statusMgr.AddCondition(RBACAvailable, v1alpha1.ReasonFailed,
			fmt.Sprintf("Failed to create Bundle Role: %v", err),
			metav1.ConditionFalse)
		return err
	}

	// RoleBinding
	roleBinding := getSpireBundleRoleBinding()
	if err := controllerutil.SetControllerReference(server, roleBinding, r.scheme); err != nil {
		r.log.Error(err, "failed to set controller reference on spire-bundle role binding")
		statusMgr.AddCondition(RBACAvailable, v1alpha1.ReasonFailed,
			fmt.Sprintf("Failed to set owner reference on Bundle RoleBinding: %v", err),
			metav1.ConditionFalse)
		return err
	}
	if err := r.createOrUpdateResource(ctx, roleBinding, createOnlyMode); err != nil {
		statusMgr.AddCondition(RBACAvailable, v1alpha1.ReasonFailed,
			fmt.Sprintf("Failed to create Bundle RoleBinding: %v", err),
			metav1.ConditionFalse)
		return err
	}

	// Success is set after all RBAC resources (including bundle) are created
	return nil
}

// Resource getter functions

func getSpireServerClusterRole() *rbacv1.ClusterRole {
	cr := utils.DecodeClusterRoleObjBytes(assets.MustAsset(utils.SpireServerClusterRoleAssetName))
	cr.Labels = utils.SpireServerLabels(cr.Labels)
	return cr
}

func getSpireServerClusterRoleBinding() *rbacv1.ClusterRoleBinding {
	crb := utils.DecodeClusterRoleBindingObjBytes(assets.MustAsset(utils.SpireServerClusterRoleBindingAssetName))
	crb.Labels = utils.SpireServerLabels(crb.Labels)
	return crb
}

func getSpireBundleRole() *rbacv1.Role {
	role := utils.DecodeRoleObjBytes(assets.MustAsset(utils.SpireBundleRoleAssetName))
	role.Labels = utils.SpireServerLabels(role.Labels)
	return role
}

func getSpireBundleRoleBinding() *rbacv1.RoleBinding {
	rb := utils.DecodeRoleBindingObjBytes(assets.MustAsset(utils.SpireBundleRoleBindingAssetName))
	rb.Labels = utils.SpireServerLabels(rb.Labels)
	return rb
}
