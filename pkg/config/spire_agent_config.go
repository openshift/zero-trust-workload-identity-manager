package config

// SpireAgentConfig represents the complete SPIRE Agent configuration
// Reference: https://github.com/spiffe/spire/blob/main/doc/spire_agent.md
type SpireAgentConfig struct {
	Agent        AgentConfig      `json:"agent"`
	Plugins      AgentPlugins     `json:"plugins"`
	HealthChecks HealthChecks     `json:"health_checks"`
	Telemetry    *TelemetryConfig `json:"telemetry,omitempty"`
}

// AgentConfig contains the main agent configuration
type AgentConfig struct {
	// DataDir is the directory where the agent stores runtime data
	DataDir string `json:"data_dir"`

	// LogLevel sets the logging level (DEBUG, INFO, WARN, ERROR)
	LogLevel string `json:"log_level"`

	// LogFormat specifies the log format (TEXT or JSON)
	LogFormat string `json:"log_format,omitempty"`

	// RetryBootstrap enables retry on bootstrap failure
	RetryBootstrap bool `json:"retry_bootstrap"`

	// ServerAddress is the DNS name or IP address of the SPIRE server
	ServerAddress string `json:"server_address"`

	// ServerPort is the port number of the SPIRE server
	ServerPort string `json:"server_port"`

	// SocketPath is the path to bind the Workload API socket
	SocketPath string `json:"socket_path"`

	// TrustBundlePath is the path to the trust bundle file
	TrustBundlePath string `json:"trust_bundle_path"`

	// TrustBundleURL is the URL to fetch the trust bundle from
	TrustBundleURL string `json:"trust_bundle_url,omitempty"`

	// TrustBundleFormat specifies the trust bundle format (pem or spiffe)
	TrustBundleFormat string `json:"trust_bundle_format,omitempty"`

	// TrustDomain corresponds to the trust domain that this agent belongs to
	TrustDomain string `json:"trust_domain"`

	// InsecureBootstrap disables server certificate verification during bootstrap
	InsecureBootstrap bool `json:"insecure_bootstrap,omitempty"`

	// JoinToken is the join token for attestation
	JoinToken string `json:"join_token,omitempty"`

	// AdminSocketPath is the path to bind the admin API socket
	AdminSocketPath string `json:"admin_socket_path,omitempty"`

	// AllowedForeignJWTClaims are JWT claims that can be set by workloads
	AllowedForeignJWTClaims []string `json:"allowed_foreign_jwt_claims,omitempty"`

	// AvailabilityTarget is the minimum time an X509-SVID is valid before rotation
	AvailabilityTarget string `json:"availability_target,omitempty"`

	// Experimental contains experimental agent features
	Experimental *AgentExperimental `json:"experimental,omitempty"`

	// SyncInterval is the interval to sync with the SPIRE server
	SyncInterval string `json:"sync_interval,omitempty"`

	// WorkloadX509SVIDKeyType sets the key type for workload X.509-SVIDs (rsa-2048, rsa-4096, ec-p256, ec-p384)
	WorkloadX509SVIDKeyType string `json:"workload_x509_svid_key_type,omitempty"`

	// WorkloadX509SVIDTTLHint is a hint for the TTL of workload X.509-SVIDs
	WorkloadX509SVIDTTLHint string `json:"workload_x509_svid_ttl_hint,omitempty"`

	// Deprecated: Use trust_bundle_path
	TrustBundle string `json:"trust_bundle,omitempty"`

	// Deprecated: Use server_address and server_port
	ServerSPIFFEIDList []string `json:"server_spiffe_id_list,omitempty"`
}

// AgentExperimental contains experimental agent configuration
type AgentExperimental struct {
	// NamedPipeName sets the named pipe name for the Workload API
	NamedPipeName string `json:"named_pipe_name,omitempty"`

	// AdminNamedPipeName sets the named pipe name for the Admin API
	AdminNamedPipeName string `json:"admin_named_pipe_name,omitempty"`

	// X509SVIDCachePath is the path to cache X509-SVIDs
	X509SVIDCachePath string `json:"x509_svid_cache_path,omitempty"`

	// SyncInterval is the interval to sync with the SPIRE server
	SyncInterval string `json:"sync_interval,omitempty"`
}

// AgentPlugins contains all SPIRE Agent plugin configurations
type AgentPlugins struct {
	KeyManager       []PluginConfig `json:"KeyManager"`
	NodeAttestor     []PluginConfig `json:"NodeAttestor,omitempty"`
	WorkloadAttestor []PluginConfig `json:"WorkloadAttestor,omitempty"`
	SVIDStore        []PluginConfig `json:"SVIDStore,omitempty"`
}

