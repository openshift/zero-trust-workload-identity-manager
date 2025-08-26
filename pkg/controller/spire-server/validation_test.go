package spire_server

import (
	"strings"
	"testing"
	"time"

	"github.com/openshift/zero-trust-workload-identity-manager/api/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestMaxSVIDTTL(t *testing.T) {
	expected := 7 * 24 * time.Hour // sevenDays
	if MaxSVIDTTL() != expected {
		t.Errorf("MaxSVIDTTL() = %v, expected %v", MaxSVIDTTL(), expected)
	}
}

func TestMaxSVIDTTLForCATTL(t *testing.T) {
	tests := []struct {
		name     string
		caTTL    time.Duration
		expected time.Duration
	}{
		{
			name:     "CA TTL allows maximum SVID TTL",
			caTTL:    42 * 24 * time.Hour, // 42 days / 6 = 7 days (cap)
			expected: 7 * 24 * time.Hour,  // activationThresholdCap
		},
		{
			name:     "CA TTL smaller than maximum",
			caTTL:    12 * time.Hour, // 12 hours / 6 = 2 hours
			expected: 2 * time.Hour,  // caTTL / activationThresholdDivisor
		},
		{
			name:     "CA TTL exactly at threshold",
			caTTL:    42 * time.Hour, // 42 hours / 6 = 7 hours
			expected: 7 * time.Hour,  // caTTL / activationThresholdDivisor
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := MaxSVIDTTLForCATTL(tt.caTTL)
			if result != tt.expected {
				t.Errorf("MaxSVIDTTLForCATTL(%v) = %v, expected %v", tt.caTTL, result, tt.expected)
			}
		})
	}
}

func TestMinCATTLForSVIDTTL(t *testing.T) {
	tests := []struct {
		name     string
		svidTTL  time.Duration
		expected time.Duration
	}{
		{
			name:     "1 hour SVID TTL",
			svidTTL:  1 * time.Hour,
			expected: 6 * time.Hour, // 1 hour * 6
		},
		{
			name:     "2 hour SVID TTL",
			svidTTL:  2 * time.Hour,
			expected: 12 * time.Hour, // 2 hours * 6
		},
		{
			name:     "30 minute SVID TTL",
			svidTTL:  30 * time.Minute,
			expected: 3 * time.Hour, // 30 minutes * 6
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := MinCATTLForSVIDTTL(tt.svidTTL)
			if result != tt.expected {
				t.Errorf("MinCATTLForSVIDTTL(%v) = %v, expected %v", tt.svidTTL, result, tt.expected)
			}
		})
	}
}

func TestHasCompatibleTTL(t *testing.T) {
	tests := []struct {
		name     string
		caTTL    time.Duration
		svidTTL  time.Duration
		expected bool
	}{
		{
			name:     "Compatible TTLs",
			caTTL:    12 * time.Hour, // 12 hours / 6 = 2 hours max SVID TTL
			svidTTL:  1 * time.Hour,  // 1 hour < 2 hours
			expected: true,
		},
		{
			name:     "Incompatible TTLs",
			caTTL:    6 * time.Hour, // 6 hours / 6 = 1 hour max SVID TTL
			svidTTL:  2 * time.Hour, // 2 hours > 1 hour
			expected: false,
		},
		{
			name:     "Exactly compatible",
			caTTL:    6 * time.Hour, // 6 hours / 6 = 1 hour max SVID TTL
			svidTTL:  1 * time.Hour, // 1 hour = 1 hour
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := hasCompatibleTTL(tt.caTTL, tt.svidTTL)
			if result != tt.expected {
				t.Errorf("hasCompatibleTTL(%v, %v) = %v, expected %v", tt.caTTL, tt.svidTTL, result, tt.expected)
			}
		})
	}
}

