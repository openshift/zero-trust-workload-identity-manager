package spire_oidc_discovery_provider

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/openshift/zero-trust-workload-identity-manager/api/v1alpha1"
	"github.com/openshift/zero-trust-workload-identity-manager/pkg/controller/utils"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// GenerateOIDCConfigMapFromCR creates a ConfigMap for the spire oidc discovery provider from the CR spec
func GenerateOIDCConfigMapFromCR(cr *v1alpha1.SpireOIDCDiscoveryProvider) (*corev1.ConfigMap, error) {
	if cr == nil {
		return nil, errors.New("spire OIDC Discovery Provider Config is nil")
	}
	// Default to "spire-agent.sock" if not provided
	agentSocketName := cr.Spec.AgentSocketName
	if agentSocketName == "" {
		agentSocketName = "spire-agent.sock"
	}

	// Determine trust domain
	trustDomain := cr.Spec.TrustDomain

	// JWT Issuer fallback
	jwtIssuer := cr.Spec.JwtIssuer
	if jwtIssuer == "" {
		jwtIssuer = fmt.Sprintf("oidc-discovery.%s", trustDomain)
	} else {
		jwtIssuer = stripProtocol(jwtIssuer)
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

	labels := map[string]string{}
	for key, value := range cr.Spec.Labels {
		labels[key] = value
	}
	labels[utils.AppManagedByLabelKey] = utils.AppManagedByLabelValue
	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "spire-spiffe-oidc-discovery-provider",
			Namespace: utils.OperatorNamespace,
			Labels:    labels,
		},
		Data: map[string]string{
			"oidc-discovery-provider.conf": string(oidcJSON),
		},
	}

	return configMap, nil
}

// stripProtocol removes "http://" or "https://"" from the beginning of a string.
// If no protocol prefix is found, it returns the original string unmodified.
func stripProtocol(url string) string {
	if strings.HasPrefix(url, "https://") {
		return strings.TrimPrefix(url, "https://")
	}
	if strings.HasPrefix(url, "http://") {
		return strings.TrimPrefix(url, "http://")
	}
	return url
}
