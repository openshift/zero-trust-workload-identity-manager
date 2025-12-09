package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster
// +kubebuilder:validation:XValidation:rule="oldSelf == null || !has(oldSelf.spec.federation) || has(self.spec.federation)",message="Federation configuration cannot be removed once set."
// +kubebuilder:validation:XValidation:rule="self.metadata.name == 'cluster'",message="SpireServer is a singleton, .metadata.name must be 'cluster'"
// +kubebuilder:validation:XValidation:rule="oldSelf.spec.persistence.size == self.spec.persistence.size",message="spec.persistence.size is immutable"
// +kubebuilder:validation:XValidation:rule="oldSelf.spec.persistence.accessMode == self.spec.persistence.accessMode",message="spec.persistence.accessMode is immutable"
// +kubebuilder:validation:XValidation:rule="oldSelf.spec.persistence.storageClass == self.spec.persistence.storageClass",message="spec.persistence.storageClass is immutable"
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

// SpireServerSpec defines the specifications for configuring the SPIRE server.
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

	// jwtIssuer is the JWT issuer url.
	// Must be a valid HTTPS or HTTP URL.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MaxLength=512
	// +kubebuilder:validation:Pattern=`^(?i)https?://[^\s?#]+$`
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

	// keyManager configures the SPIRE server key manager.
	// +kubebuilder:validation:Optional
	KeyManager *KeyManager `json:"keyManager,omitempty"`

	// caSubject contains subject information for the SPIRE CA.
	// +kubebuilder:validation:Required
	CASubject CASubject `json:"caSubject,omitempty"`

	// persistence configures storage for the SPIRE server.
	// This field is required and immutable once set.
	// +kubebuilder:validation:Required
	Persistence Persistence `json:"persistence"`

	// datastore configures the SPIRE server SQL datastore backend.
	// +kubebuilder:validation:Required
	Datastore DataStore `json:"datastore,omitempty"`

	// federation configures SPIRE federation endpoints and relationships
	// +kubebuilder:validation:Optional
	Federation *FederationConfig `json:"federation,omitempty"`

	CommonConfig `json:",inline"`
}

// FederationConfig defines federation bundle endpoint and federated trust domains
type FederationConfig struct {
	// bundleEndpoint configures this cluster's federation bundle endpoint
	// +kubebuilder:validation:Required
	BundleEndpoint BundleEndpointConfig `json:"bundleEndpoint"`

	// federatesWith lists trust domains this cluster federates with
	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:MinItems=1
	// +kubebuilder:validation:MaxItems=50
	FederatesWith []FederatesWithConfig `json:"federatesWith,omitempty"`

	// managedRoute enables or disables automatic Route creation for the federation endpoint
	// "true": Allows automatic exposure of federation endpoint through a managed OpenShift Route.
	// "false": Allows administrators to manually configure exposure using custom OpenShift Routes or ingress, offering more control over routing behavior.
	// +kubebuilder:default:="true"
	// +kubebuilder:validation:Enum:="true";"false"
	// +kubebuilder:validation:Optional
	ManagedRoute string `json:"managedRoute,omitempty"`
}

// BundleEndpointConfig configures how this cluster exposes its federation bundle
// The federation endpoint is exposed on 0.0.0.0:8443
// +kubebuilder:validation:XValidation:rule="self.profile == 'https_web' ? has(self.httpsWeb) : true",message="httpsWeb is required when profile is https_web"
// +kubebuilder:validation:XValidation:rule="!has(oldSelf.profile) || oldSelf.profile == self.profile",message="profile is immutable and cannot be changed once set"
type BundleEndpointConfig struct {
	// profile is the bundle endpoint authentication profile
	// +kubebuilder:validation:Enum=https_spiffe;https_web
	// +kubebuilder:default=https_spiffe
	Profile BundleEndpointProfile `json:"profile"`

	// refreshHint is the hint for bundle refresh interval in seconds
	// +kubebuilder:validation:Minimum=60
	// +kubebuilder:validation:Maximum=3600
	// +kubebuilder:default=300
	RefreshHint int32 `json:"refreshHint,omitempty"`

	// httpsWeb configures the https_web profile (required if profile is https_web)
	// +kubebuilder:validation:Optional
	HttpsWeb *HttpsWebConfig `json:"httpsWeb,omitempty"`
}

// BundleEndpointProfile represents the authentication profile for bundle endpoint
// +kubebuilder:validation:Enum=https_spiffe;https_web
type BundleEndpointProfile string

