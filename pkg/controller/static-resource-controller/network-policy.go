package static_resource_controller

import (
	"context"

	v1 "k8s.io/api/networking/v1"

	"github.com/openshift/zero-trust-workload-identity-manager/pkg/controller/utils"
	"github.com/openshift/zero-trust-workload-identity-manager/pkg/operator/assets"
)

func (r *StaticResourceReconciler) CreateOrApplyNetworkPolicyResources(ctx context.Context, createOnlyMode bool) error {
	for _, networkPolicy := range r.listStaticNetworkPolicy() {
		err := r.createOrUpdateResource(ctx, networkPolicy, createOnlyMode, "NetworkPolicy")
		if err != nil {
			return err
		}
	}
	return nil
}

func (r *StaticResourceReconciler) listStaticNetworkPolicy() []*v1.NetworkPolicy {
	staticNetworkPolicies := []*v1.NetworkPolicy{}

	// Add default deny policy
	staticNetworkPolicies = append(staticNetworkPolicies, r.getDefaultDenyNetworkPolicy())

	// Add spire-agent policies
	staticNetworkPolicies = append(staticNetworkPolicies,
		r.getSpireAgentAllowEgressToApiServerNetworkPolicy(),
		r.getSpireAgentAllowEgressToSpireServerNetworkPolicy(),
		r.getSpireAgentAllowIngressToMetricsNetworkPolicy(),
	)

	// Add spire-oidc-discovery-provider policies
	staticNetworkPolicies = append(staticNetworkPolicies,
		r.getSpireOIDCDiscoveryProviderAllowIngressTo8443NetworkPolicy(),
	)

	// Add spire-server policies
	staticNetworkPolicies = append(staticNetworkPolicies,
		r.getSpireServerAllowEgressIngressToFederationNetworkPolicy(),
		r.getSpireServerAllowEgressToApiServerNetworkPolicy(),
		r.getSpireServerAllowEgressToDNSNetworkPolicy(),
		r.getSpireServerAllowIngressTo8081NetworkPolicy(),
		r.getSpireServerAllowIngressToMetricsNetworkPolicy(),
		r.getSpireServerAllowIngressToWebhookNetworkPolicy(),
	)

	return staticNetworkPolicies
}

// Default deny network policy
func (r *StaticResourceReconciler) getDefaultDenyNetworkPolicy() *v1.NetworkPolicy {
	defaultDenyNetworkPolicy := utils.DecodeNetworkPolicyObjBytes(assets.MustAsset(utils.DefaultDenyNetworkPolicyAssetName))
	// Apply standardized labels - this policy is general, so use basic managed-by label
	defaultDenyNetworkPolicy.Labels = utils.SetLabel(defaultDenyNetworkPolicy.Labels, utils.AppManagedByLabelKey, utils.AppManagedByLabelValue)
	return defaultDenyNetworkPolicy
}

// Spire Agent network policies
func (r *StaticResourceReconciler) getSpireAgentAllowEgressToApiServerNetworkPolicy() *v1.NetworkPolicy {
	spireAgentNetworkPolicy := utils.DecodeNetworkPolicyObjBytes(assets.MustAsset(utils.SpireAgentAllowEgressToApiServerNetworkPolicyAssetName))
	spireAgentNetworkPolicy.Labels = utils.SpireAgentLabels(spireAgentNetworkPolicy.Labels)
	return spireAgentNetworkPolicy
}

func (r *StaticResourceReconciler) getSpireAgentAllowEgressToSpireServerNetworkPolicy() *v1.NetworkPolicy {
	spireAgentNetworkPolicy := utils.DecodeNetworkPolicyObjBytes(assets.MustAsset(utils.SpireAgentAllowEgressToSpireServerNetworkPolicyAssetName))
	spireAgentNetworkPolicy.Labels = utils.SpireAgentLabels(spireAgentNetworkPolicy.Labels)
	return spireAgentNetworkPolicy
}

