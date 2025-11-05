package spire_agent

import (
	"context"
	"fmt"
	"reflect"

	securityv1 "github.com/openshift/api/security/v1"
	customClient "github.com/openshift/zero-trust-workload-identity-manager/pkg/client"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"

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
	r.log.Info("reconciling ", utils.ZeroTrustWorkloadIdentityManagerSpireAgentControllerName)
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

	// Set consolidated success status after all static resources are created
	statusMgr.AddCondition(RBACAvailable, v1alpha1.ReasonReady,
		"All RBAC resources available",
		metav1.ConditionTrue)
	statusMgr.AddCondition(ServiceAvailable, v1alpha1.ReasonReady,
		"All Service resources available",
		metav1.ConditionTrue)

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

// reconcileSCC reconciles the Spire Agent Security Context Constraints
func (r *SpireAgentReconciler) reconcileSCC(ctx context.Context, agent *v1alpha1.SpireAgent, statusMgr *status.Manager) error {
	spireAgentSCC := generateSpireAgentSCC(agent)
	if err := controllerutil.SetControllerReference(agent, spireAgentSCC, r.scheme); err != nil {
		r.log.Error(err, "failed to set controller reference")
		statusMgr.AddCondition(SecurityContextConstraintsAvailable, "SpireAgentSCCGenerationFailed",
			err.Error(),
			metav1.ConditionFalse)
		return err
	}

	err := r.ctrlClient.Create(ctx, spireAgentSCC)
	if err != nil && !kerrors.IsAlreadyExists(err) {
		r.log.Error(err, "Failed to create SpireAgentSCC")
		statusMgr.AddCondition(SecurityContextConstraintsAvailable, "SpireAgentSCCGenerationFailed",
			err.Error(),
			metav1.ConditionFalse)
		return err
	}

	statusMgr.AddCondition(SecurityContextConstraintsAvailable, "SpireAgentSCCResourceCreated",
		"Spire Agent SCC resources applied",
		metav1.ConditionTrue)
	return nil
}

// reconcileConfigMap reconciles the Spire Agent ConfigMap
func (r *SpireAgentReconciler) reconcileConfigMap(ctx context.Context, agent *v1alpha1.SpireAgent, statusMgr *status.Manager, createOnlyMode bool) (string, error) {
	spireAgentConfigMap, spireAgentConfigHash, err := GenerateSpireAgentConfigMap(agent)
	if err != nil {
		r.log.Error(err, "failed to generate spire-agent config map")
		statusMgr.AddCondition(ConfigMapAvailable, "SpireAgentConfigMapGenerationFailed",
			err.Error(),
			metav1.ConditionFalse)
		return "", err
	}

	if err = controllerutil.SetControllerReference(agent, spireAgentConfigMap, r.scheme); err != nil {
		r.log.Error(err, "failed to set controller reference")
		statusMgr.AddCondition(ConfigMapAvailable, "SpireAgentConfigMapGenerationFailed",
			err.Error(),
			metav1.ConditionFalse)
		return "", err
	}

	var existingSpireAgentCM corev1.ConfigMap
	err = r.ctrlClient.Get(ctx, types.NamespacedName{Name: spireAgentConfigMap.Name, Namespace: spireAgentConfigMap.Namespace}, &existingSpireAgentCM)
	if err != nil && kerrors.IsNotFound(err) {
		if err = r.ctrlClient.Create(ctx, spireAgentConfigMap); err != nil {
			r.log.Error(err, "failed to create spire-agent config map")
			statusMgr.AddCondition(ConfigMapAvailable, "SpireAgentConfigMapGenerationFailed",
				err.Error(),
				metav1.ConditionFalse)
			return "", fmt.Errorf("failed to create ConfigMap: %w", err)
		}
		r.log.Info("Created spire agent ConfigMap")
	} else if err == nil && (existingSpireAgentCM.Data["agent.conf"] != spireAgentConfigMap.Data["agent.conf"] ||
		!reflect.DeepEqual(existingSpireAgentCM.Labels, spireAgentConfigMap.Labels)) {
		if createOnlyMode {
			r.log.Info("Skipping ConfigMap update due to create-only mode")
		} else {
			spireAgentConfigMap.ResourceVersion = existingSpireAgentCM.ResourceVersion
			if err = r.ctrlClient.Update(ctx, spireAgentConfigMap); err != nil {
				r.log.Error(err, "failed to update spire-agent config map")
				statusMgr.AddCondition(ConfigMapAvailable, "SpireAgentConfigMapGenerationFailed",
					err.Error(),
					metav1.ConditionFalse)
				return "", fmt.Errorf("failed to update ConfigMap: %w", err)
			}
			r.log.Info("Updated ConfigMap with new config")
		}
	} else if err != nil {
		statusMgr.AddCondition(ConfigMapAvailable, "SpireAgentConfigMapGenerationFailed",
			err.Error(),
			metav1.ConditionFalse)
		return "", err
	}

	statusMgr.AddCondition(ConfigMapAvailable, "SpireAgentConfigMapResourceCreated",
		"Spire Agent ConfigMap resources applied",
		metav1.ConditionTrue)

	return spireAgentConfigHash, nil
}

// reconcileDaemonSet reconciles the Spire Agent DaemonSet
func (r *SpireAgentReconciler) reconcileDaemonSet(ctx context.Context, agent *v1alpha1.SpireAgent, statusMgr *status.Manager, createOnlyMode bool, configHash string) error {
	spireAgentDaemonset := generateSpireAgentDaemonSet(agent.Spec, configHash)
	if err := controllerutil.SetControllerReference(agent, spireAgentDaemonset, r.scheme); err != nil {
		r.log.Error(err, "failed to set controller reference")
		statusMgr.AddCondition(DaemonSetAvailable, "SpireAgentDaemonSetGenerationFailed",
			err.Error(),
			metav1.ConditionFalse)
		return err
	}

	var existingSpireAgentDaemonSet appsv1.DaemonSet
	err := r.ctrlClient.Get(ctx, types.NamespacedName{Name: spireAgentDaemonset.Name, Namespace: spireAgentDaemonset.Namespace}, &existingSpireAgentDaemonSet)
	if err != nil && kerrors.IsNotFound(err) {
		if err = r.ctrlClient.Create(ctx, spireAgentDaemonset); err != nil {
			r.log.Error(err, "failed to create spire-agent daemonset")
			statusMgr.AddCondition(DaemonSetAvailable, "SpireAgentDaemonSetCreationFailed",
				err.Error(),
				metav1.ConditionFalse)
			return fmt.Errorf("failed to create DaemonSet: %w", err)
		}
		r.log.Info("Created spire agent DaemonSet")
	} else if err == nil && needsUpdate(existingSpireAgentDaemonSet, *spireAgentDaemonset) {
		if createOnlyMode {
			r.log.Info("Skipping DaemonSet update due to create-only mode")
		} else {
			spireAgentDaemonset.ResourceVersion = existingSpireAgentDaemonSet.ResourceVersion
			if err = r.ctrlClient.Update(ctx, spireAgentDaemonset); err != nil {
				r.log.Error(err, "failed to update spire agent DaemonSet")
				statusMgr.AddCondition(DaemonSetAvailable, "SpireAgentDaemonSetUpdateFailed",
					err.Error(),
					metav1.ConditionFalse)
				return fmt.Errorf("failed to update DaemonSet: %w", err)
			}
			r.log.Info("Updated spire agent DaemonSet")
		}
	} else if err != nil {
		r.log.Error(err, "failed to get spire-agent daemonset")
		statusMgr.AddCondition(DaemonSetAvailable, "SpireAgentDaemonSetGetFailed",
			err.Error(),
			metav1.ConditionFalse)
		return err
	}

	// Check DaemonSet health/readiness
	statusMgr.CheckDaemonSetHealth(ctx, spireAgentDaemonset.Name, spireAgentDaemonset.Namespace, DaemonSetAvailable)

	return nil
}

// needsUpdate returns true if DaemonSet needs to be updated based on config checksum
func needsUpdate(current, desired appsv1.DaemonSet) bool {
	if current.Spec.Template.Annotations[spireAgentDaemonSetSpireAgentConfigHashAnnotationKey] != desired.Spec.Template.Annotations[spireAgentDaemonSetSpireAgentConfigHashAnnotationKey] {
		return true
	} else if utils.DaemonSetSpecModified(&desired, &current) {
		return true
	} else if !reflect.DeepEqual(current.Labels, desired.Labels) {
		return true
	}
	return false
}
