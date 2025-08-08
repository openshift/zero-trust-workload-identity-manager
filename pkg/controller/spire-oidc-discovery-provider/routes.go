package spire_oidc_discovery_provider

import (
	"fmt"
	routev1 "github.com/openshift/api/route/v1"
	"github.com/openshift/zero-trust-workload-identity-manager/api/v1alpha1"
	"github.com/openshift/zero-trust-workload-identity-manager/pkg/controller/utils"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

// generateOIDCDiscoveryProviderRoute creates an OpenShift Route resource for the SPIRE OIDC Discovery Provider
func generateOIDCDiscoveryProviderRoute(config *v1alpha1.SpireOIDCDiscoveryProvider) (*routev1.Route, error) {
	labels := utils.SpireOIDCDiscoveryProviderLabels(config.Spec.Labels)

	// JWT Issuer validation and normalization
	jwtIssuer, err := utils.StripProtocolFromJWTIssuer(config.Spec.JwtIssuer)
	if err != nil {
		return nil, fmt.Errorf("invalid JWT issuer URL: %w", err)
	}

	route := &routev1.Route{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "spire-oidc-discovery-provider",
			Namespace: utils.OperatorNamespace,
			Labels:    labels,
		},
		Spec: routev1.RouteSpec{
			Host: jwtIssuer,
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

	if config.Spec.ExternalSecretRef != "" {
		route.Spec.TLS.ExternalCertificate = &routev1.LocalObjectReference{
			Name: config.Spec.ExternalSecretRef,
		}
	}

	return route, nil
}
