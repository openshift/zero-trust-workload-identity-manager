package spire_server

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/yaml"

	"github.com/openshift/zero-trust-workload-identity-manager/api/v1alpha1"
	"github.com/openshift/zero-trust-workload-identity-manager/pkg/controller/status"
	"github.com/openshift/zero-trust-workload-identity-manager/pkg/controller/utils"
	spiffev1alpha "github.com/spiffe/spire-controller-manager/api/v1alpha1"
)

const defaultCaKeyType = "rsa-2048"

type ControllerManagerConfigYAML struct {
	Kind                                  string            `json:"kind"`
	APIVersion                            string            `json:"apiVersion"`
	Metadata                              metav1.ObjectMeta `json:"metadata"`
	spiffev1alpha.ControllerManagerConfig `json:",inline"`
}

// reconcileSpireServerConfigMap reconciles the Spire Server ConfigMap
func (r *SpireServerReconciler) reconcileSpireServerConfigMap(ctx context.Context, server *v1alpha1.SpireServer, statusMgr *status.Manager, createOnlyMode bool) (string, error) {
	spireServerConfigMap, err := generateSpireServerConfigMap(&server.Spec)
	if err != nil {
		r.log.Error(err, "failed to generate spire server config map")
		statusMgr.AddCondition(ServerConfigMapAvailable, "SpireServerConfigMapGenerationFailed",
			err.Error(),
			metav1.ConditionFalse)
		return "", err
	}

	if err = controllerutil.SetControllerReference(server, spireServerConfigMap, r.scheme); err != nil {
		r.log.Error(err, "failed to set controller reference")
		statusMgr.AddCondition(ServerConfigMapAvailable, "SpireServerConfigMapGenerationFailed",
			err.Error(),
			metav1.ConditionFalse)
		return "", err
	}

	var existingSpireServerCM corev1.ConfigMap
	err = r.ctrlClient.Get(ctx, types.NamespacedName{Name: spireServerConfigMap.Name, Namespace: spireServerConfigMap.Namespace}, &existingSpireServerCM)
	if err != nil && kerrors.IsNotFound(err) {
		if err = r.ctrlClient.Create(ctx, spireServerConfigMap); err != nil {
			statusMgr.AddCondition(ServerConfigMapAvailable, "SpireServerConfigMapGenerationFailed",
				err.Error(),
				metav1.ConditionFalse)
			return "", fmt.Errorf("failed to create ConfigMap: %w", err)
		}
		r.log.Info("Created spire server ConfigMap")
	} else if err == nil && (existingSpireServerCM.Data["server.conf"] != spireServerConfigMap.Data["server.conf"] ||
		!equality.Semantic.DeepEqual(existingSpireServerCM.Labels, spireServerConfigMap.Labels)) {
		if createOnlyMode {
			r.log.Info("Skipping ConfigMap update due to create-only mode")
		} else {
			spireServerConfigMap.ResourceVersion = existingSpireServerCM.ResourceVersion
			if err = r.ctrlClient.Update(ctx, spireServerConfigMap); err != nil {
				statusMgr.AddCondition(ServerConfigMapAvailable, "SpireServerConfigMapGenerationFailed",
					err.Error(),
					metav1.ConditionFalse)
				return "", fmt.Errorf("failed to update ConfigMap: %w", err)
			}
			r.log.Info("Updated ConfigMap with new config")
		}
	} else if err != nil {
		statusMgr.AddCondition(ServerConfigMapAvailable, "SpireServerConfigMapGenerationFailed",
			err.Error(),
			metav1.ConditionFalse)
		return "", err
	}

	statusMgr.AddCondition(ServerConfigMapAvailable, "SpireConfigMapResourceCreated",
		"SpireServer config map resources applied",
		metav1.ConditionTrue)

	// Generate config hash
	spireServerConfJSON, err := marshalToJSON(generateServerConfMap(&server.Spec))
	if err != nil {
		r.log.Error(err, "failed to marshal spire server config map to JSON")
		return "", err
	}

	return generateConfigHash(spireServerConfJSON), nil
}

