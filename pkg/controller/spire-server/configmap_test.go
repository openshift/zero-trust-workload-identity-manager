package spire_server

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/openshift/zero-trust-workload-identity-manager/api/v1alpha1"
	"github.com/openshift/zero-trust-workload-identity-manager/pkg/config"
	"github.com/openshift/zero-trust-workload-identity-manager/pkg/controller/utils"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildSpireServerConfig(t *testing.T) {
	tests := []struct {
		name     string
		spec     *v1alpha1.SpireServerSpec
		validate func(t *testing.T, cfg *config.SpireServerConfig)
	}{
		{
			name: "minimal server config with required fields",
			spec: &v1alpha1.SpireServerSpec{
				TrustDomain:         "example.org",
				ClusterName:         "test-cluster",
				BundleConfigMap:     "spire-bundle",
				JwtIssuer:           "https://oidc.example.org",
				CAValidity:          metav1.Duration{Duration: 24 * 3600 * 1e9},
				DefaultX509Validity: metav1.Duration{Duration: 3600 * 1e9},
				DefaultJWTValidity:  metav1.Duration{Duration: 300 * 1e9},
				CASubject: &v1alpha1.CASubject{
					Country:      "US",
					Organization: "Example Org",
					CommonName:   "Example CA",
				},
				Datastore: &v1alpha1.DataStore{
					DatabaseType:     "sqlite3",
					ConnectionString: "/run/spire/data/datastore.sqlite3",
					MaxOpenConns:     100,
					MaxIdleConns:     2,
				},
			},
			validate: func(t *testing.T, cfg *config.SpireServerConfig) {
				// Validate server config
				assert.Equal(t, "example.org", cfg.Server.TrustDomain)
				assert.Equal(t, "0.0.0.0", cfg.Server.BindAddress)
				assert.Equal(t, "8081", cfg.Server.BindPort)
				assert.Equal(t, "/run/spire/data", cfg.Server.DataDir)
				assert.Equal(t, "info", cfg.Server.LogLevel)  // Default from utils.GetLogLevelFromString
				assert.Equal(t, "text", cfg.Server.LogFormat) // Default from utils.GetLogFormatFromString
				assert.Equal(t, "ec-p256", cfg.Server.CAKeyType)
				assert.Equal(t, "https://oidc.example.org", cfg.Server.JWTIssuer)
				assert.False(t, cfg.Server.AuditLogEnabled)

				// Validate CA subject
				require.Len(t, cfg.Server.CASubject, 1)
				assert.Equal(t, []string{"US"}, cfg.Server.CASubject[0].Country)
				assert.Equal(t, []string{"Example Org"}, cfg.Server.CASubject[0].Organization)
				assert.Equal(t, "Example CA", cfg.Server.CASubject[0].CommonName)

				// Validate health checks
				assert.Equal(t, "0.0.0.0", cfg.HealthChecks.BindAddress)
				assert.Equal(t, "8080", cfg.HealthChecks.BindPort)
				assert.True(t, cfg.HealthChecks.ListenerEnabled)

				// Validate telemetry
				require.NotNil(t, cfg.Telemetry)
				require.NotNil(t, cfg.Telemetry.Prometheus)
				assert.Equal(t, "0.0.0.0", cfg.Telemetry.Prometheus.Host)
				assert.Equal(t, "9402", cfg.Telemetry.Prometheus.Port)

				// Validate plugins
				require.Len(t, cfg.Plugins.DataStore, 1)
				require.Len(t, cfg.Plugins.KeyManager, 1)
				require.Len(t, cfg.Plugins.NodeAttestor, 1)
				require.Len(t, cfg.Plugins.Notifier, 1)
			},
		},
		{
			name: "server config with memory key manager",
			spec: &v1alpha1.SpireServerSpec{
				TrustDomain:         "example.org",
				ClusterName:         "test-cluster",
				BundleConfigMap:     "spire-bundle",
				JwtIssuer:           "https://oidc.example.org",
				CAValidity:          metav1.Duration{Duration: 24 * 3600 * 1e9},
				DefaultX509Validity: metav1.Duration{Duration: 3600 * 1e9},
				DefaultJWTValidity:  metav1.Duration{Duration: 300 * 1e9},
				CASubject: &v1alpha1.CASubject{
					Country:      "US",
					Organization: "Example Org",
					CommonName:   "Example CA",
				},
				KeyManager: &v1alpha1.KeyManager{
					MemoryEnabled: "true",
				},
				Datastore: &v1alpha1.DataStore{
					DatabaseType:     "sqlite3",
					ConnectionString: "/run/spire/data/datastore.sqlite3",
				},
			},
			validate: func(t *testing.T, cfg *config.SpireServerConfig) {
				require.Len(t, cfg.Plugins.KeyManager, 1)
				memoryPlugin, ok := cfg.Plugins.KeyManager[0]["memory"]
				require.True(t, ok, "Memory key manager plugin should be present")
				assert.Nil(t, memoryPlugin.PluginData)
			},
		},
		{
			name: "server config with PostgreSQL datastore and TLS",
			spec: &v1alpha1.SpireServerSpec{
				TrustDomain:         "secure.example.org",
				ClusterName:         "production-cluster",
				BundleConfigMap:     "spire-bundle",
				JwtIssuer:           "https://oidc.secure.example.org",
				CAValidity:          metav1.Duration{Duration: 24 * 3600 * 1e9},
				DefaultX509Validity: metav1.Duration{Duration: 3600 * 1e9},
				DefaultJWTValidity:  metav1.Duration{Duration: 300 * 1e9},
				CASubject: &v1alpha1.CASubject{
					Country:      "US",
					Organization: "Secure Org",
					CommonName:   "Secure CA",
				},
				Datastore: &v1alpha1.DataStore{
					DatabaseType:     "postgres",
					ConnectionString: "postgresql://user:password@postgres:5432/spire",
					MaxOpenConns:     50,
					MaxIdleConns:     10,
					ConnMaxLifetime:  3600,
					DisableMigration: "false",
					RootCAPath:       "/etc/ssl/certs/ca.pem",
					ClientCertPath:   "/etc/ssl/certs/client.crt",
					ClientKeyPath:    "/etc/ssl/private/client.key",
					Options:          []string{"sslmode=require"},
				},
			},
			validate: func(t *testing.T, cfg *config.SpireServerConfig) {
				require.Len(t, cfg.Plugins.DataStore, 1)
				sqlPlugin, ok := cfg.Plugins.DataStore[0]["sql"]
				require.True(t, ok)

				dsData, ok := sqlPlugin.PluginData.(config.DataStorePluginData)
				require.True(t, ok)
				assert.Equal(t, "postgres", dsData.DatabaseType)
				assert.Equal(t, "postgresql://user:password@postgres:5432/spire", dsData.ConnectionString)
				assert.Equal(t, 50, dsData.MaxOpenConns)
				assert.Equal(t, 10, dsData.MaxIdleConns)
				assert.Equal(t, "3600s", dsData.ConnMaxLifetime) // Should be formatted as duration string
				assert.False(t, dsData.DisableMigration)
				assert.Equal(t, "/etc/ssl/certs/ca.pem", dsData.RootCAPath)
				assert.Equal(t, "/etc/ssl/certs/client.crt", dsData.ClientCertPath)
				assert.Equal(t, "/etc/ssl/private/client.key", dsData.ClientKeyPath)
				assert.Equal(t, []string{"sslmode=require"}, dsData.Options)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := buildSpireServerConfig(tt.spec)
			require.NotNil(t, cfg)
			tt.validate(t, cfg)
		})
	}
}

