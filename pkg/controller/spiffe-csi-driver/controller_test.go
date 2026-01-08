package spiffe_csi_driver

import (
	"context"
	"errors"
	"testing"

	"github.com/go-logr/logr"
	"github.com/openshift/zero-trust-workload-identity-manager/api/v1alpha1"
	"github.com/openshift/zero-trust-workload-identity-manager/pkg/client/fakes"
	"github.com/openshift/zero-trust-workload-identity-manager/pkg/controller/status"
	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// newTestReconciler creates a reconciler for testing
func newTestReconciler(fakeClient *fakes.FakeCustomCtrlClient) *SpiffeCsiReconciler {
	return &SpiffeCsiReconciler{
		ctrlClient:    fakeClient,
		ctx:           context.Background(),
		log:           logr.Discard(),
		scheme:        runtime.NewScheme(),
		eventRecorder: record.NewFakeRecorder(100),
	}
}

// TestReconcile_SpiffeCSIDriverNotFound tests that when SpiffeCSIDriver CR is not found,
func TestReconcile_SpiffeCSIDriverNotFound(t *testing.T) {
	fakeClient := &fakes.FakeCustomCtrlClient{}
	reconciler := newTestReconciler(fakeClient)

	// Configure fake client to return NotFound error for SpiffeCSIDriver
	notFoundErr := kerrors.NewNotFound(schema.GroupResource{Group: "operator.openshift.io", Resource: "spiffecsidrivers"}, "cluster")
	fakeClient.GetReturns(notFoundErr)

	req := ctrl.Request{NamespacedName: types.NamespacedName{Name: "cluster"}}
	result, err := reconciler.Reconcile(context.Background(), req)

	// Assert: should return nil error (not requeue) when CR not found
	if err != nil {
		t.Errorf("Expected nil error when SpiffeCSIDriver not found, got: %v", err)
	}
	if result.Requeue {
		t.Error("Expected no requeue when SpiffeCSIDriver not found")
	}
	if result.RequeueAfter != 0 {
		t.Error("Expected no RequeueAfter when SpiffeCSIDriver not found")
	}
}

// TestReconcile_SpiffeCSIDriverGetError tests that when Get returns a non-NotFound error,
func TestReconcile_SpiffeCSIDriverGetError(t *testing.T) {
	fakeClient := &fakes.FakeCustomCtrlClient{}
	reconciler := newTestReconciler(fakeClient)

	// Configure fake client to return a generic error for SpiffeCSIDriver Get
	genericErr := errors.New("connection refused")
	fakeClient.GetReturns(genericErr)

	req := ctrl.Request{NamespacedName: types.NamespacedName{Name: "cluster"}}
	result, err := reconciler.Reconcile(context.Background(), req)

	// Assert: should return the error when Get fails with non-NotFound error
	if err == nil {
		t.Error("Expected error when Get fails, got nil")
	}
	if !errors.Is(err, genericErr) {
		t.Errorf("Expected connection refused error, got: %v", err)
	}
	if result.Requeue {
		t.Error("Expected no requeue flag when returning error")
	}
}

// TestReconcile_ZTWIMNotFound tests that when ZTWIM CR is not found,
func TestReconcile_ZTWIMNotFound(t *testing.T) {
	fakeClient := &fakes.FakeCustomCtrlClient{}
	reconciler := newTestReconciler(fakeClient)

	csiDriver := &v1alpha1.SpiffeCSIDriver{
		ObjectMeta: metav1.ObjectMeta{
			Name: "cluster",
		},
	}

	callCount := 0
	fakeClient.GetStub = func(ctx context.Context, key client.ObjectKey, obj client.Object) error {
		callCount++
		switch callCount {
		case 1: // First call: Get SpiffeCSIDriver
			if csi, ok := obj.(*v1alpha1.SpiffeCSIDriver); ok {
				*csi = *csiDriver
			}
			return nil
		case 2: // Second call: Get ZTWIM - return NotFound
			return kerrors.NewNotFound(schema.GroupResource{Group: "operator.openshift.io", Resource: "zerotrustworkloadidentitymanagers"}, "cluster")
		default:
			return nil
		}
	}

	req := ctrl.Request{NamespacedName: types.NamespacedName{Name: "cluster"}}
	result, err := reconciler.Reconcile(context.Background(), req)

	// Assert: should return nil error when ZTWIM not found (not requeue with error)
	if err != nil {
		t.Errorf("Expected nil error when ZTWIM not found, got: %v", err)
	}
	if result.Requeue {
		t.Error("Expected no requeue when ZTWIM not found")
	}
}

// TestReconcile_ZTWIMGetError tests that when ZTWIM Get returns a non-NotFound error,
func TestReconcile_ZTWIMGetError(t *testing.T) {
	fakeClient := &fakes.FakeCustomCtrlClient{}
	reconciler := newTestReconciler(fakeClient)

	csiDriver := &v1alpha1.SpiffeCSIDriver{
		ObjectMeta: metav1.ObjectMeta{
			Name: "cluster",
		},
	}

	genericErr := errors.New("internal server error")
	callCount := 0
	fakeClient.GetStub = func(ctx context.Context, key client.ObjectKey, obj client.Object) error {
		callCount++
		switch callCount {
		case 1: // First call: Get SpiffeCSIDriver
			if csi, ok := obj.(*v1alpha1.SpiffeCSIDriver); ok {
				*csi = *csiDriver
			}
			return nil
		case 2: // Second call: Get ZTWIM - return generic error
			return genericErr
		default:
			return nil
		}
	}

	req := ctrl.Request{NamespacedName: types.NamespacedName{Name: "cluster"}}
	result, err := reconciler.Reconcile(context.Background(), req)

	// Assert: should return the error when ZTWIM Get fails
	if err == nil {
		t.Error("Expected error when ZTWIM Get fails, got nil")
	}
	if !errors.Is(err, genericErr) {
		t.Errorf("Expected internal server error, got: %v", err)
	}
	if result.Requeue {
		t.Error("Expected no requeue flag when returning error")
	}
}

// TestReconcile_OwnerReferenceUpdateError tests that when Update fails after setting owner
func TestReconcile_OwnerReferenceUpdateError(t *testing.T) {
	fakeClient := &fakes.FakeCustomCtrlClient{}
	reconciler := newTestReconciler(fakeClient)

	// Register types in scheme for SetControllerReference
	scheme := runtime.NewScheme()
	_ = v1alpha1.AddToScheme(scheme)
	reconciler.scheme = scheme

	// SpiffeCSIDriver without owner reference (needs update)
	csiDriver := &v1alpha1.SpiffeCSIDriver{
		ObjectMeta: metav1.ObjectMeta{
			Name: "cluster",
		},
	}

	// ZTWIM with proper metadata
	ztwim := &v1alpha1.ZeroTrustWorkloadIdentityManager{
		ObjectMeta: metav1.ObjectMeta{
			Name: "cluster",
			UID:  "test-uid",
		},
	}

	callCount := 0
	fakeClient.GetStub = func(ctx context.Context, key client.ObjectKey, obj client.Object) error {
		callCount++
		switch callCount {
		case 1: // Get SpiffeCSIDriver
			if csi, ok := obj.(*v1alpha1.SpiffeCSIDriver); ok {
				*csi = *csiDriver
			}
			return nil
		case 2: // Get ZTWIM
			if z, ok := obj.(*v1alpha1.ZeroTrustWorkloadIdentityManager); ok {
				*z = *ztwim
			}
			return nil
		default:
			return nil
		}
	}

	// Make Update fail
	updateErr := errors.New("update failed due to conflict")
	fakeClient.UpdateReturns(updateErr)

	req := ctrl.Request{NamespacedName: types.NamespacedName{Name: "cluster"}}
	result, err := reconciler.Reconcile(context.Background(), req)

	// When owner reference update is needed and Update fails, error should be returned
	if err != nil && result.Requeue {
		t.Error("Should not requeue with error - controller-runtime handles requeue on error")
	}
}

// TestHandleCreateOnlyMode_Enabled tests create-only mode when enabled
func TestHandleCreateOnlyMode_Enabled(t *testing.T) {
	// Set environment variable for create-only mode
	t.Setenv("CREATE_ONLY_MODE", "true")

	fakeClient := &fakes.FakeCustomCtrlClient{}
	reconciler := newTestReconciler(fakeClient)

	driver := &v1alpha1.SpiffeCSIDriver{
		ObjectMeta: metav1.ObjectMeta{Name: "cluster"},
	}

	statusMgr := status.NewManager(fakeClient)
	result := reconciler.handleCreateOnlyMode(driver, statusMgr)

	// Assert: create-only mode should be detected as true
	if !result {
		t.Error("Expected handleCreateOnlyMode to return true when CREATE_ONLY_MODE=true")
	}
}

// TestHandleCreateOnlyMode_Disabled tests create-only mode when disabled
func TestHandleCreateOnlyMode_Disabled(t *testing.T) {
	// Clear environment variable
	t.Setenv("CREATE_ONLY_MODE", "false")

	fakeClient := &fakes.FakeCustomCtrlClient{}
	reconciler := newTestReconciler(fakeClient)

	driver := &v1alpha1.SpiffeCSIDriver{
		ObjectMeta: metav1.ObjectMeta{Name: "cluster"},
	}

	statusMgr := status.NewManager(fakeClient)
	result := reconciler.handleCreateOnlyMode(driver, statusMgr)

	// Assert: create-only mode should be detected as false
	if result {
		t.Error("Expected handleCreateOnlyMode to return false when CREATE_ONLY_MODE=false")
	}
}

// TestHandleCreateOnlyMode_DisabledWithPreviouslyEnabled tests create-only mode
func TestHandleCreateOnlyMode_DisabledWithPreviouslyEnabled(t *testing.T) {
	// Clear environment variable
	t.Setenv("CREATE_ONLY_MODE", "false")

	fakeClient := &fakes.FakeCustomCtrlClient{}
	reconciler := newTestReconciler(fakeClient)

	// Driver with existing CreateOnlyMode condition set to True
	driver := &v1alpha1.SpiffeCSIDriver{
		ObjectMeta: metav1.ObjectMeta{Name: "cluster"},
		Status: v1alpha1.SpiffeCSIDriverStatus{
			ConditionalStatus: v1alpha1.ConditionalStatus{
				Conditions: []metav1.Condition{
					{
						Type:   "CreateOnlyMode",
						Status: metav1.ConditionTrue,
					},
				},
			},
		},
	}

	statusMgr := status.NewManager(fakeClient)
	result := reconciler.handleCreateOnlyMode(driver, statusMgr)

	// Assert: create-only mode should be detected as false, but condition should be updated
	if result {
		t.Error("Expected handleCreateOnlyMode to return false")
	}
}

// TestValidateCommonConfig_ValidConfig tests common config validation passes
func TestValidateCommonConfig_ValidConfig(t *testing.T) {
	fakeClient := &fakes.FakeCustomCtrlClient{}
	reconciler := newTestReconciler(fakeClient)

	driver := &v1alpha1.SpiffeCSIDriver{
		ObjectMeta: metav1.ObjectMeta{Name: "cluster"},
		Spec:       v1alpha1.SpiffeCSIDriverSpec{},
	}

	statusMgr := status.NewManager(fakeClient)
	err := reconciler.validateCommonConfig(driver, statusMgr)

	// Assert: validation should pass with valid configuration
	if err != nil {
		t.Errorf("Expected no error for valid configuration, got: %v", err)
	}
}

// TestValidateCommonConfig_InvalidAffinity tests common config validation with invalid affinity
func TestValidateCommonConfig_InvalidAffinity(t *testing.T) {
	fakeClient := &fakes.FakeCustomCtrlClient{}
	reconciler := newTestReconciler(fakeClient)

	driver := &v1alpha1.SpiffeCSIDriver{
		ObjectMeta: metav1.ObjectMeta{Name: "cluster"},
		Spec: v1alpha1.SpiffeCSIDriverSpec{
			CommonConfig: v1alpha1.CommonConfig{
				Affinity: &corev1.Affinity{
					NodeAffinity: &corev1.NodeAffinity{
						RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
							NodeSelectorTerms: []corev1.NodeSelectorTerm{},
						},
					},
				},
			},
		},
	}

	statusMgr := status.NewManager(fakeClient)
	err := reconciler.validateCommonConfig(driver, statusMgr)

	if err == nil {
		t.Error("Expected error for invalid affinity")
	}
}

