package spire_agent

import (
	"testing"

	"github.com/openshift/zero-trust-workload-identity-manager/pkg/controller/utils"
)

func TestGetSpireAgentService(t *testing.T) {
	t.Run("without custom labels", func(t *testing.T) {
		svc := getSpireAgentService(nil)

		if svc == nil {
			t.Fatal("Expected Service, got nil")
		}

		if svc.Name != "spire-agent" {
			t.Errorf("Expected Service name 'spire-agent', got '%s'", svc.Name)
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

		if val, ok := svc.Labels["app.kubernetes.io/component"]; !ok || val != utils.ComponentNodeAgent {
			t.Errorf("Expected label app.kubernetes.io/component=%s", utils.ComponentNodeAgent)
		}

		// Check selectors
		if len(svc.Spec.Selector) == 0 {
			t.Error("Expected Service to have selectors, got none")
		}

		if val, ok := svc.Spec.Selector["app.kubernetes.io/name"]; !ok || val != "spire-agent" {
			t.Error("Expected selector app.kubernetes.io/name=spire-agent")
		}
	})

	t.Run("with custom labels", func(t *testing.T) {
		customLabels := map[string]string{
			"monitoring": "prometheus",
			"tier":       "infrastructure",
		}

		svc := getSpireAgentService(customLabels)

		if svc == nil {
			t.Fatal("Expected Service, got nil")
		}

		// Check that custom labels are present
		if val, ok := svc.Labels["monitoring"]; !ok || val != "prometheus" {
			t.Errorf("Expected custom label 'monitoring=prometheus', got '%s'", val)
		}

		if val, ok := svc.Labels["tier"]; !ok || val != "infrastructure" {
			t.Errorf("Expected custom label 'tier=infrastructure', got '%s'", val)
		}

		// Check that standard labels are still present
		if val, ok := svc.Labels[utils.AppManagedByLabelKey]; !ok || val != utils.AppManagedByLabelValue {
			t.Errorf("Expected label %s=%s to be preserved with custom labels", utils.AppManagedByLabelKey, utils.AppManagedByLabelValue)
		}

		if val, ok := svc.Labels["app.kubernetes.io/component"]; !ok || val != utils.ComponentNodeAgent {
			t.Errorf("Expected label app.kubernetes.io/component=%s to be preserved with custom labels", utils.ComponentNodeAgent)
		}

		// Check selectors remain unchanged
		if val, ok := svc.Spec.Selector["app.kubernetes.io/name"]; !ok || val != "spire-agent" {
			t.Error("Expected selector app.kubernetes.io/name=spire-agent to remain unchanged")
		}
	})

	t.Run("preserves all asset labels", func(t *testing.T) {
		// Get labels without custom labels (these come from asset file)
		svcWithoutCustom := getSpireAgentService(nil)
		assetLabels := make(map[string]string)
		for k, v := range svcWithoutCustom.Labels {
			assetLabels[k] = v
		}

		// Get labels with custom labels
		customLabels := map[string]string{
			"region": "us-east-1",
		}
		svcWithCustom := getSpireAgentService(customLabels)

		// All asset labels should still be present
		for k, v := range assetLabels {
			if svcWithCustom.Labels[k] != v {
				t.Errorf("Asset label '%s=%s' was not preserved when custom labels were added, got '%s'", k, v, svcWithCustom.Labels[k])
			}
		}

		// Custom label should also be present
		if val, ok := svcWithCustom.Labels["region"]; !ok || val != "us-east-1" {
			t.Errorf("Custom label was not added")
		}
	})
}
