package utils

import (
	"os"
	"testing"

	corev1 "k8s.io/api/core/v1"
)

func TestGetProxyEnvVars(t *testing.T) {
	tests := []struct {
		name           string
		httpProxy      string
		httpsProxy     string
		noProxy        string
		expectedCount  int
		expectedEnvMap map[string]string
	}{
		{
			name:          "all proxy vars set",
			httpProxy:     "http://proxy.example.com:8080",
			httpsProxy:    "https://proxy.example.com:8443",
			noProxy:       ".cluster.local,.svc",
			expectedCount: 3,
			expectedEnvMap: map[string]string{
				"HTTP_PROXY":  "http://proxy.example.com:8080",
				"HTTPS_PROXY": "https://proxy.example.com:8443",
				"NO_PROXY":    ".cluster.local,.svc",
			},
		},
		{
			name:          "only http proxy set",
			httpProxy:     "http://proxy.example.com:8080",
			httpsProxy:    "",
			noProxy:       "",
			expectedCount: 1,
			expectedEnvMap: map[string]string{
				"HTTP_PROXY": "http://proxy.example.com:8080",
			},
		},
		{
			name:           "no proxy vars set",
			httpProxy:      "",
			httpsProxy:     "",
			noProxy:        "",
			expectedCount:  0,
			expectedEnvMap: map[string]string{},
		},
		{
			name:          "only https and no proxy set",
			httpProxy:     "",
			httpsProxy:    "https://proxy.example.com:8443",
			noProxy:       "localhost,127.0.0.1",
			expectedCount: 2,
			expectedEnvMap: map[string]string{
				"HTTPS_PROXY": "https://proxy.example.com:8443",
				"NO_PROXY":    "localhost,127.0.0.1",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set environment variables
			os.Setenv(HTTPProxyEnvVar, tt.httpProxy)
			os.Setenv(HTTPSProxyEnvVar, tt.httpsProxy)
			os.Setenv(NoProxyEnvVar, tt.noProxy)
			defer func() {
				os.Unsetenv(HTTPProxyEnvVar)
				os.Unsetenv(HTTPSProxyEnvVar)
				os.Unsetenv(NoProxyEnvVar)
			}()

			result := GetProxyEnvVars()

			if len(result) != tt.expectedCount {
				t.Errorf("expected %d env vars, got %d", tt.expectedCount, len(result))
			}

			// Verify each expected env var
			resultMap := make(map[string]string)
			for _, env := range result {
				resultMap[env.Name] = env.Value
			}

			for name, expectedValue := range tt.expectedEnvMap {
				if actualValue, exists := resultMap[name]; !exists {
					t.Errorf("expected env var %s not found", name)
				} else if actualValue != expectedValue {
					t.Errorf("env var %s: expected %s, got %s", name, expectedValue, actualValue)
				}
			}
		})
	}
}

