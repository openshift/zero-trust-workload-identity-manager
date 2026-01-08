package utils

import (
	"testing"

	securityv1 "github.com/openshift/api/security/v1"
	spiffev1alpha1 "github.com/spiffe/spire-controller-manager/api/v1alpha1"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	storagev1 "k8s.io/api/storage/v1"
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

// TestResourceNeedsUpdate tests the ResourceNeedsUpdate function
func TestResourceNeedsUpdate(t *testing.T) {
	t.Run("same labels no update needed", func(t *testing.T) {
		current := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{"app": "test"},
			},
		}
		desired := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{"app": "test"},
			},
		}
		if ResourceNeedsUpdate(current, desired) {
			t.Error("Expected false when labels are the same")
		}
	})

	t.Run("different labels need update", func(t *testing.T) {
		current := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{"app": "old"},
			},
		}
		desired := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{"app": "new"},
			},
		}
		if !ResourceNeedsUpdate(current, desired) {
			t.Error("Expected true when labels differ")
		}
	})

	t.Run("different annotations need update", func(t *testing.T) {
		current := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Annotations: map[string]string{"note": "old"},
			},
		}
		desired := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Annotations: map[string]string{"note": "new"},
			},
		}
		if !ResourceNeedsUpdate(current, desired) {
			t.Error("Expected true when annotations differ")
		}
	})
}

// TestLabelsMatch tests the LabelsMatch function
func TestLabelsMatch(t *testing.T) {
	t.Run("both nil", func(t *testing.T) {
		if !LabelsMatch(nil, nil) {
			t.Error("Expected true when both are nil")
		}
	})

	t.Run("same labels", func(t *testing.T) {
		current := map[string]string{"app": "test", "env": "prod"}
		desired := map[string]string{"app": "test", "env": "prod"}
		if !LabelsMatch(current, desired) {
			t.Error("Expected true when labels match")
		}
	})

	t.Run("current has extra labels", func(t *testing.T) {
		current := map[string]string{"app": "test", "env": "prod", "extra": "value"}
		desired := map[string]string{"app": "test", "env": "prod"}
		// Current can have extra labels, just needs to contain desired
		if !LabelsMatch(current, desired) {
			t.Error("Expected true when current contains all desired labels")
		}
	})

	t.Run("desired has extra labels", func(t *testing.T) {
		current := map[string]string{"app": "test"}
		desired := map[string]string{"app": "test", "env": "prod"}
		if LabelsMatch(current, desired) {
			t.Error("Expected false when current is missing desired labels")
		}
	})
}

// TestAnnotationsMatch tests the AnnotationsMatch function
func TestAnnotationsMatch(t *testing.T) {
	t.Run("both nil", func(t *testing.T) {
		if !AnnotationsMatch(nil, nil) {
			t.Error("Expected true when both are nil")
		}
	})

	t.Run("same annotations", func(t *testing.T) {
		current := map[string]string{"note": "test"}
		desired := map[string]string{"note": "test"}
		if !AnnotationsMatch(current, desired) {
			t.Error("Expected true when annotations match")
		}
	})

	t.Run("current has extra annotations", func(t *testing.T) {
		current := map[string]string{"note": "test", "extra": "value"}
		desired := map[string]string{"note": "test"}
		if !AnnotationsMatch(current, desired) {
			t.Error("Expected true when current contains all desired annotations")
		}
	})
}

// TestServiceNeedsUpdate tests the ServiceNeedsUpdate function
func TestServiceNeedsUpdate(t *testing.T) {
	t.Run("same service no update", func(t *testing.T) {
		current := &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{"app": "test"},
			},
			Spec: corev1.ServiceSpec{
				Ports: []corev1.ServicePort{{Port: 80}},
			},
		}
		desired := &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{"app": "test"},
			},
			Spec: corev1.ServiceSpec{
				Ports: []corev1.ServicePort{{Port: 80}},
			},
		}
		if ServiceNeedsUpdate(current, desired) {
			t.Error("Expected false when services are the same")
		}
	})

	t.Run("different ports needs update", func(t *testing.T) {
		current := &corev1.Service{
			Spec: corev1.ServiceSpec{
				Ports: []corev1.ServicePort{{Port: 80}},
			},
		}
		desired := &corev1.Service{
			Spec: corev1.ServiceSpec{
				Ports: []corev1.ServicePort{{Port: 443}},
			},
		}
		if !ServiceNeedsUpdate(current, desired) {
			t.Error("Expected true when ports differ")
		}
	})
}

