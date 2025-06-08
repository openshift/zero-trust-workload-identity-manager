package oidcdiscovery

import (
	spiffev1alpha1 "github.com/spiffe/spire-controller-manager/api/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func generateSpireIODCDiscoveryProviderSpiffeID() *spiffev1alpha1.ClusterSPIFFEID {
	clusterSpiffeID := &spiffev1alpha1.ClusterSPIFFEID{
		ObjectMeta: metav1.ObjectMeta{
			Name: "zero-trust-workload-identity-manager-spire-oidc-discovery-provider",
		},
		Spec: spiffev1alpha1.ClusterSPIFFEIDSpec{
			ClassName:        "zero-trust-workload-identity-manager-spire",
			Hint:             "oidc-discovery-provider",
			SPIFFEIDTemplate: "spiffe://{{ .TrustDomain }}/ns/{{ .PodMeta.Namespace }}/sa/{{ .PodSpec.ServiceAccountName }}",
			DNSNameTemplates: []string{
				"oidc-discovery.{{ .TrustDomain }}",
			},
			AutoPopulateDNSNames: true,
			PodSelector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"component":         "oidc-discovery-provider",
					"release":           "spire",
					"release-namespace": "zero-trust-workload-identity-manager",
				},
			},
			NamespaceSelector: &metav1.LabelSelector{
				MatchExpressions: []metav1.LabelSelectorRequirement{
					{
						Key:      "kubernetes.io/metadata.name",
						Operator: metav1.LabelSelectorOpIn,
						Values: []string{
							"zero-trust-workload-identity-manager",
						},
					},
				},
			},
		},
	}
	return clusterSpiffeID
}

func generateDefaultFallbackClusterSPIFFEID() *spiffev1alpha1.ClusterSPIFFEID {
	clusterSpiffeID := &spiffev1alpha1.ClusterSPIFFEID{
		ObjectMeta: metav1.ObjectMeta{
			Name: "zero-trust-workload-identity-manager-spire-default",
		},
		Spec: spiffev1alpha1.ClusterSPIFFEIDSpec{
			ClassName:        "zero-trust-workload-identity-manager-spire",
			Hint:             "default",
			SPIFFEIDTemplate: "spiffe://{{ .TrustDomain }}/ns/{{ .PodMeta.Namespace }}/sa/{{ .PodSpec.ServiceAccountName }}",
			Fallback:         true,
			NamespaceSelector: &metav1.LabelSelector{
				MatchExpressions: []metav1.LabelSelectorRequirement{
					{
						Key:      "kubernetes.io/metadata.name",
						Operator: metav1.LabelSelectorOpNotIn,
						Values: []string{
							"zero-trust-workload-identity-manager",
						},
					},
				},
			},
		},
	}
	return clusterSpiffeID
}
