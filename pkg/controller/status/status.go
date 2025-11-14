package status

import (
	"context"
	"fmt"

	"github.com/openshift/zero-trust-workload-identity-manager/pkg/controller/utils"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/openshift/zero-trust-workload-identity-manager/api/v1alpha1"
	customClient "github.com/openshift/zero-trust-workload-identity-manager/pkg/client"
)

// Condition represents a status condition with its details
type Condition struct {
	Type    string
	Status  metav1.ConditionStatus
	Reason  string
	Message string
}

// Manager handles status updates for operand resources
type Manager struct {
	customClient customClient.CustomCtrlClient
	conditions   map[string]Condition
}

// NewManager creates a new status manager
func NewManager(customClient customClient.CustomCtrlClient) *Manager {
	return &Manager{
		customClient: customClient,
		conditions:   make(map[string]Condition),
	}
}

// SetInitialReconciliationStatus sets the Ready condition to false at the start of reconciliation.
// This ensures that during operator upgrades, the status correctly reflects that reconciliation
// is in progress. The status is applied immediately before any reconciliation logic runs.
func SetInitialReconciliationStatus(ctx context.Context, customClient customClient.CustomCtrlClient, obj client.Object, getStatus func() *v1alpha1.ConditionalStatus, resourceName string) {
	initialStatusMgr := NewManager(customClient)
	initialStatusMgr.AddCondition(v1alpha1.Ready, v1alpha1.ReasonInProgress,
		fmt.Sprintf("Reconciling %s", resourceName),
		metav1.ConditionFalse)
	if err := initialStatusMgr.ApplyStatus(ctx, obj, getStatus); err != nil {
		fmt.Printf("cannot apply the initial status %v", err)
	}
}

// AddCondition adds or updates a condition
func (m *Manager) AddCondition(conditionType, reason, message string, status metav1.ConditionStatus) {
	m.conditions[conditionType] = Condition{
		Type:    conditionType,
		Status:  status,
		Reason:  reason,
		Message: message,
	}
}

// GetCondition retrieves a condition from the manager before it's applied
func (m *Manager) GetCondition(conditionType string) *Condition {
	if cond, exists := m.conditions[conditionType]; exists {
		return &cond
	}
	return nil
}

// SetReadyCondition sets the Ready condition based on all other conditions
// Distinguishes between "Progressing" (normal startup/rollout) and "Failed" (actual errors)
func (m *Manager) SetReadyCondition() {
	// Check if any condition (except Ready, Degraded, and CreateOnlyMode) is False
	// Note: CreateOnlyMode=False is normal (disabled state), not a failure
	hasProgressing := false
	hasFailure := false
	failureMessages := []string{}
	progressingMessages := []string{}

	// Reasons that indicate normal progress (not actual failures)
	progressingReasons := map[string]bool{
		"StatefulSetNotReady": true,
		"DaemonSetNotReady":   true,
		"DeploymentNotReady":  true,
	}

	for condType, cond := range m.conditions {
		// Skip conditions that don't indicate operational health
		if condType == v1alpha1.Ready || condType == v1alpha1.Degraded || condType == utils.CreateOnlyModeStatusType {
			continue
		}
		if cond.Status == metav1.ConditionFalse {
			// Check if this is a progressing reason or an actual failure
			if progressingReasons[cond.Reason] {
				hasProgressing = true
				progressingMessages = append(progressingMessages, fmt.Sprintf("%s: %s", condType, cond.Message))
			} else {
				hasFailure = true
				failureMessages = append(failureMessages, fmt.Sprintf("%s: %s", condType, cond.Message))
			}
		}
	}

	if hasFailure {
		// Actual failure - use Failed reason
		message := "One or more components are not ready"
		if len(failureMessages) > 0 {
			message = failureMessages[0] // Show the first failure
		}
		m.AddCondition(v1alpha1.Ready, v1alpha1.ReasonFailed, message, metav1.ConditionFalse)
	} else if hasProgressing {
		// Normal startup/rollout - use Progressing reason
		message := "Components are starting up or rolling out"
		if len(progressingMessages) > 0 {
			message = progressingMessages[0] // Show the first progressing component
		}
		m.AddCondition(v1alpha1.Ready, v1alpha1.ReasonInProgress, message, metav1.ConditionFalse)
	} else {
		m.AddCondition(v1alpha1.Ready, v1alpha1.ReasonReady, "All components are ready", metav1.ConditionTrue)
	}
}

// ApplyStatus applies all collected conditions to the given resource status
func (m *Manager) ApplyStatus(ctx context.Context, obj client.Object, getStatus func() *v1alpha1.ConditionalStatus) error {
	status := getStatus()
	originalStatus := status.DeepCopy()

	if status.Conditions == nil {
		status.Conditions = []metav1.Condition{}
	}

	// Only auto-set Ready condition if it hasn't been manually set
	// Check if Ready condition was explicitly added by the controller
	_, readyExplicitlySet := m.conditions[v1alpha1.Ready]
	if !readyExplicitlySet {
		// Set the Ready condition based on all other conditions
		m.SetReadyCondition()
	}

	// Apply all conditions
	for _, cond := range m.conditions {
		newCondition := metav1.Condition{
			Type:               cond.Type,
			Status:             cond.Status,
			Reason:             cond.Reason,
			Message:            cond.Message,
			LastTransitionTime: metav1.Now(),
		}
		apimeta.SetStatusCondition(&status.Conditions, newCondition)
	}

	// Only update if status has changed
	if !equality.Semantic.DeepEqual(originalStatus, status) {
		if err := m.customClient.StatusUpdateWithRetry(ctx, obj); err != nil {
			return fmt.Errorf("failed to update status: %w", err)
		}
	}

	return nil
}

