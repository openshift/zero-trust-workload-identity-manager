package spire_agent

// Agent configuration constants
const (
	// Agent main configuration
	DefaultAgentDataDir         = "/var/lib/spire"
	DefaultAgentServerAddress   = "spire-server.zero-trust-workload-identity-manager"
	DefaultAgentServerPort      = "443"
	DefaultAgentSocketPath      = "/tmp/spire-agent/public/spire-agent.sock"
	DefaultAgentTrustBundlePath = "/run/spire/bundle/bundle.crt"

	// Health check configuration
	DefaultHealthCheckBindAddress = "0.0.0.0"
	DefaultHealthCheckBindPort    = "9982"
	DefaultHealthCheckLivePath    = "/live"
	DefaultHealthCheckReadyPath   = "/ready"

	// Telemetry configuration
	DefaultPrometheusHost = "0.0.0.0"
	DefaultPrometheusPort = "9402"

	// Workload attestor configuration
	DefaultNodeNameEnv = "MY_NODE_NAME"
)

// DaemonSet constants
const (
	// DaemonSet metadata
	SpireAgentDaemonSetName      = "spire-agent"
	SpireAgentServiceAccountName = "spire-agent"

	// Annotation keys
	SpireAgentAnnotationDefaultContainer = "kubectl.kubernetes.io/default-container"

	// Container configuration
	SpireAgentContainerName = "spire-agent"

	// Container arguments
	SpireAgentArgConfig  = "-config"
	SpireAgentConfigPath = "/opt/spire/conf/agent/agent.conf"

	// Environment variables
	SpireAgentEnvPath      = "PATH"
	SpireAgentEnvPathValue = "/opt/spire/bin:/bin"
	SpireAgentEnvNodeName  = DefaultNodeNameEnv // "MY_NODE_NAME"

	// Port configuration
	SpireAgentPortNameHealthz       = "healthz"
	SpireAgentPortHealthz     int32 = 9982 // Derived from DefaultHealthCheckBindPort

	// Probe paths (reference canonical definitions from default constants)
	SpireAgentProbePathLive  = DefaultHealthCheckLivePath  // "/live"
	SpireAgentProbePathReady = DefaultHealthCheckReadyPath // "/ready"

	// Probe timing
	SpireAgentLivenessInitialDelay  int32 = 15
	SpireAgentLivenessPeriod        int32 = 60
	SpireAgentReadinessInitialDelay int32 = 10
	SpireAgentReadinessPeriod       int32 = 30

	// Volume names
	SpireAgentVolumeNameConfig         = "spire-config"
	SpireAgentVolumeNamePersistence    = "spire-agent-persistence"
	SpireAgentVolumeNameBundle         = "spire-bundle"
	SpireAgentVolumeNameSocketDir      = "spire-agent-socket-dir"
	SpireAgentVolumeNameToken          = "spire-token"
	SpireAgentVolumeNameAdminSocketDir = "spire-agent-admin-socket-dir"
	SpireAgentVolumeNameKubeletPKI     = "kubelet-pki"

	// Mount paths
	SpireAgentMountPathConfig      = "/opt/spire/conf/agent"
	SpireAgentMountPathPersistence = DefaultAgentDataDir // "/var/lib/spire"
	SpireAgentMountPathBundle      = "/run/spire/bundle"
	SpireAgentMountPathSocketDir   = "/tmp/spire-agent/public"
	SpireAgentMountPathToken       = "/var/run/secrets/tokens"

	// ConfigMap names
	SpireAgentConfigMapNameAgent  = "spire-agent"
	SpireAgentConfigMapNameBundle = "spire-bundle"

	// Service account token configuration
	SpireAgentTokenPath                    = "spire-agent"
	SpireAgentTokenExpirationSeconds int64 = 7200
	SpireAgentTokenAudience                = "spire-server"

	// Host path configuration
	SpireAgentHostPathAgentSockets          = "/run/spire/agent-sockets"
	SpireAgentHostPathTypeDirectoryOrCreate = "DirectoryOrCreate"

	// Update strategy
	SpireAgentMaxUnavailable int32 = 1
)
