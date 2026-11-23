package spire_agent

import (
	"context"
	"fmt"
	"testing"

	"github.com/go-logr/logr"
	"github.com/openshift/zero-trust-workload-identity-manager/api/v1alpha1"
	"github.com/openshift/zero-trust-workload-identity-manager/pkg/client/fakes"
	networkingv1 "k8s.io/api/networking/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func newNetworkPolicyTestReconciler(t *testing.T, fakeClient *fakes.FakeCustomCtrlClient) *SpireAgentReconciler {
	t.Helper()
	scheme := runtime.NewScheme()
	if err := v1alpha1.AddToScheme(scheme); err != nil {
		t.Fatalf("AddToScheme v1alpha1: %v", err)
	}
	if err := networkingv1.AddToScheme(scheme); err != nil {
		t.Fatalf("AddToScheme networkingv1: %v", err)
	}
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
	reconciler := newNetworkPolicyTestReconciler(t, fc)

	agent := &v1alpha1.SpireAgent{
		ObjectMeta: metav1.ObjectMeta{Name: "cluster", UID: "uid-1"},
		Spec: v1alpha1.SpireAgentSpec{
			ManagedNetworkPolicy: "false",
		},
	}

	if err := reconciler.reconcileNetworkPolicies(context.Background(), agent, false); err != nil {
		t.Fatalf("reconcileNetworkPolicies: %v", err)
	}
	if fc.CreateCallCount() != 0 {
		t.Fatalf("expected no NetworkPolicy create, got %d", fc.CreateCallCount())
	}
	if agent.Status.NetworkPolicy == nil || agent.Status.NetworkPolicy.ManagementState != v1alpha1.NetworkPolicyManagementUnmanaged {
		t.Fatalf("expected unmanaged network policy status, got %+v", agent.Status.NetworkPolicy)
	}
	if apimeta.FindStatusCondition(agent.Status.Conditions, v1alpha1.NetworkPoliciesAvailable) != nil {
		t.Fatal("expected no NetworkPoliciesAvailable condition")
	}
}

