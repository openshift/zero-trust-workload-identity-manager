package spire_server

import (
	"context"
	"fmt"

	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"

	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/go-logr/logr"

	"github.com/openshift/zero-trust-workload-identity-manager/api/v1alpha1"
	customClient "github.com/openshift/zero-trust-workload-identity-manager/pkg/client"
	"github.com/openshift/zero-trust-workload-identity-manager/pkg/controller/status"
	"github.com/openshift/zero-trust-workload-identity-manager/pkg/controller/utils"
)

const (
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
)

// SpireServerReconciler reconciles a SpireServer object
type SpireServerReconciler struct {
	ctrlClient    customClient.CustomCtrlClient
	ctx           context.Context
	eventRecorder record.EventRecorder
	log           logr.Logger
	scheme        *runtime.Scheme
}

// +kubebuilder:rbac:groups=apps,resources=statefulsets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=configmaps,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=serviceaccounts,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=services,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=clusterroles,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=clusterrolebindings,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=roles,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=rolebindings,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=admissionregistration.k8s.io,resources=validatingwebhookconfigurations,verbs=get;list;watch;create;update;patch;delete

// New returns a new Reconciler instance.
func New(mgr ctrl.Manager) (*SpireServerReconciler, error) {
	c, err := customClient.NewCustomClient(mgr)
	if err != nil {
		return nil, err
	}
	return &SpireServerReconciler{
		ctrlClient:    c,
		ctx:           context.Background(),
		eventRecorder: mgr.GetEventRecorderFor(utils.ZeroTrustWorkloadIdentityManagerSpireServerControllerName),
		log:           ctrl.Log.WithName(utils.ZeroTrustWorkloadIdentityManagerSpireServerControllerName),
		scheme:        mgr.GetScheme(),
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

	// Set ZTWIM as the owner of SpireServer only if needed
	if utils.NeedsOwnerReferenceUpdate(&server, &ztwim) {
		if err := controllerutil.SetControllerReference(&ztwim, &server, r.scheme); err != nil {
			r.log.Error(err, "failed to set controller reference on SpireServer")
			statusMgr.AddCondition(v1alpha1.Ready, v1alpha1.ReasonFailed,
				fmt.Sprintf("Failed to set owner reference on SpireServer: %v", err),
				metav1.ConditionFalse)
			return ctrl.Result{}, err
		}

		// Persist the owner reference to the cluster
		if err := r.ctrlClient.Update(ctx, &server); err != nil {
			r.log.Error(err, "failed to update SpireServer with owner reference")
			statusMgr.AddCondition(v1alpha1.Ready, v1alpha1.ReasonFailed,
				fmt.Sprintf("Failed to update SpireServer with owner reference: %v", err),
				metav1.ConditionFalse)
			return ctrl.Result{}, err
		}
	}

	// Handle create-only mode
	createOnlyMode := r.handleCreateOnlyMode(&server, statusMgr)

	// Validate configuration
	if err := r.validateConfiguration(ctx, &server, statusMgr); err != nil {
		return ctrl.Result{}, nil
	}

	// Perform TTL validation
	if err := r.handleTTLValidation(ctx, &server, statusMgr); err != nil {
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
	spireServerConfigMapHash, err := r.reconcileSpireServerConfigMap(ctx, &server, statusMgr, &ztwim, createOnlyMode)
	if err != nil {
		return ctrl.Result{}, err
	}

	// Reconcile Spire Controller Manager ConfigMap
	spireControllerManagerConfigMapHash, err := r.reconcileSpireControllerManagerConfigMap(ctx, &server, statusMgr, &ztwim, createOnlyMode)
	if err != nil {
		return ctrl.Result{}, err
	}

	// Reconcile Spire Bundle ConfigMap
	if err := r.reconcileSpireBundleConfigMap(ctx, &server, statusMgr, &ztwim); err != nil {
		return ctrl.Result{}, err
	}

	// Reconcile StatefulSet
	if err := r.reconcileStatefulSet(ctx, &server, statusMgr, createOnlyMode, spireServerConfigMapHash, spireControllerManagerConfigMapHash); err != nil {
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
		For(&v1alpha1.SpireServer{}, builder.WithPredicates(utils.GenerationOrOwnerReferenceChangedPredicate)).
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
		Watches(&v1alpha1.ZeroTrustWorkloadIdentityManager{}, handler.EnqueueRequestsFromMapFunc(mapFunc), builder.WithPredicates(utils.ZTWIMSpecChangedPredicate)).
		Complete(r)
	if err != nil {
		return err
	}
	return nil
}

// handleCreateOnlyMode checks and updates the create-only mode status
func (r *SpireServerReconciler) handleCreateOnlyMode(server *v1alpha1.SpireServer, statusMgr *status.Manager) bool {
	createOnlyMode := utils.IsInCreateOnlyMode()
	if createOnlyMode {
		r.log.Info("Running in create-only mode - will create resources if they don't exist but skip updates")
		statusMgr.AddCondition(utils.CreateOnlyModeStatusType, utils.CreateOnlyModeEnabled,
			"Create-Only Mode is active: Updates are not reconciled to existing resources",
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
func (r *SpireServerReconciler) validateConfiguration(ctx context.Context, server *v1alpha1.SpireServer, statusMgr *status.Manager) error {
	// Validate common configuration (affinity, tolerations, node selector, resources, labels)
	if err := r.validateCommonConfig(server, statusMgr); err != nil {
		return err
	}

	// Validate proxy configuration - if proxy is enabled, CA bundle ConfigMap must be configured
	if err := r.validateProxyConfiguration(statusMgr); err != nil {
		return err
	}

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

// validateCommonConfig validates common configuration fields (affinity, tolerations, nodeSelector, resources, labels)
func (r *SpireServerReconciler) validateCommonConfig(server *v1alpha1.SpireServer, statusMgr *status.Manager) error {
	return utils.ValidateAndUpdateStatus(
		r.log,
		statusMgr,
		utils.ResourceKindSpireServer,
		server.Name,
		server.Spec.Affinity,
		server.Spec.Tolerations,
		server.Spec.NodeSelector,
		server.Spec.Resources,
		server.Spec.Labels,
	)
}

// validateProxyConfiguration validates proxy configuration using shared validation logic
func (r *SpireServerReconciler) validateProxyConfiguration(statusMgr *status.Manager) error {
	result := utils.ValidateProxyConfiguration()
	if !result.Valid {
		r.log.Error(fmt.Errorf(result.Reason), result.Message)
		statusMgr.AddCondition(ConfigurationValid, result.Reason, result.Message, metav1.ConditionFalse)
		return fmt.Errorf("proxy configuration invalid: %s", result.Message)
	}
	return nil
}

// needsUpdate returns true if StatefulSet needs to be updated
func needsUpdate(current, desired appsv1.StatefulSet) bool {
	if current.Spec.Template.Annotations[spireServerStatefulSetSpireServerConfigHashAnnotationKey] != desired.Spec.Template.Annotations[spireServerStatefulSetSpireServerConfigHashAnnotationKey] {
		return true
	} else if current.Spec.Template.Annotations[spireServerStatefulSetSpireControllerMangerConfigHashAnnotationKey] != desired.Spec.Template.Annotations[spireServerStatefulSetSpireControllerMangerConfigHashAnnotationKey] {
		return true
	}
	return utils.ResourceNeedsUpdate(&current, &desired)
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