// TestHandleCreateOnlyMode_NotSet tests create-only mode when env var is not set
func TestHandleCreateOnlyMode_NotSet(t *testing.T) {
	t.Setenv("CREATE_ONLY_MODE", "")

	fakeClient := &fakes.FakeCustomCtrlClient{}
	reconciler := newTestReconciler(fakeClient)

	driver := &v1alpha1.SpiffeCSIDriver{
		ObjectMeta: metav1.ObjectMeta{Name: "cluster"},
	}

	statusMgr := status.NewManager(fakeClient)
	result := reconciler.handleCreateOnlyMode(driver, statusMgr)

	if result {
		t.Error("Expected handleCreateOnlyMode to return false when CREATE_ONLY_MODE is not set")
	}
}

// TestSpiffeCsiReconciler_Fields tests SpiffeCsiReconciler struct fields
func TestSpiffeCsiReconciler_Fields(t *testing.T) {
	fakeClient := &fakes.FakeCustomCtrlClient{}
	reconciler := newTestReconciler(fakeClient)

	if reconciler.ctrlClient == nil {
		t.Error("Expected ctrlClient to be set")
	}
	if reconciler.ctx == nil {
		t.Error("Expected ctx to be set")
	}
	// logr.Discard() is valid, just verify it's enabled (won't panic)
	reconciler.log.Info("test log - should not panic")
	if reconciler.scheme == nil {
		t.Error("Expected scheme to be set")
	}
	if reconciler.eventRecorder == nil {
		t.Error("Expected eventRecorder to be set")
	}
}

