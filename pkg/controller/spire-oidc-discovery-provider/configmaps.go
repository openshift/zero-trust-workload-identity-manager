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
		"log_level":  utils.GetLogLevelFromString(dp.Spec.LogLevel),
		"log_format": utils.GetLogFormatFromString(dp.Spec.LogFormat),
		"serving_cert_file": map[string]string{
			"addr":           ":8443",
			"cert_file_path": "/etc/oidc/tls/tls.crt",
			"key_file_path":  "/etc/oidc/tls/tls.key",
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

	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "spire-spiffe-oidc-discovery-provider",
			Namespace: utils.OperatorNamespace,
			Labels:    utils.SpireOIDCDiscoveryProviderLabels(dp.Spec.Labels),
		},
		Data: map[string]string{
			"oidc-discovery-provider.conf": string(oidcJSON),
		},
	}

	return configMap, nil
}
