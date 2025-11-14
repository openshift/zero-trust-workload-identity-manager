package spire_agent

import (
	"context"
	"encoding/json"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/openshift/zero-trust-workload-identity-manager/api/v1alpha1"
	"github.com/openshift/zero-trust-workload-identity-manager/pkg/controller/status"
	"github.com/openshift/zero-trust-workload-identity-manager/pkg/controller/utils"
)

// reconcileConfigMap reconciles the Spire Agent ConfigMap
func (r *SpireAgentReconciler) reconcileConfigMap(ctx context.Context, agent *v1alpha1.SpireAgent, statusMgr *status.Manager, createOnlyMode bool) (string, error) {
	spireAgentConfigMap, spireAgentConfigHash, err := generateSpireAgentConfigMap(agent)
	if err != nil {
		r.log.Error(err, "failed to generate spire-agent config map")
		statusMgr.AddCondition(ConfigMapAvailable, "SpireAgentConfigMapGenerationFailed",
			err.Error(),
			metav1.ConditionFalse)
		return "", err
	}

	if err = controllerutil.SetControllerReference(agent, spireAgentConfigMap, r.scheme); err != nil {
		r.log.Error(err, "failed to set controller reference")
		statusMgr.AddCondition(ConfigMapAvailable, "SpireAgentConfigMapGenerationFailed",
			err.Error(),
			metav1.ConditionFalse)
		return "", err
	}

	var existingSpireAgentCM corev1.ConfigMap
	err = r.ctrlClient.Get(ctx, types.NamespacedName{Name: spireAgentConfigMap.Name, Namespace: spireAgentConfigMap.Namespace}, &existingSpireAgentCM)
	if err != nil && kerrors.IsNotFound(err) {
		if err = r.ctrlClient.Create(ctx, spireAgentConfigMap); err != nil {
			r.log.Error(err, "failed to create spire-agent config map")
			statusMgr.AddCondition(ConfigMapAvailable, "SpireAgentConfigMapGenerationFailed",
				err.Error(),
				metav1.ConditionFalse)
			return "", fmt.Errorf("failed to create ConfigMap: %w", err)
		}
		r.log.Info("Created spire agent ConfigMap")
	} else if err == nil && (existingSpireAgentCM.Data["agent.conf"] != spireAgentConfigMap.Data["agent.conf"] ||
		!equality.Semantic.DeepEqual(existingSpireAgentCM.Labels, spireAgentConfigMap.Labels)) {
		if createOnlyMode {
			r.log.Info("Skipping ConfigMap update due to create-only mode")
		} else {
			spireAgentConfigMap.ResourceVersion = existingSpireAgentCM.ResourceVersion
			if err = r.ctrlClient.Update(ctx, spireAgentConfigMap); err != nil {
				r.log.Error(err, "failed to update spire-agent config map")
				statusMgr.AddCondition(ConfigMapAvailable, "SpireAgentConfigMapGenerationFailed",
					err.Error(),
					metav1.ConditionFalse)
				return "", fmt.Errorf("failed to update ConfigMap: %w", err)
			}
			r.log.Info("Updated ConfigMap with new config")
		}
	} else if err != nil {
		statusMgr.AddCondition(ConfigMapAvailable, "SpireAgentConfigMapGenerationFailed",
			err.Error(),
			metav1.ConditionFalse)
		return "", err
	}

	statusMgr.AddCondition(ConfigMapAvailable, "SpireAgentConfigMapResourceCreated",
		"Spire Agent ConfigMap resources applied",
		metav1.ConditionTrue)

	return spireAgentConfigHash, nil
}

func generateAgentConfig(cfg *v1alpha1.SpireAgent) map[string]interface{} {
	agentConf := map[string]interface{}{
		"agent": map[string]interface{}{
			"data_dir":          "/var/lib/spire",
			"log_level":         utils.GetLogLevelFromString(cfg.Spec.LogLevel),
			"log_format":        utils.GetLogFormatFromString(cfg.Spec.LogFormat),
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

func generateSpireAgentConfigMap(spireAgentConfig *v1alpha1.SpireAgent) (*corev1.ConfigMap, string, error) {
	agentConfig := generateAgentConfig(spireAgentConfig)
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
