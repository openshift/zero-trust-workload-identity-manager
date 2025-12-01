package utils

import (
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

func TestStatefulSetNeedsUpdate(t *testing.T) {
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
		if !StatefulSetNeedsUpdate(nil, nil) {
			t.Error("Expected true when both inputs are nil")
		}
		if !StatefulSetNeedsUpdate(createStatefulSet(), nil) {
			t.Error("Expected true when fetched is nil")
		}
		if !StatefulSetNeedsUpdate(nil, createStatefulSet()) {
			t.Error("Expected true when desired is nil")
		}
	})

	t.Run("No modifications", func(t *testing.T) {
		desired := createStatefulSet()
		fetched := createStatefulSet()
		if StatefulSetNeedsUpdate(fetched, desired) {
			t.Error("Expected false when StatefulSets are identical")
		}
	})

	t.Run("Replicas modified", func(t *testing.T) {
		desired := createStatefulSet()
		fetched := createStatefulSet()
		fetched.Spec.Replicas = ptr.To(int32(5))
		if !StatefulSetNeedsUpdate(fetched, desired) {
			t.Error("Expected true when replicas differ")
		}
	})

	t.Run("ServiceName modified", func(t *testing.T) {
		desired := createStatefulSet()
		fetched := createStatefulSet()
		fetched.Spec.ServiceName = "different-service"
		if !StatefulSetNeedsUpdate(fetched, desired) {
			t.Error("Expected true when ServiceName differs")
		}
	})

	t.Run("Selector modified", func(t *testing.T) {
		desired := createStatefulSet()
		fetched := createStatefulSet()
		fetched.Spec.Selector.MatchLabels["app"] = "different"
		if !StatefulSetNeedsUpdate(fetched, desired) {
			t.Error("Expected true when Selector differs")
		}
	})

	t.Run("Template labels modified", func(t *testing.T) {
		desired := createStatefulSet()
		fetched := createStatefulSet()
		fetched.Spec.Template.Labels["app"] = "different"
		if !StatefulSetNeedsUpdate(fetched, desired) {
			t.Error("Expected true when Template.Labels differ")
		}
	})

	t.Run("Special annotations modified", func(t *testing.T) {
		desired := createStatefulSet()
		fetched := createStatefulSet()
		fetched.Spec.Template.Annotations["kubectl.kubernetes.io/default-container"] = "different"
		if !StatefulSetNeedsUpdate(fetched, desired) {
			t.Error("Expected true when special annotation differs")
		}
	})

	t.Run("ServiceAccountName modified", func(t *testing.T) {
		desired := createStatefulSet()
		fetched := createStatefulSet()
		fetched.Spec.Template.Spec.ServiceAccountName = "different-sa"
		if !StatefulSetNeedsUpdate(fetched, desired) {
			t.Error("Expected true when ServiceAccountName differs")
		}
	})

	t.Run("ShareProcessNamespace modified", func(t *testing.T) {
		desired := createStatefulSet()
		fetched := createStatefulSet()
		fetched.Spec.Template.Spec.ShareProcessNamespace = ptr.To(true)
		if !StatefulSetNeedsUpdate(fetched, desired) {
			t.Error("Expected true when ShareProcessNamespace differs")
		}
	})

	t.Run("NodeSelector modified", func(t *testing.T) {
		desired := createStatefulSet()
		fetched := createStatefulSet()
		fetched.Spec.Template.Spec.NodeSelector["zone"] = "west"
		if !StatefulSetNeedsUpdate(fetched, desired) {
			t.Error("Expected true when NodeSelector differs")
		}
	})

	t.Run("Affinity modified", func(t *testing.T) {
		desired := createStatefulSet()
		fetched := createStatefulSet()
		fetched.Spec.Template.Spec.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms[0].MatchExpressions[0].Values = []string{"west"}
		if !StatefulSetNeedsUpdate(fetched, desired) {
			t.Error("Expected true when Affinity differs")
		}
	})

	t.Run("Tolerations modified", func(t *testing.T) {
		desired := createStatefulSet()
		fetched := createStatefulSet()
		fetched.Spec.Template.Spec.Tolerations[0].Value = "different"
		if !StatefulSetNeedsUpdate(fetched, desired) {
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
		if !StatefulSetNeedsUpdate(fetched, desired) {
			t.Error("Expected true when container count differs")
		}
	})

	t.Run("Container image modified", func(t *testing.T) {
		desired := createStatefulSet()
		fetched := createStatefulSet()
		fetched.Spec.Template.Spec.Containers[0].Image = "nginx:1.21"
		if !StatefulSetNeedsUpdate(fetched, desired) {
			t.Error("Expected true when container image differs")
		}
	})

	t.Run("Container args modified", func(t *testing.T) {
		desired := createStatefulSet()
		fetched := createStatefulSet()
		fetched.Spec.Template.Spec.Containers[0].Args = []string{"--different", "arg"}
		if !StatefulSetNeedsUpdate(fetched, desired) {
			t.Error("Expected true when container args differ")
		}
	})

	t.Run("VolumeClaimTemplates count modified", func(t *testing.T) {
		desired := createStatefulSet()
		fetched := createStatefulSet()
		fetched.Spec.VolumeClaimTemplates = append(fetched.Spec.VolumeClaimTemplates, corev1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{Name: "logs"},
		})
		if !StatefulSetNeedsUpdate(fetched, desired) {
			t.Error("Expected true when VolumeClaimTemplates count differs")
		}
	})

	t.Run("VolumeClaimTemplate name modified", func(t *testing.T) {
		desired := createStatefulSet()
		fetched := createStatefulSet()
		fetched.Spec.VolumeClaimTemplates[0].Name = "different-data"
		if !StatefulSetNeedsUpdate(fetched, desired) {
			t.Error("Expected true when VolumeClaimTemplate name differs")
		}
	})
}

