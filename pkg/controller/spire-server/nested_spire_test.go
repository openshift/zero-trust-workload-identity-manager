package spire_server

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	spiffev1alpha1 "github.com/spiffe/spire-controller-manager/api/v1alpha1"

	"github.com/openshift/zero-trust-workload-identity-manager/api/v1alpha1"
	"github.com/openshift/zero-trust-workload-identity-manager/pkg/client/fakes"
	"github.com/openshift/zero-trust-workload-identity-manager/pkg/controller/status"
	"github.com/openshift/zero-trust-workload-identity-manager/pkg/controller/utils"
)

const (
	spireServerResourceName        = "cluster"
	nestedTestLocalOnly            = "local only"
	nestedTestList                 = "list"
	nestedTestReadError            = "read error"
	nestedTestKubeConfigMissing    = "KubeConfigMissing"
	nestedTestKubeConfigInvalid    = "KubeConfigInvalid"
	nestedTestLocalCollision       = "local collision"
	nestedTestToken                = "synthetic-token"
	nestedTestTemporary            = "temporary"
	nestedTestEntryReconcileFailed = "EntryReconcileFailed"
	nestedTestReadyCreateOnly      = "ready create only"
	nestedTestNamespace            = "test-operator"
	nestedTestAccessMode           = "ReadWriteOnce"
	nestedTestBundle               = "spire-bundle"
	nestedTestOther                = "other"
	nestedTestDelete               = "delete"
	nestedTestReady                = "Ready"
	nestedTestVolumeSize           = "1Gi"
	nestedTestTrustDomain          = "example.org"
	nestedTestSecretName           = "nested-spire-east-kubeconfig"
	nestedTestMissing              = "missing"
	nestedTestSecondCluster        = "west"
	nestedTestEntryResource        = "clusterstaticentries"
	nestedTestCreate               = "create"
	nestedTestGet                  = "get"
	nestedTestUpdate               = "update"
	nestedTestConflict             = "conflict"
	nestedTestCreateOnly           = "create only"
	nestedTestForeign              = "foreign"
	nestedTestNoOwner              = "missing owner"
	nestedTestDrift                = "drift"
	nestedTestPreserve             = "preserve"
	nestedTestGone                 = "already gone"
	nestedTestObjects              = "test-objects"
	nestedTestEmpty                = "empty"
	nestedTestReplacement          = "replacement"
)

func nestedFixture() (*v1alpha1.SpireServer, *v1alpha1.ZeroTrustWorkloadIdentityManager) {
	return &v1alpha1.SpireServer{ObjectMeta: metav1.ObjectMeta{Name: spireServerResourceName, UID: "server-uid"}, Spec: v1alpha1.SpireServerSpec{
		Persistence:  v1alpha1.Persistence{Size: nestedTestVolumeSize, AccessMode: nestedTestAccessMode},
		NodeAttestor: &v1alpha1.NodeAttestorConfig{ExternalK8sPSAT: &v1alpha1.ExternalK8sPSATConfig{Clusters: []v1alpha1.DownstreamCluster{{Name: "east"}}}},
	}}, &v1alpha1.ZeroTrustWorkloadIdentityManager{Spec: v1alpha1.ZeroTrustWorkloadIdentityManagerSpec{ClusterName: "local", TrustDomain: nestedTestTrustDomain, BundleConfigMap: nestedTestBundle}}
}

func nestedKubeconfig(_ *testing.T, token string) []byte {
	return []byte(fmt.Sprintf(`{"kind":"Config","apiVersion":"v1","current-context":"remote","contexts":[{"name":"remote","context":{"cluster":"remote","user":"remote"}}],"clusters":[{"name":"remote","cluster":{"server":"https://remote.example.org"}}],"users":[{"name":"remote","user":{"token":%q}}]}`, token))
}

func nestedReconciler(t *testing.T, objects ...client.Object) (*SpireServerReconciler, *fakes.FakeCustomCtrlClient, *nestedStore) {
	t.Helper()
	t.Setenv("OPERATOR_NAMESPACE", nestedTestNamespace)
	fc := &fakes.FakeCustomCtrlClient{}
	r := newTestReconciler(fc)
	for _, add := range []func(*runtime.Scheme) error{v1alpha1.AddToScheme, corev1.AddToScheme, spiffev1alpha1.AddToScheme} {
		if err := add(r.scheme); err != nil {
			t.Fatal(err)
		}
	}
	backing := &nestedStore{objects: map[string]client.Object{}}
	for _, obj := range objects {
		if err := backing.Create(context.Background(), obj.DeepCopyObject().(client.Object)); err != nil {
			t.Fatal(err)
		}
	}

	fc.GetStub = func(ctx context.Context, key client.ObjectKey, obj client.Object) error {
		return backing.Get(ctx, key, obj)
	}
	fc.CreateStub = backing.Create
	fc.UpdateStub = func(ctx context.Context, obj client.Object, opts ...client.UpdateOption) error {
		if entry, ok := obj.(*spiffev1alpha1.ClusterStaticEntry); ok {
			current := &spiffev1alpha1.ClusterStaticEntry{}
			if err := backing.Get(ctx, client.ObjectKeyFromObject(entry), current); err != nil {
				return err
			}
			// Mimic the API-managed UID and separate status subresource on updates.
			entry.UID = current.UID
			entry.Status = current.Status
		}
		return backing.Update(ctx, obj, opts...)
	}
	fc.ListStub = backing.List
	fc.DeleteStub = backing.Delete
	return r, fc, backing
}

