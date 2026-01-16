package spire_oidc_discovery_provider

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/go-logr/logr"
	"github.com/openshift/zero-trust-workload-identity-manager/api/v1alpha1"
	"github.com/openshift/zero-trust-workload-identity-manager/pkg/client/fakes"
	"github.com/openshift/zero-trust-workload-identity-manager/pkg/controller/status"
	"github.com/openshift/zero-trust-workload-identity-manager/pkg/controller/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// TestReconcileConfigMap tests the reconcileConfigMap function
func TestReconcileConfigMap(t *testing.T) {
	t.Run("create success", func(t *testing.T) {
		fakeClient := &fakes.FakeCustomCtrlClient{}
		reconciler := newConfigMapTestReconciler(fakeClient)

		oidc := createOIDCTestCR()
		ztwim := createOIDCTestZTWIM()
		statusMgr := status.NewManager(fakeClient)

		fakeClient.GetReturns(kerrors.NewNotFound(schema.GroupResource{}, "spire-oidc-discovery-provider"))
		fakeClient.CreateReturns(nil)

		hash, err := reconciler.reconcileConfigMap(context.Background(), oidc, statusMgr, ztwim, false)

		if err != nil {
			t.Errorf("Expected no error, got: %v", err)
		}
		if hash == "" {
			t.Error("Expected non-empty config hash")
		}
		if fakeClient.CreateCallCount() != 1 {
			t.Errorf("Expected Create to be called once, got %d", fakeClient.CreateCallCount())
		}
	})

	t.Run("create error", func(t *testing.T) {
		fakeClient := &fakes.FakeCustomCtrlClient{}
		reconciler := newConfigMapTestReconciler(fakeClient)

		oidc := createOIDCTestCR()
		ztwim := createOIDCTestZTWIM()
		statusMgr := status.NewManager(fakeClient)

		fakeClient.GetReturns(kerrors.NewNotFound(schema.GroupResource{}, "spire-oidc-discovery-provider"))
		fakeClient.CreateReturns(errors.New("create failed"))

		_, err := reconciler.reconcileConfigMap(context.Background(), oidc, statusMgr, ztwim, false)

		if err == nil {
			t.Error("Expected error when Create fails")
		}
	})

	t.Run("get error", func(t *testing.T) {
		fakeClient := &fakes.FakeCustomCtrlClient{}
		reconciler := newConfigMapTestReconciler(fakeClient)

		oidc := createOIDCTestCR()
		ztwim := createOIDCTestZTWIM()
		statusMgr := status.NewManager(fakeClient)

		fakeClient.GetReturns(errors.New("connection refused"))

		_, err := reconciler.reconcileConfigMap(context.Background(), oidc, statusMgr, ztwim, false)

		if err == nil {
			t.Error("Expected error when Get fails")
		}
	})

	t.Run("update success", func(t *testing.T) {
		fakeClient := &fakes.FakeCustomCtrlClient{}
		reconciler := newConfigMapTestReconciler(fakeClient)

		oidc := createOIDCTestCR()
		ztwim := createOIDCTestZTWIM()
		statusMgr := status.NewManager(fakeClient)

		existingCM := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:            "spire-oidc-discovery-provider",
				Namespace:       utils.GetOperatorNamespace(),
				ResourceVersion: "123",
			},
			Data: map[string]string{
				"oidc-discovery-provider.conf": "old-config",
			},
		}

		fakeClient.GetStub = func(ctx context.Context, key client.ObjectKey, obj client.Object) error {
			if cm, ok := obj.(*corev1.ConfigMap); ok {
				*cm = *existingCM
			}
			return nil
		}
		fakeClient.UpdateReturns(nil)

		hash, err := reconciler.reconcileConfigMap(context.Background(), oidc, statusMgr, ztwim, false)

		if err != nil {
			t.Errorf("Expected no error, got: %v", err)
		}
		if hash == "" {
			t.Error("Expected non-empty config hash")
		}
		if fakeClient.UpdateCallCount() != 1 {
			t.Errorf("Expected Update to be called once, got %d", fakeClient.UpdateCallCount())
		}
	})

	t.Run("update error", func(t *testing.T) {
		fakeClient := &fakes.FakeCustomCtrlClient{}
		reconciler := newConfigMapTestReconciler(fakeClient)

		oidc := createOIDCTestCR()
		ztwim := createOIDCTestZTWIM()
		statusMgr := status.NewManager(fakeClient)

		existingCM := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:            "spire-oidc-discovery-provider",
				Namespace:       utils.GetOperatorNamespace(),
				ResourceVersion: "123",
			},
			Data: map[string]string{
				"oidc-discovery-provider.conf": "old-config",
			},
		}

		fakeClient.GetStub = func(ctx context.Context, key client.ObjectKey, obj client.Object) error {
			if cm, ok := obj.(*corev1.ConfigMap); ok {
				*cm = *existingCM
			}
			return nil
		}
		fakeClient.UpdateReturns(errors.New("update conflict"))

		_, err := reconciler.reconcileConfigMap(context.Background(), oidc, statusMgr, ztwim, false)

		if err == nil {
			t.Error("Expected error when Update fails")
		}
	})

	t.Run("create only mode skips update", func(t *testing.T) {
		fakeClient := &fakes.FakeCustomCtrlClient{}
		reconciler := newConfigMapTestReconciler(fakeClient)

		oidc := createOIDCTestCR()
		ztwim := createOIDCTestZTWIM()
		statusMgr := status.NewManager(fakeClient)

		existingCM := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:            "spire-oidc-discovery-provider",
				Namespace:       utils.GetOperatorNamespace(),
				ResourceVersion: "123",
			},
			Data: map[string]string{
				"oidc-discovery-provider.conf": "old-config",
			},
		}

		fakeClient.GetStub = func(ctx context.Context, key client.ObjectKey, obj client.Object) error {
			if cm, ok := obj.(*corev1.ConfigMap); ok {
				*cm = *existingCM
			}
			return nil
		}

		hash, err := reconciler.reconcileConfigMap(context.Background(), oidc, statusMgr, ztwim, true)

		if err != nil {
			t.Errorf("Expected no error, got: %v", err)
		}
		if hash == "" {
			t.Error("Expected non-empty config hash")
		}
		if fakeClient.UpdateCallCount() != 0 {
			t.Error("Expected Update not to be called in create-only mode")
		}
	})

	t.Run("set controller reference error", func(t *testing.T) {
		fakeClient := &fakes.FakeCustomCtrlClient{}
		reconciler := &SpireOidcDiscoveryProviderReconciler{
			ctrlClient:    fakeClient,
			ctx:           context.Background(),
			log:           logr.Discard(),
			scheme:        runtime.NewScheme(), // Empty scheme causes error
			eventRecorder: record.NewFakeRecorder(100),
		}

		oidc := createOIDCTestCR()
		ztwim := createOIDCTestZTWIM()
		statusMgr := status.NewManager(fakeClient)

		_, err := reconciler.reconcileConfigMap(context.Background(), oidc, statusMgr, ztwim, false)

		if err == nil {
			t.Error("Expected error when SetControllerReference fails")
		}
	})

	t.Run("nil CR returns error", func(t *testing.T) {
		fakeClient := &fakes.FakeCustomCtrlClient{}
		reconciler := newConfigMapTestReconciler(fakeClient)

		var oidc *v1alpha1.SpireOIDCDiscoveryProvider = nil
		ztwim := createOIDCTestZTWIM()
		statusMgr := status.NewManager(fakeClient)

		_, err := reconciler.reconcileConfigMap(context.Background(), oidc, statusMgr, ztwim, false)

		if err == nil {
			t.Error("Expected error when CR is nil")
		}
	})
}

