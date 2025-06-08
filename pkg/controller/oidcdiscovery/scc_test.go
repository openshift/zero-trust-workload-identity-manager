package oidcdiscovery

import (
	"testing"

	securityv1 "github.com/openshift/api/security/v1"
	"github.com/openshift/zero-trust-workload-identity-manager/api/v1alpha1"
	"github.com/openshift/zero-trust-workload-identity-manager/pkg/controller/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerateSpireOIDCDiscoveryProviderSCC(t *testing.T) {
	t.Run("should generate SCC with minimal config", func(t *testing.T) {
		// Arrange
		config := &v1alpha1.SpireOIDCDiscoveryProviderConfig{
			Spec: v1alpha1.SpireOIDCDiscoveryProviderConfigSpec{
				CommonConfig: v1alpha1.CommonConfig{
					Labels: map[string]string{},
				},
			},
		}

		// Act
		result := generateSpireOIDCDiscoveryProviderSCC(config)

		// Assert
		require.NotNil(t, result)
		assert.IsType(t, &securityv1.SecurityContextConstraints{}, result)

		// Verify ObjectMeta
		assert.Equal(t, "spire-spiffe-oidc-discovery-provider", result.ObjectMeta.Name)

		// Should have the managed-by label added
		expectedLabels := map[string]string{
			utils.AppManagedByLabelKey: utils.AppManagedByLabelValue,
		}
		assert.Equal(t, expectedLabels, result.ObjectMeta.Labels)
	})

	t.Run("should generate SCC with custom labels", func(t *testing.T) {
		// Arrange
		customLabels := map[string]string{
			"app":        "spire-oidc",
			"version":    "v1.0.0",
			"component":  "security",
			"custom-key": "custom-value",
		}
		config := &v1alpha1.SpireOIDCDiscoveryProviderConfig{
			Spec: v1alpha1.SpireOIDCDiscoveryProviderConfigSpec{
				CommonConfig: v1alpha1.CommonConfig{
					Labels: customLabels,
				},
			},
		}

		// Act
		result := generateSpireOIDCDiscoveryProviderSCC(config)

		// Assert
		require.NotNil(t, result)

		// Should have custom labels plus the managed-by label
		expectedLabels := map[string]string{
			"app":                      "spire-oidc",
			"version":                  "v1.0.0",
			"component":                "security",
			"custom-key":               "custom-value",
			utils.AppManagedByLabelKey: utils.AppManagedByLabelValue,
		}
		assert.Equal(t, expectedLabels, result.ObjectMeta.Labels)
	})

	t.Run("should set correct security context constraints", func(t *testing.T) {
		// Arrange
		config := &v1alpha1.SpireOIDCDiscoveryProviderConfig{
			Spec: v1alpha1.SpireOIDCDiscoveryProviderConfigSpec{
				CommonConfig: v1alpha1.CommonConfig{
					Labels: map[string]string{},
				},
			},
		}

		// Act
		result := generateSpireOIDCDiscoveryProviderSCC(config)

		// Assert
		// Test ReadOnlyRootFilesystem
		assert.True(t, result.ReadOnlyRootFilesystem)

		// Test RunAsUser strategy
		assert.Equal(t, securityv1.RunAsUserStrategyRunAsAny, result.RunAsUser.Type)

		// Test SELinuxContext strategy
		assert.Equal(t, securityv1.SELinuxStrategyRunAsAny, result.SELinuxContext.Type)

		// Test SupplementalGroups strategy
		assert.Equal(t, securityv1.SupplementalGroupsStrategyRunAsAny, result.SupplementalGroups.Type)

		// Test FSGroup strategy
		assert.Equal(t, securityv1.FSGroupStrategyRunAsAny, result.FSGroup.Type)
	})

	t.Run("should set correct host permissions", func(t *testing.T) {
		// Arrange
		config := &v1alpha1.SpireOIDCDiscoveryProviderConfig{
			Spec: v1alpha1.SpireOIDCDiscoveryProviderConfigSpec{
				CommonConfig: v1alpha1.CommonConfig{
					Labels: map[string]string{},
				},
			},
		}

		// Act
		result := generateSpireOIDCDiscoveryProviderSCC(config)

		// Assert
		assert.True(t, result.AllowHostDirVolumePlugin)
		assert.True(t, result.AllowHostIPC)
		assert.True(t, result.AllowHostNetwork)
		assert.True(t, result.AllowHostPID)
		assert.True(t, result.AllowHostPorts)
	})

	t.Run("should set correct privilege settings", func(t *testing.T) {
		// Arrange
		config := &v1alpha1.SpireOIDCDiscoveryProviderConfig{
			Spec: v1alpha1.SpireOIDCDiscoveryProviderConfigSpec{
				CommonConfig: v1alpha1.CommonConfig{
					Labels: map[string]string{},
				},
			},
		}

		// Act
		result := generateSpireOIDCDiscoveryProviderSCC(config)

		// Assert
		assert.True(t, result.AllowPrivilegedContainer)
		assert.NotNil(t, result.AllowPrivilegeEscalation)
		assert.True(t, *result.AllowPrivilegeEscalation)
	})

	t.Run("should set correct users", func(t *testing.T) {
		// Arrange
		config := &v1alpha1.SpireOIDCDiscoveryProviderConfig{
			Spec: v1alpha1.SpireOIDCDiscoveryProviderConfigSpec{
				CommonConfig: v1alpha1.CommonConfig{
					Labels: map[string]string{},
				},
			},
		}

		// Act
		result := generateSpireOIDCDiscoveryProviderSCC(config)

		// Assert
		expectedUsers := []string{
			"system:serviceaccount:zero-trust-workload-identity-manager:spire-spiffe-oidc-discovery-provider",
			"system:serviceaccount:zero-trust-workload-identity-manager:spire-spiffe-oidc-discovery-provider-pre-delete",
		}
		assert.Equal(t, expectedUsers, result.Users)
		assert.Len(t, result.Users, 2)
	})

	t.Run("should set correct volume types", func(t *testing.T) {
		// Arrange
		config := &v1alpha1.SpireOIDCDiscoveryProviderConfig{
			Spec: v1alpha1.SpireOIDCDiscoveryProviderConfigSpec{
				CommonConfig: v1alpha1.CommonConfig{
					Labels: map[string]string{},
				},
			},
		}

		// Act
		result := generateSpireOIDCDiscoveryProviderSCC(config)

		// Assert
		expectedVolumes := []securityv1.FSType{
			securityv1.FSTypeConfigMap,
			securityv1.FSTypeCSI,
			securityv1.FSTypeDownwardAPI,
			securityv1.FSTypeEphemeral,
			securityv1.FSTypeHostPath,
			securityv1.FSProjected,
			securityv1.FSTypeSecret,
			securityv1.FSTypeEmptyDir,
		}
		assert.Equal(t, expectedVolumes, result.Volumes)
		assert.Len(t, result.Volumes, 8)

		// Verify each volume type is present
		for _, expectedVolume := range expectedVolumes {
			assert.Contains(t, result.Volumes, expectedVolume)
		}
	})

	t.Run("should set correct capability settings", func(t *testing.T) {
		// Arrange
		config := &v1alpha1.SpireOIDCDiscoveryProviderConfig{
			Spec: v1alpha1.SpireOIDCDiscoveryProviderConfigSpec{
				CommonConfig: v1alpha1.CommonConfig{
					Labels: map[string]string{},
				},
			},
		}

		// Act
		result := generateSpireOIDCDiscoveryProviderSCC(config)

		// Assert
		assert.Empty(t, result.AllowedCapabilities)
		assert.Empty(t, result.DefaultAddCapabilities)
		assert.Empty(t, result.RequiredDropCapabilities)
		assert.NotNil(t, result.AllowedCapabilities)
		assert.NotNil(t, result.DefaultAddCapabilities)
		assert.NotNil(t, result.RequiredDropCapabilities)
	})

	t.Run("should set correct groups and seccomp profiles", func(t *testing.T) {
		// Arrange
		config := &v1alpha1.SpireOIDCDiscoveryProviderConfig{
			Spec: v1alpha1.SpireOIDCDiscoveryProviderConfigSpec{
				CommonConfig: v1alpha1.CommonConfig{
					Labels: map[string]string{},
				},
			},
		}

		// Act
		result := generateSpireOIDCDiscoveryProviderSCC(config)

		// Assert
		assert.Empty(t, result.Groups)
		assert.NotNil(t, result.Groups)

		expectedSeccompProfiles := []string{"*"}
		assert.Equal(t, expectedSeccompProfiles, result.SeccompProfiles)
	})

	t.Run("should handle nil config labels gracefully", func(t *testing.T) {
		// Arrange
		config := &v1alpha1.SpireOIDCDiscoveryProviderConfig{
			Spec: v1alpha1.SpireOIDCDiscoveryProviderConfigSpec{
				CommonConfig: v1alpha1.CommonConfig{
					Labels: nil,
				},
			},
		}

		// Act
		result := generateSpireOIDCDiscoveryProviderSCC(config)

		// Assert
		require.NotNil(t, result)

		// Should only have the managed-by label
		expectedLabels := map[string]string{
			utils.AppManagedByLabelKey: utils.AppManagedByLabelValue,
		}
		assert.Equal(t, expectedLabels, result.ObjectMeta.Labels)
	})

	t.Run("should preserve existing managed-by label if present", func(t *testing.T) {
		// Arrange
		config := &v1alpha1.SpireOIDCDiscoveryProviderConfig{
			Spec: v1alpha1.SpireOIDCDiscoveryProviderConfigSpec{
				CommonConfig: v1alpha1.CommonConfig{
					Labels: map[string]string{
						utils.AppManagedByLabelKey: "existing-value",
						"other-label":              "other-value",
					},
				},
			},
		}

		// Act
		result := generateSpireOIDCDiscoveryProviderSCC(config)

		// Assert
		// The function should overwrite the existing managed-by label
		expectedLabels := map[string]string{
			utils.AppManagedByLabelKey: utils.AppManagedByLabelValue,
			"other-label":              "other-value",
		}
		assert.Equal(t, expectedLabels, result.ObjectMeta.Labels)
	})

	t.Run("should return consistent results across multiple calls", func(t *testing.T) {
		// Arrange
		config := &v1alpha1.SpireOIDCDiscoveryProviderConfig{
			Spec: v1alpha1.SpireOIDCDiscoveryProviderConfigSpec{
				CommonConfig: v1alpha1.CommonConfig{
					Labels: map[string]string{
						"test": "value",
					},
				},
			},
		}

		// Act
		result1 := generateSpireOIDCDiscoveryProviderSCC(config)
		result2 := generateSpireOIDCDiscoveryProviderSCC(config)

		// Assert
		assert.Equal(t, result1.ObjectMeta.Name, result2.ObjectMeta.Name)
		assert.Equal(t, result1.ObjectMeta.Labels, result2.ObjectMeta.Labels)
		assert.Equal(t, result1.ReadOnlyRootFilesystem, result2.ReadOnlyRootFilesystem)
		assert.Equal(t, result1.Users, result2.Users)
		assert.Equal(t, result1.Volumes, result2.Volumes)
		assert.Equal(t, result1.AllowPrivilegedContainer, result2.AllowPrivilegedContainer)
	})
}

