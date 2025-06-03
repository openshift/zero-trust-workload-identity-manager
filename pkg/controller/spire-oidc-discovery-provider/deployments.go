package spire_oidc_discovery_provider

import (
	"github.com/openshift/zero-trust-workload-identity-manager/api/v1alpha1"
	"github.com/openshift/zero-trust-workload-identity-manager/pkg/controller/utils"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

func buildDeployment(config *v1alpha1.SpireOIDCDiscoveryProviderConfig, spireOidcConfigMapHash string) *appsv1.Deployment {
	labels := map[string]string{
		"app.kubernetes.io/name":     "spiffe-oidc-discovery-provider",
		"app.kubernetes.io/instance": "spire",
		"component":                  "oidc-discovery-provider",
		"release":                    "spire",
		"release-namespace":          "zero-trust-workload-identity-manager",
		utils.AppManagedByLabelKey:   utils.AppManagedByLabelValue,
	}

	if config.Spec.Labels != nil {
		for k, v := range config.Spec.Labels {
			labels[k] = v
		}
	}

	replicas := int32(1)
	if config.Spec.ReplicaCount > 0 {
		replicas = int32(config.Spec.ReplicaCount)
	}

	resourceRequirements := utils.DefaultResourceRequirements()
	if config.Spec.Resources != nil && config.Spec.Resources.Limits != nil {
		resourceRequirements.Limits = config.Spec.Resources.Limits
	}
	if config.Spec.Resources != nil && config.Spec.Resources.Requests != nil {
		resourceRequirements.Requests = config.Spec.Resources.Requests
	}

	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "spire-spiffe-oidc-discovery-provider",
			Namespace: utils.OperatorNamespace,
			Labels:    labels,
			Annotations: map[string]string{
				spireOidcDeploymentSpireOidcConfigHashAnnotationKey: spireOidcConfigMapHash,
			},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app.kubernetes.io/name":     "spiffe-oidc-discovery-provider",
					"app.kubernetes.io/instance": "spire",
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels:      labels,
					Annotations: map[string]string{ // replace with actual checksum if needed
					},
				},
				Spec: corev1.PodSpec{
					ServiceAccountName: "spire-spiffe-oidc-discovery-provider",
					InitContainers: []corev1.Container{
						{
							Name:            "init",
							Image:           utils.GetSpiffeHelperImage(),
							ImagePullPolicy: corev1.PullIfNotPresent,
							Args:            []string{"-config", "/etc/spiffe-helper.conf", "-daemon-mode=false"},
							VolumeMounts: []corev1.VolumeMount{
								{Name: "spiffe-workload-api", MountPath: "/spiffe-workload-api", ReadOnly: true},
								{Name: "spire-oidc-config", MountPath: "/etc/spiffe-helper.conf", SubPath: "spiffe-helper.conf", ReadOnly: true},
								{Name: "certdir", MountPath: "/certs"},
							},
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: "spiffe-workload-api",
							VolumeSource: corev1.VolumeSource{
								CSI: &corev1.CSIVolumeSource{
									Driver:   "csi.spiffe.io",
									ReadOnly: boolPtr(true),
								},
							},
						},
						{
							Name:         "spire-oidc-sockets",
							VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}},
						},
						{
							Name: "spire-oidc-config",
							VolumeSource: corev1.VolumeSource{
								ConfigMap: &corev1.ConfigMapVolumeSource{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: "spire-spiffe-oidc-discovery-provider",
									},
								},
							},
						},
						{
							Name:         "certdir",
							VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}},
						},
						{
							Name:         "ngnix-tmp",
							VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}},
						},
					},
					Containers: []corev1.Container{
						{
							Name:            "spiffe-oidc-discovery-provider",
							Image:           utils.GetSpireOIDCDiscoveryProviderImage(),
							ImagePullPolicy: corev1.PullIfNotPresent,
							Args:            []string{"-config", "/run/spire/oidc/config/oidc-discovery-provider.conf"},
							Ports: []corev1.ContainerPort{
								{Name: "healthz", ContainerPort: 8008},
								{Name: "https", ContainerPort: 8443},
							},
							VolumeMounts: []corev1.VolumeMount{
								{Name: "spiffe-workload-api", MountPath: "/spiffe-workload-api", ReadOnly: true},
								{Name: "spire-oidc-sockets", MountPath: "/run/spire/oidc-sockets", ReadOnly: false},
								{Name: "spire-oidc-config", MountPath: "/run/spire/oidc/config/oidc-discovery-provider.conf", SubPath: "oidc-discovery-provider.conf", ReadOnly: true},
								{Name: "certdir", MountPath: "/certs", ReadOnly: true},
							},
							ReadinessProbe: &corev1.Probe{
								ProbeHandler: corev1.ProbeHandler{
									HTTPGet: &corev1.HTTPGetAction{
										Path: "/ready",
										Port: intstr.FromString("healthz"),
									},
								},
								InitialDelaySeconds: 5,
								PeriodSeconds:       5,
							},
							LivenessProbe: &corev1.Probe{
								ProbeHandler: corev1.ProbeHandler{
									HTTPGet: &corev1.HTTPGetAction{
										Path: "/live",
										Port: intstr.FromString("healthz"),
									},
								},
								InitialDelaySeconds: 5,
								PeriodSeconds:       5,
							},
						},
						{
							Name:            "spiffe-helper",
							Image:           utils.GetSpiffeHelperImage(),
							ImagePullPolicy: corev1.PullIfNotPresent,
							Args:            []string{"-config", "/etc/spiffe-helper.conf"},
							VolumeMounts: []corev1.VolumeMount{
								{Name: "spiffe-workload-api", MountPath: "/spiffe-workload-api", ReadOnly: true},
								{Name: "spire-oidc-config", MountPath: "/etc/spiffe-helper.conf", SubPath: "spiffe-helper.conf", ReadOnly: true},
								{Name: "certdir", MountPath: "/certs"},
							},
						},
					},
					Affinity:     config.Spec.Affinity,
					NodeSelector: utils.DerefNodeSelector(config.Spec.NodeSelector),
					Resources:    resourceRequirements,
					Tolerations:  utils.DerefTolerations(config.Spec.Tolerations),
				},
			},
		},
	}
}

func boolPtr(b bool) *bool {
	return &b
}
