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

// SpiffeCSIDriverSpec defines the specifications for configuration related to the SPIFFE CSI driver.
type SpiffeCSIDriverSpec struct {

	// agentSocketPath is the path to the directory containing the SPIRE agent's Workload API socket.
	// This directory will be bind-mounted into workload containers by the CSI driver.
	// The directory is shared between the SPIRE agent and CSI driver via a hostPath volume.
	// Must be an absolute path without traversal attempts or null bytes.
	// +kubebuilder:validation:MaxLength=256
	// +kubebuilder:validation:Pattern=`^/[a-zA-Z0-9._/\-]*$`
	// +kubebuilder:default:="/run/spire/agent-sockets"
	AgentSocketPath string `json:"agentSocketPath,omitempty"`

	// pluginName specifies the name of the CSI plugin.
	// This sets the CSI driver name that will be deployed to the cluster and used in
	// VolumeMount configurations. Must match the driver name referenced in workload pods.
	// Must be a valid domain name format (e.g., csi.spiffe.io).
	// +kubebuilder:validation:MaxLength=127
	// +kubebuilder:validation:Pattern=`^[a-z0-9]([-a-z0-9.]*[a-z0-9])?$`
	// +kubebuilder:default:="csi.spiffe.io"
	PluginName string `json:"pluginName,omitempty"`

	CommonConfig `json:",inline"`
}

// SpiffeCSIDriverStatus defines the observed state of the SPIFFE CSI driver reconciliation performed by the operator
type SpiffeCSIDriverStatus struct {
	// conditions holds information about the current state of the SPIFFE CSI driver deployment.
	ConditionalStatus `json:",inline,omitempty"`
}

// GetConditionalStatus returns the conditional status of the SpiffeCSIDriver
func (s *SpiffeCSIDriver) GetConditionalStatus() ConditionalStatus {
	return s.Status.ConditionalStatus
}

// +kubebuilder:object:root=true
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// SpiffeCSIDriverList contains a list of SpiffeCSIDriver
type SpiffeCSIDriverList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []SpiffeCSIDriver `json:"items"`
}

func init() {
	SchemeBuilder.Register(&SpiffeCSIDriver{}, &SpiffeCSIDriverList{})
}
