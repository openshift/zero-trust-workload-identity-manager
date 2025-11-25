package utils

import (
	"os"
	"reflect"
	"testing"

	"k8s.io/utils/ptr"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"

	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

func TestStatefulSetSpecModified(t *testing.T) {
	// Helper function to create a basic StatefulSet
	createStatefulSet := func() *appsv1.StatefulSet {
		return &appsv1.StatefulSet{
			Spec: appsv1.StatefulSetSpec{
				Replicas:    ptr.To(int32(3)),
				ServiceName: "test-service",
				Selector: &metav1.LabelSelector{
					MatchLabels: map[string]string{"app": "test"},
				},
				Template: corev1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{"app": "test"},
						Annotations: map[string]string{
							"kubectl.kubernetes.io/default-container":                 "main",
							"ztwim.openshift.io/spire-server-config-hash":             "hash1",
							"ztwim.openshift.io/spire-controller-manager-config-hash": "hash2",
						},
					},
					Spec: corev1.PodSpec{
						ServiceAccountName:    "test-sa",
						ShareProcessNamespace: ptr.To(false),
						NodeSelector:          map[string]string{"zone": "east"},
						Affinity: &corev1.Affinity{
							NodeAffinity: &corev1.NodeAffinity{
								RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
									NodeSelectorTerms: []corev1.NodeSelectorTerm{{
										MatchExpressions: []corev1.NodeSelectorRequirement{{
											Key:      "zone",
											Operator: corev1.NodeSelectorOpIn,
											Values:   []string{"east"},
										}},
									}},
								},
							},
						},
						Tolerations: []corev1.Toleration{{
							Key:      "node-type",
							Operator: corev1.TolerationOpEqual,
							Value:    "test",
							Effect:   corev1.TaintEffectNoSchedule,
						}},
						Containers: []corev1.Container{{
							Name:            "main",
							Image:           "nginx:1.20",
							ImagePullPolicy: corev1.PullAlways,
							Args:            []string{"--config", "/etc/config"},
							Env: []corev1.EnvVar{{
								Name:  "ENV_VAR",
								Value: "value",
							}},
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("100m"),
									corev1.ResourceMemory: resource.MustParse("128Mi"),
								},
							},
							VolumeMounts: []corev1.VolumeMount{{
								Name:      "config",
								MountPath: "/etc/config",
							}},
						}},
					},
				},
				VolumeClaimTemplates: []corev1.PersistentVolumeClaim{{
					ObjectMeta: metav1.ObjectMeta{
						Name: "data",
					},
					Spec: corev1.PersistentVolumeClaimSpec{
						AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
						Resources: corev1.VolumeResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceStorage: resource.MustParse("10Gi"),
							},
						},
					},
				}},
			},
		}
	}

	t.Run("Nil inputs", func(t *testing.T) {
		if !StatefulSetSpecModified(nil, nil) {
			t.Error("Expected true when both inputs are nil")
		}
		if !StatefulSetSpecModified(createStatefulSet(), nil) {
			t.Error("Expected true when fetched is nil")
		}
		if !StatefulSetSpecModified(nil, createStatefulSet()) {
			t.Error("Expected true when desired is nil")
		}
	})

	t.Run("No modifications", func(t *testing.T) {
		desired := createStatefulSet()
		fetched := createStatefulSet()
		if StatefulSetSpecModified(desired, fetched) {
			t.Error("Expected false when StatefulSets are identical")
		}
	})

	t.Run("Replicas modified", func(t *testing.T) {
		desired := createStatefulSet()
		fetched := createStatefulSet()
		fetched.Spec.Replicas = ptr.To(int32(5))
		if !StatefulSetSpecModified(desired, fetched) {
			t.Error("Expected true when replicas differ")
		}
	})

	t.Run("ServiceName modified", func(t *testing.T) {
		desired := createStatefulSet()
		fetched := createStatefulSet()
		fetched.Spec.ServiceName = "different-service"
		if !StatefulSetSpecModified(desired, fetched) {
			t.Error("Expected true when ServiceName differs")
		}
	})

	t.Run("Selector modified", func(t *testing.T) {
		desired := createStatefulSet()
		fetched := createStatefulSet()
		fetched.Spec.Selector.MatchLabels["app"] = "different"
		if !StatefulSetSpecModified(desired, fetched) {
			t.Error("Expected true when Selector differs")
		}
	})

	t.Run("Template labels modified", func(t *testing.T) {
		desired := createStatefulSet()
		fetched := createStatefulSet()
		fetched.Spec.Template.Labels["app"] = "different"
		if !StatefulSetSpecModified(desired, fetched) {
			t.Error("Expected true when Template.Labels differ")
		}
	})

	t.Run("Special annotations modified", func(t *testing.T) {
		desired := createStatefulSet()
		fetched := createStatefulSet()
		fetched.Spec.Template.Annotations["kubectl.kubernetes.io/default-container"] = "different"
		if !StatefulSetSpecModified(desired, fetched) {
			t.Error("Expected true when special annotation differs")
		}
	})

	t.Run("ServiceAccountName modified", func(t *testing.T) {
		desired := createStatefulSet()
		fetched := createStatefulSet()
		fetched.Spec.Template.Spec.ServiceAccountName = "different-sa"
		if !StatefulSetSpecModified(desired, fetched) {
			t.Error("Expected true when ServiceAccountName differs")
		}
	})

	t.Run("ShareProcessNamespace modified", func(t *testing.T) {
		desired := createStatefulSet()
		fetched := createStatefulSet()
		fetched.Spec.Template.Spec.ShareProcessNamespace = ptr.To(true)
		if !StatefulSetSpecModified(desired, fetched) {
			t.Error("Expected true when ShareProcessNamespace differs")
		}
	})

	t.Run("NodeSelector modified", func(t *testing.T) {
		desired := createStatefulSet()
		fetched := createStatefulSet()
		fetched.Spec.Template.Spec.NodeSelector["zone"] = "west"
		if !StatefulSetSpecModified(desired, fetched) {
			t.Error("Expected true when NodeSelector differs")
		}
	})

	t.Run("Affinity modified", func(t *testing.T) {
		desired := createStatefulSet()
		fetched := createStatefulSet()
		fetched.Spec.Template.Spec.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms[0].MatchExpressions[0].Values = []string{"west"}
		if !StatefulSetSpecModified(desired, fetched) {
			t.Error("Expected true when Affinity differs")
		}
	})

	t.Run("Tolerations modified", func(t *testing.T) {
		desired := createStatefulSet()
		fetched := createStatefulSet()
		fetched.Spec.Template.Spec.Tolerations[0].Value = "different"
		if !StatefulSetSpecModified(desired, fetched) {
			t.Error("Expected true when Tolerations differ")
		}
	})

	t.Run("Container count modified", func(t *testing.T) {
		desired := createStatefulSet()
		fetched := createStatefulSet()
		fetched.Spec.Template.Spec.Containers = append(fetched.Spec.Template.Spec.Containers, corev1.Container{
			Name:  "sidecar",
			Image: "sidecar:latest",
		})
		if !StatefulSetSpecModified(desired, fetched) {
			t.Error("Expected true when container count differs")
		}
	})

	t.Run("Container image modified", func(t *testing.T) {
		desired := createStatefulSet()
		fetched := createStatefulSet()
		fetched.Spec.Template.Spec.Containers[0].Image = "nginx:1.21"
		if !StatefulSetSpecModified(desired, fetched) {
			t.Error("Expected true when container image differs")
		}
	})

	t.Run("Container args modified", func(t *testing.T) {
		desired := createStatefulSet()
		fetched := createStatefulSet()
		fetched.Spec.Template.Spec.Containers[0].Args = []string{"--different", "arg"}
		if !StatefulSetSpecModified(desired, fetched) {
			t.Error("Expected true when container args differ")
		}
	})

	t.Run("VolumeClaimTemplates count modified", func(t *testing.T) {
		desired := createStatefulSet()
		fetched := createStatefulSet()
		fetched.Spec.VolumeClaimTemplates = append(fetched.Spec.VolumeClaimTemplates, corev1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{Name: "logs"},
		})
		if !StatefulSetSpecModified(desired, fetched) {
			t.Error("Expected true when VolumeClaimTemplates count differs")
		}
	})

	t.Run("VolumeClaimTemplate name modified", func(t *testing.T) {
		desired := createStatefulSet()
		fetched := createStatefulSet()
		fetched.Spec.VolumeClaimTemplates[0].Name = "different-data"
		if !StatefulSetSpecModified(desired, fetched) {
			t.Error("Expected true when VolumeClaimTemplate name differs")
		}
	})
}

