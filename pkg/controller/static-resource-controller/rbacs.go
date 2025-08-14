package static_resource_controller

import (
	"context"
	"github.com/openshift/zero-trust-workload-identity-manager/pkg/controller/utils"
	"github.com/openshift/zero-trust-workload-identity-manager/pkg/operator/assets"
	rbacv1 "k8s.io/api/rbac/v1"

	"github.com/openshift/zero-trust-workload-identity-manager/pkg/version"
)

func (r *StaticResourceReconciler) CreateOrApplyRbacResources(ctx context.Context) error {
	clusterRoleBindings := r.listStaticClusterRoleBindings()
	roles := r.listStaticRoles()
	roleBindings := r.listStaticRoleBindings()
	clusterRoles := r.listStaticClusterRoles()

	for _, clusterRole := range clusterRoles {
		err := r.ctrlClient.CreateOrUpdateObject(ctx, clusterRole)
		if err != nil {
			r.log.Error(err, "Failed to create or update ClusterRole object")
			return err
		}
	}
	for _, clusterRoleBinding := range clusterRoleBindings {
		err := r.ctrlClient.CreateOrUpdateObject(ctx, clusterRoleBinding)
		if err != nil {
			r.log.Error(err, "Failed to create or update ClusterRoleBinding object")
			return err
		}
	}
	for _, role := range roles {
		err := r.ctrlClient.CreateOrUpdateObject(ctx, role)
		if err != nil {
			r.log.Error(err, "Failed to create or update Role object")
			return err
		}
	}
	for _, roleBinding := range roleBindings {
		err := r.ctrlClient.CreateOrUpdateObject(ctx, roleBinding)
		if err != nil {
			r.log.Error(err, "Failed to create or update RoleBinding object")
			return err
		}
	}
	return nil
}

func (r *StaticResourceReconciler) listStaticClusterRoles() []*rbacv1.ClusterRole {
	clusterRoles := []*rbacv1.ClusterRole{}
	clusterRoles = append(clusterRoles, r.getSpireAgentClusterRole(), r.getSpireServerClusterRole(), r.getSpireControllerManagerClusterRole())
	return clusterRoles
}

func (r *StaticResourceReconciler) listStaticClusterRoleBindings() []*rbacv1.ClusterRoleBinding {
	clusterRoleBindings := []*rbacv1.ClusterRoleBinding{}
	clusterRoleBindings = append(clusterRoleBindings, r.getSpireAgentClusterRoleBinding(), r.getSpireServerClusterRoleBinding(), r.getSpireControllerManagerClusterRoleBinding())
	return clusterRoleBindings
}

func (r *StaticResourceReconciler) listStaticRoles() []*rbacv1.Role {
	roles := []*rbacv1.Role{}
	roles = append(roles, r.getSpireBundleRole(), r.getSpireControllerManagerLeaderElectionRole())
	return roles
}

func (r *StaticResourceReconciler) listStaticRoleBindings() []*rbacv1.RoleBinding {
	roleBindings := []*rbacv1.RoleBinding{}
	roleBindings = append(roleBindings, r.getSpireBundleRoleBinding(), r.getSpireControllerManagerLeaderElectionRoleBinding())
	return roleBindings
}

func (r *StaticResourceReconciler) getSpireAgentClusterRole() *rbacv1.ClusterRole {
	spireAgentClusterRole := utils.DecodeClusterRoleObjBytes(assets.MustAsset(utils.SpireAgentClusterRoleAssetName))
	spireAgentClusterRole.Labels = utils.SetLabel(spireAgentClusterRole.Labels, "app.kubernetes.io/version", version.SpireAgentVersion)
	return spireAgentClusterRole
}

func (r *StaticResourceReconciler) getSpireAgentClusterRoleBinding() *rbacv1.ClusterRoleBinding {
	spireAgentClusterRoleBinding := utils.DecodeClusterRoleBindingObjBytes(assets.MustAsset(utils.SpireAgentClusterRoleBindingAssetName))
	spireAgentClusterRoleBinding.Labels = utils.SetLabel(spireAgentClusterRoleBinding.Labels, "app.kubernetes.io/version", version.SpireAgentVersion)
	return spireAgentClusterRoleBinding
}

