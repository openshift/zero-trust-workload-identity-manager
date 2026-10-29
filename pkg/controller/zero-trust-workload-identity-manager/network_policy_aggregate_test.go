package zero_trust_workload_identity_manager

import (
	"testing"

	"github.com/openshift/zero-trust-workload-identity-manager/api/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestSummarizeNetworkPolicySnapshots_AllReady(t *testing.T) {
	result := summarizeNetworkPolicySnapshots([]networkPolicyOperandSnapshot{
		{Kind: "SpireServer", State: operandNetworkPolicyReady},
		{Kind: "SpireAgent", State: operandNetworkPolicyReady},
		{Kind: "SpireOIDCDiscoveryProvider", State: operandNetworkPolicyReady},
	})
	if result.Status != metav1.ConditionTrue || result.Reason != v1alpha1.ReasonOperandsNetworkPoliciesReady {
		t.Fatalf("unexpected aggregate: status=%s reason=%s", result.Status, result.Reason)
	}
}

func TestSummarizeNetworkPolicySnapshots_DisabledCountsAsReady(t *testing.T) {
	result := summarizeNetworkPolicySnapshots([]networkPolicyOperandSnapshot{
		{Kind: "SpireServer", State: operandNetworkPolicyDisabled},
		{Kind: "SpireAgent", State: operandNetworkPolicyReady},
		{Kind: "SpireOIDCDiscoveryProvider", State: operandNetworkPolicyReady},
	})
	if result.Status != metav1.ConditionTrue {
		t.Fatalf("expected True when one operand is unmanaged, got %s", result.Status)
	}
}

func TestSummarizeNetworkPolicySnapshots_Failure(t *testing.T) {
	result := summarizeNetworkPolicySnapshots([]networkPolicyOperandSnapshot{
		{Kind: "SpireServer", State: operandNetworkPolicyFailed, Message: "create failed"},
		{Kind: "SpireAgent", State: operandNetworkPolicyReady},
		{Kind: "SpireOIDCDiscoveryProvider", State: operandNetworkPolicyReady},
	})
	if result.Status != metav1.ConditionFalse || result.Reason != v1alpha1.ReasonOperandNetworkPolicyFailed {
		t.Fatalf("unexpected failure aggregate: %+v", result)
	}
}

func TestSummarizeNetworkPolicySnapshots_Pending(t *testing.T) {
	result := summarizeNetworkPolicySnapshots([]networkPolicyOperandSnapshot{
		{Kind: "SpireServer", State: operandNetworkPolicyPending},
		{Kind: "SpireAgent", State: operandNetworkPolicyReady},
		{Kind: "SpireOIDCDiscoveryProvider", State: operandNetworkPolicyReady},
	})
	if result.Status != metav1.ConditionFalse || result.Reason != v1alpha1.ReasonInProgress {
		t.Fatalf("unexpected pending aggregate: %+v", result)
	}
}

func TestSnapshotFromConditions(t *testing.T) {
	disabled := snapshotFromConditions("SpireServer", []metav1.Condition{
		{Type: v1alpha1.NetworkPoliciesAvailable, Status: metav1.ConditionFalse, Reason: v1alpha1.ReasonManagedNetworkPolicyDisabled},
	})
	if disabled.State != operandNetworkPolicyDisabled {
		t.Fatalf("expected disabled state, got %v", disabled.State)
	}

	ready := snapshotFromConditions("SpireAgent", []metav1.Condition{
		{Type: v1alpha1.NetworkPoliciesAvailable, Status: metav1.ConditionTrue, Reason: v1alpha1.ReasonNetworkPoliciesCreated},
	})
	if ready.State != operandNetworkPolicyReady {
		t.Fatalf("expected ready state, got %v", ready.State)
	}
}
