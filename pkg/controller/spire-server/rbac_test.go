package spire_server

import (
	"testing"

	"github.com/openshift/zero-trust-workload-identity-manager/pkg/controller/utils"
)

func TestGetSpireServerClusterRole(t *testing.T) {
	cr := getSpireServerClusterRole(nil)

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
	crb := getSpireServerClusterRoleBinding(nil)

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
	role := getSpireBundleRole(nil)

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
	rb := getSpireBundleRoleBinding(nil)

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

func TestGetSpireControllerManagerClusterRole(t *testing.T) {
	cr := getSpireControllerManagerClusterRole(nil)

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
	crb := getSpireControllerManagerClusterRoleBinding(nil)

	if crb == nil {
		t.Fatal("Expected ClusterRoleBinding, got nil")
	}

	if crb.Name != "spire-controller-manager" {
		t.Errorf("Expected ClusterRoleBinding name 'spire-controller-manager', got '%s'", crb.Name)
	}
}

func TestGetSpireControllerManagerLeaderElectionRole(t *testing.T) {
	role := getSpireControllerManagerLeaderElectionRole(nil)

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
	rb := getSpireControllerManagerLeaderElectionRoleBinding(nil)

	if rb == nil {
		t.Fatal("Expected RoleBinding, got nil")
	}

	if rb.Name != "spire-controller-manager-leader-election" {
		t.Errorf("Expected RoleBinding name 'spire-controller-manager-leader-election', got '%s'", rb.Name)
	}
}

// Comprehensive label preservation tests

func TestGetSpireServerClusterRole_LabelPreservation(t *testing.T) {
	t.Run("with custom labels", func(t *testing.T) {
		customLabels := map[string]string{
			"team":   "platform",
			"region": "us-west",
		}
		cr := getSpireServerClusterRole(customLabels)

		// Check custom labels
		if val, ok := cr.Labels["team"]; !ok || val != "platform" {
			t.Errorf("Expected custom label 'team=platform'")
		}

		// Check standard labels still present
		if val, ok := cr.Labels[utils.AppManagedByLabelKey]; !ok || val != utils.AppManagedByLabelValue {
			t.Errorf("Expected standard label to be preserved")
		}
	})

	t.Run("preserves all asset labels", func(t *testing.T) {
		crWithoutCustom := getSpireServerClusterRole(nil)
		assetLabels := make(map[string]string)
		for k, v := range crWithoutCustom.Labels {
			assetLabels[k] = v
		}

		customLabels := map[string]string{"test": "value"}
		crWithCustom := getSpireServerClusterRole(customLabels)

		for k, v := range assetLabels {
			if crWithCustom.Labels[k] != v {
				t.Errorf("Asset label '%s=%s' was not preserved", k, v)
			}
		}
	})
}

func TestGetSpireBundleRole_LabelPreservation(t *testing.T) {
	t.Run("with custom labels", func(t *testing.T) {
		customLabels := map[string]string{
			"bundle-type": "ca-certificates",
		}
		role := getSpireBundleRole(customLabels)

		if val, ok := role.Labels["bundle-type"]; !ok || val != "ca-certificates" {
			t.Errorf("Expected custom label 'bundle-type=ca-certificates'")
		}

		if val, ok := role.Labels[utils.AppManagedByLabelKey]; !ok || val != utils.AppManagedByLabelValue {
			t.Errorf("Expected standard label to be preserved")
		}
	})

	t.Run("preserves all asset labels", func(t *testing.T) {
		roleWithoutCustom := getSpireBundleRole(nil)
		assetLabels := make(map[string]string)
		for k, v := range roleWithoutCustom.Labels {
			assetLabels[k] = v
		}

		customLabels := map[string]string{"test": "value"}
		roleWithCustom := getSpireBundleRole(customLabels)

		for k, v := range assetLabels {
			if roleWithCustom.Labels[k] != v {
				t.Errorf("Asset label '%s=%s' was not preserved", k, v)
			}
		}
	})
}

func TestGetSpireControllerManagerClusterRole_LabelPreservation(t *testing.T) {
	t.Run("with custom labels", func(t *testing.T) {
		customLabels := map[string]string{
			"controller": "spire-manager",
		}
		cr := getSpireControllerManagerClusterRole(customLabels)

		if val, ok := cr.Labels["controller"]; !ok || val != "spire-manager" {
			t.Errorf("Expected custom label 'controller=spire-manager'")
		}

		if val, ok := cr.Labels[utils.AppManagedByLabelKey]; !ok || val != utils.AppManagedByLabelValue {
			t.Errorf("Expected standard label to be preserved")
		}
	})

	t.Run("preserves all asset labels", func(t *testing.T) {
		crWithoutCustom := getSpireControllerManagerClusterRole(nil)
		assetLabels := make(map[string]string)
		for k, v := range crWithoutCustom.Labels {
			assetLabels[k] = v
		}

		customLabels := map[string]string{"test": "value"}
		crWithCustom := getSpireControllerManagerClusterRole(customLabels)

		for k, v := range assetLabels {
			if crWithCustom.Labels[k] != v {
				t.Errorf("Asset label '%s=%s' was not preserved", k, v)
			}
		}
	})
}
