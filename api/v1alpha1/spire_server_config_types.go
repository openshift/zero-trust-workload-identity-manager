package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster
// +kubebuilder:validation:XValidation:rule="self.metadata.name == 'cluster'",message="SpireServer is a singleton, .metadata.name must be 'cluster'"
// +operator-sdk:csv:customresourcedefinitions:displayName="SpireServer"

// SpireServer defines the configuration for the SPIRE Server managed by zero trust workload identity manager.
// This includes details related to trust domain, data storage, plugins
// and other configs required for workload authentication.
type SpireServer struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              SpireServerSpec   `json:"spec,omitempty"`
	Status            SpireServerStatus `json:"status,omitempty"`
}

// SpireServerSpec will have specifications for configuration related to the spire server.
type SpireServerSpec struct {
	// logLevel sets the logging level for the operand.
	// Valid values are: debug, info, warn, error.
	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:Enum=debug;info;warn;error
	// +kubebuilder:default:="info"
	LogLevel string `json:"logLevel,omitempty"`

	// logFormat sets the logging format for the operand.
	// Valid values are: text, json.
	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:Enum=text;json
	// +kubebuilder:default:="text"
	LogFormat string `json:"logFormat,omitempty"`

	// trustDomain to be used for the SPIFFE identifiers
	// +kubebuilder:validation:Required
	TrustDomain string `json:"trustDomain,omitempty"`

	// clusterName will have the cluster name required to configure spire server.
	// +kubebuilder:validation:Required
	ClusterName string `json:"clusterName,omitempty"`

	// bundleConfigMap is Configmap name for Spire bundle, it sets the trust domain to be used for the SPIFFE identifiers
	// +kubebuilder:validation:Optional
	// +kubebuilder:default:=spire-bundle
	BundleConfigMap string `json:"bundleConfigMap"`

	// jwtIssuer is the JWT issuer url.
	// +kubebuilder:validation:Required
	JwtIssuer string `json:"jwtIssuer"`

	// caValidity is the validity period (TTL) for the SPIRE Server's own CA certificate.
	// This determines how long the server's root or intermediate certificate is valid.
	// +kubebuilder:validation:Type=string
	// +kubebuilder:validation:Format=duration
	// +kubebuilder:validation:Optional
	// +kubebuilder:default="24h"
	CAValidity metav1.Duration `json:"caValidity"`

	// defaultX509Validity is the default validity period (TTL) for X.509 SVIDs issued to workloads.
	// This value is used if a specific TTL is not configured for a registration entry.
	// +kubebuilder:validation:Type=string
	// +kubebuilder:validation:Format=duration
	// +kubebuilder:validation:Optional
	// +kubebuilder:default="1h"
	DefaultX509Validity metav1.Duration `json:"defaultX509Validity"`

	// defaultJWTValidity is the default validity period (TTL) for JWT SVIDs issued to workloads.
	// This value is used if a specific TTL is not configured for a registration entry.
	// +kubebuilder:validation:Type=string
	// +kubebuilder:validation:Format=duration
	// +kubebuilder:validation:Optional
	// +kubebuilder:default="5m"
	DefaultJWTValidity metav1.Duration `json:"defaultJWTValidity"`

	// caKeyType specifies the key type used for the server CA (both X509 and JWT).
	// Valid values are: rsa-2048, rsa-4096, ec-p256, ec-p384.
	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:Enum=rsa-2048;rsa-4096;ec-p256;ec-p384
	// +kubebuilder:default="rsa-2048"
	CAKeyType string `json:"caKeyType,omitempty"`

	// jwtKeyType specifies the key type used for JWT signing.
	// Valid values are: rsa-2048, rsa-4096, ec-p256, ec-p384.
	// This field is optional and will only be set in the SPIRE server configuration if explicitly provided.
	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:Enum=rsa-2048;rsa-4096;ec-p256;ec-p384
	JWTKeyType string `json:"jwtKeyType,omitempty"`

	// keyManager has configs for the spire server key manager.
	// +kubebuilder:validation:Optional
	KeyManager *KeyManager `json:"keyManager,omitempty"`

	// caSubject contains subject information for the Spire CA.
	// +kubebuilder:validation:Optional
	CASubject *CASubject `json:"caSubject,omitempty"`

	// persistence has config for spire server volume related configs
	// +kubebuilder:validation:Optional
	Persistence *Persistence `json:"persistence,omitempty"`

	// spireSQLConfig has the config required for the spire server SQL DataStore.
	// +kubebuilder:validation:Optional
	Datastore *DataStore `json:"datastore,omitempty"`

	// Federation configures SPIRE federation endpoints and relationships
	// +kubebuilder:validation:Optional
	Federation *FederationConfig `json:"federation,omitempty"`

	CommonConfig `json:",inline"`
}

