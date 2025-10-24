package spire_server

import (
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	"k8s.io/utils/pointer"

	"github.com/openshift/zero-trust-workload-identity-manager/api/v1alpha1"
	"github.com/openshift/zero-trust-workload-identity-manager/pkg/controller/utils"
)

const spireServerStatefulSetSpireServerConfigHashAnnotationKey = "ztwim.openshift.io/spire-server-config-hash"
const spireServerStatefulSetSpireControllerMangerConfigHashAnnotationKey = "ztwim.openshift.io/spire-controller-manager-config-hash"

func GenerateSpireServerStatefulSet(config *v1alpha1.SpireServerSpec,
	spireServerConfigMapHash string,
	spireControllerMangerConfigMapHash string) *appsv1.StatefulSet {

	// Generate standardized labels once and reuse them
	labels := utils.SpireServerLabels(config.Labels)

	// For selectors, we need only the core identifying labels (without custom user labels)
	selectorLabels := map[string]string{
		"app.kubernetes.io/name":      labels["app.kubernetes.io/name"],
		"app.kubernetes.io/instance":  labels["app.kubernetes.io/instance"],
		"app.kubernetes.io/component": labels["app.kubernetes.io/component"],
	}

	volumeResourceRequest := SpireServerDefaultVolumeSize
	if config.Persistence != nil && config.Persistence.Size != "" {
		volumeResourceRequest = config.Persistence.Size
	}
	return &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      SpireServerStatefulSetName,
			Namespace: utils.OperatorNamespace,
			Labels:    labels,
		},
		Spec: appsv1.StatefulSetSpec{
			Replicas:    pointer.Int32(SpireServerDefaultReplicas),
			ServiceName: SpireServerServiceName,
			Selector: &metav1.LabelSelector{
				MatchLabels: selectorLabels,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						SpireServerAnnotationDefaultContainer:                              SpireServerContainerNameServer,
						spireServerStatefulSetSpireServerConfigHashAnnotationKey:           spireServerConfigMapHash,
						spireServerStatefulSetSpireControllerMangerConfigHashAnnotationKey: spireControllerMangerConfigMapHash,
					},
					Labels: labels,
				},
				Spec: corev1.PodSpec{
					ServiceAccountName:    SpireServerServiceAccountName,
					ShareProcessNamespace: pointer.Bool(true),
					Containers: []corev1.Container{
						{
							Name:            SpireServerContainerNameServer,
							Image:           utils.GetSpireServerImage(),
							ImagePullPolicy: corev1.PullIfNotPresent,
							Args:            []string{SpireServerArgExpandEnv, SpireServerArgConfig, SpireServerConfigPathServer},
							Env: []corev1.EnvVar{
								{Name: SpireServerEnvPath, Value: SpireServerEnvPathValue},
							},
							Ports: []corev1.ContainerPort{
								{Name: SpireServerPortNameGRPC, ContainerPort: SpireServerPortGRPC, Protocol: corev1.ProtocolTCP},
								{Name: SpireServerPortNameHealthz, ContainerPort: SpireServerPortHealthz, Protocol: corev1.ProtocolTCP},
							},
							LivenessProbe: &corev1.Probe{
								ProbeHandler:        corev1.ProbeHandler{HTTPGet: &corev1.HTTPGetAction{Path: SpireServerProbePathLive, Port: intstr.FromString(SpireServerPortNameHealthz)}},
								InitialDelaySeconds: SpireServerLivenessInitialDelay,
								PeriodSeconds:       SpireServerLivenessPeriod,
								TimeoutSeconds:      SpireServerLivenessTimeout,
								FailureThreshold:    SpireServerLivenessFailureThresh,
							},
							ReadinessProbe: &corev1.Probe{
								ProbeHandler:        corev1.ProbeHandler{HTTPGet: &corev1.HTTPGetAction{Path: SpireServerProbePathReady, Port: intstr.FromString(SpireServerPortNameHealthz)}},
								InitialDelaySeconds: SpireServerReadinessInitialDelay,
								PeriodSeconds:       SpireServerReadinessPeriod,
							},
							Resources: utils.DerefResourceRequirements(config.Resources),
							VolumeMounts: []corev1.VolumeMount{
								{Name: SpireServerVolumeNameServerSocket, MountPath: SpireServerMountPathServerSocket},
								{Name: SpireServerVolumeNameConfig, MountPath: SpireServerMountPathConfig, ReadOnly: true},
								{Name: SpireServerVolumeNameData, MountPath: SpireServerMountPathData},
								{Name: SpireServerVolumeNameServerTmp, MountPath: SpireServerMountPathTmp},
							},
						},
						{
							Name:            SpireServerContainerNameControllerManager,
							Image:           utils.GetSpireControllerManagerImage(),
							ImagePullPolicy: corev1.PullIfNotPresent,
							Args:            []string{"--config=" + SpireServerConfigPathControllerManager},
							Env: []corev1.EnvVar{
								{Name: SpireServerEnvEnableWebhooks, Value: SpireServerEnvEnableWebhooksValue},
							},
							Ports: []corev1.ContainerPort{
								{Name: SpireServerPortNameHTTPS, ContainerPort: SpireServerPortHTTPSCM},
								{Name: SpireServerPortNameHealthz, ContainerPort: SpireServerPortHealthzCM},
							},
							LivenessProbe: &corev1.Probe{
								ProbeHandler: corev1.ProbeHandler{HTTPGet: &corev1.HTTPGetAction{Path: SpireServerProbePathHealthz, Port: intstr.FromString(SpireServerPortNameHealthz)}},
							},
							ReadinessProbe: &corev1.Probe{
								ProbeHandler: corev1.ProbeHandler{HTTPGet: &corev1.HTTPGetAction{Path: SpireServerProbePathReadyz, Port: intstr.FromString(SpireServerPortNameHealthz)}},
							},
							VolumeMounts: []corev1.VolumeMount{
								{Name: SpireServerVolumeNameServerSocket, MountPath: SpireServerMountPathServerSocket, ReadOnly: true},
								{Name: SpireServerVolumeNameControllerConfig, MountPath: SpireServerMountPathControllerManagerConfig, SubPath: SpireServerConfigPathControllerManager, ReadOnly: true},
								{Name: SpireServerVolumeNameControllerManagerTmp, MountPath: SpireServerMountPathControllerManagerTmp, SubPath: SpireServerSubPathControllerManagerTmp},
							},
							Resources: utils.DerefResourceRequirements(config.Resources),
						},
					},
					Volumes: []corev1.Volume{
						{Name: SpireServerVolumeNameServerTmp, VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}}},
						{Name: SpireServerVolumeNameConfig, VolumeSource: corev1.VolumeSource{ConfigMap: &corev1.ConfigMapVolumeSource{LocalObjectReference: corev1.LocalObjectReference{Name: SpireServerConfigMapNameServer}}}},
						{Name: SpireServerVolumeNameServerSocket, VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}}},
						{Name: SpireServerVolumeNameControllerManagerTmp, VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}}},
						{Name: SpireServerVolumeNameControllerConfig, VolumeSource: corev1.VolumeSource{ConfigMap: &corev1.ConfigMapVolumeSource{LocalObjectReference: corev1.LocalObjectReference{Name: SpireServerConfigMapNameControllerManager}}}},
					},
					Affinity:     config.Affinity,
					NodeSelector: utils.DerefNodeSelector(config.NodeSelector),
					Tolerations:  utils.DerefTolerations(config.Tolerations),
				},
			},
			VolumeClaimTemplates: []corev1.PersistentVolumeClaim{
				{
					ObjectMeta: metav1.ObjectMeta{Name: SpireServerPVCNameData},
					Spec: corev1.PersistentVolumeClaimSpec{
						AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
						Resources: corev1.VolumeResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceStorage: resource.MustParse(volumeResourceRequest),
							},
						},
					},
				},
			},
		},
	}
}
