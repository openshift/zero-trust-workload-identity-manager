package utils

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2/textlogger"
)

func TestValidateCommonConfigAffinity(t *testing.T) {
	tests := []struct {
		name      string
		affinity  *corev1.Affinity
		wantError bool
	}{
		{
			name:      "nil affinity is valid",
			affinity:  nil,
			wantError: false,
		},
		{
			name: "valid node affinity with required terms",
			affinity: &corev1.Affinity{
				NodeAffinity: &corev1.NodeAffinity{
					RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
						NodeSelectorTerms: []corev1.NodeSelectorTerm{
							{
								MatchExpressions: []corev1.NodeSelectorRequirement{
									{
										Key:      "kubernetes.io/hostname",
										Operator: corev1.NodeSelectorOpIn,
										Values:   []string{"node1", "node2"},
									},
								},
							},
						},
					},
				},
			},
			wantError: false,
		},
		{
			name: "valid node affinity with preferred terms",
			affinity: &corev1.Affinity{
				NodeAffinity: &corev1.NodeAffinity{
					PreferredDuringSchedulingIgnoredDuringExecution: []corev1.PreferredSchedulingTerm{
						{
							Weight: 50,
							Preference: corev1.NodeSelectorTerm{
								MatchExpressions: []corev1.NodeSelectorRequirement{
									{
										Key:      "zone",
										Operator: corev1.NodeSelectorOpIn,
										Values:   []string{"us-east-1a"},
									},
								},
							},
						},
					},
				},
			},
			wantError: false,
		},
		{
			name: "invalid node affinity - weight too low",
			affinity: &corev1.Affinity{
				NodeAffinity: &corev1.NodeAffinity{
					PreferredDuringSchedulingIgnoredDuringExecution: []corev1.PreferredSchedulingTerm{
						{
							Weight: 0,
							Preference: corev1.NodeSelectorTerm{
								MatchExpressions: []corev1.NodeSelectorRequirement{
									{
										Key:      "zone",
										Operator: corev1.NodeSelectorOpIn,
										Values:   []string{"us-east-1a"},
									},
								},
							},
						},
					},
				},
			},
			wantError: true,
		},
		{
			name: "invalid node affinity - weight too high",
			affinity: &corev1.Affinity{
				NodeAffinity: &corev1.NodeAffinity{
					PreferredDuringSchedulingIgnoredDuringExecution: []corev1.PreferredSchedulingTerm{
						{
							Weight: 101,
							Preference: corev1.NodeSelectorTerm{
								MatchExpressions: []corev1.NodeSelectorRequirement{
									{
										Key:      "zone",
										Operator: corev1.NodeSelectorOpIn,
										Values:   []string{"us-east-1a"},
									},
								},
							},
						},
					},
				},
			},
			wantError: true,
		},
		{
			name: "invalid node affinity - empty key",
			affinity: &corev1.Affinity{
				NodeAffinity: &corev1.NodeAffinity{
					RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
						NodeSelectorTerms: []corev1.NodeSelectorTerm{
							{
								MatchExpressions: []corev1.NodeSelectorRequirement{
									{
										Key:      "",
										Operator: corev1.NodeSelectorOpIn,
										Values:   []string{"value"},
									},
								},
							},
						},
					},
				},
			},
			wantError: true,
		},
		{
			name: "invalid node affinity - In operator without values",
			affinity: &corev1.Affinity{
				NodeAffinity: &corev1.NodeAffinity{
					RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
						NodeSelectorTerms: []corev1.NodeSelectorTerm{
							{
								MatchExpressions: []corev1.NodeSelectorRequirement{
									{
										Key:      "zone",
										Operator: corev1.NodeSelectorOpIn,
										Values:   []string{},
									},
								},
							},
						},
					},
				},
			},
			wantError: true,
		},
		{
			name: "valid pod affinity",
			affinity: &corev1.Affinity{
				PodAffinity: &corev1.PodAffinity{
					RequiredDuringSchedulingIgnoredDuringExecution: []corev1.PodAffinityTerm{
						{
							TopologyKey: "kubernetes.io/hostname",
							LabelSelector: &metav1.LabelSelector{
								MatchLabels: map[string]string{
									"app": "myapp",
								},
							},
						},
					},
				},
			},
			wantError: false,
		},
		{
			name: "invalid pod affinity - empty topology key",
			affinity: &corev1.Affinity{
				PodAffinity: &corev1.PodAffinity{
					RequiredDuringSchedulingIgnoredDuringExecution: []corev1.PodAffinityTerm{
						{
							TopologyKey: "",
							LabelSelector: &metav1.LabelSelector{
								MatchLabels: map[string]string{
									"app": "myapp",
								},
							},
						},
					},
				},
			},
			wantError: true,
		},
		{
			name: "valid pod anti-affinity",
			affinity: &corev1.Affinity{
				PodAntiAffinity: &corev1.PodAntiAffinity{
					PreferredDuringSchedulingIgnoredDuringExecution: []corev1.WeightedPodAffinityTerm{
						{
							Weight: 100,
							PodAffinityTerm: corev1.PodAffinityTerm{
								TopologyKey: "kubernetes.io/hostname",
								LabelSelector: &metav1.LabelSelector{
									MatchLabels: map[string]string{
										"app": "myapp",
									},
								},
							},
						},
					},
				},
			},
			wantError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateCommonConfigAffinity(tt.affinity)
			if (err != nil) != tt.wantError {
				t.Errorf("ValidateCommonConfigAffinity() error = %v, wantError %v", err, tt.wantError)
			}
		})
	}
}

