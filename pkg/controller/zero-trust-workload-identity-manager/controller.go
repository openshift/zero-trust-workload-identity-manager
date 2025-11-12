package zero_trust_workload_identity_manager

import (
	"context"
	"fmt"
	"reflect"
	"strings"

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
	CreateOnlyMode    = "CreateOnlyMode"
)

// Operand state constants for structured state tracking
const (
	OperandStateNotFound         = "NotFound"
	OperandStateInitialReconcile = "InitialReconcile"
	OperandStateReconciling      = "Reconciling"
	OperandStateUnhealthy        = "Unhealthy"
)

// operandStateClassification represents whether an operand is progressing or failed
type operandStateClassification string

const (
	operandProgressing operandStateClassification = "progressing"
	operandFailed      operandStateClassification = "failed"
	operandReady       operandStateClassification = "ready"
)

// classifyOperandState determines whether an operand is progressing, failed, or ready
// based on structured state (Condition.Reason) with fallback to message substring matching
func classifyOperandState(operand v1alpha1.OperandStatus, readyCondition *metav1.Condition) operandStateClassification {
	if utils.StringToBool(operand.Ready) {
		return operandReady
	}

	// 1. Prefer reading from Condition.Reason if available
	if readyCondition != nil && readyCondition.Reason != "" {
		switch readyCondition.Reason {
		// Progressing states - map known reasons to progressing
		case v1alpha1.ReasonInProgress,
			OperandStateNotFound,
			OperandStateInitialReconcile,
			OperandStateReconciling:
			return operandProgressing
		// Failed states - map known failure reasons to failed
		case v1alpha1.ReasonFailed,
			OperandStateUnhealthy:
			return operandFailed
		// Ready state (should be caught above, but included for completeness)
		case v1alpha1.ReasonReady:
			return operandReady
		}
	}

	// 2. Check for known structured states in the Message field
	// These are set by the get*Status functions when CR is not found or reconciling
	switch operand.Message {
	// Progressing cases
	case "CR not found", "Waiting for initial reconciliation", "Reconciling":
		return operandProgressing
	}

	// 3. Compatibility fallback: substring matching for unstructured messages
	// If message contains progressing indicators, treat as progressing
	msg := operand.Message
	if contains(msg, "not found") || contains(msg, "initial") || contains(msg, "reconciling") || contains(msg, "progressing") {
		return operandProgressing
	}

	// 4. Default to failed for any other non-ready state
	return operandFailed
}

// contains performs case-insensitive substring match
func contains(s, substr string) bool {
	return strings.Contains(strings.ToLower(s), strings.ToLower(substr))
}

// ZeroTrustWorkloadIdentityManagerReconciler manages the ZeroTrustWorkloadIdentityManager singleton instance
// and aggregates status from all operand CRs
type ZeroTrustWorkloadIdentityManagerReconciler struct {
	ctrlClient    customClient.CustomCtrlClient
	ctx           context.Context
	eventRecorder record.EventRecorder
	log           logr.Logger
	scheme        *runtime.Scheme
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
		ctrlClient:    c,
		ctx:           context.Background(),
		eventRecorder: mgr.GetEventRecorderFor(utils.ZeroTrustWorkloadIdentityManagerControllerName),
		log:           ctrl.Log.WithName(utils.ZeroTrustWorkloadIdentityManagerControllerName),
		scheme:        mgr.GetScheme(),
	}, nil
}

// setCreateOnlyModeCondition sets the CreateOnlyMode condition on the main CR if any operand has it
func setCreateOnlyModeCondition(statusMgr *status.Manager, anyOperandHasCreateOnlyCondition, anyCreateOnlyModeEnabled bool) {
	if anyOperandHasCreateOnlyCondition {
		if anyCreateOnlyModeEnabled {
			statusMgr.AddCondition(CreateOnlyMode, utils.CreateOnlyModeEnabled,
				"One or more operands have create-only mode enabled",
				metav1.ConditionTrue)
		} else {
			statusMgr.AddCondition(CreateOnlyMode, utils.CreateOnlyModeDisabled,
				"Create-only mode is disabled",
				metav1.ConditionFalse)
		}
	}
}

