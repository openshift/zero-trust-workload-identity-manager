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

	// keyManager has configs for the spire server key manager.
	// +kubebuilder:validation:Optional
	KeyManager *KeyManager `json:"keyManager,omitempty"`

	// caSubject contains subject information for the Spire CA.
	// +kubebuilder:validation:Required
	CASubject CASubject `json:"caSubject,omitempty"`

	// persistence has config for spire server volume related configs.
	// This field is required and immutable once set.
	// +kubebuilder:validation:Required
	Persistence Persistence `json:"persistence"`

	// spireSQLConfig has the config required for the spire server SQL DataStore.
	// +kubebuilder:validation:Required
	Datastore DataStore `json:"datastore,omitempty"`

	CommonConfig `json:",inline"`
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

	// connectionString contain connection credentials required for spire server Datastore.
	// Must not be empty and should contain valid connection parameters for the specified database type.
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=1024
	// +kubebuilder:default:=/run/spire/data/datastore.sqlite3
	ConnectionString string `json:"connectionString"`

	// DB pool config
	// maxOpenConns will specify the maximum connections for the DB pool.
	// Must be between 1 and 10000.
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=10000
	// +kubebuilder:default:=100
	MaxOpenConns int `json:"maxOpenConns"`

	// maxIdleConns specifies the maximum idle connection to be configured.
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=10000
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
