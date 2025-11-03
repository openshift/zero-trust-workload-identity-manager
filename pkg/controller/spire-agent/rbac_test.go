package spire_agent

import (
	"testing"

	"github.com/openshift/zero-trust-workload-identity-manager/pkg/controller/utils"
)

func TestGetSpireAgentClusterRole(t *testing.T) {
	cr := getSpireAgentClusterRole()

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
}

func TestGetSpireAgentClusterRoleBinding(t *testing.T) {
	crb := getSpireAgentClusterRoleBinding()

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
}
