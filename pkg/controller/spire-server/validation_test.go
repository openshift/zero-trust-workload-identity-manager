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

func TestValidateFederationConfig(t *testing.T) {
	tests := []struct {
		name        string
		federation  *v1alpha1.FederationConfig
		trustDomain string
		expectError bool
		errorMsg    string
	}{
		{
			name:        "Nil federation config",
			federation:  nil,
			trustDomain: "example.org",
			expectError: false,
		},
		{
			name: "Valid https_spiffe federation",
			federation: &v1alpha1.FederationConfig{
				BundleEndpoint: v1alpha1.BundleEndpointConfig{
					Profile:     v1alpha1.HttpsSpiffeProfile,
					RefreshHint: 300,
				},
				FederatesWith: []v1alpha1.FederatesWithConfig{
					{
						TrustDomain:           "remote.org",
						BundleEndpointUrl:     "https://remote.org:8443",
						BundleEndpointProfile: v1alpha1.HttpsSpiffeProfile,
						EndpointSpiffeId:      "spiffe://remote.org/spire/server",
					},
				},
			},
			trustDomain: "example.org",
			expectError: false,
		},
		{
			name: "Valid https_web federation with ACME",
			federation: &v1alpha1.FederationConfig{
				BundleEndpoint: v1alpha1.BundleEndpointConfig{
					Profile:     v1alpha1.HttpsWebProfile,
					RefreshHint: 300,
					HttpsWeb: &v1alpha1.HttpsWebConfig{
						Acme: &v1alpha1.AcmeConfig{
							DirectoryUrl: "https://acme-v02.api.letsencrypt.org/directory",
							DomainName:   "federation.example.org",
							Email:        "admin@example.org",
							TosAccepted:  "true",
						},
					},
				},
				FederatesWith: []v1alpha1.FederatesWithConfig{
					{
						TrustDomain:           "remote.org",
						BundleEndpointUrl:     "https://remote.org",
						BundleEndpointProfile: v1alpha1.HttpsWebProfile,
					},
				},
			},
			trustDomain: "example.org",
			expectError: false,
		},
		{
			name: "Valid https_web federation with ServingCert",
			federation: &v1alpha1.FederationConfig{
				BundleEndpoint: v1alpha1.BundleEndpointConfig{
					Profile:     v1alpha1.HttpsWebProfile,
					RefreshHint: 300,
					HttpsWeb: &v1alpha1.HttpsWebConfig{
						ServingCert: &v1alpha1.ServingCertConfig{
							FileSyncInterval: 3600,
						},
					},
				},
			},
			trustDomain: "example.org",
			expectError: false,
		},
		{
			name: "Self-federation error",
			federation: &v1alpha1.FederationConfig{
				BundleEndpoint: v1alpha1.BundleEndpointConfig{
					Profile:     v1alpha1.HttpsSpiffeProfile,
					RefreshHint: 300,
				},
				FederatesWith: []v1alpha1.FederatesWithConfig{
					{
						TrustDomain:           "example.org",
						BundleEndpointUrl:     "https://example.org:8443",
						BundleEndpointProfile: v1alpha1.HttpsSpiffeProfile,
						EndpointSpiffeId:      "spiffe://example.org/spire/server",
					},
				},
			},
			trustDomain: "example.org",
			expectError: true,
			errorMsg:    "cannot federate with own trust domain",
		},
		{
			name: "Duplicate trust domains",
			federation: &v1alpha1.FederationConfig{
				BundleEndpoint: v1alpha1.BundleEndpointConfig{
					Profile:     v1alpha1.HttpsSpiffeProfile,
					RefreshHint: 300,
				},
				FederatesWith: []v1alpha1.FederatesWithConfig{
					{
						TrustDomain:           "remote.org",
						BundleEndpointUrl:     "https://remote1.org:8443",
						BundleEndpointProfile: v1alpha1.HttpsSpiffeProfile,
						EndpointSpiffeId:      "spiffe://remote.org/spire/server",
					},
					{
						TrustDomain:           "remote.org",
						BundleEndpointUrl:     "https://remote2.org:8443",
						BundleEndpointProfile: v1alpha1.HttpsSpiffeProfile,
						EndpointSpiffeId:      "spiffe://remote.org/spire/server",
					},
				},
			},
			trustDomain: "example.org",
			expectError: true,
			errorMsg:    "duplicate trust domain",
		},
		{
			name: "Too many federatesWith entries",
			federation: &v1alpha1.FederationConfig{
				BundleEndpoint: v1alpha1.BundleEndpointConfig{
					Profile: v1alpha1.HttpsSpiffeProfile,
				},
				FederatesWith: make([]v1alpha1.FederatesWithConfig, 51),
			},
			trustDomain: "example.org",
			expectError: true,
			errorMsg:    "federatesWith array cannot exceed 50 entries",
		},
		{
			name: "Invalid refresh hint - too low",
			federation: &v1alpha1.FederationConfig{
				BundleEndpoint: v1alpha1.BundleEndpointConfig{
					Profile:     v1alpha1.HttpsSpiffeProfile,
					RefreshHint: 30,
				},
			},
			trustDomain: "example.org",
			expectError: true,
			errorMsg:    "refreshHint must be between 60 and 3600 seconds",
		},
		{
			name: "Invalid refresh hint - too high",
			federation: &v1alpha1.FederationConfig{
				BundleEndpoint: v1alpha1.BundleEndpointConfig{
					Profile:     v1alpha1.HttpsSpiffeProfile,
					RefreshHint: 4000,
				},
			},
			trustDomain: "example.org",
			expectError: true,
			errorMsg:    "refreshHint must be between 60 and 3600 seconds",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateFederationConfig(tt.federation, tt.trustDomain)

			if (err != nil) != tt.expectError {
				t.Errorf("validateFederationConfig() error = %v, expectError = %v", err, tt.expectError)
				return
			}

			if tt.expectError && err != nil && !containsString(err.Error(), tt.errorMsg) {
				t.Errorf("validateFederationConfig() error = %q, expected to contain %q", err.Error(), tt.errorMsg)
			}
		})
	}
}

