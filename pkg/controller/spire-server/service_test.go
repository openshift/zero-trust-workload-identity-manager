package spire_server

import (
	"testing"

	"github.com/openshift/zero-trust-workload-identity-manager/pkg/controller/utils"
)

func TestGetSpireServerService(t *testing.T) {
	t.Run("without custom labels", func(t *testing.T) {
		svc := getSpireServerService(nil)

		if svc == nil {
			t.Fatal("Expected Service, got nil")
		}

		if svc.Name != "spire-server" {
			t.Errorf("Expected Service name 'spire-server', got '%s'", svc.Name)
		}

		if svc.Namespace != utils.OperatorNamespace {
			t.Errorf("Expected Service namespace '%s', got '%s'", utils.OperatorNamespace, svc.Namespace)
		}

		// Check labels
		if len(svc.Labels) == 0 {
			t.Error("Expected Service to have labels, got none")
		}

		// Check for required labels
		if val, ok := svc.Labels[utils.AppManagedByLabelKey]; !ok || val != utils.AppManagedByLabelValue {
			t.Errorf("Expected label %s=%s", utils.AppManagedByLabelKey, utils.AppManagedByLabelValue)
		}

		if val, ok := svc.Labels["app.kubernetes.io/component"]; !ok || val != utils.ComponentControlPlane {
			t.Errorf("Expected label app.kubernetes.io/component=%s", utils.ComponentControlPlane)
		}

		// Check selectors
		if len(svc.Spec.Selector) == 0 {
			t.Error("Expected Service to have selectors, got none")
		}

		if val, ok := svc.Spec.Selector["app.kubernetes.io/name"]; !ok || val != "spire-server" {
			t.Error("Expected selector app.kubernetes.io/name=spire-server")
		}

		if val, ok := svc.Spec.Selector["app.kubernetes.io/instance"]; !ok || val != utils.StandardInstance {
			t.Errorf("Expected selector app.kubernetes.io/instance=%s", utils.StandardInstance)
		}
	})

	t.Run("with custom labels", func(t *testing.T) {
		customLabels := map[string]string{
			"service-type": "control-plane",
			"priority":     "critical",
		}

		svc := getSpireServerService(customLabels)

		if svc == nil {
			t.Fatal("Expected Service, got nil")
		}

		// Check that custom labels are present
		if val, ok := svc.Labels["service-type"]; !ok || val != "control-plane" {
			t.Errorf("Expected custom label 'service-type=control-plane', got '%s'", val)
		}

		if val, ok := svc.Labels["priority"]; !ok || val != "critical" {
			t.Errorf("Expected custom label 'priority=critical', got '%s'", val)
		}

		// Check that standard labels are still present
		if val, ok := svc.Labels[utils.AppManagedByLabelKey]; !ok || val != utils.AppManagedByLabelValue {
			t.Errorf("Expected label %s=%s to be preserved with custom labels", utils.AppManagedByLabelKey, utils.AppManagedByLabelValue)
		}

		if val, ok := svc.Labels["app.kubernetes.io/component"]; !ok || val != utils.ComponentControlPlane {
			t.Errorf("Expected label app.kubernetes.io/component=%s to be preserved with custom labels", utils.ComponentControlPlane)
		}
	})

	t.Run("preserves all asset labels", func(t *testing.T) {
		// Get labels without custom labels (these come from asset file)
		svcWithoutCustom := getSpireServerService(nil)
		assetLabels := make(map[string]string)
		for k, v := range svcWithoutCustom.Labels {
			assetLabels[k] = v
		}

		// Get labels with custom labels
		customLabels := map[string]string{
			"cluster": "production",
		}
		svcWithCustom := getSpireServerService(customLabels)

		// All asset labels should still be present
		for k, v := range assetLabels {
			if svcWithCustom.Labels[k] != v {
				t.Errorf("Asset label '%s=%s' was not preserved when custom labels were added, got '%s'", k, v, svcWithCustom.Labels[k])
			}
		}

		// Custom label should also be present
		if val, ok := svcWithCustom.Labels["cluster"]; !ok || val != "production" {
			t.Errorf("Custom label was not added")
		}
	})
}

func TestGetSpireControllerManagerWebhookService(t *testing.T) {
	t.Run("without custom labels", func(t *testing.T) {
		svc := getSpireControllerManagerWebhookService(nil)

		if svc == nil {
			t.Fatal("Expected Service, got nil")
		}

		if svc.Name != "spire-controller-manager-webhook" {
			t.Errorf("Expected Service name 'spire-controller-manager-webhook', got '%s'", svc.Name)
		}

		// Check selectors
		if val, ok := svc.Spec.Selector["app.kubernetes.io/name"]; !ok || val != "spire-controller-manager" {
			t.Error("Expected selector app.kubernetes.io/name=spire-controller-manager")
		}

		// Check labels
		if len(svc.Labels) == 0 {
			t.Error("Expected Service to have labels, got none")
		}
	})

	t.Run("with custom labels", func(t *testing.T) {
		customLabels := map[string]string{
			"webhook-type": "validating",
			"component":    "admission-control",
		}

		svc := getSpireControllerManagerWebhookService(customLabels)

		if svc == nil {
			t.Fatal("Expected Service, got nil")
		}

		// Check that custom labels are present
		if val, ok := svc.Labels["webhook-type"]; !ok || val != "validating" {
			t.Errorf("Expected custom label 'webhook-type=validating', got '%s'", val)
		}

		if val, ok := svc.Labels["component"]; !ok || val != "admission-control" {
			t.Errorf("Expected custom label 'component=admission-control', got '%s'", val)
		}

		// Check that standard labels are still present
		if val, ok := svc.Labels[utils.AppManagedByLabelKey]; !ok || val != utils.AppManagedByLabelValue {
			t.Errorf("Expected label %s=%s to be preserved with custom labels", utils.AppManagedByLabelKey, utils.AppManagedByLabelValue)
		}
	})

	t.Run("preserves all asset labels", func(t *testing.T) {
		// Get labels without custom labels (these come from asset file)
		svcWithoutCustom := getSpireControllerManagerWebhookService(nil)
		assetLabels := make(map[string]string)
		for k, v := range svcWithoutCustom.Labels {
			assetLabels[k] = v
		}

		// Get labels with custom labels
		customLabels := map[string]string{
			"test": "value",
		}
		svcWithCustom := getSpireControllerManagerWebhookService(customLabels)

		// All asset labels should still be present
		for k, v := range assetLabels {
			if svcWithCustom.Labels[k] != v {
				t.Errorf("Asset label '%s=%s' was not preserved when custom labels were added, got '%s'", k, v, svcWithCustom.Labels[k])
			}
		}

		// Custom label should also be present
		if val, ok := svcWithCustom.Labels["test"]; !ok || val != "value" {
			t.Errorf("Custom label was not added")
		}
	})
}
