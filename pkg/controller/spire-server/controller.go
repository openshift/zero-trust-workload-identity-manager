package spire_server

import (
	"context"
	"fmt"
	"reflect"

	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"

	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"

	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
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
	ctrlClient     customClient.CustomCtrlClient
	ctx            context.Context
	eventRecorder  record.EventRecorder
	log            logr.Logger
	scheme         *runtime.Scheme
	createOnlyMode bool
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
		ctrlClient:     c,
		ctx:            context.Background(),
		eventRecorder:  mgr.GetEventRecorderFor(utils.ZeroTrustWorkloadIdentityManagerSpireServerControllerName),
		log:            ctrl.Log.WithName(utils.ZeroTrustWorkloadIdentityManagerSpireServerControllerName),
		scheme:         mgr.GetScheme(),
		createOnlyMode: false,
	}, nil
}

func (r *SpireServerReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	r.log.Info("reconciling ", utils.ZeroTrustWorkloadIdentityManagerSpireServerControllerName)
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

// reconcileSpireServerConfigMap reconciles the Spire Server ConfigMap
func (r *SpireServerReconciler) reconcileSpireServerConfigMap(ctx context.Context, server *v1alpha1.SpireServer, statusMgr *status.Manager, createOnlyMode bool) (string, error) {
	spireServerConfigMap, err := GenerateSpireServerConfigMap(&server.Spec)
	if err != nil {
		r.log.Error(err, "failed to generate spire server config map")
		statusMgr.AddCondition(ServerConfigMapAvailable, "SpireServerConfigMapGenerationFailed",
			err.Error(),
			metav1.ConditionFalse)
		return "", err
	}

	if err = controllerutil.SetControllerReference(server, spireServerConfigMap, r.scheme); err != nil {
		r.log.Error(err, "failed to set controller reference")
		statusMgr.AddCondition(ServerConfigMapAvailable, "SpireServerConfigMapGenerationFailed",
			err.Error(),
			metav1.ConditionFalse)
		return "", err
	}

	var existingSpireServerCM corev1.ConfigMap
	err = r.ctrlClient.Get(ctx, types.NamespacedName{Name: spireServerConfigMap.Name, Namespace: spireServerConfigMap.Namespace}, &existingSpireServerCM)
	if err != nil && kerrors.IsNotFound(err) {
		if err = r.ctrlClient.Create(ctx, spireServerConfigMap); err != nil {
			statusMgr.AddCondition(ServerConfigMapAvailable, "SpireServerConfigMapGenerationFailed",
				err.Error(),
				metav1.ConditionFalse)
			return "", fmt.Errorf("failed to create ConfigMap: %w", err)
		}
		r.log.Info("Created spire server ConfigMap")
	} else if err == nil && (existingSpireServerCM.Data["server.conf"] != spireServerConfigMap.Data["server.conf"] ||
		!reflect.DeepEqual(existingSpireServerCM.Labels, spireServerConfigMap.Labels)) {
		if createOnlyMode {
			r.log.Info("Skipping ConfigMap update due to create-only mode")
		} else {
			spireServerConfigMap.ResourceVersion = existingSpireServerCM.ResourceVersion
			if err = r.ctrlClient.Update(ctx, spireServerConfigMap); err != nil {
				statusMgr.AddCondition(ServerConfigMapAvailable, "SpireServerConfigMapGenerationFailed",
					err.Error(),
					metav1.ConditionFalse)
				return "", fmt.Errorf("failed to update ConfigMap: %w", err)
			}
			r.log.Info("Updated ConfigMap with new config")
		}
	} else if err != nil {
		statusMgr.AddCondition(ServerConfigMapAvailable, "SpireServerConfigMapGenerationFailed",
			err.Error(),
			metav1.ConditionFalse)
		return "", err
	}

	statusMgr.AddCondition(ServerConfigMapAvailable, "SpireConfigMapResourceCreated",
		"SpireServer config map resources applied",
		metav1.ConditionTrue)

	// Generate config hash
	spireServerConfJSON, err := marshalToJSON(generateServerConfMap(&server.Spec))
	if err != nil {
		r.log.Error(err, "failed to marshal spire server config map to JSON")
		return "", err
	}

	return generateConfigHash(spireServerConfJSON), nil
}

// reconcileSpireControllerManagerConfigMap reconciles the Spire Controller Manager ConfigMap
func (r *SpireServerReconciler) reconcileSpireControllerManagerConfigMap(ctx context.Context, server *v1alpha1.SpireServer, statusMgr *status.Manager, createOnlyMode bool) (string, error) {
	spireControllerManagerConfig, err := generateSpireControllerManagerConfigYaml(&server.Spec)
	if err != nil {
		r.log.Error(err, "Failed to generate spire controller manager config")
		statusMgr.AddCondition(ControllerManagerConfigAvailable, "SpireControllerManagerConfigMapGenerationFailed",
			err.Error(),
			metav1.ConditionFalse)
		return "", err
	}

	spireControllerManagerConfigMap := generateControllerManagerConfigMap(spireControllerManagerConfig)
	if err = controllerutil.SetControllerReference(server, spireControllerManagerConfigMap, r.scheme); err != nil {
		r.log.Error(err, "failed to set controller reference on spire controller manager config")
		statusMgr.AddCondition(ControllerManagerConfigAvailable, "SpireControllerManagerConfigMapGenerationFailed",
			err.Error(),
			metav1.ConditionFalse)
		return "", err
	}

	var existingSpireControllerManagerCM corev1.ConfigMap
	err = r.ctrlClient.Get(ctx, types.NamespacedName{Name: spireControllerManagerConfigMap.Name, Namespace: spireControllerManagerConfigMap.Namespace}, &existingSpireControllerManagerCM)
	if err != nil && kerrors.IsNotFound(err) {
		if err = r.ctrlClient.Create(ctx, spireControllerManagerConfigMap); err != nil {
			r.log.Error(err, "failed to create spire controller manager config map")
			statusMgr.AddCondition(ControllerManagerConfigAvailable, "SpireControllerManagerConfigMapGenerationFailed",
				err.Error(),
				metav1.ConditionFalse)
			return "", fmt.Errorf("failed to create ConfigMap: %w", err)
		}
		r.log.Info("Created spire controller manager ConfigMap")
	} else if err == nil && (existingSpireControllerManagerCM.Data["controller-manager-config.yaml"] != spireControllerManagerConfigMap.Data["controller-manager-config.yaml"] ||
		!reflect.DeepEqual(existingSpireControllerManagerCM.Labels, spireControllerManagerConfigMap.Labels)) {
		if createOnlyMode {
			r.log.Info("Skipping spire controller manager ConfigMap update due to create-only mode")
		} else {
			spireControllerManagerConfigMap.ResourceVersion = existingSpireControllerManagerCM.ResourceVersion
			if err = r.ctrlClient.Update(ctx, spireControllerManagerConfigMap); err != nil {
				statusMgr.AddCondition(ControllerManagerConfigAvailable, "SpireControllerManagerConfigMapGenerationFailed",
					err.Error(),
					metav1.ConditionFalse)
				return "", fmt.Errorf("failed to update ConfigMap: %w", err)
			}
		}
		r.log.Info("Updated ConfigMap with new config")
	} else if err != nil {
		r.log.Error(err, "failed to update spire controller manager config map")
		return "", err
	}

	statusMgr.AddCondition(ControllerManagerConfigAvailable, "SpireControllerManagerConfigMapCreated",
		"spire controller manager config map resources applied",
		metav1.ConditionTrue)

	return generateConfigHashFromString(spireControllerManagerConfig), nil
}

// reconcileSpireBundleConfigMap reconciles the Spire Bundle ConfigMap
func (r *SpireServerReconciler) reconcileSpireBundleConfigMap(ctx context.Context, server *v1alpha1.SpireServer, statusMgr *status.Manager) error {
	spireBundleCM, err := generateSpireBundleConfigMap(&server.Spec)
	if err != nil {
		r.log.Error(err, "failed to generate spire bundle config map")
		statusMgr.AddCondition(BundleConfigAvailable, "SpireBundleConfigMapGenerationFailed",
			err.Error(),
			metav1.ConditionFalse)
		return err
	}

	if err := controllerutil.SetControllerReference(server, spireBundleCM, r.scheme); err != nil {
		r.log.Error(err, "failed to set controller reference on spire bundle config")
		statusMgr.AddCondition(BundleConfigAvailable, "SpireBundleConfigMapGenerationFailed",
			err.Error(),
			metav1.ConditionFalse)
		return err
	}

	err = r.ctrlClient.Create(ctx, spireBundleCM)
	if err != nil && !kerrors.IsAlreadyExists(err) {
		r.log.Error(err, "failed to create spire bundle config map")
		statusMgr.AddCondition(BundleConfigAvailable, "SpireBundleConfigMapGenerationFailed",
			err.Error(),
			metav1.ConditionFalse)
		return fmt.Errorf("failed to create spire-bundle ConfigMap: %w", err)
	}

	statusMgr.AddCondition(BundleConfigAvailable, "SpireBundleConfigMapCreated",
		"spire bundle config map resources applied",
		metav1.ConditionTrue)
	return nil
}

// reconcileStatefulSet reconciles the Spire Server StatefulSet
func (r *SpireServerReconciler) reconcileStatefulSet(ctx context.Context, server *v1alpha1.SpireServer, statusMgr *status.Manager, createOnlyMode bool, spireServerConfigMapHash, spireControllerManagerConfigMapHash string) error {
	sts := GenerateSpireServerStatefulSet(&server.Spec, spireServerConfigMapHash, spireControllerManagerConfigMapHash)
	if err := controllerutil.SetControllerReference(server, sts, r.scheme); err != nil {
		r.log.Error(err, "failed to set controller reference on spire server stateful set resource")
		statusMgr.AddCondition(StatefulSetAvailable, "SpireServerStatefulSetGenerationFailed",
			err.Error(),
			metav1.ConditionFalse)
		return err
	}

	var existingSTS appsv1.StatefulSet
	err := r.ctrlClient.Get(ctx, types.NamespacedName{Name: sts.Name, Namespace: sts.Namespace}, &existingSTS)
	if err != nil && kerrors.IsNotFound(err) {
		if err = r.ctrlClient.Create(ctx, sts); err != nil {
			statusMgr.AddCondition(StatefulSetAvailable, "SpireServerStatefulSetCreationFailed",
				err.Error(),
				metav1.ConditionFalse)
			return fmt.Errorf("failed to create StatefulSet: %w", err)
		}
		r.log.Info("Created spire server StatefulSet")
	} else if err == nil && needsUpdate(existingSTS, *sts) {
		if createOnlyMode {
			r.log.Info("Skipping StatefulSet update due to create-only mode")
		} else {
			sts.ResourceVersion = existingSTS.ResourceVersion
			if err = r.ctrlClient.Update(ctx, sts); err != nil {
				statusMgr.AddCondition(StatefulSetAvailable, "SpireServerStatefulSetUpdateFailed",
					err.Error(),
					metav1.ConditionFalse)
				return fmt.Errorf("failed to update StatefulSet: %w", err)
			}
			r.log.Info("Updated spire server StatefulSet")
		}
	} else if err != nil {
		r.log.Error(err, "failed to get spire server stateful set resource")
		statusMgr.AddCondition(StatefulSetAvailable, "SpireServerStatefulSetGetFailed",
			err.Error(),
			metav1.ConditionFalse)
		return err
	}

	// Check StatefulSet health/readiness
	statusMgr.CheckStatefulSetHealth(ctx, sts.Name, sts.Namespace, StatefulSetAvailable)

	return nil
}

// needsUpdate returns true if StatefulSet needs to be updated based on config checksum
func needsUpdate(current, desired appsv1.StatefulSet) bool {
	if current.Spec.Template.Annotations[spireServerStatefulSetSpireServerConfigHashAnnotationKey] != desired.Spec.Template.Annotations[spireServerStatefulSetSpireServerConfigHashAnnotationKey] {
		return true
	} else if current.Spec.Template.Annotations[spireServerStatefulSetSpireControllerMangerConfigHashAnnotationKey] != desired.Spec.Template.Annotations[spireServerStatefulSetSpireControllerMangerConfigHashAnnotationKey] {
		return true
	} else if !reflect.DeepEqual(current.Labels, desired.Labels) {
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
