package spire_server

import (
	"fmt"
	"strings"
	"time"

	"github.com/openshift/zero-trust-workload-identity-manager/api/v1alpha1"
	"github.com/openshift/zero-trust-workload-identity-manager/pkg/controller/utils"
)

// SPIRE upstream validation constants
const (
	sevenDays                  = 7 * 24 * time.Hour
	activationThresholdCap     = sevenDays
	activationThresholdDivisor = 6
)

// TTLValidationResult contains validation results including warnings and status messages
type TTLValidationResult struct {
	Warnings      []string
	StatusMessage string
	Error         error
}

// MaxSVIDTTL returns the maximum SVID lifetime that can be guaranteed to not
// be cut artificially short by a scheduled rotation.
func MaxSVIDTTL() time.Duration {
	return activationThresholdCap
}

// MaxSVIDTTLForCATTL returns the maximum SVID TTL that can be guaranteed given
// a specific CA TTL. In other words, given a CA TTL, what is the largest SVID
// TTL that is guaranteed to not be cut artificially short by a scheduled
// rotation?
func MaxSVIDTTLForCATTL(caTTL time.Duration) time.Duration {
	if caTTL/activationThresholdDivisor < activationThresholdCap {
		return caTTL / activationThresholdDivisor
	}
	return activationThresholdCap
}

// MinCATTLForSVIDTTL returns the minimum CA TTL necessary to guarantee an SVID
// TTL of the provided value. In other words, given an SVID TTL, what is the
// minimum CA TTL that will guarantee that the SVIDs lifetime won't be cut
// artificially short by a scheduled rotation?
func MinCATTLForSVIDTTL(svidTTL time.Duration) time.Duration {
	return svidTTL * activationThresholdDivisor
}

// hasCompatibleTTL checks if the CA TTL is compatible with the given SVID TTL
func hasCompatibleTTL(caTTL, svidTTL time.Duration) bool {
	return MaxSVIDTTLForCATTL(caTTL) >= svidTTL
}

// printDuration formats a duration for user-friendly display
func printDuration(d time.Duration) string {
	return d.String()
}

// printMaxSVIDTTL returns the maximum SVID TTL that can be guaranteed for the given CA TTL
func printMaxSVIDTTL(caTTL time.Duration) string {
	return MaxSVIDTTLForCATTL(caTTL).String()
}

// printMinCATTL returns the minimum CA TTL needed for the given SVID TTL
func printMinCATTL(svidTTL time.Duration) string {
	return MinCATTLForSVIDTTL(svidTTL).String()
}

// validateTTLDurationsWithWarnings validates TTL values using upstream SPIRE logic
func validateTTLDurationsWithWarnings(config *v1alpha1.SpireServerSpec) TTLValidationResult {
	var result TTLValidationResult
	var warningMessages []string

	if config.CAValidity.Duration <= 0 {
		result.Error = fmt.Errorf("ca_ttl must be a positive duration")
		return result
	}
	if config.DefaultX509Validity.Duration <= 0 {
		result.Error = fmt.Errorf("default_x509_svid_ttl must be a positive duration")
		return result
	}
	if config.DefaultJWTValidity.Duration <= 0 {
		result.Error = fmt.Errorf("default_jwt_svid_ttl must be a positive duration")
		return result
	}

	if config.CAValidity.Duration < config.DefaultJWTValidity.Duration {
		result.Error = fmt.Errorf("ca_validity must be greater than default_jwt_svid_ttl")
		return result
	}
	if config.CAValidity.Duration < config.DefaultX509Validity.Duration {
		result.Error = fmt.Errorf("ca_validity must be greater than default_ca_ttl")
		return result
	}

	ttlChecks := []struct {
		name string
		ttl  time.Duration
	}{
		{
			name: "default_x509_svid_ttl",
			ttl:  config.DefaultX509Validity.Duration,
		},
		{
			name: "default_jwt_svid_ttl",
			ttl:  config.DefaultJWTValidity.Duration,
		},
	}

	for _, ttlCheck := range ttlChecks {
		if !hasCompatibleTTL(config.CAValidity.Duration, ttlCheck.ttl) {
			var message string

			switch {
			case ttlCheck.ttl < MaxSVIDTTL():
				// TTL is smaller than our cap, but the CA TTL
				// is not large enough to accommodate it
				message = fmt.Sprintf("%s is too high for the configured "+
					"ca_ttl value. SVIDs with shorter lifetimes may "+
					"be issued. Please set %s to %v or less, or the ca_ttl "+
					"to %v or more, to guarantee the full %s lifetime "+
					"when CA rotations are scheduled.",
					ttlCheck.name, ttlCheck.name, printMaxSVIDTTL(config.CAValidity.Duration), printMinCATTL(ttlCheck.ttl), ttlCheck.name,
				)
			case config.CAValidity.Duration < MinCATTLForSVIDTTL(MaxSVIDTTL()):
				// TTL is larger than our cap, it needs to be
				// decreased no matter what. Additionally, the CA TTL is
				// too small to accommodate the maximum SVID TTL.
				message = fmt.Sprintf("%s is too high and "+
					"the ca_ttl is too low. SVIDs with shorter lifetimes "+
					"may be issued. Please set %s to %v or less, and the "+
					"ca_ttl to %v or more, to guarantee the full %s "+
					"lifetime when CA rotations are scheduled.",
					ttlCheck.name, ttlCheck.name, printDuration(MaxSVIDTTL()), printMinCATTL(MaxSVIDTTL()), ttlCheck.name,
				)
			default:
				// TTL is larger than our cap and needs to be
				// decreased.
				message = fmt.Sprintf("%s is too high. SVIDs with shorter "+
					"lifetimes may be issued. Please set %s to %v or less "+
					"to guarantee the full %s lifetime when CA rotations "+
					"are scheduled.",
					ttlCheck.name, ttlCheck.name, printMaxSVIDTTL(config.CAValidity.Duration), ttlCheck.name,
				)
			}
			warningMessages = append(warningMessages, message)
		}
	}

	result.Warnings = warningMessages
	if len(warningMessages) > 0 {
		result.StatusMessage = fmt.Sprintf("TTL configuration warnings: %d issues found", len(warningMessages))
	}

	return result
}

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

	// Check for duplicate trust domains and self-federation
	seenDomains := make(map[string]bool)
	for i, fedTrust := range federation.FederatesWith {
		// Check for self-federation
		if fedTrust.TrustDomain == trustDomain {
			return fmt.Errorf("federatesWith[%d]: cannot federate with own trust domain %s", i, trustDomain)
		}

		// Check for duplicates
		if seenDomains[fedTrust.TrustDomain] {
			return fmt.Errorf("federatesWith[%d]: duplicate trust domain %s", i, fedTrust.TrustDomain)
		}
		seenDomains[fedTrust.TrustDomain] = true

		if err := validateFederatedTrustDomain(&fedTrust, i); err != nil {
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

	// SecretName is optional - if empty, defaults to service CA certificate (spire-server-serving-cert)

	if servingCert.FileSyncInterval > 0 && (servingCert.FileSyncInterval < 300 || servingCert.FileSyncInterval > 86400) {
		return fmt.Errorf("fileSyncInterval must be between 300 and 86400 seconds, got %d", servingCert.FileSyncInterval)
	}

	return nil
}

// validateFederatedTrustDomain validates a single federated trust domain configuration
func validateFederatedTrustDomain(fedTrust *v1alpha1.FederatesWithConfig, index int) error {
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
