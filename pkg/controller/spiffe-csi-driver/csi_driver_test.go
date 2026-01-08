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
	storagev1 "k8s.io/api/storage/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestGetSpiffeCSIDriver(t *testing.T) {
	t.Run("without custom labels", func(t *testing.T) {
		pluginName := "csi.spiffe.io"
		csiDriver := getSpiffeCSIDriver(pluginName, nil)

		if csiDriver == nil {
			t.Fatal("Expected CSIDriver, got nil")
		}

		if csiDriver.Name != pluginName {
			t.Errorf("Expected CSIDriver name '%s', got '%s'", pluginName, csiDriver.Name)
		}

		// Check labels
		if len(csiDriver.Labels) == 0 {
			t.Error("Expected CSIDriver to have labels, got none")
		}

		// Check for required standard labels
		if val, ok := csiDriver.Labels[utils.AppManagedByLabelKey]; !ok || val != utils.AppManagedByLabelValue {
			t.Errorf("Expected label %s=%s", utils.AppManagedByLabelKey, utils.AppManagedByLabelValue)
		}

		if val, ok := csiDriver.Labels["app.kubernetes.io/component"]; !ok || val != utils.ComponentCSI {
			t.Errorf("Expected label app.kubernetes.io/component=%s", utils.ComponentCSI)
		}

		// CRITICAL: Check for the security label from asset file
		// This label is required for pod security admission to work correctly
		if val, ok := csiDriver.Labels["security.openshift.io/csi-ephemeral-volume-profile"]; !ok || val != "restricted" {
			t.Errorf("Expected security label 'security.openshift.io/csi-ephemeral-volume-profile=restricted', got '%s=%s'", "security.openshift.io/csi-ephemeral-volume-profile", val)
			t.Error("This label MUST be preserved from the asset file for pod security admission to work")
		}
	})

	t.Run("with custom labels", func(t *testing.T) {
		pluginName := "csi.spiffe.io"
		customLabels := map[string]string{
			"custom-label-1": "custom-value-1",
			"custom-label-2": "custom-value-2",
			"security.openshift.io/csi-ephemeral-volume-profile": "privileged",
		}

		csiDriver := getSpiffeCSIDriver(pluginName, customLabels)

		if csiDriver == nil {
			t.Fatal("Expected CSIDriver, got nil")
		}

		// Check that custom labels are present
		if val, ok := csiDriver.Labels["custom-label-1"]; !ok || val != "custom-value-1" {
			t.Errorf("Expected custom label 'custom-label-1=custom-value-1', got '%s'", val)
		}

		if val, ok := csiDriver.Labels["custom-label-2"]; !ok || val != "custom-value-2" {
			t.Errorf("Expected custom label 'custom-label-2=custom-value-2', got '%s'", val)
		}

		// Check that standard labels are still present
		if val, ok := csiDriver.Labels[utils.AppManagedByLabelKey]; !ok || val != utils.AppManagedByLabelValue {
			t.Errorf("Expected label %s=%s", utils.AppManagedByLabelKey, utils.AppManagedByLabelValue)
		}

		// CRITICAL: Check that the security label from asset file is preserved
		if val, ok := csiDriver.Labels["security.openshift.io/csi-ephemeral-volume-profile"]; !ok || val != "restricted" {
			t.Errorf("Expected security label 'security.openshift.io/csi-ephemeral-volume-profile=restricted', got '%s=%s'", "security.openshift.io/csi-ephemeral-volume-profile", val)
			t.Error("This label MUST be preserved from the asset file even when custom labels are provided")
		}
	})

	t.Run("with custom plugin name", func(t *testing.T) {
		pluginName := "csi.custom.io"
		csiDriver := getSpiffeCSIDriver(pluginName, nil)

		if csiDriver == nil {
			t.Fatal("Expected CSIDriver, got nil")
		}

		if csiDriver.Name != pluginName {
			t.Errorf("Expected CSIDriver name '%s', got '%s'", pluginName, csiDriver.Name)
		}

		// Verify standard labels are still present
		if val, ok := csiDriver.Labels[utils.AppManagedByLabelKey]; !ok || val != utils.AppManagedByLabelValue {
			t.Errorf("Expected label %s=%s", utils.AppManagedByLabelKey, utils.AppManagedByLabelValue)
		}
	})
}

