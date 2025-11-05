package spire_server

import (
	"context"
	"fmt"

	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"

	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"

	"k8s.io/apimachinery/pkg/api/equality"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/go-logr/logr"

	"github.com/openshift/zero-trust-workload-identity-manager/api/v1alpha1"
	customClient "github.com/openshift/zero-trust-workload-identity-manager/pkg/client"
	"github.com/openshift/zero-trust-workload-identity-manager/pkg/controller/status"
	"github.com/openshift/zero-trust-workload-identity-manager/pkg/controller/utils"
)

const (
<<<<<<< HEAD
	// Kubernetes-compliant condition names
	StatefulSetAvailable             = "StatefulSetAvailable"
	ServerConfigMapAvailable         = "ServerConfigMapAvailable"
	ControllerManagerConfigAvailable = "ControllerManagerConfigAvailable"
	BundleConfigAvailable            = "BundleConfigAvailable"
	TTLConfigurationValid            = "TTLConfigurationValid"
	ConfigurationValid               = "ConfigurationValid"
	ServiceAccountAvailable          = "ServiceAccountAvailable"
	ServiceAvailable                 = "ServiceAvailable"
	RBACAvailable                    = "RBACAvailable"
	ValidatingWebhookAvailable       = "ValidatingWebhookAvailable"
	// Federation-specific condition names
	FederationConfigurationValid = "FederationConfigurationValid"
	FederationServiceReady       = "FederationServiceReady"
	FederationRouteReady         = "FederationRouteReady"
=======
	SpireServerStatefulSetGeneration          = "SpireServerStatefulSetGeneration"
	SpireServerConfigMapGeneration            = "SpireServerConfigMapGeneration"
	SpireControllerManagerConfigMapGeneration = "SpireControllerManagerConfigMapGeneration"
	SpireBundleConfigMapGeneration            = "SpireBundleConfigMapGeneration"
	SpireServerTTLValidation                  = "SpireServerTTLValidation"
	ConfigurationValidation                   = "ConfigurationValidation"
	FederationConfigurationValid              = "FederationConfigurationValid"
	FederationRouteReady                      = "FederationRouteReady"
>>>>>>> 03323e74 (directly expose port 8443 in spire service)
)

// SpireServerReconciler reconciles a SpireServer object
type SpireServerReconciler struct {
	ctrlClient     customClient.CustomCtrlClient
	ctx            context.Context
	eventRecorder  record.EventRecorder
	log            logr.Logger
	scheme         *runtime.Scheme
	createOnlyMode bool
}

// +kubebuilder:rbac:groups=apps,resources=statefulsets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=configmaps,verbs=get;list;watch;create;update;patch;delete
<<<<<<< HEAD
// +kubebuilder:rbac:groups="",resources=serviceaccounts,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=services,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=clusterroles,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=clusterrolebindings,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=roles,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=rolebindings,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=admissionregistration.k8s.io,resources=validatingwebhookconfigurations,verbs=get;list;watch;create;update;patch;delete
=======
>>>>>>> 03323e74 (directly expose port 8443 in spire service)
// +kubebuilder:rbac:groups=route.openshift.io,resources=routes,verbs=get;list;watch;create;update;patch;delete

// New returns a new Reconciler instance.
func New(mgr ctrl.Manager) (*SpireServerReconciler, error) {
	c, err := customClient.NewCustomClient(mgr)
	if err != nil {
		return nil, err
	}
	return &SpireServerReconciler{
		ctrlClient:     c,
		ctx:            context.Background(),
		eventRecorder:  mgr.GetEventRecorderFor(utils.ZeroTrustWorkloadIdentityManagerSpireServerControllerName),
		log:            ctrl.Log.WithName(utils.ZeroTrustWorkloadIdentityManagerSpireServerControllerName),
		scheme:         mgr.GetScheme(),
		createOnlyMode: false,
	}, nil
}

