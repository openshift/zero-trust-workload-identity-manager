package spire_agent

import (
	"context"
	"errors"
	"testing"

	"github.com/go-logr/logr"
	"github.com/openshift/zero-trust-workload-identity-manager/api/v1alpha1"
	"github.com/openshift/zero-trust-workload-identity-manager/pkg/client/fakes"
	"github.com/openshift/zero-trust-workload-identity-manager/pkg/controller/status"
	"github.com/openshift/zero-trust-workload-identity-manager/pkg/controller/utils"
	rbacv1 "k8s.io/api/rbac/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestGetSpireAgentClusterRole(t *testing.T) {
	tests := []struct {
		name         string
		customLabels map[string]string
	}{
		{
			name:         "without custom labels",
			customLabels: nil,
		},
		{
			name: "with custom labels",
			customLabels: map[string]string{
				"custom-label-1": "custom-value-1",
				"env":            "production",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cr := getSpireAgentClusterRole(tt.customLabels)

			if cr == nil {
				t.Fatal("Expected ClusterRole, got nil")
			}

			if cr.Name != "spire-agent" {
				t.Errorf("Expected ClusterRole name 'spire-agent', got '%s'", cr.Name)
			}

			// Check labels
			if val, ok := cr.Labels[utils.AppManagedByLabelKey]; !ok || val != utils.AppManagedByLabelValue {
				t.Errorf("Expected label %s=%s", utils.AppManagedByLabelKey, utils.AppManagedByLabelValue)
			}

			if val, ok := cr.Labels["app.kubernetes.io/component"]; !ok || val != utils.ComponentNodeAgent {
				t.Errorf("Expected label app.kubernetes.io/component=%s", utils.ComponentNodeAgent)
			}

			// Check for asset labels
			if len(cr.Labels) == 0 {
				t.Error("Expected ClusterRole to have labels from asset file")
			}

			// Check custom labels if specified
			for key, expectedValue := range tt.customLabels {
				if val, ok := cr.Labels[key]; !ok || val != expectedValue {
					t.Errorf("Expected custom label '%s=%s', got '%s'", key, expectedValue, val)
				}
			}
		})
	}

	t.Run("preserves all asset labels", func(t *testing.T) {
		crWithoutCustom := getSpireAgentClusterRole(nil)
		assetLabels := make(map[string]string)
		for k, v := range crWithoutCustom.Labels {
			assetLabels[k] = v
		}

		customLabels := map[string]string{"custom-test": "value"}
		crWithCustom := getSpireAgentClusterRole(customLabels)

		for k, v := range assetLabels {
			if crWithCustom.Labels[k] != v {
				t.Errorf("Asset label '%s=%s' was not preserved when custom labels were added, got '%s'", k, v, crWithCustom.Labels[k])
			}
		}

		if val, ok := crWithCustom.Labels["custom-test"]; !ok || val != "value" {
			t.Errorf("Custom label was not added")
		}
	})
}

