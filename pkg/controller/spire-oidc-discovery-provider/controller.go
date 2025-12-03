package spire_oidc_discovery_provider

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	spiffev1alpha1 "github.com/spiffe/spire-controller-manager/api/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"

	kerrors "k8s.io/apimachinery/pkg/api/errors"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"

	"k8s.io/client-go/tools/record"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	routev1 "github.com/openshift/api/route/v1"
	"github.com/openshift/zero-trust-workload-identity-manager/api/v1alpha1"
	customClient "github.com/openshift/zero-trust-workload-identity-manager/pkg/client"
	"github.com/openshift/zero-trust-workload-identity-manager/pkg/controller/status"
	"github.com/openshift/zero-trust-workload-identity-manager/pkg/controller/utils"
)

const spireOidcDeploymentSpireOidcConfigHashAnnotationKey = "ztwim.openshift.io/spire-oidc-discovery-provider-config-hash"

const (
	DeploymentAvailable      = "DeploymentAvailable"
	ConfigMapAvailable       = "ConfigMapAvailable"
	ClusterSPIFFEIDAvailable = "ClusterSPIFFEIDAvailable"
	RouteAvailable           = "RouteAvailable"
	ConfigurationValid       = "ConfigurationValid"
	ServiceAccountAvailable  = "ServiceAccountAvailable"
	ServiceAvailable         = "ServiceAvailable"
)

// SpireOidcDiscoveryProviderReconciler reconciles a SpireOidcDiscoveryProvider object
type SpireOidcDiscoveryProviderReconciler struct {
	ctrlClient    customClient.CustomCtrlClient
	ctx           context.Context
	eventRecorder record.EventRecorder
	log           logr.Logger
	scheme        *runtime.Scheme
}

// +kubebuilder:rbac:groups="",resources=serviceaccounts,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=services,verbs=get;list;watch;create;update;patch;delete

// New returns a new Reconciler instance.
func New(mgr ctrl.Manager) (*SpireOidcDiscoveryProviderReconciler, error) {
	c, err := customClient.NewCustomClient(mgr)
	if err != nil {
		return nil, err
	}
	return &SpireOidcDiscoveryProviderReconciler{
		ctrlClient:    c,
		ctx:           context.Background(),
		eventRecorder: mgr.GetEventRecorderFor(utils.ZeroTrustWorkloadIdentityManagerSpireOIDCDiscoveryProviderControllerName),
		log:           ctrl.Log.WithName(utils.ZeroTrustWorkloadIdentityManagerSpireOIDCDiscoveryProviderControllerName),
		scheme:        mgr.GetScheme(),
	}, nil
}

func (r *SpireOidcDiscoveryProviderReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	r.log.Info(fmt.Sprintf("reconciling %s", utils.ZeroTrustWorkloadIdentityManagerSpireOIDCDiscoveryProviderControllerName))

	var oidcDiscoveryProviderConfig v1alpha1.SpireOIDCDiscoveryProvider
	if err := r.ctrlClient.Get(ctx, req.NamespacedName, &oidcDiscoveryProviderConfig); err != nil {
		if kerrors.IsNotFound(err) {
			r.log.Info("SpireOidcDiscoveryProvider resource not found. Ignoring since object must be deleted or not been created.")
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	// Set Ready to false at the start of reconciliation
	status.SetInitialReconciliationStatus(ctx, r.ctrlClient, &oidcDiscoveryProviderConfig, func() *v1alpha1.ConditionalStatus {
		return &oidcDiscoveryProviderConfig.Status.ConditionalStatus
	}, "SpireOIDCDiscoveryProvider")

	statusMgr := status.NewManager(r.ctrlClient)
	defer func() {
		if err := statusMgr.ApplyStatus(ctx, &oidcDiscoveryProviderConfig, func() *v1alpha1.ConditionalStatus {
			return &oidcDiscoveryProviderConfig.Status.ConditionalStatus
		}); err != nil {
			r.log.Error(err, "failed to update status")
		}
	}()

	var ztwim v1alpha1.ZeroTrustWorkloadIdentityManager
	if err := r.ctrlClient.Get(ctx, types.NamespacedName{Name: "cluster"}, &ztwim); err != nil {
		if kerrors.IsNotFound(err) {
			r.log.Error(err, "failed to get ZeroTrustWorkloadIdentityManager")
			statusMgr.AddCondition(v1alpha1.Ready, v1alpha1.ReasonFailed,
				fmt.Sprintf("Failed to retrieve ZeroTrustWorkloadIdentityManager from cluster"),
				metav1.ConditionFalse)
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	// Set ZTWIM as the owner of SpireOidcDiscoveryProvider only if needed
	if utils.NeedsOwnerReferenceUpdate(&oidcDiscoveryProviderConfig, &ztwim) {
		if err := controllerutil.SetControllerReference(&ztwim, &oidcDiscoveryProviderConfig, r.scheme); err != nil {
			r.log.Error(err, "failed to set controller reference on SpireOidcDiscoveryProvider")
			statusMgr.AddCondition(v1alpha1.Ready, v1alpha1.ReasonFailed,
				fmt.Sprintf("Failed to set owner reference on SpireOidcDiscoveryProvider: %v", err),
				metav1.ConditionFalse)
			return ctrl.Result{}, err
		}

		// Persist the owner reference to the cluster
		if err := r.ctrlClient.Update(ctx, &oidcDiscoveryProviderConfig); err != nil {
			r.log.Error(err, "failed to update SpireOIDCDiscoveryProvider with owner reference")
			statusMgr.AddCondition(v1alpha1.Ready, v1alpha1.ReasonFailed,
				fmt.Sprintf("Failed to update SpireOIDCDiscoveryProvider with owner reference: %v", err),
				metav1.ConditionFalse)
			return ctrl.Result{}, err
		}
	}

	// Handle create-only mode
	createOnlyMode := r.handleCreateOnlyMode(&oidcDiscoveryProviderConfig, statusMgr)

	// Validate configuration
	if err := r.validateConfiguration(ctx, &oidcDiscoveryProviderConfig, statusMgr); err != nil {
		return ctrl.Result{}, nil
	}

	// Reconcile static resources (ServiceAccount, Service)
	if err := r.reconcileServiceAccount(ctx, &oidcDiscoveryProviderConfig, statusMgr, createOnlyMode); err != nil {
		return ctrl.Result{}, err
	}

	if err := r.reconcileService(ctx, &oidcDiscoveryProviderConfig, statusMgr, createOnlyMode); err != nil {
		return ctrl.Result{}, err
	}

	// Reconcile ClusterSpiffeIDs
	if err := r.reconcileClusterSpiffeIDs(ctx, &oidcDiscoveryProviderConfig, statusMgr, createOnlyMode); err != nil {
		return ctrl.Result{}, err
	}

	// Reconcile ConfigMap
	configHash, err := r.reconcileConfigMap(ctx, &oidcDiscoveryProviderConfig, statusMgr, &ztwim, createOnlyMode)
	if err != nil {
		return ctrl.Result{}, err
	}

	// Reconcile Deployment
	if err := r.reconcileDeployment(ctx, &oidcDiscoveryProviderConfig, statusMgr, createOnlyMode, configHash); err != nil {
		return ctrl.Result{}, err
	}

	// Reconcile Route (if enabled)
	if err := r.reconcileRoute(ctx, &oidcDiscoveryProviderConfig, statusMgr, createOnlyMode); err != nil {
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
		For(&v1alpha1.SpireOIDCDiscoveryProvider{}, builder.WithPredicates(utils.GenerationOrOwnerReferenceChangedPredicate)).
		Named(utils.ZeroTrustWorkloadIdentityManagerSpireOIDCDiscoveryProviderControllerName).
		Watches(&appsv1.Deployment{}, handler.EnqueueRequestsFromMapFunc(mapFunc), controllerManagedResourcePredicates).
		Watches(&corev1.ConfigMap{}, handler.EnqueueRequestsFromMapFunc(mapFunc), controllerManagedResourcePredicates).
		Watches(&corev1.ServiceAccount{}, handler.EnqueueRequestsFromMapFunc(mapFunc), controllerManagedResourcePredicates).
		Watches(&corev1.Service{}, handler.EnqueueRequestsFromMapFunc(mapFunc), controllerManagedResourcePredicates).
		Watches(&routev1.Route{}, handler.EnqueueRequestsFromMapFunc(mapFunc), controllerManagedResourcePredicates).
		Watches(&spiffev1alpha1.ClusterSPIFFEID{}, handler.EnqueueRequestsFromMapFunc(mapFunc), controllerManagedResourcePredicates).
		Watches(&v1alpha1.ZeroTrustWorkloadIdentityManager{}, handler.EnqueueRequestsFromMapFunc(mapFunc), builder.WithPredicates(utils.ZTWIMSpecChangedPredicate)).
		Complete(r)
	if err != nil {
		return err
	}
	return nil
}

// handleCreateOnlyMode checks and updates the create-only mode status
func (r *SpireOidcDiscoveryProviderReconciler) handleCreateOnlyMode(oidc *v1alpha1.SpireOIDCDiscoveryProvider, statusMgr *status.Manager) bool {
	createOnlyMode := utils.IsInCreateOnlyMode()
	if createOnlyMode {
		r.log.Info("Running in create-only mode - will create resources if they don't exist but skip updates")
		statusMgr.AddCondition(utils.CreateOnlyModeStatusType, utils.CreateOnlyModeEnabled,
			"Create-Only Mode is active: Updates are not reconciled to existing resources",
			metav1.ConditionTrue)
	} else {
		existingCondition := apimeta.FindStatusCondition(oidc.Status.ConditionalStatus.Conditions, utils.CreateOnlyModeStatusType)
		if existingCondition != nil && existingCondition.Status == metav1.ConditionTrue {
			statusMgr.AddCondition(utils.CreateOnlyModeStatusType, utils.CreateOnlyModeDisabled,
				"Create-only mode is disabled",
				metav1.ConditionFalse)
		}
	}
	return createOnlyMode
}

// validateConfiguration validates the SpireOIDCDiscoveryProvider configuration
func (r *SpireOidcDiscoveryProviderReconciler) validateConfiguration(ctx context.Context, oidc *v1alpha1.SpireOIDCDiscoveryProvider, statusMgr *status.Manager) error {
	// Validate common configuration
	if err := r.validateCommonConfig(oidc, statusMgr); err != nil {
		return err
	}

	// Validate proxy configuration - if proxy is enabled, CA bundle ConfigMap must be configured
	if err := r.validateProxyConfiguration(statusMgr); err != nil {
		return err
	}

	// Validate JWT issuer URL format
	if err := utils.IsValidURL(oidc.Spec.JwtIssuer); err != nil {
		r.log.Error(err, "Invalid JWT issuer URL in SpireOIDCDiscoveryProvider configuration", "jwtIssuer", oidc.Spec.JwtIssuer)
		statusMgr.AddCondition(ConfigurationValid, "InvalidJWTIssuerURL",
			fmt.Sprintf("JWT issuer URL validation failed: %v", err),
			metav1.ConditionFalse)
		return err
	}

	// Only set to true if the condition previously existed as false
	existingCondition := apimeta.FindStatusCondition(oidc.Status.ConditionalStatus.Conditions, ConfigurationValid)
	if existingCondition != nil && existingCondition.Status == metav1.ConditionFalse {
		statusMgr.AddCondition(ConfigurationValid, v1alpha1.ReasonReady,
			"Configuration validation passed",
			metav1.ConditionTrue)
	}
	return nil
}

// validateProxyConfiguration validates proxy configuration using shared validation logic
func (r *SpireOidcDiscoveryProviderReconciler) validateProxyConfiguration(statusMgr *status.Manager) error {
	result := utils.ValidateProxyConfiguration()

	if !result.Valid {
		r.log.Error(fmt.Errorf(result.Reason), result.Message)
		statusMgr.AddCondition(ConfigurationValid, result.Reason, result.Message, metav1.ConditionFalse)
		return fmt.Errorf("proxy configuration invalid: %s", result.Message)
	}
	return nil
}

// validateCommonConfig validates common configuration fields (affinity, tolerations, nodeSelector, resources, labels)
func (r *SpireOidcDiscoveryProviderReconciler) validateCommonConfig(oidc *v1alpha1.SpireOIDCDiscoveryProvider, statusMgr *status.Manager) error {
	return utils.ValidateAndUpdateStatus(
		r.log,
		statusMgr,
		utils.ResourceKindSpireOIDCDiscoveryProvider,
		oidc.Name,
		oidc.Spec.Affinity,
		oidc.Spec.Tolerations,
		oidc.Spec.NodeSelector,
		oidc.Spec.Resources,
		oidc.Spec.Labels,
	)
}

// needsUpdate returns true if Deployment needs to be updated based on config checksum
func needsUpdate(current, desired appsv1.Deployment) bool {
	if current.Spec.Template.Annotations[spireOidcDeploymentSpireOidcConfigHashAnnotationKey] != desired.Spec.Template.Annotations[spireOidcDeploymentSpireOidcConfigHashAnnotationKey] {
		return true
	}
	return utils.ResourceNeedsUpdate(&current, &desired)
}
