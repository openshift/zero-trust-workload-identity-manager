package spire_agent

import (
	"encoding/json"
	"testing"

	"github.com/openshift/zero-trust-workload-identity-manager/api/v1alpha1"
	"github.com/openshift/zero-trust-workload-identity-manager/pkg/controller/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestGenerateAgentConfig(t *testing.T) {
	tests := []struct {
		name     string
		cfg      *v1alpha1.SpireAgent
		expected map[string]interface{}
	}{
		{
			name: "minimal config",
			cfg: &v1alpha1.SpireAgent{
				Spec: v1alpha1.SpireAgentSpec{
					TrustDomain: "example.org",
				},
			},
			expected: map[string]interface{}{
				"agent": map[string]interface{}{
					"data_dir":          "/var/lib/spire",
					"log_level":         "info",
					"log_format":        "text",
					"retry_bootstrap":   true,
					"server_address":    "spire-server.zero-trust-workload-identity-manager",
					"server_port":       "443",
					"socket_path":       "/tmp/spire-agent/public/spire-agent.sock",
					"trust_bundle_path": "/run/spire/bundle/bundle.crt",
					"trust_domain":      "example.org",
				},
				"health_checks": map[string]interface{}{
					"bind_address":     "0.0.0.0",
					"bind_port":        9982,
					"listener_enabled": true,
					"live_path":        "/live",
					"ready_path":       "/ready",
				},
				"plugins": map[string]interface{}{
					"KeyManager": []map[string]interface{}{
						{"memory": map[string]interface{}{"plugin_data": nil}},
					},
				},
				"telemetry": map[string]interface{}{
					"Prometheus": map[string]interface{}{
						"host": "0.0.0.0",
						"port": "9402",
					},
				},
			},
		},
		{
			name: "config with k8s_psat node attestor enabled",
			cfg: &v1alpha1.SpireAgent{
				Spec: v1alpha1.SpireAgentSpec{
					TrustDomain: "test.domain",
					ClusterName: "test-cluster",
					NodeAttestor: &v1alpha1.NodeAttestor{
						K8sPSATEnabled: "true",
					},
				},
			},
			expected: map[string]interface{}{
				"agent": map[string]interface{}{
					"data_dir":          "/var/lib/spire",
					"log_level":         "info",
					"log_format":        "text",
					"retry_bootstrap":   true,
					"server_address":    "spire-server.zero-trust-workload-identity-manager",
					"server_port":       "443",
					"socket_path":       "/tmp/spire-agent/public/spire-agent.sock",
					"trust_bundle_path": "/run/spire/bundle/bundle.crt",
					"trust_domain":      "test.domain",
				},
				"health_checks": map[string]interface{}{
					"bind_address":     "0.0.0.0",
					"bind_port":        9982,
					"listener_enabled": true,
					"live_path":        "/live",
					"ready_path":       "/ready",
				},
				"plugins": map[string]interface{}{
					"KeyManager": []map[string]interface{}{
						{"memory": map[string]interface{}{"plugin_data": nil}},
					},
					"NodeAttestor": []map[string]interface{}{
						{
							"k8s_psat": map[string]interface{}{
								"plugin_data": map[string]interface{}{
									"cluster": "test-cluster",
								},
							},
						},
					},
				},
				"telemetry": map[string]interface{}{
					"Prometheus": map[string]interface{}{
						"host": "0.0.0.0",
						"port": "9402",
					},
				},
			},
		},
		{
			name: "config with k8s workload attestor enabled",
			cfg: &v1alpha1.SpireAgent{
				Spec: v1alpha1.SpireAgentSpec{
					TrustDomain: "workload.domain",
					WorkloadAttestors: &v1alpha1.WorkloadAttestors{
						K8sEnabled:                "true",
						DisableContainerSelectors: "true",
						UseNewContainerLocator:    "false",
					},
				},
			},
			expected: map[string]interface{}{
				"agent": map[string]interface{}{
					"data_dir":          "/var/lib/spire",
					"log_level":         "info",
					"log_format":        "text",
					"retry_bootstrap":   true,
					"server_address":    "spire-server.zero-trust-workload-identity-manager",
					"server_port":       "443",
					"socket_path":       "/tmp/spire-agent/public/spire-agent.sock",
					"trust_bundle_path": "/run/spire/bundle/bundle.crt",
					"trust_domain":      "workload.domain",
				},
				"health_checks": map[string]interface{}{
					"bind_address":     "0.0.0.0",
					"bind_port":        9982,
					"listener_enabled": true,
					"live_path":        "/live",
					"ready_path":       "/ready",
				},
				"plugins": map[string]interface{}{
					"KeyManager": []map[string]interface{}{
						{"memory": map[string]interface{}{"plugin_data": nil}},
					},
					"WorkloadAttestor": []map[string]interface{}{
						{
							"k8s": map[string]interface{}{
								"plugin_data": map[string]interface{}{
									"disable_container_selectors":    true,
									"node_name_env":                  "MY_NODE_NAME",
									"use_new_container_locator":      false,
									"verbose_container_locator_logs": false,
									"skip_kubelet_verification":      true,
								},
							},
						},
					},
				},
				"telemetry": map[string]interface{}{
					"Prometheus": map[string]interface{}{
						"host": "0.0.0.0",
						"port": "9402",
					},
				},
			},
		},
		{
			name: "config with both attestors enabled",
			cfg: &v1alpha1.SpireAgent{
				Spec: v1alpha1.SpireAgentSpec{
					TrustDomain: "full.domain",
					ClusterName: "full-cluster",
					NodeAttestor: &v1alpha1.NodeAttestor{
						K8sPSATEnabled: "true",
					},
					WorkloadAttestors: &v1alpha1.WorkloadAttestors{
						K8sEnabled:                "true",
						DisableContainerSelectors: "false",
						UseNewContainerLocator:    "true",
					},
				},
			},
			expected: map[string]interface{}{
				"agent": map[string]interface{}{
					"data_dir":          "/var/lib/spire",
					"log_level":         "info",
					"log_format":        "text",
					"retry_bootstrap":   true,
					"server_address":    "spire-server.zero-trust-workload-identity-manager",
					"server_port":       "443",
					"socket_path":       "/tmp/spire-agent/public/spire-agent.sock",
					"trust_bundle_path": "/run/spire/bundle/bundle.crt",
					"trust_domain":      "full.domain",
				},
				"health_checks": map[string]interface{}{
					"bind_address":     "0.0.0.0",
					"bind_port":        9982,
					"listener_enabled": true,
					"live_path":        "/live",
					"ready_path":       "/ready",
				},
				"plugins": map[string]interface{}{
					"KeyManager": []map[string]interface{}{
						{"memory": map[string]interface{}{"plugin_data": nil}},
					},
					"NodeAttestor": []map[string]interface{}{
						{
							"k8s_psat": map[string]interface{}{
								"plugin_data": map[string]interface{}{
									"cluster": "full-cluster",
								},
							},
						},
					},
					"WorkloadAttestor": []map[string]interface{}{
						{
							"k8s": map[string]interface{}{
								"plugin_data": map[string]interface{}{
									"disable_container_selectors":    false,
									"node_name_env":                  "MY_NODE_NAME",
									"use_new_container_locator":      true,
									"verbose_container_locator_logs": false,
									"skip_kubelet_verification":      true,
								},
							},
						},
					},
				},
				"telemetry": map[string]interface{}{
					"Prometheus": map[string]interface{}{
						"host": "0.0.0.0",
						"port": "9402",
					},
				},
			},
		},
		{
			name: "config with node attestor disabled",
			cfg: &v1alpha1.SpireAgent{
				Spec: v1alpha1.SpireAgentSpec{
					TrustDomain: "disabled.domain",
					ClusterName: "disabled-cluster",
					NodeAttestor: &v1alpha1.NodeAttestor{
						K8sPSATEnabled: "false",
					},
				},
			},
			expected: map[string]interface{}{
				"agent": map[string]interface{}{
					"data_dir":          "/var/lib/spire",
					"log_level":         "info",
					"log_format":        "text",
					"retry_bootstrap":   true,
					"server_address":    "spire-server.zero-trust-workload-identity-manager",
					"server_port":       "443",
					"socket_path":       "/tmp/spire-agent/public/spire-agent.sock",
					"trust_bundle_path": "/run/spire/bundle/bundle.crt",
					"trust_domain":      "disabled.domain",
				},
				"health_checks": map[string]interface{}{
					"bind_address":     "0.0.0.0",
					"bind_port":        9982,
					"listener_enabled": true,
					"live_path":        "/live",
					"ready_path":       "/ready",
				},
				"plugins": map[string]interface{}{
					"KeyManager": []map[string]interface{}{
						{"memory": map[string]interface{}{"plugin_data": nil}},
					},
				},
				"telemetry": map[string]interface{}{
					"Prometheus": map[string]interface{}{
						"host": "0.0.0.0",
						"port": "9402",
					},
				},
			},
		},
		{
			name: "config with workload attestor disabled",
			cfg: &v1alpha1.SpireAgent{
				Spec: v1alpha1.SpireAgentSpec{
					TrustDomain: "workload-disabled.domain",
					WorkloadAttestors: &v1alpha1.WorkloadAttestors{
						K8sEnabled: "false",
					},
				},
			},
			expected: map[string]interface{}{
				"agent": map[string]interface{}{
					"data_dir":          "/var/lib/spire",
					"log_level":         "info",
					"log_format":        "text",
					"retry_bootstrap":   true,
					"server_address":    "spire-server.zero-trust-workload-identity-manager",
					"server_port":       "443",
					"socket_path":       "/tmp/spire-agent/public/spire-agent.sock",
					"trust_bundle_path": "/run/spire/bundle/bundle.crt",
					"trust_domain":      "workload-disabled.domain",
				},
				"health_checks": map[string]interface{}{
					"bind_address":     "0.0.0.0",
					"bind_port":        9982,
					"listener_enabled": true,
					"live_path":        "/live",
					"ready_path":       "/ready",
				},
				"plugins": map[string]interface{}{
					"KeyManager": []map[string]interface{}{
						{"memory": map[string]interface{}{"plugin_data": nil}},
					},
				},
				"telemetry": map[string]interface{}{
					"Prometheus": map[string]interface{}{
						"host": "0.0.0.0",
						"port": "9402",
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GenerateAgentConfig(tt.cfg)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGenerateSpireAgentConfigMap(t *testing.T) {
	// Mock the utils.OperatorNamespace for testing
	originalNamespace := utils.OperatorNamespace

	tests := []struct {
		name                       string
		spireAgentConfig           *v1alpha1.SpireAgent
		expectedConfigMapName      string
		expectedConfigMapNamespace string
		expectError                bool
		validateConfigData         bool
	}{
		{
			name: "successful configmap generation",
			spireAgentConfig: &v1alpha1.SpireAgent{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-agent-config",
					Namespace: originalNamespace,
				},
				Spec: v1alpha1.SpireAgentSpec{
					TrustDomain: "example.org",
				},
			},
			expectedConfigMapName:      "spire-agent",
			expectedConfigMapNamespace: originalNamespace,
			expectError:                false,
			validateConfigData:         true,
		},
		{
			name: "configmap with custom labels",
			spireAgentConfig: &v1alpha1.SpireAgent{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-agent-config",
					Namespace: originalNamespace,
				},
				Spec: v1alpha1.SpireAgentSpec{
					TrustDomain: "example.org",
					ClusterName: "test-cluster",
					NodeAttestor: &v1alpha1.NodeAttestor{
						K8sPSATEnabled: "true",
					},
					CommonConfig: v1alpha1.CommonConfig{
						Labels: map[string]string{
							"custom-label": "custom-value",
							"environment":  "test",
							"version":      "v1.0.0",
						},
					},
				},
			},
			expectedConfigMapName:      "spire-agent",
			expectedConfigMapNamespace: originalNamespace,
			expectError:                false,
			validateConfigData:         true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cm, hash, err := GenerateSpireAgentConfigMap(tt.spireAgentConfig)

			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, cm)
				assert.Empty(t, hash)
				return
			}

			require.NoError(t, err)
			require.NotNil(t, cm)
			assert.NotEmpty(t, hash)

			// Validate ConfigMap metadata
			assert.Equal(t, tt.expectedConfigMapName, cm.Name)
			assert.Equal(t, tt.expectedConfigMapNamespace, cm.Namespace)

			// Validate required labels
			expectedLabels := utils.SpireAgentLabels(nil)

			// Add custom labels from the SpireAgentConfig
			for key, value := range tt.spireAgentConfig.Spec.Labels {
				expectedLabels[key] = value
			}

			assert.Equal(t, expectedLabels, cm.Labels)

			// Validate annotations
			expectedAnnotations := map[string]string{
				utils.AppManagedByLabelKey: utils.AppManagedByLabelValue,
			}
			assert.Equal(t, expectedAnnotations, cm.Annotations)

			// Validate ConfigMap data
			assert.Contains(t, cm.Data, "agent.conf")
			assert.NotEmpty(t, cm.Data["agent.conf"])

			if tt.validateConfigData {
				// Validate that the config data is valid JSON
				var configData map[string]interface{}
				err := json.Unmarshal([]byte(cm.Data["agent.conf"]), &configData)
				require.NoError(t, err)

				// Validate basic structure
				assert.Contains(t, configData, "agent")
				assert.Contains(t, configData, "health_checks")
				assert.Contains(t, configData, "plugins")

				// Validate agent section
				agentSection := configData["agent"].(map[string]interface{})
				assert.Equal(t, tt.spireAgentConfig.Spec.TrustDomain, agentSection["trust_domain"])
				assert.Equal(t, "/var/lib/spire", agentSection["data_dir"])
				assert.Equal(t, "info", agentSection["log_level"])
				assert.Equal(t, "text", agentSection["log_format"])

				// Validate health checks section
				healthSection := configData["health_checks"].(map[string]interface{})
				assert.Equal(t, "0.0.0.0", healthSection["bind_address"])
				assert.Equal(t, float64(9982), healthSection["bind_port"])
				assert.Equal(t, true, healthSection["listener_enabled"])

				// Validate plugins section
				pluginsSection := configData["plugins"].(map[string]interface{})
				assert.Contains(t, pluginsSection, "KeyManager")

				// Test that hash is deterministic
				cm2, hash2, err2 := GenerateSpireAgentConfigMap(tt.spireAgentConfig)
				require.NoError(t, err2)
				assert.Equal(t, hash, hash2)
				assert.Equal(t, cm.Data["agent.conf"], cm2.Data["agent.conf"])
			}
		})
	}
}

func TestGenerateSpireAgentConfigMapConsistency(t *testing.T) {
	// Mock the utils.OperatorNamespace for testing
	originalNamespace := utils.OperatorNamespace

	spireAgentConfig := &v1alpha1.SpireAgent{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "consistency-test",
			Namespace: originalNamespace,
		},
		Spec: v1alpha1.SpireAgentSpec{
			TrustDomain: "consistency.test",
			ClusterName: "consistency-cluster",
			NodeAttestor: &v1alpha1.NodeAttestor{
				K8sPSATEnabled: "true",
			},
			WorkloadAttestors: &v1alpha1.WorkloadAttestors{
				K8sEnabled:                "true",
				DisableContainerSelectors: "true",
				UseNewContainerLocator:    "false",
			},
		},
	}

	// Generate the same config multiple times
	cm1, hash1, err1 := GenerateSpireAgentConfigMap(spireAgentConfig)
	require.NoError(t, err1)

	cm2, hash2, err2 := GenerateSpireAgentConfigMap(spireAgentConfig)
	require.NoError(t, err2)

	cm3, hash3, err3 := GenerateSpireAgentConfigMap(spireAgentConfig)
	require.NoError(t, err3)

	// All results should be identical
	assert.Equal(t, hash1, hash2)
	assert.Equal(t, hash2, hash3)
	assert.Equal(t, cm1.Data["agent.conf"], cm2.Data["agent.conf"])
	assert.Equal(t, cm2.Data["agent.conf"], cm3.Data["agent.conf"])
}

