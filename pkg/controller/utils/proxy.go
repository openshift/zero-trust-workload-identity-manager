package utils

import (
	"os"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/ptr"
)

const (
	// Proxy environment variable names
	HTTPProxyEnvVar  = "HTTP_PROXY"
	HTTPSProxyEnvVar = "HTTPS_PROXY"
	NoProxyEnvVar    = "NO_PROXY"

	// TrustedCABundleConfigMapEnvVar Environment variable for user-provided trusted CA bundle ConfigMap name
	// User sets this in the Subscription object to specify their ConfigMap
	TrustedCABundleConfigMapEnvVar = "TRUSTED_CA_BUNDLE_CONFIGMAP"

	// TrustedCABundlePath has Trusted CA bundle configuration
	// Mount path follows OpenShift conventions for injected CA bundles
	TrustedCABundlePath = "/etc/pki/ca-trust/extracted/pem"
	TrustedCABundleFile = "tls-ca-bundle.pem"
	TrustedCABundleKey  = "ca-bundle.crt"

	// Volume name for trusted CA bundle
	trustedCABundleVolumeName = "trusted-ca-bundle"
)

// GetProxyEnvVars retrieves proxy environment variables from the operator's environment
// These are injected by OLM when a cluster-wide proxy is configured, or can be
// overridden by the user via the Subscription object
func GetProxyEnvVars() []corev1.EnvVar {
	return GetProxyEnvVarsWithNoProxyAdditions(nil)
}

