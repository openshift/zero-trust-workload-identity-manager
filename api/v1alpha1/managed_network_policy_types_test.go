package v1alpha1

import (
	"os"
	"strings"
	"testing"
)

func TestNetworkPolicyConditionTypes(t *testing.T) {
	if NetworkPoliciesAvailable != "NetworkPoliciesAvailable" {
		t.Fatalf("unexpected NetworkPoliciesAvailable constant: %s", NetworkPoliciesAvailable)
	}
	if NetworkPoliciesApplied != "NetworkPoliciesApplied" {
		t.Fatalf("unexpected NetworkPoliciesApplied constant: %s", NetworkPoliciesApplied)
	}
}

func TestSpireAgentCRD_managedNetworkPolicySchema(t *testing.T) {
	data, err := os.ReadFile("../../config/crd/bases/operator.openshift.io_spireagents.yaml")
	if err != nil {
		t.Fatalf("read CRD: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, "managedNetworkPolicy") {
		t.Fatal("CRD missing managedNetworkPolicy field")
	}
	if !strings.Contains(content, "default: \"true\"") {
		t.Fatal("CRD missing default true for managedNetworkPolicy")
	}
	if !strings.Contains(content, "- \"false\"") || !strings.Contains(content, "- \"true\"") {
		t.Fatal("CRD missing managedNetworkPolicy enum values")
	}
	if !strings.Contains(content, "networkPolicy") {
		t.Fatal("CRD missing status.networkPolicy")
	}
}

func TestSpireServerCRD_networkPolicyStatusSchema(t *testing.T) {
	data, err := os.ReadFile("../../config/crd/bases/operator.openshift.io_spireservers.yaml")
	if err != nil {
		t.Fatalf("read CRD: %v", err)
	}
	if !strings.Contains(string(data), "managementState") {
		t.Fatal("spire server CRD missing networkPolicy.managementState")
	}
}
