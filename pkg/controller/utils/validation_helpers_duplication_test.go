/*
Copyright 2025.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package utils

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

func TestValidateLabelSelector(t *testing.T) {
	tests := []struct {
		name      string
		selector  *metav1.LabelSelector
		wantError bool
	}{
		{
			name:      "nil selector is valid",
			selector:  nil,
			wantError: false,
		},
		{
			name: "valid match labels",
			selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app":     "myapp",
					"version": "v1",
				},
			},
			wantError: false,
		},
		{
			name: "valid match expressions",
			selector: &metav1.LabelSelector{
				MatchExpressions: []metav1.LabelSelectorRequirement{
					{
						Key:      "app",
						Operator: metav1.LabelSelectorOpIn,
						Values:   []string{"myapp", "yourapp"},
					},
				},
			},
			wantError: false,
		},
		{
			name: "invalid match labels - empty key",
			selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"": "value",
				},
			},
			wantError: true,
		},
		{
			name: "invalid match expressions - empty key",
			selector: &metav1.LabelSelector{
				MatchExpressions: []metav1.LabelSelectorRequirement{
					{
						Key:      "",
						Operator: metav1.LabelSelectorOpIn,
						Values:   []string{"value"},
					},
				},
			},
			wantError: true,
		},
		{
			name: "invalid match expressions - In without values",
			selector: &metav1.LabelSelector{
				MatchExpressions: []metav1.LabelSelectorRequirement{
					{
						Key:      "app",
						Operator: metav1.LabelSelectorOpIn,
						Values:   []string{},
					},
				},
			},
			wantError: true,
		},
		{
			name: "invalid match expressions - Exists with values",
			selector: &metav1.LabelSelector{
				MatchExpressions: []metav1.LabelSelectorRequirement{
					{
						Key:      "app",
						Operator: metav1.LabelSelectorOpExists,
						Values:   []string{"value"},
					},
				},
			},
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateLabelSelector(tt.selector)
			if (err != nil) != tt.wantError {
				t.Errorf("validateLabelSelector() error = %v, wantError %v", err, tt.wantError)
			}
		})
	}
}

func TestValidateNodeSelectorTerm(t *testing.T) {
	tests := []struct {
		name      string
		term      *corev1.NodeSelectorTerm
		wantError bool
	}{
		{
			name:      "nil term is valid",
			term:      nil,
			wantError: false,
		},
		{
			name: "valid term with match expressions",
			term: &corev1.NodeSelectorTerm{
				MatchExpressions: []corev1.NodeSelectorRequirement{
					{
						Key:      "zone",
						Operator: corev1.NodeSelectorOpIn,
						Values:   []string{"us-east-1a"},
					},
				},
			},
			wantError: false,
		},
		{
			name: "valid term with match fields",
			term: &corev1.NodeSelectorTerm{
				MatchFields: []corev1.NodeSelectorRequirement{
					{
						Key:      "metadata.name",
						Operator: corev1.NodeSelectorOpIn,
						Values:   []string{"node1"},
					},
				},
			},
			wantError: false,
		},
		{
			name:      "invalid term - no match expressions or fields",
			term:      &corev1.NodeSelectorTerm{},
			wantError: true,
		},
		{
			name: "invalid term - invalid match expression",
			term: &corev1.NodeSelectorTerm{
				MatchExpressions: []corev1.NodeSelectorRequirement{
					{
						Key:      "",
						Operator: corev1.NodeSelectorOpIn,
						Values:   []string{"value"},
					},
				},
			},
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateNodeSelectorTerm(tt.term)
			if (err != nil) != tt.wantError {
				t.Errorf("validateNodeSelectorTerm() error = %v, wantError %v", err, tt.wantError)
			}
		})
	}
}

// Helper function to create int64 pointer
func int64Ptr(i int64) *int64 {
	return &i
}

