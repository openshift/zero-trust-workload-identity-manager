package spire_server

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"sigs.k8s.io/yaml"

	"github.com/openshift/zero-trust-workload-identity-manager/api/v1alpha1"
	"github.com/openshift/zero-trust-workload-identity-manager/pkg/config"
	"github.com/openshift/zero-trust-workload-identity-manager/pkg/controller/utils"
	spiffev1alpha "github.com/spiffe/spire-controller-manager/api/v1alpha1"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// buildSpireServerConfig creates a SpireServerConfig from the operator API spec
func buildSpireServerConfig(spec *v1alpha1.SpireServerSpec) *config.SpireServerConfig {
	serverConfig := &config.SpireServerConfig{
		Server: config.ServerConfig{
			BindAddress:        DefaultServerBindAddress,
			BindPort:           DefaultServerBindPort,
			TrustDomain:        spec.TrustDomain,
			DataDir:            DefaultServerDataDir,
			LogLevel:           utils.GetLogLevelFromString(spec.LogLevel),
			LogFormat:          utils.GetLogFormatFromString(spec.LogFormat),
			CAKeyType:          DefaultCAKeyType,
			CATTL:              spec.CAValidity,
			DefaultX509SVIDTTL: spec.DefaultX509Validity,
			DefaultJWTSVIDTTL:  spec.DefaultJWTValidity,
			JWTIssuer:          spec.JwtIssuer,
			AuditLogEnabled:    false,
			CASubject: []config.CASubject{
				{
					Country:      []string{spec.CASubject.Country},
					Organization: []string{spec.CASubject.Organization},
					CommonName:   spec.CASubject.CommonName,
				},
			},
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

	// Build DataStore plugin configuration
	dataStorePluginData := config.DataStorePluginData{
		DatabaseType:     spec.Datastore.DatabaseType,
		ConnectionString: spec.Datastore.ConnectionString,
		MaxOpenConns:     spec.Datastore.MaxOpenConns,
		MaxIdleConns:     spec.Datastore.MaxIdleConns,
		DisableMigration: utils.StringToBool(spec.Datastore.DisableMigration),
	}

	// Add conn_max_lifetime with seconds unit if provided
	if spec.Datastore.ConnMaxLifetime > 0 {
		dataStorePluginData.ConnMaxLifetime = fmt.Sprintf("%ds", spec.Datastore.ConnMaxLifetime)
	}

	// Add TLS options if provided
	if spec.Datastore.RootCAPath != "" {
		dataStorePluginData.RootCAPath = spec.Datastore.RootCAPath
	}
	if spec.Datastore.ClientCertPath != "" {
		dataStorePluginData.ClientCertPath = spec.Datastore.ClientCertPath
	}
	if spec.Datastore.ClientKeyPath != "" {
		dataStorePluginData.ClientKeyPath = spec.Datastore.ClientKeyPath
	}
	if len(spec.Datastore.Options) > 0 {
		dataStorePluginData.Options = spec.Datastore.Options
	}

	dataStorePlugin := config.PluginConfig{
		"sql": config.PluginData{
			PluginData: dataStorePluginData,
		},
	}

	// Build KeyManager plugin configuration
	var keyManagerPlugin config.PluginConfig
	if spec.KeyManager != nil && utils.StringToBool(spec.KeyManager.MemoryEnabled) {
		// Use memory-based key manager
		keyManagerPlugin = config.PluginConfig{
			"memory": config.PluginData{
				PluginData: nil,
			},
		}
	} else {
		// Use disk-based key manager (default)
		keyManagerPlugin = config.PluginConfig{
			"disk": config.PluginData{
				PluginData: config.KeyManagerPluginData{
					KeysPath: DefaultKeyManagerKeysPath,
				},
			},
		}
	}

	// Build NodeAttestor plugin configuration
	clusterConfig := config.ClusterConfig{
		AllowedNodeLabelKeys:    []string{},
		AllowedPodLabelKeys:     []string{},
		Audience:                []string{DefaultNodeAttestorAudience},
		ServiceAccountAllowList: getNodeAttestorServiceAccountAllowList(),
	}

	nodeAttestorPlugin := config.PluginConfig{
		"k8s_psat": config.PluginData{
			PluginData: config.NodeAttestorPluginData{
				Clusters: map[string]config.ClusterConfig{
					spec.ClusterName: clusterConfig,
				},
			},
		},
	}

	// Build Notifier plugin configuration
	notifierPluginData := config.NotifierPluginData{
		Namespace: utils.OperatorNamespace,
		ConfigMap: spec.BundleConfigMap,
	}

	notifierPlugin := config.PluginConfig{
		"k8sbundle": config.PluginData{
			PluginData: notifierPluginData,
		},
	}

	// Assemble all plugins
	serverConfig.Plugins = config.ServerPlugins{
		DataStore:    []config.PluginConfig{dataStorePlugin},
		KeyManager:   []config.PluginConfig{keyManagerPlugin},
		NodeAttestor: []config.PluginConfig{nodeAttestorPlugin},
		Notifier:     []config.PluginConfig{notifierPlugin},
	}

	return serverConfig
}

type ControllerManagerConfigYAML struct {
	Kind                                  string            `json:"kind"`
	APIVersion                            string            `json:"apiVersion"`
	Metadata                              metav1.ObjectMeta `json:"metadata"`
	spiffev1alpha.ControllerManagerConfig `json:",inline"`
}

// GenerateSpireServerConfigMap generates the spire-server ConfigMap using config structs
func GenerateSpireServerConfigMap(spec *v1alpha1.SpireServerSpec) (*corev1.ConfigMap, error) {
	if spec == nil {
		return nil, fmt.Errorf("spec is nil")
	}
	if spec.TrustDomain == "" {
		return nil, fmt.Errorf("trust_domain is empty")
	}
	if spec.ClusterName == "" {
		return nil, fmt.Errorf("cluster name is empty")
	}
	if spec.BundleConfigMap == "" {
		return nil, fmt.Errorf("bundle configmap is empty")
	}
	if spec.Datastore == nil {
		return nil, fmt.Errorf("datastore configuration is required")
	}
	if spec.CASubject == nil {
		return nil, fmt.Errorf("CASubject is empty")
	}

	// Build config struct from operator API spec
	serverConfig := buildSpireServerConfig(spec)

	// Marshal to JSON
	confJSON, err := json.MarshalIndent(serverConfig, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal server config: %w", err)
	}

	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "spire-server",
			Namespace: utils.OperatorNamespace,
			Labels:    utils.SpireServerLabels(spec.Labels),
		},
		Data: map[string]string{
			"server.conf": string(confJSON),
		},
	}

	return cm, nil
}

