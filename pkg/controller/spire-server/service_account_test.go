package spire_server

import (
	"testing"

	"github.com/openshift/zero-trust-workload-identity-manager/pkg/controller/utils"
)

func TestGetSpireServerServiceAccount(t *testing.T) {
	t.Run("without custom labels", func(t *testing.T) {
		sa := getSpireServerServiceAccount(nil)

		if sa == nil {
			t.Fatal("Expected ServiceAccount, got nil")
		}

		if sa.Name != "spire-server" {
			t.Errorf("Expected ServiceAccount name 'spire-server', got '%s'", sa.Name)
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

		if val, ok := sa.Labels["app.kubernetes.io/component"]; !ok || val != utils.ComponentControlPlane {
			t.Errorf("Expected label app.kubernetes.io/component=%s", utils.ComponentControlPlane)
		}
	})

	t.Run("with custom labels", func(t *testing.T) {
		customLabels := map[string]string{
			"project":     "identity-platform",
			"cost-center": "security",
		}

		sa := getSpireServerServiceAccount(customLabels)

		if sa == nil {
			t.Fatal("Expected ServiceAccount, got nil")
		}

		// Check that custom labels are present
		if val, ok := sa.Labels["project"]; !ok || val != "identity-platform" {
			t.Errorf("Expected custom label 'project=identity-platform', got '%s'", val)
		}

		if val, ok := sa.Labels["cost-center"]; !ok || val != "security" {
			t.Errorf("Expected custom label 'cost-center=security', got '%s'", val)
		}

		// Check that standard labels are still present
		if val, ok := sa.Labels[utils.AppManagedByLabelKey]; !ok || val != utils.AppManagedByLabelValue {
			t.Errorf("Expected label %s=%s to be preserved with custom labels", utils.AppManagedByLabelKey, utils.AppManagedByLabelValue)
		}

		if val, ok := sa.Labels["app.kubernetes.io/component"]; !ok || val != utils.ComponentControlPlane {
			t.Errorf("Expected label app.kubernetes.io/component=%s to be preserved with custom labels", utils.ComponentControlPlane)
		}
	})

	t.Run("preserves all asset labels", func(t *testing.T) {
		// Get labels without custom labels (these come from asset file)
		saWithoutCustom := getSpireServerServiceAccount(nil)
		assetLabels := make(map[string]string)
		for k, v := range saWithoutCustom.Labels {
			assetLabels[k] = v
		}

		// Get labels with custom labels
		customLabels := map[string]string{
			"deployment-id": "prod-123",
		}
		saWithCustom := getSpireServerServiceAccount(customLabels)

		// All asset labels should still be present
		for k, v := range assetLabels {
			if saWithCustom.Labels[k] != v {
				t.Errorf("Asset label '%s=%s' was not preserved when custom labels were added, got '%s'", k, v, saWithCustom.Labels[k])
			}
		}

		// Custom label should also be present
		if val, ok := saWithCustom.Labels["deployment-id"]; !ok || val != "prod-123" {
			t.Errorf("Custom label was not added")
		}
	})
}