func TestReconcileNetworkPolicies_CreatesBaselinePolicies(t *testing.T) {
	t.Setenv("OPERATOR_NAMESPACE", "zero-trust-workload-identity-manager")

	fc := &fakes.FakeCustomCtrlClient{}
	fc.GetReturns(kerrors.NewNotFound(schema.GroupResource{Resource: "networkpolicies"}, "np"))
	fc.CreateReturns(nil)
	reconciler := newNetworkPolicyTestReconciler(t, fc)

	agent := &v1alpha1.SpireAgent{
		ObjectMeta: metav1.ObjectMeta{Name: "cluster", UID: "uid-1"},
		Spec:       v1alpha1.SpireAgentSpec{ManagedNetworkPolicy: "true"},
	}

	if err := reconciler.reconcileNetworkPolicies(context.Background(), agent, false); err != nil {
		t.Fatalf("reconcileNetworkPolicies: %v", err)
	}
	if fc.CreateCallCount() != len(spireAgentBaselineNetworkPolicyAssets) {
		t.Fatalf("expected %d creates, got %d", len(spireAgentBaselineNetworkPolicyAssets), fc.CreateCallCount())
	}
	if agent.Status.NetworkPolicy == nil || len(agent.Status.NetworkPolicy.LastAppliedPolicies) != len(spireAgentBaselineNetworkPolicyAssets) {
		t.Fatalf("unexpected applied policies: %+v", agent.Status.NetworkPolicy)
	}
	if apimeta.FindStatusCondition(agent.Status.Conditions, v1alpha1.NetworkPoliciesAvailable) != nil {
		t.Fatal("expected no NetworkPoliciesAvailable condition")
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

	reconciler := newNetworkPolicyTestReconciler(t, fc)

	if err := reconciler.reconcileNetworkPolicies(context.Background(), agent, false); err != nil {
		t.Fatalf("reconcileNetworkPolicies: %v", err)
	}
	if fc.UpdateCallCount() != 1 {
		t.Fatalf("expected 1 update, got %d", fc.UpdateCallCount())
	}
	policyName := networkPolicyFromAsset(spireAgentBaselineNetworkPolicyAssets[0], agent.Spec.Labels).Name
	if len(agent.Status.NetworkPolicy.LastAppliedPolicies) != 1 || agent.Status.NetworkPolicy.LastAppliedPolicies[0] != policyName {
		t.Fatalf("expected policy recorded after update, got %+v", agent.Status.NetworkPolicy.LastAppliedPolicies)
	}
}

func TestReconcileNetworkPolicies_RepairsMissingOwnerReference(t *testing.T) {
	t.Setenv("OPERATOR_NAMESPACE", "zero-trust-workload-identity-manager")

	agent := &v1alpha1.SpireAgent{
		ObjectMeta: metav1.ObjectMeta{Name: "cluster", UID: "uid-agent"},
		Spec:       v1alpha1.SpireAgentSpec{ManagedNetworkPolicy: "true"},
	}
	desired := networkPolicyFromAsset(spireAgentBaselineNetworkPolicyAssets[0], agent.Spec.Labels)
	existing := desired.DeepCopy()
	existing.ResourceVersion = "7"
	existing.OwnerReferences = nil

	fc := &fakes.FakeCustomCtrlClient{}
	fc.GetStub = func(_ context.Context, key client.ObjectKey, obj client.Object) error {
		np, ok := obj.(*networkingv1.NetworkPolicy)
		if !ok {
			return fmt.Errorf("unexpected type %T", obj)
		}
		if key.Name != desired.Name {
			return kerrors.NewNotFound(schema.GroupResource{Resource: "networkpolicies"}, key.Name)
		}
		*np = *existing
		return nil
	}
	fc.UpdateReturns(nil)

	reconciler := newNetworkPolicyTestReconciler(t, fc)
	if err := reconciler.reconcileNetworkPolicies(context.Background(), agent, false); err != nil {
		t.Fatalf("reconcileNetworkPolicies: %v", err)
	}
	if fc.UpdateCallCount() != 1 {
		t.Fatalf("expected 1 update to repair owner reference, got %d", fc.UpdateCallCount())
	}
}

func TestReconcileNetworkPolicies_CreateOnlySkipsDrift(t *testing.T) {
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

	reconciler := newNetworkPolicyTestReconciler(t, fc)

	if err := reconciler.reconcileNetworkPolicies(context.Background(), agent, true); err != nil {
		t.Fatalf("reconcileNetworkPolicies: %v", err)
	}
	if fc.UpdateCallCount() != 0 {
		t.Fatalf("expected no update in create-only mode, got %d", fc.UpdateCallCount())
	}
	if len(agent.Status.NetworkPolicy.LastAppliedPolicies) != 0 {
		t.Fatalf("create-only skip must not record policy as applied, got %+v", agent.Status.NetworkPolicy.LastAppliedPolicies)
	}
}

func TestReconcileNetworkPolicies_CreateOnlyPreservesPriorLastApplied(t *testing.T) {
	t.Setenv("OPERATOR_NAMESPACE", "zero-trust-workload-identity-manager")

	agent := &v1alpha1.SpireAgent{
		ObjectMeta: metav1.ObjectMeta{Name: "cluster", UID: "uid-1"},
		Spec:       v1alpha1.SpireAgentSpec{ManagedNetworkPolicy: "true"},
		Status: v1alpha1.SpireAgentStatus{
			NetworkPolicy: &v1alpha1.NetworkPolicyStatus{
				ManagementState:     v1alpha1.NetworkPolicyManagementManaged,
				LastAppliedPolicies: []string{"prior-policy"},
			},
		},
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

	reconciler := newNetworkPolicyTestReconciler(t, fc)
	if err := reconciler.reconcileNetworkPolicies(context.Background(), agent, true); err != nil {
		t.Fatalf("reconcileNetworkPolicies: %v", err)
	}
	if len(agent.Status.NetworkPolicy.LastAppliedPolicies) != 1 || agent.Status.NetworkPolicy.LastAppliedPolicies[0] != "prior-policy" {
		t.Fatalf("expected prior lastApplied preserved, got %+v", agent.Status.NetworkPolicy.LastAppliedPolicies)
	}
}

func TestReconcileNetworkPolicies_NoOpDoesNotRecordLastApplied(t *testing.T) {
	t.Setenv("OPERATOR_NAMESPACE", "zero-trust-workload-identity-manager")

	agent := &v1alpha1.SpireAgent{
		TypeMeta:   metav1.TypeMeta{APIVersion: v1alpha1.GroupVersion.String(), Kind: "SpireAgent"},
		ObjectMeta: metav1.ObjectMeta{Name: "cluster", UID: "uid-1"},
		Spec:       v1alpha1.SpireAgentSpec{ManagedNetworkPolicy: "true"},
	}
	reconciler := newNetworkPolicyTestReconciler(t, &fakes.FakeCustomCtrlClient{})
	desired := networkPolicyFromAsset(spireAgentBaselineNetworkPolicyAssets[0], agent.Spec.Labels)
	existing := desired.DeepCopy()
	existing.ResourceVersion = "1"
	if err := controllerutil.SetControllerReference(agent, existing, reconciler.scheme); err != nil {
		t.Fatalf("SetControllerReference: %v", err)
	}

	fc := &fakes.FakeCustomCtrlClient{}
	fc.GetStub = func(_ context.Context, _ client.ObjectKey, obj client.Object) error {
		np, ok := obj.(*networkingv1.NetworkPolicy)
		if !ok {
			return fmt.Errorf("unexpected type %T", obj)
		}
		*np = *existing
		return nil
	}
	reconciler.ctrlClient = fc

	if err := reconciler.reconcileNetworkPolicies(context.Background(), agent, false); err != nil {
		t.Fatalf("reconcileNetworkPolicies: %v", err)
	}
	if fc.UpdateCallCount() != 0 {
		t.Fatalf("expected no update, got %d", fc.UpdateCallCount())
	}
	if len(agent.Status.NetworkPolicy.LastAppliedPolicies) != 0 {
		t.Fatalf("no-op reconcile must not record policy as applied, got %+v", agent.Status.NetworkPolicy.LastAppliedPolicies)
	}
}