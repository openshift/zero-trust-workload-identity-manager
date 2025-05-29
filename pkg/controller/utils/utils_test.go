package utils

import (
	"os"
	"reflect"
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

// Helper function to set environment variable and return cleanup function
func setEnvVar(key, value string) func() {
	original := os.Getenv(key)
	os.Setenv(key, value)
	return func() {
		if original == "" {
			os.Unsetenv(key)
		} else {
			os.Setenv(key, original)
		}
	}
}

func TestGetSpireServerImage(t *testing.T) {
	tests := []struct {
		name     string
		envValue string
		expected string
	}{
		{
			name:     "returns image when environment variable is set",
			envValue: "spire-server:v1.2.3",
			expected: "spire-server:v1.2.3",
		},
		{
			name:     "returns empty string when environment variable is empty",
			envValue: "",
			expected: "",
		},
		{
			name:     "returns image with registry and tag",
			envValue: "registry.example.com/spire-server:latest",
			expected: "registry.example.com/spire-server:latest",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cleanup := setEnvVar(SpireServerImageEnv, tt.envValue)
			defer cleanup()

			result := GetSpireServerImage()
			if result != tt.expected {
				t.Errorf("GetSpireServerImage() = %q, want %q", result, tt.expected)
			}
		})
	}

	// Test when environment variable is not set at all
	t.Run("returns empty string when environment variable is not set", func(t *testing.T) {
		os.Unsetenv(SpireServerImageEnv)
		result := GetSpireServerImage()
		if result != "" {
			t.Errorf("GetSpireServerImage() = %q, want empty string", result)
		}
	})
}

func TestGetSpireAgentImage(t *testing.T) {
	tests := []struct {
		name     string
		envValue string
		expected string
	}{
		{
			name:     "returns image when environment variable is set",
			envValue: "spire-agent:v1.2.3",
			expected: "spire-agent:v1.2.3",
		},
		{
			name:     "returns empty string when environment variable is empty",
			envValue: "",
			expected: "",
		},
		{
			name:     "returns image with registry and tag",
			envValue: "registry.example.com/spire-agent:latest",
			expected: "registry.example.com/spire-agent:latest",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cleanup := setEnvVar(SpireAgentImageEnv, tt.envValue)
			defer cleanup()

			result := GetSpireAgentImage()
			if result != tt.expected {
				t.Errorf("GetSpireAgentImage() = %q, want %q", result, tt.expected)
			}
		})
	}

	t.Run("returns empty string when environment variable is not set", func(t *testing.T) {
		os.Unsetenv(SpireAgentImageEnv)
		result := GetSpireAgentImage()
		if result != "" {
			t.Errorf("GetSpireAgentImage() = %q, want empty string", result)
		}
	})
}

func TestGetSpiffeCSIDriverImage(t *testing.T) {
	tests := []struct {
		name     string
		envValue string
		expected string
	}{
		{
			name:     "returns image when environment variable is set",
			envValue: "spiffe-csi-driver:v0.2.3",
			expected: "spiffe-csi-driver:v0.2.3",
		},
		{
			name:     "returns empty string when environment variable is empty",
			envValue: "",
			expected: "",
		},
		{
			name:     "returns image with registry and tag",
			envValue: "gcr.io/spiffe-io/spiffe-csi-driver:latest",
			expected: "gcr.io/spiffe-io/spiffe-csi-driver:latest",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cleanup := setEnvVar(SpiffeCSIDriverImageEnv, tt.envValue)
			defer cleanup()

			result := GetSpiffeCSIDriverImage()
			if result != tt.expected {
				t.Errorf("GetSpiffeCSIDriverImage() = %q, want %q", result, tt.expected)
			}
		})
	}

	t.Run("returns empty string when environment variable is not set", func(t *testing.T) {
		os.Unsetenv(SpiffeCSIDriverImageEnv)
		result := GetSpiffeCSIDriverImage()
		if result != "" {
			t.Errorf("GetSpiffeCSIDriverImage() = %q, want empty string", result)
		}
	})
}

