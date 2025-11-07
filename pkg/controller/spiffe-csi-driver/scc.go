package spiffe_csi_driver

import (
	securityv1 "github.com/openshift/api/security/v1"
	"github.com/openshift/zero-trust-workload-identity-manager/pkg/controller/utils"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

// generateSpiffeCSIDriverSCC returns a pointer to the desired SCC object
func generateSpiffeCSIDriverSCC(customLabels map[string]string) *securityv1.SecurityContextConstraints {
	return &securityv1.SecurityContextConstraints{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "spire-spiffe-csi-driver",
			Labels: utils.SpiffeCSIDriverLabels(customLabels),
		},
		ReadOnlyRootFilesystem: true,
		RunAsUser: securityv1.RunAsUserStrategyOptions{
			Type: securityv1.RunAsUserStrategyMustRunAsRange,
		},
		SELinuxContext: securityv1.SELinuxContextStrategyOptions{
			Type: securityv1.SELinuxStrategyMustRunAs,
		},
		SupplementalGroups: securityv1.SupplementalGroupsStrategyOptions{
			Type: securityv1.SupplementalGroupsStrategyMustRunAs,
		},
		FSGroup: securityv1.FSGroupStrategyOptions{
			Type: securityv1.FSGroupStrategyMustRunAs,
		},
		Users: []string{
			"system:serviceaccount:zero-trust-workload-identity-manager:spire-spiffe-csi-driver",
		},
		Volumes: []securityv1.FSType{
			securityv1.FSTypeConfigMap,
			securityv1.FSTypeHostPath,
			securityv1.FSTypeSecret,
		},
		AllowHostDirVolumePlugin: true,
		AllowHostIPC:             false,
		AllowHostNetwork:         false,
		AllowHostPID:             false,
		AllowHostPorts:           false,
		AllowPrivilegeEscalation: ptr.To(true),
		AllowPrivilegedContainer: true,
		DefaultAddCapabilities:   []corev1.Capability{},
		RequiredDropCapabilities: []corev1.Capability{
			"ALL",
		},
	}
}
