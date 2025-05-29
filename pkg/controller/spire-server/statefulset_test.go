package spire_server

import (
	"reflect"
	"testing"

	"github.com/openshift/zero-trust-workload-identity-manager/api/v1alpha1"
	"github.com/openshift/zero-trust-workload-identity-manager/pkg/controller/utils"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/pointer"
)

func TestGenerateSpireServerStatefulSet(t *testing.T) {
	// Setup test inputs
	config := &v1alpha1.SpireServerConfigSpec{
		CommonConfig: v1alpha1.CommonConfig{
			Labels: map[string]string{
				"custom-label": "test-value",
			},
		},
	}
	serverConfigHash := "test-server-hash"
	controllerConfigHash := "test-controller-hash"

	// Call the function
	statefulSet := GenerateSpireServerStatefulSet(config, serverConfigHash, controllerConfigHash)

	// Test basic metadata
	t.Run("Validates StatefulSet metadata", func(t *testing.T) {
		if statefulSet.Name != "spire-server" {
			t.Errorf("Expected name 'spire-server', got %q", statefulSet.Name)
		}

		if statefulSet.Namespace != utils.OperatorNamespace {
			t.Errorf("Expected namespace %q, got %q", utils.OperatorNamespace, statefulSet.Namespace)
		}

		// Check standard labels
		expectedLabels := map[string]string{
			"app.kubernetes.io/name":       "server",
			"app.kubernetes.io/instance":   "spire",
			"app.kubernetes.io/managed-by": "zero-trust-workload-identity-manager",
			"app.kubernetes.io/component":  "server",
			"custom-label":                 "test-value",
		}

		for k, v := range expectedLabels {
			if statefulSet.Labels[k] != v {
				t.Errorf("Expected label %q to be %q, got %q", k, v, statefulSet.Labels[k])
			}
		}
	})

	// Test StatefulSet spec
	t.Run("Validates StatefulSet spec", func(t *testing.T) {
		if *statefulSet.Spec.Replicas != 1 {
			t.Errorf("Expected 1 replica, got %d", *statefulSet.Spec.Replicas)
		}

		if statefulSet.Spec.ServiceName != "spire-server" {
			t.Errorf("Expected service name 'spire-server', got %q", statefulSet.Spec.ServiceName)
		}

		// Check if selector matches the pod template labels
		for k, v := range statefulSet.Spec.Selector.MatchLabels {
			if statefulSet.Spec.Template.Labels[k] != v {
				t.Errorf("Selector label %q=%q doesn't match pod template label %q", k, v, statefulSet.Spec.Template.Labels[k])
			}
		}
	})

	// Test Pod Template annotations
	t.Run("Validates Pod Template annotations", func(t *testing.T) {
		expectedAnnotations := map[string]string{
			"kubectl.kubernetes.io/default-container":                          "spire-server",
			spireServerStatefulSetSpireServerConfigHashAnnotationKey:           serverConfigHash,
			spireServerStatefulSetSpireControllerMangerConfigHashAnnotationKey: controllerConfigHash,
		}

		for k, v := range expectedAnnotations {
			if statefulSet.Spec.Template.Annotations[k] != v {
				t.Errorf("Expected annotation %q to be %q, got %q", k, v, statefulSet.Spec.Template.Annotations[k])
			}
		}
	})

	// Test Pod Spec
	t.Run("Validates Pod Spec", func(t *testing.T) {
		podSpec := statefulSet.Spec.Template.Spec

		if podSpec.ServiceAccountName != "spire-server" {
			t.Errorf("Expected service account name 'spire-server', got %q", podSpec.ServiceAccountName)
		}

		if *podSpec.ShareProcessNamespace != true {
			t.Errorf("Expected share process namespace to be true")
		}

		// Check volume count
		expectedVolumeCount := 5
		if len(podSpec.Volumes) != expectedVolumeCount {
			t.Errorf("Expected %d volumes, got %d", expectedVolumeCount, len(podSpec.Volumes))
		}

		// Check containers count
		expectedContainerCount := 2
		if len(podSpec.Containers) != expectedContainerCount {
			t.Errorf("Expected %d containers, got %d", expectedContainerCount, len(podSpec.Containers))
		}
	})

	// Test SPIRE server container
	t.Run("Validates SPIRE server container", func(t *testing.T) {
		spireServerContainer := findContainerByName(statefulSet.Spec.Template.Spec.Containers, "spire-server")
		if spireServerContainer == nil {
			t.Fatalf("spire-server container not found")
		}

		// Check image
		if spireServerContainer.Image != utils.GetSpireServerImage() {
			t.Errorf("Expected image %q, got %q", utils.GetSpireServerImage(), spireServerContainer.Image)
		}

		// Check arguments
		expectedArgs := []string{"-expandEnv", "-config", "/run/spire/config/server.conf"}
		if !reflect.DeepEqual(spireServerContainer.Args, expectedArgs) {
			t.Errorf("Expected args %v, got %v", expectedArgs, spireServerContainer.Args)
		}

		// Check ports
		if len(spireServerContainer.Ports) != 2 {
			t.Errorf("Expected 2 ports, got %d", len(spireServerContainer.Ports))
		}

		// Check environment variables
		if len(spireServerContainer.Env) != 1 {
			t.Errorf("Expected 1 environment variable, got %d", len(spireServerContainer.Env))
		}

		// Check volume mounts
		expectedVolumeMountCount := 4
		if len(spireServerContainer.VolumeMounts) != expectedVolumeMountCount {
			t.Errorf("Expected %d volume mounts, got %d", expectedVolumeMountCount, len(spireServerContainer.VolumeMounts))
		}

		// Check liveness probe
		if spireServerContainer.LivenessProbe == nil {
			t.Fatalf("LivenessProbe not configured")
		}

		// Check readiness probe
		if spireServerContainer.ReadinessProbe == nil {
			t.Fatalf("ReadinessProbe not configured")
		}
	})

	// Test controller manager container
	t.Run("Validates controller manager container", func(t *testing.T) {
		controllerContainer := findContainerByName(statefulSet.Spec.Template.Spec.Containers, "spire-controller-manager")
		if controllerContainer == nil {
			t.Fatalf("spire-controller-manager container not found")
		}

		// Check image
		if controllerContainer.Image != utils.GetSpireControllerManagerImage() {
			t.Errorf("Expected image %q, got %q", utils.GetSpireControllerManagerImage(), controllerContainer.Image)
		}

		// Check arguments
		expectedArgs := []string{"--config=controller-manager-config.yaml"}
		if !reflect.DeepEqual(controllerContainer.Args, expectedArgs) {
			t.Errorf("Expected args %v, got %v", expectedArgs, controllerContainer.Args)
		}

		// Check environment variables
		if len(controllerContainer.Env) != 1 || controllerContainer.Env[0].Name != "ENABLE_WEBHOOKS" || controllerContainer.Env[0].Value != "true" {
			t.Errorf("Expected environment variable ENABLE_WEBHOOKS=true, got %v", controllerContainer.Env)
		}

		// Check volume mounts
		expectedVolumeMountCount := 3
		if len(controllerContainer.VolumeMounts) != expectedVolumeMountCount {
			t.Errorf("Expected %d volume mounts, got %d", expectedVolumeMountCount, len(controllerContainer.VolumeMounts))
		}

		// Check liveness probe
		if controllerContainer.LivenessProbe == nil {
			t.Fatalf("LivenessProbe not configured")
		}

		// Check readiness probe
		if controllerContainer.ReadinessProbe == nil {
			t.Fatalf("ReadinessProbe not configured")
		}
	})

	// Test volume claims templates
	t.Run("Validates volume claim templates", func(t *testing.T) {
		if len(statefulSet.Spec.VolumeClaimTemplates) != 1 {
			t.Fatalf("Expected 1 volume claim template, got %d", len(statefulSet.Spec.VolumeClaimTemplates))
		}

		pvc := statefulSet.Spec.VolumeClaimTemplates[0]
		if pvc.Name != "spire-data" {
			t.Errorf("Expected volume claim name 'spire-data', got %q", pvc.Name)
		}

		if len(pvc.Spec.AccessModes) != 1 || pvc.Spec.AccessModes[0] != corev1.ReadWriteOnce {
			t.Errorf("Expected access mode ReadWriteOnce, got %v", pvc.Spec.AccessModes)
		}

		storageRequest := pvc.Spec.Resources.Requests[corev1.ResourceStorage]
		expectedStorage := resource.MustParse("1Gi")
		if !storageRequest.Equal(expectedStorage) {
			t.Errorf("Expected storage request %v, got %v", expectedStorage, storageRequest)
		}
	})

	// Test with nil labels
	t.Run("Handles nil labels gracefully", func(t *testing.T) {
		configWithNilLabels := &v1alpha1.SpireServerConfigSpec{
			CommonConfig: v1alpha1.CommonConfig{
				Labels: nil,
			},
		}

		statefulSet := GenerateSpireServerStatefulSet(configWithNilLabels, serverConfigHash, controllerConfigHash)

		// Verify we have all standard labels
		expectedLabels := map[string]string{
			"app.kubernetes.io/name":       "server",
			"app.kubernetes.io/instance":   "spire",
			"app.kubernetes.io/managed-by": "zero-trust-workload-identity-manager",
			"app.kubernetes.io/component":  "server",
		}

		for k, v := range expectedLabels {
			if statefulSet.Labels[k] != v {
				t.Errorf("Expected label %q to be %q, got %q", k, v, statefulSet.Labels[k])
			}
		}
	})

	// Test with empty labels map
	t.Run("Handles empty labels map gracefully", func(t *testing.T) {
		configWithEmptyLabels := &v1alpha1.SpireServerConfigSpec{
			CommonConfig: v1alpha1.CommonConfig{
				Labels: map[string]string{},
			},
		}

		statefulSet := GenerateSpireServerStatefulSet(configWithEmptyLabels, serverConfigHash, controllerConfigHash)

		// Verify we have all standard labels
		expectedLabels := map[string]string{
			"app.kubernetes.io/name":       "server",
			"app.kubernetes.io/instance":   "spire",
			"app.kubernetes.io/managed-by": "zero-trust-workload-identity-manager",
			"app.kubernetes.io/component":  "server",
		}

		for k, v := range expectedLabels {
			if statefulSet.Labels[k] != v {
				t.Errorf("Expected label %q to be %q, got %q", k, v, statefulSet.Labels[k])
			}
		}
	})

	// Test against a reference implementation to ensure no regressions
	t.Run("Matches reference implementation", func(t *testing.T) {
		expected := createReferenceStatefulSet(config, serverConfigHash, controllerConfigHash)

		// Help pinpoint differences if there are any
		if !reflect.DeepEqual(statefulSet.ObjectMeta, expected.ObjectMeta) {
			t.Errorf("ObjectMeta differs")
		}

		if !reflect.DeepEqual(statefulSet.Spec.Replicas, expected.Spec.Replicas) {
			t.Errorf("Replicas differs: got %v, expected %v", *statefulSet.Spec.Replicas, *expected.Spec.Replicas)
		}

		if !reflect.DeepEqual(statefulSet.Spec.ServiceName, expected.Spec.ServiceName) {
			t.Errorf("ServiceName differs: got %v, expected %v", statefulSet.Spec.ServiceName, expected.Spec.ServiceName)
		}
	})
}