// CheckStatefulSetHealth checks the health of a StatefulSet and adds conditions
func (m *Manager) CheckStatefulSetHealth(ctx context.Context, name, namespace, conditionType string) {
	var sts appsv1.StatefulSet
	err := m.customClient.Get(ctx, client.ObjectKey{Name: name, Namespace: namespace}, &sts)
	if err != nil {
		m.AddCondition(conditionType, "StatefulSetNotFound",
			fmt.Sprintf("Failed to get StatefulSet %s/%s: %v", namespace, name, err),
			metav1.ConditionFalse)
		return
	}

	// Check if StatefulSet is healthy
	if !IsStatefulSetHealthy(&sts) {
		message := GetStatefulSetStatusMessage(&sts)
		m.AddCondition(conditionType, "StatefulSetNotReady", message, metav1.ConditionFalse)
		return
	}

	m.AddCondition(conditionType, "StatefulSetReady",
		fmt.Sprintf("StatefulSet %s is healthy with %d/%d replicas ready",
			name, sts.Status.ReadyReplicas, *sts.Spec.Replicas),
		metav1.ConditionTrue)
}

// CheckDaemonSetHealth checks the health of a DaemonSet and adds conditions
func (m *Manager) CheckDaemonSetHealth(ctx context.Context, name, namespace, conditionType string) {
	var ds appsv1.DaemonSet
	err := m.customClient.Get(ctx, client.ObjectKey{Name: name, Namespace: namespace}, &ds)
	if err != nil {
		m.AddCondition(conditionType, "DaemonSetNotFound",
			fmt.Sprintf("Failed to get DaemonSet %s/%s: %v", namespace, name, err),
			metav1.ConditionFalse)
		return
	}

	// Check if DaemonSet is healthy
	if !IsDaemonSetHealthy(&ds) {
		message := GetDaemonSetStatusMessage(&ds)
		m.AddCondition(conditionType, "DaemonSetNotReady", message, metav1.ConditionFalse)
		return
	}

	m.AddCondition(conditionType, "DaemonSetReady",
		fmt.Sprintf("DaemonSet %s is healthy with %d/%d pods ready",
			name, ds.Status.NumberReady, ds.Status.DesiredNumberScheduled),
		metav1.ConditionTrue)
}

// CheckDeploymentHealth checks the health of a Deployment and adds conditions
func (m *Manager) CheckDeploymentHealth(ctx context.Context, name, namespace, conditionType string) {
	var deploy appsv1.Deployment
	err := m.customClient.Get(ctx, client.ObjectKey{Name: name, Namespace: namespace}, &deploy)
	if err != nil {
		m.AddCondition(conditionType, "DeploymentNotFound",
			fmt.Sprintf("Failed to get Deployment %s/%s: %v", namespace, name, err),
			metav1.ConditionFalse)
		return
	}

	// Check if Deployment is healthy
	if !IsDeploymentHealthy(&deploy) {
		message := GetDeploymentStatusMessage(&deploy)
		m.AddCondition(conditionType, "DeploymentNotReady", message, metav1.ConditionFalse)
		return
	}

	m.AddCondition(conditionType, "DeploymentReady",
		fmt.Sprintf("Deployment %s is healthy with %d/%d replicas ready",
			name, deploy.Status.ReadyReplicas, *deploy.Spec.Replicas),
		metav1.ConditionTrue)
}

// IsStatefulSetHealthy checks if a StatefulSet is healthy
func IsStatefulSetHealthy(sts *appsv1.StatefulSet) bool {
	if sts == nil || sts.Spec.Replicas == nil {
		return false
	}

	desiredReplicas := *sts.Spec.Replicas

	// Check if all replicas are ready
	if sts.Status.ReadyReplicas != desiredReplicas {
		return false
	}

	// Check if all replicas are updated
	if sts.Status.UpdatedReplicas != desiredReplicas {
		return false
	}

	// Check if the observed generation matches
	if sts.Status.ObservedGeneration != sts.Generation {
		return false
	}

	return true
}

// IsDaemonSetHealthy checks if a DaemonSet is healthy
func IsDaemonSetHealthy(ds *appsv1.DaemonSet) bool {
	if ds == nil {
		return false
	}

	desiredScheduled := ds.Status.DesiredNumberScheduled

	// Must have at least one pod scheduled
	if desiredScheduled == 0 {
		return false
	}

	// Check if all desired pods are ready
	if ds.Status.NumberReady != desiredScheduled {
		return false
	}

	// Check if all pods are up to date
	if ds.Status.UpdatedNumberScheduled != desiredScheduled {
		return false
	}

	// Check if all pods are available
	if ds.Status.NumberAvailable != desiredScheduled {
		return false
	}

	// Check if the observed generation matches
	if ds.Status.ObservedGeneration != ds.Generation {
		return false
	}

	return true
}

