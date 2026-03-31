package spiffe_helper

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"reflect"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"github.com/openshift/zero-trust-workload-identity-manager/pkg/controller/utils"
)

const (
	// Annotation keys
	AnnotationInjectHelper = "spiffe.openshift.io/inject-helper"
	AnnotationCertDir      = "spiffe.openshift.io/cert-dir"
	AnnotationHelperConfig = "spiffe.openshift.io/helper-config"

	// Default values
	DefaultCertDir      = "/var/run/secrets/tls"
	DefaultHelperConfig = "spiffe-helper-config"

	// Container and volume names
	InitContainerName      = "spiffe-helper-init"
	SidecarContainerName   = "spiffe-helper"
	VolumeNameWorkloadAPI  = "spiffe-workload-api"
	VolumeNameCerts        = "spiffe-certs"
	VolumeNameHelperConfig = "spiffe-helper-config"
	CSIDriverName          = "csi.spiffe.io"
	WorkloadAPIMountPath   = "/spiffe-workload-api"
	HelperConfigMountPath  = "/etc/spiffe-helper"
)

// SpiffeHelperInjector handles admission requests to inject spiffe-helper sidecars
type SpiffeHelperInjector struct {
	log logr.Logger
}

// NewSpiffeHelperInjector creates a new SpiffeHelperInjector
func NewSpiffeHelperInjector() *SpiffeHelperInjector {
	return &SpiffeHelperInjector{
		log: ctrl.Log.WithName("spiffe-helper-injector"),
	}
}

// Handle processes admission requests and injects spiffe-helper containers when annotated
func (s *SpiffeHelperInjector) Handle(ctx context.Context, req admission.Request) admission.Response {
	pod := &corev1.Pod{}
	if err := json.Unmarshal(req.Object.Raw, pod); err != nil {
		s.log.Error(err, "failed to decode pod")
		return admission.Errored(http.StatusBadRequest, err)
	}

	// Check for injection annotation
	annotations := pod.GetAnnotations()
	if annotations == nil || annotations[AnnotationInjectHelper] != "true" {
		return admission.Allowed("no injection requested")
	}

	// Skip if already injected
	if hasSpiffeHelperContainers(pod) {
		s.log.V(1).Info("spiffe-helper containers already present, skipping injection",
			"pod", pod.Name, "namespace", pod.Namespace)
		return admission.Allowed("already injected")
	}

	// Get image from environment variable
	image := os.Getenv(utils.SpiffeHelperImageEnv)
	if image == "" {
		s.log.Error(nil, "RELATED_IMAGE_SPIFFE_HELPER not set")
		return admission.Errored(http.StatusInternalServerError,
			errMissingImage)
	}

	// Read optional annotations
	certDir := getAnnotationOrDefault(annotations, AnnotationCertDir, DefaultCertDir)
	helperConfigMap := getAnnotationOrDefault(annotations, AnnotationHelperConfig, DefaultHelperConfig)

	// Inject containers and volumes
	if err := injectSpiffeHelper(pod, image, certDir, helperConfigMap); err != nil {
		s.log.Error(err, "failed to inject spiffe-helper", "pod", pod.Name, "namespace", pod.Namespace)
		return admission.Errored(http.StatusUnprocessableEntity, err)
	}

	marshaledPod, err := json.Marshal(pod)
	if err != nil {
		s.log.Error(err, "failed to marshal mutated pod")
		return admission.Errored(http.StatusInternalServerError, err)
	}

	s.log.Info("injected spiffe-helper sidecar", "pod", pod.Name, "namespace", pod.Namespace)
	return admission.PatchResponseFromRaw(req.Object.Raw, marshaledPod)
}

// hasSpiffeHelperContainers checks if the pod already has both spiffe-helper containers
func hasSpiffeHelperContainers(pod *corev1.Pod) bool {
	hasInit := false
	for _, c := range pod.Spec.InitContainers {
		if c.Name == InitContainerName {
			hasInit = true
			break
		}
	}
	hasSidecar := false
	for _, c := range pod.Spec.Containers {
		if c.Name == SidecarContainerName {
			hasSidecar = true
			break
		}
	}
	return hasInit && hasSidecar
}

// getAnnotationOrDefault returns the annotation value or a default
func getAnnotationOrDefault(annotations map[string]string, key, defaultValue string) string {
	if val, ok := annotations[key]; ok && val != "" {
		return val
	}
	return defaultValue
}

