package spire_server

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/openshift/zero-trust-workload-identity-manager/api/v1alpha1"
	"github.com/openshift/zero-trust-workload-identity-manager/pkg/controller/utils"
)

func TestGenerateSpireServerConfigMap(t *testing.T) {
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
			name:        "Nil config",
			config:      nil,
			expectError: true,
			errorMsg:    "config is nil",
		},
		{
			name: "Empty trust domain",
			config: &v1alpha1.SpireServerSpec{
				TrustDomain:     "",
				BundleConfigMap: "spire-bundle",
				Datastore: &v1alpha1.DataStore{
					ConnectionString: "postgresql://postgres:password@postgres:5432/spire",
					DatabaseType:     "postgres",
				},
			},
			expectError: true,
			errorMsg:    "trust_domain is empty",
		},
		{
			name: "Empty bundle configmap",
			config: &v1alpha1.SpireServerSpec{
				TrustDomain:     "example.org",
				BundleConfigMap: "",
				Datastore: &v1alpha1.DataStore{
					ConnectionString: "postgresql://postgres:password@postgres:5432/spire",
					DatabaseType:     "postgres",
				},
			},
			expectError: true,
			errorMsg:    "bundle configmap is empty",
		},
		{
			name: "Nil datastore",
			config: &v1alpha1.SpireServerSpec{
				TrustDomain:     "example.org",
				BundleConfigMap: "spire-bundle",
				Datastore:       nil,
			},
			expectError: true,
			errorMsg:    "datastore configuration is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cm, err := GenerateSpireServerConfigMap(tt.config)

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

			if cm.Namespace != utils.OperatorNamespace {
				t.Errorf("Expected namespace %q, got %q", utils.OperatorNamespace, cm.Namespace)
			}

			// Check labels
			if cm.Labels[utils.AppManagedByLabelKey] != utils.AppManagedByLabelValue {
				t.Errorf("Expected label %q to be %q, got %q",
					utils.AppManagedByLabelKey,
					utils.AppManagedByLabelValue,
					cm.Labels[utils.AppManagedByLabelKey])
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

			if td, ok := serverConfig["trust_domain"].(string); !ok || td != tt.config.TrustDomain {
				t.Errorf("Expected trust_domain %q, got %v", tt.config.TrustDomain, td)
			}
		})
	}
}