// IsDeploymentHealthy checks if a Deployment is healthy
func IsDeploymentHealthy(deploy *appsv1.Deployment) bool {
	if deploy == nil || deploy.Spec.Replicas == nil {
		return false
	}

	desiredReplicas := *deploy.Spec.Replicas

	// Check if all replicas are ready
	if deploy.Status.ReadyReplicas != desiredReplicas {
		return false
	}

	// Check if all replicas are updated
	if deploy.Status.UpdatedReplicas != desiredReplicas {
		return false
	}

	// Check if all replicas are available
	if deploy.Status.AvailableReplicas != desiredReplicas {
		return false
	}

	// Check if the observed generation matches
	if deploy.Status.ObservedGeneration != deploy.Generation {
		return false
	}

	return true
}

// GetStatefulSetStatusMessage returns a detailed status message for a StatefulSet
func GetStatefulSetStatusMessage(sts *appsv1.StatefulSet) string {
	if sts == nil || sts.Spec.Replicas == nil {
		return "StatefulSet is nil or has no replicas configured"
	}

	desiredReplicas := *sts.Spec.Replicas

	if sts.Status.ObservedGeneration != sts.Generation {
		return fmt.Sprintf("StatefulSet update in progress (generation %d, observed %d)",
			sts.Generation, sts.Status.ObservedGeneration)
	}

	if sts.Status.ReadyReplicas != desiredReplicas {
		return fmt.Sprintf("StatefulSet has %d/%d replicas ready",
			sts.Status.ReadyReplicas, desiredReplicas)
	}

	if sts.Status.UpdatedReplicas != desiredReplicas {
		return fmt.Sprintf("StatefulSet has %d/%d replicas updated",
			sts.Status.UpdatedReplicas, desiredReplicas)
	}

	return "StatefulSet is not healthy"
}

// GetDaemonSetStatusMessage returns a detailed status message for a DaemonSet
func GetDaemonSetStatusMessage(ds *appsv1.DaemonSet) string {
	if ds == nil {
		return "DaemonSet is nil"
	}

	desiredScheduled := ds.Status.DesiredNumberScheduled

	if desiredScheduled == 0 {
		return "DaemonSet has no pods scheduled"
	}

	if ds.Status.ObservedGeneration != ds.Generation {
		return fmt.Sprintf("DaemonSet update in progress (generation %d, observed %d)",
			ds.Generation, ds.Status.ObservedGeneration)
	}

	if ds.Status.NumberReady != desiredScheduled {
		return fmt.Sprintf("DaemonSet has %d/%d pods ready",
			ds.Status.NumberReady, desiredScheduled)
	}

	if ds.Status.UpdatedNumberScheduled != desiredScheduled {
		return fmt.Sprintf("DaemonSet has %d/%d pods updated",
			ds.Status.UpdatedNumberScheduled, desiredScheduled)
	}

	if ds.Status.NumberAvailable != desiredScheduled {
		return fmt.Sprintf("DaemonSet has %d/%d pods available",
			ds.Status.NumberAvailable, desiredScheduled)
	}

	if ds.Status.NumberUnavailable > 0 {
		return fmt.Sprintf("DaemonSet has %d pods unavailable",
			ds.Status.NumberUnavailable)
	}

	return "DaemonSet is not healthy"
}

// GetDeploymentStatusMessage returns a detailed status message for a Deployment
func GetDeploymentStatusMessage(deploy *appsv1.Deployment) string {
	if deploy == nil || deploy.Spec.Replicas == nil {
		return "Deployment is nil or has no replicas configured"
	}

	desiredReplicas := *deploy.Spec.Replicas

	if deploy.Status.ObservedGeneration != deploy.Generation {
		return fmt.Sprintf("Deployment update in progress (generation %d, observed %d)",
			deploy.Generation, deploy.Status.ObservedGeneration)
	}

	if deploy.Status.ReadyReplicas != desiredReplicas {
		return fmt.Sprintf("Deployment has %d/%d replicas ready",
			deploy.Status.ReadyReplicas, desiredReplicas)
	}

	if deploy.Status.UpdatedReplicas != desiredReplicas {
		return fmt.Sprintf("Deployment has %d/%d replicas updated",
			deploy.Status.UpdatedReplicas, desiredReplicas)
	}

	if deploy.Status.AvailableReplicas != desiredReplicas {
		return fmt.Sprintf("Deployment has %d/%d replicas available",
			deploy.Status.AvailableReplicas, desiredReplicas)
	}

	if deploy.Status.UnavailableReplicas > 0 {
		return fmt.Sprintf("Deployment has %d replicas unavailable",
			deploy.Status.UnavailableReplicas)
	}

	return "Deployment is not healthy"
}
