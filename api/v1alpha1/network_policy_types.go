package v1alpha1

// NetworkPoliciesAvailable is the condition type reporting operand NetworkPolicy reconcile health.
const NetworkPoliciesAvailable = "NetworkPoliciesAvailable"

// NetworkPoliciesApplied is the ZTWIM condition type reporting aggregate operand network policy readiness.
const NetworkPoliciesApplied = "NetworkPoliciesApplied"

// Network policy condition reasons (operands).
const (
	ReasonNetworkPoliciesCreated       = "NetworkPoliciesCreated"
	ReasonNetworkPoliciesUpdated       = "NetworkPoliciesUpdated"
	ReasonManagedNetworkPolicyDisabled = "ManagedNetworkPolicyDisabled"
	ReasonNetworkPolicyCreationFailed  = "NetworkPolicyCreationFailed"
	ReasonNetworkPolicyUpdateFailed    = "NetworkPolicyUpdateFailed"
	ReasonNetworkPolicyDeletionFailed  = "NetworkPolicyDeletionFailed"
	ReasonOperandsNetworkPoliciesReady = "OperandsNetworkPoliciesReady"
	ReasonOperandNetworkPolicyFailed   = "OperandNetworkPolicyFailed"
)

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

	// lastAppliedPolicies lists fixed-name NetworkPolicy resources last applied by the operator.
	// +kubebuilder:validation:Optional
	// +listType=atomic
	LastAppliedPolicies []string `json:"lastAppliedPolicies,omitempty"`
}
