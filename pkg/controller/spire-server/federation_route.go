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
	"github.com/openshift/zero-trust-workload-identity-manager/pkg/controller/utils"
)

// generateFederationRoute creates an OpenShift Route resource for the SPIRE federation endpoint
func generateFederationRoute(server *v1alpha1.SpireServer) *routev1.Route {
	labels := utils.SpireServerLabels(server.Spec.Labels)

	route := &routev1.Route{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "spire-server-federation",
			Namespace: utils.OperatorNamespace,
			Labels:    labels,
		},
		Spec: routev1.RouteSpec{
			To: routev1.RouteTargetReference{
				Kind:   "Service",
				Name:   "spire-server",
				Weight: &[]int32{100}[0], // Pointer to 100
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
		// https_web profile uses re-encrypt TLS
		route.Spec.TLS = &routev1.TLSConfig{
			Termination:                   routev1.TLSTerminationReencrypt,
			InsecureEdgeTerminationPolicy: routev1.InsecureEdgeTerminationPolicyRedirect,
		}

		// If using ServingCert, set external certificate reference
		if server.Spec.Federation.BundleEndpoint.HttpsWeb != nil &&
			server.Spec.Federation.BundleEndpoint.HttpsWeb.ServingCert != nil {
			route.Spec.TLS.ExternalCertificate = &routev1.LocalObjectReference{
				Name: server.Spec.Federation.BundleEndpoint.HttpsWeb.ServingCert.SecretName,
			}
		}
	}

	return route
}

// checkFederationRouteConflict returns true if desired & current routes have conflicts
func checkFederationRouteConflict(current, desired *routev1.Route) bool {
	return !equality.Semantic.DeepEqual(current.Spec, desired.Spec) || !equality.Semantic.DeepEqual(current.Labels, desired.Labels)
}

// managedFederationRoute creates/updates route when managedRoute is enabled else sets status to disabled
func (r *SpireServerReconciler) managedFederationRoute(ctx context.Context, reconcileStatus map[string]reconcilerStatus, server *v1alpha1.SpireServer) error {
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
					reconcileStatus["FederationRouteReady"] = reconcilerStatus{
						Status:  metav1.ConditionFalse,
						Reason:  "FederationRouteCreationFailed",
						Message: err.Error(),
					}
					return err
				}

				// Set status when route is actually created
				reconcileStatus[FederationRouteReady] = reconcilerStatus{
					Status:  metav1.ConditionTrue,
					Reason:  "FederationRouteCreated",
					Message: "Federation route created",
				}

				r.log.Info("Created federation route", "Namespace", route.Namespace, "Name", route.Name)
			} else {
				r.log.Error(err, "Failed to get existing federation route")
				reconcileStatus[FederationRouteReady] = reconcilerStatus{
					Status:  metav1.ConditionFalse,
					Reason:  "FederationRouteRetrievalFailed",
					Message: err.Error(),
				}
				return err
			}
		} else if checkFederationRouteConflict(&existingRoute, route) {
			r.log.Info("Found conflict in federation routes, updating route")
			route.ResourceVersion = existingRoute.ResourceVersion

			err = r.ctrlClient.Update(ctx, route)
			if err != nil {
				reconcileStatus[FederationRouteReady] = reconcilerStatus{
					Status:  metav1.ConditionFalse,
					Reason:  "FederationRouteUpdateFailed",
					Message: err.Error(),
				}
				return err
			}

			// Set status when route is actually updated
			reconcileStatus[FederationRouteReady] = reconcilerStatus{
				Status:  metav1.ConditionTrue,
				Reason:  "FederationRouteUpdated",
				Message: "Federation route updated",
			}

			r.log.Info("Updated federation route", "Namespace", route.Namespace, "Name", route.Name)
		} else {
			// Route exists and is up to date - only update status if it's currently not ready
			existingCondition := apimeta.FindStatusCondition(server.Status.ConditionalStatus.Conditions, FederationRouteReady)
			if existingCondition == nil || existingCondition.Status != metav1.ConditionTrue {
				reconcileStatus[FederationRouteReady] = reconcilerStatus{
					Status:  metav1.ConditionTrue,
					Reason:  "FederationRouteReady",
					Message: "Federation route is ready",
				}
			}
			// If route is already ready, don't update the status to avoid overwriting the reason
		}
	} else {
		// Only update status to disabled
		reconcileStatus[FederationRouteReady] = reconcilerStatus{
			Status:  metav1.ConditionFalse,
			Reason:  "FederationRouteDisabled",
			Message: "Federation managed route disabled",
		}
	}

	return nil
}