func TestDeploymentSpecModified(t *testing.T) {
	// Helper function to create a basic Deployment
	createDeployment := func() *appsv1.Deployment {
		return &appsv1.Deployment{
			Spec: appsv1.DeploymentSpec{
				Replicas: ptr.To(int32(3)),
				Selector: &metav1.LabelSelector{
					MatchLabels: map[string]string{"app": "test"},
				},
				Template: corev1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{"app": "test"},
					},
					Spec: corev1.PodSpec{
						ServiceAccountName:    "test-sa",
						ShareProcessNamespace: ptr.To(false),
						NodeSelector:          map[string]string{"zone": "east"},
						Affinity: &corev1.Affinity{
							NodeAffinity: &corev1.NodeAffinity{
								RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
									NodeSelectorTerms: []corev1.NodeSelectorTerm{{
										MatchExpressions: []corev1.NodeSelectorRequirement{{
											Key:      "zone",
											Operator: corev1.NodeSelectorOpIn,
											Values:   []string{"east"},
										}},
									}},
								},
							},
						},
						Tolerations: []corev1.Toleration{{
							Key:      "node-type",
							Operator: corev1.TolerationOpEqual,
							Value:    "test",
							Effect:   corev1.TaintEffectNoSchedule,
						}},
						Containers: []corev1.Container{{
							Name:            "main",
							Image:           "nginx:1.20",
							ImagePullPolicy: corev1.PullAlways,
							Args:            []string{"--config", "/etc/config"},
							Env: []corev1.EnvVar{{
								Name:  "ENV_VAR",
								Value: "value",
							}},
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("100m"),
									corev1.ResourceMemory: resource.MustParse("128Mi"),
								},
							},
							VolumeMounts: []corev1.VolumeMount{{
								Name:      "config",
								MountPath: "/etc/config",
							}},
						}},
					},
				},
			},
		}
	}

	t.Run("Nil inputs", func(t *testing.T) {
		if !DeploymentSpecModified(nil, nil) {
			t.Error("Expected true when both inputs are nil")
		}
		if !DeploymentSpecModified(createDeployment(), nil) {
			t.Error("Expected true when fetched is nil")
		}
		if !DeploymentSpecModified(nil, createDeployment()) {
			t.Error("Expected true when desired is nil")
		}
	})

	t.Run("No modifications", func(t *testing.T) {
		desired := createDeployment()
		fetched := createDeployment()
		if DeploymentSpecModified(desired, fetched) {
			t.Error("Expected false when Deployments are identical")
		}
	})

	t.Run("Replicas modified", func(t *testing.T) {
		desired := createDeployment()
		fetched := createDeployment()
		fetched.Spec.Replicas = ptr.To(int32(5))
		if !DeploymentSpecModified(desired, fetched) {
			t.Error("Expected true when replicas differ")
		}
	})

	t.Run("Selector modified", func(t *testing.T) {
		desired := createDeployment()
		fetched := createDeployment()
		fetched.Spec.Selector.MatchLabels["app"] = "different"
		if !DeploymentSpecModified(desired, fetched) {
			t.Error("Expected true when Selector differs")
		}
	})

	t.Run("Container missing", func(t *testing.T) {
		desired := createDeployment()
		fetched := createDeployment()
		// Remove container from fetched to simulate missing container
		fetched.Spec.Template.Spec.Containers = []corev1.Container{}
		if !DeploymentSpecModified(desired, fetched) {
			t.Error("Expected true when container is missing")
		}
	})

	t.Run("ImagePullPolicy modified", func(t *testing.T) {
		desired := createDeployment()
		fetched := createDeployment()
		fetched.Spec.Template.Spec.Containers[0].ImagePullPolicy = corev1.PullNever
		if !DeploymentSpecModified(desired, fetched) {
			t.Error("Expected true when ImagePullPolicy differs")
		}
	})

	t.Run("Environment variables modified", func(t *testing.T) {
		desired := createDeployment()
		fetched := createDeployment()
		fetched.Spec.Template.Spec.Containers[0].Env[0].Value = "different-value"
		if !DeploymentSpecModified(desired, fetched) {
			t.Error("Expected true when environment variables differ")
		}
	})

	t.Run("Resources modified", func(t *testing.T) {
		desired := createDeployment()
		fetched := createDeployment()
		fetched.Spec.Template.Spec.Containers[0].Resources.Requests[corev1.ResourceCPU] = resource.MustParse("200m")
		if !DeploymentSpecModified(desired, fetched) {
			t.Error("Expected true when resources differ")
		}
	})

	t.Run("VolumeMounts modified", func(t *testing.T) {
		desired := createDeployment()
		fetched := createDeployment()
		fetched.Spec.Template.Spec.Containers[0].VolumeMounts[0].MountPath = "/different/path"
		if !DeploymentSpecModified(desired, fetched) {
			t.Error("Expected true when VolumeMounts differ")
		}
	})
}

