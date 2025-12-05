package spire_server

import (
	"context"

	routev1 "github.com/openshift/api/route/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"

	"github.com/openshift/zero-trust-workload-identity-manager/api/v1alpha1"
	"github.com/openshift/zero-trust-workload-identity-manager/pkg/controller/status"
	"github.com/openshift/zero-trust-workload-identity-manager/pkg/controller/utils"
)

// generateFederationRoute creates an OpenShift Route resource for the SPIRE federation endpoint
func generateFederationRoute(server *v1alpha1.SpireServer, ztwim *v1alpha1.ZeroTrustWorkloadIdentityManager) *routev1.Route {
	labels := utils.SpireServerLabels(server.Spec.Labels)

	// Construct federation host using trust domain
	federationHost := "federation." + ztwim.Spec.TrustDomain

	route := &routev1.Route{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "spire-server-federation",
			Namespace: utils.OperatorNamespace,
			Labels:    labels,
		},
		Spec: routev1.RouteSpec{
			Host: federationHost,
			To: routev1.RouteTargetReference{
				Kind:   "Service",
				Name:   "spire-server",
				Weight: ptr.To(int32(100)),
			},
			Port: &routev1.RoutePort{
				TargetPort: intstr.FromString("federation"),
			},
			WildcardPolicy: routev1.WildcardPolicyNone,
		},
	}

	// Configure TLS based on profile
	switch server.Spec.Federation.BundleEndpoint.Profile {
	case v1alpha1.HttpsSpiffeProfile:
		// https_spiffe profile uses passthrough TLS
		route.Spec.TLS = &routev1.TLSConfig{
			Termination:                   routev1.TLSTerminationPassthrough,
			InsecureEdgeTerminationPolicy: routev1.InsecureEdgeTerminationPolicyRedirect,
		}
	case v1alpha1.HttpsWebProfile:
		// https_web profile: termination depends on ACME vs ServingCert
		if server.Spec.Federation.BundleEndpoint.HttpsWeb != nil &&
			server.Spec.Federation.BundleEndpoint.HttpsWeb.Acme != nil {
			// ACME: certificate is managed by SPIRE server, use passthrough
			// so clients see the ACME-issued cert directly
			route.Spec.TLS = &routev1.TLSConfig{
				Termination:                   routev1.TLSTerminationPassthrough,
				InsecureEdgeTerminationPolicy: routev1.InsecureEdgeTerminationPolicyRedirect,
			}
		} else {
			// ServingCert: use re-encrypt TLS
			route.Spec.TLS = &routev1.TLSConfig{
				Termination:                   routev1.TLSTerminationReencrypt,
				InsecureEdgeTerminationPolicy: routev1.InsecureEdgeTerminationPolicyRedirect,
			}

			// Set external certificate if provided
			if server.Spec.Federation.BundleEndpoint.HttpsWeb != nil &&
				server.Spec.Federation.BundleEndpoint.HttpsWeb.ServingCert != nil &&
				server.Spec.Federation.BundleEndpoint.HttpsWeb.ServingCert.ExternalSecretRef != "" {
				route.Spec.TLS.ExternalCertificate = &routev1.LocalObjectReference{
					Name: server.Spec.Federation.BundleEndpoint.HttpsWeb.ServingCert.ExternalSecretRef,
				}
			}
		}
	}

	return route
}

// checkFederationRouteConflict returns true if desired & current routes have conflicts
func checkFederationRouteConflict(current, desired *routev1.Route) bool {
	return !equality.Semantic.DeepEqual(current.Spec, desired.Spec) || !equality.Semantic.DeepEqual(current.Labels, desired.Labels)
}

// reconcileRoute creates/updates route when managedRoute is enabled else sets status to disabled
func (r *SpireServerReconciler) reconcileRoute(ctx context.Context, server *v1alpha1.SpireServer, statusMgr *status.Manager, ztwim *v1alpha1.ZeroTrustWorkloadIdentityManager, createOnlyMode bool) error {
	// Check if federation is configured
	if server.Spec.Federation == nil {
		// No federation configured - don't manage route, don't set status
		return nil
	}

	if utils.StringToBool(server.Spec.Federation.ManagedRoute) {
		// Create Route for federation endpoint
		route := generateFederationRoute(server, ztwim)

		var existingRoute routev1.Route
		err := r.ctrlClient.Get(ctx, types.NamespacedName{
			Name:      route.Name,
			Namespace: route.Namespace,
		}, &existingRoute)
		if err != nil {
			if kerrors.IsNotFound(err) {
				if err = r.ctrlClient.Create(ctx, route); err != nil {
					r.log.Error(err, "Failed to create federation route")
					statusMgr.AddCondition(RouteAvailable, "FederationRouteCreationFailed",
						err.Error(),
						metav1.ConditionFalse)
					return err
				}

				// Set status when route is actually created
				statusMgr.AddCondition(RouteAvailable, "FederationRouteCreated",
					"Federation route created",
					metav1.ConditionTrue)

				r.log.Info("Created federation route", "Namespace", route.Namespace, "Name", route.Name)
			} else {
				r.log.Error(err, "Failed to get existing federation route")
				statusMgr.AddCondition(RouteAvailable, "FederationRouteRetrievalFailed",
					err.Error(),
					metav1.ConditionFalse)
				return err
			}
		} else if checkFederationRouteConflict(&existingRoute, route) {
			if createOnlyMode {
				r.log.Info("Skipping federation route update due to create-only mode")
			} else {
				r.log.Info("Found conflict in federation routes, updating route")
				route.ResourceVersion = existingRoute.ResourceVersion

				err = r.ctrlClient.Update(ctx, route)
				if err != nil {
					statusMgr.AddCondition(RouteAvailable, "FederationRouteUpdateFailed",
						err.Error(),
						metav1.ConditionFalse)
					return err
				}

				// Set status when route is actually updated
				statusMgr.AddCondition(RouteAvailable, "FederationRouteUpdated",
					"Federation route updated",
					metav1.ConditionTrue)

				r.log.Info("Updated federation route", "Namespace", route.Namespace, "Name", route.Name)
			}
		} else {
			// Route exists and is up to date - only update status if it's currently not ready
			existingCondition := apimeta.FindStatusCondition(server.Status.ConditionalStatus.Conditions, RouteAvailable)
			if existingCondition == nil || existingCondition.Status != metav1.ConditionTrue {
				statusMgr.AddCondition(RouteAvailable, "RouteAvailable",
					"Federation route is ready",
					metav1.ConditionTrue)
			}
			// If route is already ready, don't update the status to avoid overwriting the reason
		}
	} else {
		// Only update status to disabled
		statusMgr.AddCondition(RouteAvailable, "FederationRouteDisabled",
			"Federation managed route disabled",
			metav1.ConditionFalse)
	}

	return nil
}
