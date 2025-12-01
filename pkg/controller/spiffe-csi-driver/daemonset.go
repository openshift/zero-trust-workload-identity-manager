package spiffe_csi_driver

import (
	"context"
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"

	"k8s.io/utils/ptr"

	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/openshift/zero-trust-workload-identity-manager/api/v1alpha1"
	"github.com/openshift/zero-trust-workload-identity-manager/pkg/controller/status"
	"github.com/openshift/zero-trust-workload-identity-manager/pkg/controller/utils"
)

// reconcileDaemonSet reconciles the Spiffe CSI Driver DaemonSet
func (r *SpiffeCsiReconciler) reconcileDaemonSet(ctx context.Context, driver *v1alpha1.SpiffeCSIDriver, statusMgr *status.Manager, createOnlyMode bool) error {
	spiffeCsiDaemonset := generateSpiffeCsiDriverDaemonSet(driver.Spec)
	if err := controllerutil.SetControllerReference(driver, spiffeCsiDaemonset, r.scheme); err != nil {
		r.log.Error(err, "failed to set owner reference for the DaemonSet resource")
		statusMgr.AddCondition(DaemonSetAvailable, "SpiffeCSIDaemonSetGenerationFailed",
			err.Error(),
			metav1.ConditionFalse)
		return err
	}

	var existingSpiffeCsiDaemonSet appsv1.DaemonSet
	err := r.ctrlClient.Get(ctx, types.NamespacedName{Name: spiffeCsiDaemonset.Name, Namespace: spiffeCsiDaemonset.Namespace}, &existingSpiffeCsiDaemonSet)
	if err != nil && kerrors.IsNotFound(err) {
		if err = r.ctrlClient.Create(ctx, spiffeCsiDaemonset); err != nil {
			r.log.Error(err, "Failed to create SpiffeCsiDaemon set")
			statusMgr.AddCondition(DaemonSetAvailable, "SpiffeCSIDaemonSetCreationFailed",
				err.Error(),
				metav1.ConditionFalse)
			return fmt.Errorf("failed to create DaemonSet: %w", err)
		}
		r.log.Info("Created spiffe csi DaemonSet")
	} else if err == nil && needsUpdate(existingSpiffeCsiDaemonSet, *spiffeCsiDaemonset) {
		if createOnlyMode {
			r.log.Info("Skipping DaemonSet update due to create-only mode")
		} else {
			spiffeCsiDaemonset.ResourceVersion = existingSpiffeCsiDaemonSet.ResourceVersion
			if err = r.ctrlClient.Update(ctx, spiffeCsiDaemonset); err != nil {
				r.log.Error(err, "failed to update spiffe csi daemon set")
				statusMgr.AddCondition(DaemonSetAvailable, "SpiffeCSIDaemonSetUpdateFailed",
					err.Error(),
					metav1.ConditionFalse)
				return fmt.Errorf("failed to update DaemonSet: %w", err)
			}
			r.log.Info("Updated spiffe csi DaemonSet")
		}
	} else if err != nil {
		r.log.Error(err, "Failed to get SpiffeCsiDaemon set")
		statusMgr.AddCondition(DaemonSetAvailable, "SpiffeCSIDaemonSetGetFailed",
			err.Error(),
			metav1.ConditionFalse)
		return err
	}

	// Check DaemonSet health/readiness
	statusMgr.CheckDaemonSetHealth(ctx, spiffeCsiDaemonset.Name, spiffeCsiDaemonset.Namespace, DaemonSetAvailable)

	return nil
}

// needsUpdate returns true if DaemonSet needs to be updated.
func needsUpdate(current, desired appsv1.DaemonSet) bool {
	if utils.DaemonSetNeedsUpdate(&current, &desired) {
		return true
	} else if !equality.Semantic.DeepEqual(current.Labels, desired.Labels) {
		return true
	}
	return false
}

func generateSpiffeCsiDriverDaemonSet(config v1alpha1.SpiffeCSIDriverSpec) *appsv1.DaemonSet {

	// Generate standardized labels once and reuse them
	labels := utils.SpiffeCSIDriverLabels(config.Labels)

	// For selectors, we need only the core identifying labels (without custom user labels)
	selectorLabels := map[string]string{
		"app.kubernetes.io/name":      labels["app.kubernetes.io/name"],
		"app.kubernetes.io/instance":  labels["app.kubernetes.io/instance"],
		"app.kubernetes.io/component": labels["app.kubernetes.io/component"],
	}

	ds := &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "spire-spiffe-csi-driver",
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
					MaxUnavailable: &intstr.IntOrString{
						Type:   intstr.Int,
						IntVal: 1,
					},
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: corev1.PodSpec{
					ServiceAccountName: "spire-spiffe-csi-driver",
					Affinity:           config.Affinity,
					Tolerations:        utils.DerefTolerations(config.Tolerations),
					NodeSelector:       utils.DerefNodeSelector(config.NodeSelector),
					InitContainers: []corev1.Container{
						{
							Name:  "set-context",
							Image: utils.GetSpiffeCsiInitContainerImage(),
							Command: []string{
								"chcon", "-Rvt", "container_file_t", "spire-agent-socket/",
							},
							ImagePullPolicy: corev1.PullAlways,
							SecurityContext: &corev1.SecurityContext{
								Privileged: ptr.To(true),
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
								ReadOnlyRootFilesystem: ptr.To(true),
								Privileged:             ptr.To(true),
								Capabilities: &corev1.Capabilities{
									Drop: []corev1.Capability{"all"},
								},
							},
							Resources: utils.DerefResourceRequirements(config.Resources),
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
							Resources: utils.DerefResourceRequirements(config.Resources),
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
							SecurityContext: &corev1.SecurityContext{
								Privileged: ptr.To(true),
								Capabilities: &corev1.Capabilities{
									Drop: []corev1.Capability{"all"},
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

	return ds
}

func hostPathTypePtr(t corev1.HostPathType) *corev1.HostPathType {
	return &t
}

func mountPropagationPtr(mp corev1.MountPropagationMode) *corev1.MountPropagationMode {
	return &mp
}
