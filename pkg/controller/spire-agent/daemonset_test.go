package spire_agent

import (
	"testing"

	"github.com/openshift/zero-trust-workload-identity-manager/api/v1alpha1"
	"github.com/stretchr/testify/assert"
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
					Type: "skip",
				},
			},
			expected: "",
		},
		{
			name: "hostCert type with both paths - mount needed",
			workloadAttestors: &v1alpha1.WorkloadAttestors{
				K8sEnabled: "true",
				WorkloadAttestorsVerification: &v1alpha1.WorkloadAttestorsVerification{
					Type:             "hostCert",
					HostCertBasePath: "/etc/kubernetes",
					HostCertFileName: "kubelet-ca.crt",
				},
			},
			expected: "/etc/kubernetes",
		},
		{
			name: "hostCert type with only basePath - no mount (incomplete config)",
			workloadAttestors: &v1alpha1.WorkloadAttestors{
				K8sEnabled: "true",
				WorkloadAttestorsVerification: &v1alpha1.WorkloadAttestorsVerification{
					Type:             "hostCert",
					HostCertBasePath: "/etc/kubernetes",
				},
			},
			expected: "",
		},
		{
			name: "hostCert type with only fileName - no mount (incomplete config)",
			workloadAttestors: &v1alpha1.WorkloadAttestors{
				K8sEnabled: "true",
				WorkloadAttestorsVerification: &v1alpha1.WorkloadAttestorsVerification{
					Type:             "hostCert",
					HostCertFileName: "kubelet-ca.crt",
				},
			},
			expected: "",
		},
		{
			name: "auto type without paths - no mount (uses SPIRE default)",
			workloadAttestors: &v1alpha1.WorkloadAttestors{
				K8sEnabled: "true",
				WorkloadAttestorsVerification: &v1alpha1.WorkloadAttestorsVerification{
					Type: "auto",
				},
			},
			expected: "",
		},
		{
			name: "auto type with both paths - mount needed",
			workloadAttestors: &v1alpha1.WorkloadAttestors{
				K8sEnabled: "true",
				WorkloadAttestorsVerification: &v1alpha1.WorkloadAttestorsVerification{
					Type:             "auto",
					HostCertBasePath: "/etc/kubernetes",
					HostCertFileName: "kubelet-ca.crt",
				},
			},
			expected: "/etc/kubernetes",
		},
		{
			name: "auto type with only basePath - no mount",
			workloadAttestors: &v1alpha1.WorkloadAttestors{
				K8sEnabled: "true",
				WorkloadAttestorsVerification: &v1alpha1.WorkloadAttestorsVerification{
					Type:             "auto",
					HostCertBasePath: "/etc/kubernetes",
				},
			},
			expected: "",
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
			name: "custom path",
			workloadAttestors: &v1alpha1.WorkloadAttestors{
				K8sEnabled: "true",
				WorkloadAttestorsVerification: &v1alpha1.WorkloadAttestorsVerification{
					Type:             "hostCert",
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
