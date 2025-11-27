package spire_server

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/openshift/zero-trust-workload-identity-manager/api/v1alpha1"
	"github.com/openshift/zero-trust-workload-identity-manager/pkg/controller/utils"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestGenerateSpireServerConfigMap(t *testing.T) {
	validConfig := createValidConfig()

	validZTWIM := &v1alpha1.ZeroTrustWorkloadIdentityManager{
		Spec: v1alpha1.ZeroTrustWorkloadIdentityManagerSpec{
			TrustDomain:     "example.org",
			BundleConfigMap: "spire-bundle",
		},
	}

	tests := []struct {
		name        string
		config      *v1alpha1.SpireServerSpec
		ztwim       *v1alpha1.ZeroTrustWorkloadIdentityManager
		expectError bool
		errorMsg    string
	}{
		{
			name:        "Valid config",
			config:      validConfig,
			ztwim:       validZTWIM,
			expectError: false,
		},
		{
			name:        "Nil config",
			config:      nil,
			ztwim:       validZTWIM,
			expectError: true,
			errorMsg:    "config is nil",
		},
		{
			name: "Empty trust domain",
			config: &v1alpha1.SpireServerSpec{
				Datastore: v1alpha1.DataStore{
					ConnectionString: "postgresql://postgres:password@postgres:5432/spire",
					DatabaseType:     "postgres",
				},
			},
			ztwim: &v1alpha1.ZeroTrustWorkloadIdentityManager{
				Spec: v1alpha1.ZeroTrustWorkloadIdentityManagerSpec{
					TrustDomain:     "",
					BundleConfigMap: "spire-bundle",
				},
			},
			expectError: true,
			errorMsg:    "trust_domain is empty",
		},
		{
			name: "Empty bundle configmap",
			config: &v1alpha1.SpireServerSpec{
				Datastore: v1alpha1.DataStore{
					ConnectionString: "postgresql://postgres:password@postgres:5432/spire",
					DatabaseType:     "postgres",
				},
			},
			ztwim: &v1alpha1.ZeroTrustWorkloadIdentityManager{
				Spec: v1alpha1.ZeroTrustWorkloadIdentityManagerSpec{
					TrustDomain:     "example.org",
					BundleConfigMap: "",
				},
			},
			expectError: true,
			errorMsg:    "bundle configmap is empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cm, err := generateSpireServerConfigMap(tt.config, tt.ztwim)

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

			// Verify ConfigMap
			if cm.Name != "spire-server" {
				t.Errorf("Expected name 'spire-server', got %q", cm.Name)
			}

			if cm.Namespace != utils.GetOperatorNamespace() {
				t.Errorf("Expected namespace %q, got %q", utils.GetOperatorNamespace(), cm.Namespace)
			}

			// Check labels - now using standardized labeling
			expectedLabels := utils.SpireServerLabels(tt.config.Labels)
			for k, v := range expectedLabels {
				if cm.Labels[k] != v {
					t.Errorf("Expected label %q to be %q, got %q", k, v, cm.Labels[k])
				}
			}

			// Check custom labels
			if tt.config != nil {
				for key, value := range tt.config.Labels {
					if cm.Labels[key] != value {
						t.Errorf("Expected label %q to be %q, got %q", key, value, cm.Labels[key])
					}
				}
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

			// Verify expected trust domain
			serverConfig, ok := configMap["server"].(map[string]interface{})
			if !ok {
				t.Fatal("Failed to get server section from config")
			}

			if td, ok := serverConfig["trust_domain"].(string); !ok || td != tt.ztwim.Spec.TrustDomain {
				t.Errorf("Expected trust_domain %q, got %v", tt.ztwim.Spec.TrustDomain, td)
			}
		})
	}
}

