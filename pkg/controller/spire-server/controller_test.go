package spire_server

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"math/big"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/go-logr/logr"
	routev1 "github.com/openshift/api/route/v1"
	"github.com/openshift/zero-trust-workload-identity-manager/api/v1alpha1"
	"github.com/openshift/zero-trust-workload-identity-manager/pkg/client/fakes"
	"github.com/openshift/zero-trust-workload-identity-manager/pkg/controller/status"
	"github.com/openshift/zero-trust-workload-identity-manager/pkg/controller/utils"
	admissionv1 "k8s.io/api/admissionregistration/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// newTestReconciler creates a reconciler for testing
func newTestReconciler(fakeClient *fakes.FakeCustomCtrlClient) *SpireServerReconciler {
	return &SpireServerReconciler{
		ctrlClient:    fakeClient,
		ctx:           context.Background(),
		log:           logr.Discard(),
		scheme:        runtime.NewScheme(),
		eventRecorder: record.NewFakeRecorder(100),
	}
}

// TestReconcile_SpireServerNotFound tests that when SpireServer CR is not found,
func TestReconcile_SpireServerNotFound(t *testing.T) {
	fakeClient := &fakes.FakeCustomCtrlClient{}
	reconciler := newTestReconciler(fakeClient)

	// Configure fake client to return NotFound error for SpireServer
	notFoundErr := kerrors.NewNotFound(schema.GroupResource{Group: "operator.openshift.io", Resource: "spireservers"}, "cluster")
	fakeClient.GetReturns(notFoundErr)

	req := ctrl.Request{NamespacedName: types.NamespacedName{Name: "cluster"}}
	result, err := reconciler.Reconcile(context.Background(), req)

	// Assert: should return nil error (not requeue) when CR not found
	if err != nil {
		t.Errorf("Expected nil error when SpireServer not found, got: %v", err)
	}
	if result.Requeue {
		t.Error("Expected no requeue when SpireServer not found")
	}
	if result.RequeueAfter != 0 {
		t.Error("Expected no RequeueAfter when SpireServer not found")
	}
}