func TestGetSpireAgentClusterRoleBinding(t *testing.T) {
	tests := []struct {
		name         string
		customLabels map[string]string
	}{
		{
			name:         "without custom labels",
			customLabels: nil,
		},
		{
			name: "with custom labels",
			customLabels: map[string]string{
				"team":        "security",
				"cost-center": "eng-123",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			crb := getSpireAgentClusterRoleBinding(tt.customLabels)

			if crb == nil {
				t.Fatal("Expected ClusterRoleBinding, got nil")
			}

			if crb.Name != "spire-agent" {
				t.Errorf("Expected ClusterRoleBinding name 'spire-agent', got '%s'", crb.Name)
			}

			// Check labels
			if val, ok := crb.Labels[utils.AppManagedByLabelKey]; !ok || val != utils.AppManagedByLabelValue {
				t.Errorf("Expected label %s=%s", utils.AppManagedByLabelKey, utils.AppManagedByLabelValue)
			}

			if val, ok := crb.Labels["app.kubernetes.io/component"]; !ok || val != utils.ComponentNodeAgent {
				t.Errorf("Expected label app.kubernetes.io/component=%s", utils.ComponentNodeAgent)
			}

			// Check custom labels if specified
			for key, expectedValue := range tt.customLabels {
				if val, ok := crb.Labels[key]; !ok || val != expectedValue {
					t.Errorf("Expected custom label '%s=%s', got '%s'", key, expectedValue, val)
				}
			}
		})
	}

	t.Run("preserves all asset labels", func(t *testing.T) {
		crbWithoutCustom := getSpireAgentClusterRoleBinding(nil)
		assetLabels := make(map[string]string)
		for k, v := range crbWithoutCustom.Labels {
			assetLabels[k] = v
		}

		customLabels := map[string]string{"test-label": "test-value"}
		crbWithCustom := getSpireAgentClusterRoleBinding(customLabels)

		for k, v := range assetLabels {
			if crbWithCustom.Labels[k] != v {
				t.Errorf("Asset label '%s=%s' was not preserved when custom labels were added, got '%s'", k, v, crbWithCustom.Labels[k])
			}
		}

		if val, ok := crbWithCustom.Labels["test-label"]; !ok || val != "test-value" {
			t.Errorf("Custom label was not added")
		}
	})
}

// newRBACTestReconciler creates a reconciler for RBAC tests
func newRBACTestReconciler(fakeClient *fakes.FakeCustomCtrlClient) *SpireAgentReconciler {
	scheme := runtime.NewScheme()
	_ = v1alpha1.AddToScheme(scheme)
	_ = rbacv1.AddToScheme(scheme)
	return &SpireAgentReconciler{
		ctrlClient:    fakeClient,
		ctx:           context.Background(),
		log:           logr.Discard(),
		scheme:        scheme,
		eventRecorder: record.NewFakeRecorder(100),
	}
}

func TestReconcileRBAC(t *testing.T) {
	tests := []struct {
		name         string
		setupClient  func(*fakes.FakeCustomCtrlClient)
		expectError  bool
		expectCreate int
	}{
		{
			name: "success - creates both ClusterRole and ClusterRoleBinding",
			setupClient: func(fc *fakes.FakeCustomCtrlClient) {
				fc.GetReturns(kerrors.NewNotFound(schema.GroupResource{}, ""))
				fc.CreateReturns(nil)
			},
			expectError:  false,
			expectCreate: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fakeClient := &fakes.FakeCustomCtrlClient{}
			reconciler := newRBACTestReconciler(fakeClient)
			tt.setupClient(fakeClient)

			agent := &v1alpha1.SpireAgent{
				ObjectMeta: metav1.ObjectMeta{Name: "cluster", UID: "test-uid"},
			}
			statusMgr := status.NewManager(fakeClient)
			err := reconciler.reconcileRBAC(context.Background(), agent, statusMgr, false)

			if tt.expectError && err == nil {
				t.Error("Expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Expected no error, got: %v", err)
			}
			if fakeClient.CreateCallCount() != tt.expectCreate {
				t.Errorf("Expected Create to be called %d times, called %d times", tt.expectCreate, fakeClient.CreateCallCount())
			}
		})
	}
}