// setUpgradeableCondition sets the Upgradeable condition based on operand readiness and CreateOnlyMode
// Upgradeable is False only if:
// - CreateOnlyMode is enabled, OR
// - Operand CRs exist but are not ready (failing operands)
// Upgradeable is True if:
// - All operands are ready, OR
// - Operands don't exist yet (will be created during upgrade)
func setUpgradeableCondition(statusMgr *status.Manager, allReady bool, anyCreateOnlyModeEnabled bool, operandStatuses []v1alpha1.OperandStatus) {
	if anyCreateOnlyModeEnabled {
		// CreateOnlyMode prevents updates - not safe to upgrade
		statusMgr.AddCondition(v1alpha1.Upgradeable, v1alpha1.ReasonOperandsNotReady,
			"Not safe to upgrade - create-only mode is enabled on one or more operands",
			metav1.ConditionFalse)
		return
	}

	// Check if any operands exist but are not ready (failing operands)
	var failingOperands []string
	for _, operand := range operandStatuses {
		// Only count operands that exist but are not ready
		// Operands that don't exist (CR not found) are OK for upgrade
		if !utils.StringToBool(operand.Ready) && operand.Message != "CR not found" {
			failingOperands = append(failingOperands, fmt.Sprintf("%s/%s", operand.Kind, operand.Name))
		}
	}

	if len(failingOperands) > 0 {
		// Some operands exist but are failing - not safe to upgrade
		message := fmt.Sprintf("Not safe to upgrade - operands exist but are not ready: %v", failingOperands)
		statusMgr.AddCondition(v1alpha1.Upgradeable, v1alpha1.ReasonOperandsNotReady,
			message,
			metav1.ConditionFalse)
	} else {
		// All operands are either ready or don't exist yet - safe to upgrade
		statusMgr.AddCondition(v1alpha1.Upgradeable, v1alpha1.ReasonReady,
			"All operands are ready or not yet created",
			metav1.ConditionTrue)
	}
}

// Reconcile ensures the ZeroTrustWorkloadIdentityManager 'cluster' instance exists
// and aggregates status from all managed operand CRs
func (r *ZeroTrustWorkloadIdentityManagerReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	r.log.Info(fmt.Sprintf("reconciling %s", utils.ZeroTrustWorkloadIdentityManagerControllerName))
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

	defer func() {
		if err := statusMgr.ApplyStatus(ctx, &config, func() *v1alpha1.ConditionalStatus {
			return &config.Status.ConditionalStatus
		}); err != nil {
			r.log.Error(err, "failed to update status")
		}
	}()

	// Aggregate status from all operand CRs
	result := r.aggregateOperandStatus(ctx)
	config.Status.Operands = result.operandStatuses

	// Set operands availability condition and manually control Ready condition
	if result.allReady {
		// All operands ready
		statusMgr.AddCondition(OperandsAvailable, v1alpha1.ReasonReady,
			"All operand CRs are ready",
			metav1.ConditionTrue)
		// Manually set Ready (don't let status manager auto-aggregate)
		statusMgr.AddCondition(v1alpha1.Ready, v1alpha1.ReasonReady,
			"All components are ready",
			metav1.ConditionTrue)
	} else if result.notCreatedCount > 0 && result.failedCount == 0 {
		// Operands not created or still reconciling - use Progressing for both conditions
		var pendingOperands []string
		for _, operand := range result.operandStatuses {
			// Use structured state classification instead of exact string matching
			readyCondition := apimeta.FindStatusCondition(operand.Conditions, v1alpha1.Ready)
			classification := classifyOperandState(operand, readyCondition)

			if classification == operandProgressing {
				// Differentiate between not created vs reconciling based on message
				if operand.Message == "CR not found" {
					pendingOperands = append(pendingOperands, fmt.Sprintf("%s(not created)", operand.Kind))
				} else {
					pendingOperands = append(pendingOperands, fmt.Sprintf("%s(reconciling)", operand.Kind))
				}
			}
		}
		message := fmt.Sprintf("Waiting for operands: %v", pendingOperands)
		statusMgr.AddCondition(OperandsAvailable, v1alpha1.ReasonInProgress,
			message,
			metav1.ConditionFalse)
		// Manually set Ready with Progressing (waiting for user/reconciliation)
		statusMgr.AddCondition(v1alpha1.Ready, v1alpha1.ReasonInProgress,
			message,
			metav1.ConditionFalse)
	} else {
		// Some operands are actually unhealthy - use Failed
		var unhealthyOperands []string
		for _, operand := range result.operandStatuses {
			// Use structured state classification instead of exact string matching
			readyCondition := apimeta.FindStatusCondition(operand.Conditions, v1alpha1.Ready)
			classification := classifyOperandState(operand, readyCondition)

			if classification == operandFailed {
				unhealthyOperands = append(unhealthyOperands, fmt.Sprintf("%s/%s", operand.Kind, operand.Name))
			}
		}
		// Always set conditions when we have unhealthy operands
		message := fmt.Sprintf("Some operands not ready: %v", unhealthyOperands)
		statusMgr.AddCondition(OperandsAvailable, v1alpha1.ReasonFailed,
			message,
			metav1.ConditionFalse)
		// Manually set Ready with Failed (actual failure)
		statusMgr.AddCondition(v1alpha1.Ready, v1alpha1.ReasonFailed,
			message,
			metav1.ConditionFalse)
	}

	// Set CreateOnlyMode condition if any operand has it
	setCreateOnlyModeCondition(statusMgr, result.anyOperandHasCreateOnlyCondition, result.anyCreateOnlyModeEnabled)

	// Set Upgradeable condition based on operand health and CreateOnlyMode
	setUpgradeableCondition(statusMgr, result.allReady, result.anyCreateOnlyModeEnabled, result.operandStatuses)

	r.log.Info("Aggregated operand status", "allReady", result.allReady, "notCreated", result.notCreatedCount, "failed", result.failedCount, "anyCreateOnlyModeEnabled", result.anyCreateOnlyModeEnabled, "anyOperandHasCreateOnlyCondition", result.anyOperandHasCreateOnlyCondition, "anyOperandExists", result.anyOperandExists)

	return ctrl.Result{}, nil
}

