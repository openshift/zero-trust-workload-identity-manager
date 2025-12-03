/*
Copyright 2025.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster
// +kubebuilder:validation:XValidation:rule="self.metadata.name == 'cluster'",message="ZeroTrustWorkloadIdentityManager is a singleton, .metadata.name must be 'cluster'"
// +operator-sdk:csv:customresourcedefinitions:displayName="ZeroTrustWorkloadIdentityManager"

// ZeroTrustWorkloadIdentityManager defines the configuration for the
// operator that manages the lifecycle of SPIRE components in OpenShift
// clusters.
//
// Note: This resource is *intended as a global config for operands managed
// by zero-trust-workload-identity-manager. It does not contain
// low-level configuration for SPIRE components, which is managed separately
// in the SpireConfig CRD.
type ZeroTrustWorkloadIdentityManager struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              ZeroTrustWorkloadIdentityManagerSpec   `json:"spec,omitempty"`
	Status            ZeroTrustWorkloadIdentityManagerStatus `json:"status,omitempty"`
}

// ZeroTrustWorkloadIdentityManagerStatus defines the observed state of ZeroTrustWorkloadIdentityManager.
// It aggregates the status from all managed operand CRs and provides an overall health view.
type ZeroTrustWorkloadIdentityManagerStatus struct {
	// conditions represent the latest available observations of the zero-trust-workload-identity-manager's state.
	// This includes the aggregated status from all managed operand CRs.
	ConditionalStatus `json:",inline,omitempty"`

	// operands holds the status of each managed operand CR.
	// Operands are indexed by their kind since all operands are named "cluster".
	// This provides a quick overview of the health of each SPIRE component.
	// +optional
	// +listType=map
	// +listMapKey=kind
	Operands []OperandStatus `json:"operands,omitempty"`
}

// OperandStatus represents the status of a single managed operand CR.
// Each operand corresponds to a SPIRE component (e.g., SpireServer, SpireAgent).
type OperandStatus struct {
	// name is the name of the operand resource.
	// For singleton resources, this is typically "cluster".
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=253
	// +required
	Name string `json:"name"`

	// kind is the Kind of the operand CR.
	// Must be one of: SpireServer, SpireAgent, SpiffeCSIDriver, SpireOIDCDiscoveryProvider.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Enum=SpireServer;SpireAgent;SpiffeCSIDriver;SpireOIDCDiscoveryProvider
	// +required
	Kind string `json:"kind"`

	// ready indicates whether the operand is in a ready state.
	// An operand is considered ready when all its resources are available and functioning correctly.
	// Valid values are "true" and "false".
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Pattern=`^(true|false)$`
	// +required
	Ready string `json:"ready"`

	// message provides human-readable details about the operand's current state.
	// This may include information about why an operand is not ready or other relevant status details.
	// +optional
	// +kubebuilder:validation:MaxLength=32768
	Message string `json:"message,omitempty"`

	// conditions represent the latest available observations of the operand's state.
	// This includes key conditions from the operand CR that are relevant for overall health monitoring.
	// +optional
	// +listType=map
	// +listMapKey=type
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ZeroTrustWorkloadIdentityManagerList contains a list of ZeroTrustWorkloadIdentityManager
type ZeroTrustWorkloadIdentityManagerList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ZeroTrustWorkloadIdentityManager `json:"items"`
}

// ZeroTrustWorkloadIdentityManagerSpec defines the desired state of ZeroTrustWorkloadIdentityManager
type ZeroTrustWorkloadIdentityManagerSpec struct {
	CommonConfig `json:",inline"`

	// trustDomain to be used for the SPIFFE identifiers.
	// This field is immutable.
	// Must be a valid SPIFFE trust domain (lowercase alphanumeric, hyphens, and dots).
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=255
	// +kubebuilder:validation:Pattern=`^[a-z0-9]([a-z0-9\-\.]*[a-z0-9])?$`
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="trustDomain is immutable and cannot be changed"
	TrustDomain string `json:"trustDomain,omitempty"`

	// clusterName will have the cluster name required to configure spire agent.
	// This field is immutable.
	// Must be a valid DNS-1123 subdomain.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=63
	// +kubebuilder:validation:Pattern=`^[a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*$`
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="clusterName is immutable and cannot be changed"
	ClusterName string `json:"clusterName,omitempty"`

	// bundleConfigMap is Configmap name for Spire bundle, it sets the trust domain to be used for the SPIFFE identifiers.
	// This field is immutable.
	// Must be a valid Kubernetes name.
	// +kubebuilder:validation:Optional
	// +kubebuilder:default:=spire-bundle
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=253
	// +kubebuilder:validation:Pattern=`^[a-z0-9]([-a-z0-9.]*[a-z0-9])?$`
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="bundleConfigMap is immutable and cannot be changed"
	BundleConfigMap string `json:"bundleConfigMap"`
}

// CommonConfig will have similar config required for all other APIs
type CommonConfig struct {

	// labels to apply to all resources managed by the API.
	// Maximum 64 labels allowed. Label keys and values must be valid Kubernetes labels.
	// +mapType=granular
	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:MaxProperties=64
	Labels map[string]string `json:"labels,omitempty"`

	// resources are for defining the resource requirements.
	// ref: https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/
	// +kubebuilder:validation:Optional
	Resources *corev1.ResourceRequirements `json:"resources,omitempty"`

	// affinity is for setting scheduling affinity rules.
	// ref: https://kubernetes.io/docs/concepts/scheduling-eviction/assign-pod-node/
	// +kubebuilder:validation:Optional
	Affinity *corev1.Affinity `json:"affinity,omitempty"`

	// tolerations are for setting the pod tolerations.
	// Maximum 50 tolerations allowed.
	// ref: https://kubernetes.io/docs/concepts/scheduling-eviction/taint-and-toleration/
	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:MaxItems=50
	// +listType=atomic
	Tolerations []*corev1.Toleration `json:"tolerations,omitempty"`

	// nodeSelector is for defining the scheduling criteria using node labels.
	// Maximum 50 node selectors allowed.
	// ref: https://kubernetes.io/docs/concepts/configuration/assign-pod-node/
	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:MaxProperties=50
	// +mapType=atomic
	NodeSelector map[string]string `json:"nodeSelector,omitempty"`
}

func init() {
	SchemeBuilder.Register(&ZeroTrustWorkloadIdentityManager{}, &ZeroTrustWorkloadIdentityManagerList{})
}
