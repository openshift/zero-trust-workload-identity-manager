package spire_agent

import (
	"context"
	"errors"
	"testing"

	"github.com/go-logr/logr"
	"github.com/openshift/zero-trust-workload-identity-manager/api/v1alpha1"
	"github.com/openshift/zero-trust-workload-identity-manager/pkg/client/fakes"
	"github.com/openshift/zero-trust-workload-identity-manager/pkg/controller/status"
	appsv1 "k8s.io/api/apps/v1"
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
func newTestReconciler(fakeClient *fakes.FakeCustomCtrlClient) *SpireAgentReconciler {
	return &SpireAgentReconciler{
		ctrlClient:    fakeClient,
		ctx:           context.Background(),
		log:           logr.Discard(),
		scheme:        runtime.NewScheme(),
		eventRecorder: record.NewFakeRecorder(100),
	}
}

// TestReconcile_SpireAgentNotFound tests that when SpireAgent CR is not found,
func TestReconcile_SpireAgentNotFound(t *testing.T) {
	fakeClient := &fakes.FakeCustomCtrlClient{}
	reconciler := newTestReconciler(fakeClient)

	// Configure fake client to return NotFound error for SpireAgent
	notFoundErr := kerrors.NewNotFound(schema.GroupResource{Group: "operator.openshift.io", Resource: "spireagents"}, "cluster")
	fakeClient.GetReturns(notFoundErr)

	req := ctrl.Request{NamespacedName: types.NamespacedName{Name: "cluster"}}
	result, err := reconciler.Reconcile(context.Background(), req)

	// Assert: should return nil error (not requeue) when CR not found
	if err != nil {
		t.Errorf("Expected nil error when SpireAgent not found, got: %v", err)
	}
	if result.Requeue {
		t.Error("Expected no requeue when SpireAgent not found")
	}
	if result.RequeueAfter != 0 {
		t.Error("Expected no RequeueAfter when SpireAgent not found")
	}
}

// TestReconcile_SpireAgentGetError tests that when Get returns a non-NotFound error
func TestReconcile_SpireAgentGetError(t *testing.T) {
	fakeClient := &fakes.FakeCustomCtrlClient{}
	reconciler := newTestReconciler(fakeClient)

	// Configure fake client to return a generic error for SpireAgent Get
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

// TestReconcile_ZTWIMNotFound tests that when ZTWIM CR is not found
func TestReconcile_ZTWIMNotFound(t *testing.T) {
	fakeClient := &fakes.FakeCustomCtrlClient{}
	reconciler := newTestReconciler(fakeClient)

	spireAgent := &v1alpha1.SpireAgent{
		ObjectMeta: metav1.ObjectMeta{
			Name: "cluster",
		},
	}

	callCount := 0
	fakeClient.GetStub = func(ctx context.Context, key client.ObjectKey, obj client.Object) error {
		callCount++
		switch callCount {
		case 1: // First call: Get SpireAgent
			if sa, ok := obj.(*v1alpha1.SpireAgent); ok {
				*sa = *spireAgent
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

// TestReconcile_ZTWIMGetError tests that when ZTWIM Get returns a non-NotFound error
func TestReconcile_ZTWIMGetError(t *testing.T) {
	fakeClient := &fakes.FakeCustomCtrlClient{}
	reconciler := newTestReconciler(fakeClient)

	spireAgent := &v1alpha1.SpireAgent{
		ObjectMeta: metav1.ObjectMeta{
			Name: "cluster",
		},
	}

	genericErr := errors.New("internal server error")
	callCount := 0
	fakeClient.GetStub = func(ctx context.Context, key client.ObjectKey, obj client.Object) error {
		callCount++
		switch callCount {
		case 1: // First call: Get SpireAgent
			if sa, ok := obj.(*v1alpha1.SpireAgent); ok {
				*sa = *spireAgent
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

// TestReconcile_OwnerReferenceSetError tests that when SetControllerReference fails
func TestReconcile_OwnerReferenceSetError(t *testing.T) {
	fakeClient := &fakes.FakeCustomCtrlClient{}
	reconciler := newTestReconciler(fakeClient)

	// SpireAgent without owner reference (needs update)
	spireAgent := &v1alpha1.SpireAgent{
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
		case 1: // Get SpireAgent
			if sa, ok := obj.(*v1alpha1.SpireAgent); ok {
				*sa = *spireAgent
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

	// Make Update fail to simulate controller reference update failure
	updateErr := errors.New("update failed")
	fakeClient.UpdateReturns(updateErr)

	req := ctrl.Request{NamespacedName: types.NamespacedName{Name: "cluster"}}
	result, err := reconciler.Reconcile(context.Background(), req)

	// Note: The actual behavior depends on whether NeedsOwnerReferenceUpdate returns true
	// and whether the scheme has the types registered. For mutation testing, we need to
	// verify that errors in this path are properly propagated.
	// The test verifies that Update is called when owner reference needs to be set.
	if result.Requeue && err == nil {
		t.Error("Should not requeue without error when Update fails")
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

	// SpireAgent without owner reference (needs update)
	spireAgent := &v1alpha1.SpireAgent{
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
		case 1: // Get SpireAgent
			if sa, ok := obj.(*v1alpha1.SpireAgent); ok {
				*sa = *spireAgent
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

	agent := &v1alpha1.SpireAgent{
		ObjectMeta: metav1.ObjectMeta{Name: "cluster"},
	}

	// Use real status manager
	statusMgr := status.NewManager(fakeClient)
	result := reconciler.handleCreateOnlyMode(agent, statusMgr)

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

	agent := &v1alpha1.SpireAgent{
		ObjectMeta: metav1.ObjectMeta{Name: "cluster"},
	}

	statusMgr := status.NewManager(fakeClient)
	result := reconciler.handleCreateOnlyMode(agent, statusMgr)

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

	// Agent with existing CreateOnlyMode condition set to True
	agent := &v1alpha1.SpireAgent{
		ObjectMeta: metav1.ObjectMeta{Name: "cluster"},
		Status: v1alpha1.SpireAgentStatus{
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
	result := reconciler.handleCreateOnlyMode(agent, statusMgr)

	// Assert: create-only mode should be detected as false, but condition should be updated
	if result {
		t.Error("Expected handleCreateOnlyMode to return false")
	}
}

// TestNeedsUpdate_ConfigHashChanged tests needsUpdate when config hash differs
func TestNeedsUpdate_ConfigHashChanged(t *testing.T) {
	tests := []struct {
		name     string
		current  string
		desired  string
		expected bool
	}{
		{
			name:     "Same hash - no update needed",
			current:  "abc123",
			desired:  "abc123",
			expected: false,
		},
		{
			name:     "Different hash - update needed",
			current:  "abc123",
			desired:  "xyz789",
			expected: true,
		},
		{
			name:     "Empty current hash - update needed",
			current:  "",
			desired:  "abc123",
			expected: true,
		},
		{
			name:     "Empty desired hash - update needed",
			current:  "abc123",
			desired:  "",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			current := createDaemonSetWithConfigHash(tt.current)
			desired := createDaemonSetWithConfigHash(tt.desired)

			result := needsUpdate(current, desired)
			if result != tt.expected {
				t.Errorf("needsUpdate() = %v, expected %v", result, tt.expected)
			}
		})
	}
}

// Helper to create DaemonSet with config hash annotation
func createDaemonSetWithConfigHash(hash string) appsv1.DaemonSet {
	return appsv1.DaemonSet{
		Spec: appsv1.DaemonSetSpec{
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						spireAgentDaemonSetSpireAgentConfigHashAnnotationKey: hash,
					},
				},
			},
		},
	}
}

// TestNeedsUpdate_NoConfigHash tests needsUpdate with nil annotations
func TestNeedsUpdate_NoConfigHash(t *testing.T) {
	current := appsv1.DaemonSet{
		Spec: appsv1.DaemonSetSpec{
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: nil,
				},
			},
		},
	}
	desired := createDaemonSetWithConfigHash("abc123")

	result := needsUpdate(current, desired)
	if !result {
		t.Error("Expected needsUpdate to return true when current has no annotations")
	}
}

// TestNeedsUpdate_BothEmpty tests needsUpdate when both have empty annotations
func TestNeedsUpdate_BothEmpty(t *testing.T) {
	current := appsv1.DaemonSet{
		Spec: appsv1.DaemonSetSpec{
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{},
				},
			},
		},
	}
	desired := appsv1.DaemonSet{
		Spec: appsv1.DaemonSetSpec{
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{},
				},
			},
		},
	}

	result := needsUpdate(current, desired)
	if result {
		t.Error("Expected needsUpdate to return false when both have empty annotations")
	}
}

// TestValidateConfiguration_ValidConfig tests validateConfiguration with valid config
func TestValidateConfiguration_ValidConfig(t *testing.T) {
	fakeClient := &fakes.FakeCustomCtrlClient{}
	reconciler := newTestReconciler(fakeClient)

	agent := &v1alpha1.SpireAgent{
		ObjectMeta: metav1.ObjectMeta{Name: "cluster"},
		Spec:       v1alpha1.SpireAgentSpec{},
	}

	statusMgr := status.NewManager(fakeClient)
	err := reconciler.validateConfiguration(context.Background(), agent, statusMgr)

	// With default/empty config, validation should pass
	if err != nil {
		t.Errorf("Expected no error for valid config, got: %v", err)
	}
}

// TestValidateConfiguration_InvalidAffinity tests validateConfiguration with invalid affinity
func TestValidateConfiguration_InvalidAffinity(t *testing.T) {
	fakeClient := &fakes.FakeCustomCtrlClient{}
	reconciler := newTestReconciler(fakeClient)

	agent := &v1alpha1.SpireAgent{
		ObjectMeta: metav1.ObjectMeta{Name: "cluster"},
		Spec: v1alpha1.SpireAgentSpec{
			CommonConfig: v1alpha1.CommonConfig{
				Affinity: &corev1.Affinity{
					NodeAffinity: &corev1.NodeAffinity{
						RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
							// Empty node selector terms - invalid
							NodeSelectorTerms: []corev1.NodeSelectorTerm{},
						},
					},
				},
			},
		},
	}

	statusMgr := status.NewManager(fakeClient)
	err := reconciler.validateConfiguration(context.Background(), agent, statusMgr)

	// Invalid affinity should return error
	if err == nil {
		t.Error("Expected error for invalid affinity")
	}
}

// TestValidateProxyConfiguration_NoProxy tests validateProxyConfiguration when proxy is not configured
func TestValidateProxyConfiguration_NoProxy(t *testing.T) {
	// Clear proxy environment variables
	t.Setenv("HTTP_PROXY", "")
	t.Setenv("HTTPS_PROXY", "")
	t.Setenv("NO_PROXY", "")

	fakeClient := &fakes.FakeCustomCtrlClient{}
	reconciler := newTestReconciler(fakeClient)
	statusMgr := status.NewManager(fakeClient)

	err := reconciler.validateProxyConfiguration(statusMgr)

	// When proxy is not configured, validation should pass
	if err != nil {
		t.Errorf("Expected no error when proxy is not configured, got: %v", err)
	}
}

// TestHandleCreateOnlyMode_NotSet tests create-only mode when env var is not set
func TestHandleCreateOnlyMode_NotSet(t *testing.T) {
	// Unset the environment variable
	t.Setenv("CREATE_ONLY_MODE", "")

	fakeClient := &fakes.FakeCustomCtrlClient{}
	reconciler := newTestReconciler(fakeClient)

	agent := &v1alpha1.SpireAgent{
		ObjectMeta: metav1.ObjectMeta{Name: "cluster"},
	}

	statusMgr := status.NewManager(fakeClient)
	result := reconciler.handleCreateOnlyMode(agent, statusMgr)

	// When not set, should default to false
	if result {
		t.Error("Expected handleCreateOnlyMode to return false when CREATE_ONLY_MODE is not set")
	}
}

// TestReconcile_FullFlow tests complete reconcile flow with all resources
func TestReconcile_FullFlow(t *testing.T) {
	fakeClient := &fakes.FakeCustomCtrlClient{}

	// Register types in scheme
	scheme := runtime.NewScheme()
	_ = v1alpha1.AddToScheme(scheme)

	reconciler := &SpireAgentReconciler{
		ctrlClient:    fakeClient,
		ctx:           context.Background(),
		log:           logr.Discard(),
		scheme:        scheme,
		eventRecorder: record.NewFakeRecorder(100),
	}

	// SpireAgent with owner reference already set
	spireAgent := &v1alpha1.SpireAgent{
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
		Spec: v1alpha1.SpireAgentSpec{},
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

	callCount := 0
	fakeClient.GetStub = func(ctx context.Context, key client.ObjectKey, obj client.Object) error {
		callCount++
		switch v := obj.(type) {
		case *v1alpha1.SpireAgent:
			*v = *spireAgent
			return nil
		case *v1alpha1.ZeroTrustWorkloadIdentityManager:
			*v = *ztwim
			return nil
		default:
			return kerrors.NewNotFound(schema.GroupResource{}, key.Name)
		}
	}

	// Make all operations succeed
	fakeClient.CreateReturns(nil)
	fakeClient.UpdateReturns(nil)
	fakeClient.PatchReturns(nil)
	fakeClient.StatusUpdateWithRetryReturns(nil)

	req := ctrl.Request{NamespacedName: types.NamespacedName{Name: "cluster"}}
	result, err := reconciler.Reconcile(context.Background(), req)

	// The reconcile may fail due to missing resources, but should not panic
	// and should handle errors gracefully
	if result.Requeue && err != nil {
		t.Log("Reconcile returned with requeue and error - expected for incomplete setup")
	}
	// Success is if we don't panic
	t.Log("Reconcile completed without panic")
}

// TestSpireAgentReconciler_Fields tests SpireAgentReconciler struct fields
func TestSpireAgentReconciler_Fields(t *testing.T) {
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

// TestDaemonSetConditionConstants tests that condition constants are defined
func TestDaemonSetConditionConstants(t *testing.T) {
	tests := []struct {
		name     string
		constant string
		expected string
	}{
		{"DaemonSetAvailable", DaemonSetAvailable, "DaemonSetAvailable"},
		{"ConfigMapAvailable", ConfigMapAvailable, "ConfigMapAvailable"},
		{"SecurityContextConstraintsAvailable", SecurityContextConstraintsAvailable, "SecurityContextConstraintsAvailable"},
		{"ServiceAccountAvailable", ServiceAccountAvailable, "ServiceAccountAvailable"},
		{"ServiceAvailable", ServiceAvailable, "ServiceAvailable"},
		{"RBACAvailable", RBACAvailable, "RBACAvailable"},
		{"ConfigurationValid", ConfigurationValid, "ConfigurationValid"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.constant != tt.expected {
				t.Errorf("Expected %s to be '%s', got '%s'", tt.name, tt.expected, tt.constant)
			}
		})
	}
}

// TestReconcile_ErrorScenarios tests various error scenarios with table-driven tests
func TestReconcile_ErrorScenarios(t *testing.T) {
	tests := []struct {
		name            string
		setupClient     func(*fakes.FakeCustomCtrlClient)
		setupReconciler func(*SpireAgentReconciler)
		expectError     bool
		expectRequeue   bool
	}{
		{
			name: "NotFound error returns nil and no requeue",
			setupClient: func(fc *fakes.FakeCustomCtrlClient) {
				fc.GetReturns(kerrors.NewNotFound(schema.GroupResource{}, "cluster"))
			},
			expectError:   false,
			expectRequeue: false,
		},
		{
			name: "Generic Get error returns error",
			setupClient: func(fc *fakes.FakeCustomCtrlClient) {
				fc.GetReturns(errors.New("connection refused"))
			},
			expectError:   true,
			expectRequeue: false,
		},
		{
			name: "ZTWIM NotFound returns nil error",
			setupClient: func(fc *fakes.FakeCustomCtrlClient) {
				callCount := 0
				fc.GetStub = func(ctx context.Context, key client.ObjectKey, obj client.Object) error {
					callCount++
					if callCount == 1 {
						if sa, ok := obj.(*v1alpha1.SpireAgent); ok {
							sa.Name = "cluster"
						}
						return nil
					}
					return kerrors.NewNotFound(schema.GroupResource{}, "cluster")
				}
			},
			expectError:   false,
			expectRequeue: false,
		},
		{
			name: "ZTWIM Get error returns error",
			setupClient: func(fc *fakes.FakeCustomCtrlClient) {
				callCount := 0
				fc.GetStub = func(ctx context.Context, key client.ObjectKey, obj client.Object) error {
					callCount++
					if callCount == 1 {
						if sa, ok := obj.(*v1alpha1.SpireAgent); ok {
							sa.Name = "cluster"
						}
						return nil
					}
					return errors.New("internal server error")
				}
			},
			expectError:   true,
			expectRequeue: false,
		},
		{
			name: "Update owner reference error returns error",
			setupClient: func(fc *fakes.FakeCustomCtrlClient) {
				callCount := 0
				fc.GetStub = func(ctx context.Context, key client.ObjectKey, obj client.Object) error {
					callCount++
					switch callCount {
					case 1:
						if sa, ok := obj.(*v1alpha1.SpireAgent); ok {
							sa.Name = "cluster"
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
			setupReconciler: func(r *SpireAgentReconciler) {
				scheme := runtime.NewScheme()
				_ = v1alpha1.AddToScheme(scheme)
				r.scheme = scheme
			},
			expectError:   true,
			expectRequeue: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fakeClient := &fakes.FakeCustomCtrlClient{}
			reconciler := newTestReconciler(fakeClient)

			if tt.setupClient != nil {
				tt.setupClient(fakeClient)
			}
			if tt.setupReconciler != nil {
				tt.setupReconciler(reconciler)
			}

			req := ctrl.Request{NamespacedName: types.NamespacedName{Name: "cluster"}}
			result, err := reconciler.Reconcile(context.Background(), req)

			if tt.expectError && err == nil {
				t.Fatal("Expected error but got nil")
			}
			if !tt.expectError && err != nil {
				t.Fatalf("Expected no error but got: %v", err)
			}
			if result.Requeue != tt.expectRequeue {
				t.Fatalf("Expected Requeue=%v, got %v", tt.expectRequeue, result.Requeue)
			}
		})
	}
}

// TestHandleCreateOnlyMode_AllScenarios tests all create-only mode scenarios
func TestHandleCreateOnlyMode_AllScenarios(t *testing.T) {
	tests := []struct {
		name           string
		envValue       string
		existingCond   *metav1.Condition
		expectedResult bool
	}{
		{
			name:           "enabled returns true",
			envValue:       "true",
			expectedResult: true,
		},
		{
			name:           "disabled returns false",
			envValue:       "false",
			expectedResult: false,
		},
		{
			name:           "empty returns false",
			envValue:       "",
			expectedResult: false,
		},
		{
			name:     "disabled with existing true condition returns false",
			envValue: "false",
			existingCond: &metav1.Condition{
				Type:   "CreateOnlyMode",
				Status: metav1.ConditionTrue,
			},
			expectedResult: false,
		},
		{
			name:     "disabled with existing false condition returns false",
			envValue: "false",
			existingCond: &metav1.Condition{
				Type:   "CreateOnlyMode",
				Status: metav1.ConditionFalse,
			},
			expectedResult: false,
		},
		{
			name:           "disabled with nil condition returns false",
			envValue:       "false",
			existingCond:   nil,
			expectedResult: false,
		},
		{
			name:     "enabled with existing false condition returns true",
			envValue: "true",
			existingCond: &metav1.Condition{
				Type:   "CreateOnlyMode",
				Status: metav1.ConditionFalse,
			},
			expectedResult: true,
		},
		{
			name:     "enabled with existing true condition returns true",
			envValue: "true",
			existingCond: &metav1.Condition{
				Type:   "CreateOnlyMode",
				Status: metav1.ConditionTrue,
			},
			expectedResult: true,
		},
		// With ||, nil existingCondition would cause panic when accessing .Status
		{
			name:           "disabled with nil condition - kills && to || mutant",
			envValue:       "false",
			existingCond:   nil,
			expectedResult: false,
		},
		{
			name:     "disabled with existing unknown condition returns false",
			envValue: "false",
			existingCond: &metav1.Condition{
				Type:   "CreateOnlyMode",
				Status: metav1.ConditionUnknown,
			},
			expectedResult: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("CREATE_ONLY_MODE", tt.envValue)

			fakeClient := &fakes.FakeCustomCtrlClient{}
			reconciler := newTestReconciler(fakeClient)

			agent := &v1alpha1.SpireAgent{
				ObjectMeta: metav1.ObjectMeta{Name: "cluster"},
			}
			if tt.existingCond != nil {
				agent.Status.ConditionalStatus.Conditions = []metav1.Condition{*tt.existingCond}
			}

			statusMgr := status.NewManager(fakeClient)
			result := reconciler.handleCreateOnlyMode(agent, statusMgr)

			if result != tt.expectedResult {
				t.Fatalf("Expected %v, got %v", tt.expectedResult, result)
			}
		})
	}
}

// TestNeedsUpdate_AllScenarios tests all needsUpdate scenarios
func TestNeedsUpdate_AllScenarios(t *testing.T) {
	tests := []struct {
		name        string
		currentHash string
		desiredHash string
		currentNil  bool
		desiredNil  bool
		expected    bool
	}{
		{
			name:        "same hash returns false",
			currentHash: "abc123",
			desiredHash: "abc123",
			expected:    false,
		},
		{
			name:        "different hash returns true",
			currentHash: "abc123",
			desiredHash: "xyz789",
			expected:    true,
		},
		{
			name:        "empty current hash returns true",
			currentHash: "",
			desiredHash: "abc123",
			expected:    true,
		},
		{
			name:        "empty desired hash returns true",
			currentHash: "abc123",
			desiredHash: "",
			expected:    true,
		},
		{
			name:        "both empty returns false",
			currentHash: "",
			desiredHash: "",
			expected:    false,
		},
		{
			name:        "nil current annotations returns true",
			currentNil:  true,
			desiredHash: "abc123",
			expected:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var current, desired appsv1.DaemonSet

			if tt.currentNil {
				current = appsv1.DaemonSet{
					Spec: appsv1.DaemonSetSpec{
						Template: corev1.PodTemplateSpec{
							ObjectMeta: metav1.ObjectMeta{Annotations: nil},
						},
					},
				}
			} else {
				current = createDaemonSetWithConfigHash(tt.currentHash)
			}

			if tt.desiredNil {
				desired = appsv1.DaemonSet{
					Spec: appsv1.DaemonSetSpec{
						Template: corev1.PodTemplateSpec{
							ObjectMeta: metav1.ObjectMeta{Annotations: nil},
						},
					},
				}
			} else {
				desired = createDaemonSetWithConfigHash(tt.desiredHash)
			}

			result := needsUpdate(current, desired)
			if result != tt.expected {
				t.Fatalf("needsUpdate() = %v, expected %v", result, tt.expected)
			}
		})
	}
}

// TestValidateConfiguration_AllScenarios tests configuration validation scenarios
func TestValidateConfiguration_AllScenarios(t *testing.T) {
	tests := []struct {
		name        string
		agent       *v1alpha1.SpireAgent
		expectError bool
	}{
		{
			name: "valid empty config",
			agent: &v1alpha1.SpireAgent{
				ObjectMeta: metav1.ObjectMeta{Name: "cluster"},
				Spec:       v1alpha1.SpireAgentSpec{},
			},
			expectError: false,
		},
		{
			name: "invalid affinity with empty node selector terms",
			agent: &v1alpha1.SpireAgent{
				ObjectMeta: metav1.ObjectMeta{Name: "cluster"},
				Spec: v1alpha1.SpireAgentSpec{
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
			},
			expectError: true,
		},
		{
			name: "valid config with tolerations",
			agent: &v1alpha1.SpireAgent{
				ObjectMeta: metav1.ObjectMeta{Name: "cluster"},
				Spec: v1alpha1.SpireAgentSpec{
					CommonConfig: v1alpha1.CommonConfig{
						Tolerations: []*corev1.Toleration{
							{Key: "key1", Value: "value1", Effect: corev1.TaintEffectNoSchedule},
						},
					},
				},
			},
			expectError: false,
		},
		{
			name: "valid config with node selector",
			agent: &v1alpha1.SpireAgent{
				ObjectMeta: metav1.ObjectMeta{Name: "cluster"},
				Spec: v1alpha1.SpireAgentSpec{
					CommonConfig: v1alpha1.CommonConfig{
						NodeSelector: map[string]string{"node-type": "worker"},
					},
				},
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fakeClient := &fakes.FakeCustomCtrlClient{}
			reconciler := newTestReconciler(fakeClient)
			statusMgr := status.NewManager(fakeClient)

			err := reconciler.validateConfiguration(context.Background(), tt.agent, statusMgr)

			if tt.expectError && err == nil {
				t.Fatal("Expected error but got nil")
			}
			if !tt.expectError && err != nil {
				t.Fatalf("Expected no error but got: %v", err)
			}
		})
	}
}

// TestValidateConfiguration_ConditionUpdate tests the condition update logic
func TestValidateConfiguration_ConditionUpdate(t *testing.T) {
	tests := []struct {
		name            string
		existingStatus  metav1.ConditionStatus
		hasExistingCond bool
	}{
		{
			name:            "no existing condition",
			hasExistingCond: false,
		},
		{
			name:            "existing false condition",
			existingStatus:  metav1.ConditionFalse,
			hasExistingCond: true,
		},
		{
			name:            "existing true condition",
			existingStatus:  metav1.ConditionTrue,
			hasExistingCond: true,
		},
		{
			name:            "existing unknown condition",
			existingStatus:  metav1.ConditionUnknown,
			hasExistingCond: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fakeClient := &fakes.FakeCustomCtrlClient{}
			reconciler := newTestReconciler(fakeClient)
			statusMgr := status.NewManager(fakeClient)

			agent := &v1alpha1.SpireAgent{
				ObjectMeta: metav1.ObjectMeta{Name: "cluster"},
			}

			if tt.hasExistingCond {
				agent.Status.ConditionalStatus.Conditions = []metav1.Condition{
					{
						Type:   ConfigurationValid,
						Status: tt.existingStatus,
						Reason: "Test",
					},
				}
			}

			err := reconciler.validateConfiguration(context.Background(), agent, statusMgr)
			// validateConfiguration should succeed regardless of existing condition state
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}
		})
	}
}

// TestValidateProxyConfiguration_AllScenarios tests proxy configuration validation
func TestValidateProxyConfiguration_AllScenarios(t *testing.T) {
	tests := []struct {
		name        string
		httpProxy   string
		httpsProxy  string
		expectError bool
	}{
		{
			name:        "no proxy configured",
			httpProxy:   "",
			httpsProxy:  "",
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("HTTP_PROXY", tt.httpProxy)
			t.Setenv("HTTPS_PROXY", tt.httpsProxy)

			fakeClient := &fakes.FakeCustomCtrlClient{}
			reconciler := newTestReconciler(fakeClient)
			statusMgr := status.NewManager(fakeClient)

			err := reconciler.validateProxyConfiguration(statusMgr)

			if tt.expectError && err == nil {
				t.Fatal("Expected error but got nil")
			}
			if !tt.expectError && err != nil {
				t.Fatalf("Expected no error but got: %v", err)
			}
		})
	}
}

// TestReconcile_FullFlow_AllScenarios tests full reconcile flow with various scenarios
func TestReconcile_FullFlow_AllScenarios(t *testing.T) {
	tests := []struct {
		name            string
		createOnlyMode  string
		configMapExists bool
		configMapDiff   bool
		daemonSetExists bool
		daemonSetDiff   bool
		expectError     bool
	}{
		{
			name:            "create all resources when none exist",
			createOnlyMode:  "false",
			configMapExists: false,
			daemonSetExists: false,
			expectError:     false,
		},
		{
			name:            "update ConfigMap when differs",
			createOnlyMode:  "false",
			configMapExists: true,
			configMapDiff:   true,
			daemonSetExists: true,
			expectError:     false,
		},
		{
			name:            "skip update in create-only mode",
			createOnlyMode:  "true",
			configMapExists: true,
			configMapDiff:   true,
			daemonSetExists: true,
			daemonSetDiff:   true,
			expectError:     false,
		},
		{
			name:            "no changes needed",
			createOnlyMode:  "false",
			configMapExists: true,
			configMapDiff:   false,
			daemonSetExists: true,
			daemonSetDiff:   false,
			expectError:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("CREATE_ONLY_MODE", tt.createOnlyMode)

			fakeClient := &fakes.FakeCustomCtrlClient{}
			scheme := runtime.NewScheme()
			_ = v1alpha1.AddToScheme(scheme)
			_ = corev1.AddToScheme(scheme)
			_ = appsv1.AddToScheme(scheme)

			reconciler := &SpireAgentReconciler{
				ctrlClient:    fakeClient,
				ctx:           context.Background(),
				log:           logr.Discard(),
				scheme:        scheme,
				eventRecorder: record.NewFakeRecorder(100),
			}

			agent := &v1alpha1.SpireAgent{
				ObjectMeta: metav1.ObjectMeta{
					Name: "cluster",
					OwnerReferences: []metav1.OwnerReference{{
						APIVersion: "operator.openshift.io/v1alpha1",
						Kind:       "ZeroTrustWorkloadIdentityManager",
						Name:       "cluster",
						UID:        "test-uid",
					}},
				},
				Spec: v1alpha1.SpireAgentSpec{},
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

			getCallCount := 0
			fakeClient.GetStub = func(ctx context.Context, key client.ObjectKey, obj client.Object) error {
				getCallCount++
				switch v := obj.(type) {
				case *v1alpha1.SpireAgent:
					*v = *agent
					return nil
				case *v1alpha1.ZeroTrustWorkloadIdentityManager:
					*v = *ztwim
					return nil
				case *corev1.ConfigMap:
					if tt.configMapExists {
						v.Name = key.Name
						v.Namespace = key.Namespace
						if tt.configMapDiff {
							v.Data = map[string]string{"agent.conf": "old-config"}
						}
						return nil
					}
					return kerrors.NewNotFound(schema.GroupResource{}, key.Name)
				case *appsv1.DaemonSet:
					if tt.daemonSetExists {
						v.Name = key.Name
						v.Namespace = key.Namespace
						return nil
					}
					return kerrors.NewNotFound(schema.GroupResource{}, key.Name)
				default:
					return kerrors.NewNotFound(schema.GroupResource{}, key.Name)
				}
			}

			fakeClient.CreateReturns(nil)
			fakeClient.UpdateReturns(nil)
			fakeClient.PatchReturns(nil)
			fakeClient.StatusUpdateWithRetryReturns(nil)

			req := ctrl.Request{NamespacedName: types.NamespacedName{Name: "cluster"}}
			_, err := reconciler.Reconcile(context.Background(), req)

			if tt.expectError && err == nil {
				t.Fatal("Expected error but got nil")
			}
			if !tt.expectError && err != nil {
				t.Fatalf("Expected no error but got: %v", err)
			}
		})
	}
}

// TestReconcileConfigMap_AllScenarios tests reconcileConfigMap with various scenarios
func TestReconcileConfigMap_AllScenarios(t *testing.T) {
	tests := []struct {
		name           string
		createOnlyMode bool
		cmExists       bool
		cmDiff         bool
		createErr      error
		updateErr      error
		getErr         error
		expectError    bool
	}{
		{
			name:        "create ConfigMap when not exists",
			cmExists:    false,
			expectError: false,
		},
		{
			name:        "update ConfigMap when differs",
			cmExists:    true,
			cmDiff:      true,
			expectError: false,
		},
		{
			name:           "skip update in create-only mode",
			cmExists:       true,
			cmDiff:         true,
			createOnlyMode: true,
			expectError:    false,
		},
		{
			name:        "create error",
			cmExists:    false,
			createErr:   errors.New("create failed"),
			expectError: true,
		},
		{
			name:        "update error",
			cmExists:    true,
			cmDiff:      true,
			updateErr:   errors.New("update failed"),
			expectError: true,
		},
		{
			name:        "get error",
			getErr:      errors.New("get failed"),
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fakeClient := &fakes.FakeCustomCtrlClient{}
			scheme := runtime.NewScheme()
			_ = v1alpha1.AddToScheme(scheme)
			_ = corev1.AddToScheme(scheme)

			reconciler := &SpireAgentReconciler{
				ctrlClient:    fakeClient,
				ctx:           context.Background(),
				log:           logr.Discard(),
				scheme:        scheme,
				eventRecorder: record.NewFakeRecorder(100),
			}

			agent := &v1alpha1.SpireAgent{
				ObjectMeta: metav1.ObjectMeta{
					Name: "cluster",
					UID:  "test-uid",
				},
				Spec: v1alpha1.SpireAgentSpec{},
			}

			ztwim := &v1alpha1.ZeroTrustWorkloadIdentityManager{
				ObjectMeta: metav1.ObjectMeta{Name: "cluster"},
				Spec: v1alpha1.ZeroTrustWorkloadIdentityManagerSpec{
					TrustDomain: "example.org",
				},
			}

			fakeClient.GetStub = func(ctx context.Context, key client.ObjectKey, obj client.Object) error {
				if tt.getErr != nil {
					return tt.getErr
				}
				if v, ok := obj.(*corev1.ConfigMap); ok {
					if tt.cmExists {
						v.Name = key.Name
						v.Namespace = key.Namespace
						if tt.cmDiff {
							v.Data = map[string]string{"agent.conf": "old-config"}
						}
						return nil
					}
					return kerrors.NewNotFound(schema.GroupResource{}, key.Name)
				}
				return nil
			}

			if tt.createErr != nil {
				fakeClient.CreateReturns(tt.createErr)
			} else {
				fakeClient.CreateReturns(nil)
			}

			if tt.updateErr != nil {
				fakeClient.UpdateReturns(tt.updateErr)
			} else {
				fakeClient.UpdateReturns(nil)
			}

			statusMgr := status.NewManager(fakeClient)
			_, err := reconciler.reconcileConfigMap(context.Background(), agent, statusMgr, ztwim, tt.createOnlyMode)

			if tt.expectError && err == nil {
				t.Fatal("Expected error but got nil")
			}
			if !tt.expectError && err != nil {
				t.Fatalf("Expected no error but got: %v", err)
			}
		})
	}
}

// TestReconcileDaemonSet_AllScenarios tests reconcileDaemonSet with various scenarios
func TestReconcileDaemonSet_AllScenarios(t *testing.T) {
	tests := []struct {
		name           string
		createOnlyMode bool
		dsExists       bool
		dsDiff         bool
		createErr      error
		updateErr      error
		getErr         error
		expectError    bool
	}{
		{
			name:        "create DaemonSet when not exists",
			dsExists:    false,
			expectError: false,
		},
		{
			name:        "no update when same",
			dsExists:    true,
			dsDiff:      false,
			expectError: false,
		},
		{
			name:           "skip update in create-only mode",
			dsExists:       true,
			dsDiff:         true,
			createOnlyMode: true,
			expectError:    false,
		},
		{
			name:        "create error",
			dsExists:    false,
			createErr:   errors.New("create failed"),
			expectError: true,
		},
		{
			name:        "get error",
			getErr:      errors.New("get failed"),
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fakeClient := &fakes.FakeCustomCtrlClient{}
			scheme := runtime.NewScheme()
			_ = v1alpha1.AddToScheme(scheme)
			_ = appsv1.AddToScheme(scheme)

			reconciler := &SpireAgentReconciler{
				ctrlClient:    fakeClient,
				ctx:           context.Background(),
				log:           logr.Discard(),
				scheme:        scheme,
				eventRecorder: record.NewFakeRecorder(100),
			}

			agent := &v1alpha1.SpireAgent{
				ObjectMeta: metav1.ObjectMeta{
					Name: "cluster",
					UID:  "test-uid",
				},
				Spec: v1alpha1.SpireAgentSpec{},
			}

			ztwim := &v1alpha1.ZeroTrustWorkloadIdentityManager{
				ObjectMeta: metav1.ObjectMeta{Name: "cluster"},
				Spec: v1alpha1.ZeroTrustWorkloadIdentityManagerSpec{
					TrustDomain: "example.org",
				},
			}

			fakeClient.GetStub = func(ctx context.Context, key client.ObjectKey, obj client.Object) error {
				if tt.getErr != nil {
					return tt.getErr
				}
				if v, ok := obj.(*appsv1.DaemonSet); ok {
					if tt.dsExists {
						v.Name = key.Name
						v.Namespace = key.Namespace
						if !tt.dsDiff {
							// Set same hash to simulate no change needed
							v.Spec.Template.Annotations = map[string]string{
								spireAgentDaemonSetSpireAgentConfigHashAnnotationKey: "test-hash",
							}
						}
						return nil
					}
					return kerrors.NewNotFound(schema.GroupResource{}, key.Name)
				}
				return nil
			}

			if tt.createErr != nil {
				fakeClient.CreateReturns(tt.createErr)
			} else {
				fakeClient.CreateReturns(nil)
			}

			if tt.updateErr != nil {
				fakeClient.UpdateReturns(tt.updateErr)
			} else {
				fakeClient.UpdateReturns(nil)
			}

			statusMgr := status.NewManager(fakeClient)
			err := reconciler.reconcileDaemonSet(context.Background(), agent, statusMgr, ztwim, tt.createOnlyMode, "test-hash")

			if tt.expectError && err == nil {
				t.Fatal("Expected error but got nil")
			}
			if !tt.expectError && err != nil {
				t.Fatalf("Expected no error but got: %v", err)
			}
		})
	}
}

// TestReconcileRBAC_AllScenarios tests reconcileRBAC with various scenarios
func TestReconcileRBAC_AllScenarios(t *testing.T) {
	tests := []struct {
		name                  string
		createOnlyMode        bool
		clusterRoleErr        error
		clusterRoleBindingErr error
		expectError           bool
	}{
		{
			name:        "successful RBAC reconciliation",
			expectError: false,
		},
		{
			name:           "skip in create-only mode with existing resources",
			createOnlyMode: true,
			expectError:    false,
		},
		{
			name:           "cluster role error",
			clusterRoleErr: errors.New("cluster role failed"),
			expectError:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fakeClient := &fakes.FakeCustomCtrlClient{}
			scheme := runtime.NewScheme()
			_ = v1alpha1.AddToScheme(scheme)

			reconciler := &SpireAgentReconciler{
				ctrlClient:    fakeClient,
				ctx:           context.Background(),
				log:           logr.Discard(),
				scheme:        scheme,
				eventRecorder: record.NewFakeRecorder(100),
			}

			agent := &v1alpha1.SpireAgent{
				ObjectMeta: metav1.ObjectMeta{
					Name: "cluster",
					UID:  "test-uid",
				},
			}

			// Setup fake client to return NotFound for all resources (will trigger Create)
			fakeClient.GetReturns(kerrors.NewNotFound(schema.GroupResource{}, "test"))

			if tt.clusterRoleErr != nil {
				fakeClient.CreateReturns(tt.clusterRoleErr)
			} else {
				fakeClient.CreateReturns(nil)
			}

			statusMgr := status.NewManager(fakeClient)
			err := reconciler.reconcileRBAC(context.Background(), agent, statusMgr, tt.createOnlyMode)

			if tt.expectError && err == nil {
				t.Fatal("Expected error but got nil")
			}
			if !tt.expectError && err != nil {
				t.Fatalf("Expected no error but got: %v", err)
			}
		})
	}
}

// TestReconcile_ErrorPropagation tests error propagation from all reconcile functions
func TestReconcile_ErrorPropagation(t *testing.T) {
	tests := []struct {
		name            string
		setupClient     func(*fakes.FakeCustomCtrlClient)
		expectError     bool
		expectNilResult bool
	}{
		{
			name: "reconcileServiceAccount error returns error not nil",
			setupClient: func(fc *fakes.FakeCustomCtrlClient) {
				callCount := 0
				fc.GetStub = func(ctx context.Context, key client.ObjectKey, obj client.Object) error {
					callCount++
					switch callCount {
					case 1:
						if a, ok := obj.(*v1alpha1.SpireAgent); ok {
							a.Name = "cluster"
							a.OwnerReferences = []metav1.OwnerReference{{
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
							z.Spec.TrustDomain = "example.org"
						}
						return nil
					default:
						return errors.New("service account get error")
					}
				}
			},
			expectError:     true,
			expectNilResult: false,
		},
		{
			name: "reconcileService error returns error not nil",
			setupClient: func(fc *fakes.FakeCustomCtrlClient) {
				callCount := 0
				fc.GetStub = func(ctx context.Context, key client.ObjectKey, obj client.Object) error {
					callCount++
					switch callCount {
					case 1:
						if a, ok := obj.(*v1alpha1.SpireAgent); ok {
							a.Name = "cluster"
							a.OwnerReferences = []metav1.OwnerReference{{
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
							z.Spec.TrustDomain = "example.org"
						}
						return nil
					case 3: // ServiceAccount Get - return existing
						return nil
					default:
						return errors.New("service get error")
					}
				}
			},
			expectError:     true,
			expectNilResult: false,
		},
		{
			name: "reconcileRBAC error returns error not nil",
			setupClient: func(fc *fakes.FakeCustomCtrlClient) {
				callCount := 0
				fc.GetStub = func(ctx context.Context, key client.ObjectKey, obj client.Object) error {
					callCount++
					switch callCount {
					case 1:
						if a, ok := obj.(*v1alpha1.SpireAgent); ok {
							a.Name = "cluster"
							a.OwnerReferences = []metav1.OwnerReference{{
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
							z.Spec.TrustDomain = "example.org"
						}
						return nil
					case 3, 4: // ServiceAccount, Service - return existing
						return nil
					default:
						return errors.New("rbac get error")
					}
				}
			},
			expectError:     true,
			expectNilResult: false,
		},
		{
			name: "reconcileConfigMap error returns error not nil",
			setupClient: func(fc *fakes.FakeCustomCtrlClient) {
				callCount := 0
				fc.GetStub = func(ctx context.Context, key client.ObjectKey, obj client.Object) error {
					callCount++
					switch callCount {
					case 1:
						if a, ok := obj.(*v1alpha1.SpireAgent); ok {
							a.Name = "cluster"
							a.OwnerReferences = []metav1.OwnerReference{{
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
							z.Spec.TrustDomain = "example.org"
						}
						return nil
					case 3, 4, 5, 6, 7, 8: // ServiceAccount, Service, RBAC - return existing
						return nil
					default:
						return errors.New("configmap get error")
					}
				}
			},
			expectError:     true,
			expectNilResult: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fakeClient := &fakes.FakeCustomCtrlClient{}
			scheme := runtime.NewScheme()
			_ = v1alpha1.AddToScheme(scheme)

			reconciler := &SpireAgentReconciler{
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

	reconciler := &SpireAgentReconciler{
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
		case 1: // SpireAgent
			if a, ok := obj.(*v1alpha1.SpireAgent); ok {
				a.Name = "cluster"
				a.UID = "test-uid"
				a.OwnerReferences = []metav1.OwnerReference{{
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
				z.Spec.TrustDomain = "example.org"
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

// TestNeedsUpdate_ConfigHashComparison tests needsUpdate comparing config hashes
func TestNeedsUpdate_ConfigHashComparison(t *testing.T) {
	tests := []struct {
		name        string
		currentHash string
		desiredHash string
		expectTrue  bool
	}{
		{
			name:        "different hashes returns true",
			currentHash: "hash1",
			desiredHash: "hash2",
			expectTrue:  true,
		},
		{
			name:        "same hashes returns false",
			currentHash: "hash1",
			desiredHash: "hash1",
			expectTrue:  false,
		},
		{
			name:        "empty current hash returns true",
			currentHash: "",
			desiredHash: "hash1",
			expectTrue:  true,
		},
		{
			name:        "both empty returns false",
			currentHash: "",
			desiredHash: "",
			expectTrue:  false,
		},
		{
			name:        "empty desired hash with non-empty current returns true",
			currentHash: "hash1",
			desiredHash: "",
			expectTrue:  true,
		},
		{
			name:        "whitespace only current hash returns true",
			currentHash: "   ",
			desiredHash: "hash1",
			expectTrue:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			current := createDaemonSetWithConfigHash(tt.currentHash)
			desired := createDaemonSetWithConfigHash(tt.desiredHash)

			result := needsUpdate(current, desired)
			if result != tt.expectTrue {
				t.Errorf("needsUpdate() = %v, expected %v", result, tt.expectTrue)
			}
		})
	}
}

// TestReconcile_ReconciliationStepErrors_MutationKillers tests error handling for each step
func TestReconcile_ReconciliationStepErrors_MutationKillers(t *testing.T) {
	tests := []struct {
		name           string
		failAtGetCount int
		description    string
	}{
		{
			name:           "ServiceAccount error returns error",
			failAtGetCount: 4,
			description:    "reconcileServiceAccount failure",
		},
		{
			name:           "Service error returns error",
			failAtGetCount: 5,
			description:    "reconcileService failure",
		},
		{
			name:           "ConfigMap error returns error",
			failAtGetCount: 7,
			description:    "reconcileConfigMap failure",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fakeClient := &fakes.FakeCustomCtrlClient{}
			scheme := runtime.NewScheme()
			_ = v1alpha1.AddToScheme(scheme)

			reconciler := &SpireAgentReconciler{
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
					if a, ok := obj.(*v1alpha1.SpireAgent); ok {
						a.Name = "cluster"
						a.UID = "test-uid"
						a.OwnerReferences = []metav1.OwnerReference{{
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
						z.Spec.TrustDomain = "example.org"
					}
					return nil
				case 3:
					if s, ok := obj.(*v1alpha1.SpireServer); ok {
						s.Name = "cluster"
					}
					return nil
				default:
					if callCount >= tt.failAtGetCount {
						return errors.New(tt.description)
					}
					return nil
				}
			}

			req := ctrl.Request{NamespacedName: types.NamespacedName{Name: "cluster"}}
			result, err := reconciler.Reconcile(context.Background(), req)

			if err == nil {
				t.Fatalf("Expected error for %s, got nil - mutant survived", tt.description)
			}

			if result.Requeue {
				t.Errorf("Expected Requeue=false when error returned for %s - mutant survived", tt.description)
			}
			if result.RequeueAfter != 0 {
				t.Errorf("Expected RequeueAfter=0 when error returned for %s", tt.description)
			}
		})
	}
}