// Table-driven test for different label scenarios
func TestGenerateSpireOIDCDiscoveryProviderSCC_LabelScenarios(t *testing.T) {
	testCases := []struct {
		name           string
		inputLabels    map[string]string
		expectedLabels map[string]string
	}{
		{
			name:        "empty labels map",
			inputLabels: map[string]string{},
			expectedLabels: map[string]string{
				utils.AppManagedByLabelKey: utils.AppManagedByLabelValue,
			},
		},
		{
			name:        "nil labels map",
			inputLabels: nil,
			expectedLabels: map[string]string{
				utils.AppManagedByLabelKey: utils.AppManagedByLabelValue,
			},
		},
		{
			name: "single custom label",
			inputLabels: map[string]string{
				"app": "spire",
			},
			expectedLabels: map[string]string{
				"app":                      "spire",
				utils.AppManagedByLabelKey: utils.AppManagedByLabelValue,
			},
		},
		{
			name: "multiple custom labels",
			inputLabels: map[string]string{
				"app":       "spire",
				"version":   "1.0",
				"component": "oidc",
			},
			expectedLabels: map[string]string{
				"app":                      "spire",
				"version":                  "1.0",
				"component":                "oidc",
				utils.AppManagedByLabelKey: utils.AppManagedByLabelValue,
			},
		},
		{
			name: "override managed-by label",
			inputLabels: map[string]string{
				utils.AppManagedByLabelKey: "original-value",
				"other":                    "label",
			},
			expectedLabels: map[string]string{
				utils.AppManagedByLabelKey: utils.AppManagedByLabelValue,
				"other":                    "label",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			config := &v1alpha1.SpireOIDCDiscoveryProviderConfig{
				Spec: v1alpha1.SpireOIDCDiscoveryProviderConfigSpec{
					CommonConfig: v1alpha1.CommonConfig{
						Labels: tc.inputLabels,
					},
				},
			}
			result := generateSpireOIDCDiscoveryProviderSCC(config)

			assert.Equal(t, tc.expectedLabels, result.ObjectMeta.Labels)
		})
	}
}

