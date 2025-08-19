package featuregate

import (
	"os"
	"testing"
)

func TestInitializeFeatureGates(t *testing.T) {
	// Clean environment and state
	os.Unsetenv(UnsupportedAddonFeaturesEnv)

	tests := []struct {
		name        string
		flagsStr    string
		envVal      string
		expected    map[string]bool
		expectError bool
	}{
		{
			name:     "empty flags",
			flagsStr: "",
			expected: map[string]bool{},
		},
		{
			name:     "single feature enabled",
			flagsStr: "PauseReconciliation=true",
			expected: map[string]bool{"PauseReconciliation": true},
		},
		{
			name:     "single feature disabled",
			flagsStr: "PauseReconciliation=false",
			expected: map[string]bool{"PauseReconciliation": false},
		},
		{
			name:     "multiple features",
			flagsStr: "DISABLE_AUTO_RECONCILE=true,TEST_FEATURE_A=false,TEST_FEATURE_B=true",
			expected: map[string]bool{"DISABLE_AUTO_RECONCILE": true, "TEST_FEATURE_A": false, "TEST_FEATURE_B": true},
		},
		{
			name:     "whitespace handling",
			flagsStr: " DISABLE_AUTO_RECONCILE = true , TEST_FEATURE_A = false ",
			expected: map[string]bool{"DISABLE_AUTO_RECONCILE": true, "TEST_FEATURE_A": false},
		},
		{
			name:     "env variable takes precedence",
			flagsStr: "PauseReconciliation=false",
			envVal:   "PauseReconciliation=true",
			expected: map[string]bool{"PauseReconciliation": true},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup environment
			if tt.envVal != "" {
				os.Setenv(UnsupportedAddonFeaturesEnv, tt.envVal)
				defer os.Unsetenv(UnsupportedAddonFeaturesEnv)
			}

			err := InitializeFeatureGates(tt.flagsStr)
			if (err != nil) != tt.expectError {
				t.Errorf("InitializeFeatureGates() error = %v, expectError %v", err, tt.expectError)
				return
			}

			gates := GetFeatureGates()
			if len(gates) != len(tt.expected) {
				t.Errorf("Expected %d gates, got %d", len(tt.expected), len(gates))
				return
			}

			for featureName, expectedValue := range tt.expected {
				if actualValue, exists := gates[featureName]; !exists {
					t.Errorf("Feature gate %s not found", featureName)
				} else if actualValue != expectedValue {
					t.Errorf("Feature gate %s = %v, expected %v", featureName, actualValue, expectedValue)
				}
			}
		})
	}
}

func TestIsFeatureGateEnabled(t *testing.T) {
	// Initialize with test data
	err := InitializeFeatureGates("PauseReconciliation=true,TestFeature=false")
	if err != nil {
		t.Fatalf("Failed to initialize feature gates: %v", err)
	}

	tests := []struct {
		name     string
		feature  string
		expected bool
	}{
		{
			name:     "enabled feature",
			feature:  "PauseReconciliation",
			expected: true,
		},
		{
			name:     "disabled feature",
			feature:  "TestFeature",
			expected: false,
		},
		{
			name:     "non-existent feature",
			feature:  "NonExistentFeature",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsFeatureGateEnabled(tt.feature)
			if result != tt.expected {
				t.Errorf("IsFeatureGateEnabled(%s) = %v, expected %v", tt.feature, result, tt.expected)
			}
		})
	}
}

func TestIsAutoReconcileDisabled(t *testing.T) {
	tests := []struct {
		name             string
		featureGateFlags string
		expectedDisabled bool
	}{
		{
			name:             "no feature gates",
			expectedDisabled: false,
		},
		{
			name:             "direct feature gate enabled",
			featureGateFlags: "DISABLE_AUTO_RECONCILE=true",
			expectedDisabled: true,
		},
		{
			name:             "direct feature gate disabled",
			featureGateFlags: "DISABLE_AUTO_RECONCILE=false",
			expectedDisabled: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Initialize feature gates
			err := InitializeFeatureGates(tt.featureGateFlags)
			if err != nil {
				t.Fatalf("Failed to initialize feature gates: %v", err)
			}

			result := IsAutoReconcileDisabled()
			if result != tt.expectedDisabled {
				t.Errorf("IsAutoReconcileDisabled() = %v, expected %v", result, tt.expectedDisabled)
			}
		})
	}
}

func TestMultipleFeatureUtilities(t *testing.T) {
	// Test GetEnabledFeatures, GetDisabledFeatures, and AreAnyFeaturesEnabled
	err := InitializeFeatureGates("DISABLE_AUTO_RECONCILE=true,TEST_FEATURE_A=false,TEST_FEATURE_B=true")
	if err != nil {
		t.Fatalf("Failed to initialize feature gates: %v", err)
	}

	// Test GetEnabledFeatures
	enabledFeatures := GetEnabledFeatures()
	expectedEnabled := []string{"DISABLE_AUTO_RECONCILE", "TEST_FEATURE_B"}

	if len(enabledFeatures) != len(expectedEnabled) {
		t.Errorf("GetEnabledFeatures() returned %d features, expected %d", len(enabledFeatures), len(expectedEnabled))
	}

	for _, expected := range expectedEnabled {
		found := false
		for _, enabled := range enabledFeatures {
			if enabled == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected feature %s to be in enabled features list", expected)
		}
	}

	// Test GetDisabledFeatures
	disabledFeatures := GetDisabledFeatures()
	expectedDisabled := []string{"TEST_FEATURE_A"}

	if len(disabledFeatures) != len(expectedDisabled) {
		t.Errorf("GetDisabledFeatures() returned %d features, expected %d", len(disabledFeatures), len(expectedDisabled))
	}

	for _, expected := range expectedDisabled {
		found := false
		for _, disabled := range disabledFeatures {
			if disabled == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected feature %s to be in disabled features list", expected)
		}
	}

	// Test AreAnyFeaturesEnabled
	if !AreAnyFeaturesEnabled() {
		t.Error("AreAnyFeaturesEnabled() should return true when features are enabled")
	}

	// Test with no features enabled
	err = InitializeFeatureGates("TEST_FEATURE_A=false,TEST_FEATURE_B=false")
	if err != nil {
		t.Fatalf("Failed to initialize feature gates: %v", err)
	}

	if AreAnyFeaturesEnabled() {
		t.Error("AreAnyFeaturesEnabled() should return false when no features are enabled")
	}

	// Test with empty feature gates
	err = InitializeFeatureGates("")
	if err != nil {
		t.Fatalf("Failed to initialize feature gates: %v", err)
	}

	if AreAnyFeaturesEnabled() {
		t.Error("AreAnyFeaturesEnabled() should return false when no feature gates are set")
	}
}
