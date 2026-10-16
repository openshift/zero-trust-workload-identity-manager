package zero_trust_workload_identity_manager

import (
	"context"
	"fmt"
	"strings"

	apierror "k8s.io/apimachinery/pkg/api/errors"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/openshift/zero-trust-workload-identity-manager/api/v1alpha1"
)

type operandNetworkPolicyState int

const (
	operandNetworkPolicyPending operandNetworkPolicyState = iota
	operandNetworkPolicyReady
	operandNetworkPolicyDisabled
	operandNetworkPolicyFailed
)

type networkPolicyOperandSnapshot struct {
	Kind    string
	State   operandNetworkPolicyState
	Message string
}

type networkPolicyAggregateResult struct {
	Status  metav1.ConditionStatus
	Reason  string
	Message string
}

func (r *ZeroTrustWorkloadIdentityManagerReconciler) aggregateOperandNetworkPolicies(ctx context.Context) networkPolicyAggregateResult {
	snapshots := []networkPolicyOperandSnapshot{
		r.networkPolicySnapshotForSpireServer(ctx),
		r.networkPolicySnapshotForSpireAgent(ctx),
		r.networkPolicySnapshotForSpireOIDC(ctx),
	}
	return summarizeNetworkPolicySnapshots(snapshots)
}

func (r *ZeroTrustWorkloadIdentityManagerReconciler) networkPolicySnapshotForSpireServer(ctx context.Context) networkPolicyOperandSnapshot {
	var server v1alpha1.SpireServer
	if err := r.ctrlClient.Get(ctx, types.NamespacedName{Name: "cluster"}, &server); err != nil {
		if apierror.IsNotFound(err) {
			return networkPolicyOperandSnapshot{Kind: "SpireServer", State: operandNetworkPolicyPending, Message: OperandMessageCRNotFound}
		}
		return networkPolicyOperandSnapshot{Kind: "SpireServer", State: operandNetworkPolicyFailed, Message: err.Error()}
	}
	return snapshotFromConditions("SpireServer", server.Status.Conditions)
}

func (r *ZeroTrustWorkloadIdentityManagerReconciler) networkPolicySnapshotForSpireAgent(ctx context.Context) networkPolicyOperandSnapshot {
	var agent v1alpha1.SpireAgent
	if err := r.ctrlClient.Get(ctx, types.NamespacedName{Name: "cluster"}, &agent); err != nil {
		if apierror.IsNotFound(err) {
			return networkPolicyOperandSnapshot{Kind: "SpireAgent", State: operandNetworkPolicyPending, Message: OperandMessageCRNotFound}
		}
		return networkPolicyOperandSnapshot{Kind: "SpireAgent", State: operandNetworkPolicyFailed, Message: err.Error()}
	}
	return snapshotFromConditions("SpireAgent", agent.Status.Conditions)
}

func (r *ZeroTrustWorkloadIdentityManagerReconciler) networkPolicySnapshotForSpireOIDC(ctx context.Context) networkPolicyOperandSnapshot {
	var oidc v1alpha1.SpireOIDCDiscoveryProvider
	if err := r.ctrlClient.Get(ctx, types.NamespacedName{Name: "cluster"}, &oidc); err != nil {
		if apierror.IsNotFound(err) {
			return networkPolicyOperandSnapshot{Kind: "SpireOIDCDiscoveryProvider", State: operandNetworkPolicyPending, Message: OperandMessageCRNotFound}
		}
		return networkPolicyOperandSnapshot{Kind: "SpireOIDCDiscoveryProvider", State: operandNetworkPolicyFailed, Message: err.Error()}
	}
	return snapshotFromConditions("SpireOIDCDiscoveryProvider", oidc.Status.Conditions)
}

func snapshotFromConditions(kind string, conditions []metav1.Condition) networkPolicyOperandSnapshot {
	if len(conditions) == 0 {
		return networkPolicyOperandSnapshot{Kind: kind, State: operandNetworkPolicyPending, Message: OperandMessageWaitingInitialRecon}
	}
	np := apimeta.FindStatusCondition(conditions, v1alpha1.NetworkPoliciesAvailable)
	if np == nil {
		return networkPolicyOperandSnapshot{Kind: kind, State: operandNetworkPolicyPending, Message: "NetworkPoliciesAvailable condition not set"}
	}
	if np.Status == metav1.ConditionTrue {
		return networkPolicyOperandSnapshot{Kind: kind, State: operandNetworkPolicyReady, Message: np.Message}
	}
	if np.Reason == v1alpha1.ReasonManagedNetworkPolicyDisabled {
		return networkPolicyOperandSnapshot{Kind: kind, State: operandNetworkPolicyDisabled, Message: np.Message}
	}
	return networkPolicyOperandSnapshot{Kind: kind, State: operandNetworkPolicyFailed, Message: np.Message}
}

func summarizeNetworkPolicySnapshots(snapshots []networkPolicyOperandSnapshot) networkPolicyAggregateResult {
	var pending []string
	var failed []string
	readyCount := 0

	for _, s := range snapshots {
		switch s.State {
		case operandNetworkPolicyReady, operandNetworkPolicyDisabled:
			readyCount++
		case operandNetworkPolicyPending:
			pending = append(pending, s.Kind)
		case operandNetworkPolicyFailed:
			failed = append(failed, fmt.Sprintf("%s(%s)", s.Kind, s.Message))
		}
	}

	if len(failed) > 0 {
		return networkPolicyAggregateResult{
			Status:  metav1.ConditionFalse,
			Reason:  v1alpha1.ReasonOperandNetworkPolicyFailed,
			Message: fmt.Sprintf("Operand network policies not healthy: %s", strings.Join(failed, ", ")),
		}
	}
	if readyCount == len(snapshots) {
		return networkPolicyAggregateResult{
			Status:  metav1.ConditionTrue,
			Reason:  v1alpha1.ReasonOperandsNetworkPoliciesReady,
			Message: "Network policies applied for all managed operands",
		}
	}
	return networkPolicyAggregateResult{
		Status:  metav1.ConditionFalse,
		Reason:  v1alpha1.ReasonInProgress,
		Message: fmt.Sprintf("Waiting for operand network policies: %s", strings.Join(pending, ", ")),
	}
}
