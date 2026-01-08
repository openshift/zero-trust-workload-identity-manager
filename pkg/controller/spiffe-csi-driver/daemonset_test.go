package spiffe_csi_driver

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/go-logr/logr"
	"github.com/openshift/zero-trust-workload-identity-manager/api/v1alpha1"
	"github.com/openshift/zero-trust-workload-identity-manager/pkg/client/fakes"
	"github.com/openshift/zero-trust-workload-identity-manager/pkg/controller/status"
	"github.com/openshift/zero-trust-workload-identity-manager/pkg/controller/utils"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestGenerateSpiffeCsiDriverDaemonSet(t *testing.T) {
	// Mock the utility functions that are called in the main function
	// These would need to be properly mocked in a real test environment

	config := v1alpha1.SpiffeCSIDriverSpec{
		AgentSocketPath: "/run/spire/agent-sockets",
		PluginName:      "csi.spiffe.io",
	}

	daemonSet := generateSpiffeCsiDriverDaemonSet(config)

	// Test ObjectMeta
	if daemonSet.Name != "spire-spiffe-csi-driver" {
		t.Errorf("Expected name 'spire-spiffe-csi-driver', got '%s'", daemonSet.Name)
	}

	if daemonSet.Namespace != utils.GetOperatorNamespace() {
		t.Errorf("Expected namespace '%s', got '%s'", utils.GetOperatorNamespace(), daemonSet.Namespace)
	}

	expectedLabels := utils.SpiffeCSIDriverLabels(nil)

	if !reflect.DeepEqual(daemonSet.Labels, expectedLabels) {
		t.Errorf("Expected labels %v, got %v", expectedLabels, daemonSet.Labels)
	}

	// Test Selector - using centralized labeling approach
	allLabels := utils.SpiffeCSIDriverLabels(nil)
	expectedSelectorLabels := map[string]string{
		"app.kubernetes.io/name":      allLabels["app.kubernetes.io/name"],
		"app.kubernetes.io/instance":  allLabels["app.kubernetes.io/instance"],
		"app.kubernetes.io/component": allLabels["app.kubernetes.io/component"],
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
	if !reflect.DeepEqual(daemonSet.Spec.Template.Labels, allLabels) {
		t.Errorf("Expected template labels %v, got %v", allLabels, daemonSet.Spec.Template.Labels)
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

func TestGenerateSpiffeCsiDriverDaemonSetWithCustomConfig(t *testing.T) {
	// Test with custom plugin name and agent socket path
	config := v1alpha1.SpiffeCSIDriverSpec{
		AgentSocketPath: "/custom/agent/socket",
		PluginName:      "csi.custom.io",
	}

	daemonSet := generateSpiffeCsiDriverDaemonSet(config)

	// Verify plugin name is used in container args
	spiffeContainer := daemonSet.Spec.Template.Spec.Containers[0]
	expectedArgs := []string{
		"-workload-api-socket-dir", "/spire-agent-socket",
		"-plugin-name", "csi.custom.io",
		"-csi-socket-path", "/spiffe-csi/csi.sock",
	}
	if !reflect.DeepEqual(spiffeContainer.Args, expectedArgs) {
		t.Errorf("Expected spiffe container args %v, got %v", expectedArgs, spiffeContainer.Args)
	}

	// Verify plugin name is used in node-driver-registrar args
	registrarContainer := daemonSet.Spec.Template.Spec.Containers[1]
	expectedRegistrarArgs := []string{
		"-csi-address", "/spiffe-csi/csi.sock",
		"-kubelet-registration-path", "/var/lib/kubelet/plugins/csi.custom.io/csi.sock",
		"-health-port", "9809",
	}
	if !reflect.DeepEqual(registrarContainer.Args, expectedRegistrarArgs) {
		t.Errorf("Expected registrar container args %v, got %v", expectedRegistrarArgs, registrarContainer.Args)
	}

	// Verify custom agent socket path is used in volumes
	volumes := daemonSet.Spec.Template.Spec.Volumes
	agentSocketVolume := volumes[0] // spire-agent-socket-dir
	if agentSocketVolume.HostPath.Path != "/custom/agent/socket" {
		t.Errorf("Expected agent socket hostPath '/custom/agent/socket', got '%s'", agentSocketVolume.HostPath.Path)
	}

	// Verify plugin name is used in CSI socket directory path
	csiSocketVolume := volumes[1] // spiffe-csi-socket-dir
	expectedCSIPath := "/var/lib/kubelet/plugins/csi.custom.io"
	if csiSocketVolume.HostPath.Path != expectedCSIPath {
		t.Errorf("Expected CSI socket hostPath '%s', got '%s'", expectedCSIPath, csiSocketVolume.HostPath.Path)
	}
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

	// Test SecurityContext
	if container.SecurityContext == nil {
		t.Error("Expected SecurityContext to be set")
	} else {
		if container.SecurityContext.Privileged == nil || !*container.SecurityContext.Privileged {
			t.Error("Expected node-driver-registrar container to be privileged")
		}

		if container.SecurityContext.Capabilities == nil {
			t.Error("Expected Capabilities to be set")
		} else {
			expectedCapabilities := []corev1.Capability{"all"}
			if !reflect.DeepEqual(container.SecurityContext.Capabilities.Drop, expectedCapabilities) {
				t.Errorf("Expected node-driver-registrar container capabilities drop %v, got %v",
					expectedCapabilities, container.SecurityContext.Capabilities.Drop)
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

// TestReconcileDaemonSet tests the reconcileDaemonSet function
func TestReconcileDaemonSet(t *testing.T) {
	tests := []struct {
		name           string
		notFound       bool
		getError       error
		createError    error
		updateError    error
		createOnlyMode bool
		useEmptyScheme bool
		expectError    bool
		expectCreate   bool
		expectUpdate   bool
	}{
		{
			name:         "create success",
			notFound:     true,
			expectError:  false,
			expectCreate: true,
		},
		{
			name:        "create error",
			notFound:    true,
			createError: errors.New("create failed"),
			expectError: true,
		},
		{
			name:        "get error",
			getError:    errors.New("connection refused"),
			expectError: true,
		},
		{
			name:         "update success",
			expectError:  false,
			expectUpdate: true,
		},
		{
			name:        "update error",
			updateError: errors.New("update conflict"),
			expectError: true,
		},
		{
			name:           "create only mode skips update",
			createOnlyMode: true,
			expectError:    false,
			expectUpdate:   false,
		},
		{
			name:           "set controller reference error",
			useEmptyScheme: true,
			expectError:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fakeClient := &fakes.FakeCustomCtrlClient{}
			var reconciler *SpiffeCsiReconciler
			if tt.useEmptyScheme {
				reconciler = &SpiffeCsiReconciler{
					ctrlClient:    fakeClient,
					ctx:           context.Background(),
					log:           logr.Discard(),
					scheme:        runtime.NewScheme(),
					eventRecorder: record.NewFakeRecorder(100),
				}
			} else {
				reconciler = newDaemonSetTestReconciler(fakeClient)
			}

			driver := createDaemonSetTestDriver()
			driver.Spec.Labels = map[string]string{"new": "label"}
			statusMgr := status.NewManager(fakeClient)

			if tt.notFound {
				fakeClient.GetReturns(kerrors.NewNotFound(schema.GroupResource{}, "spiffe-csi-driver"))
			} else if tt.getError != nil {
				fakeClient.GetReturns(tt.getError)
			} else {
				existingDS := &appsv1.DaemonSet{
					ObjectMeta: metav1.ObjectMeta{
						Name:            "spiffe-csi-driver",
						Namespace:       utils.GetOperatorNamespace(),
						ResourceVersion: "123",
						Labels:          map[string]string{"old": "label"},
					},
				}
				fakeClient.GetStub = func(ctx context.Context, key client.ObjectKey, obj client.Object) error {
					if ds, ok := obj.(*appsv1.DaemonSet); ok {
						*ds = *existingDS
					}
					return nil
				}
			}
			fakeClient.CreateReturns(tt.createError)
			fakeClient.UpdateReturns(tt.updateError)

			err := reconciler.reconcileDaemonSet(context.Background(), driver, statusMgr, tt.createOnlyMode)

			if tt.expectError && err == nil {
				t.Error("Expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Expected no error, got: %v", err)
			}
			if tt.expectCreate && fakeClient.CreateCallCount() != 1 {
				t.Errorf("Expected Create to be called once, got %d", fakeClient.CreateCallCount())
			}
			if tt.expectUpdate && fakeClient.UpdateCallCount() != 1 {
				t.Errorf("Expected Update to be called once, got %d", fakeClient.UpdateCallCount())
			}
			if tt.createOnlyMode && fakeClient.UpdateCallCount() != 0 {
				t.Error("Expected Update not to be called in create-only mode")
			}
		})
	}
}

// TestNeedsUpdate tests the needsUpdate function
func TestNeedsUpdate(t *testing.T) {
	t.Run("same daemonsets do not need update", func(t *testing.T) {
		ds := appsv1.DaemonSet{
			ObjectMeta: metav1.ObjectMeta{
				Name:   "test",
				Labels: map[string]string{"app": "test"},
			},
		}

		if needsUpdate(ds, ds) {
			t.Error("Expected same daemonsets not to need update")
		}
	})

	t.Run("different labels need update", func(t *testing.T) {
		current := appsv1.DaemonSet{
			ObjectMeta: metav1.ObjectMeta{
				Name:   "test",
				Labels: map[string]string{"old": "label"},
			},
		}
		desired := appsv1.DaemonSet{
			ObjectMeta: metav1.ObjectMeta{
				Name:   "test",
				Labels: map[string]string{"new": "label"},
			},
		}

		if !needsUpdate(current, desired) {
			t.Error("Expected different labels to need update")
		}
	})
}

// newDaemonSetTestReconciler creates a reconciler for DaemonSet tests
func newDaemonSetTestReconciler(fakeClient *fakes.FakeCustomCtrlClient) *SpiffeCsiReconciler {
	scheme := runtime.NewScheme()
	_ = v1alpha1.AddToScheme(scheme)
	_ = appsv1.AddToScheme(scheme)
	return &SpiffeCsiReconciler{
		ctrlClient:    fakeClient,
		ctx:           context.Background(),
		log:           logr.Discard(),
		scheme:        scheme,
		eventRecorder: record.NewFakeRecorder(100),
	}
}

// createDaemonSetTestDriver creates a test SpiffeCSIDriver for DaemonSet tests
func createDaemonSetTestDriver() *v1alpha1.SpiffeCSIDriver {
	return &v1alpha1.SpiffeCSIDriver{
		ObjectMeta: metav1.ObjectMeta{
			Name: "cluster",
			UID:  "test-uid",
		},
		Spec: v1alpha1.SpiffeCSIDriverSpec{
			AgentSocketPath: "/run/spire/agent-sockets",
			PluginName:      "csi.spiffe.io",
		},
	}
}
