package zero_trust_workload_identity_manager

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/api/errors"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"

	"k8s.io/client-go/tools/record"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
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
	// Condition types for ZTWIM
	OperandsAvailable = "OperandsAvailable"
)

// ZeroTrustWorkloadIdentityManagerReconciler manages the ZeroTrustWorkloadIdentityManager singleton instance
// and aggregates status from all operand CRs
type ZeroTrustWorkloadIdentityManagerReconciler struct {
	ctrlClient     customClient.CustomCtrlClient
	ctx            context.Context
	eventRecorder  record.EventRecorder
	log            logr.Logger
	scheme         *runtime.Scheme
	createOnlyMode bool
}

// +kubebuilder:rbac:groups=operator.openshift.io,resources=zerotrustworkloadidentitymanagers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=operator.openshift.io,resources=zerotrustworkloadidentitymanagers/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=operator.openshift.io,resources=zerotrustworkloadidentitymanagers/finalizers,verbs=update
// +kubebuilder:rbac:groups=operator.openshift.io,resources=spiffecsidrivers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=operator.openshift.io,resources=spiffecsidrivers/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=operator.openshift.io,resources=spiffecsidrivers/finalizers,verbs=update
// +kubebuilder:rbac:groups=operator.openshift.io,resources=spireagents,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=operator.openshift.io,resources=spireagents/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=operator.openshift.io,resources=spireagents/finalizers,verbs=update
// +kubebuilder:rbac:groups=operator.openshift.io,resources=spireoidcdiscoveryproviders,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=operator.openshift.io,resources=spireoidcdiscoveryproviders/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=operator.openshift.io,resources=spireoidcdiscoveryproviders/finalizers,verbs=update
// +kubebuilder:rbac:groups=operator.openshift.io,resources=spireservers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=operator.openshift.io,resources=spireservers/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=operator.openshift.io,resources=spireservers/finalizers,verbs=update
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=clusterrolebindings,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=clusterroles,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=rolebindings,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=roles,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=admissionregistration.k8s.io,resources=validatingwebhookconfigurations,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=services,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=serviceaccounts,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=configmaps,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=events,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=namespaces,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=nodes,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=pods,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=nodes/proxy,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=endpoints,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=storage.k8s.io,resources=csidrivers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=authentication.k8s.io,resources=tokenreviews,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=spire.spiffe.io,resources=clusterfederatedtrustdomains,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=spire.spiffe.io,resources=clusterfederatedtrustdomains/finalizers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=spire.spiffe.io,resources=clusterfederatedtrustdomains/status,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=spire.spiffe.io,resources=clusterspiffeids,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=spire.spiffe.io,resources=clusterspiffeids/finalizers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=spire.spiffe.io,resources=clusterspiffeids/status,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=spire.spiffe.io,resources=clusterstaticentries,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=spire.spiffe.io,resources=clusterstaticentries/finalizers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=spire.spiffe.io,resources=clusterstaticentries/status,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apps,resources=daemonsets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apps,resources=statefulsets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=security.openshift.io,resources=securitycontextconstraints,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=coordination.k8s.io,resources=leases,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=route.openshift.io,resources=routes,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=route.openshift.io,resources=routes/custom-host,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=events;secrets,verbs=get;list;watch

// New returns a new Reconciler instance.
func New(mgr ctrl.Manager) (*ZeroTrustWorkloadIdentityManagerReconciler, error) {
	c, err := customClient.NewCustomClient(mgr)
	if err != nil {
		return nil, err
	}
	return &ZeroTrustWorkloadIdentityManagerReconciler{
		ctrlClient:     c,
		ctx:            context.Background(),
		eventRecorder:  mgr.GetEventRecorderFor(utils.ZeroTrustWorkloadIdentityManagerControllerName),
		log:            ctrl.Log.WithName(utils.ZeroTrustWorkloadIdentityManagerControllerName),
		scheme:         mgr.GetScheme(),
		createOnlyMode: false,
	}, nil
}