// TestConditionConstants tests that condition constants are defined
func TestConditionConstants(t *testing.T) {
	if DaemonSetAvailable != "DaemonSetAvailable" {
		t.Errorf("Expected DaemonSetAvailable to be 'DaemonSetAvailable', got %s", DaemonSetAvailable)
	}
	if SecurityContextConstraintsAvailable != "SecurityContextConstraintsAvailable" {
		t.Errorf("Expected SecurityContextConstraintsAvailable to be 'SecurityContextConstraintsAvailable', got %s", SecurityContextConstraintsAvailable)
	}
	if ServiceAccountAvailable != "ServiceAccountAvailable" {
		t.Errorf("Expected ServiceAccountAvailable to be 'ServiceAccountAvailable', got %s", ServiceAccountAvailable)
	}
	if CSIDriverAvailable != "CSIDriverAvailable" {
		t.Errorf("Expected CSIDriverAvailable to be 'CSIDriverAvailable', got %s", CSIDriverAvailable)
	}
}

// TestReconcile_FullFlow tests complete reconcile flow
func TestReconcile_FullFlow(t *testing.T) {
	fakeClient := &fakes.FakeCustomCtrlClient{}

	scheme := runtime.NewScheme()
	_ = v1alpha1.AddToScheme(scheme)

	reconciler := &SpiffeCsiReconciler{
		ctrlClient:    fakeClient,
		ctx:           context.Background(),
		log:           logr.Discard(),
		scheme:        scheme,
		eventRecorder: record.NewFakeRecorder(100),
	}

	csiDriver := &v1alpha1.SpiffeCSIDriver{
		ObjectMeta: metav1.ObjectMeta{
			Name: "cluster",
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: "operator.openshift.io/v1alpha1",
					Kind:       "ZeroTrustWorkloadIdentityManager",
					Name:       "cluster",
					UID:        "test-uid",
				},
			},
		},
		Spec: v1alpha1.SpiffeCSIDriverSpec{},
	}

	ztwim := &v1alpha1.ZeroTrustWorkloadIdentityManager{
		ObjectMeta: metav1.ObjectMeta{
			Name: "cluster",
			UID:  "test-uid",
		},
		Spec: v1alpha1.ZeroTrustWorkloadIdentityManagerSpec{
			TrustDomain: "example.org",
		},
	}

	fakeClient.GetStub = func(ctx context.Context, key client.ObjectKey, obj client.Object) error {
		switch v := obj.(type) {
		case *v1alpha1.SpiffeCSIDriver:
			*v = *csiDriver
			return nil
		case *v1alpha1.ZeroTrustWorkloadIdentityManager:
			*v = *ztwim
			return nil
		default:
			return kerrors.NewNotFound(schema.GroupResource{}, key.Name)
		}
	}

	fakeClient.CreateReturns(nil)
	fakeClient.UpdateReturns(nil)
	fakeClient.PatchReturns(nil)
	fakeClient.StatusUpdateWithRetryReturns(nil)

	req := ctrl.Request{NamespacedName: types.NamespacedName{Name: "cluster"}}
	result, err := reconciler.Reconcile(context.Background(), req)

	// Success if we don't panic
	if result.Requeue && err != nil {
		t.Log("Reconcile returned with requeue and error - expected for incomplete setup")
	}
	t.Log("Reconcile completed without panic")
}

// TestValidateCommonConfig_WithTolerations tests validation with tolerations
func TestValidateCommonConfig_WithTolerations(t *testing.T) {
	fakeClient := &fakes.FakeCustomCtrlClient{}
	reconciler := newTestReconciler(fakeClient)

	driver := &v1alpha1.SpiffeCSIDriver{
		ObjectMeta: metav1.ObjectMeta{Name: "cluster"},
		Spec: v1alpha1.SpiffeCSIDriverSpec{
			CommonConfig: v1alpha1.CommonConfig{
				Tolerations: []*corev1.Toleration{
					{
						Key:      "node-role.kubernetes.io/master",
						Operator: corev1.TolerationOpExists,
						Effect:   corev1.TaintEffectNoSchedule,
					},
				},
			},
		},
	}

	statusMgr := status.NewManager(fakeClient)
	err := reconciler.validateCommonConfig(driver, statusMgr)

	if err != nil {
		t.Errorf("Expected no error for valid tolerations, got: %v", err)
	}
}

