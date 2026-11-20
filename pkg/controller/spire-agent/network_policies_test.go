package spire_agent

import (
	"context"
	"fmt"
	"testing"

	"github.com/go-logr/logr"
	"github.com/openshift/zero-trust-workload-identity-manager/api/v1alpha1"
	"github.com/openshift/zero-trust-workload-identity-manager/pkg/client/fakes"
	"github.com/openshift/zero-trust-workload-identity-manager/pkg/controller/status"
	networkingv1 "k8s.io/api/networking/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func newNetworkPolicyTestReconciler(fakeClient *fakes.FakeCustomCtrlClient) *SpireAgentReconciler {
	scheme := runtime.NewScheme()
	_ = v1alpha1.AddToScheme(scheme)
	_ = networkingv1.AddToScheme(scheme)
	return &SpireAgentReconciler{
		ctrlClient:    fakeClient,
		ctx:           context.Background(),
		log:           logr.Discard(),
		scheme:        scheme,
		eventRecorder: record.NewFakeRecorder(100),
	}
}

func TestReconcileNetworkPolicies_ManagedDisabled(t *testing.T) {
	t.Setenv("OPERATOR_NAMESPACE", "zero-trust-workload-identity-manager")

	fc := &fakes.FakeCustomCtrlClient{}
	fc.StatusUpdateWithRetryReturns(nil)
	reconciler := newNetworkPolicyTestReconciler(fc)
	statusMgr := status.NewManager(fc)

	agent := &v1alpha1.SpireAgent{
		ObjectMeta: metav1.ObjectMeta{Name: "cluster", UID: "uid-1"},
		Spec: v1alpha1.SpireAgentSpec{
			ManagedNetworkPolicy: "false",
		},
	}

	if err := reconciler.reconcileNetworkPolicies(context.Background(), agent, statusMgr, false); err != nil {
		t.Fatalf("reconcileNetworkPolicies: %v", err)
	}
	if fc.CreateCallCount() != 0 {
		t.Fatalf("expected no NetworkPolicy create, got %d", fc.CreateCallCount())
	}
	if agent.Status.NetworkPolicy == nil || agent.Status.NetworkPolicy.ManagementState != v1alpha1.NetworkPolicyManagementUnmanaged {
		t.Fatalf("expected unmanaged network policy status, got %+v", agent.Status.NetworkPolicy)
	}
}

func TestReconcileNetworkPolicies_CreatesBaselinePolicies(t *testing.T) {
	t.Setenv("OPERATOR_NAMESPACE", "zero-trust-workload-identity-manager")

	fc := &fakes.FakeCustomCtrlClient{}
	fc.GetReturns(kerrors.NewNotFound(schema.GroupResource{Resource: "networkpolicies"}, "np"))
	fc.CreateReturns(nil)
	fc.StatusUpdateWithRetryReturns(nil)
	reconciler := newNetworkPolicyTestReconciler(fc)
	statusMgr := status.NewManager(fc)

	agent := &v1alpha1.SpireAgent{
		ObjectMeta: metav1.ObjectMeta{Name: "cluster", UID: "uid-1"},
		Spec:       v1alpha1.SpireAgentSpec{ManagedNetworkPolicy: "true"},
	}

	if err := reconciler.reconcileNetworkPolicies(context.Background(), agent, statusMgr, false); err != nil {
		t.Fatalf("reconcileNetworkPolicies: %v", err)
	}
	if fc.CreateCallCount() != len(spireAgentBaselineNetworkPolicyAssets) {
		t.Fatalf("expected %d creates, got %d", len(spireAgentBaselineNetworkPolicyAssets), fc.CreateCallCount())
	}
	if agent.Status.NetworkPolicy == nil || len(agent.Status.NetworkPolicy.LastAppliedPolicies) != len(spireAgentBaselineNetworkPolicyAssets) {
		t.Fatalf("unexpected applied policies: %+v", agent.Status.NetworkPolicy)
	}

	if err := statusMgr.ApplyStatus(context.Background(), agent, func() *v1alpha1.ConditionalStatus {
		return &agent.Status.ConditionalStatus
	}); err != nil {
		t.Fatalf("ApplyStatus: %v", err)
	}
	npCond := apimeta.FindStatusCondition(agent.Status.Conditions, v1alpha1.NetworkPoliciesAvailable)
	if npCond == nil || npCond.Reason != v1alpha1.ReasonNetworkPolicyCreated {
		t.Fatalf("expected NetworkPolicyCreated, got %+v", npCond)
	}
}

