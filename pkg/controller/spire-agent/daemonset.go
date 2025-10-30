package spire_agent

import (
	"github.com/openshift/zero-trust-workload-identity-manager/api/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	"github.com/openshift/zero-trust-workload-identity-manager/pkg/controller/utils"
)

func generateSpireAgentDaemonSet(config v1alpha1.SpireAgentSpec, spireAgentConfigHash string) *appsv1.DaemonSet {

	// Generate standardized labels once and reuse them
	labels := utils.SpireAgentLabels(config.Labels)

	// For selectors, we need only the core identifying labels (without custom user labels)
	selectorLabels := map[string]string{
		"app.kubernetes.io/name":      labels["app.kubernetes.io/name"],
		"app.kubernetes.io/instance":  labels["app.kubernetes.io/instance"],
		"app.kubernetes.io/component": labels["app.kubernetes.io/component"],
	}

	// Check if we need to mount kubelet PKI directory for verification
	needsKubeletPKI := false
	kubeletPKIPath := ""
	if config.WorkloadAttestors != nil && config.WorkloadAttestors.WorkloadAttestorsVerification != nil {
		verification := config.WorkloadAttestors.WorkloadAttestorsVerification
		// Only mount kubelet PKI for hostCert mode
		if verification.Type == "hostCert" && verification.HostCertBasePath != "" {
			needsKubeletPKI = true
			kubeletPKIPath = verification.HostCertBasePath
		}
	}

	ds := &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      SpireAgentDaemonSetName,
			Namespace: utils.OperatorNamespace,
			Labels:    labels,
		},
		Spec: appsv1.DaemonSetSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: selectorLabels,
			},
			UpdateStrategy: appsv1.DaemonSetUpdateStrategy{
				Type: appsv1.RollingUpdateDaemonSetStrategyType,
				RollingUpdate: &appsv1.RollingUpdateDaemonSet{
					MaxUnavailable: &intstr.IntOrString{IntVal: SpireAgentMaxUnavailable},
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						SpireAgentAnnotationDefaultContainer:                 SpireAgentContainerName,
						spireAgentDaemonSetSpireAgentConfigHashAnnotationKey: spireAgentConfigHash,
					},
					Labels: labels,
				},
				Spec: corev1.PodSpec{
					HostPID:            true,
					HostNetwork:        true,
					DNSPolicy:          corev1.DNSClusterFirstWithHostNet,
					ServiceAccountName: SpireAgentServiceAccountName,
					Containers: []corev1.Container{
						{
							Name:            SpireAgentContainerName,
							Image:           utils.GetSpireAgentImage(),
							ImagePullPolicy: corev1.PullIfNotPresent,
							Args:            []string{SpireAgentArgConfig, SpireAgentConfigPath},
							Env: []corev1.EnvVar{
								{Name: SpireAgentEnvPath, Value: SpireAgentEnvPathValue},
								{
									Name: SpireAgentEnvNodeName,
									ValueFrom: &corev1.EnvVarSource{
										FieldRef: &corev1.ObjectFieldSelector{FieldPath: "spec.nodeName"},
									},
								},
							},
							Ports: []corev1.ContainerPort{
								{Name: SpireAgentPortNameHealthz, ContainerPort: SpireAgentPortHealthz},
							},
							LivenessProbe: &corev1.Probe{
								InitialDelaySeconds: SpireAgentLivenessInitialDelay,
								PeriodSeconds:       SpireAgentLivenessPeriod,
								ProbeHandler: corev1.ProbeHandler{
									HTTPGet: &corev1.HTTPGetAction{
										Path: SpireAgentProbePathLive,
										Port: intstr.FromString(SpireAgentPortNameHealthz),
									},
								},
							},
							ReadinessProbe: &corev1.Probe{
								InitialDelaySeconds: SpireAgentReadinessInitialDelay,
								PeriodSeconds:       SpireAgentReadinessPeriod,
								ProbeHandler: corev1.ProbeHandler{
									HTTPGet: &corev1.HTTPGetAction{
										Path: SpireAgentProbePathReady,
										Port: intstr.FromString(SpireAgentPortNameHealthz),
									},
								},
							},
							VolumeMounts: func() []corev1.VolumeMount {
								mounts := []corev1.VolumeMount{
									{Name: SpireAgentVolumeNameConfig, MountPath: SpireAgentMountPathConfig, ReadOnly: true},
									{Name: SpireAgentVolumeNamePersistence, MountPath: SpireAgentMountPathPersistence},
									{Name: SpireAgentVolumeNameBundle, MountPath: SpireAgentMountPathBundle, ReadOnly: true},
									{Name: SpireAgentVolumeNameSocketDir, MountPath: SpireAgentMountPathSocketDir},
									{Name: SpireAgentVolumeNameToken, MountPath: SpireAgentMountPathToken},
								}
								if needsKubeletPKI {
									mounts = append(mounts, corev1.VolumeMount{
										Name:      SpireAgentVolumeNameKubeletPKI,
										MountPath: kubeletPKIPath,
										ReadOnly:  true,
									})
								}
								return mounts
							}(),
							Resources: utils.DerefResourceRequirements(config.Resources),
						},
					},
					Affinity:     config.Affinity,
					NodeSelector: utils.DerefNodeSelector(config.NodeSelector),
					Tolerations:  utils.DerefTolerations(config.Tolerations),
					Volumes: func() []corev1.Volume {
						volumes := []corev1.Volume{
							{
								Name: SpireAgentVolumeNameConfig,
								VolumeSource: corev1.VolumeSource{
									ConfigMap: &corev1.ConfigMapVolumeSource{LocalObjectReference: corev1.LocalObjectReference{Name: SpireAgentConfigMapNameAgent}},
								},
							},
							{Name: SpireAgentVolumeNameAdminSocketDir, VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}}},
							{Name: SpireAgentVolumeNamePersistence, VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}}},
							{
								Name: SpireAgentVolumeNameBundle,
								VolumeSource: corev1.VolumeSource{
									ConfigMap: &corev1.ConfigMapVolumeSource{LocalObjectReference: corev1.LocalObjectReference{Name: SpireAgentConfigMapNameBundle}},
								},
							},
							{
								Name: SpireAgentVolumeNameToken,
								VolumeSource: corev1.VolumeSource{
									Projected: &corev1.ProjectedVolumeSource{
										Sources: []corev1.VolumeProjection{
											{
												ServiceAccountToken: &corev1.ServiceAccountTokenProjection{
													Path:              SpireAgentTokenPath,
													ExpirationSeconds: int64Ptr(SpireAgentTokenExpirationSeconds),
													Audience:          SpireAgentTokenAudience,
												},
											},
										},
									},
								},
							},
							{
								Name: SpireAgentVolumeNameSocketDir,
								VolumeSource: corev1.VolumeSource{
									HostPath: &corev1.HostPathVolumeSource{
										Path: SpireAgentHostPathAgentSockets,
										Type: hostPathTypePtr(corev1.HostPathDirectoryOrCreate),
									},
								},
							},
						}
						if needsKubeletPKI {
							volumes = append(volumes, corev1.Volume{
								Name: SpireAgentVolumeNameKubeletPKI,
								VolumeSource: corev1.VolumeSource{
									HostPath: &corev1.HostPathVolumeSource{
										Path: kubeletPKIPath,
										Type: hostPathTypePtr(corev1.HostPathDirectory),
									},
								},
							})
						}
						return volumes
					}(),
				},
			},
		},
	}

	return ds
}

func int64Ptr(val int64) *int64 {
	return &val
}

func hostPathTypePtr(t corev1.HostPathType) *corev1.HostPathType {
	return &t
}
