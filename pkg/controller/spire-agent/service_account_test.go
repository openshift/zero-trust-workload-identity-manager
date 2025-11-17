package spire_agent

import (
	"testing"

	"github.com/openshift/zero-trust-workload-identity-manager/pkg/controller/utils"
)

func TestGetSpireAgentServiceAccount(t *testing.T) {
	t.Run("without custom labels", func(t *testing.T) {
		sa := getSpireAgentServiceAccount(nil)

		if sa == nil {
			t.Fatal("Expected ServiceAccount, got nil")
		}

		if sa.Name != "spire-agent" {
			t.Errorf("Expected ServiceAccount name 'spire-agent', got '%s'", sa.Name)
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

		if val, ok := sa.Labels["app.kubernetes.io/component"]; !ok || val != utils.ComponentNodeAgent {
			t.Errorf("Expected label app.kubernetes.io/component=%s", utils.ComponentNodeAgent)
		}
	})

	t.Run("with custom labels", func(t *testing.T) {
		customLabels := map[string]string{
			"owner":       "platform-team",
			"environment": "staging",
		}

		sa := getSpireAgentServiceAccount(customLabels)

		if sa == nil {
			t.Fatal("Expected ServiceAccount, got nil")
		}

		// Check that custom labels are present
		if val, ok := sa.Labels["owner"]; !ok || val != "platform-team" {
			t.Errorf("Expected custom label 'owner=platform-team', got '%s'", val)
		}

		if val, ok := sa.Labels["environment"]; !ok || val != "staging" {
			t.Errorf("Expected custom label 'environment=staging', got '%s'", val)
		}

		// Check that standard labels are still present
		if val, ok := sa.Labels[utils.AppManagedByLabelKey]; !ok || val != utils.AppManagedByLabelValue {
			t.Errorf("Expected label %s=%s to be preserved with custom labels", utils.AppManagedByLabelKey, utils.AppManagedByLabelValue)
		}

		if val, ok := sa.Labels["app.kubernetes.io/component"]; !ok || val != utils.ComponentNodeAgent {
			t.Errorf("Expected label app.kubernetes.io/component=%s to be preserved with custom labels", utils.ComponentNodeAgent)
		}
	})

	t.Run("preserves all asset labels", func(t *testing.T) {
		// Get labels without custom labels (these come from asset file)
		saWithoutCustom := getSpireAgentServiceAccount(nil)
		assetLabels := make(map[string]string)
		for k, v := range saWithoutCustom.Labels {
			assetLabels[k] = v
		}

		// Get labels with custom labels
		customLabels := map[string]string{
			"app-version": "v1.2.3",
		}
		saWithCustom := getSpireAgentServiceAccount(customLabels)

		// All asset labels should still be present
		for k, v := range assetLabels {
			if saWithCustom.Labels[k] != v {
				t.Errorf("Asset label '%s=%s' was not preserved when custom labels were added, got '%s'", k, v, saWithCustom.Labels[k])
			}
		}

		// Custom label should also be present
		if val, ok := saWithCustom.Labels["app-version"]; !ok || val != "v1.2.3" {
			t.Errorf("Custom label was not added")
		}
	})
}
