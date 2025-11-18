package spire_oidc_discovery_provider

import (
	"testing"

	"github.com/openshift/zero-trust-workload-identity-manager/pkg/controller/utils"
)

func TestGetSpireOIDCDiscoveryProviderServiceAccount(t *testing.T) {
	t.Run("without custom labels", func(t *testing.T) {
		sa := getSpireOIDCDiscoveryProviderServiceAccount(nil)

		if sa == nil {
			t.Fatal("Expected ServiceAccount, got nil")
		}

		if sa.Name != "spire-spiffe-oidc-discovery-provider" {
			t.Errorf("Expected ServiceAccount name 'spire-spiffe-oidc-discovery-provider', got '%s'", sa.Name)
		}

		if sa.Namespace != utils.GetOperatorNamespace() {
			t.Errorf("Expected ServiceAccount namespace '%s', got '%s'", utils.GetOperatorNamespace(), sa.Namespace)
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
	})

	t.Run("with custom labels", func(t *testing.T) {
		customLabels := map[string]string{
			"service-tier": "discovery",
			"zone":         "global",
		}

		sa := getSpireOIDCDiscoveryProviderServiceAccount(customLabels)

		if sa == nil {
			t.Fatal("Expected ServiceAccount, got nil")
		}

		// Check that custom labels are present
		if val, ok := sa.Labels["service-tier"]; !ok || val != "discovery" {
			t.Errorf("Expected custom label 'service-tier=discovery', got '%s'", val)
		}

		if val, ok := sa.Labels["zone"]; !ok || val != "global" {
			t.Errorf("Expected custom label 'zone=global', got '%s'", val)
		}

		// Check that standard labels are still present
		if val, ok := sa.Labels[utils.AppManagedByLabelKey]; !ok || val != utils.AppManagedByLabelValue {
			t.Errorf("Expected label %s=%s to be preserved with custom labels", utils.AppManagedByLabelKey, utils.AppManagedByLabelValue)
		}

		if val, ok := sa.Labels["app.kubernetes.io/component"]; !ok || val != utils.ComponentDiscovery {
			t.Errorf("Expected label app.kubernetes.io/component=%s to be preserved with custom labels", utils.ComponentDiscovery)
		}
	})

	t.Run("preserves all asset labels", func(t *testing.T) {
		// Get labels without custom labels (these come from asset file)
		saWithoutCustom := getSpireOIDCDiscoveryProviderServiceAccount(nil)
		assetLabels := make(map[string]string)
		for k, v := range saWithoutCustom.Labels {
			assetLabels[k] = v
		}

		// Get labels with custom labels
		customLabels := map[string]string{
			"release": "v2.5.0",
		}
		saWithCustom := getSpireOIDCDiscoveryProviderServiceAccount(customLabels)

		// All asset labels should still be present
		for k, v := range assetLabels {
			if saWithCustom.Labels[k] != v {
				t.Errorf("Asset label '%s=%s' was not preserved when custom labels were added, got '%s'", k, v, saWithCustom.Labels[k])
			}
		}

		// Custom label should also be present
		if val, ok := saWithCustom.Labels["release"]; !ok || val != "v2.5.0" {
			t.Errorf("Custom label was not added")
		}
	})
}
