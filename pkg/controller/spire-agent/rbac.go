package spire_agent

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

// reconcileRBAC reconciles Spire Agent RBAC resources
func (r *SpireAgentReconciler) reconcileRBAC(ctx context.Context, agent *v1alpha1.SpireAgent, statusMgr *status.Manager, createOnlyMode bool) error {
	// ClusterRole
	cr := getSpireAgentClusterRole()
	if err := controllerutil.SetControllerReference(agent, cr, r.scheme); err != nil {
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
	crb := getSpireAgentClusterRoleBinding()
	if err := controllerutil.SetControllerReference(agent, crb, r.scheme); err != nil {
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

	// Success status is set after ALL RBAC resources are created
	return nil
}

// Resource getter functions

func getSpireAgentClusterRole() *rbacv1.ClusterRole {
	cr := utils.DecodeClusterRoleObjBytes(assets.MustAsset(utils.SpireAgentClusterRoleAssetName))
	cr.Labels = utils.SpireAgentLabels(cr.Labels)
	return cr
}

func getSpireAgentClusterRoleBinding() *rbacv1.ClusterRoleBinding {
	crb := utils.DecodeClusterRoleBindingObjBytes(assets.MustAsset(utils.SpireAgentClusterRoleBindingAssetName))
	crb.Labels = utils.SpireAgentLabels(crb.Labels)
	return crb
}
