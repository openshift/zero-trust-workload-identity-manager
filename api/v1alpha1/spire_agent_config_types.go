package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster
// +kubebuilder:validation:XValidation:rule="self.metadata.name == 'cluster'",message="SpireAgentConfig is a singleton, .metadata.name must be 'cluster'"
// +operator-sdk:csv:customresourcedefinitions:displayName="SpireAgentConfig"

// SpireAgentConfig defines the configuration for the SPIRE Agent managed by zero trust workload identity manager.
// The agent runs on each node and is responsible for node attestation,
// SVID rotation, and exposing the Workload API to local workloads.
type SpireAgentConfig struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              SpireAgentConfigSpec   `json:"spec,omitempty"`
	Status            SpireAgentConfigStatus `json:"status,omitempty"`
}

// SpireAgentConfigSpec will have specifications for configuration related to the spire agents.
type SpireAgentConfigSpec struct {

	// trustDomain to be used for the SPIFFE identifiers
	// +kubebuilder:validation:Required
	TrustDomain string `json:"trustDomain,omitempty"`

	// clusterName will have the cluster name required to configure spire agent.
	// +kubebuilder:validation:Required
	ClusterName string `json:"clusterName,omitempty"`

	// bundleConfigMap is Configmap name for Spire bundle, it sets the trust domain to be used for the SPIFFE identifiers
	// +kubebuilder:validation:Optional
	// +kubebuilder:default:=spire-bundle
	BundleConfigMap string `json:"bundleConfigMap"`

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
	// k8sPSATEnabled tells if k8sPSAT configuration is enabled
	// +kubebuilder:default:="true"
	// +kubebuilder:validation:Enum:="true";"false"
	// +kubebuilder:validation:Optional
	K8sPSATEnabled string `json:"k8sPSATEnabled,omitempty"`
}

// WorkloadAttestors defines the configuration for the Workload Attestors.
// +kubebuilder:validation:Optional
type WorkloadAttestors struct {

	// k8sEnabled explains if the configuration is enabled for k8s.
	// +kubebuilder:default:="true"
	// +kubebuilder:validation:Enum:="true";"false"
	// +kubebuilder:validation:Optional
	K8sEnabled string `json:"k8sEnabled,omitempty"`

	// workloadAttestorsVerification tells what kind of verification to do against kubelet.
	// auto will first attempt to use hostCert, and then fall back to apiServerCA.
	// Valid options are [auto, hostCert, apiServerCA, skip]
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

type WorkloadAttestorsVerification struct {
	// type specifies the type of verification to be used.
	// +kubebuilder: default:="skip"
	Type string `json:"type,omitempty"`

	// hostCertBasePath specifies the base Path where kubelet places its certificates.
	// +kubebuilder:validation:Optional
	// +kubebuilder:default:="/var/lib/kubelet/pki"
	HostCertBasePath string `json:"hostCertBasePath,omitempty"`

	// hostCertFileName specifies the file name for the host certificate.
	// +kubebuilder:validation:Optional
	HostCertFileName string `json:"hostCertFileName,omitempty"`
}

// SpireAgentConfigStatus defines the observed state of spire agents related reconciliation made by operator
type SpireAgentConfigStatus struct {
	// conditions holds information of the current state of the spire agents deployment.
	ConditionalStatus `json:",inline,omitempty"`
}

// +kubebuilder:object:root=true
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// SpireAgentConfigList contain the list of SpireAgentConfig
type SpireAgentConfigList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []SpireAgentConfig `json:"items"`
}

func init() {
	SchemeBuilder.Register(&SpireAgentConfig{}, &SpireAgentConfigList{})
}