func TestValidateBundleEndpoint(t *testing.T) {
	tests := []struct {
		name           string
		bundleEndpoint *v1alpha1.BundleEndpointConfig
		expectError    bool
		errorMsg       string
	}{
		{
			name: "Valid https_spiffe profile",
			bundleEndpoint: &v1alpha1.BundleEndpointConfig{
				Profile:     v1alpha1.HttpsSpiffeProfile,
				RefreshHint: 300,
			},
			expectError: false,
		},
		{
			name: "Valid https_web with ACME",
			bundleEndpoint: &v1alpha1.BundleEndpointConfig{
				Profile:     v1alpha1.HttpsWebProfile,
				RefreshHint: 300,
				HttpsWeb: &v1alpha1.HttpsWebConfig{
					Acme: &v1alpha1.AcmeConfig{
						DirectoryUrl: "https://acme-v02.api.letsencrypt.org/directory",
						DomainName:   "example.org",
						Email:        "admin@example.org",
						TosAccepted:  "true",
					},
				},
			},
			expectError: false,
		},
		{
			name: "Valid https_web with ServingCert",
			bundleEndpoint: &v1alpha1.BundleEndpointConfig{
				Profile:     v1alpha1.HttpsWebProfile,
				RefreshHint: 300,
				HttpsWeb: &v1alpha1.HttpsWebConfig{
					ServingCert: &v1alpha1.ServingCertConfig{
						FileSyncInterval: 3600,
					},
				},
			},
			expectError: false,
		},
		{
			name: "https_web without HttpsWeb config",
			bundleEndpoint: &v1alpha1.BundleEndpointConfig{
				Profile:     v1alpha1.HttpsWebProfile,
				RefreshHint: 300,
			},
			expectError: true,
			errorMsg:    "httpsWeb configuration is required when profile is https_web",
		},
		{
			name: "https_web with both ACME and ServingCert",
			bundleEndpoint: &v1alpha1.BundleEndpointConfig{
				Profile:     v1alpha1.HttpsWebProfile,
				RefreshHint: 300,
				HttpsWeb: &v1alpha1.HttpsWebConfig{
					Acme: &v1alpha1.AcmeConfig{
						DirectoryUrl: "https://acme-v02.api.letsencrypt.org/directory",
						DomainName:   "example.org",
						Email:        "admin@example.org",
						TosAccepted:  "true",
					},
					ServingCert: &v1alpha1.ServingCertConfig{},
				},
			},
			expectError: true,
			errorMsg:    "acme and servingCert are mutually exclusive",
		},
		{
			name: "https_web with neither ACME nor ServingCert",
			bundleEndpoint: &v1alpha1.BundleEndpointConfig{
				Profile:     v1alpha1.HttpsWebProfile,
				RefreshHint: 300,
				HttpsWeb:    &v1alpha1.HttpsWebConfig{},
			},
			expectError: true,
			errorMsg:    "either acme or servingCert must be set for https_web profile",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateBundleEndpoint(tt.bundleEndpoint)

			if (err != nil) != tt.expectError {
				t.Errorf("validateBundleEndpoint() error = %v, expectError = %v", err, tt.expectError)
				return
			}

			if tt.expectError && err != nil && !containsString(err.Error(), tt.errorMsg) {
				t.Errorf("validateBundleEndpoint() error = %q, expected to contain %q", err.Error(), tt.errorMsg)
			}
		})
	}
}