func TestDaemonSetSpecModified(t *testing.T) {
	// Helper function to create a basic DaemonSet
	createDaemonSet := func() *appsv1.DaemonSet {
		return &appsv1.DaemonSet{
			Spec: appsv1.DaemonSetSpec{
				Selector: &metav1.LabelSelector{
					MatchLabels: map[string]string{"app": "test"},
				},
				Template: corev1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{"app": "test"},
					},
					Spec: corev1.PodSpec{
						ServiceAccountName:    "test-sa",
						ShareProcessNamespace: ptr.To(false),
						NodeSelector:          map[string]string{"zone": "east"},
						Affinity: &corev1.Affinity{
							NodeAffinity: &corev1.NodeAffinity{
								RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
									NodeSelectorTerms: []corev1.NodeSelectorTerm{{
										MatchExpressions: []corev1.NodeSelectorRequirement{{
											Key:      "zone",
											Operator: corev1.NodeSelectorOpIn,
											Values:   []string{"east"},
										}},
									}},
								},
							},
						},
						Tolerations: []corev1.Toleration{{
							Key:      "node-type",
							Operator: corev1.TolerationOpEqual,
							Value:    "test",
							Effect:   corev1.TaintEffectNoSchedule,
						}},
						Containers: []corev1.Container{{
							Name:            "main",
							Image:           "nginx:1.20",
							ImagePullPolicy: corev1.PullAlways,
							Args:            []string{"--config", "/etc/config"},
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("100m"),
									corev1.ResourceMemory: resource.MustParse("128Mi"),
								},
							},
							VolumeMounts: []corev1.VolumeMount{{
								Name:      "config",
								MountPath: "/etc/config",
							}},
						}},
					},
				},
			},
		}
	}

	t.Run("Nil inputs", func(t *testing.T) {
		if !DaemonSetSpecModified(nil, nil) {
			t.Error("Expected true when both inputs are nil")
		}
		if !DaemonSetSpecModified(createDaemonSet(), nil) {
			t.Error("Expected true when fetched is nil")
		}
		if !DaemonSetSpecModified(nil, createDaemonSet()) {
			t.Error("Expected true when desired is nil")
		}
	})

	t.Run("No modifications", func(t *testing.T) {
		desired := createDaemonSet()
		fetched := createDaemonSet()
		if DaemonSetSpecModified(desired, fetched) {
			t.Error("Expected false when DaemonSets are identical")
		}
	})

	t.Run("Selector modified", func(t *testing.T) {
		desired := createDaemonSet()
		fetched := createDaemonSet()
		fetched.Spec.Selector.MatchLabels["app"] = "different"
		if !DaemonSetSpecModified(desired, fetched) {
			t.Error("Expected true when Selector differs")
		}
	})

	t.Run("Template labels modified", func(t *testing.T) {
		desired := createDaemonSet()
		fetched := createDaemonSet()
		fetched.Spec.Template.Labels["app"] = "different"
		if !DaemonSetSpecModified(desired, fetched) {
			t.Error("Expected true when Template.Labels differ")
		}
	})

	t.Run("Container missing", func(t *testing.T) {
		desired := createDaemonSet()
		fetched := createDaemonSet()
		// Add extra container to desired to test missing container scenario
		desired.Spec.Template.Spec.Containers = append(desired.Spec.Template.Spec.Containers, corev1.Container{
			Name:  "sidecar",
			Image: "sidecar:latest",
		})
		if !DaemonSetSpecModified(desired, fetched) {
			t.Error("Expected true when container is missing in fetched")
		}
	})

	t.Run("Container image modified", func(t *testing.T) {
		desired := createDaemonSet()
		fetched := createDaemonSet()
		fetched.Spec.Template.Spec.Containers[0].Image = "nginx:1.21"
		if !DaemonSetSpecModified(desired, fetched) {
			t.Error("Expected true when container image differs")
		}
	})

	t.Run("Container resources modified", func(t *testing.T) {
		desired := createDaemonSet()
		fetched := createDaemonSet()
		fetched.Spec.Template.Spec.Containers[0].Resources.Requests[corev1.ResourceMemory] = resource.MustParse("256Mi")
		if !DaemonSetSpecModified(desired, fetched) {
			t.Error("Expected true when container resources differ")
		}
	})

	// Test the bug in the original code where Tolerations check uses NodeSelector length
	t.Run("Tolerations check with empty NodeSelector", func(t *testing.T) {
		desired := createDaemonSet()
		fetched := createDaemonSet()

		// Set NodeSelector to empty but keep Tolerations
		desired.Spec.Template.Spec.NodeSelector = map[string]string{}
		fetched.Spec.Template.Spec.NodeSelector = map[string]string{}

		// Modify tolerations
		fetched.Spec.Template.Spec.Tolerations[0].Value = "different"

		// Due to the bug in the original code, this should return false
		// because len(desired.Spec.Template.Spec.NodeSelector) == 0
		if DaemonSetSpecModified(desired, fetched) {
			t.Error("Expected false due to bug in original code - Tolerations check uses NodeSelector length")
		}
	})
}

