package spire_oidc_discovery_provider

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/openshift/zero-trust-workload-identity-manager/api/v1alpha1"
	"github.com/openshift/zero-trust-workload-identity-manager/pkg/controller/utils"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// GenerateOIDCConfigMapFromCR creates a ConfigMap for the spire oidc discovery provider from the CR spec
func GenerateOIDCConfigMapFromCR(dp *v1alpha1.SpireOIDCDiscoveryProvider) (*corev1.ConfigMap, error) {
	if dp == nil {
		return nil, errors.New("spire OIDC Discovery Provider Config is nil")
	}
	// Default to "spire-agent.sock" if not provided
	agentSocketName := dp.Spec.AgentSocketName
	if agentSocketName == "" {
		agentSocketName = "spire-agent.sock"
	}

	// Determine trust domain
	trustDomain := dp.Spec.TrustDomain

	// JWT Issuer validation and normalization
	jwtIssuer, err := utils.StripProtocolFromJWTIssuer(dp.Spec.JwtIssuer)
	if err != nil {
		return nil, fmt.Errorf("invalid JWT issuer URL: %w", err)
	}

	// OIDC config map data
	oidcConfig := map[string]interface{}{
		"domains": []string{
			"spire-spiffe-oidc-discovery-provider",
			"spire-spiffe-oidc-discovery-provider.zero-trust-workload-identity-manager",
			"spire-spiffe-oidc-discovery-provider.zero-trust-workload-identity-manager.svc.cluster.local",
			jwtIssuer,
		},
		"health_checks": map[string]string{
			"bind_port":  "8008",
			"live_path":  "/live",
			"ready_path": "/ready",
		},
		"log_level": "debug",
		"serving_cert_file": map[string]string{
			"addr":           ":8443",
			"cert_file_path": "/certs/tls.crt",
			"key_file_path":  "/certs/tls.key",
		},
		"workload_api": map[string]string{
			"socket_path":  "/spiffe-workload-api/" + agentSocketName,
			"trust_domain": trustDomain,
		},
	}

	oidcJSON, err := json.MarshalIndent(oidcConfig, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal OIDC config: %w", err)
	}

	spiffeHelperConf := `agent_address = "/spiffe-workload-api/` + agentSocketName + `"
cert_dir = "/certs"
svid_file_name = "tls.crt"
svid_key_file_name = "tls.key"
svid_bundle_file_name = "ca.pem"`

	defaultConf := `upstream oidc {
  server unix:/run/spire/oidc-sockets/spire-oidc-server.sock;
}

server {
  listen            8080;
  listen       [::]:8080;

  location / {
    proxy_pass http://oidc;
    proxy_set_header Host $host;
  }

  location /stub_status {
    allow 127.0.0.1/32;
    deny  all;
    stub_status on;
  }
}`

	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "spire-spiffe-oidc-discovery-provider",
			Namespace: utils.OperatorNamespace,
			Labels:    utils.SpireOIDCDiscoveryProviderLabels(dp.Spec.Labels),
		},
		Data: map[string]string{
			"oidc-discovery-provider.conf": string(oidcJSON),
			"spiffe-helper.conf":           spiffeHelperConf,
			"default.conf":                 defaultConf,
		},
	}

	return configMap, nil
}