// Persistence defines volume-related settings.
type Persistence struct {
	// type of volume to use for persistence.
	// +kubebuilder:validation:Enum=pvc;hostPath;emptyDir
	// +kubebuilder:default:=pvc
	Type string `json:"type"`

	// size of the persistent volume (e.g., 1Gi).
	// +kubebuilder:validation:Pattern=^[1-9][0-9]*Gi$
	// +kubebuilder:default:="1Gi"
	Size string `json:"size"`

	// accessMode for the volume.
	// +kubebuilder:validation:Enum=ReadWriteOnce;ReadWriteOncePod;ReadWriteMany
	// +kubebuilder:default:=ReadWriteOnce
	AccessMode string `json:"accessMode"`

	// storageClass to be used for the PVC.
	// +kubebuilder:validation:optional
	// +kubebuilder:default:=""
	StorageClass string `json:"storageClass,omitempty"`

	// hostPath to be used when type is hostPath.
	// +kubebuilder:validation:optional
	// +kubebuilder:default:=""
	HostPath string `json:"hostPath,omitempty"`
}

// DataStore configures the Spire SQL datastore backend.
type DataStore struct {
	// databaseType specifies type of database to use.
	// +kubebuilder:validation:Enum=sql;sqlite3;postgres;mysql;aws_postgresql;aws_mysql
	// +kubebuilder:default:=sqlite3
	DatabaseType string `json:"databaseType"`

	// connectionString contain connection credentials required for spire server Datastore.
	// +kubebuilder:default:=/run/spire/data/datastore.sqlite3
	ConnectionString string `json:"connectionString"`

	// options specifies extra DB options.
	// +kubebuilder:validation:optional
	// +kubebuilder:default:={}
	Options []string `json:"options,omitempty"`

	// MySQL TLS options.
	// +kubebuilder:default:=""
	RootCAPath     string `json:"rootCAPath,omitempty"`
	ClientCertPath string `json:"clientCertPath,omitempty"`
	ClientKeyPath  string `json:"clientKeyPath,omitempty"`

	// DB pool config
	// maxOpenConns will specify the maximum connections for the DB pool.
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:default:=100
	MaxOpenConns int `json:"maxOpenConns"`

	// maxIdleConns specifies the maximum idle connection to be configured.
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:default:=2
	MaxIdleConns int `json:"maxIdleConns"`

	// connMaxLifetime will specify maximum lifetime connections.
	// Max time (in seconds) a connection may live.
	// +kubebuilder:validation:Minimum=0
	ConnMaxLifetime int `json:"connMaxLifetime"`

	// disableMigration specifies the migration state
	// If true, disables DB auto-migration.
	// +kubebuilder:default:="false"
	// +kubebuilder:validation:Enum:="true";"false"
	// +kubebuilder:validation:Optional
	DisableMigration string `json:"disableMigration"`
}

// KeyManager will contain configs for the spire server key manager
type KeyManager struct {
	// diskEnabled is a flag to enable keyManager on disk.
	// +kubebuilder:default:="true"
	// +kubebuilder:validation:Enum:="true";"false"
	// +kubebuilder:validation:Optional
	DiskEnabled string `json:"diskEnabled,omitempty"`

	// memoryEnabled is a flag to enable keyManager on memory
	// +kubebuilder:default:="false"
	// +kubebuilder:validation:Enum:="true";"false"
	// +kubebuilder:validation:Optional
	MemoryEnabled string `json:"memoryEnabled,omitempty"`
}

// CASubject defines the subject information for the Spire CA.
type CASubject struct {
	// country specifies the country for the CA.
	// +kubebuilder:validation:Optional
	Country string `json:"country,omitempty"`

	// organization specifies the organization for the CA.
	// +kubebuilder:validation:Optional
	Organization string `json:"organization,omitempty"`

	// commonName specifies the common name for the CA.
	// +kubebuilder:validation:Optional
	CommonName string `json:"commonName,omitempty"`
}

// FederationConfig defines federation bundle endpoint and federated trust domains
type FederationConfig struct {
	// BundleEndpoint configures this cluster's federation bundle endpoint
	// +kubebuilder:validation:Required
	BundleEndpoint BundleEndpointConfig `json:"bundleEndpoint"`

	// FederatesWith lists trust domains this cluster federates with
	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:MaxItems=50
	FederatesWith []FederatesWithConfig `json:"federatesWith,omitempty"`

	// ManagedRoute enables or disables automatic Route creation for federation endpoint
	// "true": Allows automatic exposure through a managed OpenShift Route
	// "false": Allows administrators to manually configure Routes
	// +kubebuilder:default:="true"
	// +kubebuilder:validation:Enum:="true";"false"
	// +kubebuilder:validation:Optional
	ManagedRoute string `json:"managedRoute,omitempty"`
}