func TestReconcileDownstreamClustersDisabled(t *testing.T) {
	for _, mode := range []string{"no node attestor", "no external attestor", "empty clusters", nestedTestCreateOnly, nestedTestList} {
		t.Run(mode, func(t *testing.T) {
			ctx := context.Background()
			server, parent := nestedFixture()
			r, fc, backing := nestedReconciler(t)
			entries := downstreamEntries(server, parent)
			for _, entry := range entries {
				if err := controllerutil.SetControllerReference(server, entry, r.scheme); err != nil {
					t.Fatal(err)
				}
				if err := backing.Create(ctx, entry); err != nil {
					t.Fatal(err)
				}
			}
			switch mode {
			case "no external attestor":
				server.Spec.NodeAttestor.ExternalK8sPSAT = nil
			case "empty clusters":
				server.Spec.NodeAttestor.ExternalK8sPSAT.Clusters = nil
			default:
				server.Spec.NodeAttestor = nil
			}
			server.Status.Conditions = []metav1.Condition{{
				Type: DownstreamClustersValid, Status: metav1.ConditionFalse,
				Reason: nestedTestKubeConfigMissing, Message: "previous failure",
			}}
			expected := kerrors.NewServiceUnavailable(nestedTestTemporary)
			if mode == nestedTestList {
				fc.ListReturns(expected)
			}
			mgr := status.NewManager(fc)
			effective, issues, err := r.reconcileDownstreamClusters(ctx, server, parent, mgr, mode == nestedTestCreateOnly)
			if mode == nestedTestList {
				if !errors.Is(err, expected) {
					t.Fatalf("expected cleanup error, got %v", err)
				}
			} else if err != nil || effective != server || len(issues) != 0 {
				t.Fatalf("unexpected disabled-feature result: effective=%p, issues=%v, err=%v", effective, issues, err)
			}
			if fc.GetCallCount() != 0 {
				t.Fatal("disabled downstream configuration must not read credential Secrets or reconcile entries")
			}
			if err := mgr.ApplyStatus(ctx, server, func() *v1alpha1.ConditionalStatus {
				return &server.Status.ConditionalStatus
			}); err != nil {
				t.Fatal(err)
			}
			if fc.StatusUpdateWithRetryCallCount() != 1 {
				t.Fatal("expected removal to be persisted")
			}
			_, updated, _ := fc.StatusUpdateWithRetryArgsForCall(0)
			if condition := apimeta.FindStatusCondition(updated.(*v1alpha1.SpireServer).Status.Conditions, DownstreamClustersValid); condition != nil {
				t.Fatalf("disabled feature retained its condition: %+v", condition)
			}
			for _, entry := range entries {
				err := backing.Get(ctx, client.ObjectKeyFromObject(entry), &spiffev1alpha1.ClusterStaticEntry{})
				if mode == nestedTestCreateOnly || mode == nestedTestList {
					if err != nil {
						t.Fatalf("entry should remain: %v", err)
					}
				} else if !kerrors.IsNotFound(err) {
					t.Fatalf("obsolete entry was not deleted: %v", err)
				}
			}
		})
	}
}

func TestResolveDownstreamClusters(t *testing.T) {
	cases := []struct {
		name     string
		change   func(*v1alpha1.SpireServer, *corev1.Secret)
		reason   string
		valid    int
		apiError bool
	}{
		{name: "valid", valid: 1},
		{name: nestedTestMissing, reason: nestedTestKubeConfigMissing, change: func(_ *v1alpha1.SpireServer, s *corev1.Secret) { s.Name = nestedTestOther }},
		{name: "missing key", reason: nestedTestKubeConfigInvalid, change: func(_ *v1alpha1.SpireServer, s *corev1.Secret) { s.Data = nil }},
		{name: "empty key", reason: nestedTestKubeConfigInvalid, change: func(_ *v1alpha1.SpireServer, s *corev1.Secret) {
			s.Data[downstreamKubeConfigKey] = nil
		}},
		{name: nestedTestLocalCollision, reason: "ClusterNameConflict", change: func(s *v1alpha1.SpireServer, _ *corev1.Secret) {
			s.Spec.NodeAttestor.ExternalK8sPSAT.Clusters[0].Name = "local"
		}},
		{name: "duplicate", reason: "InvalidDownstreamCluster", change: func(s *v1alpha1.SpireServer, _ *corev1.Secret) {
			s.Spec.NodeAttestor.ExternalK8sPSAT.Clusters = append(s.Spec.NodeAttestor.ExternalK8sPSAT.Clusters, s.Spec.NodeAttestor.ExternalK8sPSAT.Clusters[0])
		}},
		{name: "nonempty malformed data is delegated to SPIRE", valid: 1, change: func(_ *v1alpha1.SpireServer, s *corev1.Secret) {
			s.Data[downstreamKubeConfigKey] = []byte("not a kubeconfig")
		}},
		{name: "empty node attestor", change: func(s *v1alpha1.SpireServer, _ *corev1.Secret) { s.Spec.NodeAttestor = &v1alpha1.NodeAttestorConfig{} }},
		{name: "empty external attestor", change: func(s *v1alpha1.SpireServer, _ *corev1.Secret) {
			s.Spec.NodeAttestor.ExternalK8sPSAT = &v1alpha1.ExternalK8sPSATConfig{}
		}},
		{name: "one invalid of two", reason: nestedTestKubeConfigMissing, valid: 1, change: func(s *v1alpha1.SpireServer, _ *corev1.Secret) {
			s.Spec.NodeAttestor.ExternalK8sPSAT.Clusters = append(s.Spec.NodeAttestor.ExternalK8sPSAT.Clusters, v1alpha1.DownstreamCluster{Name: nestedTestSecondCluster})
		}},
		{name: "API failure", apiError: true},
		{name: "none", change: func(s *v1alpha1.SpireServer, _ *corev1.Secret) { s.Spec.NodeAttestor = nil }},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			server, parent := nestedFixture()
			secret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: nestedTestSecretName, Namespace: nestedTestNamespace}, Data: map[string][]byte{downstreamKubeConfigKey: nestedKubeconfig(t, nestedTestToken)}}
			if tt.change != nil {
				tt.change(server, secret)
			}
			before := server.DeepCopy()
			r, fc, _ := nestedReconciler(t, secret)
			if tt.apiError {
				fc.GetReturns(kerrors.NewServiceUnavailable("unavailable"))
			}
			effective, issues, err := r.resolveDownstreamClusters(context.Background(), server, parent)
			if tt.apiError {
				if err == nil {
					t.Fatal("expected retryable error")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got := len(sortedDownstreamClusters(&effective.Spec)); got != tt.valid {
				t.Fatalf("effective count %d, want %d", got, tt.valid)
			}
			if tt.reason != "" {
				if len(issues) == 0 || issues[0].reason != tt.reason {
					t.Fatalf("unexpected issues: %+v", issues)
				}
			} else if len(issues) != 0 {
				t.Fatalf("unexpected issues: %+v", issues)
			}
			if strings.Contains(downstreamIssuesMessage(issues), "synthetic") {
				t.Fatal("credential leaked")
			}
			if !reflect.DeepEqual(before, server) {
				t.Fatal("persisted spec mutated")
			}
		})
	}
}

