package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster
// +kubebuilder:validation:XValidation:rule="self.metadata.name == 'cluster'",message="SpiffeCSIDriver is a singleton, .metadata.name must be 'cluster'"
// +operator-sdk:csv:customresourcedefinitions:displayName="SpiffeCSIDriver"

// SpiffeCSIDriver defines the configuration for the SPIFFE CSI Driver managed by zero trust workload identity manager.
// This includes settings related to the registration, socket paths, plugin name and optional runtime flags that influence how the driver operates.
type SpiffeCSIDriver struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              SpiffeCSIDriverSpec   `json:"spec,omitempty"`
	Status            SpiffeCSIDriverStatus `json:"status,omitempty"`
}

// SpiffeCSIDriverSpec will have specifications for configuration related to the spiffe-csi driver.
type SpiffeCSIDriverSpec struct {

	// agentSocketPath is the path to spiffe csi driver the agent socket.
	// +kubebuilder:default:="/run/spire/agent-sockets/spire-agent.sock"
	AgentSocket string `json:"agentSocketPath,omitempty"`

	// pluginName defines the name of the CSI plugin, Sets the csi driver name deployed to the cluster.
	// +kubebuilder:default:="csi.spiffe.io"
	PluginName string `json:"pluginName,omitempty"`

	CommonConfig `json:",inline"`
}

// SpiffeCSIDriverStatus defines the observed state of spiffe csi driver related reconciliation  made by operator
type SpiffeCSIDriverStatus struct {
	// conditions holds information of the states of spiffe csi driver related changes.
	ConditionalStatus `json:",inline,omitempty"`
}

// GetConditionalStatus returns the conditional status of the SpiffeCSIDriver
func (s *SpiffeCSIDriver) GetConditionalStatus() ConditionalStatus {
	return s.Status.ConditionalStatus
}

// +kubebuilder:object:root=true
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// SpiffeCSIDriverList contain the list of SpiffeCSIDriver
type SpiffeCSIDriverList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []SpiffeCSIDriver `json:"items"`
}

func init() {
	SchemeBuilder.Register(&SpiffeCSIDriver{}, &SpiffeCSIDriverList{})
}
