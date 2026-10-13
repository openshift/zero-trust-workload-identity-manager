package spire_agent

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

var spireAgentBaselineNetworkPolicyAssets = []string{
	utils.SpireAgentNetworkPolicyIngressAssetName,
	utils.SpireAgentNetworkPolicyEgressBaselineAssetName,
}

func (r *SpireAgentReconciler) reconcileNetworkPolicies(ctx context.Context, agent *v1alpha1.SpireAgent, statusMgr *status.Manager, createOnlyMode bool) error {
	if !utils.ManagedNetworkPolicyEnabled(agent.Spec.ManagedNetworkPolicy) {
		agent.Status.NetworkPolicy = &v1alpha1.NetworkPolicyStatus{
			ManagementState: v1alpha1.NetworkPolicyManagementUnmanaged,
		}
		statusMgr.AddCondition(v1alpha1.NetworkPoliciesAvailable, v1alpha1.ReasonManagedNetworkPolicyDisabled,
			"Operator management of operand NetworkPolicies is disabled",
			metav1.ConditionFalse)
		if err := r.ctrlClient.StatusUpdateWithRetry(ctx, agent); err != nil {
			return err
		}
		return nil
	}

	applied := make([]string, 0, len(spireAgentBaselineNetworkPolicyAssets))
	created := false
	updated := false

	for _, assetName := range spireAgentBaselineNetworkPolicyAssets {
		desired := networkPolicyFromAsset(assetName, agent.Spec.Labels)
		if err := controllerutil.SetControllerReference(agent, desired, r.scheme); err != nil {
			statusMgr.AddCondition(v1alpha1.NetworkPoliciesAvailable, v1alpha1.ReasonNetworkPolicyCreationFailed,
				fmt.Sprintf("Failed to set owner reference on NetworkPolicy %s: %v", desired.Name, err),
				metav1.ConditionFalse)
			return err
		}

		existing := &networkingv1.NetworkPolicy{}
		err := r.ctrlClient.Get(ctx, types.NamespacedName{Name: desired.Name, Namespace: desired.Namespace}, existing)
		if err != nil {
			if !kerrors.IsNotFound(err) {
				statusMgr.AddCondition(v1alpha1.NetworkPoliciesAvailable, v1alpha1.ReasonNetworkPolicyCreationFailed,
					fmt.Sprintf("Failed to get NetworkPolicy %s: %v", desired.Name, err),
					metav1.ConditionFalse)
				return err
			}
			if err := r.ctrlClient.Create(ctx, desired); err != nil {
				statusMgr.AddCondition(v1alpha1.NetworkPoliciesAvailable, v1alpha1.ReasonNetworkPolicyCreationFailed,
					fmt.Sprintf("Failed to create NetworkPolicy %s: %v", desired.Name, err),
					metav1.ConditionFalse)
				return err
			}
			created = true
			applied = append(applied, desired.Name)
			continue
		}

		if createOnlyMode {
			applied = append(applied, desired.Name)
			continue
		}

		if utils.ResourceNeedsUpdate(existing, desired) {
			desired.ResourceVersion = existing.ResourceVersion
			if err := r.ctrlClient.Update(ctx, desired); err != nil {
				statusMgr.AddCondition(v1alpha1.NetworkPoliciesAvailable, v1alpha1.ReasonNetworkPolicyUpdateFailed,
					fmt.Sprintf("Failed to update NetworkPolicy %s: %v", desired.Name, err),
					metav1.ConditionFalse)
				return err
			}
			updated = true
		}
		applied = append(applied, desired.Name)
	}

	ns := utils.GetOperatorNamespace()
	proxyPorts := utils.ProxyEgressTCPPorts()
	proxyEnabled := utils.IsProxyEnabled() && len(proxyPorts) > 0
	proxyResult, err := utils.ApplyNetworkPolicy(ctx, r.ctrlClient, r.scheme, agent, ns, utils.SpireAgentNetworkPolicyProxyEgressName, proxyEnabled,
		func() *networkingv1.NetworkPolicy {
			return utils.MultiPortEgressNetworkPolicy(utils.SpireAgentNetworkPolicyProxyEgressName, ns, "spire-agent", proxyPorts, utils.SpireAgentLabels(agent.Spec.Labels))
		}, createOnlyMode)
	if err != nil {
		statusMgr.AddCondition(v1alpha1.NetworkPoliciesAvailable, v1alpha1.ReasonNetworkPolicyCreationFailed,
			fmt.Sprintf("Failed to reconcile proxy NetworkPolicy: %v", err),
			metav1.ConditionFalse)
		return err
	}
	if proxyEnabled {
		if proxyResult.Created {
			created = true
		}
		if proxyResult.Updated {
			updated = true
		}
		applied = append(applied, proxyResult.Name)
	}

	agent.Status.NetworkPolicy = &v1alpha1.NetworkPolicyStatus{
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
	if err := r.ctrlClient.StatusUpdateWithRetry(ctx, agent); err != nil {
		return err
	}
	return nil
}

func networkPolicyFromAsset(assetName string, customLabels map[string]string) *networkingv1.NetworkPolicy {
	np := utils.DecodeNetworkPolicyObjBytes(assets.MustAsset(assetName))
	np.Namespace = utils.GetOperatorNamespace()
	labels := utils.SpireAgentLabels(customLabels)
	for k, v := range labels {
		np.Labels[k] = v
	}
	return np
}
