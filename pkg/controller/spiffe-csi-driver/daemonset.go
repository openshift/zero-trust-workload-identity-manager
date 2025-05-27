package spiffe_csi_driver

import (
	"github.com/openshift/zero-trust-workload-identity-manager/pkg/controller/utils"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

func generateSpiffeCsiDriverDaemonSet() *appsv1.DaemonSet {
	return &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "spire-spiffe-csi-driver",
			Namespace: utils.OperatorNamespace,
			Labels: map[string]string{
				"app.kubernetes.io/name":     "spiffe-csi-driver",
				"app.kubernetes.io/instance": "spire",
				utils.AppManagedByLabelKey:   utils.AppManagedByLabelValue,
			},
		},
		Spec: appsv1.DaemonSetSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app.kubernetes.io/name":     "spiffe-csi-driver",
					"app.kubernetes.io/instance": "spire",
				},
			},
			UpdateStrategy: appsv1.DaemonSetUpdateStrategy{
				Type: appsv1.RollingUpdateDaemonSetStrategyType,
				RollingUpdate: &appsv1.RollingUpdateDaemonSet{
					MaxUnavailable: &intstr.IntOrString{
						Type:   intstr.Int,
						IntVal: 1,
					},
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app.kubernetes.io/name":     "spiffe-csi-driver",
						"app.kubernetes.io/instance": "spire",
					},
				},
				Spec: corev1.PodSpec{
					ServiceAccountName: "spire-spiffe-csi-driver",
					InitContainers: []corev1.Container{
						{
							Name:  "set-context",
							Image: "registry.access.redhat.com/ubi9:latest",
							Command: []string{
								"chcon", "-Rvt", "container_file_t", "spire-agent-socket/",
							},
							ImagePullPolicy: corev1.PullAlways,
							SecurityContext: &corev1.SecurityContext{
								Privileged: boolPtr(true),
								Capabilities: &corev1.Capabilities{
									Drop: []corev1.Capability{"all"},
								},
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "spire-agent-socket-dir",
									MountPath: "/spire-agent-socket",
								},
							},
							TerminationMessagePath:   "/dev/termination-log",
							TerminationMessagePolicy: corev1.TerminationMessageReadFile,
						},
					},
					Containers: []corev1.Container{
						{
							Name:  "spiffe-csi-driver",
							Image: utils.GetSpiffeCSIDriverImage(),
							Args: []string{
								"-workload-api-socket-dir", "/spire-agent-socket",
								"-plugin-name", "csi.spiffe.io",
								"-csi-socket-path", "/spiffe-csi/csi.sock",
							},
							ImagePullPolicy: corev1.PullIfNotPresent,
							Env: []corev1.EnvVar{
								{
									Name: "MY_NODE_NAME",
									ValueFrom: &corev1.EnvVarSource{
										FieldRef: &corev1.ObjectFieldSelector{
											FieldPath: "spec.nodeName",
										},
									},
								},
							},
							SecurityContext: &corev1.SecurityContext{
								ReadOnlyRootFilesystem: boolPtr(true),
								Privileged:             boolPtr(true),
								Capabilities: &corev1.Capabilities{
									Drop: []corev1.Capability{"all"},
								},
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "spire-agent-socket-dir",
									MountPath: "/spire-agent-socket",
									ReadOnly:  true,
								},
								{
									Name:      "spiffe-csi-socket-dir",
									MountPath: "/spiffe-csi",
								},
								{
									Name:             "mountpoint-dir",
									MountPath:        "/var/lib/kubelet/pods",
									MountPropagation: mountPropagationPtr(corev1.MountPropagationBidirectional),
								},
							},
						},
						{
							Name:  "node-driver-registrar",
							Image: utils.GetNodeDriverRegistrarImage(),
							Args: []string{
								"-csi-address", "/spiffe-csi/csi.sock",
								"-kubelet-registration-path", "/var/lib/kubelet/plugins/csi.spiffe.io/csi.sock",
								"-health-port", "9809",
							},
							ImagePullPolicy: corev1.PullIfNotPresent,
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "spiffe-csi-socket-dir",
									MountPath: "/spiffe-csi",
								},
								{
									Name:      "kubelet-plugin-registration-dir",
									MountPath: "/registration",
								},
							},
							Ports: []corev1.ContainerPort{
								{
									ContainerPort: 9809,
									Name:          "healthz",
								},
							},
							LivenessProbe: &corev1.Probe{
								InitialDelaySeconds: 5,
								TimeoutSeconds:      5,
								ProbeHandler: corev1.ProbeHandler{
									HTTPGet: &corev1.HTTPGetAction{
										Path: "/healthz",
										Port: intstr.FromString("healthz"),
									},
								},
							},
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: "spire-agent-socket-dir",
							VolumeSource: corev1.VolumeSource{
								HostPath: &corev1.HostPathVolumeSource{
									Path: "/run/spire/agent-sockets",
									Type: hostPathTypePtr(corev1.HostPathDirectoryOrCreate),
								},
							},
						},
						{
							Name: "spiffe-csi-socket-dir",
							VolumeSource: corev1.VolumeSource{
								HostPath: &corev1.HostPathVolumeSource{
									Path: "/var/lib/kubelet/plugins/csi.spiffe.io",
									Type: hostPathTypePtr(corev1.HostPathDirectoryOrCreate),
								},
							},
						},
						{
							Name: "mountpoint-dir",
							VolumeSource: corev1.VolumeSource{
								HostPath: &corev1.HostPathVolumeSource{
									Path: "/var/lib/kubelet/pods",
									Type: hostPathTypePtr(corev1.HostPathDirectory),
								},
							},
						},
						{
							Name: "kubelet-plugin-registration-dir",
							VolumeSource: corev1.VolumeSource{
								HostPath: &corev1.HostPathVolumeSource{
									Path: "/var/lib/kubelet/plugins_registry",
									Type: hostPathTypePtr(corev1.HostPathDirectory),
								},
							},
						},
					},
				},
			},
		},
	}
}

func boolPtr(b bool) *bool {
	return &b
}

func hostPathTypePtr(t corev1.HostPathType) *corev1.HostPathType {
	return &t
}

func mountPropagationPtr(mp corev1.MountPropagationMode) *corev1.MountPropagationMode {
	return &mp
}