// operandAggregateState holds the aggregate state tracked across all operands
type operandAggregateState struct {
	allReady                         bool
	notCreatedCount                  int
	failedCount                      int
	anyCreateOnlyModeEnabled         bool
	anyOperandHasCreateOnlyCondition bool
	anyOperandExists                 bool
}

// operandAggregateResult holds the result of aggregating operand statuses
type operandAggregateResult struct {
	operandStatuses                  []v1alpha1.OperandStatus
	allReady                         bool
	notCreatedCount                  int
	failedCount                      int
	anyCreateOnlyModeEnabled         bool
	anyOperandHasCreateOnlyCondition bool
	anyOperandExists                 bool
}

// processOperandStatus processes a single operand's status and updates aggregate state
func processOperandStatus(operand v1alpha1.OperandStatus, state *operandAggregateState) {
	// Check if operand exists
	if operand.Message != "CR not found" {
		state.anyOperandExists = true

		// Check if this operand has CreateOnlyMode condition
		createOnlyCondition := apimeta.FindStatusCondition(operand.Conditions, utils.CreateOnlyModeStatusType)
		if createOnlyCondition != nil {
			state.anyOperandHasCreateOnlyCondition = true
			if createOnlyCondition.Status == metav1.ConditionTrue {
				state.anyCreateOnlyModeEnabled = true
			}
		}
	}

	// Check if operand is ready
	if !utils.StringToBool(operand.Ready) {
		state.allReady = false
		// Use structured state classification
		readyCondition := apimeta.FindStatusCondition(operand.Conditions, v1alpha1.Ready)
		classification := classifyOperandState(operand, readyCondition)
		if classification == operandProgressing {
			state.notCreatedCount++
		} else {
			state.failedCount++
		}
	}
}

// aggregateOperandStatus collects status from all managed operand CRs
func (r *ZeroTrustWorkloadIdentityManagerReconciler) aggregateOperandStatus(ctx context.Context) operandAggregateResult {
	// Initialize aggregate state
	state := &operandAggregateState{
		allReady: true,
	}

	// Collect status from all operands
	operandStatuses := []v1alpha1.OperandStatus{
		r.getSpireServerStatus(ctx),
		r.getSpireAgentStatus(ctx),
		r.getSpiffeCSIDriverStatus(ctx),
		r.getSpireOIDCDiscoveryProviderStatus(ctx),
	}

	// Process each operand status
	for _, operand := range operandStatuses {
		processOperandStatus(operand, state)
	}

	return operandAggregateResult{
		operandStatuses:                  operandStatuses,
		allReady:                         state.allReady,
		notCreatedCount:                  state.notCreatedCount,
		failedCount:                      state.failedCount,
		anyCreateOnlyModeEnabled:         state.anyCreateOnlyModeEnabled,
		anyOperandHasCreateOnlyCondition: state.anyOperandHasCreateOnlyCondition,
		anyOperandExists:                 state.anyOperandExists,
	}
}

// operandStatusGetter defines the interface for types that have conditional status
type operandStatusGetter interface {
	client.Object
	GetConditionalStatus() v1alpha1.ConditionalStatus
}