func TestGenerateSpireServerConfigMap(t *testing.T) {
	tests := []struct {
		name         string
		spec         *v1alpha1.SpireServerSpec
		validateCM   func(t *testing.T, cm *corev1.ConfigMap)
		validateJSON func(t *testing.T, jsonData string)
		expectError  bool
	}{
		{
			name: "valid server config generates ConfigMap",
			spec: &v1alpha1.SpireServerSpec{
				TrustDomain:         "example.org",
				ClusterName:         "test-cluster",
				BundleConfigMap:     "spire-bundle",
				JwtIssuer:           "https://oidc.example.org",
				CAValidity:          metav1.Duration{Duration: 24 * 3600 * 1e9},
				DefaultX509Validity: metav1.Duration{Duration: 3600 * 1e9},
				DefaultJWTValidity:  metav1.Duration{Duration: 300 * 1e9},
				CASubject: &v1alpha1.CASubject{
					Country:      "US",
					Organization: "Example Org",
					CommonName:   "Example CA",
				},
				Datastore: &v1alpha1.DataStore{
					DatabaseType:     "sqlite3",
					ConnectionString: "/run/spire/data/datastore.sqlite3",
				},
				CommonConfig: v1alpha1.CommonConfig{
					Labels: map[string]string{
						"app": "spire-server",
						"env": "test",
					},
				},
			},
			validateJSON: func(t *testing.T, jsonData string) {
				var parsed map[string]interface{}
				err := json.Unmarshal([]byte(jsonData), &parsed)
				require.NoError(t, err)

				// Validate top-level keys
				assert.Contains(t, parsed, "server")
				assert.Contains(t, parsed, "plugins")
				assert.Contains(t, parsed, "health_checks")
				assert.Contains(t, parsed, "telemetry")

				// Validate server section
				server := parsed["server"].(map[string]interface{})
				assert.Equal(t, "example.org", server["trust_domain"])
				assert.Equal(t, "0.0.0.0", server["bind_address"])
				assert.Equal(t, "8081", server["bind_port"])
				assert.Equal(t, "https://oidc.example.org", server["jwt_issuer"])

				// Validate plugins section
				plugins := parsed["plugins"].(map[string]interface{})
				assert.Contains(t, plugins, "DataStore")
				assert.Contains(t, plugins, "KeyManager")
				assert.Contains(t, plugins, "NodeAttestor")
				assert.Contains(t, plugins, "Notifier")

				// Validate NodeAttestor plugin structure
				nodeAttestors := plugins["NodeAttestor"].([]interface{})
				require.Len(t, nodeAttestors, 1)
				na := nodeAttestors[0].(map[string]interface{})
				assert.Contains(t, na, "k8s_psat")
			},
		},
	{
		name: "missing trust domain returns error",
		spec: &v1alpha1.SpireServerSpec{
			TrustDomain:     "",
			ClusterName:     "test-cluster",
			BundleConfigMap: "spire-bundle",
		},
		expectError: true,
	},
	{
		name: "missing cluster name returns error",
		spec: &v1alpha1.SpireServerSpec{
			TrustDomain:     "example.org",
			ClusterName:     "",
			BundleConfigMap: "spire-bundle",
		},
		expectError: true,
	},
	{
		name: "missing bundle configmap returns error",
		spec: &v1alpha1.SpireServerSpec{
			TrustDomain:     "example.org",
			ClusterName:     "test-cluster",
			BundleConfigMap: "",
		},
		expectError: true,
	},
		{
			name: "missing datastore returns error",
			spec: &v1alpha1.SpireServerSpec{
				TrustDomain:     "example.org",
				ClusterName:     "test-cluster",
				BundleConfigMap: "spire-bundle",
				Datastore:       nil,
			},
			expectError: true,
		},
		{
			name: "missing CA subject returns error",
			spec: &v1alpha1.SpireServerSpec{
				TrustDomain:     "example.org",
				ClusterName:     "test-cluster",
				BundleConfigMap: "spire-bundle",
				CASubject:       nil,
				Datastore: &v1alpha1.DataStore{
					DatabaseType:     "sqlite3",
					ConnectionString: "/run/spire/data/datastore.sqlite3",
				},
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cm, err := GenerateSpireServerConfigMap(tt.spec)

			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, cm)
				return
			}

			require.NoError(t, err)
			require.NotNil(t, cm)

			// Validate ConfigMap metadata
			assert.Equal(t, "spire-server", cm.ObjectMeta.Name)
			assert.Equal(t, utils.OperatorNamespace, cm.ObjectMeta.Namespace)

			// Validate ConfigMap data
			require.Contains(t, cm.Data, "server.conf")
			serverConfJSON := cm.Data["server.conf"]
			assert.NotEmpty(t, serverConfJSON)

			// Validate JSON structure
			if tt.validateJSON != nil {
				tt.validateJSON(t, serverConfJSON)
			}

			// Ensure JSON is valid
			var parsed map[string]interface{}
			err = json.Unmarshal([]byte(serverConfJSON), &parsed)
			require.NoError(t, err, "Generated JSON should be valid")
		})
	}
}

