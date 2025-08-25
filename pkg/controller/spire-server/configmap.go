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

const (
	// Mount paths (directories where secrets are mounted)
	vaultCaCertMountPath     = "/vault-ca-cert"
	vaultClientCertMountPath = "/vault-client-cert"
	vaultClientKeyMountPath  = "/vault-client-key"

	// Volume names (used in Kubernetes volume definitions)
	vaultCaCertVolumeName     = "vault-ca-cert"
	vaultClientCertVolumeName = "vault-client-cert"
	vaultClientKeyVolumeName  = "vault-client-key"

	// Full file paths (mount path + filename)
	defaultVaultCaCertpath = vaultCaCertMountPath + "/ca.crt"
	vaultClientCertPath    = vaultClientCertMountPath + "/tls.crt"
	vaultClientKeyPath     = vaultClientKeyMountPath + "/tls.key"

	// Kubernetes auth token path
	k8sAuthTokenPath = "/var/run/secrets/kubernetes.io/serviceaccount/token"
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
	confMap, err := generateServerConfMap(config)
	if err != nil {
		return nil, err
	}
	confJSON, err := marshalToJSON(confMap)
	if err != nil {
		return nil, err
	}
	labels := map[string]string{}
	for key, value := range config.Labels {
		labels[key] = value
	}
	labels[utils.AppManagedByLabelKey] = utils.AppManagedByLabelValue

	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "spire-server",
			Namespace: utils.OperatorNamespace,
			Labels:    labels,
		},
		Data: map[string]string{
			"server.conf": string(confJSON),
		},
	}

	return cm, nil
}

// generateServerConfMap builds the server.conf structure as a Go map
func generateServerConfMap(config *v1alpha1.SpireServerSpec) (map[string]interface{}, error) {
	plugins := map[string]interface{}{
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
	}

	// Add UpstreamAuthority plugin if configured
	if config.UpstreamAuthority != nil {
		upstreamAuthority, err := generateUpstreamAuthorityPlugin(config.UpstreamAuthority)
		if err != nil {
			return nil, err
		}
		if upstreamAuthority != nil {
			plugins["UpstreamAuthority"] = []map[string]interface{}{upstreamAuthority}
		}
	}

	return map[string]interface{}{
		"health_checks": map[string]interface{}{
			"bind_address":     "0.0.0.0",
			"bind_port":        "8080",
			"listener_enabled": true,
			"live_path":        "/live",
			"ready_path":       "/ready",
		},
		"plugins": plugins,
		"server": map[string]interface{}{
			"audit_log_enabled": false,
			"bind_address":      "0.0.0.0",
			"bind_port":         "8081",
			"ca_key_type":       "rsa-2048",
			"ca_subject": []map[string]interface{}{
				{
					"common_name":  config.CASubject.CommonName,
					"country":      []string{config.CASubject.Country},
					"organization": []string{config.CASubject.Organization},
				},
			},
			"ca_ttl":                "24h",
			"data_dir":              "/run/spire/data",
			"default_jwt_svid_ttl":  "1h",
			"default_x509_svid_ttl": "4h",
			"jwt_issuer":            config.JwtIssuer,
			"log_level":             "debug",
			"trust_domain":          config.TrustDomain,
		},
		"telemetry": map[string]interface{}{
			"Prometheus": map[string]interface{}{
				"host": "0.0.0.0",
				"port": "9402",
			},
		},
	}, nil
}

// generateUpstreamAuthorityPlugin generates the UpstreamAuthority plugin configuration
func generateUpstreamAuthorityPlugin(upstreamAuthority *v1alpha1.UpstreamAuthority) (map[string]interface{}, error) {
	switch upstreamAuthority.Type {
	case "cert-manager":
		if upstreamAuthority.CertManager == nil {
			return nil, errors.New("upstreamAuthority.CertManager is not set")
		}

		pluginData := map[string]interface{}{
			"issuer_name":  upstreamAuthority.CertManager.IssuerName,
			"issuer_kind":  getOrDefault(upstreamAuthority.CertManager.IssuerKind, "Issuer"),
			"issuer_group": getOrDefault(upstreamAuthority.CertManager.IssuerGroup, "cert-manager.io"),
			"namespace":    upstreamAuthority.CertManager.Namespace,
		}

		return map[string]interface{}{
			"cert-manager": map[string]interface{}{
				"plugin_data": pluginData,
			},
		}, nil

	case "vault":
		if upstreamAuthority.Vault == nil {
			return nil, errors.New("upstreamAuthority.Vault is not set")
		}

		vault := upstreamAuthority.Vault
		pluginData := map[string]interface{}{
			"vault_addr":      vault.VaultAddress,
			"pki_mount_point": vault.PkiMountPoint,
			"ca_cert_path":    defaultVaultCaCertpath,
		}

		// Add namespace if specified
		if vault.Namespace != "" {
			pluginData["namespace"] = vault.Namespace
		}

		// Configure authentication method
		if vault.TokenAuth != nil {
			pluginData["token_auth"] = map[string]interface{}{
				"token": vault.TokenAuth.Token,
			}
		} else if vault.CertAuth != nil {
			pluginData["cert_auth"] = map[string]interface{}{
				"cert_auth_mount_point": vault.CertAuth.CertAuthMountPoint,
				"client_cert_path":      vaultClientCertPath,
				"client_key_path":       vaultClientKeyPath,
			}
			if vault.CertAuth.CertAuthRoleName != "" {
				pluginData["cert_auth"].(map[string]interface{})["cert_auth_role_name"] = vault.CertAuth.CertAuthRoleName
			}
		} else if vault.AppRoleAuth != nil {
			pluginData["approle_auth"] = map[string]interface{}{
				"approle_auth_mount_point": vault.AppRoleAuth.AppRoleMountPoint,
				"approle_id":               vault.AppRoleAuth.AppRoleID,
				"approle_secret_id":        vault.AppRoleAuth.AppRoleSecretID,
			}
		} else if vault.K8sAuth != nil {
			pluginData["k8s_auth"] = map[string]interface{}{
				"k8s_auth_mount_point": vault.K8sAuth.K8sAuthMountPoint,
				"k8s_auth_role_name":   vault.K8sAuth.K8sAuthRoleName,
				"token_path":           getOrDefault(vault.K8sAuth.TokenPath, k8sAuthTokenPath),
			}
		} else {
			return nil, errors.New("vault upstream authority requires one authentication method to be configured")
		}

		return map[string]interface{}{
			"vault": map[string]interface{}{
				"plugin_data": pluginData,
			},
		}, nil

	default:
		return nil, fmt.Errorf("unsupported upstream authority type: %s", upstreamAuthority.Type)
	}
}

// getOrDefault returns the value if it's not empty, otherwise returns the default value
func getOrDefault(value, defaultValue string) string {
	if value != "" {
		return value
	}
	return defaultValue
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
			Labels: map[string]string{
				"app.kubernetes.io/name":     "server",
				"app.kubernetes.io/instance": "spire",
				"app.kubernetes.io/version":  "1.12.0",
				utils.AppManagedByLabelKey:   utils.AppManagedByLabelValue,
			},
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
			Labels: map[string]string{
				"app":                      "spire-controller-manager",
				utils.AppManagedByLabelKey: utils.AppManagedByLabelValue,
			},
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
			Labels: map[string]string{
				"app":                      "spire-server",
				utils.AppManagedByLabelKey: utils.AppManagedByLabelValue,
			},
		},
	}, nil
}
