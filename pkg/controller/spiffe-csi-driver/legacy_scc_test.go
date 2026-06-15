package spiffe_csi_driver

import (
	"context"
	"errors"
	"testing"

	"github.com/go-logr/logr"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"

	securityv1 "github.com/openshift/api/security/v1"
	"github.com/openshift/zero-trust-workload-identity-manager/pkg/client/fakes"
	"github.com/openshift/zero-trust-workload-identity-manager/pkg/controller/utils"
)

func TestLegacyCSIDriverSCCPredicate(t *testing.T) {
	pred := legacyCSIDriverSCCPredicate()

	legacy := &securityv1.SecurityContextConstraints{ObjectMeta: metav1.ObjectMeta{Name: legacyCSIDriverSCCName}}
	other := &securityv1.SecurityContextConstraints{ObjectMeta: metav1.ObjectMeta{Name: "spire-agent"}}

	if !pred.Create(event.CreateEvent{Object: legacy}) {
		t.Error("expected legacy SCC create event to match predicate")
	}
	if pred.Create(event.CreateEvent{Object: other}) {
		t.Error("expected non-legacy SCC create event to be filtered out")
	}
	if !pred.Update(event.UpdateEvent{ObjectNew: legacy}) {
		t.Error("expected legacy SCC update event to match predicate")
	}
	if !pred.Delete(event.DeleteEvent{Object: legacy}) {
		t.Error("expected legacy SCC delete event to match predicate")
	}
}

func TestDeleteLegacyCSIDriverSCC(t *testing.T) {
	managedLabels := map[string]string{utils.AppManagedByLabelKey: utils.AppManagedByLabelValue}
	managedSCC := &securityv1.SecurityContextConstraints{
		ObjectMeta: metav1.ObjectMeta{
			Name:   legacyCSIDriverSCCName,
			Labels: managedLabels,
		},
	}

	tests := []struct {
		name         string
		getStub      func(context.Context, client.ObjectKey, client.Object) error
		deleteErr    error
		expectError  bool
		expectDelete bool
	}{
		{
			name: "no-op when legacy SCC not found",
			getStub: func(_ context.Context, key client.ObjectKey, obj client.Object) error {
				if _, ok := obj.(*securityv1.SecurityContextConstraints); ok {
					return kerrors.NewNotFound(schema.GroupResource{}, key.Name)
				}
				return nil
			},
		},
		{
			name: "skips delete when not managed by operator",
			getStub: func(_ context.Context, _ client.ObjectKey, obj client.Object) error {
				if scc, ok := obj.(*securityv1.SecurityContextConstraints); ok {
					*scc = *managedSCC
					scc.Labels = map[string]string{"app.kubernetes.io/managed-by": "other"}
					return nil
				}
				return nil
			},
		},
		{
			name: "deletes managed legacy SCC",
			getStub: func(_ context.Context, _ client.ObjectKey, obj client.Object) error {
				if scc, ok := obj.(*securityv1.SecurityContextConstraints); ok {
					*scc = *managedSCC
					return nil
				}
				return nil
			},
			expectDelete: true,
		},
		{
			name: "returns delete error",
			getStub: func(_ context.Context, _ client.ObjectKey, obj client.Object) error {
				if scc, ok := obj.(*securityv1.SecurityContextConstraints); ok {
					*scc = *managedSCC
					return nil
				}
				return nil
			},
			deleteErr:    errors.New("delete failed"),
			expectError:  true,
			expectDelete: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fakeClient := &fakes.FakeCustomCtrlClient{}
			reconciler := &SpiffeCsiReconciler{
				ctrlClient:    fakeClient,
				log:           logr.Discard(),
				eventRecorder: record.NewFakeRecorder(100),
			}
			fakeClient.GetStub = tt.getStub
			fakeClient.DeleteReturns(tt.deleteErr)

			err := reconciler.deleteLegacyCSIDriverSCC(context.Background())

			if tt.expectError && err == nil {
				t.Fatal("expected error but got nil")
			}
			if !tt.expectError && err != nil {
				t.Fatalf("expected no error but got: %v", err)
			}
			if tt.expectDelete && fakeClient.DeleteCallCount() != 1 {
				t.Fatalf("expected Delete call count 1, got %d", fakeClient.DeleteCallCount())
			}
			if !tt.expectDelete && fakeClient.DeleteCallCount() != 0 {
				t.Fatalf("expected no Delete call, got %d", fakeClient.DeleteCallCount())
			}
		})
	}
}