// Helper function to find a container by name
func findContainerByName(containers []corev1.Container, name string) *corev1.Container {
	for i := range containers {
		if containers[i].Name == name {
			return &containers[i]
		}
	}
	return nil
}

// Helper function creating a reference implementation of the expected StatefulSet
// This is essentially a copy of the function being tested, used to detect regressions
func createReferenceStatefulSet(config *v1alpha1.SpireServerConfigSpec, spireServerConfigMapHash string,
	spireControllerMangerConfigMapHash string) *appsv1.StatefulSet {
	labels := map[string]string{
		"app.kubernetes.io/name":       "server",
		"app.kubernetes.io/instance":   "spire",
		"app.kubernetes.io/managed-by": "zero-trust-workload-identity-manager",
		"app.kubernetes.io/component":  "server",
	}
	for k, v := range config.Labels {
		labels[k] = v
	}

	return &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "spire-server",
			Namespace: utils.OperatorNamespace,
			Labels:    labels,
		},
		Spec: appsv1.StatefulSetSpec{
			Replicas:    pointer.Int32(1),
			ServiceName: "spire-server",
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
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
					ShareProcessNamespace: pointer.Bool(true),
					Containers: []corev1.Container{
						{
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
							VolumeMounts: []corev1.VolumeMount{
								{Name: "spire-server-socket", MountPath: "/tmp/spire-server/private"},
								{Name: "spire-config", MountPath: "/run/spire/config", ReadOnly: true},
								{Name: "spire-data", MountPath: "/run/spire/data"},
								{Name: "server-tmp", MountPath: "/tmp"},
							},
						},
						{
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
						},
					},
					Volumes: []corev1.Volume{
						{Name: "server-tmp", VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}}},
						{Name: "spire-config", VolumeSource: corev1.VolumeSource{ConfigMap: &corev1.ConfigMapVolumeSource{LocalObjectReference: corev1.LocalObjectReference{Name: "spire-server"}}}},
						{Name: "spire-server-socket", VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}}},
						{Name: "spire-controller-manager-tmp", VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}}},
						{Name: "controller-manager-config", VolumeSource: corev1.VolumeSource{ConfigMap: &corev1.ConfigMapVolumeSource{LocalObjectReference: corev1.LocalObjectReference{Name: "spire-controller-manager"}}}},
					},
				},
			},
			VolumeClaimTemplates: []corev1.PersistentVolumeClaim{
				{
					ObjectMeta: metav1.ObjectMeta{Name: "spire-data"},
					Spec: corev1.PersistentVolumeClaimSpec{
						AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
						Resources: corev1.VolumeResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceStorage: resource.MustParse("1Gi"),
							},
						},
					},
				},
			},
		},
	}
}
