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
	"k8s.io/utils/ptr"
)

func TestGenerateSpireServerStatefulSet(t *testing.T) {
	// Setup test inputs with required Persistence field (now a value type, not pointer)
	config := &v1alpha1.SpireServerSpec{
		Persistence: v1alpha1.Persistence{
			Size:         "1Gi",
			AccessMode:   "ReadWriteOnce",
			StorageClass: "",
		},
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

		if statefulSet.Namespace != utils.GetOperatorNamespace() {
			t.Errorf("Expected namespace %q, got %q", utils.GetOperatorNamespace(), statefulSet.Namespace)
		}

		// Check standard labels
		customLabels := map[string]string{"custom-label": "test-value"}
		expectedLabels := utils.SpireServerLabels(customLabels)

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

		// Default should have no storage class set (uses cluster default)
		if pvc.Spec.StorageClassName != nil {
			t.Errorf("Expected nil storage class name for default config, got %v", *pvc.Spec.StorageClassName)
		}

		storageRequest := pvc.Spec.Resources.Requests[corev1.ResourceStorage]
		expectedStorage := resource.MustParse("1Gi")
		if !storageRequest.Equal(expectedStorage) {
			t.Errorf("Expected storage request %v, got %v", expectedStorage, storageRequest)
		}
	})

	// Test custom persistence settings (AccessMode and StorageClass)
	t.Run("Validates custom persistence settings", func(t *testing.T) {
		customStorageClass := "fast-ssd"
		configWithPersistence := &v1alpha1.SpireServerSpec{
			Persistence: v1alpha1.Persistence{
				Size:         "10Gi",
				AccessMode:   "ReadWriteOncePod",
				StorageClass: customStorageClass,
			},
		}

		customStatefulSet := GenerateSpireServerStatefulSet(configWithPersistence, serverConfigHash, controllerConfigHash)
		pvc := customStatefulSet.Spec.VolumeClaimTemplates[0]

		// Check AccessMode
		if len(pvc.Spec.AccessModes) != 1 || pvc.Spec.AccessModes[0] != corev1.ReadWriteOncePod {
			t.Errorf("Expected access mode ReadWriteOncePod, got %v", pvc.Spec.AccessModes)
		}

		// Check StorageClassName
		if pvc.Spec.StorageClassName == nil || *pvc.Spec.StorageClassName != customStorageClass {
			var actualStorageClass string
			if pvc.Spec.StorageClassName != nil {
				actualStorageClass = *pvc.Spec.StorageClassName
			}
			t.Errorf("Expected storage class name %q, got %q", customStorageClass, actualStorageClass)
		}

		// Check Size
		storageRequest := pvc.Spec.Resources.Requests[corev1.ResourceStorage]
		expectedStorage := resource.MustParse("10Gi")
		if !storageRequest.Equal(expectedStorage) {
			t.Errorf("Expected storage request %v, got %v", expectedStorage, storageRequest)
		}
	})

	// Test ReadWriteMany access mode
	t.Run("Validates ReadWriteMany access mode", func(t *testing.T) {
		configWithRWX := &v1alpha1.SpireServerSpec{
			Persistence: v1alpha1.Persistence{
				Size:       "1Gi",
				AccessMode: "ReadWriteMany",
			},
		}

		rwxStatefulSet := GenerateSpireServerStatefulSet(configWithRWX, serverConfigHash, controllerConfigHash)
		pvc := rwxStatefulSet.Spec.VolumeClaimTemplates[0]

		if len(pvc.Spec.AccessModes) != 1 || pvc.Spec.AccessModes[0] != corev1.ReadWriteMany {
			t.Errorf("Expected access mode ReadWriteMany, got %v", pvc.Spec.AccessModes)
		}
	})

	// Test persistence with custom StorageClass
	t.Run("Validates persistence with custom StorageClass", func(t *testing.T) {
		customStorageClass := "premium-storage"
		configWithCustomStorageClass := &v1alpha1.SpireServerSpec{
			Persistence: v1alpha1.Persistence{
				Size:         "1Gi",
				AccessMode:   "ReadWriteOnce",
				StorageClass: customStorageClass,
			},
		}

		storageClassStatefulSet := GenerateSpireServerStatefulSet(configWithCustomStorageClass, serverConfigHash, controllerConfigHash)
		pvc := storageClassStatefulSet.Spec.VolumeClaimTemplates[0]

		// Verify AccessMode is set correctly
		if len(pvc.Spec.AccessModes) != 1 || pvc.Spec.AccessModes[0] != corev1.ReadWriteOnce {
			t.Errorf("Expected access mode ReadWriteOnce, got %v", pvc.Spec.AccessModes)
		}

		// Verify StorageClassName is set correctly
		if pvc.Spec.StorageClassName == nil || *pvc.Spec.StorageClassName != customStorageClass {
			var actualStorageClass string
			if pvc.Spec.StorageClassName != nil {
				actualStorageClass = *pvc.Spec.StorageClassName
			}
			t.Errorf("Expected storage class name %q, got %q", customStorageClass, actualStorageClass)
		}
	})

	// Test with nil labels
	t.Run("Handles nil labels gracefully", func(t *testing.T) {
		configWithNilLabels := &v1alpha1.SpireServerSpec{
			Persistence: v1alpha1.Persistence{
				Size:       "1Gi",
				AccessMode: "ReadWriteOnce",
			},
			CommonConfig: v1alpha1.CommonConfig{
				Labels: nil,
			},
		}

		statefulSet := GenerateSpireServerStatefulSet(configWithNilLabels, serverConfigHash, controllerConfigHash)

		// Verify we have all standard labels
		expectedLabels := utils.SpireServerLabels(nil)

		for k, v := range expectedLabels {
			if statefulSet.Labels[k] != v {
				t.Errorf("Expected label %q to be %q, got %q", k, v, statefulSet.Labels[k])
			}
		}
	})

	// Test with empty labels map
	t.Run("Handles empty labels map gracefully", func(t *testing.T) {
		configWithEmptyLabels := &v1alpha1.SpireServerSpec{
			Persistence: v1alpha1.Persistence{
				Size:       "1Gi",
				AccessMode: "ReadWriteOnce",
			},
			CommonConfig: v1alpha1.CommonConfig{
				Labels: map[string]string{},
			},
		}

		statefulSet := GenerateSpireServerStatefulSet(configWithEmptyLabels, serverConfigHash, controllerConfigHash)

		// Verify we have all standard labels
		expectedLabels := utils.SpireServerLabels(nil)

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

func TestGenerateSpireServerStatefulSetWithTLSSecret(t *testing.T) {
	serverConfigHash := "test-server-hash"
	controllerConfigHash := "test-controller-hash"

	t.Run("Adds TLS Secret volume and mount at fixed path", func(t *testing.T) {
		config := &v1alpha1.SpireServerSpec{
			Persistence: v1alpha1.Persistence{
				Size: "1Gi",
			},
			Datastore: v1alpha1.DataStore{
				DatabaseType:     "postgres",
				ConnectionString: "dbname=spire user=spire host=postgres.example.com sslmode=verify-full sslrootcert=/run/spire/db-tls/ca.crt",
				TLSSecretName:    "postgres-tls-certs",
			},
		}

		statefulSet := GenerateSpireServerStatefulSet(config, serverConfigHash, controllerConfigHash)
		podSpec := statefulSet.Spec.Template.Spec

		// Check that we have 6 volumes (5 base + 1 TLS)
		expectedVolumeCount := 6
		if len(podSpec.Volumes) != expectedVolumeCount {
			t.Errorf("Expected %d volumes, got %d", expectedVolumeCount, len(podSpec.Volumes))
		}

		// Find the db-tls volume
		var dbTLSVolume *corev1.Volume
		for i := range podSpec.Volumes {
			if podSpec.Volumes[i].Name == "db-tls" {
				dbTLSVolume = &podSpec.Volumes[i]
				break
			}
		}

		if dbTLSVolume == nil {
			t.Fatal("db-tls volume not found")
		}

		if dbTLSVolume.VolumeSource.Secret == nil {
			t.Fatal("db-tls volume should be a Secret volume")
		}

		if dbTLSVolume.VolumeSource.Secret.SecretName != "postgres-tls-certs" {
			t.Errorf("Expected secret name 'postgres-tls-certs', got %q", dbTLSVolume.VolumeSource.Secret.SecretName)
		}

		// Check the spire-server container has the TLS volume mount
		spireServerContainer := findContainerByName(podSpec.Containers, "spire-server")
		if spireServerContainer == nil {
			t.Fatal("spire-server container not found")
		}

		// Should have 5 volume mounts (4 base + 1 TLS)
		expectedVolumeMountCount := 5
		if len(spireServerContainer.VolumeMounts) != expectedVolumeMountCount {
			t.Errorf("Expected %d volume mounts, got %d", expectedVolumeMountCount, len(spireServerContainer.VolumeMounts))
		}

		// Find the db-tls volume mount
		var dbTLSMount *corev1.VolumeMount
		for i := range spireServerContainer.VolumeMounts {
			if spireServerContainer.VolumeMounts[i].Name == "db-tls" {
				dbTLSMount = &spireServerContainer.VolumeMounts[i]
				break
			}
		}

		if dbTLSMount == nil {
			t.Fatal("db-tls volume mount not found")
		}

		// Mount path should always be the fixed path
		if dbTLSMount.MountPath != DBTLSMountPath {
			t.Errorf("Expected mount path %q, got %q", DBTLSMountPath, dbTLSMount.MountPath)
		}
	})

	t.Run("Does not add TLS volume when TLSSecretName is empty", func(t *testing.T) {
		config := &v1alpha1.SpireServerSpec{
			Persistence: v1alpha1.Persistence{
				Size: "1Gi",
			},
			Datastore: v1alpha1.DataStore{
				DatabaseType:     "postgres",
				ConnectionString: "dbname=spire user=spire host=postgres.example.com sslmode=disable",
				TLSSecretName:    "",
			},
		}

		statefulSet := GenerateSpireServerStatefulSet(config, serverConfigHash, controllerConfigHash)
		podSpec := statefulSet.Spec.Template.Spec

		// Should have 5 volumes (no TLS volume)
		expectedVolumeCount := 5
		if len(podSpec.Volumes) != expectedVolumeCount {
			t.Errorf("Expected %d volumes, got %d", expectedVolumeCount, len(podSpec.Volumes))
		}

		// Ensure no db-tls volume exists
		for _, volume := range podSpec.Volumes {
			if volume.Name == "db-tls" {
				t.Error("db-tls volume should not exist when TLSSecretName is empty")
			}
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
func createReferenceStatefulSet(config *v1alpha1.SpireServerSpec, spireServerConfigMapHash string,
	spireControllerMangerConfigMapHash string) *appsv1.StatefulSet {
	// Use the same standardized labeling as the actual implementation
	labels := utils.SpireServerLabels(config.Labels)

	// For selectors, we need only the core identifying labels (without custom user labels)
	selectorLabels := map[string]string{
		"app.kubernetes.io/name":      labels["app.kubernetes.io/name"],
		"app.kubernetes.io/instance":  labels["app.kubernetes.io/instance"],
		"app.kubernetes.io/component": labels["app.kubernetes.io/component"],
	}

	// Handle persistence settings (Persistence is required, so fields are always present)
	volumeAccessMode := corev1.PersistentVolumeAccessMode(config.Persistence.AccessMode)
	var storageClassName *string
	if config.Persistence.StorageClass != "" {
		storageClassName = ptr.To(config.Persistence.StorageClass)
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
						AccessModes:      []corev1.PersistentVolumeAccessMode{volumeAccessMode},
						StorageClassName: storageClassName,
						Resources: corev1.VolumeResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceStorage: resource.MustParse(config.Persistence.Size),
							},
						},
					},
				},
			},
		},
	}
}

