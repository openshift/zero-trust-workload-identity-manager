package spire_oidc_discovery_provider

import (
	"context"

	"github.com/openshift/zero-trust-workload-identity-manager/api/v1alpha1"
	"github.com/openshift/zero-trust-workload-identity-manager/pkg/controller/status"
	"github.com/openshift/zero-trust-workload-identity-manager/pkg/controller/utils"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// reconcileDeployment reconciles the OIDC Discovery Provider Deployment
func (r *SpireOidcDiscoveryProviderReconciler) reconcileDeployment(ctx context.Context, oidc *v1alpha1.SpireOIDCDiscoveryProvider, statusMgr *status.Manager, createOnlyMode bool, configHash string) error {
	deployment := generateDeployment(oidc, configHash)
	if err := controllerutil.SetControllerReference(oidc, deployment, r.scheme); err != nil {
		r.log.Error(err, "failed to set controller reference")
		statusMgr.AddCondition(DeploymentAvailable, "SpireOIDCDeploymentCreationFailed",
			err.Error(),
			metav1.ConditionFalse)
		return err
	}

	var existingSpireOidcDeployment appsv1.Deployment
	err := r.ctrlClient.Get(ctx, types.NamespacedName{
		Name:      deployment.Name,
		Namespace: deployment.Namespace,
	}, &existingSpireOidcDeployment)
	if err != nil && kerrors.IsNotFound(err) {
		if err = r.ctrlClient.Create(ctx, deployment); err != nil {
			r.log.Error(err, "Failed to create spire oidc discovery provider deployment")
			statusMgr.AddCondition(DeploymentAvailable, "SpireOIDCDeploymentCreationFailed",
				err.Error(),
				metav1.ConditionFalse)
			return err
		}
		r.log.Info("Created spire oidc discovery provider deployment")
	} else if err == nil && needsUpdate(existingSpireOidcDeployment, *deployment) {
		if createOnlyMode {
			r.log.Info("Skipping Deployment update due to create-only mode")
		} else {
			deployment.ResourceVersion = existingSpireOidcDeployment.ResourceVersion
			if err = r.ctrlClient.Update(ctx, deployment); err != nil {
				r.log.Error(err, "Failed to update spire oidc discovery provider deployment")
				statusMgr.AddCondition(DeploymentAvailable, "SpireOIDCDeploymentUpdateFailed",
					err.Error(),
					metav1.ConditionFalse)
				return err
			}
			r.log.Info("Updated spire oidc discovery provider deployment")
		}
	} else if err != nil {
		r.log.Error(err, "Failed to get existing spire oidc discovery provider deployment")
		statusMgr.AddCondition(DeploymentAvailable, "SpireOIDCDeploymentGetFailed",
			err.Error(),
			metav1.ConditionFalse)
		return err
	}

	// Check Deployment health/readiness
	statusMgr.CheckDeploymentHealth(ctx, deployment.Name, deployment.Namespace, DeploymentAvailable)

	return nil
}

// generateDeployment generates and return the deployment manifest based on configuration provided via SpireOIDCDiscoveryProvider spec.
func generateDeployment(config *v1alpha1.SpireOIDCDiscoveryProvider, spireOidcConfigMapHash string) *appsv1.Deployment {

	// Generate standardized labels once and reuse them
	labels := utils.SpireOIDCDiscoveryProviderLabels(config.Spec.Labels)

	// For selectors, we need only the core identifying labels (without custom user labels)
	selectorLabels := map[string]string{
		"app.kubernetes.io/name":      labels["app.kubernetes.io/name"],
		"app.kubernetes.io/instance":  labels["app.kubernetes.io/instance"],
		"app.kubernetes.io/component": labels["app.kubernetes.io/component"],
	}

	replicas := int32(1)
	if config.Spec.ReplicaCount > 0 {
		replicas = int32(config.Spec.ReplicaCount)
	}

	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "spire-spiffe-oidc-discovery-provider",
			Namespace: utils.GetOperatorNamespace(),
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
					ServiceAccountName: "spire-spiffe-oidc-discovery-provider",
					Volumes: []corev1.Volume{
						{
							Name: "spiffe-workload-api",
							VolumeSource: corev1.VolumeSource{
								CSI: &corev1.CSIVolumeSource{
									Driver:   "csi.spiffe.io",
									ReadOnly: ptr.To(true),
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
							Name: "tls-certs",
							VolumeSource: corev1.VolumeSource{
								Secret: &corev1.SecretVolumeSource{
									SecretName: "oidc-serving-cert",
								},
							},
						},
					},
					Containers: []corev1.Container{
						{
							SecurityContext: &corev1.SecurityContext{
								ReadOnlyRootFilesystem: ptr.To(true),
							},
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
								{Name: "tls-certs", MountPath: "/etc/oidc/tls", ReadOnly: true},
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
