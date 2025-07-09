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

	// UpstreamAuthority configures the upstream certificate authority plugin used by the SPIRE Server.
	// This may be one of the supported plugins: "spire", "vault", or "cert-manager".
	// If not specified, the SPIRE Server will not use an upstream authority.
	// +kubebuilder:validation:Optional
	UpstreamAuthority *UpstreamAuthority `json:"upstreamAuthority,omitempty"`

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

// UpstreamAuthority defines the configuration for the upstream certificate authority
// that SPIRE Server will use to obtain its signing certificate.
// It supports different plugins such as SPIRE, Vault, and cert-manager for upstream CA integration.
type UpstreamAuthority struct {
	// Type specifies the type of upstream authority plugin to use.
	// Allowed values: "spire", "vault", "cert-manager".
	// It determines which one of the optional configurations below should be populated.
	Type string `json:"type,omitempty"`

	// spire plugin uses credentials fetched from the Workload API to call an upstream
	// SPIRE server in the same trust domain, requesting an intermediate signing certificate to use as the server's
	// X.509 signing authority.
	// The SVIDs minted in a nested configuration are valid in the entire trust domain, not only in the scope of the
	// server that originated the SVID.
	// In the case of X509-SVID, this is easily achieved because of the chaining semantics that X.509 has.
	// On the other hand, for JWT-SVID, this capability is accomplished by propagating every JWT-SVID public
	// signing key to the whole topology.
	// +kubebuilder:validation:Optional
	Spire *UpstreamAuthoritySpire `json:"spire,omitempty"`

	// vault plugin signs intermediate CA certificates for SPIRE using the Vault PKI Engine.
	// The plugin does not support the PublishJWTKey RPC and is therefore not appropriate for use in nested
	// SPIRE topologies where JWT-SVIDs are in use.
	// +kubebuilder:validation:Optional
	Vault *UpstreamAuthorityVault `json:"vault,omitempty"`

	// certManager plugin uses an instance of cert-manager running in Kubernetes to request
	// intermediate signing certificates for SPIRE Server.
	// This plugin will request a signing certificate from cert-manager via a CertificateRequest resource.
	// Once the referenced issuer has signed the request, the intermediate and CA bundle is retrieved by SPIRE.
	// +kubebuilder:validation:Optional
	CertManager *UpstreamAuthorityCertManager `json:"certManager,omitempty"`
}

// UpstreamAuthorityCertManager contains the configuration required to use
// cert-manager as an upstream authority for the SPIRE Server.
// It allows the SPIRE Server to request intermediate signing certificates via
// cert-manager's CertificateRequest resources.
type UpstreamAuthorityCertManager struct {
	// issuerName is the name of the issuer to reference in CertificateRequests.
	// +kubebuilder:validation:Required
	IssuerName string `json:"issuerName,omitempty"`

	// issuerKind is the kind of the issuer to reference in CertificateRequests. Defaults to "Issuer" if empty.
	// +kubebuilder:validation:Optional
	// +kubebuilder:default:="Issuer"
	IssuerKind string `json:"issuerKind,omitempty"`

	// issuerGroup is the group of the issuer to reference in CertificateRequests. Defaults to "cert-manager.io" if empty.
	// +kubebuilder:validation:Optional
	// +kubebuilder:default:="cert-manager.io"
	IssuerGroup string `json:"issuerGroup,omitempty"`

	// namespace in which to create CertificateRequests for signing.
	// +kubebuilder:validation:Required
	Namespace string `json:"namespace,omitempty"`

	// kubeConfigSecretName is the name of the Kubernetes Secret that stores the kubeconfig
	// used to connect to the Kubernetes cluster where cert-manager is running.
	// If empty, in-cluster configuration will be used.
	// +kubebuilder:validation:Optional
	KubeConfigSecretName string `json:"kubeConfigSecretName,omitempty"`
}