func TestDownstreamConfigurationLifecycle(t *testing.T) {
	ctx := context.Background()
	server, parent := nestedFixture()
	secret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: nestedTestSecretName, Namespace: nestedTestNamespace}, Data: map[string][]byte{downstreamKubeConfigKey: nestedKubeconfig(t, "old-synthetic")}}
	r, _, backing := nestedReconciler(t, secret)
	effective, issues, err := r.resolveDownstreamClusters(ctx, server, parent)
	if err != nil || len(issues) != 0 {
		t.Fatalf("resolution: %v %+v", err, issues)
	}
	firstCM, err := generateSpireServerConfigMap(&effective.Spec, parent, nil)
	if err != nil {
		t.Fatal(err)
	}
	firstHash := generateConfigHashFromString(firstCM.Data[utils.SpireServerConfigKey])
	firstPod := GenerateSpireServerStatefulSet(&effective.Spec, firstHash, "scm").Spec.Template
	if err := backing.Get(ctx, client.ObjectKeyFromObject(secret), secret); err != nil {
		t.Fatal(err)
	}
	secret.Data[downstreamKubeConfigKey] = nestedKubeconfig(t, "new-synthetic")
	if err := backing.Update(ctx, secret); err != nil {
		t.Fatal(err)
	}
	rotated, issues, err := r.resolveDownstreamClusters(ctx, server, parent)
	if err != nil || len(issues) != 0 {
		t.Fatalf("rotation: %v %+v", err, issues)
	}
	rotatedCM, err := generateSpireServerConfigMap(&rotated.Spec, parent, nil)
	if err != nil {
		t.Fatal(err)
	}
	rotatedHash := generateConfigHashFromString(rotatedCM.Data[utils.SpireServerConfigKey])
	rotatedPod := GenerateSpireServerStatefulSet(&rotated.Spec, rotatedHash, "scm").Spec.Template
	if !reflect.DeepEqual(firstCM.Data, rotatedCM.Data) || !reflect.DeepEqual(firstPod, rotatedPod) {
		t.Fatal("valid credential rotation changed config or pod template")
	}
	effective.Spec.NodeAttestor.ExternalK8sPSAT.Clusters[0].Name = nestedTestReplacement
	changedCM, err := generateSpireServerConfigMap(&effective.Spec, parent, nil)
	if err != nil {
		t.Fatal(err)
	}
	changedHash := generateConfigHashFromString(changedCM.Data[utils.SpireServerConfigKey])
	changedPod := GenerateSpireServerStatefulSet(&effective.Spec, changedHash, "scm").Spec.Template
	if changedPod.Annotations[spireServerStatefulSetSpireServerConfigHashAnnotationKey] == firstPod.Annotations[spireServerStatefulSetSpireServerConfigHashAnnotationKey] {
		t.Fatal("config hash not propagated")
	}
}

func TestDownstreamEntryContract(t *testing.T) {
	t.Setenv("OPERATOR_NAMESPACE", nestedTestNamespace)
	server, parent := nestedFixture()
	server.Spec.DefaultX509Validity = metav1.Duration{Duration: 30 * time.Minute}
	entries := downstreamEntries(server, parent)
	if len(entries) != 2 {
		t.Fatal("expected two entries")
	}
	alias, downstream := entries[0], entries[1]
	if alias.Name != "nested-spire-east-node-alias" || alias.Spec.SPIFFEID != "spiffe://example.org/downstream/east" || alias.Spec.ParentID != "spiffe://example.org/spire/server" {
		t.Fatalf("unexpected alias: %+v", alias)
	}
	want := []string{"k8s_psat:cluster:east", "k8s_psat:agent_ns:test-operator", "k8s_psat:agent_sa:spire-server", "k8s_psat:agent_pod_name:spire-server-0"}
	if !reflect.DeepEqual(alias.Spec.Selectors, want) || alias.Spec.Downstream {
		t.Fatal("incorrect alias authorization")
	}
	if downstream.Spec.ParentID != alias.Spec.SPIFFEID || downstream.Spec.SPIFFEID != alias.Spec.SPIFFEID+"/spire-server" || !downstream.Spec.Downstream || !reflect.DeepEqual(downstream.Spec.Selectors, []string{"unix:path:/spire-server"}) {
		t.Fatalf("incorrect downstream authorization: %+v", downstream.Spec)
	}
	for _, entry := range entries {
		if entry.Spec.X509SVIDTTL.Duration != 0 {
			t.Fatalf("entry %s must inherit the server's X.509-SVID TTL", entry.Name)
		}
		if entry.Spec.ClassName != downstreamClassName || entry.Labels[utils.AppComponentLabelKey] != utils.ComponentControlPlane {
			t.Fatal("incorrect class or labels")
		}
	}
}