func TestReconcileClusterRole(t *testing.T) {
	tests := []struct {
		name           string
		agent          *v1alpha1.SpireAgent
		setupClient    func(*fakes.FakeCustomCtrlClient)
		createOnlyMode bool
		useEmptyScheme bool
		expectError    bool
		expectCreate   bool
		expectUpdate   bool
	}{
		{
			name: "create success",
			agent: &v1alpha1.SpireAgent{
				ObjectMeta: metav1.ObjectMeta{Name: "cluster", UID: "test-uid"},
			},
			setupClient: func(fc *fakes.FakeCustomCtrlClient) {
				fc.GetReturns(kerrors.NewNotFound(schema.GroupResource{}, "spire-agent"))
				fc.CreateReturns(nil)
			},
			expectError:  false,
			expectCreate: true,
		},
		{
			name: "create error",
			agent: &v1alpha1.SpireAgent{
				ObjectMeta: metav1.ObjectMeta{Name: "cluster", UID: "test-uid"},
			},
			setupClient: func(fc *fakes.FakeCustomCtrlClient) {
				fc.GetReturns(kerrors.NewNotFound(schema.GroupResource{}, "spire-agent"))
				fc.CreateReturns(errors.New("create failed"))
			},
			expectError: true,
		},
		{
			name: "get error",
			agent: &v1alpha1.SpireAgent{
				ObjectMeta: metav1.ObjectMeta{Name: "cluster", UID: "test-uid"},
			},
			setupClient: func(fc *fakes.FakeCustomCtrlClient) {
				fc.GetReturns(errors.New("connection refused"))
			},
			expectError: true,
		},
		{
			name: "create only mode skips update",
			agent: &v1alpha1.SpireAgent{
				ObjectMeta: metav1.ObjectMeta{Name: "cluster", UID: "test-uid"},
			},
			setupClient: func(fc *fakes.FakeCustomCtrlClient) {
				existingCR := &rbacv1.ClusterRole{
					ObjectMeta: metav1.ObjectMeta{
						Name:            "spire-agent",
						ResourceVersion: "123",
					},
				}
				fc.GetStub = func(ctx context.Context, key client.ObjectKey, obj client.Object) error {
					if cr, ok := obj.(*rbacv1.ClusterRole); ok {
						*cr = *existingCR
					}
					return nil
				}
			},
			createOnlyMode: true,
			expectError:    false,
			expectUpdate:   false,
		},
		{
			name: "update success",
			agent: &v1alpha1.SpireAgent{
				ObjectMeta: metav1.ObjectMeta{Name: "cluster", UID: "test-uid"},
				Spec: v1alpha1.SpireAgentSpec{
					CommonConfig: v1alpha1.CommonConfig{
						Labels: map[string]string{"new-label": "new-value"},
					},
				},
			},
			setupClient: func(fc *fakes.FakeCustomCtrlClient) {
				existingCR := &rbacv1.ClusterRole{
					ObjectMeta: metav1.ObjectMeta{
						Name:            "spire-agent",
						ResourceVersion: "123",
						Labels:          map[string]string{"old-label": "old-value"},
					},
				}
				fc.GetStub = func(ctx context.Context, key client.ObjectKey, obj client.Object) error {
					if cr, ok := obj.(*rbacv1.ClusterRole); ok {
						*cr = *existingCR
					}
					return nil
				}
				fc.UpdateReturns(nil)
			},
			expectError:  false,
			expectUpdate: true,
		},
		{
			name: "update error",
			agent: &v1alpha1.SpireAgent{
				ObjectMeta: metav1.ObjectMeta{Name: "cluster", UID: "test-uid"},
				Spec: v1alpha1.SpireAgentSpec{
					CommonConfig: v1alpha1.CommonConfig{
						Labels: map[string]string{"new-label": "new-value"},
					},
				},
			},
			setupClient: func(fc *fakes.FakeCustomCtrlClient) {
				existingCR := &rbacv1.ClusterRole{
					ObjectMeta: metav1.ObjectMeta{
						Name:            "spire-agent",
						ResourceVersion: "123",
						Labels:          map[string]string{"old-label": "old-value"},
					},
				}
				fc.GetStub = func(ctx context.Context, key client.ObjectKey, obj client.Object) error {
					if cr, ok := obj.(*rbacv1.ClusterRole); ok {
						*cr = *existingCR
					}
					return nil
				}
				fc.UpdateReturns(errors.New("update conflict"))
			},
			expectError:  true,
			expectUpdate: true,
		},
		{
			name: "set controller ref error",
			agent: &v1alpha1.SpireAgent{
				ObjectMeta: metav1.ObjectMeta{Name: "cluster", UID: "test-uid"},
			},
			setupClient:    func(fc *fakes.FakeCustomCtrlClient) {},
			useEmptyScheme: true,
			expectError:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fakeClient := &fakes.FakeCustomCtrlClient{}
			var reconciler *SpireAgentReconciler
			if tt.useEmptyScheme {
				reconciler = &SpireAgentReconciler{
					ctrlClient:    fakeClient,
					ctx:           context.Background(),
					log:           logr.Discard(),
					scheme:        runtime.NewScheme(),
					eventRecorder: record.NewFakeRecorder(100),
				}
			} else {
				reconciler = newRBACTestReconciler(fakeClient)
			}
			tt.setupClient(fakeClient)

			statusMgr := status.NewManager(fakeClient)
			err := reconciler.reconcileClusterRole(context.Background(), tt.agent, statusMgr, tt.createOnlyMode)

			if tt.expectError && err == nil {
				t.Error("Expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Expected no error, got: %v", err)
			}
			if tt.expectCreate && fakeClient.CreateCallCount() != 1 {
				t.Errorf("Expected Create to be called once, called %d times", fakeClient.CreateCallCount())
			}
			if tt.expectUpdate && fakeClient.UpdateCallCount() != 1 {
				t.Errorf("Expected Update to be called once, called %d times", fakeClient.UpdateCallCount())
			}
			if !tt.expectUpdate && fakeClient.UpdateCallCount() != 0 {
				t.Error("Expected Update not to be called")
			}
		})
	}
}

