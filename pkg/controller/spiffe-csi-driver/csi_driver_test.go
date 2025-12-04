package spiffe_csi_driver

import (
	"testing"

	"github.com/openshift/zero-trust-workload-identity-manager/pkg/controller/utils"
)

func TestGetSpiffeCSIDriver(t *testing.T) {
	t.Run("without custom labels", func(t *testing.T) {
		pluginName := "csi.spiffe.io"
		csiDriver := getSpiffeCSIDriver(pluginName, nil)

		if csiDriver == nil {
			t.Fatal("Expected CSIDriver, got nil")
		}

		if csiDriver.Name != pluginName {
			t.Errorf("Expected CSIDriver name '%s', got '%s'", pluginName, csiDriver.Name)
		}

		// Check labels
		if len(csiDriver.Labels) == 0 {
			t.Error("Expected CSIDriver to have labels, got none")
		}

		// Check for required standard labels
		if val, ok := csiDriver.Labels[utils.AppManagedByLabelKey]; !ok || val != utils.AppManagedByLabelValue {
			t.Errorf("Expected label %s=%s", utils.AppManagedByLabelKey, utils.AppManagedByLabelValue)
		}

		if val, ok := csiDriver.Labels["app.kubernetes.io/component"]; !ok || val != utils.ComponentCSI {
			t.Errorf("Expected label app.kubernetes.io/component=%s", utils.ComponentCSI)
		}

		// CRITICAL: Check for the security label from asset file
		// This label is required for pod security admission to work correctly
		if val, ok := csiDriver.Labels["security.openshift.io/csi-ephemeral-volume-profile"]; !ok || val != "restricted" {
			t.Errorf("Expected security label 'security.openshift.io/csi-ephemeral-volume-profile=restricted', got '%s=%s'", "security.openshift.io/csi-ephemeral-volume-profile", val)
			t.Error("This label MUST be preserved from the asset file for pod security admission to work")
		}
	})

	t.Run("with custom labels", func(t *testing.T) {
		pluginName := "csi.spiffe.io"
		customLabels := map[string]string{
			"custom-label-1": "custom-value-1",
			"custom-label-2": "custom-value-2",
			"security.openshift.io/csi-ephemeral-volume-profile": "privileged",
		}

		csiDriver := getSpiffeCSIDriver(pluginName, customLabels)

		if csiDriver == nil {
			t.Fatal("Expected CSIDriver, got nil")
		}

		// Check that custom labels are present
		if val, ok := csiDriver.Labels["custom-label-1"]; !ok || val != "custom-value-1" {
			t.Errorf("Expected custom label 'custom-label-1=custom-value-1', got '%s'", val)
		}

		if val, ok := csiDriver.Labels["custom-label-2"]; !ok || val != "custom-value-2" {
			t.Errorf("Expected custom label 'custom-label-2=custom-value-2', got '%s'", val)
		}

		// Check that standard labels are still present
		if val, ok := csiDriver.Labels[utils.AppManagedByLabelKey]; !ok || val != utils.AppManagedByLabelValue {
			t.Errorf("Expected label %s=%s", utils.AppManagedByLabelKey, utils.AppManagedByLabelValue)
		}

		// CRITICAL: Check that the security label from asset file is preserved
		if val, ok := csiDriver.Labels["security.openshift.io/csi-ephemeral-volume-profile"]; !ok || val != "restricted" {
			t.Errorf("Expected security label 'security.openshift.io/csi-ephemeral-volume-profile=restricted', got '%s=%s'", "security.openshift.io/csi-ephemeral-volume-profile", val)
			t.Error("This label MUST be preserved from the asset file even when custom labels are provided")
		}
	})

	t.Run("with custom plugin name", func(t *testing.T) {
		pluginName := "csi.custom.io"
		csiDriver := getSpiffeCSIDriver(pluginName, nil)

		if csiDriver == nil {
			t.Fatal("Expected CSIDriver, got nil")
		}

		if csiDriver.Name != pluginName {
			t.Errorf("Expected CSIDriver name '%s', got '%s'", pluginName, csiDriver.Name)
		}

		// Verify standard labels are still present
		if val, ok := csiDriver.Labels[utils.AppManagedByLabelKey]; !ok || val != utils.AppManagedByLabelValue {
			t.Errorf("Expected label %s=%s", utils.AppManagedByLabelKey, utils.AppManagedByLabelValue)
		}
	})
}
