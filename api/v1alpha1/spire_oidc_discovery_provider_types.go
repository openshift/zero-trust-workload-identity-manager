package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster
// +kubebuilder:validation:XValidation:rule="self.metadata.name == 'cluster'",message="SpireOIDCDiscoveryProvider is a singleton, .metadata.name must be 'cluster'"
// +operator-sdk:csv:customresourcedefinitions:displayName="SpireOIDCDiscoveryProvider"

// SpireOIDCDiscoveryProvider defines the configuration for the SPIRE OIDC Discovery Provider managed by zero trust workload identity manager.
// This component allows workloads to authenticate using SPIFFE SVIDs via standard OIDC protocols.
type SpireOIDCDiscoveryProvider struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              SpireOIDCDiscoveryProviderSpec   `json:"spec,omitempty"`
	Status            SpireOIDCDiscoveryProviderStatus `json:"status,omitempty"`
}

// SpireOIDCDiscoveryProviderSpec defines the specifications for configuration related to the SPIRE OIDC
// discovery provider
type SpireOIDCDiscoveryProviderSpec struct {

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

	// csiDriverName is the name of the CSI driver to use for mounting the Workload API socket.
	// This must match SpiffeCSIDriver.spec.pluginName for the OIDC provider to access SPIFFE identities.
	// Must be a valid DNS subdomain format (e.g., csi.spiffe.io).
	// +kubebuilder:validation:MaxLength=127
	// +kubebuilder:validation:Pattern=`^[a-z0-9]([-a-z0-9.]*[a-z0-9])?$`
	// +kubebuilder:default:="csi.spiffe.io"
	CSIDriverName string `json:"csiDriverName,omitempty"`

	// jwtIssuer is the JWT issuer url.
	// Must be a valid HTTPS or HTTP URL.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MaxLength=512
	// +kubebuilder:validation:Pattern=`^(?i)https?://[^\s?#]+$`
	JwtIssuer string `json:"jwtIssuer,omitempty"`

	// replicaCount is the number of replicas for the OIDC provider.
	// Must be between 1 and 5.
	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=5
	// +kubebuilder:default:=1
	ReplicaCount int `json:"replicaCount,omitempty"`

	// managedRoute controls whether the operator automatically creates an OpenShift Route
	// for the OIDC discovery provider endpoints.
	// "true": The operator creates and maintains an OpenShift Route automatically for OIDC discovery endpoints (*.apps.).
	// "false": Administrators manually configure Routes or ingress, offering more control over routing behavior.
	// +kubebuilder:default:="true"
	// +kubebuilder:validation:Enum:="true";"false"
	// +kubebuilder:validation:Optional
	ManagedRoute string `json:"managedRoute,omitempty"`

	// externalSecretRef is a reference to an externally managed secret that
	// contains the TLS certificate for the oidc-discovery-provider Route host.
	// Must be a valid Kubernetes secret reference name.
	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:MaxLength=253
	// +kubebuilder:validation:Pattern=`^[a-z0-9]([-a-z0-9.]*[a-z0-9])?$`
	ExternalSecretRef string `json:"externalSecretRef,omitempty"`

	CommonConfig `json:",inline"`
}

// SpireOIDCDiscoveryProviderStatus defines the observed state of the SPIRE OIDC discovery provider
// reconciliation performed by the operator
type SpireOIDCDiscoveryProviderStatus struct {
	// conditions holds information about the current state of the SPIRE OIDC discovery provider deployment.
	ConditionalStatus `json:",inline,omitempty"`
}

// GetConditionalStatus returns the conditional status of the SpireOIDCDiscoveryProvider
func (s *SpireOIDCDiscoveryProvider) GetConditionalStatus() ConditionalStatus {
	return s.Status.ConditionalStatus
}

// +kubebuilder:object:root=true
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// SpireOIDCDiscoveryProviderList contains a list of SpireOIDCDiscoveryProvider
type SpireOIDCDiscoveryProviderList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []SpireOIDCDiscoveryProvider `json:"items"`
}

func init() {
	SchemeBuilder.Register(&SpireOIDCDiscoveryProvider{}, &SpireOIDCDiscoveryProviderList{})
}
