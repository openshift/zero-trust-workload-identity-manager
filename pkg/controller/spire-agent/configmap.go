package spire_agent

import (
	"encoding/json"
	"fmt"
	"log"

	"github.com/openshift/zero-trust-workload-identity-manager/api/v1alpha1"
	"github.com/openshift/zero-trust-workload-identity-manager/pkg/config"
	"github.com/openshift/zero-trust-workload-identity-manager/pkg/controller/utils"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// buildSpireAgentConfig creates a SpireAgentConfig from the operator API spec
func buildSpireAgentConfig(spec *v1alpha1.SpireAgentSpec) (*config.SpireAgentConfig, error) {
	agentConfig := &config.SpireAgentConfig{
		Agent: config.AgentConfig{
			DataDir:         DefaultAgentDataDir,
			LogLevel:        utils.GetLogLevelFromString(spec.LogLevel),
			LogFormat:       utils.GetLogFormatFromString(spec.LogFormat),
			RetryBootstrap:  true,
			ServerAddress:   DefaultAgentServerAddress,
			ServerPort:      DefaultAgentServerPort,
			SocketPath:      DefaultAgentSocketPath,
			TrustBundlePath: DefaultAgentTrustBundlePath,
			TrustDomain:     spec.TrustDomain,
		},
		HealthChecks: config.HealthChecks{
			BindAddress:     DefaultHealthCheckBindAddress,
			BindPort:        DefaultHealthCheckBindPort,
			ListenerEnabled: true,
			LivePath:        DefaultHealthCheckLivePath,
			ReadyPath:       DefaultHealthCheckReadyPath,
		},
		Telemetry: &config.TelemetryConfig{
			Prometheus: &config.PrometheusConfig{
				Host: DefaultPrometheusHost,
				Port: DefaultPrometheusPort,
			},
		},
	}

	// Build KeyManager plugin (memory for agent)
	keyManagerPlugin := config.PluginConfig{
		"memory": config.PluginData{
			PluginData: nil,
		},
	}

	agentConfig.Plugins = config.AgentPlugins{
		KeyManager: []config.PluginConfig{keyManagerPlugin},
	}

	// Add NodeAttestor plugin if k8s PSAT is enabled
	if spec.NodeAttestor != nil && utils.StringToBool(spec.NodeAttestor.K8sPSATEnabled) {
		// Validate ClusterName is non-empty before constructing the plugin
		if spec.ClusterName == "" {
			return nil, fmt.Errorf("clusterName is required when k8s_psat node attestor is enabled")
		}

		nodeAttestorPluginData := config.AgentNodeAttestorPluginData{
			Cluster: spec.ClusterName,
		}

		nodeAttestorPlugin := config.PluginConfig{
			"k8s_psat": config.PluginData{
				PluginData: nodeAttestorPluginData,
			},
		}
		agentConfig.Plugins.NodeAttestor = []config.PluginConfig{nodeAttestorPlugin}
	}

	// Add WorkloadAttestor plugin if k8s is enabled
	if spec.WorkloadAttestors != nil && utils.StringToBool(spec.WorkloadAttestors.K8sEnabled) {
		workloadAttestorPluginData := config.WorkloadAttestorPluginData{
			DisableContainerSelectors:   utils.StringToBool(spec.WorkloadAttestors.DisableContainerSelectors),
			NodeNameEnv:                 DefaultNodeNameEnv,
			UseNewContainerLocator:      utils.StringToBool(spec.WorkloadAttestors.UseNewContainerLocator),
			VerboseContainerLocatorLogs: false,
			SkipKubeletVerification:     true,
		}

		// Add workload attestor verification settings if provided
		if spec.WorkloadAttestors.WorkloadAttestorsVerification != nil {
			verification := spec.WorkloadAttestors.WorkloadAttestorsVerification

		switch verification.Type {
		case "hostCert":
			// Validate HostCertBasePath is non-empty when hostCert verification is enabled
			if verification.HostCertBasePath == "" {
				return nil, fmt.Errorf("hostCertBasePath is required when workload attestor verification type is 'hostCert'")
			}
			workloadAttestorPluginData.SkipKubeletVerification = false
			workloadAttestorPluginData.VerifyKubeletCertificate = true
			workloadAttestorPluginData.KubeletCAPath = verification.HostCertBasePath
			case "apiServerCA":
				workloadAttestorPluginData.SkipKubeletVerification = false
				workloadAttestorPluginData.VerifyKubeletCertificate = true
			case "skip":
				workloadAttestorPluginData.SkipKubeletVerification = true
			case "auto":
				// Let SPIRE decide
				workloadAttestorPluginData.SkipKubeletVerification = false
			}
		}

		workloadAttestorPlugin := config.PluginConfig{
			"k8s": config.PluginData{
				PluginData: workloadAttestorPluginData,
			},
		}
		agentConfig.Plugins.WorkloadAttestor = []config.PluginConfig{workloadAttestorPlugin}
	}

	return agentConfig, nil
}

// GenerateAgentConfig creates a structured agent configuration from the operator API spec (deprecated)
// Use buildSpireAgentConfig instead
func GenerateAgentConfig(cfg *v1alpha1.SpireAgent) map[string]interface{} {
	// Build using the new config struct approach
	agentConfig, err := buildSpireAgentConfig(&cfg.Spec)
	if err != nil {
		log.Printf("Error building agent config: %v", err)
		return make(map[string]interface{})
	}

	// Convert struct to map for backward compatibility
	jsonBytes, marshalErr := json.Marshal(agentConfig)
	if marshalErr != nil {
		log.Printf("Error marshaling agent config: %v", marshalErr)
		return make(map[string]interface{})
	}

	var result map[string]interface{}
	unmarshalErr := json.Unmarshal(jsonBytes, &result)
	if unmarshalErr != nil {
		log.Printf("Error unmarshaling agent config: %v", unmarshalErr)
		return make(map[string]interface{})
	}

	return result
}

func GenerateSpireAgentConfigMap(spireAgentConfig *v1alpha1.SpireAgent) (*corev1.ConfigMap, string, error) {
	// Build config struct from operator API spec
	agentConfig, err := buildSpireAgentConfig(&spireAgentConfig.Spec)
	if err != nil {
		return nil, "", fmt.Errorf("failed to build agent config: %w", err)
	}

	// Marshal to JSON
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