func TestGetSpireControllerManagerImage(t *testing.T) {
	tests := []struct {
		name     string
		envValue string
		expected string
	}{
		{
			name:     "returns image when environment variable is set",
			envValue: "spire-controller-manager:v0.13.0",
			expected: "spire-controller-manager:v0.13.0",
		},
		{
			name:     "returns empty string when environment variable is empty",
			envValue: "",
			expected: "",
		},
		{
			name:     "returns image with registry and tag",
			envValue: "ghcr.io/spiffe/spire-controller-manager:nightly",
			expected: "ghcr.io/spiffe/spire-controller-manager:nightly",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cleanup := setEnvVar(SpireControllerManagerImageEnv, tt.envValue)
			defer cleanup()

			result := GetSpireControllerManagerImage()
			if result != tt.expected {
				t.Errorf("GetSpireControllerManagerImage() = %q, want %q", result, tt.expected)
			}
		})
	}

	t.Run("returns empty string when environment variable is not set", func(t *testing.T) {
		os.Unsetenv(SpireControllerManagerImageEnv)
		result := GetSpireControllerManagerImage()
		if result != "" {
			t.Errorf("GetSpireControllerManagerImage() = %q, want empty string", result)
		}
	})
}

func TestGetSpireOIDCDiscoveryProviderImage(t *testing.T) {
	tests := []struct {
		name     string
		envValue string
		expected string
	}{
		{
			name:     "returns image when environment variable is set",
			envValue: "oidc-discovery-provider:v1.9.0",
			expected: "oidc-discovery-provider:v1.9.0",
		},
		{
			name:     "returns empty string when environment variable is empty",
			envValue: "",
			expected: "",
		},
		{
			name:     "returns image with registry and tag",
			envValue: "ghcr.io/spiffe/oidc-discovery-provider:latest",
			expected: "ghcr.io/spiffe/oidc-discovery-provider:latest",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cleanup := setEnvVar(SpireOIDCDiscoveryProviderImageEnv, tt.envValue)
			defer cleanup()

			result := GetSpireOIDCDiscoveryProviderImage()
			if result != tt.expected {
				t.Errorf("GetSpireOIDCDiscoveryProviderImage() = %q, want %q", result, tt.expected)
			}
		})
	}

	t.Run("returns empty string when environment variable is not set", func(t *testing.T) {
		os.Unsetenv(SpireOIDCDiscoveryProviderImageEnv)
		result := GetSpireOIDCDiscoveryProviderImage()
		if result != "" {
			t.Errorf("GetSpireOIDCDiscoveryProviderImage() = %q, want empty string", result)
		}
	})
}

func TestGetSpiffeHelperImage(t *testing.T) {
	tests := []struct {
		name     string
		envValue string
		expected string
	}{
		{
			name:     "returns image when environment variable is set",
			envValue: "spiffe-helper:v0.8.0",
			expected: "spiffe-helper:v0.8.0",
		},
		{
			name:     "returns empty string when environment variable is empty",
			envValue: "",
			expected: "",
		},
		{
			name:     "returns image with registry and tag",
			envValue: "ghcr.io/spiffe/spiffe-helper:main",
			expected: "ghcr.io/spiffe/spiffe-helper:main",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cleanup := setEnvVar(SpiffeHelperImageEnv, tt.envValue)
			defer cleanup()

			result := GetSpiffeHelperImage()
			if result != tt.expected {
				t.Errorf("GetSpiffeHelperImage() = %q, want %q", result, tt.expected)
			}
		})
	}

	t.Run("returns empty string when environment variable is not set", func(t *testing.T) {
		os.Unsetenv(SpiffeHelperImageEnv)
		result := GetSpiffeHelperImage()
		if result != "" {
			t.Errorf("GetSpiffeHelperImage() = %q, want empty string", result)
		}
	})
}