func TestGenerateServerConfMap_BackwardCompatibility(t *testing.T) {
	spec := &v1alpha1.SpireServerSpec{
		TrustDomain:         "example.org",
		ClusterName:         "test-cluster",
		BundleConfigMap:     "spire-bundle",
		JwtIssuer:           "https://oidc.example.org",
		CAValidity:          metav1.Duration{Duration: 24 * 3600 * 1e9},
		DefaultX509Validity: metav1.Duration{Duration: 3600 * 1e9},
		DefaultJWTValidity:  metav1.Duration{Duration: 300 * 1e9},
		CASubject: &v1alpha1.CASubject{
			Country:      "US",
			Organization: "Example Org",
			CommonName:   "Example CA",
		},
		Datastore: &v1alpha1.DataStore{
			DatabaseType:     "sqlite3",
			ConnectionString: "/run/spire/data/datastore.sqlite3",
		},
	}

	// Test that deprecated function still works
	result, err := generateServerConfMap(spec)
	require.NoError(t, err)
	require.NotNil(t, result)

	// Validate structure
	assert.Contains(t, result, "server")
	assert.Contains(t, result, "plugins")
	assert.Contains(t, result, "health_checks")
	assert.Contains(t, result, "telemetry")

	// Verify it produces the same structure as the new function
	newConfig := buildSpireServerConfig(spec)
	newJSON, err := json.Marshal(newConfig)
	require.NoError(t, err)

	oldJSON, err := json.Marshal(result)
	require.NoError(t, err)

	// Both should produce equivalent JSON
	var newParsed, oldParsed map[string]interface{}
	json.Unmarshal(newJSON, &newParsed)
	json.Unmarshal(oldJSON, &oldParsed)

	assert.Equal(t, newParsed["server"], oldParsed["server"])
	assert.Equal(t, newParsed["health_checks"], oldParsed["health_checks"])
}

