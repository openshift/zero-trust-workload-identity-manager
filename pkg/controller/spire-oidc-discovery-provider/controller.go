package spire_oidc_discovery_provider

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"

	"k8s.io/apimachinery/pkg/api/equality"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"

	"k8s.io/client-go/tools/record"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	routev1 "github.com/openshift/api/route/v1"
	"github.com/openshift/zero-trust-workload-identity-manager/api/v1alpha1"
	customClient "github.com/openshift/zero-trust-workload-identity-manager/pkg/client"
	"github.com/openshift/zero-trust-workload-identity-manager/pkg/controller/utils"
)

const spireOidcDeploymentSpireOidcConfigHashAnnotationKey = "ztwim.openshift.io/spire-oidc-discovery-provider-config-hash"

const (
	SpireOIDCDeploymentGeneration  = "SpireOIDCDeploymentGeneration"
	SpireOIDCConfigMapGeneration   = "SpireOIDCConfigMapGeneration"
	SpireOIDCSCCGeneration         = "SpireOIDCSCCGeneration"
	SpireClusterSpiffeIDGeneration = "SpireClusterSpiffeIDGeneration"
	ManagedRouteReady              = "ManagedRouteReady"
	ConfigurationValidation        = "ConfigurationValidation"
)

type reconcilerStatus struct {
	Status  metav1.ConditionStatus
	Message string
	Reason  string
}

// SpireOidcDiscoveryProviderReconciler reconciles a SpireOidcDiscoveryProvider object
type SpireOidcDiscoveryProviderReconciler struct {
	ctrlClient     customClient.CustomCtrlClient
	ctx            context.Context
	eventRecorder  record.EventRecorder
	log            logr.Logger
	scheme         *runtime.Scheme
	createOnlyMode bool
}

// New returns a new Reconciler instance.
func New(mgr ctrl.Manager) (*SpireOidcDiscoveryProviderReconciler, error) {
	c, err := customClient.NewCustomClient(mgr)
	if err != nil {
		return nil, err
	}
	return &SpireOidcDiscoveryProviderReconciler{
		ctrlClient:     c,
		ctx:            context.Background(),
		eventRecorder:  mgr.GetEventRecorderFor(utils.ZeroTrustWorkloadIdentityManagerSpireOIDCDiscoveryProviderControllerName),
		log:            ctrl.Log.WithName(utils.ZeroTrustWorkloadIdentityManagerSpireOIDCDiscoveryProviderControllerName),
		scheme:         mgr.GetScheme(),
		createOnlyMode: false,
	}, nil
}

func (r *SpireOidcDiscoveryProviderReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	r.log.Info("Reconciling SpireOIDCDiscoveryProvider controller")

	var oidcDiscoveryProviderConfig v1alpha1.SpireOIDCDiscoveryProvider
	if err := r.ctrlClient.Get(ctx, req.NamespacedName, &oidcDiscoveryProviderConfig); err != nil {
		if kerrors.IsNotFound(err) {
			r.log.Info("SpireOidcDiscoveryProvider resource not found. Ignoring since object must be deleted or not been created.")
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}
	reconcileStatus := map[string]reconcilerStatus{}
	defer func(reconcileStatus map[string]reconcilerStatus) {
		originalStatus := oidcDiscoveryProviderConfig.Status.DeepCopy()
		if oidcDiscoveryProviderConfig.Status.ConditionalStatus.Conditions == nil {
			oidcDiscoveryProviderConfig.Status.ConditionalStatus = v1alpha1.ConditionalStatus{
				Conditions: []metav1.Condition{},
			}
		}
		for key, value := range reconcileStatus {
			newCondition := metav1.Condition{
				Type:               key,
				Status:             value.Status,
				Reason:             value.Reason,
				Message:            value.Message,
				LastTransitionTime: metav1.Now(),
			}
			apimeta.SetStatusCondition(&oidcDiscoveryProviderConfig.Status.ConditionalStatus.Conditions, newCondition)
		}
		if !equality.Semantic.DeepEqual(originalStatus, &oidcDiscoveryProviderConfig.Status) {
			newConfig := oidcDiscoveryProviderConfig.DeepCopy()
			if err := r.ctrlClient.StatusUpdateWithRetry(ctx, newConfig); err != nil {
				r.log.Error(err, "failed to update status")
			}
		}
	}(reconcileStatus)

	createOnlyMode := utils.IsInCreateOnlyMode(&oidcDiscoveryProviderConfig, &r.createOnlyMode)
	if createOnlyMode {
		r.log.Info("Running in create-only mode - will create resources if they don't exist but skip updates")
		reconcileStatus[utils.CreateOnlyModeStatusType] = reconcilerStatus{
			Status:  metav1.ConditionTrue,
			Reason:  utils.CreateOnlyModeEnabled,
			Message: "Create-only mode is enabled via ztwim.openshift.io/create-only annotation",
		}
	} else {
		existingCondition := apimeta.FindStatusCondition(oidcDiscoveryProviderConfig.Status.ConditionalStatus.Conditions, utils.CreateOnlyModeStatusType)
		if existingCondition != nil && existingCondition.Status == metav1.ConditionTrue {
			reconcileStatus[utils.CreateOnlyModeStatusType] = reconcilerStatus{
				Status:  metav1.ConditionFalse,
				Reason:  utils.CreateOnlyModeDisabled,
				Message: "Create-only mode is disabled",
			}
		}
	}

	// Validate JWT issuer URL format to prevent unintended formats during OIDC discovery document creation
	if err := utils.IsValidURL(oidcDiscoveryProviderConfig.Spec.JwtIssuer); err != nil {
		r.log.Error(err, "Invalid JWT issuer URL in SpireOIDCDiscoveryProvider configuration", "jwtIssuer", oidcDiscoveryProviderConfig.Spec.JwtIssuer)
		reconcileStatus[ConfigurationValidation] = reconcilerStatus{
			Status:  metav1.ConditionFalse,
			Reason:  "InvalidJWTIssuerURL",
			Message: fmt.Sprintf("JWT issuer URL validation failed: %v", err),
		}
		// do not requeue if the user input validation error exist.
		return ctrl.Result{}, nil
	}

	// Only set to true if the condition previously existed as false
	existingCondition := apimeta.FindStatusCondition(oidcDiscoveryProviderConfig.Status.ConditionalStatus.Conditions, ConfigurationValidation)
	if existingCondition != nil && existingCondition.Status == metav1.ConditionFalse {
		reconcileStatus[ConfigurationValidation] = reconcilerStatus{
			Status:  metav1.ConditionTrue,
			Reason:  "ValidJWTIssuerURL",
			Message: "JWT issuer URL validation passed",
		}
	}

	spireOIDCClusterSpiffeID := generateSpireIODCDiscoveryProviderSpiffeID()
	if err := controllerutil.SetControllerReference(&oidcDiscoveryProviderConfig, spireOIDCClusterSpiffeID, r.scheme); err != nil {
		r.log.Error(err, "failed to set controller reference")
		reconcileStatus[SpireClusterSpiffeIDGeneration] = reconcilerStatus{
			Status:  metav1.ConditionFalse,
			Reason:  "SpireClusterSpiffeIDGenerationFailed",
			Message: err.Error(),
		}
		return ctrl.Result{}, err
	}
	err := r.ctrlClient.Create(ctx, spireOIDCClusterSpiffeID)
	if err != nil && !kerrors.IsAlreadyExists(err) {
		r.log.Error(err, "Failed to create oidc cluster spiffe id")
		reconcileStatus[SpireClusterSpiffeIDGeneration] = reconcilerStatus{
			Status:  metav1.ConditionFalse,
			Reason:  "SpireClusterSpiffeIDCreationFailed",
			Message: err.Error(),
		}
		return ctrl.Result{}, err
	}
	defaultSpiffeID := generateDefaultFallbackClusterSPIFFEID()
	if err = controllerutil.SetControllerReference(&oidcDiscoveryProviderConfig, defaultSpiffeID, r.scheme); err != nil {
		r.log.Error(err, "failed to set controller reference")
		reconcileStatus[SpireClusterSpiffeIDGeneration] = reconcilerStatus{
			Status:  metav1.ConditionFalse,
			Reason:  "SpireClusterSpiffeIDCreationFailed",
			Message: err.Error(),
		}
		return ctrl.Result{}, err
	}
	err = r.ctrlClient.Create(ctx, defaultSpiffeID)
	if err != nil && !kerrors.IsAlreadyExists(err) {
		r.log.Error(err, "Failed to create DefaultFallbackClusterSPIFFEID")
		reconcileStatus[SpireClusterSpiffeIDGeneration] = reconcilerStatus{
			Status:  metav1.ConditionFalse,
			Reason:  "SpireClusterSpiffeIDCreationFailed",
			Message: err.Error(),
		}
		return ctrl.Result{}, err
	}
	reconcileStatus[SpireClusterSpiffeIDGeneration] = reconcilerStatus{
		Status:  metav1.ConditionTrue,
		Reason:  "SpireClusterSpiffeIDCreationSucceeded",
		Message: "Spire OIDC and default ClusterSpiffeID created successfully",
	}

	cm, err := GenerateOIDCConfigMapFromCR(&oidcDiscoveryProviderConfig)
	if err != nil {
		r.log.Error(err, "failed to generate OIDC ConfigMap from CR")
		reconcileStatus[SpireOIDCConfigMapGeneration] = reconcilerStatus{
			Status:  metav1.ConditionFalse,
			Reason:  "SpireOIDCConfigMapCreationFailed",
			Message: err.Error(),
		}
		return ctrl.Result{}, err
	}
	if err = controllerutil.SetControllerReference(&oidcDiscoveryProviderConfig, cm, r.scheme); err != nil {
		r.log.Error(err, "failed to set controller reference")
		reconcileStatus[SpireOIDCConfigMapGeneration] = reconcilerStatus{
			Status:  metav1.ConditionFalse,
			Reason:  "SpireOIDCConfigMapCreationFailed",
			Message: err.Error(),
		}
		return ctrl.Result{}, err
	}
	var existingOidcCm corev1.ConfigMap
	err = r.ctrlClient.Get(ctx, types.NamespacedName{Name: cm.Name, Namespace: cm.Namespace}, &existingOidcCm)
	if err != nil && kerrors.IsNotFound(err) {
		if err = r.ctrlClient.Create(ctx, cm); err != nil {
			r.log.Error(err, "Failed to create ConfigMap")
			reconcileStatus[SpireOIDCConfigMapGeneration] = reconcilerStatus{
				Status:  metav1.ConditionFalse,
				Reason:  "SpireOIDCConfigMapCreationFailed",
				Message: err.Error(),
			}
			return ctrl.Result{}, err
		}
		r.log.Info("Created ConfigMap", "Namespace", cm.Namespace, "Name", cm.Name)
	} else if utils.GenerateMapHash(existingOidcCm.Data) != utils.GenerateMapHash(cm.Data) {
		if createOnlyMode {
			r.log.Info("Skipping ConfigMap update due to create-only mode", "Namespace", cm.Namespace, "Name", cm.Name)
		} else {
			existingOidcCm.Data = cm.Data
			if err = r.ctrlClient.Update(ctx, &existingOidcCm); err != nil {
				r.log.Error(err, "Failed to update ConfigMap", "Namespace", cm.Namespace, "Name", cm.Name)
				reconcileStatus[SpireOIDCConfigMapGeneration] = reconcilerStatus{
					Status:  metav1.ConditionFalse,
					Reason:  "SpireOIDCConfigMapCreationFailed",
					Message: err.Error(),
				}
				return ctrl.Result{}, err
			}
			r.log.Info("Updated ConfigMap", "Namespace", cm.Namespace, "Name", cm.Name)
		}
	} else if err != nil {
		r.log.Error(err, "Failed to get ConfigMap")
		reconcileStatus[SpireOIDCConfigMapGeneration] = reconcilerStatus{
			Status:  metav1.ConditionFalse,
			Reason:  "SpireOIDCConfigMapCreationFailed",
			Message: err.Error(),
		}
		return ctrl.Result{}, err
	}
	reconcileStatus[SpireOIDCConfigMapGeneration] = reconcilerStatus{
		Status:  metav1.ConditionTrue,
		Reason:  "SpireOIDCConfigMapCreationSucceeded",
		Message: "Spire OIDC ConfigMap created",
	}

	configMapHash := utils.GenerateMapHash(cm.Data)
	deployment := buildDeployment(&oidcDiscoveryProviderConfig, configMapHash)
	if err = controllerutil.SetControllerReference(&oidcDiscoveryProviderConfig, deployment, r.scheme); err != nil {
		r.log.Error(err, "failed to set controller reference")
		reconcileStatus[SpireOIDCDeploymentGeneration] = reconcilerStatus{
			Status:  metav1.ConditionFalse,
			Reason:  "SpireOIDCDeploymentCreationFailed",
			Message: err.Error(),
		}
		return ctrl.Result{}, err
	}
	var existingSpireOidcDeployment appsv1.Deployment
	err = r.ctrlClient.Get(ctx, types.NamespacedName{
		Name:      deployment.Name,
		Namespace: deployment.Namespace,
	}, &existingSpireOidcDeployment)
	if err != nil && kerrors.IsNotFound(err) {
		if err = r.ctrlClient.Create(ctx, deployment); err != nil {
			r.log.Error(err, "Failed to create spire oidc discovery provider deployment")
			reconcileStatus[SpireOIDCDeploymentGeneration] = reconcilerStatus{
				Status:  metav1.ConditionFalse,
				Reason:  "SpireOIDCDeploymentCreationFailed",
				Message: err.Error(),
			}
			return ctrl.Result{}, err
		}
		r.log.Info("Created spire oidc discovery provider deployment")
	} else if err == nil && needsUpdate(existingSpireOidcDeployment, *deployment) {
		if createOnlyMode {
			r.log.Info("Skipping Deployment update due to create-only mode")
		} else {
			existingSpireOidcDeployment.Spec = deployment.Spec
			if err = r.ctrlClient.Update(ctx, &existingSpireOidcDeployment); err != nil {
				r.log.Error(err, "Failed to update spire oidc discovery provider deployment")
				reconcileStatus[SpireOIDCDeploymentGeneration] = reconcilerStatus{
					Status:  metav1.ConditionFalse,
					Reason:  "SpireOIDCDeploymentCreationFailed",
					Message: err.Error(),
				}
				return ctrl.Result{}, err
			}
			r.log.Info("Updated spire oidc discovery provider deployment")
		}
	} else if err != nil {
		r.log.Error(err, "Failed to get existing spire oidc discovery provider deployment")
		reconcileStatus[SpireOIDCDeploymentGeneration] = reconcilerStatus{
			Status:  metav1.ConditionFalse,
			Reason:  "SpireOIDCDeploymentCreationFailed",
			Message: err.Error(),
		}
		return ctrl.Result{}, err
	}
	reconcileStatus[SpireOIDCDeploymentGeneration] = reconcilerStatus{
		Status:  metav1.ConditionTrue,
		Reason:  "SpireOIDCDeploymentCreationSucceeded",
		Message: "Spire OIDC Deployment created",
	}

	err = r.managedRoute(ctx, reconcileStatus, &oidcDiscoveryProviderConfig)
	if err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *SpireOidcDiscoveryProviderReconciler) SetupWithManager(mgr ctrl.Manager) error {
	// Always enqueue the "cluster" CR for reconciliation
	mapFunc := func(ctx context.Context, _ client.Object) []reconcile.Request {
		return []reconcile.Request{
			{
				NamespacedName: types.NamespacedName{
					Name: "cluster",
				},
			},
		}
	}

	// Use component-specific predicate to only reconcile for discovery component resources
	controllerManagedResourcePredicates := builder.WithPredicates(utils.ControllerManagedResourcesForComponent(utils.ComponentDiscovery))

	err := ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.SpireOIDCDiscoveryProvider{}, builder.WithPredicates(predicate.GenerationChangedPredicate{})).
		Named(utils.ZeroTrustWorkloadIdentityManagerSpireOIDCDiscoveryProviderControllerName).
		Watches(&appsv1.Deployment{}, handler.EnqueueRequestsFromMapFunc(mapFunc), controllerManagedResourcePredicates).
		Watches(&corev1.ConfigMap{}, handler.EnqueueRequestsFromMapFunc(mapFunc), controllerManagedResourcePredicates).
		Watches(&routev1.Route{}, handler.EnqueueRequestsFromMapFunc(mapFunc), controllerManagedResourcePredicates).
		Complete(r)
	if err != nil {
		return err
	}
	return nil
}

