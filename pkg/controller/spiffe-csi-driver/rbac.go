package spiffe_csi_driver

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

const (
	privilegedSCCClusterRoleName = "system:openshift:scc:privileged"
	privilegedSCCRoleBindingName = "spire-csi-use-privileged-scc"
)

// reconcilePrivilegedRoleBinding binds the CSI driver ServiceAccount to the platform privileged SCC ClusterRole.
func (r *SpiffeCsiReconciler) reconcilePrivilegedRoleBinding(ctx context.Context, driver *v1alpha1.SpiffeCSIDriver, statusMgr *status.Manager, createOnlyMode bool) error {
	desired := getSpiffeCSIDriverPrivilegedRoleBinding(driver.Spec.Labels)

	if err := controllerutil.SetControllerReference(driver, desired, r.scheme); err != nil {
		r.log.Error(err, "failed to set controller reference on privileged RoleBinding")
		statusMgr.AddCondition(SecurityContextConstraintsAvailable, "SpiffeCSIPrivilegedRoleBindingGenerationFailed",
			err.Error(),
			metav1.ConditionFalse)
		return err
	}

	existing := &rbacv1.RoleBinding{}
	err := r.ctrlClient.Get(ctx, types.NamespacedName{Name: desired.Name, Namespace: desired.Namespace}, existing)
	if err != nil {
		if !kerrors.IsNotFound(err) {
			r.log.Error(err, "failed to get privileged RoleBinding")
			statusMgr.AddCondition(SecurityContextConstraintsAvailable, "SpiffeCSIPrivilegedRoleBindingGetFailed",
				fmt.Sprintf("Failed to get privileged RoleBinding: %v", err),
				metav1.ConditionFalse)
			return err
		}

		if err := r.ctrlClient.Create(ctx, desired); err != nil {
			if conflictErr := utils.HandleCreateConflict(err, desired, r.log, statusMgr, SecurityContextConstraintsAvailable); conflictErr != nil {
				return conflictErr
			}
			r.log.Error(err, "failed to create privileged RoleBinding")
			statusMgr.AddCondition(SecurityContextConstraintsAvailable, "SpiffeCSIPrivilegedRoleBindingCreationFailed",
				err.Error(),
				metav1.ConditionFalse)
			return err
		}

		r.log.Info("Created privileged RoleBinding", "name", desired.Name, "namespace", desired.Namespace)
		statusMgr.AddCondition(SecurityContextConstraintsAvailable, "SpiffeCSIPrivilegedRoleBindingCreated",
			"Privileged SCC RoleBinding created",
			metav1.ConditionTrue)
		return nil
	}

	if createOnlyMode {
		r.log.V(1).Info("Privileged RoleBinding exists, skipping update due to create-only mode", "name", desired.Name)
		return nil
	}

	if !utils.ResourceNeedsUpdate(existing, desired) {
		r.log.V(1).Info("Privileged RoleBinding is up to date", "name", desired.Name)
		statusMgr.AddCondition(SecurityContextConstraintsAvailable, "SpiffeCSIPrivilegedRoleBindingUpToDate",
			"Privileged SCC RoleBinding is up to date",
			metav1.ConditionTrue)
		return nil
	}

	desired.ResourceVersion = existing.ResourceVersion
	if err := r.ctrlClient.Update(ctx, desired); err != nil {
		r.log.Error(err, "failed to update privileged RoleBinding")
		statusMgr.AddCondition(SecurityContextConstraintsAvailable, "SpiffeCSIPrivilegedRoleBindingUpdateFailed",
			fmt.Sprintf("Failed to update privileged RoleBinding: %v", err),
			metav1.ConditionFalse)
		return err
	}

	r.log.Info("Updated privileged RoleBinding", "name", desired.Name, "namespace", desired.Namespace)
	statusMgr.AddCondition(SecurityContextConstraintsAvailable, "SpiffeCSIPrivilegedRoleBindingUpdated",
		"Privileged SCC RoleBinding updated",
		metav1.ConditionTrue)
	return nil
}

func getSpiffeCSIDriverPrivilegedRoleBinding(customLabels map[string]string) *rbacv1.RoleBinding {
	rb := utils.DecodeRoleBindingObjBytes(assets.MustAsset(utils.SpiffeCsiDriverPrivilegedRoleBindingAssetName))
	rb.Labels = utils.SpiffeCSIDriverLabels(customLabels)
	rb.Namespace = utils.GetOperatorNamespace()
	rb.Name = privilegedSCCRoleBindingName
	rb.Subjects[0].Namespace = utils.GetOperatorNamespace()
	rb.RoleRef.Name = privilegedSCCClusterRoleName
	return rb
}