// UpstreamAuthorityVault contains the configuration required to use
// HashiCorp Vault as the upstream authority for SPIRE.
// It supports multiple authentication mechanisms including Token, Cert, AppRole, and Kubernetes.
type UpstreamAuthorityVault struct {
	// VaultAddress is the URL of the Vault server (e.g., https://vault.example.com:8443).
	// +kubebuilder:validation:Required
	VaultAddress string `json:"vaultAddress,omitempty"`

	// Namespace is the Vault Enterprise namespace (optional).
	// +kubebuilder:validation:Optional
	Namespace string `json:"namespace,omitempty"`

	// PkiMountPoint is the mount point where the PKI secrets engine is enabled (e.g., "pki").
	// +kubebuilder:validation:Required
	PkiMountPoint string `json:"pkiMountPoint,omitempty"`

	// CaCertSecret is the name of the Kubernetes secret that contains the CA certificate (PEM format).
	// +kubebuilder:validation:Required
	CaCertSecret string `json:"caCertSecret,omitempty"`

	// TokenAuth configures Vault token-based authentication.
	// +kubebuilder:validation:Optional
	TokenAuth *TokenAuth `json:"tokenAuth,omitempty"`

	// CertAuth configures Vault client certificate authentication.
	// +kubebuilder:validation:Optional
	CertAuth *CertAuth `json:"certAuth,omitempty"`

	// AppRoleAuth configures Vault AppRole authentication.
	// +kubebuilder:validation:Optional
	AppRoleAuth *AppRoleAuth `json:"appRoleAuth,omitempty"`

	// K8sAuth configures Vault Kubernetes authentication.
	// +kubebuilder:validation:Optional
	K8sAuth *K8sAuth `json:"k8sAuth,omitempty"`
}

// TokenAuth configures the Vault token authentication method.
// The token is used as a bearer token in the "X-Vault-Token" header.
type TokenAuth struct {
	// Token is the Vault token string.
	// +kubebuilder:validation:Required
	Token string `json:"token,omitempty"`
}

// CertAuth configures the Vault client certificate authentication method.
type CertAuth struct {
	// CertAuthMountPoint is the mount point where the TLS certificate auth method is enabled.
	// +kubebuilder:validation:Required
	CertAuthMountPoint string `json:"certAuthMountPoint,omitempty"`

	// ClientCertSecret is the name of the Kubernetes secret containing the client certificate (PEM).
	// +kubebuilder:validation:Required
	ClientCertSecret string `json:"clientCertSecret,omitempty"`

	// ClientKeySecret is the name of the Kubernetes secret containing the client private key (PEM).
	// +kubebuilder:validation:Required
	ClientKeySecret string `json:"clientKeySecret,omitempty"`

	// CertAuthRoleName is the name of the Vault role to authenticate against, Default to trying all roles.
	// +kubebuilder:validation:Optional
	CertAuthRoleName string `json:"certAuthRoleName,omitempty"`
}

// AppRoleAuth configures the Vault AppRole authentication method.
type AppRoleAuth struct {
	// AppRoleMountPoint is the mount point where the AppRole auth method is enabled (e.g., "approle").
	// +kubebuilder:validation:Required
	AppRoleMountPoint string `json:"appRoleMountPoint,omitempty"`

	// AppRoleID is the AppRole ID used for authentication.
	// +kubebuilder:validation:Required
	AppRoleID string `json:"appRoleID,omitempty"`

	// AppRoleSecretID is the AppRole SecretID used for authentication.
	// +kubebuilder:validation:Required
	AppRoleSecretID string `json:"appRoleSecretID,omitempty"`
}

// K8sAuth configures the Vault Kubernetes authentication method.
type K8sAuth struct {
	// K8sAuthMountPoint is the mount point where the Kubernetes auth method is enabled (e.g., "kubernetes").
	// +kubebuilder:validation:Required
	K8sAuthMountPoint string `json:"k8sAuthMountPoint,omitempty"`

	// K8sAuthRoleName is the name of the Vault role the plugin authenticates against.
	// +kubebuilder:validation:Required
	K8sAuthRoleName string `json:"k8sAuthRoleName,omitempty"`

	// TokenPath is the path to the Kubernetes ServiceAccount token file.
	// +kubebuilder:validation:Required
	TokenPath string `json:"tokenPath,omitempty"`
}

// UpstreamAuthoritySpire contains the configuration required to use another
// SPIRE Server within the same trust domain as the upstream authority.
// This plugin fetches an intermediate signing certificate by communicating
// with the upstream SPIRE Server via its Workload API.
type UpstreamAuthoritySpire struct {
	// serverAddress is the IP address or DNS name of the upstream SPIRE Server
	// in the same trust domain.
	// +kubebuilder:validation:Required
	ServerAddress string `json:"serverAddress,omitempty"`

	// serverPort is the port number on which the upstream SPIRE Server is listening.
	// +kubebuilder:validation:Required
	ServerPort string `json:"serverPort,omitempty"`

	// workloadSocketApi is the path to the SPIRE Workload API socket (Unix only).
	// This socket is used to fetch credentials from the local SPIRE Agent.
	// +kubebuilder:validation:Required
	WorkloadSocketAPI string `json:"workloadSocketApi,omitempty"`
}

// SpireServerStatus defines the observed state of spire-server related reconciliation made by operator
type SpireServerStatus struct {
	// conditions holds information of the current state of the spire-server resources.
	ConditionalStatus `json:",inline,omitempty"`
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