func TestValidateCommonConfigTolerations(t *testing.T) {
	tests := []struct {
		name        string
		tolerations []*corev1.Toleration
		wantError   bool
	}{
		{
			name:        "nil tolerations is valid",
			tolerations: nil,
			wantError:   false,
		},
		{
			name:        "empty tolerations is valid",
			tolerations: []*corev1.Toleration{},
			wantError:   false,
		},
		{
			name: "valid toleration with Equal operator",
			tolerations: []*corev1.Toleration{
				{
					Key:      "key1",
					Operator: corev1.TolerationOpEqual,
					Value:    "value1",
					Effect:   corev1.TaintEffectNoSchedule,
				},
			},
			wantError: false,
		},
		{
			name: "valid toleration with Exists operator",
			tolerations: []*corev1.Toleration{
				{
					Key:      "key1",
					Operator: corev1.TolerationOpExists,
					Effect:   corev1.TaintEffectNoSchedule,
				},
			},
			wantError: false,
		},
		{
			name: "valid toleration with NoExecute and TolerationSeconds",
			tolerations: []*corev1.Toleration{
				{
					Key:               "key1",
					Operator:          corev1.TolerationOpEqual,
					Value:             "value1",
					Effect:            corev1.TaintEffectNoExecute,
					TolerationSeconds: int64Ptr(300),
				},
			},
			wantError: false,
		},
		{
			name: "invalid toleration - invalid operator",
			tolerations: []*corev1.Toleration{
				{
					Key:      "key1",
					Operator: "InvalidOperator",
					Value:    "value1",
					Effect:   corev1.TaintEffectNoSchedule,
				},
			},
			wantError: true,
		},
		{
			name: "invalid toleration - invalid effect",
			tolerations: []*corev1.Toleration{
				{
					Key:      "key1",
					Operator: corev1.TolerationOpEqual,
					Value:    "value1",
					Effect:   "InvalidEffect",
				},
			},
			wantError: true,
		},
		{
			name: "invalid toleration - TolerationSeconds with non-NoExecute effect",
			tolerations: []*corev1.Toleration{
				{
					Key:               "key1",
					Operator:          corev1.TolerationOpEqual,
					Value:             "value1",
					Effect:            corev1.TaintEffectNoSchedule,
					TolerationSeconds: int64Ptr(300),
				},
			},
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateCommonConfigTolerations(tt.tolerations)
			if (err != nil) != tt.wantError {
				t.Errorf("ValidateCommonConfigTolerations() error = %v, wantError %v", err, tt.wantError)
			}
		})
	}
}

func TestValidateCommonConfigNodeSelector(t *testing.T) {
	tests := []struct {
		name         string
		nodeSelector map[string]string
		wantError    bool
	}{
		{
			name:         "nil node selector is valid",
			nodeSelector: nil,
			wantError:    false,
		},
		{
			name:         "empty node selector is valid",
			nodeSelector: map[string]string{},
			wantError:    false,
		},
		{
			name: "valid node selector",
			nodeSelector: map[string]string{
				"kubernetes.io/hostname": "node1",
				"zone":                   "us-east-1a",
			},
			wantError: false,
		},
		{
			name: "valid node selector with empty value (common for node role labels)",
			nodeSelector: map[string]string{
				"node-role.kubernetes.io/control-plane": "",
				"node-role.kubernetes.io/master":        "",
			},
			wantError: false,
		},
		{
			name: "invalid node selector - empty key",
			nodeSelector: map[string]string{
				"":     "value",
				"key1": "value1",
			},
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateCommonConfigNodeSelector(tt.nodeSelector)
			if (err != nil) != tt.wantError {
				t.Errorf("ValidateCommonConfigNodeSelector() error = %v, wantError %v", err, tt.wantError)
			}
		})
	}
}