func TestReconcileDownstreamEntry(t *testing.T) {
	boom := errors.New("API unavailable")
	conflict := kerrors.NewConflict(schema.GroupResource{Group: "spire.spiffe.io", Resource: nestedTestEntryResource}, "entry", errors.New("stale version"))
	cases := []struct {
		name             string
		existing         bool
		drift            bool
		createOnly       bool
		failure          string
		change           func(*spiffev1alpha1.ClusterStaticEntry)
		wantErr          bool
		wantErrText      string
		wantReason       string
		creates, updates int
	}{
		{name: nestedTestCreate, creates: 1},
		{name: "create failure", failure: nestedTestCreate, wantErr: true, creates: 1},
		{name: "hidden unmanaged collision", failure: "exists", wantErr: true, creates: 1},
		{name: "get failure", failure: nestedTestGet, wantErr: true},
		{name: nestedTestUpdate, existing: true, drift: true, updates: 1},
		{
			name: "explicit TTL removed", existing: true, updates: 1,
			change: func(entry *spiffev1alpha1.ClusterStaticEntry) {
				entry.Spec.X509SVIDTTL = metav1.Duration{Duration: time.Hour}
			},
		},
		{name: "update failure", existing: true, drift: true, failure: nestedTestUpdate, wantErr: true, updates: 1},
		{name: "409", existing: true, drift: true, failure: nestedTestConflict, wantErr: true, updates: 1},
		{name: "no op", existing: true},
		{name: nestedTestCreateOnly, existing: true, drift: true, createOnly: true, wantReason: utils.CreateOnlyModeStatusType},
		{name: "missing in create only", createOnly: true, creates: 1},
		{name: "owner generation failure", failure: "scheme", wantErr: true},
		{
			name: "foreign owner", existing: true, wantErr: true,
			wantErrText: "is controlled by another resource",
			change: func(entry *spiffev1alpha1.ClusterStaticEntry) {
				entry.OwnerReferences[0].UID = nestedTestOther
			},
		},
		{
			name: "foreign owner with spec drift", existing: true, drift: true, wantErr: true,
			wantErrText: "is controlled by another resource",
			change: func(entry *spiffev1alpha1.ClusterStaticEntry) {
				entry.OwnerReferences[0].UID = nestedTestOther
			},
		},
		{
			name: "foreign owner in create only", existing: true, drift: true, createOnly: true, wantErr: true,
			wantErrText: "is controlled by another resource",
			change: func(entry *spiffev1alpha1.ClusterStaticEntry) {
				entry.OwnerReferences[0].UID = nestedTestOther
			},
		},
		{
			name: "foreign owner with label drift", existing: true, wantErr: true,
			wantErrText: "is controlled by another resource",
			change: func(entry *spiffev1alpha1.ClusterStaticEntry) {
				entry.OwnerReferences[0].UID = nestedTestOther
				delete(entry.Labels, utils.AppComponentLabelKey)
			},
		},
		{
			name: nestedTestNoOwner, existing: true, updates: 1,
			change: func(entry *spiffev1alpha1.ClusterStaticEntry) {
				entry.OwnerReferences = nil
			},
		},
		{
			name: "missing owner with spec drift", existing: true, drift: true, updates: 1,
			change: func(entry *spiffev1alpha1.ClusterStaticEntry) {
				entry.OwnerReferences = nil
			},
		},
		{
			name: "missing owner in create only", existing: true, createOnly: true,
			wantReason: utils.CreateOnlyModeStatusType,
			change: func(entry *spiffev1alpha1.ClusterStaticEntry) {
				entry.OwnerReferences = nil
			},
		},
		{
			name: "missing owner with spec drift in create only", existing: true, drift: true, createOnly: true, wantReason: utils.CreateOnlyModeStatusType,
			change: func(entry *spiffev1alpha1.ClusterStaticEntry) {
				entry.OwnerReferences = nil
			},
		},
		{
			name: "missing owner with label drift", existing: true, updates: 1,
			change: func(entry *spiffev1alpha1.ClusterStaticEntry) {
				entry.OwnerReferences = nil
				delete(entry.Labels, utils.AppComponentLabelKey)
			},
		},
		{
			name: "missing component label", existing: true, updates: 1,
			change: func(entry *spiffev1alpha1.ClusterStaticEntry) {
				delete(entry.Labels, utils.AppComponentLabelKey)
			},
		},
		{name: "terminating", existing: true, wantErr: true, change: func(entry *spiffev1alpha1.ClusterStaticEntry) {
			now := metav1.Now()
			entry.DeletionTimestamp = &now
		}},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			server, parent := nestedFixture()
			r, fc, backing := nestedReconciler(t)
			desired := downstreamEntries(server, parent)[0]
			want := desired.DeepCopy()
			if tt.existing {
				existing := desired.DeepCopy()
				if err := controllerutil.SetControllerReference(server, existing, r.scheme); err != nil {
					t.Fatal(err)
				}
				if tt.drift {
					existing.Spec.Hint = nestedTestDrift
				}
				if tt.change != nil {
					tt.change(existing)
				}
				existing.Annotations = map[string]string{"user-note": nestedTestPreserve}
				if err := backing.Create(context.Background(), existing); err != nil {
					t.Fatal(err)
				}
			}
			switch tt.failure {
			case nestedTestCreate:
				fc.CreateReturns(boom)
			case "exists":
				fc.CreateReturns(kerrors.NewAlreadyExists(schema.GroupResource{Resource: nestedTestEntryResource}, desired.Name))
			case nestedTestGet:
				fc.GetReturns(boom)
			case nestedTestUpdate:
				fc.UpdateReturns(boom)
			case nestedTestConflict:
				fc.UpdateReturns(conflict)
			case "scheme":
				r.scheme = runtime.NewScheme()
			}
			issue, err := r.reconcileDownstreamEntry(context.Background(), server, desired, status.NewManager(fc), tt.createOnly)
			if (err != nil) != tt.wantErr {
				t.Fatalf("error=%v", err)
			}
			if tt.wantErrText != "" && (err == nil || !strings.Contains(err.Error(), tt.wantErrText)) {
				t.Fatalf("error=%v, want %q", err, tt.wantErrText)
			}
			if tt.wantReason == "" {
				if issue != nil {
					t.Fatalf("unexpected reconciliation issue: %+v", issue)
				}
			} else if issue == nil || issue.reason != tt.wantReason {
				t.Fatalf("issue=%+v, want %s", issue, tt.wantReason)
			}
			if tt.failure != "scheme" && fc.GetCallCount() != 1 {
				t.Fatalf("expected one entry read, got %d", fc.GetCallCount())
			}
			if tt.failure == nestedTestConflict && !kerrors.IsConflict(err) {
				t.Fatal("conflict not propagated")
			}
			if fc.CreateCallCount() != tt.creates || fc.UpdateCallCount() != tt.updates {
				t.Fatalf("creates=%d updates=%d", fc.CreateCallCount(), fc.UpdateCallCount())
			}
			if fc.StatusUpdateCallCount() != 0 || fc.StatusUpdateWithRetryCallCount() != 0 {
				t.Fatal("entry reconciliation must not write controller-manager status")
			}
			if tt.updates == 1 && !tt.wantErr {
				_, obj, _ := fc.UpdateArgsForCall(0)
				got := obj.(*spiffev1alpha1.ClusterStaticEntry)
				if utils.ResourceNeedsUpdate(got, want) {
					t.Fatal("entry configuration not reconciled")
				}
				if !metav1.IsControlledBy(got, server) {
					t.Fatal("controller owner reference not reconciled")
				}
				if !reflect.DeepEqual(got.Labels, want.Labels) || !reflect.DeepEqual(got.Annotations, want.Annotations) {
					t.Fatal("update did not apply desired metadata")
				}
				if got.ResourceVersion == "" || got.UID == "" {
					t.Fatal("API version/UID lost")
				}
			}
		})
	}
}

