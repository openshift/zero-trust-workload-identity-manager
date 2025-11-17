package spire_oidc_discovery_provider

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/openshift/zero-trust-workload-identity-manager/api/v1alpha1"
	"github.com/openshift/zero-trust-workload-identity-manager/pkg/controller/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerateOIDCConfigMapFromCR(t *testing.T) {
	t.Run("should generate ConfigMap with all default values", func(t *testing.T) {
		// Arrange
		cr := &v1alpha1.SpireOIDCDiscoveryProvider{
			Spec: v1alpha1.SpireOIDCDiscoveryProviderSpec{
				TrustDomain: "example.org",
				JwtIssuer:   "https://oidc-discovery.example.org",
			},
		}

		// Act
		result, err := generateOIDCConfigMapFromCR(cr)

		// Assert
		require.NoError(t, err)
		require.NotNil(t, result)

		// Verify ConfigMap metadata
		assert.Equal(t, "spire-spiffe-oidc-discovery-provider", result.ObjectMeta.Name)
		assert.Equal(t, utils.OperatorNamespace, result.ObjectMeta.Namespace)

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
			"spire-spiffe-oidc-discovery-provider.zero-trust-workload-identity-manager",
			"spire-spiffe-oidc-discovery-provider.zero-trust-workload-identity-manager.svc.cluster.local",
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
				TrustDomain:     "custom.domain.com",
				AgentSocketName: "custom-agent.sock",
				JwtIssuer:       "https://custom-jwt-issuer.example.com",
				CommonConfig: v1alpha1.CommonConfig{
					Labels: customLabels,
				},
			},
		}

		// Act
		result, err := generateOIDCConfigMapFromCR(cr)

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
			"spire-spiffe-oidc-discovery-provider.zero-trust-workload-identity-manager",
			"spire-spiffe-oidc-discovery-provider.zero-trust-workload-identity-manager.svc.cluster.local",
			"custom-jwt-issuer.example.com",
		}
		assert.Len(t, domains, len(expectedDomains))
		for i, domain := range domains {
			assert.Equal(t, expectedDomains[i], domain.(string))
		}

		// Check workload_api with custom values
		workloadAPI, ok := oidcConfig["workload_api"].(map[string]interface{})
		require.True(t, ok)
		assert.Equal(t, "/spiffe-workload-api/custom-agent.sock", workloadAPI["socket_path"])
		assert.Equal(t, "custom.domain.com", workloadAPI["trust_domain"])
	})

	t.Run("should handle empty AgentSocketName with default", func(t *testing.T) {
		// Arrange
		cr := &v1alpha1.SpireOIDCDiscoveryProvider{
			Spec: v1alpha1.SpireOIDCDiscoveryProviderSpec{
				TrustDomain:     "test.domain",
				AgentSocketName: "", // Empty should use default
			},
		}

		// Act
		result, err := generateOIDCConfigMapFromCR(cr)

		// Assert
		require.NoError(t, err)

		var oidcConfig map[string]interface{}
		err = json.Unmarshal([]byte(result.Data["oidc-discovery-provider.conf"]), &oidcConfig)
		require.NoError(t, err)

		workloadAPI := oidcConfig["workload_api"].(map[string]interface{})
		assert.Equal(t, "/spiffe-workload-api/spire-agent.sock", workloadAPI["socket_path"])
	})

	t.Run("should generate valid OIDC config structure", func(t *testing.T) {
		// Arrange
		cr := &v1alpha1.SpireOIDCDiscoveryProvider{
			Spec: v1alpha1.SpireOIDCDiscoveryProviderSpec{
				TrustDomain: "example.org",
			},
		}

		// Act
		result, err := generateOIDCConfigMapFromCR(cr)

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
		Spec: v1alpha1.SpireOIDCDiscoveryProviderSpec{
			TrustDomain: "example.org",
		},
	}

	result, err := generateOIDCConfigMapFromCR(cr)
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
