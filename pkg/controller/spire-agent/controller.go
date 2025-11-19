package spire_agent

import (
	"context"
	"fmt"

	securityv1 "github.com/openshift/api/security/v1"
	customClient "github.com/openshift/zero-trust-workload-identity-manager/pkg/client"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
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
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/go-logr/logr"

	"github.com/openshift/zero-trust-workload-identity-manager/api/v1alpha1"
	"github.com/openshift/zero-trust-workload-identity-manager/pkg/controller/status"
	"github.com/openshift/zero-trust-workload-identity-manager/pkg/controller/utils"
)

const (
	DaemonSetAvailable                  = "DaemonSetAvailable"
	ConfigMapAvailable                  = "ConfigMapAvailable"
	SecurityContextConstraintsAvailable = "SecurityContextConstraintsAvailable"
	ServiceAccountAvailable             = "ServiceAccountAvailable"
	ServiceAvailable                    = "ServiceAvailable"
	RBACAvailable                       = "RBACAvailable"
)

const spireAgentDaemonSetSpireAgentConfigHashAnnotationKey = "ztwim.openshift.io/spire-agent-config-hash"

// SpireAgentReconciler reconciles a SpireAgent object
type SpireAgentReconciler struct {
	ctrlClient     customClient.CustomCtrlClient
	ctx            context.Context
	eventRecorder  record.EventRecorder
	log            logr.Logger
	scheme         *runtime.Scheme
	createOnlyMode bool
}

// +kubebuilder:rbac:groups="",resources=serviceaccounts,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=services,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=clusterroles,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=clusterrolebindings,verbs=get;list;watch;create;update;patch;delete

// New returns a new Reconciler instance.
func New(mgr ctrl.Manager) (*SpireAgentReconciler, error) {
	c, err := customClient.NewCustomClient(mgr)
	if err != nil {
		return nil, err
	}
	return &SpireAgentReconciler{
		ctrlClient:     c,
		ctx:            context.Background(),
		eventRecorder:  mgr.GetEventRecorderFor(utils.ZeroTrustWorkloadIdentityManagerSpireAgentControllerName),
		log:            ctrl.Log.WithName(utils.ZeroTrustWorkloadIdentityManagerSpireAgentControllerName),
		scheme:         mgr.GetScheme(),
		createOnlyMode: false,
	}, nil
}