// TestServiceAccountNeedsUpdate tests the ServiceAccountNeedsUpdate function
func TestServiceAccountNeedsUpdate(t *testing.T) {
	t.Run("same service account no update", func(t *testing.T) {
		current := &corev1.ServiceAccount{}
		desired := &corev1.ServiceAccount{}
		if ServiceAccountNeedsUpdate(current, desired) {
			t.Error("Expected false when service accounts are the same")
		}
	})

	t.Run("different automountServiceAccountToken", func(t *testing.T) {
		trueVal := true
		falseVal := false
		current := &corev1.ServiceAccount{
			AutomountServiceAccountToken: &trueVal,
		}
		desired := &corev1.ServiceAccount{
			AutomountServiceAccountToken: &falseVal,
		}
		if !ServiceAccountNeedsUpdate(current, desired) {
			t.Error("Expected true when AutomountServiceAccountToken differs")
		}
	})

	t.Run("different ImagePullSecrets", func(t *testing.T) {
		current := &corev1.ServiceAccount{
			ImagePullSecrets: []corev1.LocalObjectReference{{Name: "old-secret"}},
		}
		desired := &corev1.ServiceAccount{
			ImagePullSecrets: []corev1.LocalObjectReference{{Name: "new-secret"}},
		}
		if !ServiceAccountNeedsUpdate(current, desired) {
			t.Error("Expected true when ImagePullSecrets differ")
		}
	})
}

// TestClusterRoleNeedsUpdate tests the ClusterRoleNeedsUpdate function
func TestClusterRoleNeedsUpdate(t *testing.T) {
	t.Run("same cluster role no update", func(t *testing.T) {
		current := &rbacv1.ClusterRole{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{"app": "test"},
			},
			Rules: []rbacv1.PolicyRule{{
				APIGroups: []string{""},
				Resources: []string{"pods"},
				Verbs:     []string{"get", "list"},
			}},
		}
		desired := &rbacv1.ClusterRole{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{"app": "test"},
			},
			Rules: []rbacv1.PolicyRule{{
				APIGroups: []string{""},
				Resources: []string{"pods"},
				Verbs:     []string{"get", "list"},
			}},
		}
		if ClusterRoleNeedsUpdate(current, desired) {
			t.Error("Expected false when cluster roles are the same")
		}
	})

	t.Run("different rules needs update", func(t *testing.T) {
		current := &rbacv1.ClusterRole{
			Rules: []rbacv1.PolicyRule{{
				APIGroups: []string{""},
				Resources: []string{"pods"},
				Verbs:     []string{"get"},
			}},
		}
		desired := &rbacv1.ClusterRole{
			Rules: []rbacv1.PolicyRule{{
				APIGroups: []string{""},
				Resources: []string{"pods"},
				Verbs:     []string{"get", "list", "watch"},
			}},
		}
		if !ClusterRoleNeedsUpdate(current, desired) {
			t.Error("Expected true when rules differ")
		}
	})
}

// TestClusterRoleBindingNeedsUpdate tests the ClusterRoleBindingNeedsUpdate function
func TestClusterRoleBindingNeedsUpdate(t *testing.T) {
	t.Run("same cluster role binding no update", func(t *testing.T) {
		current := &rbacv1.ClusterRoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{"app": "test"},
			},
			RoleRef: rbacv1.RoleRef{
				APIGroup: "rbac.authorization.k8s.io",
				Kind:     "ClusterRole",
				Name:     "test-role",
			},
			Subjects: []rbacv1.Subject{{
				Kind:      "ServiceAccount",
				Name:      "test-sa",
				Namespace: "default",
			}},
		}
		desired := &rbacv1.ClusterRoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{"app": "test"},
			},
			RoleRef: rbacv1.RoleRef{
				APIGroup: "rbac.authorization.k8s.io",
				Kind:     "ClusterRole",
				Name:     "test-role",
			},
			Subjects: []rbacv1.Subject{{
				Kind:      "ServiceAccount",
				Name:      "test-sa",
				Namespace: "default",
			}},
		}
		if ClusterRoleBindingNeedsUpdate(current, desired) {
			t.Error("Expected false when cluster role bindings are the same")
		}
	})

	t.Run("different subjects needs update", func(t *testing.T) {
		current := &rbacv1.ClusterRoleBinding{
			Subjects: []rbacv1.Subject{{
				Kind: "ServiceAccount",
				Name: "old-sa",
			}},
		}
		desired := &rbacv1.ClusterRoleBinding{
			Subjects: []rbacv1.Subject{{
				Kind: "ServiceAccount",
				Name: "new-sa",
			}},
		}
		if !ClusterRoleBindingNeedsUpdate(current, desired) {
			t.Error("Expected true when subjects differ")
		}
	})
}

// TestRoleNeedsUpdate tests the RoleNeedsUpdate function
func TestRoleNeedsUpdate(t *testing.T) {
	t.Run("same role no update", func(t *testing.T) {
		current := &rbacv1.Role{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{"app": "test"},
			},
			Rules: []rbacv1.PolicyRule{{
				APIGroups: []string{""},
				Resources: []string{"pods"},
				Verbs:     []string{"get"},
			}},
		}
		desired := &rbacv1.Role{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{"app": "test"},
			},
			Rules: []rbacv1.PolicyRule{{
				APIGroups: []string{""},
				Resources: []string{"pods"},
				Verbs:     []string{"get"},
			}},
		}
		if RoleNeedsUpdate(current, desired) {
			t.Error("Expected false when roles are the same")
		}
	})
}

