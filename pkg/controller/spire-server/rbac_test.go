package spire_server

import (
	"testing"

	"github.com/openshift/zero-trust-workload-identity-manager/pkg/controller/utils"
)

func TestGetSpireServerClusterRole(t *testing.T) {
	cr := getSpireServerClusterRole()

	if cr == nil {
		t.Fatal("Expected ClusterRole, got nil")
	}

	if cr.Name != "spire-server" {
		t.Errorf("Expected ClusterRole name 'spire-server', got '%s'", cr.Name)
	}

	// Check labels
	if val, ok := cr.Labels[utils.AppManagedByLabelKey]; !ok || val != utils.AppManagedByLabelValue {
		t.Errorf("Expected label %s=%s", utils.AppManagedByLabelKey, utils.AppManagedByLabelValue)
	}

	if val, ok := cr.Labels["app.kubernetes.io/component"]; !ok || val != utils.ComponentControlPlane {
		t.Errorf("Expected label app.kubernetes.io/component=%s", utils.ComponentControlPlane)
	}
}

func TestGetSpireServerClusterRoleBinding(t *testing.T) {
	crb := getSpireServerClusterRoleBinding()

	if crb == nil {
		t.Fatal("Expected ClusterRoleBinding, got nil")
	}

	if crb.Name != "spire-server" {
		t.Errorf("Expected ClusterRoleBinding name 'spire-server', got '%s'", crb.Name)
	}

	// Check labels
	if val, ok := crb.Labels[utils.AppManagedByLabelKey]; !ok || val != utils.AppManagedByLabelValue {
		t.Errorf("Expected label %s=%s", utils.AppManagedByLabelKey, utils.AppManagedByLabelValue)
	}
}

func TestGetSpireBundleRole(t *testing.T) {
	role := getSpireBundleRole()

	if role == nil {
		t.Fatal("Expected Role, got nil")
	}

	if role.Name != "spire-bundle" {
		t.Errorf("Expected Role name 'spire-bundle', got '%s'", role.Name)
	}

	if role.Namespace != utils.OperatorNamespace {
		t.Errorf("Expected Role namespace '%s', got '%s'", utils.OperatorNamespace, role.Namespace)
	}

	// Check labels - bundle resources use spire-server labels
	if val, ok := role.Labels[utils.AppManagedByLabelKey]; !ok || val != utils.AppManagedByLabelValue {
		t.Errorf("Expected label %s=%s", utils.AppManagedByLabelKey, utils.AppManagedByLabelValue)
	}
}

func TestGetSpireBundleRoleBinding(t *testing.T) {
	rb := getSpireBundleRoleBinding()

	if rb == nil {
		t.Fatal("Expected RoleBinding, got nil")
	}

	if rb.Name != "spire-bundle" {
		t.Errorf("Expected RoleBinding name 'spire-bundle', got '%s'", rb.Name)
	}

	if rb.Namespace != utils.OperatorNamespace {
		t.Errorf("Expected RoleBinding namespace '%s', got '%s'", utils.OperatorNamespace, rb.Namespace)
	}
}
