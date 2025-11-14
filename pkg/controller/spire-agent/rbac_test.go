package spire_agent

import (
	"testing"

	"github.com/openshift/zero-trust-workload-identity-manager/pkg/controller/utils"
)

func TestGetSpireAgentClusterRole(t *testing.T) {
	t.Run("without custom labels", func(t *testing.T) {
		cr := getSpireAgentClusterRole(nil)

		if cr == nil {
			t.Fatal("Expected ClusterRole, got nil")
		}

		if cr.Name != "spire-agent" {
			t.Errorf("Expected ClusterRole name 'spire-agent', got '%s'", cr.Name)
		}

		// Check labels
		if val, ok := cr.Labels[utils.AppManagedByLabelKey]; !ok || val != utils.AppManagedByLabelValue {
			t.Errorf("Expected label %s=%s", utils.AppManagedByLabelKey, utils.AppManagedByLabelValue)
		}

		if val, ok := cr.Labels["app.kubernetes.io/component"]; !ok || val != utils.ComponentNodeAgent {
			t.Errorf("Expected label app.kubernetes.io/component=%s", utils.ComponentNodeAgent)
		}

		// Check for asset labels
		if len(cr.Labels) == 0 {
			t.Error("Expected ClusterRole to have labels from asset file")
		}
	})

	t.Run("with custom labels", func(t *testing.T) {
		customLabels := map[string]string{
			"custom-label-1": "custom-value-1",
			"env":            "production",
		}

		cr := getSpireAgentClusterRole(customLabels)

		if cr == nil {
			t.Fatal("Expected ClusterRole, got nil")
		}

		// Check that custom labels are present
		if val, ok := cr.Labels["custom-label-1"]; !ok || val != "custom-value-1" {
			t.Errorf("Expected custom label 'custom-label-1=custom-value-1', got '%s'", val)
		}

		if val, ok := cr.Labels["env"]; !ok || val != "production" {
			t.Errorf("Expected custom label 'env=production', got '%s'", val)
		}

		// Check that standard labels are still present
		if val, ok := cr.Labels[utils.AppManagedByLabelKey]; !ok || val != utils.AppManagedByLabelValue {
			t.Errorf("Expected label %s=%s to be preserved with custom labels", utils.AppManagedByLabelKey, utils.AppManagedByLabelValue)
		}

		if val, ok := cr.Labels["app.kubernetes.io/component"]; !ok || val != utils.ComponentNodeAgent {
			t.Errorf("Expected label app.kubernetes.io/component=%s to be preserved with custom labels", utils.ComponentNodeAgent)
		}
	})

	t.Run("preserves all asset labels", func(t *testing.T) {
		// Get labels without custom labels
		crWithoutCustom := getSpireAgentClusterRole(nil)
		assetLabels := make(map[string]string)
		for k, v := range crWithoutCustom.Labels {
			assetLabels[k] = v
		}

		// Get labels with custom labels
		customLabels := map[string]string{
			"custom-test": "value",
		}
		crWithCustom := getSpireAgentClusterRole(customLabels)

		// All asset labels should still be present
		for k, v := range assetLabels {
			if crWithCustom.Labels[k] != v {
				t.Errorf("Asset label '%s=%s' was not preserved when custom labels were added, got '%s'", k, v, crWithCustom.Labels[k])
			}
		}

		// Custom label should also be present
		if val, ok := crWithCustom.Labels["custom-test"]; !ok || val != "value" {
			t.Errorf("Custom label was not added")
		}
	})
}

func TestGetSpireAgentClusterRoleBinding(t *testing.T) {
	t.Run("without custom labels", func(t *testing.T) {
		crb := getSpireAgentClusterRoleBinding(nil)

		if crb == nil {
			t.Fatal("Expected ClusterRoleBinding, got nil")
		}

		if crb.Name != "spire-agent" {
			t.Errorf("Expected ClusterRoleBinding name 'spire-agent', got '%s'", crb.Name)
		}

		// Check labels
		if val, ok := crb.Labels[utils.AppManagedByLabelKey]; !ok || val != utils.AppManagedByLabelValue {
			t.Errorf("Expected label %s=%s", utils.AppManagedByLabelKey, utils.AppManagedByLabelValue)
		}

		if val, ok := crb.Labels["app.kubernetes.io/component"]; !ok || val != utils.ComponentNodeAgent {
			t.Errorf("Expected label app.kubernetes.io/component=%s", utils.ComponentNodeAgent)
		}
	})

	t.Run("with custom labels", func(t *testing.T) {
		customLabels := map[string]string{
			"team":        "security",
			"cost-center": "eng-123",
		}

		crb := getSpireAgentClusterRoleBinding(customLabels)

		if crb == nil {
			t.Fatal("Expected ClusterRoleBinding, got nil")
		}

		// Check that custom labels are present
		if val, ok := crb.Labels["team"]; !ok || val != "security" {
			t.Errorf("Expected custom label 'team=security', got '%s'", val)
		}

		if val, ok := crb.Labels["cost-center"]; !ok || val != "eng-123" {
			t.Errorf("Expected custom label 'cost-center=eng-123', got '%s'", val)
		}

		// Check that standard labels are still present
		if val, ok := crb.Labels[utils.AppManagedByLabelKey]; !ok || val != utils.AppManagedByLabelValue {
			t.Errorf("Expected label %s=%s to be preserved with custom labels", utils.AppManagedByLabelKey, utils.AppManagedByLabelValue)
		}

		if val, ok := crb.Labels["app.kubernetes.io/component"]; !ok || val != utils.ComponentNodeAgent {
			t.Errorf("Expected label app.kubernetes.io/component=%s to be preserved with custom labels", utils.ComponentNodeAgent)
		}
	})

	t.Run("preserves all asset labels", func(t *testing.T) {
		// Get labels without custom labels
		crbWithoutCustom := getSpireAgentClusterRoleBinding(nil)
		assetLabels := make(map[string]string)
		for k, v := range crbWithoutCustom.Labels {
			assetLabels[k] = v
		}

		// Get labels with custom labels
		customLabels := map[string]string{
			"test-label": "test-value",
		}
		crbWithCustom := getSpireAgentClusterRoleBinding(customLabels)

		// All asset labels should still be present
		for k, v := range assetLabels {
			if crbWithCustom.Labels[k] != v {
				t.Errorf("Asset label '%s=%s' was not preserved when custom labels were added, got '%s'", k, v, crbWithCustom.Labels[k])
			}
		}

		// Custom label should also be present
		if val, ok := crbWithCustom.Labels["test-label"]; !ok || val != "test-value" {
			t.Errorf("Custom label was not added")
		}
	})
}