func TestReconcileClusterRoleBinding(t *testing.T) {
	tests := []struct {
		name           string
		agent          *v1alpha1.SpireAgent
		setupClient    func(*fakes.FakeCustomCtrlClient)
		createOnlyMode bool
		useEmptyScheme bool
		expectError    bool
		expectCreate   bool
		expectUpdate   bool
	}{
		{
			name: "create success",
			agent: &v1alpha1.SpireAgent{
				ObjectMeta: metav1.ObjectMeta{Name: "cluster", UID: "test-uid"},
			},
			setupClient: func(fc *fakes.FakeCustomCtrlClient) {
				fc.GetReturns(kerrors.NewNotFound(schema.GroupResource{}, "spire-agent"))
				fc.CreateReturns(nil)
			},
			expectError:  false,
			expectCreate: true,
		},
		{
			name: "create error",
			agent: &v1alpha1.SpireAgent{
				ObjectMeta: metav1.ObjectMeta{Name: "cluster", UID: "test-uid"},
			},
			setupClient: func(fc *fakes.FakeCustomCtrlClient) {
				fc.GetReturns(kerrors.NewNotFound(schema.GroupResource{}, "spire-agent"))
				fc.CreateReturns(errors.New("create failed"))
			},
			expectError: true,
		},
		{
			name: "get error",
			agent: &v1alpha1.SpireAgent{
				ObjectMeta: metav1.ObjectMeta{Name: "cluster", UID: "test-uid"},
			},
			setupClient: func(fc *fakes.FakeCustomCtrlClient) {
				fc.GetReturns(errors.New("connection refused"))
			},
			expectError: true,
		},
		{
			name: "create only mode skips update",
			agent: &v1alpha1.SpireAgent{
				ObjectMeta: metav1.ObjectMeta{Name: "cluster", UID: "test-uid"},
			},
			setupClient: func(fc *fakes.FakeCustomCtrlClient) {
				existingCRB := &rbacv1.ClusterRoleBinding{
					ObjectMeta: metav1.ObjectMeta{
						Name:            "spire-agent",
						ResourceVersion: "123",
					},
				}
				fc.GetStub = func(ctx context.Context, key client.ObjectKey, obj client.Object) error {
					if crb, ok := obj.(*rbacv1.ClusterRoleBinding); ok {
						*crb = *existingCRB
					}
					return nil
				}
			},
			createOnlyMode: true,
			expectError:    false,
			expectUpdate:   false,
		},
		{
			name: "update success",
			agent: &v1alpha1.SpireAgent{
				ObjectMeta: metav1.ObjectMeta{Name: "cluster", UID: "test-uid"},
				Spec: v1alpha1.SpireAgentSpec{
					CommonConfig: v1alpha1.CommonConfig{
						Labels: map[string]string{"new-label": "new-value"},
					},
				},
			},
			setupClient: func(fc *fakes.FakeCustomCtrlClient) {
				existingCRB := &rbacv1.ClusterRoleBinding{
					ObjectMeta: metav1.ObjectMeta{
						Name:            "spire-agent",
						ResourceVersion: "123",
						Labels:          map[string]string{"old-label": "old-value"},
					},
				}
				fc.GetStub = func(ctx context.Context, key client.ObjectKey, obj client.Object) error {
					if crb, ok := obj.(*rbacv1.ClusterRoleBinding); ok {
						*crb = *existingCRB
					}
					return nil
				}
				fc.UpdateReturns(nil)
			},
			expectError:  false,
			expectUpdate: true,
		},
		{
			name: "update error",
			agent: &v1alpha1.SpireAgent{
				ObjectMeta: metav1.ObjectMeta{Name: "cluster", UID: "test-uid"},
				Spec: v1alpha1.SpireAgentSpec{
					CommonConfig: v1alpha1.CommonConfig{
						Labels: map[string]string{"new-label": "new-value"},
					},
				},
			},
			setupClient: func(fc *fakes.FakeCustomCtrlClient) {
				existingCRB := &rbacv1.ClusterRoleBinding{
					ObjectMeta: metav1.ObjectMeta{
						Name:            "spire-agent",
						ResourceVersion: "123",
						Labels:          map[string]string{"old-label": "old-value"},
					},
				}
				fc.GetStub = func(ctx context.Context, key client.ObjectKey, obj client.Object) error {
					if crb, ok := obj.(*rbacv1.ClusterRoleBinding); ok {
						*crb = *existingCRB
					}
					return nil
				}
				fc.UpdateReturns(errors.New("update conflict"))
			},
			expectError:  true,
			expectUpdate: true,
		},
		{
			name: "set controller ref error",
			agent: &v1alpha1.SpireAgent{
				ObjectMeta: metav1.ObjectMeta{Name: "cluster", UID: "test-uid"},
			},
			setupClient:    func(fc *fakes.FakeCustomCtrlClient) {},
			useEmptyScheme: true,
			expectError:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fakeClient := &fakes.FakeCustomCtrlClient{}
			var reconciler *SpireAgentReconciler
			if tt.useEmptyScheme {
				reconciler = &SpireAgentReconciler{
					ctrlClient:    fakeClient,
					ctx:           context.Background(),
					log:           logr.Discard(),
					scheme:        runtime.NewScheme(),
					eventRecorder: record.NewFakeRecorder(100),
				}
			} else {
				reconciler = newRBACTestReconciler(fakeClient)
			}
			tt.setupClient(fakeClient)

			statusMgr := status.NewManager(fakeClient)
			err := reconciler.reconcileClusterRoleBinding(context.Background(), tt.agent, statusMgr, tt.createOnlyMode)

			if tt.expectError && err == nil {
				t.Error("Expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Expected no error, got: %v", err)
			}
			if tt.expectCreate && fakeClient.CreateCallCount() != 1 {
				t.Errorf("Expected Create to be called once, called %d times", fakeClient.CreateCallCount())
			}
			if tt.expectUpdate && fakeClient.UpdateCallCount() != 1 {
				t.Errorf("Expected Update to be called once, called %d times", fakeClient.UpdateCallCount())
			}
			if !tt.expectUpdate && fakeClient.UpdateCallCount() != 0 {
				t.Error("Expected Update not to be called")
			}
		})
	}
}