// needsUpdate returns true if Deployment needs to be updated based on config checksum
func needsUpdate(current, desired appsv1.Deployment) bool {
	if current.Spec.Template.Annotations[spireOidcDeploymentSpireOidcConfigHashAnnotationKey] != desired.Spec.Template.Annotations[spireOidcDeploymentSpireOidcConfigHashAnnotationKey] {
		return true
	} else if utils.DeploymentSpecModified(&desired, &current) {
		return true
	}
	return false
}

// checkRouteConflict returns true if desired & current routes has conflicts else return false
func checkRouteConflict(current, desired *routev1.Route) bool {
	return !equality.Semantic.DeepEqual(current.Spec, desired.Spec) || !equality.Semantic.DeepEqual(current.Labels, desired.Labels)
}

// managedRoute route creates/updates route when managedRoute is enabled else skips when disabled
func (r *SpireOidcDiscoveryProviderReconciler) managedRoute(ctx context.Context, reconcileStatus map[string]reconcilerStatus, oidcDiscoveryProviderConfig *v1alpha1.SpireOIDCDiscoveryProvider) error {
	if utils.StringToBool(oidcDiscoveryProviderConfig.Spec.ManagedRoute) {
		// Create Route for OIDC Discovery Provider
		route, err := generateOIDCDiscoveryProviderRoute(oidcDiscoveryProviderConfig)
		if err != nil {
			r.log.Error(err, "Failed to generate OIDC discovery provider route")
			reconcileStatus[ManagedRouteReady] = reconcilerStatus{
				Status:  metav1.ConditionFalse,
				Reason:  "ManagedRouteCreationFailed",
				Message: err.Error(),
			}
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
					reconcileStatus[ManagedRouteReady] = reconcilerStatus{
						Status:  metav1.ConditionFalse,
						Reason:  "ManagedRouteCreationFailed",
						Message: err.Error(),
					}
					return err
				}

				// Set status when route is actually created
				reconcileStatus[ManagedRouteReady] = reconcilerStatus{
					Status:  metav1.ConditionTrue,
					Reason:  "ManagedRouteCreated",
					Message: "Spire OIDC Managed Route created",
				}

				r.log.Info("Created route", "Namespace", route.Namespace, "Name", route.Name)
			} else {
				r.log.Error(err, "Failed to get existing route")
				reconcileStatus[ManagedRouteReady] = reconcilerStatus{
					Status:  metav1.ConditionFalse,
					Reason:  "ManagedRouteRetrievalFailed",
					Message: err.Error(),
				}
				return err
			}
		} else if checkRouteConflict(&existingRoute, route) {
			r.log.Info("Found conflict in routes, updating route")
			route.ResourceVersion = existingRoute.ResourceVersion

			err = r.ctrlClient.Update(ctx, route)
			if err != nil {
				reconcileStatus[ManagedRouteReady] = reconcilerStatus{
					Status:  metav1.ConditionFalse,
					Reason:  "ManagedRouteUpdateFailed",
					Message: err.Error(),
				}
				return err
			}

			// Set status when route is actually updated
			reconcileStatus[ManagedRouteReady] = reconcilerStatus{
				Status:  metav1.ConditionTrue,
				Reason:  "ManagedRouteUpdated",
				Message: "Spire OIDC Managed Route updated",
			}

			r.log.Info("Updated route", "Namespace", route.Namespace, "Name", route.Name)
		} else {
			// Route exists and is up to date - only update status if it's currently not ready
			existingCondition := apimeta.FindStatusCondition(oidcDiscoveryProviderConfig.Status.ConditionalStatus.Conditions, ManagedRouteReady)
			if existingCondition == nil || existingCondition.Status != metav1.ConditionTrue {
				reconcileStatus[ManagedRouteReady] = reconcilerStatus{
					Status:  metav1.ConditionTrue,
					Reason:  "ManagedRouteReady",
					Message: "Spire OIDC Managed Route is ready",
				}
			}
			// If route is already ready, don't update the status to avoid overwriting the reason
		}
	} else {
		// Only update status if it's currently enabled
		reconcileStatus[ManagedRouteReady] = reconcilerStatus{
			Status:  metav1.ConditionFalse,
			Reason:  "ManagedRouteDisabled",
			Message: "Spire OIDC Managed Route disabled",
		}
	}

	return nil
}
