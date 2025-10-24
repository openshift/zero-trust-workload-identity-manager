package spire_server

// Server configuration constants
const (
	// Server bind configuration
	DefaultServerBindAddress = "0.0.0.0"
	DefaultServerBindPort    = "8081"
	DefaultServerDataDir     = "/run/spire/data"

	// CA configuration
	DefaultCAKeyType = "ec-p256"

	// Health check configuration
	DefaultHealthCheckBindAddress = "0.0.0.0"
	DefaultHealthCheckBindPort    = "8080"
	DefaultHealthCheckLivePath    = "/live"
	DefaultHealthCheckReadyPath   = "/ready"

	// Telemetry configuration
	DefaultPrometheusHost = "0.0.0.0"
	DefaultPrometheusPort = "9402"

	// Plugin configuration
	DefaultKeyManagerKeysPath = "/run/spire/data/keys.json"

	// Node attestor configuration
	DefaultNodeAttestorAudience         = "spire-server"
	DefaultNodeAttestorServiceAccountNS = "zero-trust-workload-identity-manager"
	DefaultNodeAttestorServiceAccount   = "spire-agent"
)

// StatefulSet constants
const (
	// StatefulSet metadata
	SpireServerStatefulSetName    = "spire-server"
	SpireServerServiceAccountName = "spire-server"
	SpireServerServiceName        = "spire-server"
	SpireServerDefaultReplicas    = 1

	// Annotation keys
	SpireServerAnnotationDefaultContainer = "kubectl.kubernetes.io/default-container"

	// Container names
	SpireServerContainerNameServer            = "spire-server"
	SpireServerContainerNameControllerManager = "spire-controller-manager"

	// Container arguments
	SpireServerArgExpandEnv = "-expandEnv"
	SpireServerArgConfig    = "-config"

	// Configuration paths
	SpireServerConfigPathServer            = "/run/spire/config/server.conf"
	SpireServerConfigPathControllerManager = "controller-manager-config.yaml"

	// Environment variables
	SpireServerEnvPath                = "PATH"
	SpireServerEnvPathValue           = "/opt/spire/bin:/bin"
	SpireServerEnvEnableWebhooks      = "ENABLE_WEBHOOKS"
	SpireServerEnvEnableWebhooksValue = "true"

	// Port configuration
	SpireServerPortNameGRPC          = "grpc"
	SpireServerPortNameHealthz       = "healthz"
	SpireServerPortNameHTTPS         = "https"
	SpireServerPortGRPC        int32 = 8081
	SpireServerPortHealthz     int32 = 8080
	SpireServerPortHTTPSCM     int32 = 9443
	SpireServerPortHealthzCM   int32 = 8083

	// Probe paths
	SpireServerProbePathLive    = "/live"
	SpireServerProbePathReady   = "/ready"
	SpireServerProbePathHealthz = "/healthz"
	SpireServerProbePathReadyz  = "/readyz"

	// Probe timing - Server
	SpireServerLivenessInitialDelay  int32 = 15
	SpireServerLivenessPeriod        int32 = 60
	SpireServerLivenessTimeout       int32 = 3
	SpireServerLivenessFailureThresh int32 = 2
	SpireServerReadinessInitialDelay int32 = 5
	SpireServerReadinessPeriod       int32 = 5

	// Volume names
	SpireServerVolumeNameServerSocket         = "spire-server-socket"
	SpireServerVolumeNameConfig               = "spire-config"
	SpireServerVolumeNameData                 = "spire-data"
	SpireServerVolumeNameServerTmp            = "server-tmp"
	SpireServerVolumeNameControllerManagerTmp = "spire-controller-manager-tmp"
	SpireServerVolumeNameControllerConfig     = "controller-manager-config"

	// Mount paths
	SpireServerMountPathServerSocket            = "/tmp/spire-server/private"
	SpireServerMountPathConfig                  = "/run/spire/config"
	SpireServerMountPathData                    = "/run/spire/data"
	SpireServerMountPathTmp                     = "/tmp"
	SpireServerMountPathControllerManagerConfig = "/controller-manager-config.yaml"
	SpireServerMountPathControllerManagerTmp    = "/tmp"
	SpireServerSubPathControllerManagerTmp      = "spire-controller-manager"

	// ConfigMap names
	SpireServerConfigMapNameServer            = "spire-server"
	SpireServerConfigMapNameControllerManager = "spire-controller-manager"

	// PVC configuration
	SpireServerPVCNameData       = "spire-data"
	SpireServerDefaultVolumeSize = "1Gi"
	SpireServerDefaultAccessMode = "ReadWriteOnce"
)

// getNodeAttestorServiceAccountAllowList returns the default service account allow list
func getNodeAttestorServiceAccountAllowList() []string {
	return []string{
		DefaultNodeAttestorServiceAccountNS + ":" + DefaultNodeAttestorServiceAccount,
	}
}
