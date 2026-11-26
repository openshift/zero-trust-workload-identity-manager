package spire_oidc_discovery_provider

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/go-logr/logr"
	routev1 "github.com/openshift/api/route/v1"
	securityv1 "github.com/openshift/api/security/v1"
	"github.com/openshift/zero-trust-workload-identity-manager/api/v1alpha1"
	"github.com/openshift/zero-trust-workload-identity-manager/pkg/client/fakes"
	spiffev1alpha1 "github.com/spiffe/spire-controller-manager/api/v1alpha1"
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

func newNetworkPolicyTestReconciler(t *testing.T, fakeClient *fakes.FakeCustomCtrlClient) *SpireOidcDiscoveryProviderReconciler {
	t.Helper()
	scheme := runtime.NewScheme()
	if err := v1alpha1.AddToScheme(scheme); err != nil {
		t.Fatalf("AddToScheme v1alpha1: %v", err)
	}
	if err := networkingv1.AddToScheme(scheme); err != nil {
		t.Fatalf("AddToScheme networkingv1: %v", err)
	}
	return &SpireOidcDiscoveryProviderReconciler{
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

	oidc := &v1alpha1.SpireOIDCDiscoveryProvider{
		ObjectMeta: metav1.ObjectMeta{Name: "cluster", UID: "uid-1"},
		Spec: v1alpha1.SpireOIDCDiscoveryProviderSpec{
			ManagedNetworkPolicy: "false",
		},
		Status: v1alpha1.SpireOIDCDiscoveryProviderStatus{
			NetworkPolicy: &v1alpha1.NetworkPolicyStatus{
				ManagementState:     v1alpha1.NetworkPolicyManagementManaged,
				LastAppliedPolicies: []v1alpha1.AppliedNetworkPolicy{{Name: "prior-policy", LastAppliedTime: metav1.Unix(10, 0)}},
			},
		},
	}

	if err := reconciler.reconcileNetworkPolicies(context.Background(), oidc, false); err != nil {
		t.Fatalf("reconcileNetworkPolicies: %v", err)
	}
	if fc.CreateCallCount() != 0 {
		t.Fatalf("expected no NetworkPolicy create, got %d", fc.CreateCallCount())
	}
	if oidc.Status.NetworkPolicy == nil || oidc.Status.NetworkPolicy.ManagementState != v1alpha1.NetworkPolicyManagementUnmanaged {
		t.Fatalf("expected unmanaged network policy status, got %+v", oidc.Status.NetworkPolicy)
	}
	if len(oidc.Status.NetworkPolicy.LastAppliedPolicies) != 1 || oidc.Status.NetworkPolicy.LastAppliedPolicies[0].Name != "prior-policy" {
		t.Fatalf("expected prior lastAppliedPolicies kept, got %+v", oidc.Status.NetworkPolicy.LastAppliedPolicies)
	}
}

func TestReconcileNetworkPolicies_CreateFailureKeepsPriorApplied(t *testing.T) {
	t.Setenv("OPERATOR_NAMESPACE", "zero-trust-workload-identity-manager")

	fc := &fakes.FakeCustomCtrlClient{}
	fc.GetReturns(kerrors.NewNotFound(schema.GroupResource{Resource: "networkpolicies"}, "np"))
	fc.CreateReturns(fmt.Errorf("quota exceeded"))
	reconciler := newNetworkPolicyTestReconciler(t, fc)

	oidc := &v1alpha1.SpireOIDCDiscoveryProvider{
		ObjectMeta: metav1.ObjectMeta{Name: "cluster", UID: "uid-1"},
		Spec:       v1alpha1.SpireOIDCDiscoveryProviderSpec{ManagedNetworkPolicy: "true"},
		Status: v1alpha1.SpireOIDCDiscoveryProviderStatus{
			NetworkPolicy: &v1alpha1.NetworkPolicyStatus{
				LastAppliedPolicies: []v1alpha1.AppliedNetworkPolicy{{Name: "prior-policy", LastAppliedTime: metav1.Unix(10, 0)}},
			},
		},
	}
	if err := reconciler.reconcileNetworkPolicies(context.Background(), oidc, false); err == nil {
		t.Fatal("expected create failure")
	}
	got := oidc.Status.NetworkPolicy
	if got == nil || got.ManagementState != v1alpha1.NetworkPolicyManagementManaged || len(got.LastAppliedPolicies) != 1 || got.LastAppliedPolicies[0].Name != "prior-policy" {
		t.Fatalf("expected prior lastAppliedPolicies kept on failure, got %+v", got)
	}
}

func TestReconcileNetworkPolicies_CreatesBaselinePolicies(t *testing.T) {
	t.Setenv("OPERATOR_NAMESPACE", "zero-trust-workload-identity-manager")

	fc := &fakes.FakeCustomCtrlClient{}
	fc.GetReturns(kerrors.NewNotFound(schema.GroupResource{Resource: "networkpolicies"}, "np"))
	fc.CreateReturns(nil)
	reconciler := newNetworkPolicyTestReconciler(t, fc)

	oidc := &v1alpha1.SpireOIDCDiscoveryProvider{
		ObjectMeta: metav1.ObjectMeta{Name: "cluster", UID: "uid-1"},
		Spec:       v1alpha1.SpireOIDCDiscoveryProviderSpec{ManagedNetworkPolicy: "true"},
	}

	if err := reconciler.reconcileNetworkPolicies(context.Background(), oidc, false); err != nil {
		t.Fatalf("reconcileNetworkPolicies: %v", err)
	}
	if fc.CreateCallCount() != len(spireOIDCBaselineNetworkPolicyAssets) {
		t.Fatalf("expected %d creates, got %d", len(spireOIDCBaselineNetworkPolicyAssets), fc.CreateCallCount())
	}
	if oidc.Status.NetworkPolicy == nil || len(oidc.Status.NetworkPolicy.LastAppliedPolicies) != len(spireOIDCBaselineNetworkPolicyAssets) {
		t.Fatalf("unexpected applied policies: %+v", oidc.Status.NetworkPolicy)
	}
}

func staleOIDCIngressNetworkPolicy(oidc *v1alpha1.SpireOIDCDiscoveryProvider) *networkingv1.NetworkPolicy {
	desired := networkPolicyFromAsset(spireOIDCBaselineNetworkPolicyAssets[0], oidc.Spec.Labels)
	stale := desired.DeepCopy()
	stale.ResourceVersion = "42"
	stale.CreationTimestamp = metav1.Unix(50, 0)
	delete(stale.Labels, "app.kubernetes.io/managed-by")
	return stale
}

func TestReconcileNetworkPolicies_UpdatesStalePolicy(t *testing.T) {
	t.Setenv("OPERATOR_NAMESPACE", "zero-trust-workload-identity-manager")

	oidc := &v1alpha1.SpireOIDCDiscoveryProvider{
		ObjectMeta: metav1.ObjectMeta{Name: "cluster", UID: "uid-1"},
		Spec:       v1alpha1.SpireOIDCDiscoveryProviderSpec{ManagedNetworkPolicy: "true"},
	}
	stale := staleOIDCIngressNetworkPolicy(oidc)

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

	if err := reconciler.reconcileNetworkPolicies(context.Background(), oidc, false); err != nil {
		t.Fatalf("reconcileNetworkPolicies: %v", err)
	}
	if fc.UpdateCallCount() != 1 {
		t.Fatalf("expected 1 update, got %d", fc.UpdateCallCount())
	}
}

func TestReconcileNetworkPolicies_RepairsMissingOwnerReference(t *testing.T) {
	t.Setenv("OPERATOR_NAMESPACE", "zero-trust-workload-identity-manager")

	oidc := &v1alpha1.SpireOIDCDiscoveryProvider{
		ObjectMeta: metav1.ObjectMeta{Name: "cluster", UID: "uid-oidc"},
		Spec:       v1alpha1.SpireOIDCDiscoveryProviderSpec{ManagedNetworkPolicy: "true"},
	}
	desired := networkPolicyFromAsset(spireOIDCBaselineNetworkPolicyAssets[0], oidc.Spec.Labels)
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
	if err := reconciler.reconcileNetworkPolicies(context.Background(), oidc, false); err != nil {
		t.Fatalf("reconcileNetworkPolicies: %v", err)
	}
	if fc.UpdateCallCount() != 1 {
		t.Fatalf("expected 1 update to repair owner reference, got %d", fc.UpdateCallCount())
	}
}

func TestReconcileNetworkPolicies_CreateOnlySkipsDrift(t *testing.T) {
	t.Setenv("OPERATOR_NAMESPACE", "zero-trust-workload-identity-manager")

	oidc := &v1alpha1.SpireOIDCDiscoveryProvider{
		ObjectMeta: metav1.ObjectMeta{Name: "cluster", UID: "uid-1"},
		Spec:       v1alpha1.SpireOIDCDiscoveryProviderSpec{ManagedNetworkPolicy: "true"},
	}
	stale := staleOIDCIngressNetworkPolicy(oidc)

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

	if err := reconciler.reconcileNetworkPolicies(context.Background(), oidc, true); err != nil {
		t.Fatalf("reconcileNetworkPolicies: %v", err)
	}
	if fc.UpdateCallCount() != 0 {
		t.Fatalf("expected no update in create-only mode, got %d", fc.UpdateCallCount())
	}
	got := oidc.Status.NetworkPolicy.LastAppliedPolicies
	if len(got) != 1 || got[0].Name != stale.Name || !got[0].LastAppliedTime.Time.Equal(stale.CreationTimestamp.Time) {
		t.Fatalf("create-only existing policy must record creationTimestamp, got %+v", got)
	}
}

func TestReconcileNetworkPolicies_CreateOnlyPreservesPriorLastApplied(t *testing.T) {
	t.Setenv("OPERATOR_NAMESPACE", "zero-trust-workload-identity-manager")

	oidc := &v1alpha1.SpireOIDCDiscoveryProvider{
		ObjectMeta: metav1.ObjectMeta{Name: "cluster", UID: "uid-1"},
		Spec:       v1alpha1.SpireOIDCDiscoveryProviderSpec{ManagedNetworkPolicy: "true"},
		Status: v1alpha1.SpireOIDCDiscoveryProviderStatus{
			NetworkPolicy: &v1alpha1.NetworkPolicyStatus{
				ManagementState:     v1alpha1.NetworkPolicyManagementManaged,
				LastAppliedPolicies: []v1alpha1.AppliedNetworkPolicy{{Name: "prior-policy", LastAppliedTime: metav1.Unix(10, 0)}},
			},
		},
	}
	stale := staleOIDCIngressNetworkPolicy(oidc)

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
	if err := reconciler.reconcileNetworkPolicies(context.Background(), oidc, true); err != nil {
		t.Fatalf("reconcileNetworkPolicies: %v", err)
	}
	names := map[string]metav1.Time{}
	for _, policy := range oidc.Status.NetworkPolicy.LastAppliedPolicies {
		names[policy.Name] = policy.LastAppliedTime
	}
	if !names["prior-policy"].Time.Equal(metav1.Unix(10, 0).Time) {
		t.Fatalf("expected prior lastApplied preserved, got %+v", oidc.Status.NetworkPolicy.LastAppliedPolicies)
	}
	if !names[stale.Name].Time.Equal(stale.CreationTimestamp.Time) {
		t.Fatalf("expected existing policy recorded with creationTimestamp, got %+v", oidc.Status.NetworkPolicy.LastAppliedPolicies)
	}
}

func TestReconcileNetworkPolicies_UpToDateRecordsCreationTimestamp(t *testing.T) {
	t.Setenv("OPERATOR_NAMESPACE", "zero-trust-workload-identity-manager")

	oidc := &v1alpha1.SpireOIDCDiscoveryProvider{
		TypeMeta:   metav1.TypeMeta{APIVersion: v1alpha1.GroupVersion.String(), Kind: "SpireOIDCDiscoveryProvider"},
		ObjectMeta: metav1.ObjectMeta{Name: "cluster", UID: "uid-1"},
		Spec:       v1alpha1.SpireOIDCDiscoveryProviderSpec{ManagedNetworkPolicy: "true"},
	}
	reconciler := newNetworkPolicyTestReconciler(t, &fakes.FakeCustomCtrlClient{})
	desired := networkPolicyFromAsset(spireOIDCBaselineNetworkPolicyAssets[0], oidc.Spec.Labels)
	existing := desired.DeepCopy()
	existing.ResourceVersion = "1"
	existing.CreationTimestamp = metav1.Unix(77, 0)
	if err := controllerutil.SetControllerReference(oidc, existing, reconciler.scheme); err != nil {
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

	if err := reconciler.reconcileNetworkPolicies(context.Background(), oidc, false); err != nil {
		t.Fatalf("reconcileNetworkPolicies: %v", err)
	}
	if fc.UpdateCallCount() != 0 {
		t.Fatalf("expected no update, got %d", fc.UpdateCallCount())
	}
	got := oidc.Status.NetworkPolicy.LastAppliedPolicies
	if len(got) != 1 || got[0].Name != existing.Name || !got[0].LastAppliedTime.Time.Equal(existing.CreationTimestamp.Time) {
		t.Fatalf("up-to-date policy must record creationTimestamp, got %+v", got)
	}
}

func TestReconcile_UnmanagedNetworkPolicyPersistsWithoutReadyFailure(t *testing.T) {
	t.Setenv("OPERATOR_NAMESPACE", "zero-trust-workload-identity-manager")
	persisted := runOIDCReconcileForNetworkPolicy(t, "false", nil)
	if persisted.Status.NetworkPolicy == nil || persisted.Status.NetworkPolicy.ManagementState != v1alpha1.NetworkPolicyManagementUnmanaged {
		t.Fatalf("expected persisted Unmanaged networkPolicy, got %+v", persisted.Status.NetworkPolicy)
	}
	ready := apimeta.FindStatusCondition(persisted.Status.Conditions, v1alpha1.Ready)
	if ready != nil && ready.Status == metav1.ConditionFalse && strings.Contains(ready.Message, "NetworkPolicy reconcile failed") {
		t.Fatalf("unmanaged mode must not fail Ready for NetworkPolicy, got %+v", ready)
	}
}

func TestReconcile_NetworkPolicyErrorSetsReadyFalse(t *testing.T) {
	t.Setenv("OPERATOR_NAMESPACE", "zero-trust-workload-identity-manager")
	persisted := runOIDCReconcileForNetworkPolicy(t, "true", fmt.Errorf("networkpolicy api down"))
	ready := apimeta.FindStatusCondition(persisted.Status.Conditions, v1alpha1.Ready)
	if ready == nil || ready.Status != metav1.ConditionFalse || ready.Reason != v1alpha1.ReasonFailed || !strings.Contains(ready.Message, "NetworkPolicy reconcile failed") {
		t.Fatalf("expected Ready=False for NetworkPolicy error, got %+v", ready)
	}
}

func runOIDCReconcileForNetworkPolicy(t *testing.T, managed string, npGetErr error) *v1alpha1.SpireOIDCDiscoveryProvider {
	t.Helper()
	fc := &fakes.FakeCustomCtrlClient{}
	scheme := runtime.NewScheme()
	for _, add := range []func(*runtime.Scheme) error{
		v1alpha1.AddToScheme,
		corev1.AddToScheme,
		appsv1.AddToScheme,
		rbacv1.AddToScheme,
		networkingv1.AddToScheme,
		securityv1.AddToScheme,
		routev1.AddToScheme,
		spiffev1alpha1.AddToScheme,
	} {
		if err := add(scheme); err != nil {
			t.Fatalf("AddToScheme: %v", err)
		}
	}
	reconciler := &SpireOidcDiscoveryProviderReconciler{
		ctrlClient:    fc,
		ctx:           context.Background(),
		log:           logr.Discard(),
		scheme:        scheme,
		eventRecorder: record.NewFakeRecorder(100),
	}
	oidc := &v1alpha1.SpireOIDCDiscoveryProvider{
		TypeMeta: metav1.TypeMeta{APIVersion: v1alpha1.GroupVersion.String(), Kind: "SpireOIDCDiscoveryProvider"},
		ObjectMeta: metav1.ObjectMeta{
			Name: "cluster",
			UID:  "oidc-uid",
			OwnerReferences: []metav1.OwnerReference{{
				APIVersion: v1alpha1.GroupVersion.String(),
				Kind:       "ZeroTrustWorkloadIdentityManager",
				Name:       "cluster",
				UID:        "ztwim-uid",
			}},
		},
		Spec: v1alpha1.SpireOIDCDiscoveryProviderSpec{
			ManagedNetworkPolicy: managed,
			JwtIssuer:            "https://oidc.example.org",
		},
	}
	ztwim := &v1alpha1.ZeroTrustWorkloadIdentityManager{
		TypeMeta:   metav1.TypeMeta{APIVersion: v1alpha1.GroupVersion.String(), Kind: "ZeroTrustWorkloadIdentityManager"},
		ObjectMeta: metav1.ObjectMeta{Name: "cluster", UID: "ztwim-uid"},
		Spec:       v1alpha1.ZeroTrustWorkloadIdentityManagerSpec{TrustDomain: "example.org"},
	}
	fc.GetStub = func(_ context.Context, key client.ObjectKey, obj client.Object) error {
		switch typed := obj.(type) {
		case *v1alpha1.SpireOIDCDiscoveryProvider:
			*typed = *oidc.DeepCopy()
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
	var persisted *v1alpha1.SpireOIDCDiscoveryProvider
	fc.StatusUpdateWithRetryStub = func(_ context.Context, obj client.Object, _ ...client.SubResourceUpdateOption) error {
		if got, ok := obj.(*v1alpha1.SpireOIDCDiscoveryProvider); ok {
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