func TestValidateTTLDurationsWithWarnings(t *testing.T) {
	tests := []struct {
		name            string
		config          *v1alpha1.SpireServerSpec
		expectError     bool
		expectWarnings  int
		warningContains []string
		statusMessage   string
	}{
		{
			name: "valid configuration - no warnings",
			config: &v1alpha1.SpireServerSpec{
				CAValidity:          metav1.Duration{Duration: 24 * time.Hour}, // 24h / 6 = 4h max SVID TTL
				DefaultX509Validity: metav1.Duration{Duration: 2 * time.Hour},  // 2h < 4h (compatible)
				DefaultJWTValidity:  metav1.Duration{Duration: 1 * time.Hour},  // 1h < 4h (compatible)
			},
			expectError:    false,
			expectWarnings: 0,
		},
		{
			name: "incompatible X509 SVID TTL - generates warning",
			config: &v1alpha1.SpireServerSpec{
				CAValidity:          metav1.Duration{Duration: 12 * time.Hour},   // 12h / 6 = 2h max SVID TTL
				DefaultX509Validity: metav1.Duration{Duration: 4 * time.Hour},    // 4h > 2h (incompatible)
				DefaultJWTValidity:  metav1.Duration{Duration: 30 * time.Minute}, // 30m < 2h (compatible)
			},
			expectError:    false,
			expectWarnings: 1,
			warningContains: []string{
				"default_x509_svid_ttl is too high for the configured ca_ttl value",
			},
			statusMessage: "TTL configuration warnings: 1 issues found",
		},
		{
			name: "incompatible JWT SVID TTL - generates warning",
			config: &v1alpha1.SpireServerSpec{
				CAValidity:          metav1.Duration{Duration: 6 * time.Hour},    // 6h / 6 = 1h max SVID TTL
				DefaultX509Validity: metav1.Duration{Duration: 30 * time.Minute}, // 30m < 1h (compatible)
				DefaultJWTValidity:  metav1.Duration{Duration: 2 * time.Hour},    // 2h > 1h (incompatible)
			},
			expectError:    false,
			expectWarnings: 1,
			warningContains: []string{
				"default_jwt_svid_ttl is too high for the configured ca_ttl value",
			},
			statusMessage: "TTL configuration warnings: 1 issues found",
		},
		{
			name: "multiple incompatible TTLs - generates multiple warnings",
			config: &v1alpha1.SpireServerSpec{
				CAValidity:          metav1.Duration{Duration: 6 * time.Hour}, // 6h / 6 = 1h max SVID TTL
				DefaultX509Validity: metav1.Duration{Duration: 3 * time.Hour}, // 3h > 1h (incompatible)
				DefaultJWTValidity:  metav1.Duration{Duration: 2 * time.Hour}, // 2h > 1h (incompatible)
			},
			expectError:    false,
			expectWarnings: 2,
			warningContains: []string{
				"default_x509_svid_ttl is too high for the configured ca_ttl value",
				"default_jwt_svid_ttl is too high for the configured ca_ttl value",
			},
			statusMessage: "TTL configuration warnings: 2 issues found",
		},
		{
			name: "error - zero CA TTL",
			config: &v1alpha1.SpireServerSpec{
				CAValidity:          metav1.Duration{Duration: 0},
				DefaultX509Validity: metav1.Duration{Duration: 1 * time.Hour},
				DefaultJWTValidity:  metav1.Duration{Duration: 30 * time.Minute},
			},
			expectError:    true,
			expectWarnings: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := validateTTLDurationsWithWarnings(tt.config)

			// Check error expectation
			if (result.Error != nil) != tt.expectError {
				t.Errorf("validateTTLDurationsWithWarnings() error = %v, expectError = %v", result.Error, tt.expectError)
				return
			}

			// Check warnings count
			if len(result.Warnings) != tt.expectWarnings {
				t.Errorf("validateTTLDurationsWithWarnings() returned %d warnings, expected %d. Warnings: %v",
					len(result.Warnings), tt.expectWarnings, result.Warnings)
			}

			// Check warning content
			for _, expectedContent := range tt.warningContains {
				found := false
				for _, warning := range result.Warnings {
					if containsString(warning, expectedContent) {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("validateTTLDurationsWithWarnings() warnings don't contain expected content %q. Warnings: %v",
						expectedContent, result.Warnings)
				}
			}

			// Check status message
			if tt.statusMessage != "" && result.StatusMessage != tt.statusMessage {
				t.Errorf("validateTTLDurationsWithWarnings() statusMessage = %q, expected %q",
					result.StatusMessage, tt.statusMessage)
			}
		})
	}
}

// containsString checks if a string contains a substring
func containsString(s, substr string) bool {
	return strings.Contains(s, substr)
}