// Edge case tests
func TestEdgeCases(t *testing.T) {
	t.Run("StatefulSet with nil replicas", func(t *testing.T) {
		desired := &appsv1.StatefulSet{
			Spec: appsv1.StatefulSetSpec{
				Replicas: nil,
				Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "test"}},
				Template: corev1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"app": "test"}},
					Spec:       corev1.PodSpec{Containers: []corev1.Container{{Name: "test", Image: "test"}}},
				},
			},
		}
		fetched := &appsv1.StatefulSet{
			Spec: appsv1.StatefulSetSpec{
				Replicas: ptr.To(int32(3)),
				Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "test"}},
				Template: corev1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"app": "test"}},
					Spec:       corev1.PodSpec{Containers: []corev1.Container{{Name: "test", Image: "test"}}},
				},
			},
		}
		// Should not trigger modification since desired.Replicas is nil
		if StatefulSetSpecModified(desired, fetched) {
			t.Error("Expected false when desired replicas is nil")
		}
	})

	t.Run("Empty NodeSelector and Affinity", func(t *testing.T) {
		desired := &appsv1.Deployment{
			Spec: appsv1.DeploymentSpec{
				Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "test"}},
				Template: corev1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"app": "test"}},
					Spec: corev1.PodSpec{
						NodeSelector: map[string]string{}, // Empty but not nil
						Affinity:     nil,                 // Nil
						Containers:   []corev1.Container{{Name: "test", Image: "test"}},
					},
				},
			},
		}
		fetched := &appsv1.Deployment{
			Spec: appsv1.DeploymentSpec{
				Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "test"}},
				Template: corev1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"app": "test"}},
					Spec: corev1.PodSpec{
						NodeSelector: map[string]string{"zone": "east"}, // Different
						Affinity:     nil,
						Containers:   []corev1.Container{{Name: "test", Image: "test"}},
					},
				},
			},
		}
		// Should not trigger modification since desired NodeSelector is empty (len == 0)
		if DeploymentSpecModified(desired, fetched) {
			t.Error("Expected false when desired NodeSelector is empty")
		}
	})
}