func TestDownstreamLifecycleAndOwnership(t *testing.T) {
	ctx := context.Background()
	server, parent := nestedFixture()
	r, fc, backing := nestedReconciler(t)
	server.Spec.NodeAttestor.ExternalK8sPSAT.Clusters = append(server.Spec.NodeAttestor.ExternalK8sPSAT.Clusters, v1alpha1.DownstreamCluster{Name: nestedTestSecondCluster})
	run := func(createOnly bool) {
		t.Helper()
		if err := r.reconcileDownstreamEntries(ctx, server, parent, status.NewManager(fc), createOnly); err != nil {
			t.Fatal(err)
		}
		if err := r.pruneDownstreamEntries(ctx, server, parent, createOnly); err != nil {
			t.Fatal(err)
		}
	}
	run(false)
	if fc.CreateCallCount() != 4 {
		t.Fatal("expected four entries")
	}
	run(false)
	if fc.UpdateCallCount() != 0 || fc.CreateCallCount() != 4 {
		t.Fatal("repeat reconcile wrote resources")
	}
	entry := &spiffev1alpha1.ClusterStaticEntry{}
	key := client.ObjectKey{Name: "nested-spire-west-node-alias"}
	if err := backing.Get(ctx, key, entry); err != nil {
		t.Fatal(err)
	}
	entry.OwnerReferences = nil
	if err := backing.Update(ctx, entry); err != nil {
		t.Fatal(err)
	}
	run(false)
	if err := backing.Get(ctx, key, entry); err != nil {
		t.Fatal(err)
	}
	if fc.UpdateCallCount() != 1 || !metav1.IsControlledBy(entry, server) {
		t.Fatal("owner-only drift did not restore the controller owner reference")
	}
	run(false)
	if fc.UpdateCallCount() != 1 {
		t.Fatal("repeat reconcile after owner repair wrote resources")
	}
	server.Spec.NodeAttestor.ExternalK8sPSAT.Clusters = server.Spec.NodeAttestor.ExternalK8sPSAT.Clusters[:1]
	run(true)
	if fc.DeleteCallCount() != 0 {
		t.Fatal("create-only pruned")
	}
	run(false)
	if fc.DeleteCallCount() != 2 {
		t.Fatal("remove one must prune exactly two")
	}
	foreign := downstreamEntries(server, parent)[0].DeepCopy()
	foreign.Name = "nested-spire-foreign-node-alias"
	foreign.Labels[downstreamClusterLabel] = nestedTestForeign
	if err := controllerutil.SetControllerReference(server, foreign, r.scheme); err != nil {
		t.Fatal(err)
	}
	foreign.OwnerReferences[0].UID = "replacement-server"
	if err := backing.Create(ctx, foreign); err != nil {
		t.Fatal(err)
	}
	unrelated := foreign.DeepCopy()
	unrelated.ResourceVersion = ""
	unrelated.Name = "user-owned-entry"
	unrelated.OwnerReferences[0].UID = server.UID
	if err := backing.Create(ctx, unrelated); err != nil {
		t.Fatal(err)
	}
	unowned := foreign.DeepCopy()
	unowned.ResourceVersion = ""
	unowned.Name = "nested-spire-unowned-node-alias"
	unowned.OwnerReferences = nil
	if err := backing.Create(ctx, unowned); err != nil {
		t.Fatal(err)
	}
	server.Spec.NodeAttestor = nil
	run(false)
	if fc.DeleteCallCount() != 4 {
		t.Fatal("remove last must prune remaining two only")
	}
	var left spiffev1alpha1.ClusterStaticEntryList
	if err := backing.List(ctx, &left); err != nil {
		t.Fatal(err)
	}
	if len(left.Items) != 3 {
		t.Fatal("foreign, unowned, or unrelated entry deleted")
	}
	for i := 0; i < fc.DeleteCallCount(); i++ {
		_, obj, options := fc.DeleteArgsForCall(i)
		o := (&client.DeleteOptions{}).ApplyOptions(options)
		if o.Preconditions == nil || o.Preconditions.UID == nil || o.Preconditions.ResourceVersion == nil || *o.Preconditions.UID != obj.GetUID() || *o.Preconditions.ResourceVersion != obj.GetResourceVersion() {
			t.Fatal("delete lacks matching UID/version preconditions")
		}
	}
}

func TestDownstreamPartialFailureAndRetry(t *testing.T) {
	ctx := context.Background()
	server, parent := nestedFixture()
	r, fc, backing := nestedReconciler(t)
	calls := 0
	fc.CreateStub = func(ctx context.Context, obj client.Object, opts ...client.CreateOption) error {
		calls++
		if calls == 2 {
			return kerrors.NewServiceUnavailable(nestedTestTemporary)
		}
		return backing.Create(ctx, obj, opts...)
	}
	if err := r.reconcileDownstreamEntries(ctx, server, parent, status.NewManager(fc), false); err == nil {
		t.Fatal("expected partial failure")
	}
	fc.CreateStub = backing.Create
	if err := r.reconcileDownstreamEntries(ctx, server, parent, status.NewManager(fc), false); err != nil {
		t.Fatal(err)
	}
	var entries spiffev1alpha1.ClusterStaticEntryList
	if err := backing.List(ctx, &entries); err != nil {
		t.Fatal(err)
	}
	if len(entries.Items) != 2 || fc.UpdateCallCount() != 0 {
		t.Fatal("partial success did not recover without unnecessary update")
	}
	// Force drift, then a conflict; the next reconciliation must fetch the new version.
	existing := &entries.Items[0]
	existing.Spec.Hint = nestedTestDrift
	if err := backing.Update(ctx, existing); err != nil {
		t.Fatal(err)
	}
	update := fc.UpdateStub
	fc.UpdateReturns(kerrors.NewConflict(schema.GroupResource{Resource: nestedTestEntryResource}, existing.Name, errors.New("stale")))
	if err := r.reconcileDownstreamEntries(ctx, server, parent, status.NewManager(fc), false); !kerrors.IsConflict(err) {
		t.Fatalf("expected conflict: %v", err)
	}
	existing.Annotations = map[string]string{"concurrent": nestedTestPreserve}
	if err := backing.Update(ctx, existing); err != nil {
		t.Fatal(err)
	}
	rv := existing.ResourceVersion
	fc.UpdateStub = func(ctx context.Context, obj client.Object, opts ...client.UpdateOption) error {
		if obj.GetResourceVersion() != rv {
			t.Fatalf("retry used stale version %s, want %s", obj.GetResourceVersion(), rv)
		}
		return update(ctx, obj, opts...)
	}
	if err := r.reconcileDownstreamEntries(ctx, server, parent, status.NewManager(fc), false); err != nil {
		t.Fatal(err)
	}
}

func TestDownstreamPruneLabels(t *testing.T) {
	for _, tt := range []struct {
		name        string
		label       string
		value       string
		wantDeleted bool
	}{
		{
			name: "missing component label", label: utils.AppComponentLabelKey,
			wantDeleted: true,
		},
		{
			name: "changed component label", label: utils.AppComponentLabelKey,
			value: nestedTestOther, wantDeleted: true,
		},
		{
			name: "missing managed-by label", label: utils.AppManagedByLabelKey,
		},
		{
			name: "changed managed-by label", label: utils.AppManagedByLabelKey,
			value: nestedTestOther,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			server, parent := nestedFixture()
			r, _, backing := nestedReconciler(t)
			entry := downstreamEntries(server, parent)[0]
			if err := controllerutil.SetControllerReference(server, entry, r.scheme); err != nil {
				t.Fatal(err)
			}
			if tt.value == "" {
				delete(entry.Labels, tt.label)
			} else {
				entry.Labels[tt.label] = tt.value
			}
			if err := backing.Create(ctx, entry); err != nil {
				t.Fatal(err)
			}
			server.Spec.NodeAttestor = nil
			if err := r.pruneDownstreamEntries(ctx, server, parent, false); err != nil {
				t.Fatal(err)
			}
			got := &spiffev1alpha1.ClusterStaticEntry{}
			err := backing.Get(ctx, client.ObjectKeyFromObject(entry), got)
			if tt.wantDeleted {
				if !kerrors.IsNotFound(err) {
					t.Fatalf("expected obsolete entry to be deleted, got %v", err)
				}
			} else if err != nil {
				t.Fatalf("expected unmanaged entry to be preserved: %v", err)
			}
		})
	}
}

