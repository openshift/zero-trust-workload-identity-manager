package utils

const (
	ZeroTrustWorkloadIdentityManagerStaticResourceControllerName = "zero-trust-workload-identity-manager-static-resource-controller-controller"

	AppManagedByLabelKey   = "app.kubernetes.io/managed-by"
	AppManagedByLabelValue = "zero-trust-workload-identity-manager"

	// CSI ASSET PATH
	SpiffeCsiDriverAssetName = "spiffe-csi/spiffe-csi-csi-driver.yaml"

	// RBAC ASSET PATH
	SpireAgentClusterRoleAssetName                           = "spire-agent/spire-agent-cluster-role.yaml"
	SpireAgentClusterRoleBindingAssetName                    = "spire-agent/spire-agent-cluster-role-binding.yaml"
	SpireBundleRoleAssetName                                 = "spire-bundle/spire-bundle-role.yaml"
	SpireBundleRoleBindingAssetName                          = "spire-bundle/spire-bundle-role-binding.yaml"
	SpireControllerManagerClusterRoleAssetName               = "spire-controller-manager/spire-controller-manager-cluster-role.yaml"
	SpireControllerManagerClusterRoleBindingAssetName        = "spire-controller-manager/spire-controller-manager-cluster-role-binding.yaml"
	SpireControllerManagerLeaderElectionRoleAssetName        = "spire-controller-manager/spire-controller-manager-leader-election-role.yaml"
	SpireControllerManagerLeaderElectionRoleBindingAssetName = "spire-controller-manager/spire-controller-manager-leader-election-role-binding.yaml"
	SpireServerClusterRoleAssetName                          = "spire-server/spire-server-cluster-role.yaml"
	SpireServerClusterRoleBindingAssetName                   = "spire-server/spire-server-cluster-role-binding.yaml"

	// Service Accounts
	SpiffeCsiDriverServiceAccountAssetName            = "spiffe-csi/spiffe-csi-service-account.yaml"
	SpireAgentServiceAccountAssetName                 = "spire-agent/spire-agent-service-account.yaml"
	SpireOIDCDiscoveryProviderServiceAccountAssetName = "spire-oidc-discovery-provider/spire-oidc-discovery-provider-service-account.yaml"
	SpireServerServiceAccountAssetName                = "spire-server/spire-server-service-account.yaml"

	// Service
	SpireOIDCDiscoveryProviderServiceAssetName   = "spire-oidc-discovery-provider/spire-oidc-discovery-provider-service.yaml"
	SpireServerServiceAssetName                  = "spire-server/spire-server-service.yaml"
	SpireControllerMangerWebhookServiceAssetName = "spire-controller-manager/spire-controller-manager-webhook-service.yaml"

	// Validating Webhook Configurations
	SpireControllerManagerValidatingWebhookConfigurationAssetName = "spire-controller-manager/spire-controller-manager-webhook-validating-webhook.yaml"
)