func TestGenerateAgentConfigNilChecks(t *testing.T) {
	tests := []struct {
		name string
		cfg  *v1alpha1.SpireAgent
	}{
		{
			name: "nil node attestor",
			cfg: &v1alpha1.SpireAgent{
				Spec: v1alpha1.SpireAgentSpec{
					TrustDomain:  "test.domain",
					NodeAttestor: nil,
				},
			},
		},
		{
			name: "nil workload attestors",
			cfg: &v1alpha1.SpireAgent{
				Spec: v1alpha1.SpireAgentSpec{
					TrustDomain:       "test.domain",
					WorkloadAttestors: nil,
				},
			},
		},
		{
			name: "both nil",
			cfg: &v1alpha1.SpireAgent{
				Spec: v1alpha1.SpireAgentSpec{
					TrustDomain:       "test.domain",
					NodeAttestor:      nil,
					WorkloadAttestors: nil,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Should not panic
			result := GenerateAgentConfig(tt.cfg)

			// Basic validation
			assert.Contains(t, result, "agent")
			assert.Contains(t, result, "health_checks")
			assert.Contains(t, result, "plugins")

			// Should have KeyManager but not NodeAttestor or WorkloadAttestor
			plugins := result["plugins"].(map[string]interface{})
			assert.Contains(t, plugins, "KeyManager")
			assert.NotContains(t, plugins, "NodeAttestor")
			assert.NotContains(t, plugins, "WorkloadAttestor")
		})
	}
}

func TestGenerateSpireAgentConfigMapEmptyLabels(t *testing.T) {
	// Mock the utils.OperatorNamespace for testing
	originalNamespace := utils.OperatorNamespace

	spireAgentConfig := &v1alpha1.SpireAgent{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "empty-labels-test",
			Namespace: originalNamespace,
			Labels:    nil, // Explicitly nil labels
		},
		Spec: v1alpha1.SpireAgentSpec{
			TrustDomain: "empty.labels",
		},
	}

	cm, hash, err := GenerateSpireAgentConfigMap(spireAgentConfig)
	require.NoError(t, err)
	require.NotNil(t, cm)
	assert.NotEmpty(t, hash)

	// Should only have the required labels
	expectedLabels := utils.SpireAgentLabels(nil)
	assert.Equal(t, expectedLabels, cm.Labels)
}
