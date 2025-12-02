package utils

import (
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"
)

func TestVolumesEqual(t *testing.T) {
	t.Run("Both empty", func(t *testing.T) {
		if !volumesEqual(nil, nil) {
			t.Error("Expected true when both are nil")
		}
		if !volumesEqual([]corev1.Volume{}, []corev1.Volume{}) {
			t.Error("Expected true when both are empty")
		}
	})

	t.Run("Length mismatch", func(t *testing.T) {
		fetched := []corev1.Volume{{Name: "vol1"}}
		desired := []corev1.Volume{}
		if volumesEqual(fetched, desired) {
			t.Error("Expected false when lengths differ")
		}
	})

	t.Run("ConfigMap volume - same", func(t *testing.T) {
		fetched := []corev1.Volume{{
			Name: "config",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{Name: "my-config"},
				},
			},
		}}
		desired := []corev1.Volume{{
			Name: "config",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{Name: "my-config"},
				},
			},
		}}
		if !volumesEqual(fetched, desired) {
			t.Error("Expected true when ConfigMap volumes are identical")
		}
	})

	t.Run("ConfigMap volume - different name", func(t *testing.T) {
		fetched := []corev1.Volume{{
			Name: "config",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{Name: "my-config"},
				},
			},
		}}
		desired := []corev1.Volume{{
			Name: "config",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{Name: "different-config"},
				},
			},
		}}
		if volumesEqual(fetched, desired) {
			t.Error("Expected false when ConfigMap names differ")
		}
	})

	t.Run("ConfigMap volume - desired has but fetched doesn't", func(t *testing.T) {
		fetched := []corev1.Volume{{
			Name:         "config",
			VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}},
		}}
		desired := []corev1.Volume{{
			Name: "config",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{Name: "my-config"},
				},
			},
		}}
		if volumesEqual(fetched, desired) {
			t.Error("Expected false when fetched doesn't have ConfigMap")
		}
	})

	t.Run("ConfigMap volume - different default mode", func(t *testing.T) {
		mode1 := int32(0644)
		mode2 := int32(0600)
		fetched := []corev1.Volume{{
			Name: "config",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{Name: "my-config"},
					DefaultMode:          &mode1,
				},
			},
		}}
		desired := []corev1.Volume{{
			Name: "config",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{Name: "my-config"},
					DefaultMode:          &mode2,
				},
			},
		}}
		if volumesEqual(fetched, desired) {
			t.Error("Expected false when ConfigMap default modes differ")
		}
	})

	t.Run("ConfigMap volume - desired DefaultMode nil does not trigger update", func(t *testing.T) {
		mode := int32(0644)
		fetched := []corev1.Volume{{
			Name: "config",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{Name: "my-config"},
					DefaultMode:          &mode,
				},
			},
		}}
		desired := []corev1.Volume{{
			Name: "config",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{Name: "my-config"},
					DefaultMode:          nil,
				},
			},
		}}
		if !volumesEqual(fetched, desired) {
			t.Error("Expected true when desired ConfigMap DefaultMode is nil")
		}
	})

	t.Run("ConfigMap volume - different items", func(t *testing.T) {
		fetched := []corev1.Volume{{
			Name: "config",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{Name: "my-config"},
					Items:                []corev1.KeyToPath{{Key: "key1", Path: "path1"}},
				},
			},
		}}
		desired := []corev1.Volume{{
			Name: "config",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{Name: "my-config"},
					Items:                []corev1.KeyToPath{{Key: "key2", Path: "path2"}},
				},
			},
		}}
		if volumesEqual(fetched, desired) {
			t.Error("Expected false when ConfigMap items differ")
		}
	})

	t.Run("Secret volume - same", func(t *testing.T) {
		mode := int32(0644)
		fetched := []corev1.Volume{{
			Name: "secret",
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName:  "my-secret",
					DefaultMode: &mode,
					Items:       []corev1.KeyToPath{{Key: "key", Path: "path"}},
				},
			},
		}}
		desired := []corev1.Volume{{
			Name: "secret",
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName:  "my-secret",
					DefaultMode: &mode,
					Items:       []corev1.KeyToPath{{Key: "key", Path: "path"}},
				},
			},
		}}
		if !volumesEqual(fetched, desired) {
			t.Error("Expected true when Secret volumes are identical")
		}
	})

	t.Run("Secret volume - different secret name", func(t *testing.T) {
		fetched := []corev1.Volume{{
			Name: "secret",
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{SecretName: "my-secret"},
			},
		}}
		desired := []corev1.Volume{{
			Name: "secret",
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{SecretName: "different-secret"},
			},
		}}
		if volumesEqual(fetched, desired) {
			t.Error("Expected false when Secret names differ")
		}
	})

	t.Run("Secret volume - different default mode", func(t *testing.T) {
		mode1 := int32(0644)
		mode2 := int32(0600)
		fetched := []corev1.Volume{{
			Name: "secret",
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{SecretName: "my-secret", DefaultMode: &mode1},
			},
		}}
		desired := []corev1.Volume{{
			Name: "secret",
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{SecretName: "my-secret", DefaultMode: &mode2},
			},
		}}
		if volumesEqual(fetched, desired) {
			t.Error("Expected false when Secret default modes differ")
		}
	})

	t.Run("Secret volume - desired DefaultMode nil does not trigger update", func(t *testing.T) {
		mode := int32(0644)
		fetched := []corev1.Volume{{
			Name: "secret",
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{SecretName: "my-secret", DefaultMode: &mode},
			},
		}}
		desired := []corev1.Volume{{
			Name: "secret",
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{SecretName: "my-secret", DefaultMode: nil},
			},
		}}
		if !volumesEqual(fetched, desired) {
			t.Error("Expected true when desired Secret DefaultMode is nil")
		}
	})

	t.Run("Secret volume - different items", func(t *testing.T) {
		fetched := []corev1.Volume{{
			Name: "secret",
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: "my-secret",
					Items:      []corev1.KeyToPath{{Key: "key1", Path: "path1"}},
				},
			},
		}}
		desired := []corev1.Volume{{
			Name: "secret",
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: "my-secret",
					Items:      []corev1.KeyToPath{{Key: "key2", Path: "path2"}},
				},
			},
		}}
		if volumesEqual(fetched, desired) {
			t.Error("Expected false when Secret items differ")
		}
	})

	t.Run("Secret volume - desired has but fetched doesn't", func(t *testing.T) {
		fetched := []corev1.Volume{{
			Name:         "secret",
			VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}},
		}}
		desired := []corev1.Volume{{
			Name: "secret",
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{SecretName: "my-secret"},
			},
		}}
		if volumesEqual(fetched, desired) {
			t.Error("Expected false when fetched doesn't have Secret")
		}
	})

	t.Run("EmptyDir volume - same", func(t *testing.T) {
		fetched := []corev1.Volume{{
			Name:         "cache",
			VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}},
		}}
		desired := []corev1.Volume{{
			Name:         "cache",
			VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}},
		}}
		if !volumesEqual(fetched, desired) {
			t.Error("Expected true when EmptyDir volumes are identical")
		}
	})

	t.Run("EmptyDir volume - desired has but fetched doesn't", func(t *testing.T) {
		fetched := []corev1.Volume{{
			Name: "cache",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{Name: "cm"},
				},
			},
		}}
		desired := []corev1.Volume{{
			Name:         "cache",
			VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}},
		}}
		if volumesEqual(fetched, desired) {
			t.Error("Expected false when fetched doesn't have EmptyDir")
		}
	})

	t.Run("HostPath volume - same", func(t *testing.T) {
		fetched := []corev1.Volume{{
			Name: "hostpath",
			VolumeSource: corev1.VolumeSource{
				HostPath: &corev1.HostPathVolumeSource{Path: "/var/run"},
			},
		}}
		desired := []corev1.Volume{{
			Name: "hostpath",
			VolumeSource: corev1.VolumeSource{
				HostPath: &corev1.HostPathVolumeSource{Path: "/var/run"},
			},
		}}
		if !volumesEqual(fetched, desired) {
			t.Error("Expected true when HostPath volumes are identical")
		}
	})

	t.Run("HostPath volume - different path", func(t *testing.T) {
		fetched := []corev1.Volume{{
			Name: "hostpath",
			VolumeSource: corev1.VolumeSource{
				HostPath: &corev1.HostPathVolumeSource{Path: "/var/run"},
			},
		}}
		desired := []corev1.Volume{{
			Name: "hostpath",
			VolumeSource: corev1.VolumeSource{
				HostPath: &corev1.HostPathVolumeSource{Path: "/var/log"},
			},
		}}
		if volumesEqual(fetched, desired) {
			t.Error("Expected false when HostPath paths differ")
		}
	})

	t.Run("HostPath volume - desired has but fetched doesn't", func(t *testing.T) {
		fetched := []corev1.Volume{{
			Name:         "hostpath",
			VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}},
		}}
		desired := []corev1.Volume{{
			Name: "hostpath",
			VolumeSource: corev1.VolumeSource{
				HostPath: &corev1.HostPathVolumeSource{Path: "/var/run"},
			},
		}}
		if volumesEqual(fetched, desired) {
			t.Error("Expected false when fetched doesn't have HostPath")
		}
	})

	t.Run("PVC volume - same", func(t *testing.T) {
		fetched := []corev1.Volume{{
			Name: "data",
			VolumeSource: corev1.VolumeSource{
				PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
					ClaimName: "my-claim",
				},
			},
		}}
		desired := []corev1.Volume{{
			Name: "data",
			VolumeSource: corev1.VolumeSource{
				PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
					ClaimName: "my-claim",
				},
			},
		}}
		if !volumesEqual(fetched, desired) {
			t.Error("Expected true when PVC volumes are identical")
		}
	})

	t.Run("PVC volume - different claim name", func(t *testing.T) {
		fetched := []corev1.Volume{{
			Name: "data",
			VolumeSource: corev1.VolumeSource{
				PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
					ClaimName: "my-claim",
				},
			},
		}}
		desired := []corev1.Volume{{
			Name: "data",
			VolumeSource: corev1.VolumeSource{
				PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
					ClaimName: "different-claim",
				},
			},
		}}
		if volumesEqual(fetched, desired) {
			t.Error("Expected false when PVC claim names differ")
		}
	})

	t.Run("PVC volume - desired has but fetched doesn't", func(t *testing.T) {
		fetched := []corev1.Volume{{
			Name:         "data",
			VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}},
		}}
		desired := []corev1.Volume{{
			Name: "data",
			VolumeSource: corev1.VolumeSource{
				PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
					ClaimName: "my-claim",
				},
			},
		}}
		if volumesEqual(fetched, desired) {
			t.Error("Expected false when fetched doesn't have PVC")
		}
	})

	t.Run("Projected volume - same", func(t *testing.T) {
		fetched := []corev1.Volume{{
			Name: "projected",
			VolumeSource: corev1.VolumeSource{
				Projected: &corev1.ProjectedVolumeSource{
					Sources: []corev1.VolumeProjection{{
						ConfigMap: &corev1.ConfigMapProjection{
							LocalObjectReference: corev1.LocalObjectReference{Name: "cm"},
						},
					}},
				},
			},
		}}
		desired := []corev1.Volume{{
			Name: "projected",
			VolumeSource: corev1.VolumeSource{
				Projected: &corev1.ProjectedVolumeSource{
					Sources: []corev1.VolumeProjection{{
						ConfigMap: &corev1.ConfigMapProjection{
							LocalObjectReference: corev1.LocalObjectReference{Name: "cm"},
						},
					}},
				},
			},
		}}
		if !volumesEqual(fetched, desired) {
			t.Error("Expected true when Projected volumes are identical")
		}
	})

	t.Run("Projected volume - different", func(t *testing.T) {
		fetched := []corev1.Volume{{
			Name: "projected",
			VolumeSource: corev1.VolumeSource{
				Projected: &corev1.ProjectedVolumeSource{
					Sources: []corev1.VolumeProjection{{
						ConfigMap: &corev1.ConfigMapProjection{
							LocalObjectReference: corev1.LocalObjectReference{Name: "cm1"},
						},
					}},
				},
			},
		}}
		desired := []corev1.Volume{{
			Name: "projected",
			VolumeSource: corev1.VolumeSource{
				Projected: &corev1.ProjectedVolumeSource{
					Sources: []corev1.VolumeProjection{{
						ConfigMap: &corev1.ConfigMapProjection{
							LocalObjectReference: corev1.LocalObjectReference{Name: "cm2"},
						},
					}},
				},
			},
		}}
		if volumesEqual(fetched, desired) {
			t.Error("Expected false when Projected volumes differ")
		}
	})

	t.Run("Projected volume - desired has but fetched doesn't", func(t *testing.T) {
		fetched := []corev1.Volume{{
			Name:         "projected",
			VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}},
		}}
		desired := []corev1.Volume{{
			Name: "projected",
			VolumeSource: corev1.VolumeSource{
				Projected: &corev1.ProjectedVolumeSource{
					Sources: []corev1.VolumeProjection{},
				},
			},
		}}
		if volumesEqual(fetched, desired) {
			t.Error("Expected false when fetched doesn't have Projected")
		}
	})

	t.Run("CSI volume - same", func(t *testing.T) {
		fetched := []corev1.Volume{{
			Name: "csi",
			VolumeSource: corev1.VolumeSource{
				CSI: &corev1.CSIVolumeSource{
					Driver:           "csi.spiffe.io",
					ReadOnly:         ptr.To(true),
					VolumeAttributes: map[string]string{"key": "value"},
				},
			},
		}}
		desired := []corev1.Volume{{
			Name: "csi",
			VolumeSource: corev1.VolumeSource{
				CSI: &corev1.CSIVolumeSource{
					Driver:           "csi.spiffe.io",
					ReadOnly:         ptr.To(true),
					VolumeAttributes: map[string]string{"key": "value"},
				},
			},
		}}
		if !volumesEqual(fetched, desired) {
			t.Error("Expected true when CSI volumes are identical")
		}
	})

	t.Run("CSI volume - different", func(t *testing.T) {
		fetched := []corev1.Volume{{
			Name: "csi",
			VolumeSource: corev1.VolumeSource{
				CSI: &corev1.CSIVolumeSource{
					Driver: "csi.spiffe.io",
				},
			},
		}}
		desired := []corev1.Volume{{
			Name: "csi",
			VolumeSource: corev1.VolumeSource{
				CSI: &corev1.CSIVolumeSource{
					Driver: "different.csi.io",
				},
			},
		}}
		if volumesEqual(fetched, desired) {
			t.Error("Expected false when CSI drivers differ")
		}
	})

	t.Run("CSI volume - desired has but fetched doesn't", func(t *testing.T) {
		fetched := []corev1.Volume{{
			Name:         "csi",
			VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}},
		}}
		desired := []corev1.Volume{{
			Name: "csi",
			VolumeSource: corev1.VolumeSource{
				CSI: &corev1.CSIVolumeSource{Driver: "csi.spiffe.io"},
			},
		}}
		if volumesEqual(fetched, desired) {
			t.Error("Expected false when fetched doesn't have CSI")
		}
	})

	t.Run("Volume name mismatch", func(t *testing.T) {
		fetched := []corev1.Volume{{
			Name:         "vol1",
			VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}},
		}}
		desired := []corev1.Volume{{
			Name:         "vol2",
			VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}},
		}}
		if volumesEqual(fetched, desired) {
			t.Error("Expected false when volume names don't match")
		}
	})
}