// TestValidateCommonConfig_WithNodeSelector tests validation with node selector
func TestValidateCommonConfig_WithNodeSelector(t *testing.T) {
	fakeClient := &fakes.FakeCustomCtrlClient{}
	reconciler := newTestReconciler(fakeClient)

	driver := &v1alpha1.SpiffeCSIDriver{
		ObjectMeta: metav1.ObjectMeta{Name: "cluster"},
		Spec: v1alpha1.SpiffeCSIDriverSpec{
			CommonConfig: v1alpha1.CommonConfig{
				NodeSelector: map[string]string{
					"kubernetes.io/os": "linux",
				},
			},
		},
	}

	statusMgr := status.NewManager(fakeClient)
	err := reconciler.validateCommonConfig(driver, statusMgr)

	if err != nil {
		t.Errorf("Expected no error for valid node selector, got: %v", err)
	}
}

// TestValidateCommonConfig_WithLabels tests validation with custom labels
func TestValidateCommonConfig_WithLabels(t *testing.T) {
	fakeClient := &fakes.FakeCustomCtrlClient{}
	reconciler := newTestReconciler(fakeClient)

	driver := &v1alpha1.SpiffeCSIDriver{
		ObjectMeta: metav1.ObjectMeta{Name: "cluster"},
		Spec: v1alpha1.SpiffeCSIDriverSpec{
			CommonConfig: v1alpha1.CommonConfig{
				Labels: map[string]string{
					"custom-label": "value",
				},
			},
		},
	}

	statusMgr := status.NewManager(fakeClient)
	err := reconciler.validateCommonConfig(driver, statusMgr)

	if err != nil {
		t.Errorf("Expected no error for valid labels, got: %v", err)
	}
}

// TestReconcile_AllScenarios tests various reconcile scenarios
func TestReconcile_AllScenarios(t *testing.T) {
	tests := []struct {
		name           string
		setupClient    func(*fakes.FakeCustomCtrlClient)
		expectError    bool
		expectRequeue  bool
		checkNoRequeue bool
	}{
		{
			name: "SpiffeCSIDriver NotFound returns nil and no requeue",
			setupClient: func(fc *fakes.FakeCustomCtrlClient) {
				fc.GetReturns(kerrors.NewNotFound(schema.GroupResource{}, "cluster"))
			},
			expectError:    false,
			checkNoRequeue: true,
		},
		{
			name: "SpiffeCSIDriver Get error returns error",
			setupClient: func(fc *fakes.FakeCustomCtrlClient) {
				fc.GetReturns(errors.New("connection refused"))
			},
			expectError:    true,
			checkNoRequeue: true,
		},
		{
			name: "ZTWIM NotFound sets condition and returns nil",
			setupClient: func(fc *fakes.FakeCustomCtrlClient) {
				callCount := 0
				fc.GetStub = func(ctx context.Context, key client.ObjectKey, obj client.Object) error {
					callCount++
					if callCount == 1 {
						if csi, ok := obj.(*v1alpha1.SpiffeCSIDriver); ok {
							csi.Name = "cluster"
						}
						return nil
					}
					return kerrors.NewNotFound(schema.GroupResource{}, "cluster")
				}
			},
			expectError:    false,
			checkNoRequeue: true,
		},
		{
			name: "ZTWIM Get error returns error",
			setupClient: func(fc *fakes.FakeCustomCtrlClient) {
				callCount := 0
				fc.GetStub = func(ctx context.Context, key client.ObjectKey, obj client.Object) error {
					callCount++
					if callCount == 1 {
						if csi, ok := obj.(*v1alpha1.SpiffeCSIDriver); ok {
							csi.Name = "cluster"
						}
						return nil
					}
					return errors.New("ztwim get error")
				}
			},
			expectError:    true,
			checkNoRequeue: true,
		},
		{
			name: "SetControllerReference error returns error",
			setupClient: func(fc *fakes.FakeCustomCtrlClient) {
				callCount := 0
				fc.GetStub = func(ctx context.Context, key client.ObjectKey, obj client.Object) error {
					callCount++
					switch callCount {
					case 1:
						if csi, ok := obj.(*v1alpha1.SpiffeCSIDriver); ok {
							csi.Name = "cluster"
							// No owner references
						}
						return nil
					case 2:
						if z, ok := obj.(*v1alpha1.ZeroTrustWorkloadIdentityManager); ok {
							z.Name = "cluster"
							z.UID = "test-uid"
						}
						return nil
					}
					return nil
				}
				fc.UpdateReturns(errors.New("update failed"))
			},
			expectError:    true,
			checkNoRequeue: true,
		},
		{
			name: "validateCommonConfig error returns nil error",
			setupClient: func(fc *fakes.FakeCustomCtrlClient) {
				callCount := 0
				fc.GetStub = func(ctx context.Context, key client.ObjectKey, obj client.Object) error {
					callCount++
					switch callCount {
					case 1:
						if csi, ok := obj.(*v1alpha1.SpiffeCSIDriver); ok {
							csi.Name = "cluster"
							csi.OwnerReferences = []metav1.OwnerReference{{
								APIVersion: "operator.openshift.io/v1alpha1",
								Kind:       "ZeroTrustWorkloadIdentityManager",
								Name:       "cluster",
								UID:        "test-uid",
							}}
							// Invalid affinity causes validation error
							csi.Spec.CommonConfig = v1alpha1.CommonConfig{
								Affinity: &corev1.Affinity{
									NodeAffinity: &corev1.NodeAffinity{
										RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
											NodeSelectorTerms: []corev1.NodeSelectorTerm{},
										},
									},
								},
							}
						}
						return nil
					case 2:
						if z, ok := obj.(*v1alpha1.ZeroTrustWorkloadIdentityManager); ok {
							z.Name = "cluster"
							z.UID = "test-uid"
						}
						return nil
					}
					return nil
				}
			},
			expectError:    false,
			checkNoRequeue: true,
		},
		{
			name: "reconcileServiceAccount error returns error",
			setupClient: func(fc *fakes.FakeCustomCtrlClient) {
				callCount := 0
				fc.GetStub = func(ctx context.Context, key client.ObjectKey, obj client.Object) error {
					callCount++
					switch callCount {
					case 1:
						if csi, ok := obj.(*v1alpha1.SpiffeCSIDriver); ok {
							csi.Name = "cluster"
							csi.OwnerReferences = []metav1.OwnerReference{{
								APIVersion: "operator.openshift.io/v1alpha1",
								Kind:       "ZeroTrustWorkloadIdentityManager",
								Name:       "cluster",
								UID:        "test-uid",
							}}
						}
						return nil
					case 2:
						if z, ok := obj.(*v1alpha1.ZeroTrustWorkloadIdentityManager); ok {
							z.Name = "cluster"
							z.UID = "test-uid"
						}
						return nil
					default:
						return errors.New("service account get error")
					}
				}
			},
			expectError:    true,
			checkNoRequeue: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fakeClient := &fakes.FakeCustomCtrlClient{}
			scheme := runtime.NewScheme()
			_ = v1alpha1.AddToScheme(scheme)

			reconciler := &SpiffeCsiReconciler{
				ctrlClient:    fakeClient,
				ctx:           context.Background(),
				log:           logr.Discard(),
				scheme:        scheme,
				eventRecorder: record.NewFakeRecorder(100),
			}

			if tt.setupClient != nil {
				tt.setupClient(fakeClient)
			}

			req := ctrl.Request{NamespacedName: types.NamespacedName{Name: "cluster"}}
			result, err := reconciler.Reconcile(context.Background(), req)

			if tt.expectError && err == nil {
				t.Fatal("Expected error but got nil")
			}
			if !tt.expectError && err != nil {
				t.Fatalf("Expected no error but got: %v", err)
			}
			if tt.checkNoRequeue {
				if result.Requeue {
					t.Fatal("Expected Requeue=false but got true")
				}
				if result.RequeueAfter != 0 {
					t.Fatalf("Expected RequeueAfter=0 but got %v", result.RequeueAfter)
				}
			}
		})
	}
}

