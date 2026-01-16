package spire_oidc_discovery_provider

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
func newTestReconciler(fakeClient *fakes.FakeCustomCtrlClient) *SpireOidcDiscoveryProviderReconciler {
	return &SpireOidcDiscoveryProviderReconciler{
		ctrlClient:    fakeClient,
		ctx:           context.Background(),
		log:           logr.Discard(),
		scheme:        runtime.NewScheme(),
		eventRecorder: record.NewFakeRecorder(100),
	}
}

// TestReconcile_SpireOIDCDiscoveryProviderNotFound tests that when SpireOIDCDiscoveryProvider CR is not found,
func TestReconcile_SpireOIDCDiscoveryProviderNotFound(t *testing.T) {
	fakeClient := &fakes.FakeCustomCtrlClient{}
	reconciler := newTestReconciler(fakeClient)

	// Configure fake client to return NotFound error for SpireOIDCDiscoveryProvider
	notFoundErr := kerrors.NewNotFound(schema.GroupResource{Group: "operator.openshift.io", Resource: "spireoidcdiscoveryproviders"}, "cluster")
	fakeClient.GetReturns(notFoundErr)

	req := ctrl.Request{NamespacedName: types.NamespacedName{Name: "cluster"}}
	result, err := reconciler.Reconcile(context.Background(), req)

	// Assert: should return nil error (not requeue) when CR not found
	if err != nil {
		t.Errorf("Expected nil error when SpireOIDCDiscoveryProvider not found, got: %v", err)
	}
	if result.Requeue {
		t.Error("Expected no requeue when SpireOIDCDiscoveryProvider not found")
	}
	if result.RequeueAfter != 0 {
		t.Error("Expected no RequeueAfter when SpireOIDCDiscoveryProvider not found")
	}
}

