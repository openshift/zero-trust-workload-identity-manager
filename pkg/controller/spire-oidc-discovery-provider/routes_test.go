package spire_oidc_discovery_provider

import (
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"

	routev1 "github.com/openshift/api/route/v1"
	"github.com/openshift/zero-trust-workload-identity-manager/api/v1alpha1"
	"github.com/openshift/zero-trust-workload-identity-manager/pkg/controller/utils"

	"k8s.io/apimachinery/pkg/util/intstr"
)

func TestGenerateOIDCDiscoveryProviderRoute(t *testing.T) {
	t.Run("should generate Route with basic configuration", func(t *testing.T) {
		// Arrange
		config := &v1alpha1.SpireOIDCDiscoveryProvider{
			Spec: v1alpha1.SpireOIDCDiscoveryProviderSpec{
				JwtIssuer:   "oidc-discovery.apps.example.com",
				TrustDomain: "example.org",
			},
		}

		// Act
		result := generateOIDCDiscoveryProviderRoute(config)

		// Assert
		require.NotNil(t, result)
		assert.IsType(t, &routev1.Route{}, result)

		// Verify ObjectMeta
		assert.Equal(t, "spiffe-oidc-discovery-provider", result.ObjectMeta.Name)
		assert.Equal(t, utils.OperatorNamespace, result.ObjectMeta.Namespace)

		// Verify default labels
		expectedLabels := map[string]string{
			"app.kubernetes.io/name":     "spiffe-oidc-discovery-provider",
			"app.kubernetes.io/instance": "spire",
			"app.kubernetes.io/part-of":  "zero-trust-workload-identity-manager",
			"app.kubernetes.io/version":  "1.12.0",
			utils.AppManagedByLabelKey:   utils.AppManagedByLabelValue,
		}
		assert.Equal(t, expectedLabels, result.ObjectMeta.Labels)

		// Verify Route Spec
		assert.Equal(t, "oidc-discovery.apps.example.com", result.Spec.Host)

		// Verify Port
		require.NotNil(t, result.Spec.Port)
		assert.Equal(t, intstr.FromString("https"), result.Spec.Port.TargetPort)

		// Verify TLS
		require.NotNil(t, result.Spec.TLS)
		assert.Equal(t, routev1.TLSTerminationReencrypt, result.Spec.TLS.Termination)
		assert.Equal(t, routev1.InsecureEdgeTerminationPolicyRedirect, result.Spec.TLS.InsecureEdgeTerminationPolicy)

		// Verify Target Service
		assert.Equal(t, "Service", result.Spec.To.Kind)
		assert.Equal(t, "spire-spiffe-oidc-discovery-provider", result.Spec.To.Name)
		require.NotNil(t, result.Spec.To.Weight)
		assert.Equal(t, int32(100), *result.Spec.To.Weight)

		// Verify Wildcard Policy
		assert.Equal(t, routev1.WildcardPolicyNone, result.Spec.WildcardPolicy)
	})

	t.Run("should generate Route with custom labels", func(t *testing.T) {
		// Arrange
		customLabels := map[string]string{
			"app":                       "spire-oidc",
			"version":                   "v2.0.0",         // This should override the default
			"app.kubernetes.io/part-of": "custom-part-of", // This should override the default
			"custom-key":                "custom-value",
		}
		config := &v1alpha1.SpireOIDCDiscoveryProvider{
			Spec: v1alpha1.SpireOIDCDiscoveryProviderSpec{
				JwtIssuer:   "test.apps.cluster.com",
				TrustDomain: "test.domain",
				CommonConfig: v1alpha1.CommonConfig{
					Labels: customLabels,
				},
			},
		}

		// Act
		result := generateOIDCDiscoveryProviderRoute(config)

		// Assert
		require.NotNil(t, result)

		// Check that custom labels are merged with defaults
		expectedLabels := map[string]string{
			"app.kubernetes.io/name":     "spiffe-oidc-discovery-provider",
			"app.kubernetes.io/instance": "spire",
			"app.kubernetes.io/part-of":  "custom-part-of", // Custom override
			"app.kubernetes.io/version":  "1.12.0",         // Default preserved
			"app":                        "spire-oidc",     // Custom addition
			"version":                    "v2.0.0",         // Custom addition
			"custom-key":                 "custom-value",   // Custom addition
			utils.AppManagedByLabelKey:   utils.AppManagedByLabelValue,
		}
		assert.Equal(t, expectedLabels, result.ObjectMeta.Labels)

		// Verify the host is correctly set
		assert.Equal(t, "test.apps.cluster.com", result.Spec.Host)
	})

	t.Run("should use TrustDomain fallback when JwtIssuer is empty", func(t *testing.T) {
		// Arrange
		config := &v1alpha1.SpireOIDCDiscoveryProvider{
			Spec: v1alpha1.SpireOIDCDiscoveryProviderSpec{
				JwtIssuer:   "", // Empty
				TrustDomain: "fallback.example.org",
			},
		}

		// Act
		result := generateOIDCDiscoveryProviderRoute(config)

		// Assert
		require.NotNil(t, result)
		assert.Equal(t, "oidc-discovery.fallback.example.org", result.Spec.Host)
	})

	t.Run("should handle nil labels gracefully", func(t *testing.T) {
		// Arrange
		config := &v1alpha1.SpireOIDCDiscoveryProvider{
			Spec: v1alpha1.SpireOIDCDiscoveryProviderSpec{
				JwtIssuer:   "nil-labels.example.com",
				TrustDomain: "example.org",
				CommonConfig: v1alpha1.CommonConfig{
					Labels: nil,
				},
			},
		}

		// Act
		result := generateOIDCDiscoveryProviderRoute(config)

		// Assert
		require.NotNil(t, result)

		// Should only have default labels
		expectedLabels := map[string]string{
			"app.kubernetes.io/name":     "spiffe-oidc-discovery-provider",
			"app.kubernetes.io/instance": "spire",
			"app.kubernetes.io/part-of":  "zero-trust-workload-identity-manager",
			"app.kubernetes.io/version":  "1.12.0",
			utils.AppManagedByLabelKey:   utils.AppManagedByLabelValue,
		}
		assert.Equal(t, expectedLabels, result.ObjectMeta.Labels)
	})

	t.Run("should allow custom managed-by label to override default", func(t *testing.T) {
		// Arrange
		config := &v1alpha1.SpireOIDCDiscoveryProvider{
			Spec: v1alpha1.SpireOIDCDiscoveryProviderSpec{
				JwtIssuer:   "test.example.com",
				TrustDomain: "example.org",
				CommonConfig: v1alpha1.CommonConfig{
					Labels: map[string]string{
						utils.AppManagedByLabelKey: "different-value",
						"other-label":              "other-value",
					},
				},
			},
		}

		// Act
		result := generateOIDCDiscoveryProviderRoute(config)

		// Assert
		// The function should allow custom labels to override defaults, including managed-by
		assert.Equal(t, "different-value", result.ObjectMeta.Labels[utils.AppManagedByLabelKey])
		assert.Equal(t, "other-value", result.ObjectMeta.Labels["other-label"])
	})

	t.Run("should return consistent results across multiple calls", func(t *testing.T) {
		// Arrange
		config := &v1alpha1.SpireOIDCDiscoveryProvider{
			Spec: v1alpha1.SpireOIDCDiscoveryProviderSpec{
				JwtIssuer:   "consistent.example.com",
				TrustDomain: "example.org",
			},
		}

		// Act
		result1 := generateOIDCDiscoveryProviderRoute(config)
		result2 := generateOIDCDiscoveryProviderRoute(config)

		// Assert
		assert.Equal(t, result1.ObjectMeta.Name, result2.ObjectMeta.Name)
		assert.Equal(t, result1.ObjectMeta.Namespace, result2.ObjectMeta.Namespace)
		assert.Equal(t, result1.ObjectMeta.Labels, result2.ObjectMeta.Labels)
		assert.Equal(t, result1.Spec.Host, result2.Spec.Host)
		assert.Equal(t, result1.Spec.To.Name, result2.Spec.To.Name)
		assert.Equal(t, result1.Spec.TLS.Termination, result2.Spec.TLS.Termination)
	})

	t.Run("should verify route spec structure", func(t *testing.T) {
		// Arrange
		config := &v1alpha1.SpireOIDCDiscoveryProvider{
			Spec: v1alpha1.SpireOIDCDiscoveryProviderSpec{
				JwtIssuer:   "structure-test.example.com",
				TrustDomain: "example.org",
			},
		}

		// Act
		result := generateOIDCDiscoveryProviderRoute(config)

		// Assert detailed structure verification
		require.NotNil(t, result)

		// Verify Port structure
		require.NotNil(t, result.Spec.Port)
		assert.Equal(t, intstr.FromString("https"), result.Spec.Port.TargetPort)

		// Verify TLS structure
		require.NotNil(t, result.Spec.TLS)
		assert.Equal(t, routev1.TLSTerminationReencrypt, result.Spec.TLS.Termination)
		assert.Equal(t, routev1.InsecureEdgeTerminationPolicyRedirect, result.Spec.TLS.InsecureEdgeTerminationPolicy)

		// Verify To structure
		assert.Equal(t, "Service", result.Spec.To.Kind)
		assert.Equal(t, "spire-spiffe-oidc-discovery-provider", result.Spec.To.Name)
		require.NotNil(t, result.Spec.To.Weight)
		assert.Equal(t, int32(100), *result.Spec.To.Weight)

		// Verify Wildcard Policy
		assert.Equal(t, routev1.WildcardPolicyNone, result.Spec.WildcardPolicy)
	})
}