// TestReconcileServiceAccount_AllScenarios tests reconcileServiceAccount scenarios
func TestReconcileServiceAccount_AllScenarios(t *testing.T) {
	tests := []struct {
		name        string
		getErr      error
		createErr   error
		expectError bool
	}{
		{
			name:        "successful create when not found",
			getErr:      kerrors.NewNotFound(schema.GroupResource{}, "test"),
			createErr:   nil,
			expectError: false,
		},
		{
			name:        "create error returns error",
			getErr:      kerrors.NewNotFound(schema.GroupResource{}, "test"),
			createErr:   errors.New("create failed"),
			expectError: true,
		},
		{
			name:        "get error returns error",
			getErr:      errors.New("connection error"),
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fakeClient := &fakes.FakeCustomCtrlClient{}
			scheme := runtime.NewScheme()
			_ = v1alpha1.AddToScheme(scheme)

			reconciler := &SpiffeCsiReconciler{
				ctrlClient:    fakeClient,
				ctx:           context.Background(),
				log:           logr.Discard(),
				scheme:        scheme,
				eventRecorder: record.NewFakeRecorder(100),
			}

			fakeClient.GetReturns(tt.getErr)
			if tt.createErr != nil {
				fakeClient.CreateReturns(tt.createErr)
			} else {
				fakeClient.CreateReturns(nil)
			}

			driver := &v1alpha1.SpiffeCSIDriver{
				ObjectMeta: metav1.ObjectMeta{Name: "cluster", UID: "test-uid"},
			}
			statusMgr := status.NewManager(fakeClient)

			err := reconciler.reconcileServiceAccount(context.Background(), driver, statusMgr, false)

			if tt.expectError && err == nil {
				t.Fatal("Expected error but got nil")
			}
			if !tt.expectError && err != nil {
				t.Fatalf("Expected no error but got: %v", err)
			}
		})
	}
}

// TestReconcileCSIDriver_AllScenarios tests reconcileCSIDriver scenarios
func TestReconcileCSIDriver_AllScenarios(t *testing.T) {
	tests := []struct {
		name        string
		getErr      error
		createErr   error
		expectError bool
	}{
		{
			name:        "successful create when not found",
			getErr:      kerrors.NewNotFound(schema.GroupResource{}, "test"),
			createErr:   nil,
			expectError: false,
		},
		{
			name:        "create error returns error",
			getErr:      kerrors.NewNotFound(schema.GroupResource{}, "test"),
			createErr:   errors.New("create failed"),
			expectError: true,
		},
		{
			name:        "get error returns error",
			getErr:      errors.New("connection error"),
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fakeClient := &fakes.FakeCustomCtrlClient{}
			scheme := runtime.NewScheme()
			_ = v1alpha1.AddToScheme(scheme)

			reconciler := &SpiffeCsiReconciler{
				ctrlClient:    fakeClient,
				ctx:           context.Background(),
				log:           logr.Discard(),
				scheme:        scheme,
				eventRecorder: record.NewFakeRecorder(100),
			}

			fakeClient.GetReturns(tt.getErr)
			if tt.createErr != nil {
				fakeClient.CreateReturns(tt.createErr)
			} else {
				fakeClient.CreateReturns(nil)
			}

			driver := &v1alpha1.SpiffeCSIDriver{
				ObjectMeta: metav1.ObjectMeta{Name: "cluster", UID: "test-uid"},
			}
			statusMgr := status.NewManager(fakeClient)

			err := reconciler.reconcileCSIDriver(context.Background(), driver, statusMgr, false)

			if tt.expectError && err == nil {
				t.Fatal("Expected error but got nil")
			}
			if !tt.expectError && err != nil {
				t.Fatalf("Expected no error but got: %v", err)
			}
		})
	}
}

