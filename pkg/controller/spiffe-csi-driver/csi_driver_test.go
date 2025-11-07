package spiffe_csi_driver

import (
	"testing"

	"github.com/openshift/zero-trust-workload-identity-manager/pkg/controller/utils"
)

func TestGetSpiffeCSIDriver(t *testing.T) {
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

	// Check for required labels
	if val, ok := csiDriver.Labels[utils.AppManagedByLabelKey]; !ok || val != utils.AppManagedByLabelValue {
		t.Errorf("Expected label %s=%s", utils.AppManagedByLabelKey, utils.AppManagedByLabelValue)
	}

	if val, ok := csiDriver.Labels["app.kubernetes.io/component"]; !ok || val != utils.ComponentCSI {
		t.Errorf("Expected label app.kubernetes.io/component=%s", utils.ComponentCSI)
	}
}
