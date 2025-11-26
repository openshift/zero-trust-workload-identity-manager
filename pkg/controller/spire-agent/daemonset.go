package spire_agent

import (
	"context"
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	"github.com/openshift/zero-trust-workload-identity-manager/api/v1alpha1"
	"github.com/openshift/zero-trust-workload-identity-manager/pkg/controller/status"
	"github.com/openshift/zero-trust-workload-identity-manager/pkg/controller/utils"
)

// reconcileDaemonSet reconciles the Spire Agent DaemonSet
func (r *SpireAgentReconciler) reconcileDaemonSet(ctx context.Context, agent *v1alpha1.SpireAgent, statusMgr *status.Manager, createOnlyMode bool, configHash string) error {
	spireAgentDaemonset := generateSpireAgentDaemonSet(agent.Spec, configHash)
	if err := controllerutil.SetControllerReference(agent, spireAgentDaemonset, r.scheme); err != nil {
		r.log.Error(err, "failed to set controller reference")
		statusMgr.AddCondition(DaemonSetAvailable, "SpireAgentDaemonSetGenerationFailed",
			err.Error(),
			metav1.ConditionFalse)
		return err
	}

	var existingSpireAgentDaemonSet appsv1.DaemonSet
	err := r.ctrlClient.Get(ctx, types.NamespacedName{Name: spireAgentDaemonset.Name, Namespace: spireAgentDaemonset.Namespace}, &existingSpireAgentDaemonSet)
	if err != nil && kerrors.IsNotFound(err) {
		if err = r.ctrlClient.Create(ctx, spireAgentDaemonset); err != nil {
			r.log.Error(err, "failed to create spire-agent daemonset")
			statusMgr.AddCondition(DaemonSetAvailable, "SpireAgentDaemonSetCreationFailed",
				err.Error(),
				metav1.ConditionFalse)
			return fmt.Errorf("failed to create DaemonSet: %w", err)
		}
		r.log.Info("Created spire agent DaemonSet")
	} else if err == nil && needsUpdate(existingSpireAgentDaemonSet, *spireAgentDaemonset) {
		if createOnlyMode {
			r.log.Info("Skipping DaemonSet update due to create-only mode")
		} else {
			spireAgentDaemonset.ResourceVersion = existingSpireAgentDaemonSet.ResourceVersion
			if err = r.ctrlClient.Update(ctx, spireAgentDaemonset); err != nil {
				r.log.Error(err, "failed to update spire agent DaemonSet")
				statusMgr.AddCondition(DaemonSetAvailable, "SpireAgentDaemonSetUpdateFailed",
					err.Error(),
					metav1.ConditionFalse)
				return fmt.Errorf("failed to update DaemonSet: %w", err)
			}
			r.log.Info("Updated spire agent DaemonSet")
		}
	} else if err != nil {
		r.log.Error(err, "failed to get spire-agent daemonset")
		statusMgr.AddCondition(DaemonSetAvailable, "SpireAgentDaemonSetGetFailed",
			err.Error(),
			metav1.ConditionFalse)
		return err
	}

	// Check DaemonSet health/readiness
	statusMgr.CheckDaemonSetHealth(ctx, spireAgentDaemonset.Name, spireAgentDaemonset.Namespace, DaemonSetAvailable)

	return nil
}

func generateSpireAgentDaemonSet(config v1alpha1.SpireAgentSpec, spireAgentConfigHash string) *appsv1.DaemonSet {

	// Generate standardized labels once and reuse them
	labels := utils.SpireAgentLabels(config.Labels)

	// For selectors, we need only the core identifying labels (without custom user labels)
	selectorLabels := map[string]string{
		"app.kubernetes.io/name":      labels["app.kubernetes.io/name"],
		"app.kubernetes.io/instance":  labels["app.kubernetes.io/instance"],
		"app.kubernetes.io/component": labels["app.kubernetes.io/component"],
	}

	ds := &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "spire-agent",
			Namespace: utils.GetOperatorNamespace(),
			Labels:    labels,
		},
		Spec: appsv1.DaemonSetSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: selectorLabels,
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
					Labels: labels,
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
							SecurityContext: &corev1.SecurityContext{
								Privileged: ptr.To(true),
							},
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
												ExpirationSeconds: ptr.To(int64(7200)),
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

	// Add proxy configuration if enabled
	utils.AddProxyConfigToPod(&ds.Spec.Template.Spec)

	return ds
}

func hostPathTypePtr(t corev1.HostPathType) *corev1.HostPathType {
	return &t
}