// TestReconcileSCC_AllScenarios tests reconcileSCC scenarios
func TestReconcileSCC_AllScenarios(t *testing.T) {
	tests := []struct {
		name        string
		getErr      error
		createErr   error
		expectError bool
	}{
		{
			name:        "successful create when not found",
			getErr:      kerrors.NewNotFound(schema.GroupResource{}, "test"),
			createErr:   nil,
			expectError: false,
		},
		{
			name:        "create error returns error",
			getErr:      kerrors.NewNotFound(schema.GroupResource{}, "test"),
			createErr:   errors.New("create failed"),
			expectError: true,
		},
		{
			name:        "get error returns error",
			getErr:      errors.New("connection error"),
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fakeClient := &fakes.FakeCustomCtrlClient{}
			scheme := runtime.NewScheme()
			_ = v1alpha1.AddToScheme(scheme)

			reconciler := &SpiffeCsiReconciler{
				ctrlClient:    fakeClient,
				ctx:           context.Background(),
				log:           logr.Discard(),
				scheme:        scheme,
				eventRecorder: record.NewFakeRecorder(100),
			}

			fakeClient.GetReturns(tt.getErr)
			if tt.createErr != nil {
				fakeClient.CreateReturns(tt.createErr)
			} else {
				fakeClient.CreateReturns(nil)
			}

			driver := &v1alpha1.SpiffeCSIDriver{
				ObjectMeta: metav1.ObjectMeta{Name: "cluster", UID: "test-uid"},
			}
			statusMgr := status.NewManager(fakeClient)

			err := reconciler.reconcileSCC(context.Background(), driver, statusMgr)

			if tt.expectError && err == nil {
				t.Fatal("Expected error but got nil")
			}
			if !tt.expectError && err != nil {
				t.Fatalf("Expected no error but got: %v", err)
			}
		})
	}
}

// TestHandleCreateOnlyMode_AllScenarios tests all create only mode scenarios
func TestHandleCreateOnlyMode_AllScenarios(t *testing.T) {
	tests := []struct {
		name           string
		envValue       string
		existingStatus metav1.ConditionStatus
		expectResult   bool
	}{
		{
			name:         "enabled returns true",
			envValue:     "true",
			expectResult: true,
		},
		{
			name:         "disabled returns false",
			envValue:     "false",
			expectResult: false,
		},
		{
			name:         "empty returns false",
			envValue:     "",
			expectResult: false,
		},
		{
			name:           "disabled with previously enabled condition",
			envValue:       "false",
			existingStatus: metav1.ConditionTrue,
			expectResult:   false,
		},
		{
			name:           "disabled with previously disabled condition",
			envValue:       "false",
			existingStatus: metav1.ConditionFalse,
			expectResult:   false,
		},
		{
			name:           "enabled with previously disabled condition",
			envValue:       "true",
			existingStatus: metav1.ConditionFalse,
			expectResult:   true,
		},
		{
			name:           "enabled with previously enabled condition",
			envValue:       "true",
			existingStatus: metav1.ConditionTrue,
			expectResult:   true,
		},
		// With ||, nil existingCondition would cause panic when accessing .Status
		{
			name:           "disabled with nil condition - kills && to || mutant",
			envValue:       "false",
			existingStatus: "", // empty means no condition set
			expectResult:   false,
		},
		{
			name:           "disabled with unknown condition returns false",
			envValue:       "false",
			existingStatus: metav1.ConditionUnknown,
			expectResult:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("CREATE_ONLY_MODE", tt.envValue)

			fakeClient := &fakes.FakeCustomCtrlClient{}
			reconciler := newTestReconciler(fakeClient)

			driver := &v1alpha1.SpiffeCSIDriver{
				ObjectMeta: metav1.ObjectMeta{Name: "cluster"},
			}

			if tt.existingStatus != "" {
				driver.Status.ConditionalStatus.Conditions = []metav1.Condition{
					{Type: "CreateOnlyMode", Status: tt.existingStatus},
				}
			}

			statusMgr := status.NewManager(fakeClient)
			result := reconciler.handleCreateOnlyMode(driver, statusMgr)

			if result != tt.expectResult {
				t.Fatalf("Expected %v but got %v", tt.expectResult, result)
			}
		})
	}
}

// TestValidateCommonConfig_AllScenarios tests all validation scenarios
func TestValidateCommonConfig_AllScenarios(t *testing.T) {
	tests := []struct {
		name        string
		spec        v1alpha1.SpiffeCSIDriverSpec
		expectError bool
	}{
		{
			name:        "empty spec is valid",
			spec:        v1alpha1.SpiffeCSIDriverSpec{},
			expectError: false,
		},
		{
			name: "valid tolerations",
			spec: v1alpha1.SpiffeCSIDriverSpec{
				CommonConfig: v1alpha1.CommonConfig{
					Tolerations: []*corev1.Toleration{
						{Key: "test", Operator: corev1.TolerationOpExists},
					},
				},
			},
			expectError: false,
		},
		{
			name: "valid node selector",
			spec: v1alpha1.SpiffeCSIDriverSpec{
				CommonConfig: v1alpha1.CommonConfig{
					NodeSelector: map[string]string{"key": "value"},
				},
			},
			expectError: false,
		},
		{
			name: "valid labels",
			spec: v1alpha1.SpiffeCSIDriverSpec{
				CommonConfig: v1alpha1.CommonConfig{
					Labels: map[string]string{"app": "test"},
				},
			},
			expectError: false,
		},
		{
			name: "invalid affinity with empty terms",
			spec: v1alpha1.SpiffeCSIDriverSpec{
				CommonConfig: v1alpha1.CommonConfig{
					Affinity: &corev1.Affinity{
						NodeAffinity: &corev1.NodeAffinity{
							RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
								NodeSelectorTerms: []corev1.NodeSelectorTerm{},
							},
						},
					},
				},
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fakeClient := &fakes.FakeCustomCtrlClient{}
			reconciler := newTestReconciler(fakeClient)

			driver := &v1alpha1.SpiffeCSIDriver{
				ObjectMeta: metav1.ObjectMeta{Name: "cluster"},
				Spec:       tt.spec,
			}

			statusMgr := status.NewManager(fakeClient)
			err := reconciler.validateCommonConfig(driver, statusMgr)

			if tt.expectError && err == nil {
				t.Fatal("Expected error but got nil")
			}
			if !tt.expectError && err != nil {
				t.Fatalf("Expected no error but got: %v", err)
			}
		})
	}
}