// Table-driven test for different host scenarios
func TestGenerateOIDCDiscoveryProviderRoute_HostScenarios(t *testing.T) {
	testCases := []struct {
		name         string
		jwtIssuer    string
		trustDomain  string
		expectedHost string
	}{
		{
			name:         "simple hostname from JwtIssuer",
			jwtIssuer:    "oidc.example.com",
			trustDomain:  "example.com",
			expectedHost: "oidc.example.com",
		},
		{
			name:         "openshift apps subdomain from JwtIssuer",
			jwtIssuer:    "oidc-discovery.apps.cluster.example.com",
			trustDomain:  "cluster.example.com",
			expectedHost: "oidc-discovery.apps.cluster.example.com",
		},
		{
			name:         "fallback to TrustDomain when JwtIssuer is empty",
			jwtIssuer:    "",
			trustDomain:  "fallback.local",
			expectedHost: "oidc-discovery.fallback.local",
		},
		{
			name:         "IP address in JwtIssuer",
			jwtIssuer:    "192.168.1.100",
			trustDomain:  "internal.local",
			expectedHost: "192.168.1.100",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			config := &v1alpha1.SpireOIDCDiscoveryProvider{
				Spec: v1alpha1.SpireOIDCDiscoveryProviderSpec{
					JwtIssuer:   tc.jwtIssuer,
					TrustDomain: tc.trustDomain,
				},
			}

			result := generateOIDCDiscoveryProviderRoute(config)

			require.NotNil(t, result)
			assert.Equal(t, tc.expectedHost, result.Spec.Host)
		})
	}
}