// TestRoleBindingNeedsUpdate tests the RoleBindingNeedsUpdate function
func TestRoleBindingNeedsUpdate(t *testing.T) {
	t.Run("same role binding no update", func(t *testing.T) {
		current := &rbacv1.RoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{"app": "test"},
			},
			RoleRef: rbacv1.RoleRef{
				APIGroup: "rbac.authorization.k8s.io",
				Kind:     "Role",
				Name:     "test-role",
			},
		}
		desired := &rbacv1.RoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{"app": "test"},
			},
			RoleRef: rbacv1.RoleRef{
				APIGroup: "rbac.authorization.k8s.io",
				Kind:     "Role",
				Name:     "test-role",
			},
		}
		if RoleBindingNeedsUpdate(current, desired) {
			t.Error("Expected false when role bindings are the same")
		}
	})
}

// TestCSIDriverNeedsUpdate tests the CSIDriverNeedsUpdate function
func TestCSIDriverNeedsUpdate(t *testing.T) {
	t.Run("same CSI driver no update", func(t *testing.T) {
		attachRequired := false
		current := &storagev1.CSIDriver{
			Spec: storagev1.CSIDriverSpec{
				AttachRequired: &attachRequired,
			},
		}
		desired := &storagev1.CSIDriver{
			Spec: storagev1.CSIDriverSpec{
				AttachRequired: &attachRequired,
			},
		}
		if CSIDriverNeedsUpdate(current, desired) {
			t.Error("Expected false when CSI drivers are the same")
		}
	})

	t.Run("different AttachRequired needs update", func(t *testing.T) {
		trueVal := true
		falseVal := false
		current := &storagev1.CSIDriver{
			Spec: storagev1.CSIDriverSpec{
				AttachRequired: &trueVal,
			},
		}
		desired := &storagev1.CSIDriver{
			Spec: storagev1.CSIDriverSpec{
				AttachRequired: &falseVal,
			},
		}
		if !CSIDriverNeedsUpdate(current, desired) {
			t.Error("Expected true when AttachRequired differs")
		}
	})

	t.Run("different PodInfoOnMount needs update", func(t *testing.T) {
		trueVal := true
		falseVal := false
		current := &storagev1.CSIDriver{
			Spec: storagev1.CSIDriverSpec{
				PodInfoOnMount: &trueVal,
			},
		}
		desired := &storagev1.CSIDriver{
			Spec: storagev1.CSIDriverSpec{
				PodInfoOnMount: &falseVal,
			},
		}
		if !CSIDriverNeedsUpdate(current, desired) {
			t.Error("Expected true when PodInfoOnMount differs")
		}
	})
}

// TestValidatingWebhookConfigurationNeedsUpdate tests the ValidatingWebhookConfigurationNeedsUpdate function
func TestValidatingWebhookConfigurationNeedsUpdate(t *testing.T) {
	t.Run("same webhook config no update", func(t *testing.T) {
		current := &admissionregistrationv1.ValidatingWebhookConfiguration{
			Webhooks: []admissionregistrationv1.ValidatingWebhook{},
		}
		desired := &admissionregistrationv1.ValidatingWebhookConfiguration{
			Webhooks: []admissionregistrationv1.ValidatingWebhook{},
		}
		if ValidatingWebhookConfigurationNeedsUpdate(current, desired) {
			t.Error("Expected false when webhook configs are the same")
		}
	})

	t.Run("different webhooks needs update", func(t *testing.T) {
		sideEffects := admissionregistrationv1.SideEffectClassNone
		current := &admissionregistrationv1.ValidatingWebhookConfiguration{
			Webhooks: []admissionregistrationv1.ValidatingWebhook{{
				Name:        "old.webhook.example.com",
				SideEffects: &sideEffects,
			}},
		}
		desired := &admissionregistrationv1.ValidatingWebhookConfiguration{
			Webhooks: []admissionregistrationv1.ValidatingWebhook{{
				Name:        "new.webhook.example.com",
				SideEffects: &sideEffects,
			}},
		}
		if !ValidatingWebhookConfigurationNeedsUpdate(current, desired) {
			t.Error("Expected true when webhooks differ")
		}
	})
}

// TestSecurityContextConstraintsNeedsUpdate tests the SecurityContextConstraintsNeedsUpdate function
func TestSecurityContextConstraintsNeedsUpdate(t *testing.T) {
	t.Run("same SCC no update", func(t *testing.T) {
		current := &securityv1.SecurityContextConstraints{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{"app": "test"},
			},
			AllowPrivilegedContainer: true,
			AllowHostNetwork:         false,
		}
		desired := &securityv1.SecurityContextConstraints{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{"app": "test"},
			},
			AllowPrivilegedContainer: true,
			AllowHostNetwork:         false,
		}
		if SecurityContextConstraintsNeedsUpdate(current, desired) {
			t.Error("Expected false when SCCs are the same")
		}
	})

	t.Run("different AllowPrivilegedContainer needs update", func(t *testing.T) {
		current := &securityv1.SecurityContextConstraints{
			AllowPrivilegedContainer: false,
		}
		desired := &securityv1.SecurityContextConstraints{
			AllowPrivilegedContainer: true,
		}
		if !SecurityContextConstraintsNeedsUpdate(current, desired) {
			t.Error("Expected true when AllowPrivilegedContainer differs")
		}
	})
}

