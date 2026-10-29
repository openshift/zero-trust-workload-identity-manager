package spire_server

import (
	"context"
	"fmt"

	networkingv1 "k8s.io/api/networking/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/openshift/zero-trust-workload-identity-manager/api/v1alpha1"
	"github.com/openshift/zero-trust-workload-identity-manager/pkg/controller/status"
	"github.com/openshift/zero-trust-workload-identity-manager/pkg/controller/utils"
	"github.com/openshift/zero-trust-workload-identity-manager/pkg/operator/assets"
)

var spireServerBaselineNetworkPolicyAssets = []string{
	utils.SpireServerNetworkPolicyIngressAssetName,
	utils.SpireServerNetworkPolicyEgressBaselineAssetName,
	utils.SpireServerNetworkPolicyEgressAllowAllAssetName,
}

var spireServerFederationNetworkPolicyAssets = []string{
	utils.SpireServerNetworkPolicyFederationIngressAssetName,
}

var spireServerLegacyPortScopedEgressNetworkPolicyNames = []string{
	utils.SpireServerNetworkPolicyFederationEgressName,
	utils.SpireServerNetworkPolicyVaultEgressName,
	utils.SpireServerNetworkPolicyDatabaseEgressName,
	utils.SpireServerNetworkPolicyProxyEgressName,
}

func (r *SpireServerReconciler) reconcileNetworkPolicies(ctx context.Context, server *v1alpha1.SpireServer, statusMgr *status.Manager, createOnlyMode bool) error {
	if !utils.ManagedNetworkPolicyEnabled(server.Spec.ManagedNetworkPolicy) {
		server.Status.NetworkPolicy = &v1alpha1.NetworkPolicyStatus{
			ManagementState: v1alpha1.NetworkPolicyManagementUnmanaged,
		}
		statusMgr.AddCondition(v1alpha1.NetworkPoliciesAvailable, v1alpha1.ReasonManagedNetworkPolicyDisabled,
			"Operator management of operand NetworkPolicies is disabled",
			metav1.ConditionFalse)
		if err := r.ctrlClient.StatusUpdateWithRetry(ctx, server); err != nil {
			return err
		}
		return nil
	}

	ns := utils.GetOperatorNamespace()
	applied := make([]string, 0, 8)
	created := false
	updated := false

	mergeOutcome := func(result utils.NetworkPolicyApplyResult) {
		if result.Name != "" {
			applied = append(applied, result.Name)
		}
		if result.Created {
			created = true
		}
		if result.Updated {
			updated = true
		}
	}

	for _, assetName := range spireServerBaselineNetworkPolicyAssets {
		result, err := r.applyNetworkPolicyFromAsset(ctx, server, assetName, server.Spec.Labels, createOnlyMode)
		if err != nil {
			return r.networkPolicyError(statusMgr, err, "create/update")
		}
		mergeOutcome(result)
	}

	federationEnabled := server.Spec.Federation != nil
	for _, assetName := range spireServerFederationNetworkPolicyAssets {
		result, err := utils.ApplyNetworkPolicy(ctx, r.ctrlClient, r.scheme, server, ns, networkPolicyFromAsset(assetName, server.Spec.Labels).Name, federationEnabled, func() *networkingv1.NetworkPolicy {
			return networkPolicyFromAsset(assetName, server.Spec.Labels)
		}, createOnlyMode)
		if err != nil {
			return r.networkPolicyError(statusMgr, err, "federation")
		}
		if federationEnabled {
			mergeOutcome(result)
		}
	}

	for _, name := range spireServerLegacyPortScopedEgressNetworkPolicyNames {
		if err := utils.DeleteNetworkPolicyIfExists(ctx, r.ctrlClient, ns, name); err != nil {
			return r.networkPolicyError(statusMgr, err, "legacy egress cleanup")
		}
	}

	server.Status.NetworkPolicy = &v1alpha1.NetworkPolicyStatus{
		ManagementState: v1alpha1.NetworkPolicyManagementManaged,
		AppliedPolicies: applied,
	}

	reason := v1alpha1.ReasonNetworkPoliciesUpdated
	message := fmt.Sprintf("Applied %d operator NetworkPolicies", len(applied))
	if created && !updated {
		reason = v1alpha1.ReasonNetworkPoliciesCreated
		message = fmt.Sprintf("Created %d operator NetworkPolicies", len(applied))
	} else if updated {
		message = fmt.Sprintf("Updated operator NetworkPolicies (%d total)", len(applied))
	}

	statusMgr.AddCondition(v1alpha1.NetworkPoliciesAvailable, reason, message, metav1.ConditionTrue)
	if err := r.ctrlClient.StatusUpdateWithRetry(ctx, server); err != nil {
		return err
	}
	return nil
}

func (r *SpireServerReconciler) applyNetworkPolicyFromAsset(ctx context.Context, server *v1alpha1.SpireServer, assetName string, customLabels map[string]string, createOnlyMode bool) (utils.NetworkPolicyApplyResult, error) {
	desired := networkPolicyFromAsset(assetName, customLabels)
	return r.applyNetworkPolicyDesired(ctx, server, desired, createOnlyMode)
}

func (r *SpireServerReconciler) applyNetworkPolicyDesired(ctx context.Context, server *v1alpha1.SpireServer, desired *networkingv1.NetworkPolicy, createOnlyMode bool) (utils.NetworkPolicyApplyResult, error) {
	if err := controllerutil.SetControllerReference(server, desired, r.scheme); err != nil {
		return utils.NetworkPolicyApplyResult{}, err
	}

	existing := &networkingv1.NetworkPolicy{}
	err := r.ctrlClient.Get(ctx, types.NamespacedName{Name: desired.Name, Namespace: desired.Namespace}, existing)
	if err != nil {
		if !kerrors.IsNotFound(err) {
			return utils.NetworkPolicyApplyResult{}, err
		}
		if err := r.ctrlClient.Create(ctx, desired); err != nil {
			return utils.NetworkPolicyApplyResult{}, err
		}
		return utils.NetworkPolicyApplyResult{Name: desired.Name, Created: true}, nil
	}

	if createOnlyMode {
		return utils.NetworkPolicyApplyResult{Name: desired.Name}, nil
	}

	if utils.ResourceNeedsUpdate(existing, desired) {
		desired.ResourceVersion = existing.ResourceVersion
		if err := r.ctrlClient.Update(ctx, desired); err != nil {
			return utils.NetworkPolicyApplyResult{}, err
		}
		return utils.NetworkPolicyApplyResult{Name: desired.Name, Updated: true}, nil
	}
	return utils.NetworkPolicyApplyResult{Name: desired.Name}, nil
}

func (r *SpireServerReconciler) networkPolicyError(statusMgr *status.Manager, err error, context string) error {
	statusMgr.AddCondition(v1alpha1.NetworkPoliciesAvailable, v1alpha1.ReasonNetworkPolicyCreationFailed,
		fmt.Sprintf("Failed to reconcile NetworkPolicy (%s): %v", context, err),
		metav1.ConditionFalse)
	return err
}

func networkPolicyFromAsset(assetName string, customLabels map[string]string) *networkingv1.NetworkPolicy {
	np := utils.DecodeNetworkPolicyObjBytes(assets.MustAsset(assetName))
	np.Namespace = utils.GetOperatorNamespace()
	labels := utils.SpireServerLabels(customLabels)
	for k, v := range labels {
		np.Labels[k] = v
	}
	return np
}
