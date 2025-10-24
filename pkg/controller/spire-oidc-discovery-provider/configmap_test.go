package spire_oidc_discovery_provider

import (
	"encoding/json"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"

	"github.com/openshift/zero-trust-workload-identity-manager/api/v1alpha1"
	"github.com/openshift/zero-trust-workload-identity-manager/pkg/config"
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
		result, err := GenerateOIDCConfigMapFromCR(cr)

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
		result, err := GenerateOIDCConfigMapFromCR(cr)

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
		result, err := GenerateOIDCConfigMapFromCR(cr)

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

func TestBuildSpireOIDCDiscoveryProviderConfig(t *testing.T) {
	tests := []struct {
		name              string
		spec              *v1alpha1.SpireOIDCDiscoveryProviderSpec
		jwtIssuerStripped string
		validate          func(t *testing.T, cfg *config.SpireOIDCDiscoveryProviderConfig)
	}{
		{
			name: "minimal OIDC config with default agent socket",
			spec: &v1alpha1.SpireOIDCDiscoveryProviderSpec{
				TrustDomain:     "example.org",
				AgentSocketName: "", // Empty means use default
				JwtIssuer:       "https://oidc.example.org",
			},
			jwtIssuerStripped: "oidc.example.org",
			validate: func(t *testing.T, cfg *config.SpireOIDCDiscoveryProviderConfig) {
				// Validate domains
				expectedDomains := []string{
					"spire-spiffe-oidc-discovery-provider",
					"spire-spiffe-oidc-discovery-provider.zero-trust-workload-identity-manager",
					"spire-spiffe-oidc-discovery-provider.zero-trust-workload-identity-manager.svc.cluster.local",
					"oidc.example.org",
				}
				assert.Equal(t, expectedDomains, cfg.Domains)

				// Validate log settings
				assert.Equal(t, "info", cfg.LogLevel)  // Default from utils.GetLogLevelFromString
				assert.Equal(t, "text", cfg.LogFormat) // Default from utils.GetLogFormatFromString

				// Validate workload API
				assert.Equal(t, "/spiffe-workload-api/spire-agent.sock", cfg.WorkloadAPI.SocketPath)
				assert.Equal(t, "example.org", cfg.WorkloadAPI.TrustDomain)

				// Validate serving cert file
				require.NotNil(t, cfg.ServingCertFile)
				assert.Equal(t, ":8443", cfg.ServingCertFile.Addr)
				assert.Equal(t, "/etc/oidc/tls/tls.crt", cfg.ServingCertFile.CertFilePath)
				assert.Equal(t, "/etc/oidc/tls/tls.key", cfg.ServingCertFile.KeyFilePath)

				// Validate health checks
				assert.Equal(t, "8008", cfg.HealthChecks.BindPort)
				assert.Equal(t, "", cfg.HealthChecks.BindAddr)
				assert.Equal(t, "/live", cfg.HealthChecks.LivePath)
				assert.Equal(t, "/ready", cfg.HealthChecks.ReadyPath)
			},
		},
		{
			name: "OIDC config with custom agent socket name",
			spec: &v1alpha1.SpireOIDCDiscoveryProviderSpec{
				TrustDomain:     "custom.example.org",
				AgentSocketName: "custom-agent.sock",
				JwtIssuer:       "https://custom-oidc.example.org",
			},
			jwtIssuerStripped: "custom-oidc.example.org",
			validate: func(t *testing.T, cfg *config.SpireOIDCDiscoveryProviderConfig) {
				// Validate custom socket path
				assert.Equal(t, "/spiffe-workload-api/custom-agent.sock", cfg.WorkloadAPI.SocketPath)
				assert.Equal(t, "custom.example.org", cfg.WorkloadAPI.TrustDomain)

				// Validate custom domain in domains list
				assert.Contains(t, cfg.Domains, "custom-oidc.example.org")
			},
		},
		{
			name: "OIDC config with complex JWT issuer",
			spec: &v1alpha1.SpireOIDCDiscoveryProviderSpec{
				TrustDomain:     "secure.example.org",
				AgentSocketName: "spire-agent.sock",
				JwtIssuer:       "https://oidc.secure.example.org:8443",
			},
			jwtIssuerStripped: "oidc.secure.example.org:8443",
			validate: func(t *testing.T, cfg *config.SpireOIDCDiscoveryProviderConfig) {
				// Validate that port is preserved in stripped issuer
				assert.Contains(t, cfg.Domains, "oidc.secure.example.org:8443")

				// Validate trust domain
				assert.Equal(t, "secure.example.org", cfg.WorkloadAPI.TrustDomain)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := buildSpireOIDCDiscoveryProviderConfig(tt.spec, tt.jwtIssuerStripped)
			require.NotNil(t, cfg)
			tt.validate(t, cfg)
		})
	}
}

func TestGenerateOIDCConfigMapFromCR_WithStructs(t *testing.T) {
	tests := []struct {
		name         string
		cr           *v1alpha1.SpireOIDCDiscoveryProvider
		validateCM   func(t *testing.T, cm *corev1.ConfigMap)
		validateJSON func(t *testing.T, jsonData string)
		expectError  bool
		errorMsg     string
	}{
		{
			name: "valid OIDC config generates ConfigMap with correct structure",
			cr: &v1alpha1.SpireOIDCDiscoveryProvider{
				Spec: v1alpha1.SpireOIDCDiscoveryProviderSpec{
					TrustDomain:     "example.org",
					AgentSocketName: "spire-agent.sock",
					JwtIssuer:       "https://oidc.example.org",
				},
			},
			validateJSON: func(t *testing.T, jsonData string) {
				var parsed map[string]interface{}
				err := json.Unmarshal([]byte(jsonData), &parsed)
				require.NoError(t, err)

				// Validate top-level keys
				assert.Contains(t, parsed, "domains")
				assert.Contains(t, parsed, "log_level")
				assert.Contains(t, parsed, "log_format")
				assert.Contains(t, parsed, "workload_api")
				assert.Contains(t, parsed, "serving_cert_file")
				assert.Contains(t, parsed, "health_checks")

				// Validate domains
				domains := parsed["domains"].([]interface{})
				require.Len(t, domains, 4)
				assert.Contains(t, domains, "oidc.example.org")

				// Validate workload_api
				workloadAPI := parsed["workload_api"].(map[string]interface{})
				assert.Equal(t, "/spiffe-workload-api/spire-agent.sock", workloadAPI["socket_path"])
				assert.Equal(t, "example.org", workloadAPI["trust_domain"])

				// Validate serving_cert_file
				servingCert := parsed["serving_cert_file"].(map[string]interface{})
				assert.Equal(t, ":8443", servingCert["addr"])
				assert.Equal(t, "/etc/oidc/tls/tls.crt", servingCert["cert_file_path"])
				assert.Equal(t, "/etc/oidc/tls/tls.key", servingCert["key_file_path"])

				// Validate health_checks
				healthChecks := parsed["health_checks"].(map[string]interface{})
				assert.Equal(t, "8008", healthChecks["bind_port"])
				assert.Equal(t, "/live", healthChecks["live_path"])
				assert.Equal(t, "/ready", healthChecks["ready_path"])
			},
		},
		{
			name: "OIDC config with custom labels",
			cr: &v1alpha1.SpireOIDCDiscoveryProvider{
				Spec: v1alpha1.SpireOIDCDiscoveryProviderSpec{
					TrustDomain:     "custom.example.org",
					AgentSocketName: "custom-agent.sock",
					JwtIssuer:       "https://custom-oidc.example.org",
					CommonConfig: v1alpha1.CommonConfig{
						Labels: map[string]string{
							"app":     "spire-oidc",
							"env":     "production",
							"version": "v1.0",
						},
					},
				},
			},
			validateCM: func(t *testing.T, cm *corev1.ConfigMap) {
				// Validate labels are propagated
				assert.Contains(t, cm.ObjectMeta.Labels, "app")
				assert.Contains(t, cm.ObjectMeta.Labels, "env")
				assert.Contains(t, cm.ObjectMeta.Labels, "version")
			},
		},
		{
			name:        "nil CR returns error",
			cr:          nil,
			expectError: true,
			errorMsg:    "spire OIDC Discovery Provider Config is nil",
		},
		{
			name: "invalid JWT issuer returns error",
			cr: &v1alpha1.SpireOIDCDiscoveryProvider{
				Spec: v1alpha1.SpireOIDCDiscoveryProviderSpec{
					TrustDomain: "example.org",
					JwtIssuer:   "not-a-valid-url",
				},
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cm, err := GenerateOIDCConfigMapFromCR(tt.cr)

			if tt.expectError {
				assert.Error(t, err)
				if tt.errorMsg != "" {
					assert.Contains(t, err.Error(), tt.errorMsg)
				}
				return
			}

			require.NoError(t, err)
			require.NotNil(t, cm)

			// Validate ConfigMap data
			require.Contains(t, cm.Data, "oidc-discovery-provider.conf")
			oidcConfJSON := cm.Data["oidc-discovery-provider.conf"]
			assert.NotEmpty(t, oidcConfJSON)

			if tt.validateCM != nil {
				tt.validateCM(t, cm)
			}

			if tt.validateJSON != nil {
				tt.validateJSON(t, oidcConfJSON)
			}

			// Ensure JSON is valid
			var parsed map[string]interface{}
			err = json.Unmarshal([]byte(oidcConfJSON), &parsed)
			require.NoError(t, err, "Generated JSON should be valid")
		})
	}
}

func TestOIDCConfig_JSONMarshaling(t *testing.T) {
	spec := &v1alpha1.SpireOIDCDiscoveryProviderSpec{
		TrustDomain:     "example.org",
		AgentSocketName: "spire-agent.sock",
		JwtIssuer:       "https://oidc.example.org",
	}

	cfg := buildSpireOIDCDiscoveryProviderConfig(spec, "oidc.example.org")

	// Marshal to JSON
	jsonBytes, err := json.MarshalIndent(cfg, "", "  ")
	require.NoError(t, err)

	// Unmarshal back
	var unmarshaled config.SpireOIDCDiscoveryProviderConfig
	err = json.Unmarshal(jsonBytes, &unmarshaled)
	require.NoError(t, err)

	// Verify round-trip consistency
	assert.Equal(t, cfg.Domains, unmarshaled.Domains)
	assert.Equal(t, cfg.LogLevel, unmarshaled.LogLevel)
	assert.Equal(t, cfg.WorkloadAPI.SocketPath, unmarshaled.WorkloadAPI.SocketPath)
	assert.Equal(t, cfg.WorkloadAPI.TrustDomain, unmarshaled.WorkloadAPI.TrustDomain)
	assert.Equal(t, cfg.ServingCertFile.Addr, unmarshaled.ServingCertFile.Addr)
	assert.Equal(t, cfg.HealthChecks.BindPort, unmarshaled.HealthChecks.BindPort)
}
