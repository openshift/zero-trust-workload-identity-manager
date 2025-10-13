package utils

import (
	"github.com/openshift/zero-trust-workload-identity-manager/pkg/version"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

const (
	// New standardized label values
	StandardManagedByValue = "zero-trust-workload-identity-manager"
	StandardPartOfValue    = "zero-trust-workload-identity-manager"
	StandardInstance       = "cluster-zero-trust-workload-identity-manager"

	// Label keys
	AppComponentLabelKey = "app.kubernetes.io/component"

	// Component values
	ComponentCSI          = "csi"
	ComponentControlPlane = "control-plane"
	ComponentNodeAgent    = "node-agent"
	ComponentDiscovery    = "discovery"
)

// StandardizedLabels generates the new standardized label set for Kubernetes resources
func StandardizedLabels(name, component, version string, customLabels map[string]string) map[string]string {
	labels := make(map[string]string)

	// Add custom labels first (for non-standard labels like security.openshift.io/*)
	for k, v := range customLabels {
		labels[k] = v
	}

	// Then add standardized labels (these will override any conflicting custom labels)
	labels["app.kubernetes.io/name"] = name
	labels["app.kubernetes.io/instance"] = StandardInstance
	labels["app.kubernetes.io/part-of"] = StandardPartOfValue
	labels["app.kubernetes.io/component"] = component
	labels["app.kubernetes.io/managed-by"] = StandardManagedByValue
	labels["app.kubernetes.io/version"] = version

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
	return StandardizedLabels("spire-controller-manager", ComponentControlPlane, version.SpireControllerManagerVersion, customLabels)
}

// hasControllerManagedLabelWithComponent checks if an object has both the managed-by label
// and the specified component label
func hasControllerManagedLabelWithComponent(obj client.Object, component string) bool {
	labels := obj.GetLabels()
	if labels == nil {
		return false
	}
	managedByVal, hasManagedBy := labels[AppManagedByLabelKey]
	componentVal, hasComponent := labels[AppComponentLabelKey]
	return hasManagedBy && managedByVal == AppManagedByLabelValue &&
		hasComponent && componentVal == component
}

// ControllerManagedResourcesForComponent creates a predicate that filters resources by both
// the managed-by label and the component label
func ControllerManagedResourcesForComponent(component string) predicate.Funcs {
	return predicate.Funcs{
		UpdateFunc: func(e event.UpdateEvent) bool {
			return hasControllerManagedLabelWithComponent(e.ObjectNew, component)
		},
		CreateFunc: func(e event.CreateEvent) bool {
			return hasControllerManagedLabelWithComponent(e.Object, component)
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			return hasControllerManagedLabelWithComponent(e.Object, component)
		},
	}
}
