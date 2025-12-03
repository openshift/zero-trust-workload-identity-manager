package spire_agent

import (
	"encoding/json"
	"fmt"
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
		ztwim    *v1alpha1.ZeroTrustWorkloadIdentityManager
		expected map[string]interface{}
	}{
		{
			name: "minimal config",
			cfg: &v1alpha1.SpireAgent{
				Spec: v1alpha1.SpireAgentSpec{},
			},
			ztwim: &v1alpha1.ZeroTrustWorkloadIdentityManager{
				Spec: v1alpha1.ZeroTrustWorkloadIdentityManagerSpec{
					TrustDomain:     "example.org",
					BundleConfigMap: "spire-bundle",
				},
			},
			expected: map[string]interface{}{
				"agent": map[string]interface{}{
					"data_dir":          "/var/lib/spire",
					"log_level":         "info",
					"log_format":        "text",
					"retry_bootstrap":   true,
					"server_address":    fmt.Sprintf("spire-server.%s", utils.GetOperatorNamespace()),
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
					NodeAttestor: &v1alpha1.NodeAttestor{
						K8sPSATEnabled: "true",
					},
				},
			},
			ztwim: &v1alpha1.ZeroTrustWorkloadIdentityManager{
				Spec: v1alpha1.ZeroTrustWorkloadIdentityManagerSpec{
					TrustDomain:     "test.domain",
					ClusterName:     "test-cluster",
					BundleConfigMap: "spire-bundle",
				},
			},
			expected: map[string]interface{}{
				"agent": map[string]interface{}{
					"data_dir":          "/var/lib/spire",
					"log_level":         "info",
					"log_format":        "text",
					"retry_bootstrap":   true,
					"server_address":    fmt.Sprintf("spire-server.%s", utils.GetOperatorNamespace()),
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
					WorkloadAttestors: &v1alpha1.WorkloadAttestors{
						K8sEnabled:                "true",
						DisableContainerSelectors: "true",
						UseNewContainerLocator:    "false",
					},
				},
			},
			ztwim: &v1alpha1.ZeroTrustWorkloadIdentityManager{
				Spec: v1alpha1.ZeroTrustWorkloadIdentityManagerSpec{
					TrustDomain:     "workload.domain",
					BundleConfigMap: "spire-bundle",
				},
			},
			expected: map[string]interface{}{
				"agent": map[string]interface{}{
					"data_dir":          "/var/lib/spire",
					"log_level":         "info",
					"log_format":        "text",
					"retry_bootstrap":   true,
					"server_address":    fmt.Sprintf("spire-server.%s", utils.GetOperatorNamespace()),
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
			ztwim: &v1alpha1.ZeroTrustWorkloadIdentityManager{
				Spec: v1alpha1.ZeroTrustWorkloadIdentityManagerSpec{
					TrustDomain:     "full.domain",
					ClusterName:     "full-cluster",
					BundleConfigMap: "spire-bundle",
				},
			},
			expected: map[string]interface{}{
				"agent": map[string]interface{}{
					"data_dir":          "/var/lib/spire",
					"log_level":         "info",
					"log_format":        "text",
					"retry_bootstrap":   true,
					"server_address":    fmt.Sprintf("spire-server.%s", utils.GetOperatorNamespace()),
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
					NodeAttestor: &v1alpha1.NodeAttestor{
						K8sPSATEnabled: "false",
					},
				},
			},
			ztwim: &v1alpha1.ZeroTrustWorkloadIdentityManager{
				Spec: v1alpha1.ZeroTrustWorkloadIdentityManagerSpec{
					TrustDomain:     "disabled.domain",
					ClusterName:     "disabled-cluster",
					BundleConfigMap: "spire-bundle",
				},
			},
			expected: map[string]interface{}{
				"agent": map[string]interface{}{
					"data_dir":          "/var/lib/spire",
					"log_level":         "info",
					"log_format":        "text",
					"retry_bootstrap":   true,
					"server_address":    fmt.Sprintf("spire-server.%s", utils.GetOperatorNamespace()),
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
					WorkloadAttestors: &v1alpha1.WorkloadAttestors{
						K8sEnabled: "false",
					},
				},
			},
			ztwim: &v1alpha1.ZeroTrustWorkloadIdentityManager{
				Spec: v1alpha1.ZeroTrustWorkloadIdentityManagerSpec{
					TrustDomain:     "workload-disabled.domain",
					BundleConfigMap: "spire-bundle",
				},
			},
			expected: map[string]interface{}{
				"agent": map[string]interface{}{
					"data_dir":          "/var/lib/spire",
					"log_level":         "info",
					"log_format":        "text",
					"retry_bootstrap":   true,
					"server_address":    fmt.Sprintf("spire-server.%s", utils.GetOperatorNamespace()),
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
			result := generateAgentConfig(tt.cfg, tt.ztwim)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGenerateSpireAgentConfigMap(t *testing.T) {
	tests := []struct {
		name                       string
		spireAgentConfig           *v1alpha1.SpireAgent
		ztwim                      *v1alpha1.ZeroTrustWorkloadIdentityManager
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
					Namespace: utils.GetOperatorNamespace(),
				},
				Spec: v1alpha1.SpireAgentSpec{},
			},
			ztwim: &v1alpha1.ZeroTrustWorkloadIdentityManager{
				Spec: v1alpha1.ZeroTrustWorkloadIdentityManagerSpec{
					TrustDomain:     "example.org",
					BundleConfigMap: "spire-bundle",
				},
			},
			expectedConfigMapName:      "spire-agent",
			expectedConfigMapNamespace: utils.GetOperatorNamespace(),
			expectError:                false,
			validateConfigData:         true,
		},
		{
			name: "configmap with custom labels",
			spireAgentConfig: &v1alpha1.SpireAgent{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-agent-config",
					Namespace: utils.GetOperatorNamespace(),
				},
				Spec: v1alpha1.SpireAgentSpec{
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
			ztwim: &v1alpha1.ZeroTrustWorkloadIdentityManager{
				Spec: v1alpha1.ZeroTrustWorkloadIdentityManagerSpec{
					TrustDomain:     "example.org",
					ClusterName:     "test-cluster",
					BundleConfigMap: "spire-bundle",
				},
			},
			expectedConfigMapName:      "spire-agent",
			expectedConfigMapNamespace: utils.GetOperatorNamespace(),
			expectError:                false,
			validateConfigData:         true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cm, hash, err := generateSpireAgentConfigMap(tt.spireAgentConfig, tt.ztwim)

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
				assert.Equal(t, tt.ztwim.Spec.TrustDomain, agentSection["trust_domain"])
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
				cm2, hash2, err2 := generateSpireAgentConfigMap(tt.spireAgentConfig, tt.ztwim)
				require.NoError(t, err2)
				assert.Equal(t, hash, hash2)
				assert.Equal(t, cm.Data["agent.conf"], cm2.Data["agent.conf"])
			}
		})
	}
}

func TestGenerateSpireAgentConfigMapConsistency(t *testing.T) {
	spireAgentConfig := &v1alpha1.SpireAgent{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "consistency-test",
			Namespace: utils.GetOperatorNamespace(),
		},
		Spec: v1alpha1.SpireAgentSpec{
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

	ztwim := &v1alpha1.ZeroTrustWorkloadIdentityManager{
		Spec: v1alpha1.ZeroTrustWorkloadIdentityManagerSpec{
			TrustDomain:     "consistency.test",
			ClusterName:     "consistency-cluster",
			BundleConfigMap: "spire-bundle",
		},
	}

	// Generate the same config multiple times
	cm1, hash1, err1 := generateSpireAgentConfigMap(spireAgentConfig, ztwim)
	require.NoError(t, err1)

	cm2, hash2, err2 := generateSpireAgentConfigMap(spireAgentConfig, ztwim)
	require.NoError(t, err2)

	cm3, hash3, err3 := generateSpireAgentConfigMap(spireAgentConfig, ztwim)
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
					NodeAttestor: nil,
				},
			},
		},
		{
			name: "nil workload attestors",
			cfg: &v1alpha1.SpireAgent{
				Spec: v1alpha1.SpireAgentSpec{
					WorkloadAttestors: nil,
				},
			},
		},
		{
			name: "both nil",
			cfg: &v1alpha1.SpireAgent{
				Spec: v1alpha1.SpireAgentSpec{
					NodeAttestor:      nil,
					WorkloadAttestors: nil,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Should not panic
			ztwim := &v1alpha1.ZeroTrustWorkloadIdentityManager{
				Spec: v1alpha1.ZeroTrustWorkloadIdentityManagerSpec{
					TrustDomain:     "test.domain",
					ClusterName:     "test-cluster",
					BundleConfigMap: "spire-bundle",
				},
			}
			result := generateAgentConfig(tt.cfg, ztwim)

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
	spireAgentConfig := &v1alpha1.SpireAgent{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "empty-labels-test",
			Namespace: utils.GetOperatorNamespace(),
			Labels:    nil, // Explicitly nil labels
		},
		Spec: v1alpha1.SpireAgentSpec{},
	}

	ztwim := &v1alpha1.ZeroTrustWorkloadIdentityManager{
		Spec: v1alpha1.ZeroTrustWorkloadIdentityManagerSpec{
			TrustDomain:     "empty.labels",
			BundleConfigMap: "spire-bundle",
		},
	}

	cm, hash, err := generateSpireAgentConfigMap(spireAgentConfig, ztwim)
	require.NoError(t, err)
	require.NotNil(t, cm)
	assert.NotEmpty(t, hash)

	// Should only have the required labels
	expectedLabels := utils.SpireAgentLabels(nil)
	assert.Equal(t, expectedLabels, cm.Labels)
}

func TestConfigureKubeletVerification(t *testing.T) {
	tests := []struct {
		name         string
		verification *v1alpha1.WorkloadAttestorsVerification
		expected     map[string]interface{}
	}{
		{
			name:         "nil verification defaults to skip",
			verification: nil,
			expected: map[string]interface{}{
				"skip_kubelet_verification": true,
			},
		},
		{
			name:         "empty type defaults to skip",
			verification: &v1alpha1.WorkloadAttestorsVerification{},
			expected: map[string]interface{}{
				"skip_kubelet_verification": true,
			},
		},
		{
			name: "skip type",
			verification: &v1alpha1.WorkloadAttestorsVerification{
				Type: utils.WorkloadAttestorVerificationTypeSkip,
			},
			expected: map[string]interface{}{
				"skip_kubelet_verification": true,
			},
		},
		{
			name: "hostCert type with paths",
			verification: &v1alpha1.WorkloadAttestorsVerification{
				Type:             utils.WorkloadAttestorVerificationTypeHostCert,
				HostCertBasePath: "/etc/kubernetes",
				HostCertFileName: "kubelet-ca.crt",
			},
			expected: map[string]interface{}{
				"skip_kubelet_verification": false,
				"kubelet_ca_path":           "/etc/kubernetes/kubelet-ca.crt",
			},
		},
		{
			name: "hostCert type with trailing slash in basePath",
			verification: &v1alpha1.WorkloadAttestorsVerification{
				Type:             utils.WorkloadAttestorVerificationTypeHostCert,
				HostCertBasePath: "/etc/kubernetes/",
				HostCertFileName: "kubelet-ca.crt",
			},
			expected: map[string]interface{}{
				"skip_kubelet_verification": false,
				"kubelet_ca_path":           "/etc/kubernetes/kubelet-ca.crt",
			},
		},
		{
			name: "hostCert type without paths (CEL would block this, but test the fallback)",
			verification: &v1alpha1.WorkloadAttestorsVerification{
				Type: utils.WorkloadAttestorVerificationTypeHostCert,
			},
			expected: map[string]interface{}{
				"skip_kubelet_verification": false,
			},
		},
		{
			name: "auto type without paths (uses SPIRE default)",
			verification: &v1alpha1.WorkloadAttestorsVerification{
				Type: utils.WorkloadAttestorVerificationTypeAuto,
			},
			expected: map[string]interface{}{
				"skip_kubelet_verification": false,
			},
		},
		{
			name: "auto type with paths",
			verification: &v1alpha1.WorkloadAttestorsVerification{
				Type:             utils.WorkloadAttestorVerificationTypeAuto,
				HostCertBasePath: "/etc/kubernetes",
				HostCertFileName: "kubelet-ca.crt",
			},
			expected: map[string]interface{}{
				"skip_kubelet_verification": false,
				"kubelet_ca_path":           "/etc/kubernetes/kubelet-ca.crt",
			},
		},
		{
			name: "auto type with trailing slash in basePath",
			verification: &v1alpha1.WorkloadAttestorsVerification{
				Type:             utils.WorkloadAttestorVerificationTypeAuto,
				HostCertBasePath: "/etc/kubernetes/",
				HostCertFileName: "kubelet-ca.crt",
			},
			expected: map[string]interface{}{
				"skip_kubelet_verification": false,
				"kubelet_ca_path":           "/etc/kubernetes/kubelet-ca.crt",
			},
		},
		{
			name: "auto type with only basePath (incomplete)",
			verification: &v1alpha1.WorkloadAttestorsVerification{
				Type:             utils.WorkloadAttestorVerificationTypeAuto,
				HostCertBasePath: "/etc/kubernetes",
			},
			expected: map[string]interface{}{
				"skip_kubelet_verification": false,
			},
		},
		{
			name: "unknown type defaults to skip",
			verification: &v1alpha1.WorkloadAttestorsVerification{
				Type: "unknown",
			},
			expected: map[string]interface{}{
				"skip_kubelet_verification": true,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			plugin := make(map[string]interface{})
			configureKubeletVerification(plugin, tt.verification)

			for key, expectedValue := range tt.expected {
				assert.Equal(t, expectedValue, plugin[key], "key: %s", key)
			}

			// Ensure no extra keys
			assert.Equal(t, len(tt.expected), len(plugin))
		})
	}
}

func TestBuildHostCertPath(t *testing.T) {
	tests := []struct {
		name         string
		verification *v1alpha1.WorkloadAttestorsVerification
		expected     string
	}{
		{
			name:         "nil verification",
			verification: nil,
			expected:     "",
		},
		{
			name:         "empty verification",
			verification: &v1alpha1.WorkloadAttestorsVerification{},
			expected:     "",
		},
		{
			name: "only basePath",
			verification: &v1alpha1.WorkloadAttestorsVerification{
				HostCertBasePath: "/etc/kubernetes",
			},
			expected: "",
		},
		{
			name: "only fileName",
			verification: &v1alpha1.WorkloadAttestorsVerification{
				HostCertFileName: "kubelet-ca.crt",
			},
			expected: "",
		},
		{
			name: "both basePath and fileName",
			verification: &v1alpha1.WorkloadAttestorsVerification{
				HostCertBasePath: "/etc/kubernetes",
				HostCertFileName: "kubelet-ca.crt",
			},
			expected: "/etc/kubernetes/kubelet-ca.crt",
		},
		{
			name: "basePath with trailing slash normalizes correctly",
			verification: &v1alpha1.WorkloadAttestorsVerification{
				HostCertBasePath: "/etc/kubernetes/",
				HostCertFileName: "kubelet-ca.crt",
			},
			expected: "/etc/kubernetes/kubelet-ca.crt",
		},
		{
			name: "basePath with multiple trailing slashes normalizes correctly",
			verification: &v1alpha1.WorkloadAttestorsVerification{
				HostCertBasePath: "/etc/kubernetes///",
				HostCertFileName: "kubelet-ca.crt",
			},
			expected: "/etc/kubernetes/kubelet-ca.crt",
		},
		{
			name: "custom paths",
			verification: &v1alpha1.WorkloadAttestorsVerification{
				HostCertBasePath: "/custom/path",
				HostCertFileName: "custom-ca.pem",
			},
			expected: "/custom/path/custom-ca.pem",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := buildHostCertPath(tt.verification)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGenerateAgentConfigWithVerification(t *testing.T) {
	tests := []struct {
		name                     string
		cfg                      *v1alpha1.SpireAgent
		expectedSkipVerification bool
		expectedKubeletCAPath    string
		kubeletCAPathShouldBeSet bool
	}{
		{
			name: "workload attestor with skip verification",
			cfg: &v1alpha1.SpireAgent{
				Spec: v1alpha1.SpireAgentSpec{
					TrustDomain: "test.domain",
					WorkloadAttestors: &v1alpha1.WorkloadAttestors{
						K8sEnabled: "true",
						WorkloadAttestorsVerification: &v1alpha1.WorkloadAttestorsVerification{
							Type: utils.WorkloadAttestorVerificationTypeSkip,
						},
					},
				},
			},
			expectedSkipVerification: true,
			kubeletCAPathShouldBeSet: false,
		},
		{
			name: "workload attestor with hostCert verification",
			cfg: &v1alpha1.SpireAgent{
				Spec: v1alpha1.SpireAgentSpec{
					TrustDomain: "test.domain",
					WorkloadAttestors: &v1alpha1.WorkloadAttestors{
						K8sEnabled: "true",
						WorkloadAttestorsVerification: &v1alpha1.WorkloadAttestorsVerification{
							Type:             utils.WorkloadAttestorVerificationTypeHostCert,
							HostCertBasePath: "/etc/kubernetes",
							HostCertFileName: "kubelet-ca.crt",
						},
					},
				},
			},
			expectedSkipVerification: false,
			expectedKubeletCAPath:    "/etc/kubernetes/kubelet-ca.crt",
			kubeletCAPathShouldBeSet: true,
		},
		{
			name: "workload attestor with hostCert verification (trailing slash)",
			cfg: &v1alpha1.SpireAgent{
				Spec: v1alpha1.SpireAgentSpec{
					TrustDomain: "test.domain",
					WorkloadAttestors: &v1alpha1.WorkloadAttestors{
						K8sEnabled: "true",
						WorkloadAttestorsVerification: &v1alpha1.WorkloadAttestorsVerification{
							Type:             utils.WorkloadAttestorVerificationTypeHostCert,
							HostCertBasePath: "/etc/kubernetes/",
							HostCertFileName: "kubelet-ca.crt",
						},
					},
				},
			},
			expectedSkipVerification: false,
			expectedKubeletCAPath:    "/etc/kubernetes/kubelet-ca.crt",
			kubeletCAPathShouldBeSet: true,
		},
		{
			name: "workload attestor with auto verification (no paths)",
			cfg: &v1alpha1.SpireAgent{
				Spec: v1alpha1.SpireAgentSpec{
					TrustDomain: "test.domain",
					WorkloadAttestors: &v1alpha1.WorkloadAttestors{
						K8sEnabled: "true",
						WorkloadAttestorsVerification: &v1alpha1.WorkloadAttestorsVerification{
							Type: utils.WorkloadAttestorVerificationTypeAuto,
						},
					},
				},
			},
			expectedSkipVerification: false,
			kubeletCAPathShouldBeSet: false,
		},
		{
			name: "workload attestor with auto verification (with paths)",
			cfg: &v1alpha1.SpireAgent{
				Spec: v1alpha1.SpireAgentSpec{
					TrustDomain: "test.domain",
					WorkloadAttestors: &v1alpha1.WorkloadAttestors{
						K8sEnabled: "true",
						WorkloadAttestorsVerification: &v1alpha1.WorkloadAttestorsVerification{
							Type:             utils.WorkloadAttestorVerificationTypeAuto,
							HostCertBasePath: "/etc/kubernetes",
							HostCertFileName: "kubelet-ca.crt",
						},
					},
				},
			},
			expectedSkipVerification: false,
			expectedKubeletCAPath:    "/etc/kubernetes/kubelet-ca.crt",
			kubeletCAPathShouldBeSet: true,
		},
		{
			name: "workload attestor without verification config (defaults to skip)",
			cfg: &v1alpha1.SpireAgent{
				Spec: v1alpha1.SpireAgentSpec{
					TrustDomain: "test.domain",
					WorkloadAttestors: &v1alpha1.WorkloadAttestors{
						K8sEnabled: "true",
					},
				},
			},
			expectedSkipVerification: true,
			kubeletCAPathShouldBeSet: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := generateAgentConfig(tt.cfg)

			// Get the WorkloadAttestor plugin data
			plugins := result["plugins"].(map[string]interface{})
			workloadAttestors, ok := plugins["WorkloadAttestor"]
			require.True(t, ok, "WorkloadAttestor should be present")

			attestorList := workloadAttestors.([]map[string]interface{})
			require.Len(t, attestorList, 1)

			k8sAttestor := attestorList[0]["k8s"].(map[string]interface{})
			pluginData := k8sAttestor["plugin_data"].(map[string]interface{})

			// Check skip_kubelet_verification
			assert.Equal(t, tt.expectedSkipVerification, pluginData["skip_kubelet_verification"])

			// Check kubelet_ca_path
			if tt.kubeletCAPathShouldBeSet {
				assert.Equal(t, tt.expectedKubeletCAPath, pluginData["kubelet_ca_path"])
			} else {
				_, exists := pluginData["kubelet_ca_path"]
				assert.False(t, exists, "kubelet_ca_path should not be set")
			}
		})
	}
}
