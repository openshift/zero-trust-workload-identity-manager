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
			expected:   false,
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

func TestGetTrustedCABundleConfigMapName(t *testing.T) {
	tests := []struct {
		name     string
		envValue string
		expected string
	}{
		{
			name:     "not configured",
			envValue: "",
			expected: "",
		},
		{
			name:     "configured with custom name",
			envValue: "my-trusted-ca-bundle",
			expected: "my-trusted-ca-bundle",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			os.Unsetenv(TrustedCABundleConfigMapEnvVar)
			if tt.envValue != "" {
				os.Setenv(TrustedCABundleConfigMapEnvVar, tt.envValue)
			}
			defer os.Unsetenv(TrustedCABundleConfigMapEnvVar)

			result := GetTrustedCABundleConfigMapName()
			if result != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, result)
			}
		})
	}
}

func TestIsTrustedCABundleConfigured(t *testing.T) {
	tests := []struct {
		name     string
		envValue string
		expected bool
	}{
		{
			name:     "not configured",
			envValue: "",
			expected: false,
		},
		{
			name:     "configured",
			envValue: "my-ca-bundle",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			os.Unsetenv(TrustedCABundleConfigMapEnvVar)
			if tt.envValue != "" {
				os.Setenv(TrustedCABundleConfigMapEnvVar, tt.envValue)
			}
			defer os.Unsetenv(TrustedCABundleConfigMapEnvVar)

			result := IsTrustedCABundleConfigured()
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestInjectProxyEnvVars(t *testing.T) {
	tests := []struct {
		name             string
		httpProxy        string
		httpsProxy       string
		noProxy          string
		existingEnvVars  []corev1.EnvVar
		expectedEnvCount int
		expectedContains map[string]string
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

func TestGetTrustedCABundleVolume(t *testing.T) {
	tests := []struct {
		name          string
		configMapName string
		expectEmpty   bool
	}{
		{
			name:          "no ConfigMap configured",
			configMapName: "",
			expectEmpty:   true,
		},
		{
			name:          "ConfigMap configured",
			configMapName: "my-ca-bundle",
			expectEmpty:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			os.Unsetenv(TrustedCABundleConfigMapEnvVar)
			if tt.configMapName != "" {
				os.Setenv(TrustedCABundleConfigMapEnvVar, tt.configMapName)
			}
			defer os.Unsetenv(TrustedCABundleConfigMapEnvVar)

			volume := GetTrustedCABundleVolume()

			if tt.expectEmpty {
				if volume.Name != "" {
					t.Errorf("expected empty volume, got %v", volume)
				}
			} else {
				if volume.Name != "trusted-ca-bundle" {
					t.Errorf("expected volume name 'trusted-ca-bundle', got %s", volume.Name)
				}
				if volume.VolumeSource.ConfigMap == nil {
					t.Fatal("expected ConfigMap volume source, got nil")
				}
				if volume.VolumeSource.ConfigMap.Name != tt.configMapName {
					t.Errorf("expected ConfigMap name %s, got %s",
						tt.configMapName,
						volume.VolumeSource.ConfigMap.Name)
				}
			}
		})
	}
}

func TestAddTrustedCABundleToContainer(t *testing.T) {
	tests := []struct {
		name                 string
		configMapName        string
		existingVolumeMounts []corev1.VolumeMount
		expectedMountCount   int
		shouldAddMount       bool
	}{
		{
			name:                 "no ConfigMap configured - no change",
			configMapName:        "",
			existingVolumeMounts: []corev1.VolumeMount{{Name: "config", MountPath: "/config"}},
			expectedMountCount:   1,
			shouldAddMount:       false,
		},
		{
			name:                 "add to container with no mounts",
			configMapName:        "my-ca-bundle",
			existingVolumeMounts: []corev1.VolumeMount{},
			expectedMountCount:   1,
			shouldAddMount:       true,
		},
		{
			name:          "add to container with existing mounts",
			configMapName: "my-ca-bundle",
			existingVolumeMounts: []corev1.VolumeMount{
				{Name: "config", MountPath: "/config"},
			},
			expectedMountCount: 2,
			shouldAddMount:     true,
		},
		{
			name:          "don't add duplicate mount",
			configMapName: "my-ca-bundle",
			existingVolumeMounts: []corev1.VolumeMount{
				{Name: "trusted-ca-bundle", MountPath: TrustedCABundlePath, ReadOnly: true},
			},
			expectedMountCount: 1,
			shouldAddMount:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			os.Unsetenv(TrustedCABundleConfigMapEnvVar)
			if tt.configMapName != "" {
				os.Setenv(TrustedCABundleConfigMapEnvVar, tt.configMapName)
			}
			defer os.Unsetenv(TrustedCABundleConfigMapEnvVar)

			container := &corev1.Container{
				Name:         "test-container",
				VolumeMounts: tt.existingVolumeMounts,
			}

			AddTrustedCABundleToContainer(container)

			if len(container.VolumeMounts) != tt.expectedMountCount {
				t.Errorf("expected %d volume mounts, got %d", tt.expectedMountCount, len(container.VolumeMounts))
			}

			if tt.shouldAddMount {
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
				if !found {
					t.Error("expected trusted-ca-bundle mount to be added, but it wasn't")
				}
			}
		})
	}
}

func TestAddProxyConfigToPod(t *testing.T) {
	tests := []struct {
		name                 string
		httpProxy            string
		configMapName        string
		podSpec              *corev1.PodSpec
		expectedContainerEnv int
		expectedInitEnv      int
		expectedVolumes      int
		shouldModify         bool
	}{
		{
			name:          "no proxy and no CA bundle - no changes",
			httpProxy:     "",
			configMapName: "",
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
			name:          "proxy only - inject env vars, no volume",
			httpProxy:     "http://proxy.example.com:8080",
			configMapName: "",
			podSpec: &corev1.PodSpec{
				Containers: []corev1.Container{
					{Name: "main", Env: []corev1.EnvVar{}},
				},
				InitContainers: []corev1.Container{
					{Name: "init", Env: []corev1.EnvVar{}},
				},
				Volumes: []corev1.Volume{},
			},
			expectedContainerEnv: 1,
			expectedInitEnv:      1,
			expectedVolumes:      0, // No volume - no CA bundle configured
			shouldModify:         true,
		},
		{
			name:          "CA bundle only - inject volume and mounts, no env vars",
			httpProxy:     "",
			configMapName: "my-ca-bundle",
			podSpec: &corev1.PodSpec{
				Containers: []corev1.Container{
					{Name: "main", Env: []corev1.EnvVar{}},
				},
				Volumes: []corev1.Volume{},
			},
			expectedContainerEnv: 0, // No env vars - no proxy
			expectedVolumes:      1, // Volume added
			shouldModify:         true,
		},
		{
			name:          "both proxy and CA bundle",
			httpProxy:     "http://proxy.example.com:8080",
			configMapName: "my-ca-bundle",
			podSpec: &corev1.PodSpec{
				Containers: []corev1.Container{
					{Name: "main", Env: []corev1.EnvVar{}},
				},
				Volumes: []corev1.Volume{},
			},
			expectedContainerEnv: 1,
			expectedVolumes:      1,
			shouldModify:         true,
		},
		{
			name:          "don't add duplicate volume",
			httpProxy:     "http://proxy.example.com:8080",
			configMapName: "my-ca-bundle",
			podSpec: &corev1.PodSpec{
				Containers: []corev1.Container{
					{Name: "main", Env: []corev1.EnvVar{}},
				},
				Volumes: []corev1.Volume{
					{Name: "trusted-ca-bundle", VolumeSource: corev1.VolumeSource{}},
					{Name: "config", VolumeSource: corev1.VolumeSource{}},
				},
			},
			expectedContainerEnv: 1,
			expectedVolumes:      2, // Don't add duplicate
			shouldModify:         true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			os.Unsetenv(HTTPProxyEnvVar)
			os.Unsetenv(TrustedCABundleConfigMapEnvVar)
			if tt.httpProxy != "" {
				os.Setenv(HTTPProxyEnvVar, tt.httpProxy)
			}
			if tt.configMapName != "" {
				os.Setenv(TrustedCABundleConfigMapEnvVar, tt.configMapName)
			}
			defer func() {
				os.Unsetenv(HTTPProxyEnvVar)
				os.Unsetenv(TrustedCABundleConfigMapEnvVar)
			}()

			AddProxyConfigToPod(tt.podSpec)

			// Check containers
			if len(tt.podSpec.Containers) > 0 {
				for _, container := range tt.podSpec.Containers {
					if len(container.Env) != tt.expectedContainerEnv {
						t.Errorf("container %s: expected %d env vars, got %d",
							container.Name, tt.expectedContainerEnv, len(container.Env))
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
				}
			}

			// Check volumes
			if len(tt.podSpec.Volumes) != tt.expectedVolumes {
				t.Errorf("expected %d volumes, got %d", tt.expectedVolumes, len(tt.podSpec.Volumes))
			}
		})
	}
}

func TestAddProxyConfigToPodIdempotency(t *testing.T) {
	os.Setenv(HTTPProxyEnvVar, "http://proxy.example.com:8080")
	os.Setenv(TrustedCABundleConfigMapEnvVar, "my-ca-bundle")
	defer func() {
		os.Unsetenv(HTTPProxyEnvVar)
		os.Unsetenv(TrustedCABundleConfigMapEnvVar)
	}()

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
