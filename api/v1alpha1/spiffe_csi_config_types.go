package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster
// +kubebuilder:validation:XValidation:rule="self.metadata.name == 'cluster'",message="SpiffeCSIDriverConfig is a singleton, .metadata.name must be 'cluster'"
// +operator-sdk:csv:customresourcedefinitions:displayName="SpiffeCSIDriverConfig"

// SpiffeCSIDriverConfig defines the configuration for the SPIFFE CSI Driver managed by zero trust workload identity manager.
// This includes settings related to the registration, socket paths, plugin name and optional runtime flags that influence how the driver operates.
type SpiffeCSIDriverConfig struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              SpiffeCSIDriverConfigSpec   `json:"spec,omitempty"`
	Status            SpiffeCSIDriverConfigStatus `json:"status,omitempty"`
}

// SpiffeCSIDriverConfigSpec will have specifications for configuration related to the spiffe-csi driver.
type SpiffeCSIDriverConfigSpec struct {

	// agentSocketPath is the path to spiffe csi driver the agent socket.
	// +kubebuilder:default:="/run/spire/agent-sockets/spire-agent.sock"
	AgentSocket string `json:"agentSocketPath,omitempty"`

	// pluginName defines the name of the CSI plugin, Sets the csi driver name deployed to the cluster.
	// +kubebuilder:default:="csi.spiffe.io"
	PluginName string `json:"pluginName,omitempty"`

	CommonConfig `json:",inline"`
}

// SpiffeCSIDriverConfigStatus defines the observed state of spiffe csi driver related reconciliation  made by operator
type SpiffeCSIDriverConfigStatus struct {
	// conditions holds information of the states of spiffe csi driver related changes.
	ConditionalStatus `json:",inline,omitempty"`
}

// +kubebuilder:object:root=true
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// SpiffeCSIDriverConfigList contain the list of SpiffeCSIDriverConfig
type SpiffeCSIDriverConfigList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []SpiffeCSIDriverConfig `json:"items"`
}

func init() {
	SchemeBuilder.Register(&SpiffeCSIDriverConfig{}, &SpiffeCSIDriverConfigList{})
}
