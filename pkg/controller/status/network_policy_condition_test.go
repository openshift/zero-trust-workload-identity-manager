package status

import (
	"context"
	"testing"

	"github.com/openshift/zero-trust-workload-identity-manager/api/v1alpha1"
	"github.com/openshift/zero-trust-workload-identity-manager/pkg/client/fakes"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestSetOperandNetworkPoliciesAvailableCondition(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		conditions     []metav1.Condition
		created        bool
		updated        bool
		driftSkipped   bool
		policyCount    int
		expectReason   string
		expectNoUpdate bool
	}{
		{
			name:         "created",
			created:      true,
			policyCount:  1,
			expectReason: v1alpha1.ReasonNetworkPolicyCreated,
		},
		{
			name:         "updated",
			updated:      true,
			policyCount:  2,
			expectReason: v1alpha1.ReasonNetworkPolicyUpdated,
		},
		{
			name:        "steady state already true",
			policyCount: 1,
			conditions: []metav1.Condition{{
				Type:   v1alpha1.NetworkPoliciesAvailable,
				Status: metav1.ConditionTrue,
				Reason: v1alpha1.ReasonNetworkPolicyCreated,
			}},
			expectNoUpdate: true,
		},
		{
			name:        "in sync but condition false",
			policyCount: 1,
			conditions: []metav1.Condition{{
				Type:   v1alpha1.NetworkPoliciesAvailable,
				Status: metav1.ConditionFalse,
				Reason: v1alpha1.ReasonManagedNetworkPolicyDisabled,
			}},
			expectReason: v1alpha1.ReasonNetworkPoliciesReady,
		},
		{
			name:           "drift skipped in create-only mode",
			driftSkipped:   true,
			policyCount:    1,
			expectNoUpdate: true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			fc := &fakes.FakeCustomCtrlClient{}
			statusMgr := NewManager(fc)

			agent := &v1alpha1.SpireAgent{}
			agent.Status.Conditions = append([]metav1.Condition(nil), tt.conditions...)

			SetOperandNetworkPoliciesAvailableCondition(statusMgr, agent.Status.Conditions, tt.created, tt.updated, tt.driftSkipped, tt.policyCount)

			if tt.expectNoUpdate {
				if _, ok := statusMgr.conditions[v1alpha1.NetworkPoliciesAvailable]; ok {
					t.Fatalf("expected no NetworkPoliciesAvailable condition queued")
				}
				return
			}

			if _, ok := statusMgr.conditions[v1alpha1.NetworkPoliciesAvailable]; !ok {
				t.Fatal("expected NetworkPoliciesAvailable condition queued")
			}

			if err := statusMgr.ApplyStatus(context.Background(), agent, func() *v1alpha1.ConditionalStatus {
				return &agent.Status.ConditionalStatus
			}); err != nil {
				t.Fatalf("ApplyStatus: %v", err)
			}

			cond := apimeta.FindStatusCondition(agent.Status.Conditions, v1alpha1.NetworkPoliciesAvailable)
			if cond == nil {
				t.Fatal("expected NetworkPoliciesAvailable condition")
			}
			if cond.Reason != tt.expectReason {
				t.Fatalf("reason: got %q want %q", cond.Reason, tt.expectReason)
			}
			if cond.Status != metav1.ConditionTrue {
				t.Fatalf("status: got %v want True", cond.Status)
			}
		})
	}
}
