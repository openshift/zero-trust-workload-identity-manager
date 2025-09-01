package static_resource_controller

import (
	"github.com/openshift/zero-trust-workload-identity-manager/pkg/version"
	"github.com/stretchr/testify/assert"
	"testing"
)

var requiredAgentResourceLabels = map[string]string{
	"app.kubernetes.io/name":       "spire-agent",
	"app.kubernetes.io/instance":   "cluster-zero-trust-workload-identity-manager",
	"app.kubernetes.io/component":  "node-agent",
	"app.kubernetes.io/version":    version.SpireAgentVersion,
	"app.kubernetes.io/managed-by": "zero-trust-workload-identity-manager",
	"app.kubernetes.io/part-of":    "zero-trust-workload-identity-manager",
}

var requiredServerResourceLabels = map[string]string{
	"app.kubernetes.io/name":       "spire-server",
	"app.kubernetes.io/instance":   "cluster-zero-trust-workload-identity-manager",
	"app.kubernetes.io/component":  "control-plane",
	"app.kubernetes.io/version":    version.SpireServerVersion,
	"app.kubernetes.io/managed-by": "zero-trust-workload-identity-manager",
	"app.kubernetes.io/part-of":    "zero-trust-workload-identity-manager",
}

var requiredControllerManagerResourceLabels = map[string]string{
	"app.kubernetes.io/name":       "spire-controller-manager",
	"app.kubernetes.io/instance":   "cluster-zero-trust-workload-identity-manager",
	"app.kubernetes.io/component":  "control-plane",
	"app.kubernetes.io/version":    version.SpireControllerManagerVersion,
	"app.kubernetes.io/managed-by": "zero-trust-workload-identity-manager",
	"app.kubernetes.io/part-of":    "zero-trust-workload-identity-manager",
}

var requiredOIDCResourceLabels = map[string]string{
	"app.kubernetes.io/name":       "spiffe-oidc-discovery-provider",
	"app.kubernetes.io/instance":   "cluster-zero-trust-workload-identity-manager",
	"app.kubernetes.io/component":  "discovery",
	"app.kubernetes.io/version":    version.SpireOIDCDiscoveryProviderVersion,
	"app.kubernetes.io/managed-by": "zero-trust-workload-identity-manager",
	"app.kubernetes.io/part-of":    "zero-trust-workload-identity-manager",
}

func TestSpireAgentClusterRole(t *testing.T) {
	r := &StaticResourceReconciler{}
	cr := r.getSpireAgentClusterRole()

	assert.Equal(t, "spire-agent", cr.Name)
	assert.Equal(t, "ClusterRole", cr.Kind)

	expectedLabels := requiredAgentResourceLabels
	assert.Equal(t, expectedLabels, cr.Labels)

	assert.Len(t, cr.Rules, 1)
	assert.ElementsMatch(t, cr.Rules[0].Resources, []string{"pods", "nodes", "nodes/proxy"})
	assert.ElementsMatch(t, cr.Rules[0].Verbs, []string{"get"})
}

func TestSpireAgentClusterRoleBinding(t *testing.T) {
	r := &StaticResourceReconciler{}
	crb := r.getSpireAgentClusterRoleBinding()

	assert.Equal(t, "spire-agent", crb.Name)
	assert.Equal(t, "ClusterRoleBinding", crb.Kind)

	expectedLabels := requiredAgentResourceLabels
	assert.Equal(t, expectedLabels, crb.Labels)

	assert.Equal(t, 1, len(crb.Subjects))
	assert.Equal(t, "ServiceAccount", crb.Subjects[0].Kind)
	assert.Equal(t, "spire-agent", crb.Subjects[0].Name)
	assert.Equal(t, "zero-trust-workload-identity-manager", crb.Subjects[0].Namespace)

	assert.Equal(t, "ClusterRole", crb.RoleRef.Kind)
	assert.Equal(t, "spire-agent", crb.RoleRef.Name)
	assert.Equal(t, "rbac.authorization.k8s.io", crb.RoleRef.APIGroup)
}

