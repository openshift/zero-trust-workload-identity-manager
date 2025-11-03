package spire_server

import (
	"context"
	"fmt"

	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/openshift/zero-trust-workload-identity-manager/api/v1alpha1"
	"github.com/openshift/zero-trust-workload-identity-manager/pkg/controller/status"
	"github.com/openshift/zero-trust-workload-identity-manager/pkg/controller/utils"
	"github.com/openshift/zero-trust-workload-identity-manager/pkg/operator/assets"
)

// Constants for status conditions are defined in controller.go

// reconcileSpireControllerManagerStaticResources reconciles all Spire Controller Manager static resources
// Note: Service and RBAC contribute to the consolidated ServiceAvailable and RBACAvailable conditions
func (r *SpireServerReconciler) reconcileSpireControllerManagerStaticResources(ctx context.Context, server *v1alpha1.SpireServer, statusMgr *status.Manager, createOnlyMode bool) error {
	// Service
	if err := r.reconcileSpireControllerManagerService(ctx, server, statusMgr, createOnlyMode); err != nil {
		return err
	}

	// RBAC
	if err := r.reconcileSpireControllerManagerRBAC(ctx, server, statusMgr, createOnlyMode); err != nil {
		return err
	}

	// Validating Webhook
	if err := r.reconcileSpireControllerManagerWebhook(ctx, server, statusMgr, createOnlyMode); err != nil {
		return err
	}

	return nil
}

// reconcileSpireControllerManagerService reconciles the Controller Manager webhook Service
func (r *SpireServerReconciler) reconcileSpireControllerManagerService(ctx context.Context, server *v1alpha1.SpireServer, statusMgr *status.Manager, createOnlyMode bool) error {
	svc := getSpireControllerManagerWebhookService()

	if err := controllerutil.SetControllerReference(server, svc, r.scheme); err != nil {
		r.log.Error(err, "failed to set controller reference on controller manager service")
		statusMgr.AddCondition(ServiceAvailable, v1alpha1.ReasonFailed,
			fmt.Sprintf("Failed to set owner reference on Controller Manager Service: %v", err),
			metav1.ConditionFalse)
		return err
	}

	if err := r.createOrUpdateResource(ctx, svc, createOnlyMode); err != nil {
		statusMgr.AddCondition(ServiceAvailable, v1alpha1.ReasonFailed,
			fmt.Sprintf("Failed to create Controller Manager Service: %v", err),
			metav1.ConditionFalse)
		return err
	}

	// Success status is set after all Services are created
	return nil
}

// reconcileSpireControllerManagerRBAC reconciles the Controller Manager RBAC resources
func (r *SpireServerReconciler) reconcileSpireControllerManagerRBAC(ctx context.Context, server *v1alpha1.SpireServer, statusMgr *status.Manager, createOnlyMode bool) error {
	// ClusterRole
	cr := getSpireControllerManagerClusterRole()
	if err := controllerutil.SetControllerReference(server, cr, r.scheme); err != nil {
		r.log.Error(err, "failed to set controller reference on controller manager cluster role")
		statusMgr.AddCondition(RBACAvailable, v1alpha1.ReasonFailed,
			fmt.Sprintf("Failed to set owner reference on Controller Manager ClusterRole: %v", err),
			metav1.ConditionFalse)
		return err
	}
	if err := r.createOrUpdateResource(ctx, cr, createOnlyMode); err != nil {
		statusMgr.AddCondition(RBACAvailable, v1alpha1.ReasonFailed,
			fmt.Sprintf("Failed to create Controller Manager ClusterRole: %v", err),
			metav1.ConditionFalse)
		return err
	}

	// ClusterRoleBinding
	crb := getSpireControllerManagerClusterRoleBinding()
	if err := controllerutil.SetControllerReference(server, crb, r.scheme); err != nil {
		r.log.Error(err, "failed to set controller reference on controller manager cluster role binding")
		statusMgr.AddCondition(RBACAvailable, v1alpha1.ReasonFailed,
			fmt.Sprintf("Failed to set owner reference on Controller Manager ClusterRoleBinding: %v", err),
			metav1.ConditionFalse)
		return err
	}
	if err := r.createOrUpdateResource(ctx, crb, createOnlyMode); err != nil {
		statusMgr.AddCondition(RBACAvailable, v1alpha1.ReasonFailed,
			fmt.Sprintf("Failed to create Controller Manager ClusterRoleBinding: %v", err),
			metav1.ConditionFalse)
		return err
	}

	// Leader Election Role
	leaderRole := getSpireControllerManagerLeaderElectionRole()
	if err := controllerutil.SetControllerReference(server, leaderRole, r.scheme); err != nil {
		r.log.Error(err, "failed to set controller reference on leader election role")
		statusMgr.AddCondition(RBACAvailable, v1alpha1.ReasonFailed,
			fmt.Sprintf("Failed to set owner reference on Leader Election Role: %v", err),
			metav1.ConditionFalse)
		return err
	}
	if err := r.createOrUpdateResource(ctx, leaderRole, createOnlyMode); err != nil {
		statusMgr.AddCondition(RBACAvailable, v1alpha1.ReasonFailed,
			fmt.Sprintf("Failed to create Leader Election Role: %v", err),
			metav1.ConditionFalse)
		return err
	}

	// Leader Election RoleBinding
	leaderRoleBinding := getSpireControllerManagerLeaderElectionRoleBinding()
	if err := controllerutil.SetControllerReference(server, leaderRoleBinding, r.scheme); err != nil {
		r.log.Error(err, "failed to set controller reference on leader election role binding")
		statusMgr.AddCondition(RBACAvailable, v1alpha1.ReasonFailed,
			fmt.Sprintf("Failed to set owner reference on Leader Election RoleBinding: %v", err),
			metav1.ConditionFalse)
		return err
	}
	if err := r.createOrUpdateResource(ctx, leaderRoleBinding, createOnlyMode); err != nil {
		statusMgr.AddCondition(RBACAvailable, v1alpha1.ReasonFailed,
			fmt.Sprintf("Failed to create Leader Election RoleBinding: %v", err),
			metav1.ConditionFalse)
		return err
	}

	// Success status is set after ALL RBAC resources are created
	return nil
}