func TestValidateAcmeConfig(t *testing.T) {
	tests := []struct {
		name        string
		acme        *v1alpha1.AcmeConfig
		expectError bool
		errorMsg    string
	}{
		{
			name:        "Nil ACME config",
			acme:        nil,
			expectError: false,
		},
		{
			name: "Valid ACME config",
			acme: &v1alpha1.AcmeConfig{
				DirectoryUrl: "https://acme-v02.api.letsencrypt.org/directory",
				DomainName:   "example.org",
				Email:        "admin@example.org",
				TosAccepted:  "true",
			},
			expectError: false,
		},
		{
			name: "Valid ACME config with complex email",
			acme: &v1alpha1.AcmeConfig{
				DirectoryUrl: "https://acme-v02.api.letsencrypt.org/directory",
				DomainName:   "example.org",
				Email:        "user.name+tag@sub-domain.example.org",
				TosAccepted:  "true",
			},
			expectError: false,
		},
		{
			name: "Valid ACME config with numbered email",
			acme: &v1alpha1.AcmeConfig{
				DirectoryUrl: "https://acme-v02.api.letsencrypt.org/directory",
				DomainName:   "example.org",
				Email:        "admin123@example456.com",
				TosAccepted:  "true",
			},
			expectError: false,
		},
		{
			name: "Invalid directory URL - not https",
			acme: &v1alpha1.AcmeConfig{
				DirectoryUrl: "http://acme.example.org/directory",
				DomainName:   "example.org",
				Email:        "admin@example.org",
				TosAccepted:  "true",
			},
			expectError: true,
			errorMsg:    "directoryUrl must use https://",
		},
		{
			name: "Missing domain name",
			acme: &v1alpha1.AcmeConfig{
				DirectoryUrl: "https://acme-v02.api.letsencrypt.org/directory",
				DomainName:   "",
				Email:        "admin@example.org",
				TosAccepted:  "true",
			},
			expectError: true,
			errorMsg:    "domainName is required",
		},
		{
			name: "Missing email",
			acme: &v1alpha1.AcmeConfig{
				DirectoryUrl: "https://acme-v02.api.letsencrypt.org/directory",
				DomainName:   "example.org",
				Email:        "",
				TosAccepted:  "true",
			},
			expectError: true,
			errorMsg:    "email is required",
		},
		{
			name: "TOS not accepted",
			acme: &v1alpha1.AcmeConfig{
				DirectoryUrl: "https://acme-v02.api.letsencrypt.org/directory",
				DomainName:   "example.org",
				Email:        "admin@example.org",
				TosAccepted:  "false",
			},
			expectError: true,
			errorMsg:    "tosAccepted must be true to use ACME",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateAcmeConfig(tt.acme)

			if (err != nil) != tt.expectError {
				t.Errorf("validateAcmeConfig() error = %v, expectError = %v", err, tt.expectError)
				return
			}

			if tt.expectError && err != nil && !containsString(err.Error(), tt.errorMsg) {
				t.Errorf("validateAcmeConfig() error = %q, expected to contain %q", err.Error(), tt.errorMsg)
			}
		})
	}
}

