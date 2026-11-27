package spire_server

import (
	"context"

	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/openshift/zero-trust-workload-identity-manager/api/v1alpha1"
	"github.com/openshift/zero-trust-workload-identity-manager/pkg/controller/utils"
	"github.com/openshift/zero-trust-workload-identity-manager/pkg/operator/assets"
)

var spireServerBaselineNetworkPolicyAssets = []string{
	utils.SpireServerNetworkPolicyIngressAssetName,
}

func (r *SpireServerReconciler) reconcileNetworkPolicies(ctx context.Context, server *v1alpha1.SpireServer, createOnlyMode bool) error {
	applied := utils.BaselineNetworkPolicyLastApplied(server.Status.NetworkPolicy, len(spireServerBaselineNetworkPolicyAssets))
	if !utils.ManagedNetworkPolicyEnabled(server.Spec.ManagedNetworkPolicy) {
		r.log.V(1).Info("Operand NetworkPolicy management is disabled")
		server.Status.NetworkPolicy = utils.NetworkPolicyStatusWith(v1alpha1.NetworkPolicyManagementUnmanaged, applied)
		return nil
	}

	defer func() {
		server.Status.NetworkPolicy = utils.NetworkPolicyStatusWith(v1alpha1.NetworkPolicyManagementManaged, applied)
	}()

	ns := utils.GetOperatorNamespace()
	for _, assetName := range spireServerBaselineNetworkPolicyAssets {
		desiredName := networkPolicyFromAsset(assetName, server.Spec.Labels).Name
		result, err := utils.ApplyNetworkPolicy(ctx, r.ctrlClient, r.scheme, server, ns, desiredName, true, func() *networkingv1.NetworkPolicy {
			return networkPolicyFromAsset(assetName, server.Spec.Labels)
		}, createOnlyMode)
		if err != nil {
			r.log.Error(err, "failed to reconcile NetworkPolicy", "name", desiredName, "namespace", ns)
			return err
		}
		if result.Created {
			r.log.Info("Created NetworkPolicy", "name", result.Name, "namespace", ns)
			applied = utils.RecordNetworkPolicyApplied(applied, result.Name, metav1.Now(), true)
		} else if result.Updated {
			r.log.Info("Updated NetworkPolicy", "name", result.Name, "namespace", ns)
			applied = utils.RecordNetworkPolicyApplied(applied, result.Name, metav1.Now(), true)
		} else {
			if !createOnlyMode {
				r.log.V(1).Info("NetworkPolicy is up to date", "name", result.Name)
			} else {
				r.log.V(1).Info("Skipping NetworkPolicy update due to create-only mode", "name", result.Name)
			}
			applied = utils.RecordNetworkPolicyApplied(applied, result.Name, result.CreationTimestamp, false)
		}
	}

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