// generateServerConfMap builds the server.conf structure as a Go map (deprecated - kept for backward compatibility)
// Use buildSpireServerConfig instead
func generateServerConfMap(spec *v1alpha1.SpireServerSpec) (map[string]interface{}, error) {
	// Build using the new config struct approach
	serverConfig := buildSpireServerConfig(spec)

	// Convert struct to map for backward compatibility
	jsonBytes, err := json.Marshal(serverConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal server config: %w", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(jsonBytes, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal to map: %w", err)
	}

	return result, nil
}

// generateConfigHash returns a SHA256 hex string of the trimmed input string
func generateConfigHashFromString(data string) string {
	normalized := strings.TrimSpace(data) // Removes leading/trailing whitespace and newlines
	return generateConfigHash([]byte(normalized))
}

// generateConfigHash returns a SHA256 hex string of the trimmed input bytes
func generateConfigHash(data []byte) string {
	normalized := strings.TrimSpace(string(data)) // Convert to string, trim, convert back to bytes
	hash := sha256.Sum256([]byte(normalized))
	return hex.EncodeToString(hash[:])
}

func generateControllerManagerConfig(config *v1alpha1.SpireServerSpec) (*ControllerManagerConfigYAML, error) {
	if config.TrustDomain == "" {
		return nil, errors.New("trust_domain is empty")
	}
	if config.ClusterName == "" {
		return nil, errors.New("cluster name is empty")
	}
	return &ControllerManagerConfigYAML{
		Kind:       "ControllerManagerConfig",
		APIVersion: "spire.spiffe.io/v1alpha1",
		Metadata: metav1.ObjectMeta{
			Name:      "spire-controller-manager",
			Namespace: utils.OperatorNamespace,
			Labels:    utils.SpireControllerManagerLabels(config.Labels),
		},
		ControllerManagerConfig: spiffev1alpha.ControllerManagerConfig{
			ClusterName: config.ClusterName,
			TrustDomain: config.TrustDomain,
			ControllerManagerConfigurationSpec: spiffev1alpha.ControllerManagerConfigurationSpec{
				Metrics: spiffev1alpha.ControllerMetrics{
					BindAddress: "0.0.0.0:8082",
				},
				Health: spiffev1alpha.ControllerHealth{
					HealthProbeBindAddress: "0.0.0.0:8083",
				},
				EntryIDPrefix:    config.ClusterName,
				WatchClassless:   false,
				ClassName:        "zero-trust-workload-identity-manager-spire",
				ParentIDTemplate: "spiffe://{{ .TrustDomain }}/spire/agent/k8s_psat/{{ .ClusterName }}/{{ .NodeMeta.UID }}",
				Reconcile: &spiffev1alpha.ReconcileConfig{
					ClusterSPIFFEIDs:             true,
					ClusterFederatedTrustDomains: true,
					ClusterStaticEntries:         true,
				},
			},
			ValidatingWebhookConfigurationName: "spire-controller-manager-webhook",
			SPIREServerSocketPath:              "/tmp/spire-server/private/api.sock",
			IgnoreNamespaces: []string{
				"kube-system",
				"kube-public",
				"local-path-storage",
				"openshift-*",
			},
		},
	}, nil
}

func generateSpireControllerManagerConfigYaml(config *v1alpha1.SpireServerSpec) (string, error) {
	controllerManagerConfig, err := generateControllerManagerConfig(config)
	if err != nil {
		return "", err
	}
	configData, err := yaml.Marshal(controllerManagerConfig)
	if err != nil {
		return "", err
	}
	return string(configData), nil
}

func generateControllerManagerConfigMap(configYAML string) *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "spire-controller-manager",
			Namespace: utils.OperatorNamespace,
			Labels:    utils.SpireControllerManagerLabels(nil),
		},
		Data: map[string]string{
			"controller-manager-config.yaml": configYAML,
		},
	}
}

func generateSpireBundleConfigMap(config *v1alpha1.SpireServerSpec) (*corev1.ConfigMap, error) {
	if config.BundleConfigMap == "" {
		return nil, errors.New("bundle ConfigMap is empty")
	}
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      config.BundleConfigMap,
			Namespace: utils.OperatorNamespace,
			Labels:    utils.SpireServerLabels(config.Labels),
		},
	}, nil
}