func TestAddFederationConfigurationToStatefulSet(t *testing.T) {
	tests := []struct {
		name               string
		federation         *v1alpha1.FederationConfig
		expectVolume       bool
		expectVolumeMount  bool
		expectedSecretName string
	}{
		{
			name: "Federation with ServingCert using service CA certificate",
			federation: &v1alpha1.FederationConfig{
				BundleEndpoint: v1alpha1.BundleEndpointConfig{
					Profile: v1alpha1.HttpsWebProfile,
					HttpsWeb: &v1alpha1.HttpsWebConfig{
						ServingCert: &v1alpha1.ServingCertConfig{},
					},
				},
			},
			expectVolume:       true,
			expectVolumeMount:  true,
			expectedSecretName: utils.SpireServerServingCertName,
		},
		{
			name: "Federation with ACME (no volume needed)",
			federation: &v1alpha1.FederationConfig{
				BundleEndpoint: v1alpha1.BundleEndpointConfig{
					Profile: v1alpha1.HttpsWebProfile,
					HttpsWeb: &v1alpha1.HttpsWebConfig{
						Acme: &v1alpha1.AcmeConfig{
							DirectoryUrl: "https://acme-v02.api.letsencrypt.org/directory",
							DomainName:   "example.org",
							Email:        "admin@example.org",
							TosAccepted:  "true",
						},
					},
				},
			},
			expectVolume:      false,
			expectVolumeMount: false,
		},
		{
			name: "Federation with https_spiffe (no volume needed)",
			federation: &v1alpha1.FederationConfig{
				BundleEndpoint: v1alpha1.BundleEndpointConfig{
					Profile: v1alpha1.HttpsSpiffeProfile,
				},
			},
			expectVolume:      false,
			expectVolumeMount: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a basic StatefulSet
			sts := &appsv1.StatefulSet{
				Spec: appsv1.StatefulSetSpec{
					Template: corev1.PodTemplateSpec{
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{
								{
									Name:         "spire-server",
									VolumeMounts: []corev1.VolumeMount{},
								},
							},
							Volumes: []corev1.Volume{},
						},
					},
				},
			}

			// Add federation configuration
			addFederationConfigurationToStatefulSet(sts, tt.federation)

			// Check volume
			volumeFound := false
			var foundVolume *corev1.Volume
			for i := range sts.Spec.Template.Spec.Volumes {
				if sts.Spec.Template.Spec.Volumes[i].Name == "spire-server-tls" {
					volumeFound = true
					foundVolume = &sts.Spec.Template.Spec.Volumes[i]
					break
				}
			}

			if tt.expectVolume != volumeFound {
				t.Errorf("Expected volume presence: %v, got: %v", tt.expectVolume, volumeFound)
			}

			if tt.expectVolume && foundVolume != nil {
				// Verify the secret name
				if foundVolume.VolumeSource.Secret == nil {
					t.Fatal("Expected volume to be from Secret, got nil")
				}
				if foundVolume.VolumeSource.Secret.SecretName != tt.expectedSecretName {
					t.Errorf("Expected secret name %q, got %q", tt.expectedSecretName, foundVolume.VolumeSource.Secret.SecretName)
				}
			}

			// Check volume mount
			volumeMountFound := false
			var foundVolumeMount *corev1.VolumeMount
			for i := range sts.Spec.Template.Spec.Containers[0].VolumeMounts {
				if sts.Spec.Template.Spec.Containers[0].VolumeMounts[i].Name == "spire-server-tls" {
					volumeMountFound = true
					foundVolumeMount = &sts.Spec.Template.Spec.Containers[0].VolumeMounts[i]
					break
				}
			}

			if tt.expectVolumeMount != volumeMountFound {
				t.Errorf("Expected volume mount presence: %v, got: %v", tt.expectVolumeMount, volumeMountFound)
			}

			if tt.expectVolumeMount && foundVolumeMount != nil {
				// Verify the mount path
				expectedPath := "/run/spire/server-tls"
				if foundVolumeMount.MountPath != expectedPath {
					t.Errorf("Expected mount path %q, got %q", expectedPath, foundVolumeMount.MountPath)
				}
				if !foundVolumeMount.ReadOnly {
					t.Error("Expected volume mount to be read-only")
				}
			}
		})
	}
}

