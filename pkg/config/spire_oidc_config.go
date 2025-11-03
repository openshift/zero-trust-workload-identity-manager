package config

// SpireOIDCDiscoveryProviderConfig represents the complete OIDC Discovery Provider configuration
// Reference: https://github.com/spiffe/spire/tree/main/support/oidc-discovery-provider
type SpireOIDCDiscoveryProviderConfig struct {
	// Domains is a list of domains that the OIDC provider will respond to
	Domains []string `json:"domains"`

	// LogLevel sets the logging level (DEBUG, INFO, WARN, ERROR)
	LogLevel string `json:"log_level"`

	// LogFormat specifies the log format (TEXT or JSON)
	LogFormat string `json:"log_format,omitempty"`

	// LogPath sets the path to write logs to (empty for stderr)
	LogPath string `json:"log_path,omitempty"`

	// WorkloadAPI contains the workload API configuration
	WorkloadAPI WorkloadAPIConfig `json:"workload_api"`

	// ServingCertFile contains the TLS certificate configuration
	ServingCertFile *ServingCertFileConfig `json:"serving_cert_file,omitempty"`

	// ACME contains ACME configuration for automatic certificate management
	ACME *ACMEConfig `json:"acme,omitempty"`

	// HealthChecks contains health check configuration
	HealthChecks OIDCHealthChecksConfig `json:"health_checks"`

	// ServerAPI contains server API configuration (for pulling trust bundle)
	ServerAPI *ServerAPIConfig `json:"server_api,omitempty"`

	// AllowInsecureScheme allows insecure HTTP scheme (for development only)
	AllowInsecureScheme bool `json:"allow_insecure_scheme,omitempty"`

	// InsecureAddr is the address to bind to for insecure HTTP
	InsecureAddr string `json:"insecure_addr,omitempty"`

	// ListenSocketPath is the Unix domain socket path to listen on
	ListenSocketPath string `json:"listen_socket_path,omitempty"`
}

// WorkloadAPIConfig contains the workload API configuration for OIDC provider
type WorkloadAPIConfig struct {
	// SocketPath is the path to the Workload API Unix domain socket
	SocketPath string `json:"socket_path"`

	// TrustDomain is the trust domain to limit the workload API to
	TrustDomain string `json:"trust_domain"`

	// PollInterval is the interval to poll the Workload API for updates
	PollInterval string `json:"poll_interval,omitempty"`

	// Experimental contains experimental workload API features
	Experimental *WorkloadAPIExperimental `json:"experimental,omitempty"`
}

// WorkloadAPIExperimental contains experimental workload API configuration
type WorkloadAPIExperimental struct {
	// NamedPipeName sets the named pipe name for the Workload API
	NamedPipeName string `json:"named_pipe_name,omitempty"`
}

// ServingCertFileConfig contains the TLS certificate configuration
type ServingCertFileConfig struct {
	// Addr is the address to bind the HTTPS listener to
	Addr string `json:"addr"`

	// CertFilePath is the path to the TLS certificate file
	CertFilePath string `json:"cert_file_path"`

	// KeyFilePath is the path to the TLS private key file
	KeyFilePath string `json:"key_file_path"`

	// ClientCA is the path to the client CA file for mTLS
	ClientCA string `json:"client_ca,omitempty"`
}

// ACMEConfig contains ACME configuration for automatic certificate management
type ACMEConfig struct {
	// DirectoryURL is the ACME directory URL
	DirectoryURL string `json:"directory_url"`

	// DomainName is the domain name to request certificates for
	DomainName string `json:"domain_name"`

	// CacheDir is the directory to cache certificates and keys
	CacheDir string `json:"cache_dir,omitempty"`

	// Email is the email address for account registration
	Email string `json:"email,omitempty"`

	// ToSAccepted indicates acceptance of the Terms of Service
	ToSAccepted bool `json:"tos_accepted"`

	// ListenAddr is the address to listen on for ACME challenges
	ListenAddr string `json:"listen_addr,omitempty"`

	// RawPublicKey indicates whether to use raw public keys
	RawPublicKey bool `json:"raw_public_key,omitempty"`
}

// OIDCHealthChecksConfig defines health check configuration for OIDC provider
type OIDCHealthChecksConfig struct {
	// BindPort is the port to bind the health check listener to
	BindPort string `json:"bind_port"`

	// BindAddr is the address to bind the health check listener to
	BindAddr string `json:"bind_addr,omitempty"`

	// LivePath is the path for the liveness probe
	LivePath string `json:"live_path"`

	// ReadyPath is the path for the readiness probe
	ReadyPath string `json:"ready_path"`
}

// ServerAPIConfig contains server API configuration
type ServerAPIConfig struct {
	// Address is the SPIRE server API address
	Address string `json:"address,omitempty"`

	// PollInterval is the interval to poll the server API for updates
	PollInterval string `json:"poll_interval,omitempty"`

	// Experimental contains experimental server API features
	Experimental *ServerAPIExperimental `json:"experimental,omitempty"`
}

// ServerAPIExperimental contains experimental server API configuration
type ServerAPIExperimental struct {
	// NamedPipeName sets the named pipe name for the Server API
	NamedPipeName string `json:"named_pipe_name,omitempty"`
}