// TestGenerateOIDCConfigMapFromCR_NilConfig tests that nil config returns an error
func TestGenerateOIDCConfigMapFromCR_NilConfig(t *testing.T) {
	ztwim := createOIDCTestZTWIM()

	_, err := generateOIDCConfigMapFromCR(nil, ztwim)

	if err == nil {
		t.Error("Expected error when config is nil")
	}
}

// newConfigMapTestReconciler creates a reconciler for ConfigMap tests
func newConfigMapTestReconciler(fakeClient *fakes.FakeCustomCtrlClient) *SpireOidcDiscoveryProviderReconciler {
	scheme := runtime.NewScheme()
	_ = v1alpha1.AddToScheme(scheme)
	_ = corev1.AddToScheme(scheme)
	return &SpireOidcDiscoveryProviderReconciler{
		ctrlClient:    fakeClient,
		ctx:           context.Background(),
		log:           logr.Discard(),
		scheme:        scheme,
		eventRecorder: record.NewFakeRecorder(100),
	}
}

// createOIDCTestCR creates a test SpireOIDCDiscoveryProvider for tests
func createOIDCTestCR() *v1alpha1.SpireOIDCDiscoveryProvider {
	return &v1alpha1.SpireOIDCDiscoveryProvider{
		ObjectMeta: metav1.ObjectMeta{
			Name: "cluster",
			UID:  "test-uid",
		},
		Spec: v1alpha1.SpireOIDCDiscoveryProviderSpec{
			JwtIssuer: "https://oidc.example.org",
		},
	}
}

