package utils

import (
	"reflect"
	"testing"

	"k8s.io/utils/ptr"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"

	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

func TestDecodeClusterRoleObjBytes(t *testing.T) {
	t.Run("valid YAML", func(t *testing.T) {
		yaml := `apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: test-cluster-role
rules:
- apiGroups: [""]
  resources: ["pods"]
  verbs: ["get", "list"]
`
		result := DecodeClusterRoleObjBytes([]byte(yaml))
		if result.Name != "test-cluster-role" {
			t.Errorf("Expected name 'test-cluster-role', got %q", result.Name)
		}
		if len(result.Rules) != 1 {
			t.Errorf("Expected 1 rule, got %d", len(result.Rules))
		}
	})

	t.Run("invalid YAML panics", func(t *testing.T) {
		defer func() {
			if r := recover(); r == nil {
				t.Error("Expected panic for invalid YAML, but did not panic")
			}
		}()
		DecodeClusterRoleObjBytes([]byte("invalid yaml content"))
	})
}

func TestDecodeClusterRoleBindingObjBytes(t *testing.T) {
	t.Run("valid YAML", func(t *testing.T) {
		yaml := `apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: test-binding
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: test-role
subjects:
- kind: ServiceAccount
  name: test-sa
  namespace: default
`
		result := DecodeClusterRoleBindingObjBytes([]byte(yaml))
		if result.Name != "test-binding" {
			t.Errorf("Expected name 'test-binding', got %q", result.Name)
		}
		if result.RoleRef.Name != "test-role" {
			t.Errorf("Expected roleRef 'test-role', got %q", result.RoleRef.Name)
		}
	})

	t.Run("invalid YAML panics", func(t *testing.T) {
		defer func() {
			if r := recover(); r == nil {
				t.Error("Expected panic for invalid YAML, but did not panic")
			}
		}()
		DecodeClusterRoleBindingObjBytes([]byte("invalid yaml content"))
	})
}

func TestDecodeRoleObjBytes(t *testing.T) {
	t.Run("valid YAML", func(t *testing.T) {
		yaml := `apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: test-role
  namespace: test-ns
rules:
- apiGroups: [""]
  resources: ["configmaps"]
  verbs: ["get"]
`
		result := DecodeRoleObjBytes([]byte(yaml))
		if result.Name != "test-role" {
			t.Errorf("Expected name 'test-role', got %q", result.Name)
		}
		if result.Namespace != "test-ns" {
			t.Errorf("Expected namespace 'test-ns', got %q", result.Namespace)
		}
	})

	t.Run("invalid YAML panics", func(t *testing.T) {
		defer func() {
			if r := recover(); r == nil {
				t.Error("Expected panic for invalid YAML, but did not panic")
			}
		}()
		DecodeRoleObjBytes([]byte("invalid yaml content"))
	})
}

func TestDecodeRoleBindingObjBytes(t *testing.T) {
	t.Run("valid YAML", func(t *testing.T) {
		yaml := `apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: test-role-binding
  namespace: test-ns
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: Role
  name: test-role
subjects:
- kind: ServiceAccount
  name: test-sa
  namespace: test-ns
`
		result := DecodeRoleBindingObjBytes([]byte(yaml))
		if result.Name != "test-role-binding" {
			t.Errorf("Expected name 'test-role-binding', got %q", result.Name)
		}
		if len(result.Subjects) != 1 {
			t.Errorf("Expected 1 subject, got %d", len(result.Subjects))
		}
	})

	t.Run("invalid YAML panics", func(t *testing.T) {
		defer func() {
			if r := recover(); r == nil {
				t.Error("Expected panic for invalid YAML, but did not panic")
			}
		}()
		DecodeRoleBindingObjBytes([]byte("invalid yaml content"))
	})
}

func TestDecodeServiceObjBytes(t *testing.T) {
	t.Run("valid YAML", func(t *testing.T) {
		yaml := `apiVersion: v1
kind: Service
metadata:
  name: test-service
  namespace: test-ns
spec:
  selector:
    app: test
  ports:
  - port: 80
    targetPort: 8080
`
		result := DecodeServiceObjBytes([]byte(yaml))
		if result.Name != "test-service" {
			t.Errorf("Expected name 'test-service', got %q", result.Name)
		}
		if len(result.Spec.Ports) != 1 {
			t.Errorf("Expected 1 port, got %d", len(result.Spec.Ports))
		}
	})

	t.Run("invalid YAML panics", func(t *testing.T) {
		defer func() {
			if r := recover(); r == nil {
				t.Error("Expected panic for invalid YAML, but did not panic")
			}
		}()
		DecodeServiceObjBytes([]byte("invalid yaml content"))
	})
}

func TestDecodeServiceAccountObjBytes(t *testing.T) {
	t.Run("valid YAML", func(t *testing.T) {
		yaml := `apiVersion: v1
kind: ServiceAccount
metadata:
  name: test-sa
  namespace: test-ns
`
		result := DecodeServiceAccountObjBytes([]byte(yaml))
		if result.Name != "test-sa" {
			t.Errorf("Expected name 'test-sa', got %q", result.Name)
		}
		if result.Namespace != "test-ns" {
			t.Errorf("Expected namespace 'test-ns', got %q", result.Namespace)
		}
	})

	t.Run("invalid YAML panics", func(t *testing.T) {
		defer func() {
			if r := recover(); r == nil {
				t.Error("Expected panic for invalid YAML, but did not panic")
			}
		}()
		DecodeServiceAccountObjBytes([]byte("invalid yaml content"))
	})
}

func TestDecodeCsiDriverObjBytes(t *testing.T) {
	t.Run("valid YAML", func(t *testing.T) {
		yaml := `apiVersion: storage.k8s.io/v1
kind: CSIDriver
metadata:
  name: test-driver
spec:
  attachRequired: false
  podInfoOnMount: true
`
		result := DecodeCsiDriverObjBytes([]byte(yaml))
		if result.Name != "test-driver" {
			t.Errorf("Expected name 'test-driver', got %q", result.Name)
		}
		if result.Spec.AttachRequired == nil || *result.Spec.AttachRequired {
			t.Error("Expected attachRequired to be false")
		}
	})

	t.Run("invalid YAML panics", func(t *testing.T) {
		defer func() {
			if r := recover(); r == nil {
				t.Error("Expected panic for invalid YAML, but did not panic")
			}
		}()
		DecodeCsiDriverObjBytes([]byte("invalid yaml content"))
	})
}

func TestDecodeValidatingWebhookConfigurationByBytes(t *testing.T) {
	t.Run("valid YAML", func(t *testing.T) {
		yaml := `apiVersion: admissionregistration.k8s.io/v1
kind: ValidatingWebhookConfiguration
metadata:
  name: test-webhook
webhooks:
- name: test.example.com
  admissionReviewVersions: ["v1"]
  sideEffects: None
  clientConfig:
    service:
      name: test-service
      namespace: test-ns
      path: /validate
`
		result := DecodeValidatingWebhookConfigurationByBytes([]byte(yaml))
		if result.Name != "test-webhook" {
			t.Errorf("Expected name 'test-webhook', got %q", result.Name)
		}
		if len(result.Webhooks) != 1 {
			t.Errorf("Expected 1 webhook, got %d", len(result.Webhooks))
		}
	})

	t.Run("invalid YAML panics", func(t *testing.T) {
		defer func() {
			if r := recover(); r == nil {
				t.Error("Expected panic for invalid YAML, but did not panic")
			}
		}()
		DecodeValidatingWebhookConfigurationByBytes([]byte("invalid yaml content"))
	})
}

func TestSetLabel(t *testing.T) {
	tests := []struct {
		name     string
		labels   map[string]string
		key      string
		value    string
		expected map[string]string
	}{
		{
			name:     "nil labels map creates new map",
			labels:   nil,
			key:      "app",
			value:    "test",
			expected: map[string]string{"app": "test"},
		},
		{
			name:     "empty labels map adds label",
			labels:   map[string]string{},
			key:      "env",
			value:    "prod",
			expected: map[string]string{"env": "prod"},
		},
		{
			name:     "existing labels map adds new label",
			labels:   map[string]string{"app": "test"},
			key:      "version",
			value:    "v1",
			expected: map[string]string{"app": "test", "version": "v1"},
		},
		{
			name:     "existing labels map updates existing label",
			labels:   map[string]string{"app": "old"},
			key:      "app",
			value:    "new",
			expected: map[string]string{"app": "new"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SetLabel(tt.labels, tt.key, tt.value)
			if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("SetLabel() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestGenerateConfigHashFromString(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "simple string",
			input:    "test",
			expected: "9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08",
		},
		{
			name:     "string with leading/trailing whitespace",
			input:    "  test  ",
			expected: "9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08",
		},
		{
			name:     "string with newlines",
			input:    "\ntest\n",
			expected: "9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08",
		},
		{
			name:     "empty string",
			input:    "",
			expected: "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
		},
		{
			name:     "multiline config",
			input:    "key1=value1\nkey2=value2",
			expected: GenerateConfigHash([]byte("key1=value1\nkey2=value2")),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GenerateConfigHashFromString(tt.input)
			if result != tt.expected {
				t.Errorf("GenerateConfigHashFromString() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestGenerateConfigHash(t *testing.T) {
	tests := []struct {
		name     string
		input    []byte
		expected string
	}{
		{
			name:     "simple bytes",
			input:    []byte("test"),
			expected: "9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08",
		},
		{
			name:     "bytes with whitespace",
			input:    []byte("  test  "),
			expected: "9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08",
		},
		{
			name:     "empty bytes",
			input:    []byte{},
			expected: "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
		},
		{
			name:     "nil bytes",
			input:    nil,
			expected: "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GenerateConfigHash(tt.input)
			if result != tt.expected {
				t.Errorf("GenerateConfigHash() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestGenerateMapHash(t *testing.T) {
	tests := []struct {
		name  string
		input map[string]string
	}{
		{
			name:  "empty map",
			input: map[string]string{},
		},
		{
			name:  "single entry",
			input: map[string]string{"key": "value"},
		},
		{
			name:  "multiple entries",
			input: map[string]string{"key1": "value1", "key2": "value2", "key3": "value3"},
		},
		{
			name:  "entries with whitespace",
			input: map[string]string{"  key1  ": "  value1  ", "key2": "value2"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test that it generates a valid hash (64 char hex string)
			result := GenerateMapHash(tt.input)
			if len(result) != 64 {
				t.Errorf("GenerateMapHash() returned hash of length %d, want 64", len(result))
			}

			// Test that same input generates same hash
			result2 := GenerateMapHash(tt.input)
			if result != result2 {
				t.Error("GenerateMapHash() should be deterministic")
			}

			// Test that order doesn't matter (keys are sorted)
			if len(tt.input) > 1 {
				// Create a new map with same entries (Go randomizes iteration order)
				input2 := make(map[string]string)
				for k, v := range tt.input {
					input2[k] = v
				}
				result3 := GenerateMapHash(input2)
				if result != result3 {
					t.Error("GenerateMapHash() should produce same hash regardless of map iteration order")
				}
			}
		})
	}

	// Test that different maps produce different hashes
	t.Run("different maps produce different hashes", func(t *testing.T) {
		hash1 := GenerateMapHash(map[string]string{"key": "value1"})
		hash2 := GenerateMapHash(map[string]string{"key": "value2"})
		if hash1 == hash2 {
			t.Error("Different maps should produce different hashes")
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

			// Verify it's a copy (different memory address) when input has values
			if len(tt.input) > 0 {
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

	t.Run("Container ImagePullPolicy modified", func(t *testing.T) {
		desired := createStatefulSet()
		fetched := createStatefulSet()
		fetched.Spec.Template.Spec.Containers[0].ImagePullPolicy = corev1.PullNever
		if !StatefulSetSpecModified(desired, fetched) {
			t.Error("Expected true when ImagePullPolicy differs")
		}
	})

	t.Run("Container Env modified", func(t *testing.T) {
		desired := createStatefulSet()
		fetched := createStatefulSet()
		fetched.Spec.Template.Spec.Containers[0].Env = []corev1.EnvVar{{
			Name:  "DIFFERENT",
			Value: "value",
		}}
		if !StatefulSetSpecModified(desired, fetched) {
			t.Error("Expected true when container Env differs")
		}
	})

	t.Run("Container Resources modified", func(t *testing.T) {
		desired := createStatefulSet()
		fetched := createStatefulSet()
		fetched.Spec.Template.Spec.Containers[0].Resources.Requests[corev1.ResourceCPU] = resource.MustParse("200m")
		if !StatefulSetSpecModified(desired, fetched) {
			t.Error("Expected true when container Resources differ")
		}
	})

	t.Run("Container VolumeMounts modified", func(t *testing.T) {
		desired := createStatefulSet()
		fetched := createStatefulSet()
		fetched.Spec.Template.Spec.Containers[0].VolumeMounts[0].MountPath = "/different/path"
		if !StatefulSetSpecModified(desired, fetched) {
			t.Error("Expected true when container VolumeMounts differ")
		}
	})

	t.Run("Container name not found in fetched", func(t *testing.T) {
		desired := createStatefulSet()
		fetched := createStatefulSet()
		// Change the container name in fetched so desired container won't be found
		fetched.Spec.Template.Spec.Containers[0].Name = "different-name"
		if !StatefulSetSpecModified(desired, fetched) {
			t.Error("Expected true when container name in desired not found in fetched")
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

	t.Run("VolumeClaimTemplate AccessModes modified", func(t *testing.T) {
		desired := createStatefulSet()
		fetched := createStatefulSet()
		fetched.Spec.VolumeClaimTemplates[0].Spec.AccessModes = []corev1.PersistentVolumeAccessMode{corev1.ReadWriteMany}
		if !StatefulSetSpecModified(desired, fetched) {
			t.Error("Expected true when VolumeClaimTemplate AccessModes differ")
		}
	})

	t.Run("VolumeClaimTemplate Resources.Requests modified", func(t *testing.T) {
		desired := createStatefulSet()
		fetched := createStatefulSet()
		fetched.Spec.VolumeClaimTemplates[0].Spec.Resources.Requests[corev1.ResourceStorage] = resource.MustParse("20Gi")
		if !StatefulSetSpecModified(desired, fetched) {
			t.Error("Expected true when VolumeClaimTemplate Resources.Requests differ")
		}
	})

	t.Run("DNSPolicy modified", func(t *testing.T) {
		desired := createStatefulSet()
		fetched := createStatefulSet()
		desired.Spec.Template.Spec.DNSPolicy = corev1.DNSClusterFirst
		fetched.Spec.Template.Spec.DNSPolicy = corev1.DNSDefault
		if !StatefulSetSpecModified(desired, fetched) {
			t.Error("Expected true when DNSPolicy differs")
		}
	})

	t.Run("Volumes modified - ConfigMap", func(t *testing.T) {
		desired := createStatefulSet()
		fetched := createStatefulSet()
		desired.Spec.Template.Spec.Volumes = []corev1.Volume{{
			Name: "config",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{Name: "my-config"},
				},
			},
		}}
		fetched.Spec.Template.Spec.Volumes = []corev1.Volume{{
			Name: "config",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{Name: "different-config"},
				},
			},
		}}
		if !StatefulSetSpecModified(desired, fetched) {
			t.Error("Expected true when ConfigMap volume differs")
		}
	})

	t.Run("Volumes modified - Secret", func(t *testing.T) {
		desired := createStatefulSet()
		fetched := createStatefulSet()
		desired.Spec.Template.Spec.Volumes = []corev1.Volume{{
			Name: "secret",
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: "my-secret",
				},
			},
		}}
		fetched.Spec.Template.Spec.Volumes = []corev1.Volume{{
			Name: "secret",
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: "different-secret",
				},
			},
		}}
		if !StatefulSetSpecModified(desired, fetched) {
			t.Error("Expected true when Secret volume differs")
		}
	})

	t.Run("InitContainers added", func(t *testing.T) {
		desired := createStatefulSet()
		fetched := createStatefulSet()
		desired.Spec.Template.Spec.InitContainers = []corev1.Container{{
			Name:  "init",
			Image: "init:latest",
		}}
		if !StatefulSetSpecModified(desired, fetched) {
			t.Error("Expected true when InitContainers differ")
		}
	})

	t.Run("InitContainer image modified", func(t *testing.T) {
		desired := createStatefulSet()
		fetched := createStatefulSet()
		desired.Spec.Template.Spec.InitContainers = []corev1.Container{{
			Name:  "init",
			Image: "init:v1",
		}}
		fetched.Spec.Template.Spec.InitContainers = []corev1.Container{{
			Name:  "init",
			Image: "init:v2",
		}}
		if !StatefulSetSpecModified(desired, fetched) {
			t.Error("Expected true when InitContainer image differs")
		}
	})

	t.Run("Container Ports modified", func(t *testing.T) {
		desired := createStatefulSet()
		fetched := createStatefulSet()
		desired.Spec.Template.Spec.Containers[0].Ports = []corev1.ContainerPort{{
			ContainerPort: 8080,
		}}
		fetched.Spec.Template.Spec.Containers[0].Ports = []corev1.ContainerPort{{
			ContainerPort: 9090,
		}}
		if !StatefulSetSpecModified(desired, fetched) {
			t.Error("Expected true when container ports differ")
		}
	})

	t.Run("Container ReadinessProbe modified", func(t *testing.T) {
		desired := createStatefulSet()
		fetched := createStatefulSet()
		desired.Spec.Template.Spec.Containers[0].ReadinessProbe = &corev1.Probe{
			ProbeHandler: corev1.ProbeHandler{
				HTTPGet: &corev1.HTTPGetAction{
					Path: "/health",
				},
			},
		}
		fetched.Spec.Template.Spec.Containers[0].ReadinessProbe = &corev1.Probe{
			ProbeHandler: corev1.ProbeHandler{
				HTTPGet: &corev1.HTTPGetAction{
					Path: "/ready",
				},
			},
		}
		if !StatefulSetSpecModified(desired, fetched) {
			t.Error("Expected true when ReadinessProbe differs")
		}
	})

	t.Run("Container SecurityContext modified", func(t *testing.T) {
		desired := createStatefulSet()
		fetched := createStatefulSet()
		desired.Spec.Template.Spec.Containers[0].SecurityContext = &corev1.SecurityContext{
			RunAsUser: ptr.To(int64(1000)),
		}
		fetched.Spec.Template.Spec.Containers[0].SecurityContext = &corev1.SecurityContext{
			RunAsUser: ptr.To(int64(2000)),
		}
		if !StatefulSetSpecModified(desired, fetched) {
			t.Error("Expected true when SecurityContext differs")
		}
	})

	t.Run("Container SecurityContext nil vs non-nil", func(t *testing.T) {
		desired := createStatefulSet()
		fetched := createStatefulSet()
		desired.Spec.Template.Spec.Containers[0].SecurityContext = &corev1.SecurityContext{
			RunAsUser: ptr.To(int64(1000)),
		}
		fetched.Spec.Template.Spec.Containers[0].SecurityContext = nil
		if !StatefulSetSpecModified(desired, fetched) {
			t.Error("Expected true when desired has SecurityContext but fetched is nil")
		}
	})

	t.Run("Container Ports with same port different protocol", func(t *testing.T) {
		desired := createStatefulSet()
		fetched := createStatefulSet()
		desired.Spec.Template.Spec.Containers[0].Ports = []corev1.ContainerPort{{
			ContainerPort: 8080,
			Protocol:      corev1.ProtocolTCP,
		}}
		fetched.Spec.Template.Spec.Containers[0].Ports = []corev1.ContainerPort{{
			ContainerPort: 8080,
			Protocol:      corev1.ProtocolUDP,
		}}
		if !StatefulSetSpecModified(desired, fetched) {
			t.Error("Expected true when container port protocols differ")
		}
	})

	t.Run("Container Ports with multiple protocols", func(t *testing.T) {
		desired := createStatefulSet()
		fetched := createStatefulSet()
		desired.Spec.Template.Spec.Containers[0].Ports = []corev1.ContainerPort{
			{ContainerPort: 8080, Protocol: corev1.ProtocolTCP},
			{ContainerPort: 8080, Protocol: corev1.ProtocolUDP},
		}
		fetched.Spec.Template.Spec.Containers[0].Ports = []corev1.ContainerPort{
			{ContainerPort: 8080, Protocol: corev1.ProtocolTCP},
			{ContainerPort: 8080, Protocol: corev1.ProtocolUDP},
		}
		if StatefulSetSpecModified(desired, fetched) {
			t.Error("Expected false when ports with different protocols match")
		}
	})

	t.Run("Volumes modified - Projected", func(t *testing.T) {
		desired := createStatefulSet()
		fetched := createStatefulSet()
		desired.Spec.Template.Spec.Volumes = []corev1.Volume{{
			Name: "projected-vol",
			VolumeSource: corev1.VolumeSource{
				Projected: &corev1.ProjectedVolumeSource{
					Sources: []corev1.VolumeProjection{{
						Secret: &corev1.SecretProjection{
							LocalObjectReference: corev1.LocalObjectReference{Name: "my-secret"},
						},
					}},
				},
			},
		}}
		fetched.Spec.Template.Spec.Volumes = []corev1.Volume{{
			Name: "projected-vol",
			VolumeSource: corev1.VolumeSource{
				Projected: &corev1.ProjectedVolumeSource{
					Sources: []corev1.VolumeProjection{{
						Secret: &corev1.SecretProjection{
							LocalObjectReference: corev1.LocalObjectReference{Name: "different-secret"},
						},
					}},
				},
			},
		}}
		if !StatefulSetSpecModified(desired, fetched) {
			t.Error("Expected true when Projected volume differs")
		}
	})

	t.Run("Volumes modified - CSI", func(t *testing.T) {
		desired := createStatefulSet()
		fetched := createStatefulSet()
		desired.Spec.Template.Spec.Volumes = []corev1.Volume{{
			Name: "csi-vol",
			VolumeSource: corev1.VolumeSource{
				CSI: &corev1.CSIVolumeSource{
					Driver: "csi.spiffe.io",
				},
			},
		}}
		fetched.Spec.Template.Spec.Volumes = []corev1.Volume{{
			Name: "csi-vol",
			VolumeSource: corev1.VolumeSource{
				CSI: &corev1.CSIVolumeSource{
					Driver: "different.csi.driver",
				},
			},
		}}
		if !StatefulSetSpecModified(desired, fetched) {
			t.Error("Expected true when CSI volume differs")
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

	t.Run("Container name not found in fetched", func(t *testing.T) {
		desired := createDeployment()
		fetched := createDeployment()
		// Change the container name in fetched so desired container won't be found
		fetched.Spec.Template.Spec.Containers[0].Name = "different-name"
		if !DeploymentSpecModified(desired, fetched) {
			t.Error("Expected true when container name in desired not found in fetched")
		}
	})

	t.Run("Container image modified", func(t *testing.T) {
		desired := createDeployment()
		fetched := createDeployment()
		fetched.Spec.Template.Spec.Containers[0].Image = "nginx:1.21"
		if !DeploymentSpecModified(desired, fetched) {
			t.Error("Expected true when container image differs")
		}
	})

	t.Run("Container args modified", func(t *testing.T) {
		desired := createDeployment()
		fetched := createDeployment()
		fetched.Spec.Template.Spec.Containers[0].Args = []string{"--different", "arg"}
		if !DeploymentSpecModified(desired, fetched) {
			t.Error("Expected true when container args differ")
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

	t.Run("ServiceAccountName modified", func(t *testing.T) {
		desired := createDeployment()
		fetched := createDeployment()
		fetched.Spec.Template.Spec.ServiceAccountName = "different-sa"
		if !DeploymentSpecModified(desired, fetched) {
			t.Error("Expected true when ServiceAccountName differs")
		}
	})

	t.Run("ShareProcessNamespace modified", func(t *testing.T) {
		desired := createDeployment()
		fetched := createDeployment()
		fetched.Spec.Template.Spec.ShareProcessNamespace = ptr.To(true)
		if !DeploymentSpecModified(desired, fetched) {
			t.Error("Expected true when ShareProcessNamespace differs")
		}
	})

	t.Run("NodeSelector modified", func(t *testing.T) {
		desired := createDeployment()
		fetched := createDeployment()
		fetched.Spec.Template.Spec.NodeSelector["zone"] = "west"
		if !DeploymentSpecModified(desired, fetched) {
			t.Error("Expected true when NodeSelector differs")
		}
	})

	t.Run("Affinity modified", func(t *testing.T) {
		desired := createDeployment()
		fetched := createDeployment()
		fetched.Spec.Template.Spec.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms[0].MatchExpressions[0].Values = []string{"west"}
		if !DeploymentSpecModified(desired, fetched) {
			t.Error("Expected true when Affinity differs")
		}
	})

	t.Run("Tolerations modified", func(t *testing.T) {
		desired := createDeployment()
		fetched := createDeployment()
		fetched.Spec.Template.Spec.Tolerations[0].Value = "different"
		if !DeploymentSpecModified(desired, fetched) {
			t.Error("Expected true when Tolerations differ")
		}
	})

	t.Run("Template labels modified", func(t *testing.T) {
		desired := createDeployment()
		fetched := createDeployment()
		fetched.Spec.Template.Labels["app"] = "different"
		if !DeploymentSpecModified(desired, fetched) {
			t.Error("Expected true when Template.Labels differ")
		}
	})

	t.Run("DNSPolicy modified", func(t *testing.T) {
		desired := createDeployment()
		fetched := createDeployment()
		desired.Spec.Template.Spec.DNSPolicy = corev1.DNSClusterFirst
		fetched.Spec.Template.Spec.DNSPolicy = corev1.DNSDefault
		if !DeploymentSpecModified(desired, fetched) {
			t.Error("Expected true when DNSPolicy differs")
		}
	})

	t.Run("Volumes modified - EmptyDir", func(t *testing.T) {
		desired := createDeployment()
		fetched := createDeployment()
		desired.Spec.Template.Spec.Volumes = []corev1.Volume{{
			Name: "cache",
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			},
		}}
		fetched.Spec.Template.Spec.Volumes = []corev1.Volume{}
		if !DeploymentSpecModified(desired, fetched) {
			t.Error("Expected true when volume count differs")
		}
	})

	t.Run("InitContainers count modified", func(t *testing.T) {
		desired := createDeployment()
		fetched := createDeployment()
		desired.Spec.Template.Spec.InitContainers = []corev1.Container{{
			Name:  "init",
			Image: "init:latest",
		}}
		if !DeploymentSpecModified(desired, fetched) {
			t.Error("Expected true when InitContainers differ")
		}
	})

	t.Run("InitContainer args modified", func(t *testing.T) {
		desired := createDeployment()
		fetched := createDeployment()
		desired.Spec.Template.Spec.InitContainers = []corev1.Container{{
			Name:  "init",
			Image: "init:latest",
			Args:  []string{"arg1"},
		}}
		fetched.Spec.Template.Spec.InitContainers = []corev1.Container{{
			Name:  "init",
			Image: "init:latest",
			Args:  []string{"arg2"},
		}}
		if !DeploymentSpecModified(desired, fetched) {
			t.Error("Expected true when InitContainer args differ")
		}
	})

	t.Run("Container Ports added", func(t *testing.T) {
		desired := createDeployment()
		fetched := createDeployment()
		desired.Spec.Template.Spec.Containers[0].Ports = []corev1.ContainerPort{{
			ContainerPort: 8080,
			Protocol:      corev1.ProtocolTCP,
		}}
		fetched.Spec.Template.Spec.Containers[0].Ports = []corev1.ContainerPort{}
		if !DeploymentSpecModified(desired, fetched) {
			t.Error("Expected true when container ports differ")
		}
	})

	t.Run("Container LivenessProbe modified", func(t *testing.T) {
		desired := createDeployment()
		fetched := createDeployment()
		desired.Spec.Template.Spec.Containers[0].LivenessProbe = &corev1.Probe{
			ProbeHandler: corev1.ProbeHandler{
				HTTPGet: &corev1.HTTPGetAction{
					Path: "/healthz",
					Port: intstr.FromInt(8080),
				},
			},
		}
		fetched.Spec.Template.Spec.Containers[0].LivenessProbe = nil
		if !DeploymentSpecModified(desired, fetched) {
			t.Error("Expected true when LivenessProbe differs")
		}
	})

	t.Run("Container SecurityContext added", func(t *testing.T) {
		desired := createDeployment()
		fetched := createDeployment()
		desired.Spec.Template.Spec.Containers[0].SecurityContext = &corev1.SecurityContext{
			Privileged: ptr.To(false),
		}
		fetched.Spec.Template.Spec.Containers[0].SecurityContext = nil
		if !DeploymentSpecModified(desired, fetched) {
			t.Error("Expected true when SecurityContext differs")
		}
	})

	t.Run("Container Ports with different protocol", func(t *testing.T) {
		desired := createDeployment()
		fetched := createDeployment()
		desired.Spec.Template.Spec.Containers[0].Ports = []corev1.ContainerPort{{
			ContainerPort: 8080,
			Protocol:      corev1.ProtocolTCP,
		}}
		fetched.Spec.Template.Spec.Containers[0].Ports = []corev1.ContainerPort{{
			ContainerPort: 8080,
			Protocol:      corev1.ProtocolUDP,
		}}
		if !DeploymentSpecModified(desired, fetched) {
			t.Error("Expected true when port protocol differs")
		}
	})

	t.Run("Container Ports matching with protocol", func(t *testing.T) {
		desired := createDeployment()
		fetched := createDeployment()
		desired.Spec.Template.Spec.Containers[0].Ports = []corev1.ContainerPort{
			{ContainerPort: 8080, Protocol: corev1.ProtocolTCP, Name: "http"},
			{ContainerPort: 8080, Protocol: corev1.ProtocolUDP, Name: "udp"},
		}
		fetched.Spec.Template.Spec.Containers[0].Ports = []corev1.ContainerPort{
			{ContainerPort: 8080, Protocol: corev1.ProtocolUDP, Name: "udp"},
			{ContainerPort: 8080, Protocol: corev1.ProtocolTCP, Name: "http"},
		}
		if DeploymentSpecModified(desired, fetched) {
			t.Error("Expected false when ports match including protocol (order independent)")
		}
	})

	t.Run("Volumes modified - Projected volume", func(t *testing.T) {
		desired := createDeployment()
		fetched := createDeployment()
		desired.Spec.Template.Spec.Volumes = []corev1.Volume{{
			Name: "projected",
			VolumeSource: corev1.VolumeSource{
				Projected: &corev1.ProjectedVolumeSource{
					Sources: []corev1.VolumeProjection{{
						ConfigMap: &corev1.ConfigMapProjection{
							LocalObjectReference: corev1.LocalObjectReference{Name: "config1"},
						},
					}},
				},
			},
		}}
		fetched.Spec.Template.Spec.Volumes = []corev1.Volume{{
			Name: "projected",
			VolumeSource: corev1.VolumeSource{
				Projected: &corev1.ProjectedVolumeSource{
					Sources: []corev1.VolumeProjection{{
						ConfigMap: &corev1.ConfigMapProjection{
							LocalObjectReference: corev1.LocalObjectReference{Name: "config2"},
						},
					}},
				},
			},
		}}
		if !DeploymentSpecModified(desired, fetched) {
			t.Error("Expected true when projected volume differs")
		}
	})

	t.Run("Volumes modified - CSI volume", func(t *testing.T) {
		desired := createDeployment()
		fetched := createDeployment()
		desired.Spec.Template.Spec.Volumes = []corev1.Volume{{
			Name: "csi",
			VolumeSource: corev1.VolumeSource{
				CSI: &corev1.CSIVolumeSource{
					Driver:   "csi.spiffe.io",
					ReadOnly: ptr.To(true),
				},
			},
		}}
		fetched.Spec.Template.Spec.Volumes = []corev1.Volume{{
			Name: "csi",
			VolumeSource: corev1.VolumeSource{
				CSI: &corev1.CSIVolumeSource{
					Driver:   "csi.spiffe.io",
					ReadOnly: ptr.To(false),
				},
			},
		}}
		if !DeploymentSpecModified(desired, fetched) {
			t.Error("Expected true when CSI volume ReadOnly differs")
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

	t.Run("Container name not found in fetched", func(t *testing.T) {
		desired := createDaemonSet()
		fetched := createDaemonSet()
		// Change the container name in fetched so desired container won't be found
		fetched.Spec.Template.Spec.Containers[0].Name = "different-name"
		if !DaemonSetSpecModified(desired, fetched) {
			t.Error("Expected true when container name in desired not found in fetched")
		}
	})

	t.Run("Container ImagePullPolicy modified", func(t *testing.T) {
		desired := createDaemonSet()
		fetched := createDaemonSet()
		fetched.Spec.Template.Spec.Containers[0].ImagePullPolicy = corev1.PullNever
		if !DaemonSetSpecModified(desired, fetched) {
			t.Error("Expected true when ImagePullPolicy differs")
		}
	})

	t.Run("Container Args modified", func(t *testing.T) {
		desired := createDaemonSet()
		fetched := createDaemonSet()
		fetched.Spec.Template.Spec.Containers[0].Args = []string{"--different", "arg"}
		if !DaemonSetSpecModified(desired, fetched) {
			t.Error("Expected true when container Args differ")
		}
	})

	t.Run("Container VolumeMounts modified", func(t *testing.T) {
		desired := createDaemonSet()
		fetched := createDaemonSet()
		fetched.Spec.Template.Spec.Containers[0].VolumeMounts[0].MountPath = "/different/path"
		if !DaemonSetSpecModified(desired, fetched) {
			t.Error("Expected true when container VolumeMounts differ")
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

	t.Run("ServiceAccountName modified", func(t *testing.T) {
		desired := createDaemonSet()
		fetched := createDaemonSet()
		fetched.Spec.Template.Spec.ServiceAccountName = "different-sa"
		if !DaemonSetSpecModified(desired, fetched) {
			t.Error("Expected true when ServiceAccountName differs")
		}
	})

	t.Run("ShareProcessNamespace modified", func(t *testing.T) {
		desired := createDaemonSet()
		fetched := createDaemonSet()
		fetched.Spec.Template.Spec.ShareProcessNamespace = ptr.To(true)
		if !DaemonSetSpecModified(desired, fetched) {
			t.Error("Expected true when ShareProcessNamespace differs")
		}
	})

	t.Run("NodeSelector modified", func(t *testing.T) {
		desired := createDaemonSet()
		fetched := createDaemonSet()
		fetched.Spec.Template.Spec.NodeSelector["zone"] = "west"
		if !DaemonSetSpecModified(desired, fetched) {
			t.Error("Expected true when NodeSelector differs")
		}
	})

	t.Run("Affinity modified", func(t *testing.T) {
		desired := createDaemonSet()
		fetched := createDaemonSet()
		fetched.Spec.Template.Spec.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms[0].MatchExpressions[0].Values = []string{"west"}
		if !DaemonSetSpecModified(desired, fetched) {
			t.Error("Expected true when Affinity differs")
		}
	})

	t.Run("Tolerations modified", func(t *testing.T) {
		desired := createDaemonSet()
		fetched := createDaemonSet()
		fetched.Spec.Template.Spec.Tolerations[0].Value = "different"
		if !DaemonSetSpecModified(desired, fetched) {
			t.Error("Expected true when Tolerations differ")
		}
	})

	// Test that Tolerations are properly detected regardless of NodeSelector
	t.Run("Tolerations modified with empty NodeSelector", func(t *testing.T) {
		desired := createDaemonSet()
		fetched := createDaemonSet()

		// Set NodeSelector to empty but keep Tolerations
		desired.Spec.Template.Spec.NodeSelector = map[string]string{}
		fetched.Spec.Template.Spec.NodeSelector = map[string]string{}

		// Modify tolerations
		fetched.Spec.Template.Spec.Tolerations[0].Value = "different"

		// Should detect the tolerations change even with empty NodeSelector
		if !DaemonSetSpecModified(desired, fetched) {
			t.Error("Expected true when Tolerations differ, regardless of NodeSelector")
		}
	})

	// Test that empty desired Tolerations vs non-empty fetched triggers modification
	t.Run("Desired has empty Tolerations but fetched has Tolerations", func(t *testing.T) {
		desired := createDaemonSet()
		fetched := createDaemonSet()

		// Desired has no tolerations
		desired.Spec.Template.Spec.Tolerations = []corev1.Toleration{}

		// Fetched has tolerations
		// (fetched already has tolerations from createDaemonSet)

		// Should detect the difference
		if !DaemonSetSpecModified(desired, fetched) {
			t.Error("Expected true when desired has no Tolerations but fetched does")
		}
	})

	// Test Env field comparison in DaemonSet
	t.Run("Container Env modified", func(t *testing.T) {
		desired := createDaemonSet()
		fetched := createDaemonSet()

		// Add Env to containers
		desired.Spec.Template.Spec.Containers[0].Env = []corev1.EnvVar{{
			Name:  "ENV_VAR",
			Value: "value",
		}}
		fetched.Spec.Template.Spec.Containers[0].Env = []corev1.EnvVar{{
			Name:  "ENV_VAR",
			Value: "different-value",
		}}

		if !DaemonSetSpecModified(desired, fetched) {
			t.Error("Expected true when container Env differs")
		}
	})

	t.Run("DNSPolicy modified", func(t *testing.T) {
		desired := createDaemonSet()
		fetched := createDaemonSet()
		desired.Spec.Template.Spec.DNSPolicy = corev1.DNSClusterFirstWithHostNet
		fetched.Spec.Template.Spec.DNSPolicy = corev1.DNSClusterFirst
		if !DaemonSetSpecModified(desired, fetched) {
			t.Error("Expected true when DNSPolicy differs")
		}
	})

	t.Run("Volumes modified - HostPath", func(t *testing.T) {
		desired := createDaemonSet()
		fetched := createDaemonSet()
		desired.Spec.Template.Spec.Volumes = []corev1.Volume{{
			Name: "hostpath",
			VolumeSource: corev1.VolumeSource{
				HostPath: &corev1.HostPathVolumeSource{
					Path: "/var/lib/data",
				},
			},
		}}
		fetched.Spec.Template.Spec.Volumes = []corev1.Volume{{
			Name: "hostpath",
			VolumeSource: corev1.VolumeSource{
				HostPath: &corev1.HostPathVolumeSource{
					Path: "/var/lib/other",
				},
			},
		}}
		if !DaemonSetSpecModified(desired, fetched) {
			t.Error("Expected true when HostPath volume differs")
		}
	})

	t.Run("InitContainers not found by name", func(t *testing.T) {
		desired := createDaemonSet()
		fetched := createDaemonSet()
		desired.Spec.Template.Spec.InitContainers = []corev1.Container{{
			Name:  "init-setup",
			Image: "setup:v1",
		}}
		fetched.Spec.Template.Spec.InitContainers = []corev1.Container{{
			Name:  "init-different",
			Image: "setup:v1",
		}}
		if !DaemonSetSpecModified(desired, fetched) {
			t.Error("Expected true when InitContainer name not found")
		}
	})

	t.Run("Container Ports count differs", func(t *testing.T) {
		desired := createDaemonSet()
		fetched := createDaemonSet()
		desired.Spec.Template.Spec.Containers[0].Ports = []corev1.ContainerPort{{
			ContainerPort: 8080,
		}, {
			ContainerPort: 9090,
		}}
		fetched.Spec.Template.Spec.Containers[0].Ports = []corev1.ContainerPort{{
			ContainerPort: 8080,
		}}
		if !DaemonSetSpecModified(desired, fetched) {
			t.Error("Expected true when container port count differs")
		}
	})

	t.Run("Container ReadinessProbe added", func(t *testing.T) {
		desired := createDaemonSet()
		fetched := createDaemonSet()
		desired.Spec.Template.Spec.Containers[0].ReadinessProbe = &corev1.Probe{
			ProbeHandler: corev1.ProbeHandler{
				TCPSocket: &corev1.TCPSocketAction{
					Port: intstr.FromInt(8080),
				},
			},
		}
		fetched.Spec.Template.Spec.Containers[0].ReadinessProbe = nil
		if !DaemonSetSpecModified(desired, fetched) {
			t.Error("Expected true when ReadinessProbe is added")
		}
	})

	t.Run("Container SecurityContext nil check", func(t *testing.T) {
		desired := createDaemonSet()
		fetched := createDaemonSet()
		desired.Spec.Template.Spec.Containers[0].SecurityContext = nil
		fetched.Spec.Template.Spec.Containers[0].SecurityContext = &corev1.SecurityContext{
			RunAsNonRoot: ptr.To(true),
		}
		if !DaemonSetSpecModified(desired, fetched) {
			t.Error("Expected true when desired SecurityContext is nil but fetched is not")
		}
	})

	t.Run("Container Ports protocol matching", func(t *testing.T) {
		desired := createDaemonSet()
		fetched := createDaemonSet()
		desired.Spec.Template.Spec.Containers[0].Ports = []corev1.ContainerPort{
			{ContainerPort: 9090, Protocol: corev1.ProtocolTCP},
			{ContainerPort: 9091, Protocol: corev1.ProtocolUDP},
		}
		fetched.Spec.Template.Spec.Containers[0].Ports = []corev1.ContainerPort{
			{ContainerPort: 9090, Protocol: corev1.ProtocolTCP},
			{ContainerPort: 9091, Protocol: corev1.ProtocolUDP},
		}
		if DaemonSetSpecModified(desired, fetched) {
			t.Error("Expected false when ports with protocols match exactly")
		}
	})

	t.Run("Container Ports protocol mismatch", func(t *testing.T) {
		desired := createDaemonSet()
		fetched := createDaemonSet()
		desired.Spec.Template.Spec.Containers[0].Ports = []corev1.ContainerPort{
			{ContainerPort: 9090, Protocol: corev1.ProtocolTCP},
		}
		fetched.Spec.Template.Spec.Containers[0].Ports = []corev1.ContainerPort{
			{ContainerPort: 9090, Protocol: corev1.ProtocolUDP},
		}
		if !DaemonSetSpecModified(desired, fetched) {
			t.Error("Expected true when port protocols differ")
		}
	})

	t.Run("Volumes modified - CSI with VolumeAttributes", func(t *testing.T) {
		desired := createDaemonSet()
		fetched := createDaemonSet()
		desired.Spec.Template.Spec.Volumes = []corev1.Volume{{
			Name: "csi-with-attrs",
			VolumeSource: corev1.VolumeSource{
				CSI: &corev1.CSIVolumeSource{
					Driver: "csi.spiffe.io",
					VolumeAttributes: map[string]string{
						"key1": "value1",
					},
				},
			},
		}}
		fetched.Spec.Template.Spec.Volumes = []corev1.Volume{{
			Name: "csi-with-attrs",
			VolumeSource: corev1.VolumeSource{
				CSI: &corev1.CSIVolumeSource{
					Driver: "csi.spiffe.io",
					VolumeAttributes: map[string]string{
						"key1": "value2",
					},
				},
			},
		}}
		if !DaemonSetSpecModified(desired, fetched) {
			t.Error("Expected true when CSI VolumeAttributes differ")
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

	t.Run("Empty desired NodeSelector vs non-empty fetched", func(t *testing.T) {
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
		// Should trigger modification since desired has empty NodeSelector but fetched has values
		if !DeploymentSpecModified(desired, fetched) {
			t.Error("Expected true when desired NodeSelector is empty but fetched has values")
		}
	})

	t.Run("Nil desired Affinity vs non-nil fetched", func(t *testing.T) {
		desired := &appsv1.StatefulSet{
			Spec: appsv1.StatefulSetSpec{
				Selector:    &metav1.LabelSelector{MatchLabels: map[string]string{"app": "test"}},
				ServiceName: "test",
				Template: corev1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"app": "test"}},
					Spec: corev1.PodSpec{
						Affinity:   nil, // Nil
						Containers: []corev1.Container{{Name: "test", Image: "test"}},
					},
				},
			},
		}
		fetched := &appsv1.StatefulSet{
			Spec: appsv1.StatefulSetSpec{
				Selector:    &metav1.LabelSelector{MatchLabels: map[string]string{"app": "test"}},
				ServiceName: "test",
				Template: corev1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"app": "test"}},
					Spec: corev1.PodSpec{
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
						Containers: []corev1.Container{{Name: "test", Image: "test"}},
					},
				},
			},
		}
		// Should trigger modification since desired has nil Affinity but fetched has values
		if !StatefulSetSpecModified(desired, fetched) {
			t.Error("Expected true when desired Affinity is nil but fetched has values")
		}
	})

	t.Run("Empty desired Tolerations vs non-empty fetched", func(t *testing.T) {
		desired := &appsv1.Deployment{
			Spec: appsv1.DeploymentSpec{
				Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "test"}},
				Template: corev1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"app": "test"}},
					Spec: corev1.PodSpec{
						Tolerations: []corev1.Toleration{}, // Empty
						Containers:  []corev1.Container{{Name: "test", Image: "test"}},
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
						Tolerations: []corev1.Toleration{{
							Key:      "node-type",
							Operator: corev1.TolerationOpEqual,
							Value:    "test",
							Effect:   corev1.TaintEffectNoSchedule,
						}},
						Containers: []corev1.Container{{Name: "test", Image: "test"}},
					},
				},
			},
		}
		// Should trigger modification since desired has empty Tolerations but fetched has values
		if !DeploymentSpecModified(desired, fetched) {
			t.Error("Expected true when desired Tolerations is empty but fetched has values")
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
