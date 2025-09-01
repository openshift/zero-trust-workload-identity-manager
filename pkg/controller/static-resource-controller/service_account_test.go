package static_resource_controller

import (
	"github.com/openshift/zero-trust-workload-identity-manager/pkg/version"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	"testing"
)

func TestGetSpiffeCsiDriverServiceAccount(t *testing.T) {
	r := &StaticResourceReconciler{}
	sa := r.getSpiffeCsiDriverServiceAccount()

	assert.Equal(t, "spire-spiffe-csi-driver", sa.Name)
	assert.Equal(t, "ServiceAccount", sa.Kind)
	assert.Equal(t, "zero-trust-workload-identity-manager", sa.Namespace)

	expectedLabels := map[string]string{
		"app.kubernetes.io/name":       "spiffe-csi-driver",
		"app.kubernetes.io/instance":   "cluster-zero-trust-workload-identity-manager",
		"app.kubernetes.io/component":  "csi",
		"app.kubernetes.io/version":    version.SpiffeCsiVersion,
		"app.kubernetes.io/managed-by": "zero-trust-workload-identity-manager",
		"app.kubernetes.io/part-of":    "zero-trust-workload-identity-manager",
	}
	assert.Equal(t, expectedLabels, sa.Labels)
}

func TestGetSpireAgentServiceAccount(t *testing.T) {
	r := &StaticResourceReconciler{}
	sa := r.getSpireAgentServiceAccount()

	assert.Equal(t, "spire-agent", sa.Name)
	assert.Equal(t, "ServiceAccount", sa.Kind)
	assert.Equal(t, "zero-trust-workload-identity-manager", sa.Namespace)

	expectedLabels := requiredAgentResourceLabels
	assert.Equal(t, expectedLabels, sa.Labels)
}

func TestGetSpireOIDCDiscoveryProviderServiceAccount(t *testing.T) {
	r := &StaticResourceReconciler{}
	sa := r.getSpireOIDCDiscoveryProviderServiceAccount()

	assert.Equal(t, "spire-spiffe-oidc-discovery-provider", sa.Name)
	assert.Equal(t, "ServiceAccount", sa.Kind)
	assert.Equal(t, "zero-trust-workload-identity-manager", sa.Namespace)

	expectedLabels := requiredOIDCResourceLabels
	assert.Equal(t, expectedLabels, sa.Labels)
}

func TestGetSpireServerServiceAccount(t *testing.T) {
	r := &StaticResourceReconciler{}
	sa := r.getSpireServerServiceAccount()

	assert.Equal(t, "spire-server", sa.Name)
	assert.Equal(t, "ServiceAccount", sa.Kind)
	assert.Equal(t, "zero-trust-workload-identity-manager", sa.Namespace)

	expectedLabels := requiredServerResourceLabels
	assert.Equal(t, expectedLabels, sa.Labels)
}

func TestStaticResourceReconciler_ListStaticServiceAccount(t *testing.T) {
	r := &StaticResourceReconciler{}

	serviceAccounts := r.listStaticServiceAccount()

	// Expect 4 service accounts
	assert.Len(t, serviceAccounts, 4)

	// Helper to check labels and namespace common to all
	checkCommonLabels := func(sa *corev1.ServiceAccount) {
		assert.Equal(t, "zero-trust-workload-identity-manager", sa.Namespace)
		expectedLabels := map[string]string{
			"app.kubernetes.io/instance":   "cluster-zero-trust-workload-identity-manager",
			"app.kubernetes.io/managed-by": "zero-trust-workload-identity-manager",
			"app.kubernetes.io/part-of":    "zero-trust-workload-identity-manager",
		}
		for k, v := range expectedLabels {
			assert.Equal(t, v, sa.Labels[k])
		}
	}

	// Check individual service accounts
	for _, sa := range serviceAccounts {
		checkCommonLabels(sa)
	}

	// spiffe-csi-driver SA
	spiffeCsi := serviceAccounts[0]
	assert.Equal(t, "spire-spiffe-csi-driver", spiffeCsi.Name)
	assert.Equal(t, version.SpiffeCsiVersion, spiffeCsi.Labels["app.kubernetes.io/version"])
	assert.Equal(t, "spiffe-csi-driver", spiffeCsi.Labels["app.kubernetes.io/name"])

	// spire-agent SA
	spireAgent := serviceAccounts[1]
	assert.Equal(t, "spire-agent", spireAgent.Name)
	assert.Equal(t, version.SpireAgentVersion, spireAgent.Labels["app.kubernetes.io/version"])
	assert.Equal(t, "spire-agent", spireAgent.Labels["app.kubernetes.io/name"])

	// spire-spiffe-oidc-discovery-provider SA
	spireOIDC := serviceAccounts[2]
	assert.Equal(t, "spire-spiffe-oidc-discovery-provider", spireOIDC.Name)
	assert.Equal(t, version.SpireOIDCDiscoveryProviderVersion, spireOIDC.Labels["app.kubernetes.io/version"])
	assert.Equal(t, "spiffe-oidc-discovery-provider", spireOIDC.Labels["app.kubernetes.io/name"])

	// spire-server SA
	spireServer := serviceAccounts[3]
	assert.Equal(t, "spire-server", spireServer.Name)
	assert.Equal(t, version.SpireServerVersion, spireServer.Labels["app.kubernetes.io/version"])
	assert.Equal(t, "spire-server", spireServer.Labels["app.kubernetes.io/name"])
}