func TestValidateCommonConfigResources(t *testing.T) {
	tests := []struct {
		name      string
		resources *corev1.ResourceRequirements
		wantError bool
	}{
		{
			name:      "nil resources is valid",
			resources: nil,
			wantError: false,
		},
		{
			name: "valid resources with requests only",
			resources: &corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("100m"),
					corev1.ResourceMemory: resource.MustParse("128Mi"),
				},
			},
			wantError: false,
		},
		{
			name: "valid resources with limits only",
			resources: &corev1.ResourceRequirements{
				Limits: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("500m"),
					corev1.ResourceMemory: resource.MustParse("512Mi"),
				},
			},
			wantError: false,
		},
		{
			name: "valid resources with requests and limits",
			resources: &corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("100m"),
					corev1.ResourceMemory: resource.MustParse("128Mi"),
				},
				Limits: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("500m"),
					corev1.ResourceMemory: resource.MustParse("512Mi"),
				},
			},
			wantError: false,
		},
		{
			name: "invalid resources - limit less than request",
			resources: &corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceCPU: resource.MustParse("500m"),
				},
				Limits: corev1.ResourceList{
					corev1.ResourceCPU: resource.MustParse("100m"),
				},
			},
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateCommonConfigResources(tt.resources)
			if (err != nil) != tt.wantError {
				t.Errorf("ValidateCommonConfigResources() error = %v, wantError %v", err, tt.wantError)
			}
		})
	}
}

func TestValidateCommonConfigLabels(t *testing.T) {
	tests := []struct {
		name      string
		labels    map[string]string
		wantError bool
	}{
		{
			name:      "nil labels is valid",
			labels:    nil,
			wantError: false,
		},
		{
			name:      "empty labels is valid",
			labels:    map[string]string{},
			wantError: false,
		},
		{
			name: "valid labels",
			labels: map[string]string{
				"app":              "myapp",
				"version":          "v1",
				"example.com/name": "value",
			},
			wantError: false,
		},
		{
			name: "invalid labels - empty key",
			labels: map[string]string{
				"": "value",
			},
			wantError: true,
		},
		{
			name: "invalid labels - value too long",
			labels: map[string]string{
				"key": "this-is-a-very-long-label-value-that-exceeds-the-maximum-allowed-length-of-sixty-three-characters",
			},
			wantError: true,
		},
		{
			name: "invalid labels - key name too long",
			labels: map[string]string{
				"this-is-a-very-long-label-key-name-that-exceeds-the-maximum-allowed-length": "value",
			},
			wantError: true,
		},
		{
			name: "invalid labels - too many slashes in key",
			labels: map[string]string{
				"example.com/namespace/name": "value",
			},
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateCommonConfigLabels(tt.labels)
			if (err != nil) != tt.wantError {
				t.Errorf("ValidateCommonConfigLabels() error = %v, wantError %v", err, tt.wantError)
			}
		})
	}
}

