package spire_server

import (
	"context"
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"

	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"

	"k8s.io/utils/ptr"

	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/openshift/zero-trust-workload-identity-manager/api/v1alpha1"
	"github.com/openshift/zero-trust-workload-identity-manager/pkg/controller/status"
	"github.com/openshift/zero-trust-workload-identity-manager/pkg/controller/utils"
)

const spireServerStatefulSetSpireServerConfigHashAnnotationKey = "ztwim.openshift.io/spire-server-config-hash"
const spireServerStatefulSetSpireControllerMangerConfigHashAnnotationKey = "ztwim.openshift.io/spire-controller-manager-config-hash"

// reconcileStatefulSet reconciles the Spire Server StatefulSet
func (r *SpireServerReconciler) reconcileStatefulSet(ctx context.Context, server *v1alpha1.SpireServer, statusMgr *status.Manager, createOnlyMode bool, spireServerConfigMapHash, spireControllerManagerConfigMapHash string) error {
	sts := GenerateSpireServerStatefulSet(&server.Spec, spireServerConfigMapHash, spireControllerManagerConfigMapHash)
	if err := controllerutil.SetControllerReference(server, sts, r.scheme); err != nil {
		r.log.Error(err, "failed to set controller reference on spire server stateful set resource")
		statusMgr.AddCondition(StatefulSetAvailable, "SpireServerStatefulSetGenerationFailed",
			err.Error(),
			metav1.ConditionFalse)
		return err
	}

	var existingSTS appsv1.StatefulSet
	err := r.ctrlClient.Get(ctx, types.NamespacedName{Name: sts.Name, Namespace: sts.Namespace}, &existingSTS)
	if err != nil && kerrors.IsNotFound(err) {
		if err = r.ctrlClient.Create(ctx, sts); err != nil {
			statusMgr.AddCondition(StatefulSetAvailable, "SpireServerStatefulSetCreationFailed",
				err.Error(),
				metav1.ConditionFalse)
			return fmt.Errorf("failed to create StatefulSet: %w", err)
		}
		r.log.Info("Created spire server StatefulSet")
	} else if err == nil && needsUpdate(existingSTS, *sts) {
		if createOnlyMode {
			r.log.Info("Skipping StatefulSet update due to create-only mode")
		} else {
			sts.ResourceVersion = existingSTS.ResourceVersion
			if err = r.ctrlClient.Update(ctx, sts); err != nil {
				statusMgr.AddCondition(StatefulSetAvailable, "SpireServerStatefulSetUpdateFailed",
					err.Error(),
					metav1.ConditionFalse)
				return fmt.Errorf("failed to update StatefulSet: %w", err)
			}
			r.log.Info("Updated spire server StatefulSet")
		}
	} else if err != nil {
		r.log.Error(err, "failed to get spire server stateful set resource")
		statusMgr.AddCondition(StatefulSetAvailable, "SpireServerStatefulSetGetFailed",
			err.Error(),
			metav1.ConditionFalse)
		return err
	}

	// Check StatefulSet health/readiness
	statusMgr.CheckStatefulSetHealth(ctx, sts.Name, sts.Namespace, StatefulSetAvailable)

	return nil
}

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

	volumeResourceRequest := "1Gi"
	if config.Persistence != nil && config.Persistence.Size != "" {
		volumeResourceRequest = config.Persistence.Size
	}
	return &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "spire-server",
			Namespace: utils.GetOperatorNamespace(),
			Labels:    labels,
		},
		Spec: appsv1.StatefulSetSpec{
			Replicas:    ptr.To(int32(1)),
			ServiceName: "spire-server",
			Selector: &metav1.LabelSelector{
				MatchLabels: selectorLabels,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						"kubectl.kubernetes.io/default-container":                          "spire-server",
						spireServerStatefulSetSpireServerConfigHashAnnotationKey:           spireServerConfigMapHash,
						spireServerStatefulSetSpireControllerMangerConfigHashAnnotationKey: spireControllerMangerConfigMapHash,
					},
					Labels: labels,
				},
				Spec: corev1.PodSpec{
					ServiceAccountName:    "spire-server",
					ShareProcessNamespace: ptr.To(true),
					Containers: []corev1.Container{
						{
							SecurityContext: &corev1.SecurityContext{
								ReadOnlyRootFilesystem: ptr.To(true),
							},
							Name:            "spire-server",
							Image:           utils.GetSpireServerImage(),
							ImagePullPolicy: corev1.PullIfNotPresent,
							Args:            []string{"-expandEnv", "-config", "/run/spire/config/server.conf"},
							Env: []corev1.EnvVar{
								{Name: "PATH", Value: "/opt/spire/bin:/bin"},
							},
							Ports: []corev1.ContainerPort{
								{Name: "grpc", ContainerPort: 8081, Protocol: corev1.ProtocolTCP},
								{Name: "healthz", ContainerPort: 8080, Protocol: corev1.ProtocolTCP},
							},
							LivenessProbe: &corev1.Probe{
								ProbeHandler:        corev1.ProbeHandler{HTTPGet: &corev1.HTTPGetAction{Path: "/live", Port: intstr.FromString("healthz")}},
								InitialDelaySeconds: 15,
								PeriodSeconds:       60,
								TimeoutSeconds:      3,
								FailureThreshold:    2,
							},
							ReadinessProbe: &corev1.Probe{
								ProbeHandler:        corev1.ProbeHandler{HTTPGet: &corev1.HTTPGetAction{Path: "/ready", Port: intstr.FromString("healthz")}},
								InitialDelaySeconds: 5,
								PeriodSeconds:       5,
							},
							Resources: utils.DerefResourceRequirements(config.Resources),
							VolumeMounts: []corev1.VolumeMount{
								{Name: "spire-server-socket", MountPath: "/tmp/spire-server/private"},
								{Name: "spire-config", MountPath: "/run/spire/config", ReadOnly: true},
								{Name: "spire-data", MountPath: "/run/spire/data"},
								{Name: "server-tmp", MountPath: "/tmp"},
							},
						},
						{
							SecurityContext: &corev1.SecurityContext{
								ReadOnlyRootFilesystem: ptr.To(true),
							},
							Name:            "spire-controller-manager",
							Image:           utils.GetSpireControllerManagerImage(),
							ImagePullPolicy: corev1.PullIfNotPresent,
							Args:            []string{"--config=controller-manager-config.yaml"},
							Env: []corev1.EnvVar{
								{Name: "ENABLE_WEBHOOKS", Value: "true"},
							},
							Ports: []corev1.ContainerPort{
								{Name: "https", ContainerPort: 9443},
								{Name: "healthz", ContainerPort: 8083},
							},
							LivenessProbe: &corev1.Probe{
								ProbeHandler: corev1.ProbeHandler{HTTPGet: &corev1.HTTPGetAction{Path: "/healthz", Port: intstr.FromString("healthz")}},
							},
							ReadinessProbe: &corev1.Probe{
								ProbeHandler: corev1.ProbeHandler{HTTPGet: &corev1.HTTPGetAction{Path: "/readyz", Port: intstr.FromString("healthz")}},
							},
							VolumeMounts: []corev1.VolumeMount{
								{Name: "spire-server-socket", MountPath: "/tmp/spire-server/private", ReadOnly: true},
								{Name: "controller-manager-config", MountPath: "/controller-manager-config.yaml", SubPath: "controller-manager-config.yaml", ReadOnly: true},
								{Name: "spire-controller-manager-tmp", MountPath: "/tmp", SubPath: "spire-controller-manager"},
							},
							Resources: utils.DerefResourceRequirements(config.Resources),
						},
					},
					Volumes: []corev1.Volume{
						{Name: "server-tmp", VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}}},
						{Name: "spire-config", VolumeSource: corev1.VolumeSource{ConfigMap: &corev1.ConfigMapVolumeSource{LocalObjectReference: corev1.LocalObjectReference{Name: "spire-server"}}}},
						{Name: "spire-server-socket", VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}}},
						{Name: "spire-controller-manager-tmp", VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}}},
						{Name: "controller-manager-config", VolumeSource: corev1.VolumeSource{ConfigMap: &corev1.ConfigMapVolumeSource{LocalObjectReference: corev1.LocalObjectReference{Name: "spire-controller-manager"}}}},
					},
					Affinity:     config.Affinity,
					NodeSelector: utils.DerefNodeSelector(config.NodeSelector),
					Tolerations:  utils.DerefTolerations(config.Tolerations),
				},
			},
			VolumeClaimTemplates: []corev1.PersistentVolumeClaim{
				{
					ObjectMeta: metav1.ObjectMeta{Name: "spire-data"},
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