func (r *StaticResourceReconciler) getSpireBundleRole() *rbacv1.Role {
	spireBundleRole := utils.DecodeRoleObjBytes(assets.MustAsset(utils.SpireBundleRoleAssetName))
	spireBundleRole.Labels = utils.SetLabel(spireBundleRole.Labels, "app.kubernetes.io/version", version.SpireServerVersion)
	return spireBundleRole
}

func (r *StaticResourceReconciler) getSpireBundleRoleBinding() *rbacv1.RoleBinding {
	spireBundleRoleBinding := utils.DecodeRoleBindingObjBytes(assets.MustAsset(utils.SpireBundleRoleBindingAssetName))
	spireBundleRoleBinding.Labels = utils.SetLabel(spireBundleRoleBinding.Labels, "app.kubernetes.io/version", version.SpireServerVersion)
	return spireBundleRoleBinding
}

func (r *StaticResourceReconciler) getSpireControllerManagerClusterRole() *rbacv1.ClusterRole {
	spireControllerManagerClusterRole := utils.DecodeClusterRoleObjBytes(assets.MustAsset(utils.SpireControllerManagerClusterRoleAssetName))
	spireControllerManagerClusterRole.Labels = utils.SetLabel(spireControllerManagerClusterRole.Labels, "app.kubernetes.io/version", version.SpireControllerManagerVersion)
	return spireControllerManagerClusterRole
}

func (r *StaticResourceReconciler) getSpireControllerManagerClusterRoleBinding() *rbacv1.ClusterRoleBinding {
	spireControllerManagerClusterRoleBinding := utils.DecodeClusterRoleBindingObjBytes(assets.MustAsset(utils.SpireControllerManagerClusterRoleBindingAssetName))
	spireControllerManagerClusterRoleBinding.Labels = utils.SetLabel(spireControllerManagerClusterRoleBinding.Labels, "app.kubernetes.io/version", version.SpireControllerManagerVersion)
	return spireControllerManagerClusterRoleBinding
}

func (r *StaticResourceReconciler) getSpireControllerManagerLeaderElectionRole() *rbacv1.Role {
	spireControllerManagerLeaderElectionRole := utils.DecodeRoleObjBytes(assets.MustAsset(utils.SpireControllerManagerLeaderElectionRoleAssetName))
	spireControllerManagerLeaderElectionRole.Labels = utils.SetLabel(spireControllerManagerLeaderElectionRole.Labels, "app.kubernetes.io/version", version.SpireControllerManagerVersion)
	return spireControllerManagerLeaderElectionRole
}

func (r *StaticResourceReconciler) getSpireControllerManagerLeaderElectionRoleBinding() *rbacv1.RoleBinding {
	spireControllerManagerLeaderElectionRoleBinding := utils.DecodeRoleBindingObjBytes(assets.MustAsset(utils.SpireControllerManagerLeaderElectionRoleBindingAssetName))
	spireControllerManagerLeaderElectionRoleBinding.Labels = utils.SetLabel(spireControllerManagerLeaderElectionRoleBinding.Labels, "app.kubernetes.io/version", version.SpireControllerManagerVersion)
	return spireControllerManagerLeaderElectionRoleBinding
}

func (r *StaticResourceReconciler) getSpireServerClusterRole() *rbacv1.ClusterRole {
	spireServerClusterRole := utils.DecodeClusterRoleObjBytes(assets.MustAsset(utils.SpireServerClusterRoleAssetName))
	spireServerClusterRole.Labels = utils.SetLabel(spireServerClusterRole.Labels, "app.kubernetes.io/version", version.SpireServerVersion)
	return spireServerClusterRole
}

func (r *StaticResourceReconciler) getSpireServerClusterRoleBinding() *rbacv1.ClusterRoleBinding {
	spireServerClusterRoleBinding := utils.DecodeClusterRoleBindingObjBytes(assets.MustAsset(utils.SpireServerClusterRoleBindingAssetName))
	spireServerClusterRoleBinding.Labels = utils.SetLabel(spireServerClusterRoleBinding.Labels, "app.kubernetes.io/version", version.SpireServerVersion)
	return spireServerClusterRoleBinding
}