func TestGenerateServerConfMap(t *testing.T) {
	validConfig := createValidConfig()

	confMap, err := generateServerConfMap(validConfig)
	if err != nil {
		t.Fatalf("Failed to generate server config map: %v", err)
	}

	// Test server section
	server, ok := confMap["server"].(map[string]interface{})
	if !ok {
		t.Fatal("Failed to get server section")
	}

	if server["trust_domain"] != validConfig.TrustDomain {
		t.Errorf("Expected trust_domain %q, got %v", validConfig.TrustDomain, server["trust_domain"])
	}

	if server["jwt_issuer"] != validConfig.JwtIssuer {
		t.Errorf("Expected jwt_issuer %q, got %v", validConfig.JwtIssuer, server["jwt_issuer"])
	}

	// Test CA TTL - JSON marshaling converts Duration to string
	caTTL, ok := server["ca_ttl"].(string)
	if !ok {
		t.Errorf("Expected ca_ttl to be string, got %T", server["ca_ttl"])
	} else if caTTL != validConfig.CAValidity.Duration.String() {
		t.Errorf("Expected ca_ttl %v, got %v", validConfig.CAValidity.Duration.String(), caTTL)
	}

	// Test default X509 SVID TTL
	x509TTL, ok := server["default_x509_svid_ttl"].(string)
	if !ok {
		t.Errorf("Expected default_x509_svid_ttl to be string, got %T", server["default_x509_svid_ttl"])
	} else if x509TTL != validConfig.DefaultX509Validity.Duration.String() {
		t.Errorf("Expected default_x509_svid_ttl %v, got %v", validConfig.DefaultX509Validity.Duration.String(), x509TTL)
	}

	// Test default JWT SVID TTL
	jwtTTL, ok := server["default_jwt_svid_ttl"].(string)
	if !ok {
		t.Errorf("Expected default_jwt_svid_ttl to be string, got %T", server["default_jwt_svid_ttl"])
	} else if jwtTTL != validConfig.DefaultJWTValidity.Duration.String() {
		t.Errorf("Expected default_jwt_svid_ttl %v, got %v", validConfig.DefaultJWTValidity.Duration.String(), jwtTTL)
	}

	// Test CA subject - JSON unmarshaling returns []interface{}
	caSubjectsRaw, ok := server["ca_subject"].([]interface{})
	if !ok || len(caSubjectsRaw) == 0 {
		t.Fatalf("Failed to get CA subject, got type %T", server["ca_subject"])
	}
	caSubjects := caSubjectsRaw[0].(map[string]interface{})

	if caSubjects["common_name"] != validConfig.CASubject.CommonName {
		t.Errorf("Expected common_name %q, got %v", validConfig.CASubject.CommonName, caSubjects["common_name"])
	}

	// Test plugins section
	plugins, ok := confMap["plugins"].(map[string]interface{})
	if !ok {
		t.Fatal("Failed to get plugins section")
	}

	// Test DataStore plugin - JSON unmarshaling returns []interface{}
	dataStoreRaw, ok := plugins["DataStore"].([]interface{})
	if !ok || len(dataStoreRaw) == 0 {
		t.Fatalf("Failed to get DataStore plugin, got type %T", plugins["DataStore"])
	}
	dataStore := dataStoreRaw[0].(map[string]interface{})

	sqlPlugin := dataStore["sql"].(map[string]interface{})
	pluginData := sqlPlugin["plugin_data"].(map[string]interface{})

	if pluginData["connection_string"] != validConfig.Datastore.ConnectionString {
		t.Errorf("Expected connection_string %q, got %v",
			validConfig.Datastore.ConnectionString,
			pluginData["connection_string"])
	}

	if pluginData["database_type"] != validConfig.Datastore.DatabaseType {
		t.Errorf("Expected database_type %q, got %v",
			validConfig.Datastore.DatabaseType,
			pluginData["database_type"])
	}

	// Test Notifier plugin - JSON unmarshaling returns []interface{}
	notifierRaw, ok := plugins["Notifier"].([]interface{})
	if !ok || len(notifierRaw) == 0 {
		t.Fatalf("Failed to get Notifier plugin, got type %T", plugins["Notifier"])
	}
	notifier := notifierRaw[0].(map[string]interface{})

	k8sBundle := notifier["k8sbundle"].(map[string]interface{})
	bundleData := k8sBundle["plugin_data"].(map[string]interface{})

	if bundleData["config_map"] != validConfig.BundleConfigMap {
		t.Errorf("Expected config_map %q, got %v",
			validConfig.BundleConfigMap,
			bundleData["config_map"])
	}

	if bundleData["namespace"] != utils.OperatorNamespace {
		t.Errorf("Expected namespace %q, got %v",
			utils.OperatorNamespace,
			bundleData["namespace"])
	}
}

