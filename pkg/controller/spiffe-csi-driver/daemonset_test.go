package spiffe_csi_driver

import (
	"reflect"
	"testing"

	"github.com/openshift/zero-trust-workload-identity-manager/pkg/controller/utils"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

func TestGenerateSpiffeCsiDriverDaemonSet(t *testing.T) {
	// Mock the utility functions that are called in the main function
	// These would need to be properly mocked in a real test environment

	daemonSet := generateSpiffeCsiDriverDaemonSet()

	// Test ObjectMeta
	if daemonSet.Name != "spire-spiffe-csi-driver" {
		t.Errorf("Expected name 'spire-spiffe-csi-driver', got '%s'", daemonSet.Name)
	}

	if daemonSet.Namespace != utils.OperatorNamespace {
		t.Errorf("Expected namespace '%s', got '%s'", utils.OperatorNamespace, daemonSet.Namespace)
	}

	expectedLabels := map[string]string{
		"app.kubernetes.io/name":     "spiffe-csi-driver",
		"app.kubernetes.io/instance": "spire",
		utils.AppManagedByLabelKey:   utils.AppManagedByLabelValue,
	}

	if !reflect.DeepEqual(daemonSet.Labels, expectedLabels) {
		t.Errorf("Expected labels %v, got %v", expectedLabels, daemonSet.Labels)
	}

	// Test Selector
	expectedSelectorLabels := map[string]string{
		"app.kubernetes.io/name":     "spiffe-csi-driver",
		"app.kubernetes.io/instance": "spire",
	}

	if !reflect.DeepEqual(daemonSet.Spec.Selector.MatchLabels, expectedSelectorLabels) {
		t.Errorf("Expected selector labels %v, got %v", expectedSelectorLabels, daemonSet.Spec.Selector.MatchLabels)
	}

	// Test UpdateStrategy
	if daemonSet.Spec.UpdateStrategy.Type != appsv1.RollingUpdateDaemonSetStrategyType {
		t.Errorf("Expected update strategy type '%s', got '%s'",
			appsv1.RollingUpdateDaemonSetStrategyType, daemonSet.Spec.UpdateStrategy.Type)
	}

	expectedMaxUnavailable := &intstr.IntOrString{
		Type:   intstr.Int,
		IntVal: 1,
	}

	if !reflect.DeepEqual(daemonSet.Spec.UpdateStrategy.RollingUpdate.MaxUnavailable, expectedMaxUnavailable) {
		t.Errorf("Expected MaxUnavailable %v, got %v",
			expectedMaxUnavailable, daemonSet.Spec.UpdateStrategy.RollingUpdate.MaxUnavailable)
	}

	// Test PodTemplateSpec Labels
	if !reflect.DeepEqual(daemonSet.Spec.Template.Labels, expectedSelectorLabels) {
		t.Errorf("Expected template labels %v, got %v", expectedSelectorLabels, daemonSet.Spec.Template.Labels)
	}

	// Test ServiceAccountName
	if daemonSet.Spec.Template.Spec.ServiceAccountName != "spire-spiffe-csi-driver" {
		t.Errorf("Expected service account name 'spire-spiffe-csi-driver', got '%s'",
			daemonSet.Spec.Template.Spec.ServiceAccountName)
	}

	// Test InitContainers
	if len(daemonSet.Spec.Template.Spec.InitContainers) != 1 {
		t.Errorf("Expected 1 init container, got %d", len(daemonSet.Spec.Template.Spec.InitContainers))
	}

	initContainer := daemonSet.Spec.Template.Spec.InitContainers[0]
	testInitContainer(t, initContainer)

	// Test Containers
	if len(daemonSet.Spec.Template.Spec.Containers) != 2 {
		t.Errorf("Expected 2 containers, got %d", len(daemonSet.Spec.Template.Spec.Containers))
	}

	spiffeContainer := daemonSet.Spec.Template.Spec.Containers[0]
	registrarContainer := daemonSet.Spec.Template.Spec.Containers[1]

	testSpiffeContainer(t, spiffeContainer)
	testNodeDriverRegistrarContainer(t, registrarContainer)

	// Test Volumes
	if len(daemonSet.Spec.Template.Spec.Volumes) != 4 {
		t.Errorf("Expected 4 volumes, got %d", len(daemonSet.Spec.Template.Spec.Volumes))
	}

	testVolumes(t, daemonSet.Spec.Template.Spec.Volumes)
}

func testInitContainer(t *testing.T, container corev1.Container) {
	if container.Name != "set-context" {
		t.Errorf("Expected init container name 'set-context', got '%s'", container.Name)
	}

	if container.Image != "registry.access.redhat.com/ubi9:latest" {
		t.Errorf("Expected init container image 'registry.access.redhat.com/ubi9:latest', got '%s'", container.Image)
	}

	expectedCommand := []string{"chcon", "-Rvt", "container_file_t", "spire-agent-socket/"}
	if !reflect.DeepEqual(container.Command, expectedCommand) {
		t.Errorf("Expected init container command %v, got %v", expectedCommand, container.Command)
	}

	if container.ImagePullPolicy != corev1.PullAlways {
		t.Errorf("Expected init container pull policy '%s', got '%s'", corev1.PullAlways, container.ImagePullPolicy)
	}

	// Test SecurityContext
	if container.SecurityContext.Privileged == nil || !*container.SecurityContext.Privileged {
		t.Error("Expected init container to be privileged")
	}

	expectedCapabilities := []corev1.Capability{"all"}
	if !reflect.DeepEqual(container.SecurityContext.Capabilities.Drop, expectedCapabilities) {
		t.Errorf("Expected init container capabilities drop %v, got %v",
			expectedCapabilities, container.SecurityContext.Capabilities.Drop)
	}

	// Test VolumeMounts
	if len(container.VolumeMounts) != 1 {
		t.Errorf("Expected 1 volume mount for init container, got %d", len(container.VolumeMounts))
	}

	expectedVolumeMount := corev1.VolumeMount{
		Name:      "spire-agent-socket-dir",
		MountPath: "/spire-agent-socket",
	}

	if !reflect.DeepEqual(container.VolumeMounts[0], expectedVolumeMount) {
		t.Errorf("Expected init container volume mount %v, got %v", expectedVolumeMount, container.VolumeMounts[0])
	}

	// Test termination message settings
	if container.TerminationMessagePath != "/dev/termination-log" {
		t.Errorf("Expected termination message path '/dev/termination-log', got '%s'", container.TerminationMessagePath)
	}

	if container.TerminationMessagePolicy != corev1.TerminationMessageReadFile {
		t.Errorf("Expected termination message policy '%s', got '%s'",
			corev1.TerminationMessageReadFile, container.TerminationMessagePolicy)
	}
}

func testSpiffeContainer(t *testing.T, container corev1.Container) {
	if container.Name != "spiffe-csi-driver" {
		t.Errorf("Expected container name 'spiffe-csi-driver', got '%s'", container.Name)
	}

	// Note: In a real test, you'd mock utils.GetSpiffeCSIDriverImage()
	if container.Image != utils.GetSpiffeCSIDriverImage() {
		t.Errorf("Expected container image from utils.GetSpiffeCSIDriverImage(), got '%s'", container.Image)
	}

	expectedArgs := []string{
		"-workload-api-socket-dir", "/spire-agent-socket",
		"-plugin-name", "csi.spiffe.io",
		"-csi-socket-path", "/spiffe-csi/csi.sock",
	}

	if !reflect.DeepEqual(container.Args, expectedArgs) {
		t.Errorf("Expected container args %v, got %v", expectedArgs, container.Args)
	}

	if container.ImagePullPolicy != corev1.PullIfNotPresent {
		t.Errorf("Expected container pull policy '%s', got '%s'", corev1.PullIfNotPresent, container.ImagePullPolicy)
	}

	// Test Environment Variables
	if len(container.Env) != 1 {
		t.Errorf("Expected 1 environment variable, got %d", len(container.Env))
	}

	expectedEnv := corev1.EnvVar{
		Name: "MY_NODE_NAME",
		ValueFrom: &corev1.EnvVarSource{
			FieldRef: &corev1.ObjectFieldSelector{
				FieldPath: "spec.nodeName",
			},
		},
	}

	if !reflect.DeepEqual(container.Env[0], expectedEnv) {
		t.Errorf("Expected environment variable %v, got %v", expectedEnv, container.Env[0])
	}

	// Test SecurityContext
	if container.SecurityContext.ReadOnlyRootFilesystem == nil || !*container.SecurityContext.ReadOnlyRootFilesystem {
		t.Error("Expected container to have read-only root filesystem")
	}

	if container.SecurityContext.Privileged == nil || !*container.SecurityContext.Privileged {
		t.Error("Expected container to be privileged")
	}

	expectedCapabilities := []corev1.Capability{"all"}
	if !reflect.DeepEqual(container.SecurityContext.Capabilities.Drop, expectedCapabilities) {
		t.Errorf("Expected container capabilities drop %v, got %v",
			expectedCapabilities, container.SecurityContext.Capabilities.Drop)
	}

	// Test VolumeMounts
	if len(container.VolumeMounts) != 3 {
		t.Errorf("Expected 3 volume mounts for spiffe container, got %d", len(container.VolumeMounts))
	}

	expectedVolumeMounts := []corev1.VolumeMount{
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
	}

	for i, expectedMount := range expectedVolumeMounts {
		if !reflect.DeepEqual(container.VolumeMounts[i], expectedMount) {
			t.Errorf("Expected volume mount %d to be %v, got %v", i, expectedMount, container.VolumeMounts[i])
		}
	}
}

func testNodeDriverRegistrarContainer(t *testing.T, container corev1.Container) {
	if container.Name != "node-driver-registrar" {
		t.Errorf("Expected container name 'node-driver-registrar', got '%s'", container.Name)
	}

	// Note: In a real test, you'd mock utils.GetNodeDriverRegistrarImage()
	if container.Image != utils.GetNodeDriverRegistrarImage() {
		t.Errorf("Expected container image from utils.GetNodeDriverRegistrarImage(), got '%s'", container.Image)
	}

	expectedArgs := []string{
		"-csi-address", "/spiffe-csi/csi.sock",
		"-kubelet-registration-path", "/var/lib/kubelet/plugins/csi.spiffe.io/csi.sock",
		"-health-port", "9809",
	}

	if !reflect.DeepEqual(container.Args, expectedArgs) {
		t.Errorf("Expected container args %v, got %v", expectedArgs, container.Args)
	}

	if container.ImagePullPolicy != corev1.PullIfNotPresent {
		t.Errorf("Expected container pull policy '%s', got '%s'", corev1.PullIfNotPresent, container.ImagePullPolicy)
	}

	// Test VolumeMounts
	if len(container.VolumeMounts) != 2 {
		t.Errorf("Expected 2 volume mounts for registrar container, got %d", len(container.VolumeMounts))
	}

	expectedVolumeMounts := []corev1.VolumeMount{
		{
			Name:      "spiffe-csi-socket-dir",
			MountPath: "/spiffe-csi",
		},
		{
			Name:      "kubelet-plugin-registration-dir",
			MountPath: "/registration",
		},
	}

	for i, expectedMount := range expectedVolumeMounts {
		if !reflect.DeepEqual(container.VolumeMounts[i], expectedMount) {
			t.Errorf("Expected volume mount %d to be %v, got %v", i, expectedMount, container.VolumeMounts[i])
		}
	}

	// Test Ports
	if len(container.Ports) != 1 {
		t.Errorf("Expected 1 port for registrar container, got %d", len(container.Ports))
	}

	expectedPort := corev1.ContainerPort{
		ContainerPort: 9809,
		Name:          "healthz",
	}

	if !reflect.DeepEqual(container.Ports[0], expectedPort) {
		t.Errorf("Expected port %v, got %v", expectedPort, container.Ports[0])
	}

	// Test LivenessProbe
	if container.LivenessProbe == nil {
		t.Error("Expected liveness probe to be set")
	} else {
		if container.LivenessProbe.InitialDelaySeconds != 5 {
			t.Errorf("Expected liveness probe initial delay 5, got %d", container.LivenessProbe.InitialDelaySeconds)
		}

		if container.LivenessProbe.TimeoutSeconds != 5 {
			t.Errorf("Expected liveness probe timeout 5, got %d", container.LivenessProbe.TimeoutSeconds)
		}

		if container.LivenessProbe.HTTPGet == nil {
			t.Error("Expected HTTPGet probe handler")
		} else {
			if container.LivenessProbe.HTTPGet.Path != "/healthz" {
				t.Errorf("Expected probe path '/healthz', got '%s'", container.LivenessProbe.HTTPGet.Path)
			}

			expectedPort := intstr.FromString("healthz")
			if !reflect.DeepEqual(container.LivenessProbe.HTTPGet.Port, expectedPort) {
				t.Errorf("Expected probe port %v, got %v", expectedPort, container.LivenessProbe.HTTPGet.Port)
			}
		}
	}
}

func testVolumes(t *testing.T, volumes []corev1.Volume) {
	expectedVolumes := []corev1.Volume{
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
	}

	for i, expectedVolume := range expectedVolumes {
		if !reflect.DeepEqual(volumes[i], expectedVolume) {
			t.Errorf("Expected volume %d to be %v, got %v", i, expectedVolume, volumes[i])
		}
	}
}

func TestBoolPtr(t *testing.T) {
	tests := []struct {
		name     string
		input    bool
		expected bool
	}{
		{
			name:     "true value",
			input:    true,
			expected: true,
		},
		{
			name:     "false value",
			input:    false,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := boolPtr(tt.input)
			if result == nil {
				t.Error("Expected non-nil pointer")
				return
			}
			if *result != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, *result)
			}
		})
	}
}

