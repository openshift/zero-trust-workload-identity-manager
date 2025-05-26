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

// ZeroTrustWorkloadIdentityManagerStatus defines the observed state of ZeroTrustWorkloadIdentityManager
type ZeroTrustWorkloadIdentityManagerStatus struct {
	// conditions holds information of the current state of the zero-trust-workload-identity-manager deployment.
	ConditionalStatus `json:",inline,omitempty"`
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
	// logLevel supports value range as per [kubernetes logging guidelines](https://github.com/kubernetes/community/blob/master/contributors/devel/sig-instrumentation/logging.md#what-method-to-use).
	// +kubebuilder:default:=1
	// +kubebuilder:validation:Minimum:=1
	// +kubebuilder:validation:Maximum:=5
	// +kubebuilder:validation:Optional
	LogLevel int32 `json:"logLevel,omitempty"`

	// namespace to install the deployments and other resources managed by
	// zero-trust-workload-identity-manager.
	// +kubebuilder:validation:Optional
	// +kubebuilder:default:="zero-trust-workload-identity-manager"
	Namespace string `json:"namespace,omitempty"`

	CommonConfig `json:",inline"`
}

// CommonConfig will have similar config required for all other APIs
type CommonConfig struct {
	// labels to apply to all resources managed by the API.
	// +mapType=granular
	// +kubebuilder:validation:Optional
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
	// ref: https://kubernetes.io/docs/concepts/scheduling-eviction/taint-and-toleration/
	// +kubebuilder:validation:Optional
	// +listType=atomic
	Tolerations []*corev1.Toleration `json:"tolerations,omitempty"`

	// nodeSelector is for defining the scheduling criteria using node labels.
	// ref: https://kubernetes.io/docs/concepts/configuration/assign-pod-node/
	// +kubebuilder:validation:Optional
	// +mapType=atomic
	NodeSelector map[string]string `json:"nodeSelector,omitempty"`
}

func init() {
	SchemeBuilder.Register(&ZeroTrustWorkloadIdentityManager{}, &ZeroTrustWorkloadIdentityManagerList{})
}