// AgentNodeAttestorPluginData contains node attestor configuration for agent
type AgentNodeAttestorPluginData struct {
	// For k8s_psat
	Cluster   string `json:"cluster,omitempty"`
	TokenPath string `json:"token_path,omitempty"`

	// For k8s_sat (deprecated)
	ServiceAccountPath string `json:"service_account_path,omitempty"`

	// For join_token
	JoinToken string `json:"join_token,omitempty"`

	// For x509pop
	PrivateKeyPath    string `json:"private_key_path,omitempty"`
	CertificatePath   string `json:"certificate_path,omitempty"`
	IntermediatesPath string `json:"intermediates_path,omitempty"`

	// For AWS IID
	AccessKeyID     string `json:"access_key_id,omitempty"`
	SecretAccessKey string `json:"secret_access_key,omitempty"`
	SecurityToken   string `json:"security_token,omitempty"`
	Region          string `json:"region,omitempty"`

	// For Azure MSI
	TenantID   string `json:"tenant_id,omitempty"`
	ResourceID string `json:"resource_id,omitempty"`

	// For GCP IIT
	ProjectID          string `json:"project_id,omitempty"`
	ServiceAccountFile string `json:"service_account_file,omitempty"`
	ZoneID             string `json:"zone_id,omitempty"`

	// For TPM
	TPMPath    string `json:"tpm_path,omitempty"`
	DevicePath string `json:"device_path,omitempty"`
}

// WorkloadAttestorPluginData contains workload attestor configuration
type WorkloadAttestorPluginData struct {
	// For k8s attestor
	NodeNameEnv                 string `json:"node_name_env,omitempty"`
	DisableContainerSelectors   bool   `json:"disable_container_selectors,omitempty"`
	UseNewContainerLocator      bool   `json:"use_new_container_locator,omitempty"`
	VerboseContainerLocatorLogs bool   `json:"verbose_container_locator_logs,omitempty"`
	SkipKubeletVerification     bool   `json:"skip_kubelet_verification,omitempty"`
	KubeletCAPath               string `json:"kubelet_ca_path,omitempty"`
	KubeletReadOnlyPort         int    `json:"kubelet_read_only_port,omitempty"`
	KubeletSecurePort           int    `json:"kubelet_secure_port,omitempty"`
	MaxPollAttempts             int    `json:"max_poll_attempts,omitempty"`
	PollRetryInterval           string `json:"poll_retry_interval,omitempty"`
	ReloadInterval              string `json:"reload_interval,omitempty"`
	UseAnonymousAuthentication  bool   `json:"use_anonymous_authentication,omitempty"`
	VerifyKubeletCertificate    bool   `json:"verify_kubelet_certificate,omitempty"`
	CertificateStore            string `json:"certificate_store,omitempty"`

	// For Docker attestor
	DockerSocketPath          string   `json:"docker_socket_path,omitempty"`
	DockerVersion             string   `json:"docker_version,omitempty"`
	ContainerIDCGroupMatchers []string `json:"container_id_cgroup_matchers,omitempty"`
	// For Unix attestor
	DiscoverWorkloadPath bool  `json:"discover_workload_path,omitempty"`
	WorkloadSizeLimit    int64 `json:"workload_size_limit,omitempty"`

	// For systemd attestor
	SystemdScopePattern string `json:"systemd_scope_pattern,omitempty"`
}

// SVIDStorePluginData contains SVID store configuration
type SVIDStorePluginData struct {
	// For AWS Secrets Manager
	Region          string `json:"region,omitempty"`
	AccessKeyID     string `json:"access_key_id,omitempty"`
	SecretAccessKey string `json:"secret_access_key,omitempty"`
	CertFileARN     string `json:"cert_file_arn,omitempty"`
	KeyFileARN      string `json:"key_file_arn,omitempty"`

	// For GCP Secret Manager
	ProjectID          string `json:"project_id,omitempty"`
	ServiceAccountFile string `json:"service_account_file,omitempty"`

	// For Vault
	VaultAddr     string `json:"vault_addr,omitempty"`
	Namespace     string `json:"namespace,omitempty"`
	PKIMountPoint string `json:"pki_mount_point,omitempty"`
	CACertPath    string `json:"ca_cert_path,omitempty"`
	Token         string `json:"token,omitempty"`
	RenewToken    bool   `json:"renew_token,omitempty"`

	// For file-based SVID store
	CertPath           string `json:"cert_path,omitempty"`
	KeyPath            string `json:"key_path,omitempty"`
	BundlePath         string `json:"bundle_path,omitempty"`
	SVIDFileName       string `json:"svid_file_name,omitempty"`
	SVIDKeyFileName    string `json:"svid_key_file_name,omitempty"`
	SVIDBundleFileName string `json:"svid_bundle_file_name,omitempty"`
}
