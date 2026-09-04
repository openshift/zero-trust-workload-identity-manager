/*
Copyright 2026 Red Hat, Inc.
Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at
    http://www.apache.org/licenses/LICENSE-2.0
Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package tls

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/go-logr/logr"
	"github.com/openshift/zero-trust-workload-identity-manager/api/v1alpha1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	ztwimAPIResourcePath = "/apis/operator.openshift.io/v1alpha1/zerotrustworkloadidentitymanagers/cluster"
	ztwimStatusPath      = ztwimAPIResourcePath + "/status"
)

func ztwimStatusTestScheme(t *testing.T) *runtime.Scheme {
	t.Helper()

	scheme := runtime.NewScheme()
	utilruntime.Must(v1alpha1.AddToScheme(scheme))
	return scheme
}

type ztwimStatusStore struct {
	mu  sync.Mutex
	obj *v1alpha1.ZeroTrustWorkloadIdentityManager
}

func (s *ztwimStatusStore) get() (*v1alpha1.ZeroTrustWorkloadIdentityManager, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.obj == nil {
		return nil, false
	}
	return s.obj.DeepCopy(), true
}

func (s *ztwimStatusStore) updateStatus(status v1alpha1.ZeroTrustWorkloadIdentityManagerStatus) (*v1alpha1.ZeroTrustWorkloadIdentityManager, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.obj == nil {
		return nil, apierrors.NewNotFound(v1alpha1.Resource("zerotrustworkloadidentitymanagers"), ztwimClusterName)
	}
	s.obj.Status = *status.DeepCopy()
	if rv, err := parseResourceVersion(s.obj.ResourceVersion); err == nil {
		s.obj.SetResourceVersion(formatResourceVersion(rv + 1))
	}
	return s.obj.DeepCopy(), nil
}

func newZTWIMStatusTestClient(t *testing.T, initial *v1alpha1.ZeroTrustWorkloadIdentityManager) client.Client {
	t.Helper()

	srv := newZTWIMStatusTestServer(t, initial)
	t.Cleanup(srv.Close)

	cfg := &rest.Config{
		Host: srv.URL,
		TLSClientConfig: rest.TLSClientConfig{
			Insecure: true,
		},
	}

	k8sClient, err := client.New(cfg, client.Options{Scheme: ztwimStatusTestScheme(t)})
	if err != nil {
		t.Fatalf("client.New() error = %v", err)
	}
	return k8sClient
}

func newZTWIMStatusTestServer(t *testing.T, initial *v1alpha1.ZeroTrustWorkloadIdentityManager) *httptest.Server {
	t.Helper()

	store := &ztwimStatusStore{}
	if initial != nil {
		obj := initial.DeepCopy()
		obj.APIVersion = v1alpha1.GroupVersion.String()
		obj.Kind = "ZeroTrustWorkloadIdentityManager"
		if obj.ResourceVersion == "" {
			obj.SetResourceVersion("1")
		}
		store.obj = obj
	}

	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api":
			writeTestJSON(w, `{"kind":"APIVersions","apiVersion":"v1","versions":["v1"]}`)
		case "/apis":
			writeTestJSON(w, `{"kind":"APIGroupList","apiVersion":"v1","groups":[{"name":"operator.openshift.io","versions":[{"groupVersion":"operator.openshift.io/v1alpha1","version":"v1alpha1"}]}]}`)
		case "/apis/operator.openshift.io/v1alpha1":
			writeTestJSON(w, `{"kind":"APIResourceList","apiVersion":"v1","groupVersion":"operator.openshift.io/v1alpha1","resources":[{"name":"zerotrustworkloadidentitymanagers","namespaced":false,"kind":"ZeroTrustWorkloadIdentityManager","verbs":["get","list","watch","update","patch"]}]}`)
		case ztwimAPIResourcePath:
			handleZTWIMGet(t, w, r, store)
		case ztwimStatusPath:
			handleZTWIMStatusUpdate(t, w, r, store)
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
}

func handleZTWIMGet(t *testing.T, w http.ResponseWriter, r *http.Request, store *ztwimStatusStore) {
	t.Helper()

	if r.Method != http.MethodGet {
		t.Fatalf("unexpected method for cluster CR: %s", r.Method)
	}

	obj, ok := store.get()
	if !ok {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	writeTestJSON(w, obj)
}

func handleZTWIMStatusUpdate(t *testing.T, w http.ResponseWriter, r *http.Request, store *ztwimStatusStore) {
	t.Helper()

	if r.Method != http.MethodPut {
		t.Fatalf("unexpected method for status subresource: %s", r.Method)
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		t.Fatalf("failed to read status update body: %v", err)
	}

	var updated v1alpha1.ZeroTrustWorkloadIdentityManager
	if err := json.Unmarshal(body, &updated); err != nil {
		t.Fatalf("failed to decode status update body: %v", err)
	}

	obj, err := store.updateStatus(updated.Status)
	if apierrors.IsNotFound(err) {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	if err != nil {
		t.Fatalf("updateStatus() error = %v", err)
	}
	writeTestJSON(w, obj)
}

func writeTestJSON(w http.ResponseWriter, obj any) {
	w.Header().Set("Content-Type", "application/json")
	switch v := obj.(type) {
	case string:
		_, _ = w.Write([]byte(v))
	default:
		body, err := json.Marshal(v)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		_, _ = w.Write(body)
	}
}

func parseResourceVersion(rv string) (int, error) {
	if rv == "" {
		return 0, nil
	}
	var version int
	_, err := fmt.Sscanf(rv, "%d", &version)
	return version, err
}

func formatResourceVersion(version int) string {
	return fmt.Sprintf("%d", version)
}

func getZTWIMReadyCondition(t *testing.T, c client.Client) *metav1.Condition {
	t.Helper()

	var ztwim v1alpha1.ZeroTrustWorkloadIdentityManager
	if err := c.Get(context.Background(), client.ObjectKey{Name: ztwimClusterName}, &ztwim); err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	return apimeta.FindStatusCondition(ztwim.Status.Conditions, v1alpha1.Ready)
}

func TestReportTLSResolutionFailure_updatesReadyOnClusterCR(t *testing.T) {
	k8sClient := newZTWIMStatusTestClient(t, &v1alpha1.ZeroTrustWorkloadIdentityManager{
		ObjectMeta: metav1.ObjectMeta{Name: ztwimClusterName},
		Status: v1alpha1.ZeroTrustWorkloadIdentityManagerStatus{
			ConditionalStatus: v1alpha1.ConditionalStatus{
				Conditions: []metav1.Condition{{
					Type:    v1alpha1.Ready,
					Status:  metav1.ConditionTrue,
					Reason:  v1alpha1.ReasonReady,
					Message: "operands ready",
				}},
			},
		},
	})

	ReportTLSResolutionFailure(context.Background(), k8sClient, logr.Discard(), errors.New("apiserver forbidden"))

	ready := getZTWIMReadyCondition(t, k8sClient)
	if ready == nil {
		t.Fatal("expected Ready condition on cluster CR")
	}
	if ready.Status != metav1.ConditionFalse {
		t.Fatalf("Ready status = %q, want False", ready.Status)
	}
	if ready.Reason != v1alpha1.ReasonFailed {
		t.Fatalf("Ready reason = %q, want %q", ready.Reason, v1alpha1.ReasonFailed)
	}
	if ready.Message != "failed to resolve cluster TLS profile" {
		t.Fatalf("Ready message = %q", ready.Message)
	}
}

func TestReportTLSResolutionFailure_preservesOtherConditions(t *testing.T) {
	k8sClient := newZTWIMStatusTestClient(t, &v1alpha1.ZeroTrustWorkloadIdentityManager{
		ObjectMeta: metav1.ObjectMeta{Name: ztwimClusterName},
		Status: v1alpha1.ZeroTrustWorkloadIdentityManagerStatus{
			ConditionalStatus: v1alpha1.ConditionalStatus{
				Conditions: []metav1.Condition{
					{
						Type:    v1alpha1.Ready,
						Status:  metav1.ConditionTrue,
						Reason:  v1alpha1.ReasonReady,
						Message: "operands ready",
					},
					{
						Type:    v1alpha1.Upgradeable,
						Status:  metav1.ConditionTrue,
						Reason:  v1alpha1.ReasonReady,
						Message: "safe to upgrade",
					},
				},
			},
		},
	})

	ReportTLSResolutionFailure(context.Background(), k8sClient, logr.Discard(), errors.New("timeout"))

	var ztwim v1alpha1.ZeroTrustWorkloadIdentityManager
	if err := k8sClient.Get(context.Background(), client.ObjectKey{Name: ztwimClusterName}, &ztwim); err != nil {
		t.Fatalf("Get() error = %v", err)
	}

	upgradeable := apimeta.FindStatusCondition(ztwim.Status.Conditions, v1alpha1.Upgradeable)
	if upgradeable == nil {
		t.Fatal("expected Upgradeable condition to remain on cluster CR")
	}
	if upgradeable.Status != metav1.ConditionTrue || upgradeable.Message != "safe to upgrade" {
		t.Fatalf("Upgradeable condition changed: %#v", upgradeable)
	}
}

func TestReportTLSResolutionFailure_clusterCRNotFoundIsNoOp(t *testing.T) {
	k8sClient := newZTWIMStatusTestClient(t, nil)

	ReportTLSResolutionFailure(context.Background(), k8sClient, logr.Discard(), errors.New("tls fetch failed"))

	var ztwim v1alpha1.ZeroTrustWorkloadIdentityManager
	err := k8sClient.Get(context.Background(), client.ObjectKey{Name: ztwimClusterName}, &ztwim)
	if !apierrors.IsNotFound(err) {
		t.Fatalf("Get() error = %v, want NotFound", err)
	}
}

func TestReportTLSResolutionFailure_getErrorLeavesStatusUnchanged(t *testing.T) {
	inner := newZTWIMStatusTestClient(t, &v1alpha1.ZeroTrustWorkloadIdentityManager{
		ObjectMeta: metav1.ObjectMeta{Name: ztwimClusterName},
		Status: v1alpha1.ZeroTrustWorkloadIdentityManagerStatus{
			ConditionalStatus: v1alpha1.ConditionalStatus{
				Conditions: []metav1.Condition{{
					Type:    v1alpha1.Ready,
					Status:  metav1.ConditionTrue,
					Reason:  v1alpha1.ReasonReady,
					Message: "operands ready",
				}},
			},
		},
	})

	k8sClient := &failingGetClient{
		Client: inner,
		err: apierrors.NewForbidden(
			schema.GroupResource{Group: v1alpha1.GroupVersion.Group, Resource: "zerotrustworkloadidentitymanagers"},
			ztwimClusterName,
			errors.New("denied"),
		),
	}

	ReportTLSResolutionFailure(context.Background(), k8sClient, logr.Discard(), errors.New("tls fetch failed"))

	ready := getZTWIMReadyCondition(t, inner)
	if ready == nil || ready.Status != metav1.ConditionTrue || ready.Message != "operands ready" {
		t.Fatalf("expected Ready to remain unchanged, got %#v", ready)
	}
}

type failingGetClient struct {
	client.Client
	err error
}

func (c *failingGetClient) Get(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
	return c.err
}