func TestSpireBundleRole(t *testing.T) {
	r := &StaticResourceReconciler{}
	role := r.getSpireBundleRole()

	assert.Equal(t, "spire-bundle", role.Name)
	assert.Equal(t, "Role", role.Kind)
	assert.Equal(t, "zero-trust-workload-identity-manager", role.Namespace)

	expectedLabels := requiredServerResourceLabels
	assert.Equal(t, expectedLabels, role.Labels)

	assert.Len(t, role.Rules, 1)
	assert.ElementsMatch(t, role.Rules[0].Resources, []string{"configmaps"})
	assert.ElementsMatch(t, role.Rules[0].ResourceNames, []string{"spire-bundle"})
	assert.ElementsMatch(t, role.Rules[0].Verbs, []string{"get", "patch"})
}

func TestSpireBundleRoleBinding(t *testing.T) {
	r := &StaticResourceReconciler{}
	rb := r.getSpireBundleRoleBinding()

	assert.Equal(t, "spire-bundle", rb.Name)
	assert.Equal(t, "RoleBinding", rb.Kind)
	assert.Equal(t, "zero-trust-workload-identity-manager", rb.Namespace)

	expectedLabels := requiredServerResourceLabels
	assert.Equal(t, expectedLabels, rb.Labels)

	assert.Equal(t, 1, len(rb.Subjects))
	assert.Equal(t, "ServiceAccount", rb.Subjects[0].Kind)
	assert.Equal(t, "spire-server", rb.Subjects[0].Name)
	assert.Equal(t, "zero-trust-workload-identity-manager", rb.Subjects[0].Namespace)

	assert.Equal(t, "Role", rb.RoleRef.Kind)
	assert.Equal(t, "spire-bundle", rb.RoleRef.Name)
	assert.Equal(t, "rbac.authorization.k8s.io", rb.RoleRef.APIGroup)
}

func TestSpireControllerManagerClusterRole(t *testing.T) {
	r := &StaticResourceReconciler{}
	cr := r.getSpireControllerManagerClusterRole()

	assert.Equal(t, "spire-controller-manager", cr.Name)
	assert.Equal(t, "ClusterRole", cr.Kind)

	expectedLabels := requiredControllerManagerResourceLabels
	assert.Equal(t, expectedLabels, cr.Labels)

	assert.True(t, len(cr.Rules) > 0)
}

func TestSpireControllerManagerClusterRoleBinding(t *testing.T) {
	r := &StaticResourceReconciler{}
	crb := r.getSpireControllerManagerClusterRoleBinding()

	assert.Equal(t, "spire-controller-manager", crb.Name)
	assert.Equal(t, "ClusterRoleBinding", crb.Kind)

	expectedLabels := requiredControllerManagerResourceLabels
	assert.Equal(t, expectedLabels, crb.Labels)

	assert.Equal(t, "ClusterRole", crb.RoleRef.Kind)
	assert.Equal(t, "spire-controller-manager", crb.RoleRef.Name)
	assert.Equal(t, "rbac.authorization.k8s.io", crb.RoleRef.APIGroup)
}

func TestSpireControllerManagerLeaderElectionRole(t *testing.T) {
	r := &StaticResourceReconciler{}
	role := r.getSpireControllerManagerLeaderElectionRole()

	assert.Equal(t, "spire-controller-manager-leader-election", role.Name)
	assert.Equal(t, "Role", role.Kind)

	expectedLabels := requiredControllerManagerResourceLabels
	assert.Equal(t, expectedLabels, role.Labels)

	assert.NotEmpty(t, role.Rules)
}