// TestClusterSPIFFEIDNeedsUpdate tests the ClusterSPIFFEIDNeedsUpdate function
func TestClusterSPIFFEIDNeedsUpdate(t *testing.T) {
	t.Run("same ClusterSPIFFEID no update", func(t *testing.T) {
		current := &spiffev1alpha1.ClusterSPIFFEID{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{"app": "test"},
			},
			Spec: spiffev1alpha1.ClusterSPIFFEIDSpec{
				SPIFFEIDTemplate: "spiffe://example.org/ns/{{.PodMeta.Namespace}}/sa/{{.PodSpec.ServiceAccountName}}",
			},
		}
		desired := &spiffev1alpha1.ClusterSPIFFEID{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{"app": "test"},
			},
			Spec: spiffev1alpha1.ClusterSPIFFEIDSpec{
				SPIFFEIDTemplate: "spiffe://example.org/ns/{{.PodMeta.Namespace}}/sa/{{.PodSpec.ServiceAccountName}}",
			},
		}
		if ClusterSPIFFEIDNeedsUpdate(current, desired) {
			t.Error("Expected false when ClusterSPIFFEIDs are the same")
		}
	})

	t.Run("different SPIFFEIDTemplate needs update", func(t *testing.T) {
		current := &spiffev1alpha1.ClusterSPIFFEID{
			Spec: spiffev1alpha1.ClusterSPIFFEIDSpec{
				SPIFFEIDTemplate: "spiffe://old.org/test",
			},
		}
		desired := &spiffev1alpha1.ClusterSPIFFEID{
			Spec: spiffev1alpha1.ClusterSPIFFEIDSpec{
				SPIFFEIDTemplate: "spiffe://new.org/test",
			},
		}
		if !ClusterSPIFFEIDNeedsUpdate(current, desired) {
			t.Error("Expected true when SPIFFEIDTemplate differs")
		}
	})
}

// TestResourceNeedsUpdate_AllScenarios tests ResourceNeedsUpdate with table-driven tests
func TestResourceNeedsUpdate_AllScenarios(t *testing.T) {
	tests := []struct {
		name           string
		currentLabels  map[string]string
		desiredLabels  map[string]string
		currentAnnots  map[string]string
		desiredAnnots  map[string]string
		expectedResult bool
	}{
		{
			name:           "same labels and annotations",
			currentLabels:  map[string]string{"app": "test"},
			desiredLabels:  map[string]string{"app": "test"},
			currentAnnots:  map[string]string{"note": "test"},
			desiredAnnots:  map[string]string{"note": "test"},
			expectedResult: false,
		},
		{
			name:           "different labels",
			currentLabels:  map[string]string{"app": "old"},
			desiredLabels:  map[string]string{"app": "new"},
			currentAnnots:  nil,
			desiredAnnots:  nil,
			expectedResult: true,
		},
		{
			name:           "different annotations",
			currentLabels:  nil,
			desiredLabels:  nil,
			currentAnnots:  map[string]string{"note": "old"},
			desiredAnnots:  map[string]string{"note": "new"},
			expectedResult: true,
		},
		{
			name:           "missing label in current",
			currentLabels:  map[string]string{"app": "test"},
			desiredLabels:  map[string]string{"app": "test", "env": "prod"},
			currentAnnots:  nil,
			desiredAnnots:  nil,
			expectedResult: true,
		},
		{
			name:           "extra label in current is ok",
			currentLabels:  map[string]string{"app": "test", "extra": "value"},
			desiredLabels:  map[string]string{"app": "test"},
			currentAnnots:  nil,
			desiredAnnots:  nil,
			expectedResult: false,
		},
		{
			name:           "nil vs empty labels",
			currentLabels:  nil,
			desiredLabels:  map[string]string{},
			currentAnnots:  nil,
			desiredAnnots:  nil,
			expectedResult: false,
		},
		{
			name:           "nil vs non-empty labels",
			currentLabels:  nil,
			desiredLabels:  map[string]string{"app": "test"},
			currentAnnots:  nil,
			desiredAnnots:  nil,
			expectedResult: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			current := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Labels:      tt.currentLabels,
					Annotations: tt.currentAnnots,
				},
			}
			desired := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Labels:      tt.desiredLabels,
					Annotations: tt.desiredAnnots,
				},
			}
			result := ResourceNeedsUpdate(current, desired)
			if result != tt.expectedResult {
				t.Errorf("ResourceNeedsUpdate() = %v, expected %v", result, tt.expectedResult)
			}
		})
	}
}