// Reconcile ensures the ZeroTrustWorkloadIdentityManager 'cluster' instance exists
// and aggregates status from all managed operand CRs
func (r *ZeroTrustWorkloadIdentityManagerReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	r.log.Info("reconciling ", utils.ZeroTrustWorkloadIdentityManagerControllerName)
	var config v1alpha1.ZeroTrustWorkloadIdentityManager
	err := r.ctrlClient.Get(ctx, req.NamespacedName, &config)
	if err != nil {
		if errors.IsNotFound(err) {
			// Ensure the 'cluster' instance always exists
			if req.Name == "cluster" {
				return r.recreateClusterInstance(ctx, req.Name)
			}
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	// Set Ready to false at the start of reconciliation
	status.SetInitialReconciliationStatus(ctx, r.ctrlClient, &config, func() *v1alpha1.ConditionalStatus {
		return &config.Status.ConditionalStatus
	}, "ZeroTrustWorkloadIdentityManager")

	statusMgr := status.NewManager(r.ctrlClient)

	// For ZTWIM, we manually control the Ready condition instead of auto-aggregation
	// This allows us to use "Progressing" when waiting for user to create CRs
	manualReadyControl := true

	defer func() {
		if err := statusMgr.ApplyStatus(ctx, &config, func() *v1alpha1.ConditionalStatus {
			return &config.Status.ConditionalStatus
		}); err != nil {
			r.log.Error(err, "failed to update status")
		}
	}()

	// Aggregate status from all operand CRs
	operandStatuses, allReady, notCreatedCount, failedCount := r.aggregateOperandStatus(ctx)
	config.Status.Operands = operandStatuses

	// Set operands availability condition and manually control Ready condition
	if allReady {
		// All operands ready
		statusMgr.AddCondition(OperandsAvailable, v1alpha1.ReasonReady,
			"All operand CRs are ready",
			metav1.ConditionTrue)
		// Manually set Ready (don't let status manager auto-aggregate)
		if manualReadyControl {
			statusMgr.AddCondition(v1alpha1.Ready, v1alpha1.ReasonReady,
				"All components are ready",
				metav1.ConditionTrue)
		}
	} else if notCreatedCount > 0 && failedCount == 0 {
		// Operands not created or still reconciling - use Progressing for both conditions
		var pendingOperands []string
		for _, operand := range operandStatuses {
			if operand.Message == "CR not found" {
				pendingOperands = append(pendingOperands, fmt.Sprintf("%s(not created)", operand.Kind))
			} else if operand.Message == "Waiting for initial reconciliation" || operand.Message == "Reconciling" {
				pendingOperands = append(pendingOperands, fmt.Sprintf("%s(reconciling)", operand.Kind))
			}
		}
		message := fmt.Sprintf("Waiting for operands: %v", pendingOperands)
		statusMgr.AddCondition(OperandsAvailable, v1alpha1.ReasonInProgress,
			message,
			metav1.ConditionFalse)
		// Manually set Ready with Progressing (waiting for user/reconciliation)
		if manualReadyControl {
			statusMgr.AddCondition(v1alpha1.Ready, v1alpha1.ReasonInProgress,
				message,
				metav1.ConditionFalse)
		}
	} else {
		// Some operands are actually unhealthy - use Failed
		var unhealthyOperands []string
		for _, operand := range operandStatuses {
			if !operand.Ready && operand.Message != "CR not found" && operand.Message != "Waiting for initial reconciliation" && operand.Message != "Reconciling" {
				unhealthyOperands = append(unhealthyOperands, fmt.Sprintf("%s/%s", operand.Kind, operand.Name))
			}
		}
		if len(unhealthyOperands) > 0 {
			message := fmt.Sprintf("Some operands not ready: %v", unhealthyOperands)
			statusMgr.AddCondition(OperandsAvailable, v1alpha1.ReasonFailed,
				message,
				metav1.ConditionFalse)
			// Manually set Ready with Failed (actual failure)
			if manualReadyControl {
				statusMgr.AddCondition(v1alpha1.Ready, v1alpha1.ReasonFailed,
					message,
					metav1.ConditionFalse)
			}
		}
	}

	r.log.Info("Aggregated operand status", "allReady", allReady, "notCreated", notCreatedCount, "failed", failedCount)

	return ctrl.Result{}, nil
}

// aggregateOperandStatus collects status from all managed operand CRs
// Returns: operandStatuses, allReady, notCreatedCount, failedCount
func (r *ZeroTrustWorkloadIdentityManagerReconciler) aggregateOperandStatus(ctx context.Context) ([]v1alpha1.OperandStatus, bool, int, int) {
	operandStatuses := []v1alpha1.OperandStatus{}
	allReady := true
	notCreatedCount := 0
	failedCount := 0

	// Check SpireServer
	spireServerStatus := r.getSpireServerStatus(ctx)

	operandStatuses = append(operandStatuses, spireServerStatus)
	if !spireServerStatus.Ready {
		allReady = false
		if spireServerStatus.Message == "CR not found" {
			notCreatedCount++
		} else if spireServerStatus.Message == "Waiting for initial reconciliation" || spireServerStatus.Message == "Reconciling" {
			notCreatedCount++
		} else {
			failedCount++
		}
	}

	// Check SpireAgent
	spireAgentStatus := r.getSpireAgentStatus(ctx)

	operandStatuses = append(operandStatuses, spireAgentStatus)
	if !spireAgentStatus.Ready {
		allReady = false
		if spireAgentStatus.Message == "CR not found" {
			notCreatedCount++
		} else if spireAgentStatus.Message == "Waiting for initial reconciliation" || spireAgentStatus.Message == "Reconciling" {
			notCreatedCount++
		} else {
			failedCount++
		}
	}

	// Check SpiffeCSIDriver
	spiffeCSIStatus := r.getSpiffeCSIDriverStatus(ctx)

	operandStatuses = append(operandStatuses, spiffeCSIStatus)
	if !spiffeCSIStatus.Ready {
		allReady = false
		if spiffeCSIStatus.Message == "CR not found" {
			notCreatedCount++
		} else if spiffeCSIStatus.Message == "Waiting for initial reconciliation" || spiffeCSIStatus.Message == "Reconciling" {
			notCreatedCount++
		} else {
			failedCount++
		}
	}

	// Check SpireOIDCDiscoveryProvider
	oidcStatus := r.getSpireOIDCDiscoveryProviderStatus(ctx)

	operandStatuses = append(operandStatuses, oidcStatus)
	if !oidcStatus.Ready {
		allReady = false
		if oidcStatus.Message == "CR not found" {
			notCreatedCount++
		} else if oidcStatus.Message == "Waiting for initial reconciliation" || oidcStatus.Message == "Reconciling" {
			notCreatedCount++
		} else {
			failedCount++
		}
	}

	return operandStatuses, allReady, notCreatedCount, failedCount
}

// getSpireServerStatus retrieves and summarizes SpireServer status
func (r *ZeroTrustWorkloadIdentityManagerReconciler) getSpireServerStatus(ctx context.Context) v1alpha1.OperandStatus {
	var server v1alpha1.SpireServer
	err := r.ctrlClient.Get(ctx, types.NamespacedName{Name: "cluster"}, &server)

	operandStatus := v1alpha1.OperandStatus{
		Name: "cluster",
		Kind: "SpireServer",
	}

	if err != nil {
		if errors.IsNotFound(err) {
			operandStatus.Ready = false
			operandStatus.Message = "CR not found"
			return operandStatus
		}
		operandStatus.Ready = false
		operandStatus.Message = fmt.Sprintf("Failed to get CR: %v", err)
		return operandStatus
	}

	// Check if operand has been reconciled (has at least one condition)
	if len(server.Status.Conditions) == 0 {
		operandStatus.Ready = false
		operandStatus.Message = "Waiting for initial reconciliation"
		return operandStatus
	}

	// Check if Ready condition exists and is True
	readyCondition := apimeta.FindStatusCondition(server.Status.Conditions, v1alpha1.Ready)
	if readyCondition != nil && readyCondition.Status == metav1.ConditionTrue {
		operandStatus.Ready = true
		operandStatus.Message = "Ready"
	} else {
		operandStatus.Ready = false
		if readyCondition != nil {
			operandStatus.Message = readyCondition.Message
		} else {
			operandStatus.Message = "Reconciling"
		}
	}

	// Include only failed conditions (reduces clutter)
	operandStatus.Conditions = extractKeyConditions(server.Status.Conditions, operandStatus.Ready)

	return operandStatus
}

// getSpireAgentStatus retrieves and summarizes SpireAgent status
func (r *ZeroTrustWorkloadIdentityManagerReconciler) getSpireAgentStatus(ctx context.Context) v1alpha1.OperandStatus {
	var agent v1alpha1.SpireAgent
	err := r.ctrlClient.Get(ctx, types.NamespacedName{Name: "cluster"}, &agent)

	operandStatus := v1alpha1.OperandStatus{
		Name: "cluster",
		Kind: "SpireAgent",
	}

	if err != nil {
		if errors.IsNotFound(err) {
			operandStatus.Ready = false
			operandStatus.Message = "CR not found"
			return operandStatus
		}
		operandStatus.Ready = false
		operandStatus.Message = fmt.Sprintf("Failed to get CR: %v", err)
		return operandStatus
	}

	// Check if operand has been reconciled
	if len(agent.Status.Conditions) == 0 {
		operandStatus.Ready = false
		operandStatus.Message = "Waiting for initial reconciliation"
		return operandStatus
	}

	// Check Ready condition
	readyCondition := apimeta.FindStatusCondition(agent.Status.Conditions, v1alpha1.Ready)
	if readyCondition != nil && readyCondition.Status == metav1.ConditionTrue {
		operandStatus.Ready = true
		operandStatus.Message = "Ready"
	} else {
		operandStatus.Ready = false
		if readyCondition != nil {
			operandStatus.Message = readyCondition.Message
		} else {
			operandStatus.Message = "Reconciling"
		}
	}

	operandStatus.Conditions = extractKeyConditions(agent.Status.Conditions, operandStatus.Ready)

	return operandStatus
}

// getSpiffeCSIDriverStatus retrieves and summarizes SpiffeCSIDriver status
func (r *ZeroTrustWorkloadIdentityManagerReconciler) getSpiffeCSIDriverStatus(ctx context.Context) v1alpha1.OperandStatus {
	var driver v1alpha1.SpiffeCSIDriver
	err := r.ctrlClient.Get(ctx, types.NamespacedName{Name: "cluster"}, &driver)

	operandStatus := v1alpha1.OperandStatus{
		Name: "cluster",
		Kind: "SpiffeCSIDriver",
	}

	if err != nil {
		if errors.IsNotFound(err) {
			operandStatus.Ready = false
			operandStatus.Message = "CR not found"
			return operandStatus
		}
		operandStatus.Ready = false
		operandStatus.Message = fmt.Sprintf("Failed to get CR: %v", err)
		return operandStatus
	}

	// Check if operand has been reconciled
	if len(driver.Status.Conditions) == 0 {
		operandStatus.Ready = false
		operandStatus.Message = "Waiting for initial reconciliation"
		return operandStatus
	}

	// Check Ready condition
	readyCondition := apimeta.FindStatusCondition(driver.Status.Conditions, v1alpha1.Ready)
	if readyCondition != nil && readyCondition.Status == metav1.ConditionTrue {
		operandStatus.Ready = true
		operandStatus.Message = "Ready"
	} else {
		operandStatus.Ready = false
		if readyCondition != nil {
			operandStatus.Message = readyCondition.Message
		} else {
			operandStatus.Message = "Reconciling"
		}
	}

	operandStatus.Conditions = extractKeyConditions(driver.Status.Conditions, operandStatus.Ready)

	return operandStatus
}

// getSpireOIDCDiscoveryProviderStatus retrieves and summarizes SpireOIDCDiscoveryProvider status
func (r *ZeroTrustWorkloadIdentityManagerReconciler) getSpireOIDCDiscoveryProviderStatus(ctx context.Context) v1alpha1.OperandStatus {
	var oidc v1alpha1.SpireOIDCDiscoveryProvider
	err := r.ctrlClient.Get(ctx, types.NamespacedName{Name: "cluster"}, &oidc)

	operandStatus := v1alpha1.OperandStatus{
		Name: "cluster",
		Kind: "SpireOIDCDiscoveryProvider",
	}

	if err != nil {
		if errors.IsNotFound(err) {
			operandStatus.Ready = false
			operandStatus.Message = "CR not found"
			return operandStatus
		}
		operandStatus.Ready = false
		operandStatus.Message = fmt.Sprintf("Failed to get CR: %v", err)
		return operandStatus
	}

	// Check if operand has been reconciled
	if len(oidc.Status.Conditions) == 0 {
		operandStatus.Ready = false
		operandStatus.Message = "Waiting for initial reconciliation"
		return operandStatus
	}

	// Check Ready condition
	readyCondition := apimeta.FindStatusCondition(oidc.Status.Conditions, v1alpha1.Ready)
	if readyCondition != nil && readyCondition.Status == metav1.ConditionTrue {
		operandStatus.Ready = true
		operandStatus.Message = "Ready"
	} else {
		operandStatus.Ready = false
		if readyCondition != nil {
			operandStatus.Message = readyCondition.Message
		} else {
			operandStatus.Message = "Reconciling"
		}
	}

	operandStatus.Conditions = extractKeyConditions(oidc.Status.Conditions, operandStatus.Ready)

	return operandStatus
}

// extractKeyConditions extracts only failed conditions from operand status
// When operand is healthy, no need to show conditions (reduces clutter)
// When operand is unhealthy, show what's wrong
func extractKeyConditions(conditions []metav1.Condition, isReady bool) []metav1.Condition {
	keyConditions := []metav1.Condition{}

	// If operand is ready, don't show any conditions (reduces clutter)
	if isReady {
		return keyConditions
	}

	// If operand is not ready, show only failed conditions
	for _, cond := range conditions {
		// Skip the overall Ready condition (message already shows it)
		if cond.Type == v1alpha1.Ready {
			continue
		}

		// Include any Failed conditions to show what's wrong
		if cond.Status == metav1.ConditionFalse {
			keyConditions = append(keyConditions, cond)
		}
	}

	return keyConditions
}

// recreateClusterInstance recreates the cluster instance if it was deleted
func (r *ZeroTrustWorkloadIdentityManagerReconciler) recreateClusterInstance(ctx context.Context, name string) (ctrl.Result, error) {
	r.log.Info("Recreating ZeroTrustWorkloadIdentityManager 'cluster' as it was deleted")
	newConfig := &v1alpha1.ZeroTrustWorkloadIdentityManager{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	}
	if err := r.ctrlClient.Create(ctx, newConfig); err != nil {
		r.log.Error(err, "failed to recreate ZeroTrustWorkloadIdentityManager 'cluster'")
		return ctrl.Result{}, err
	}
	return ctrl.Result{Requeue: true}, nil
}

// operandStatusChangedPredicate only triggers reconciliation when operand status changes
// This prevents unnecessary reconciliations when only spec changes
var operandStatusChangedPredicate = predicate.Funcs{
	CreateFunc: func(e event.CreateEvent) bool {
		// Always reconcile on create
		return true
	},
	UpdateFunc: func(e event.UpdateEvent) bool {
		// Only reconcile if status changed
		oldObj, okOld := e.ObjectOld.(interface{ GetStatus() interface{} })
		newObj, okNew := e.ObjectNew.(interface{ GetStatus() interface{} })

		if !okOld || !okNew {
			// If we can't get status, reconcile to be safe
			return true
		}

		// Check if status has changed
		oldStatus := fmt.Sprintf("%+v", oldObj.GetStatus())
		newStatus := fmt.Sprintf("%+v", newObj.GetStatus())

		return oldStatus != newStatus
	},
	DeleteFunc: func(e event.DeleteEvent) bool {
		// Always reconcile on delete
		return true
	},
	GenericFunc: func(e event.GenericEvent) bool {
		return false
	},
}

func (r *ZeroTrustWorkloadIdentityManagerReconciler) SetupWithManager(mgr ctrl.Manager) error {
	// Always enqueue the "cluster" CR for reconciliation when any operand status changes
	mapFunc := func(ctx context.Context, _ client.Object) []reconcile.Request {
		return []reconcile.Request{
			{
				NamespacedName: types.NamespacedName{
					Name: "cluster",
				},
			},
		}
	}

	// Watch ZTWIM CR and all operand CRs to aggregate their status
	// Reconcile on operand creation and status changes
	err := ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.ZeroTrustWorkloadIdentityManager{}, builder.WithPredicates(predicate.GenerationChangedPredicate{})).
		Named(utils.ZeroTrustWorkloadIdentityManagerControllerName).
		Watches(&v1alpha1.SpireServer{}, handler.EnqueueRequestsFromMapFunc(mapFunc), builder.WithPredicates(operandStatusChangedPredicate)).
		Watches(&v1alpha1.SpireAgent{}, handler.EnqueueRequestsFromMapFunc(mapFunc), builder.WithPredicates(operandStatusChangedPredicate)).
		Watches(&v1alpha1.SpiffeCSIDriver{}, handler.EnqueueRequestsFromMapFunc(mapFunc), builder.WithPredicates(operandStatusChangedPredicate)).
		Watches(&v1alpha1.SpireOIDCDiscoveryProvider{}, handler.EnqueueRequestsFromMapFunc(mapFunc), builder.WithPredicates(operandStatusChangedPredicate)).
		Complete(r)
	if err != nil {
		return err
	}
	return nil
}
