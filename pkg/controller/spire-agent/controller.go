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
func (r *SpireAgentReconciler) validateCommonConfig(agent *v1alpha1.SpireAgent, statusMgr *status.Manager) error {
	return utils.ValidateAndUpdateStatus(
		r.log,
		statusMgr,
		utils.ResourceKindSpireAgent,
		agent.Name,
		agent.Spec.Affinity,
		agent.Spec.Tolerations,
		agent.Spec.NodeSelector,
		agent.Spec.Resources,
		agent.Spec.Labels,
	)
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