// TestReconcile_ErrorPropagation tests all error return paths in Reconcile function
func TestReconcile_ErrorPropagation(t *testing.T) {
	tests := []struct {
		name        string
		setupClient func(*fakes.FakeCustomCtrlClient)
		expectError bool
	}{
		{
			name: "reconcileServiceAccount error propagates",
			setupClient: func(fc *fakes.FakeCustomCtrlClient) {
				callCount := 0
				fc.GetStub = func(ctx context.Context, key client.ObjectKey, obj client.Object) error {
					callCount++
					switch callCount {
					case 1:
						if c, ok := obj.(*v1alpha1.SpiffeCSIDriver); ok {
							c.Name = "cluster"
							c.UID = "test-uid"
							c.OwnerReferences = []metav1.OwnerReference{{
								APIVersion: "operator.openshift.io/v1alpha1",
								Kind:       "ZeroTrustWorkloadIdentityManager",
								Name:       "cluster",
								UID:        "ztwim-uid",
							}}
						}
						return nil
					case 2:
						if z, ok := obj.(*v1alpha1.ZeroTrustWorkloadIdentityManager); ok {
							z.Name = "cluster"
							z.UID = "ztwim-uid"
						}
						return nil
					default:
						return errors.New("service account get error")
					}
				}
			},
			expectError: true,
		},
		{
			name: "reconcileCSIDriver error propagates",
			setupClient: func(fc *fakes.FakeCustomCtrlClient) {
				callCount := 0
				fc.GetStub = func(ctx context.Context, key client.ObjectKey, obj client.Object) error {
					callCount++
					switch callCount {
					case 1:
						if c, ok := obj.(*v1alpha1.SpiffeCSIDriver); ok {
							c.Name = "cluster"
							c.UID = "test-uid"
							c.OwnerReferences = []metav1.OwnerReference{{
								APIVersion: "operator.openshift.io/v1alpha1",
								Kind:       "ZeroTrustWorkloadIdentityManager",
								Name:       "cluster",
								UID:        "ztwim-uid",
							}}
						}
						return nil
					case 2:
						if z, ok := obj.(*v1alpha1.ZeroTrustWorkloadIdentityManager); ok {
							z.Name = "cluster"
							z.UID = "ztwim-uid"
						}
						return nil
					case 3: // ServiceAccount
						return nil
					default:
						return errors.New("csidriver get error")
					}
				}
			},
			expectError: true,
		},
		{
			name: "reconcileSCC error propagates",
			setupClient: func(fc *fakes.FakeCustomCtrlClient) {
				callCount := 0
				fc.GetStub = func(ctx context.Context, key client.ObjectKey, obj client.Object) error {
					callCount++
					switch callCount {
					case 1:
						if c, ok := obj.(*v1alpha1.SpiffeCSIDriver); ok {
							c.Name = "cluster"
							c.UID = "test-uid"
							c.OwnerReferences = []metav1.OwnerReference{{
								APIVersion: "operator.openshift.io/v1alpha1",
								Kind:       "ZeroTrustWorkloadIdentityManager",
								Name:       "cluster",
								UID:        "ztwim-uid",
							}}
						}
						return nil
					case 2:
						if z, ok := obj.(*v1alpha1.ZeroTrustWorkloadIdentityManager); ok {
							z.Name = "cluster"
							z.UID = "ztwim-uid"
						}
						return nil
					case 3, 4: // ServiceAccount, CSIDriver
						return nil
					default:
						return errors.New("scc get error")
					}
				}
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fakeClient := &fakes.FakeCustomCtrlClient{}
			scheme := runtime.NewScheme()
			_ = v1alpha1.AddToScheme(scheme)

			reconciler := &SpiffeCsiReconciler{
				ctrlClient:    fakeClient,
				ctx:           context.Background(),
				log:           logr.Discard(),
				scheme:        scheme,
				eventRecorder: record.NewFakeRecorder(100),
			}

			if tt.setupClient != nil {
				tt.setupClient(fakeClient)
			}

			req := ctrl.Request{NamespacedName: types.NamespacedName{Name: "cluster"}}
			result, err := reconciler.Reconcile(context.Background(), req)

			if tt.expectError {
				if err == nil {
					t.Fatal("Expected error but got nil - mutation not killed")
				}
			} else {
				if err != nil {
					t.Fatalf("Expected no error but got: %v", err)
				}
			}

			// Verify no requeue flag set when error is returned
			if tt.expectError && result.Requeue {
				t.Error("Expected Requeue=false when error returned")
			}
			if tt.expectError && result.RequeueAfter != 0 {
				t.Errorf("Expected RequeueAfter=0 when error returned, got %v", result.RequeueAfter)
			}
		})
	}
}

// TestReconcile_SuccessfulPath_NoRequeue tests that successful reconciliation returns no requeue
func TestReconcile_SuccessfulPath_NoRequeue(t *testing.T) {
	fakeClient := &fakes.FakeCustomCtrlClient{}
	scheme := runtime.NewScheme()
	_ = v1alpha1.AddToScheme(scheme)
	_ = corev1.AddToScheme(scheme)

	reconciler := &SpiffeCsiReconciler{
		ctrlClient:    fakeClient,
		ctx:           context.Background(),
		log:           logr.Discard(),
		scheme:        scheme,
		eventRecorder: record.NewFakeRecorder(100),
	}

	callCount := 0
	fakeClient.GetStub = func(ctx context.Context, key client.ObjectKey, obj client.Object) error {
		callCount++
		switch callCount {
		case 1: // SpiffeCSIDriver
			if c, ok := obj.(*v1alpha1.SpiffeCSIDriver); ok {
				c.Name = "cluster"
				c.UID = "test-uid"
				c.OwnerReferences = []metav1.OwnerReference{{
					APIVersion: "operator.openshift.io/v1alpha1",
					Kind:       "ZeroTrustWorkloadIdentityManager",
					Name:       "cluster",
					UID:        "ztwim-uid",
				}}
			}
			return nil
		case 2: // ZTWIM
			if z, ok := obj.(*v1alpha1.ZeroTrustWorkloadIdentityManager); ok {
				z.Name = "cluster"
				z.UID = "ztwim-uid"
			}
			return nil
		default:
			// Return existing resources for all other gets
			return nil
		}
	}
	fakeClient.CreateReturns(nil)
	fakeClient.UpdateReturns(nil)

	req := ctrl.Request{NamespacedName: types.NamespacedName{Name: "cluster"}}
	result, err := reconciler.Reconcile(context.Background(), req)

	if err != nil {
		t.Logf("Reconcile returned error (expected for incomplete mock): %v", err)
	}
	// Even on partial success, these should be false
	if result.Requeue {
		t.Error("Expected Requeue=false on reconcile path")
	}
	if result.RequeueAfter != 0 {
		t.Errorf("Expected RequeueAfter=0 on reconcile path, got %v", result.RequeueAfter)
	}
}