func TestContainerSpecModified(t *testing.T) {
	createContainer := func() *corev1.Container {
		return &corev1.Container{
			Name:            "main",
			Image:           "nginx:1.20",
			ImagePullPolicy: corev1.PullAlways,
			Command:         []string{"/bin/sh"},
			Args:            []string{"-c", "echo hello"},
			Env: []corev1.EnvVar{{
				Name:  "ENV_VAR",
				Value: "value",
			}},
			Ports: []corev1.ContainerPort{{
				ContainerPort: 8080,
				Protocol:      corev1.ProtocolTCP,
			}},
			ReadinessProbe: &corev1.Probe{
				ProbeHandler: corev1.ProbeHandler{
					HTTPGet: &corev1.HTTPGetAction{
						Path: "/ready",
						Port: intstr.FromInt(8080),
					},
				},
			},
			LivenessProbe: &corev1.Probe{
				ProbeHandler: corev1.ProbeHandler{
					HTTPGet: &corev1.HTTPGetAction{
						Path: "/health",
						Port: intstr.FromInt(8080),
					},
				},
			},
			SecurityContext: &corev1.SecurityContext{
				RunAsNonRoot: ptr.To(true),
				RunAsUser:    ptr.To(int64(1000)),
			},
			VolumeMounts: []corev1.VolumeMount{{
				Name:      "config",
				MountPath: "/etc/config",
			}},
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("100m"),
					corev1.ResourceMemory: resource.MustParse("128Mi"),
				},
			},
		}
	}

	t.Run("Identical containers", func(t *testing.T) {
		fetched := createContainer()
		desired := createContainer()
		if containerSpecModified(fetched, desired) {
			t.Error("Expected false when containers are identical")
		}
	})

	t.Run("Name modified", func(t *testing.T) {
		fetched := createContainer()
		desired := createContainer()
		fetched.Name = "different"
		if !containerSpecModified(fetched, desired) {
			t.Error("Expected true when Name differs")
		}
	})

	t.Run("Image modified", func(t *testing.T) {
		fetched := createContainer()
		desired := createContainer()
		fetched.Image = "nginx:1.21"
		if !containerSpecModified(fetched, desired) {
			t.Error("Expected true when Image differs")
		}
	})

	t.Run("ImagePullPolicy modified", func(t *testing.T) {
		fetched := createContainer()
		desired := createContainer()
		fetched.ImagePullPolicy = corev1.PullNever
		if !containerSpecModified(fetched, desired) {
			t.Error("Expected true when ImagePullPolicy differs")
		}
	})

	t.Run("Command modified", func(t *testing.T) {
		fetched := createContainer()
		desired := createContainer()
		fetched.Command = []string{"/bin/bash"}
		if !containerSpecModified(fetched, desired) {
			t.Error("Expected true when Command differs")
		}
	})

	t.Run("Args modified", func(t *testing.T) {
		fetched := createContainer()
		desired := createContainer()
		fetched.Args = []string{"different", "args"}
		if !containerSpecModified(fetched, desired) {
			t.Error("Expected true when Args differ")
		}
	})

	t.Run("Env modified", func(t *testing.T) {
		fetched := createContainer()
		desired := createContainer()
		fetched.Env[0].Value = "different"
		if !containerSpecModified(fetched, desired) {
			t.Error("Expected true when Env differs")
		}
	})

	t.Run("Ports count modified", func(t *testing.T) {
		fetched := createContainer()
		desired := createContainer()
		fetched.Ports = append(fetched.Ports, corev1.ContainerPort{
			ContainerPort: 9090,
			Protocol:      corev1.ProtocolTCP,
		})
		if !containerSpecModified(fetched, desired) {
			t.Error("Expected true when Ports count differs")
		}
	})

	t.Run("Port not found in fetched", func(t *testing.T) {
		fetched := createContainer()
		desired := createContainer()
		fetched.Ports = []corev1.ContainerPort{{
			ContainerPort: 9090,
			Protocol:      corev1.ProtocolTCP,
		}}
		if !containerSpecModified(fetched, desired) {
			t.Error("Expected true when port not found in fetched")
		}
	})

	t.Run("Port details differ", func(t *testing.T) {
		fetched := createContainer()
		desired := createContainer()
		fetched.Ports = []corev1.ContainerPort{{
			ContainerPort: 8080,
			Protocol:      corev1.ProtocolTCP,
			Name:          "different-name",
		}}
		if !containerSpecModified(fetched, desired) {
			t.Error("Expected true when port details differ")
		}
	})

	t.Run("ReadinessProbe nil vs non-nil", func(t *testing.T) {
		fetched := createContainer()
		desired := createContainer()
		fetched.ReadinessProbe = nil
		if !containerSpecModified(fetched, desired) {
			t.Error("Expected true when ReadinessProbe nil state differs")
		}
	})

	t.Run("ReadinessProbe HTTPGet modified", func(t *testing.T) {
		fetched := createContainer()
		desired := createContainer()
		fetched.ReadinessProbe.HTTPGet.Path = "/different"
		if !containerSpecModified(fetched, desired) {
			t.Error("Expected true when ReadinessProbe HTTPGet differs")
		}
	})

	t.Run("LivenessProbe nil vs non-nil", func(t *testing.T) {
		fetched := createContainer()
		desired := createContainer()
		fetched.LivenessProbe = nil
		if !containerSpecModified(fetched, desired) {
			t.Error("Expected true when LivenessProbe nil state differs")
		}
	})

	t.Run("LivenessProbe HTTPGet modified", func(t *testing.T) {
		fetched := createContainer()
		desired := createContainer()
		fetched.LivenessProbe.HTTPGet.Path = "/different"
		if !containerSpecModified(fetched, desired) {
			t.Error("Expected true when LivenessProbe HTTPGet differs")
		}
	})

	t.Run("SecurityContext nil vs non-nil", func(t *testing.T) {
		fetched := createContainer()
		desired := createContainer()
		fetched.SecurityContext = nil
		if !containerSpecModified(fetched, desired) {
			t.Error("Expected true when SecurityContext nil state differs")
		}
	})

	t.Run("SecurityContext modified", func(t *testing.T) {
		fetched := createContainer()
		desired := createContainer()
		fetched.SecurityContext.RunAsUser = ptr.To(int64(2000))
		if !containerSpecModified(fetched, desired) {
			t.Error("Expected true when SecurityContext differs")
		}
	})

	t.Run("VolumeMounts modified", func(t *testing.T) {
		fetched := createContainer()
		desired := createContainer()
		fetched.VolumeMounts[0].MountPath = "/different/path"
		if !containerSpecModified(fetched, desired) {
			t.Error("Expected true when VolumeMounts differ")
		}
	})

	t.Run("Resources modified", func(t *testing.T) {
		fetched := createContainer()
		desired := createContainer()
		fetched.Resources.Requests[corev1.ResourceCPU] = resource.MustParse("200m")
		if !containerSpecModified(fetched, desired) {
			t.Error("Expected true when Resources differ")
		}
	})
}