func TestGenerateServerConfMapTTLFields(t *testing.T) {
	tests := []struct {
		name                 string
		caValidityDuration   string
		defaultX509Duration  string
		defaultJWTDuration   string
		expectedCAValidity   metav1.Duration
		expectedX509Validity metav1.Duration
		expectedJWTValidity  metav1.Duration
	}{
		{
			name:                 "Custom TTL values",
			caValidityDuration:   "48h",
			defaultX509Duration:  "2h",
			defaultJWTDuration:   "30m",
			expectedCAValidity:   metav1.Duration{Duration: mustParseDuration("48h")},
			expectedX509Validity: metav1.Duration{Duration: mustParseDuration("2h")},
			expectedJWTValidity:  metav1.Duration{Duration: mustParseDuration("30m")},
		},
		{
			name:                 "Default TTL values",
			caValidityDuration:   "24h",
			defaultX509Duration:  "1h",
			defaultJWTDuration:   "10m",
			expectedCAValidity:   metav1.Duration{Duration: mustParseDuration("24h")},
			expectedX509Validity: metav1.Duration{Duration: mustParseDuration("1h")},
			expectedJWTValidity:  metav1.Duration{Duration: mustParseDuration("10m")},
		},
		{
			name:                 "Short TTL values",
			caValidityDuration:   "1h",
			defaultX509Duration:  "15m",
			defaultJWTDuration:   "5m",
			expectedCAValidity:   metav1.Duration{Duration: mustParseDuration("1h")},
			expectedX509Validity: metav1.Duration{Duration: mustParseDuration("15m")},
			expectedJWTValidity:  metav1.Duration{Duration: mustParseDuration("5m")},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := createValidConfig()
		config.CAValidity = tt.expectedCAValidity
		config.DefaultX509Validity = tt.expectedX509Validity
		config.DefaultJWTValidity = tt.expectedJWTValidity

		confMap, err := generateServerConfMap(config)
		if err != nil {
			t.Fatalf("Failed to generate server config map: %v", err)
		}

		server, ok := confMap["server"].(map[string]interface{})
			if !ok {
				t.Fatal("Failed to get server section")
			}

			// Test CA TTL - JSON marshaling converts Duration to string
			caTTL, ok := server["ca_ttl"].(string)
			if !ok {
				t.Errorf("Expected ca_ttl to be string, got %T", server["ca_ttl"])
			} else if caTTL != config.CAValidity.Duration.String() {
				t.Errorf("Expected ca_ttl %v, got %v", config.CAValidity.Duration.String(), caTTL)
			}

			// Test default X509 SVID TTL
			x509TTL, ok := server["default_x509_svid_ttl"].(string)
			if !ok {
				t.Errorf("Expected default_x509_svid_ttl to be string, got %T", server["default_x509_svid_ttl"])
			} else if x509TTL != config.DefaultX509Validity.Duration.String() {
				t.Errorf("Expected default_x509_svid_ttl %v, got %v", config.DefaultX509Validity.Duration.String(), x509TTL)
			}

			// Test default JWT SVID TTL
			jwtTTL, ok := server["default_jwt_svid_ttl"].(string)
			if !ok {
				t.Errorf("Expected default_jwt_svid_ttl to be string, got %T", server["default_jwt_svid_ttl"])
			} else if jwtTTL != config.DefaultJWTValidity.Duration.String() {
				t.Errorf("Expected default_jwt_svid_ttl %v, got %v", config.DefaultJWTValidity.Duration.String(), jwtTTL)
			}
		})
	}
}

