package spire_agent

import (
	"testing"

	"github.com/openshift/zero-trust-workload-identity-manager/api/v1alpha1"
	"github.com/openshift/zero-trust-workload-identity-manager/pkg/controller/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
)

func TestGetHostCertMountPath(t *testing.T) {
	tests := []struct {
		name              string
		workloadAttestors *v1alpha1.WorkloadAttestors
		expected          string
	}{
		{
			name:              "nil workloadAttestors",
			workloadAttestors: nil,
			expected:          "",
		},
		{
			name: "nil verification",
			workloadAttestors: &v1alpha1.WorkloadAttestors{
				K8sEnabled:                    "true",
				WorkloadAttestorsVerification: nil,
			},
			expected: "",
		},
		{
			name: "skip type - no mount needed",
			workloadAttestors: &v1alpha1.WorkloadAttestors{
				K8sEnabled: "true",
				WorkloadAttestorsVerification: &v1alpha1.WorkloadAttestorsVerification{
					Type: utils.WorkloadAttestorVerificationTypeSkip,
				},
			},
			expected: "",
		},
		{
			name: "hostCert type with both paths - mount needed",
			workloadAttestors: &v1alpha1.WorkloadAttestors{
				K8sEnabled: "true",
				WorkloadAttestorsVerification: &v1alpha1.WorkloadAttestorsVerification{
					Type:             utils.WorkloadAttestorVerificationTypeHostCert,
					HostCertBasePath: "/etc/kubernetes",
					HostCertFileName: "kubelet-ca.crt",
				},
			},
			expected: "/etc/kubernetes",
		},
		{
			name: "hostCert type with only basePath - uses basePath (CEL would block this)",
			workloadAttestors: &v1alpha1.WorkloadAttestors{
				K8sEnabled: "true",
				WorkloadAttestorsVerification: &v1alpha1.WorkloadAttestorsVerification{
					Type:             utils.WorkloadAttestorVerificationTypeHostCert,
					HostCertBasePath: "/etc/kubernetes",
				},
			},
			expected: "/etc/kubernetes",
		},
		{
			name: "hostCert type with only fileName - uses empty basePath (CEL would block this)",
			workloadAttestors: &v1alpha1.WorkloadAttestors{
				K8sEnabled: "true",
				WorkloadAttestorsVerification: &v1alpha1.WorkloadAttestorsVerification{
					Type:             utils.WorkloadAttestorVerificationTypeHostCert,
					HostCertFileName: "kubelet-ca.crt",
				},
			},
			expected: "",
		},
		{
			name: "auto type without paths - uses OpenShift defaults",
			workloadAttestors: &v1alpha1.WorkloadAttestors{
				K8sEnabled: "true",
				WorkloadAttestorsVerification: &v1alpha1.WorkloadAttestorsVerification{
					Type: utils.WorkloadAttestorVerificationTypeAuto,
				},
			},
			expected: utils.DefaultKubeletCABasePath,
		},
		{
			name: "auto type with both paths - mount specified path",
			workloadAttestors: &v1alpha1.WorkloadAttestors{
				K8sEnabled: "true",
				WorkloadAttestorsVerification: &v1alpha1.WorkloadAttestorsVerification{
					Type:             utils.WorkloadAttestorVerificationTypeAuto,
					HostCertBasePath: "/custom/path",
					HostCertFileName: "custom-ca.crt",
				},
			},
			expected: "/custom/path",
		},
		{
			name: "auto type with only basePath - uses OpenShift defaults",
			workloadAttestors: &v1alpha1.WorkloadAttestors{
				K8sEnabled: "true",
				WorkloadAttestorsVerification: &v1alpha1.WorkloadAttestorsVerification{
					Type:             utils.WorkloadAttestorVerificationTypeAuto,
					HostCertBasePath: "/etc/kubernetes",
				},
			},
			expected: utils.DefaultKubeletCABasePath,
		},
		{
			name: "unknown type - no mount",
			workloadAttestors: &v1alpha1.WorkloadAttestors{
				K8sEnabled: "true",
				WorkloadAttestorsVerification: &v1alpha1.WorkloadAttestorsVerification{
					Type:             "unknown",
					HostCertBasePath: "/etc/kubernetes",
					HostCertFileName: "kubelet-ca.crt",
				},
			},
			expected: "",
		},
		{
			name: "empty type - no mount",
			workloadAttestors: &v1alpha1.WorkloadAttestors{
				K8sEnabled: "true",
				WorkloadAttestorsVerification: &v1alpha1.WorkloadAttestorsVerification{
					Type:             "",
					HostCertBasePath: "/etc/kubernetes",
					HostCertFileName: "kubelet-ca.crt",
				},
			},
			expected: "",
		},
		{
			name: "custom path with hostCert",
			workloadAttestors: &v1alpha1.WorkloadAttestors{
				K8sEnabled: "true",
				WorkloadAttestorsVerification: &v1alpha1.WorkloadAttestorsVerification{
					Type:             utils.WorkloadAttestorVerificationTypeHostCert,
					HostCertBasePath: "/var/lib/kubelet/pki",
					HostCertFileName: "ca.crt",
				},
			},
			expected: "/var/lib/kubelet/pki",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getHostCertMountPath(tt.workloadAttestors)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestHostPathTypePtr(t *testing.T) {
	// Test that the helper function works correctly
	result := hostPathTypePtr("DirectoryOrCreate")
	assert.NotNil(t, result)
	assert.Equal(t, "DirectoryOrCreate", string(*result))
}

// assertSpireAgentContainerHardening checks container securityContext matches
// generateSpireAgentSCC: non-privileged, no escalation, cap drop ALL, R/O rootfs (see scc.go).
func assertSpireAgentContainerHardening(t *testing.T, c *corev1.Container) {
	t.Helper()
	require.NotNil(t, c.SecurityContext, "spire-agent container must set securityContext for SCC hardening")
	sc := c.SecurityContext
	require.NotNil(t, sc.AllowPrivilegeEscalation, "allowPrivilegeEscalation must be explicit (false)")
	assert.False(t, *sc.AllowPrivilegeEscalation, "AllowPrivilegeEscalation must be false to match spire-agent SCC")
	require.NotNil(t, sc.Privileged, "privileged must be explicit (false)")
	assert.False(t, *sc.Privileged, "container must not run privileged; matches AllowPrivilegedContainer: false in SCC")
	require.NotNil(t, sc.ReadOnlyRootFilesystem, "readOnlyRootFilesystem must be explicit (true)")
	assert.True(t, *sc.ReadOnlyRootFilesystem, "ReadOnlyRootFilesystem must be true; matches spire-agent SCC")
	require.NotNil(t, sc.Capabilities, "capabilities must be set")
	require.NotNil(t, sc.Capabilities.Drop, "must drop capabilities")
	assert.Equal(t, []corev1.Capability{corev1.Capability("ALL")}, sc.Capabilities.Drop, "requiredDropCapabilities [ALL] in SCC")
}

func TestGenerateSpireAgentDaemonSet_SCCSecurityHardening(t *testing.T) {
	ztwim := &v1alpha1.ZeroTrustWorkloadIdentityManager{
		Spec: v1alpha1.ZeroTrustWorkloadIdentityManagerSpec{
			TrustDomain:     "example.org",
			BundleConfigMap: "spire-bundle",
		},
	}

	t.Run("pod boundary matches spire-agent SCC (host PID, no host network/IPC/ports in SCC)", func(t *testing.T) {
		spec := v1alpha1.SpireAgentSpec{
			SocketPath: "/tmp/spire-agent/public",
		}
		ds := generateSpireAgentDaemonSet(spec, ztwim, "test-config-hash")
		require.NotNil(t, ds)
		pod := &ds.Spec.Template.Spec
		assert.Equal(t, "spire-agent", pod.ServiceAccountName)
		assert.True(t, pod.HostPID, "HostPID required for k8s workload attestation; SCC AllowHostPID: true")
		assert.False(t, pod.HostNetwork, "HostNetwork disabled; SCC allowHostNetwork: false")
		assert.Equal(t, corev1.DNSClusterFirst, pod.DNSPolicy)
		require.Len(t, pod.Containers, 1)
		assertSpireAgentContainerHardening(t, &pod.Containers[0])
	})

	t.Run("security context unchanged when kubelet CA hostPath is present", func(t *testing.T) {
		spec := v1alpha1.SpireAgentSpec{
			SocketPath: "/tmp/spire-agent/public",
			WorkloadAttestors: &v1alpha1.WorkloadAttestors{
				K8sEnabled: "true",
				WorkloadAttestorsVerification: &v1alpha1.WorkloadAttestorsVerification{
					Type: utils.WorkloadAttestorVerificationTypeAuto,
				},
			},
		}
		ds := generateSpireAgentDaemonSet(spec, ztwim, "hash")
		var sawKubeletCA bool
		for _, v := range ds.Spec.Template.Spec.Volumes {
			if v.Name == "kubelet-ca" {
				sawKubeletCA = true
				break
			}
		}
		require.True(t, sawKubeletCA, "expected kubelet-ca volume for auto verification / host cert")
		require.Len(t, ds.Spec.Template.Spec.Containers, 1)
		assertSpireAgentContainerHardening(t, &ds.Spec.Template.Spec.Containers[0])
	})
}
