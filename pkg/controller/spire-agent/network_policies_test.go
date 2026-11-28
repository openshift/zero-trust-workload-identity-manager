package spire_agent

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/go-logr/logr"
	securityv1 "github.com/openshift/api/security/v1"
	"github.com/openshift/zero-trust-workload-identity-manager/api/v1alpha1"
	"github.com/openshift/zero-trust-workload-identity-manager/pkg/client/fakes"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
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
		Status: v1alpha1.SpireAgentStatus{
			NetworkPolicy: &v1alpha1.NetworkPolicyStatus{
				ManagementState:     v1alpha1.NetworkPolicyManagementManaged,
				LastAppliedPolicies: []v1alpha1.AppliedNetworkPolicy{{Name: "prior-policy", LastAppliedTime: metav1.Unix(10, 0)}},
			},
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
	if len(agent.Status.NetworkPolicy.LastAppliedPolicies) != 1 || agent.Status.NetworkPolicy.LastAppliedPolicies[0].Name != "prior-policy" {
		t.Fatalf("expected prior lastAppliedPolicies kept, got %+v", agent.Status.NetworkPolicy.LastAppliedPolicies)
	}
}

func TestReconcileNetworkPolicies_CreateFailureKeepsEarlierApplied(t *testing.T) {
	t.Setenv("OPERATOR_NAMESPACE", "zero-trust-workload-identity-manager")

	fc := &fakes.FakeCustomCtrlClient{}
	fc.GetReturns(kerrors.NewNotFound(schema.GroupResource{Resource: "networkpolicies"}, "np"))
	if len(spireAgentBaselineNetworkPolicyAssets) > 1 {
		fc.CreateReturnsOnCall(0, nil)
		fc.CreateReturnsOnCall(1, fmt.Errorf("quota exceeded"))
	} else {
		fc.CreateReturns(fmt.Errorf("quota exceeded"))
	}
	reconciler := newNetworkPolicyTestReconciler(t, fc)

	agent := &v1alpha1.SpireAgent{
		ObjectMeta: metav1.ObjectMeta{Name: "cluster", UID: "uid-1"},
		Spec:       v1alpha1.SpireAgentSpec{ManagedNetworkPolicy: "true"},
		Status: v1alpha1.SpireAgentStatus{
			NetworkPolicy: &v1alpha1.NetworkPolicyStatus{
				LastAppliedPolicies: []v1alpha1.AppliedNetworkPolicy{{Name: "prior-policy", LastAppliedTime: metav1.Unix(10, 0)}},
			},
		},
	}
	first := networkPolicyFromAsset(spireAgentBaselineNetworkPolicyAssets[0], nil).Name

	err := reconciler.reconcileNetworkPolicies(context.Background(), agent, false)
	if err == nil {
		t.Fatal("expected create failure")
	}
	got := agent.Status.NetworkPolicy
	if got == nil || got.ManagementState != v1alpha1.NetworkPolicyManagementManaged {
		t.Fatalf("expected Managed status with policies applied before the error, got %+v", got)
	}
	names := map[string]bool{}
	for _, policy := range got.LastAppliedPolicies {
		names[policy.Name] = true
	}
	if !names["prior-policy"] {
		t.Fatalf("expected prior policy kept, got %+v", got.LastAppliedPolicies)
	}
	if len(spireAgentBaselineNetworkPolicyAssets) > 1 {
		second := networkPolicyFromAsset(spireAgentBaselineNetworkPolicyAssets[1], nil).Name
		if !names[first] || names[second] {
			t.Fatalf("expected %s recorded and %s omitted, got %+v", first, second, got.LastAppliedPolicies)
		}
		return
	}
	if names[first] {
		t.Fatalf("failed policy must not be recorded, got %+v", got.LastAppliedPolicies)
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
}

func staleAgentIngressNetworkPolicy(agent *v1alpha1.SpireAgent) *networkingv1.NetworkPolicy {
	desired := networkPolicyFromAsset(spireAgentBaselineNetworkPolicyAssets[0], agent.Spec.Labels)
	stale := desired.DeepCopy()
	stale.ResourceVersion = "42"
	stale.CreationTimestamp = metav1.Unix(50, 0)
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
	if len(agent.Status.NetworkPolicy.LastAppliedPolicies) != 1 || agent.Status.NetworkPolicy.LastAppliedPolicies[0].Name != policyName || agent.Status.NetworkPolicy.LastAppliedPolicies[0].LastAppliedTime.IsZero() {
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
	got := agent.Status.NetworkPolicy.LastAppliedPolicies
	if len(got) != 1 || got[0].Name != stale.Name || !got[0].LastAppliedTime.Time.Equal(stale.CreationTimestamp.Time) {
		t.Fatalf("create-only existing policy must record creationTimestamp, got %+v", got)
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
				LastAppliedPolicies: []v1alpha1.AppliedNetworkPolicy{{Name: "prior-policy", LastAppliedTime: metav1.Unix(10, 0)}},
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
	names := map[string]metav1.Time{}
	for _, policy := range agent.Status.NetworkPolicy.LastAppliedPolicies {
		names[policy.Name] = policy.LastAppliedTime
	}
	if !names["prior-policy"].Time.Equal(metav1.Unix(10, 0).Time) {
		t.Fatalf("expected prior lastApplied preserved, got %+v", agent.Status.NetworkPolicy.LastAppliedPolicies)
	}
	if !names[stale.Name].Time.Equal(stale.CreationTimestamp.Time) {
		t.Fatalf("expected existing policy recorded with creationTimestamp, got %+v", agent.Status.NetworkPolicy.LastAppliedPolicies)
	}
}

func TestReconcileNetworkPolicies_UpToDateRecordsCreationTimestamp(t *testing.T) {
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
	existing.CreationTimestamp = metav1.Unix(77, 0)
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
	got := agent.Status.NetworkPolicy.LastAppliedPolicies
	if len(got) != 1 || got[0].Name != existing.Name || !got[0].LastAppliedTime.Time.Equal(existing.CreationTimestamp.Time) {
		t.Fatalf("up-to-date policy must record creationTimestamp, got %+v", got)
	}
}

func TestReconcile_UnmanagedNetworkPolicyPersistsWithoutReadyFailure(t *testing.T) {
	t.Setenv("OPERATOR_NAMESPACE", "zero-trust-workload-identity-manager")
	persisted := runAgentReconcileForNetworkPolicy(t, "false", nil)
	if persisted.Status.NetworkPolicy == nil || persisted.Status.NetworkPolicy.ManagementState != v1alpha1.NetworkPolicyManagementUnmanaged {
		t.Fatalf("expected persisted Unmanaged networkPolicy, got %+v", persisted.Status.NetworkPolicy)
	}
	ready := apimeta.FindStatusCondition(persisted.Status.Conditions, v1alpha1.Ready)
	if ready != nil && ready.Status == metav1.ConditionFalse && ready.Message != "" && containsNetworkPolicyFailure(ready.Message) {
		t.Fatalf("unmanaged mode must not fail Ready for NetworkPolicy, got %+v", ready)
	}
}

func TestReconcile_NetworkPolicyErrorSetsReadyFalse(t *testing.T) {
	t.Setenv("OPERATOR_NAMESPACE", "zero-trust-workload-identity-manager")
	npErr := fmt.Errorf("networkpolicy api down")
	persisted := runAgentReconcileForNetworkPolicy(t, "true", npErr)
	ready := apimeta.FindStatusCondition(persisted.Status.Conditions, v1alpha1.Ready)
	if ready == nil || ready.Status != metav1.ConditionFalse || ready.Reason != v1alpha1.ReasonFailed || !containsNetworkPolicyFailure(ready.Message) {
		t.Fatalf("expected Ready=False for NetworkPolicy error, got %+v", ready)
	}
}

func containsNetworkPolicyFailure(message string) bool {
	return strings.Contains(message, "NetworkPolicy reconcile failed")
}

func runAgentReconcileForNetworkPolicy(t *testing.T, managed string, npGetErr error) *v1alpha1.SpireAgent {
	t.Helper()
	fc := &fakes.FakeCustomCtrlClient{}
	scheme := runtime.NewScheme()
	for _, add := range []func(*runtime.Scheme) error{
		v1alpha1.AddToScheme,
		corev1.AddToScheme,
		appsv1.AddToScheme,
		rbacv1.AddToScheme,
		networkingv1.AddToScheme,
		admissionregistrationv1.AddToScheme,
		securityv1.AddToScheme,
	} {
		if err := add(scheme); err != nil {
			t.Fatalf("AddToScheme: %v", err)
		}
	}
	reconciler := &SpireAgentReconciler{
		ctrlClient:    fc,
		ctx:           context.Background(),
		log:           logr.Discard(),
		scheme:        scheme,
		eventRecorder: record.NewFakeRecorder(100),
	}

	agent := &v1alpha1.SpireAgent{
		TypeMeta: metav1.TypeMeta{APIVersion: v1alpha1.GroupVersion.String(), Kind: "SpireAgent"},
		ObjectMeta: metav1.ObjectMeta{
			Name: "cluster",
			UID:  "agent-uid",
			OwnerReferences: []metav1.OwnerReference{{
				APIVersion: v1alpha1.GroupVersion.String(),
				Kind:       "ZeroTrustWorkloadIdentityManager",
				Name:       "cluster",
				UID:        "ztwim-uid",
			}},
		},
		Spec: v1alpha1.SpireAgentSpec{ManagedNetworkPolicy: managed},
	}
	ztwim := &v1alpha1.ZeroTrustWorkloadIdentityManager{
		TypeMeta:   metav1.TypeMeta{APIVersion: v1alpha1.GroupVersion.String(), Kind: "ZeroTrustWorkloadIdentityManager"},
		ObjectMeta: metav1.ObjectMeta{Name: "cluster", UID: "ztwim-uid"},
		Spec:       v1alpha1.ZeroTrustWorkloadIdentityManagerSpec{TrustDomain: "example.org"},
	}

	fc.GetStub = func(_ context.Context, key client.ObjectKey, obj client.Object) error {
		switch typed := obj.(type) {
		case *v1alpha1.SpireAgent:
			*typed = *agent.DeepCopy()
			return nil
		case *v1alpha1.ZeroTrustWorkloadIdentityManager:
			*typed = *ztwim.DeepCopy()
			return nil
		case *networkingv1.NetworkPolicy:
			if npGetErr != nil {
				return npGetErr
			}
		}
		return kerrors.NewNotFound(schema.GroupResource{Resource: key.Name}, key.Name)
	}
	fc.CreateReturns(nil)
	fc.UpdateReturns(nil)

	var persisted *v1alpha1.SpireAgent
	fc.StatusUpdateWithRetryStub = func(_ context.Context, obj client.Object, _ ...client.SubResourceUpdateOption) error {
		if got, ok := obj.(*v1alpha1.SpireAgent); ok {
			persisted = got.DeepCopy()
		}
		return nil
	}

	_, err := reconciler.Reconcile(context.Background(), ctrl.Request{NamespacedName: types.NamespacedName{Name: "cluster"}})
	if npGetErr == nil && err != nil {
		t.Fatalf("Reconcile: %v", err)
	}
	if npGetErr != nil && err == nil {
		t.Fatal("expected NetworkPolicy reconcile error")
	}
	if persisted == nil {
		t.Fatal("expected status to be persisted")
	}
	return persisted
}
