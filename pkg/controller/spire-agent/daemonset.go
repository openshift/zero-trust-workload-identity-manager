package spire_agent

import (
	"github.com/openshift/zero-trust-workload-identity-manager/api/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	"github.com/openshift/zero-trust-workload-identity-manager/pkg/controller/utils"
)

func generateSpireAgentDaemonSet(config v1alpha1.SpireAgentConfigSpec, spireAgentConfigHash string) *appsv1.DaemonSet {
	return &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "spire-agent",
			Namespace: utils.OperatorNamespace,
			Labels: map[string]string{
				"app.kubernetes.io/name":      "agent",
				"app.kubernetes.io/instance":  "spire",
				"app.kubernetes.io/version":   "1.11.2",
				"app.kubernetes.io/component": "default",
				utils.AppManagedByLabelKey:    utils.AppManagedByLabelValue,
			},
		},
		Spec: appsv1.DaemonSetSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app.kubernetes.io/name":      "agent",
					"app.kubernetes.io/instance":  "spire",
					"app.kubernetes.io/component": "default",
				},
			},
			UpdateStrategy: appsv1.DaemonSetUpdateStrategy{
				Type: appsv1.RollingUpdateDaemonSetStrategyType,
				RollingUpdate: &appsv1.RollingUpdateDaemonSet{
					MaxUnavailable: &intstr.IntOrString{IntVal: 1},
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						"kubectl.kubernetes.io/default-container":            "spire-agent",
						spireAgentDaemonSetSpireAgentConfigHashAnnotationKey: spireAgentConfigHash,
					},
					Labels: map[string]string{
						"app.kubernetes.io/name":      "agent",
						"app.kubernetes.io/instance":  "spire",
						"app.kubernetes.io/component": "default",
					},
				},
				Spec: corev1.PodSpec{
					HostPID:            true,
					HostNetwork:        true,
					DNSPolicy:          corev1.DNSClusterFirstWithHostNet,
					ServiceAccountName: "spire-agent",
					Containers: []corev1.Container{
						{
							Name:            "spire-agent",
							Image:           utils.GetSpireAgentImage(),
							ImagePullPolicy: corev1.PullIfNotPresent,
							Args:            []string{"-config", "/opt/spire/conf/agent/agent.conf"},
							Env: []corev1.EnvVar{
								{Name: "PATH", Value: "/opt/spire/bin:/bin"},
								{
									Name: "MY_NODE_NAME",
									ValueFrom: &corev1.EnvVarSource{
										FieldRef: &corev1.ObjectFieldSelector{FieldPath: "spec.nodeName"},
									},
								},
							},
							Ports: []corev1.ContainerPort{
								{Name: "healthz", ContainerPort: 9982},
							},
							LivenessProbe: &corev1.Probe{
								InitialDelaySeconds: 15,
								PeriodSeconds:       60,
								ProbeHandler: corev1.ProbeHandler{
									HTTPGet: &corev1.HTTPGetAction{
										Path: "/live",
										Port: intstr.FromString("healthz"),
									},
								},
							},
							ReadinessProbe: &corev1.Probe{
								InitialDelaySeconds: 10,
								PeriodSeconds:       30,
								ProbeHandler: corev1.ProbeHandler{
									HTTPGet: &corev1.HTTPGetAction{
										Path: "/ready",
										Port: intstr.FromString("healthz"),
									},
								},
							},
							VolumeMounts: []corev1.VolumeMount{
								{Name: "spire-config", MountPath: "/opt/spire/conf/agent", ReadOnly: true},
								{Name: "spire-agent-persistence", MountPath: "/var/lib/spire"},
								{Name: "spire-bundle", MountPath: "/run/spire/bundle", ReadOnly: true},
								{Name: "spire-agent-socket-dir", MountPath: "/tmp/spire-agent/public"},
								{Name: "spire-token", MountPath: "/var/run/secrets/tokens"},
							},
							Resources: utils.DerefResourceRequirements(config.Resources),
						},
					},
					Affinity:     config.Affinity,
					NodeSelector: utils.DerefNodeSelector(config.NodeSelector),
					Tolerations:  utils.DerefTolerations(config.Tolerations),
					Volumes: []corev1.Volume{
						{
							Name: "spire-config",
							VolumeSource: corev1.VolumeSource{
								ConfigMap: &corev1.ConfigMapVolumeSource{LocalObjectReference: corev1.LocalObjectReference{Name: "spire-agent"}},
							},
						},
						{Name: "spire-agent-admin-socket-dir", VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}}},
						{Name: "spire-agent-persistence", VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}}},
						{
							Name: "spire-bundle",
							VolumeSource: corev1.VolumeSource{
								ConfigMap: &corev1.ConfigMapVolumeSource{LocalObjectReference: corev1.LocalObjectReference{Name: "spire-bundle"}},
							},
						},
						{
							Name: "spire-token",
							VolumeSource: corev1.VolumeSource{
								Projected: &corev1.ProjectedVolumeSource{
									Sources: []corev1.VolumeProjection{
										{
											ServiceAccountToken: &corev1.ServiceAccountTokenProjection{
												Path:              "spire-agent",
												ExpirationSeconds: int64Ptr(7200),
												Audience:          "spire-server",
											},
										},
									},
								},
							},
						},
						{
							Name: "spire-agent-socket-dir",
							VolumeSource: corev1.VolumeSource{
								HostPath: &corev1.HostPathVolumeSource{
									Path: "/run/spire/agent-sockets",
									Type: hostPathTypePtr(corev1.HostPathDirectoryOrCreate),
								},
							},
						},
					},
				},
			},
		},
	}
}

func int64Ptr(val int64) *int64 {
	return &val
}

func hostPathTypePtr(t corev1.HostPathType) *corev1.HostPathType {
	return &t
}