func (r *SpireAgentReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	r.log.Info(fmt.Sprintf("reconciling %s", utils.ZeroTrustWorkloadIdentityManagerSpireAgentControllerName))
	var agent v1alpha1.SpireAgent
	if err := r.ctrlClient.Get(ctx, req.NamespacedName, &agent); err != nil {
		if kerrors.IsNotFound(err) {
			r.log.Info("SpireAgent resource not found. Ignoring since object must be deleted or not been created.")
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	// Set Ready to false at the start of reconciliation
	status.SetInitialReconciliationStatus(ctx, r.ctrlClient, &agent, func() *v1alpha1.ConditionalStatus {
		return &agent.Status.ConditionalStatus
	}, "SpireAgent")

	statusMgr := status.NewManager(r.ctrlClient)
	defer func() {
		if err := statusMgr.ApplyStatus(ctx, &agent, func() *v1alpha1.ConditionalStatus {
			return &agent.Status.ConditionalStatus
		}); err != nil {
			r.log.Error(err, "failed to update status")
		}
	}()

	// Handle create-only mode
	createOnlyMode := r.handleCreateOnlyMode(&agent, statusMgr)

	// Validate common configuration
	if err := r.validateCommonConfig(&agent, statusMgr); err != nil {
		return ctrl.Result{}, nil
	}

	// Reconcile static resources (RBAC, ServiceAccount, Service)
	if err := r.reconcileServiceAccount(ctx, &agent, statusMgr, createOnlyMode); err != nil {
		return ctrl.Result{}, err
	}

	if err := r.reconcileService(ctx, &agent, statusMgr, createOnlyMode); err != nil {
		return ctrl.Result{}, err
	}

	if err := r.reconcileRBAC(ctx, &agent, statusMgr, createOnlyMode); err != nil {
		return ctrl.Result{}, err
	}

	// Reconcile SCC
	if err := r.reconcileSCC(ctx, &agent, statusMgr); err != nil {
		return ctrl.Result{}, err
	}

	// Reconcile ConfigMap
	configHash, err := r.reconcileConfigMap(ctx, &agent, statusMgr, createOnlyMode)
	if err != nil {
		return ctrl.Result{}, err
	}

	// Reconcile DaemonSet
	if err := r.reconcileDaemonSet(ctx, &agent, statusMgr, createOnlyMode, configHash); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *SpireAgentReconciler) SetupWithManager(mgr ctrl.Manager) error {
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

	// Use component-specific predicate to only reconcile for node-agent component resources
	controllerManagedResourcePredicates := builder.WithPredicates(utils.ControllerManagedResourcesForComponent(utils.ComponentNodeAgent))

	err := ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.SpireAgent{}, builder.WithPredicates(predicate.GenerationChangedPredicate{})).
		Named(utils.ZeroTrustWorkloadIdentityManagerSpireAgentControllerName).
		Watches(&appsv1.DaemonSet{}, handler.EnqueueRequestsFromMapFunc(mapFunc), controllerManagedResourcePredicates).
		Watches(&corev1.ConfigMap{}, handler.EnqueueRequestsFromMapFunc(mapFunc), controllerManagedResourcePredicates).
		Watches(&corev1.ServiceAccount{}, handler.EnqueueRequestsFromMapFunc(mapFunc), controllerManagedResourcePredicates).
		Watches(&corev1.Service{}, handler.EnqueueRequestsFromMapFunc(mapFunc), controllerManagedResourcePredicates).
		Watches(&rbacv1.ClusterRole{}, handler.EnqueueRequestsFromMapFunc(mapFunc), controllerManagedResourcePredicates).
		Watches(&rbacv1.ClusterRoleBinding{}, handler.EnqueueRequestsFromMapFunc(mapFunc), controllerManagedResourcePredicates).
		Watches(&securityv1.SecurityContextConstraints{}, handler.EnqueueRequestsFromMapFunc(mapFunc), controllerManagedResourcePredicates).
		Complete(r)
	if err != nil {
		return err
	}
	return nil
}

// handleCreateOnlyMode checks and updates the create-only mode status
func (r *SpireAgentReconciler) handleCreateOnlyMode(agent *v1alpha1.SpireAgent, statusMgr *status.Manager) bool {
	createOnlyMode := utils.IsInCreateOnlyMode(agent, &r.createOnlyMode)
	if createOnlyMode {
		r.log.Info("Running in create-only mode - will create resources if they don't exist but skip updates")
		statusMgr.AddCondition(utils.CreateOnlyModeStatusType, utils.CreateOnlyModeEnabled,
			"Create-only mode is enabled via ztwim.openshift.io/create-only annotation",
			metav1.ConditionTrue)
	} else {
		existingCondition := apimeta.FindStatusCondition(agent.Status.ConditionalStatus.Conditions, utils.CreateOnlyModeStatusType)
		if existingCondition != nil && existingCondition.Status == metav1.ConditionTrue {
			statusMgr.AddCondition(utils.CreateOnlyModeStatusType, utils.CreateOnlyModeDisabled,
				"Create-only mode is disabled",
				metav1.ConditionFalse)
		}
	}
	return createOnlyMode
}

// validateCommonConfig validates common configuration fields (affinity, tolerations, nodeSelector, resources, labels)
// using individual validation functions from the utils package. This approach provides:
//   - Early validation before any resources are created
//   - Specific error messages for each field (e.g., "InvalidAffinity" vs generic "InvalidCommonConfig")
//   - Better user experience by identifying exactly which field has validation errors
//   - Kubernetes-compliant validation (e.g., affinity weights must be 1-100, resource limits >= requests)
//
// Returns an error if any validation fails, which stops reconciliation and sets the appropriate status condition.
func (r *SpireAgentReconciler) validateCommonConfig(agent *v1alpha1.SpireAgent, statusMgr *status.Manager) error {
	// Validate affinity
	if err := utils.ValidateCommonConfigAffinity(agent.Spec.Affinity); err != nil {
		r.log.Error(err, "Affinity validation failed", "name", agent.Name)
		statusMgr.AddCondition("ConfigurationValid", "InvalidAffinity",
			fmt.Sprintf("Affinity validation failed: %v", err),
			metav1.ConditionFalse)
		return fmt.Errorf("SpireAgent/%s affinity validation failed: %w", agent.Name, err)
	}

	// Validate tolerations
	if err := utils.ValidateCommonConfigTolerations(agent.Spec.Tolerations); err != nil {
		r.log.Error(err, "Tolerations validation failed", "name", agent.Name)
		statusMgr.AddCondition("ConfigurationValid", "InvalidTolerations",
			fmt.Sprintf("Tolerations validation failed: %v", err),
			metav1.ConditionFalse)
		return fmt.Errorf("SpireAgent/%s tolerations validation failed: %w", agent.Name, err)
	}

	// Validate node selector
	if err := utils.ValidateCommonConfigNodeSelector(agent.Spec.NodeSelector); err != nil {
		r.log.Error(err, "NodeSelector validation failed", "name", agent.Name)
		statusMgr.AddCondition("ConfigurationValid", "InvalidNodeSelector",
			fmt.Sprintf("NodeSelector validation failed: %v", err),
			metav1.ConditionFalse)
		return fmt.Errorf("SpireAgent/%s node selector validation failed: %w", agent.Name, err)
	}

	// Validate resources
	if err := utils.ValidateCommonConfigResources(agent.Spec.Resources); err != nil {
		r.log.Error(err, "Resources validation failed", "name", agent.Name)
		statusMgr.AddCondition("ConfigurationValid", "InvalidResources",
			fmt.Sprintf("Resources validation failed: %v", err),
			metav1.ConditionFalse)
		return fmt.Errorf("SpireAgent/%s resources validation failed: %w", agent.Name, err)
	}

	// Validate labels
	if err := utils.ValidateCommonConfigLabels(agent.Spec.Labels); err != nil {
		r.log.Error(err, "Labels validation failed", "name", agent.Name)
		statusMgr.AddCondition("ConfigurationValid", "InvalidLabels",
			fmt.Sprintf("Labels validation failed: %v", err),
			metav1.ConditionFalse)
		return fmt.Errorf("SpireAgent/%s labels validation failed: %w", agent.Name, err)
	}

	return nil
}

// needsUpdate returns true if DaemonSet needs to be updated based on config checksum
func needsUpdate(current, desired appsv1.DaemonSet) bool {
	if current.Spec.Template.Annotations[spireAgentDaemonSetSpireAgentConfigHashAnnotationKey] != desired.Spec.Template.Annotations[spireAgentDaemonSetSpireAgentConfigHashAnnotationKey] {
		return true
	} else if utils.DaemonSetSpecModified(&desired, &current) {
		return true
	} else if !equality.Semantic.DeepEqual(current.Labels, desired.Labels) {
		return true
	}
	return false
}
