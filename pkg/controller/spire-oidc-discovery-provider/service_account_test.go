package spire_oidc_discovery_provider

import (
	"testing"

	"github.com/openshift/zero-trust-workload-identity-manager/pkg/controller/utils"
)

func TestGetSpireOIDCDiscoveryProviderServiceAccount(t *testing.T) {
	sa := getSpireOIDCDiscoveryProviderServiceAccount()

	if sa == nil {
		t.Fatal("Expected ServiceAccount, got nil")
	}

	if sa.Name != "spire-spiffe-oidc-discovery-provider" {
		t.Errorf("Expected ServiceAccount name 'spire-spiffe-oidc-discovery-provider', got '%s'", sa.Name)
	}

	if sa.Namespace != utils.OperatorNamespace {
		t.Errorf("Expected ServiceAccount namespace '%s', got '%s'", utils.OperatorNamespace, sa.Namespace)
	}

	// Check labels
	if len(sa.Labels) == 0 {
		t.Error("Expected ServiceAccount to have labels, got none")
	}

	// Check for required labels
	if val, ok := sa.Labels[utils.AppManagedByLabelKey]; !ok || val != utils.AppManagedByLabelValue {
		t.Errorf("Expected label %s=%s", utils.AppManagedByLabelKey, utils.AppManagedByLabelValue)
	}

	if val, ok := sa.Labels["app.kubernetes.io/component"]; !ok || val != utils.ComponentDiscovery {
		t.Errorf("Expected label app.kubernetes.io/component=%s", utils.ComponentDiscovery)
	}
}
