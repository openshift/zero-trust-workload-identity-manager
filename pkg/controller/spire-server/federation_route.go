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

	"github.com/openshift/zero-trust-workload-identity-manager/api/v1alpha1"
	"github.com/openshift/zero-trust-workload-identity-manager/pkg/controller/status"
	"github.com/openshift/zero-trust-workload-identity-manager/pkg/controller/utils"
)

// generateFederationRoute creates an OpenShift Route resource for the SPIRE federation endpoint
func generateFederationRoute(server *v1alpha1.SpireServer) *routev1.Route {
	labels := utils.SpireServerLabels(server.Spec.Labels)

	return &routev1.Route{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "spire-server-federation",
			Namespace: utils.OperatorNamespace,
			Labels:    labels,
		},
		Spec: routev1.RouteSpec{
			To: routev1.RouteTargetReference{
				Kind: "Service",
				Name: "spire-server",
			},
			Port: &routev1.RoutePort{
				TargetPort: intstr.FromString("federation"),
			},
			TLS: &routev1.TLSConfig{
				Termination:                   routev1.TLSTerminationPassthrough,
				InsecureEdgeTerminationPolicy: routev1.InsecureEdgeTerminationPolicyRedirect,
			},
			WildcardPolicy: routev1.WildcardPolicyNone,
		},
	}
}

// checkFederationRouteConflict returns true if desired & current routes have conflicts
func checkFederationRouteConflict(current, desired *routev1.Route) bool {
	return !equality.Semantic.DeepEqual(current.Spec, desired.Spec) || !equality.Semantic.DeepEqual(current.Labels, desired.Labels)
}

// managedFederationRoute creates/updates route when managedRoute is enabled else sets status to disabled
func (r *SpireServerReconciler) managedFederationRoute(ctx context.Context, statusMgr *status.Manager, server *v1alpha1.SpireServer) error {
	// Check if federation is configured
	if server.Spec.Federation == nil {
		// No federation configured - don't manage route, don't set status
		return nil
	}

	if utils.StringToBool(server.Spec.Federation.ManagedRoute) {
		// Create Route for federation endpoint
		route := generateFederationRoute(server)

		var existingRoute routev1.Route
		err := r.ctrlClient.Get(ctx, types.NamespacedName{
			Name:      route.Name,
			Namespace: route.Namespace,
		}, &existingRoute)
		if err != nil {
			if kerrors.IsNotFound(err) {
				if err = r.ctrlClient.Create(ctx, route); err != nil {
					r.log.Error(err, "Failed to create federation route")
					statusMgr.AddCondition(FederationRouteReady, "FederationRouteCreationFailed",
						err.Error(),
						metav1.ConditionFalse)
					return err
				}

				// Set status when route is actually created
				statusMgr.AddCondition(FederationRouteReady, "FederationRouteCreated",
					"Federation route created",
					metav1.ConditionTrue)

				r.log.Info("Created federation route", "Namespace", route.Namespace, "Name", route.Name)
			} else {
				r.log.Error(err, "Failed to get existing federation route")
				statusMgr.AddCondition(FederationRouteReady, "FederationRouteRetrievalFailed",
					err.Error(),
					metav1.ConditionFalse)
				return err
			}
		} else if checkFederationRouteConflict(&existingRoute, route) {
			r.log.Info("Found conflict in federation routes, updating route")
			route.ResourceVersion = existingRoute.ResourceVersion

			err = r.ctrlClient.Update(ctx, route)
			if err != nil {
				statusMgr.AddCondition(FederationRouteReady, "FederationRouteUpdateFailed",
					err.Error(),
					metav1.ConditionFalse)
				return err
			}

			// Set status when route is actually updated
			statusMgr.AddCondition(FederationRouteReady, "FederationRouteUpdated",
				"Federation route updated",
				metav1.ConditionTrue)

			r.log.Info("Updated federation route", "Namespace", route.Namespace, "Name", route.Name)
		} else {
			// Route exists and is up to date - only update status if it's currently not ready
			existingCondition := apimeta.FindStatusCondition(server.Status.ConditionalStatus.Conditions, FederationRouteReady)
			if existingCondition == nil || existingCondition.Status != metav1.ConditionTrue {
				statusMgr.AddCondition(FederationRouteReady, "FederationRouteReady",
					"Federation route is ready",
					metav1.ConditionTrue)
			}
			// If route is already ready, don't update the status to avoid overwriting the reason
		}
	} else {
		// Only update status to disabled
		statusMgr.AddCondition(FederationRouteReady, "FederationRouteDisabled",
			"Federation managed route disabled",
			metav1.ConditionFalse)
	}

	return nil
}
