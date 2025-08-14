package spire_agent

import (
	"encoding/json"
	"fmt"
	"github.com/openshift/zero-trust-workload-identity-manager/api/v1alpha1"
	"github.com/openshift/zero-trust-workload-identity-manager/pkg/controller/utils"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func GenerateAgentConfig(cfg *v1alpha1.SpireAgent) map[string]interface{} {
	agentConf := map[string]interface{}{
		"agent": map[string]interface{}{
			"data_dir":          "/var/lib/spire",
			"log_level":         "debug",
			"retry_bootstrap":   true,
			"server_address":    "spire-server.zero-trust-workload-identity-manager",
			"server_port":       "443",
			"socket_path":       "/tmp/spire-agent/public/spire-agent.sock",
			"trust_bundle_path": "/run/spire/bundle/bundle.crt",
			"trust_domain":      cfg.Spec.TrustDomain,
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
	}

	if cfg.Spec.NodeAttestor != nil && cfg.Spec.NodeAttestor.K8sPSATEnabled == "true" {
		agentConf["plugins"].(map[string]interface{})["NodeAttestor"] = []map[string]interface{}{
			{
				"k8s_psat": map[string]interface{}{
					"plugin_data": map[string]interface{}{
						"cluster": cfg.Spec.ClusterName,
					},
				},
			},
		}
	}

	if cfg.Spec.WorkloadAttestors != nil && cfg.Spec.WorkloadAttestors.K8sEnabled == "true" {
		plugin := map[string]interface{}{
			"disable_container_selectors":    utils.StringToBool(cfg.Spec.WorkloadAttestors.DisableContainerSelectors),
			"node_name_env":                  "MY_NODE_NAME",
			"use_new_container_locator":      utils.StringToBool(cfg.Spec.WorkloadAttestors.UseNewContainerLocator),
			"verbose_container_locator_logs": false,
			"skip_kubelet_verification":      true,
		}

		agentConf["plugins"].(map[string]interface{})["WorkloadAttestor"] = []map[string]interface{}{
			{"k8s": map[string]interface{}{"plugin_data": plugin}},
		}
	}

	return agentConf
}

func GenerateSpireAgentConfigMap(spireAgentConfig *v1alpha1.SpireAgent) (*corev1.ConfigMap, string, error) {
	agentConfig := GenerateAgentConfig(spireAgentConfig)
	agentConfigJSON, err := json.MarshalIndent(agentConfig, "", "  ")
	if err != nil {
		return nil, "", fmt.Errorf("failed to marshal agent config: %w", err)
	}

	spireAgentConfigHash := utils.GenerateConfigHash(agentConfigJSON)

	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "spire-agent",
			Namespace: utils.OperatorNamespace,
			Labels:    utils.SpireAgentLabels(spireAgentConfig.Spec.Labels),
			Annotations: map[string]string{
				utils.AppManagedByLabelKey: utils.AppManagedByLabelValue,
			},
		},
		Data: map[string]string{
			"agent.conf": string(agentConfigJSON),
		},
	}

	return cm, spireAgentConfigHash, nil
}
