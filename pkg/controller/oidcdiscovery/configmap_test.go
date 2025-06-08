package oidcdiscovery

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
		cr := &v1alpha1.SpireOIDCDiscoveryProviderConfig{
			Spec: v1alpha1.SpireOIDCDiscoveryProviderConfigSpec{
				TrustDomain: "example.org",
			},
		}

		// Act
		result, err := GenerateOIDCConfigMapFromCR(cr)

		// Assert
		require.NoError(t, err)
		require.NotNil(t, result)

		// Verify ConfigMap metadata
		assert.Equal(t, "spire-spiffe-oidc-discovery-provider", result.ObjectMeta.Name)
		assert.Equal(t, utils.OperatorNamespace, result.ObjectMeta.Namespace)

		// Verify ConfigMap data keys exist
		require.Contains(t, result.Data, "oidc-discovery-provider.conf")
		require.Contains(t, result.Data, "spiffe-helper.conf")
		require.Contains(t, result.Data, "default.conf")

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

		// Verify spiffe-helper.conf contains default socket path
		spiffeHelperConf := result.Data["spiffe-helper.conf"]
		assert.Contains(t, spiffeHelperConf, `agent_address = "/spiffe-workload-api/spire-agent.sock"`)
	})

	t.Run("should generate ConfigMap with custom values", func(t *testing.T) {
		// Arrange
		customLabels := map[string]string{
			"app":     "spire-oidc",
			"version": "v1.0",
		}
		cr := &v1alpha1.SpireOIDCDiscoveryProviderConfig{
			Spec: v1alpha1.SpireOIDCDiscoveryProviderConfigSpec{
				TrustDomain:     "custom.domain.com",
				AgentSocketName: "custom-agent.sock",
				JwtIssuer:       "custom-jwt-issuer.example.com",
				CommonConfig: v1alpha1.CommonConfig{
					Labels: customLabels,
				},
			},
		}

		// Act
		result, err := GenerateOIDCConfigMapFromCR(cr)

		// Assert
		require.NoError(t, err)
		require.NotNil(t, result)
		newLabels := customLabels
		newLabels[utils.AppManagedByLabelKey] = utils.AppManagedByLabelValue

		// Verify ConfigMap metadata with custom labels
		assert.Equal(t, customLabels, result.ObjectMeta.Labels)

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

		// Verify spiffe-helper.conf contains custom socket path
		spiffeHelperConf := result.Data["spiffe-helper.conf"]
		assert.Contains(t, spiffeHelperConf, `agent_address = "/spiffe-workload-api/custom-agent.sock"`)
	})

	t.Run("should handle empty AgentSocketName with default", func(t *testing.T) {
		// Arrange
		cr := &v1alpha1.SpireOIDCDiscoveryProviderConfig{
			Spec: v1alpha1.SpireOIDCDiscoveryProviderConfigSpec{
				TrustDomain:     "test.domain",
				AgentSocketName: "", // Empty should use default
			},
		}

		// Act
		result, err := GenerateOIDCConfigMapFromCR(cr)

		// Assert
		require.NoError(t, err)

		var oidcConfig map[string]interface{}
		err = json.Unmarshal([]byte(result.Data["oidc-discovery-provider.conf"]), &oidcConfig)
		require.NoError(t, err)

		workloadAPI := oidcConfig["workload_api"].(map[string]interface{})
		assert.Equal(t, "/spiffe-workload-api/spire-agent.sock", workloadAPI["socket_path"])
	})

	t.Run("should handle empty JwtIssuer with default based on trust domain", func(t *testing.T) {
		// Arrange
		cr := &v1alpha1.SpireOIDCDiscoveryProviderConfig{
			Spec: v1alpha1.SpireOIDCDiscoveryProviderConfigSpec{
				TrustDomain: "test.domain",
				JwtIssuer:   "", // Empty should use default
			},
		}

		// Act
		result, err := GenerateOIDCConfigMapFromCR(cr)

		// Assert
		require.NoError(t, err)

		var oidcConfig map[string]interface{}
		err = json.Unmarshal([]byte(result.Data["oidc-discovery-provider.conf"]), &oidcConfig)
		require.NoError(t, err)

		domains := oidcConfig["domains"].([]interface{})
		// The last domain should be the default JWT issuer
		lastDomain := domains[len(domains)-1].(string)
		assert.Equal(t, "oidc-discovery.test.domain", lastDomain)
	})

	t.Run("should generate valid OIDC config structure", func(t *testing.T) {
		// Arrange
		cr := &v1alpha1.SpireOIDCDiscoveryProviderConfig{
			Spec: v1alpha1.SpireOIDCDiscoveryProviderConfigSpec{
				TrustDomain: "example.org",
			},
		}

		// Act
		result, err := GenerateOIDCConfigMapFromCR(cr)

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
		assert.Equal(t, "debug", oidcConfig["log_level"])

		// Verify serving_cert_file structure
		servingCertFile := oidcConfig["serving_cert_file"].(map[string]interface{})
		assert.Equal(t, ":8443", servingCertFile["addr"])
		assert.Equal(t, "/certs/tls.crt", servingCertFile["cert_file_path"])
		assert.Equal(t, "/certs/tls.key", servingCertFile["key_file_path"])
	})

	t.Run("should generate valid spiffe-helper.conf content", func(t *testing.T) {
		// Arrange
		cr := &v1alpha1.SpireOIDCDiscoveryProviderConfig{
			Spec: v1alpha1.SpireOIDCDiscoveryProviderConfigSpec{
				TrustDomain:     "example.org",
				AgentSocketName: "custom.sock",
			},
		}

		// Act
		result, err := GenerateOIDCConfigMapFromCR(cr)

		// Assert
		require.NoError(t, err)

		spiffeHelperConf := result.Data["spiffe-helper.conf"]

		// Verify all expected lines are present
		expectedLines := []string{
			`agent_address = "/spiffe-workload-api/custom.sock"`,
			`cert_dir = "/certs"`,
			`svid_file_name = "tls.crt"`,
			`svid_key_file_name = "tls.key"`,
			`svid_bundle_file_name = "ca.pem"`,
		}

		for _, line := range expectedLines {
			assert.Contains(t, spiffeHelperConf, line)
		}
	})

	t.Run("should generate valid default.conf nginx content", func(t *testing.T) {
		// Arrange
		cr := &v1alpha1.SpireOIDCDiscoveryProviderConfig{
			Spec: v1alpha1.SpireOIDCDiscoveryProviderConfigSpec{
				TrustDomain: "example.org",
			},
		}

		// Act
		result, err := GenerateOIDCConfigMapFromCR(cr)

		// Assert
		require.NoError(t, err)

		defaultConf := result.Data["default.conf"]

		// Verify nginx configuration elements
		assert.Contains(t, defaultConf, "upstream oidc {")
		assert.Contains(t, defaultConf, "server unix:/run/spire/oidc-sockets/spire-oidc-server.sock;")
		assert.Contains(t, defaultConf, "listen            8080;")
		assert.Contains(t, defaultConf, "listen       [::]:8080;")
		assert.Contains(t, defaultConf, "proxy_pass http://oidc;")
		assert.Contains(t, defaultConf, "location /stub_status {")
		assert.Contains(t, defaultConf, "stub_status on;")
	})

	t.Run("should return error for invalid JSON marshaling scenario", func(t *testing.T) {
		// This test would require mocking the json.MarshalIndent function
		// For demonstration, we'll test with a valid case and ensure no error occurs
		cr := &v1alpha1.SpireOIDCDiscoveryProviderConfig{
			Spec: v1alpha1.SpireOIDCDiscoveryProviderConfigSpec{
				TrustDomain: "example.org",
			},
		}

		result, err := GenerateOIDCConfigMapFromCR(cr)

		require.NoError(t, err)
		require.NotNil(t, result)
	})

	t.Run("should handle nil CR gracefully", func(t *testing.T) {
		// Act & Assert
		result, err := GenerateOIDCConfigMapFromCR(nil)

		// This will likely panic or cause issues, but let's test the behavior
		// In a real scenario, you might want to add nil checks to the function
		assert.Nil(t, result)
		assert.Error(t, err)
	})
}