func TestGetNodeDriverRegistrarImage(t *testing.T) {
	tests := []struct {
		name     string
		envValue string
		expected string
	}{
		{
			name:     "returns image when environment variable is set",
			envValue: "node-driver-registrar:v2.8.0",
			expected: "node-driver-registrar:v2.8.0",
		},
		{
			name:     "returns empty string when environment variable is empty",
			envValue: "",
			expected: "",
		},
		{
			name:     "returns image with registry and tag",
			envValue: "registry.k8s.io/sig-storage/csi-node-driver-registrar:v2.8.0",
			expected: "registry.k8s.io/sig-storage/csi-node-driver-registrar:v2.8.0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cleanup := setEnvVar(NodeDriverRegistrarImageEnv, tt.envValue)
			defer cleanup()

			result := GetNodeDriverRegistrarImage()
			if result != tt.expected {
				t.Errorf("GetNodeDriverRegistrarImage() = %q, want %q", result, tt.expected)
			}
		})
	}

	t.Run("returns empty string when environment variable is not set", func(t *testing.T) {
		os.Unsetenv(NodeDriverRegistrarImageEnv)
		result := GetNodeDriverRegistrarImage()
		if result != "" {
			t.Errorf("GetNodeDriverRegistrarImage() = %q, want empty string", result)
		}
	})
}

func TestStringToBool(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{
			name:     "true string returns true",
			input:    "true",
			expected: true,
		},
		{
			name:     "false string returns false",
			input:    "false",
			expected: false,
		},
		{
			name:     "empty string returns false",
			input:    "",
			expected: false,
		},
		{
			name:     "True (capitalized) returns false",
			input:    "True",
			expected: false,
		},
		{
			name:     "TRUE (uppercase) returns false",
			input:    "TRUE",
			expected: false,
		},
		{
			name:     "random string returns false",
			input:    "random",
			expected: false,
		},
		{
			name:     "1 returns false",
			input:    "1",
			expected: false,
		},
		{
			name:     "0 returns false",
			input:    "0",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := StringToBool(tt.input)
			if result != tt.expected {
				t.Errorf("StringToBool(%q) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestDerefResourceRequirements(t *testing.T) {
	tests := []struct {
		name     string
		input    *corev1.ResourceRequirements
		expected corev1.ResourceRequirements
	}{
		{
			name:     "nil pointer returns empty ResourceRequirements",
			input:    nil,
			expected: corev1.ResourceRequirements{},
		},
		{
			name: "valid pointer returns dereferenced value",
			input: &corev1.ResourceRequirements{
				Limits: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("100m"),
					corev1.ResourceMemory: resource.MustParse("128Mi"),
				},
				Requests: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("50m"),
					corev1.ResourceMemory: resource.MustParse("64Mi"),
				},
			},
			expected: corev1.ResourceRequirements{
				Limits: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("100m"),
					corev1.ResourceMemory: resource.MustParse("128Mi"),
				},
				Requests: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("50m"),
					corev1.ResourceMemory: resource.MustParse("64Mi"),
				},
			},
		},
		{
			name:     "empty ResourceRequirements pointer returns empty value",
			input:    &corev1.ResourceRequirements{},
			expected: corev1.ResourceRequirements{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := DerefResourceRequirements(tt.input)
			if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("DerefResourceRequirements() = %+v, want %+v", result, tt.expected)
			}
		})
	}
}

func TestDerefAffinity(t *testing.T) {
	tests := []struct {
		name     string
		input    *corev1.Affinity
		expected corev1.Affinity
	}{
		{
			name:     "nil pointer returns empty Affinity",
			input:    nil,
			expected: corev1.Affinity{},
		},
		{
			name: "valid pointer returns dereferenced value",
			input: &corev1.Affinity{
				NodeAffinity: &corev1.NodeAffinity{
					RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
						NodeSelectorTerms: []corev1.NodeSelectorTerm{
							{
								MatchExpressions: []corev1.NodeSelectorRequirement{
									{
										Key:      "kubernetes.io/arch",
										Operator: corev1.NodeSelectorOpIn,
										Values:   []string{"amd64"},
									},
								},
							},
						},
					},
				},
			},
			expected: corev1.Affinity{
				NodeAffinity: &corev1.NodeAffinity{
					RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
						NodeSelectorTerms: []corev1.NodeSelectorTerm{
							{
								MatchExpressions: []corev1.NodeSelectorRequirement{
									{
										Key:      "kubernetes.io/arch",
										Operator: corev1.NodeSelectorOpIn,
										Values:   []string{"amd64"},
									},
								},
							},
						},
					},
				},
			},
		},
		{
			name:     "empty Affinity pointer returns empty value",
			input:    &corev1.Affinity{},
			expected: corev1.Affinity{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := DerefAffinity(tt.input)
			if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("DerefAffinity() = %+v, want %+v", result, tt.expected)
			}
		})
	}
}