func TestStatefulSetNeedsUpdate(t *testing.T) {
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
						DNSPolicy:             corev1.DNSClusterFirst,
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
						Volumes: []corev1.Volume{{
							Name: "config",
							VolumeSource: corev1.VolumeSource{
								ConfigMap: &corev1.ConfigMapVolumeSource{
									LocalObjectReference: corev1.LocalObjectReference{Name: "my-config"},
								},
							},
						}},
						Containers: []corev1.Container{{
							Name:  "main",
							Image: "nginx:1.20",
						}},
						InitContainers: []corev1.Container{{
							Name:  "init",
							Image: "busybox:1.35",
						}},
					},
				},
				VolumeClaimTemplates: []corev1.PersistentVolumeClaim{{
					ObjectMeta: metav1.ObjectMeta{Name: "data"},
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
			t.Error("Expected true when desired is nil")
		}
		if !StatefulSetNeedsUpdate(nil, createStatefulSet()) {
			t.Error("Expected true when fetched is nil")
		}
	})

	t.Run("No modifications", func(t *testing.T) {
		desired := createStatefulSet()
		fetched := createStatefulSet()
		if StatefulSetNeedsUpdate(fetched, desired) {
			t.Error("Expected false when StatefulSets are identical")
		}
	})

	// StatefulSet-specific fields
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

	t.Run("VolumeClaimTemplates count modified", func(t *testing.T) {
		desired := createStatefulSet()
		fetched := createStatefulSet()
		fetched.Spec.VolumeClaimTemplates = append(fetched.Spec.VolumeClaimTemplates,
			corev1.PersistentVolumeClaim{ObjectMeta: metav1.ObjectMeta{Name: "logs"}})
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

	// Pod spec fields
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

	t.Run("DNSPolicy modified", func(t *testing.T) {
		desired := createStatefulSet()
		fetched := createStatefulSet()
		fetched.Spec.Template.Spec.DNSPolicy = corev1.DNSDefault
		if !StatefulSetNeedsUpdate(fetched, desired) {
			t.Error("Expected true when DNSPolicy differs")
		}
	})

	t.Run("DNSPolicy empty desired does not trigger update", func(t *testing.T) {
		desired := createStatefulSet()
		fetched := createStatefulSet()
		desired.Spec.Template.Spec.DNSPolicy = ""
		if StatefulSetNeedsUpdate(fetched, desired) {
			t.Error("Expected false when desired DNSPolicy is empty")
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

	t.Run("Volumes modified", func(t *testing.T) {
		desired := createStatefulSet()
		fetched := createStatefulSet()
		fetched.Spec.Template.Spec.Volumes[0].ConfigMap.Name = "different-config"
		if !StatefulSetNeedsUpdate(fetched, desired) {
			t.Error("Expected true when Volumes differ")
		}
	})

	// Container integration tests (detailed container tests are in TestContainerSpecModified)
	t.Run("Container count modified", func(t *testing.T) {
		desired := createStatefulSet()
		fetched := createStatefulSet()
		fetched.Spec.Template.Spec.Containers = append(fetched.Spec.Template.Spec.Containers,
			corev1.Container{Name: "sidecar", Image: "sidecar:latest"})
		if !StatefulSetNeedsUpdate(fetched, desired) {
			t.Error("Expected true when container count differs")
		}
	})

	t.Run("Container missing from fetched", func(t *testing.T) {
		desired := createStatefulSet()
		fetched := createStatefulSet()
		fetched.Spec.Template.Spec.Containers[0].Name = "different-main"
		if !StatefulSetNeedsUpdate(fetched, desired) {
			t.Error("Expected true when container name doesn't match")
		}
	})

	t.Run("Init container count modified", func(t *testing.T) {
		desired := createStatefulSet()
		fetched := createStatefulSet()
		fetched.Spec.Template.Spec.InitContainers = append(fetched.Spec.Template.Spec.InitContainers,
			corev1.Container{Name: "init2", Image: "busybox:latest"})
		if !StatefulSetNeedsUpdate(fetched, desired) {
			t.Error("Expected true when init container count differs")
		}
	})

	t.Run("Init container missing from fetched", func(t *testing.T) {
		desired := createStatefulSet()
		fetched := createStatefulSet()
		fetched.Spec.Template.Spec.InitContainers[0].Name = "different-init"
		if !StatefulSetNeedsUpdate(fetched, desired) {
			t.Error("Expected true when init container name doesn't match")
		}
	})
}

func TestDeploymentNeedsUpdate(t *testing.T) {
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
						DNSPolicy:             corev1.DNSClusterFirst,
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
						Volumes: []corev1.Volume{{
							Name: "config",
							VolumeSource: corev1.VolumeSource{
								ConfigMap: &corev1.ConfigMapVolumeSource{
									LocalObjectReference: corev1.LocalObjectReference{Name: "my-config"},
								},
							},
						}},
						Containers: []corev1.Container{{
							Name:  "main",
							Image: "nginx:1.20",
						}},
						InitContainers: []corev1.Container{{
							Name:  "init",
							Image: "busybox:1.35",
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
			t.Error("Expected true when desired is nil")
		}
		if !DeploymentNeedsUpdate(nil, createDeployment()) {
			t.Error("Expected true when fetched is nil")
		}
	})

	t.Run("No modifications", func(t *testing.T) {
		desired := createDeployment()
		fetched := createDeployment()
		if DeploymentNeedsUpdate(fetched, desired) {
			t.Error("Expected false when Deployments are identical")
		}
	})

	// Deployment-specific fields
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

	t.Run("Template labels modified", func(t *testing.T) {
		desired := createDeployment()
		fetched := createDeployment()
		fetched.Spec.Template.Labels["app"] = "different"
		if !DeploymentNeedsUpdate(fetched, desired) {
			t.Error("Expected true when Template.Labels differ")
		}
	})

	t.Run("DNSPolicy modified", func(t *testing.T) {
		desired := createDeployment()
		fetched := createDeployment()
		fetched.Spec.Template.Spec.DNSPolicy = corev1.DNSDefault
		if !DeploymentNeedsUpdate(fetched, desired) {
			t.Error("Expected true when DNSPolicy differs")
		}
	})

	t.Run("DNSPolicy empty desired does not trigger update", func(t *testing.T) {
		desired := createDeployment()
		fetched := createDeployment()
		desired.Spec.Template.Spec.DNSPolicy = ""
		if DeploymentNeedsUpdate(fetched, desired) {
			t.Error("Expected false when desired DNSPolicy is empty")
		}
	})

	t.Run("Volumes modified", func(t *testing.T) {
		desired := createDeployment()
		fetched := createDeployment()
		fetched.Spec.Template.Spec.Volumes[0].ConfigMap.Name = "different-config"
		if !DeploymentNeedsUpdate(fetched, desired) {
			t.Error("Expected true when Volumes differ")
		}
	})

	// Container integration tests
	t.Run("Container missing", func(t *testing.T) {
		desired := createDeployment()
		fetched := createDeployment()
		fetched.Spec.Template.Spec.Containers = []corev1.Container{}
		if !DeploymentNeedsUpdate(fetched, desired) {
			t.Error("Expected true when container is missing")
		}
	})

	t.Run("Container name not found", func(t *testing.T) {
		desired := createDeployment()
		fetched := createDeployment()
		fetched.Spec.Template.Spec.Containers[0].Name = "different-main"
		if !DeploymentNeedsUpdate(fetched, desired) {
			t.Error("Expected true when container name doesn't match")
		}
	})

	t.Run("Init container count modified", func(t *testing.T) {
		desired := createDeployment()
		fetched := createDeployment()
		fetched.Spec.Template.Spec.InitContainers = append(fetched.Spec.Template.Spec.InitContainers,
			corev1.Container{Name: "init2", Image: "busybox:latest"})
		if !DeploymentNeedsUpdate(fetched, desired) {
			t.Error("Expected true when init container count differs")
		}
	})

	t.Run("Init container missing from fetched", func(t *testing.T) {
		desired := createDeployment()
		fetched := createDeployment()
		fetched.Spec.Template.Spec.InitContainers[0].Name = "different-init"
		if !DeploymentNeedsUpdate(fetched, desired) {
			t.Error("Expected true when init container name doesn't match")
		}
	})
}

func TestDaemonSetNeedsUpdate(t *testing.T) {
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
						DNSPolicy:             corev1.DNSClusterFirst,
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
						Volumes: []corev1.Volume{{
							Name: "config",
							VolumeSource: corev1.VolumeSource{
								ConfigMap: &corev1.ConfigMapVolumeSource{
									LocalObjectReference: corev1.LocalObjectReference{Name: "my-config"},
								},
							},
						}},
						Containers: []corev1.Container{{
							Name:  "main",
							Image: "nginx:1.20",
						}},
						InitContainers: []corev1.Container{{
							Name:  "init",
							Image: "busybox:1.35",
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
			t.Error("Expected true when desired is nil")
		}
		if !DaemonSetNeedsUpdate(nil, createDaemonSet()) {
			t.Error("Expected true when fetched is nil")
		}
	})

	t.Run("No modifications", func(t *testing.T) {
		desired := createDaemonSet()
		fetched := createDaemonSet()
		if DaemonSetNeedsUpdate(fetched, desired) {
			t.Error("Expected false when DaemonSets are identical")
		}
	})

	// DaemonSet-specific fields (no Replicas)
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

	t.Run("DNSPolicy modified", func(t *testing.T) {
		desired := createDaemonSet()
		fetched := createDaemonSet()
		fetched.Spec.Template.Spec.DNSPolicy = corev1.DNSDefault
		if !DaemonSetNeedsUpdate(fetched, desired) {
			t.Error("Expected true when DNSPolicy differs")
		}
	})

	t.Run("DNSPolicy empty desired does not trigger update", func(t *testing.T) {
		desired := createDaemonSet()
		fetched := createDaemonSet()
		desired.Spec.Template.Spec.DNSPolicy = ""
		if DaemonSetNeedsUpdate(fetched, desired) {
			t.Error("Expected false when desired DNSPolicy is empty")
		}
	})

	t.Run("Volumes modified", func(t *testing.T) {
		desired := createDaemonSet()
		fetched := createDaemonSet()
		fetched.Spec.Template.Spec.Volumes[0].ConfigMap.Name = "different-config"
		if !DaemonSetNeedsUpdate(fetched, desired) {
			t.Error("Expected true when Volumes differ")
		}
	})

	// Container integration tests
	t.Run("Container missing", func(t *testing.T) {
		desired := createDaemonSet()
		fetched := createDaemonSet()
		desired.Spec.Template.Spec.Containers = append(desired.Spec.Template.Spec.Containers,
			corev1.Container{Name: "sidecar", Image: "sidecar:latest"})
		if !DaemonSetNeedsUpdate(fetched, desired) {
			t.Error("Expected true when container is missing in fetched")
		}
	})

	t.Run("Container name not found", func(t *testing.T) {
		desired := createDaemonSet()
		fetched := createDaemonSet()
		fetched.Spec.Template.Spec.Containers[0].Name = "different-main"
		if !DaemonSetNeedsUpdate(fetched, desired) {
			t.Error("Expected true when container name doesn't match")
		}
	})

	// Special test for tolerations with empty NodeSelector
	t.Run("Tolerations check with empty NodeSelector", func(t *testing.T) {
		desired := createDaemonSet()
		fetched := createDaemonSet()
		desired.Spec.Template.Spec.NodeSelector = map[string]string{}
		fetched.Spec.Template.Spec.NodeSelector = map[string]string{}
		fetched.Spec.Template.Spec.Tolerations[0].Value = "different"
		if !DaemonSetNeedsUpdate(fetched, desired) {
			t.Error("Expected true when Tolerations differ, even with empty NodeSelector")
		}
	})

	t.Run("Init container count modified", func(t *testing.T) {
		desired := createDaemonSet()
		fetched := createDaemonSet()
		fetched.Spec.Template.Spec.InitContainers = append(fetched.Spec.Template.Spec.InitContainers,
			corev1.Container{Name: "init2", Image: "busybox:latest"})
		if !DaemonSetNeedsUpdate(fetched, desired) {
			t.Error("Expected true when init container count differs")
		}
	})

	t.Run("Init container missing from fetched", func(t *testing.T) {
		desired := createDaemonSet()
		fetched := createDaemonSet()
		fetched.Spec.Template.Spec.InitContainers[0].Name = "different-init"
		if !DaemonSetNeedsUpdate(fetched, desired) {
			t.Error("Expected true when init container name doesn't match")
		}
	})
}

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
		if StatefulSetNeedsUpdate(fetched, desired) {
			t.Error("Expected false when desired replicas is nil")
		}
	})

	t.Run("Empty NodeSelector vs non-empty", func(t *testing.T) {
		desired := &appsv1.Deployment{
			Spec: appsv1.DeploymentSpec{
				Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "test"}},
				Template: corev1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"app": "test"}},
					Spec: corev1.PodSpec{
						NodeSelector: map[string]string{},
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
						NodeSelector: map[string]string{"zone": "east"},
						Containers:   []corev1.Container{{Name: "test", Image: "test"}},
					},
				},
			},
		}
		if !DeploymentNeedsUpdate(fetched, desired) {
			t.Error("Expected true when desired NodeSelector is empty but fetched has values")
		}
	})
}
