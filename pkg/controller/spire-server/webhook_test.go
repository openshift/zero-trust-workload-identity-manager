package spire_server

import (
	"testing"

	"github.com/openshift/zero-trust-workload-identity-manager/pkg/controller/utils"
)

func TestGetSpireControllerManagerValidatingWebhookConfiguration(t *testing.T) {
	webhook := getSpireControllerManagerValidatingWebhookConfiguration(nil)

	if webhook == nil {
		t.Fatal("Expected ValidatingWebhookConfiguration, got nil")
	}

	if webhook.Name != "spire-controller-manager-webhook" {
		t.Errorf("Expected ValidatingWebhookConfiguration name 'spire-controller-manager-webhook', got '%s'", webhook.Name)
	}

	// Check labels
	if val, ok := webhook.Labels[utils.AppManagedByLabelKey]; !ok || val != utils.AppManagedByLabelValue {
		t.Errorf("Expected label %s=%s", utils.AppManagedByLabelKey, utils.AppManagedByLabelValue)
	}
}
