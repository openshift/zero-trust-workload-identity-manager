package spiffe_csi_driver

import (
	"github.com/openshift/zero-trust-workload-identity-manager/api/v1alpha1"
	"github.com/openshift/zero-trust-workload-identity-manager/pkg/controller/utils"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

func generateSpiffeCsiDriverDaemonSet(config v1alpha1.SpiffeCSIDriverSpec) *appsv1.DaemonSet {

	// Generate standardized labels once and reuse them
	labels := utils.SpiffeCSIDriverLabels(config.Labels)

	// For selectors, we need only the core identifying labels (without custom user labels)
	selectorLabels := map[string]string{
		"app.kubernetes.io/name":      labels["app.kubernetes.io/name"],
		"app.kubernetes.io/instance":  labels["app.kubernetes.io/instance"],
		"app.kubernetes.io/component": labels["app.kubernetes.io/component"],
	}

	return &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      SpiffeCSIDaemonSetName,
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
					MaxUnavailable: &intstr.IntOrString{
						Type:   intstr.Int,
						IntVal: SpiffeCSIMaxUnavailable,
					},
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: corev1.PodSpec{
					ServiceAccountName: SpiffeCSIServiceAccountName,
					Affinity:           config.Affinity,
					Tolerations:        utils.DerefTolerations(config.Tolerations),
					NodeSelector:       utils.DerefNodeSelector(config.NodeSelector),
					InitContainers: []corev1.Container{
						{
							Name:  SpiffeCSIInitContainerName,
							Image: utils.GetSpiffeCsiInitContainerImage(),
							Command: []string{
								SpiffeCSICommandChcon, SpiffeCSIArgRecursive, SpiffeCSIArgSELinuxType, SpiffeCSIArgTargetDir,
							},
							ImagePullPolicy: corev1.PullAlways,
							SecurityContext: &corev1.SecurityContext{
								Privileged: boolPtr(true),
								Capabilities: &corev1.Capabilities{
									Drop: []corev1.Capability{SpiffeCSICapabilityDropAll},
								},
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      SpiffeCSIVolumeNameAgentSocketDir,
									MountPath: SpiffeCSIMountPathAgentSocket,
								},
							},
							TerminationMessagePath:   SpiffeCSITerminationMessagePath,
							TerminationMessagePolicy: SpiffeCSITerminationMessageReadFileType,
						},
					},
					Containers: []corev1.Container{
						{
							Name:  SpiffeCSIContainerNameDriver,
							Image: utils.GetSpiffeCSIDriverImage(),
							Args: []string{
								SpiffeCSIArgWorkloadAPISocketDir, SpiffeCSIWorkloadAPISocketDirPath,
								SpiffeCSIArgPluginName, SpiffeCSIDefaultPluginName,
								SpiffeCSIArgCSISocketPath, SpiffeCSISocketPath,
							},
							ImagePullPolicy: corev1.PullIfNotPresent,
							Env: []corev1.EnvVar{
								{
									Name: SpiffeCSIEnvMyNodeName,
									ValueFrom: &corev1.EnvVarSource{
										FieldRef: &corev1.ObjectFieldSelector{
											FieldPath: SpiffeCSIEnvFieldPath,
										},
									},
								},
							},
							SecurityContext: &corev1.SecurityContext{
								ReadOnlyRootFilesystem: boolPtr(true),
								Privileged:             boolPtr(true),
								Capabilities: &corev1.Capabilities{
									Drop: []corev1.Capability{SpiffeCSICapabilityDropAll},
								},
							},
							Resources: utils.DerefResourceRequirements(config.Resources),
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      SpiffeCSIVolumeNameAgentSocketDir,
									MountPath: SpiffeCSIMountPathAgentSocket,
									ReadOnly:  true,
								},
								{
									Name:      SpiffeCSIVolumeNameCSISocketDir,
									MountPath: SpiffeCSIMountPathCSISocket,
								},
								{
									Name:             SpiffeCSIVolumeNameMountpoint,
									MountPath:        SpiffeCSIMountPathKubeletPods,
									MountPropagation: mountPropagationPtr(corev1.MountPropagationBidirectional),
								},
							},
						},
						{
							Name:  SpiffeCSIContainerNameRegistrar,
							Image: utils.GetNodeDriverRegistrarImage(),
							Args: []string{
								SpiffeCSIArgCSIAddress, SpiffeCSICSIAddressPath,
								SpiffeCSIArgKubeletRegistrationPath, SpiffeCSIKubeletRegistrationPath,
								SpiffeCSIArgHealthPort, SpiffeCSIHealthPort,
							},
							ImagePullPolicy: corev1.PullIfNotPresent,
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      SpiffeCSIVolumeNameCSISocketDir,
									MountPath: SpiffeCSIMountPathCSISocket,
								},
								{
									Name:      SpiffeCSIVolumeNameKubeletPluginRegistration,
									MountPath: SpiffeCSIMountPathKubeletPluginRegistration,
								},
							},
							Ports: []corev1.ContainerPort{
								{
									ContainerPort: SpiffeCSIHealthPortInt,
									Name:          SpiffeCSIPortNameHealthz,
								},
							},
							Resources: utils.DerefResourceRequirements(config.Resources),
							LivenessProbe: &corev1.Probe{
								InitialDelaySeconds: SpiffeCSIRegistrarLivenessInitialDelay,
								TimeoutSeconds:      SpiffeCSIRegistrarLivenessTimeout,
								ProbeHandler: corev1.ProbeHandler{
									HTTPGet: &corev1.HTTPGetAction{
										Path: SpiffeCSIProbePathHealthz,
										Port: intstr.FromString(SpiffeCSIPortNameHealthz),
									},
								},
							},
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: SpiffeCSIVolumeNameAgentSocketDir,
							VolumeSource: corev1.VolumeSource{
								HostPath: &corev1.HostPathVolumeSource{
									Path: SpiffeCSIHostPathAgentSockets,
									Type: hostPathTypePtr(SpiffeCSIHostPathTypeDirectoryOrCreate),
								},
							},
						},
						{
							Name: SpiffeCSIVolumeNameCSISocketDir,
							VolumeSource: corev1.VolumeSource{
								HostPath: &corev1.HostPathVolumeSource{
									Path: SpiffeCSIHostPathCSIPlugin,
									Type: hostPathTypePtr(SpiffeCSIHostPathTypeDirectoryOrCreate),
								},
							},
						},
						{
							Name: SpiffeCSIVolumeNameMountpoint,
							VolumeSource: corev1.VolumeSource{
								HostPath: &corev1.HostPathVolumeSource{
									Path: SpiffeCSIHostPathKubeletPods,
									Type: hostPathTypePtr(SpiffeCSIHostPathTypeDirectory),
								},
							},
						},
						{
							Name: SpiffeCSIVolumeNameKubeletPluginRegistration,
							VolumeSource: corev1.VolumeSource{
								HostPath: &corev1.HostPathVolumeSource{
									Path: SpiffeCSIHostPathPluginsRegistry,
									Type: hostPathTypePtr(SpiffeCSIHostPathTypeDirectory),
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
