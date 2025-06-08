package agent

import (
	"reflect"
	"testing"

	securityv1 "github.com/openshift/api/security/v1"
	"github.com/openshift/zero-trust-workload-identity-manager/api/v1alpha1"
	"github.com/openshift/zero-trust-workload-identity-manager/pkg/controller/utils"
)

func TestGenerateSpireAgentSCC(t *testing.T) {
	customLabels := map[string]string{
		"custom-label": "custom-value",
	}
	config := &v1alpha1.SpireAgentConfig{
		Spec: v1alpha1.SpireAgentConfigSpec{
			CommonConfig: v1alpha1.CommonConfig{
				Labels: customLabels,
			},
		},
	}

	scc := generateSpireAgentSCC(config)

	expectedLabels := map[string]string{
		"app":                      "spire-agent",
		utils.AppManagedByLabelKey: utils.AppManagedByLabelValue,
		"custom-label":             "custom-value",
	}

	if scc.Name != "spire-agent" {
		t.Errorf("expected SCC name to be 'spire-agent', got %s", scc.Name)
	}

	if !reflect.DeepEqual(scc.Labels, expectedLabels) {
		t.Errorf("expected labels %v, got %v", expectedLabels, scc.Labels)
	}

	if !scc.ReadOnlyRootFilesystem {
		t.Errorf("expected ReadOnlyRootFilesystem to be true")
	}

	if scc.RunAsUser.Type != securityv1.RunAsUserStrategyRunAsAny {
		t.Errorf("expected RunAsUser.Type to be RunAsAny")
	}

	if scc.SELinuxContext.Type != securityv1.SELinuxStrategyRunAsAny {
		t.Errorf("expected SELinuxContext.Type to be RunAsAny")
	}

	if scc.SupplementalGroups.Type != securityv1.SupplementalGroupsStrategyRunAsAny {
		t.Errorf("expected SupplementalGroups.Type to be RunAsAny")
	}

	if scc.FSGroup.Type != securityv1.FSGroupStrategyRunAsAny {
		t.Errorf("expected FSGroup.Type to be RunAsAny")
	}

	expectedUser := "system:serviceaccount:zero-trust-workload-identity-manager:spire-agent"
	if len(scc.Users) != 1 || scc.Users[0] != expectedUser {
		t.Errorf("expected Users to contain %s, got %v", expectedUser, scc.Users)
	}

	expectedVolumes := []securityv1.FSType{
		securityv1.FSTypeConfigMap,
		securityv1.FSTypeHostPath,
		securityv1.FSProjected,
		securityv1.FSTypeSecret,
		securityv1.FSTypeEmptyDir,
	}
	if !reflect.DeepEqual(scc.Volumes, expectedVolumes) {
		t.Errorf("expected Volumes %v, got %v", expectedVolumes, scc.Volumes)
	}

	if !scc.AllowHostDirVolumePlugin {
		t.Errorf("expected AllowHostDirVolumePlugin to be true")
	}
	if !scc.AllowHostIPC {
		t.Errorf("expected AllowHostIPC to be true")
	}
	if !scc.AllowHostNetwork {
		t.Errorf("expected AllowHostNetwork to be true")
	}
	if !scc.AllowHostPID {
		t.Errorf("expected AllowHostPID to be true")
	}
	if !scc.AllowHostPorts {
		t.Errorf("expected AllowHostPorts to be true")
	}
	if scc.AllowPrivilegeEscalation == nil || !*scc.AllowPrivilegeEscalation {
		t.Errorf("expected AllowPrivilegeEscalation to be true")
	}
	if !scc.AllowPrivilegedContainer {
		t.Errorf("expected AllowPrivilegedContainer to be true")
	}

	if len(scc.AllowedCapabilities) != 0 {
		t.Errorf("expected AllowedCapabilities to be empty")
	}
	if len(scc.DefaultAddCapabilities) != 0 {
		t.Errorf("expected DefaultAddCapabilities to be empty")
	}
	if len(scc.RequiredDropCapabilities) != 0 {
		t.Errorf("expected RequiredDropCapabilities to be empty")
	}
	if len(scc.Groups) != 0 {
		t.Errorf("expected Groups to be empty")
	}
}
