package spire_oidc_discovery_provider

// OIDC Discovery Provider configuration constants
const (

	// Default agent socket name
	DefaultAgentSocketName = "spire-agent.sock"

	// Workload API configuration
	DefaultWorkloadAPISocketBasePath = "/spiffe-workload-api"

	// Serving certificate configuration
	DefaultServingCertAddr        = ":8443"
	DefaultServingCertFilePath    = "/etc/oidc/tls/tls.crt"
	DefaultServingCertKeyFilePath = "/etc/oidc/tls/tls.key"

	// Health check configuration
	DefaultHealthCheckBindPort  = "8008"
	DefaultHealthCheckLivePath  = "/live"
	DefaultHealthCheckReadyPath = "/ready"

	// Domain configuration
	DefaultOIDCServiceName             = "spire-spiffe-oidc-discovery-provider"
	DefaultOIDCServiceNamespacedName   = "spire-spiffe-oidc-discovery-provider.zero-trust-workload-identity-manager"
	DefaultOIDCServiceFullyQualifiedDN = "spire-spiffe-oidc-discovery-provider.zero-trust-workload-identity-manager.svc.cluster.local"
)

// Deployment constants
const (
	// Deployment metadata
	SpireOIDCDeploymentName      = "spire-spiffe-oidc-discovery-provider"
	SpireOIDCServiceAccountName  = "spire-spiffe-oidc-discovery-provider"
	SpireOIDCContainerName       = "spiffe-oidc-discovery-provider"
	SpireOIDCDefaultReplicaCount = 1

	// Volume names
	SpireOIDCVolumeNameWorkloadAPI = "spiffe-workload-api"
	SpireOIDCVolumeNameOIDCSockets = "spire-oidc-sockets"
	SpireOIDCVolumeNameOIDCConfig  = "spire-oidc-config"
	SpireOIDCVolumeNameTLSCerts    = "tls-certs"

	// CSI configuration
	SpireOIDCCSIDriverName = "csi.spiffe.io"

	// ConfigMap and Secret names
	SpireOIDCConfigMapName = "spire-spiffe-oidc-discovery-provider"
	SpireOIDCSecretName    = "oidc-serving-cert"

	// Container configuration
	SpireOIDCConfigFlag    = "-config"
	SpireOIDCConfigPath    = "/run/spire/oidc/config/oidc-discovery-provider.conf"
	SpireOIDCConfigSubPath = "oidc-discovery-provider.conf"

	// Mount paths (directories, not full file paths)
	SpireOIDCMountPathWorkloadAPI = "/spiffe-workload-api"
	SpireOIDCMountPathOIDCSockets = "/run/spire/oidc-sockets"
	SpireOIDCMountPathOIDCConfig  = "/run/spire/oidc/config"
	SpireOIDCMountPathTLSCerts    = "/etc/oidc/tls"

	// Port configuration
	SpireOIDCPortNameHealthz       = "healthz"
	SpireOIDCPortNameHTTPS         = "https"
	SpireOIDCPortHealthz     int32 = 8008
	SpireOIDCPortHTTPS       int32 = 8443

	// Probe paths (reuse health check constants)
	SpireOIDCProbePathReady = DefaultHealthCheckReadyPath
	SpireOIDCProbePathLive  = DefaultHealthCheckLivePath

	// Probe timing
	SpireOIDCProbeInitialDelaySeconds int32 = 5
	SpireOIDCProbePeriodSeconds       int32 = 5
)

// getDefaultDomains returns the default domains list for OIDC provider
func getDefaultDomains(jwtIssuerStripped string) []string {
	return []string{
		DefaultOIDCServiceName,
		DefaultOIDCServiceNamespacedName,
		DefaultOIDCServiceFullyQualifiedDN,
		jwtIssuerStripped,
	}
}
