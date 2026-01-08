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
	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestGetSpireAgentService(t *testing.T) {
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
				"monitoring": "prometheus",
				"tier":       "infrastructure",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := getSpireAgentService(tt.customLabels)

			if svc == nil {
				t.Fatal("Expected Service, got nil")
			}

			if svc.Name != "spire-agent" {
				t.Errorf("Expected Service name 'spire-agent', got '%s'", svc.Name)
			}

			if svc.Namespace != utils.GetOperatorNamespace() {
				t.Errorf("Expected Service namespace '%s', got '%s'", utils.GetOperatorNamespace(), svc.Namespace)
			}

			if len(svc.Labels) == 0 {
				t.Error("Expected Service to have labels, got none")
			}

			if val, ok := svc.Labels[utils.AppManagedByLabelKey]; !ok || val != utils.AppManagedByLabelValue {
				t.Errorf("Expected label %s=%s", utils.AppManagedByLabelKey, utils.AppManagedByLabelValue)
			}

			if val, ok := svc.Labels["app.kubernetes.io/component"]; !ok || val != utils.ComponentNodeAgent {
				t.Errorf("Expected label app.kubernetes.io/component=%s", utils.ComponentNodeAgent)
			}

			if len(svc.Spec.Selector) == 0 {
				t.Error("Expected Service to have selectors, got none")
			}

			if val, ok := svc.Spec.Selector["app.kubernetes.io/name"]; !ok || val != "spire-agent" {
				t.Error("Expected selector app.kubernetes.io/name=spire-agent")
			}

			for key, expectedValue := range tt.customLabels {
				if val, ok := svc.Labels[key]; !ok || val != expectedValue {
					t.Errorf("Expected custom label '%s=%s', got '%s'", key, expectedValue, val)
				}
			}
		})
	}

	t.Run("preserves all asset labels", func(t *testing.T) {
		svcWithoutCustom := getSpireAgentService(nil)
		assetLabels := make(map[string]string)
		for k, v := range svcWithoutCustom.Labels {
			assetLabels[k] = v
		}

		customLabels := map[string]string{"region": "us-east-1"}
		svcWithCustom := getSpireAgentService(customLabels)

		for k, v := range assetLabels {
			if svcWithCustom.Labels[k] != v {
				t.Errorf("Asset label '%s=%s' was not preserved when custom labels were added, got '%s'", k, v, svcWithCustom.Labels[k])
			}
		}

		if val, ok := svcWithCustom.Labels["region"]; !ok || val != "us-east-1" {
			t.Errorf("Custom label was not added")
		}
	})
}

// newServiceTestReconciler creates a reconciler for Service tests
func newServiceTestReconciler(fakeClient *fakes.FakeCustomCtrlClient) *SpireAgentReconciler {
	scheme := runtime.NewScheme()
	_ = v1alpha1.AddToScheme(scheme)
	_ = corev1.AddToScheme(scheme)
	return &SpireAgentReconciler{
		ctrlClient:    fakeClient,
		ctx:           context.Background(),
		log:           logr.Discard(),
		scheme:        scheme,
		eventRecorder: record.NewFakeRecorder(100),
	}
}

// TestReconcileAgentService tests the reconcileAgentService function
func TestReconcileAgentService(t *testing.T) {
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
				existingSvc := &corev1.Service{
					ObjectMeta: metav1.ObjectMeta{
						Name:            "spire-agent",
						Namespace:       utils.GetOperatorNamespace(),
						ResourceVersion: "123",
					},
				}
				fc.GetStub = func(ctx context.Context, key client.ObjectKey, obj client.Object) error {
					if svc, ok := obj.(*corev1.Service); ok {
						*svc = *existingSvc
					}
					return nil
				}
			},
			createOnlyMode: true,
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
				existingSvc := &corev1.Service{
					ObjectMeta: metav1.ObjectMeta{
						Name:            "spire-agent",
						Namespace:       utils.GetOperatorNamespace(),
						ResourceVersion: "123",
						Labels:          map[string]string{"old-label": "old-value"},
					},
					Spec: corev1.ServiceSpec{ClusterIP: "10.0.0.1"},
				}
				fc.GetStub = func(ctx context.Context, key client.ObjectKey, obj client.Object) error {
					if svc, ok := obj.(*corev1.Service); ok {
						*svc = *existingSvc
					}
					return nil
				}
				fc.UpdateReturns(nil)
			},
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
				existingSvc := &corev1.Service{
					ObjectMeta: metav1.ObjectMeta{
						Name:            "spire-agent",
						Namespace:       utils.GetOperatorNamespace(),
						ResourceVersion: "123",
						Labels:          map[string]string{"old-label": "old-value"},
					},
					Spec: corev1.ServiceSpec{ClusterIP: "10.0.0.1"},
				}
				fc.GetStub = func(ctx context.Context, key client.ObjectKey, obj client.Object) error {
					if svc, ok := obj.(*corev1.Service); ok {
						*svc = *existingSvc
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
				reconciler = newServiceTestReconciler(fakeClient)
			}
			tt.setupClient(fakeClient)

			statusMgr := status.NewManager(fakeClient)
			err := reconciler.reconcileAgentService(context.Background(), tt.agent, statusMgr, tt.createOnlyMode)

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
			if !tt.expectUpdate && !tt.expectError && fakeClient.UpdateCallCount() != 0 {
				t.Error("Expected Update not to be called")
			}
		})
	}
}

// TestReconcileService tests the reconcileService function
func TestReconcileService(t *testing.T) {
	tests := []struct {
		name         string
		setupClient  func(*fakes.FakeCustomCtrlClient)
		expectError  bool
		expectCreate bool
	}{
		{
			name: "success",
			setupClient: func(fc *fakes.FakeCustomCtrlClient) {
				fc.GetReturns(kerrors.NewNotFound(schema.GroupResource{}, "spire-agent"))
				fc.CreateReturns(nil)
			},
			expectCreate: true,
		},
		{
			name: "error propagated",
			setupClient: func(fc *fakes.FakeCustomCtrlClient) {
				fc.GetReturns(errors.New("connection refused"))
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fakeClient := &fakes.FakeCustomCtrlClient{}
			reconciler := newServiceTestReconciler(fakeClient)
			tt.setupClient(fakeClient)

			agent := &v1alpha1.SpireAgent{
				ObjectMeta: metav1.ObjectMeta{Name: "cluster", UID: "test-uid"},
			}

			statusMgr := status.NewManager(fakeClient)
			err := reconciler.reconcileService(context.Background(), agent, statusMgr, false)

			if tt.expectError && err == nil {
				t.Error("Expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Expected no error, got: %v", err)
			}
			if tt.expectCreate && fakeClient.CreateCallCount() != 1 {
				t.Errorf("Expected Create to be called once, called %d times", fakeClient.CreateCallCount())
			}
		})
	}
}