func TestValidateCommonConfig(t *testing.T) {
	tests := []struct {
		name         string
		affinity     *corev1.Affinity
		tolerations  []*corev1.Toleration
		nodeSelector map[string]string
		resources    *corev1.ResourceRequirements
		labels       map[string]string
		wantError    bool
	}{
		{
			name:         "all nil/empty is valid",
			affinity:     nil,
			tolerations:  nil,
			nodeSelector: nil,
			resources:    nil,
			labels:       nil,
			wantError:    false,
		},
		{
			name: "all valid configurations",
			affinity: &corev1.Affinity{
				NodeAffinity: &corev1.NodeAffinity{
					RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
						NodeSelectorTerms: []corev1.NodeSelectorTerm{
							{
								MatchExpressions: []corev1.NodeSelectorRequirement{
									{
										Key:      "kubernetes.io/hostname",
										Operator: corev1.NodeSelectorOpIn,
										Values:   []string{"node1"},
									},
								},
							},
						},
					},
				},
			},
			tolerations: []*corev1.Toleration{
				{
					Key:      "key1",
					Operator: corev1.TolerationOpEqual,
					Value:    "value1",
					Effect:   corev1.TaintEffectNoSchedule,
				},
			},
			nodeSelector: map[string]string{
				"zone": "us-east-1a",
			},
			resources: &corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceCPU: resource.MustParse("100m"),
				},
				Limits: corev1.ResourceList{
					corev1.ResourceCPU: resource.MustParse("500m"),
				},
			},
			labels: map[string]string{
				"app": "myapp",
			},
			wantError: false,
		},
		{
			name: "invalid affinity",
			affinity: &corev1.Affinity{
				PodAffinity: &corev1.PodAffinity{
					RequiredDuringSchedulingIgnoredDuringExecution: []corev1.PodAffinityTerm{
						{
							TopologyKey: "",
						},
					},
				},
			},
			tolerations:  nil,
			nodeSelector: nil,
			resources:    nil,
			labels:       nil,
			wantError:    true,
		},
		{
			name:     "invalid tolerations",
			affinity: nil,
			tolerations: []*corev1.Toleration{
				{
					Key:      "key1",
					Operator: "InvalidOperator",
				},
			},
			nodeSelector: nil,
			resources:    nil,
			labels:       nil,
			wantError:    true,
		},
		{
			name:        "invalid node selector",
			affinity:    nil,
			tolerations: nil,
			nodeSelector: map[string]string{
				"": "value",
			},
			resources: nil,
			labels:    nil,
			wantError: true,
		},
		{
			name:         "invalid resources",
			affinity:     nil,
			tolerations:  nil,
			nodeSelector: nil,
			resources: &corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceCPU: resource.MustParse("500m"),
				},
				Limits: corev1.ResourceList{
					corev1.ResourceCPU: resource.MustParse("100m"),
				},
			},
			labels:    nil,
			wantError: true,
		},
		{
			name:         "invalid labels",
			affinity:     nil,
			tolerations:  nil,
			nodeSelector: nil,
			resources:    nil,
			labels: map[string]string{
				"": "value",
			},
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateCommonConfig(tt.affinity, tt.tolerations, tt.nodeSelector, tt.resources, tt.labels)
			if (err != nil) != tt.wantError {
				t.Errorf("ValidateCommonConfig() error = %v, wantError %v", err, tt.wantError)
			}
		})
	}
}

// Helper function to create int64 pointer
func int64Ptr(i int64) *int64 {
	return &i
}

