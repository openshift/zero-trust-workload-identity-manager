package spire_oidc_discovery_provider

import (
	"testing"

	"github.com/openshift/zero-trust-workload-identity-manager/pkg/controller/utils"
)

func TestGetSpireOIDCDiscoveryProviderService(t *testing.T) {
	t.Run("without custom labels", func(t *testing.T) {
		svc := getSpireOIDCDiscoveryProviderService(nil)

		if svc == nil {
			t.Fatal("Expected Service, got nil")
		}

		if svc.Name != "spire-spiffe-oidc-discovery-provider" {
			t.Errorf("Expected Service name 'spire-spiffe-oidc-discovery-provider', got '%s'", svc.Name)
		}

		if svc.Namespace != utils.GetOperatorNamespace() {
			t.Errorf("Expected Service namespace '%s', got '%s'", utils.GetOperatorNamespace(), svc.Namespace)
		}

		// Check labels
		if len(svc.Labels) == 0 {
			t.Error("Expected Service to have labels, got none")
		}

		// Check for required labels
		if val, ok := svc.Labels[utils.AppManagedByLabelKey]; !ok || val != utils.AppManagedByLabelValue {
			t.Errorf("Expected label %s=%s", utils.AppManagedByLabelKey, utils.AppManagedByLabelValue)
		}

		if val, ok := svc.Labels["app.kubernetes.io/component"]; !ok || val != utils.ComponentDiscovery {
			t.Errorf("Expected label app.kubernetes.io/component=%s", utils.ComponentDiscovery)
		}

		// Check selectors
		if len(svc.Spec.Selector) == 0 {
			t.Error("Expected Service to have selectors, got none")
		}

		if val, ok := svc.Spec.Selector["app.kubernetes.io/name"]; !ok || val != "spiffe-oidc-discovery-provider" {
			t.Error("Expected selector app.kubernetes.io/name=spiffe-oidc-discovery-provider")
		}

		if val, ok := svc.Spec.Selector["app.kubernetes.io/instance"]; !ok || val != utils.StandardInstance {
			t.Errorf("Expected selector app.kubernetes.io/instance=%s", utils.StandardInstance)
		}
	})

	t.Run("with custom labels", func(t *testing.T) {
		customLabels := map[string]string{
			"discovery-type": "oidc",
			"public":         "true",
		}

		svc := getSpireOIDCDiscoveryProviderService(customLabels)

		if svc == nil {
			t.Fatal("Expected Service, got nil")
		}

		// Check that custom labels are present
		if val, ok := svc.Labels["discovery-type"]; !ok || val != "oidc" {
			t.Errorf("Expected custom label 'discovery-type=oidc', got '%s'", val)
		}

		if val, ok := svc.Labels["public"]; !ok || val != "true" {
			t.Errorf("Expected custom label 'public=true', got '%s'", val)
		}

		// Check that standard labels are still present
		if val, ok := svc.Labels[utils.AppManagedByLabelKey]; !ok || val != utils.AppManagedByLabelValue {
			t.Errorf("Expected label %s=%s to be preserved with custom labels", utils.AppManagedByLabelKey, utils.AppManagedByLabelValue)
		}

		if val, ok := svc.Labels["app.kubernetes.io/component"]; !ok || val != utils.ComponentDiscovery {
			t.Errorf("Expected label app.kubernetes.io/component=%s to be preserved with custom labels", utils.ComponentDiscovery)
		}
	})

	t.Run("preserves all asset labels", func(t *testing.T) {
		// Get labels without custom labels (these come from asset file)
		svcWithoutCustom := getSpireOIDCDiscoveryProviderService(nil)
		assetLabels := make(map[string]string)
		for k, v := range svcWithoutCustom.Labels {
			assetLabels[k] = v
		}

		// Get labels with custom labels
		customLabels := map[string]string{
			"endpoint": "/.well-known/openid-configuration",
		}
		svcWithCustom := getSpireOIDCDiscoveryProviderService(customLabels)

		// All asset labels should still be present
		for k, v := range assetLabels {
			if svcWithCustom.Labels[k] != v {
				t.Errorf("Asset label '%s=%s' was not preserved when custom labels were added, got '%s'", k, v, svcWithCustom.Labels[k])
			}
		}

		// Custom label should also be present
		if val, ok := svcWithCustom.Labels["endpoint"]; !ok || val != "/.well-known/openid-configuration" {
			t.Errorf("Custom label was not added")
		}
	})
}