// newCSITestReconciler creates a reconciler for CSI driver tests
func newCSITestReconciler(fakeClient *fakes.FakeCustomCtrlClient) *SpiffeCsiReconciler {
	scheme := runtime.NewScheme()
	_ = v1alpha1.AddToScheme(scheme)
	_ = storagev1.AddToScheme(scheme)
	return &SpiffeCsiReconciler{
		ctrlClient:    fakeClient,
		ctx:           context.Background(),
		log:           logr.Discard(),
		scheme:        scheme,
		eventRecorder: record.NewFakeRecorder(100),
	}
}

// TestReconcileCSIDriver tests the reconcileCSIDriver function
func TestReconcileCSIDriver(t *testing.T) {
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
				Spec:       v1alpha1.SpiffeCSIDriverSpec{PluginName: "csi.spiffe.io"},
			},
			setupClient: func(fc *fakes.FakeCustomCtrlClient) {
				fc.GetReturns(kerrors.NewNotFound(schema.GroupResource{}, "csi.spiffe.io"))
				fc.CreateReturns(nil)
			},
			expectCreate: true,
		},
		{
			name: "create error",
			driver: &v1alpha1.SpiffeCSIDriver{
				ObjectMeta: metav1.ObjectMeta{Name: "cluster", UID: "test-uid"},
				Spec:       v1alpha1.SpiffeCSIDriverSpec{PluginName: "csi.spiffe.io"},
			},
			setupClient: func(fc *fakes.FakeCustomCtrlClient) {
				fc.GetReturns(kerrors.NewNotFound(schema.GroupResource{}, "csi.spiffe.io"))
				fc.CreateReturns(errors.New("create failed"))
			},
			expectError: true,
		},
		{
			name: "get error",
			driver: &v1alpha1.SpiffeCSIDriver{
				ObjectMeta: metav1.ObjectMeta{Name: "cluster", UID: "test-uid"},
				Spec:       v1alpha1.SpiffeCSIDriverSpec{PluginName: "csi.spiffe.io"},
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
				Spec:       v1alpha1.SpiffeCSIDriverSpec{PluginName: "csi.spiffe.io"},
			},
			setupClient: func(fc *fakes.FakeCustomCtrlClient) {
				existingCSI := &storagev1.CSIDriver{
					ObjectMeta: metav1.ObjectMeta{Name: "csi.spiffe.io", ResourceVersion: "123"},
				}
				fc.GetStub = func(ctx context.Context, key client.ObjectKey, obj client.Object) error {
					if csi, ok := obj.(*storagev1.CSIDriver); ok {
						*csi = *existingCSI
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
					PluginName: "csi.spiffe.io",
					CommonConfig: v1alpha1.CommonConfig{
						Labels: map[string]string{"new-label": "new-value"},
					},
				},
			},
			setupClient: func(fc *fakes.FakeCustomCtrlClient) {
				existingCSI := &storagev1.CSIDriver{
					ObjectMeta: metav1.ObjectMeta{
						Name:            "csi.spiffe.io",
						ResourceVersion: "123",
						Labels:          map[string]string{"old-label": "old-value"},
					},
				}
				fc.GetStub = func(ctx context.Context, key client.ObjectKey, obj client.Object) error {
					if csi, ok := obj.(*storagev1.CSIDriver); ok {
						*csi = *existingCSI
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
					PluginName: "csi.spiffe.io",
					CommonConfig: v1alpha1.CommonConfig{
						Labels: map[string]string{"new-label": "new-value"},
					},
				},
			},
			setupClient: func(fc *fakes.FakeCustomCtrlClient) {
				existingCSI := &storagev1.CSIDriver{
					ObjectMeta: metav1.ObjectMeta{
						Name:            "csi.spiffe.io",
						ResourceVersion: "123",
						Labels:          map[string]string{"old-label": "old-value"},
					},
				}
				fc.GetStub = func(ctx context.Context, key client.ObjectKey, obj client.Object) error {
					if csi, ok := obj.(*storagev1.CSIDriver); ok {
						*csi = *existingCSI
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
				Spec:       v1alpha1.SpiffeCSIDriverSpec{PluginName: "csi.spiffe.io"},
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
				reconciler = newCSITestReconciler(fakeClient)
			}
			tt.setupClient(fakeClient)

			statusMgr := status.NewManager(fakeClient)
			err := reconciler.reconcileCSIDriver(context.Background(), tt.driver, statusMgr, tt.createOnlyMode)

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