func TestDeploymentNeedsUpdate(t *testing.T) {
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
		if !DeploymentNeedsUpdate(nil, nil) {
			t.Error("Expected true when both inputs are nil")
		}
		if !DeploymentNeedsUpdate(createDeployment(), nil) {
			t.Error("Expected true when fetched is nil")
		}
		if !DeploymentNeedsUpdate(nil, createDeployment()) {
			t.Error("Expected true when desired is nil")
		}
	})

	t.Run("No modifications", func(t *testing.T) {
		desired := createDeployment()
		fetched := createDeployment()
		if DeploymentNeedsUpdate(fetched, desired) {
			t.Error("Expected false when Deployments are identical")
		}
	})

	t.Run("Replicas modified", func(t *testing.T) {
		desired := createDeployment()
		fetched := createDeployment()
		fetched.Spec.Replicas = ptr.To(int32(5))
		if !DeploymentNeedsUpdate(fetched, desired) {
			t.Error("Expected true when replicas differ")
		}
	})

	t.Run("Selector modified", func(t *testing.T) {
		desired := createDeployment()
		fetched := createDeployment()
		fetched.Spec.Selector.MatchLabels["app"] = "different"
		if !DeploymentNeedsUpdate(fetched, desired) {
			t.Error("Expected true when Selector differs")
		}
	})

	t.Run("Container missing", func(t *testing.T) {
		desired := createDeployment()
		fetched := createDeployment()
		// Remove container from fetched to simulate missing container
		fetched.Spec.Template.Spec.Containers = []corev1.Container{}
		if !DeploymentNeedsUpdate(fetched, desired) {
			t.Error("Expected true when container is missing")
		}
	})

	t.Run("ImagePullPolicy modified", func(t *testing.T) {
		desired := createDeployment()
		fetched := createDeployment()
		fetched.Spec.Template.Spec.Containers[0].ImagePullPolicy = corev1.PullNever
		if !DeploymentNeedsUpdate(fetched, desired) {
			t.Error("Expected true when ImagePullPolicy differs")
		}
	})

	t.Run("Environment variables modified", func(t *testing.T) {
		desired := createDeployment()
		fetched := createDeployment()
		fetched.Spec.Template.Spec.Containers[0].Env[0].Value = "different-value"
		if !DeploymentNeedsUpdate(fetched, desired) {
			t.Error("Expected true when environment variables differ")
		}
	})

	t.Run("Resources modified", func(t *testing.T) {
		desired := createDeployment()
		fetched := createDeployment()
		fetched.Spec.Template.Spec.Containers[0].Resources.Requests[corev1.ResourceCPU] = resource.MustParse("200m")
		if !DeploymentNeedsUpdate(fetched, desired) {
			t.Error("Expected true when resources differ")
		}
	})

	t.Run("VolumeMounts modified", func(t *testing.T) {
		desired := createDeployment()
		fetched := createDeployment()
		fetched.Spec.Template.Spec.Containers[0].VolumeMounts[0].MountPath = "/different/path"
		if !DeploymentNeedsUpdate(fetched, desired) {
			t.Error("Expected true when VolumeMounts differ")
		}
	})
}

