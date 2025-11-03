package spire_oidc_discovery_provider

import (
	"github.com/openshift/zero-trust-workload-identity-manager/api/v1alpha1"
	"github.com/openshift/zero-trust-workload-identity-manager/pkg/controller/utils"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

func buildDeployment(config *v1alpha1.SpireOIDCDiscoveryProvider, spireOidcConfigMapHash string) *appsv1.Deployment {

	// Generate standardized labels once and reuse them
	labels := utils.SpireOIDCDiscoveryProviderLabels(config.Spec.Labels)

	// For selectors, we need only the core identifying labels (without custom user labels)
	selectorLabels := map[string]string{
		"app.kubernetes.io/name":      labels["app.kubernetes.io/name"],
		"app.kubernetes.io/instance":  labels["app.kubernetes.io/instance"],
		"app.kubernetes.io/component": labels["app.kubernetes.io/component"],
	}

	replicas := int32(SpireOIDCDefaultReplicaCount)
	if config.Spec.ReplicaCount > 0 {
		replicas = int32(config.Spec.ReplicaCount)
	}

	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      SpireOIDCDeploymentName,
			Namespace: utils.OperatorNamespace,
			Labels:    labels,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: selectorLabels,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
					Annotations: map[string]string{
						spireOidcDeploymentSpireOidcConfigHashAnnotationKey: spireOidcConfigMapHash,
					},
				},
				Spec: corev1.PodSpec{
					ServiceAccountName: SpireOIDCServiceAccountName,
					Volumes: []corev1.Volume{
						{
							Name: SpireOIDCVolumeNameWorkloadAPI,
							VolumeSource: corev1.VolumeSource{
								CSI: &corev1.CSIVolumeSource{
									Driver:   SpireOIDCCSIDriverName,
									ReadOnly: boolPtr(true),
								},
							},
						},
						{
							Name:         SpireOIDCVolumeNameOIDCSockets,
							VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}},
						},
						{
							Name: SpireOIDCVolumeNameOIDCConfig,
							VolumeSource: corev1.VolumeSource{
								ConfigMap: &corev1.ConfigMapVolumeSource{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: SpireOIDCConfigMapName,
									},
								},
							},
						},
						{
							Name: SpireOIDCVolumeNameTLSCerts,
							VolumeSource: corev1.VolumeSource{
								Secret: &corev1.SecretVolumeSource{
									SecretName: SpireOIDCSecretName,
								},
							},
						},
					},
					Containers: []corev1.Container{
						{
							Name:            SpireOIDCContainerName,
							Image:           utils.GetSpireOIDCDiscoveryProviderImage(),
							ImagePullPolicy: corev1.PullIfNotPresent,
							Args:            []string{SpireOIDCConfigFlag, SpireOIDCConfigPath},
							Ports: []corev1.ContainerPort{
								{Name: SpireOIDCPortNameHealthz, ContainerPort: SpireOIDCPortHealthz},
								{Name: SpireOIDCPortNameHTTPS, ContainerPort: SpireOIDCPortHTTPS},
							},
							VolumeMounts: []corev1.VolumeMount{
								{Name: SpireOIDCVolumeNameWorkloadAPI, MountPath: SpireOIDCMountPathWorkloadAPI, ReadOnly: true},
								{Name: SpireOIDCVolumeNameOIDCSockets, MountPath: SpireOIDCMountPathOIDCSockets, ReadOnly: false},
								{Name: SpireOIDCVolumeNameOIDCConfig, MountPath: SpireOIDCMountPathOIDCConfig, ReadOnly: true},
								{Name: SpireOIDCVolumeNameTLSCerts, MountPath: SpireOIDCMountPathTLSCerts, ReadOnly: true},
							},
							ReadinessProbe: &corev1.Probe{
								ProbeHandler: corev1.ProbeHandler{
									HTTPGet: &corev1.HTTPGetAction{
										Path: SpireOIDCProbePathReady,
										Port: intstr.FromString(SpireOIDCPortNameHealthz),
									},
								},
								InitialDelaySeconds: SpireOIDCProbeInitialDelaySeconds,
								PeriodSeconds:       SpireOIDCProbePeriodSeconds,
							},
							LivenessProbe: &corev1.Probe{
								ProbeHandler: corev1.ProbeHandler{
									HTTPGet: &corev1.HTTPGetAction{
										Path: SpireOIDCProbePathLive,
										Port: intstr.FromString(SpireOIDCPortNameHealthz),
									},
								},
								InitialDelaySeconds: SpireOIDCProbeInitialDelaySeconds,
								PeriodSeconds:       SpireOIDCProbePeriodSeconds,
							},
							Resources: utils.DerefResourceRequirements(config.Spec.Resources),
						},
					},
					Affinity:     config.Spec.Affinity,
					NodeSelector: utils.DerefNodeSelector(config.Spec.NodeSelector),
					Tolerations:  utils.DerefTolerations(config.Spec.Tolerations),
				},
			},
		},
	}
}

func boolPtr(b bool) *bool {
	return &b
}
