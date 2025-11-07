package spire_agent

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

// reconcileRBAC reconciles Spire Agent RBAC resources
func (r *SpireAgentReconciler) reconcileRBAC(ctx context.Context, agent *v1alpha1.SpireAgent, statusMgr *status.Manager, createOnlyMode bool) error {
	// ClusterRole
	if err := r.reconcileClusterRole(ctx, agent, statusMgr, createOnlyMode); err != nil {
		return err
	}

	// ClusterRoleBinding
	if err := r.reconcileClusterRoleBinding(ctx, agent, statusMgr, createOnlyMode); err != nil {
		return err
	}

	// Success status is set after ALL RBAC resources are created
	// Set consolidated success status after all static resources are created
	statusMgr.AddCondition(RBACAvailable, v1alpha1.ReasonReady,
		"All RBAC resources available",
		metav1.ConditionTrue)
	return nil
}

// reconcileClusterRole reconciles the Spire Agent ClusterRole
func (r *SpireAgentReconciler) reconcileClusterRole(ctx context.Context, agent *v1alpha1.SpireAgent, statusMgr *status.Manager, createOnlyMode bool) error {
	desired := getSpireAgentClusterRole(agent.Spec.Labels)

	if err := controllerutil.SetControllerReference(agent, desired, r.scheme); err != nil {
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

// reconcileClusterRoleBinding reconciles the Spire Agent ClusterRoleBinding
func (r *SpireAgentReconciler) reconcileClusterRoleBinding(ctx context.Context, agent *v1alpha1.SpireAgent, statusMgr *status.Manager, createOnlyMode bool) error {
	desired := getSpireAgentClusterRoleBinding(agent.Spec.Labels)

	if err := controllerutil.SetControllerReference(agent, desired, r.scheme); err != nil {
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

// Resource getter functions

func getSpireAgentClusterRole(customLabels map[string]string) *rbacv1.ClusterRole {
	cr := utils.DecodeClusterRoleObjBytes(assets.MustAsset(utils.SpireAgentClusterRoleAssetName))
	cr.Labels = utils.SpireAgentLabels(customLabels)
	return cr
}

func getSpireAgentClusterRoleBinding(customLabels map[string]string) *rbacv1.ClusterRoleBinding {
	crb := utils.DecodeClusterRoleBindingObjBytes(assets.MustAsset(utils.SpireAgentClusterRoleBindingAssetName))
	crb.Labels = utils.SpireAgentLabels(customLabels)
	return crb
}
