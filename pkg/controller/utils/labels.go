package utils

import (
	"github.com/openshift/zero-trust-workload-identity-manager/pkg/version"
)

const (
	// New standardized label values
	StandardManagedByValue = "zero-trust-workload-identity-manager"
	StandardPartOfValue    = "zero-trust-workload-identity-manager"
	StandardInstance       = "cluster-zero-trust-workload-identity-manager"

	// Component values
	ComponentOperator     = "operator"
	ComponentCSI          = "csi"
	ComponentControlPlane = "control-plane"
	ComponentNodeAgent    = "node-agent"
	ComponentDiscovery    = "discovery"
)

// StandardizedLabels generates the new standardized label set for Kubernetes resources
func StandardizedLabels(name, component, version string, customLabels map[string]string) map[string]string {
	labels := map[string]string{
		"app.kubernetes.io/name":       name,
		"app.kubernetes.io/instance":   StandardInstance,
		"app.kubernetes.io/part-of":    StandardPartOfValue,
		"app.kubernetes.io/component":  component,
		"app.kubernetes.io/managed-by": StandardManagedByValue,
		"app.kubernetes.io/version":    version,
	}

	// Add custom labels, allowing them to override defaults if needed
	for k, v := range customLabels {
		labels[k] = v
	}

	return labels
}

// Component-specific label generators
func SpireServerLabels(customLabels map[string]string) map[string]string {
	return StandardizedLabels("spire-server", ComponentControlPlane, version.SpireServerVersion, customLabels)
}

func SpireAgentLabels(customLabels map[string]string) map[string]string {
	return StandardizedLabels("spire-agent", ComponentNodeAgent, version.SpireAgentVersion, customLabels)
}

func SpireOIDCDiscoveryProviderLabels(customLabels map[string]string) map[string]string {
	return StandardizedLabels("spiffe-oidc-discovery-provider", ComponentDiscovery, version.SpireOIDCDiscoveryProviderVersion, customLabels)
}

func SpiffeCSIDriverLabels(customLabels map[string]string) map[string]string {
	return StandardizedLabels("spiffe-csi-driver", ComponentCSI, version.SpiffeCsiVersion, customLabels)
}

func SpireControllerManagerLabels(customLabels map[string]string) map[string]string {
	return StandardizedLabels("spire-controller-manager", ComponentOperator, version.SpireControllerManagerVersion, customLabels)
}