// Test to verify all security settings are correctly configured
func TestGenerateSpireOIDCDiscoveryProviderSCC_SecuritySettings(t *testing.T) {
	config := &v1alpha1.SpireOIDCDiscoveryProviderConfig{
		Spec: v1alpha1.SpireOIDCDiscoveryProviderConfigSpec{
			CommonConfig: v1alpha1.CommonConfig{
				Labels: map[string]string{},
			},
		},
	}

	result := generateSpireOIDCDiscoveryProviderSCC(config)

	t.Run("verify strategy types", func(t *testing.T) {
		assert.Equal(t, securityv1.RunAsUserStrategyRunAsAny, result.RunAsUser.Type)
		assert.Equal(t, securityv1.SELinuxStrategyRunAsAny, result.SELinuxContext.Type)
		assert.Equal(t, securityv1.SupplementalGroupsStrategyRunAsAny, result.SupplementalGroups.Type)
		assert.Equal(t, securityv1.FSGroupStrategyRunAsAny, result.FSGroup.Type)
	})

	t.Run("verify boolean flags", func(t *testing.T) {
		assert.True(t, result.ReadOnlyRootFilesystem)
		assert.True(t, result.AllowHostDirVolumePlugin)
		assert.True(t, result.AllowHostIPC)
		assert.True(t, result.AllowHostNetwork)
		assert.True(t, result.AllowHostPID)
		assert.True(t, result.AllowHostPorts)
		assert.True(t, result.AllowPrivilegedContainer)
	})

	t.Run("verify pointer fields", func(t *testing.T) {
		require.NotNil(t, result.AllowPrivilegeEscalation)
		assert.True(t, *result.AllowPrivilegeEscalation)
	})
}

// Test to ensure no mutation of input config
func TestGenerateSpireOIDCDiscoveryProviderSCC_NoMutation(t *testing.T) {
	originalLabels := map[string]string{
		"original": "value",
	}
	config := &v1alpha1.SpireOIDCDiscoveryProviderConfig{
		Spec: v1alpha1.SpireOIDCDiscoveryProviderConfigSpec{
			CommonConfig: v1alpha1.CommonConfig{
				Labels: originalLabels,
			},
		},
	}

	// Act
	_ = generateSpireOIDCDiscoveryProviderSCC(config)

	// Assert - original config should not be modified
	assert.Equal(t, map[string]string{"original": "value"}, config.Spec.Labels)
	assert.Len(t, config.Spec.Labels, 1) // Should still have only one label
}