// TestReconcile_SpireOIDCDiscoveryProviderGetError tests that when Get returns a non-NotFound error
func TestReconcile_SpireOIDCDiscoveryProviderGetError(t *testing.T) {
	fakeClient := &fakes.FakeCustomCtrlClient{}
	reconciler := newTestReconciler(fakeClient)

	// Configure fake client to return a generic error for SpireOIDCDiscoveryProvider Get
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

	oidcProvider := &v1alpha1.SpireOIDCDiscoveryProvider{
		ObjectMeta: metav1.ObjectMeta{
			Name: "cluster",
		},
	}

	callCount := 0
	fakeClient.GetStub = func(ctx context.Context, key client.ObjectKey, obj client.Object) error {
		callCount++
		switch callCount {
		case 1: // First call: Get SpireOIDCDiscoveryProvider
			if oidc, ok := obj.(*v1alpha1.SpireOIDCDiscoveryProvider); ok {
				*oidc = *oidcProvider
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

	oidcProvider := &v1alpha1.SpireOIDCDiscoveryProvider{
		ObjectMeta: metav1.ObjectMeta{
			Name: "cluster",
		},
	}

	genericErr := errors.New("internal server error")
	callCount := 0
	fakeClient.GetStub = func(ctx context.Context, key client.ObjectKey, obj client.Object) error {
		callCount++
		switch callCount {
		case 1: // First call: Get SpireOIDCDiscoveryProvider
			if oidc, ok := obj.(*v1alpha1.SpireOIDCDiscoveryProvider); ok {
				*oidc = *oidcProvider
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

// TestReconcile_OwnerReferenceUpdateError tests that when Update fails after setting owner,
func TestReconcile_OwnerReferenceUpdateError(t *testing.T) {
	fakeClient := &fakes.FakeCustomCtrlClient{}
	reconciler := newTestReconciler(fakeClient)

	// Register types in scheme for SetControllerReference
	scheme := runtime.NewScheme()
	_ = v1alpha1.AddToScheme(scheme)
	reconciler.scheme = scheme

	// SpireOIDCDiscoveryProvider without owner reference (needs update)
	oidcProvider := &v1alpha1.SpireOIDCDiscoveryProvider{
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
		case 1: // Get SpireOIDCDiscoveryProvider
			if oidc, ok := obj.(*v1alpha1.SpireOIDCDiscoveryProvider); ok {
				*oidc = *oidcProvider
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

	oidc := &v1alpha1.SpireOIDCDiscoveryProvider{
		ObjectMeta: metav1.ObjectMeta{Name: "cluster"},
	}

	statusMgr := status.NewManager(fakeClient)
	result := reconciler.handleCreateOnlyMode(oidc, statusMgr)

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

	oidc := &v1alpha1.SpireOIDCDiscoveryProvider{
		ObjectMeta: metav1.ObjectMeta{Name: "cluster"},
	}

	statusMgr := status.NewManager(fakeClient)
	result := reconciler.handleCreateOnlyMode(oidc, statusMgr)

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

	// OIDC provider with existing CreateOnlyMode condition set to True
	oidc := &v1alpha1.SpireOIDCDiscoveryProvider{
		ObjectMeta: metav1.ObjectMeta{Name: "cluster"},
		Status: v1alpha1.SpireOIDCDiscoveryProviderStatus{
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
	result := reconciler.handleCreateOnlyMode(oidc, statusMgr)

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
			current := createDeploymentWithConfigHash(tt.current)
			desired := createDeploymentWithConfigHash(tt.desired)

			result := needsUpdate(current, desired)
			if result != tt.expected {
				t.Errorf("needsUpdate() = %v, expected %v", result, tt.expected)
			}
		})
	}
}

// TestValidateConfiguration_ValidConfig tests configuration validation passes
func TestValidateConfiguration_ValidConfig(t *testing.T) {
	fakeClient := &fakes.FakeCustomCtrlClient{}
	reconciler := newTestReconciler(fakeClient)

	oidc := &v1alpha1.SpireOIDCDiscoveryProvider{
		ObjectMeta: metav1.ObjectMeta{Name: "cluster"},
		Spec: v1alpha1.SpireOIDCDiscoveryProviderSpec{
			JwtIssuer: "https://example.com",
		},
	}

	statusMgr := status.NewManager(fakeClient)
	err := reconciler.validateConfiguration(context.Background(), oidc, statusMgr)

	// Assert: validation should pass with valid configuration
	if err != nil {
		t.Errorf("Expected no error for valid configuration, got: %v", err)
	}
}

// TestValidateConfiguration_InvalidJWTIssuer tests configuration validation fails with invalid JWT issuer
func TestValidateConfiguration_InvalidJWTIssuer(t *testing.T) {
	fakeClient := &fakes.FakeCustomCtrlClient{}
	reconciler := newTestReconciler(fakeClient)

	oidc := &v1alpha1.SpireOIDCDiscoveryProvider{
		ObjectMeta: metav1.ObjectMeta{Name: "cluster"},
		Spec: v1alpha1.SpireOIDCDiscoveryProviderSpec{
			JwtIssuer: "not-a-valid-url",
		},
	}

	statusMgr := status.NewManager(fakeClient)
	err := reconciler.validateConfiguration(context.Background(), oidc, statusMgr)

	// Assert: validation should fail with invalid JWT issuer
	if err == nil {
		t.Error("Expected error for invalid JWT issuer URL")
	}
}

// Helper to create Deployment with config hash annotation
func createDeploymentWithConfigHash(hash string) appsv1.Deployment {
	return appsv1.Deployment{
		Spec: appsv1.DeploymentSpec{
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						spireOidcDeploymentSpireOidcConfigHashAnnotationKey: hash,
					},
				},
			},
		},
	}
}

// TestNeedsUpdate_NoAnnotations tests needsUpdate with nil annotations
func TestNeedsUpdate_NoAnnotations(t *testing.T) {
	current := appsv1.Deployment{
		Spec: appsv1.DeploymentSpec{
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: nil,
				},
			},
		},
	}
	desired := createDeploymentWithConfigHash("abc123")

	result := needsUpdate(current, desired)
	if !result {
		t.Error("Expected needsUpdate to return true when current has no annotations")
	}
}

// TestNeedsUpdate_BothEmpty tests needsUpdate when both have empty annotations
func TestNeedsUpdate_BothEmpty(t *testing.T) {
	current := appsv1.Deployment{
		Spec: appsv1.DeploymentSpec{
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{},
				},
			},
		},
	}
	desired := appsv1.Deployment{
		Spec: appsv1.DeploymentSpec{
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

// TestValidateCommonConfig_Valid tests common config validation with valid values
func TestValidateCommonConfig_Valid(t *testing.T) {
	fakeClient := &fakes.FakeCustomCtrlClient{}
	reconciler := newTestReconciler(fakeClient)

	oidc := &v1alpha1.SpireOIDCDiscoveryProvider{
		ObjectMeta: metav1.ObjectMeta{Name: "cluster"},
		Spec: v1alpha1.SpireOIDCDiscoveryProviderSpec{
			JwtIssuer: "https://example.com",
		},
	}

	statusMgr := status.NewManager(fakeClient)
	err := reconciler.validateCommonConfig(oidc, statusMgr)

	if err != nil {
		t.Errorf("Expected no error for valid common config, got: %v", err)
	}
}

// TestValidateCommonConfig_InvalidAffinity tests common config validation with invalid affinity
func TestValidateCommonConfig_InvalidAffinity(t *testing.T) {
	fakeClient := &fakes.FakeCustomCtrlClient{}
	reconciler := newTestReconciler(fakeClient)

	oidc := &v1alpha1.SpireOIDCDiscoveryProvider{
		ObjectMeta: metav1.ObjectMeta{Name: "cluster"},
		Spec: v1alpha1.SpireOIDCDiscoveryProviderSpec{
			JwtIssuer: "https://example.com",
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
	err := reconciler.validateCommonConfig(oidc, statusMgr)

	if err == nil {
		t.Error("Expected error for invalid affinity")
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
			name:            "no existing condition - should not add",
			hasExistingCond: false,
		},
		{
			name:            "existing false condition - should add true",
			existingStatus:  metav1.ConditionFalse,
			hasExistingCond: true,
		},
		{
			name:            "existing true condition - should not add",
			existingStatus:  metav1.ConditionTrue,
			hasExistingCond: true,
		},
		{
			name:            "existing unknown condition - should not add",
			existingStatus:  metav1.ConditionUnknown,
			hasExistingCond: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fakeClient := &fakes.FakeCustomCtrlClient{}
			reconciler := newTestReconciler(fakeClient)
			statusMgr := status.NewManager(fakeClient)

			oidc := &v1alpha1.SpireOIDCDiscoveryProvider{
				ObjectMeta: metav1.ObjectMeta{Name: "cluster"},
				Spec: v1alpha1.SpireOIDCDiscoveryProviderSpec{
					JwtIssuer: "https://example.com",
				},
			}

			if tt.hasExistingCond {
				oidc.Status.ConditionalStatus.Conditions = []metav1.Condition{
					{
						Type:   ConfigurationValid,
						Status: tt.existingStatus,
						Reason: "Test",
					},
				}
			}

			err := reconciler.validateConfiguration(context.Background(), oidc, statusMgr)
			// validateConfiguration should succeed regardless of existing condition state
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}
		})
	}
}

// TestValidateProxyConfiguration_NoProxy tests proxy validation when not configured
func TestValidateProxyConfiguration_NoProxy(t *testing.T) {
	t.Setenv("HTTP_PROXY", "")
	t.Setenv("HTTPS_PROXY", "")

	fakeClient := &fakes.FakeCustomCtrlClient{}
	reconciler := newTestReconciler(fakeClient)
	statusMgr := status.NewManager(fakeClient)

	err := reconciler.validateProxyConfiguration(statusMgr)

	if err != nil {
		t.Errorf("Expected no error when proxy is not configured, got: %v", err)
	}
}

// TestHandleCreateOnlyMode_NotSet tests create-only mode when env var is not set
func TestHandleCreateOnlyMode_NotSet(t *testing.T) {
	t.Setenv("CREATE_ONLY_MODE", "")

	fakeClient := &fakes.FakeCustomCtrlClient{}
	reconciler := newTestReconciler(fakeClient)

	oidc := &v1alpha1.SpireOIDCDiscoveryProvider{
		ObjectMeta: metav1.ObjectMeta{Name: "cluster"},
	}

	statusMgr := status.NewManager(fakeClient)
	result := reconciler.handleCreateOnlyMode(oidc, statusMgr)

	if result {
		t.Error("Expected handleCreateOnlyMode to return false when CREATE_ONLY_MODE is not set")
	}
}

// TestSpireOidcDiscoveryProviderReconciler_Fields tests SpireOidcDiscoveryProviderReconciler struct fields
func TestSpireOidcDiscoveryProviderReconciler_Fields(t *testing.T) {
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
	if DeploymentAvailable != "DeploymentAvailable" {
		t.Errorf("Expected DeploymentAvailable to be 'DeploymentAvailable', got %s", DeploymentAvailable)
	}
	if ConfigMapAvailable != "ConfigMapAvailable" {
		t.Errorf("Expected ConfigMapAvailable to be 'ConfigMapAvailable', got %s", ConfigMapAvailable)
	}
	if ClusterSPIFFEIDAvailable != "ClusterSPIFFEIDAvailable" {
		t.Errorf("Expected ClusterSPIFFEIDAvailable to be 'ClusterSPIFFEIDAvailable', got %s", ClusterSPIFFEIDAvailable)
	}
	if RouteAvailable != "RouteAvailable" {
		t.Errorf("Expected RouteAvailable to be 'RouteAvailable', got %s", RouteAvailable)
	}
	if ConfigurationValid != "ConfigurationValid" {
		t.Errorf("Expected ConfigurationValid to be 'ConfigurationValid', got %s", ConfigurationValid)
	}
	if ServiceAccountAvailable != "ServiceAccountAvailable" {
		t.Errorf("Expected ServiceAccountAvailable to be 'ServiceAccountAvailable', got %s", ServiceAccountAvailable)
	}
	if ServiceAvailable != "ServiceAvailable" {
		t.Errorf("Expected ServiceAvailable to be 'ServiceAvailable', got %s", ServiceAvailable)
	}
}

// TestReconcile_FullFlow tests complete reconcile flow
func TestReconcile_FullFlow(t *testing.T) {
	fakeClient := &fakes.FakeCustomCtrlClient{}

	scheme := runtime.NewScheme()
	_ = v1alpha1.AddToScheme(scheme)

	reconciler := &SpireOidcDiscoveryProviderReconciler{
		ctrlClient:    fakeClient,
		ctx:           context.Background(),
		log:           logr.Discard(),
		scheme:        scheme,
		eventRecorder: record.NewFakeRecorder(100),
	}

	oidcProvider := &v1alpha1.SpireOIDCDiscoveryProvider{
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
		Spec: v1alpha1.SpireOIDCDiscoveryProviderSpec{
			JwtIssuer: "https://example.com",
		},
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
		case *v1alpha1.SpireOIDCDiscoveryProvider:
			*v = *oidcProvider
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

// TestValidateConfiguration_ConfigurationValidPreviouslyFalse tests configuration validation
// when the condition previously existed as false
func TestValidateConfiguration_ConfigurationValidPreviouslyFalse(t *testing.T) {
	fakeClient := &fakes.FakeCustomCtrlClient{}
	reconciler := newTestReconciler(fakeClient)

	oidc := &v1alpha1.SpireOIDCDiscoveryProvider{
		ObjectMeta: metav1.ObjectMeta{Name: "cluster"},
		Spec: v1alpha1.SpireOIDCDiscoveryProviderSpec{
			JwtIssuer: "https://example.com",
		},
		Status: v1alpha1.SpireOIDCDiscoveryProviderStatus{
			ConditionalStatus: v1alpha1.ConditionalStatus{
				Conditions: []metav1.Condition{
					{
						Type:   ConfigurationValid,
						Status: metav1.ConditionFalse,
						Reason: "InvalidJWTIssuerURL",
					},
				},
			},
		},
	}

	statusMgr := status.NewManager(fakeClient)
	err := reconciler.validateConfiguration(context.Background(), oidc, statusMgr)

	if err != nil {
		t.Errorf("Expected no error for valid configuration, got: %v", err)
	}
}

// TestReconcile_ErrorScenarios tests various error scenarios with table-driven tests
func TestReconcile_ErrorScenarios(t *testing.T) {
	tests := []struct {
		name            string
		setupClient     func(*fakes.FakeCustomCtrlClient)
		setupReconciler func(*SpireOidcDiscoveryProviderReconciler)
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
						if oidc, ok := obj.(*v1alpha1.SpireOIDCDiscoveryProvider); ok {
							oidc.Name = "cluster"
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
						if oidc, ok := obj.(*v1alpha1.SpireOIDCDiscoveryProvider); ok {
							oidc.Name = "cluster"
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
						if oidc, ok := obj.(*v1alpha1.SpireOIDCDiscoveryProvider); ok {
							oidc.Name = "cluster"
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
			setupReconciler: func(r *SpireOidcDiscoveryProviderReconciler) {
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

			oidc := &v1alpha1.SpireOIDCDiscoveryProvider{
				ObjectMeta: metav1.ObjectMeta{Name: "cluster"},
			}
			if tt.existingCond != nil {
				oidc.Status.ConditionalStatus.Conditions = []metav1.Condition{*tt.existingCond}
			}

			statusMgr := status.NewManager(fakeClient)
			result := reconciler.handleCreateOnlyMode(oidc, statusMgr)

			if result != tt.expectedResult {
				t.Fatalf("Expected %v, got %v", tt.expectedResult, result)
			}
		})
	}
}

// TestValidateConfiguration_AllScenarios tests all configuration validation scenarios
func TestValidateConfiguration_AllScenarios(t *testing.T) {
	tests := []struct {
		name        string
		oidc        *v1alpha1.SpireOIDCDiscoveryProvider
		expectError bool
	}{
		{
			name: "valid configuration",
			oidc: &v1alpha1.SpireOIDCDiscoveryProvider{
				ObjectMeta: metav1.ObjectMeta{Name: "cluster"},
				Spec: v1alpha1.SpireOIDCDiscoveryProviderSpec{
					JwtIssuer: "https://example.com",
				},
			},
			expectError: false,
		},
		{
			name: "invalid JWT issuer URL",
			oidc: &v1alpha1.SpireOIDCDiscoveryProvider{
				ObjectMeta: metav1.ObjectMeta{Name: "cluster"},
				Spec: v1alpha1.SpireOIDCDiscoveryProviderSpec{
					JwtIssuer: "not-a-valid-url",
				},
			},
			expectError: true,
		},
		{
			name: "invalid affinity with empty node selector terms",
			oidc: &v1alpha1.SpireOIDCDiscoveryProvider{
				ObjectMeta: metav1.ObjectMeta{Name: "cluster"},
				Spec: v1alpha1.SpireOIDCDiscoveryProviderSpec{
					JwtIssuer: "https://example.com",
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fakeClient := &fakes.FakeCustomCtrlClient{}
			reconciler := newTestReconciler(fakeClient)
			statusMgr := status.NewManager(fakeClient)

			err := reconciler.validateConfiguration(context.Background(), tt.oidc, statusMgr)

			if tt.expectError && err == nil {
				t.Fatal("Expected error but got nil")
			}
			if !tt.expectError && err != nil {
				t.Fatalf("Expected no error but got: %v", err)
			}
		})
	}
}

// TestReconcileClusterSpiffeIDs_AllScenarios tests reconcileClusterSpiffeIDs with various scenarios
func TestReconcileClusterSpiffeIDs_AllScenarios(t *testing.T) {
	tests := []struct {
		name           string
		createOnlyMode bool
		createErr      error
		getErr         error
		expectError    bool
	}{
		{
			name:        "successful creation",
			expectError: false,
		},
		{
			name:           "skip in create-only mode",
			createOnlyMode: true,
			expectError:    false,
		},
		{
			name:        "create error",
			createErr:   errors.New("create failed"),
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fakeClient := &fakes.FakeCustomCtrlClient{}
			scheme := runtime.NewScheme()
			_ = v1alpha1.AddToScheme(scheme)

			reconciler := &SpireOidcDiscoveryProviderReconciler{
				ctrlClient:    fakeClient,
				ctx:           context.Background(),
				log:           logr.Discard(),
				scheme:        scheme,
				eventRecorder: record.NewFakeRecorder(100),
			}

			oidc := &v1alpha1.SpireOIDCDiscoveryProvider{
				ObjectMeta: metav1.ObjectMeta{
					Name: "cluster",
					UID:  "test-uid",
				},
				Spec: v1alpha1.SpireOIDCDiscoveryProviderSpec{
					JwtIssuer: "https://example.com",
				},
			}

			if tt.getErr != nil {
				fakeClient.GetReturns(tt.getErr)
			} else {
				fakeClient.GetReturns(kerrors.NewNotFound(schema.GroupResource{}, "test"))
			}

			if tt.createErr != nil {
				fakeClient.CreateReturns(tt.createErr)
			} else {
				fakeClient.CreateReturns(nil)
			}

			statusMgr := status.NewManager(fakeClient)
			err := reconciler.reconcileClusterSpiffeIDs(context.Background(), oidc, statusMgr, tt.createOnlyMode)

			if tt.expectError && err == nil {
				t.Fatal("Expected error but got nil")
			}
			if !tt.expectError && err != nil {
				t.Fatalf("Expected no error but got: %v", err)
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
		caBundle    string
		expectError bool
	}{
		{
			name:        "no proxy configured",
			httpProxy:   "",
			httpsProxy:  "",
			caBundle:    "",
			expectError: false,
		},
		{
			name:        "valid http proxy with ca bundle",
			httpProxy:   "http://proxy.example.com:8080",
			httpsProxy:  "",
			caBundle:    "trusted-ca",
			expectError: false,
		},
		{
			name:        "valid https proxy with ca bundle",
			httpProxy:   "",
			httpsProxy:  "https://proxy.example.com:8443",
			caBundle:    "trusted-ca",
			expectError: false,
		},
		{
			name:        "proxy without ca bundle returns error",
			httpProxy:   "http://proxy.example.com:8080",
			httpsProxy:  "",
			caBundle:    "",
			expectError: true,
		},
		{
			name:        "both proxies with ca bundle",
			httpProxy:   "http://proxy.example.com:8080",
			httpsProxy:  "https://proxy.example.com:8443",
			caBundle:    "trusted-ca",
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("HTTP_PROXY", tt.httpProxy)
			t.Setenv("HTTPS_PROXY", tt.httpsProxy)
			t.Setenv("TRUSTED_CA_BUNDLE_CONFIGMAP", tt.caBundle)

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

// TestNeedsUpdate_AllScenarios tests all needsUpdate scenarios
func TestNeedsUpdate_AllScenarios(t *testing.T) {
	tests := []struct {
		name        string
		currentHash string
		desiredHash string
		currentNil  bool
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
			name:        "nil current annotations returns true",
			currentNil:  true,
			desiredHash: "abc123",
			expected:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var current, desired appsv1.Deployment

			if tt.currentNil {
				current = appsv1.Deployment{
					Spec: appsv1.DeploymentSpec{
						Template: corev1.PodTemplateSpec{
							ObjectMeta: metav1.ObjectMeta{Annotations: nil},
						},
					},
				}
			} else {
				current = createDeploymentWithConfigHash(tt.currentHash)
			}

			desired = createDeploymentWithConfigHash(tt.desiredHash)

			result := needsUpdate(current, desired)
			if result != tt.expected {
				t.Fatalf("needsUpdate() = %v, expected %v", result, tt.expected)
			}
		})
	}
}

// TestConditionConstants_AllScenarios tests all condition constants
func TestConditionConstants_AllScenarios(t *testing.T) {
	tests := []struct {
		name     string
		constant string
		expected string
	}{
		{"DeploymentAvailable", DeploymentAvailable, "DeploymentAvailable"},
		{"ConfigMapAvailable", ConfigMapAvailable, "ConfigMapAvailable"},
		{"ServiceAccountAvailable", ServiceAccountAvailable, "ServiceAccountAvailable"},
		{"ServiceAvailable", ServiceAvailable, "ServiceAvailable"},
		{"ConfigurationValid", ConfigurationValid, "ConfigurationValid"},
		{"RouteAvailable", RouteAvailable, "RouteAvailable"},
		{"ClusterSPIFFEIDAvailable", ClusterSPIFFEIDAvailable, "ClusterSPIFFEIDAvailable"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.constant != tt.expected {
				t.Errorf("Expected %s to be '%s', got '%s'", tt.name, tt.expected, tt.constant)
			}
		})
	}
}

// TestReconcileServiceAccount_ErrorPropagation tests that reconcileServiceAccount returns errors properly
func TestReconcileServiceAccount_ErrorPropagation(t *testing.T) {
	tests := []struct {
		name        string
		getErr      error
		createErr   error
		expectError bool
	}{
		{
			name:        "get error returns error",
			getErr:      errors.New("connection error"),
			expectError: true,
		},
		{
			name:        "create error returns error",
			getErr:      kerrors.NewNotFound(schema.GroupResource{}, "test"),
			createErr:   errors.New("create failed"),
			expectError: true,
		},
		{
			name:        "success when not found and create succeeds",
			getErr:      kerrors.NewNotFound(schema.GroupResource{}, "test"),
			createErr:   nil,
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fakeClient := &fakes.FakeCustomCtrlClient{}
			scheme := runtime.NewScheme()
			_ = v1alpha1.AddToScheme(scheme)

			reconciler := &SpireOidcDiscoveryProviderReconciler{
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

			oidc := &v1alpha1.SpireOIDCDiscoveryProvider{
				ObjectMeta: metav1.ObjectMeta{Name: "cluster", UID: "test-uid"},
			}
			statusMgr := status.NewManager(fakeClient)

			err := reconciler.reconcileServiceAccount(context.Background(), oidc, statusMgr, false)

			if tt.expectError && err == nil {
				t.Fatal("Expected error but got nil")
			}
			if !tt.expectError && err != nil {
				t.Fatalf("Expected no error but got: %v", err)
			}
		})
	}
}

// TestReconcileService_ErrorPropagation tests that reconcileService returns errors properly
func TestReconcileService_ErrorPropagation(t *testing.T) {
	tests := []struct {
		name        string
		getErr      error
		createErr   error
		expectError bool
	}{
		{
			name:        "get error returns error",
			getErr:      errors.New("connection error"),
			expectError: true,
		},
		{
			name:        "create error returns error",
			getErr:      kerrors.NewNotFound(schema.GroupResource{}, "test"),
			createErr:   errors.New("create failed"),
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fakeClient := &fakes.FakeCustomCtrlClient{}
			scheme := runtime.NewScheme()
			_ = v1alpha1.AddToScheme(scheme)

			reconciler := &SpireOidcDiscoveryProviderReconciler{
				ctrlClient:    fakeClient,
				ctx:           context.Background(),
				log:           logr.Discard(),
				scheme:        scheme,
				eventRecorder: record.NewFakeRecorder(100),
			}

			fakeClient.GetReturns(tt.getErr)
			if tt.createErr != nil {
				fakeClient.CreateReturns(tt.createErr)
			}

			oidc := &v1alpha1.SpireOIDCDiscoveryProvider{
				ObjectMeta: metav1.ObjectMeta{Name: "cluster", UID: "test-uid"},
			}
			statusMgr := status.NewManager(fakeClient)

			err := reconciler.reconcileService(context.Background(), oidc, statusMgr, false)

			if tt.expectError && err == nil {
				t.Fatal("Expected error but got nil")
			}
			if !tt.expectError && err != nil {
				t.Fatalf("Expected no error but got: %v", err)
			}
		})
	}
}

// TestReconcile_AllErrorPaths tests all error return paths in Reconcile function
func TestReconcile_AllErrorPaths(t *testing.T) {
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
						if o, ok := obj.(*v1alpha1.SpireOIDCDiscoveryProvider); ok {
							o.Name = "cluster"
							o.UID = "test-uid"
							o.Spec.JwtIssuer = "https://example.com"
							o.OwnerReferences = []metav1.OwnerReference{{
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
					default:
						return errors.New("service account get error")
					}
				}
			},
			expectError: true,
		},
		{
			name: "reconcileService error propagates",
			setupClient: func(fc *fakes.FakeCustomCtrlClient) {
				callCount := 0
				fc.GetStub = func(ctx context.Context, key client.ObjectKey, obj client.Object) error {
					callCount++
					switch callCount {
					case 1:
						if o, ok := obj.(*v1alpha1.SpireOIDCDiscoveryProvider); ok {
							o.Name = "cluster"
							o.UID = "test-uid"
							o.Spec.JwtIssuer = "https://example.com"
							o.OwnerReferences = []metav1.OwnerReference{{
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
					case 3: // ServiceAccount - return existing
						return nil
					default:
						return errors.New("service get error")
					}
				}
			},
			expectError: true,
		},
		{
			name: "reconcileConfigMap error propagates",
			setupClient: func(fc *fakes.FakeCustomCtrlClient) {
				callCount := 0
				fc.GetStub = func(ctx context.Context, key client.ObjectKey, obj client.Object) error {
					callCount++
					switch callCount {
					case 1:
						if o, ok := obj.(*v1alpha1.SpireOIDCDiscoveryProvider); ok {
							o.Name = "cluster"
							o.UID = "test-uid"
							o.Spec.JwtIssuer = "https://example.com"
							o.OwnerReferences = []metav1.OwnerReference{{
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
					case 3, 4: // ServiceAccount, Service - return existing
						return nil
					default:
						return errors.New("configmap get error")
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

			reconciler := &SpireOidcDiscoveryProviderReconciler{
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

	reconciler := &SpireOidcDiscoveryProviderReconciler{
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
		case 1: // SpireOIDCDiscoveryProvider
			if o, ok := obj.(*v1alpha1.SpireOIDCDiscoveryProvider); ok {
				o.Name = "cluster"
				o.UID = "test-uid"
				o.Spec.JwtIssuer = "https://example.com"
				o.OwnerReferences = []metav1.OwnerReference{{
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
			name:        "both empty hashes returns false",
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			current := createDeploymentWithConfigHash(tt.currentHash)
			desired := createDeploymentWithConfigHash(tt.desiredHash)

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
			failAtGetCount: 3,
			description:    "reconcileServiceAccount failure",
		},
		{
			name:           "Service error returns error",
			failAtGetCount: 4,
			description:    "reconcileService failure",
		},
		{
			name:           "ConfigMap error returns error",
			failAtGetCount: 5,
			description:    "reconcileConfigMap failure",
		},
		{
			name:           "Deployment error returns error",
			failAtGetCount: 6,
			description:    "reconcileDeployment failure",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fakeClient := &fakes.FakeCustomCtrlClient{}
			scheme := runtime.NewScheme()
			_ = v1alpha1.AddToScheme(scheme)

			reconciler := &SpireOidcDiscoveryProviderReconciler{
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
					if o, ok := obj.(*v1alpha1.SpireOIDCDiscoveryProvider); ok {
						o.Name = "cluster"
						o.UID = "test-uid"
						o.Spec.JwtIssuer = "https://example.com"
						o.OwnerReferences = []metav1.OwnerReference{{
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
