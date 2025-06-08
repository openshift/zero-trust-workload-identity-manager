package oidcdiscovery

import (
	securityv1 "github.com/openshift/api/security/v1"
	"github.com/openshift/zero-trust-workload-identity-manager/api/v1alpha1"
	"github.com/openshift/zero-trust-workload-identity-manager/pkg/controller/utils"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

// generateSpireOIDCDiscoveryProviderSCC returns a SecurityContextConstraints object for spire-oidc-discovery-provider
func generateSpireOIDCDiscoveryProviderSCC(config *v1alpha1.SpireOIDCDiscoveryProviderConfig) *securityv1.SecurityContextConstraints {
	labels := map[string]string{}
	for key, value := range config.Spec.Labels {
		labels[key] = value
	}
	labels[utils.AppManagedByLabelKey] = utils.AppManagedByLabelValue
	return &securityv1.SecurityContextConstraints{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "spire-spiffe-oidc-discovery-provider",
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
			"system:serviceaccount:zero-trust-workload-identity-manager:spire-spiffe-oidc-discovery-provider",
			"system:serviceaccount:zero-trust-workload-identity-manager:spire-spiffe-oidc-discovery-provider-pre-delete",
		},
		Volumes: []securityv1.FSType{
			securityv1.FSTypeConfigMap,
			securityv1.FSTypeCSI,
			securityv1.FSTypeDownwardAPI,
			securityv1.FSTypeEphemeral,
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
		SeccompProfiles:          []string{"*"},
	}
}