func TestGenerateSpireServerConfigMapWithTTLFields(t *testing.T) {
	// Test that the new TTL fields are properly included in the generated ConfigMap
	config := createValidConfig()
	config.CAValidity = metav1.Duration{Duration: mustParseDuration("48h")}
	config.DefaultX509Validity = metav1.Duration{Duration: mustParseDuration("2h")}
	config.DefaultJWTValidity = metav1.Duration{Duration: mustParseDuration("15m")}

	cm, err := GenerateSpireServerConfigMap(config)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Verify config data exists
	configData, exists := cm.Data["server.conf"]
	if !exists {
		t.Fatal("Expected server.conf data to exist in ConfigMap")
	}

	// Validate JSON
	var configMap map[string]interface{}
	if err := json.Unmarshal([]byte(configData), &configMap); err != nil {
		t.Fatalf("Failed to unmarshal server.conf JSON: %v", err)
	}

	// Verify server section contains TTL fields
	serverConfig, ok := configMap["server"].(map[string]interface{})
	if !ok {
		t.Fatal("Failed to get server section from config")
	}

	// Check CA TTL is properly set (JSON marshaling converts Duration to string)
	if caValidity, ok := serverConfig["ca_ttl"].(string); !ok {
		t.Errorf("Expected ca_ttl to be a string, got %T", serverConfig["ca_ttl"])
	} else if caValidity != config.CAValidity.Duration.String() {
		t.Errorf("Expected ca_ttl %v, got %v", config.CAValidity.Duration.String(), caValidity)
	}

	// Check X509 TTL is properly set (JSON marshaling converts Duration to string)
	if x509Validity, ok := serverConfig["default_x509_svid_ttl"].(string); !ok {
		t.Errorf("Expected default_x509_svid_ttl to be a string, got %T", serverConfig["default_x509_svid_ttl"])
	} else if x509Validity != config.DefaultX509Validity.Duration.String() {
		t.Errorf("Expected default_x509_svid_ttl %v, got %v", config.DefaultX509Validity.Duration.String(), x509Validity)
	}

	// Check JWT TTL is properly set (JSON marshaling converts Duration to string)
	if jwtValidity, ok := serverConfig["default_jwt_svid_ttl"].(string); !ok {
		t.Errorf("Expected default_jwt_svid_ttl to be a string, got %T", serverConfig["default_jwt_svid_ttl"])
	} else if jwtValidity != config.DefaultJWTValidity.Duration.String() {
		t.Errorf("Expected default_jwt_svid_ttl %v, got %v", config.DefaultJWTValidity.Duration.String(), jwtValidity)
	}
}