// TestResourceNeedsUpdate_TypeSwitch tests ResourceNeedsUpdate for various resource types
func TestResourceNeedsUpdate_TypeSwitch(t *testing.T) {
	t.Run("StatefulSet different replicas", func(t *testing.T) {
		current := &appsv1.StatefulSet{Spec: appsv1.StatefulSetSpec{Replicas: ptr.To(int32(1))}}
		desired := &appsv1.StatefulSet{Spec: appsv1.StatefulSetSpec{Replicas: ptr.To(int32(2))}}
		if !ResourceNeedsUpdate(current, desired) {
			t.Error("Expected true for different replicas")
		}
	})

	t.Run("Deployment different replicas", func(t *testing.T) {
		current := &appsv1.Deployment{Spec: appsv1.DeploymentSpec{Replicas: ptr.To(int32(1))}}
		desired := &appsv1.Deployment{Spec: appsv1.DeploymentSpec{Replicas: ptr.To(int32(2))}}
		if !ResourceNeedsUpdate(current, desired) {
			t.Error("Expected true for different replicas")
		}
	})

	t.Run("DaemonSet different selector", func(t *testing.T) {
		current := &appsv1.DaemonSet{
			Spec: appsv1.DaemonSetSpec{
				Template: corev1.PodTemplateSpec{
					Spec: corev1.PodSpec{NodeSelector: map[string]string{"key": "old"}},
				},
			},
		}
		desired := &appsv1.DaemonSet{
			Spec: appsv1.DaemonSetSpec{
				Template: corev1.PodTemplateSpec{
					Spec: corev1.PodSpec{NodeSelector: map[string]string{"key": "new"}},
				},
			},
		}
		if !ResourceNeedsUpdate(current, desired) {
			t.Error("Expected true for different selector")
		}
	})

	t.Run("Service different type", func(t *testing.T) {
		current := &corev1.Service{Spec: corev1.ServiceSpec{Type: corev1.ServiceTypeClusterIP}}
		desired := &corev1.Service{Spec: corev1.ServiceSpec{Type: corev1.ServiceTypeNodePort}}
		if !ResourceNeedsUpdate(current, desired) {
			t.Error("Expected true for different service type")
		}
	})

	t.Run("ServiceAccount identical", func(t *testing.T) {
		current := &corev1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{Name: "test"}}
		desired := &corev1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{Name: "test"}}
		if ResourceNeedsUpdate(current, desired) {
			t.Error("Expected false for identical ServiceAccounts")
		}
	})

	t.Run("ClusterRole different rules", func(t *testing.T) {
		current := &rbacv1.ClusterRole{Rules: []rbacv1.PolicyRule{{Verbs: []string{"get"}}}}
		desired := &rbacv1.ClusterRole{Rules: []rbacv1.PolicyRule{{Verbs: []string{"get", "list"}}}}
		if !ResourceNeedsUpdate(current, desired) {
			t.Error("Expected true for different rules")
		}
	})

	t.Run("ClusterRoleBinding different role ref", func(t *testing.T) {
		current := &rbacv1.ClusterRoleBinding{RoleRef: rbacv1.RoleRef{Name: "old"}}
		desired := &rbacv1.ClusterRoleBinding{RoleRef: rbacv1.RoleRef{Name: "new"}}
		if !ResourceNeedsUpdate(current, desired) {
			t.Error("Expected true for different role ref")
		}
	})

	t.Run("Role different rules", func(t *testing.T) {
		current := &rbacv1.Role{Rules: []rbacv1.PolicyRule{{Verbs: []string{"get"}}}}
		desired := &rbacv1.Role{Rules: []rbacv1.PolicyRule{{Verbs: []string{"get", "list"}}}}
		if !ResourceNeedsUpdate(current, desired) {
			t.Error("Expected true for different rules")
		}
	})

	t.Run("RoleBinding different role ref", func(t *testing.T) {
		current := &rbacv1.RoleBinding{RoleRef: rbacv1.RoleRef{Name: "old"}}
		desired := &rbacv1.RoleBinding{RoleRef: rbacv1.RoleRef{Name: "new"}}
		if !ResourceNeedsUpdate(current, desired) {
			t.Error("Expected true for different role ref")
		}
	})
}

// TestServiceNeedsUpdate_AllScenarios tests ServiceNeedsUpdate with table-driven tests
func TestServiceNeedsUpdate_AllScenarios(t *testing.T) {
	tests := []struct {
		name           string
		currentPorts   []corev1.ServicePort
		desiredPorts   []corev1.ServicePort
		currentType    corev1.ServiceType
		desiredType    corev1.ServiceType
		expectedResult bool
	}{
		{
			name:           "same ports",
			currentPorts:   []corev1.ServicePort{{Port: 80, Protocol: corev1.ProtocolTCP}},
			desiredPorts:   []corev1.ServicePort{{Port: 80, Protocol: corev1.ProtocolTCP}},
			expectedResult: false,
		},
		{
			name:           "different ports",
			currentPorts:   []corev1.ServicePort{{Port: 80}},
			desiredPorts:   []corev1.ServicePort{{Port: 443}},
			expectedResult: true,
		},
		{
			name:           "different port count",
			currentPorts:   []corev1.ServicePort{{Port: 80}},
			desiredPorts:   []corev1.ServicePort{{Port: 80}, {Port: 443}},
			expectedResult: true,
		},
		{
			name:           "different service type",
			currentPorts:   []corev1.ServicePort{{Port: 80}},
			desiredPorts:   []corev1.ServicePort{{Port: 80}},
			currentType:    corev1.ServiceTypeClusterIP,
			desiredType:    corev1.ServiceTypeLoadBalancer,
			expectedResult: true,
		},
		{
			name:           "different target port",
			currentPorts:   []corev1.ServicePort{{Port: 80, TargetPort: intstr.FromInt(8080)}},
			desiredPorts:   []corev1.ServicePort{{Port: 80, TargetPort: intstr.FromInt(9090)}},
			expectedResult: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			current := &corev1.Service{
				Spec: corev1.ServiceSpec{
					Ports: tt.currentPorts,
					Type:  tt.currentType,
				},
			}
			desired := &corev1.Service{
				Spec: corev1.ServiceSpec{
					Ports: tt.desiredPorts,
					Type:  tt.desiredType,
				},
			}
			result := ServiceNeedsUpdate(current, desired)
			if result != tt.expectedResult {
				t.Errorf("ServiceNeedsUpdate() = %v, expected %v", result, tt.expectedResult)
			}
		})
	}
}

