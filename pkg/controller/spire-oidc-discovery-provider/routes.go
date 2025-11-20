package spire_oidc_discovery_provider

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/api/equality"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/types"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	routev1 "github.com/openshift/api/route/v1"
	"github.com/openshift/zero-trust-workload-identity-manager/api/v1alpha1"
	"github.com/openshift/zero-trust-workload-identity-manager/pkg/controller/status"
	"github.com/openshift/zero-trust-workload-identity-manager/pkg/controller/utils"
)

// reconcileRoute reconciles the OIDC Discovery Provider Route
func (r *SpireOidcDiscoveryProviderReconciler) reconcileRoute(ctx context.Context, oidc *v1alpha1.SpireOIDCDiscoveryProvider, statusMgr *status.Manager, createOnlyMode bool) error {
	if utils.StringToBool(oidc.Spec.ManagedRoute) {
		// Create Route for OIDC Discovery Provider
		route, err := generateOIDCDiscoveryProviderRoute(oidc)
		if err != nil {
			r.log.Error(err, "Failed to generate OIDC discovery provider route")
			statusMgr.AddCondition(RouteAvailable, "ManagedRouteCreationFailed",
				err.Error(),
				metav1.ConditionFalse)
			return err
		}

		var existingRoute routev1.Route
		err = r.ctrlClient.Get(ctx, types.NamespacedName{
			Name:      route.Name,
			Namespace: route.Namespace,
		}, &existingRoute)
		if err != nil {
			if kerrors.IsNotFound(err) {
				if err = r.ctrlClient.Create(ctx, route); err != nil {
					r.log.Error(err, "Failed to create route")
					statusMgr.AddCondition(RouteAvailable, "ManagedRouteCreationFailed",
						err.Error(),
						metav1.ConditionFalse)
					return err
				}

				// Set status when route is actually created
				statusMgr.AddCondition(RouteAvailable, "ManagedRouteCreated",
					"Spire OIDC Managed Route created",
					metav1.ConditionTrue)

				r.log.Info("Created route", "Namespace", route.Namespace, "Name", route.Name)
			} else {
				r.log.Error(err, "Failed to get existing route")
				statusMgr.AddCondition(RouteAvailable, "ManagedRouteRetrievalFailed",
					err.Error(),
					metav1.ConditionFalse)
				return err
			}
		} else if checkRouteConflict(&existingRoute, route) {
			r.log.Info("Found conflict in routes, updating route")
			route.ResourceVersion = existingRoute.ResourceVersion

			if createOnlyMode {
				r.log.Info("Skipping Route update due to create-only mode", "Namespace", route.Namespace, "Name", route.Name)
			} else {
				err = r.ctrlClient.Update(ctx, route)
				if err != nil {
					statusMgr.AddCondition(RouteAvailable, "ManagedRouteUpdateFailed",
						err.Error(),
						metav1.ConditionFalse)
					return err
				}

				// Set status when route is actually updated
				statusMgr.AddCondition(RouteAvailable, "ManagedRouteUpdated",
					"Spire OIDC Managed Route updated",
					metav1.ConditionTrue)

				r.log.Info("Updated route", "Namespace", route.Namespace, "Name", route.Name)
			}
		} else {
			// Route exists and is up to date - only update status if it's currently not ready
			existingCondition := apimeta.FindStatusCondition(oidc.Status.ConditionalStatus.Conditions, RouteAvailable)
			if existingCondition == nil || existingCondition.Status != metav1.ConditionTrue {
				statusMgr.AddCondition(RouteAvailable, "ManagedRouteReady",
					"Spire OIDC Managed Route is ready",
					metav1.ConditionTrue)
			}
			// If route is already ready, don't update the status to avoid overwriting the reason
		}
	} else {
		// Only update status if it's currently enabled
		statusMgr.AddCondition(RouteAvailable, "ManagedRouteDisabled",
			"Spire OIDC Managed Route disabled",
			metav1.ConditionFalse)
	}

	return nil
}

// checkRouteConflict returns true if desired & current routes has conflicts else return false
func checkRouteConflict(current, desired *routev1.Route) bool {
	return !equality.Semantic.DeepEqual(current.Spec, desired.Spec) || !equality.Semantic.DeepEqual(current.Labels, desired.Labels)
}

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
			Namespace: utils.GetOperatorNamespace(),
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
