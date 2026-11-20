package status

import (
	"fmt"

	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/openshift/zero-trust-workload-identity-manager/api/v1alpha1"
)

// SetOperandNetworkPoliciesAvailableCondition updates NetworkPoliciesAvailable using the same
// pattern as managed Route reconciliation: Created/Updated on changes, Ready when in sync but
// the condition is not yet True, and no condition write on steady state when already healthy.
func SetOperandNetworkPoliciesAvailableCondition(
	statusMgr *Manager,
	conditions []metav1.Condition,
	created, updated, driftSkipped bool,
	policyCount int,
) {
	if created {
		statusMgr.AddCondition(
			v1alpha1.NetworkPoliciesAvailable,
			v1alpha1.ReasonNetworkPolicyCreated,
			fmt.Sprintf("Created %d operator NetworkPolicies", policyCount),
			metav1.ConditionTrue,
		)
		return
	}
	if updated {
		statusMgr.AddCondition(
			v1alpha1.NetworkPoliciesAvailable,
			v1alpha1.ReasonNetworkPolicyUpdated,
			fmt.Sprintf("Updated operator NetworkPolicies (%d total)", policyCount),
			metav1.ConditionTrue,
		)
		return
	}
	if driftSkipped {
		return
	}

	existing := apimeta.FindStatusCondition(conditions, v1alpha1.NetworkPoliciesAvailable)
	if existing == nil || existing.Status != metav1.ConditionTrue {
		statusMgr.AddCondition(
			v1alpha1.NetworkPoliciesAvailable,
			v1alpha1.ReasonNetworkPoliciesReady,
			fmt.Sprintf("All %d operator NetworkPolicies up to date", policyCount),
			metav1.ConditionTrue,
		)
	}
}