func TestDaemonSetNeedsUpdate(t *testing.T) {
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
		if !DaemonSetNeedsUpdate(nil, nil) {
			t.Error("Expected true when both inputs are nil")
		}
		if !DaemonSetNeedsUpdate(createDaemonSet(), nil) {
			t.Error("Expected true when fetched is nil")
		}
		if !DaemonSetNeedsUpdate(nil, createDaemonSet()) {
			t.Error("Expected true when desired is nil")
		}
	})

	t.Run("No modifications", func(t *testing.T) {
		desired := createDaemonSet()
		fetched := createDaemonSet()
		if DaemonSetNeedsUpdate(fetched, desired) {
			t.Error("Expected false when DaemonSets are identical")
		}
	})

	t.Run("Selector modified", func(t *testing.T) {
		desired := createDaemonSet()
		fetched := createDaemonSet()
		fetched.Spec.Selector.MatchLabels["app"] = "different"
		if !DaemonSetNeedsUpdate(fetched, desired) {
			t.Error("Expected true when Selector differs")
		}
	})

	t.Run("Template labels modified", func(t *testing.T) {
		desired := createDaemonSet()
		fetched := createDaemonSet()
		fetched.Spec.Template.Labels["app"] = "different"
		if !DaemonSetNeedsUpdate(fetched, desired) {
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
		if !DaemonSetNeedsUpdate(fetched, desired) {
			t.Error("Expected true when container is missing in fetched")
		}
	})

	t.Run("Container image modified", func(t *testing.T) {
		desired := createDaemonSet()
		fetched := createDaemonSet()
		fetched.Spec.Template.Spec.Containers[0].Image = "nginx:1.21"
		if !DaemonSetNeedsUpdate(fetched, desired) {
			t.Error("Expected true when container image differs")
		}
	})

	t.Run("Container resources modified", func(t *testing.T) {
		desired := createDaemonSet()
		fetched := createDaemonSet()
		fetched.Spec.Template.Spec.Containers[0].Resources.Requests[corev1.ResourceMemory] = resource.MustParse("256Mi")
		if !DaemonSetNeedsUpdate(fetched, desired) {
			t.Error("Expected true when container resources differ")
		}
	})

	// Test that Tolerations are properly checked even when NodeSelector is empty
	t.Run("Tolerations check with empty NodeSelector", func(t *testing.T) {
		desired := createDaemonSet()
		fetched := createDaemonSet()

		// Set NodeSelector to empty but keep Tolerations
		desired.Spec.Template.Spec.NodeSelector = map[string]string{}
		fetched.Spec.Template.Spec.NodeSelector = map[string]string{}

		// Modify tolerations
		fetched.Spec.Template.Spec.Tolerations[0].Value = "different"

		// Should properly detect Tolerations difference regardless of NodeSelector
		if !DaemonSetNeedsUpdate(fetched, desired) {
			t.Error("Expected true when Tolerations differ, even with empty NodeSelector")
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
		if StatefulSetNeedsUpdate(fetched, desired) {
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
		// Should detect difference even when desired NodeSelector is empty
		// because fetched has a non-empty NodeSelector
		if !DeploymentNeedsUpdate(fetched, desired) {
			t.Error("Expected true when desired NodeSelector is empty but fetched has values")
		}
	})
}