// reconcileSpireControllerManagerConfigMap reconciles the Spire Controller Manager ConfigMap
func (r *SpireServerReconciler) reconcileSpireControllerManagerConfigMap(ctx context.Context, server *v1alpha1.SpireServer, statusMgr *status.Manager, createOnlyMode bool) (string, error) {
	spireControllerManagerConfig, err := generateSpireControllerManagerConfigYaml(&server.Spec)
	if err != nil {
		r.log.Error(err, "Failed to generate spire controller manager config")
		statusMgr.AddCondition(ControllerManagerConfigAvailable, "SpireControllerManagerConfigMapGenerationFailed",
			err.Error(),
			metav1.ConditionFalse)
		return "", err
	}

	spireControllerManagerConfigMap := generateControllerManagerConfigMap(spireControllerManagerConfig)
	if err = controllerutil.SetControllerReference(server, spireControllerManagerConfigMap, r.scheme); err != nil {
		r.log.Error(err, "failed to set controller reference on spire controller manager config")
		statusMgr.AddCondition(ControllerManagerConfigAvailable, "SpireControllerManagerConfigMapGenerationFailed",
			err.Error(),
			metav1.ConditionFalse)
		return "", err
	}

	var existingSpireControllerManagerCM corev1.ConfigMap
	err = r.ctrlClient.Get(ctx, types.NamespacedName{Name: spireControllerManagerConfigMap.Name, Namespace: spireControllerManagerConfigMap.Namespace}, &existingSpireControllerManagerCM)
	if err != nil && kerrors.IsNotFound(err) {
		if err = r.ctrlClient.Create(ctx, spireControllerManagerConfigMap); err != nil {
			r.log.Error(err, "failed to create spire controller manager config map")
			statusMgr.AddCondition(ControllerManagerConfigAvailable, "SpireControllerManagerConfigMapGenerationFailed",
				err.Error(),
				metav1.ConditionFalse)
			return "", fmt.Errorf("failed to create ConfigMap: %w", err)
		}
		r.log.Info("Created spire controller manager ConfigMap")
	} else if err == nil && (existingSpireControllerManagerCM.Data["controller-manager-config.yaml"] != spireControllerManagerConfigMap.Data["controller-manager-config.yaml"] ||
		!equality.Semantic.DeepEqual(existingSpireControllerManagerCM.Labels, spireControllerManagerConfigMap.Labels)) {
		if createOnlyMode {
			r.log.Info("Skipping spire controller manager ConfigMap update due to create-only mode")
		} else {
			spireControllerManagerConfigMap.ResourceVersion = existingSpireControllerManagerCM.ResourceVersion
			if err = r.ctrlClient.Update(ctx, spireControllerManagerConfigMap); err != nil {
				statusMgr.AddCondition(ControllerManagerConfigAvailable, "SpireControllerManagerConfigMapGenerationFailed",
					err.Error(),
					metav1.ConditionFalse)
				return "", fmt.Errorf("failed to update ConfigMap: %w", err)
			}
		}
		r.log.Info("Updated ConfigMap with new config")
	} else if err != nil {
		r.log.Error(err, "failed to update spire controller manager config map")
		return "", err
	}

	statusMgr.AddCondition(ControllerManagerConfigAvailable, "SpireControllerManagerConfigMapCreated",
		"spire controller manager config map resources applied",
		metav1.ConditionTrue)

	return generateConfigHashFromString(spireControllerManagerConfig), nil
}

// reconcileSpireBundleConfigMap reconciles the Spire Bundle ConfigMap
func (r *SpireServerReconciler) reconcileSpireBundleConfigMap(ctx context.Context, server *v1alpha1.SpireServer, statusMgr *status.Manager) error {
	spireBundleCM, err := generateSpireBundleConfigMap(&server.Spec)
	if err != nil {
		r.log.Error(err, "failed to generate spire bundle config map")
		statusMgr.AddCondition(BundleConfigAvailable, "SpireBundleConfigMapGenerationFailed",
			err.Error(),
			metav1.ConditionFalse)
		return err
	}

	if err := controllerutil.SetControllerReference(server, spireBundleCM, r.scheme); err != nil {
		r.log.Error(err, "failed to set controller reference on spire bundle config")
		statusMgr.AddCondition(BundleConfigAvailable, "SpireBundleConfigMapGenerationFailed",
			err.Error(),
			metav1.ConditionFalse)
		return err
	}

	err = r.ctrlClient.Create(ctx, spireBundleCM)
	if err != nil && !kerrors.IsAlreadyExists(err) {
		r.log.Error(err, "failed to create spire bundle config map")
		statusMgr.AddCondition(BundleConfigAvailable, "SpireBundleConfigMapGenerationFailed",
			err.Error(),
			metav1.ConditionFalse)
		return fmt.Errorf("failed to create spire-bundle ConfigMap: %w", err)
	}

	statusMgr.AddCondition(BundleConfigAvailable, "SpireBundleConfigMapCreated",
		"spire bundle config map resources applied",
		metav1.ConditionTrue)
	return nil
}