func TestHostPathTypePtr(t *testing.T) {
	tests := []struct {
		name     string
		input    corev1.HostPathType
		expected corev1.HostPathType
	}{
		{
			name:     "DirectoryOrCreate",
			input:    corev1.HostPathDirectoryOrCreate,
			expected: corev1.HostPathDirectoryOrCreate,
		},
		{
			name:     "Directory",
			input:    corev1.HostPathDirectory,
			expected: corev1.HostPathDirectory,
		},
		{
			name:     "File",
			input:    corev1.HostPathFile,
			expected: corev1.HostPathFile,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := hostPathTypePtr(tt.input)
			if result == nil {
				t.Error("Expected non-nil pointer")
				return
			}
			if *result != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, *result)
			}
		})
	}
}

func TestMountPropagationPtr(t *testing.T) {
	tests := []struct {
		name     string
		input    corev1.MountPropagationMode
		expected corev1.MountPropagationMode
	}{
		{
			name:     "Bidirectional",
			input:    corev1.MountPropagationBidirectional,
			expected: corev1.MountPropagationBidirectional,
		},
		{
			name:     "HostToContainer",
			input:    corev1.MountPropagationHostToContainer,
			expected: corev1.MountPropagationHostToContainer,
		},
		{
			name:     "None",
			input:    corev1.MountPropagationNone,
			expected: corev1.MountPropagationNone,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := mountPropagationPtr(tt.input)
			if result == nil {
				t.Error("Expected non-nil pointer")
				return
			}
			if *result != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, *result)
			}
		})
	}
}

// Benchmark tests for performance validation
func BenchmarkGenerateSpiffeCsiDriverDaemonSet(b *testing.B) {
	for i := 0; i < b.N; i++ {
		generateSpiffeCsiDriverDaemonSet()
	}
}

func BenchmarkBoolPtr(b *testing.B) {
	for i := 0; i < b.N; i++ {
		boolPtr(true)
		boolPtr(false)
	}
}

func BenchmarkHostPathTypePtr(b *testing.B) {
	for i := 0; i < b.N; i++ {
		hostPathTypePtr(corev1.HostPathDirectoryOrCreate)
	}
}

func BenchmarkMountPropagationPtr(b *testing.B) {
	for i := 0; i < b.N; i++ {
		mountPropagationPtr(corev1.MountPropagationBidirectional)
	}
}