const (
	// HttpsSpiffeProfile uses SPIFFE authentication (default)
	HttpsSpiffeProfile BundleEndpointProfile = "https_spiffe"

	// HttpsWebProfile uses Web PKI (X.509 certificates from public CA)
	HttpsWebProfile BundleEndpointProfile = "https_web"
)

// HttpsWebConfig configures https_web profile authentication
// +kubebuilder:validation:XValidation:rule="(has(self.acme) && !has(self.servingCert)) || (!has(self.acme) && has(self.servingCert))",message="exactly one of acme or servingCert must be set"
// +kubebuilder:validation:XValidation:rule="!has(oldSelf.acme) || has(self.acme)",message="cannot switch from acme to servingCert configuration"
// +kubebuilder:validation:XValidation:rule="!has(oldSelf.servingCert) || has(self.servingCert)",message="cannot switch from servingCert to acme configuration"
type HttpsWebConfig struct {
	// acme configures automatic certificate management using ACME protocol
	// Mutually exclusive with servingCert
	// +kubebuilder:validation:Optional
	Acme *AcmeConfig `json:"acme,omitempty"`

	// servingCert configures certificate from a Kubernetes Secret
	// Mutually exclusive with acme
	// +kubebuilder:validation:Optional
	ServingCert *ServingCertConfig `json:"servingCert,omitempty"`
}

// AcmeConfig configures ACME certificate provisioning
type AcmeConfig struct {
	// directoryUrl is the ACME directory URL (e.g., Let's Encrypt)
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Pattern=`^https://.*`
	DirectoryUrl string `json:"directoryUrl"`

	// domainName is the domain name for the certificate
	// +kubebuilder:validation:Required
	DomainName string `json:"domainName"`

	// email for ACME account registration
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Pattern=`^[a-zA-Z0-9][a-zA-Z0-9._%+-]*[a-zA-Z0-9]@[a-zA-Z0-9]([a-zA-Z0-9-]*[a-zA-Z0-9])?(\.[a-zA-Z0-9]([a-zA-Z0-9-]*[a-zA-Z0-9])?)*\.[a-zA-Z]{2,}$`
	Email string `json:"email"`

	// tosAccepted indicates acceptance of Terms of Service
	// +kubebuilder:default:="false"
	// +kubebuilder:validation:Enum:="true";"false"
	// +kubebuilder:validation:Optional
	TosAccepted string `json:"tosAccepted,omitempty"`
}

// ServingCertConfig configures TLS certificates for the federation endpoint.
// The service CA certificate is always used for internal communication from the Route to the
// SPIRE server pod. For external communication from clients to the Route, the certificate is
// controlled by ExternalSecretRef.
type ServingCertConfig struct {
	// fileSyncInterval is how often to check for certificate updates (seconds)
	// +kubebuilder:validation:Minimum=3600
	// +kubebuilder:validation:Maximum=7776000
	// +kubebuilder:default=86400
	FileSyncInterval int32 `json:"fileSyncInterval,omitempty"`

	// externalSecretRef is a reference to an externally managed secret that contains
	// the TLS certificate for the SPIRE server federation Route host. The secret must
	// be in the same namespace where the operator and operands are deployed and must
	// contain tls.crt and tls.key fields. The OpenShift Ingress Operator will read
	// this secret to configure the route's TLS certificate.
	// +kubebuilder:validation:Optional
	ExternalSecretRef string `json:"externalSecretRef,omitempty"`
}

// FederatesWithConfig represents a remote trust domain to federate with
// +kubebuilder:validation:XValidation:rule="self.bundleEndpointProfile == 'https_spiffe' ? has(self.endpointSpiffeId) && self.endpointSpiffeId != '' : true",message="endpointSpiffeId is required when bundleEndpointProfile is https_spiffe"
type FederatesWithConfig struct {
	// trustDomain is the federated trust domain name
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Pattern=`^[a-z0-9._-]{1,255}$`
	TrustDomain string `json:"trustDomain"`

	// bundleEndpointUrl is the URL of the remote federation endpoint
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Pattern=`^https://.*`
	BundleEndpointUrl string `json:"bundleEndpointUrl"`

	// bundleEndpointProfile is the authentication profile of the remote endpoint
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Enum=https_spiffe;https_web
	BundleEndpointProfile BundleEndpointProfile `json:"bundleEndpointProfile"`

	// endpointSpiffeId is required for https_spiffe profile
	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:Pattern=`^spiffe://.*`
	EndpointSpiffeId string `json:"endpointSpiffeId,omitempty"`
}