func TestGenerateServerConfMap(t *testing.T) {
	validConfig := createValidConfig()

	validZTWIM := &v1alpha1.ZeroTrustWorkloadIdentityManager{
		Spec: v1alpha1.ZeroTrustWorkloadIdentityManagerSpec{
			TrustDomain:     "example.org",
			BundleConfigMap: "spire-bundle",
		},
	}

	confMap := generateServerConfMap(validConfig, validZTWIM)

	// Test server section
	server, ok := confMap["server"].(map[string]interface{})
	if !ok {
		t.Fatal("Failed to get server section")
	}

	if server["trust_domain"] != validZTWIM.Spec.TrustDomain {
		t.Errorf("Expected trust_domain %q, got %v", validZTWIM.Spec.TrustDomain, server["trust_domain"])
	}

	if server["jwt_issuer"] != validConfig.JwtIssuer {
		t.Errorf("Expected jwt_issuer %q, got %v", validConfig.JwtIssuer, server["jwt_issuer"])
	}

	// Test CA TTL (direct comparison of metav1.Duration objects)
	if server["ca_ttl"] != validConfig.CAValidity {
		t.Errorf("Expected ca_ttl %v, got %v", validConfig.CAValidity, server["ca_ttl"])
	}

	// Test default X509 SVID TTL (direct comparison of metav1.Duration objects)
	if server["default_x509_svid_ttl"] != validConfig.DefaultX509Validity {
		t.Errorf("Expected default_x509_svid_ttl %v, got %v", validConfig.DefaultX509Validity, server["default_x509_svid_ttl"])
	}

	// Test default JWT SVID TTL (direct comparison of metav1.Duration objects)
	if server["default_jwt_svid_ttl"] != validConfig.DefaultJWTValidity {
		t.Errorf("Expected default_jwt_svid_ttl %v, got %v", validConfig.DefaultJWTValidity, server["default_jwt_svid_ttl"])
	}

	// Test CA subject
	caSubjects, ok := server["ca_subject"].([]map[string]interface{})
	if !ok || len(caSubjects) == 0 {
		t.Fatal("Failed to get CA subject")
	}

	caSubject := caSubjects[0]
	if caSubject["common_name"] != validConfig.CASubject.CommonName {
		t.Errorf("Expected common_name %q, got %v", validConfig.CASubject.CommonName, caSubject["common_name"])
	}

	// Test plugins section
	plugins, ok := confMap["plugins"].(map[string]interface{})
	if !ok {
		t.Fatal("Failed to get plugins section")
	}

	// Test DataStore plugin
	dataStore, ok := plugins["DataStore"].([]map[string]interface{})
	if !ok || len(dataStore) == 0 {
		t.Fatal("Failed to get DataStore plugin")
	}

	sqlPlugin := dataStore[0]["sql"].(map[string]interface{})
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

	// Test Notifier plugin
	notifier, ok := plugins["Notifier"].([]map[string]interface{})
	if !ok || len(notifier) == 0 {
		t.Fatal("Failed to get Notifier plugin")
	}

	k8sBundle := notifier[0]["k8sbundle"].(map[string]interface{})
	bundleData := k8sBundle["plugin_data"].(map[string]interface{})

	if bundleData["config_map"] != validZTWIM.Spec.BundleConfigMap {
		t.Errorf("Expected config_map %q, got %v",
			validZTWIM.Spec.BundleConfigMap,
			bundleData["config_map"])
	}

	if bundleData["namespace"] != utils.GetOperatorNamespace() {
		t.Errorf("Expected namespace %q, got %v",
			utils.GetOperatorNamespace(),
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

			validZTWIM := &v1alpha1.ZeroTrustWorkloadIdentityManager{
				Spec: v1alpha1.ZeroTrustWorkloadIdentityManagerSpec{
					TrustDomain:     "example.org",
					BundleConfigMap: "spire-bundle",
				},
			}

			confMap := generateServerConfMap(config, validZTWIM)

			server, ok := confMap["server"].(map[string]interface{})
			if !ok {
				t.Fatal("Failed to get server section")
			}

			// Test CA TTL (direct comparison of metav1.Duration objects)
			if server["ca_ttl"] != config.CAValidity {
				t.Errorf("Expected ca_ttl %v, got %v", config.CAValidity, server["ca_ttl"])
			}

			// Test default X509 SVID TTL (direct comparison of metav1.Duration objects)
			if server["default_x509_svid_ttl"] != config.DefaultX509Validity {
				t.Errorf("Expected default_x509_svid_ttl %v, got %v", config.DefaultX509Validity, server["default_x509_svid_ttl"])
			}

			// Test default JWT SVID TTL (direct comparison of metav1.Duration objects)
			if server["default_jwt_svid_ttl"] != config.DefaultJWTValidity {
				t.Errorf("Expected default_jwt_svid_ttl %v, got %v", config.DefaultJWTValidity, server["default_jwt_svid_ttl"])
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

	validZTWIM := &v1alpha1.ZeroTrustWorkloadIdentityManager{
		Spec: v1alpha1.ZeroTrustWorkloadIdentityManagerSpec{
			TrustDomain:     "example.org",
			BundleConfigMap: "spire-bundle",
		},
	}

	cm, err := generateSpireServerConfigMap(config, validZTWIM)
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

	jsonBytes, err := marshalToJSON(testMap)
	if err != nil {
		t.Fatalf("Failed to marshal to JSON: %v", err)
	}

	// Check that result is valid JSON
	var result map[string]interface{}
	if err := json.Unmarshal(jsonBytes, &result); err != nil {
		t.Fatalf("Result is not valid JSON: %v", err)
	}

	// Check indentation
	jsonStr := string(jsonBytes)
	if !strings.Contains(jsonStr, "  \"key1\"") {
		t.Errorf("JSON is not properly indented with two spaces")
	}

	// Validate content
	if result["key1"] != "value1" || result["key2"].(float64) != 123 {
		t.Errorf("JSON content does not match input map")
	}

	nested, ok := result["key3"].(map[string]interface{})
	if !ok || nested["nested"] != "value" {
		t.Errorf("Nested JSON content does not match input map")
	}
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

	validZTWIM := &v1alpha1.ZeroTrustWorkloadIdentityManager{
		Spec: v1alpha1.ZeroTrustWorkloadIdentityManagerSpec{
			TrustDomain:     "example.org",
			ClusterName:     "test-cluster",
			BundleConfigMap: "spire-bundle",
		},
	}

	tests := []struct {
		name        string
		config      *v1alpha1.SpireServerSpec
		ztwim       *v1alpha1.ZeroTrustWorkloadIdentityManager
		expectError bool
		errorMsg    string
		checkFields map[string]string
	}{
		{
			name:        "Valid config",
			config:      validConfig,
			ztwim:       validZTWIM,
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
			name:   "Empty trust domain",
			config: &v1alpha1.SpireServerSpec{},
			ztwim: &v1alpha1.ZeroTrustWorkloadIdentityManager{
				Spec: v1alpha1.ZeroTrustWorkloadIdentityManagerSpec{
					TrustDomain: "",
					ClusterName: "test-cluster",
				},
			},
			expectError: true,
			errorMsg:    "trust_domain is empty",
		},
		{
			name:   "Empty cluster name",
			config: &v1alpha1.SpireServerSpec{},
			ztwim: &v1alpha1.ZeroTrustWorkloadIdentityManager{
				Spec: v1alpha1.ZeroTrustWorkloadIdentityManagerSpec{
					TrustDomain: "example.org",
					ClusterName: "",
				},
			},
			expectError: true,
			errorMsg:    "cluster name is empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			yamlStr, err := generateSpireControllerManagerConfigYaml(tt.config, tt.ztwim)

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

	if cm.Namespace != utils.GetOperatorNamespace() {
		t.Errorf("Expected namespace %q, got %q", utils.GetOperatorNamespace(), cm.Namespace)
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

	validZTWIM := &v1alpha1.ZeroTrustWorkloadIdentityManager{
		Spec: v1alpha1.ZeroTrustWorkloadIdentityManagerSpec{
			TrustDomain:     "example.org",
			BundleConfigMap: "spire-bundle",
		},
	}

	tests := []struct {
		name        string
		config      *v1alpha1.SpireServerSpec
		ztwim       *v1alpha1.ZeroTrustWorkloadIdentityManager
		expectError bool
		errorMsg    string
	}{
		{
			name:        "Valid config",
			config:      validConfig,
			ztwim:       validZTWIM,
			expectError: false,
		},
		{
			name:   "Empty bundle configmap",
			config: &v1alpha1.SpireServerSpec{},
			ztwim: &v1alpha1.ZeroTrustWorkloadIdentityManager{
				Spec: v1alpha1.ZeroTrustWorkloadIdentityManagerSpec{
					TrustDomain:     "example.org",
					BundleConfigMap: "",
				},
			},
			expectError: true,
			errorMsg:    "bundle ConfigMap is empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cm, err := generateSpireBundleConfigMap(tt.config, tt.ztwim)

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
			if cm.Name != tt.ztwim.Spec.BundleConfigMap {
				t.Errorf("Expected name %q, got %q", tt.ztwim.Spec.BundleConfigMap, cm.Name)
			}

			if cm.Namespace != utils.GetOperatorNamespace() {
				t.Errorf("Expected namespace %q, got %q", utils.GetOperatorNamespace(), cm.Namespace)
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
		JwtIssuer: "example.org",
		CASubject: v1alpha1.CASubject{
			CommonName:   "SPIRE Server CA",
			Country:      "US",
			Organization: "SPIRE",
		},
		Datastore: v1alpha1.DataStore{
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

func TestGetCAKeyType(t *testing.T) {
	tests := []struct {
		name     string
		keyType  string
		expected string
	}{
		{
			name:     "Empty string returns default rsa-2048",
			keyType:  "",
			expected: "rsa-2048",
		},
		{
			name:     "rsa-2048 key type",
			keyType:  "rsa-2048",
			expected: "rsa-2048",
		},
		{
			name:     "rsa-4096 key type",
			keyType:  "rsa-4096",
			expected: "rsa-4096",
		},
		{
			name:     "ec-p256 key type",
			keyType:  "ec-p256",
			expected: "ec-p256",
		},
		{
			name:     "ec-p384 key type",
			keyType:  "ec-p384",
			expected: "ec-p384",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getCAKeyType(tt.keyType)
			if result != tt.expected {
				t.Errorf("Expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestGenerateServerConfMapWithKeyTypes(t *testing.T) {
	tests := []struct {
		name           string
		caKeyType      string
		jwtKeyType     string
		expectedCAKey  string
		expectJWTKey   bool
		expectedJWTKey string
	}{
		{
			name:          "Default CA key type, no JWT key type",
			caKeyType:     "",
			jwtKeyType:    "",
			expectedCAKey: "rsa-2048",
			expectJWTKey:  false,
		},
		{
			name:           "RSA-2048 key types",
			caKeyType:      "rsa-2048",
			jwtKeyType:     "rsa-2048",
			expectedCAKey:  "rsa-2048",
			expectJWTKey:   true,
			expectedJWTKey: "rsa-2048",
		},
		{
			name:           "RSA-4096 key types",
			caKeyType:      "rsa-4096",
			jwtKeyType:     "rsa-4096",
			expectedCAKey:  "rsa-4096",
			expectJWTKey:   true,
			expectedJWTKey: "rsa-4096",
		},
		{
			name:           "EC-P256 key types",
			caKeyType:      "ec-p256",
			jwtKeyType:     "ec-p256",
			expectedCAKey:  "ec-p256",
			expectJWTKey:   true,
			expectedJWTKey: "ec-p256",
		},
		{
			name:           "EC-P384 key types",
			caKeyType:      "ec-p384",
			jwtKeyType:     "ec-p384",
			expectedCAKey:  "ec-p384",
			expectJWTKey:   true,
			expectedJWTKey: "ec-p384",
		},
		{
			name:           "Mixed key types - CA rsa-2048, JWT ec-p384",
			caKeyType:      "rsa-2048",
			jwtKeyType:     "ec-p384",
			expectedCAKey:  "rsa-2048",
			expectJWTKey:   true,
			expectedJWTKey: "ec-p384",
		},
		{
			name:          "Only CA key type set",
			caKeyType:     "ec-p384",
			jwtKeyType:    "",
			expectedCAKey: "ec-p384",
			expectJWTKey:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := createValidConfig()
			config.CAKeyType = tt.caKeyType
			config.JWTKeyType = tt.jwtKeyType

			validZTWIM := &v1alpha1.ZeroTrustWorkloadIdentityManager{
				Spec: v1alpha1.ZeroTrustWorkloadIdentityManagerSpec{
					TrustDomain:     "example.org",
					BundleConfigMap: "spire-bundle",
				},
			}

			confMap := generateServerConfMap(config, validZTWIM)

			// Get server section
			server, ok := confMap["server"].(map[string]interface{})
			if !ok {
				t.Fatal("Failed to get server section")
			}

			// Check CA key type
			if caKeyType, ok := server["ca_key_type"].(string); !ok {
				t.Errorf("Expected ca_key_type to be a string, got %T", server["ca_key_type"])
			} else if caKeyType != tt.expectedCAKey {
				t.Errorf("Expected ca_key_type %q, got %q", tt.expectedCAKey, caKeyType)
			}

			// Check JWT key type
			if tt.expectJWTKey {
				if jwtKeyType, ok := server["jwt_key_type"].(string); !ok {
					t.Errorf("Expected jwt_key_type to be a string, got %T", server["jwt_key_type"])
				} else if jwtKeyType != tt.expectedJWTKey {
					t.Errorf("Expected jwt_key_type %q, got %q", tt.expectedJWTKey, jwtKeyType)
				}
			} else {
				if _, exists := server["jwt_key_type"]; exists {
					t.Errorf("Expected jwt_key_type to not be present, but it exists with value %v", server["jwt_key_type"])
				}
			}
		})
	}
}

func TestGenerateSpireServerConfigMapWithKeyTypes(t *testing.T) {
	tests := []struct {
		name           string
		caKeyType      string
		jwtKeyType     string
		expectedCAKey  string
		expectJWTKey   bool
		expectedJWTKey string
	}{
		{
			name:           "Both key types specified",
			caKeyType:      "rsa-4096",
			jwtKeyType:     "ec-p384",
			expectedCAKey:  "rsa-4096",
			expectJWTKey:   true,
			expectedJWTKey: "ec-p384",
		},
		{
			name:          "Only CA key type, JWT not specified",
			caKeyType:     "ec-p256",
			jwtKeyType:    "",
			expectedCAKey: "ec-p256",
			expectJWTKey:  false,
		},
		{
			name:           "Default CA key type, JWT specified",
			caKeyType:      "",
			jwtKeyType:     "rsa-2048",
			expectedCAKey:  "rsa-2048",
			expectJWTKey:   true,
			expectedJWTKey: "rsa-2048",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := createValidConfig()
			config.CAKeyType = tt.caKeyType
			config.JWTKeyType = tt.jwtKeyType

			validZTWIM := &v1alpha1.ZeroTrustWorkloadIdentityManager{
				Spec: v1alpha1.ZeroTrustWorkloadIdentityManagerSpec{
					TrustDomain:     "example.org",
					BundleConfigMap: "spire-bundle",
				},
			}

			cm, err := generateSpireServerConfigMap(config, validZTWIM)
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

			// Verify server section contains key type fields
			serverConfig, ok := configMap["server"].(map[string]interface{})
			if !ok {
				t.Fatal("Failed to get server section from config")
			}

			// Check CA key type
			if caKeyType, ok := serverConfig["ca_key_type"].(string); !ok {
				t.Errorf("Expected ca_key_type to be a string, got %T", serverConfig["ca_key_type"])
			} else if caKeyType != tt.expectedCAKey {
				t.Errorf("Expected ca_key_type %q, got %q", tt.expectedCAKey, caKeyType)
			}

			// Check JWT key type
			if tt.expectJWTKey {
				if jwtKeyType, ok := serverConfig["jwt_key_type"].(string); !ok {
					t.Errorf("Expected jwt_key_type to be a string, got %T", serverConfig["jwt_key_type"])
				} else if jwtKeyType != tt.expectedJWTKey {
					t.Errorf("Expected jwt_key_type %q, got %q", tt.expectedJWTKey, jwtKeyType)
				}
			} else {
				if _, exists := serverConfig["jwt_key_type"]; exists {
					t.Errorf("Expected jwt_key_type to not be present, but it exists with value %v", serverConfig["jwt_key_type"])
				}
			}
		})
	}
}

func TestGenerateFederationConfig(t *testing.T) {
	tests := []struct {
		name        string
		federation  *v1alpha1.FederationConfig
		checkFields map[string]interface{}
	}{
		{
			name: "https_spiffe bundle endpoint",
			federation: &v1alpha1.FederationConfig{
				BundleEndpoint: v1alpha1.BundleEndpointConfig{
					Profile:     v1alpha1.HttpsSpiffeProfile,
					RefreshHint: 300,
				},
			},
			checkFields: map[string]interface{}{
				"bundle_endpoint_exists": true,
				"federates_with_exists":  false,
			},
		},
		{
			name: "https_web bundle endpoint with ACME",
			federation: &v1alpha1.FederationConfig{
				BundleEndpoint: v1alpha1.BundleEndpointConfig{
					Profile:     v1alpha1.HttpsWebProfile,
					RefreshHint: 600,
					HttpsWeb: &v1alpha1.HttpsWebConfig{
						Acme: &v1alpha1.AcmeConfig{
							DirectoryUrl: "https://acme-v02.api.letsencrypt.org/directory",
							DomainName:   "federation.example.org",
							Email:        "admin@example.org",
							TosAccepted:  "true",
						},
					},
				},
			},
			checkFields: map[string]interface{}{
				"bundle_endpoint_exists": true,
			},
		},
		{
			name: "Federation with remote trust domains",
			federation: &v1alpha1.FederationConfig{
				BundleEndpoint: v1alpha1.BundleEndpointConfig{
					Profile:     v1alpha1.HttpsSpiffeProfile,
					RefreshHint: 300,
				},
				FederatesWith: []v1alpha1.FederatesWithConfig{
					{
						TrustDomain:           "remote1.org",
						BundleEndpointUrl:     "https://remote1.org:8443",
						BundleEndpointProfile: v1alpha1.HttpsSpiffeProfile,
						EndpointSpiffeId:      "spiffe://remote1.org/spire/server",
					},
					{
						TrustDomain:           "remote2.org",
						BundleEndpointUrl:     "https://remote2.org",
						BundleEndpointProfile: v1alpha1.HttpsWebProfile,
					},
				},
			},
			checkFields: map[string]interface{}{
				"bundle_endpoint_exists": true,
				"federates_with_exists":  true,
				"federates_with_count":   2,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			federationConf := generateFederationConfig(tt.federation)

			// Check bundle_endpoint exists
			if tt.checkFields["bundle_endpoint_exists"].(bool) {
				if _, exists := federationConf["bundle_endpoint"]; !exists {
					t.Error("Expected bundle_endpoint to exist in federation config")
				}
			}

			// Check federates_with
			if exists, ok := tt.checkFields["federates_with_exists"].(bool); ok {
				federatesWith, hasField := federationConf["federates_with"]
				if exists && !hasField {
					t.Error("Expected federates_with to exist in federation config")
				}
				if !exists && hasField {
					t.Error("Expected federates_with to not exist in federation config")
				}

				// Check federates_with count
				if count, ok := tt.checkFields["federates_with_count"].(int); ok && hasField {
					federatesWithMap := federatesWith.(map[string]interface{})
					if len(federatesWithMap) != count {
						t.Errorf("Expected %d federates_with entries, got %d", count, len(federatesWithMap))
					}
				}
			}
		})
	}
}

func TestGenerateBundleEndpointConfig(t *testing.T) {
	tests := []struct {
		name             string
		bundleEndpoint   *v1alpha1.BundleEndpointConfig
		expectedPort     int
		expectedProfile  string
		checkRefreshHint bool
		expectedRefresh  string
	}{
		{
			name: "https_spiffe profile",
			bundleEndpoint: &v1alpha1.BundleEndpointConfig{
				Profile:     v1alpha1.HttpsSpiffeProfile,
				RefreshHint: 300,
			},
			expectedPort:     8443,
			expectedProfile:  "https_spiffe",
			checkRefreshHint: true,
			expectedRefresh:  "300s",
		},
		{
			name: "https_spiffe with custom refresh hint",
			bundleEndpoint: &v1alpha1.BundleEndpointConfig{
				Profile:     v1alpha1.HttpsSpiffeProfile,
				RefreshHint: 600,
			},
			expectedPort:     8443,
			expectedProfile:  "https_spiffe",
			checkRefreshHint: true,
			expectedRefresh:  "600s",
		},
		{
			name: "https_web with ACME",
			bundleEndpoint: &v1alpha1.BundleEndpointConfig{
				Profile:     v1alpha1.HttpsWebProfile,
				RefreshHint: 300,
				HttpsWeb: &v1alpha1.HttpsWebConfig{
					Acme: &v1alpha1.AcmeConfig{
						DirectoryUrl: "https://acme-v02.api.letsencrypt.org/directory",
						DomainName:   "federation.example.org",
						Email:        "admin@example.org",
						TosAccepted:  "true",
					},
				},
			},
			expectedPort:     8443,
			expectedProfile:  "https_web",
			checkRefreshHint: true,
			expectedRefresh:  "300s",
		},
		{
			name: "https_web with ServingCert",
			bundleEndpoint: &v1alpha1.BundleEndpointConfig{
				Profile:     v1alpha1.HttpsWebProfile,
				RefreshHint: 300,
				HttpsWeb: &v1alpha1.HttpsWebConfig{
					ServingCert: &v1alpha1.ServingCertConfig{
						FileSyncInterval: 3600,
					},
				},
			},
			expectedPort:     8443,
			expectedProfile:  "https_web",
			checkRefreshHint: true,
			expectedRefresh:  "300s",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			endpointConfig := generateBundleEndpointConfig(tt.bundleEndpoint)

			// Check port
			if port, ok := endpointConfig["port"].(int); !ok || port != tt.expectedPort {
				t.Errorf("Expected port %d, got %v", tt.expectedPort, endpointConfig["port"])
			}

			// Check address
			if address, ok := endpointConfig["address"].(string); !ok || address != "0.0.0.0" {
				t.Errorf("Expected address 0.0.0.0, got %v", endpointConfig["address"])
			}

			// Check refresh hint
			if tt.checkRefreshHint {
				if refresh, ok := endpointConfig["refresh_hint"].(string); !ok || refresh != tt.expectedRefresh {
					t.Errorf("Expected refresh_hint %q, got %v", tt.expectedRefresh, endpointConfig["refresh_hint"])
				}
			}

			// Check profile exists
			if _, exists := endpointConfig["profile"]; !exists {
				t.Error("Expected profile to exist in endpoint config")
			}
		})
	}
}

func TestGenerateFederationConfigWithFederatesWith(t *testing.T) {
	federation := &v1alpha1.FederationConfig{
		BundleEndpoint: v1alpha1.BundleEndpointConfig{
			Profile:     v1alpha1.HttpsSpiffeProfile,
			RefreshHint: 300,
		},
		FederatesWith: []v1alpha1.FederatesWithConfig{
			{
				TrustDomain:           "remote1.org",
				BundleEndpointUrl:     "https://remote1.org:8443",
				BundleEndpointProfile: v1alpha1.HttpsSpiffeProfile,
				EndpointSpiffeId:      "spiffe://remote1.org/spire/server",
			},
			{
				TrustDomain:           "remote2.org",
				BundleEndpointUrl:     "https://remote2.org",
				BundleEndpointProfile: v1alpha1.HttpsWebProfile,
			},
		},
	}

	federationConf := generateFederationConfig(federation)

	// Check federates_with exists and has correct entries
	federatesWith, exists := federationConf["federates_with"]
	if !exists {
		t.Fatal("Expected federates_with to exist")
	}

	federatesWithMap, ok := federatesWith.(map[string]interface{})
	if !ok {
		t.Fatal("Expected federates_with to be a map")
	}

	// Check remote1.org
	remote1, exists := federatesWithMap["remote1.org"]
	if !exists {
		t.Error("Expected remote1.org to exist in federates_with")
	} else {
		remote1Map := remote1.(map[string]interface{})
		if remote1Map["bundle_endpoint_url"] != "https://remote1.org:8443" {
			t.Errorf("Expected bundle_endpoint_url https://remote1.org:8443, got %v", remote1Map["bundle_endpoint_url"])
		}

		// Check https_spiffe profile
		if profile, exists := remote1Map["bundle_endpoint_profile"]; exists {
			profileMap := profile.(map[string]interface{})
			if _, exists := profileMap["https_spiffe"]; !exists {
				t.Error("Expected https_spiffe profile for remote1.org")
			}
		}
	}

	// Check remote2.org
	remote2, exists := federatesWithMap["remote2.org"]
	if !exists {
		t.Error("Expected remote2.org to exist in federates_with")
	} else {
		remote2Map := remote2.(map[string]interface{})
		if remote2Map["bundle_endpoint_url"] != "https://remote2.org" {
			t.Errorf("Expected bundle_endpoint_url https://remote2.org, got %v", remote2Map["bundle_endpoint_url"])
		}

		// Check https_web profile
		if profile, exists := remote2Map["bundle_endpoint_profile"]; exists {
			profileMap := profile.(map[string]interface{})
			if _, exists := profileMap["https_web"]; !exists {
				t.Error("Expected https_web profile for remote2.org")
			}
		}
	}
}