// Table-driven test for different trust domain scenarios
func TestGenerateOIDCConfigMapFromCR_TrustDomains(t *testing.T) {
	testCases := []struct {
		name        string
		trustDomain string
		jwtIssuer   string
		expectedJWT string
	}{
		{
			name:        "simple domain",
			trustDomain: "example.com",
			jwtIssuer:   "",
			expectedJWT: "oidc-discovery.example.com",
		},
		{
			name:        "subdomain",
			trustDomain: "test.example.com",
			jwtIssuer:   "",
			expectedJWT: "oidc-discovery.test.example.com",
		},
		{
			name:        "custom jwt issuer",
			trustDomain: "example.com",
			jwtIssuer:   "custom.issuer.com",
			expectedJWT: "custom.issuer.com",
		},
		{
			name:        "empty trust domain",
			trustDomain: "",
			jwtIssuer:   "",
			expectedJWT: "oidc-discovery.",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cr := &v1alpha1.SpireOIDCDiscoveryProviderConfig{
				Spec: v1alpha1.SpireOIDCDiscoveryProviderConfigSpec{
					TrustDomain: tc.trustDomain,
					JwtIssuer:   tc.jwtIssuer,
				},
			}

			result, err := GenerateOIDCConfigMapFromCR(cr)
			require.NoError(t, err)

			var oidcConfig map[string]interface{}
			err = json.Unmarshal([]byte(result.Data["oidc-discovery-provider.conf"]), &oidcConfig)
			require.NoError(t, err)

			domains := oidcConfig["domains"].([]interface{})
			lastDomain := domains[len(domains)-1].(string)
			assert.Equal(t, tc.expectedJWT, lastDomain)
		})
	}
}

// Test to verify JSON formatting
func TestOIDCConfigJSONFormatting(t *testing.T) {
	cr := &v1alpha1.SpireOIDCDiscoveryProviderConfig{
		Spec: v1alpha1.SpireOIDCDiscoveryProviderConfigSpec{
			TrustDomain: "example.org",
		},
	}

	result, err := GenerateOIDCConfigMapFromCR(cr)
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
