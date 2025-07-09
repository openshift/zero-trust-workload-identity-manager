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
	config := &v1alpha1.SpireServerSpec{
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
		configWithNilLabels := &v1alpha1.SpireServerSpec{
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
		configWithEmptyLabels := &v1alpha1.SpireServerSpec{
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

func TestGetUpstreamAuthoritySecretMounts(t *testing.T) {
	tests := []struct {
		name           string
		upstreamAuth   *v1alpha1.UpstreamAuthority
		expectedMounts []secretMountInfo
	}{
		{
			name:           "nil upstream authority",
			upstreamAuth:   nil,
			expectedMounts: []secretMountInfo{},
		},
		{
			name: "cert-manager without kubeconfig",
			upstreamAuth: &v1alpha1.UpstreamAuthority{
				Type: "cert-manager",
				CertManager: &v1alpha1.UpstreamAuthorityCertManager{
					IssuerName: "spire-ca",
				},
			},
			expectedMounts: []secretMountInfo{},
		},
		{
			name: "cert-manager with kubeconfig",
			upstreamAuth: &v1alpha1.UpstreamAuthority{
				Type: "cert-manager",
				CertManager: &v1alpha1.UpstreamAuthorityCertManager{
					IssuerName:           "spire-ca",
					KubeConfigSecretName: "kubeconfig-secret",
				},
			},
			expectedMounts: []secretMountInfo{
				{
					secretName: "kubeconfig-secret",
					mountPath:  "/cert-manager-kubeconfig",
					volumeName: "cert-manager-kubeconfig",
				},
			},
		},
		{
			name: "spire upstream authority",
			upstreamAuth: &v1alpha1.UpstreamAuthority{
				Type: "spire",
				Spire: &v1alpha1.UpstreamAuthoritySpire{
					ServerAddress:     "upstream-spire-server",
					ServerPort:        "8081",
					WorkloadSocketAPI: "/tmp/spire-agent/public/api.sock",
				},
			},
			expectedMounts: []secretMountInfo{},
		},
		{
			name: "vault with token auth",
			upstreamAuth: &v1alpha1.UpstreamAuthority{
				Type: "vault",
				Vault: &v1alpha1.UpstreamAuthorityVault{
					VaultAddress:  "https://vault.example.org/",
					PkiMountPoint: "pki",
					CaCertSecret:  "vault-ca-secret",
					TokenAuth: &v1alpha1.TokenAuth{
						Token: "hvs.test-token",
					},
				},
			},
			expectedMounts: []secretMountInfo{
				{
					secretName: "vault-ca-secret",
					mountPath:  "/vault-ca-cert",
					volumeName: "vault-ca-cert",
				},
			},
		},
		{
			name: "vault with cert auth",
			upstreamAuth: &v1alpha1.UpstreamAuthority{
				Type: "vault",
				Vault: &v1alpha1.UpstreamAuthorityVault{
					VaultAddress:  "https://vault.example.org/",
					PkiMountPoint: "pki",
					CaCertSecret:  "vault-ca-secret",
					CertAuth: &v1alpha1.CertAuth{
						CertAuthMountPoint: "cert",
						ClientCertSecret:   "client-cert-secret",
						ClientKeySecret:    "client-key-secret",
					},
				},
			},
			expectedMounts: []secretMountInfo{
				{
					secretName: "vault-ca-secret",
					mountPath:  "/vault-ca-cert",
					volumeName: "vault-ca-cert",
				},
				{
					secretName: "client-cert-secret",
					mountPath:  "/vault-client-cert",
					volumeName: "vault-client-cert",
				},
				{
					secretName: "client-key-secret",
					mountPath:  "/vault-client-key",
					volumeName: "vault-client-key",
				},
			},
		},
		{
			name: "vault with approle auth",
			upstreamAuth: &v1alpha1.UpstreamAuthority{
				Type: "vault",
				Vault: &v1alpha1.UpstreamAuthorityVault{
					VaultAddress:  "https://vault.example.org/",
					PkiMountPoint: "pki",
					CaCertSecret:  "vault-ca-secret",
					AppRoleAuth: &v1alpha1.AppRoleAuth{
						AppRoleMountPoint: "approle",
						AppRoleID:         "role-id-123",
						AppRoleSecretID:   "secret-id-456",
					},
				},
			},
			expectedMounts: []secretMountInfo{
				{
					secretName: "vault-ca-secret",
					mountPath:  "/vault-ca-cert",
					volumeName: "vault-ca-cert",
				},
			},
		},
		{
			name: "vault with k8s auth",
			upstreamAuth: &v1alpha1.UpstreamAuthority{
				Type: "vault",
				Vault: &v1alpha1.UpstreamAuthorityVault{
					VaultAddress:  "https://vault.example.org/",
					PkiMountPoint: "pki",
					CaCertSecret:  "vault-ca-secret",
					K8sAuth: &v1alpha1.K8sAuth{
						K8sAuthMountPoint: "kubernetes",
						K8sAuthRoleName:   "spire-role",
					},
				},
			},
			expectedMounts: []secretMountInfo{
				{
					secretName: "vault-ca-secret",
					mountPath:  "/vault-ca-cert",
					volumeName: "vault-ca-cert",
				},
			},
		},
		{
			name: "unsupported upstream authority type",
			upstreamAuth: &v1alpha1.UpstreamAuthority{
				Type: "unsupported",
			},
			expectedMounts: []secretMountInfo{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mounts := getUpstreamAuthoritySecretMounts(tt.upstreamAuth)

			if len(mounts) != len(tt.expectedMounts) {
				t.Errorf("Expected %d mounts, got %d", len(tt.expectedMounts), len(mounts))
			}

			for i, expectedMount := range tt.expectedMounts {
				if i >= len(mounts) {
					t.Errorf("Expected mount at index %d not found", i)
					continue
				}

				actualMount := mounts[i]
				if actualMount.secretName != expectedMount.secretName {
					t.Errorf("Expected secretName %q, got %q", expectedMount.secretName, actualMount.secretName)
				}
				if actualMount.mountPath != expectedMount.mountPath {
					t.Errorf("Expected mountPath %q, got %q", expectedMount.mountPath, actualMount.mountPath)
				}
				if actualMount.volumeName != expectedMount.volumeName {
					t.Errorf("Expected volumeName %q, got %q", expectedMount.volumeName, actualMount.volumeName)
				}
			}
		})
	}
}

func TestGenerateSpireServerStatefulSetWithSecretMounts(t *testing.T) {
	tests := []struct {
		name                     string
		upstreamAuth             *v1alpha1.UpstreamAuthority
		expectedSecretVolumes    []string
		expectedVolumeMounts     []string
		expectedBasicVolumeCount int
		expectedBasicMountCount  int
	}{
		{
			name:                     "no upstream authority",
			upstreamAuth:             nil,
			expectedSecretVolumes:    []string{},
			expectedVolumeMounts:     []string{},
			expectedBasicVolumeCount: 5,
			expectedBasicMountCount:  4,
		},
		{
			name: "cert-manager with kubeconfig",
			upstreamAuth: &v1alpha1.UpstreamAuthority{
				Type: "cert-manager",
				CertManager: &v1alpha1.UpstreamAuthorityCertManager{
					IssuerName:           "spire-ca",
					KubeConfigSecretName: "kubeconfig-secret",
				},
			},
			expectedSecretVolumes:    []string{"cert-manager-kubeconfig"},
			expectedVolumeMounts:     []string{"cert-manager-kubeconfig"},
			expectedBasicVolumeCount: 6,
			expectedBasicMountCount:  5,
		},
		{
			name: "vault with cert auth",
			upstreamAuth: &v1alpha1.UpstreamAuthority{
				Type: "vault",
				Vault: &v1alpha1.UpstreamAuthorityVault{
					VaultAddress:  "https://vault.example.org/",
					PkiMountPoint: "pki",
					CaCertSecret:  "vault-ca-secret",
					CertAuth: &v1alpha1.CertAuth{
						CertAuthMountPoint: "cert",
						ClientCertSecret:   "client-cert-secret",
						ClientKeySecret:    "client-key-secret",
					},
				},
			},
			expectedSecretVolumes:    []string{"vault-ca-cert", "vault-client-cert", "vault-client-key"},
			expectedVolumeMounts:     []string{"vault-ca-cert", "vault-client-cert", "vault-client-key"},
			expectedBasicVolumeCount: 8,
			expectedBasicMountCount:  7,
		},
		{
			name: "vault with token auth",
			upstreamAuth: &v1alpha1.UpstreamAuthority{
				Type: "vault",
				Vault: &v1alpha1.UpstreamAuthorityVault{
					VaultAddress:  "https://vault.example.org/",
					PkiMountPoint: "pki",
					CaCertSecret:  "vault-ca-secret",
					TokenAuth: &v1alpha1.TokenAuth{
						Token: "hvs.test-token",
					},
				},
			},
			expectedSecretVolumes:    []string{"vault-ca-cert"},
			expectedVolumeMounts:     []string{"vault-ca-cert"},
			expectedBasicVolumeCount: 6,
			expectedBasicMountCount:  5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &v1alpha1.SpireServerSpec{
				TrustDomain:     "example.org",
				ClusterName:     "test-cluster",
				BundleConfigMap: "spire-bundle",
				JwtIssuer:       "example.org",
				CASubject: &v1alpha1.CASubject{
					CommonName:   "SPIRE Server CA",
					Country:      "US",
					Organization: "SPIRE",
				},
				Datastore: &v1alpha1.DataStore{
					ConnectionString: "postgresql://postgres:password@postgres:5432/spire",
					DatabaseType:     "postgres",
					DisableMigration: "false",
					MaxIdleConns:     10,
					MaxOpenConns:     20,
				},
				Persistence: &v1alpha1.Persistence{
					Type:       "pvc",
					Size:       "1Gi",
					AccessMode: "ReadWriteOnce",
				},
				CommonConfig: v1alpha1.CommonConfig{
					Labels: map[string]string{
						"custom-label": "value",
					},
				},
				UpstreamAuthority: tt.upstreamAuth,
			}

			statefulSet := GenerateSpireServerStatefulSet(config, "test-hash", "test-hash")

			// Check total volume count
			volumes := statefulSet.Spec.Template.Spec.Volumes
			if len(volumes) != tt.expectedBasicVolumeCount {
				t.Errorf("Expected %d total volumes, got %d", tt.expectedBasicVolumeCount, len(volumes))
			}

			// Check that the StatefulSet has the expected secret volumes
			secretVolumeNames := []string{}
			for _, volume := range volumes {
				if volume.Secret != nil {
					secretVolumeNames = append(secretVolumeNames, volume.Name)
				}
			}

			if len(secretVolumeNames) != len(tt.expectedSecretVolumes) {
				t.Errorf("Expected %d secret volumes, got %d", len(tt.expectedSecretVolumes), len(secretVolumeNames))
			}

			for _, expectedVolume := range tt.expectedSecretVolumes {
				found := false
				for _, actualVolume := range secretVolumeNames {
					if actualVolume == expectedVolume {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Expected secret volume %q not found", expectedVolume)
				}
			}

			// Check that the spire-server container has the expected volume mounts
			spireServerContainer := findContainerByName(statefulSet.Spec.Template.Spec.Containers, "spire-server")
			if spireServerContainer == nil {
				t.Fatal("spire-server container not found")
			}

			// Check total mount count
			if len(spireServerContainer.VolumeMounts) != tt.expectedBasicMountCount {
				t.Errorf("Expected %d total volume mounts, got %d", tt.expectedBasicMountCount, len(spireServerContainer.VolumeMounts))
			}

			secretVolumeMountNames := []string{}
			for _, volumeMount := range spireServerContainer.VolumeMounts {
				// Check if this volume mount corresponds to a secret volume
				for _, volumeName := range tt.expectedVolumeMounts {
					if volumeMount.Name == volumeName {
						secretVolumeMountNames = append(secretVolumeMountNames, volumeMount.Name)
						break
					}
				}
			}

			if len(secretVolumeMountNames) != len(tt.expectedVolumeMounts) {
				t.Errorf("Expected %d secret volume mounts, got %d", len(tt.expectedVolumeMounts), len(secretVolumeMountNames))
			}

			for _, expectedMount := range tt.expectedVolumeMounts {
				found := false
				for _, actualMount := range secretVolumeMountNames {
					if actualMount == expectedMount {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Expected secret volume mount %q not found", expectedMount)
				}
			}

			// Verify that secret volume mounts are read-only
			for _, volumeMount := range spireServerContainer.VolumeMounts {
				for _, expectedMount := range tt.expectedVolumeMounts {
					if volumeMount.Name == expectedMount {
						if !volumeMount.ReadOnly {
							t.Errorf("Expected secret volume mount %q to be read-only", expectedMount)
						}
					}
				}
			}
		})
	}
}

func TestGenerateSpireServerStatefulSetSecretVolumeMapping(t *testing.T) {
	tests := []struct {
		name              string
		upstreamAuth      *v1alpha1.UpstreamAuthority
		expectedSecretMap map[string]string // volumeName -> secretName
	}{
		{
			name: "cert-manager with kubeconfig",
			upstreamAuth: &v1alpha1.UpstreamAuthority{
				Type: "cert-manager",
				CertManager: &v1alpha1.UpstreamAuthorityCertManager{
					IssuerName:           "spire-ca",
					KubeConfigSecretName: "my-kubeconfig-secret",
				},
			},
			expectedSecretMap: map[string]string{
				"cert-manager-kubeconfig": "my-kubeconfig-secret",
			},
		},
		{
			name: "vault with cert auth",
			upstreamAuth: &v1alpha1.UpstreamAuthority{
				Type: "vault",
				Vault: &v1alpha1.UpstreamAuthorityVault{
					VaultAddress:  "https://vault.example.org/",
					PkiMountPoint: "pki",
					CaCertSecret:  "my-vault-ca-secret",
					CertAuth: &v1alpha1.CertAuth{
						CertAuthMountPoint: "cert",
						ClientCertSecret:   "my-client-cert-secret",
						ClientKeySecret:    "my-client-key-secret",
					},
				},
			},
			expectedSecretMap: map[string]string{
				"vault-ca-cert":     "my-vault-ca-secret",
				"vault-client-cert": "my-client-cert-secret",
				"vault-client-key":  "my-client-key-secret",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &v1alpha1.SpireServerSpec{
				TrustDomain:     "example.org",
				ClusterName:     "test-cluster",
				BundleConfigMap: "spire-bundle",
				JwtIssuer:       "example.org",
				CASubject: &v1alpha1.CASubject{
					CommonName:   "SPIRE Server CA",
					Country:      "US",
					Organization: "SPIRE",
				},
				Datastore: &v1alpha1.DataStore{
					ConnectionString: "postgresql://postgres:password@postgres:5432/spire",
					DatabaseType:     "postgres",
					DisableMigration: "false",
					MaxIdleConns:     10,
					MaxOpenConns:     20,
				},
				UpstreamAuthority: tt.upstreamAuth,
			}

			statefulSet := GenerateSpireServerStatefulSet(config, "test-hash", "test-hash")

			// Check that volumes are correctly mapped to secrets
			volumes := statefulSet.Spec.Template.Spec.Volumes
			for volumeName, expectedSecretName := range tt.expectedSecretMap {
				found := false
				for _, volume := range volumes {
					if volume.Name == volumeName {
						found = true
						if volume.Secret == nil {
							t.Errorf("Expected volume %q to be a secret volume", volumeName)
						} else if volume.Secret.SecretName != expectedSecretName {
							t.Errorf("Expected volume %q to reference secret %q, got %q", volumeName, expectedSecretName, volume.Secret.SecretName)
						}
						break
					}
				}
				if !found {
					t.Errorf("Expected volume %q not found", volumeName)
				}
			}
		})
	}
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