func TestSpireControllerManagerLeaderElectionRoleBinding(t *testing.T) {
	r := &StaticResourceReconciler{}
	rb := r.getSpireControllerManagerLeaderElectionRoleBinding()

	assert.Equal(t, "spire-controller-manager-leader-election", rb.Name)
	assert.Equal(t, "RoleBinding", rb.Kind)

	expectedLabels := requiredControllerManagerResourceLabels
	assert.Equal(t, expectedLabels, rb.Labels)

	assert.Equal(t, "Role", rb.RoleRef.Kind)
	assert.Equal(t, "spire-controller-manager-leader-election", rb.RoleRef.Name)
	assert.Equal(t, "rbac.authorization.k8s.io", rb.RoleRef.APIGroup)
}

func TestSpireServerClusterRole(t *testing.T) {
	r := &StaticResourceReconciler{}
	cr := r.getSpireServerClusterRole()

	assert.Equal(t, "spire-server", cr.Name)
	assert.Equal(t, "ClusterRole", cr.Kind)

	expectedLabels := requiredServerResourceLabels
	assert.Equal(t, expectedLabels, cr.Labels)

	assert.NotEmpty(t, cr.Rules)
}

func TestSpireServerClusterRoleBinding(t *testing.T) {
	r := &StaticResourceReconciler{}
	crb := r.getSpireServerClusterRoleBinding()

	assert.Equal(t, "spire-server", crb.Name)
	assert.Equal(t, "ClusterRoleBinding", crb.Kind)

	expectedLabels := requiredServerResourceLabels
	assert.Equal(t, expectedLabels, crb.Labels)

	assert.Equal(t, "ClusterRole", crb.RoleRef.Kind)
	assert.Equal(t, "spire-server", crb.RoleRef.Name)
	assert.Equal(t, "rbac.authorization.k8s.io", crb.RoleRef.APIGroup)
}

func TestStaticResourceReconciler_ListStaticResources(t *testing.T) {
	r := &StaticResourceReconciler{}

	t.Run("listStaticClusterRoles", func(t *testing.T) {
		clusterRoles := r.listStaticClusterRoles()
		assert.Len(t, clusterRoles, 3)

		expectedNames := []string{
			"spire-agent",
			"spire-server",
			"spire-controller-manager",
		}
		for i, cr := range clusterRoles {
			assert.Equal(t, expectedNames[i], cr.Name)
			assert.Equal(t, "ClusterRole", cr.Kind)
			assert.NotEmpty(t, cr.Labels)
		}
	})

	t.Run("listStaticClusterRoleBindings", func(t *testing.T) {
		clusterRoleBindings := r.listStaticClusterRoleBindings()
		assert.Len(t, clusterRoleBindings, 3)

		expectedNames := []string{
			"spire-agent",
			"spire-server",
			"spire-controller-manager",
		}
		for i, crb := range clusterRoleBindings {
			assert.Equal(t, expectedNames[i], crb.Name)
			assert.Equal(t, "ClusterRoleBinding", crb.Kind)
			assert.NotEmpty(t, crb.Labels)
		}
	})

	t.Run("listStaticRoles", func(t *testing.T) {
		roles := r.listStaticRoles()
		assert.Len(t, roles, 2)

		expectedNames := []string{
			"spire-bundle",
			"spire-controller-manager-leader-election",
		}
		for i, role := range roles {
			assert.Equal(t, expectedNames[i], role.Name)
			assert.Equal(t, "Role", role.Kind)
			assert.NotEmpty(t, role.Labels)
		}
	})

	t.Run("listStaticRoleBindings", func(t *testing.T) {
		roleBindings := r.listStaticRoleBindings()
		assert.Len(t, roleBindings, 2)

		expectedNames := []string{
			"spire-bundle",
			"spire-controller-manager-leader-election",
		}
		for i, rb := range roleBindings {
			assert.Equal(t, expectedNames[i], rb.Name)
			assert.Equal(t, "RoleBinding", rb.Kind)
			assert.NotEmpty(t, rb.Labels)
		}
	})
}
