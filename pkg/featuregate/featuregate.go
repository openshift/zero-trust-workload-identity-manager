package featuregate

import (
	"os"
	"strconv"
	"strings"
	"sync"
)

// Feature gate management
var (
	featureGates      = make(map[string]bool)
	featureGatesMutex sync.RWMutex
)

// InitializeFeatureGates initializes the feature gates from the provided flags string
// Format: "Feature1=true,Feature2=false"
func InitializeFeatureGates(flagsStr string) error {
	featureGatesMutex.Lock()
	defer featureGatesMutex.Unlock()

	// Clear existing gates
	featureGates = make(map[string]bool)

	if flagsStr == "" {
		return nil
	}

	// Parse environment variable if present
	if envVal := os.Getenv(UnsupportedAddonFeaturesEnv); envVal != "" {
		// Environment variable takes precedence
		flagsStr = envVal
	}

	features := strings.Split(flagsStr, ",")
	for _, feature := range features {
		feature = strings.TrimSpace(feature)
		if feature == "" {
			continue
		}

		parts := strings.SplitN(feature, "=", 2)
		if len(parts) != 2 {
			continue
		}

		featureName := strings.TrimSpace(parts[0])
		featureValueStr := strings.TrimSpace(parts[1])

		featureValue, err := strconv.ParseBool(featureValueStr)
		if err != nil {
			continue
		}

		featureGates[featureName] = featureValue
	}

	return nil
}

// IsFeatureGateEnabled checks if a specific feature gate is enabled
func IsFeatureGateEnabled(featureName string) bool {
	featureGatesMutex.RLock()
	defer featureGatesMutex.RUnlock()

	if enabled, exists := featureGates[featureName]; exists {
		return enabled
	}

	return false
}

// GetFeatureGates returns a copy of all feature gates
func GetFeatureGates() map[string]bool {
	featureGatesMutex.RLock()
	defer featureGatesMutex.RUnlock()

	gates := make(map[string]bool, len(featureGates))
	for k, v := range featureGates {
		gates[k] = v
	}

	return gates
}

// IsAutoReconcileDisabled returns true if the DISABLE_AUTO_RECONCILE feature gate is enabled
func IsAutoReconcileDisabled() bool {
	return IsFeatureGateEnabled(TechPreviewFeature)
}

// GetEnabledFeatures returns a slice of all enabled feature gate names
func GetEnabledFeatures() []string {
	featureGatesMutex.RLock()
	defer featureGatesMutex.RUnlock()

	var enabled []string
	for feature, isEnabled := range featureGates {
		if isEnabled {
			enabled = append(enabled, feature)
		}
	}
	return enabled
}

// GetDisabledFeatures returns a slice of all explicitly disabled feature gate names
func GetDisabledFeatures() []string {
	featureGatesMutex.RLock()
	defer featureGatesMutex.RUnlock()

	var disabled []string
	for feature, isEnabled := range featureGates {
		if !isEnabled {
			disabled = append(disabled, feature)
		}
	}
	return disabled
}

// AreAnyFeaturesEnabled returns true if any feature gates are enabled
func AreAnyFeaturesEnabled() bool {
	featureGatesMutex.RLock()
	defer featureGatesMutex.RUnlock()

	for _, isEnabled := range featureGates {
		if isEnabled {
			return true
		}
	}
	return false
}
