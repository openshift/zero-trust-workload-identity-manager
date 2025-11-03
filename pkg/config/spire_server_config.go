package config

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// SpireServerConfig represents the complete SPIRE Server configuration
// Reference: https://github.com/spiffe/spire/blob/main/doc/spire_server.md
type SpireServerConfig struct {
	Server       ServerConfig     `json:"server"`
	Plugins      ServerPlugins    `json:"plugins"`
	HealthChecks HealthChecks     `json:"health_checks"`
	Telemetry    *TelemetryConfig `json:"telemetry,omitempty"`
}

// ServerConfig contains the main server configuration
type ServerConfig struct {
	// BindAddress is the IP address or DNS name of the SPIRE server
	BindAddress string `json:"bind_address"`

	// BindPort is the port number which this server is listening on
	BindPort string `json:"bind_port"`

	// TrustDomain corresponds to the trust domain that this server belongs to
	TrustDomain string `json:"trust_domain"`

	// DataDir is the directory to store server runtime data
	DataDir string `json:"data_dir"`

	// LogLevel sets the logging level (DEBUG, INFO, WARN, ERROR)
	LogLevel string `json:"log_level"`

	// LogFormat specifies the log format (TEXT or JSON)
	LogFormat string `json:"log_format,omitempty"`

	// CASubject represents the Subject that CA certificates should use
	CASubject []CASubject `json:"ca_subject"`

	// CAKeyType sets the key type used for the X.509 CA (rsa-2048, rsa-4096, ec-p256, ec-p384)
	CAKeyType string `json:"ca_key_type"`

	// CATTL is the TTL of the server CA
	CATTL metav1.Duration `json:"ca_ttl"`

	// DefaultX509SVIDTTL is the default TTL for X509-SVIDs
	DefaultX509SVIDTTL metav1.Duration `json:"default_x509_svid_ttl"`

	// DefaultJWTSVIDTTL is the default TTL for JWT-SVIDs
	DefaultJWTSVIDTTL metav1.Duration `json:"default_jwt_svid_ttl"`

	// JWTIssuer is the issuer claim in JWT-SVIDs minted by the server
	JWTIssuer string `json:"jwt_issuer"`

	// JWTKeyType sets the key type used for JWT signing (rsa-2048, rsa-4096, ec-p256, ec-p384)
	JWTKeyType string `json:"jwt_key_type,omitempty"`

	// AuditLogEnabled enables audit logging
	AuditLogEnabled bool `json:"audit_log_enabled"`

	// AdminIDs is a list of SPIFFE IDs that have admin rights
	AdminIDs []string `json:"admin_ids,omitempty"`

	// Experimental contains experimental server features
	Experimental *ServerExperimental `json:"experimental,omitempty"`

	// Federation contains federation configuration
	Federation *FederationConfig `json:"federation,omitempty"`

	// RateLimit contains rate limiting configuration
	RateLimit *RateLimitConfig `json:"rate_limit,omitempty"`

	// CacheReloadInterval is the interval to reload in-memory entry cache
	CacheReloadInterval string `json:"cache_reload_interval,omitempty"`

	// Deprecated: Use bind_address and bind_port
	SocketPath string `json:"socket_path,omitempty"`
}

// CASubject defines the subject information for the CA certificate
type CASubject struct {
	Country      []string `json:"country,omitempty"`
	Organization []string `json:"organization,omitempty"`
	CommonName   string   `json:"common_name,omitempty"`
}

// ServerExperimental contains experimental server configuration
type ServerExperimental struct {
	// NamedPipeName sets the named pipe name for the server API
	NamedPipeName string `json:"named_pipe_name,omitempty"`

	// CacheReloadInterval is the interval to reload in-memory entry cache
	CacheReloadInterval string `json:"cache_reload_interval,omitempty"`

	// EventsBasedCache enables the events-based cache
	EventsBasedCache bool `json:"events_based_cache,omitempty"`

	// PruneEventsOlderThan sets how long to keep old events
	PruneEventsOlderThan string `json:"prune_events_older_than,omitempty"`

	// SQLTransactionTimeout is the timeout for SQL transactions
	SQLTransactionTimeout string `json:"sql_transaction_timeout,omitempty"`

	// AuthOpaPolicyEnginePort sets the OPA policy engine port
	AuthOpaPolicyEnginePort int `json:"auth_opa_policy_engine_port,omitempty"`
}