// TestValidateCommonConfigWithDetails tests the new detailed validation function
func TestValidateCommonConfigWithDetails(t *testing.T) {
	tests := []struct {
		name              string
		affinity          *corev1.Affinity
		tolerations       []*corev1.Toleration
		nodeSelector      map[string]string
		resources         *corev1.ResourceRequirements
		labels            map[string]string
		expectedResultNum int
		expectedReasons   []string
	}{
		{
			name:              "all valid - no errors",
			affinity:          nil,
			tolerations:       nil,
			nodeSelector:      nil,
			resources:         nil,
			labels:            nil,
			expectedResultNum: 0,
			expectedReasons:   []string{},
		},
		{
			name: "single error - invalid affinity",
			affinity: &corev1.Affinity{
				NodeAffinity: &corev1.NodeAffinity{
					RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
						NodeSelectorTerms: []corev1.NodeSelectorTerm{},
					},
				},
			},
			tolerations:       nil,
			nodeSelector:      nil,
			resources:         nil,
			labels:            nil,
			expectedResultNum: 1,
			expectedReasons:   []string{ConditionReasonInvalidAffinity},
		},
		{
			name:     "single error - invalid tolerations",
			affinity: nil,
			tolerations: []*corev1.Toleration{
				{
					Key:               "key1",
					Effect:            corev1.TaintEffectNoSchedule,
					TolerationSeconds: int64Ptr(300),
				},
			},
			nodeSelector:      nil,
			resources:         nil,
			labels:            nil,
			expectedResultNum: 1,
			expectedReasons:   []string{ConditionReasonInvalidTolerations},
		},
		{
			name:        "single error - invalid nodeSelector",
			affinity:    nil,
			tolerations: nil,
			nodeSelector: map[string]string{
				"": "value",
			},
			resources:         nil,
			labels:            nil,
			expectedResultNum: 1,
			expectedReasons:   []string{ConditionReasonInvalidNodeSelector},
		},
		{
			name:         "single error - invalid resources",
			affinity:     nil,
			tolerations:  nil,
			nodeSelector: nil,
			resources: &corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceCPU: resource.MustParse("500m"),
				},
				Limits: corev1.ResourceList{
					corev1.ResourceCPU: resource.MustParse("100m"),
				},
			},
			labels:            nil,
			expectedResultNum: 1,
			expectedReasons:   []string{ConditionReasonInvalidResources},
		},
		{
			name:         "single error - invalid labels",
			affinity:     nil,
			tolerations:  nil,
			nodeSelector: nil,
			resources:    nil,
			labels: map[string]string{
				"": "value",
			},
			expectedResultNum: 1,
			expectedReasons:   []string{ConditionReasonInvalidLabels},
		},
		{
			name: "multiple errors - invalid affinity and resources",
			affinity: &corev1.Affinity{
				NodeAffinity: &corev1.NodeAffinity{
					PreferredDuringSchedulingIgnoredDuringExecution: []corev1.PreferredSchedulingTerm{
						{
							Weight: 0, // Invalid weight
						},
					},
				},
			},
			tolerations:  nil,
			nodeSelector: nil,
			resources: &corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceCPU: resource.MustParse("500m"),
				},
				Limits: corev1.ResourceList{
					corev1.ResourceCPU: resource.MustParse("100m"),
				},
			},
			labels:            nil,
			expectedResultNum: 2,
			expectedReasons:   []string{ConditionReasonInvalidAffinity, ConditionReasonInvalidResources},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results := ValidateCommonConfigWithDetails(tt.affinity, tt.tolerations, tt.nodeSelector, tt.resources, tt.labels)

			if len(results) != tt.expectedResultNum {
				t.Errorf("ValidateCommonConfigWithDetails() returned %d results, expected %d", len(results), tt.expectedResultNum)
			}

			// Check that we got the expected condition reasons
			for i, expectedReason := range tt.expectedReasons {
				if i >= len(results) {
					t.Errorf("Expected reason %s but got no result at index %d", expectedReason, i)
					continue
				}
				if results[i].ConditionValue != expectedReason {
					t.Errorf("Expected condition reason %s, got %s", expectedReason, results[i].ConditionValue)
				}
				// Verify all results have the correct ConditionType
				if results[i].ConditionType != ConditionTypeConfigurationValid {
					t.Errorf("Expected condition type %s, got %s", ConditionTypeConfigurationValid, results[i].ConditionType)
				}
				// Verify error is not nil
				if results[i].Error == nil {
					t.Errorf("Expected error to be set in validation result")
				}
				// Verify error message is not empty
				if results[i].ErrorMessage == "" {
					t.Errorf("Expected error message to be set in validation result")
				}
			}
		})
	}
}

// mockStatusManager is a mock implementation of the StatusManager interface for testing
type mockStatusManager struct {
	conditions []mockCondition
}

type mockCondition struct {
	conditionType string
	reason        string
	message       string
	status        metav1.ConditionStatus
}

func (m *mockStatusManager) AddCondition(conditionType, reason, message string, status metav1.ConditionStatus) {
	m.conditions = append(m.conditions, mockCondition{
		conditionType: conditionType,
		reason:        reason,
		message:       message,
		status:        status,
	})
}