func TestGenerateServerConfMap(t *testing.T) {
	validConfig := createValidConfig()

	confMap, err := generateServerConfMap(validConfig)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
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

	// Check labels
	if cm.Labels["app"] != "spire-controller-manager" {
		t.Errorf("Expected app label 'spire-controller-manager', got %q", cm.Labels["app"])
	}

	if cm.Labels[utils.AppManagedByLabelKey] != utils.AppManagedByLabelValue {
		t.Errorf("Expected label %q to be %q, got %q",
			utils.AppManagedByLabelKey,
			utils.AppManagedByLabelValue,
			cm.Labels[utils.AppManagedByLabelKey])
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
			if cm.Labels["app"] != "spire-server" {
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

func TestGenerateUpstreamAuthorityPlugin(t *testing.T) {
	tests := []struct {
		name            string
		upstreamAuth    *v1alpha1.UpstreamAuthority
		expectedKey     string
		expectedPlugins map[string]interface{}
		expectError     bool
		errorMsg        string
	}{
		{
			name: "cert-manager plugin with defaults",
			upstreamAuth: &v1alpha1.UpstreamAuthority{
				Type: "cert-manager",
				CertManager: &v1alpha1.UpstreamAuthorityCertManager{
					IssuerName: "spire-ca",
				},
			},
			expectedKey: "cert-manager",
			expectedPlugins: map[string]interface{}{
				"issuer_name":  "spire-ca",
				"issuer_kind":  "Issuer",
				"issuer_group": "cert-manager.io",
				"namespace":    utils.OperatorNamespace,
			},
		},
		{
			name: "cert-manager plugin with kubeconfig",
			upstreamAuth: &v1alpha1.UpstreamAuthority{
				Type: "cert-manager",
				CertManager: &v1alpha1.UpstreamAuthorityCertManager{
					IssuerName:           "spire-ca",
					IssuerKind:           "ClusterIssuer",
					IssuerGroup:          "cert-manager.io",
					Namespace:            "cert-manager",
					KubeConfigSecretName: "kubeconfig-secret",
				},
			},
			expectedKey: "cert-manager",
			expectedPlugins: map[string]interface{}{
				"issuer_name":      "spire-ca",
				"issuer_kind":      "ClusterIssuer",
				"issuer_group":     "cert-manager.io",
				"namespace":        "cert-manager",
				"kube_config_file": "/cert-manager-kubeconfig/kubeconfig",
			},
		},
		{
			name: "spire plugin",
			upstreamAuth: &v1alpha1.UpstreamAuthority{
				Type: "spire",
				Spire: &v1alpha1.UpstreamAuthoritySpire{
					ServerAddress:     "upstream-spire-server",
					ServerPort:        "8081",
					WorkloadSocketAPI: "/tmp/spire-agent/public/api.sock",
				},
			},
			expectedKey: "spire",
			expectedPlugins: map[string]interface{}{
				"server_address":      "upstream-spire-server",
				"server_port":         "8081",
				"workload_api_socket": "/tmp/spire-agent/public/api.sock",
			},
		},
		{
			name: "vault plugin with token auth",
			upstreamAuth: &v1alpha1.UpstreamAuthority{
				Type: "vault",
				Vault: &v1alpha1.UpstreamAuthorityVault{
					VaultAddress:  "https://vault.example.org/",
					Namespace:     "vault-ns",
					PkiMountPoint: "test-pki",
					CaCertSecret:  "vault-ca-secret",
					TokenAuth: &v1alpha1.TokenAuth{
						Token: "hvs.test-token",
					},
				},
			},
			expectedKey: "vault",
			expectedPlugins: map[string]interface{}{
				"vault_addr":      "https://vault.example.org/",
				"pki_mount_point": "test-pki",
				"ca_cert_path":    "/vault-ca-cert/ca.crt",
				"namespace":       "vault-ns",
				"token_auth": map[string]interface{}{
					"token": "hvs.test-token",
				},
			},
		},
		{
			name: "vault plugin with cert auth",
			upstreamAuth: &v1alpha1.UpstreamAuthority{
				Type: "vault",
				Vault: &v1alpha1.UpstreamAuthorityVault{
					VaultAddress:  "https://vault.example.org/",
					PkiMountPoint: "pki",
					CaCertSecret:  "vault-ca-secret",
					CertAuth: &v1alpha1.CertAuth{
						CertAuthMountPoint: "cert",
						ClientCertSecret:   "client-cert-secret",
						ClientKeySecret:    "client-key-secret",
						CertAuthRoleName:   "spire-role",
					},
				},
			},
			expectedKey: "vault",
			expectedPlugins: map[string]interface{}{
				"vault_addr":      "https://vault.example.org/",
				"pki_mount_point": "pki",
				"ca_cert_path":    "/vault-ca-cert/ca.crt",
				"cert_auth": map[string]interface{}{
					"cert_auth_mount_point": "cert",
					"client_cert_path":      "/vault-client-cert/tls.crt",
					"client_key_path":       "/vault-client-key/tls.key",
					"cert_auth_role_name":   "spire-role",
				},
			},
		},
		{
			name: "vault plugin with approle auth",
			upstreamAuth: &v1alpha1.UpstreamAuthority{
				Type: "vault",
				Vault: &v1alpha1.UpstreamAuthorityVault{
					VaultAddress:  "https://vault.example.org/",
					PkiMountPoint: "pki",
					CaCertSecret:  "vault-ca-secret",
					AppRoleAuth: &v1alpha1.AppRoleAuth{
						AppRoleMountPoint: "approle",
						AppRoleID:         "role-id-123",
						AppRoleSecretID:   "secret-id-456",
					},
				},
			},
			expectedKey: "vault",
			expectedPlugins: map[string]interface{}{
				"vault_addr":      "https://vault.example.org/",
				"pki_mount_point": "pki",
				"ca_cert_path":    "/vault-ca-cert/ca.crt",
				"approle_auth": map[string]interface{}{
					"approle_auth_mount_point": "approle",
					"approle_id":               "role-id-123",
					"approle_secret_id":        "secret-id-456",
				},
			},
		},
		{
			name: "vault plugin with k8s auth",
			upstreamAuth: &v1alpha1.UpstreamAuthority{
				Type: "vault",
				Vault: &v1alpha1.UpstreamAuthorityVault{
					VaultAddress:  "https://vault.example.org/",
					PkiMountPoint: "pki",
					CaCertSecret:  "vault-ca-secret",
					K8sAuth: &v1alpha1.K8sAuth{
						K8sAuthMountPoint: "kubernetes",
						K8sAuthRoleName:   "spire-role",
						TokenPath:         "/custom/token/path",
					},
				},
			},
			expectedKey: "vault",
			expectedPlugins: map[string]interface{}{
				"vault_addr":      "https://vault.example.org/",
				"pki_mount_point": "pki",
				"ca_cert_path":    "/vault-ca-cert/ca.crt",
				"k8s_auth": map[string]interface{}{
					"k8s_auth_mount_point": "kubernetes",
					"k8s_auth_role_name":   "spire-role",
					"token_path":           "/custom/token/path",
				},
			},
		},
		{
			name: "vault plugin with k8s auth using default token path",
			upstreamAuth: &v1alpha1.UpstreamAuthority{
				Type: "vault",
				Vault: &v1alpha1.UpstreamAuthorityVault{
					VaultAddress:  "https://vault.example.org/",
					PkiMountPoint: "pki",
					CaCertSecret:  "vault-ca-secret",
					K8sAuth: &v1alpha1.K8sAuth{
						K8sAuthMountPoint: "kubernetes",
						K8sAuthRoleName:   "spire-role",
					},
				},
			},
			expectedKey: "vault",
			expectedPlugins: map[string]interface{}{
				"vault_addr":      "https://vault.example.org/",
				"pki_mount_point": "pki",
				"ca_cert_path":    "/vault-ca-cert/ca.crt",
				"k8s_auth": map[string]interface{}{
					"k8s_auth_mount_point": "kubernetes",
					"k8s_auth_role_name":   "spire-role",
					"token_path":           "/var/run/secrets/kubernetes.io/serviceaccount/token",
				},
			},
		},
		{
			name: "unsupported type",
			upstreamAuth: &v1alpha1.UpstreamAuthority{
				Type: "unsupported",
			},
			expectError: true,
			errorMsg:    "unsupported upstream authority type",
		},
		{
			name: "cert-manager with missing config",
			upstreamAuth: &v1alpha1.UpstreamAuthority{
				Type: "cert-manager",
			},
			expectError: true,
			errorMsg:    "upstreamAuthority.CertManager is not set",
		},
		{
			name: "spire with missing config",
			upstreamAuth: &v1alpha1.UpstreamAuthority{
				Type: "spire",
			},
			expectError: true,
			errorMsg:    "upstreamAuthority.Spire is not set",
		},
		{
			name: "vault with missing config",
			upstreamAuth: &v1alpha1.UpstreamAuthority{
				Type: "vault",
			},
			expectError: true,
			errorMsg:    "upstreamAuthority.Vault is not set",
		},
		{
			name: "vault with no auth method",
			upstreamAuth: &v1alpha1.UpstreamAuthority{
				Type: "vault",
				Vault: &v1alpha1.UpstreamAuthorityVault{
					VaultAddress:  "https://vault.example.org/",
					PkiMountPoint: "pki",
					CaCertSecret:  "vault-ca-secret",
				},
			},
			expectError: true,
			errorMsg:    "vault upstream authority requires one authentication method to be configured",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := generateUpstreamAuthorityPlugin(tt.upstreamAuth)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error containing %q, got nil", tt.errorMsg)
				} else if !strings.Contains(err.Error(), tt.errorMsg) {
					t.Errorf("Expected error containing %q, got %q", tt.errorMsg, err.Error())
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
			}

			if result == nil {
				t.Fatal("Expected non-nil result")
			}

			// Check that the expected key exists
			plugin, exists := result[tt.expectedKey]
			if !exists {
				t.Fatalf("Expected key %q to exist in result", tt.expectedKey)
			}

			// Check plugin structure
			pluginMap, ok := plugin.(map[string]interface{})
			if !ok {
				t.Fatalf("Expected plugin to be map[string]interface{}, got %T", plugin)
			}

			pluginData, exists := pluginMap["plugin_data"]
			if !exists {
				t.Fatal("Expected plugin_data to exist")
			}

			pluginDataMap, ok := pluginData.(map[string]interface{})
			if !ok {
				t.Fatalf("Expected plugin_data to be map[string]interface{}, got %T", pluginData)
			}

			// Check all expected plugin data fields
			for key, expectedValue := range tt.expectedPlugins {
				actualValue, exists := pluginDataMap[key]
				if !exists {
					t.Errorf("Expected plugin_data to contain key %q", key)
					continue
				}

				// Handle nested maps (like auth configurations)
				if expectedMap, ok := expectedValue.(map[string]interface{}); ok {
					actualMap, ok := actualValue.(map[string]interface{})
					if !ok {
						t.Errorf("Expected plugin_data[%q] to be map[string]interface{}, got %T", key, actualValue)
						continue
					}

					for nestedKey, nestedExpectedValue := range expectedMap {
						nestedActualValue, exists := actualMap[nestedKey]
						if !exists {
							t.Errorf("Expected plugin_data[%q][%q] to exist", key, nestedKey)
							continue
						}

						if nestedActualValue != nestedExpectedValue {
							t.Errorf("Expected plugin_data[%q][%q] to be %v, got %v", key, nestedKey, nestedExpectedValue, nestedActualValue)
						}
					}
				} else {
					if actualValue != expectedValue {
						t.Errorf("Expected plugin_data[%q] to be %v, got %v", key, expectedValue, actualValue)
					}
				}
			}
		})
	}
}

func TestGenerateServerConfMapWithUpstreamAuthority(t *testing.T) {
	tests := []struct {
		name         string
		upstreamAuth *v1alpha1.UpstreamAuthority
		expectPlugin bool
		pluginType   string
		expectError  bool
		errorMsg     string
	}{
		{
			name:         "no upstream authority",
			upstreamAuth: nil,
			expectPlugin: false,
		},
		{
			name: "cert-manager upstream authority",
			upstreamAuth: &v1alpha1.UpstreamAuthority{
				Type: "cert-manager",
				CertManager: &v1alpha1.UpstreamAuthorityCertManager{
					IssuerName: "spire-ca",
				},
			},
			expectPlugin: true,
			pluginType:   "cert-manager",
		},
		{
			name: "spire upstream authority",
			upstreamAuth: &v1alpha1.UpstreamAuthority{
				Type: "spire",
				Spire: &v1alpha1.UpstreamAuthoritySpire{
					ServerAddress:     "upstream-spire-server",
					ServerPort:        "8081",
					WorkloadSocketAPI: "/tmp/spire-agent/public/api.sock",
				},
			},
			expectPlugin: true,
			pluginType:   "spire",
		},
		{
			name: "vault upstream authority with token auth",
			upstreamAuth: &v1alpha1.UpstreamAuthority{
				Type: "vault",
				Vault: &v1alpha1.UpstreamAuthorityVault{
					VaultAddress:  "https://vault.example.org/",
					PkiMountPoint: "pki",
					CaCertSecret:  "vault-ca-secret",
					TokenAuth: &v1alpha1.TokenAuth{
						Token: "hvs.test-token",
					},
				},
			},
			expectPlugin: true,
			pluginType:   "vault",
		},
		{
			name: "invalid upstream authority type",
			upstreamAuth: &v1alpha1.UpstreamAuthority{
				Type: "unsupported",
			},
			expectError: true,
			errorMsg:    "unsupported upstream authority type",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := createValidConfig()
			config.UpstreamAuthority = tt.upstreamAuth

			confMap, err := generateServerConfMap(config)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error containing %q, got nil", tt.errorMsg)
				} else if !strings.Contains(err.Error(), tt.errorMsg) {
					t.Errorf("Expected error containing %q, got %q", tt.errorMsg, err.Error())
				}
				return
			}

			if err != nil {
				t.Fatalf("Failed to generate server config: %v", err)
			}

			// Test plugins section
			plugins, ok := confMap["plugins"].(map[string]interface{})
			if !ok {
				t.Fatal("Failed to get plugins section")
			}

			// Check UpstreamAuthority plugin presence
			upstreamAuthority, exists := plugins["UpstreamAuthority"]
			if tt.expectPlugin {
				if !exists {
					t.Fatal("Expected UpstreamAuthority plugin to exist")
				}

				upstreamAuthorityList, ok := upstreamAuthority.([]map[string]interface{})
				if !ok || len(upstreamAuthorityList) == 0 {
					t.Fatal("Expected UpstreamAuthority to be non-empty list")
				}

				plugin := upstreamAuthorityList[0]

				// Check that the expected plugin type exists
				_, exists := plugin[tt.pluginType]
				if !exists {
					t.Errorf("Expected plugin type %q to exist in UpstreamAuthority plugin", tt.pluginType)
				}
			} else {
				if exists {
					t.Error("Expected UpstreamAuthority plugin to not exist")
				}
			}

			// Verify other expected plugins still exist
			expectedPlugins := []string{"DataStore", "KeyManager", "NodeAttestor", "Notifier"}
			for _, pluginName := range expectedPlugins {
				if _, exists := plugins[pluginName]; !exists {
					t.Errorf("Expected plugin %q to exist", pluginName)
				}
			}
		})
	}
}