func TestDerefTolerations(t *testing.T) {
	tests := []struct {
		name     string
		input    []*corev1.Toleration
		expected []corev1.Toleration
	}{
		{
			name:     "nil slice returns empty slice",
			input:    nil,
			expected: []corev1.Toleration{},
		},
		{
			name:     "empty slice returns empty slice",
			input:    []*corev1.Toleration{},
			expected: []corev1.Toleration{},
		},
		{
			name: "slice with valid pointers returns dereferenced values",
			input: []*corev1.Toleration{
				{
					Key:      "key1",
					Operator: corev1.TolerationOpEqual,
					Value:    "value1",
					Effect:   corev1.TaintEffectNoSchedule,
				},
				{
					Key:      "key2",
					Operator: corev1.TolerationOpExists,
					Effect:   corev1.TaintEffectPreferNoSchedule,
				},
			},
			expected: []corev1.Toleration{
				{
					Key:      "key1",
					Operator: corev1.TolerationOpEqual,
					Value:    "value1",
					Effect:   corev1.TaintEffectNoSchedule,
				},
				{
					Key:      "key2",
					Operator: corev1.TolerationOpExists,
					Effect:   corev1.TaintEffectPreferNoSchedule,
				},
			},
		},
		{
			name: "slice with nil pointers filters them out",
			input: []*corev1.Toleration{
				{
					Key:      "key1",
					Operator: corev1.TolerationOpEqual,
					Value:    "value1",
					Effect:   corev1.TaintEffectNoSchedule,
				},
				nil,
				{
					Key:      "key2",
					Operator: corev1.TolerationOpExists,
					Effect:   corev1.TaintEffectPreferNoSchedule,
				},
				nil,
			},
			expected: []corev1.Toleration{
				{
					Key:      "key1",
					Operator: corev1.TolerationOpEqual,
					Value:    "value1",
					Effect:   corev1.TaintEffectNoSchedule,
				},
				{
					Key:      "key2",
					Operator: corev1.TolerationOpExists,
					Effect:   corev1.TaintEffectPreferNoSchedule,
				},
			},
		},
		{
			name:     "slice with only nil pointers returns empty slice",
			input:    []*corev1.Toleration{nil, nil, nil},
			expected: []corev1.Toleration{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := DerefTolerations(tt.input)
			if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("DerefTolerations() = %+v, want %+v", result, tt.expected)
			}
		})
	}
}

func TestDerefNodeSelector(t *testing.T) {
	tests := []struct {
		name     string
		input    map[string]string
		expected map[string]string
	}{
		{
			name:     "nil map returns empty map",
			input:    nil,
			expected: map[string]string{},
		},
		{
			name:     "empty map returns empty map",
			input:    map[string]string{},
			expected: map[string]string{},
		},
		{
			name: "map with values returns copy",
			input: map[string]string{
				"kubernetes.io/arch": "amd64",
				"kubernetes.io/os":   "linux",
				"node-type":          "worker",
			},
			expected: map[string]string{
				"kubernetes.io/arch": "amd64",
				"kubernetes.io/os":   "linux",
				"node-type":          "worker",
			},
		},
		{
			name: "single key-value pair",
			input: map[string]string{
				"zone": "us-west-1a",
			},
			expected: map[string]string{
				"zone": "us-west-1a",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := DerefNodeSelector(tt.input)
			if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("DerefNodeSelector() = %+v, want %+v", result, tt.expected)
			}

			// Verify it's a copy (different memory address) when input is not nil
			if tt.input != nil && len(tt.input) > 0 {
				// Modify the result to ensure it doesn't affect the original
				result["test"] = "modification"
				if _, exists := tt.input["test"]; exists {
					t.Errorf("DerefNodeSelector() did not create a proper copy - original map was modified")
				}
			}
		})
	}
}
