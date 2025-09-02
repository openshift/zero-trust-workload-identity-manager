package static_resource_controller

import (
	"context"
	"github.com/openshift/zero-trust-workload-identity-manager/pkg/controller/utils"
	"github.com/openshift/zero-trust-workload-identity-manager/pkg/operator/assets"
	rbacv1 "k8s.io/api/rbac/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func (r *StaticResourceReconciler) CreateOrApplyRbacResources(ctx context.Context, createOnlyMode bool) error {
	clusterRoleBindings := r.listStaticClusterRoleBindings()
	roles := r.listStaticRoles()
	roleBindings := r.listStaticRoleBindings()
	clusterRoles := r.listStaticClusterRoles()

	for _, clusterRole := range clusterRoles {
		err := r.createOrUpdateResource(ctx, clusterRole, createOnlyMode, "ClusterRole")
		if err != nil {
			return err
		}
	}
	for _, clusterRoleBinding := range clusterRoleBindings {
		err := r.createOrUpdateResource(ctx, clusterRoleBinding, createOnlyMode, "ClusterRoleBinding")
		if err != nil {
			return err
		}
	}
	for _, role := range roles {
		err := r.createOrUpdateResource(ctx, role, createOnlyMode, "Role")
		if err != nil {
			return err
		}
	}
	for _, roleBinding := range roleBindings {
		err := r.createOrUpdateResource(ctx, roleBinding, createOnlyMode, "RoleBinding")
		if err != nil {
			return err
		}
	}
	return nil
}

func (r *StaticResourceReconciler) createOrUpdateResource(ctx context.Context, obj client.Object, createOnlyMode bool, resourceType string) error {
	if createOnlyMode {
		err := r.ctrlClient.Create(ctx, obj)
		if err != nil && kerrors.IsAlreadyExists(err) {
			r.log.Info("Skipping update due to create-only mode", "resourceType", resourceType, "name", obj.GetName())
			return nil
		}
		if err != nil {
			r.log.Error(err, "Failed to create resource", "resourceType", resourceType, "name", obj.GetName())
			return err
		}
		r.log.Info("Created resource", "resourceType", resourceType, "name", obj.GetName())
		return nil
	}

	err := r.ctrlClient.CreateOrUpdateObject(ctx, obj)
	if err != nil {
		r.log.Error(err, "Failed to create or update resource", "resourceType", resourceType, "name", obj.GetName())
		return err
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
	spireAgentClusterRole.Labels = utils.SpireAgentLabels(spireAgentClusterRole.Labels)
	return spireAgentClusterRole
}

func (r *StaticResourceReconciler) getSpireAgentClusterRoleBinding() *rbacv1.ClusterRoleBinding {
	spireAgentClusterRoleBinding := utils.DecodeClusterRoleBindingObjBytes(assets.MustAsset(utils.SpireAgentClusterRoleBindingAssetName))
	spireAgentClusterRoleBinding.Labels = utils.SpireAgentLabels(spireAgentClusterRoleBinding.Labels)
	return spireAgentClusterRoleBinding
}

func (r *StaticResourceReconciler) getSpireBundleRole() *rbacv1.Role {
	spireBundleRole := utils.DecodeRoleObjBytes(assets.MustAsset(utils.SpireBundleRoleAssetName))
	spireBundleRole.Labels = utils.SpireServerLabels(spireBundleRole.Labels)
	return spireBundleRole
}

func (r *StaticResourceReconciler) getSpireBundleRoleBinding() *rbacv1.RoleBinding {
	spireBundleRoleBinding := utils.DecodeRoleBindingObjBytes(assets.MustAsset(utils.SpireBundleRoleBindingAssetName))
	spireBundleRoleBinding.Labels = utils.SpireServerLabels(spireBundleRoleBinding.Labels)
	return spireBundleRoleBinding
}

func (r *StaticResourceReconciler) getSpireControllerManagerClusterRole() *rbacv1.ClusterRole {
	spireControllerManagerClusterRole := utils.DecodeClusterRoleObjBytes(assets.MustAsset(utils.SpireControllerManagerClusterRoleAssetName))
	spireControllerManagerClusterRole.Labels = utils.SpireControllerManagerLabels(spireControllerManagerClusterRole.Labels)
	return spireControllerManagerClusterRole
}

func (r *StaticResourceReconciler) getSpireControllerManagerClusterRoleBinding() *rbacv1.ClusterRoleBinding {
	spireControllerManagerClusterRoleBinding := utils.DecodeClusterRoleBindingObjBytes(assets.MustAsset(utils.SpireControllerManagerClusterRoleBindingAssetName))
	spireControllerManagerClusterRoleBinding.Labels = utils.SpireControllerManagerLabels(spireControllerManagerClusterRoleBinding.Labels)
	return spireControllerManagerClusterRoleBinding
}

func (r *StaticResourceReconciler) getSpireControllerManagerLeaderElectionRole() *rbacv1.Role {
	spireControllerManagerLeaderElectionRole := utils.DecodeRoleObjBytes(assets.MustAsset(utils.SpireControllerManagerLeaderElectionRoleAssetName))
	spireControllerManagerLeaderElectionRole.Labels = utils.SpireControllerManagerLabels(spireControllerManagerLeaderElectionRole.Labels)
	return spireControllerManagerLeaderElectionRole
}

func (r *StaticResourceReconciler) getSpireControllerManagerLeaderElectionRoleBinding() *rbacv1.RoleBinding {
	spireControllerManagerLeaderElectionRoleBinding := utils.DecodeRoleBindingObjBytes(assets.MustAsset(utils.SpireControllerManagerLeaderElectionRoleBindingAssetName))
	spireControllerManagerLeaderElectionRoleBinding.Labels = utils.SpireControllerManagerLabels(spireControllerManagerLeaderElectionRoleBinding.Labels)
	return spireControllerManagerLeaderElectionRoleBinding
}

func (r *StaticResourceReconciler) getSpireServerClusterRole() *rbacv1.ClusterRole {
	spireServerClusterRole := utils.DecodeClusterRoleObjBytes(assets.MustAsset(utils.SpireServerClusterRoleAssetName))
	spireServerClusterRole.Labels = utils.SpireServerLabels(spireServerClusterRole.Labels)
	return spireServerClusterRole
}

func (r *StaticResourceReconciler) getSpireServerClusterRoleBinding() *rbacv1.ClusterRoleBinding {
	spireServerClusterRoleBinding := utils.DecodeClusterRoleBindingObjBytes(assets.MustAsset(utils.SpireServerClusterRoleBindingAssetName))
	spireServerClusterRoleBinding.Labels = utils.SpireServerLabels(spireServerClusterRoleBinding.Labels)
	return spireServerClusterRoleBinding
}
