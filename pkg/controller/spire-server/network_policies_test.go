package spire_server

import (
	"context"
	"testing"

	"github.com/go-logr/logr"
	"github.com/openshift/zero-trust-workload-identity-manager/api/v1alpha1"
	"github.com/openshift/zero-trust-workload-identity-manager/pkg/client/fakes"
	"github.com/openshift/zero-trust-workload-identity-manager/pkg/controller/status"
	"github.com/openshift/zero-trust-workload-identity-manager/pkg/controller/utils"
	networkingv1 "k8s.io/api/networking/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/tools/record"
)

func newNetworkPolicyTestReconciler(fakeClient *fakes.FakeCustomCtrlClient) *SpireServerReconciler {
	scheme := runtime.NewScheme()
	_ = v1alpha1.AddToScheme(scheme)
	_ = networkingv1.AddToScheme(scheme)
	return &SpireServerReconciler{
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

	server := &v1alpha1.SpireServer{
		ObjectMeta: metav1.ObjectMeta{Name: "cluster", UID: "uid-1"},
		Spec: v1alpha1.SpireServerSpec{
			CommonConfig: v1alpha1.CommonConfig{ManagedNetworkPolicy: "false"},
		},
	}

	if err := reconciler.reconcileNetworkPolicies(context.Background(), server, statusMgr, false); err != nil {
		t.Fatalf("reconcileNetworkPolicies: %v", err)
	}
	if fc.CreateCallCount() != 0 {
		t.Fatalf("expected no NetworkPolicy create, got %d", fc.CreateCallCount())
	}
	if server.Status.NetworkPolicy == nil || server.Status.NetworkPolicy.ManagementState != v1alpha1.NetworkPolicyManagementUnmanaged {
		t.Fatalf("expected unmanaged network policy status, got %+v", server.Status.NetworkPolicy)
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

	server := &v1alpha1.SpireServer{
		ObjectMeta: metav1.ObjectMeta{Name: "cluster", UID: "uid-1"},
		Spec:       v1alpha1.SpireServerSpec{CommonConfig: v1alpha1.CommonConfig{ManagedNetworkPolicy: "true"}},
	}

	if err := reconciler.reconcileNetworkPolicies(context.Background(), server, statusMgr, false); err != nil {
		t.Fatalf("reconcileNetworkPolicies: %v", err)
	}
	if fc.CreateCallCount() != len(spireServerBaselineNetworkPolicyAssets) {
		t.Fatalf("expected %d creates, got %d", len(spireServerBaselineNetworkPolicyAssets), fc.CreateCallCount())
	}
	if server.Status.NetworkPolicy == nil || len(server.Status.NetworkPolicy.AppliedPolicies) != len(spireServerBaselineNetworkPolicyAssets) {
		t.Fatalf("unexpected applied policies: %+v", server.Status.NetworkPolicy)
	}
}

func TestReconcileNetworkPolicies_FederationCreatesPolicies(t *testing.T) {
	t.Setenv("OPERATOR_NAMESPACE", "zero-trust-workload-identity-manager")

	fc := &fakes.FakeCustomCtrlClient{}
	fc.GetReturns(kerrors.NewNotFound(schema.GroupResource{Resource: "networkpolicies"}, "np"))
	fc.CreateReturns(nil)
	fc.StatusUpdateWithRetryReturns(nil)
	reconciler := newNetworkPolicyTestReconciler(fc)
	statusMgr := status.NewManager(fc)

	server := &v1alpha1.SpireServer{
		ObjectMeta: metav1.ObjectMeta{Name: "cluster", UID: "uid-1"},
		Spec: v1alpha1.SpireServerSpec{
			CommonConfig: v1alpha1.CommonConfig{ManagedNetworkPolicy: "true"},
			Federation:   &v1alpha1.FederationConfig{},
			Datastore:    v1alpha1.DataStore{DatabaseType: "sqlite3"},
		},
	}

	if err := reconciler.reconcileNetworkPolicies(context.Background(), server, statusMgr, false); err != nil {
		t.Fatalf("reconcileNetworkPolicies: %v", err)
	}
	expectedCreates := len(spireServerBaselineNetworkPolicyAssets) + len(spireServerFederationNetworkPolicyAssets)
	if fc.CreateCallCount() != expectedCreates {
		t.Fatalf("expected %d creates, got %d", expectedCreates, fc.CreateCallCount())
	}
}

func TestReconcileNetworkPolicies_PostgresUsesAllowAllBaselineOnly(t *testing.T) {
	t.Setenv("OPERATOR_NAMESPACE", "zero-trust-workload-identity-manager")

	fc := &fakes.FakeCustomCtrlClient{}
	fc.GetReturns(kerrors.NewNotFound(schema.GroupResource{Resource: "networkpolicies"}, "np"))
	fc.CreateReturns(nil)
	fc.StatusUpdateWithRetryReturns(nil)
	reconciler := newNetworkPolicyTestReconciler(fc)
	statusMgr := status.NewManager(fc)

	server := &v1alpha1.SpireServer{
		ObjectMeta: metav1.ObjectMeta{Name: "cluster", UID: "uid-1"},
		Spec: v1alpha1.SpireServerSpec{
			CommonConfig: v1alpha1.CommonConfig{ManagedNetworkPolicy: "true"},
			Datastore:    v1alpha1.DataStore{DatabaseType: "postgres"},
		},
	}

	if err := reconciler.reconcileNetworkPolicies(context.Background(), server, statusMgr, false); err != nil {
		t.Fatalf("reconcileNetworkPolicies: %v", err)
	}
	if fc.CreateCallCount() != len(spireServerBaselineNetworkPolicyAssets) {
		t.Fatalf("expected %d creates, got %d", len(spireServerBaselineNetworkPolicyAssets), fc.CreateCallCount())
	}
}

func TestSpireServerEgressPolicies_BaselineAndAllowAllAddon(t *testing.T) {
	t.Setenv("OPERATOR_NAMESPACE", "zero-trust-workload-identity-manager")
	baseline := networkPolicyFromAsset(utils.SpireServerNetworkPolicyEgressBaselineAssetName, nil)
	if utils.NetworkPolicyAllowsAllEgress(baseline) {
		t.Fatal("expected spire-server egress baseline to remain port-scoped")
	}
	if !utils.NetworkPolicyHasEgressTCPPort(baseline, 6443) {
		t.Fatal("expected spire-server egress baseline to allow Kubernetes API egress")
	}
	allowAll := networkPolicyFromAsset(utils.SpireServerNetworkPolicyEgressAllowAllAssetName, nil)
	if !utils.NetworkPolicyAllowsAllEgress(allowAll) {
		t.Fatal("expected spire-server egress allow-all add-on")
	}
}

func TestManagedNetworkPolicyEnabled(t *testing.T) {
	if !utils.ManagedNetworkPolicyEnabled("") {
		t.Fatal("empty should default to managed")
	}
	if !utils.ManagedNetworkPolicyEnabled("true") {
		t.Fatal("true should be managed")
	}
	if utils.ManagedNetworkPolicyEnabled("false") {
		t.Fatal("false should be unmanaged")
	}
}