// TestClusterRoleNeedsUpdate_AllScenarios tests ClusterRoleNeedsUpdate with table-driven tests
func TestClusterRoleNeedsUpdate_AllScenarios(t *testing.T) {
	tests := []struct {
		name            string
		currentRules    []rbacv1.PolicyRule
		desiredRules    []rbacv1.PolicyRule
		currentAggRules *rbacv1.AggregationRule
		desiredAggRules *rbacv1.AggregationRule
		expectedResult  bool
	}{
		{
			name: "same rules",
			currentRules: []rbacv1.PolicyRule{{
				APIGroups: []string{""},
				Resources: []string{"pods"},
				Verbs:     []string{"get", "list"},
			}},
			desiredRules: []rbacv1.PolicyRule{{
				APIGroups: []string{""},
				Resources: []string{"pods"},
				Verbs:     []string{"get", "list"},
			}},
			expectedResult: false,
		},
		{
			name: "different verbs",
			currentRules: []rbacv1.PolicyRule{{
				APIGroups: []string{""},
				Resources: []string{"pods"},
				Verbs:     []string{"get"},
			}},
			desiredRules: []rbacv1.PolicyRule{{
				APIGroups: []string{""},
				Resources: []string{"pods"},
				Verbs:     []string{"get", "list", "watch"},
			}},
			expectedResult: true,
		},
		{
			name: "different rule count",
			currentRules: []rbacv1.PolicyRule{{
				APIGroups: []string{""},
				Resources: []string{"pods"},
				Verbs:     []string{"get"},
			}},
			desiredRules: []rbacv1.PolicyRule{
				{APIGroups: []string{""}, Resources: []string{"pods"}, Verbs: []string{"get"}},
				{APIGroups: []string{""}, Resources: []string{"secrets"}, Verbs: []string{"get"}},
			},
			expectedResult: true,
		},
		{
			name:         "nil vs non-nil aggregation rules",
			currentRules: []rbacv1.PolicyRule{},
			desiredRules: []rbacv1.PolicyRule{},
			currentAggRules: &rbacv1.AggregationRule{
				ClusterRoleSelectors: []metav1.LabelSelector{{
					MatchLabels: map[string]string{"test": "value"},
				}},
			},
			desiredAggRules: nil,
			expectedResult:  true,
		},
		{
			name:         "different aggregation rules",
			currentRules: []rbacv1.PolicyRule{},
			desiredRules: []rbacv1.PolicyRule{},
			currentAggRules: &rbacv1.AggregationRule{
				ClusterRoleSelectors: []metav1.LabelSelector{{
					MatchLabels: map[string]string{"test": "old"},
				}},
			},
			desiredAggRules: &rbacv1.AggregationRule{
				ClusterRoleSelectors: []metav1.LabelSelector{{
					MatchLabels: map[string]string{"test": "new"},
				}},
			},
			expectedResult: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			current := &rbacv1.ClusterRole{
				Rules:           tt.currentRules,
				AggregationRule: tt.currentAggRules,
			}
			desired := &rbacv1.ClusterRole{
				Rules:           tt.desiredRules,
				AggregationRule: tt.desiredAggRules,
			}
			result := ClusterRoleNeedsUpdate(current, desired)
			if result != tt.expectedResult {
				t.Errorf("ClusterRoleNeedsUpdate() = %v, expected %v", result, tt.expectedResult)
			}
		})
	}
}