func TestDownstreamPruneErrors(t *testing.T) {
	for _, mode := range []string{nestedTestList, nestedTestDelete, nestedTestConflict, nestedTestGone} {
		t.Run(mode, func(t *testing.T) {
			ctx := context.Background()
			server, parent := nestedFixture()
			r, fc, backing := nestedReconciler(t)
			old := downstreamEntries(server, parent)[0]
			if err := controllerutil.SetControllerReference(server, old, r.scheme); err != nil {
				t.Fatal(err)
			}
			old.UID = types.UID("original-entry")
			if err := backing.Create(ctx, old); err != nil {
				t.Fatal(err)
			}
			server.Spec.NodeAttestor = nil
			switch mode {
			case nestedTestList:
				fc.ListReturns(kerrors.NewServiceUnavailable(nestedTestTemporary))
			case nestedTestDelete:
				fc.DeleteReturns(kerrors.NewServiceUnavailable(nestedTestTemporary))
			case nestedTestConflict:
				fc.DeleteReturns(kerrors.NewConflict(schema.GroupResource{Resource: nestedTestEntryResource}, old.Name, errors.New("UID changed")))
			case nestedTestGone:
				fc.DeleteReturns(kerrors.NewNotFound(schema.GroupResource{Resource: nestedTestEntryResource}, old.Name))
			}
			err := r.pruneDownstreamEntries(ctx, server, parent, false)
			if (err != nil) != (mode != nestedTestGone) {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

// nestedStore supplies state to the existing counterfeiter fake so consecutive
// reconciles can observe prior writes. API admission is tested separately.
type nestedStore struct {
	objects  map[string]client.Object
	sequence int
}

func nestedStoreKey(obj client.Object) string {
	return fmt.Sprintf("%T/%s/%s", obj, obj.GetNamespace(), obj.GetName())
}
func (s *nestedStore) Get(_ context.Context, key client.ObjectKey, obj client.Object) error {
	obj.SetName(key.Name)
	obj.SetNamespace(key.Namespace)
	stored, ok := s.objects[nestedStoreKey(obj)]
	if !ok {
		return kerrors.NewNotFound(schema.GroupResource{Resource: nestedTestObjects}, key.Name)
	}
	reflect.ValueOf(obj).Elem().Set(reflect.ValueOf(stored.DeepCopyObject()).Elem())
	return nil
}
func (s *nestedStore) Create(_ context.Context, obj client.Object, _ ...client.CreateOption) error {
	key := nestedStoreKey(obj)
	if _, ok := s.objects[key]; ok {
		return kerrors.NewAlreadyExists(schema.GroupResource{Resource: nestedTestObjects}, obj.GetName())
	}
	s.sequence++
	if obj.GetUID() == "" {
		obj.SetUID(types.UID(fmt.Sprintf("test-%d", s.sequence)))
	}
	obj.SetResourceVersion("1")
	s.objects[key] = obj.DeepCopyObject().(client.Object)
	return nil
}
func (s *nestedStore) Update(_ context.Context, obj client.Object, _ ...client.UpdateOption) error {
	key := nestedStoreKey(obj)
	old, ok := s.objects[key]
	if !ok {
		return kerrors.NewNotFound(schema.GroupResource{Resource: nestedTestObjects}, obj.GetName())
	}
	if old.GetResourceVersion() != obj.GetResourceVersion() {
		return kerrors.NewConflict(schema.GroupResource{Resource: nestedTestObjects}, obj.GetName(), errors.New("version mismatch"))
	}
	version, err := strconv.Atoi(old.GetResourceVersion())
	if err != nil {
		return err
	}
	obj.SetResourceVersion(strconv.Itoa(version + 1))
	s.objects[key] = obj.DeepCopyObject().(client.Object)
	return nil
}
func (s *nestedStore) List(_ context.Context, list client.ObjectList, opts ...client.ListOption) error {
	entries, ok := list.(*spiffev1alpha1.ClusterStaticEntryList)
	if !ok {
		return fmt.Errorf("unexpected list %T", list)
	}
	options := (&client.ListOptions{}).ApplyOptions(opts)
	for _, obj := range s.objects {
		if entry, ok := obj.(*spiffev1alpha1.ClusterStaticEntry); ok && (options.LabelSelector == nil || options.LabelSelector.Matches(labels.Set(entry.Labels))) {
			entries.Items = append(entries.Items, *entry.DeepCopy())
		}
	}
	sort.Slice(entries.Items, func(i, j int) bool { return entries.Items[i].Name < entries.Items[j].Name })
	return nil
}
func (s *nestedStore) Delete(_ context.Context, obj client.Object, opts ...client.DeleteOption) error {
	key := nestedStoreKey(obj)
	old, ok := s.objects[key]
	if !ok {
		return kerrors.NewNotFound(schema.GroupResource{Resource: nestedTestObjects}, obj.GetName())
	}
	options := (&client.DeleteOptions{}).ApplyOptions(opts)
	if p := options.Preconditions; p != nil {
		if (p.UID != nil && *p.UID != old.GetUID()) || (p.ResourceVersion != nil && *p.ResourceVersion != old.GetResourceVersion()) {
			return kerrors.NewConflict(schema.GroupResource{Resource: nestedTestObjects}, obj.GetName(), errors.New("precondition failed"))
		}
	}
	delete(s.objects, key)
	return nil
}

func TestReconcileDownstreamEntriesStatus(t *testing.T) {
	for _, mode := range []string{
		"ready", "recovered", nestedTestReadyCreateOnly, nestedTestMissing,
		nestedTestDrift, nestedTestForeign, nestedTestNoOwner, nestedTestCreateOnly,
		nestedTestEmpty, "obsolete", "obsolete alongside ready", "obsolete create only", nestedTestReadError,
	} {
		t.Run(mode, func(t *testing.T) {
			ctx := context.Background()
			server, parent := nestedFixture()
			r, fc, backing := nestedReconciler(t)
			for _, entry := range downstreamEntries(server, parent) {
				if err := controllerutil.SetControllerReference(server, entry, r.scheme); err != nil {
					t.Fatal(err)
				}
				switch mode {
				case nestedTestDrift:
					entry.Spec.Hint = nestedTestDrift
				case nestedTestCreateOnly:
					if entry.Spec.Downstream {
						entry.Spec.Hint = nestedTestDrift
					}
				case nestedTestForeign:
					entry.OwnerReferences[0].UID = nestedTestOther
				case nestedTestNoOwner:
					entry.OwnerReferences = nil
				}
				if mode != nestedTestMissing && mode != nestedTestEmpty {
					if err := backing.Create(ctx, entry); err != nil {
						t.Fatal(err)
					}
				}
			}
			if mode == "obsolete alongside ready" {
				obsolete := downstreamEntries(server, parent)[0]
				obsolete.Name = "nested-spire-offboarded-node-alias"
				if err := controllerutil.SetControllerReference(server, obsolete, r.scheme); err != nil {
					t.Fatal(err)
				}
				if err := backing.Create(ctx, obsolete); err != nil {
					t.Fatal(err)
				}
			}
			if mode == nestedTestEmpty || mode == "obsolete" || mode == "obsolete create only" {
				server.Spec.NodeAttestor = nil
			}
			if mode == nestedTestReadError {
				fc.GetReturns(kerrors.NewServiceUnavailable(nestedTestTemporary))
			}
			if mode == "recovered" {
				apimeta.SetStatusCondition(&server.Status.Conditions, metav1.Condition{
					Type: DownstreamClustersValid, Status: metav1.ConditionFalse,
					Reason: nestedTestEntryReconcileFailed, Message: "Previous reconciliation failed",
				})
			}
			manager := status.NewManager(fc)
			createOnly := mode == nestedTestCreateOnly || mode == "obsolete create only" || mode == nestedTestReadyCreateOnly
			err := r.reconcileDownstreamEntries(ctx, server, parent, manager, createOnly)
			if (err != nil) != (mode == nestedTestReadError || mode == nestedTestForeign) {
				t.Fatalf("unexpected error: %v", err)
			}
			if err == nil && fc.GetCallCount() != len(downstreamEntries(server, parent)) {
				t.Fatalf("expected one read per entry, got %d", fc.GetCallCount())
			}
			if mode == nestedTestMissing && fc.CreateCallCount() != 2 {
				t.Fatal("expected both missing entries to be created")
			}
			if (mode == nestedTestDrift || mode == nestedTestNoOwner) && fc.UpdateCallCount() != 2 {
				t.Fatal("expected both entries to be updated")
			}
			if createOnly && fc.UpdateCallCount() != 0 {
				t.Fatal("create-only mode updated entries")
			}
			if err := manager.ApplyStatus(ctx, server, func() *v1alpha1.ConditionalStatus { return &server.Status.ConditionalStatus }); err != nil {
				t.Fatal(err)
			}
			condition := apimeta.FindStatusCondition(server.Status.Conditions, DownstreamClustersValid)
			reason := map[string]string{
				nestedTestForeign:    nestedTestEntryReconcileFailed,
				nestedTestCreateOnly: utils.CreateOnlyModeStatusType,
				nestedTestReadError:  nestedTestEntryReconcileFailed,
			}[mode]
			if reason == "" {
				reason = nestedTestReady
			}
			if condition == nil || condition.Reason != reason {
				t.Fatalf("condition=%+v, want %s", condition, reason)
			}
			if (condition.Status == metav1.ConditionTrue) != (reason == nestedTestReady) {
				t.Fatal("incorrect condition status")
			}
			if reason == nestedTestReady && condition.Message != "All configured downstream ClusterStaticEntry resources are reconciled" {
				t.Fatalf("unexpected success message: %q", condition.Message)
			}
			if mode == nestedTestCreateOnly {
				for _, detail := range []string{"east", "nested-spire-east-downstream", "updates are skipped because create-only mode is enabled"} {
					if !strings.Contains(condition.Message, detail) {
						t.Fatalf("create-only message %q does not contain %q", condition.Message, detail)
					}
				}
			}
			before := fc.StatusUpdateWithRetryCallCount()
			if err == nil {
				manager = status.NewManager(fc)
				if err := r.reconcileDownstreamEntries(ctx, server, parent, manager, createOnly); err != nil {
					t.Fatal(err)
				}
				if err := manager.ApplyStatus(ctx, server, func() *v1alpha1.ConditionalStatus { return &server.Status.ConditionalStatus }); err != nil {
					t.Fatal(err)
				}
				if fc.StatusUpdateWithRetryCallCount() != before {
					t.Fatal("stable condition wrote status again")
				}
			}
		})
	}
}

func TestDownstreamConditionIgnoresEntryStatus(t *testing.T) {
	for _, tt := range []struct {
		name   string
		status spiffev1alpha1.ClusterStaticEntryStatus
	}{
		{name: "unreported"},
		{name: "rendered", status: spiffev1alpha1.ClusterStaticEntryStatus{Rendered: true}},
		{name: "masked", status: spiffev1alpha1.ClusterStaticEntryStatus{Rendered: true, Masked: true}},
		{name: "set", status: spiffev1alpha1.ClusterStaticEntryStatus{Rendered: true, Set: true}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			server, parent := nestedFixture()
			r, fc, backing := nestedReconciler(t)
			for i, entry := range downstreamEntries(server, parent) {
				if err := controllerutil.SetControllerReference(server, entry, r.scheme); err != nil {
					t.Fatal(err)
				}
				entry.Status = tt.status
				// Exercise both an update and an unchanged entry with this status.
				if i == 0 {
					entry.Spec.Hint = nestedTestDrift
				}
				if err := backing.Create(ctx, entry); err != nil {
					t.Fatal(err)
				}
			}
			manager := status.NewManager(fc)
			if err := r.reconcileDownstreamEntries(ctx, server, parent, manager, false); err != nil {
				t.Fatal(err)
			}
			if err := manager.ApplyStatus(ctx, server, func() *v1alpha1.ConditionalStatus { return &server.Status.ConditionalStatus }); err != nil {
				t.Fatal(err)
			}
			condition := apimeta.FindStatusCondition(server.Status.Conditions, DownstreamClustersValid)
			if condition == nil || condition.Status != metav1.ConditionTrue || condition.Reason != nestedTestReady {
				t.Fatalf("controller-manager status affected resource reconciliation: %+v", condition)
			}
			if fc.CreateCallCount() != 0 || fc.UpdateCallCount() != 1 {
				t.Fatalf("expected only the spec drift to trigger a write: creates=%d updates=%d", fc.CreateCallCount(), fc.UpdateCallCount())
			}
		})
	}
}

func TestDownstreamEntryFailureAfterFirstEntry(t *testing.T) {
	for _, createOnly := range []bool{false, true} {
		for _, failure := range []string{nestedTestGet, nestedTestCreate} {
			t.Run(fmt.Sprintf("create-only=%t/%s", createOnly, failure), func(t *testing.T) {
				ctx := context.Background()
				server, parent := nestedFixture()
				r, fc, backing := nestedReconciler(t)
				entries := downstreamEntries(server, parent)
				// The first entry is reconciled, or has unapplied create-only changes.
				first := entries[0].DeepCopy()
				if err := controllerutil.SetControllerReference(server, first, r.scheme); err != nil {
					t.Fatal(err)
				}
				if createOnly {
					first.Spec.Hint = nestedTestDrift
				}
				if err := backing.Create(ctx, first); err != nil {
					t.Fatal(err)
				}
				wantReason := nestedTestEntryReconcileFailed
				if failure == nestedTestGet {
					fc.GetStub = func(ctx context.Context, key client.ObjectKey, obj client.Object) error {
						if key.Name == entries[1].Name {
							return kerrors.NewServiceUnavailable(nestedTestTemporary)
						}
						return backing.Get(ctx, key, obj)
					}
				} else {
					fc.CreateReturns(kerrors.NewAlreadyExists(schema.GroupResource{Resource: nestedTestEntryResource}, entries[1].Name))
					wantReason = "ResourceConflict"
				}
				manager := status.NewManager(fc)
				if err := r.reconcileDownstreamEntries(ctx, server, parent, manager, createOnly); err == nil {
					t.Fatal("expected second-entry error to be returned")
				}
				if fc.GetCallCount() != 2 {
					t.Fatal("first entry prevented reconciling the second entry")
				}
				if err := manager.ApplyStatus(ctx, server, func() *v1alpha1.ConditionalStatus { return &server.Status.ConditionalStatus }); err != nil {
					t.Fatal(err)
				}
				condition := apimeta.FindStatusCondition(server.Status.Conditions, DownstreamClustersValid)
				if condition == nil || condition.Status != metav1.ConditionFalse || condition.Reason != wantReason {
					t.Fatalf("first entry hid API failure: %+v, want %s", condition, wantReason)
				}
			})
		}
	}
}

func TestDownstreamCleanupPreservesConfiguredClusters(t *testing.T) {
	for _, tc := range []struct {
		name      string
		empty     bool
		validWest bool
	}{
		{name: "all Secrets missing"},
		{name: "all Secrets empty", empty: true},
		{name: "one Secret missing", validWest: true},
		{name: "one Secret empty", empty: true, validWest: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			server, parent := nestedFixture()
			server.Spec.NodeAttestor.ExternalK8sPSAT.Clusters = append(server.Spec.NodeAttestor.ExternalK8sPSAT.Clusters,
				v1alpha1.DownstreamCluster{Name: nestedTestSecondCluster},
				v1alpha1.DownstreamCluster{Name: "removed"},
			)
			r, fc, backing := nestedReconciler(t)
			// Start with registrations from a previously working configuration.
			if err := r.reconcileDownstreamEntries(ctx, server, parent, status.NewManager(fc), false); err != nil {
				t.Fatal(err)
			}
			server.Spec.NodeAttestor.ExternalK8sPSAT.Clusters = server.Spec.NodeAttestor.ExternalK8sPSAT.Clusters[:2]
			for _, cluster := range sortedDownstreamClusters(&server.Spec) {
				valid := tc.validWest && cluster.Name == nestedTestSecondCluster
				if !tc.empty && !valid {
					continue
				}
				secret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: downstreamSecretName(cluster), Namespace: nestedTestNamespace}}
				secret.Data = map[string][]byte{downstreamKubeConfigKey: {}}
				if valid {
					secret.Data[downstreamKubeConfigKey] = nestedKubeconfig(t, nestedTestToken)
				}
				if err := backing.Create(ctx, secret); err != nil {
					t.Fatal(err)
				}
			}
			effective, issues, err := r.reconcileDownstreamClusters(ctx, server, parent, status.NewManager(fc), false)
			if err != nil {
				t.Fatal(err)
			}
			wantValid := 0
			if tc.validWest {
				wantValid = 1
			}
			if len(sortedDownstreamClusters(&effective.Spec)) != wantValid || len(issues) != 2-wantValid {
				t.Fatalf("credential filtering changed: effective=%+v, issues=%+v", effective.Spec.NodeAttestor, issues)
			}
			var entries spiffev1alpha1.ClusterStaticEntryList
			if err := backing.List(ctx, &entries); err != nil {
				t.Fatal(err)
			}
			if len(entries.Items) != 4 {
				t.Fatalf("expected four entries for the two configured clusters, got %d", len(entries.Items))
			}
			for _, desired := range downstreamEntries(server, parent) {
				if err := backing.Get(ctx, client.ObjectKeyFromObject(desired), &spiffev1alpha1.ClusterStaticEntry{}); err != nil {
					t.Fatalf("configured cluster lost entry %s: %v", desired.Name, err)
				}
			}
		})
	}
}

func TestDownstreamCredentialRecovery(t *testing.T) {
	ctx := context.Background()
	server, parent := nestedFixture()
	r, fc, backing := nestedReconciler(t)
	secret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: nestedTestSecretName, Namespace: nestedTestNamespace}, Data: map[string][]byte{downstreamKubeConfigKey: nestedKubeconfig(t, nestedTestToken)}}
	check := func(want string, wantEntries int) {
		t.Helper()
		mgr := status.NewManager(fc)
		_, _, err := r.reconcileDownstreamClusters(ctx, server, parent, mgr, false)
		if err != nil {
			t.Fatal(err)
		}
		if err := mgr.ApplyStatus(ctx, server, func() *v1alpha1.ConditionalStatus { return &server.Status.ConditionalStatus }); err != nil {
			t.Fatal(err)
		}
		var entries spiffev1alpha1.ClusterStaticEntryList
		if err := backing.List(ctx, &entries); err != nil {
			t.Fatal(err)
		}
		if len(entries.Items) != wantEntries {
			t.Fatalf("got %d registration entries, want %d", len(entries.Items), wantEntries)
		}
		got := apimeta.FindStatusCondition(server.Status.Conditions, DownstreamClustersValid)
		if want == "" {
			if got != nil {
				t.Fatalf("disabled feature retained its condition: %+v", got)
			}
			return
		}
		if got == nil || got.Reason != want {
			t.Fatalf("condition=%+v, want %s", got, want)
		}
		if strings.Contains(got.Message, nestedTestToken) {
			t.Fatal("credential leaked to status")
		}
	}
	check(nestedTestKubeConfigMissing, 0)
	if err := backing.Create(ctx, secret); err != nil {
		t.Fatal(err)
	}
	check(nestedTestReady, 2)
	if err := backing.Delete(ctx, secret); err != nil {
		t.Fatal(err)
	}
	check(nestedTestKubeConfigMissing, 2)
	secret.ResourceVersion = ""
	secret.UID = ""
	if err := backing.Create(ctx, secret); err != nil {
		t.Fatal(err)
	}
	check(nestedTestReady, 2)
	if fc.CreateCallCount() != 2 {
		t.Fatal("credential recovery recreated existing registrations")
	}
	server.Spec.NodeAttestor = nil
	check("", 0)
	if err := backing.Get(ctx, client.ObjectKeyFromObject(secret), &corev1.Secret{}); err != nil {
		t.Fatal("administrator Secret was removed")
	}
}