func (r *SpireServerReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	r.log.Info(fmt.Sprintf("reconciling %s", utils.ZeroTrustWorkloadIdentityManagerSpireServerControllerName))
	var server v1alpha1.SpireServer
	if err := r.ctrlClient.Get(ctx, req.NamespacedName, &server); err != nil {
		if kerrors.IsNotFound(err) {
			r.log.Info("SpireServer resource not found. Ignoring since object must be deleted or not been created.")
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	// Set Ready to false at the start of reconciliation
	status.SetInitialReconciliationStatus(ctx, r.ctrlClient, &server, func() *v1alpha1.ConditionalStatus {
		return &server.Status.ConditionalStatus
	}, "SpireServer")

	statusMgr := status.NewManager(r.ctrlClient)
	defer func() {
		if err := statusMgr.ApplyStatus(ctx, &server, func() *v1alpha1.ConditionalStatus {
			return &server.Status.ConditionalStatus
		}); err != nil {
			r.log.Error(err, "failed to update status")
		}
	}()

	// Handle create-only mode
	createOnlyMode := r.handleCreateOnlyMode(&server, statusMgr)

	// Validate configuration
	if err := r.validateConfiguration(&server, statusMgr); err != nil {
		return ctrl.Result{}, nil
	}

	// Perform TTL validation
	if err := r.handleTTLValidation(ctx, &server, statusMgr); err != nil {
		return ctrl.Result{}, nil
	}

	// Validate federation configuration if present
	if err := r.validateFederationConfiguration(&server, statusMgr); err != nil {
		return ctrl.Result{}, nil
	}

	// Reconcile ServiceAccount
	if err := r.reconcileServiceAccount(ctx, &server, statusMgr, createOnlyMode); err != nil {
		return ctrl.Result{}, err
	}

	// Reconcile Services (spire-server and controller-manager)
	if err := r.reconcileService(ctx, &server, statusMgr, createOnlyMode); err != nil {
		return ctrl.Result{}, err
	}

	// Reconcile RBAC (spire-server, bundle, and controller-manager)
	if err := r.reconcileRBAC(ctx, &server, statusMgr, createOnlyMode); err != nil {
		return ctrl.Result{}, err
	}

	// Reconcile Webhook
	if err := r.reconcileWebhook(ctx, &server, statusMgr, createOnlyMode); err != nil {
		return ctrl.Result{}, err
	}

	// Reconcile ConfigMaps
	spireServerConfigMapHash, err := r.reconcileSpireServerConfigMap(ctx, &server, statusMgr, createOnlyMode)
	if err != nil {
		return ctrl.Result{}, err
	}

	// Reconcile Spire Controller Manager ConfigMap
	spireControllerManagerConfigMapHash, err := r.reconcileSpireControllerManagerConfigMap(ctx, &server, statusMgr, createOnlyMode)
	if err != nil {
		return ctrl.Result{}, err
	}

	// Reconcile Spire Bundle ConfigMap
	if err := r.reconcileSpireBundleConfigMap(ctx, &server, statusMgr); err != nil {
		return ctrl.Result{}, err
	}

	// Reconcile StatefulSet
	if err := r.reconcileStatefulSet(ctx, &server, statusMgr, createOnlyMode, spireServerConfigMapHash, spireControllerManagerConfigMapHash); err != nil {
		return ctrl.Result{}, err
	}

	// Manage federation Route
	if err := r.managedFederationRoute(ctx, statusMgr, &server); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *SpireServerReconciler) SetupWithManager(mgr ctrl.Manager) error {
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

	// Use component-specific predicate to only reconcile for control-plane component resources
	controllerManagedResourcePredicates := builder.WithPredicates(utils.ControllerManagedResourcesForComponent(utils.ComponentControlPlane))

	err := ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.SpireServer{}, builder.WithPredicates(predicate.GenerationChangedPredicate{})).
		Named(utils.ZeroTrustWorkloadIdentityManagerSpireServerControllerName).
		Watches(&appsv1.StatefulSet{}, handler.EnqueueRequestsFromMapFunc(mapFunc), controllerManagedResourcePredicates).
		Watches(&corev1.ConfigMap{}, handler.EnqueueRequestsFromMapFunc(mapFunc), controllerManagedResourcePredicates).
		Watches(&corev1.ServiceAccount{}, handler.EnqueueRequestsFromMapFunc(mapFunc), controllerManagedResourcePredicates).
		Watches(&corev1.Service{}, handler.EnqueueRequestsFromMapFunc(mapFunc), controllerManagedResourcePredicates).
		Watches(&rbacv1.ClusterRole{}, handler.EnqueueRequestsFromMapFunc(mapFunc), controllerManagedResourcePredicates).
		Watches(&rbacv1.ClusterRoleBinding{}, handler.EnqueueRequestsFromMapFunc(mapFunc), controllerManagedResourcePredicates).
		Watches(&rbacv1.Role{}, handler.EnqueueRequestsFromMapFunc(mapFunc), controllerManagedResourcePredicates).
		Watches(&rbacv1.RoleBinding{}, handler.EnqueueRequestsFromMapFunc(mapFunc), controllerManagedResourcePredicates).
		Watches(&admissionregistrationv1.ValidatingWebhookConfiguration{}, handler.EnqueueRequestsFromMapFunc(mapFunc), controllerManagedResourcePredicates).
		Complete(r)
	if err != nil {
		return err
	}
	return nil
}

// handleCreateOnlyMode checks and updates the create-only mode status
func (r *SpireServerReconciler) handleCreateOnlyMode(server *v1alpha1.SpireServer, statusMgr *status.Manager) bool {
	createOnlyMode := utils.IsInCreateOnlyMode(server, &r.createOnlyMode)
	if createOnlyMode {
		r.log.Info("Running in create-only mode - will create resources if they don't exist but skip updates")
		statusMgr.AddCondition(utils.CreateOnlyModeStatusType, utils.CreateOnlyModeEnabled,
			"Create-only mode is enabled via ztwim.openshift.io/create-only annotation",
			metav1.ConditionTrue)
	} else {
		existingCondition := apimeta.FindStatusCondition(server.Status.ConditionalStatus.Conditions, utils.CreateOnlyModeStatusType)
		if existingCondition != nil && existingCondition.Status == metav1.ConditionTrue {
			statusMgr.AddCondition(utils.CreateOnlyModeStatusType, utils.CreateOnlyModeDisabled,
				"Create-only mode is disabled",
				metav1.ConditionFalse)
		}
	}
	return createOnlyMode
}

// validateConfiguration validates the SpireServer configuration
func (r *SpireServerReconciler) validateConfiguration(server *v1alpha1.SpireServer, statusMgr *status.Manager) error {
	// Validate JWT issuer URL format
	if err := utils.IsValidURL(server.Spec.JwtIssuer); err != nil {
		r.log.Error(err, "Invalid JWT issuer URL in SpireServer configuration", "jwtIssuer", server.Spec.JwtIssuer)
		statusMgr.AddCondition(ConfigurationValid, "InvalidJWTIssuerURL",
			fmt.Sprintf("JWT issuer URL validation failed: %v", err),
			metav1.ConditionFalse)
		return err
	}

	// Only set to true if the condition previously existed as false
	existingCondition := apimeta.FindStatusCondition(server.Status.ConditionalStatus.Conditions, ConfigurationValid)
	if existingCondition != nil && existingCondition.Status == metav1.ConditionFalse {
		statusMgr.AddCondition(ConfigurationValid, v1alpha1.ReasonReady,
			"Configuration validation passed",
			metav1.ConditionTrue)
	}
	return nil
}

// validateFederationConfiguration validates the federation configuration if present
func (r *SpireServerReconciler) validateFederationConfiguration(server *v1alpha1.SpireServer, statusMgr *status.Manager) error {
	if server.Spec.Federation != nil {
		if err := validateFederationConfig(server.Spec.Federation, server.Spec.TrustDomain); err != nil {
			r.log.Error(err, "Invalid federation configuration", "trustDomain", server.Spec.TrustDomain)
			statusMgr.AddCondition(FederationConfigurationValid, "InvalidFederationConfiguration",
				fmt.Sprintf("Federation configuration validation failed: %v", err),
				metav1.ConditionFalse)
			return err
		}
		// Only set to true if the condition previously existed as false
		existingFedCondition := apimeta.FindStatusCondition(server.Status.ConditionalStatus.Conditions, FederationConfigurationValid)
		if existingFedCondition == nil || existingFedCondition.Status == metav1.ConditionFalse {
			statusMgr.AddCondition(FederationConfigurationValid, "ValidFederationConfiguration",
				"Federation configuration validation passed",
				metav1.ConditionTrue)
		}
	}
	return nil
}

// needsUpdate returns true if StatefulSet needs to be updated based on config checksum
func needsUpdate(current, desired appsv1.StatefulSet) bool {
	if current.Spec.Template.Annotations[spireServerStatefulSetSpireServerConfigHashAnnotationKey] != desired.Spec.Template.Annotations[spireServerStatefulSetSpireServerConfigHashAnnotationKey] {
		return true
	} else if current.Spec.Template.Annotations[spireServerStatefulSetSpireControllerMangerConfigHashAnnotationKey] != desired.Spec.Template.Annotations[spireServerStatefulSetSpireControllerMangerConfigHashAnnotationKey] {
		return true
	} else if !equality.Semantic.DeepEqual(current.Labels, desired.Labels) {
		return true
	} else if utils.StatefulSetSpecModified(&desired, &current) {
		return true
	}
	return false
}

// handleTTLValidation performs TTL validation and handles warnings, events, and status updates
func (r *SpireServerReconciler) handleTTLValidation(ctx context.Context, server *v1alpha1.SpireServer, statusMgr *status.Manager) error {
	ttlValidationResult := validateTTLDurationsWithWarnings(&server.Spec)

	if ttlValidationResult.Error != nil {
		r.log.Error(ttlValidationResult.Error, "TTL validation failed")
		statusMgr.AddCondition(TTLConfigurationValid, "TTLValidationFailed",
			ttlValidationResult.Error.Error(),
			metav1.ConditionFalse)
		return ttlValidationResult.Error
	}

	// Handle warnings
	if len(ttlValidationResult.Warnings) > 0 {
		// Log each warning
		for _, warning := range ttlValidationResult.Warnings {
			r.log.Info("TTL configuration warning", "warning", warning)
		}

		// Record events for each warning
		for _, warning := range ttlValidationResult.Warnings {
			r.eventRecorder.Event(server, corev1.EventTypeWarning, "TTLConfigurationWarning", warning)
		}

		// Set status condition with warning
		statusMgr.AddCondition(TTLConfigurationValid, "TTLValidationWarning",
			ttlValidationResult.StatusMessage,
			metav1.ConditionTrue)
	} else {
		// No warnings - set success status
		statusMgr.AddCondition(TTLConfigurationValid, "TTLValidationSucceeded",
			"TTL configuration is valid",
			metav1.ConditionTrue)
	}

	return nil
}
