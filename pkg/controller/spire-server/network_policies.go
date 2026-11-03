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
	utils.SpireServerNetworkPolicyEgressAllowAllAssetName,
}

var spireServerFederationNetworkPolicyAssets = []string{
	utils.SpireServerNetworkPolicyFederationIngressAssetName,
}

func (r *SpireServerReconciler) reconcileNetworkPolicies(ctx context.Context, server *v1alpha1.SpireServer, statusMgr *status.Manager, createOnlyMode bool) error {
	if !utils.ManagedNetworkPolicyEnabled(server.Spec.ManagedNetworkPolicy) {
		server.Status.NetworkPolicy = &v1alpha1.NetworkPolicyStatus{
			ManagementState: v1alpha1.NetworkPolicyManagementUnmanaged,
		}
		statusMgr.AddCondition(v1alpha1.NetworkPoliciesAvailable, v1alpha1.ReasonManagedNetworkPolicyDisabled,
			"Operator management of operand NetworkPolicies is disabled",
			metav1.ConditionFalse)
		return nil
	}

	ns := utils.GetOperatorNamespace()
	applied := make([]string, 0, len(spireServerBaselineNetworkPolicyAssets)+len(spireServerFederationNetworkPolicyAssets))
	created := false
	updated := false
	driftSkipped := false

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
		desired := networkPolicyFromAsset(assetName, server.Spec.Labels)
		if err := controllerutil.SetControllerReference(server, desired, r.scheme); err != nil {
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
			if utils.ResourceNeedsUpdate(existing, desired) {
				driftSkipped = true
				r.log.V(1).Info("Skipping NetworkPolicy update due to create-only mode", "name", desired.Name)
			}
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

	federationEnabled := server.Spec.Federation != nil
	for _, assetName := range spireServerFederationNetworkPolicyAssets {
		desired := networkPolicyFromAsset(assetName, server.Spec.Labels)
		result, err := utils.ApplyNetworkPolicy(ctx, r.ctrlClient, r.scheme, server, ns, desired.Name, federationEnabled, func() *networkingv1.NetworkPolicy {
			return networkPolicyFromAsset(assetName, server.Spec.Labels)
		}, createOnlyMode)
		if err != nil {
			statusMgr.AddCondition(v1alpha1.NetworkPoliciesAvailable, v1alpha1.ReasonNetworkPolicyCreationFailed,
				fmt.Sprintf("Failed to reconcile federation NetworkPolicy: %v", err),
				metav1.ConditionFalse)
			return err
		}
		if federationEnabled {
			mergeOutcome(result)
		} else if result.Name != "" {
			// Federation disabled: policy removed if it existed; do not list it as applied.
			if result.Updated {
				updated = true
			}
		}
	}

	server.Status.NetworkPolicy = &v1alpha1.NetworkPolicyStatus{
		ManagementState:     v1alpha1.NetworkPolicyManagementManaged,
		LastAppliedPolicies: applied,
	}

	status.SetOperandNetworkPoliciesAvailableCondition(statusMgr, server.Status.Conditions, created, updated, driftSkipped, len(applied))
	return nil
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