// BundleEndpointConfig configures how this cluster exposes its federation bundle
type BundleEndpointConfig struct {
	// Port for the federation bundle endpoint
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=65535
	// +kubebuilder:default=8443
	Port int32 `json:"port"`

	// Address to bind the bundle endpoint to
	// +kubebuilder:default="0.0.0.0"
	Address string `json:"address"`

	// Profile is the bundle endpoint authentication profile
	// +kubebuilder:validation:Enum=https_spiffe;https_web
	// +kubebuilder:default=https_spiffe
	Profile BundleEndpointProfile `json:"profile"`

	// RefreshHint is the hint for bundle refresh interval in seconds
	// +kubebuilder:validation:Minimum=60
	// +kubebuilder:validation:Maximum=3600
	// +kubebuilder:default=300
	RefreshHint int32 `json:"refreshHint,omitempty"`

	// HttpsWeb configures the https_web profile (required if profile is https_web)
	// +kubebuilder:validation:Optional
	HttpsWeb *HttpsWebConfig `json:"httpsWeb,omitempty"`
}

// BundleEndpointProfile represents the authentication profile for bundle endpoint
// +kubebuilder:validation:Enum=https_spiffe;https_web
type BundleEndpointProfile string

const (
	// HttpsSpiffeProfile uses SPIFFE authentication (default, recommended)
	HttpsSpiffeProfile BundleEndpointProfile = "https_spiffe"

	// HttpsWebProfile uses Web PKI (X.509 certificates from public CA)
	HttpsWebProfile BundleEndpointProfile = "https_web"
)

// HttpsWebConfig configures https_web profile authentication
type HttpsWebConfig struct {
	// Acme configures automatic certificate management using ACME protocol
	// Mutually exclusive with ServingCert
	// +kubebuilder:validation:Optional
	Acme *AcmeConfig `json:"acme,omitempty"`

	// ServingCert configures certificate from a Kubernetes Secret
	// Mutually exclusive with Acme
	// +kubebuilder:validation:Optional
	ServingCert *ServingCertConfig `json:"servingCert,omitempty"`
}

// AcmeConfig configures ACME certificate provisioning
type AcmeConfig struct {
	// DirectoryUrl is the ACME directory URL (e.g., Let's Encrypt)
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Pattern=`^https://.*`
	DirectoryUrl string `json:"directoryUrl"`

	// DomainName is the domain name for the certificate
	// +kubebuilder:validation:Required
	DomainName string `json:"domainName"`

	// Email for ACME account registration
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Pattern=`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`
	Email string `json:"email"`

	// TosAccepted indicates acceptance of Terms of Service
	// +kubebuilder:default:="false"
	// +kubebuilder:validation:Enum:="true";"false"
	// +kubebuilder:validation:Optional
	TosAccepted string `json:"tosAccepted,omitempty"`
}

// ServingCertConfig references a Secret containing TLS certificate
type ServingCertConfig struct {
	// SecretName is the name of the Secret containing tls.crt and tls.key
	// +kubebuilder:validation:Required
	SecretName string `json:"secretName"`

	// FileSyncInterval is how often to check for certificate updates (seconds)
	// +kubebuilder:validation:Minimum=30
	// +kubebuilder:validation:Maximum=3600
	// +kubebuilder:default=300
	FileSyncInterval int32 `json:"fileSyncInterval,omitempty"`
}

// FederatesWithConfig represents a remote trust domain to federate with
type FederatesWithConfig struct {
	// TrustDomain is the federated trust domain name
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Pattern=`^[a-z0-9._-]{1,255}$`
	TrustDomain string `json:"trustDomain"`

	// BundleEndpointUrl is the URL of the remote federation endpoint
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Pattern=`^https://.*`
	BundleEndpointUrl string `json:"bundleEndpointUrl"`

	// BundleEndpointProfile is the authentication profile of remote endpoint
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Enum=https_spiffe;https_web
	BundleEndpointProfile BundleEndpointProfile `json:"bundleEndpointProfile"`

	// EndpointSpiffeId is required for https_spiffe profile
	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:Pattern=`^spiffe://.*`
	EndpointSpiffeId string `json:"endpointSpiffeId,omitempty"`
}

// SpireServerStatus defines the observed state of spire-server related reconciliation made by operator
type SpireServerStatus struct {
	// conditions holds information of the current state of the spire-server resources.
	ConditionalStatus `json:",inline,omitempty"`
}

// GetConditionalStatus returns the conditional status of the SpireServer
func (s *SpireServer) GetConditionalStatus() ConditionalStatus {
	return s.Status.ConditionalStatus
}

// +kubebuilder:object:root=true
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// SpireServerList contain the list of SpireServer
type SpireServerList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []SpireServer `json:"items"`
}

func init() {
	SchemeBuilder.Register(&SpireServer{}, &SpireServerList{})
}