// generateSpireServerConfigMap generates the spire-server ConfigMap
func generateSpireServerConfigMap(config *v1alpha1.SpireServerSpec) (*corev1.ConfigMap, error) {
	if config == nil {
		return nil, fmt.Errorf("config is nil")
	}
	if config.TrustDomain == "" {
		return nil, fmt.Errorf("trust_domain is empty")
	}
	if config.BundleConfigMap == "" {
		return nil, fmt.Errorf("bundle configmap is empty")
	}
	if config.Datastore == nil {
		return nil, fmt.Errorf("datastore configuration is required")
	}
	if config.CASubject == nil {
		return nil, fmt.Errorf("CASubject is empty")
	}
	confMap := generateServerConfMap(config)
	confJSON, err := marshalToJSON(confMap)
	if err != nil {
		return nil, err
	}

	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "spire-server",
			Namespace: utils.GetOperatorNamespace(),
			Labels:    utils.SpireServerLabels(config.Labels),
		},
		Data: map[string]string{
			"server.conf": string(confJSON),
		},
	}

	return cm, nil
}

// generateServerConfMap builds the server.conf structure as a Go map
func generateServerConfMap(config *v1alpha1.SpireServerSpec) map[string]interface{} {
	// Build the server config
	serverConfig := map[string]interface{}{
		"audit_log_enabled": false,
		"bind_address":      "0.0.0.0",
		"bind_port":         "8081",
		"ca_key_type":       getCAKeyType(config.CAKeyType),
		"ca_subject": []map[string]interface{}{
			{
				"common_name":  config.CASubject.CommonName,
				"country":      []string{config.CASubject.Country},
				"organization": []string{config.CASubject.Organization},
			},
		},
		"ca_ttl":                config.CAValidity,
		"data_dir":              "/run/spire/data",
		"default_jwt_svid_ttl":  config.DefaultJWTValidity,
		"default_x509_svid_ttl": config.DefaultX509Validity,
		"jwt_issuer":            config.JwtIssuer,
		"log_level":             utils.GetLogLevelFromString(config.LogLevel),
		"log_format":            utils.GetLogFormatFromString(config.LogFormat),
		"trust_domain":          config.TrustDomain,
	}

	// Only add jwt_key_type if it's explicitly set
	if config.JWTKeyType != "" {
		serverConfig["jwt_key_type"] = config.JWTKeyType
	}

	return map[string]interface{}{
		"health_checks": map[string]interface{}{
			"bind_address":     "0.0.0.0",
			"bind_port":        "8080",
			"listener_enabled": true,
			"live_path":        "/live",
			"ready_path":       "/ready",
		},
		"plugins": map[string]interface{}{
			"DataStore": []map[string]interface{}{
				{
					"sql": map[string]interface{}{
						"plugin_data": map[string]interface{}{
							"connection_string": config.Datastore.ConnectionString,
							"database_type":     config.Datastore.DatabaseType,
							"disable_migration": utils.StringToBool(config.Datastore.DisableMigration),
							"max_idle_conns":    config.Datastore.MaxIdleConns,
							"max_open_conns":    config.Datastore.MaxOpenConns,
						},
					},
				},
			},
			"KeyManager": []map[string]interface{}{
				{
					"disk": map[string]interface{}{
						"plugin_data": map[string]interface{}{
							"keys_path": "/run/spire/data/keys.json",
						},
					},
				},
			},
			"NodeAttestor": []map[string]interface{}{
				{
					"k8s_psat": map[string]interface{}{
						"plugin_data": map[string]interface{}{
							"clusters": []map[string]interface{}{
								{
									config.ClusterName: map[string]interface{}{
										"allowed_node_label_keys": []string{},
										"allowed_pod_label_keys":  []string{},
										"audience":                []string{"spire-server"},
										"service_account_allow_list": []string{
											fmt.Sprintf("%s:spire-agent", utils.GetOperatorNamespace()),
										},
									},
								},
							},
						},
					},
				},
			},
			"Notifier": []map[string]interface{}{
				{
					"k8sbundle": map[string]interface{}{
						"plugin_data": map[string]interface{}{
							"config_map": config.BundleConfigMap,
							"namespace":  utils.GetOperatorNamespace(),
						},
					},
				},
			},
		},
		"server": serverConfig,
		"telemetry": map[string]interface{}{
			"Prometheus": map[string]interface{}{
				"host": "0.0.0.0",
				"port": "9402",
			},
		},
	}
}

// marshalToJSON marshals a map to JSON with indentation
func marshalToJSON(data map[string]interface{}) ([]byte, error) {
	jsonBytes, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal server.conf: %w", err)
	}
	return jsonBytes, nil
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

// getCAKeyType returns the CA key type from config, defaulting to "rsa-2048" if not set
func getCAKeyType(keyType string) string {
	if keyType == "" {
		return defaultCaKeyType
	}
	return keyType
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
			Namespace: utils.GetOperatorNamespace(),
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
			Namespace: utils.GetOperatorNamespace(),
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
			Namespace: utils.GetOperatorNamespace(),
			Labels:    utils.SpireServerLabels(config.Labels),
		},
	}, nil
}