// getOperandStatus is a generic helper that retrieves and summarizes operand status for any CR type
func getOperandStatus[T operandStatusGetter](ctx context.Context, r *ZeroTrustWorkloadIdentityManagerReconciler, kind string) v1alpha1.OperandStatus {
	var obj T
	// Since T is a pointer type, create a new instance of the underlying type
	objValue := reflect.New(reflect.TypeOf(obj).Elem()).Interface().(T)
	err := r.ctrlClient.Get(ctx, types.NamespacedName{Name: "cluster"}, objValue)

	operandStatus := v1alpha1.OperandStatus{
		Name: "cluster",
		Kind: kind,
	}

	if err != nil {
		if errors.IsNotFound(err) {
			operandStatus.Ready = "false"
			operandStatus.Message = "CR not found"
			return operandStatus
		}
		operandStatus.Ready = "false"
		operandStatus.Message = fmt.Sprintf("Failed to get CR: %v", err)
		return operandStatus
	}

	// Get the conditions from the status
	conditionalStatus := objValue.GetConditionalStatus()
	conditions := conditionalStatus.Conditions

	// Check if operand has been reconciled (has at least one condition)
	if len(conditions) == 0 {
		operandStatus.Ready = "false"
		operandStatus.Message = "Waiting for initial reconciliation"
		return operandStatus
	}

	// Check if Ready condition exists and is True
	readyCondition := apimeta.FindStatusCondition(conditions, v1alpha1.Ready)
	if readyCondition != nil && readyCondition.Status == metav1.ConditionTrue {
		operandStatus.Ready = "true"
		operandStatus.Message = "Ready"
	} else {
		operandStatus.Ready = "false"
		if readyCondition != nil {
			operandStatus.Message = readyCondition.Message
		} else {
			operandStatus.Message = "Reconciling"
		}
	}

	// Include only failed conditions (reduces clutter)
	operandStatus.Conditions = extractKeyConditions(conditions, utils.StringToBool(operandStatus.Ready))

	return operandStatus
}

// getSpireServerStatus retrieves and summarizes SpireServer status
func (r *ZeroTrustWorkloadIdentityManagerReconciler) getSpireServerStatus(ctx context.Context) v1alpha1.OperandStatus {
	return getOperandStatus[*v1alpha1.SpireServer](ctx, r, "SpireServer")
}

// getSpireAgentStatus retrieves and summarizes SpireAgent status
func (r *ZeroTrustWorkloadIdentityManagerReconciler) getSpireAgentStatus(ctx context.Context) v1alpha1.OperandStatus {
	return getOperandStatus[*v1alpha1.SpireAgent](ctx, r, "SpireAgent")
}

// getSpiffeCSIDriverStatus retrieves and summarizes SpiffeCSIDriver status
func (r *ZeroTrustWorkloadIdentityManagerReconciler) getSpiffeCSIDriverStatus(ctx context.Context) v1alpha1.OperandStatus {
	return getOperandStatus[*v1alpha1.SpiffeCSIDriver](ctx, r, "SpiffeCSIDriver")
}

// getSpireOIDCDiscoveryProviderStatus retrieves and summarizes SpireOIDCDiscoveryProvider status
func (r *ZeroTrustWorkloadIdentityManagerReconciler) getSpireOIDCDiscoveryProviderStatus(ctx context.Context) v1alpha1.OperandStatus {
	return getOperandStatus[*v1alpha1.SpireOIDCDiscoveryProvider](ctx, r, "SpireOIDCDiscoveryProvider")
}

// extractKeyConditions extracts key conditions from operand status
// Only includes CreateOnlyMode condition when it's enabled (True) - needed for ZTWIM aggregation
// When operand is not ready, also includes Ready condition and other failed conditions
func extractKeyConditions(conditions []metav1.Condition, isReady bool) []metav1.Condition {
	keyConditions := []metav1.Condition{}

	// Only include CreateOnlyMode condition if it's enabled (True)
	// When disabled (False), it's just clutter and not needed
	createOnlyCondition := apimeta.FindStatusCondition(conditions, utils.CreateOnlyModeStatusType)
	if createOnlyCondition != nil && createOnlyCondition.Status == metav1.ConditionTrue {
		keyConditions = append(keyConditions, *createOnlyCondition)
	}

	// If operand is ready and no CreateOnlyMode, return empty (reduces clutter)
	if isReady {
		return keyConditions
	}

	// If operand is not ready, include the Ready condition for structured state classification
	readyCondition := apimeta.FindStatusCondition(conditions, v1alpha1.Ready)
	if readyCondition != nil {
		keyConditions = append(keyConditions, *readyCondition)
	}

	// Also include other failed conditions to show what's wrong
	for _, cond := range conditions {
		// Skip conditions we've already checked
		if cond.Type == v1alpha1.Ready || cond.Type == utils.CreateOnlyModeStatusType {
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