func TestMarshalToJSON(t *testing.T) {
	testMap := map[string]interface{}{
		"key1": "value1",
		"key2": 123,
		"key3": map[string]interface{}{
			"nested": "value",
		},
	}

	jsonBytes, err := json.Marshal(testMap)
	if err != nil {
		t.Fatalf("Failed to marshal to JSON: %v", err)
	}

	// Check that result is valid JSON
	var result map[string]interface{}
	if err := json.Unmarshal(jsonBytes, &result); err != nil {
		t.Fatalf("Result is not valid JSON: %v", err)
	}

	// Validate content
	assert.Equal(t, "value1", result["key1"])
	assert.Equal(t, float64(123), result["key2"])

	nested, ok := result["key3"].(map[string]interface{})
	assert.True(t, ok)
	assert.Equal(t, "value", nested["nested"])
}

func TestGenerateConfigHashFromString(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:  "Basic string",
			input: "test string",
			// Pre-computed SHA256 hash for "test string"
			expected: "d5579c46dfcc7f18207013e65b44e4cb4e2c2298f4ac457ba8f82743f31e930b",
		},
		{
			name:  "String with whitespace to trim",
			input: "  test string  \n",
			// Should be the same as above after trimming
			expected: "d5579c46dfcc7f18207013e65b44e4cb4e2c2298f4ac457ba8f82743f31e930b",
		},
		{
			name:  "Empty string",
			input: "",
			// SHA256 hash of empty string
			expected: "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
		},
		{
			name:  "String with only whitespace",
			input: "  \n  \t  ",
			// Should be the same as empty string after trimming
			expected: "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := generateConfigHashFromString(tt.input)
			if result != tt.expected {
				t.Errorf("Expected hash %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestGenerateConfigHash(t *testing.T) {
	tests := []struct {
		name     string
		input    []byte
		expected string
	}{
		{
			name:  "Basic string as bytes",
			input: []byte("test string"),
			// Pre-computed SHA256 hash for "test string"
			expected: "d5579c46dfcc7f18207013e65b44e4cb4e2c2298f4ac457ba8f82743f31e930b",
		},
		{
			name:  "Bytes with whitespace to trim",
			input: []byte("  test string  \n"),
			// Should be the same as above after trimming
			expected: "d5579c46dfcc7f18207013e65b44e4cb4e2c2298f4ac457ba8f82743f31e930b",
		},
		{
			name:  "Empty bytes",
			input: []byte{},
			// SHA256 hash of empty string
			expected: "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := generateConfigHash(tt.input)
			if result != tt.expected {
				t.Errorf("Expected hash %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestGenerateSpireControllerManagerConfigYaml(t *testing.T) {
	validConfig := createValidConfig()

	tests := []struct {
		name        string
		config      *v1alpha1.SpireServerSpec
		expectError bool
		errorMsg    string
		checkFields map[string]string
	}{
		{
			name:        "Valid config",
			config:      validConfig,
			expectError: false,
			checkFields: map[string]string{
				"clusterName: test-cluster":            "",
				"trustDomain: example.org":             "",
				"entryIDPrefix: test-cluster":          "",
				"spireServerSocketPath":                "/tmp/spire-server/private/api.sock",
				"apiVersion: spire.spiffe.io/v1alpha1": "",
			},
		},
		{
			name: "Empty trust domain",
			config: &v1alpha1.SpireServerSpec{
				TrustDomain: "",
				ClusterName: "test-cluster",
			},
			expectError: true,
			errorMsg:    "trust_domain is empty",
		},
		{
			name: "Empty cluster name",
			config: &v1alpha1.SpireServerSpec{
				TrustDomain: "example.org",
				ClusterName: "",
			},
			expectError: true,
			errorMsg:    "cluster name is empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			yamlStr, err := generateSpireControllerManagerConfigYaml(tt.config)

			// Check error expectations
			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error containing %q, got nil", tt.errorMsg)
				} else if !strings.Contains(err.Error(), tt.errorMsg) {
					t.Errorf("Expected error containing %q, got %q", tt.errorMsg, err.Error())
				}
				return
			}

			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			// Check expected content
			for content := range tt.checkFields {
				if !strings.Contains(yamlStr, content) {
					t.Errorf("Expected YAML to contain %q, but it doesn't", content)
				}
			}
		})
	}
}

func TestGenerateControllerManagerConfigMap(t *testing.T) {
	testYAML := "test: yaml\nkey: value"

	cm := generateControllerManagerConfigMap(testYAML)

	// Check ConfigMap metadata
	if cm.Name != "spire-controller-manager" {
		t.Errorf("Expected name 'spire-controller-manager', got %q", cm.Name)
	}

	if cm.Namespace != utils.OperatorNamespace {
		t.Errorf("Expected namespace %q, got %q", utils.OperatorNamespace, cm.Namespace)
	}

	// Check labels - now using standardized labeling
	expectedLabels := utils.SpireControllerManagerLabels(nil)
	for k, v := range expectedLabels {
		if cm.Labels[k] != v {
			t.Errorf("Expected label %q to be %q, got %q", k, v, cm.Labels[k])
		}
	}

	// Check data
	configData, exists := cm.Data["controller-manager-config.yaml"]
	if !exists {
		t.Fatal("Expected controller-manager-config.yaml data to exist in ConfigMap")
	}

	if configData != testYAML {
		t.Errorf("Expected YAML data %q, got %q", testYAML, configData)
	}
}

func TestGenerateSpireBundleConfigMap(t *testing.T) {
	validConfig := createValidConfig()

	tests := []struct {
		name        string
		config      *v1alpha1.SpireServerSpec
		expectError bool
		errorMsg    string
	}{
		{
			name:        "Valid config",
			config:      validConfig,
			expectError: false,
		},
		{
			name: "Empty bundle configmap",
			config: &v1alpha1.SpireServerSpec{
				BundleConfigMap: "",
			},
			expectError: true,
			errorMsg:    "bundle ConfigMap is empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cm, err := generateSpireBundleConfigMap(tt.config)

			// Check error expectations
			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error containing %q, got nil", tt.errorMsg)
				} else if !strings.Contains(err.Error(), tt.errorMsg) {
					t.Errorf("Expected error containing %q, got %q", tt.errorMsg, err.Error())
				}
				return
			}

			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			// Check ConfigMap metadata
			if cm.Name != tt.config.BundleConfigMap {
				t.Errorf("Expected name %q, got %q", tt.config.BundleConfigMap, cm.Name)
			}

			if cm.Namespace != utils.OperatorNamespace {
				t.Errorf("Expected namespace %q, got %q", utils.OperatorNamespace, cm.Namespace)
			}

			// Check labels
			if cm.Labels["app.kubernetes.io/name"] != "spire-server" {
				t.Errorf("Expected app label 'spire-server', got %q", cm.Labels["app"])
			}

			if cm.Labels[utils.AppManagedByLabelKey] != utils.AppManagedByLabelValue {
				t.Errorf("Expected label %q to be %q, got %q",
					utils.AppManagedByLabelKey,
					utils.AppManagedByLabelValue,
					cm.Labels[utils.AppManagedByLabelKey])
			}
		})
	}
}