func TestIsProxyEnabled(t *testing.T) {
	tests := []struct {
		name       string
		httpProxy  string
		httpsProxy string
		noProxy    string
		expected   bool
	}{
		{
			name:       "all proxy vars set",
			httpProxy:  "http://proxy.example.com:8080",
			httpsProxy: "https://proxy.example.com:8443",
			noProxy:    ".cluster.local",
			expected:   true,
		},
		{
			name:       "only http proxy set",
			httpProxy:  "http://proxy.example.com:8080",
			httpsProxy: "",
			noProxy:    "",
			expected:   true,
		},
		{
			name:       "only https proxy set",
			httpProxy:  "",
			httpsProxy: "https://proxy.example.com:8443",
			noProxy:    "",
			expected:   true,
		},
		{
			name:       "only no proxy set",
			httpProxy:  "",
			httpsProxy: "",
			noProxy:    "localhost",
			expected:   true,
		},
		{
			name:       "no proxy vars set",
			httpProxy:  "",
			httpsProxy: "",
			noProxy:    "",
			expected:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			os.Setenv(HTTPProxyEnvVar, tt.httpProxy)
			os.Setenv(HTTPSProxyEnvVar, tt.httpsProxy)
			os.Setenv(NoProxyEnvVar, tt.noProxy)
			defer func() {
				os.Unsetenv(HTTPProxyEnvVar)
				os.Unsetenv(HTTPSProxyEnvVar)
				os.Unsetenv(NoProxyEnvVar)
			}()

			result := IsProxyEnabled()
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestInjectProxyEnvVars(t *testing.T) {
	tests := []struct {
		name               string
		httpProxy          string
		httpsProxy         string
		noProxy            string
		existingEnvVars    []corev1.EnvVar
		expectedEnvCount   int
		expectedContains   map[string]string
		expectedNotChanged []string
	}{
		{
			name:             "inject into empty container",
			httpProxy:        "http://proxy.example.com:8080",
			httpsProxy:       "https://proxy.example.com:8443",
			noProxy:          ".cluster.local",
			existingEnvVars:  []corev1.EnvVar{},
			expectedEnvCount: 3,
			expectedContains: map[string]string{
				"HTTP_PROXY":  "http://proxy.example.com:8080",
				"HTTPS_PROXY": "https://proxy.example.com:8443",
				"NO_PROXY":    ".cluster.local",
			},
		},
		{
			name:       "don't override existing proxy vars",
			httpProxy:  "http://proxy.example.com:8080",
			httpsProxy: "https://proxy.example.com:8443",
			noProxy:    ".cluster.local",
			existingEnvVars: []corev1.EnvVar{
				{Name: "HTTP_PROXY", Value: "http://existing.proxy.com:3128"},
				{Name: "APP_NAME", Value: "test"},
			},
			expectedEnvCount: 4, // existing 2 + HTTPS_PROXY + NO_PROXY
			expectedContains: map[string]string{
				"HTTP_PROXY":  "http://existing.proxy.com:3128", // unchanged
				"HTTPS_PROXY": "https://proxy.example.com:8443",
				"NO_PROXY":    ".cluster.local",
				"APP_NAME":    "test",
			},
			expectedNotChanged: []string{"HTTP_PROXY", "APP_NAME"},
		},
		{
			name:             "no proxy vars to inject",
			httpProxy:        "",
			httpsProxy:       "",
			noProxy:          "",
			existingEnvVars:  []corev1.EnvVar{{Name: "APP_NAME", Value: "test"}},
			expectedEnvCount: 1,
			expectedContains: map[string]string{
				"APP_NAME": "test",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			os.Setenv(HTTPProxyEnvVar, tt.httpProxy)
			os.Setenv(HTTPSProxyEnvVar, tt.httpsProxy)
			os.Setenv(NoProxyEnvVar, tt.noProxy)
			defer func() {
				os.Unsetenv(HTTPProxyEnvVar)
				os.Unsetenv(HTTPSProxyEnvVar)
				os.Unsetenv(NoProxyEnvVar)
			}()

			container := &corev1.Container{
				Name: "test-container",
				Env:  tt.existingEnvVars,
			}

			InjectProxyEnvVars(container)

			if len(container.Env) != tt.expectedEnvCount {
				t.Errorf("expected %d env vars, got %d", tt.expectedEnvCount, len(container.Env))
			}

			// Build map of actual env vars
			actualEnvMap := make(map[string]string)
			for _, env := range container.Env {
				actualEnvMap[env.Name] = env.Value
			}

			// Verify all expected env vars are present with correct values
			for name, expectedValue := range tt.expectedContains {
				if actualValue, exists := actualEnvMap[name]; !exists {
					t.Errorf("expected env var %s not found", name)
				} else if actualValue != expectedValue {
					t.Errorf("env var %s: expected %s, got %s", name, expectedValue, actualValue)
				}
			}
		})
	}
}

func TestAddTrustedCABundleToContainer(t *testing.T) {
	tests := []struct {
		name                 string
		existingVolumeMounts []corev1.VolumeMount
		expectedMountCount   int
		shouldAddMount       bool
	}{
		{
			name:                 "add to container with no mounts",
			existingVolumeMounts: []corev1.VolumeMount{},
			expectedMountCount:   1,
			shouldAddMount:       true,
		},
		{
			name: "add to container with existing mounts",
			existingVolumeMounts: []corev1.VolumeMount{
				{Name: "config", MountPath: "/config"},
			},
			expectedMountCount: 2,
			shouldAddMount:     true,
		},
		{
			name: "don't add duplicate mount",
			existingVolumeMounts: []corev1.VolumeMount{
				{Name: "trusted-ca-bundle", MountPath: "/etc/pki/tls/certs", ReadOnly: true},
			},
			expectedMountCount: 1,
			shouldAddMount:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			container := &corev1.Container{
				Name:         "test-container",
				VolumeMounts: tt.existingVolumeMounts,
			}

			AddTrustedCABundleToContainer(container)

			if len(container.VolumeMounts) != tt.expectedMountCount {
				t.Errorf("expected %d volume mounts, got %d", tt.expectedMountCount, len(container.VolumeMounts))
			}

			// Verify the trusted-ca-bundle mount is present
			found := false
			for _, vm := range container.VolumeMounts {
				if vm.Name == "trusted-ca-bundle" {
					found = true
					if vm.MountPath != TrustedCABundlePath {
						t.Errorf("expected mount path %s, got %s", TrustedCABundlePath, vm.MountPath)
					}
					if !vm.ReadOnly {
						t.Error("expected volume mount to be read-only")
					}
				}
			}

			if tt.shouldAddMount && !found {
				t.Error("expected trusted-ca-bundle mount to be added, but it wasn't")
			}
		})
	}
}

func TestGetTrustedCABundleVolume(t *testing.T) {
	volume := GetTrustedCABundleVolume()

	if volume.Name != "trusted-ca-bundle" {
		t.Errorf("expected volume name 'trusted-ca-bundle', got %s", volume.Name)
	}

	if volume.VolumeSource.ConfigMap == nil {
		t.Fatal("expected ConfigMap volume source, got nil")
	}

	if volume.VolumeSource.ConfigMap.Name != OperandTrustedCABundleConfigMapName {
		t.Errorf("expected ConfigMap name %s, got %s",
			OperandTrustedCABundleConfigMapName,
			volume.VolumeSource.ConfigMap.Name)
	}

	if volume.VolumeSource.ConfigMap.Optional == nil || !*volume.VolumeSource.ConfigMap.Optional {
		t.Error("expected ConfigMap to be optional")
	}

	if len(volume.VolumeSource.ConfigMap.Items) != 1 {
		t.Fatalf("expected 1 item in ConfigMap projection, got %d", len(volume.VolumeSource.ConfigMap.Items))
	}

	item := volume.VolumeSource.ConfigMap.Items[0]
	if item.Key != TrustedCABundleKey {
		t.Errorf("expected key %s, got %s", TrustedCABundleKey, item.Key)
	}
	if item.Path != "ca-bundle.crt" {
		t.Errorf("expected path 'ca-bundle.crt', got %s", item.Path)
	}
}

func TestGetTrustedCABundleVolumeMount(t *testing.T) {
	volumeMount := GetTrustedCABundleVolumeMount()

	if volumeMount.Name != "trusted-ca-bundle" {
		t.Errorf("expected volume mount name 'trusted-ca-bundle', got %s", volumeMount.Name)
	}

	if volumeMount.MountPath != TrustedCABundlePath {
		t.Errorf("expected mount path %s, got %s", TrustedCABundlePath, volumeMount.MountPath)
	}

	if !volumeMount.ReadOnly {
		t.Error("expected volume mount to be read-only")
	}
}

func TestAddProxyConfigToPod(t *testing.T) {
	tests := []struct {
		name                 string
		httpProxy            string
		httpsProxy           string
		noProxy              string
		podSpec              *corev1.PodSpec
		expectedContainerEnv int
		expectedInitEnv      int
		expectedVolumes      int
		shouldModify         bool
	}{
		{
			name:       "add proxy config to pod with one container",
			httpProxy:  "http://proxy.example.com:8080",
			httpsProxy: "https://proxy.example.com:8443",
			noProxy:    ".cluster.local",
			podSpec: &corev1.PodSpec{
				Containers: []corev1.Container{
					{Name: "main", Env: []corev1.EnvVar{}},
				},
				Volumes: []corev1.Volume{},
			},
			expectedContainerEnv: 3, // HTTP_PROXY, HTTPS_PROXY, NO_PROXY
			expectedVolumes:      1, // trusted-ca-bundle
			shouldModify:         true,
		},
		{
			name:       "add proxy config to pod with containers and init containers",
			httpProxy:  "http://proxy.example.com:8080",
			httpsProxy: "",
			noProxy:    "",
			podSpec: &corev1.PodSpec{
				InitContainers: []corev1.Container{
					{Name: "init", Env: []corev1.EnvVar{}},
				},
				Containers: []corev1.Container{
					{Name: "main", Env: []corev1.EnvVar{}},
					{Name: "sidecar", Env: []corev1.EnvVar{}},
				},
				Volumes: []corev1.Volume{},
			},
			expectedContainerEnv: 1, // HTTP_PROXY only
			expectedInitEnv:      1, // HTTP_PROXY only
			expectedVolumes:      1, // trusted-ca-bundle
			shouldModify:         true,
		},
		{
			name:      "no proxy enabled - no changes",
			httpProxy: "",
			podSpec: &corev1.PodSpec{
				Containers: []corev1.Container{
					{Name: "main", Env: []corev1.EnvVar{}},
				},
				Volumes: []corev1.Volume{},
			},
			expectedContainerEnv: 0,
			expectedVolumes:      0,
			shouldModify:         false,
		},
		{
			name:       "don't add duplicate volume",
			httpProxy:  "http://proxy.example.com:8080",
			httpsProxy: "https://proxy.example.com:8443",
			noProxy:    ".cluster.local",
			podSpec: &corev1.PodSpec{
				Containers: []corev1.Container{
					{Name: "main", Env: []corev1.EnvVar{}},
				},
				Volumes: []corev1.Volume{
					{
						Name: "trusted-ca-bundle",
						VolumeSource: corev1.VolumeSource{
							ConfigMap: &corev1.ConfigMapVolumeSource{
								LocalObjectReference: corev1.LocalObjectReference{
									Name: OperandTrustedCABundleConfigMapName,
								},
							},
						},
					},
					{Name: "config", VolumeSource: corev1.VolumeSource{}},
				},
			},
			expectedContainerEnv: 3,
			expectedVolumes:      2, // Don't add duplicate
			shouldModify:         true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			os.Setenv(HTTPProxyEnvVar, tt.httpProxy)
			os.Setenv(HTTPSProxyEnvVar, tt.httpsProxy)
			os.Setenv(NoProxyEnvVar, tt.noProxy)
			defer func() {
				os.Unsetenv(HTTPProxyEnvVar)
				os.Unsetenv(HTTPSProxyEnvVar)
				os.Unsetenv(NoProxyEnvVar)
			}()

			AddProxyConfigToPod(tt.podSpec)

			// Check containers
			if len(tt.podSpec.Containers) > 0 {
				for _, container := range tt.podSpec.Containers {
					if len(container.Env) != tt.expectedContainerEnv {
						t.Errorf("container %s: expected %d env vars, got %d",
							container.Name, tt.expectedContainerEnv, len(container.Env))
					}
					if tt.shouldModify && tt.expectedContainerEnv > 0 {
						// Verify volume mount was added
						found := false
						for _, vm := range container.VolumeMounts {
							if vm.Name == "trusted-ca-bundle" {
								found = true
								break
							}
						}
						if !found {
							t.Errorf("container %s: expected trusted-ca-bundle volume mount", container.Name)
						}
					}
				}
			}

			// Check init containers
			if len(tt.podSpec.InitContainers) > 0 {
				for _, container := range tt.podSpec.InitContainers {
					if len(container.Env) != tt.expectedInitEnv {
						t.Errorf("init container %s: expected %d env vars, got %d",
							container.Name, tt.expectedInitEnv, len(container.Env))
					}
					if tt.shouldModify && tt.expectedInitEnv > 0 {
						// Verify volume mount was added
						found := false
						for _, vm := range container.VolumeMounts {
							if vm.Name == "trusted-ca-bundle" {
								found = true
								break
							}
						}
						if !found {
							t.Errorf("init container %s: expected trusted-ca-bundle volume mount", container.Name)
						}
					}
				}
			}

			// Check volumes
			if len(tt.podSpec.Volumes) != tt.expectedVolumes {
				t.Errorf("expected %d volumes, got %d", tt.expectedVolumes, len(tt.podSpec.Volumes))
			}

			if tt.shouldModify && tt.expectedVolumes > 0 {
				// Verify trusted-ca-bundle volume was added
				found := false
				for _, vol := range tt.podSpec.Volumes {
					if vol.Name == "trusted-ca-bundle" {
						found = true
						if vol.VolumeSource.ConfigMap == nil {
							t.Error("expected ConfigMap volume source")
						} else if vol.VolumeSource.ConfigMap.Name != OperandTrustedCABundleConfigMapName {
							t.Errorf("expected ConfigMap name %s, got %s",
								OperandTrustedCABundleConfigMapName,
								vol.VolumeSource.ConfigMap.Name)
						}
						break
					}
				}
				if !found {
					t.Error("expected trusted-ca-bundle volume to be added")
				}
			}
		})
	}
}

func TestAddProxyConfigToPodIdempotency(t *testing.T) {
	os.Setenv(HTTPProxyEnvVar, "http://proxy.example.com:8080")
	defer os.Unsetenv(HTTPProxyEnvVar)

	podSpec := &corev1.PodSpec{
		Containers: []corev1.Container{
			{Name: "main", Env: []corev1.EnvVar{}},
		},
		Volumes: []corev1.Volume{},
	}

	// Call multiple times
	AddProxyConfigToPod(podSpec)
	AddProxyConfigToPod(podSpec)
	AddProxyConfigToPod(podSpec)

	// Should still have exactly 1 env var, 1 volume mount, 1 volume
	if len(podSpec.Containers[0].Env) != 1 {
		t.Errorf("expected 1 env var after multiple calls, got %d", len(podSpec.Containers[0].Env))
	}

	if len(podSpec.Containers[0].VolumeMounts) != 1 {
		t.Errorf("expected 1 volume mount after multiple calls, got %d", len(podSpec.Containers[0].VolumeMounts))
	}

	if len(podSpec.Volumes) != 1 {
		t.Errorf("expected 1 volume after multiple calls, got %d", len(podSpec.Volumes))
	}
}
