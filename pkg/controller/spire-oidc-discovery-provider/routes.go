package spire_oidc_discovery_provider

import (
	routev1 "github.com/openshift/api/route/v1"
	"github.com/openshift/zero-trust-workload-identity-manager/api/v1alpha1"
	"github.com/openshift/zero-trust-workload-identity-manager/pkg/controller/utils"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

// generateOIDCDiscoveryProviderRoute creates an OpenShift Route resource for the SPIRE OIDC Discovery Provider
func generateOIDCDiscoveryProviderRoute(config *v1alpha1.SpireOIDCDiscoveryProvider) *routev1.Route {
	labels := map[string]string{
		"app.kubernetes.io/name":     "spiffe-oidc-discovery-provider",
		"app.kubernetes.io/instance": "spire",
		"app.kubernetes.io/part-of":  "zero-trust-workload-identity-manager",
		"app.kubernetes.io/version":  "1.12.0",
		utils.AppManagedByLabelKey:   utils.AppManagedByLabelValue,
	}

	if config.Spec.Labels != nil {
		for k, v := range config.Spec.Labels {
			labels[k] = v
		}
	}

	// Use JwtIssuer as the route host
	host := config.Spec.JwtIssuer
	if host == "" {
		host = "oidc-discovery." + config.Spec.TrustDomain
	}

	return &routev1.Route{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "spiffe-oidc-discovery-provider",
			Namespace: utils.OperatorNamespace,
			Labels:    labels,
		},
		Spec: routev1.RouteSpec{
			Host: host,
			Port: &routev1.RoutePort{
				TargetPort: intstr.FromString("https"),
			},
			TLS: &routev1.TLSConfig{
				Termination:                   routev1.TLSTerminationReencrypt,
				InsecureEdgeTerminationPolicy: routev1.InsecureEdgeTerminationPolicyRedirect,
			},
			To: routev1.RouteTargetReference{
				Kind:   "Service",
				Name:   "spire-spiffe-oidc-discovery-provider",
				Weight: &[]int32{100}[0], // Pointer to 100
			},
			WildcardPolicy: routev1.WildcardPolicyNone,
		},
	}
}
