package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster
// +kubebuilder:validation:XValidation:rule="self.metadata.name == 'cluster'",message="SpireAgent is a singleton, .metadata.name must be 'cluster'"
// +operator-sdk:csv:customresourcedefinitions:displayName="SpireAgent"

// SpireAgent defines the configuration for the SPIRE Agent managed by zero trust workload identity manager.
// The agent runs on each node and is responsible for node attestation,
// SVID rotation, and exposing the Workload API to local workloads.
type SpireAgent struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              SpireAgentSpec   `json:"spec,omitempty"`
	Status            SpireAgentStatus `json:"status,omitempty"`
}

// SpireAgentSpec defines the specifications for configuring the SPIRE agent.
type SpireAgentSpec struct {

	// socketPath is the directory on the host where the SPIRE agent socket will be created.
	// This directory is shared with the SPIFFE CSI driver via hostPath volume.
	// Must match SpiffeCSIDriver.spec.agentSocketPath for workloads to access the socket.
	// Must be an absolute path without traversal attempts or null bytes.
	// +kubebuilder:validation:MaxLength=256
	// +kubebuilder:validation:Pattern=`^/[a-zA-Z0-9._/\-]*$`
	// +kubebuilder:default:="/run/spire/agent-sockets"
	SocketPath string `json:"socketPath,omitempty"`

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

	// nodeAttestor specifies the configuration for the Node Attestor.
	// +kubebuilder:validation:Optional
	NodeAttestor *NodeAttestor `json:"nodeAttestor,omitempty"`

	// workloadAttestors specifies the configuration for the Workload Attestors.
	// +kubebuilder:validation:Optional
	WorkloadAttestors *WorkloadAttestors `json:"workloadAttestors,omitempty"`

	CommonConfig `json:",inline"`
}

// NodeAttestor defines the configuration for the Node Attestor.
type NodeAttestor struct {
	// k8sPSATEnabled specifies whether Kubernetes Projected Service Account Token (PSAT)
	// node attestation is enabled. When enabled, the SPIRE agent uses K8s PSATs to prove
	// its identity to the SPIRE server during node attestation.
	// +kubebuilder:default:="true"
	// +kubebuilder:validation:Enum:="true";"false"
	// +kubebuilder:validation:Optional
	K8sPSATEnabled string `json:"k8sPSATEnabled,omitempty"`
}

// WorkloadAttestors defines the configuration for the Workload Attestors.
// +kubebuilder:validation:Optional
type WorkloadAttestors struct {

	// k8sEnabled specifies whether the Kubernetes workload attestor is enabled.
	// When enabled, the SPIRE agent can verify workload identities using Kubernetes
	// pod information and service account tokens.
	// +kubebuilder:default:="true"
	// +kubebuilder:validation:Enum:="true";"false"
	// +kubebuilder:validation:Optional
	K8sEnabled string `json:"k8sEnabled,omitempty"`

	// workloadAttestorsVerification configures how the SPIRE agent verifies the kubelet's TLS certificate
	// +kubebuilder:validation:Optional
	WorkloadAttestorsVerification *WorkloadAttestorsVerification `json:"workloadAttestorsVerification,omitempty"`

	// disableContainerSelectors specifies whether to disable container selectors in the Kubernetes workload attestor.
	// Set to true if using holdApplicationUntilProxyStarts in Istio
	// +kubebuilder:default:="false"
	// +kubebuilder:validation:Enum:="true";"false"
	// +kubebuilder:validation:Optional
	DisableContainerSelectors string `json:"disableContainerSelectors,omitempty"`

	// useNewContainerLocator enables the new container locator algorithm that has support for cgroups v2.
	// Defaults to true
	// +kubebuilder:default:="true"
	// +kubebuilder:validation:Enum:="true";"false"
	// +kubebuilder:validation:Optional
	UseNewContainerLocator string `json:"useNewContainerLocator,omitempty"`
}

// WorkloadAttestorsVerification configures kubelet TLS certificate verification.
// +kubebuilder:validation:Optional
// +kubebuilder:validation:XValidation:rule="self.type != 'hostCert' || (has(self.hostCertBasePath) && self.hostCertBasePath != '')",message="hostCertBasePath is required when type is 'hostCert'"
// +kubebuilder:validation:XValidation:rule="self.type != 'hostCert' || (has(self.hostCertFileName) && self.hostCertFileName != '')",message="hostCertFileName is required when type is 'hostCert'"
type WorkloadAttestorsVerification struct {
	// type specifies the kubelet certificate verification mode.
	// - skip: Skip TLS verification entirely.
	// - auto: Verify kubelet certificate using OpenShift defaults (/etc/kubernetes/kubelet-ca.crt)
	//   unless hostCertBasePath and hostCertFileName are explicitly specified.
	// - hostCert: Use a custom CA certificate for kubelet verification. Requires hostCertBasePath
	//   and hostCertFileName to be specified.
	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:Enum=auto;hostCert;skip
	// +kubebuilder:default:="auto"
	Type string `json:"type,omitempty"`

	// hostCertBasePath specifies the directory containing the kubelet CA certificate.
	// Required when type is "hostCert".
	// Optional when type is "auto" (defaults to "/etc/kubernetes" if not specified).
	// +kubebuilder:validation:Optional
	HostCertBasePath string `json:"hostCertBasePath,omitempty"`

	// hostCertFileName specifies the file name for the kubelet's CA certificate.
	// Combined with hostCertBasePath to form the full path for SPIRE's kubelet_ca_path.
	// Required when type is "hostCert".
	// Optional when type is "auto" (defaults to "kubelet-ca.crt" if not specified).
	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:MaxLength=256
	// +kubebuilder:validation:Pattern=`^[a-zA-Z0-9._-]+$`
	HostCertFileName string `json:"hostCertFileName,omitempty"`
}

// SpireAgentStatus defines the observed state of the SPIRE agent reconciliation performed by the operator.
type SpireAgentStatus struct {
	// conditions holds information about the current state of the SPIRE agent deployment.
	ConditionalStatus `json:",inline,omitempty"`
}

// GetConditionalStatus returns the conditional status of the SpireAgent
func (s *SpireAgent) GetConditionalStatus() ConditionalStatus {
	return s.Status.ConditionalStatus
}

// +kubebuilder:object:root=true
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// SpireAgentList contains a list of SpireAgent
type SpireAgentList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []SpireAgent `json:"items"`
}

func init() {
	SchemeBuilder.Register(&SpireAgent{}, &SpireAgentList{})
}