// TestValidateAndUpdateStatus tests the main generic validation function
func TestValidateAndUpdateStatus(t *testing.T) {
	tests := []struct {
		name              string
		resourceKind      string
		resourceName      string
		affinity          *corev1.Affinity
		tolerations       []*corev1.Toleration
		nodeSelector      map[string]string
		resources         *corev1.ResourceRequirements
		labels            map[string]string
		expectError       bool
		expectedCondCount int
	}{
		{
			name:              "valid configuration - no errors",
			resourceKind:      ResourceKindSpireServer,
			resourceName:      "cluster",
			affinity:          nil,
			tolerations:       nil,
			nodeSelector:      nil,
			resources:         nil,
			labels:            nil,
			expectError:       false,
			expectedCondCount: 0,
		},
		{
			name:         "invalid affinity - error logged and condition set",
			resourceKind: ResourceKindSpireAgent,
			resourceName: "cluster",
			affinity: &corev1.Affinity{
				NodeAffinity: &corev1.NodeAffinity{
					RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
						NodeSelectorTerms: []corev1.NodeSelectorTerm{},
					},
				},
			},
			tolerations:       nil,
			nodeSelector:      nil,
			resources:         nil,
			labels:            nil,
			expectError:       true,
			expectedCondCount: 1,
		},
		{
			name:         "invalid resources - error logged and condition set",
			resourceKind: ResourceKindSpiffeCSIDriver,
			resourceName: "cluster",
			affinity:     nil,
			tolerations:  nil,
			nodeSelector: nil,
			resources: &corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceCPU: resource.MustParse("500m"),
				},
				Limits: corev1.ResourceList{
					corev1.ResourceCPU: resource.MustParse("100m"),
				},
			},
			labels:            nil,
			expectError:       true,
			expectedCondCount: 1,
		},
		{
			name:         "invalid labels - error logged and condition set",
			resourceKind: ResourceKindSpireOIDCDiscoveryProvider,
			resourceName: "cluster",
			affinity:     nil,
			tolerations:  nil,
			nodeSelector: nil,
			resources:    nil,
			labels: map[string]string{
				"": "value",
			},
			expectError:       true,
			expectedCondCount: 1,
		},
		{
			name:         "multiple errors - stops at first error",
			resourceKind: ResourceKindSpireServer,
			resourceName: "cluster",
			affinity: &corev1.Affinity{
				NodeAffinity: &corev1.NodeAffinity{
					RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
						NodeSelectorTerms: []corev1.NodeSelectorTerm{},
					},
				},
			},
			tolerations:  nil,
			nodeSelector: nil,
			resources: &corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceCPU: resource.MustParse("500m"),
				},
				Limits: corev1.ResourceList{
					corev1.ResourceCPU: resource.MustParse("100m"),
				},
			},
			labels:            nil,
			expectError:       true,
			expectedCondCount: 2, // All errors are logged, but function returns after first
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := textlogger.NewLogger(textlogger.NewConfig())
			statusMgr := &mockStatusManager{}

			err := ValidateAndUpdateStatus(
				logger,
				statusMgr,
				tt.resourceKind,
				tt.resourceName,
				tt.affinity,
				tt.tolerations,
				tt.nodeSelector,
				tt.resources,
				tt.labels,
			)

			// Check error expectation
			if (err != nil) != tt.expectError {
				t.Errorf("ValidateAndUpdateStatus() error = %v, expectError %v", err, tt.expectError)
			}

			// Check that conditions were set correctly
			if len(statusMgr.conditions) != tt.expectedCondCount {
				t.Errorf("Expected %d conditions, got %d", tt.expectedCondCount, len(statusMgr.conditions))
			}

			// If there was an error, verify the error message includes the resource kind and name
			if err != nil {
				expectedPrefix := tt.resourceKind + "/" + tt.resourceName
				if !contains(err.Error(), expectedPrefix) {
					t.Errorf("Error message should contain '%s', got: %s", expectedPrefix, err.Error())
				}
			}

			// Verify condition types are correct
			for _, cond := range statusMgr.conditions {
				if cond.conditionType != ConditionTypeConfigurationValid {
					t.Errorf("Expected condition type %s, got %s", ConditionTypeConfigurationValid, cond.conditionType)
				}
			}
		})
	}
}

// TestValidateAndUpdateStatusWithConstants verifies that constants are used correctly
func TestValidateAndUpdateStatusWithConstants(t *testing.T) {
	logger := textlogger.NewLogger(textlogger.NewConfig())
	statusMgr := &mockStatusManager{}

	// Create invalid resources
	err := ValidateAndUpdateStatus(
		logger,
		statusMgr,
		ResourceKindSpireServer,
		"cluster",
		nil,
		nil,
		nil,
		&corev1.ResourceRequirements{
			Requests: corev1.ResourceList{
				corev1.ResourceCPU: resource.MustParse("500m"),
			},
			Limits: corev1.ResourceList{
				corev1.ResourceCPU: resource.MustParse("100m"),
			},
		},
		nil,
	)

	if err == nil {
		t.Fatal("Expected error but got nil")
	}

	// Verify the condition uses constants
	if len(statusMgr.conditions) != 1 {
		t.Fatalf("Expected 1 condition, got %d", len(statusMgr.conditions))
	}

	cond := statusMgr.conditions[0]
	if cond.conditionType != ConditionTypeConfigurationValid {
		t.Errorf("Expected condition type constant %s, got %s", ConditionTypeConfigurationValid, cond.conditionType)
	}
	if cond.reason != ConditionReasonInvalidResources {
		t.Errorf("Expected condition reason constant %s, got %s", ConditionReasonInvalidResources, cond.reason)
	}

	// Verify error message includes resource kind constant
	if !contains(err.Error(), ResourceKindSpireServer) {
		t.Errorf("Error should contain resource kind constant %s, got: %s", ResourceKindSpireServer, err.Error())
	}
}

// Helper function for string contains check
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && stringContains(s, substr)))
}

func stringContains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