// GetProxyEnvVarsWithNoProxyAdditions retrieves proxy environment variables and appends
// additional entries to NO_PROXY. This is useful for ensuring internal services bypass the proxy.
// The additionalNoProxy entries are appended to the existing NO_PROXY value.
func GetProxyEnvVarsWithNoProxyAdditions(additionalNoProxy []string) []corev1.EnvVar {
	var envVars []corev1.EnvVar

	if httpProxy := os.Getenv(HTTPProxyEnvVar); httpProxy != "" {
		envVars = append(envVars, corev1.EnvVar{
			Name:  HTTPProxyEnvVar,
			Value: httpProxy,
		})
	}

	if httpsProxy := os.Getenv(HTTPSProxyEnvVar); httpsProxy != "" {
		envVars = append(envVars, corev1.EnvVar{
			Name:  HTTPSProxyEnvVar,
			Value: httpsProxy,
		})
	}

	noProxy := os.Getenv(NoProxyEnvVar)
	// Append additional NO_PROXY entries if provided
	if len(additionalNoProxy) > 0 {
		for _, entry := range additionalNoProxy {
			if entry != "" {
				if noProxy != "" {
					noProxy = noProxy + "," + entry
				} else {
					noProxy = entry
				}
			}
		}
	}

	if noProxy != "" {
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

// ProxyValidationResult contains the result of proxy configuration validation
type ProxyValidationResult struct {
	Valid   bool
	Reason  string
	Message string
}

// ValidateProxyConfiguration validates proxy configuration:
// 1. If proxy is not enabled, returns valid (no validation needed)
// 2. If proxy is enabled, CA bundle ConfigMap name must be configured via TRUSTED_CA_BUNDLE_CONFIGMAP env var
// Note: We don't validate if the ConfigMap actually exists - the volume mount uses optional:true
// so pods will start even if the ConfigMap doesn't exist yet.
func ValidateProxyConfiguration() *ProxyValidationResult {
	if !IsProxyEnabled() {
		return &ProxyValidationResult{Valid: true}
	}

	configMapName := GetTrustedCABundleConfigMapName()
	if configMapName == "" {
		return &ProxyValidationResult{
			Valid:   false,
			Reason:  "ProxyConfigurationInvalid",
			Message: "Proxy is enabled (HTTP_PROXY/HTTPS_PROXY set) but trusted CA bundle ConfigMap is not configured. set TRUSTED_CA_BUNDLE_CONFIGMAP environment variable in the Subscription.",
		}
	}

	return &ProxyValidationResult{Valid: true}
}

// GetTrustedCABundleConfigMapName returns the user-configured ConfigMap name
// for the trusted CA bundle. Returns empty string if not configured.
// User sets this via TRUSTED_CA_BUNDLE_CONFIGMAP env var in the Subscription.
func GetTrustedCABundleConfigMapName() string {
	return os.Getenv(TrustedCABundleConfigMapEnvVar)
}

// IsTrustedCABundleConfigured checks if user has specified a CA bundle ConfigMap
func IsTrustedCABundleConfigured() bool {
	return GetTrustedCABundleConfigMapName() != ""
}

// InjectProxyEnvVars adds proxy environment variables to a container's Env list
// if they are not already present
func InjectProxyEnvVars(container *corev1.Container) {
	InjectProxyEnvVarsWithNoProxyAdditions(container, nil)
}

// InjectProxyEnvVarsWithNoProxyAdditions adds proxy environment variables to a container's Env list
// with additional NO_PROXY entries appended. This ensures internal services bypass the proxy.
func InjectProxyEnvVarsWithNoProxyAdditions(container *corev1.Container, additionalNoProxy []string) {
	proxyEnvVars := GetProxyEnvVarsWithNoProxyAdditions(additionalNoProxy)
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

// GetInternalNoProxyEntries returns NO_PROXY entries for internal cluster services.
// These should be added to NO_PROXY for components that need proxy for external access
// but must bypass proxy for internal cluster communication.
func GetInternalNoProxyEntries() []string {
	namespace := GetOperatorNamespace()
	return []string{
		// Internal service names used by SPIRE components
		"spire-server." + namespace,
		"spire-server." + namespace + ".svc",
		"spire-server." + namespace + ".svc.cluster.local",
		// Kubernetes API server
		"kubernetes.default",
		"kubernetes.default.svc",
		"kubernetes.default.svc.cluster.local",
	}
}

// GetTrustedCABundleVolume returns a Volume for mounting the user-specified trusted CA bundle ConfigMap.
// Returns an empty Volume if no ConfigMap is configured.
func GetTrustedCABundleVolume() corev1.Volume {
	configMapName := GetTrustedCABundleConfigMapName()
	if configMapName == "" {
		return corev1.Volume{}
	}

	return corev1.Volume{
		Name: trustedCABundleVolumeName,
		VolumeSource: corev1.VolumeSource{
			ConfigMap: &corev1.ConfigMapVolumeSource{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: configMapName,
				},
				Items: []corev1.KeyToPath{
					{
						Key:  TrustedCABundleKey,
						Path: TrustedCABundleFile,
					},
				},
				// Optional: true allows the pod to start even if the ConfigMap doesn't exist yet
				Optional: ptr.To(true),
			},
		},
	}
}

// GetTrustedCABundleVolumeMount returns a VolumeMount for the trusted CA bundle
// Mounts to the standard OpenShift CA trust directory. The ConfigMap volume uses
// items projection to only include tls-ca-bundle.pem, so no SubPath is needed.
func GetTrustedCABundleVolumeMount() corev1.VolumeMount {
	return corev1.VolumeMount{
		Name:      trustedCABundleVolumeName,
		MountPath: TrustedCABundlePath,
		ReadOnly:  true,
	}
}

// AddTrustedCABundleToContainer adds the trusted CA bundle volume mount to a container
// if a ConfigMap is configured and the mount doesn't already exist
func AddTrustedCABundleToContainer(container *corev1.Container) {
	if !IsTrustedCABundleConfigured() {
		return
	}

	// Check if volume mount already exists
	for _, vm := range container.VolumeMounts {
		if vm.Name == trustedCABundleVolumeName {
			return
		}
	}

	container.VolumeMounts = append(container.VolumeMounts, GetTrustedCABundleVolumeMount())
}

// AddProxyConfigToPod adds proxy environment variables and trusted CA bundle to all containers in a pod spec.
// This should be called after all containers are added to the pod spec.
//
// Proxy env vars are added if any proxy environment variables are set (HTTP_PROXY, HTTPS_PROXY, NO_PROXY).
// Trusted CA bundle is mounted if the user has specified a ConfigMap name via TRUSTED_CA_BUNDLE_CONFIGMAP.
func AddProxyConfigToPod(podSpec *corev1.PodSpec) {
	AddProxyConfigToPodWithNoProxyAdditions(podSpec, nil)
}

// AddProxyConfigToPodWithInternalNoProxy adds proxy configuration to a pod spec and ensures
// internal cluster services are added to NO_PROXY. Use this for components that need proxy
// for external access but must bypass proxy for internal cluster communication (e.g., spire-agent).
func AddProxyConfigToPodWithInternalNoProxy(podSpec *corev1.PodSpec) {
	AddProxyConfigToPodWithNoProxyAdditions(podSpec, GetInternalNoProxyEntries())
}

// AddProxyConfigToPodWithNoProxyAdditions adds proxy configuration with additional NO_PROXY entries.
func AddProxyConfigToPodWithNoProxyAdditions(podSpec *corev1.PodSpec, additionalNoProxy []string) {
	proxyEnabled := IsProxyEnabled()
	caConfigured := IsTrustedCABundleConfigured()

	if !proxyEnabled && !caConfigured {
		return // Nothing to do
	}

	// Add proxy env vars and CA bundle mounts to all containers
	for i := range podSpec.Containers {
		if proxyEnabled {
			InjectProxyEnvVarsWithNoProxyAdditions(&podSpec.Containers[i], additionalNoProxy)
		}
		if caConfigured {
			AddTrustedCABundleToContainer(&podSpec.Containers[i])
		}
	}

	// Add proxy env vars and CA bundle mounts to all init containers
	for i := range podSpec.InitContainers {
		if proxyEnabled {
			InjectProxyEnvVarsWithNoProxyAdditions(&podSpec.InitContainers[i], additionalNoProxy)
		}
		if caConfigured {
			AddTrustedCABundleToContainer(&podSpec.InitContainers[i])
		}
	}

	// Add trusted CA bundle volume if configured
	if caConfigured {
		// Check if volume already exists
		volumeExists := false
		for _, vol := range podSpec.Volumes {
			if vol.Name == trustedCABundleVolumeName {
				volumeExists = true
				break
			}
		}

		if !volumeExists {
			podSpec.Volumes = append(podSpec.Volumes, GetTrustedCABundleVolume())
		}
	}
}