// Persistence defines volume-related settings.
type Persistence struct {
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
}

// DataStore configures the Spire SQL datastore backend.
type DataStore struct {
	// databaseType specifies type of database to use.
	// +kubebuilder:validation:Enum=sql;sqlite3;postgres;mysql;aws_postgresql;aws_mysql
	// +kubebuilder:default:=sqlite3
	DatabaseType string `json:"databaseType"`

	// connectionString contains connection credentials required for the SPIRE server datastore.
	// Must not be empty and should contain valid connection parameters for the specified database type.
	// For PostgreSQL with SSL, include sslmode and certificate paths in the connection string.
	// Example: "dbname=spire user=spire host=postgres.example.com sslmode=verify-full sslrootcert=/run/spire/db/certs/ca.crt"
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=2048
	// +kubebuilder:default:=/run/spire/data/datastore.sqlite3
	ConnectionString string `json:"connectionString"`

	// tlsSecretName specifies the name of a Kubernetes Secret containing TLS certificates for database connections.
	// The Secret will be mounted at /run/spire/db/certs in the SPIRE server container.
	// The Secret should contain keys like 'ca.crt', 'tls.crt', 'tls.key' for the respective certificates.
	// For PostgreSQL, reference these certificates in the connectionString, e.g.:
	// "sslmode=verify-full sslrootcert=/run/spire/db/certs/ca.crt sslcert=/run/spire/db/certs/tls.crt sslkey=/run/spire/db/certs/tls.key"
	// +kubebuilder:validation:Optional
	TLSSecretName string `json:"tlsSecretName,omitempty"`

	// DB pool config
	// maxOpenConns specifies the maximum number of open database connections.
	// Must be between 1 and 10000.
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=10000
	// +kubebuilder:default:=100
	// +kubebuilder:validation:Optional
	MaxOpenConns int `json:"maxOpenConns"`

	// maxIdleConns specifies the maximum number of idle database connections.
	// Must be between 0 and 10000.
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=10000
	// +kubebuilder:default:=2
	// +kubebuilder:validation:Optional
	MaxIdleConns int `json:"maxIdleConns"`

	// connMaxLifetime specifies the maximum lifetime of a database connection in seconds.
	// A value of 0 means connections are not closed due to age.
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Optional
	ConnMaxLifetime int `json:"connMaxLifetime"`

	// disableMigration specifies the migration state
	// If true, disables DB auto-migration.
	// +kubebuilder:default:="false"
	// +kubebuilder:validation:Enum:="true";"false"
	// +kubebuilder:validation:Optional
	DisableMigration string `json:"disableMigration"`
}

// KeyManager defines configuration for the SPIRE server key manager
type KeyManager struct {
	// diskEnabled enables the disk-based key manager.
	// +kubebuilder:default:="true"
	// +kubebuilder:validation:Enum:="true";"false"
	// +kubebuilder:validation:Optional
	DiskEnabled string `json:"diskEnabled,omitempty"`

	// memoryEnabled enables the memory-based key manager
	// +kubebuilder:default:="false"
	// +kubebuilder:validation:Enum:="true";"false"
	// +kubebuilder:validation:Optional
	MemoryEnabled string `json:"memoryEnabled,omitempty"`
}

// CASubject defines the subject information for the SPIRE CA.
type CASubject struct {
	// country specifies the country for the CA.
	// ISO 3166-1 alpha-2 country code (2 characters).
	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:MaxLength=2
	Country string `json:"country,omitempty"`

	// organization specifies the organization for the CA.
	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:MaxLength=64
	Organization string `json:"organization,omitempty"`

	// commonName specifies the common name for the CA.
	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:MaxLength=255
	CommonName string `json:"commonName,omitempty"`
}

// SpireServerStatus defines the observed state of the SPIRE server reconciliation performed by the operator.
type SpireServerStatus struct {
	// conditions holds information about the current state of the SPIRE server resources.
	ConditionalStatus `json:",inline,omitempty"`
}

// GetConditionalStatus returns the conditional status of the SpireServer
func (s *SpireServer) GetConditionalStatus() ConditionalStatus {
	return s.Status.ConditionalStatus
}

// +kubebuilder:object:root=true
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// SpireServerList contains a list of SpireServer
type SpireServerList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []SpireServer `json:"items"`
}

func init() {
	SchemeBuilder.Register(&SpireServer{}, &SpireServerList{})
}
