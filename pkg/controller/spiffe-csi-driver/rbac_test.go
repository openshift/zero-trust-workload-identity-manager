package spiffe_csi_driver

import (
	"context"
	"errors"
	"testing"

	"github.com/go-logr/logr"
	rbacv1 "k8s.io/api/rbac/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/openshift/zero-trust-workload-identity-manager/api/v1alpha1"
	"github.com/openshift/zero-trust-workload-identity-manager/pkg/client/fakes"
	"github.com/openshift/zero-trust-workload-identity-manager/pkg/controller/status"
	"github.com/openshift/zero-trust-workload-identity-manager/pkg/controller/utils"
)

func TestGetSpiffeCSIDriverPrivilegedRoleBinding(t *testing.T) {
	rb := getSpiffeCSIDriverPrivilegedRoleBinding(nil)
	if rb == nil {
		t.Fatal("expected RoleBinding, got nil")
	}
	if rb.Name != privilegedSCCRoleBindingName {
		t.Errorf("expected name %q, got %q", privilegedSCCRoleBindingName, rb.Name)
	}
	if rb.RoleRef.Kind != "ClusterRole" {
		t.Errorf("expected ClusterRole roleRef, got %q", rb.RoleRef.Kind)
	}
	if rb.RoleRef.Name != privilegedSCCClusterRoleName {
		t.Errorf("expected roleRef %q, got %q", privilegedSCCClusterRoleName, rb.RoleRef.Name)
	}
	if len(rb.Subjects) != 1 || rb.Subjects[0].Name != "spire-spiffe-csi-driver" {
		t.Errorf("unexpected subjects: %+v", rb.Subjects)
	}

	expectedLabels := utils.SpiffeCSIDriverLabels(nil)
	if len(rb.Labels) != len(expectedLabels) {
		t.Errorf("expected %d labels, got %d: %v", len(expectedLabels), len(rb.Labels), rb.Labels)
	}
	for k, v := range expectedLabels {
		if rb.Labels[k] != v {
			t.Errorf("expected label %q=%q, got %q", k, v, rb.Labels[k])
		}
	}
}

func TestReconcilePrivilegedRoleBinding(t *testing.T) {
	tests := []struct {
		name        string
		getErr      error
		createErr   error
		expectError bool
	}{
		{
			name:        "creates RoleBinding when not found",
			getErr:      kerrors.NewNotFound(schema.GroupResource{}, privilegedSCCRoleBindingName),
			expectError: false,
		},
		{
			name:        "returns get error",
			getErr:      errors.New("connection error"),
			expectError: true,
		},
		{
			name:        "returns create error",
			getErr:      kerrors.NewNotFound(schema.GroupResource{}, privilegedSCCRoleBindingName),
			createErr:   errors.New("create failed"),
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fakeClient := &fakes.FakeCustomCtrlClient{}
			scheme := runtime.NewScheme()
			_ = v1alpha1.AddToScheme(scheme)
			_ = rbacv1.AddToScheme(scheme)

			reconciler := &SpiffeCsiReconciler{
				ctrlClient:    fakeClient,
				log:           logr.Discard(),
				scheme:        scheme,
				eventRecorder: record.NewFakeRecorder(100),
			}

			fakeClient.GetReturns(tt.getErr)
			if tt.createErr != nil {
				fakeClient.CreateReturns(tt.createErr)
			}

			driver := &v1alpha1.SpiffeCSIDriver{
				ObjectMeta: metav1.ObjectMeta{Name: "cluster", UID: "test-uid"},
			}
			statusMgr := status.NewManager(fakeClient)

			err := reconciler.reconcilePrivilegedRoleBinding(context.Background(), driver, statusMgr, false)
			if tt.expectError && err == nil {
				t.Fatal("expected error but got nil")
			}
			if !tt.expectError && err != nil {
				t.Fatalf("expected no error but got: %v", err)
			}
		})
	}
}

func TestReconcilePrivilegedRoleBinding_UpToDate(t *testing.T) {
	fakeClient := &fakes.FakeCustomCtrlClient{}
	scheme := runtime.NewScheme()
	_ = v1alpha1.AddToScheme(scheme)
	_ = rbacv1.AddToScheme(scheme)

	reconciler := &SpiffeCsiReconciler{
		ctrlClient:    fakeClient,
		log:           logr.Discard(),
		scheme:        scheme,
		eventRecorder: record.NewFakeRecorder(100),
	}

	desired := getSpiffeCSIDriverPrivilegedRoleBinding(nil)
	existing := desired.DeepCopy()
	existing.ResourceVersion = "1"

	fakeClient.GetStub = func(_ context.Context, _ client.ObjectKey, obj client.Object) error {
		if rb, ok := obj.(*rbacv1.RoleBinding); ok {
			*rb = *existing
		}
		return nil
	}

	driver := &v1alpha1.SpiffeCSIDriver{
		ObjectMeta: metav1.ObjectMeta{Name: "cluster", UID: "test-uid"},
	}
	statusMgr := status.NewManager(fakeClient)

	if err := reconciler.reconcilePrivilegedRoleBinding(context.Background(), driver, statusMgr, false); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if fakeClient.UpdateCallCount() != 0 {
		t.Fatal("expected no update when RoleBinding is up to date")
	}
}

func TestReconcilePrivilegedRoleBinding_CustomLabels(t *testing.T) {
	customLabels := map[string]string{"environment": "production"}
	rb := getSpiffeCSIDriverPrivilegedRoleBinding(customLabels)

	expectedLabels := utils.SpiffeCSIDriverLabels(customLabels)
	for k, v := range expectedLabels {
		if rb.Labels[k] != v {
			t.Errorf("expected label %q=%q, got %q", k, v, rb.Labels[k])
		}
	}
}
