package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster
// +kubebuilder:validation:XValidation:rule="self.metadata.name == 'cluster'",message="SpireOIDCDiscoveryProviderConfig is a singleton, .metadata.name must be 'cluster'"
// +operator-sdk:csv:customresourcedefinitions:displayName="SpireOIDCDiscoveryProviderConfig"

// SpireOIDCDiscoveryProviderConfig defines the configuration for the SPIRE OIDC Discovery Provider managed by zero trust workload identity manager.
// This component allows workloads to authenticate using SPIFFE SVIDs via standard OIDC protocols.
type SpireOIDCDiscoveryProviderConfig struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              SpireOIDCDiscoveryProviderConfigSpec   `json:"spec,omitempty"`
	Status            SpireOIDCDiscoveryProviderConfigStatus `json:"status,omitempty"`
}

// SpireOIDCDiscoveryProviderConfigSpec will have specifications for configuration related to the spire oidc
// discovery provider
type SpireOIDCDiscoveryProviderConfigSpec struct {

	// trustDomain to be used for the SPIFFE identifiers
	// +kubebuilder:validation:Required
	TrustDomain string `json:"trustDomain,omitempty"`

	// agentSocketName is the name of the agent socket.
	// +kubebuilder:default:="spire-agent.sock"
	AgentSocketName string `json:"agentSocketName,omitempty"`

	// jwtIssuer is jwtIssuerPath to JWT issuer. Defaults to oidc-discovery.$trustDomain if unset
	// +kubebuilder:validation:Optional
	JwtIssuer string `json:"jwtIssuer,omitempty"`

	// replicaCount is the number of replicas for the OIDC provider.
	// +kubebuilder:validation:Optional
	// +kubebuilder:default:=1
	ReplicaCount int `json:"replicaCount,omitempty"`

	CommonConfig `json:",inline"`
}

// SpireOIDCDiscoveryProviderConfigStatus defines the observed state of spire-oidc discovery provider
// related reconciliation made by operator
type SpireOIDCDiscoveryProviderConfigStatus struct {
	// conditions holds information of the current state of the spire-oidc resources.
	ConditionalStatus `json:",inline,omitempty"`
}

// +kubebuilder:object:root=true
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// SpireOIDCDiscoveryProviderConfigList contain the list of SpireOIDCDiscoveryProviderConfig
type SpireOIDCDiscoveryProviderConfigList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []SpireOIDCDiscoveryProviderConfig `json:"items"`
}

func init() {
	SchemeBuilder.Register(&SpireOIDCDiscoveryProviderConfig{}, &SpireOIDCDiscoveryProviderConfigList{})
}