func TestGenerateSpireServerStatefulSetWithFederation(t *testing.T) {
	tests := []struct {
		name                 string
		federation           *v1alpha1.FederationConfig
		expectedVolumeCount  int
		expectedPortCount    int
		expectTLSVolume      bool
		expectFederationPort bool
	}{
		{
			name:                 "Without federation",
			federation:           nil,
			expectedVolumeCount:  5,
			expectedPortCount:    2, // grpc, healthz only
			expectTLSVolume:      false,
			expectFederationPort: false,
		},
		{
			name: "With https_spiffe federation",
			federation: &v1alpha1.FederationConfig{
				BundleEndpoint: v1alpha1.BundleEndpointConfig{
					Profile: v1alpha1.HttpsSpiffeProfile,
				},
			},
			expectedVolumeCount:  5, // No additional volume needed for https_spiffe
			expectedPortCount:    3, // grpc, healthz, federation
			expectTLSVolume:      false,
			expectFederationPort: true,
		},
		{
			name: "With https_web federation and ServingCert",
			federation: &v1alpha1.FederationConfig{
				BundleEndpoint: v1alpha1.BundleEndpointConfig{
					Profile: v1alpha1.HttpsWebProfile,
					HttpsWeb: &v1alpha1.HttpsWebConfig{
						ServingCert: &v1alpha1.ServingCertConfig{},
					},
				},
			},
			expectedVolumeCount:  6, // One additional volume for TLS cert
			expectedPortCount:    3,
			expectTLSVolume:      true,
			expectFederationPort: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &v1alpha1.SpireServerSpec{
				Persistence: v1alpha1.Persistence{
					Size:         "1Gi",
					AccessMode:   "ReadWriteOnce",
					StorageClass: "",
				},
				Federation: tt.federation,
			}

			sts := GenerateSpireServerStatefulSet(config, "test-hash", "test-hash")

			// Check volume count
			if len(sts.Spec.Template.Spec.Volumes) != tt.expectedVolumeCount {
				t.Errorf("Expected %d volumes, got %d", tt.expectedVolumeCount, len(sts.Spec.Template.Spec.Volumes))
			}

			// Check for TLS volume
			tlsVolumeFound := false
			for _, vol := range sts.Spec.Template.Spec.Volumes {
				if vol.Name == "spire-server-tls" {
					tlsVolumeFound = true
					break
				}
			}
			if tt.expectTLSVolume != tlsVolumeFound {
				t.Errorf("Expected TLS volume presence: %v, got: %v", tt.expectTLSVolume, tlsVolumeFound)
			}

			// Check ports
			spireServerContainer := findContainerByName(sts.Spec.Template.Spec.Containers, "spire-server")
			if spireServerContainer == nil {
				t.Fatal("spire-server container not found")
			}

			if len(spireServerContainer.Ports) != tt.expectedPortCount {
				t.Errorf("Expected %d ports, got %d", tt.expectedPortCount, len(spireServerContainer.Ports))
			}

			// Check for federation port (only when federation is configured)
			federationPortFound := false
			for _, port := range spireServerContainer.Ports {
				if port.Name == "federation" {
					federationPortFound = true
					if port.ContainerPort != 8443 {
						t.Errorf("Expected federation port to be 8443, got %d", port.ContainerPort)
					}
					if port.Protocol != corev1.ProtocolTCP {
						t.Errorf("Expected federation port protocol to be TCP, got %s", port.Protocol)
					}
					break
				}
			}

			if tt.expectFederationPort != federationPortFound {
				t.Errorf("Expected federation port presence: %v, got: %v", tt.expectFederationPort, federationPortFound)
			}
		})
	}
}