// TestRoleBindingNeedsUpdate_AllScenarios tests RoleBindingNeedsUpdate with table-driven tests
func TestRoleBindingNeedsUpdate_AllScenarios(t *testing.T) {
	tests := []struct {
		name            string
		currentSubjects []rbacv1.Subject
		desiredSubjects []rbacv1.Subject
		currentRoleRef  rbacv1.RoleRef
		desiredRoleRef  rbacv1.RoleRef
		expectedResult  bool
	}{
		{
			name: "same role binding",
			currentSubjects: []rbacv1.Subject{{
				Kind: "ServiceAccount", Name: "test-sa", Namespace: "default",
			}},
			desiredSubjects: []rbacv1.Subject{{
				Kind: "ServiceAccount", Name: "test-sa", Namespace: "default",
			}},
			currentRoleRef: rbacv1.RoleRef{APIGroup: "rbac.authorization.k8s.io", Kind: "Role", Name: "test-role"},
			desiredRoleRef: rbacv1.RoleRef{APIGroup: "rbac.authorization.k8s.io", Kind: "Role", Name: "test-role"},
			expectedResult: false,
		},
		{
			name: "different subjects",
			currentSubjects: []rbacv1.Subject{{
				Kind: "ServiceAccount", Name: "old-sa", Namespace: "default",
			}},
			desiredSubjects: []rbacv1.Subject{{
				Kind: "ServiceAccount", Name: "new-sa", Namespace: "default",
			}},
			currentRoleRef: rbacv1.RoleRef{APIGroup: "rbac.authorization.k8s.io", Kind: "Role", Name: "test-role"},
			desiredRoleRef: rbacv1.RoleRef{APIGroup: "rbac.authorization.k8s.io", Kind: "Role", Name: "test-role"},
			expectedResult: true,
		},
		{
			name: "different role ref",
			currentSubjects: []rbacv1.Subject{{
				Kind: "ServiceAccount", Name: "test-sa", Namespace: "default",
			}},
			desiredSubjects: []rbacv1.Subject{{
				Kind: "ServiceAccount", Name: "test-sa", Namespace: "default",
			}},
			currentRoleRef: rbacv1.RoleRef{APIGroup: "rbac.authorization.k8s.io", Kind: "Role", Name: "old-role"},
			desiredRoleRef: rbacv1.RoleRef{APIGroup: "rbac.authorization.k8s.io", Kind: "Role", Name: "new-role"},
			expectedResult: true,
		},
		{
			name: "different subject count",
			currentSubjects: []rbacv1.Subject{{
				Kind: "ServiceAccount", Name: "test-sa", Namespace: "default",
			}},
			desiredSubjects: []rbacv1.Subject{
				{Kind: "ServiceAccount", Name: "test-sa", Namespace: "default"},
				{Kind: "ServiceAccount", Name: "test-sa-2", Namespace: "default"},
			},
			currentRoleRef: rbacv1.RoleRef{APIGroup: "rbac.authorization.k8s.io", Kind: "Role", Name: "test-role"},
			desiredRoleRef: rbacv1.RoleRef{APIGroup: "rbac.authorization.k8s.io", Kind: "Role", Name: "test-role"},
			expectedResult: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			current := &rbacv1.RoleBinding{
				Subjects: tt.currentSubjects,
				RoleRef:  tt.currentRoleRef,
			}
			desired := &rbacv1.RoleBinding{
				Subjects: tt.desiredSubjects,
				RoleRef:  tt.desiredRoleRef,
			}
			result := RoleBindingNeedsUpdate(current, desired)
			if result != tt.expectedResult {
				t.Errorf("RoleBindingNeedsUpdate() = %v, expected %v", result, tt.expectedResult)
			}
		})
	}
}

// TestCSIDriverNeedsUpdate_AllScenarios tests CSIDriverNeedsUpdate with table-driven tests
func TestCSIDriverNeedsUpdate_AllScenarios(t *testing.T) {
	trueVal := true
	falseVal := false
	filePolicy := storagev1.FileFSGroupPolicy
	nonePolicy := storagev1.NoneFSGroupPolicy

	tests := []struct {
		name           string
		current        *storagev1.CSIDriver
		desired        *storagev1.CSIDriver
		expectedResult bool
	}{
		{
			name: "same csi driver",
			current: &storagev1.CSIDriver{
				Spec: storagev1.CSIDriverSpec{
					AttachRequired: &falseVal,
					PodInfoOnMount: &trueVal,
				},
			},
			desired: &storagev1.CSIDriver{
				Spec: storagev1.CSIDriverSpec{
					AttachRequired: &falseVal,
					PodInfoOnMount: &trueVal,
				},
			},
			expectedResult: false,
		},
		{
			name: "different AttachRequired",
			current: &storagev1.CSIDriver{
				Spec: storagev1.CSIDriverSpec{
					AttachRequired: &trueVal,
				},
			},
			desired: &storagev1.CSIDriver{
				Spec: storagev1.CSIDriverSpec{
					AttachRequired: &falseVal,
				},
			},
			expectedResult: true,
		},
		{
			name: "different FSGroupPolicy",
			current: &storagev1.CSIDriver{
				Spec: storagev1.CSIDriverSpec{
					FSGroupPolicy: &filePolicy,
				},
			},
			desired: &storagev1.CSIDriver{
				Spec: storagev1.CSIDriverSpec{
					FSGroupPolicy: &nonePolicy,
				},
			},
			expectedResult: true,
		},
		{
			name: "nil vs non-nil FSGroupPolicy",
			current: &storagev1.CSIDriver{
				Spec: storagev1.CSIDriverSpec{
					FSGroupPolicy: nil,
				},
			},
			desired: &storagev1.CSIDriver{
				Spec: storagev1.CSIDriverSpec{
					FSGroupPolicy: &filePolicy,
				},
			},
			expectedResult: true,
		},
		{
			name: "different volume lifecycle modes",
			current: &storagev1.CSIDriver{
				Spec: storagev1.CSIDriverSpec{
					VolumeLifecycleModes: []storagev1.VolumeLifecycleMode{storagev1.VolumeLifecyclePersistent},
				},
			},
			desired: &storagev1.CSIDriver{
				Spec: storagev1.CSIDriverSpec{
					VolumeLifecycleModes: []storagev1.VolumeLifecycleMode{storagev1.VolumeLifecycleEphemeral},
				},
			},
			expectedResult: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CSIDriverNeedsUpdate(tt.current, tt.desired)
			if result != tt.expectedResult {
				t.Errorf("CSIDriverNeedsUpdate() = %v, expected %v", result, tt.expectedResult)
			}
		})
	}
}