// Test to ensure no mutation of input config
func TestGenerateOIDCDiscoveryProviderRoute_NoMutation(t *testing.T) {
	originalLabels := map[string]string{
		"original": "value",
	}
	config := &v1alpha1.SpireOIDCDiscoveryProvider{
		Spec: v1alpha1.SpireOIDCDiscoveryProviderSpec{
			JwtIssuer:   "test.example.com",
			TrustDomain: "example.org",
			CommonConfig: v1alpha1.CommonConfig{
				Labels: originalLabels,
			},
		},
	}

	// Act
	_ = generateOIDCDiscoveryProviderRoute(config)

	// Assert - original config should not be modified
	assert.Equal(t, map[string]string{"original": "value"}, config.Spec.Labels)
	assert.Len(t, config.Spec.Labels, 1) // Should still have only one label
}

// TestCheckRouteConflict tests the checkRouteConflict function with various scenarios
func TestCheckRouteConflict(t *testing.T) {
	t.Run("should return false when routes are identical", func(t *testing.T) {
		// Arrange
		current := &routev1.Route{
			Spec: routev1.RouteSpec{
				Host: "test.example.com",
				TLS: &routev1.TLSConfig{
					Termination: routev1.TLSTerminationReencrypt,
				},
				To: routev1.RouteTargetReference{
					Kind: "Service",
					Name: "test-service",
				},
			},
		}

		desired := &routev1.Route{
			Spec: routev1.RouteSpec{
				Host: "test.example.com",
				TLS: &routev1.TLSConfig{
					Termination: routev1.TLSTerminationReencrypt,
				},
				To: routev1.RouteTargetReference{
					Kind: "Service",
					Name: "test-service",
				},
			},
		}

		// Act
		result := checkRouteConflict(current, desired)

		// Assert
		assert.False(t, result, "identical routes should not have conflicts")
	})

	t.Run("should return true when current TLS is nil", func(t *testing.T) {
		// Arrange
		current := &routev1.Route{
			Spec: routev1.RouteSpec{
				Host: "test.example.com",
				TLS:  nil, // nil TLS
				To: routev1.RouteTargetReference{
					Kind: "Service",
					Name: "test-service",
				},
			},
		}

		desired := &routev1.Route{
			Spec: routev1.RouteSpec{
				Host: "test.example.com",
				TLS: &routev1.TLSConfig{
					Termination: routev1.TLSTerminationReencrypt,
				},
				To: routev1.RouteTargetReference{
					Kind: "Service",
					Name: "test-service",
				},
			},
		}

		// Act
		result := checkRouteConflict(current, desired)

		// Assert
		assert.True(t, result, "nil TLS should cause conflict")
	})

	t.Run("should return true when TLS termination differs", func(t *testing.T) {
		// Arrange
		current := &routev1.Route{
			Spec: routev1.RouteSpec{
				Host: "test.example.com",
				TLS: &routev1.TLSConfig{
					Termination: routev1.TLSTerminationEdge,
				},
				To: routev1.RouteTargetReference{
					Kind: "Service",
					Name: "test-service",
				},
			},
		}

		desired := &routev1.Route{
			Spec: routev1.RouteSpec{
				Host: "test.example.com",
				TLS: &routev1.TLSConfig{
					Termination: routev1.TLSTerminationReencrypt,
				},
				To: routev1.RouteTargetReference{
					Kind: "Service",
					Name: "test-service",
				},
			},
		}

		// Act
		result := checkRouteConflict(current, desired)

		// Assert
		assert.True(t, result, "different TLS termination should cause conflict")
	})

	t.Run("should return true when host differs", func(t *testing.T) {
		// Arrange
		current := &routev1.Route{
			Spec: routev1.RouteSpec{
				Host: "current.example.com",
				TLS: &routev1.TLSConfig{
					Termination: routev1.TLSTerminationReencrypt,
				},
				To: routev1.RouteTargetReference{
					Kind: "Service",
					Name: "test-service",
				},
			},
		}

		desired := &routev1.Route{
			Spec: routev1.RouteSpec{
				Host: "desired.example.com",
				TLS: &routev1.TLSConfig{
					Termination: routev1.TLSTerminationReencrypt,
				},
				To: routev1.RouteTargetReference{
					Kind: "Service",
					Name: "test-service",
				},
			},
		}

		// Act
		result := checkRouteConflict(current, desired)

		// Assert
		assert.True(t, result, "different host should cause conflict")
	})

	t.Run("should return true when target service name differs", func(t *testing.T) {
		// Arrange
		current := &routev1.Route{
			Spec: routev1.RouteSpec{
				Host: "test.example.com",
				TLS: &routev1.TLSConfig{
					Termination: routev1.TLSTerminationReencrypt,
				},
				To: routev1.RouteTargetReference{
					Kind: "Service",
					Name: "current-service",
				},
			},
		}

		desired := &routev1.Route{
			Spec: routev1.RouteSpec{
				Host: "test.example.com",
				TLS: &routev1.TLSConfig{
					Termination: routev1.TLSTerminationReencrypt,
				},
				To: routev1.RouteTargetReference{
					Kind: "Service",
					Name: "desired-service",
				},
			},
		}

		// Act
		result := checkRouteConflict(current, desired)

		// Assert
		assert.True(t, result, "different target service name should cause conflict")
	})

	t.Run("should return true when target kind differs", func(t *testing.T) {
		// Arrange
		current := &routev1.Route{
			Spec: routev1.RouteSpec{
				Host: "test.example.com",
				TLS: &routev1.TLSConfig{
					Termination: routev1.TLSTerminationReencrypt,
				},
				To: routev1.RouteTargetReference{
					Kind: "Service",
					Name: "test-service",
				},
			},
		}

		desired := &routev1.Route{
			Spec: routev1.RouteSpec{
				Host: "test.example.com",
				TLS: &routev1.TLSConfig{
					Termination: routev1.TLSTerminationReencrypt,
				},
				To: routev1.RouteTargetReference{
					Kind: "Pod",
					Name: "test-service",
				},
			},
		}

		// Act
		result := checkRouteConflict(current, desired)

		// Assert
		assert.True(t, result, "different target kind should cause conflict")
	})

	t.Run("should handle multiple conflicts", func(t *testing.T) {
		// Arrange
		current := &routev1.Route{
			Spec: routev1.RouteSpec{
				Host: "current.example.com",
				TLS: &routev1.TLSConfig{
					Termination: routev1.TLSTerminationEdge,
				},
				To: routev1.RouteTargetReference{
					Kind: "Pod",
					Name: "current-service",
				},
			},
		}

		desired := &routev1.Route{
			Spec: routev1.RouteSpec{
				Host: "desired.example.com",
				TLS: &routev1.TLSConfig{
					Termination: routev1.TLSTerminationReencrypt,
				},
				To: routev1.RouteTargetReference{
					Kind: "Service",
					Name: "desired-service",
				},
			},
		}

		// Act
		result := checkRouteConflict(current, desired)

		// Assert
		assert.True(t, result, "multiple conflicts should return true")
	})
}

