package utils

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/event"
)

func TestStandardizedLabels(t *testing.T) {
	tests := []struct {
		name         string
		appName      string
		component    string
		version      string
		customLabels map[string]string
		checkKey     string
		checkValue   string
	}{
		{
			name:       "with nil custom labels",
			appName:    "spire-server",
			component:  ComponentControlPlane,
			version:    "1.0.0",
			checkKey:   "app.kubernetes.io/name",
			checkValue: "spire-server",
		},
		{
			name:         "with custom labels",
			appName:      "spire-server",
			component:    ComponentControlPlane,
			version:      "1.0.0",
			customLabels: map[string]string{"custom-label": "custom-value"},
			checkKey:     "custom-label",
			checkValue:   "custom-value",
		},
		{
			name:         "standard labels override custom",
			appName:      "spire-server",
			component:    ComponentControlPlane,
			version:      "1.0.0",
			customLabels: map[string]string{"app.kubernetes.io/name": "my-override"},
			checkKey:     "app.kubernetes.io/name",
			checkValue:   "spire-server",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			labels := StandardizedLabels(tt.appName, tt.component, tt.version, tt.customLabels)
			if labels == nil {
				t.Fatal("Expected non-nil labels")
			}
			if labels[tt.checkKey] != tt.checkValue {
				t.Errorf("Expected %s=%s, got %s", tt.checkKey, tt.checkValue, labels[tt.checkKey])
			}
		})
	}
}

func TestComponentLabelFunctions(t *testing.T) {
	tests := []struct {
		name              string
		labelFunc         func(map[string]string) map[string]string
		expectedComponent string
		expectedName      string
	}{
		{
			name:              "SpireServerLabels",
			labelFunc:         SpireServerLabels,
			expectedComponent: ComponentControlPlane,
			expectedName:      "spire-server",
		},
		{
			name:              "SpireAgentLabels",
			labelFunc:         SpireAgentLabels,
			expectedComponent: ComponentNodeAgent,
			expectedName:      "spire-agent",
		},
		{
			name:              "SpireOIDCDiscoveryProviderLabels",
			labelFunc:         SpireOIDCDiscoveryProviderLabels,
			expectedComponent: ComponentDiscovery,
			expectedName:      "spiffe-oidc-discovery-provider",
		},
		{
			name:              "SpiffeCSIDriverLabels",
			labelFunc:         SpiffeCSIDriverLabels,
			expectedComponent: ComponentCSI,
			expectedName:      "spiffe-csi-driver",
		},
		{
			name:              "SpireControllerManagerLabels",
			labelFunc:         SpireControllerManagerLabels,
			expectedComponent: ComponentControlPlane,
			expectedName:      "spire-controller-manager",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			labels := tt.labelFunc(nil)
			if labels == nil {
				t.Fatal("Expected non-nil labels")
			}
			if labels["app.kubernetes.io/component"] != tt.expectedComponent {
				t.Errorf("Expected component %s, got %s", tt.expectedComponent, labels["app.kubernetes.io/component"])
			}
			if labels["app.kubernetes.io/name"] != tt.expectedName {
				t.Errorf("Expected name %s, got %s", tt.expectedName, labels["app.kubernetes.io/name"])
			}
		})
	}
}

func TestControllerManagedResourcesForComponent(t *testing.T) {
	pred := ControllerManagedResourcesForComponent(ComponentControlPlane)

	tests := []struct {
		name     string
		labels   map[string]string
		expected bool
	}{
		{
			name: "matching labels",
			labels: map[string]string{
				AppManagedByLabelKey: AppManagedByLabelValue,
				AppComponentLabelKey: ComponentControlPlane,
			},
			expected: true,
		},
		{
			name: "non-matching component",
			labels: map[string]string{
				AppManagedByLabelKey: AppManagedByLabelValue,
				AppComponentLabelKey: ComponentNodeAgent,
			},
			expected: false,
		},
		{
			name:     "nil labels",
			labels:   nil,
			expected: false,
		},
		{
			name:     "missing managed-by",
			labels:   map[string]string{AppComponentLabelKey: ComponentControlPlane},
			expected: false,
		},
		{
			name: "wrong managed-by value",
			labels: map[string]string{
				AppManagedByLabelKey: "other-manager",
				AppComponentLabelKey: ComponentControlPlane,
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			obj := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Labels: tt.labels}}

			// Test all event types
			if pred.CreateFunc(event.CreateEvent{Object: obj}) != tt.expected {
				t.Errorf("CreateFunc: expected %v", tt.expected)
			}
			if pred.UpdateFunc(event.UpdateEvent{ObjectNew: obj}) != tt.expected {
				t.Errorf("UpdateFunc: expected %v", tt.expected)
			}
			if pred.DeleteFunc(event.DeleteEvent{Object: obj}) != tt.expected {
				t.Errorf("DeleteFunc: expected %v", tt.expected)
			}
		})
	}
}
