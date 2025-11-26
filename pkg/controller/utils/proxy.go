package utils

import (
	"os"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/ptr"
)

const (
	// Proxy environment variable names
	HTTPProxyEnvVar     = "HTTP_PROXY"
	HTTPSProxyEnvVar    = "HTTPS_PROXY"
	NoProxyEnvVar       = "NO_PROXY"
	TrustedCABundlePath = "/etc/pki/tls/certs"
	TrustedCABundleKey  = "ca-bundle.crt"

	// ConfigMap names for trusted CA bundle
	// Operator ConfigMap: Created by OLM, used by operator pod
	OperatorTrustedCABundleConfigMapName = "zero-trust-workload-identity-manager-trusted-ca-bundle"
	// Operand ConfigMap: Created by operator, used by all operand pods
	OperandTrustedCABundleConfigMapName = "ztwim-operand-trusted-ca-bundle"

	// Label for OpenShift CNO to inject trusted CA bundle
	InjectCABundleLabel = "config.openshift.io/inject-trusted-cabundle"
)

// GetProxyEnvVars retrieves proxy environment variables from the operator's environment
// These are injected by OLM when a cluster-wide proxy is configured
func GetProxyEnvVars() []corev1.EnvVar {
	var envVars []corev1.EnvVar

	// Get HTTP_PROXY
	if httpProxy := os.Getenv(HTTPProxyEnvVar); httpProxy != "" {
		envVars = append(envVars, corev1.EnvVar{
			Name:  HTTPProxyEnvVar,
			Value: httpProxy,
		})
	}

	// Get HTTPS_PROXY
	if httpsProxy := os.Getenv(HTTPSProxyEnvVar); httpsProxy != "" {
		envVars = append(envVars, corev1.EnvVar{
			Name:  HTTPSProxyEnvVar,
			Value: httpsProxy,
		})
	}

	// Get NO_PROXY
	if noProxy := os.Getenv(NoProxyEnvVar); noProxy != "" {
		envVars = append(envVars, corev1.EnvVar{
			Name:  NoProxyEnvVar,
			Value: noProxy,
		})
	}

	return envVars
}

// IsProxyEnabled checks if any proxy environment variables are set
func IsProxyEnabled() bool {
	return os.Getenv(HTTPProxyEnvVar) != "" ||
		os.Getenv(HTTPSProxyEnvVar) != "" ||
		os.Getenv(NoProxyEnvVar) != ""
}

// InjectProxyEnvVars adds proxy environment variables to a container's Env list
// if they are not already present
func InjectProxyEnvVars(container *corev1.Container) {
	proxyEnvVars := GetProxyEnvVars()
	if len(proxyEnvVars) == 0 {
		return
	}

	// Create a map of existing env var names for quick lookup
	existingEnvVars := make(map[string]bool)
	for _, env := range container.Env {
		existingEnvVars[env.Name] = true
	}

	// Add proxy env vars that don't already exist
	for _, proxyEnv := range proxyEnvVars {
		if !existingEnvVars[proxyEnv.Name] {
			container.Env = append(container.Env, proxyEnv)
		}
	}
}

// GetTrustedCABundleVolume returns a Volume for mounting the trusted CA bundle
// Uses the operand ConfigMap created and managed by the operator
func GetTrustedCABundleVolume() corev1.Volume {
	return corev1.Volume{
		Name: "trusted-ca-bundle",
		VolumeSource: corev1.VolumeSource{
			ConfigMap: &corev1.ConfigMapVolumeSource{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: OperandTrustedCABundleConfigMapName,
				},
				Items: []corev1.KeyToPath{
					{
						Key:  TrustedCABundleKey,
						Path: "ca-bundle.crt",
					},
				},
				Optional: ptr.To(true), // ConfigMap is optional if proxy not configured
			},
		},
	}
}

// GetTrustedCABundleVolumeMount returns a VolumeMount for the trusted CA bundle
func GetTrustedCABundleVolumeMount() corev1.VolumeMount {
	return corev1.VolumeMount{
		Name:      "trusted-ca-bundle",
		MountPath: TrustedCABundlePath,
		ReadOnly:  true,
	}
}

// AddTrustedCABundleToContainer adds the trusted CA bundle volume mount to a container
func AddTrustedCABundleToContainer(container *corev1.Container) {
	// Check if volume mount already exists
	for _, vm := range container.VolumeMounts {
		if vm.Name == "trusted-ca-bundle" {
			return
		}
	}

	container.VolumeMounts = append(container.VolumeMounts, GetTrustedCABundleVolumeMount())
}

// AddProxyConfigToPod adds proxy environment variables and trusted CA bundle to all containers in a pod spec
// This should be called after all containers are added to the pod spec
func AddProxyConfigToPod(podSpec *corev1.PodSpec) {
	if !IsProxyEnabled() {
		return
	}

	// Add proxy env vars to all containers
	for i := range podSpec.Containers {
		InjectProxyEnvVars(&podSpec.Containers[i])
		AddTrustedCABundleToContainer(&podSpec.Containers[i])
	}

	// Add proxy env vars to all init containers
	for i := range podSpec.InitContainers {
		InjectProxyEnvVars(&podSpec.InitContainers[i])
		AddTrustedCABundleToContainer(&podSpec.InitContainers[i])
	}

	// Add trusted CA bundle volume
	// Check if volume already exists
	volumeExists := false
	for _, vol := range podSpec.Volumes {
		if vol.Name == "trusted-ca-bundle" {
			volumeExists = true
			break
		}
	}

	if !volumeExists {
		podSpec.Volumes = append(podSpec.Volumes, GetTrustedCABundleVolume())
	}
}
