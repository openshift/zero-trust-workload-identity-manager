package agent

import (
	"github.com/openshift/zero-trust-workload-identity-manager/api/v1alpha1"
	"github.com/openshift/zero-trust-workload-identity-manager/pkg/controller/utils"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	securityv1 "github.com/openshift/api/security/v1"
)

// generateSpireAgentSCC returns a SecurityContextConstraints object for spire-agent
func generateSpireAgentSCC(config *v1alpha1.SpireAgentConfig) *securityv1.SecurityContextConstraints {
	labels := map[string]string{
		"app":                      "spire-agent",
		utils.AppManagedByLabelKey: utils.AppManagedByLabelValue,
	}
	for key, value := range config.Spec.Labels {
		labels[key] = value
	}
	return &securityv1.SecurityContextConstraints{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "spire-agent",
			Labels: labels,
		},
		ReadOnlyRootFilesystem: true,
		RunAsUser: securityv1.RunAsUserStrategyOptions{
			Type: securityv1.RunAsUserStrategyRunAsAny,
		},
		SELinuxContext: securityv1.SELinuxContextStrategyOptions{
			Type: securityv1.SELinuxStrategyRunAsAny,
		},
		SupplementalGroups: securityv1.SupplementalGroupsStrategyOptions{
			Type: securityv1.SupplementalGroupsStrategyRunAsAny,
		},
		FSGroup: securityv1.FSGroupStrategyOptions{
			Type: securityv1.FSGroupStrategyRunAsAny,
		},
		Users: []string{
			"system:serviceaccount:zero-trust-workload-identity-manager:spire-agent",
		},
		Volumes: []securityv1.FSType{
			securityv1.FSTypeConfigMap,
			securityv1.FSTypeHostPath,
			securityv1.FSProjected,
			securityv1.FSTypeSecret,
			securityv1.FSTypeEmptyDir,
		},
		AllowHostDirVolumePlugin: true,
		AllowHostIPC:             true,
		AllowHostNetwork:         true,
		AllowHostPID:             true,
		AllowHostPorts:           true,
		AllowPrivilegeEscalation: ptr.To(true),
		AllowPrivilegedContainer: true,
		AllowedCapabilities:      []corev1.Capability{},
		DefaultAddCapabilities:   []corev1.Capability{},
		RequiredDropCapabilities: []corev1.Capability{},
		Groups:                   []string{},
	}
}