// Test the getOrDefault helper function
func TestGetOrDefault(t *testing.T) {
	tests := []struct {
		name         string
		value        string
		defaultValue string
		expected     string
	}{
		{
			name:         "non-empty value",
			value:        "custom-value",
			defaultValue: "default-value",
			expected:     "custom-value",
		},
		{
			name:         "empty value",
			value:        "",
			defaultValue: "default-value",
			expected:     "default-value",
		},
		{
			name:         "empty default",
			value:        "",
			defaultValue: "",
			expected:     "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getOrDefault(tt.value, tt.defaultValue)
			if result != tt.expected {
				t.Errorf("Expected %q, got %q", tt.expected, result)
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
	}
}

// TestUpstreamAuthorityJSONOmitemptyBehavior tests that JSON omitempty tags work correctly
// for UpstreamAuthority structs - empty fields should be omitted, non-empty fields included
func TestUpstreamAuthorityJSONOmitemptyBehavior(t *testing.T) {
	tests := []struct {
		name                  string
		upstreamAuth          *v1alpha1.UpstreamAuthority
		expectedPresent       []string            // JSON keys that should be present
		expectedAbsent        []string            // JSON keys that should be absent
		expectedNestedPresent map[string][]string // nested object keys that should be present
		expectedNestedAbsent  map[string][]string // nested object keys that should be absent
	}{
		{
			name:           "Empty UpstreamAuthority should omit all fields",
			upstreamAuth:   &v1alpha1.UpstreamAuthority{},
			expectedAbsent: []string{"type", "spire", "vault", "certManager"},
		},
		{
			name: "UpstreamAuthority with only type should omit other fields",
			upstreamAuth: &v1alpha1.UpstreamAuthority{
				Type: "cert-manager",
			},
			expectedPresent: []string{"type"},
			expectedAbsent:  []string{"spire", "vault", "certManager"},
		},
		{
			name: "CertManager with minimal config should omit empty optional fields",
			upstreamAuth: &v1alpha1.UpstreamAuthority{
				Type: "cert-manager",
				CertManager: &v1alpha1.UpstreamAuthorityCertManager{
					IssuerName: "test-issuer",
				},
			},
			expectedPresent: []string{"type", "certManager"},
			expectedAbsent:  []string{"spire", "vault"},
			expectedNestedPresent: map[string][]string{
				"certManager": {"issuerName"},
			},
			expectedNestedAbsent: map[string][]string{
				"certManager": {"issuerKind", "issuerGroup", "namespace", "kubeConfigSecretName"},
			},
		},
		{
			name: "CertManager with all fields should include all",
			upstreamAuth: &v1alpha1.UpstreamAuthority{
				Type: "cert-manager",
				CertManager: &v1alpha1.UpstreamAuthorityCertManager{
					IssuerName:           "test-issuer",
					IssuerKind:           "ClusterIssuer",
					IssuerGroup:          "cert-manager.io",
					Namespace:            "test-namespace",
					KubeConfigSecretName: "kubeconfig-secret",
				},
			},
			expectedPresent: []string{"type", "certManager"},
			expectedAbsent:  []string{"spire", "vault"},
			expectedNestedPresent: map[string][]string{
				"certManager": {"issuerName", "issuerKind", "issuerGroup", "namespace", "kubeConfigSecretName"},
			},
		},
		{
			name: "Vault with minimal config should omit empty fields",
			upstreamAuth: &v1alpha1.UpstreamAuthority{
				Type: "vault",
				Vault: &v1alpha1.UpstreamAuthorityVault{
					VaultAddress:  "https://vault.example.com",
					PkiMountPoint: "pki",
					CaCertSecret:  "vault-ca-secret",
					TokenAuth: &v1alpha1.TokenAuth{
						Token: "test-token",
					},
				},
			},
			expectedPresent: []string{"type", "vault"},
			expectedAbsent:  []string{"spire", "certManager"},
			expectedNestedPresent: map[string][]string{
				"vault": {"vaultAddress", "pkiMountPoint", "caCertSecret", "tokenAuth"},
			},
			expectedNestedAbsent: map[string][]string{
				"vault": {"namespace", "certAuth", "appRoleAuth", "k8sAuth"},
			},
		},
		{
			name: "Vault with namespace should include namespace field",
			upstreamAuth: &v1alpha1.UpstreamAuthority{
				Type: "vault",
				Vault: &v1alpha1.UpstreamAuthorityVault{
					VaultAddress:  "https://vault.example.com",
					Namespace:     "vault-namespace",
					PkiMountPoint: "pki",
					CaCertSecret:  "vault-ca-secret",
					TokenAuth: &v1alpha1.TokenAuth{
						Token: "test-token",
					},
				},
			},
			expectedPresent: []string{"type", "vault"},
			expectedAbsent:  []string{"spire", "certManager"},
			expectedNestedPresent: map[string][]string{
				"vault": {"vaultAddress", "namespace", "pkiMountPoint", "caCertSecret", "tokenAuth"},
			},
			expectedNestedAbsent: map[string][]string{
				"vault": {"certAuth", "appRoleAuth", "k8sAuth"},
			},
		},
		{
			name: "Vault CertAuth without optional role name should omit it",
			upstreamAuth: &v1alpha1.UpstreamAuthority{
				Type: "vault",
				Vault: &v1alpha1.UpstreamAuthorityVault{
					VaultAddress:  "https://vault.example.com",
					PkiMountPoint: "pki",
					CaCertSecret:  "vault-ca-secret",
					CertAuth: &v1alpha1.CertAuth{
						CertAuthMountPoint: "cert",
						ClientCertSecret:   "client-cert-secret",
						ClientKeySecret:    "client-key-secret",
					},
				},
			},
			expectedPresent: []string{"type", "vault"},
			expectedAbsent:  []string{"spire", "certManager"},
			expectedNestedPresent: map[string][]string{
				"vault": {"vaultAddress", "pkiMountPoint", "caCertSecret", "certAuth"},
			},
			expectedNestedAbsent: map[string][]string{
				"vault":    {"namespace", "tokenAuth", "appRoleAuth", "k8sAuth"},
				"certAuth": {"certAuthRoleName"},
			},
		},
		{
			name: "Vault CertAuth with role name should include it",
			upstreamAuth: &v1alpha1.UpstreamAuthority{
				Type: "vault",
				Vault: &v1alpha1.UpstreamAuthorityVault{
					VaultAddress:  "https://vault.example.com",
					PkiMountPoint: "pki",
					CaCertSecret:  "vault-ca-secret",
					CertAuth: &v1alpha1.CertAuth{
						CertAuthMountPoint: "cert",
						ClientCertSecret:   "client-cert-secret",
						ClientKeySecret:    "client-key-secret",
						CertAuthRoleName:   "test-role",
					},
				},
			},
			expectedPresent: []string{"type", "vault"},
			expectedAbsent:  []string{"spire", "certManager"},
			expectedNestedPresent: map[string][]string{
				"vault":    {"vaultAddress", "pkiMountPoint", "caCertSecret", "certAuth"},
				"certAuth": {"certAuthMountPoint", "clientCertSecret", "clientKeySecret", "certAuthRoleName"},
			},
			expectedNestedAbsent: map[string][]string{
				"vault": {"namespace", "tokenAuth", "appRoleAuth", "k8sAuth"},
			},
		},
		{
			name: "Spire config should include all fields",
			upstreamAuth: &v1alpha1.UpstreamAuthority{
				Type: "spire",
				Spire: &v1alpha1.UpstreamAuthoritySpire{
					ServerAddress:     "spire-server.example.com",
					ServerPort:        "8081",
					WorkloadSocketAPI: "/tmp/spire-agent/public/api.sock",
				},
			},
			expectedPresent: []string{"type", "spire"},
			expectedAbsent:  []string{"vault", "certManager"},
			expectedNestedPresent: map[string][]string{
				"spire": {"serverAddress", "serverPort", "workloadSocketApi"},
			},
		},
		{
			name: "Empty Spire config should omit all optional fields",
			upstreamAuth: &v1alpha1.UpstreamAuthority{
				Type:  "spire",
				Spire: &v1alpha1.UpstreamAuthoritySpire{},
			},
			expectedPresent: []string{"type", "spire"},
			expectedAbsent:  []string{"vault", "certManager"},
			expectedNestedAbsent: map[string][]string{
				"spire": {"serverAddress", "serverPort", "workloadSocketApi"},
			},
		},
		{
			name: "CertManager with empty strings should omit empty fields due to omitempty",
			upstreamAuth: &v1alpha1.UpstreamAuthority{
				Type: "cert-manager",
				CertManager: &v1alpha1.UpstreamAuthorityCertManager{
					IssuerName:  "test-issuer",
					IssuerKind:  "", // empty string should be omitted with omitempty
					IssuerGroup: "", // empty string should be omitted with omitempty
					Namespace:   "", // empty string should be omitted with omitempty
				},
			},
			expectedPresent: []string{"type", "certManager"},
			expectedAbsent:  []string{"spire", "vault"},
			expectedNestedPresent: map[string][]string{
				"certManager": {"issuerName"},
			},
			expectedNestedAbsent: map[string][]string{
				"certManager": {"issuerKind", "issuerGroup", "namespace", "kubeConfigSecretName"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Marshal to JSON
			jsonBytes, err := json.Marshal(tt.upstreamAuth)
			if err != nil {
				t.Fatalf("Failed to marshal UpstreamAuthority to JSON: %v", err)
			}

			// Parse back to generic map for easier inspection
			var result map[string]interface{}
			if err := json.Unmarshal(jsonBytes, &result); err != nil {
				t.Fatalf("Failed to unmarshal JSON back to map: %v", err)
			}

			// Check that expected present fields are actually present
			for _, key := range tt.expectedPresent {
				if _, exists := result[key]; !exists {
					t.Errorf("Expected key %q to be present in JSON, but it was omitted. JSON: %s", key, string(jsonBytes))
				}
			}

			// Check that expected absent fields are actually absent
			for _, key := range tt.expectedAbsent {
				if _, exists := result[key]; exists {
					t.Errorf("Expected key %q to be omitted from JSON due to omitempty, but it was present. JSON: %s", key, string(jsonBytes))
				}
			}

			// Check nested object fields
			for parentKey, expectedKeys := range tt.expectedNestedPresent {
				if parentObj, exists := result[parentKey]; exists {
					if parentMap, ok := parentObj.(map[string]interface{}); ok {
						for _, nestedKey := range expectedKeys {
							if _, nestedExists := parentMap[nestedKey]; !nestedExists {
								t.Errorf("Expected nested key %q.%q to be present in JSON, but it was omitted. JSON: %s", parentKey, nestedKey, string(jsonBytes))
							}
						}
					} else {
						t.Errorf("Expected %q to be an object, got %T", parentKey, parentObj)
					}
				}
			}

			for parentKey, expectedAbsentKeys := range tt.expectedNestedAbsent {
				if parentObj, exists := result[parentKey]; exists {
					if parentMap, ok := parentObj.(map[string]interface{}); ok {
						for _, nestedKey := range expectedAbsentKeys {
							if _, nestedExists := parentMap[nestedKey]; nestedExists {
								t.Errorf("Expected nested key %q.%q to be omitted from JSON due to omitempty, but it was present. JSON: %s", parentKey, nestedKey, string(jsonBytes))
							}
						}
					}
				}
			}

			// Test round-trip: unmarshal back to struct and ensure it matches
			var unmarshaled v1alpha1.UpstreamAuthority
			if err := json.Unmarshal(jsonBytes, &unmarshaled); err != nil {
				t.Fatalf("Failed to unmarshal JSON back to UpstreamAuthority struct: %v", err)
			}

			// Verify the unmarshaled struct equals the original (for non-nil fields)
			if unmarshaled.Type != tt.upstreamAuth.Type {
				t.Errorf("Round-trip failed: Type field mismatch. Expected %q, got %q", tt.upstreamAuth.Type, unmarshaled.Type)
			}
		})
	}
}

// TestUpstreamAuthorityJSONOmitemptyEdgeCases tests unique edge cases for JSON omitempty behavior
// that complement the main behavior test without duplication
func TestUpstreamAuthorityJSONOmitemptyEdgeCases(t *testing.T) {
	tests := []struct {
		name                  string
		upstreamAuth          *v1alpha1.UpstreamAuthority
		expectedPresent       []string
		expectedAbsent        []string
		expectedNestedPresent map[string][]string
		expectedNestedAbsent  map[string][]string
	}{
		{
			name: "Vault with empty TokenAuth struct but all auth methods nil should omit all auth fields",
			upstreamAuth: &v1alpha1.UpstreamAuthority{
				Type: "vault",
				Vault: &v1alpha1.UpstreamAuthorityVault{
					VaultAddress:  "https://vault.example.com",
					PkiMountPoint: "pki",
					CaCertSecret:  "vault-ca-secret",
					// All auth methods explicitly nil
					TokenAuth:   nil,
					CertAuth:    nil,
					AppRoleAuth: nil,
					K8sAuth:     nil,
				},
			},
			expectedPresent: []string{"type", "vault"},
			expectedNestedPresent: map[string][]string{
				"vault": {"vaultAddress", "pkiMountPoint", "caCertSecret"},
			},
			expectedNestedAbsent: map[string][]string{
				"vault": {"namespace", "tokenAuth", "certAuth", "appRoleAuth", "k8sAuth"},
			},
		},
		{
			name: "Empty TokenAuth struct should include tokenAuth field but omit empty token",
			upstreamAuth: &v1alpha1.UpstreamAuthority{
				Type: "vault",
				Vault: &v1alpha1.UpstreamAuthorityVault{
					VaultAddress:  "https://vault.example.com",
					PkiMountPoint: "pki",
					CaCertSecret:  "vault-ca-secret",
					TokenAuth: &v1alpha1.TokenAuth{
						Token: "", // empty string will be omitted
					},
				},
			},
			expectedPresent: []string{"type", "vault"},
			expectedNestedPresent: map[string][]string{
				"vault": {"vaultAddress", "pkiMountPoint", "caCertSecret", "tokenAuth"},
			},
			expectedNestedAbsent: map[string][]string{
				"vault": {"namespace", "certAuth", "appRoleAuth", "k8sAuth"},
			},
		},
		{
			name: "Mixed populated and empty fields in K8sAuth should handle correctly",
			upstreamAuth: &v1alpha1.UpstreamAuthority{
				Type: "vault",
				Vault: &v1alpha1.UpstreamAuthorityVault{
					VaultAddress:  "https://vault.example.com",
					Namespace:     "vault-ns", // populated
					PkiMountPoint: "pki",
					CaCertSecret:  "vault-ca-secret",
					K8sAuth: &v1alpha1.K8sAuth{
						K8sAuthMountPoint: "kubernetes",
						K8sAuthRoleName:   "test-role",
						TokenPath:         "", // empty string will be omitted
					},
				},
			},
			expectedPresent: []string{"type", "vault"},
			expectedNestedPresent: map[string][]string{
				"vault": {"vaultAddress", "namespace", "pkiMountPoint", "caCertSecret", "k8sAuth"},
			},
			expectedNestedAbsent: map[string][]string{
				"vault": {"tokenAuth", "certAuth", "appRoleAuth"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			jsonBytes, err := json.Marshal(tt.upstreamAuth)
			if err != nil {
				t.Fatalf("Failed to marshal UpstreamAuthority to JSON: %v", err)
			}

			var result map[string]interface{}
			if err := json.Unmarshal(jsonBytes, &result); err != nil {
				t.Fatalf("Failed to unmarshal JSON back to map: %v", err)
			}

			// Check top-level fields
			for _, key := range tt.expectedPresent {
				if _, exists := result[key]; !exists {
					t.Errorf("Expected key %q to be present in JSON, but it was omitted. JSON: %s", key, string(jsonBytes))
				}
			}

			for _, key := range tt.expectedAbsent {
				if _, exists := result[key]; exists {
					t.Errorf("Expected key %q to be omitted from JSON due to omitempty, but it was present. JSON: %s", key, string(jsonBytes))
				}
			}

			// Check nested fields
			for parentKey, expectedKeys := range tt.expectedNestedPresent {
				if parentObj, exists := result[parentKey]; exists {
					if parentMap, ok := parentObj.(map[string]interface{}); ok {
						for _, nestedKey := range expectedKeys {
							if _, nestedExists := parentMap[nestedKey]; !nestedExists {
								t.Errorf("Expected nested key %q.%q to be present in JSON, but it was omitted. JSON: %s", parentKey, nestedKey, string(jsonBytes))
							}
						}
					} else {
						// Handle deep nesting (e.g., vault.certAuth)
						if nested, nestedOk := findNestedObject(result, parentKey); nestedOk {
							for _, nestedKey := range expectedKeys {
								if _, nestedExists := nested[nestedKey]; !nestedExists {
									t.Errorf("Expected nested key %q.%q to be present in JSON, but it was omitted. JSON: %s", parentKey, nestedKey, string(jsonBytes))
								}
							}
						}
					}
				} else if len(expectedKeys) > 0 {
					t.Errorf("Expected parent key %q to exist for nested key checks", parentKey)
				}
			}

			for parentKey, expectedAbsentKeys := range tt.expectedNestedAbsent {
				if parentObj, exists := result[parentKey]; exists {
					if parentMap, ok := parentObj.(map[string]interface{}); ok {
						for _, nestedKey := range expectedAbsentKeys {
							if _, nestedExists := parentMap[nestedKey]; nestedExists {
								t.Errorf("Expected nested key %q.%q to be omitted from JSON due to omitempty, but it was present. JSON: %s", parentKey, nestedKey, string(jsonBytes))
							}
						}
					}
				}
			}

			// Test round-trip consistency
			var unmarshaled v1alpha1.UpstreamAuthority
			if err := json.Unmarshal(jsonBytes, &unmarshaled); err != nil {
				t.Fatalf("Failed to unmarshal JSON back to UpstreamAuthority struct: %v", err)
			}

			// Marshal the unmarshaled struct again
			jsonBytes2, err := json.Marshal(&unmarshaled)
			if err != nil {
				t.Fatalf("Failed to marshal unmarshaled struct back to JSON: %v", err)
			}

			// The JSON should be equivalent (may not be identical due to field ordering)
			var result2 map[string]interface{}
			if err := json.Unmarshal(jsonBytes2, &result2); err != nil {
				t.Fatalf("Failed to unmarshal second JSON: %v", err)
			}
		})
	}
}

// TestUpstreamAuthorityComplexConfigurations tests complex real-world configurations
func TestUpstreamAuthorityComplexConfigurations(t *testing.T) {
	tests := []struct {
		name         string
		upstreamAuth *v1alpha1.UpstreamAuthority
		expectError  bool
		errorMsg     string
	}{
		{
			name: "Vault with all auth methods configured should marshal successfully",
			upstreamAuth: &v1alpha1.UpstreamAuthority{
				Type: "vault",
				Vault: &v1alpha1.UpstreamAuthorityVault{
					VaultAddress:  "https://vault.example.com",
					PkiMountPoint: "pki",
					CaCertSecret:  "vault-ca-secret",
					TokenAuth: &v1alpha1.TokenAuth{
						Token: "test-token",
					},
					CertAuth: &v1alpha1.CertAuth{
						CertAuthMountPoint: "cert",
						ClientCertSecret:   "client-cert-secret",
						ClientKeySecret:    "client-key-secret",
						CertAuthRoleName:   "test-role",
					},
					AppRoleAuth: &v1alpha1.AppRoleAuth{
						AppRoleMountPoint: "approle",
						AppRoleID:         "role-id",
						AppRoleSecretID:   "secret-id",
					},
					K8sAuth: &v1alpha1.K8sAuth{
						K8sAuthMountPoint: "kubernetes",
						K8sAuthRoleName:   "test-role",
						TokenPath:         "/custom/path",
					},
				},
			},
			expectError: false, // JSON marshaling should work
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			jsonBytes, err := json.Marshal(tt.upstreamAuth)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error containing %q, got nil", tt.errorMsg)
				} else if !strings.Contains(err.Error(), tt.errorMsg) {
					t.Errorf("Expected error containing %q, got %q", tt.errorMsg, err.Error())
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			// Validate JSON structure
			var result map[string]interface{}
			if err := json.Unmarshal(jsonBytes, &result); err != nil {
				t.Fatalf("Failed to unmarshal JSON back to map: %v", err)
			}

			// Ensure the basic structure is correct
			if tt.upstreamAuth.Type != "" {
				if typeVal, exists := result["type"]; !exists || typeVal != tt.upstreamAuth.Type {
					t.Errorf("Expected type %q, got %v", tt.upstreamAuth.Type, typeVal)
				}
			}
		})
	}
}

// TestUpstreamAuthorityConfigMapGenerationEdgeCases tests edge cases in configmap generation
func TestUpstreamAuthorityConfigMapGenerationEdgeCases(t *testing.T) {
	tests := []struct {
		name                    string
		upstreamAuth            *v1alpha1.UpstreamAuthority
		expectedInConfigData    []string
		expectedNotInConfigData []string
		expectError             bool
		errorMsg                string
	}{
		{
			name: "Vault with special characters in fields should be properly escaped",
			upstreamAuth: &v1alpha1.UpstreamAuthority{
				Type: "vault",
				Vault: &v1alpha1.UpstreamAuthorityVault{
					VaultAddress:  "https://vault.example.com:8200",
					Namespace:     "vault/namespace/with/slashes",
					PkiMountPoint: "pki-with-dashes",
					CaCertSecret:  "vault-ca-secret",
					TokenAuth: &v1alpha1.TokenAuth{
						Token: "hvs.CAESIJ_special_chars_123",
					},
				},
			},
			expectedInConfigData: []string{
				"vault_addr",
				"pki_mount_point",
				"token_auth",
				"https://vault.example.com:8200",
				"vault/namespace/with/slashes",
				"pki-with-dashes",
			},
		},
		{
			name: "CertManager with very long issuer name should work",
			upstreamAuth: &v1alpha1.UpstreamAuthority{
				Type: "cert-manager",
				CertManager: &v1alpha1.UpstreamAuthorityCertManager{
					IssuerName:  "very-long-issuer-name-that-exceeds-normal-length-boundaries-but-should-still-work-fine",
					IssuerKind:  "ClusterIssuer",
					IssuerGroup: "cert-manager.io",
					Namespace:   "very-long-namespace-name-that-also-exceeds-normal-boundaries",
				},
			},
			expectedInConfigData: []string{
				"issuer_name",
				"very-long-issuer-name-that-exceeds-normal-length-boundaries-but-should-still-work-fine",
				"very-long-namespace-name-that-also-exceeds-normal-boundaries",
			},
		},
		{
			name: "Spire with IPv6 address should work",
			upstreamAuth: &v1alpha1.UpstreamAuthority{
				Type: "spire",
				Spire: &v1alpha1.UpstreamAuthoritySpire{
					ServerAddress:     "2001:db8::1",
					ServerPort:        "8081",
					WorkloadSocketAPI: "/tmp/spire-agent/public/api.sock",
				},
			},
			expectedInConfigData: []string{
				"server_address",
				"server_port",
				"workload_api_socket",
				"2001:db8::1",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := createValidConfig()
			config.UpstreamAuthority = tt.upstreamAuth

			cm, err := GenerateSpireServerConfigMap(config)

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

			configData, exists := cm.Data["server.conf"]
			if !exists {
				t.Fatal("Expected server.conf data to exist")
			}

			// Validate JSON structure
			var configMap map[string]interface{}
			if err := json.Unmarshal([]byte(configData), &configMap); err != nil {
				t.Fatalf("Generated config is not valid JSON: %v", err)
			}

			// Check expected strings
			for _, expectedStr := range tt.expectedInConfigData {
				if !strings.Contains(configData, expectedStr) {
					t.Errorf("Expected %q to be present in config data, but it was not found. Config: %s", expectedStr, configData)
				}
			}

			// Check strings that should not be present
			for _, notExpectedStr := range tt.expectedNotInConfigData {
				if strings.Contains(configData, notExpectedStr) {
					t.Errorf("Expected %q to NOT be present in config data due to omitempty, but it was found. Config: %s", notExpectedStr, configData)
				}
			}

		})
	}
}

// TestUpstreamAuthoritySecretFieldsOmitemptyBehavior tests JSON omitempty behavior specifically for secret fields
func TestUpstreamAuthoritySecretFieldsOmitemptyBehavior(t *testing.T) {
	tests := []struct {
		name                  string
		upstreamAuth          *v1alpha1.UpstreamAuthority
		expectedPresent       []string
		expectedAbsent        []string
		expectedNestedPresent map[string][]string
		expectedNestedAbsent  map[string][]string
	}{
		{
			name: "CertManager without KubeConfigSecretName should omit it",
			upstreamAuth: &v1alpha1.UpstreamAuthority{
				Type: "cert-manager",
				CertManager: &v1alpha1.UpstreamAuthorityCertManager{
					IssuerName: "test-issuer",
					// KubeConfigSecretName not set - should be omitted
				},
			},
			expectedPresent: []string{"type", "certManager"},
			expectedNestedPresent: map[string][]string{
				"certManager": {"issuerName"},
			},
			expectedNestedAbsent: map[string][]string{
				"certManager": {"kube_config_file"},
			},
		},
		{
			name: "CertManager with empty KubeConfigSecretName should omit it",
			upstreamAuth: &v1alpha1.UpstreamAuthority{
				Type: "cert-manager",
				CertManager: &v1alpha1.UpstreamAuthorityCertManager{
					IssuerName:           "test-issuer",
					KubeConfigSecretName: "", // empty string should be omitted
				},
			},
			expectedPresent: []string{"type", "certManager"},
			expectedNestedPresent: map[string][]string{
				"certManager": {"issuerName"},
			},
			expectedNestedAbsent: map[string][]string{
				"certManager": {"kube_config_file"},
			},
		},
		{
			name: "CertManager with KubeConfigSecretName should include it",
			upstreamAuth: &v1alpha1.UpstreamAuthority{
				Type: "cert-manager",
				CertManager: &v1alpha1.UpstreamAuthorityCertManager{
					IssuerName:           "test-issuer",
					KubeConfigSecretName: "kubeconfig-secret",
				},
			},
			expectedPresent: []string{"type", "certManager"},
			expectedNestedPresent: map[string][]string{
				"certManager": {"issuerName", "kube_config_file"},
			},
		},
		{
			name: "Vault with empty CaCertSecret should omit it",
			upstreamAuth: &v1alpha1.UpstreamAuthority{
				Type: "vault",
				Vault: &v1alpha1.UpstreamAuthorityVault{
					VaultAddress:  "https://vault.example.com",
					PkiMountPoint: "pki",
					CaCertSecret:  "", // empty string should be omitted
					TokenAuth: &v1alpha1.TokenAuth{
						Token: "test-token",
					},
				},
			},
			expectedPresent: []string{"type", "vault"},
			expectedNestedPresent: map[string][]string{
				"vault": {"vaultAddress", "pkiMountPoint", "tokenAuth"},
			},
			expectedNestedAbsent: map[string][]string{
				"vault": {"caCertSecret"},
			},
		},
		{
			name: "Vault with CaCertSecret should include it",
			upstreamAuth: &v1alpha1.UpstreamAuthority{
				Type: "vault",
				Vault: &v1alpha1.UpstreamAuthorityVault{
					VaultAddress:  "https://vault.example.com",
					PkiMountPoint: "pki",
					CaCertSecret:  "vault-ca-secret",
					TokenAuth: &v1alpha1.TokenAuth{
						Token: "test-token",
					},
				},
			},
			expectedPresent: []string{"type", "vault"},
			expectedNestedPresent: map[string][]string{
				"vault": {"vaultAddress", "pkiMountPoint", "caCertSecret", "tokenAuth"},
			},
		},
		{
			name: "Vault CertAuth with empty secret names should omit them",
			upstreamAuth: &v1alpha1.UpstreamAuthority{
				Type: "vault",
				Vault: &v1alpha1.UpstreamAuthorityVault{
					VaultAddress:  "https://vault.example.com",
					PkiMountPoint: "pki",
					CaCertSecret:  "vault-ca-secret",
					CertAuth: &v1alpha1.CertAuth{
						CertAuthMountPoint: "cert",
						ClientCertSecret:   "", // empty string should be omitted
						ClientKeySecret:    "", // empty string should be omitted
					},
				},
			},
			expectedPresent: []string{"type", "vault"},
			expectedNestedPresent: map[string][]string{
				"vault":    {"vaultAddress", "pkiMountPoint", "caCertSecret", "certAuth"},
				"certAuth": {"certAuthMountPoint"},
			},
			expectedNestedAbsent: map[string][]string{
				"certAuth": {"clientCertSecret", "clientKeySecret", "certAuthRoleName"},
			},
		},
		{
			name: "Vault CertAuth with secret names should include them",
			upstreamAuth: &v1alpha1.UpstreamAuthority{
				Type: "vault",
				Vault: &v1alpha1.UpstreamAuthorityVault{
					VaultAddress:  "https://vault.example.com",
					PkiMountPoint: "pki",
					CaCertSecret:  "vault-ca-secret",
					CertAuth: &v1alpha1.CertAuth{
						CertAuthMountPoint: "cert",
						ClientCertSecret:   "client-cert-secret",
						ClientKeySecret:    "client-key-secret",
						CertAuthRoleName:   "test-role",
					},
				},
			},
			expectedPresent: []string{"type", "vault"},
			expectedNestedPresent: map[string][]string{
				"vault":    {"vaultAddress", "pkiMountPoint", "caCertSecret", "certAuth"},
				"certAuth": {"certAuthMountPoint", "clientCertSecret", "clientKeySecret", "certAuthRoleName"},
			},
		},
		{
			name: "Vault AppRoleAuth with empty AppRoleSecretID should omit it",
			upstreamAuth: &v1alpha1.UpstreamAuthority{
				Type: "vault",
				Vault: &v1alpha1.UpstreamAuthorityVault{
					VaultAddress:  "https://vault.example.com",
					PkiMountPoint: "pki",
					CaCertSecret:  "vault-ca-secret",
					AppRoleAuth: &v1alpha1.AppRoleAuth{
						AppRoleMountPoint: "approle",
						AppRoleID:         "role-id-123",
						AppRoleSecretID:   "", // empty string should be omitted
					},
				},
			},
			expectedPresent: []string{"type", "vault"},
			expectedNestedPresent: map[string][]string{
				"vault":       {"vaultAddress", "pkiMountPoint", "caCertSecret", "appRoleAuth"},
				"appRoleAuth": {"appRoleMountPoint", "appRoleID"},
			},
			expectedNestedAbsent: map[string][]string{
				"appRoleAuth": {"appRoleSecretID"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			jsonBytes, err := json.Marshal(tt.upstreamAuth)
			if err != nil {
				t.Fatalf("Failed to marshal UpstreamAuthority to JSON: %v", err)
			}

			var result map[string]interface{}
			if err := json.Unmarshal(jsonBytes, &result); err != nil {
				t.Fatalf("Failed to unmarshal JSON back to map: %v", err)
			}

			// Check top-level fields
			for _, key := range tt.expectedPresent {
				if _, exists := result[key]; !exists {
					t.Errorf("Expected key %q to be present in JSON, but it was omitted. JSON: %s", key, string(jsonBytes))
				}
			}

			for _, key := range tt.expectedAbsent {
				if _, exists := result[key]; exists {
					t.Errorf("Expected key %q to be omitted from JSON due to omitempty, but it was present. JSON: %s", key, string(jsonBytes))
				}
			}

			// Check nested fields
			for parentKey, expectedKeys := range tt.expectedNestedPresent {
				if parentObj, exists := result[parentKey]; exists {
					if parentMap, ok := parentObj.(map[string]interface{}); ok {
						for _, nestedKey := range expectedKeys {
							if _, nestedExists := parentMap[nestedKey]; !nestedExists {
								t.Errorf("Expected nested key %q.%q to be present in JSON, but it was omitted. JSON: %s", parentKey, nestedKey, string(jsonBytes))
							}
						}
					} else {
						// Handle deep nesting (e.g., vault.certAuth)
						if nested, nestedOk := findNestedObject(result, parentKey); nestedOk {
							for _, nestedKey := range expectedKeys {
								if _, nestedExists := nested[nestedKey]; !nestedExists {
									t.Errorf("Expected nested key %q.%q to be present in JSON, but it was omitted. JSON: %s", parentKey, nestedKey, string(jsonBytes))
								}
							}
						}
					}
				}
			}

			for parentKey, expectedAbsentKeys := range tt.expectedNestedAbsent {
				if parentObj, exists := result[parentKey]; exists {
					if parentMap, ok := parentObj.(map[string]interface{}); ok {
						for _, nestedKey := range expectedAbsentKeys {
							if _, nestedExists := parentMap[nestedKey]; nestedExists {
								t.Errorf("Expected nested key %q.%q to be omitted from JSON due to omitempty, but it was present. JSON: %s", parentKey, nestedKey, string(jsonBytes))
							}
						}
					}
				}
			}
		})
	}
}

// findNestedObject finds a nested object by key, handling multiple levels of nesting
func findNestedObject(result map[string]interface{}, key string) (map[string]interface{}, bool) {
	// First try direct access
	if obj, exists := result[key]; exists {
		if objMap, ok := obj.(map[string]interface{}); ok {
			return objMap, true
		}
	}

	// Try to find it within other objects (e.g., certAuth within vault)
	for _, value := range result {
		if valueMap, ok := value.(map[string]interface{}); ok {
			if nested, exists := valueMap[key]; exists {
				if nestedMap, ok := nested.(map[string]interface{}); ok {
					return nestedMap, true
				}
			}
		}
	}

	return nil, false
}

// TestGetUpstreamAuthoritySecretMountsEdgeCases tests edge cases in secret mounting logic
func TestGetUpstreamAuthoritySecretMountsEdgeCases(t *testing.T) {
	tests := []struct {
		name           string
		upstreamAuth   *v1alpha1.UpstreamAuthority
		expectedMounts []secretMountInfo
	}{
		{
			name:           "nil upstream authority returns empty mounts",
			upstreamAuth:   nil,
			expectedMounts: []secretMountInfo{},
		},
		{
			name: "cert-manager with empty KubeConfigSecretName should not mount",
			upstreamAuth: &v1alpha1.UpstreamAuthority{
				Type: "cert-manager",
				CertManager: &v1alpha1.UpstreamAuthorityCertManager{
					IssuerName:           "test-issuer",
					KubeConfigSecretName: "", // empty string should not create mount
				},
			},
			expectedMounts: []secretMountInfo{},
		},
		{
			name: "vault with empty CaCertSecret should still mount (required field)",
			upstreamAuth: &v1alpha1.UpstreamAuthority{
				Type: "vault",
				Vault: &v1alpha1.UpstreamAuthorityVault{
					CaCertSecret: "", // empty but still creates mount since it's required
					TokenAuth: &v1alpha1.TokenAuth{
						Token: "test-token",
					},
				},
			},
			expectedMounts: []secretMountInfo{
				{
					secretName: "",
					mountPath:  "/vault-ca-cert",
					volumeName: "vault-ca-cert",
				},
			},
		},
		{
			name: "vault with CertAuth but empty secret names should still mount",
			upstreamAuth: &v1alpha1.UpstreamAuthority{
				Type: "vault",
				Vault: &v1alpha1.UpstreamAuthorityVault{
					CaCertSecret: "vault-ca-secret",
					CertAuth: &v1alpha1.CertAuth{
						ClientCertSecret: "", // empty but still creates mount
						ClientKeySecret:  "", // empty but still creates mount
					},
				},
			},
			expectedMounts: []secretMountInfo{
				{
					secretName: "vault-ca-secret",
					mountPath:  "/vault-ca-cert",
					volumeName: "vault-ca-cert",
				},
				{
					secretName: "",
					mountPath:  "/vault-client-cert",
					volumeName: "vault-client-cert",
				},
				{
					secretName: "",
					mountPath:  "/vault-client-key",
					volumeName: "vault-client-key",
				},
			},
		},
		{
			name: "vault with all cert auth secrets should mount all",
			upstreamAuth: &v1alpha1.UpstreamAuthority{
				Type: "vault",
				Vault: &v1alpha1.UpstreamAuthorityVault{
					CaCertSecret: "vault-ca-secret",
					CertAuth: &v1alpha1.CertAuth{
						ClientCertSecret: "client-cert-secret",
						ClientKeySecret:  "client-key-secret",
					},
				},
			},
			expectedMounts: []secretMountInfo{
				{
					secretName: "vault-ca-secret",
					mountPath:  "/vault-ca-cert",
					volumeName: "vault-ca-cert",
				},
				{
					secretName: "client-cert-secret",
					mountPath:  "/vault-client-cert",
					volumeName: "vault-client-cert",
				},
				{
					secretName: "client-key-secret",
					mountPath:  "/vault-client-key",
					volumeName: "vault-client-key",
				},
			},
		},
		{
			name: "unsupported upstream authority type should return no mounts",
			upstreamAuth: &v1alpha1.UpstreamAuthority{
				Type: "unsupported-type",
			},
			expectedMounts: []secretMountInfo{},
		},
		{
			name: "vault with nil CertAuth should only mount CA cert",
			upstreamAuth: &v1alpha1.UpstreamAuthority{
				Type: "vault",
				Vault: &v1alpha1.UpstreamAuthorityVault{
					CaCertSecret: "vault-ca-secret",
					CertAuth:     nil, // no cert auth
					TokenAuth: &v1alpha1.TokenAuth{
						Token: "test-token",
					},
				},
			},
			expectedMounts: []secretMountInfo{
				{
					secretName: "vault-ca-secret",
					mountPath:  "/vault-ca-cert",
					volumeName: "vault-ca-cert",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mounts := getUpstreamAuthoritySecretMounts(tt.upstreamAuth)

			if len(mounts) != len(tt.expectedMounts) {
				t.Errorf("Expected %d mounts, got %d. Expected: %+v, Got: %+v", len(tt.expectedMounts), len(mounts), tt.expectedMounts, mounts)
				return
			}

			for i, expectedMount := range tt.expectedMounts {
				if i >= len(mounts) {
					t.Errorf("Expected mount %d not found: %+v", i, expectedMount)
					continue
				}

				actualMount := mounts[i]
				if actualMount.secretName != expectedMount.secretName {
					t.Errorf("Mount %d: expected secretName %q, got %q", i, expectedMount.secretName, actualMount.secretName)
				}
				if actualMount.mountPath != expectedMount.mountPath {
					t.Errorf("Mount %d: expected mountPath %q, got %q", i, expectedMount.mountPath, actualMount.mountPath)
				}
				if actualMount.volumeName != expectedMount.volumeName {
					t.Errorf("Mount %d: expected volumeName %q, got %q", i, expectedMount.volumeName, actualMount.volumeName)
				}
			}
		})
	}
}

// TestUpstreamAuthoritySecretPathIntegration tests that secrets are correctly referenced in config generation
func TestUpstreamAuthoritySecretPathIntegration(t *testing.T) {
	tests := []struct {
		name                 string
		upstreamAuth         *v1alpha1.UpstreamAuthority
		expectedConfigPaths  []string // paths that should appear in generated config
		expectedSecretMounts int      // number of secret mounts expected
	}{
		{
			name: "cert-manager with kubeconfig secret should reference correct path",
			upstreamAuth: &v1alpha1.UpstreamAuthority{
				Type: "cert-manager",
				CertManager: &v1alpha1.UpstreamAuthorityCertManager{
					IssuerName:           "test-issuer",
					KubeConfigSecretName: "kubeconfig-secret",
				},
			},
			expectedConfigPaths:  []string{"/cert-manager-kubeconfig/kubeconfig"},
			expectedSecretMounts: 1,
		},
		{
			name: "cert-manager without kubeconfig should not reference any secret path",
			upstreamAuth: &v1alpha1.UpstreamAuthority{
				Type: "cert-manager",
				CertManager: &v1alpha1.UpstreamAuthorityCertManager{
					IssuerName: "test-issuer",
				},
			},
			expectedConfigPaths:  []string{}, // no secret paths
			expectedSecretMounts: 0,
		},
		{
			name: "vault with token auth should reference CA cert path",
			upstreamAuth: &v1alpha1.UpstreamAuthority{
				Type: "vault",
				Vault: &v1alpha1.UpstreamAuthorityVault{
					VaultAddress:  "https://vault.example.com",
					PkiMountPoint: "pki",
					CaCertSecret:  "vault-ca-secret",
					TokenAuth: &v1alpha1.TokenAuth{
						Token: "test-token",
					},
				},
			},
			expectedConfigPaths:  []string{"/vault-ca-cert/ca.crt"},
			expectedSecretMounts: 1,
		},
		{
			name: "vault with cert auth should reference all cert paths",
			upstreamAuth: &v1alpha1.UpstreamAuthority{
				Type: "vault",
				Vault: &v1alpha1.UpstreamAuthorityVault{
					VaultAddress:  "https://vault.example.com",
					PkiMountPoint: "pki",
					CaCertSecret:  "vault-ca-secret",
					CertAuth: &v1alpha1.CertAuth{
						CertAuthMountPoint: "cert",
						ClientCertSecret:   "client-cert-secret",
						ClientKeySecret:    "client-key-secret",
					},
				},
			},
			expectedConfigPaths: []string{
				"/vault-ca-cert/ca.crt",
				"/vault-client-cert/tls.crt",
				"/vault-client-key/tls.key",
			},
			expectedSecretMounts: 3,
		},
		{
			name: "spire upstream authority should not reference any secret paths",
			upstreamAuth: &v1alpha1.UpstreamAuthority{
				Type: "spire",
				Spire: &v1alpha1.UpstreamAuthoritySpire{
					ServerAddress:     "upstream-spire-server",
					ServerPort:        "8081",
					WorkloadSocketAPI: "/tmp/spire-agent/public/api.sock",
				},
			},
			expectedConfigPaths:  []string{},
			expectedSecretMounts: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test secret mount generation
			mounts := getUpstreamAuthoritySecretMounts(tt.upstreamAuth)
			if len(mounts) != tt.expectedSecretMounts {
				t.Errorf("Expected %d secret mounts, got %d. Mounts: %+v", tt.expectedSecretMounts, len(mounts), mounts)
			}

			// Test plugin configuration generation
			plugin, err := generateUpstreamAuthorityPlugin(tt.upstreamAuth)
			if err != nil {
				t.Fatalf("Failed to generate upstream authority plugin: %v", err)
			}

			// Convert to JSON to check for paths
			pluginJSON, err := json.Marshal(plugin)
			if err != nil {
				t.Fatalf("Failed to marshal plugin to JSON: %v", err)
			}

			pluginJSONStr := string(pluginJSON)

			// Check that expected paths are referenced in the generated config
			for _, expectedPath := range tt.expectedConfigPaths {
				if !strings.Contains(pluginJSONStr, expectedPath) {
					t.Errorf("Expected path %q to be referenced in plugin config, but it was not found. Config: %s", expectedPath, pluginJSONStr)
				}
			}

			// Test full config map generation
			config := createValidConfig()
			config.UpstreamAuthority = tt.upstreamAuth

			cm, err := GenerateSpireServerConfigMap(config)
			if err != nil {
				t.Fatalf("Failed to generate config map: %v", err)
			}

			configData := cm.Data["server.conf"]

			// Check that expected paths are in the full config
			for _, expectedPath := range tt.expectedConfigPaths {
				if !strings.Contains(configData, expectedPath) {
					t.Errorf("Expected path %q to be referenced in server config, but it was not found. Config: %s", expectedPath, configData)
				}
			}
		})
	}
}