// TestSecurityContextConstraintsNeedsUpdate_AllScenarios tests SecurityContextConstraintsNeedsUpdate
func TestSecurityContextConstraintsNeedsUpdate_AllScenarios(t *testing.T) {
	tests := []struct {
		name           string
		current        *securityv1.SecurityContextConstraints
		desired        *securityv1.SecurityContextConstraints
		expectedResult bool
	}{
		{
			name: "same SCC",
			current: &securityv1.SecurityContextConstraints{
				AllowPrivilegedContainer: true,
				AllowHostNetwork:         false,
				AllowHostPorts:           false,
				AllowHostPID:             false,
				AllowHostIPC:             false,
				ReadOnlyRootFilesystem:   false,
			},
			desired: &securityv1.SecurityContextConstraints{
				AllowPrivilegedContainer: true,
				AllowHostNetwork:         false,
				AllowHostPorts:           false,
				AllowHostPID:             false,
				AllowHostIPC:             false,
				ReadOnlyRootFilesystem:   false,
			},
			expectedResult: false,
		},
		{
			name: "different AllowHostNetwork",
			current: &securityv1.SecurityContextConstraints{
				AllowHostNetwork: false,
			},
			desired: &securityv1.SecurityContextConstraints{
				AllowHostNetwork: true,
			},
			expectedResult: true,
		},
		{
			name: "different AllowHostPorts",
			current: &securityv1.SecurityContextConstraints{
				AllowHostPorts: false,
			},
			desired: &securityv1.SecurityContextConstraints{
				AllowHostPorts: true,
			},
			expectedResult: true,
		},
		{
			name: "different AllowHostPID",
			current: &securityv1.SecurityContextConstraints{
				AllowHostPID: false,
			},
			desired: &securityv1.SecurityContextConstraints{
				AllowHostPID: true,
			},
			expectedResult: true,
		},
		{
			name: "different AllowHostIPC",
			current: &securityv1.SecurityContextConstraints{
				AllowHostIPC: false,
			},
			desired: &securityv1.SecurityContextConstraints{
				AllowHostIPC: true,
			},
			expectedResult: true,
		},
		{
			name: "different ReadOnlyRootFilesystem",
			current: &securityv1.SecurityContextConstraints{
				ReadOnlyRootFilesystem: false,
			},
			desired: &securityv1.SecurityContextConstraints{
				ReadOnlyRootFilesystem: true,
			},
			expectedResult: true,
		},
		{
			name: "different Volumes",
			current: &securityv1.SecurityContextConstraints{
				Volumes: []securityv1.FSType{securityv1.FSTypeConfigMap},
			},
			desired: &securityv1.SecurityContextConstraints{
				Volumes: []securityv1.FSType{securityv1.FSTypeSecret},
			},
			expectedResult: true,
		},
		{
			name: "different Users",
			current: &securityv1.SecurityContextConstraints{
				Users: []string{"system:serviceaccount:ns1:sa1"},
			},
			desired: &securityv1.SecurityContextConstraints{
				Users: []string{"system:serviceaccount:ns2:sa2"},
			},
			expectedResult: true,
		},
		{
			name: "different RequiredDropCapabilities",
			current: &securityv1.SecurityContextConstraints{
				RequiredDropCapabilities: []corev1.Capability{"CAP_NET_ADMIN"},
			},
			desired: &securityv1.SecurityContextConstraints{
				RequiredDropCapabilities: []corev1.Capability{"CAP_SYS_ADMIN"},
			},
			expectedResult: true,
		},
		{
			name: "different AllowedCapabilities",
			current: &securityv1.SecurityContextConstraints{
				AllowedCapabilities: []corev1.Capability{"CAP_NET_ADMIN"},
			},
			desired: &securityv1.SecurityContextConstraints{
				AllowedCapabilities: []corev1.Capability{"CAP_SYS_ADMIN"},
			},
			expectedResult: true,
		},
		{
			name: "different DefaultAddCapabilities",
			current: &securityv1.SecurityContextConstraints{
				DefaultAddCapabilities: []corev1.Capability{"CAP_NET_ADMIN"},
			},
			desired: &securityv1.SecurityContextConstraints{
				DefaultAddCapabilities: []corev1.Capability{"CAP_SYS_ADMIN"},
			},
			expectedResult: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SecurityContextConstraintsNeedsUpdate(tt.current, tt.desired)
			if result != tt.expectedResult {
				t.Errorf("SecurityContextConstraintsNeedsUpdate() = %v, expected %v", result, tt.expectedResult)
			}
		})
	}
}
