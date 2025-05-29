package utils

const (

	// Controller Names
	ZeroTrustWorkloadIdentityManagerStaticResourceControllerName             = "zero-trust-workload-identity-manager-static-resource-controller"
	ZeroTrustWorkloadIdentityManagerSpireServerControllerName                = "zero-trust-workload-identity-manager-spire-server-controller"
	ZeroTrustWorkloadIdentityManagerSpireAgentControllerName                 = "zero-trust-workload-identity-manager-spire-agent-controller"
	ZeroTrustWorkloadIdentityManagerSpiffeCsiDriverControllerName            = "zero-trust-workload-identity-manager-spiffe-csi-driver-controller"
	ZeroTrustWorkloadIdentityManagerSpireOIDCDiscoveryProviderControllerName = "zero-trust-workload-identity-manager-spire-oidc-discovery-provider-controller"

	OperatorNamespace = "zero-trust-workload-identity-manager"

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

	// Image Reference
	SpireServerImageEnv                = "SPIRE_SERVER_IMAGE"
	SpireAgentImageEnv                 = "SPIRE_AGENT_IMAGE"
	SpiffeCSIDriverImageEnv            = "SPIFFE_CSI_DRIVER_IMAGE"
	SpireOIDCDiscoveryProviderImageEnv = "SPIRE_OIDC_DISCOVERY_PROVIDER_IMAGE"
	SpireControllerManagerImageEnv     = "SPIRE_CONTROLLER_MANAGER_IMAGE"
	SpiffeHelperImageEnv               = "SPIFFE_HELPER_IMAGE"
	NodeDriverRegistrarImageEnv        = "NODE_DRIVER_REGISTRAR_IMAGE"
)