// Helper function to create a valid config for testing
func createValidConfig() *v1alpha1.SpireServerSpec {
	return &v1alpha1.SpireServerSpec{
		TrustDomain:     "example.org",
		ClusterName:     "test-cluster",
		BundleConfigMap: "spire-bundle",
		JwtIssuer:       "example.org",
		CASubject: &v1alpha1.CASubject{
			CommonName:   "SPIRE Server CA",
			Country:      "US",
			Organization: "SPIRE",
		},
		Datastore: &v1alpha1.DataStore{
			ConnectionString: "postgresql://postgres:password@postgres:5432/spire",
			DatabaseType:     "postgres",
			DisableMigration: "false",
			MaxIdleConns:     10,
			MaxOpenConns:     20,
		},
		CommonConfig: v1alpha1.CommonConfig{
			Labels: map[string]string{
				"custom-label": "value",
			},
		},
		// Add the new TTL configuration fields with default values
		CAValidity:          metav1.Duration{Duration: mustParseDuration("24h")},
		DefaultX509Validity: metav1.Duration{Duration: mustParseDuration("1h")},
		DefaultJWTValidity:  metav1.Duration{Duration: mustParseDuration("10m")},
	}
}

// Helper function to parse duration strings for testing
func mustParseDuration(s string) time.Duration {
	d, err := time.ParseDuration(s)
	if err != nil {
		panic(err)
	}
	return d
}