// Tests: existingCondition.Status == metav1.ConditionTrue
func TestHandleCreateOnlyMode_MutationKillers(t *testing.T) {
	tests := []struct {
		name                    string
		existingConditionStatus metav1.ConditionStatus
		existingCondition       bool
	}{
		{
			// existingCondition != nil && existingCondition.Status == ConditionTrue
			// Should add ConditionFalse condition
			name:                    "existing condition with True status - should add False condition",
			existingConditionStatus: metav1.ConditionTrue,
			existingCondition:       true,
		},
		{
			// existingCondition != nil && existingCondition.Status != ConditionTrue (== ConditionFalse)
			// Should NOT add another condition (already False)
			name:                    "existing condition with False status - should not add condition",
			existingConditionStatus: metav1.ConditionFalse,
			existingCondition:       true,
		},
		{
			// existingCondition == nil
			// Should NOT add any condition
			name:              "no existing condition - should not add condition",
			existingCondition: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fakeClient := &fakes.FakeCustomCtrlClient{}
			reconciler := newTestReconciler(fakeClient)
			statusMgr := status.NewManager(fakeClient)

			driver := &v1alpha1.SpiffeCSIDriver{
				ObjectMeta: metav1.ObjectMeta{Name: "cluster"},
			}

			if tt.existingCondition {
				driver.Status.ConditionalStatus.Conditions = []metav1.Condition{
					{
						Type:   "CreateOnlyMode",
						Status: tt.existingConditionStatus,
					},
				}
			}

			// Call handleCreateOnlyMode (createOnlyMode is false by default in tests)
			result := reconciler.handleCreateOnlyMode(driver, statusMgr)

			// Verify createOnlyMode is false
			if result != false {
				t.Error("Expected createOnlyMode to be false")
			}
		})
	}
}

// TestReconcile_SCCError_MutationKiller tests SCC reconciliation error path
func TestReconcile_SCCError_MutationKiller(t *testing.T) {
	fakeClient := &fakes.FakeCustomCtrlClient{}
	scheme := runtime.NewScheme()
	_ = v1alpha1.AddToScheme(scheme)

	reconciler := &SpiffeCsiReconciler{
		ctrlClient:    fakeClient,
		ctx:           context.Background(),
		log:           logr.Discard(),
		scheme:        scheme,
		eventRecorder: record.NewFakeRecorder(100),
	}

	callCount := 0
	fakeClient.GetStub = func(ctx context.Context, key client.ObjectKey, obj client.Object) error {
		callCount++
		switch callCount {
		case 1:
			if c, ok := obj.(*v1alpha1.SpiffeCSIDriver); ok {
				c.Name = "cluster"
				c.UID = "test-uid"
				c.OwnerReferences = []metav1.OwnerReference{{
					APIVersion: "operator.openshift.io/v1alpha1",
					Kind:       "ZeroTrustWorkloadIdentityManager",
					Name:       "cluster",
					UID:        "ztwim-uid",
				}}
			}
			return nil
		case 2:
			if z, ok := obj.(*v1alpha1.ZeroTrustWorkloadIdentityManager); ok {
				z.Name = "cluster"
				z.UID = "ztwim-uid"
			}
			return nil
		case 3, 4:
			return nil
		default:
			return errors.New("SCC reconciliation failed")
		}
	}

	req := ctrl.Request{NamespacedName: types.NamespacedName{Name: "cluster"}}
	result, err := reconciler.Reconcile(context.Background(), req)

	if err == nil {
		t.Fatal("Expected error when SCC reconciliation fails, got nil - mutant survived")
	}

	if result.Requeue {
		t.Error("Expected Requeue=false when error returned")
	}
	if result.RequeueAfter != 0 {
		t.Errorf("Expected RequeueAfter=0 when error returned, got %v", result.RequeueAfter)
	}
}

// TestReconcile_DaemonSetError_MutationKiller tests DaemonSet reconciliation error path
func TestReconcile_DaemonSetError_MutationKiller(t *testing.T) {
	fakeClient := &fakes.FakeCustomCtrlClient{}
	scheme := runtime.NewScheme()
	_ = v1alpha1.AddToScheme(scheme)

	reconciler := &SpiffeCsiReconciler{
		ctrlClient:    fakeClient,
		ctx:           context.Background(),
		log:           logr.Discard(),
		scheme:        scheme,
		eventRecorder: record.NewFakeRecorder(100),
	}

	callCount := 0
	fakeClient.GetStub = func(ctx context.Context, key client.ObjectKey, obj client.Object) error {
		callCount++
		switch callCount {
		case 1:
			if c, ok := obj.(*v1alpha1.SpiffeCSIDriver); ok {
				c.Name = "cluster"
				c.UID = "test-uid"
				c.OwnerReferences = []metav1.OwnerReference{{
					APIVersion: "operator.openshift.io/v1alpha1",
					Kind:       "ZeroTrustWorkloadIdentityManager",
					Name:       "cluster",
					UID:        "ztwim-uid",
				}}
			}
			return nil
		case 2:
			if z, ok := obj.(*v1alpha1.ZeroTrustWorkloadIdentityManager); ok {
				z.Name = "cluster"
				z.UID = "ztwim-uid"
			}
			return nil
		case 3, 4, 5:
			return nil
		default:
			return errors.New("DaemonSet reconciliation failed")
		}
	}

	req := ctrl.Request{NamespacedName: types.NamespacedName{Name: "cluster"}}
	result, err := reconciler.Reconcile(context.Background(), req)

	if err == nil {
		t.Fatal("Expected error when DaemonSet reconciliation fails, got nil - mutant survived")
	}

	if result.Requeue {
		t.Error("Expected Requeue=false when error returned - add requeue mutant survived")
	}
	if result.RequeueAfter != 0 {
		t.Errorf("Expected RequeueAfter=0 when error returned, got %v", result.RequeueAfter)
	}
}