func TestGetLogLevelFromString(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "returns default when empty string",
			input:    "",
			expected: LogLevelInfo,
		},
		{
			name:     "returns input when non-empty string",
			input:    "debug",
			expected: "debug",
		},
		{
			name:     "returns input for info level",
			input:    "info",
			expected: "info",
		},
		{
			name:     "returns input for error level",
			input:    "error",
			expected: "error",
		},
		{
			name:     "returns input for warn level",
			input:    "warn",
			expected: "warn",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetLogLevelFromString(tt.input)
			if result != tt.expected {
				t.Errorf("GetLogLevelFromString(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestGetLogFormatFromString(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "returns default when empty string",
			input:    "",
			expected: LogFormatText,
		},
		{
			name:     "returns input when non-empty string",
			input:    "json",
			expected: "json",
		},
		{
			name:     "returns input for text format",
			input:    "text",
			expected: "text",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetLogFormatFromString(tt.input)
			if result != tt.expected {
				t.Errorf("GetLogFormatFromString(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestGetOperatorNamespace(t *testing.T) {
	tests := []struct {
		name     string
		envValue string
		expected string
	}{
		{
			name:     "returns custom namespace when environment variable is set",
			envValue: "custom-namespace",
			expected: "custom-namespace",
		},
		{
			name:     "returns empty string when environment variable is empty",
			envValue: "",
			expected: "",
		},
		{
			name:     "returns namespace with hyphens and special characters",
			envValue: "my-custom-operator-namespace-123",
			expected: "my-custom-operator-namespace-123",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cleanup := setEnvVar("OPERATOR_NAMESPACE", tt.envValue)
			defer cleanup()

			result := GetOperatorNamespace()
			if result != tt.expected {
				t.Errorf("GetOperatorNamespace() = %q, want %q", result, tt.expected)
			}
		})
	}

	// Test when environment variable is not set at all
	t.Run("returns empty string when environment variable is not set", func(t *testing.T) {
		os.Unsetenv("OPERATOR_NAMESPACE")
		result := GetOperatorNamespace()
		if result != "" {
			t.Errorf("GetOperatorNamespace() = %q, want empty string", result)
		}
	})
}
