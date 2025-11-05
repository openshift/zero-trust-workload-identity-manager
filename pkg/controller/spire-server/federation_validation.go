package spire_server

import (
	"fmt"
	"strings"

	"github.com/openshift/zero-trust-workload-identity-manager/api/v1alpha1"
	"github.com/openshift/zero-trust-workload-identity-manager/pkg/controller/utils"
)

// validateFederationConfig validates the federation configuration
func validateFederationConfig(federation *v1alpha1.FederationConfig, trustDomain string) error {
	if federation == nil {
		return nil
	}

	// Validate bundle endpoint configuration
	if err := validateBundleEndpoint(&federation.BundleEndpoint); err != nil {
		return fmt.Errorf("invalid bundle endpoint configuration: %w", err)
	}

	// Validate federatesWith entries
	if len(federation.FederatesWith) > 50 {
		return fmt.Errorf("federatesWith array cannot exceed 50 entries, got %d", len(federation.FederatesWith))
	}

	for i, fedTrust := range federation.FederatesWith {
		if err := validateFederatedTrustDomain(&fedTrust, i, trustDomain); err != nil {
			return err
		}
	}

	return nil
}

// validateBundleEndpoint validates the bundle endpoint configuration
func validateBundleEndpoint(bundleEndpoint *v1alpha1.BundleEndpointConfig) error {
	// Validate profile-specific configuration
	if bundleEndpoint.Profile == v1alpha1.HttpsWebProfile {
		if bundleEndpoint.HttpsWeb == nil {
			return fmt.Errorf("httpsWeb configuration is required when profile is https_web")
		}

		acmeSet := bundleEndpoint.HttpsWeb.Acme != nil
		certSet := bundleEndpoint.HttpsWeb.ServingCert != nil

		if acmeSet && certSet {
			return fmt.Errorf("acme and servingCert are mutually exclusive, only one can be set")
		}

		if !acmeSet && !certSet {
			return fmt.Errorf("either acme or servingCert must be set for https_web profile")
		}

		// Validate ACME configuration
		if acmeSet {
			if err := validateAcmeConfig(bundleEndpoint.HttpsWeb.Acme); err != nil {
				return fmt.Errorf("invalid ACME configuration: %w", err)
			}
		}

		// Validate ServingCert configuration
		if certSet {
			if err := validateServingCertConfig(bundleEndpoint.HttpsWeb.ServingCert); err != nil {
				return fmt.Errorf("invalid ServingCert configuration: %w", err)
			}
		}
	}

	// Validate port range
	if bundleEndpoint.Port < 1 || bundleEndpoint.Port > 65535 {
		return fmt.Errorf("port must be between 1 and 65535, got %d", bundleEndpoint.Port)
	}

	// Validate refresh hint
	if bundleEndpoint.RefreshHint > 0 && (bundleEndpoint.RefreshHint < 60 || bundleEndpoint.RefreshHint > 3600) {
		return fmt.Errorf("refreshHint must be between 60 and 3600 seconds, got %d", bundleEndpoint.RefreshHint)
	}

	return nil
}

// validateAcmeConfig validates ACME configuration
func validateAcmeConfig(acme *v1alpha1.AcmeConfig) error {
	if acme == nil {
		return nil
	}

	if !strings.HasPrefix(acme.DirectoryUrl, "https://") {
		return fmt.Errorf("directoryUrl must use https://, got %s", acme.DirectoryUrl)
	}

	if acme.DomainName == "" {
		return fmt.Errorf("domainName is required")
	}

	if acme.Email == "" {
		return fmt.Errorf("email is required")
	}

	if !utils.StringToBool(acme.TosAccepted) {
		return fmt.Errorf("tosAccepted must be true to use ACME")
	}

	return nil
}

// validateServingCertConfig validates ServingCert configuration
func validateServingCertConfig(servingCert *v1alpha1.ServingCertConfig) error {
	if servingCert == nil {
		return nil
	}

	if servingCert.SecretName == "" {
		return fmt.Errorf("secretName is required")
	}

	if servingCert.FileSyncInterval > 0 && (servingCert.FileSyncInterval < 30 || servingCert.FileSyncInterval > 3600) {
		return fmt.Errorf("fileSyncInterval must be between 30 and 3600 seconds, got %d", servingCert.FileSyncInterval)
	}

	return nil
}

// validateFederatedTrustDomain validates a single federated trust domain configuration
func validateFederatedTrustDomain(fedTrust *v1alpha1.FederatesWithConfig, index int, trustDomain string) error {
	// Cannot federate with self
	if fedTrust.TrustDomain == trustDomain {
		return fmt.Errorf("federatesWith[%d]: cannot federate with own trust domain %s", index, trustDomain)
	}

	// Validate trust domain format
	if fedTrust.TrustDomain == "" {
		return fmt.Errorf("federatesWith[%d]: trustDomain is required", index)
	}

	// Validate URL format
	if !strings.HasPrefix(fedTrust.BundleEndpointUrl, "https://") {
		return fmt.Errorf("federatesWith[%d]: bundleEndpointUrl must use https://, got %s", index, fedTrust.BundleEndpointUrl)
	}

	// Validate https_spiffe requires endpointSpiffeId
	if fedTrust.BundleEndpointProfile == v1alpha1.HttpsSpiffeProfile {
		if fedTrust.EndpointSpiffeId == "" {
			return fmt.Errorf("federatesWith[%d]: endpointSpiffeId is required for https_spiffe profile", index)
		}
		if !strings.HasPrefix(fedTrust.EndpointSpiffeId, "spiffe://") {
			return fmt.Errorf("federatesWith[%d]: endpointSpiffeId must start with spiffe://, got %s", index, fedTrust.EndpointSpiffeId)
		}
	}

	return nil
}
