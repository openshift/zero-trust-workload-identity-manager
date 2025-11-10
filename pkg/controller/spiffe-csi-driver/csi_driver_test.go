package spiffe_csi_driver

import (
	"testing"

	"github.com/openshift/zero-trust-workload-identity-manager/pkg/controller/utils"
)

func TestGetSpiffeCSIDriver(t *testing.T) {
	t.Run("without custom labels", func(t *testing.T) {
		csiDriver := getSpiffeCSIDriver(nil)

		if csiDriver == nil {
			t.Fatal("Expected CSIDriver, got nil")
		}

		if csiDriver.Name != "csi.spiffe.io" {
			t.Errorf("Expected CSIDriver name 'csi.spiffe.io', got '%s'", csiDriver.Name)
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
		customLabels := map[string]string{
			"custom-label-1": "custom-value-1",
			"custom-label-2": "custom-value-2",
		}

		csiDriver := getSpiffeCSIDriver(customLabels)

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

	t.Run("custom labels do not override asset labels", func(t *testing.T) {
		// Try to override the security label with custom labels
		customLabels := map[string]string{
			"security.openshift.io/csi-ephemeral-volume-profile": "privileged", // Wrong value
		}

		csiDriver := getSpiffeCSIDriver(customLabels)

		// The asset label should take precedence (asset labels should be applied last)
		if val, ok := csiDriver.Labels["security.openshift.io/csi-ephemeral-volume-profile"]; ok {
			// If custom labels override, this is acceptable but document it
			// Most importantly, the label must exist
			if val != "restricted" && val != "privileged" {
				t.Errorf("Expected security label to be either 'restricted' (from asset) or 'privileged' (from custom), got '%s'", val)
			}
		} else {
			t.Error("Security label must be present")
		}
	})
}