// injectSpiffeHelper adds init container, sidecar, and volumes to the pod
func injectSpiffeHelper(pod *corev1.Pod, image, certDir, helperConfigMap string) error {
	// Add volumes (only if not already present, reject if incompatible)
	if err := ensureCompatibleVolume(pod, corev1.Volume{
		Name: VolumeNameWorkloadAPI,
		VolumeSource: corev1.VolumeSource{
			CSI: &corev1.CSIVolumeSource{
				Driver:   CSIDriverName,
				ReadOnly: boolPtr(true),
			},
		},
	}); err != nil {
		return err
	}
	if err := ensureCompatibleVolume(pod, corev1.Volume{
		Name: VolumeNameCerts,
		VolumeSource: corev1.VolumeSource{
			EmptyDir: &corev1.EmptyDirVolumeSource{},
		},
	}); err != nil {
		return err
	}
	if err := ensureCompatibleVolume(pod, corev1.Volume{
		Name: VolumeNameHelperConfig,
		VolumeSource: corev1.VolumeSource{
			ConfigMap: &corev1.ConfigMapVolumeSource{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: helperConfigMap,
				},
			},
		},
	}); err != nil {
		return err
	}

	// Common volume mounts for spiffe-helper containers
	helperVolumeMounts := []corev1.VolumeMount{
		{
			Name:      VolumeNameWorkloadAPI,
			MountPath: WorkloadAPIMountPath,
			ReadOnly:  true,
		},
		{
			Name:      VolumeNameCerts,
			MountPath: certDir,
		},
		{
			Name:      VolumeNameHelperConfig,
			MountPath: HelperConfigMountPath,
			ReadOnly:  true,
		},
	}

	// Security context for compatibility with restricted namespaces
	securityContext := &corev1.SecurityContext{
		AllowPrivilegeEscalation: boolPtr(false),
		RunAsNonRoot:             boolPtr(true),
		Capabilities: &corev1.Capabilities{
			Drop: []corev1.Capability{"ALL"},
		},
		SeccompProfile: &corev1.SeccompProfile{
			Type: corev1.SeccompProfileTypeRuntimeDefault,
		},
	}

	// Add init container
	pod.Spec.InitContainers = append(pod.Spec.InitContainers, corev1.Container{
		Name:            InitContainerName,
		Image:           image,
		Args:            []string{"-config", "/etc/spiffe-helper/helper-init.conf"},
		VolumeMounts:    helperVolumeMounts,
		SecurityContext: securityContext,
	})

	// Add sidecar container
	pod.Spec.Containers = append(pod.Spec.Containers, corev1.Container{
		Name:            SidecarContainerName,
		Image:           image,
		Args:            []string{"-config", "/etc/spiffe-helper/helper.conf"},
		VolumeMounts:    helperVolumeMounts,
		SecurityContext: securityContext,
	})

	// Mount certs volume into existing containers (only if not already mounted)
	for i := range pod.Spec.Containers {
		if pod.Spec.Containers[i].Name == SidecarContainerName {
			continue
		}
		if !hasVolumeMount(&pod.Spec.Containers[i], VolumeNameCerts) {
			pod.Spec.Containers[i].VolumeMounts = append(pod.Spec.Containers[i].VolumeMounts,
				corev1.VolumeMount{
					Name:      VolumeNameCerts,
					MountPath: certDir,
					ReadOnly:  true,
				},
			)
		}
	}
	return nil
}

// ensureCompatibleVolume adds a volume to the pod if absent, or returns an error if
// a volume with the same name exists but has an incompatible source
func ensureCompatibleVolume(pod *corev1.Pod, expected corev1.Volume) error {
	for _, v := range pod.Spec.Volumes {
		if v.Name == expected.Name {
			if !reflect.DeepEqual(v.VolumeSource, expected.VolumeSource) {
				return fmt.Errorf("volume %q exists with incompatible source", expected.Name)
			}
			return nil
		}
	}
	pod.Spec.Volumes = append(pod.Spec.Volumes, expected)
	return nil
}

// hasVolumeMount checks if a container already has a volume mount with the given name
func hasVolumeMount(container *corev1.Container, name string) bool {
	for _, vm := range container.VolumeMounts {
		if vm.Name == name {
			return true
		}
	}
	return false
}

func boolPtr(b bool) *bool {
	return &b
}

var errMissingImage = errorf("RELATED_IMAGE_SPIFFE_HELPER environment variable not set")

type constError string

func errorf(s string) constError { return constError(s) }

func (e constError) Error() string { return string(e) }
