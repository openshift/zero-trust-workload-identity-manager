package spire_server

import (
	"testing"

	"github.com/openshift/zero-trust-workload-identity-manager/pkg/controller/utils"
)

func TestGetSpireControllerManagerValidatingWebhookConfiguration(t *testing.T) {
	t.Run("without custom labels", func(t *testing.T) {
		webhook := getSpireControllerManagerValidatingWebhookConfiguration(nil)

		if webhook == nil {
			t.Fatal("Expected ValidatingWebhookConfiguration, got nil")
		}

		if webhook.Name != "spire-controller-manager-webhook" {
			t.Errorf("Expected ValidatingWebhookConfiguration name 'spire-controller-manager-webhook', got '%s'", webhook.Name)
		}

		// Check labels
		if len(webhook.Labels) == 0 {
			t.Error("Expected ValidatingWebhookConfiguration to have labels, got none")
		}

		if val, ok := webhook.Labels[utils.AppManagedByLabelKey]; !ok || val != utils.AppManagedByLabelValue {
			t.Errorf("Expected label %s=%s", utils.AppManagedByLabelKey, utils.AppManagedByLabelValue)
		}

		if val, ok := webhook.Labels["app.kubernetes.io/component"]; !ok || val != utils.ComponentControlPlane {
			t.Errorf("Expected label app.kubernetes.io/component=%s", utils.ComponentControlPlane)
		}
	})

	t.Run("with custom labels", func(t *testing.T) {
		customLabels := map[string]string{
			"admission-type": "validating",
			"security-tier":  "high",
		}

		webhook := getSpireControllerManagerValidatingWebhookConfiguration(customLabels)

		if webhook == nil {
			t.Fatal("Expected ValidatingWebhookConfiguration, got nil")
		}

		// Check that custom labels are present
		if val, ok := webhook.Labels["admission-type"]; !ok || val != "validating" {
			t.Errorf("Expected custom label 'admission-type=validating', got '%s'", val)
		}

		if val, ok := webhook.Labels["security-tier"]; !ok || val != "high" {
			t.Errorf("Expected custom label 'security-tier=high', got '%s'", val)
		}

		// Check that standard labels are still present
		if val, ok := webhook.Labels[utils.AppManagedByLabelKey]; !ok || val != utils.AppManagedByLabelValue {
			t.Errorf("Expected label %s=%s to be preserved with custom labels", utils.AppManagedByLabelKey, utils.AppManagedByLabelValue)
		}

		if val, ok := webhook.Labels["app.kubernetes.io/component"]; !ok || val != utils.ComponentControlPlane {
			t.Errorf("Expected label app.kubernetes.io/component=%s to be preserved with custom labels", utils.ComponentControlPlane)
		}
	})

	t.Run("preserves all asset labels", func(t *testing.T) {
		// Get labels without custom labels (these come from asset file)
		webhookWithoutCustom := getSpireControllerManagerValidatingWebhookConfiguration(nil)
		assetLabels := make(map[string]string)
		for k, v := range webhookWithoutCustom.Labels {
			assetLabels[k] = v
		}

		// Get labels with custom labels
		customLabels := map[string]string{
			"webhook-version": "v1",
		}
		webhookWithCustom := getSpireControllerManagerValidatingWebhookConfiguration(customLabels)

		// All asset labels should still be present
		for k, v := range assetLabels {
			if webhookWithCustom.Labels[k] != v {
				t.Errorf("Asset label '%s=%s' was not preserved when custom labels were added, got '%s'", k, v, webhookWithCustom.Labels[k])
			}
		}

		// Custom label should also be present
		if val, ok := webhookWithCustom.Labels["webhook-version"]; !ok || val != "v1" {
			t.Errorf("Custom label was not added")
		}
	})
}
