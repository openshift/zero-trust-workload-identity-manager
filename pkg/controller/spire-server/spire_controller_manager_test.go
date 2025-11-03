package spire_server

import (
	"testing"

	"github.com/openshift/zero-trust-workload-identity-manager/pkg/controller/utils"
)

func TestGetSpireControllerManagerClusterRole(t *testing.T) {
	cr := getSpireControllerManagerClusterRole()

	if cr == nil {
		t.Fatal("Expected ClusterRole, got nil")
	}

	if cr.Name != "spire-controller-manager" {
		t.Errorf("Expected ClusterRole name 'spire-controller-manager', got '%s'", cr.Name)
	}

	// Check labels
	if val, ok := cr.Labels[utils.AppManagedByLabelKey]; !ok || val != utils.AppManagedByLabelValue {
		t.Errorf("Expected label %s=%s", utils.AppManagedByLabelKey, utils.AppManagedByLabelValue)
	}

	if val, ok := cr.Labels["app.kubernetes.io/component"]; !ok || val != utils.ComponentControlPlane {
		t.Errorf("Expected label app.kubernetes.io/component=%s", utils.ComponentControlPlane)
	}
}

func TestGetSpireControllerManagerClusterRoleBinding(t *testing.T) {
	crb := getSpireControllerManagerClusterRoleBinding()

	if crb == nil {
		t.Fatal("Expected ClusterRoleBinding, got nil")
	}

	if crb.Name != "spire-controller-manager" {
		t.Errorf("Expected ClusterRoleBinding name 'spire-controller-manager', got '%s'", crb.Name)
	}
}

func TestGetSpireControllerManagerLeaderElectionRole(t *testing.T) {
	role := getSpireControllerManagerLeaderElectionRole()

	if role == nil {
		t.Fatal("Expected Role, got nil")
	}

	if role.Name != "spire-controller-manager-leader-election" {
		t.Errorf("Expected Role name 'spire-controller-manager-leader-election', got '%s'", role.Name)
	}

	if role.Namespace != utils.OperatorNamespace {
		t.Errorf("Expected Role namespace '%s', got '%s'", utils.OperatorNamespace, role.Namespace)
	}
}

func TestGetSpireControllerManagerLeaderElectionRoleBinding(t *testing.T) {
	rb := getSpireControllerManagerLeaderElectionRoleBinding()

	if rb == nil {
		t.Fatal("Expected RoleBinding, got nil")
	}

	if rb.Name != "spire-controller-manager-leader-election" {
		t.Errorf("Expected RoleBinding name 'spire-controller-manager-leader-election', got '%s'", rb.Name)
	}
}

func TestGetSpireControllerManagerWebhookService(t *testing.T) {
	svc := getSpireControllerManagerWebhookService()

	if svc == nil {
		t.Fatal("Expected Service, got nil")
	}

	if svc.Name != "spire-controller-manager-webhook" {
		t.Errorf("Expected Service name 'spire-controller-manager-webhook', got '%s'", svc.Name)
	}

	// Check selectors
	if val, ok := svc.Spec.Selector["app.kubernetes.io/name"]; !ok || val != "spire-controller-manager" {
		t.Error("Expected selector app.kubernetes.io/name=spire-controller-manager")
	}
}

func TestGetSpireControllerManagerValidatingWebhookConfiguration(t *testing.T) {
	webhook := getSpireControllerManagerValidatingWebhookConfiguration()

	if webhook == nil {
		t.Fatal("Expected ValidatingWebhookConfiguration, got nil")
	}

	if webhook.Name != "spire-controller-manager-webhook" {
		t.Errorf("Expected ValidatingWebhookConfiguration name 'spire-controller-manager-webhook', got '%s'", webhook.Name)
	}

	// Check labels
	if val, ok := webhook.Labels[utils.AppManagedByLabelKey]; !ok || val != utils.AppManagedByLabelValue {
		t.Errorf("Expected label %s=%s", utils.AppManagedByLabelKey, utils.AppManagedByLabelValue)
	}
}
