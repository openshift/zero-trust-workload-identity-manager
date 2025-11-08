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
	"github.com/openshift/zero-trust-workload-identity-manager/pkg/controller/utils"
	spiffev1alpha "github.com/spiffe/spire-controller-manager/api/v1alpha1"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type ControllerManagerConfigYAML struct {
	Kind                                  string            `json:"kind"`
	APIVersion                            string            `json:"apiVersion"`
	Metadata                              metav1.ObjectMeta `json:"metadata"`
	spiffev1alpha.ControllerManagerConfig `json:",inline"`
}

// GenerateSpireServerConfigMap generates the spire-server ConfigMap
func GenerateSpireServerConfigMap(config *v1alpha1.SpireServerSpec) (*corev1.ConfigMap, error) {
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
			Namespace: utils.OperatorNamespace,
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
	confMap := map[string]interface{}{
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
											"zero-trust-workload-identity-manager:spire-agent",
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
							"namespace":  utils.OperatorNamespace,
						},
					},
				},
			},
		},
		"server": map[string]interface{}{
			"audit_log_enabled": false,
			"bind_address":      "0.0.0.0",
			"bind_port":         "8081",
			"ca_key_type":       "ec-p256",
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
		},
		"telemetry": map[string]interface{}{
			"Prometheus": map[string]interface{}{
				"host": "0.0.0.0",
				"port": "9402",
			},
		},
	}

	// Add federation configuration if present (inside server section)
	if config.Federation != nil {
		serverSection := confMap["server"].(map[string]interface{})
		serverSection["federation"] = generateFederationConfig(config.Federation, config.TrustDomain)
	}

	return confMap
}

// marshalToJSON marshals a map to JSON with indentation
func marshalToJSON(data map[string]interface{}) ([]byte, error) {
	jsonBytes, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal server.conf: %w", err)
	}
	return jsonBytes, nil
}

// generateFederationConfig generates the federation configuration for SPIRE server
func generateFederationConfig(federation *v1alpha1.FederationConfig, trustDomain string) map[string]interface{} {
	federationConf := map[string]interface{}{
		"bundle_endpoint": generateBundleEndpointConfig(&federation.BundleEndpoint),
	}

	// Add federates_with configuration if present
	if len(federation.FederatesWith) > 0 {
		federatesWith := make(map[string]interface{})
		for _, fedTrust := range federation.FederatesWith {
			// Skip if this is the same as our trust domain (should be caught by validation)
			if fedTrust.TrustDomain == trustDomain {
				continue
			}

			trustConfig := map[string]interface{}{
				"bundle_endpoint_url": fedTrust.BundleEndpointUrl,
			}

			// Add bundle endpoint profile configuration
			switch fedTrust.BundleEndpointProfile {
			case v1alpha1.HttpsSpiffeProfile:
				trustConfig["bundle_endpoint_profile"] = map[string]interface{}{
					"https_spiffe": map[string]interface{}{
						"endpoint_spiffe_id": fedTrust.EndpointSpiffeId,
					},
				}
			case v1alpha1.HttpsWebProfile:
				trustConfig["bundle_endpoint_profile"] = map[string]interface{}{
					"https_web": map[string]interface{}{},
				}
			}

			federatesWith[fedTrust.TrustDomain] = trustConfig
		}
		federationConf["federates_with"] = federatesWith
	}

	return federationConf
}

// generateBundleEndpointConfig generates the bundle endpoint configuration
func generateBundleEndpointConfig(bundleEndpoint *v1alpha1.BundleEndpointConfig) map[string]interface{} {
	endpointConf := map[string]interface{}{
		"address": "0.0.0.0",
		"port":    8443,
	}

	// Add refresh hint if specified
	if bundleEndpoint.RefreshHint > 0 {
		endpointConf["refresh_hint"] = fmt.Sprintf("%ds", bundleEndpoint.RefreshHint)
	}

	// Configure profile-specific settings
	// According to SPIRE docs, profile-specific config should be nested under profile blocks
	switch bundleEndpoint.Profile {
	case v1alpha1.HttpsSpiffeProfile:
		// For https_spiffe, SPIRE uses its own SVID for TLS authentication
		// profile "https_spiffe" { }
		endpointConf["profile"] = map[string]interface{}{
			"https_spiffe": map[string]interface{}{},
		}
	case v1alpha1.HttpsWebProfile:
		// Configure https_web profile
		// profile "https_web" { acme { ... } or serving_cert_file { ... } }
		httpsWebProfile := map[string]interface{}{}

		if bundleEndpoint.HttpsWeb != nil {
			if bundleEndpoint.HttpsWeb.Acme != nil {
				httpsWebProfile["acme"] = map[string]interface{}{
					"directory_url": bundleEndpoint.HttpsWeb.Acme.DirectoryUrl,
					"domain_name":   bundleEndpoint.HttpsWeb.Acme.DomainName,
					"email":         bundleEndpoint.HttpsWeb.Acme.Email,
					"tos_accepted":  utils.StringToBool(bundleEndpoint.HttpsWeb.Acme.TosAccepted),
				}
			} else if bundleEndpoint.HttpsWeb.ServingCert != nil {
				// Mount certificate from Secret to /run/spire/federation-certs/
				servingCertFile := map[string]interface{}{
					"cert_file_path": "/run/spire/federation-certs/tls.crt",
					"key_file_path":  "/run/spire/federation-certs/tls.key",
				}
				if bundleEndpoint.HttpsWeb.ServingCert.FileSyncInterval > 0 {
					servingCertFile["file_sync_interval"] = fmt.Sprintf("%ds", bundleEndpoint.HttpsWeb.ServingCert.FileSyncInterval)
				}
				httpsWebProfile["serving_cert_file"] = servingCertFile
			}
		} else {
			// Use service CA certificate (for re-encrypt routes)
			httpsWebProfile["serving_cert_file"] = map[string]interface{}{
				"cert_file_path": "/run/spire/server-tls/tls.crt",
				"key_file_path":  "/run/spire/server-tls/tls.key",
			}
		}

		endpointConf["profile"] = map[string]interface{}{
			"https_web": httpsWebProfile,
		}
	}

	return endpointConf
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