// createOIDCTestZTWIM creates a test ZeroTrustWorkloadIdentityManager for tests
func createOIDCTestZTWIM() *v1alpha1.ZeroTrustWorkloadIdentityManager {
	return &v1alpha1.ZeroTrustWorkloadIdentityManager{
		ObjectMeta: metav1.ObjectMeta{
			Name: "cluster",
		},
		Spec: v1alpha1.ZeroTrustWorkloadIdentityManagerSpec{
			TrustDomain:     "example.org",
			ClusterName:     "test-cluster",
			BundleConfigMap: "spire-bundle",
		},
	}
}

func TestGenerateOIDCConfigMapFromCR(t *testing.T) {
	t.Run("should generate ConfigMap with all default values", func(t *testing.T) {
		// Arrange
		cr := &v1alpha1.SpireOIDCDiscoveryProvider{
			Spec: v1alpha1.SpireOIDCDiscoveryProviderSpec{
				JwtIssuer: "https://oidc-discovery.example.org",
			},
		}

		ztwim := &v1alpha1.ZeroTrustWorkloadIdentityManager{
			Spec: v1alpha1.ZeroTrustWorkloadIdentityManagerSpec{
				TrustDomain:     "example.org",
				BundleConfigMap: "spire-bundle",
			},
		}

		// Act
		result, err := generateOIDCConfigMapFromCR(cr, ztwim)

		// Assert
		require.NoError(t, err)
		require.NotNil(t, result)

		// Verify ConfigMap metadata
		assert.Equal(t, "spire-spiffe-oidc-discovery-provider", result.ObjectMeta.Name)
		assert.Equal(t, utils.GetOperatorNamespace(), result.ObjectMeta.Namespace)

		// Verify ConfigMap data keys exist
		require.Contains(t, result.Data, "oidc-discovery-provider.conf")

		// Verify OIDC config JSON
		var oidcConfig map[string]interface{}
		err = json.Unmarshal([]byte(result.Data["oidc-discovery-provider.conf"]), &oidcConfig)
		require.NoError(t, err)

		// Check domains
		domains, ok := oidcConfig["domains"].([]interface{})
		require.True(t, ok)
		expectedDomains := []string{
			"spire-spiffe-oidc-discovery-provider",
			fmt.Sprintf("spire-spiffe-oidc-discovery-provider.%s", utils.GetOperatorNamespace()),
			fmt.Sprintf("spire-spiffe-oidc-discovery-provider.%s.svc.cluster.local", utils.GetOperatorNamespace()),
			"oidc-discovery.example.org", // Default JWT issuer
		}
		assert.Len(t, domains, len(expectedDomains))
		for i, domain := range domains {
			assert.Equal(t, expectedDomains[i], domain.(string))
		}

		// Check workload_api with default agent socket
		workloadAPI, ok := oidcConfig["workload_api"].(map[string]interface{})
		require.True(t, ok)
		assert.Equal(t, "/spiffe-workload-api/spire-agent.sock", workloadAPI["socket_path"])
		assert.Equal(t, "example.org", workloadAPI["trust_domain"])
	})

	t.Run("should generate ConfigMap with custom values", func(t *testing.T) {
		// Arrange
		customLabels := map[string]string{
			"app":     "spire-oidc",
			"version": "v1.0",
		}
		cr := &v1alpha1.SpireOIDCDiscoveryProvider{
			Spec: v1alpha1.SpireOIDCDiscoveryProviderSpec{
				JwtIssuer: "https://custom-jwt-issuer.example.com",
				CommonConfig: v1alpha1.CommonConfig{
					Labels: customLabels,
				},
			},
		}

		ztwim := &v1alpha1.ZeroTrustWorkloadIdentityManager{
			Spec: v1alpha1.ZeroTrustWorkloadIdentityManagerSpec{
				TrustDomain:     "custom.domain.com",
				BundleConfigMap: "spire-bundle",
			},
		}

		// Act
		result, err := generateOIDCConfigMapFromCR(cr, ztwim)

		// Assert
		require.NoError(t, err)
		require.NotNil(t, result)
		expectedLabels := utils.SpireOIDCDiscoveryProviderLabels(customLabels)

		// Verify ConfigMap metadata with custom labels
		assert.Equal(t, expectedLabels, result.ObjectMeta.Labels)

		// Verify OIDC config JSON with custom values
		var oidcConfig map[string]interface{}
		err = json.Unmarshal([]byte(result.Data["oidc-discovery-provider.conf"]), &oidcConfig)
		require.NoError(t, err)

		// Check domains with custom JWT issuer
		domains, ok := oidcConfig["domains"].([]interface{})
		require.True(t, ok)
		expectedDomains := []string{
			"spire-spiffe-oidc-discovery-provider",
			fmt.Sprintf("spire-spiffe-oidc-discovery-provider.%s", utils.GetOperatorNamespace()),
			fmt.Sprintf("spire-spiffe-oidc-discovery-provider.%s.svc.cluster.local", utils.GetOperatorNamespace()),
			"custom-jwt-issuer.example.com",
		}
		assert.Len(t, domains, len(expectedDomains))
		for i, domain := range domains {
			assert.Equal(t, expectedDomains[i], domain.(string))
		}

		// Check workload_api - socket filename is always hardcoded to match SPIRE Agent
		workloadAPI, ok := oidcConfig["workload_api"].(map[string]interface{})
		require.True(t, ok)
		assert.Equal(t, "/spiffe-workload-api/spire-agent.sock", workloadAPI["socket_path"])
		assert.Equal(t, "custom.domain.com", workloadAPI["trust_domain"])
	})

	t.Run("should generate valid OIDC config structure", func(t *testing.T) {
		// Arrange
		cr := &v1alpha1.SpireOIDCDiscoveryProvider{
			Spec: v1alpha1.SpireOIDCDiscoveryProviderSpec{},
		}

		ztwim := &v1alpha1.ZeroTrustWorkloadIdentityManager{
			Spec: v1alpha1.ZeroTrustWorkloadIdentityManagerSpec{
				TrustDomain: "example.org",
			},
		}

		// Act
		result, err := generateOIDCConfigMapFromCR(cr, ztwim)

		// Assert
		require.NoError(t, err)

		var oidcConfig map[string]interface{}
		err = json.Unmarshal([]byte(result.Data["oidc-discovery-provider.conf"]), &oidcConfig)
		require.NoError(t, err)

		// Verify all expected top-level keys exist
		assert.Contains(t, oidcConfig, "domains")
		assert.Contains(t, oidcConfig, "health_checks")
		assert.Contains(t, oidcConfig, "log_level")
		assert.Contains(t, oidcConfig, "serving_cert_file")
		assert.Contains(t, oidcConfig, "workload_api")

		// Verify health_checks structure
		healthChecks := oidcConfig["health_checks"].(map[string]interface{})
		assert.Equal(t, "8008", healthChecks["bind_port"])
		assert.Equal(t, "/live", healthChecks["live_path"])
		assert.Equal(t, "/ready", healthChecks["ready_path"])

		// Verify log_level
		assert.Equal(t, "info", oidcConfig["log_level"])
		assert.Equal(t, "text", oidcConfig["log_format"])

		// Verify serving_cert_file structure
		servingCertFile := oidcConfig["serving_cert_file"].(map[string]interface{})
		assert.Equal(t, ":8443", servingCertFile["addr"])
		assert.Equal(t, "/etc/oidc/tls/tls.crt", servingCertFile["cert_file_path"])
		assert.Equal(t, "/etc/oidc/tls/tls.key", servingCertFile["key_file_path"])
	})

}

// Test to verify JSON formatting
func TestOIDCConfigJSONFormatting(t *testing.T) {
	cr := &v1alpha1.SpireOIDCDiscoveryProvider{
		Spec: v1alpha1.SpireOIDCDiscoveryProviderSpec{},
	}

	ztwim := &v1alpha1.ZeroTrustWorkloadIdentityManager{
		Spec: v1alpha1.ZeroTrustWorkloadIdentityManagerSpec{
			TrustDomain:     "example.org",
			BundleConfigMap: "spire-bundle",
		},
	}

	result, err := generateOIDCConfigMapFromCR(cr, ztwim)
	require.NoError(t, err)

	oidcJSON := result.Data["oidc-discovery-provider.conf"]

	// Verify it's properly formatted JSON (indented)
	assert.True(t, strings.Contains(oidcJSON, "\n"))
	assert.True(t, strings.Contains(oidcJSON, "  ")) // Should contain spaces for indentation

	// Verify it's valid JSON
	var temp interface{}
	err = json.Unmarshal([]byte(oidcJSON), &temp)
	assert.NoError(t, err)
}
