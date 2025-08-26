package spire_server

import (
	"fmt"
	"time"

	"github.com/openshift/zero-trust-workload-identity-manager/api/v1alpha1"
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