func staleAgentIngressNetworkPolicy(agent *v1alpha1.SpireAgent) *networkingv1.NetworkPolicy {
	desired := networkPolicyFromAsset(spireAgentBaselineNetworkPolicyAssets[0], agent.Spec.Labels)
	stale := desired.DeepCopy()
	stale.ResourceVersion = "42"
	delete(stale.Labels, "app.kubernetes.io/managed-by")
	return stale
}

func TestReconcileNetworkPolicies_UpdatesStalePolicy(t *testing.T) {
	t.Setenv("OPERATOR_NAMESPACE", "zero-trust-workload-identity-manager")

	agent := &v1alpha1.SpireAgent{
		ObjectMeta: metav1.ObjectMeta{Name: "cluster", UID: "uid-1"},
		Spec:       v1alpha1.SpireAgentSpec{ManagedNetworkPolicy: "true"},
	}
	stale := staleAgentIngressNetworkPolicy(agent)

	fc := &fakes.FakeCustomCtrlClient{}
	fc.GetStub = func(_ context.Context, _ client.ObjectKey, obj client.Object) error {
		np, ok := obj.(*networkingv1.NetworkPolicy)
		if !ok {
			return fmt.Errorf("unexpected type %T", obj)
		}
		*np = *stale
		return nil
	}
	fc.UpdateReturns(nil)

	reconciler := newNetworkPolicyTestReconciler(fc)
	statusMgr := status.NewManager(fc)

	if err := reconciler.reconcileNetworkPolicies(context.Background(), agent, statusMgr, false); err != nil {
		t.Fatalf("reconcileNetworkPolicies: %v", err)
	}
	if fc.UpdateCallCount() != 1 {
		t.Fatalf("expected 1 update, got %d", fc.UpdateCallCount())
	}
	if err := statusMgr.ApplyStatus(context.Background(), agent, func() *v1alpha1.ConditionalStatus {
		return &agent.Status.ConditionalStatus
	}); err != nil {
		t.Fatalf("ApplyStatus: %v", err)
	}
	npCond := apimeta.FindStatusCondition(agent.Status.Conditions, v1alpha1.NetworkPoliciesAvailable)
	if npCond == nil || npCond.Reason != v1alpha1.ReasonNetworkPolicyUpdated {
		t.Fatalf("expected NetworkPolicyUpdated, got %+v", npCond)
	}
}

func TestReconcileNetworkPolicies_CreateOnlySkipsDrift(t *testing.T) {
	t.Setenv("OPERATOR_NAMESPACE", "zero-trust-workload-identity-manager")

	agent := &v1alpha1.SpireAgent{
		ObjectMeta: metav1.ObjectMeta{Name: "cluster", UID: "uid-1"},
		Spec:       v1alpha1.SpireAgentSpec{ManagedNetworkPolicy: "true"},
	}
	agent.Status.Conditions = []metav1.Condition{{
		Type:   v1alpha1.NetworkPoliciesAvailable,
		Status: metav1.ConditionTrue,
		Reason: v1alpha1.ReasonNetworkPolicyCreated,
	}}
	stale := staleAgentIngressNetworkPolicy(agent)

	fc := &fakes.FakeCustomCtrlClient{}
	fc.GetStub = func(_ context.Context, _ client.ObjectKey, obj client.Object) error {
		np, ok := obj.(*networkingv1.NetworkPolicy)
		if !ok {
			return fmt.Errorf("unexpected type %T", obj)
		}
		*np = *stale
		return nil
	}

	reconciler := newNetworkPolicyTestReconciler(fc)
	statusMgr := status.NewManager(fc)

	if err := reconciler.reconcileNetworkPolicies(context.Background(), agent, statusMgr, true); err != nil {
		t.Fatalf("reconcileNetworkPolicies: %v", err)
	}
	if fc.UpdateCallCount() != 0 {
		t.Fatalf("expected no update in create-only mode, got %d", fc.UpdateCallCount())
	}
	if err := statusMgr.ApplyStatus(context.Background(), agent, func() *v1alpha1.ConditionalStatus {
		return &agent.Status.ConditionalStatus
	}); err != nil {
		t.Fatalf("ApplyStatus: %v", err)
	}
	npCond := apimeta.FindStatusCondition(agent.Status.Conditions, v1alpha1.NetworkPoliciesAvailable)
	if npCond == nil || npCond.Reason != v1alpha1.ReasonNetworkPolicyCreated {
		t.Fatalf("expected prior NetworkPolicyCreated preserved, got %+v", npCond)
	}
}
