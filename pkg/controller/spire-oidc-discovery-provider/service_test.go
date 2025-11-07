package spire_oidc_discovery_provider

import (
	"testing"

	"github.com/openshift/zero-trust-workload-identity-manager/pkg/controller/utils"
)

func TestGetSpireOIDCDiscoveryProviderService(t *testing.T) {
	svc := getSpireOIDCDiscoveryProviderService(nil)

	if svc == nil {
		t.Fatal("Expected Service, got nil")
	}

	if svc.Name != "spire-spiffe-oidc-discovery-provider" {
		t.Errorf("Expected Service name 'spire-spiffe-oidc-discovery-provider', got '%s'", svc.Name)
	}

	if svc.Namespace != utils.OperatorNamespace {
		t.Errorf("Expected Service namespace '%s', got '%s'", utils.OperatorNamespace, svc.Namespace)
	}

	// Check labels
	if len(svc.Labels) == 0 {
		t.Error("Expected Service to have labels, got none")
	}

	// Check for required labels
	if val, ok := svc.Labels[utils.AppManagedByLabelKey]; !ok || val != utils.AppManagedByLabelValue {
		t.Errorf("Expected label %s=%s", utils.AppManagedByLabelKey, utils.AppManagedByLabelValue)
	}

	if val, ok := svc.Labels["app.kubernetes.io/component"]; !ok || val != utils.ComponentDiscovery {
		t.Errorf("Expected label app.kubernetes.io/component=%s", utils.ComponentDiscovery)
	}

	// Check selectors
	if len(svc.Spec.Selector) == 0 {
		t.Error("Expected Service to have selectors, got none")
	}

	if val, ok := svc.Spec.Selector["app.kubernetes.io/name"]; !ok || val != "spiffe-oidc-discovery-provider" {
		t.Error("Expected selector app.kubernetes.io/name=spiffe-oidc-discovery-provider")
	}

	if val, ok := svc.Spec.Selector["app.kubernetes.io/instance"]; !ok || val != utils.StandardInstance {
		t.Errorf("Expected selector app.kubernetes.io/instance=%s", utils.StandardInstance)
	}
}