// FederationConfig contains federation configuration
type FederationConfig struct {
	// BundleEndpoint contains bundle endpoint configuration
	BundleEndpoint *BundleEndpointConfig `json:"bundle_endpoint,omitempty"`

	// FederatesWith contains federated trust domains
	FederatesWith map[string]FederatedTrustDomain `json:"federates_with,omitempty"`
}

// BundleEndpointConfig contains bundle endpoint configuration
type BundleEndpointConfig struct {
	Address string                    `json:"address,omitempty"`
	Port    int                       `json:"port,omitempty"`
	ACME    *BundleEndpointACMEConfig `json:"acme,omitempty"`
}

// BundleEndpointACMEConfig contains ACME configuration for bundle endpoint
type BundleEndpointACMEConfig struct {
	DirectoryURL string `json:"directory_url,omitempty"`
	DomainName   string `json:"domain_name,omitempty"`
	Email        string `json:"email,omitempty"`
	ToSAccepted  bool   `json:"tos_accepted,omitempty"`
}

// FederatedTrustDomain contains configuration for a federated trust domain
type FederatedTrustDomain struct {
	BundleEndpointURL     string                 `json:"bundle_endpoint_url,omitempty"`
	BundleEndpointProfile *BundleEndpointProfile `json:"bundle_endpoint_profile,omitempty"`
}

// BundleEndpointProfile contains bundle endpoint profile configuration
type BundleEndpointProfile struct {
	Type             string `json:"type,omitempty"`
	EndpointSPIFFEID string `json:"endpoint_spiffe_id,omitempty"`
}

// RateLimitConfig contains rate limiting configuration
type RateLimitConfig struct {
	Attestation bool                `json:"attestation,omitempty"`
	Signing     bool                `json:"signing,omitempty"`
	JWT         *RateLimitJWTConfig `json:"jwt,omitempty"`
}

// RateLimitJWTConfig contains JWT rate limit configuration
type RateLimitJWTConfig struct {
	Count    int    `json:"count,omitempty"`
	Interval string `json:"interval,omitempty"`
}

// ServerPlugins contains all SPIRE Server plugin configurations
type ServerPlugins struct {
	DataStore         []PluginConfig `json:"DataStore"`
	KeyManager        []PluginConfig `json:"KeyManager"`
	NodeAttestor      []PluginConfig `json:"NodeAttestor"`
	NodeResolver      []PluginConfig `json:"NodeResolver,omitempty"`
	Notifier          []PluginConfig `json:"Notifier,omitempty"`
	UpstreamAuthority []PluginConfig `json:"UpstreamAuthority,omitempty"`
}

// PluginConfig is a generic plugin configuration structure
// Each plugin is represented as a map with the plugin name as key
type PluginConfig map[string]PluginData

// PluginData contains the plugin-specific data
type PluginData struct {
	PluginData     interface{} `json:"plugin_data,omitempty"`
	PluginCmd      string      `json:"plugin_cmd,omitempty"`
	PluginArgs     []string    `json:"plugin_args,omitempty"`
	PluginChecksum string      `json:"plugin_checksum,omitempty"`
	Enabled        *bool       `json:"enabled,omitempty"`
}

// DataStorePluginData contains SQL datastore configuration
type DataStorePluginData struct {
	DatabaseType     string   `json:"database_type"`
	ConnectionString string   `json:"connection_string"`
	MaxOpenConns     int      `json:"max_open_conns,omitempty"`
	MaxIdleConns     int      `json:"max_idle_conns,omitempty"`
	ConnMaxLifetime  string   `json:"conn_max_lifetime,omitempty"`
	DisableMigration bool     `json:"disable_migration,omitempty"`
	RootCAPath       string   `json:"root_ca_path,omitempty"`
	ClientCertPath   string   `json:"client_cert_path,omitempty"`
	ClientKeyPath    string   `json:"client_key_path,omitempty"`
	Options          []string `json:"options,omitempty"`
}

