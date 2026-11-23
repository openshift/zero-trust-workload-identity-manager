package spire_oidc_discovery_provider

import (
	"context"

	networkingv1 "k8s.io/api/networking/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/openshift/zero-trust-workload-identity-manager/api/v1alpha1"
	"github.com/openshift/zero-trust-workload-identity-manager/pkg/controller/utils"
	"github.com/openshift/zero-trust-workload-identity-manager/pkg/operator/assets"
)

var spireOIDCBaselineNetworkPolicyAssets = []string{
	utils.SpireOIDCDiscoveryProviderNetworkPolicyIngressAssetName,
}

func (r *SpireOidcDiscoveryProviderReconciler) reconcileNetworkPolicies(ctx context.Context, oidc *v1alpha1.SpireOIDCDiscoveryProvider, createOnlyMode bool) error {
	if !utils.ManagedNetworkPolicyEnabled(oidc.Spec.ManagedNetworkPolicy) {
		r.log.V(1).Info("Operand NetworkPolicy management is disabled")
		oidc.Status.NetworkPolicy = &v1alpha1.NetworkPolicyStatus{
			ManagementState: v1alpha1.NetworkPolicyManagementUnmanaged,
		}
		return nil
	}

	applied := utils.BaselineNetworkPolicyLastApplied(oidc.Status.NetworkPolicy, len(spireOIDCBaselineNetworkPolicyAssets))

	for _, assetName := range spireOIDCBaselineNetworkPolicyAssets {
		desired := networkPolicyFromAsset(assetName, oidc.Spec.Labels)
		if err := controllerutil.SetControllerReference(oidc, desired, r.scheme); err != nil {
			r.log.Error(err, "failed to set controller reference on NetworkPolicy", "name", desired.Name)
			return err
		}

		existing := &networkingv1.NetworkPolicy{}
		err := r.ctrlClient.Get(ctx, types.NamespacedName{Name: desired.Name, Namespace: desired.Namespace}, existing)
		if err != nil {
			if !kerrors.IsNotFound(err) {
				r.log.Error(err, "failed to get NetworkPolicy", "name", desired.Name, "namespace", desired.Namespace)
				return err
			}
			if err := r.ctrlClient.Create(ctx, desired); err != nil {
				err = utils.NetworkPolicyCreateError(err, desired)
				r.log.Error(err, "failed to create NetworkPolicy", "name", desired.Name, "namespace", desired.Namespace)
				return err
			}
			r.log.Info("Created NetworkPolicy", "name", desired.Name, "namespace", desired.Namespace)
			applied = utils.RecordNetworkPolicyApplied(applied, desired.Name)
			continue
		}

		if createOnlyMode {
			if utils.OperandManagedResourceNeedsUpdate(existing, desired, oidc) {
				r.log.V(1).Info("Skipping NetworkPolicy update due to create-only mode", "name", desired.Name)
			}
			continue
		}

		if utils.OperandManagedResourceNeedsUpdate(existing, desired, oidc) {
			desired.ResourceVersion = existing.ResourceVersion
			if err := r.ctrlClient.Update(ctx, desired); err != nil {
				r.log.Error(err, "failed to update NetworkPolicy", "name", desired.Name, "namespace", desired.Namespace)
				return err
			}
			r.log.Info("Updated NetworkPolicy", "name", desired.Name, "namespace", desired.Namespace)
			applied = utils.RecordNetworkPolicyApplied(applied, desired.Name)
			continue
		}

		r.log.V(1).Info("NetworkPolicy is up to date", "name", desired.Name)
	}

	oidc.Status.NetworkPolicy = &v1alpha1.NetworkPolicyStatus{
		ManagementState:     v1alpha1.NetworkPolicyManagementManaged,
		LastAppliedPolicies: applied,
	}
	return nil
}

func networkPolicyFromAsset(assetName string, customLabels map[string]string) *networkingv1.NetworkPolicy {
	np := utils.DecodeNetworkPolicyObjBytes(assets.MustAsset(assetName))
	np.Namespace = utils.GetOperatorNamespace()
	labels := utils.SpireOIDCDiscoveryProviderLabels(customLabels)
	for k, v := range labels {
		np.Labels[k] = v
	}
	return np
}
