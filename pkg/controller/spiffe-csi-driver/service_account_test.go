package spiffe_csi_driver

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

func TestGetSpiffeCSIDriverServiceAccount(t *testing.T) {
	sa := getSpiffeCSIDriverServiceAccount(nil)

	if sa == nil {
		t.Fatal("Expected ServiceAccount, got nil")
	}

	if sa.Name != "spire-spiffe-csi-driver" {
		t.Errorf("Expected ServiceAccount name 'spire-spiffe-csi-driver', got '%s'", sa.Name)
	}

	if sa.Namespace != utils.GetOperatorNamespace() {
		t.Errorf("Expected ServiceAccount namespace '%s', got '%s'", utils.GetOperatorNamespace(), sa.Namespace)
	}

	// Check labels
	if len(sa.Labels) == 0 {
		t.Error("Expected ServiceAccount to have labels, got none")
	}

	// Check for required labels
	if val, ok := sa.Labels[utils.AppManagedByLabelKey]; !ok || val != utils.AppManagedByLabelValue {
		t.Errorf("Expected label %s=%s", utils.AppManagedByLabelKey, utils.AppManagedByLabelValue)
	}

	if val, ok := sa.Labels["app.kubernetes.io/component"]; !ok || val != utils.ComponentCSI {
		t.Errorf("Expected label app.kubernetes.io/component=%s", utils.ComponentCSI)
	}
}

// newSATestReconciler creates a reconciler for ServiceAccount tests
func newSATestReconciler(fakeClient *fakes.FakeCustomCtrlClient) *SpiffeCsiReconciler {
	scheme := runtime.NewScheme()
	_ = v1alpha1.AddToScheme(scheme)
	_ = corev1.AddToScheme(scheme)
	return &SpiffeCsiReconciler{
		ctrlClient:    fakeClient,
		ctx:           context.Background(),
		log:           logr.Discard(),
		scheme:        scheme,
		eventRecorder: record.NewFakeRecorder(100),
	}
}

// TestReconcileServiceAccount tests the reconcileServiceAccount function
func TestReconcileServiceAccount(t *testing.T) {
	tests := []struct {
		name           string
		driver         *v1alpha1.SpiffeCSIDriver
		setupClient    func(*fakes.FakeCustomCtrlClient)
		createOnlyMode bool
		useEmptyScheme bool
		expectError    bool
		expectCreate   bool
		expectUpdate   bool
	}{
		{
			name: "create success",
			driver: &v1alpha1.SpiffeCSIDriver{
				ObjectMeta: metav1.ObjectMeta{Name: "cluster", UID: "test-uid"},
			},
			setupClient: func(fc *fakes.FakeCustomCtrlClient) {
				fc.GetReturns(kerrors.NewNotFound(schema.GroupResource{}, "spire-spiffe-csi-driver"))
				fc.CreateReturns(nil)
			},
			expectCreate: true,
		},
		{
			name: "create error",
			driver: &v1alpha1.SpiffeCSIDriver{
				ObjectMeta: metav1.ObjectMeta{Name: "cluster", UID: "test-uid"},
			},
			setupClient: func(fc *fakes.FakeCustomCtrlClient) {
				fc.GetReturns(kerrors.NewNotFound(schema.GroupResource{}, "spire-spiffe-csi-driver"))
				fc.CreateReturns(errors.New("create failed"))
			},
			expectError: true,
		},
		{
			name: "get error",
			driver: &v1alpha1.SpiffeCSIDriver{
				ObjectMeta: metav1.ObjectMeta{Name: "cluster", UID: "test-uid"},
			},
			setupClient: func(fc *fakes.FakeCustomCtrlClient) {
				fc.GetReturns(errors.New("connection refused"))
			},
			expectError: true,
		},
		{
			name: "create only mode skips update",
			driver: &v1alpha1.SpiffeCSIDriver{
				ObjectMeta: metav1.ObjectMeta{Name: "cluster", UID: "test-uid"},
			},
			setupClient: func(fc *fakes.FakeCustomCtrlClient) {
				existingSA := &corev1.ServiceAccount{
					ObjectMeta: metav1.ObjectMeta{
						Name:            "spire-spiffe-csi-driver",
						Namespace:       utils.GetOperatorNamespace(),
						ResourceVersion: "123",
					},
				}
				fc.GetStub = func(ctx context.Context, key client.ObjectKey, obj client.Object) error {
					if sa, ok := obj.(*corev1.ServiceAccount); ok {
						*sa = *existingSA
					}
					return nil
				}
			},
			createOnlyMode: true,
		},
		{
			name: "update success",
			driver: &v1alpha1.SpiffeCSIDriver{
				ObjectMeta: metav1.ObjectMeta{Name: "cluster", UID: "test-uid"},
				Spec: v1alpha1.SpiffeCSIDriverSpec{
					CommonConfig: v1alpha1.CommonConfig{
						Labels: map[string]string{"new-label": "new-value"},
					},
				},
			},
			setupClient: func(fc *fakes.FakeCustomCtrlClient) {
				existingSA := &corev1.ServiceAccount{
					ObjectMeta: metav1.ObjectMeta{
						Name:            "spire-spiffe-csi-driver",
						Namespace:       utils.GetOperatorNamespace(),
						ResourceVersion: "123",
						Labels:          map[string]string{"old-label": "old-value"},
					},
				}
				fc.GetStub = func(ctx context.Context, key client.ObjectKey, obj client.Object) error {
					if sa, ok := obj.(*corev1.ServiceAccount); ok {
						*sa = *existingSA
					}
					return nil
				}
				fc.UpdateReturns(nil)
			},
			expectUpdate: true,
		},
		{
			name: "update error",
			driver: &v1alpha1.SpiffeCSIDriver{
				ObjectMeta: metav1.ObjectMeta{Name: "cluster", UID: "test-uid"},
				Spec: v1alpha1.SpiffeCSIDriverSpec{
					CommonConfig: v1alpha1.CommonConfig{
						Labels: map[string]string{"new-label": "new-value"},
					},
				},
			},
			setupClient: func(fc *fakes.FakeCustomCtrlClient) {
				existingSA := &corev1.ServiceAccount{
					ObjectMeta: metav1.ObjectMeta{
						Name:            "spire-spiffe-csi-driver",
						Namespace:       utils.GetOperatorNamespace(),
						ResourceVersion: "123",
						Labels:          map[string]string{"old-label": "old-value"},
					},
				}
				fc.GetStub = func(ctx context.Context, key client.ObjectKey, obj client.Object) error {
					if sa, ok := obj.(*corev1.ServiceAccount); ok {
						*sa = *existingSA
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
			driver: &v1alpha1.SpiffeCSIDriver{
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
			var reconciler *SpiffeCsiReconciler
			if tt.useEmptyScheme {
				reconciler = &SpiffeCsiReconciler{
					ctrlClient:    fakeClient,
					ctx:           context.Background(),
					log:           logr.Discard(),
					scheme:        runtime.NewScheme(),
					eventRecorder: record.NewFakeRecorder(100),
				}
			} else {
				reconciler = newSATestReconciler(fakeClient)
			}
			tt.setupClient(fakeClient)

			statusMgr := status.NewManager(fakeClient)
			err := reconciler.reconcileServiceAccount(context.Background(), tt.driver, statusMgr, tt.createOnlyMode)

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