func TestValidateServingCertConfig(t *testing.T) {
	tests := []struct {
		name        string
		servingCert *v1alpha1.ServingCertConfig
		expectError bool
		errorMsg    string
	}{
		{
			name:        "Nil ServingCert config",
			servingCert: nil,
			expectError: false,
		},
		{
			name: "Valid ServingCert config",
			servingCert: &v1alpha1.ServingCertConfig{
				FileSyncInterval: 3600,
			},
			expectError: false,
		},
		{
			name: "Valid ServingCert with service CA certificate",
			servingCert: &v1alpha1.ServingCertConfig{
				FileSyncInterval: 3600,
			},
			expectError: false,
		},
		{
			name: "Valid FileSyncInterval at minimum",
			servingCert: &v1alpha1.ServingCertConfig{
				FileSyncInterval: 3600,
			},
			expectError: false,
		},
		{
			name: "Valid FileSyncInterval at maximum",
			servingCert: &v1alpha1.ServingCertConfig{
				FileSyncInterval: 7776000,
			},
			expectError: false,
		},
		{
			name: "Invalid FileSyncInterval - too low",
			servingCert: &v1alpha1.ServingCertConfig{
				FileSyncInterval: 3599,
			},
			expectError: true,
			errorMsg:    "fileSyncInterval must be between 3600 and 7776000 seconds",
		},
		{
			name: "Invalid FileSyncInterval - too high",
			servingCert: &v1alpha1.ServingCertConfig{
				FileSyncInterval: 7776001,
			},
			expectError: true,
			errorMsg:    "fileSyncInterval must be between 3600 and 7776000 seconds",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateServingCertConfig(tt.servingCert)

			if (err != nil) != tt.expectError {
				t.Errorf("validateServingCertConfig() error = %v, expectError = %v", err, tt.expectError)
				return
			}

			if tt.expectError && err != nil && !containsString(err.Error(), tt.errorMsg) {
				t.Errorf("validateServingCertConfig() error = %q, expected to contain %q", err.Error(), tt.errorMsg)
			}
		})
	}
}

func TestValidateFederatedTrustDomain(t *testing.T) {
	tests := []struct {
		name        string
		fedTrust    *v1alpha1.FederatesWithConfig
		index       int
		expectError bool
		errorMsg    string
	}{
		{
			name: "Valid https_spiffe trust domain",
			fedTrust: &v1alpha1.FederatesWithConfig{
				TrustDomain:           "remote.org",
				BundleEndpointUrl:     "https://remote.org:8443",
				BundleEndpointProfile: v1alpha1.HttpsSpiffeProfile,
				EndpointSpiffeId:      "spiffe://remote.org/spire/server",
			},
			index:       0,
			expectError: false,
		},
		{
			name: "Valid https_web trust domain",
			fedTrust: &v1alpha1.FederatesWithConfig{
				TrustDomain:           "remote.org",
				BundleEndpointUrl:     "https://remote.org",
				BundleEndpointProfile: v1alpha1.HttpsWebProfile,
			},
			index:       0,
			expectError: false,
		},
		{
			name: "Empty trust domain",
			fedTrust: &v1alpha1.FederatesWithConfig{
				TrustDomain:           "",
				BundleEndpointUrl:     "https://remote.org:8443",
				BundleEndpointProfile: v1alpha1.HttpsSpiffeProfile,
				EndpointSpiffeId:      "spiffe://remote.org/spire/server",
			},
			index:       0,
			expectError: true,
			errorMsg:    "trustDomain is required",
		},
		{
			name: "Invalid URL - not https",
			fedTrust: &v1alpha1.FederatesWithConfig{
				TrustDomain:           "remote.org",
				BundleEndpointUrl:     "http://remote.org:8443",
				BundleEndpointProfile: v1alpha1.HttpsSpiffeProfile,
				EndpointSpiffeId:      "spiffe://remote.org/spire/server",
			},
			index:       0,
			expectError: true,
			errorMsg:    "bundleEndpointUrl must use https://",
		},
		{
			name: "https_spiffe without endpointSpiffeId",
			fedTrust: &v1alpha1.FederatesWithConfig{
				TrustDomain:           "remote.org",
				BundleEndpointUrl:     "https://remote.org:8443",
				BundleEndpointProfile: v1alpha1.HttpsSpiffeProfile,
				EndpointSpiffeId:      "",
			},
			index:       0,
			expectError: true,
			errorMsg:    "endpointSpiffeId is required for https_spiffe profile",
		},
		{
			name: "https_spiffe with invalid endpointSpiffeId",
			fedTrust: &v1alpha1.FederatesWithConfig{
				TrustDomain:           "remote.org",
				BundleEndpointUrl:     "https://remote.org:8443",
				BundleEndpointProfile: v1alpha1.HttpsSpiffeProfile,
				EndpointSpiffeId:      "invalid://remote.org/spire/server",
			},
			index:       0,
			expectError: true,
			errorMsg:    "endpointSpiffeId must start with spiffe://",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateFederatedTrustDomain(tt.fedTrust, tt.index)

			if (err != nil) != tt.expectError {
				t.Errorf("validateFederatedTrustDomain() error = %v, expectError = %v", err, tt.expectError)
				return
			}

			if tt.expectError && err != nil && !containsString(err.Error(), tt.errorMsg) {
				t.Errorf("validateFederatedTrustDomain() error = %q, expected to contain %q", err.Error(), tt.errorMsg)
			}
		})
	}
}
