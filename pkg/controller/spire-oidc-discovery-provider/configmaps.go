package spire_oidc_discovery_provider

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/openshift/zero-trust-workload-identity-manager/api/v1alpha1"
	"github.com/openshift/zero-trust-workload-identity-manager/pkg/config"
	"github.com/openshift/zero-trust-workload-identity-manager/pkg/controller/utils"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// buildSpireOIDCDiscoveryProviderConfig creates a SpireOIDCDiscoveryProviderConfig from the operator API spec
func buildSpireOIDCDiscoveryProviderConfig(spec *v1alpha1.SpireOIDCDiscoveryProviderSpec, jwtIssuerStripped string) *config.SpireOIDCDiscoveryProviderConfig {
	agentSocketName := spec.AgentSocketName
	if agentSocketName == "" {
		agentSocketName = DefaultAgentSocketName
	}

	oidcConfig := &config.SpireOIDCDiscoveryProviderConfig{
		Domains:   getDefaultDomains(jwtIssuerStripped),
		LogLevel:  utils.GetLogLevelFromString(spec.LogLevel),
		LogFormat: utils.GetLogFormatFromString(spec.LogFormat),
		WorkloadAPI: config.WorkloadAPIConfig{
			SocketPath:  DefaultWorkloadAPISocketBasePath + "/" + agentSocketName,
			TrustDomain: spec.TrustDomain,
		},
		ServingCertFile: &config.ServingCertFileConfig{
			Addr:         DefaultServingCertAddr,
			CertFilePath: DefaultServingCertFilePath,
			KeyFilePath:  DefaultServingCertKeyFilePath,
		},
		HealthChecks: config.OIDCHealthChecksConfig{
			BindPort:  DefaultHealthCheckBindPort,
			LivePath:  DefaultHealthCheckLivePath,
			ReadyPath: DefaultHealthCheckReadyPath,
		},
	}

	return oidcConfig
}

// GenerateOIDCConfigMapFromCR creates a ConfigMap for the spire oidc discovery provider from the CR spec
func GenerateOIDCConfigMapFromCR(dp *v1alpha1.SpireOIDCDiscoveryProvider) (*corev1.ConfigMap, error) {
	if dp == nil {
		return nil, errors.New("spire OIDC Discovery Provider Config is nil")
	}

	// JWT Issuer validation and normalization
	jwtIssuerStripped, err := utils.StripProtocolFromJWTIssuer(dp.Spec.JwtIssuer)
	if err != nil {
		return nil, fmt.Errorf("invalid JWT issuer URL: %w", err)
	}

	// Build config struct from operator API spec
	oidcConfig := buildSpireOIDCDiscoveryProviderConfig(&dp.Spec, jwtIssuerStripped)

	// Marshal to JSON
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