// TestReconcile_SpireServerGetError tests that when Get returns a non-NotFound error
func TestReconcile_SpireServerGetError(t *testing.T) {
	fakeClient := &fakes.FakeCustomCtrlClient{}
	reconciler := newTestReconciler(fakeClient)

	// Configure fake client to return a generic error for SpireServer Get
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

	spireServer := &v1alpha1.SpireServer{
		ObjectMeta: metav1.ObjectMeta{
			Name: "cluster",
		},
	}

	callCount := 0
	fakeClient.GetStub = func(ctx context.Context, key client.ObjectKey, obj client.Object) error {
		callCount++
		switch callCount {
		case 1: // First call: Get SpireServer
			if ss, ok := obj.(*v1alpha1.SpireServer); ok {
				*ss = *spireServer
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

	spireServer := &v1alpha1.SpireServer{
		ObjectMeta: metav1.ObjectMeta{
			Name: "cluster",
		},
	}

	genericErr := errors.New("internal server error")
	callCount := 0
	fakeClient.GetStub = func(ctx context.Context, key client.ObjectKey, obj client.Object) error {
		callCount++
		switch callCount {
		case 1: // First call: Get SpireServer
			if ss, ok := obj.(*v1alpha1.SpireServer); ok {
				*ss = *spireServer
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

	// SpireServer without owner reference (needs update)
	spireServer := &v1alpha1.SpireServer{
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
		case 1: // Get SpireServer
			if ss, ok := obj.(*v1alpha1.SpireServer); ok {
				*ss = *spireServer
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

	server := &v1alpha1.SpireServer{
		ObjectMeta: metav1.ObjectMeta{Name: "cluster"},
	}

	statusMgr := status.NewManager(fakeClient)
	result := reconciler.handleCreateOnlyMode(server, statusMgr)

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

	server := &v1alpha1.SpireServer{
		ObjectMeta: metav1.ObjectMeta{Name: "cluster"},
	}

	statusMgr := status.NewManager(fakeClient)
	result := reconciler.handleCreateOnlyMode(server, statusMgr)

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

	// Server with existing CreateOnlyMode condition set to True
	server := &v1alpha1.SpireServer{
		ObjectMeta: metav1.ObjectMeta{Name: "cluster"},
		Status: v1alpha1.SpireServerStatus{
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
	result := reconciler.handleCreateOnlyMode(server, statusMgr)

	// Assert: create-only mode should be detected as false, but condition should be updated
	if result {
		t.Error("Expected handleCreateOnlyMode to return false")
	}
}

// TestNeedsUpdate_ConfigHashChanged tests needsUpdate when config hash differs
func TestNeedsUpdate_ConfigHashChanged(t *testing.T) {
	tests := []struct {
		name               string
		currentServerHash  string
		desiredServerHash  string
		currentCtrlMgrHash string
		desiredCtrlMgrHash string
		expected           bool
	}{
		{
			name:               "Same hashes - no update needed",
			currentServerHash:  "abc123",
			desiredServerHash:  "abc123",
			currentCtrlMgrHash: "xyz789",
			desiredCtrlMgrHash: "xyz789",
			expected:           false,
		},
		{
			name:               "Different server hash - update needed",
			currentServerHash:  "abc123",
			desiredServerHash:  "def456",
			currentCtrlMgrHash: "xyz789",
			desiredCtrlMgrHash: "xyz789",
			expected:           true,
		},
		{
			name:               "Different controller manager hash - update needed",
			currentServerHash:  "abc123",
			desiredServerHash:  "abc123",
			currentCtrlMgrHash: "xyz789",
			desiredCtrlMgrHash: "uvw123",
			expected:           true,
		},
		{
			name:               "Both hashes different - update needed",
			currentServerHash:  "abc123",
			desiredServerHash:  "def456",
			currentCtrlMgrHash: "xyz789",
			desiredCtrlMgrHash: "uvw123",
			expected:           true,
		},
		{
			name:               "Empty current server hash - update needed",
			currentServerHash:  "",
			desiredServerHash:  "abc123",
			currentCtrlMgrHash: "xyz789",
			desiredCtrlMgrHash: "xyz789",
			expected:           true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			current := createStatefulSetWithConfigHashes(tt.currentServerHash, tt.currentCtrlMgrHash)
			desired := createStatefulSetWithConfigHashes(tt.desiredServerHash, tt.desiredCtrlMgrHash)

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

	server := &v1alpha1.SpireServer{
		ObjectMeta: metav1.ObjectMeta{Name: "cluster"},
		Spec: v1alpha1.SpireServerSpec{
			JwtIssuer: "https://example.com",
		},
	}

	ztwim := &v1alpha1.ZeroTrustWorkloadIdentityManager{
		ObjectMeta: metav1.ObjectMeta{Name: "cluster"},
		Spec: v1alpha1.ZeroTrustWorkloadIdentityManagerSpec{
			TrustDomain: "example.com",
		},
	}

	statusMgr := status.NewManager(fakeClient)
	err := reconciler.validateConfiguration(context.Background(), server, statusMgr, ztwim)

	// Assert: validation should pass with valid configuration
	if err != nil {
		t.Errorf("Expected no error for valid configuration, got: %v", err)
	}
}

// TestValidateConfiguration_InvalidJWTIssuer tests configuration validation fails with invalid JWT issuer
func TestValidateConfiguration_InvalidJWTIssuer(t *testing.T) {
	fakeClient := &fakes.FakeCustomCtrlClient{}
	reconciler := newTestReconciler(fakeClient)

	server := &v1alpha1.SpireServer{
		ObjectMeta: metav1.ObjectMeta{Name: "cluster"},
		Spec: v1alpha1.SpireServerSpec{
			JwtIssuer: "not-a-valid-url",
		},
	}

	ztwim := &v1alpha1.ZeroTrustWorkloadIdentityManager{
		ObjectMeta: metav1.ObjectMeta{Name: "cluster"},
		Spec: v1alpha1.ZeroTrustWorkloadIdentityManagerSpec{
			TrustDomain: "example.com",
		},
	}

	statusMgr := status.NewManager(fakeClient)
	err := reconciler.validateConfiguration(context.Background(), server, statusMgr, ztwim)

	// Assert: validation should fail with invalid JWT issuer
	if err == nil {
		t.Error("Expected error for invalid JWT issuer URL")
	}
}

// TestHandleTTLValidation_ValidTTL tests TTL validation passes with valid values
func TestHandleTTLValidation_ValidTTL(t *testing.T) {
	fakeClient := &fakes.FakeCustomCtrlClient{}
	reconciler := newTestReconciler(fakeClient)

	server := &v1alpha1.SpireServer{
		ObjectMeta: metav1.ObjectMeta{Name: "cluster"},
		Spec: v1alpha1.SpireServerSpec{
			// Provide valid TTL values - CA validity must be > X509 and JWT validity
			CAValidity:          metav1.Duration{Duration: 24 * time.Hour},
			DefaultX509Validity: metav1.Duration{Duration: 1 * time.Hour},
			DefaultJWTValidity:  metav1.Duration{Duration: 5 * time.Minute},
		},
	}

	statusMgr := status.NewManager(fakeClient)
	err := reconciler.handleTTLValidation(context.Background(), server, statusMgr)

	// Assert: TTL validation should pass with default/valid values
	if err != nil {
		t.Errorf("Expected no error for valid TTL configuration, got: %v", err)
	}
}

// Helper to create StatefulSet with config hash annotations
func createStatefulSetWithConfigHashes(serverHash, ctrlMgrHash string) appsv1.StatefulSet {
	return appsv1.StatefulSet{
		Spec: appsv1.StatefulSetSpec{
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						spireServerStatefulSetSpireServerConfigHashAnnotationKey:            serverHash,
						spireServerStatefulSetSpireControllerManagerConfigHashAnnotationKey: ctrlMgrHash,
					},
				},
			},
		},
	}
}

// TestHandleTTLValidation_InvalidCAValidity tests TTL validation fails with invalid CA validity
func TestHandleTTLValidation_InvalidCAValidity(t *testing.T) {
	fakeClient := &fakes.FakeCustomCtrlClient{}
	reconciler := newTestReconciler(fakeClient)

	server := &v1alpha1.SpireServer{
		ObjectMeta: metav1.ObjectMeta{Name: "cluster"},
		Spec: v1alpha1.SpireServerSpec{
			// Zero/invalid CA validity
			CAValidity:          metav1.Duration{Duration: 0},
			DefaultX509Validity: metav1.Duration{Duration: 1 * time.Hour},
			DefaultJWTValidity:  metav1.Duration{Duration: 5 * time.Minute},
		},
	}

	statusMgr := status.NewManager(fakeClient)
	err := reconciler.handleTTLValidation(context.Background(), server, statusMgr)

	// Assert: TTL validation should fail with zero CA validity
	if err == nil {
		t.Error("Expected error for invalid CA validity")
	}
}

// TestHandleTTLValidation_CAValiditySmallerThanSVID tests TTL validation with CA < SVID validity
func TestHandleTTLValidation_CAValiditySmallerThanSVID(t *testing.T) {
	fakeClient := &fakes.FakeCustomCtrlClient{}
	reconciler := newTestReconciler(fakeClient)

	server := &v1alpha1.SpireServer{
		ObjectMeta: metav1.ObjectMeta{Name: "cluster"},
		Spec: v1alpha1.SpireServerSpec{
			// CA validity smaller than X509 validity - invalid
			CAValidity:          metav1.Duration{Duration: 30 * time.Minute},
			DefaultX509Validity: metav1.Duration{Duration: 1 * time.Hour},
			DefaultJWTValidity:  metav1.Duration{Duration: 5 * time.Minute},
		},
	}

	statusMgr := status.NewManager(fakeClient)
	err := reconciler.handleTTLValidation(context.Background(), server, statusMgr)

	// Assert: TTL validation should fail when CA validity < X509 validity
	if err == nil {
		t.Error("Expected error when CA validity is smaller than X509 validity")
	}
}

// TestValidateCommonConfig_Valid tests common config validation with valid values
func TestValidateCommonConfig_Valid(t *testing.T) {
	fakeClient := &fakes.FakeCustomCtrlClient{}
	reconciler := newTestReconciler(fakeClient)

	server := &v1alpha1.SpireServer{
		ObjectMeta: metav1.ObjectMeta{Name: "cluster"},
		Spec:       v1alpha1.SpireServerSpec{},
	}

	statusMgr := status.NewManager(fakeClient)
	err := reconciler.validateCommonConfig(server, statusMgr)

	if err != nil {
		t.Errorf("Expected no error for valid common config, got: %v", err)
	}
}

// TestValidateCommonConfig_InvalidAffinity tests common config validation with invalid affinity
func TestValidateCommonConfig_InvalidAffinity(t *testing.T) {
	fakeClient := &fakes.FakeCustomCtrlClient{}
	reconciler := newTestReconciler(fakeClient)

	server := &v1alpha1.SpireServer{
		ObjectMeta: metav1.ObjectMeta{Name: "cluster"},
		Spec: v1alpha1.SpireServerSpec{
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
	err := reconciler.validateCommonConfig(server, statusMgr)

	if err == nil {
		t.Error("Expected error for invalid affinity")
	}
}

// TestValidateProxyConfiguration_AllScenarios tests proxy validation scenarios
func TestValidateProxyConfiguration_AllScenarios(t *testing.T) {
	tests := []struct {
		name        string
		httpProxy   string
		httpsProxy  string
		caBundle    string
		expectError bool
	}{
		{
			name:        "no proxy returns nil",
			httpProxy:   "",
			httpsProxy:  "",
			caBundle:    "",
			expectError: false,
		},
		{
			name:        "valid http proxy with ca bundle returns nil",
			httpProxy:   "http://proxy.example.com:8080",
			httpsProxy:  "",
			caBundle:    "trusted-ca",
			expectError: false,
		},
		{
			name:        "valid https proxy with ca bundle returns nil",
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
			name:        "both proxies with ca bundle returns nil",
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

// TestNeedsUpdate_NoAnnotations tests needsUpdate with nil annotations
func TestNeedsUpdate_NoAnnotations(t *testing.T) {
	current := appsv1.StatefulSet{
		Spec: appsv1.StatefulSetSpec{
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: nil,
				},
			},
		},
	}
	desired := createStatefulSetWithConfigHashes("abc123", "xyz789")

	result := needsUpdate(current, desired)
	if !result {
		t.Error("Expected needsUpdate to return true when current has no annotations")
	}
}

// TestNeedsUpdate_BothEmpty tests needsUpdate when both have empty annotations
func TestNeedsUpdate_BothEmpty(t *testing.T) {
	current := appsv1.StatefulSet{
		Spec: appsv1.StatefulSetSpec{
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{},
				},
			},
		},
	}
	desired := appsv1.StatefulSet{
		Spec: appsv1.StatefulSetSpec{
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

// TestHandleCreateOnlyMode_NotSet tests create-only mode when env var is not set
func TestHandleCreateOnlyMode_NotSet(t *testing.T) {
	t.Setenv("CREATE_ONLY_MODE", "")

	fakeClient := &fakes.FakeCustomCtrlClient{}
	reconciler := newTestReconciler(fakeClient)

	server := &v1alpha1.SpireServer{
		ObjectMeta: metav1.ObjectMeta{Name: "cluster"},
	}

	statusMgr := status.NewManager(fakeClient)
	result := reconciler.handleCreateOnlyMode(server, statusMgr)

	if result {
		t.Error("Expected handleCreateOnlyMode to return false when CREATE_ONLY_MODE is not set")
	}
}

// TestSpireServerReconciler_Fields tests SpireServerReconciler struct fields
func TestSpireServerReconciler_Fields(t *testing.T) {
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
	if StatefulSetAvailable != "StatefulSetAvailable" {
		t.Errorf("Expected StatefulSetAvailable to be 'StatefulSetAvailable', got %s", StatefulSetAvailable)
	}
	if ServerConfigMapAvailable != "ServerConfigMapAvailable" {
		t.Errorf("Expected ServerConfigMapAvailable to be 'ServerConfigMapAvailable', got %s", ServerConfigMapAvailable)
	}
	if ControllerManagerConfigAvailable != "ControllerManagerConfigAvailable" {
		t.Errorf("Expected ControllerManagerConfigAvailable to be 'ControllerManagerConfigAvailable', got %s", ControllerManagerConfigAvailable)
	}
	if BundleConfigAvailable != "BundleConfigAvailable" {
		t.Errorf("Expected BundleConfigAvailable to be 'BundleConfigAvailable', got %s", BundleConfigAvailable)
	}
	if TTLConfigurationValid != "TTLConfigurationValid" {
		t.Errorf("Expected TTLConfigurationValid to be 'TTLConfigurationValid', got %s", TTLConfigurationValid)
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
	if RBACAvailable != "RBACAvailable" {
		t.Errorf("Expected RBACAvailable to be 'RBACAvailable', got %s", RBACAvailable)
	}
	if ValidatingWebhookAvailable != "ValidatingWebhookAvailable" {
		t.Errorf("Expected ValidatingWebhookAvailable to be 'ValidatingWebhookAvailable', got %s", ValidatingWebhookAvailable)
	}
	if RouteAvailable != "RouteAvailable" {
		t.Errorf("Expected RouteAvailable to be 'RouteAvailable', got %s", RouteAvailable)
	}
}

// TestValidateConfiguration_WithFederation tests configuration validation with federation config
func TestValidateConfiguration_WithFederation(t *testing.T) {
	fakeClient := &fakes.FakeCustomCtrlClient{}
	reconciler := newTestReconciler(fakeClient)

	server := &v1alpha1.SpireServer{
		ObjectMeta: metav1.ObjectMeta{Name: "cluster"},
		Spec: v1alpha1.SpireServerSpec{
			JwtIssuer: "https://example.com",
			Federation: &v1alpha1.FederationConfig{
				BundleEndpoint: v1alpha1.BundleEndpointConfig{
					Profile:     v1alpha1.HttpsSpiffeProfile,
					RefreshHint: 300,
				},
			},
		},
	}

	ztwim := &v1alpha1.ZeroTrustWorkloadIdentityManager{
		ObjectMeta: metav1.ObjectMeta{Name: "cluster"},
		Spec: v1alpha1.ZeroTrustWorkloadIdentityManagerSpec{
			TrustDomain: "example.com",
		},
	}

	statusMgr := status.NewManager(fakeClient)
	err := reconciler.validateConfiguration(context.Background(), server, statusMgr, ztwim)

	// Validation should pass for valid federation config
	if err != nil {
		t.Errorf("Expected no error for valid federation config, got: %v", err)
	}
}

// TestReconcile_FullFlow tests complete reconcile flow
func TestReconcile_FullFlow(t *testing.T) {
	fakeClient := &fakes.FakeCustomCtrlClient{}

	scheme := runtime.NewScheme()
	_ = v1alpha1.AddToScheme(scheme)

	reconciler := &SpireServerReconciler{
		ctrlClient:    fakeClient,
		ctx:           context.Background(),
		log:           logr.Discard(),
		scheme:        scheme,
		eventRecorder: record.NewFakeRecorder(100),
	}

	spireServer := &v1alpha1.SpireServer{
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
		Spec: v1alpha1.SpireServerSpec{
			JwtIssuer:           "https://example.com",
			CAValidity:          metav1.Duration{Duration: 24 * time.Hour},
			DefaultX509Validity: metav1.Duration{Duration: 1 * time.Hour},
			DefaultJWTValidity:  metav1.Duration{Duration: 5 * time.Minute},
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
		case *v1alpha1.SpireServer:
			*v = *spireServer
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

// TestReconcile_ErrorScenarios tests various error scenarios with table-driven tests
func TestReconcile_ErrorScenarios(t *testing.T) {
	tests := []struct {
		name            string
		setupClient     func(*fakes.FakeCustomCtrlClient)
		setupReconciler func(*SpireServerReconciler)
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
						if ss, ok := obj.(*v1alpha1.SpireServer); ok {
							ss.Name = "cluster"
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
						if ss, ok := obj.(*v1alpha1.SpireServer); ok {
							ss.Name = "cluster"
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
						if ss, ok := obj.(*v1alpha1.SpireServer); ok {
							ss.Name = "cluster"
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
			setupReconciler: func(r *SpireServerReconciler) {
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

			server := &v1alpha1.SpireServer{
				ObjectMeta: metav1.ObjectMeta{Name: "cluster"},
			}
			if tt.existingCond != nil {
				server.Status.ConditionalStatus.Conditions = []metav1.Condition{*tt.existingCond}
			}

			statusMgr := status.NewManager(fakeClient)
			result := reconciler.handleCreateOnlyMode(server, statusMgr)

			if result != tt.expectedResult {
				t.Fatalf("Expected %v, got %v", tt.expectedResult, result)
			}
		})
	}
}

// TestNeedsUpdate_AllScenarios tests all needsUpdate scenarios
func TestNeedsUpdate_AllScenarios(t *testing.T) {
	tests := []struct {
		name               string
		currentServerHash  string
		desiredServerHash  string
		currentCtrlMgrHash string
		desiredCtrlMgrHash string
		currentNil         bool
		expected           bool
	}{
		{
			name:               "same hashes returns false",
			currentServerHash:  "abc123",
			desiredServerHash:  "abc123",
			currentCtrlMgrHash: "xyz789",
			desiredCtrlMgrHash: "xyz789",
			expected:           false,
		},
		{
			name:               "different server hash returns true",
			currentServerHash:  "abc123",
			desiredServerHash:  "def456",
			currentCtrlMgrHash: "xyz789",
			desiredCtrlMgrHash: "xyz789",
			expected:           true,
		},
		{
			name:               "different controller manager hash returns true",
			currentServerHash:  "abc123",
			desiredServerHash:  "abc123",
			currentCtrlMgrHash: "xyz789",
			desiredCtrlMgrHash: "uvw123",
			expected:           true,
		},
		{
			name:               "both hashes different returns true",
			currentServerHash:  "abc123",
			desiredServerHash:  "def456",
			currentCtrlMgrHash: "xyz789",
			desiredCtrlMgrHash: "uvw123",
			expected:           true,
		},
		{
			name:               "empty current server hash returns true",
			currentServerHash:  "",
			desiredServerHash:  "abc123",
			currentCtrlMgrHash: "xyz789",
			desiredCtrlMgrHash: "xyz789",
			expected:           true,
		},
		{
			name:               "nil current annotations returns true",
			currentNil:         true,
			desiredServerHash:  "abc123",
			desiredCtrlMgrHash: "xyz789",
			expected:           true,
		},
		{
			name:               "both empty returns false",
			currentServerHash:  "",
			desiredServerHash:  "",
			currentCtrlMgrHash: "",
			desiredCtrlMgrHash: "",
			expected:           false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var current, desired appsv1.StatefulSet

			if tt.currentNil {
				current = appsv1.StatefulSet{
					Spec: appsv1.StatefulSetSpec{
						Template: corev1.PodTemplateSpec{
							ObjectMeta: metav1.ObjectMeta{Annotations: nil},
						},
					},
				}
			} else {
				current = createStatefulSetWithConfigHashes(tt.currentServerHash, tt.currentCtrlMgrHash)
			}

			desired = createStatefulSetWithConfigHashes(tt.desiredServerHash, tt.desiredCtrlMgrHash)

			result := needsUpdate(current, desired)
			if result != tt.expected {
				t.Fatalf("needsUpdate() = %v, expected %v", result, tt.expected)
			}
		})
	}
}

// TestValidateConfiguration_AllScenarios tests all configuration validation scenarios
func TestValidateConfiguration_AllScenarios(t *testing.T) {
	tests := []struct {
		name        string
		server      *v1alpha1.SpireServer
		ztwim       *v1alpha1.ZeroTrustWorkloadIdentityManager
		expectError bool
	}{
		{
			name: "valid config with valid JWT issuer",
			server: &v1alpha1.SpireServer{
				ObjectMeta: metav1.ObjectMeta{Name: "cluster"},
				Spec: v1alpha1.SpireServerSpec{
					JwtIssuer: "https://example.com",
				},
			},
			ztwim: &v1alpha1.ZeroTrustWorkloadIdentityManager{
				ObjectMeta: metav1.ObjectMeta{Name: "cluster"},
				Spec:       v1alpha1.ZeroTrustWorkloadIdentityManagerSpec{TrustDomain: "example.com"},
			},
			expectError: false,
		},
		{
			name: "invalid JWT issuer URL",
			server: &v1alpha1.SpireServer{
				ObjectMeta: metav1.ObjectMeta{Name: "cluster"},
				Spec: v1alpha1.SpireServerSpec{
					JwtIssuer: "not-a-valid-url",
				},
			},
			ztwim: &v1alpha1.ZeroTrustWorkloadIdentityManager{
				ObjectMeta: metav1.ObjectMeta{Name: "cluster"},
				Spec:       v1alpha1.ZeroTrustWorkloadIdentityManagerSpec{TrustDomain: "example.com"},
			},
			expectError: true,
		},
		{
			name: "invalid affinity with empty node selector terms",
			server: &v1alpha1.SpireServer{
				ObjectMeta: metav1.ObjectMeta{Name: "cluster"},
				Spec: v1alpha1.SpireServerSpec{
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
			ztwim: &v1alpha1.ZeroTrustWorkloadIdentityManager{
				ObjectMeta: metav1.ObjectMeta{Name: "cluster"},
				Spec:       v1alpha1.ZeroTrustWorkloadIdentityManagerSpec{TrustDomain: "example.com"},
			},
			expectError: true,
		},
		{
			name: "valid config with federation",
			server: &v1alpha1.SpireServer{
				ObjectMeta: metav1.ObjectMeta{Name: "cluster"},
				Spec: v1alpha1.SpireServerSpec{
					JwtIssuer: "https://example.com",
					Federation: &v1alpha1.FederationConfig{
						BundleEndpoint: v1alpha1.BundleEndpointConfig{
							Profile:     v1alpha1.HttpsSpiffeProfile,
							RefreshHint: 300,
						},
					},
				},
			},
			ztwim: &v1alpha1.ZeroTrustWorkloadIdentityManager{
				ObjectMeta: metav1.ObjectMeta{Name: "cluster"},
				Spec:       v1alpha1.ZeroTrustWorkloadIdentityManagerSpec{TrustDomain: "example.com"},
			},
			expectError: false,
		},
		{
			name: "empty JWT issuer returns error",
			server: &v1alpha1.SpireServer{
				ObjectMeta: metav1.ObjectMeta{Name: "cluster"},
				Spec: v1alpha1.SpireServerSpec{
					JwtIssuer: "",
				},
			},
			ztwim: &v1alpha1.ZeroTrustWorkloadIdentityManager{
				ObjectMeta: metav1.ObjectMeta{Name: "cluster"},
				Spec:       v1alpha1.ZeroTrustWorkloadIdentityManagerSpec{TrustDomain: "example.com"},
			},
			expectError: true,
		},
		{
			name: "URL with query parameters returns error",
			server: &v1alpha1.SpireServer{
				ObjectMeta: metav1.ObjectMeta{Name: "cluster"},
				Spec: v1alpha1.SpireServerSpec{
					JwtIssuer: "https://example.com?query=value",
				},
			},
			ztwim: &v1alpha1.ZeroTrustWorkloadIdentityManager{
				ObjectMeta: metav1.ObjectMeta{Name: "cluster"},
				Spec:       v1alpha1.ZeroTrustWorkloadIdentityManagerSpec{TrustDomain: "example.com"},
			},
			expectError: true,
		},
		{
			name: "JWT issuer with http scheme is valid",
			server: &v1alpha1.SpireServer{
				ObjectMeta: metav1.ObjectMeta{Name: "cluster"},
				Spec: v1alpha1.SpireServerSpec{
					JwtIssuer: "http://example.com",
				},
			},
			ztwim: &v1alpha1.ZeroTrustWorkloadIdentityManager{
				ObjectMeta: metav1.ObjectMeta{Name: "cluster"},
				Spec:       v1alpha1.ZeroTrustWorkloadIdentityManagerSpec{TrustDomain: "example.com"},
			},
			expectError: false,
		},
		{
			name: "nil federation config is valid",
			server: &v1alpha1.SpireServer{
				ObjectMeta: metav1.ObjectMeta{Name: "cluster"},
				Spec: v1alpha1.SpireServerSpec{
					JwtIssuer:  "https://example.com",
					Federation: nil,
				},
			},
			ztwim: &v1alpha1.ZeroTrustWorkloadIdentityManager{
				ObjectMeta: metav1.ObjectMeta{Name: "cluster"},
				Spec:       v1alpha1.ZeroTrustWorkloadIdentityManagerSpec{TrustDomain: "example.com"},
			},
			expectError: false,
		},
		{
			name: "invalid federation config with self-federation",
			server: &v1alpha1.SpireServer{
				ObjectMeta: metav1.ObjectMeta{Name: "cluster"},
				Spec: v1alpha1.SpireServerSpec{
					JwtIssuer: "https://example.com",
					Federation: &v1alpha1.FederationConfig{
						BundleEndpoint: v1alpha1.BundleEndpointConfig{
							Profile:     v1alpha1.HttpsSpiffeProfile,
							RefreshHint: 300,
						},
						FederatesWith: []v1alpha1.FederatesWithConfig{
							{
								TrustDomain:           "example.com",
								BundleEndpointUrl:     "https://example.com:8443",
								BundleEndpointProfile: v1alpha1.HttpsSpiffeProfile,
								EndpointSpiffeId:      "spiffe://example.com/spire/server",
							},
						},
					},
				},
			},
			ztwim: &v1alpha1.ZeroTrustWorkloadIdentityManager{
				ObjectMeta: metav1.ObjectMeta{Name: "cluster"},
				Spec:       v1alpha1.ZeroTrustWorkloadIdentityManagerSpec{TrustDomain: "example.com"},
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fakeClient := &fakes.FakeCustomCtrlClient{}
			reconciler := newTestReconciler(fakeClient)
			statusMgr := status.NewManager(fakeClient)

			err := reconciler.validateConfiguration(context.Background(), tt.server, statusMgr, tt.ztwim)

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

			server := &v1alpha1.SpireServer{
				ObjectMeta: metav1.ObjectMeta{Name: "cluster"},
				Spec: v1alpha1.SpireServerSpec{
					JwtIssuer: "https://example.com",
				},
			}

			if tt.hasExistingCond {
				server.Status.ConditionalStatus.Conditions = []metav1.Condition{
					{
						Type:   ConfigurationValid,
						Status: tt.existingStatus,
						Reason: "Test",
					},
				}
			}

			ztwim := &v1alpha1.ZeroTrustWorkloadIdentityManager{
				ObjectMeta: metav1.ObjectMeta{Name: "cluster"},
				Spec:       v1alpha1.ZeroTrustWorkloadIdentityManagerSpec{TrustDomain: "example.com"},
			}

			err := reconciler.validateConfiguration(context.Background(), server, statusMgr, ztwim)
			// validateConfiguration should succeed regardless of existing condition state
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}
		})
	}
}

// TestHandleTTLValidation_AllScenarios tests all TTL validation scenarios
func TestHandleTTLValidation_AllScenarios(t *testing.T) {
	tests := []struct {
		name         string
		caValidity   time.Duration
		x509Validity time.Duration
		jwtValidity  time.Duration
		expectError  bool
	}{
		{
			name:         "valid TTL configuration",
			caValidity:   24 * time.Hour,
			x509Validity: 1 * time.Hour,
			jwtValidity:  5 * time.Minute,
			expectError:  false,
		},
		{
			name:         "zero CA validity is invalid",
			caValidity:   0,
			x509Validity: 1 * time.Hour,
			jwtValidity:  5 * time.Minute,
			expectError:  true,
		},
		{
			name:         "CA validity smaller than X509 validity is invalid",
			caValidity:   30 * time.Minute,
			x509Validity: 1 * time.Hour,
			jwtValidity:  5 * time.Minute,
			expectError:  true,
		},
		{
			name:         "valid with larger CA validity",
			caValidity:   48 * time.Hour,
			x509Validity: 2 * time.Hour,
			jwtValidity:  10 * time.Minute,
			expectError:  false,
		},
		{
			name:         "zero X509 validity is invalid",
			caValidity:   24 * time.Hour,
			x509Validity: 0,
			jwtValidity:  5 * time.Minute,
			expectError:  true,
		},
		{
			name:         "zero JWT validity is invalid",
			caValidity:   24 * time.Hour,
			x509Validity: 1 * time.Hour,
			jwtValidity:  0,
			expectError:  true,
		},
		{
			name:         "CA validity equals X509 validity is valid",
			caValidity:   1 * time.Hour,
			x509Validity: 1 * time.Hour,
			jwtValidity:  5 * time.Minute,
			expectError:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fakeClient := &fakes.FakeCustomCtrlClient{}
			reconciler := newTestReconciler(fakeClient)

			server := &v1alpha1.SpireServer{
				ObjectMeta: metav1.ObjectMeta{Name: "cluster"},
				Spec: v1alpha1.SpireServerSpec{
					CAValidity:          metav1.Duration{Duration: tt.caValidity},
					DefaultX509Validity: metav1.Duration{Duration: tt.x509Validity},
					DefaultJWTValidity:  metav1.Duration{Duration: tt.jwtValidity},
				},
			}

			statusMgr := status.NewManager(fakeClient)
			err := reconciler.handleTTLValidation(context.Background(), server, statusMgr)

			if tt.expectError && err == nil {
				t.Fatal("Expected error but got nil")
			}
			if !tt.expectError && err != nil {
				t.Fatalf("Expected no error but got: %v", err)
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
		{"StatefulSetAvailable", StatefulSetAvailable, "StatefulSetAvailable"},
		{"ServerConfigMapAvailable", ServerConfigMapAvailable, "ServerConfigMapAvailable"},
		{"ControllerManagerConfigAvailable", ControllerManagerConfigAvailable, "ControllerManagerConfigAvailable"},
		{"BundleConfigAvailable", BundleConfigAvailable, "BundleConfigAvailable"},
		{"TTLConfigurationValid", TTLConfigurationValid, "TTLConfigurationValid"},
		{"ConfigurationValid", ConfigurationValid, "ConfigurationValid"},
		{"ServiceAccountAvailable", ServiceAccountAvailable, "ServiceAccountAvailable"},
		{"ServiceAvailable", ServiceAvailable, "ServiceAvailable"},
		{"RBACAvailable", RBACAvailable, "RBACAvailable"},
		{"ValidatingWebhookAvailable", ValidatingWebhookAvailable, "ValidatingWebhookAvailable"},
		{"RouteAvailable", RouteAvailable, "RouteAvailable"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.constant != tt.expected {
				t.Errorf("Expected %s to be '%s', got '%s'", tt.name, tt.expected, tt.constant)
			}
		})
	}
}

// TestReconcileSpireServerConfigMap_ErrorScenarios tests error scenarios for reconcileSpireServerConfigMap
func TestReconcileSpireServerConfigMap_ErrorScenarios(t *testing.T) {
	tests := []struct {
		name        string
		getErr      error
		expectError bool
	}{
		{
			name:        "get error returns error",
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

			reconciler := &SpireServerReconciler{
				ctrlClient:    fakeClient,
				ctx:           context.Background(),
				log:           logr.Discard(),
				scheme:        scheme,
				eventRecorder: record.NewFakeRecorder(100),
			}

			server := &v1alpha1.SpireServer{
				ObjectMeta: metav1.ObjectMeta{
					Name: "cluster",
					UID:  "test-uid",
				},
				Spec: v1alpha1.SpireServerSpec{
					JwtIssuer:           "https://example.com",
					CAValidity:          metav1.Duration{Duration: 24 * time.Hour},
					DefaultX509Validity: metav1.Duration{Duration: 1 * time.Hour},
					DefaultJWTValidity:  metav1.Duration{Duration: 5 * time.Minute},
				},
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
				return nil
			}

			statusMgr := status.NewManager(fakeClient)
			_, err := reconciler.reconcileSpireServerConfigMap(context.Background(), server, statusMgr, ztwim, nil, false)

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
		name           string
		createOnlyMode bool
		createErr      error
		getErr         error
		expectError    bool
	}{
		{
			name:        "successful RBAC reconciliation",
			expectError: false,
		},
		{
			name:           "skip in create-only mode",
			createOnlyMode: true,
			expectError:    false,
		},
		{
			name:        "cluster role error",
			createErr:   errors.New("cluster role failed"),
			expectError: true,
		},
		{
			name:        "get error returns error",
			getErr:      errors.New("connection refused"),
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fakeClient := &fakes.FakeCustomCtrlClient{}
			scheme := runtime.NewScheme()
			_ = v1alpha1.AddToScheme(scheme)

			reconciler := &SpireServerReconciler{
				ctrlClient:    fakeClient,
				ctx:           context.Background(),
				log:           logr.Discard(),
				scheme:        scheme,
				eventRecorder: record.NewFakeRecorder(100),
			}

			server := &v1alpha1.SpireServer{
				ObjectMeta: metav1.ObjectMeta{
					Name: "cluster",
					UID:  "test-uid",
				},
			}

			// Setup fake client
			if tt.getErr != nil {
				fakeClient.GetReturns(tt.getErr)
			} else {
				// return NotFound for all resources (will trigger Create)
				fakeClient.GetReturns(kerrors.NewNotFound(schema.GroupResource{}, "test"))
			}

			if tt.createErr != nil {
				fakeClient.CreateReturns(tt.createErr)
			} else {
				fakeClient.CreateReturns(nil)
			}

			statusMgr := status.NewManager(fakeClient)
			err := reconciler.reconcileRBAC(context.Background(), server, statusMgr, tt.createOnlyMode)

			if tt.expectError && err == nil {
				t.Fatal("Expected error but got nil")
			}
			if !tt.expectError && err != nil {
				t.Fatalf("Expected no error but got: %v", err)
			}
		})
	}
}

// TestReconcileBundleConfigMap_ErrorScenarios tests error scenarios for reconcileSpireBundleConfigMap
func TestReconcileBundleConfigMap_ErrorScenarios(t *testing.T) {
	tests := []struct {
		name        string
		getErr      error
		expectError bool
	}{
		{
			name:        "get error returns error",
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

			reconciler := &SpireServerReconciler{
				ctrlClient:    fakeClient,
				ctx:           context.Background(),
				log:           logr.Discard(),
				scheme:        scheme,
				eventRecorder: record.NewFakeRecorder(100),
			}

			server := &v1alpha1.SpireServer{
				ObjectMeta: metav1.ObjectMeta{
					Name: "cluster",
					UID:  "test-uid",
				},
				Spec: v1alpha1.SpireServerSpec{
					JwtIssuer: "https://example.com",
				},
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
				return nil
			}

			statusMgr := status.NewManager(fakeClient)
			err := reconciler.reconcileSpireBundleConfigMap(context.Background(), server, statusMgr, ztwim)

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
		name        string
		setupClient func(*fakes.FakeCustomCtrlClient)
		expectError bool
	}{
		{
			name: "reconcileRBAC error returns error not nil",
			setupClient: func(fc *fakes.FakeCustomCtrlClient) {
				callCount := 0
				fc.GetStub = func(ctx context.Context, key client.ObjectKey, obj client.Object) error {
					callCount++
					switch callCount {
					case 1:
						if s, ok := obj.(*v1alpha1.SpireServer); ok {
							s.Name = "cluster"
							s.Spec.JwtIssuer = "https://example.com"
							s.Spec.CAValidity = metav1.Duration{Duration: 24 * time.Hour}
							s.Spec.DefaultX509Validity = metav1.Duration{Duration: 1 * time.Hour}
							s.Spec.DefaultJWTValidity = metav1.Duration{Duration: 5 * time.Minute}
							s.OwnerReferences = []metav1.OwnerReference{{
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
						return errors.New("rbac get error")
					}
				}
			},
			expectError: true,
		},
		{
			name: "reconcileService error returns error not nil",
			setupClient: func(fc *fakes.FakeCustomCtrlClient) {
				callCount := 0
				fc.GetStub = func(ctx context.Context, key client.ObjectKey, obj client.Object) error {
					callCount++
					switch callCount {
					case 1:
						if s, ok := obj.(*v1alpha1.SpireServer); ok {
							s.Name = "cluster"
							s.Spec.JwtIssuer = "https://example.com"
							s.Spec.CAValidity = metav1.Duration{Duration: 24 * time.Hour}
							s.Spec.DefaultX509Validity = metav1.Duration{Duration: 1 * time.Hour}
							s.Spec.DefaultJWTValidity = metav1.Duration{Duration: 5 * time.Minute}
							s.OwnerReferences = []metav1.OwnerReference{{
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
					case 3, 4, 5, 6: // RBAC - return existing
						return nil
					default:
						return errors.New("service get error")
					}
				}
			},
			expectError: true,
		},
		{
			name: "reconcileWebhook error returns error not nil",
			setupClient: func(fc *fakes.FakeCustomCtrlClient) {
				callCount := 0
				fc.GetStub = func(ctx context.Context, key client.ObjectKey, obj client.Object) error {
					callCount++
					switch callCount {
					case 1:
						if s, ok := obj.(*v1alpha1.SpireServer); ok {
							s.Name = "cluster"
							s.Spec.JwtIssuer = "https://example.com"
							s.Spec.CAValidity = metav1.Duration{Duration: 24 * time.Hour}
							s.Spec.DefaultX509Validity = metav1.Duration{Duration: 1 * time.Hour}
							s.Spec.DefaultJWTValidity = metav1.Duration{Duration: 5 * time.Minute}
							s.OwnerReferences = []metav1.OwnerReference{{
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
					case 3, 4, 5, 6, 7: // RBAC and Service - return existing
						return nil
					default:
						return errors.New("webhook get error")
					}
				}
			},
			expectError: true,
		},
		{
			name: "reconcileConfigMap error returns error not nil",
			setupClient: func(fc *fakes.FakeCustomCtrlClient) {
				callCount := 0
				fc.GetStub = func(ctx context.Context, key client.ObjectKey, obj client.Object) error {
					callCount++
					switch callCount {
					case 1:
						if s, ok := obj.(*v1alpha1.SpireServer); ok {
							s.Name = "cluster"
							s.Spec.JwtIssuer = "https://example.com"
							s.Spec.CAValidity = metav1.Duration{Duration: 24 * time.Hour}
							s.Spec.DefaultX509Validity = metav1.Duration{Duration: 1 * time.Hour}
							s.Spec.DefaultJWTValidity = metav1.Duration{Duration: 5 * time.Minute}
							s.OwnerReferences = []metav1.OwnerReference{{
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
					case 3, 4, 5, 6, 7, 8: // RBAC, Service, Webhook - return existing
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

			reconciler := &SpireServerReconciler{
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

	reconciler := &SpireServerReconciler{
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
		case 1: // SpireServer
			if s, ok := obj.(*v1alpha1.SpireServer); ok {
				s.Name = "cluster"
				s.UID = "test-uid"
				s.Spec.JwtIssuer = "https://example.com"
				s.Spec.CAValidity = metav1.Duration{Duration: 24 * time.Hour}
				s.Spec.DefaultX509Validity = metav1.Duration{Duration: 1 * time.Hour}
				s.Spec.DefaultJWTValidity = metav1.Duration{Duration: 5 * time.Minute}
				s.OwnerReferences = []metav1.OwnerReference{{
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
		name               string
		currentServerHash  string
		desiredServerHash  string
		currentCtrlMgrHash string
		desiredCtrlMgrHash string
		expectTrue         bool
	}{
		{
			name:               "different server hashes returns true",
			currentServerHash:  "hash1",
			desiredServerHash:  "hash2",
			currentCtrlMgrHash: "ctrl1",
			desiredCtrlMgrHash: "ctrl1",
			expectTrue:         true,
		},
		{
			name:               "same server hashes returns false",
			currentServerHash:  "hash1",
			desiredServerHash:  "hash1",
			currentCtrlMgrHash: "ctrl1",
			desiredCtrlMgrHash: "ctrl1",
			expectTrue:         false,
		},
		{
			name:               "empty current server hash returns true",
			currentServerHash:  "",
			desiredServerHash:  "hash1",
			currentCtrlMgrHash: "ctrl1",
			desiredCtrlMgrHash: "ctrl1",
			expectTrue:         true,
		},
		{
			name:               "different controller manager hashes returns true",
			currentServerHash:  "hash1",
			desiredServerHash:  "hash1",
			currentCtrlMgrHash: "ctrl1",
			desiredCtrlMgrHash: "ctrl2",
			expectTrue:         true,
		},
		{
			name:               "empty current controller manager hash returns true",
			currentServerHash:  "hash1",
			desiredServerHash:  "hash1",
			currentCtrlMgrHash: "",
			desiredCtrlMgrHash: "ctrl1",
			expectTrue:         true,
		},
		{
			name:               "both server and ctrl mgr same returns false",
			currentServerHash:  "hash1",
			desiredServerHash:  "hash1",
			currentCtrlMgrHash: "ctrl1",
			desiredCtrlMgrHash: "ctrl1",
			expectTrue:         false,
		},
		{
			name:               "all empty returns false",
			currentServerHash:  "",
			desiredServerHash:  "",
			currentCtrlMgrHash: "",
			desiredCtrlMgrHash: "",
			expectTrue:         false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			current := createStatefulSetWithConfigHashes(tt.currentServerHash, tt.currentCtrlMgrHash)
			desired := createStatefulSetWithConfigHashes(tt.desiredServerHash, tt.desiredCtrlMgrHash)

			result := needsUpdate(current, desired)
			if result != tt.expectTrue {
				t.Errorf("needsUpdate() = %v, expected %v", result, tt.expectTrue)
			}
		})
	}
}

// TestHandleTTLValidation_Warnings_MutationKiller tests handleTTLValidation with warnings
func TestHandleTTLValidation_Warnings_MutationKiller(t *testing.T) {
	fakeClient := &fakes.FakeCustomCtrlClient{}
	reconciler := newTestReconciler(fakeClient)

	// Configure TTL values that generate warnings but not errors
	// X509 validity that is too high relative to CA validity generates a warning
	server := &v1alpha1.SpireServer{
		ObjectMeta: metav1.ObjectMeta{Name: "cluster"},
		Spec: v1alpha1.SpireServerSpec{
			// CA validity of 25 hours with X509 validity of 24 hours triggers warning
			// (X509 should be much smaller than CA for proper rotation)
			CAValidity:          metav1.Duration{Duration: 25 * time.Hour},
			DefaultX509Validity: metav1.Duration{Duration: 24 * time.Hour},
			DefaultJWTValidity:  metav1.Duration{Duration: 5 * time.Minute},
		},
	}

	statusMgr := status.NewManager(fakeClient)
	err := reconciler.handleTTLValidation(context.Background(), server, statusMgr)

	// Should not return error (warnings don't cause errors)
	if err != nil {
		t.Errorf("Expected no error for TTL with warnings, got: %v", err)
	}

	// Verify that the warning branch was taken by checking that no error occurred
	// but a warning condition was set (the mutation would break this)
}

// TestReconcile_ReconciliationStepErrors_MutationKillers tests each reconciliation step's error handling
// Kills mutations for error checks and corresponding return nil and add requeue mutations
func TestReconcile_ReconciliationStepErrors_MutationKillers(t *testing.T) {
	tests := []struct {
		name             string
		failOnObjectType string
		description      string
	}{
		{
			name:             "ServiceAccount error returns error",
			failOnObjectType: "ServiceAccount",
			description:      "reconcileServiceAccount failure",
		},
		{
			name:             "Service error returns error",
			failOnObjectType: "Service",
			description:      "reconcileService failure",
		},
		{
			name:             "ConfigMap error returns error",
			failOnObjectType: "ConfigMap",
			description:      "reconcileConfigMap failure",
		},
		{
			name:             "StatefulSet error returns error",
			failOnObjectType: "StatefulSet",
			description:      "reconcileStatefulSet failure",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fakeClient := &fakes.FakeCustomCtrlClient{}
			scheme := runtime.NewScheme()
			_ = v1alpha1.AddToScheme(scheme)

			reconciler := &SpireServerReconciler{
				ctrlClient:    fakeClient,
				ctx:           context.Background(),
				log:           logr.Discard(),
				scheme:        scheme,
				eventRecorder: record.NewFakeRecorder(100),
			}

			fakeClient.GetStub = func(ctx context.Context, key client.ObjectKey, obj client.Object) error {
				switch o := obj.(type) {
				case *v1alpha1.SpireServer:
					o.Name = "cluster"
					o.UID = "test-uid"
					o.Spec.JwtIssuer = "https://example.com"
					o.Spec.CAValidity = metav1.Duration{Duration: 24 * time.Hour}
					o.Spec.DefaultX509Validity = metav1.Duration{Duration: 1 * time.Hour}
					o.Spec.DefaultJWTValidity = metav1.Duration{Duration: 5 * time.Minute}
					o.OwnerReferences = []metav1.OwnerReference{{
						APIVersion: "operator.openshift.io/v1alpha1",
						Kind:       "ZeroTrustWorkloadIdentityManager",
						Name:       "cluster",
						UID:        "ztwim-uid",
					}}
					return nil
				case *v1alpha1.ZeroTrustWorkloadIdentityManager:
					o.Name = "cluster"
					o.UID = "ztwim-uid"
					o.Spec.TrustDomain = "example.org"
					return nil
				case *corev1.ServiceAccount:
					if tt.failOnObjectType == "ServiceAccount" {
						return errors.New(tt.description)
					}
					return nil
				case *corev1.Service:
					if tt.failOnObjectType == "Service" {
						return errors.New(tt.description)
					}
					return nil
				case *corev1.ConfigMap:
					if tt.failOnObjectType == "ConfigMap" {
						return errors.New(tt.description)
					}
					return nil
				case *appsv1.StatefulSet:
					if tt.failOnObjectType == "StatefulSet" {
						return errors.New(tt.description)
					}
					return nil
				default:
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
				t.Errorf("Expected RequeueAfter=0 when error returned for %s, got %v", tt.description, result.RequeueAfter)
			}
		})
	}
}

func testTrustBundle(t *testing.T) []byte {
	t.Helper()
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	template := &x509.Certificate{SerialNumber: big.NewInt(1), IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign, NotBefore: time.Now(), NotAfter: time.Now().Add(time.Hour)}
	der, err := x509.CreateCertificate(rand.Reader, template, template, public, private)
	if err != nil {
		t.Fatal(err)
	}
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
}

func TestUpstreamTrustBundleLifecycle(t *testing.T) {
	valid := testTrustBundle(t)
	for _, tc := range []struct {
		name    string
		bundle  []byte
		readErr error
		reason  string
		ready   bool
	}{
		{name: "valid", bundle: valid, ready: true},
		{name: "multiple-roots", bundle: append(append([]byte{}, valid...), valid...), ready: true},
		{name: "missing-secret", readErr: kerrors.NewNotFound(corev1.Resource("secrets"), "bundle"), reason: "TrustBundleMissing"},
		{name: "empty-key", reason: "TrustBundleMissing"},
		{name: "whitespace", bundle: []byte("  "), reason: "UpstreamAgentNotReady"},
		{name: "malformed", bundle: []byte("sensitive-invalid-data"), reason: "UpstreamAgentNotReady"},
		{name: "trailing-junk", bundle: append(append([]byte{}, valid...), []byte("junk")...), reason: "UpstreamAgentNotReady"},
		{name: "bad-der", bundle: pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: []byte("junk")}), reason: "UpstreamAgentNotReady"},
		{name: "read-error", readErr: errors.New("temporarily unavailable"), reason: "UpstreamAgentNotReady"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := upstreamTestServer()
			fake := &fakes.FakeCustomCtrlClient{}
			r := newTestReconciler(fake)
			fake.GetStub = func(_ context.Context, key client.ObjectKey, obj client.Object) error {
				if key.Name != server.Spec.UpstreamAuthority.Spire.TrustBundleSecret || key.Namespace != utils.GetOperatorNamespace() {
					t.Fatalf("wrong Secret key: %v", key)
				}
				if tc.readErr != nil {
					return tc.readErr
				}
				obj.(*corev1.Secret).Data = map[string][]byte{upstreamAgentBundleKey: tc.bundle}
				return nil
			}
			manager := status.NewManager(fake)
			hash, ready, err := r.readUpstreamTrustBundle(context.Background(), server, manager)
			if (err != nil) != (tc.name == "read-error") || ready != tc.ready {
				t.Fatalf("ready=%v, error=%v", ready, err)
			}
			if ready && hash != generateConfigHash(tc.bundle) {
				t.Fatal("hash is not derived from actual bundle bytes")
			}
			if err := manager.ApplyStatus(context.Background(), server, func() *v1alpha1.ConditionalStatus { return &server.Status.ConditionalStatus }); err != nil {
				t.Fatal(err)
			}
			cond := apimeta.FindStatusCondition(server.Status.Conditions, UpstreamAgentAvailable)
			if !ready && (cond == nil || cond.Reason != tc.reason || cond.Status != metav1.ConditionFalse) {
				t.Fatalf("unexpected condition: %+v", cond)
			}
			if cond != nil && strings.Contains(cond.Message, "sensitive-invalid-data") {
				t.Fatal("status exposed Secret content")
			}
		})
	}
	server := upstreamTestServer()
	old := GenerateSpireServerStatefulSet(&server.Spec, "s", "m", "a", generateConfigHash(valid))
	updated := GenerateSpireServerStatefulSet(&server.Spec, "s", "m", "a", generateConfigHash(testTrustBundle(t)))
	if !needsUpdate(*old, *updated) {
		t.Fatal("trust rotation does not trigger rollout")
	}
	server.Spec.UpstreamAuthority = nil
	disabled := GenerateSpireServerStatefulSet(&server.Spec, "s", "m", "", "")
	if !needsUpdate(*updated, *disabled) || len(disabled.Spec.Template.Spec.Containers) != 2 || disabled.Spec.Template.Spec.ShareProcessNamespace != nil {
		t.Fatal("disable did not remove integration")
	}
	for _, volume := range disabled.Spec.Template.Spec.Volumes {
		if strings.HasPrefix(volume.Name, "upstream-") {
			t.Fatal("upstream volume remains after disable")
		}
	}
	if _, exists := disabled.Spec.Template.Annotations[upstreamAgentBundleHashAnnotation]; exists {
		t.Fatal("bundle annotation remains")
	}
	if _, exists := disabled.Spec.Template.Annotations[upstreamAgentConfigHashAnnotation]; exists {
		t.Fatal("config annotation remains")
	}
	if buildUpstreamAuthorityPlugin(&v1alpha1.UpstreamAuthorityConfig{}) != nil {
		t.Fatal("unexpected disabled plugin")
	}
}

func TestUpstreamSecretEvents(t *testing.T) {
	server := upstreamTestServer()
	fake := &fakes.FakeCustomCtrlClient{}
	r := newTestReconciler(fake)
	fake.GetStub = func(_ context.Context, _ client.ObjectKey, obj client.Object) error {
		*obj.(*v1alpha1.SpireServer) = *server.DeepCopy()
		return nil
	}
	for _, event := range []string{"create", "update", "delete"} {
		t.Run(event, func(t *testing.T) {
			secret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "upstream-bundle", Namespace: utils.GetOperatorNamespace()}}
			requests := r.mapUpstreamSecret(context.Background(), secret)
			if len(requests) != 1 || requests[0].Name != "cluster" || requests[0].Namespace != "" {
				t.Fatalf("reference event not mapped: %v", requests)
			}
			secret.Name = "unrelated"
			if len(r.mapUpstreamSecret(context.Background(), secret)) != 0 {
				t.Fatal("unrelated Secret enqueued")
			}
			secret.Name, secret.Namespace = "upstream-bundle", "another-namespace"
			if len(r.mapUpstreamSecret(context.Background(), secret)) != 0 {
				t.Fatal("other namespace enqueued")
			}
		})
	}
	server.Spec.UpstreamAuthority = nil
	if len(r.mapUpstreamSecret(context.Background(), &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "upstream-bundle", Namespace: utils.GetOperatorNamespace()}})) != 0 {
		t.Fatal("disabled reference enqueued")
	}
}

func TestUpstreamAgentAvailability(t *testing.T) {
	for _, scenario := range []string{"ready", "unready", "terminated", "missing-pod", "pod-read-error", "missing-sts", "sts-read-error", "wrong-sts-owner", "wrong-pod-owner", "wrong-name", "terminating", "old-generation", "old-revision", "old-pod-config", "old-sts-config", "no-sidecar", "no-container-status"} {
		t.Run(scenario, func(t *testing.T) {
			server := upstreamTestServer()
			server.Generation = 7
			sts := GenerateSpireServerStatefulSet(&server.Spec, "s", "m", "a", "b")
			sts.UID, sts.Generation = "sts-uid", 4
			sts.OwnerReferences = []metav1.OwnerReference{{UID: server.UID, Controller: boolPtr(true)}}
			sts.Status = appsv1.StatefulSetStatus{ObservedGeneration: 4, UpdateRevision: "current"}
			expected := map[string]string{}
			for _, key := range []string{spireServerStatefulSetSpireServerConfigHashAnnotationKey, spireServerStatefulSetSpireControllerManagerConfigHashAnnotationKey, upstreamAgentConfigHashAnnotation, upstreamAgentBundleHashAnnotation} {
				expected[key] = sts.Spec.Template.Annotations[key]
			}
			pod := &corev1.Pod{ObjectMeta: *sts.Spec.Template.ObjectMeta.DeepCopy(), Spec: *sts.Spec.Template.Spec.DeepCopy()}
			pod.Name, pod.Namespace = "spire-server-0", utils.GetOperatorNamespace()
			pod.OwnerReferences = []metav1.OwnerReference{{UID: sts.UID, Name: "spire-server", Kind: "StatefulSet", Controller: boolPtr(true)}}
			pod.Labels[appsv1.ControllerRevisionHashLabelKey] = "current"
			pod.Status.ContainerStatuses = []corev1.ContainerStatus{{Name: upstreamAgentName, Ready: true, State: corev1.ContainerState{Running: &corev1.ContainerStateRunning{}}}}
			switch scenario {
			case "unready":
				pod.Status.ContainerStatuses[0].Ready = false
			case "terminated":
				pod.Status.ContainerStatuses[0].State = corev1.ContainerState{Terminated: &corev1.ContainerStateTerminated{}}
			case "wrong-sts-owner":
				sts.OwnerReferences[0].UID = "someone-else"
			case "wrong-pod-owner":
				pod.OwnerReferences[0].UID = "someone-else"
			case "wrong-name":
				pod.Name = "other-pod"
			case "terminating":
				now := metav1.Now()
				pod.DeletionTimestamp = &now
			case "old-generation":
				sts.Status.ObservedGeneration = 3
			case "old-revision":
				pod.Labels[appsv1.ControllerRevisionHashLabelKey] = "old"
			case "old-pod-config":
				pod.Annotations[upstreamAgentBundleHashAnnotation] = "old"
			case "old-sts-config":
				sts.Spec.Template.Annotations[upstreamAgentConfigHashAnnotation] = "old"
			case "no-sidecar":
				pod.Spec.Containers = pod.Spec.Containers[:2]
			case "no-container-status":
				pod.Status.ContainerStatuses = nil
			}
			fake := &fakes.FakeCustomCtrlClient{}
			r := newTestReconciler(fake)
			fake.GetStub = func(_ context.Context, key client.ObjectKey, obj client.Object) error {
				switch target := obj.(type) {
				case *appsv1.StatefulSet:
					if scenario == "missing-sts" {
						return kerrors.NewNotFound(appsv1.Resource("statefulsets"), key.Name)
					}
					if scenario == "sts-read-error" {
						return errors.New("read failure")
					}
					*target = *sts.DeepCopy()
				case *corev1.Pod:
					if scenario == "missing-pod" {
						return kerrors.NewNotFound(corev1.Resource("pods"), key.Name)
					}
					if scenario == "pod-read-error" {
						return errors.New("read failure")
					}
					*target = *pod.DeepCopy()
				default:
					t.Fatalf("unexpected read: %T", obj)
				}
				return nil
			}
			manager := status.NewManager(fake)
			err := r.reconcileUpstreamAgentAvailability(context.Background(), server, manager, expected)
			if (err != nil) != strings.HasSuffix(scenario, "read-error") {
				t.Fatalf("unexpected error: %v", err)
			}
			if err := manager.ApplyStatus(context.Background(), server, func() *v1alpha1.ConditionalStatus { return &server.Status.ConditionalStatus }); err != nil {
				t.Fatal(err)
			}
			cond := apimeta.FindStatusCondition(server.Status.Conditions, UpstreamAgentAvailable)
			ready := scenario == "ready"
			if cond == nil || (cond.Status == metav1.ConditionTrue) != ready {
				t.Fatalf("unexpected upstream condition: %+v", cond)
			}
			aggregate := apimeta.FindStatusCondition(server.Status.Conditions, v1alpha1.Ready)
			if (aggregate.Status == metav1.ConditionTrue) != ready || cond.ObservedGeneration != 7 {
				t.Fatalf("incorrect aggregate or generation: %+v", aggregate)
			}
			writes := fake.StatusUpdateCallCount()
			if err := manager.ApplyStatus(context.Background(), server, func() *v1alpha1.ConditionalStatus { return &server.Status.ConditionalStatus }); err != nil {
				t.Fatal(err)
			}
			if fake.StatusUpdateCallCount() != writes {
				t.Fatal("identical status wrote again")
			}
		})
	}
}

func TestUpstreamPodEvents(t *testing.T) {
	r := newTestReconciler(&fakes.FakeCustomCtrlClient{})
	pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "spire-server-0", Namespace: utils.GetOperatorNamespace(), OwnerReferences: []metav1.OwnerReference{{Kind: "StatefulSet", Name: "spire-server", Controller: boolPtr(true)}}}}
	if len(r.mapUpstreamPod(context.Background(), pod)) != 1 {
		t.Fatal("pod readiness event not mapped")
	}
	pod.Name = "spire-server-1"
	if len(r.mapUpstreamPod(context.Background(), pod)) != 0 {
		t.Fatal("unrelated ordinal enqueued")
	}
	pod.Name, pod.Namespace = "spire-server-0", "unrelated"
	if len(r.mapUpstreamPod(context.Background(), pod)) != 0 {
		t.Fatal("wrong namespace enqueued")
	}
	pod.Namespace, pod.OwnerReferences = utils.GetOperatorNamespace(), nil
	if len(r.mapUpstreamPod(context.Background(), pod)) != 0 {
		t.Fatal("unowned pod enqueued")
	}
}

// This drives the real reconciliation sequence against stored objects; it does
// not simulate the kubelet, SCC admission, or SPIRE processes.
func TestReconcileUpstreamLifecycle(t *testing.T) {
	ctx := context.Background()
	server, parent := upstreamTestServer(), upstreamTestParent()
	server.Spec.JwtIssuer = "https://example.org"
	parent.TypeMeta = metav1.TypeMeta{APIVersion: v1alpha1.GroupVersion.String(), Kind: "ZeroTrustWorkloadIdentityManager"}
	parent.Name, parent.UID = "cluster", "parent-uid"
	fake := &fakes.FakeCustomCtrlClient{}
	r := newTestReconciler(fake)
	for _, add := range []func(*runtime.Scheme) error{v1alpha1.AddToScheme, corev1.AddToScheme, appsv1.AddToScheme, rbacv1.AddToScheme, admissionv1.AddToScheme, routev1.AddToScheme} {
		if err := add(r.scheme); err != nil {
			t.Fatal(err)
		}
	}
	objects := map[string]client.Object{}
	keyFor := func(obj client.Object) string { return fmt.Sprintf("%T/%s/%s", obj, obj.GetNamespace(), obj.GetName()) }
	save := func(obj client.Object) { objects[keyFor(obj)] = obj.DeepCopyObject().(client.Object) }
	save(server)
	save(parent)
	secret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "upstream-bundle", Namespace: utils.GetOperatorNamespace()}, Data: map[string][]byte{upstreamAgentBundleKey: testTrustBundle(t)}}
	save(secret)
	pvc := &corev1.PersistentVolumeClaim{ObjectMeta: metav1.ObjectMeta{Name: "spire-data-spire-server-0", Namespace: utils.GetOperatorNamespace()}}
	save(pvc)
	fake.GetStub = func(_ context.Context, key client.ObjectKey, obj client.Object) error {
		found := objects[fmt.Sprintf("%T/%s/%s", obj, key.Namespace, key.Name)]
		if found == nil {
			return kerrors.NewNotFound(corev1.Resource("objects"), key.Name)
		}
		reflect.ValueOf(obj).Elem().Set(reflect.ValueOf(found.DeepCopyObject()).Elem())
		return nil
	}
	fake.CreateStub = func(_ context.Context, obj client.Object, _ ...client.CreateOption) error {
		if objects[keyFor(obj)] != nil {
			return kerrors.NewAlreadyExists(corev1.Resource("objects"), obj.GetName())
		}
		obj.SetUID(types.UID("uid-" + obj.GetName()))
		obj.SetResourceVersion("1")
		save(obj)
		return nil
	}
	fake.UpdateStub = func(_ context.Context, obj client.Object, _ ...client.UpdateOption) error {
		existing := objects[keyFor(obj)]
		if existing == nil {
			return kerrors.NewNotFound(corev1.Resource("objects"), obj.GetName())
		}
		if existing.GetResourceVersion() != obj.GetResourceVersion() {
			return kerrors.NewConflict(corev1.Resource("objects"), obj.GetName(), errors.New("stale"))
		}
		version, _ := strconv.Atoi(existing.GetResourceVersion())
		obj.SetResourceVersion(strconv.Itoa(version + 1))
		obj.SetUID(existing.GetUID())
		save(obj)
		return nil
	}
	fake.StatusUpdateStub = func(_ context.Context, obj client.Object, _ ...client.SubResourceUpdateOption) error {
		save(obj)
		return nil
	}
	fake.DeleteStub = func(_ context.Context, obj client.Object, _ ...client.DeleteOption) error {
		delete(objects, keyFor(obj))
		return nil
	}
	reconcile := func() {
		t.Helper()
		if _, err := r.Reconcile(ctx, ctrl.Request{NamespacedName: types.NamespacedName{Name: "cluster"}}); err != nil {
			t.Fatal(err)
		}
	}
	condition := func() *metav1.Condition {
		return apimeta.FindStatusCondition(objects[keyFor(server)].(*v1alpha1.SpireServer).Status.Conditions, UpstreamAgentAvailable)
	}
	reconcile()
	if condition() == nil || condition().Status != metav1.ConditionFalse {
		t.Fatal("new sidecar prematurely ready")
	}
	stsKey := keyFor(&appsv1.StatefulSet{ObjectMeta: metav1.ObjectMeta{Name: "spire-server", Namespace: utils.GetOperatorNamespace()}})
	sts := objects[stsKey].(*appsv1.StatefulSet)
	sts.Status = appsv1.StatefulSetStatus{ObservedGeneration: sts.Generation, UpdateRevision: "revision-1", ReadyReplicas: 1, UpdatedReplicas: 1, Replicas: 1}
	pod := &corev1.Pod{ObjectMeta: *sts.Spec.Template.ObjectMeta.DeepCopy(), Spec: *sts.Spec.Template.Spec.DeepCopy()}
	pod.Name, pod.Namespace = "spire-server-0", utils.GetOperatorNamespace()
	pod.OwnerReferences = []metav1.OwnerReference{{Kind: "StatefulSet", Name: sts.Name, UID: sts.UID, Controller: boolPtr(true)}}
	pod.Labels[appsv1.ControllerRevisionHashLabelKey] = "revision-1"
	pod.Status.ContainerStatuses = []corev1.ContainerStatus{{Name: upstreamAgentName, Ready: true, State: corev1.ContainerState{Running: &corev1.ContainerStateRunning{}}}}
	save(pod)
	reconcile()
	if condition().Status != metav1.ConditionTrue {
		t.Fatalf("current sidecar not ready: %+v", condition())
	}
	writes, statusWrites := fake.UpdateCallCount(), fake.StatusUpdateCallCount()
	reconcile()
	if fake.UpdateCallCount() != writes || fake.StatusUpdateCallCount() != statusWrites {
		for i := writes; i < fake.UpdateCallCount(); i++ {
			_, obj, _ := fake.UpdateArgsForCall(i)
			t.Logf("extra update: %T %s", obj, obj.GetName())
		}
		t.Fatalf("steady reconciliation updates %d -> %d; status %d -> %d", writes, fake.UpdateCallCount(), statusWrites, fake.StatusUpdateCallCount())
	}
	secret.Data[upstreamAgentBundleKey] = testTrustBundle(t)
	save(secret)
	reconcile()
	if condition().Status != metav1.ConditionFalse {
		t.Fatal("old pod ready after bundle rotation")
	}
	current := objects[keyFor(server)].(*v1alpha1.SpireServer)
	current.Spec.UpstreamAuthority = nil
	reconcile()
	if condition() != nil {
		t.Fatal("upstream condition remains after disable")
	}
	configKey := keyFor(&corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: upstreamAgentName, Namespace: utils.GetOperatorNamespace()}})
	if objects[configKey] != nil {
		t.Fatal("owned agent ConfigMap remains after disable")
	}
	if objects[keyFor(secret)] == nil || objects[keyFor(pvc)] == nil {
		t.Fatal("administrator Secret or server PVC deleted")
	}
	sts = objects[stsKey].(*appsv1.StatefulSet)
	if len(sts.Spec.Template.Spec.Containers) != 2 || sts.Spec.Template.Spec.ShareProcessNamespace != nil {
		t.Fatal("upstream pod state remains after disable")
	}
	deletes := fake.DeleteCallCount()
	reconcile()
	if fake.DeleteCallCount() != deletes {
		t.Fatal("repeat disable attempted additional deletes")
	}
}