// reconcileSpireControllerManagerWebhook reconciles the ValidatingWebhookConfiguration
func (r *SpireServerReconciler) reconcileSpireControllerManagerWebhook(ctx context.Context, server *v1alpha1.SpireServer, statusMgr *status.Manager, createOnlyMode bool) error {
	webhook := getSpireControllerManagerValidatingWebhookConfiguration()

	if err := controllerutil.SetControllerReference(server, webhook, r.scheme); err != nil {
		r.log.Error(err, "failed to set controller reference on validating webhook")
		statusMgr.AddCondition(ValidatingWebhookAvailable, v1alpha1.ReasonFailed,
			fmt.Sprintf("Failed to set owner reference on ValidatingWebhookConfiguration: %v", err),
			metav1.ConditionFalse)
		return err
	}

	if err := r.createOrUpdateResource(ctx, webhook, createOnlyMode); err != nil {
		statusMgr.AddCondition(ValidatingWebhookAvailable, v1alpha1.ReasonFailed,
			fmt.Sprintf("Failed to create ValidatingWebhookConfiguration: %v", err),
			metav1.ConditionFalse)
		return err
	}

	statusMgr.AddCondition(ValidatingWebhookAvailable, v1alpha1.ReasonReady,
		"All ValidatingWebhookConfiguration resources available",
		metav1.ConditionTrue)
	return nil
}

func getSpireControllerManagerClusterRole() *rbacv1.ClusterRole {
	cr := utils.DecodeClusterRoleObjBytes(assets.MustAsset(utils.SpireControllerManagerClusterRoleAssetName))
	cr.Labels = utils.SpireControllerManagerLabels(cr.Labels)
	return cr
}

func getSpireControllerManagerClusterRoleBinding() *rbacv1.ClusterRoleBinding {
	crb := utils.DecodeClusterRoleBindingObjBytes(assets.MustAsset(utils.SpireControllerManagerClusterRoleBindingAssetName))
	crb.Labels = utils.SpireControllerManagerLabels(crb.Labels)
	return crb
}

func getSpireControllerManagerLeaderElectionRole() *rbacv1.Role {
	role := utils.DecodeRoleObjBytes(assets.MustAsset(utils.SpireControllerManagerLeaderElectionRoleAssetName))
	role.Labels = utils.SpireControllerManagerLabels(role.Labels)
	return role
}

func getSpireControllerManagerLeaderElectionRoleBinding() *rbacv1.RoleBinding {
	rb := utils.DecodeRoleBindingObjBytes(assets.MustAsset(utils.SpireControllerManagerLeaderElectionRoleBindingAssetName))
	rb.Labels = utils.SpireControllerManagerLabels(rb.Labels)
	return rb
}

func getSpireControllerManagerWebhookService() *corev1.Service {
	svc := utils.DecodeServiceObjBytes(assets.MustAsset(utils.SpireControllerMangerWebhookServiceAssetName))
	svc.Labels = utils.SpireControllerManagerLabels(svc.Labels)
	svc.Spec.Selector = map[string]string{
		"app.kubernetes.io/name":     "spire-controller-manager",
		"app.kubernetes.io/instance": utils.StandardInstance,
	}
	return svc
}

func getSpireControllerManagerValidatingWebhookConfiguration() *admissionregistrationv1.ValidatingWebhookConfiguration {
	webhook := utils.DecodeValidatingWebhookConfigurationByBytes(assets.MustAsset(utils.SpireControllerManagerValidatingWebhookConfigurationAssetName))
	webhook.Labels = utils.SpireControllerManagerLabels(webhook.Labels)
	return webhook
}