// TestCheckRouteConflict_TableDriven provides comprehensive test coverage using table-driven tests
func TestCheckRouteConflict_TableDriven(t *testing.T) {
	testCases := []struct {
		name             string
		currentTLS       *routev1.TLSConfig
		desiredTLS       *routev1.TLSConfig
		currentHost      string
		desiredHost      string
		currentToName    string
		desiredToName    string
		currentToKind    string
		desiredToKind    string
		expectedConflict bool
		description      string
	}{
		{
			name: "no conflict - all fields match",
			currentTLS: &routev1.TLSConfig{
				Termination: routev1.TLSTerminationReencrypt,
			},
			desiredTLS: &routev1.TLSConfig{
				Termination: routev1.TLSTerminationReencrypt,
			},
			currentHost:      "test.example.com",
			desiredHost:      "test.example.com",
			currentToName:    "test-service",
			desiredToName:    "test-service",
			currentToKind:    "Service",
			desiredToKind:    "Service",
			expectedConflict: false,
			description:      "identical routes should not conflict",
		},
		{
			name:       "conflict - current TLS is nil",
			currentTLS: nil,
			desiredTLS: &routev1.TLSConfig{
				Termination: routev1.TLSTerminationReencrypt,
			},
			currentHost:      "test.example.com",
			desiredHost:      "test.example.com",
			currentToName:    "test-service",
			desiredToName:    "test-service",
			currentToKind:    "Service",
			desiredToKind:    "Service",
			expectedConflict: true,
			description:      "nil current TLS should cause conflict",
		},
		{
			name: "conflict - TLS termination differs",
			currentTLS: &routev1.TLSConfig{
				Termination: routev1.TLSTerminationEdge,
			},
			desiredTLS: &routev1.TLSConfig{
				Termination: routev1.TLSTerminationReencrypt,
			},
			currentHost:      "test.example.com",
			desiredHost:      "test.example.com",
			currentToName:    "test-service",
			desiredToName:    "test-service",
			currentToKind:    "Service",
			desiredToKind:    "Service",
			expectedConflict: true,
			description:      "different TLS termination should cause conflict",
		},
		{
			name: "conflict - host differs",
			currentTLS: &routev1.TLSConfig{
				Termination: routev1.TLSTerminationReencrypt,
			},
			desiredTLS: &routev1.TLSConfig{
				Termination: routev1.TLSTerminationReencrypt,
			},
			currentHost:      "current.example.com",
			desiredHost:      "desired.example.com",
			currentToName:    "test-service",
			desiredToName:    "test-service",
			currentToKind:    "Service",
			desiredToKind:    "Service",
			expectedConflict: true,
			description:      "different host should cause conflict",
		},
		{
			name: "conflict - target service name differs",
			currentTLS: &routev1.TLSConfig{
				Termination: routev1.TLSTerminationReencrypt,
			},
			desiredTLS: &routev1.TLSConfig{
				Termination: routev1.TLSTerminationReencrypt,
			},
			currentHost:      "test.example.com",
			desiredHost:      "test.example.com",
			currentToName:    "current-service",
			desiredToName:    "desired-service",
			currentToKind:    "Service",
			desiredToKind:    "Service",
			expectedConflict: true,
			description:      "different target service name should cause conflict",
		},
		{
			name: "conflict - target kind differs",
			currentTLS: &routev1.TLSConfig{
				Termination: routev1.TLSTerminationReencrypt,
			},
			desiredTLS: &routev1.TLSConfig{
				Termination: routev1.TLSTerminationReencrypt,
			},
			currentHost:      "test.example.com",
			desiredHost:      "test.example.com",
			currentToName:    "test-service",
			desiredToName:    "test-service",
			currentToKind:    "Service",
			desiredToKind:    "Pod",
			expectedConflict: true,
			description:      "different target kind should cause conflict",
		},
		{
			name: "conflict - multiple fields differ",
			currentTLS: &routev1.TLSConfig{
				Termination: routev1.TLSTerminationEdge,
			},
			desiredTLS: &routev1.TLSConfig{
				Termination: routev1.TLSTerminationReencrypt,
			},
			currentHost:      "current.example.com",
			desiredHost:      "desired.example.com",
			currentToName:    "current-service",
			desiredToName:    "desired-service",
			currentToKind:    "Pod",
			desiredToKind:    "Service",
			expectedConflict: true,
			description:      "multiple field differences should cause conflict",
		},
		{
			name: "no conflict - empty hosts",
			currentTLS: &routev1.TLSConfig{
				Termination: routev1.TLSTerminationReencrypt,
			},
			desiredTLS: &routev1.TLSConfig{
				Termination: routev1.TLSTerminationReencrypt,
			},
			currentHost:      "",
			desiredHost:      "",
			currentToName:    "test-service",
			desiredToName:    "test-service",
			currentToKind:    "Service",
			desiredToKind:    "Service",
			expectedConflict: false,
			description:      "empty hosts should not cause conflict if they match",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Arrange
			current := &routev1.Route{
				Spec: routev1.RouteSpec{
					Host: tc.currentHost,
					TLS:  tc.currentTLS,
					To: routev1.RouteTargetReference{
						Kind: tc.currentToKind,
						Name: tc.currentToName,
					},
				},
			}

			desired := &routev1.Route{
				Spec: routev1.RouteSpec{
					Host: tc.desiredHost,
					TLS:  tc.desiredTLS,
					To: routev1.RouteTargetReference{
						Kind: tc.desiredToKind,
						Name: tc.desiredToName,
					},
				},
			}

			// Act
			result := checkRouteConflict(current, desired)

			// Assert
			assert.Equal(t, tc.expectedConflict, result, tc.description)
		})
	}
}
