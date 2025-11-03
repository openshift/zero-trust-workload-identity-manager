package spire_server

import (
	"testing"

	"github.com/openshift/zero-trust-workload-identity-manager/pkg/controller/utils"
)

func TestGetSpireServerServiceAccount(t *testing.T) {
	sa := getSpireServerServiceAccount()

	if sa == nil {
		t.Fatal("Expected ServiceAccount, got nil")
	}

	if sa.Name != "spire-server" {
		t.Errorf("Expected ServiceAccount name 'spire-server', got '%s'", sa.Name)
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

	if val, ok := sa.Labels["app.kubernetes.io/component"]; !ok || val != utils.ComponentControlPlane {
		t.Errorf("Expected label app.kubernetes.io/component=%s", utils.ComponentControlPlane)
	}
}