// KeyManagerPluginData contains key manager configuration
type KeyManagerPluginData struct {
	KeysPath      string `json:"keys_path,omitempty"`
	KeyIdentifier string `json:"key_identifier,omitempty"`
	// For AWS KMS
	Region      string            `json:"region,omitempty"`
	KeyMetadata map[string]string `json:"key_metadata,omitempty"`
	// For GCP Cloud KMS
	KeyRing   string `json:"key_ring,omitempty"`
	ProjectID string `json:"project_id,omitempty"`
}

// NodeAttestorPluginData contains k8s PSAT node attestor configuration
type NodeAttestorPluginData struct {
	Clusters map[string]ClusterConfig `json:"clusters"`
}

// ClusterConfig contains cluster-specific node attestation configuration
type ClusterConfig struct {
	ServiceAccountAllowList []string `json:"service_account_allow_list,omitempty"`
	ServiceAccountWhitelist []string `json:"service_account_whitelist,omitempty"` // Deprecated
	Audience                []string `json:"audience,omitempty"`
	AllowedNodeLabelKeys    []string `json:"allowed_node_label_keys,omitempty"`
	AllowedPodLabelKeys     []string `json:"allowed_pod_label_keys,omitempty"`
	KubeConfigFile          string   `json:"kube_config_file,omitempty"`
	UseTokenReviewAPI       bool     `json:"use_token_review_api,omitempty"`
}

// NotifierPluginData contains k8s bundle notifier configuration
type NotifierPluginData struct {
	Namespace          string `json:"namespace"`
	ConfigMap          string `json:"config_map"`
	ConfigMapKey       string `json:"config_map_key,omitempty"`
	KubeConfigFilePath string `json:"kube_config_file_path,omitempty"`
}

// NodeResolverPluginData contains node resolver configuration
type NodeResolverPluginData struct {
	KubeConfigFile string `json:"kube_config_file,omitempty"`
	// For AWS IID resolver
	AssumeRole string `json:"assume_role,omitempty"`
	Region     string `json:"region,omitempty"`
	// For Azure MSI resolver
	TenantID string `json:"tenant_id,omitempty"`
}

// UpstreamAuthorityPluginData contains upstream authority configuration
type UpstreamAuthorityPluginData struct {
	// For disk-based upstream
	CertFilePath   string `json:"cert_file_path,omitempty"`
	KeyFilePath    string `json:"key_file_path,omitempty"`
	BundleFilePath string `json:"bundle_file_path,omitempty"`
	// For SPIRE-based upstream
	ServerAddress     string `json:"server_address,omitempty"`
	ServerPort        string `json:"server_port,omitempty"`
	WorkloadAPISocket string `json:"workload_api_socket,omitempty"`
}

// HealthChecks defines health check configuration
type HealthChecks struct {
	BindAddress     string `json:"bind_address"`
	BindPort        string `json:"bind_port"`
	ListenerEnabled bool   `json:"listener_enabled"`
	LivePath        string `json:"live_path"`
	ReadyPath       string `json:"ready_path"`
}

// TelemetryConfig defines telemetry configuration
type TelemetryConfig struct {
	Prometheus *PrometheusConfig `json:"Prometheus,omitempty"`
	Statsd     *StatsdConfig     `json:"Statsd,omitempty"`
	DogStatsd  *DogStatsdConfig  `json:"DogStatsd,omitempty"`
	M3         *M3Config         `json:"M3,omitempty"`
	InMem      *InMemConfig      `json:"InMem,omitempty"`
}

// PrometheusConfig defines Prometheus-specific telemetry configuration
type PrometheusConfig struct {
	Host string `json:"host,omitempty"`
	Port string `json:"port,omitempty"`
}

// StatsdConfig defines Statsd-specific telemetry configuration
type StatsdConfig struct {
	Address string `json:"address,omitempty"`
}

// DogStatsdConfig defines DogStatsd-specific telemetry configuration
type DogStatsdConfig struct {
	Address string `json:"address,omitempty"`
}

// M3Config defines M3-specific telemetry configuration
type M3Config struct {
	Address string `json:"address,omitempty"`
	Env     string `json:"env,omitempty"`
}

// InMemConfig defines in-memory telemetry configuration
type InMemConfig struct {
	Enabled bool `json:"enabled,omitempty"`
}
