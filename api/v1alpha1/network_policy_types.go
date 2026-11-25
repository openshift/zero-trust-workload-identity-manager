package v1alpha1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// AppliedNetworkPolicy is one operator-applied NetworkPolicy and the time it was last created or updated.
type AppliedNetworkPolicy struct {
	// name is the NetworkPolicy metadata.name.
	Name string `json:"name"`

	// lastAppliedTime is when the operator last successfully created or updated this NetworkPolicy.
	LastAppliedTime metav1.Time `json:"lastAppliedTime"`
}

// NetworkPolicyManagementState mirrors effective spec.managedNetworkPolicy on the operand.
// - Managed: spec.managedNetworkPolicy is "true" or unset (default "true").
// - Unmanaged: spec.managedNetworkPolicy is "false"; the operator does not reconcile ztwim-sys-* NetworkPolicies for this operand.
// +kubebuilder:validation:Enum=Managed;Unmanaged
type NetworkPolicyManagementState string

const (
	NetworkPolicyManagementManaged   NetworkPolicyManagementState = "Managed"
	NetworkPolicyManagementUnmanaged NetworkPolicyManagementState = "Unmanaged"
)

// NetworkPolicyStatus reports operator-managed NetworkPolicy state for an operand.
type NetworkPolicyStatus struct {
	// managementState reflects whether the operator manages ztwim-sys-* policies for this operand.
	// Managed corresponds to spec.managedNetworkPolicy "true" (or unset/default); Unmanaged corresponds to "false".
	// +kubebuilder:validation:Optional
	ManagementState NetworkPolicyManagementState `json:"managementState,omitempty"`

	// lastAppliedPolicies lists NetworkPolicies the operator has successfully created or updated.
	// Entries are kept across reconciles, including when spec.managedNetworkPolicy changes to "false"
	// and when a later policy in the same reconcile fails.
	// A later create or update appends a new name or refreshes lastAppliedTime.
	// +kubebuilder:validation:Optional
	// +listType=atomic
	LastAppliedPolicies []AppliedNetworkPolicy `json:"lastAppliedPolicies,omitempty"`
}