func (r *StaticResourceReconciler) getSpireAgentAllowIngressToMetricsNetworkPolicy() *v1.NetworkPolicy {
	spireAgentNetworkPolicy := utils.DecodeNetworkPolicyObjBytes(assets.MustAsset(utils.SpireAgentAllowIngressToMetricsNetworkPolicyAssetName))
	spireAgentNetworkPolicy.Labels = utils.SpireAgentLabels(spireAgentNetworkPolicy.Labels)
	return spireAgentNetworkPolicy
}

// Spire OIDC Discovery Provider network policies
func (r *StaticResourceReconciler) getSpireOIDCDiscoveryProviderAllowIngressTo8443NetworkPolicy() *v1.NetworkPolicy {
	spireOIDCNetworkPolicy := utils.DecodeNetworkPolicyObjBytes(assets.MustAsset(utils.SpireOIDCDiscoveryProviderAllowIngressTo8443NetworkPolicyAssetName))
	spireOIDCNetworkPolicy.Labels = utils.SpireOIDCDiscoveryProviderLabels(spireOIDCNetworkPolicy.Labels)
	return spireOIDCNetworkPolicy
}

// Spire Server network policies
func (r *StaticResourceReconciler) getSpireServerAllowEgressIngressToFederationNetworkPolicy() *v1.NetworkPolicy {
	spireServerNetworkPolicy := utils.DecodeNetworkPolicyObjBytes(assets.MustAsset(utils.SpireServerAllowEgressIngressToFederationNetworkPolicyAssetName))
	spireServerNetworkPolicy.Labels = utils.SpireServerLabels(spireServerNetworkPolicy.Labels)
	return spireServerNetworkPolicy
}

func (r *StaticResourceReconciler) getSpireServerAllowEgressToApiServerNetworkPolicy() *v1.NetworkPolicy {
	spireServerNetworkPolicy := utils.DecodeNetworkPolicyObjBytes(assets.MustAsset(utils.SpireServerAllowEgressToApiServerNetworkPolicyAssetName))
	spireServerNetworkPolicy.Labels = utils.SpireServerLabels(spireServerNetworkPolicy.Labels)
	return spireServerNetworkPolicy
}

func (r *StaticResourceReconciler) getSpireServerAllowEgressToDNSNetworkPolicy() *v1.NetworkPolicy {
	spireServerNetworkPolicy := utils.DecodeNetworkPolicyObjBytes(assets.MustAsset(utils.SpireServerAllowEgressToDNSNetworkPolicyAssetName))
	spireServerNetworkPolicy.Labels = utils.SpireServerLabels(spireServerNetworkPolicy.Labels)
	return spireServerNetworkPolicy
}

func (r *StaticResourceReconciler) getSpireServerAllowIngressTo8081NetworkPolicy() *v1.NetworkPolicy {
	spireServerNetworkPolicy := utils.DecodeNetworkPolicyObjBytes(assets.MustAsset(utils.SpireServerAllowIngressTo8081NetworkPolicyAssetName))
	spireServerNetworkPolicy.Labels = utils.SpireServerLabels(spireServerNetworkPolicy.Labels)
	return spireServerNetworkPolicy
}

func (r *StaticResourceReconciler) getSpireServerAllowIngressToMetricsNetworkPolicy() *v1.NetworkPolicy {
	spireServerNetworkPolicy := utils.DecodeNetworkPolicyObjBytes(assets.MustAsset(utils.SpireServerAllowIngressToMetricsNetworkPolicyAssetName))
	spireServerNetworkPolicy.Labels = utils.SpireServerLabels(spireServerNetworkPolicy.Labels)
	return spireServerNetworkPolicy
}

func (r *StaticResourceReconciler) getSpireServerAllowIngressToWebhookNetworkPolicy() *v1.NetworkPolicy {
	spireServerNetworkPolicy := utils.DecodeNetworkPolicyObjBytes(assets.MustAsset(utils.SpireServerAllowIngressToWebhookNetworkPolicyAssetName))
	spireServerNetworkPolicy.Labels = utils.SpireServerLabels(spireServerNetworkPolicy.Labels)
	return spireServerNetworkPolicy
}
