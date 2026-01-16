package status

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/openshift/zero-trust-workload-identity-manager/api/v1alpha1"
	"github.com/openshift/zero-trust-workload-identity-manager/pkg/client/fakes"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestAddCondition(t *testing.T) {
	tests := []struct {
		name          string
		conditionType string
		reason        string
		message       string
		status        metav1.ConditionStatus
	}{
		{"Add True condition", "TestReady", "AllGood", "Everything is working", metav1.ConditionTrue},
		{"Add False condition", "TestFailed", "SomethingWrong", "An error occurred", metav1.ConditionFalse},
		{"Add Unknown condition", "TestUnknown", "NotSure", "Status is unknown", metav1.ConditionUnknown},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mgr := &Manager{conditions: make(map[string]Condition)}
			mgr.AddCondition(tt.conditionType, tt.reason, tt.message, tt.status)

			cond, exists := mgr.conditions[tt.conditionType]
			if !exists {
				t.Fatalf("Expected condition %s to be added", tt.conditionType)
			}
			if cond.Type != tt.conditionType || cond.Reason != tt.reason || cond.Message != tt.message || cond.Status != tt.status {
				t.Errorf("Condition mismatch: got %+v", cond)
			}
		})
	}
}

func TestSetReadyCondition(t *testing.T) {
	tests := []struct {
		name               string
		existingConditions map[string]Condition
		expectedStatus     metav1.ConditionStatus
		expectedReason     string
	}{
		{
			name: "All conditions true",
			existingConditions: map[string]Condition{
				"Component1": {Type: "Component1", Status: metav1.ConditionTrue, Reason: "OK"},
				"Component2": {Type: "Component2", Status: metav1.ConditionTrue, Reason: "OK"},
			},
			expectedStatus: metav1.ConditionTrue,
			expectedReason: v1alpha1.ReasonReady,
		},
		{
			name: "One condition false - Failed",
			existingConditions: map[string]Condition{
				"Component1": {Type: "Component1", Status: metav1.ConditionTrue, Reason: "OK"},
				"Component2": {Type: "Component2", Status: metav1.ConditionFalse, Reason: "Failed"},
			},
			expectedStatus: metav1.ConditionFalse,
			expectedReason: v1alpha1.ReasonFailed,
		},
		{
			name: "StatefulSet starting - Progressing",
			existingConditions: map[string]Condition{
				"StatefulSetAvailable": {Type: "StatefulSetAvailable", Status: metav1.ConditionFalse, Reason: "StatefulSetNotReady", Message: "StatefulSet has 0/1 replicas ready"},
			},
			expectedStatus: metav1.ConditionFalse,
			expectedReason: v1alpha1.ReasonInProgress,
		},
		{
			name: "DaemonSet starting - Progressing",
			existingConditions: map[string]Condition{
				"DaemonSetAvailable": {Type: "DaemonSetAvailable", Status: metav1.ConditionFalse, Reason: "DaemonSetNotReady", Message: "DaemonSet has 0/3 pods ready"},
			},
			expectedStatus: metav1.ConditionFalse,
			expectedReason: v1alpha1.ReasonInProgress,
		},
		{
			name: "Deployment rolling - Progressing",
			existingConditions: map[string]Condition{
				"DeploymentAvailable": {Type: "DeploymentAvailable", Status: metav1.ConditionFalse, Reason: "DeploymentNotReady", Message: "Deployment has 1/3 replicas ready"},
			},
			expectedStatus: metav1.ConditionFalse,
			expectedReason: v1alpha1.ReasonInProgress,
		},
		{
			name: "Failure takes precedence over progressing",
			existingConditions: map[string]Condition{
				"StatefulSetAvailable": {Type: "StatefulSetAvailable", Status: metav1.ConditionFalse, Reason: "StatefulSetNotReady", Message: "StatefulSet has 0/1 replicas ready"},
				"ConfigValid":          {Type: "ConfigValid", Status: metav1.ConditionFalse, Reason: "InvalidConfig"},
			},
			expectedStatus: metav1.ConditionFalse,
			expectedReason: v1alpha1.ReasonFailed,
		},
		{
			name:               "No conditions - Ready",
			existingConditions: map[string]Condition{},
			expectedStatus:     metav1.ConditionTrue,
			expectedReason:     v1alpha1.ReasonReady,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mgr := &Manager{conditions: tt.existingConditions}
			mgr.SetReadyCondition()

			readyCond, exists := mgr.conditions[v1alpha1.Ready]
			if !exists {
				t.Fatal("Expected Ready condition to be set")
			}
			if readyCond.Status != tt.expectedStatus {
				t.Errorf("Expected status %s, got %s", tt.expectedStatus, readyCond.Status)
			}
			if readyCond.Reason != tt.expectedReason {
				t.Errorf("Expected reason %s, got %s", tt.expectedReason, readyCond.Reason)
			}
		})
	}
}

func TestIsStatefulSetHealthy(t *testing.T) {
	tests := []struct {
		name     string
		sts      *appsv1.StatefulSet
		expected bool
	}{
		{name: "Nil StatefulSet", sts: nil, expected: false},
		{name: "Nil replicas", sts: &appsv1.StatefulSet{Spec: appsv1.StatefulSetSpec{Replicas: nil}}, expected: false},
		{
			name: "Healthy",
			sts: &appsv1.StatefulSet{
				ObjectMeta: metav1.ObjectMeta{Generation: 5},
				Spec:       appsv1.StatefulSetSpec{Replicas: pointer.Int32(3)},
				Status:     appsv1.StatefulSetStatus{ReadyReplicas: 3, UpdatedReplicas: 3, ObservedGeneration: 5},
			},
			expected: true,
		},
		{
			name: "Not all ready",
			sts: &appsv1.StatefulSet{
				ObjectMeta: metav1.ObjectMeta{Generation: 5},
				Spec:       appsv1.StatefulSetSpec{Replicas: pointer.Int32(3)},
				Status:     appsv1.StatefulSetStatus{ReadyReplicas: 1, UpdatedReplicas: 3, ObservedGeneration: 5},
			},
			expected: false,
		},
		{
			name: "Not all updated",
			sts: &appsv1.StatefulSet{
				ObjectMeta: metav1.ObjectMeta{Generation: 5},
				Spec:       appsv1.StatefulSetSpec{Replicas: pointer.Int32(3)},
				Status:     appsv1.StatefulSetStatus{ReadyReplicas: 3, UpdatedReplicas: 2, ObservedGeneration: 5},
			},
			expected: false,
		},
		{
			name: "Generation mismatch",
			sts: &appsv1.StatefulSet{
				ObjectMeta: metav1.ObjectMeta{Generation: 5},
				Spec:       appsv1.StatefulSetSpec{Replicas: pointer.Int32(3)},
				Status:     appsv1.StatefulSetStatus{ReadyReplicas: 3, UpdatedReplicas: 3, ObservedGeneration: 4},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsStatefulSetHealthy(tt.sts); got != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, got)
			}
		})
	}
}

func TestIsDaemonSetHealthy(t *testing.T) {
	tests := []struct {
		name     string
		ds       *appsv1.DaemonSet
		expected bool
	}{
		{name: "Nil DaemonSet", ds: nil, expected: false},
		{name: "No pods scheduled", ds: &appsv1.DaemonSet{Status: appsv1.DaemonSetStatus{DesiredNumberScheduled: 0}}, expected: false},
		{
			name: "Healthy",
			ds: &appsv1.DaemonSet{
				ObjectMeta: metav1.ObjectMeta{Generation: 3},
				Status:     appsv1.DaemonSetStatus{DesiredNumberScheduled: 10, NumberReady: 10, UpdatedNumberScheduled: 10, NumberAvailable: 10, ObservedGeneration: 3},
			},
			expected: true,
		},
		{
			name: "Not all ready",
			ds: &appsv1.DaemonSet{
				ObjectMeta: metav1.ObjectMeta{Generation: 3},
				Status:     appsv1.DaemonSetStatus{DesiredNumberScheduled: 10, NumberReady: 7, UpdatedNumberScheduled: 10, NumberAvailable: 10, ObservedGeneration: 3},
			},
			expected: false,
		},
		{
			name: "Not all updated",
			ds: &appsv1.DaemonSet{
				ObjectMeta: metav1.ObjectMeta{Generation: 3},
				Status:     appsv1.DaemonSetStatus{DesiredNumberScheduled: 10, NumberReady: 10, UpdatedNumberScheduled: 8, NumberAvailable: 10, ObservedGeneration: 3},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsDaemonSetHealthy(tt.ds); got != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, got)
			}
		})
	}
}

func TestIsDeploymentHealthy(t *testing.T) {
	tests := []struct {
		name     string
		deploy   *appsv1.Deployment
		expected bool
	}{
		{name: "Nil Deployment", deploy: nil, expected: false},
		{name: "Nil replicas", deploy: &appsv1.Deployment{Spec: appsv1.DeploymentSpec{Replicas: nil}}, expected: false},
		{
			name: "Healthy",
			deploy: &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{Generation: 7},
				Spec:       appsv1.DeploymentSpec{Replicas: pointer.Int32(5)},
				Status:     appsv1.DeploymentStatus{ReadyReplicas: 5, UpdatedReplicas: 5, AvailableReplicas: 5, ObservedGeneration: 7},
			},
			expected: true,
		},
		{
			name: "Not all ready",
			deploy: &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{Generation: 7},
				Spec:       appsv1.DeploymentSpec{Replicas: pointer.Int32(5)},
				Status:     appsv1.DeploymentStatus{ReadyReplicas: 3, UpdatedReplicas: 5, AvailableReplicas: 5, ObservedGeneration: 7},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsDeploymentHealthy(tt.deploy); got != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, got)
			}
		})
	}
}

func TestGetStatefulSetStatusMessage(t *testing.T) {
	tests := []struct {
		name     string
		sts      *appsv1.StatefulSet
		expected string
	}{
		{name: "Nil", sts: nil, expected: "StatefulSet is nil or has no replicas configured"},
		{name: "Nil replicas", sts: &appsv1.StatefulSet{Spec: appsv1.StatefulSetSpec{Replicas: nil}}, expected: "StatefulSet is nil or has no replicas configured"},
		{
			name: "Generation mismatch",
			sts: &appsv1.StatefulSet{
				ObjectMeta: metav1.ObjectMeta{Generation: 5},
				Spec:       appsv1.StatefulSetSpec{Replicas: pointer.Int32(3)},
				Status:     appsv1.StatefulSetStatus{ObservedGeneration: 4},
			},
			expected: "StatefulSet update in progress (generation 5, observed 4)",
		},
		{
			name: "Not all ready",
			sts: &appsv1.StatefulSet{
				ObjectMeta: metav1.ObjectMeta{Generation: 5},
				Spec:       appsv1.StatefulSetSpec{Replicas: pointer.Int32(3)},
				Status:     appsv1.StatefulSetStatus{ReadyReplicas: 1, ObservedGeneration: 5},
			},
			expected: "StatefulSet has 1/3 replicas ready",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := GetStatefulSetStatusMessage(tt.sts); got != tt.expected {
				t.Errorf("Expected '%s', got '%s'", tt.expected, got)
			}
		})
	}
}

func TestGetDaemonSetStatusMessage(t *testing.T) {
	tests := []struct {
		name     string
		ds       *appsv1.DaemonSet
		expected string
	}{
		{name: "Nil", ds: nil, expected: "DaemonSet is nil"},
		{name: "No pods scheduled", ds: &appsv1.DaemonSet{Status: appsv1.DaemonSetStatus{DesiredNumberScheduled: 0}}, expected: "DaemonSet has no pods scheduled"},
		{
			name: "Generation mismatch",
			ds: &appsv1.DaemonSet{
				ObjectMeta: metav1.ObjectMeta{Generation: 5},
				Status:     appsv1.DaemonSetStatus{DesiredNumberScheduled: 10, ObservedGeneration: 4},
			},
			expected: "DaemonSet update in progress (generation 5, observed 4)",
		},
		{
			name: "Not all ready",
			ds: &appsv1.DaemonSet{
				ObjectMeta: metav1.ObjectMeta{Generation: 5},
				Status:     appsv1.DaemonSetStatus{DesiredNumberScheduled: 10, NumberReady: 7, ObservedGeneration: 5},
			},
			expected: "DaemonSet has 7/10 pods ready",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := GetDaemonSetStatusMessage(tt.ds); got != tt.expected {
				t.Errorf("Expected '%s', got '%s'", tt.expected, got)
			}
		})
	}
}

func TestGetDeploymentStatusMessage(t *testing.T) {
	tests := []struct {
		name     string
		deploy   *appsv1.Deployment
		expected string
	}{
		{name: "Nil", deploy: nil, expected: "Deployment is nil or has no replicas configured"},
		{name: "Nil replicas", deploy: &appsv1.Deployment{Spec: appsv1.DeploymentSpec{Replicas: nil}}, expected: "Deployment is nil or has no replicas configured"},
		{
			name: "Generation mismatch",
			deploy: &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{Generation: 8},
				Spec:       appsv1.DeploymentSpec{Replicas: pointer.Int32(5)},
				Status:     appsv1.DeploymentStatus{ObservedGeneration: 7},
			},
			expected: "Deployment update in progress (generation 8, observed 7)",
		},
		{
			name: "Not all ready",
			deploy: &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{Generation: 8},
				Spec:       appsv1.DeploymentSpec{Replicas: pointer.Int32(5)},
				Status:     appsv1.DeploymentStatus{ReadyReplicas: 3, ObservedGeneration: 8},
			},
			expected: "Deployment has 3/5 replicas ready",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := GetDeploymentStatusMessage(tt.deploy); got != tt.expected {
				t.Errorf("Expected '%s', got '%s'", tt.expected, got)
			}
		})
	}
}

func TestNewManager(t *testing.T) {
	fakeClient := &fakes.FakeCustomCtrlClient{}
	mgr := NewManager(fakeClient)

	if mgr == nil || mgr.conditions == nil || len(mgr.conditions) != 0 {
		t.Error("Manager not properly initialized")
	}
}

func TestSetInitialReconciliationStatus(t *testing.T) {
	tests := []struct {
		name        string
		updateError error
	}{
		{name: "successful update", updateError: nil},
		{name: "update error handled gracefully", updateError: errors.New("update failed")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fakeClient := &fakes.FakeCustomCtrlClient{}
			fakeClient.StatusUpdateWithRetryReturns(tt.updateError)

			obj := &v1alpha1.SpireServer{ObjectMeta: metav1.ObjectMeta{Name: "cluster"}}
			SetInitialReconciliationStatus(context.Background(), fakeClient, obj, func() *v1alpha1.ConditionalStatus {
				return &obj.Status.ConditionalStatus
			}, "SpireServer")
		})
	}
}

func TestApplyStatus(t *testing.T) {
	tests := []struct {
		name        string
		updateError error
		expectError bool
	}{
		{name: "apply status success", updateError: nil, expectError: false},
		{name: "apply status error", updateError: errors.New("update failed"), expectError: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fakeClient := &fakes.FakeCustomCtrlClient{}
			fakeClient.StatusUpdateWithRetryReturns(tt.updateError)

			mgr := NewManager(fakeClient)
			mgr.AddCondition("TestCondition", "TestReason", "Test message", metav1.ConditionTrue)

			obj := &v1alpha1.SpireServer{ObjectMeta: metav1.ObjectMeta{Name: "cluster"}}
			err := mgr.ApplyStatus(context.Background(), obj, func() *v1alpha1.ConditionalStatus {
				return &obj.Status.ConditionalStatus
			})

			if tt.expectError && err == nil {
				t.Error("Expected error")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
		})
	}
}

func TestCheckStatefulSetHealth(t *testing.T) {
	tests := []struct {
		name           string
		getError       error
		healthy        bool
		expectedStatus metav1.ConditionStatus
	}{
		{name: "not found", getError: errors.New("not found"), expectedStatus: metav1.ConditionFalse},
		{name: "healthy", healthy: true, expectedStatus: metav1.ConditionTrue},
		{name: "not ready", healthy: false, expectedStatus: metav1.ConditionFalse},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fakeClient := &fakes.FakeCustomCtrlClient{}
			mgr := NewManager(fakeClient)

			if tt.getError != nil {
				fakeClient.GetReturns(tt.getError)
			} else {
				readyReplicas := int32(1)
				if tt.healthy {
					readyReplicas = 3
				}
				sts := &appsv1.StatefulSet{
					ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "ns", Generation: 1},
					Spec:       appsv1.StatefulSetSpec{Replicas: pointer.Int32(3)},
					Status:     appsv1.StatefulSetStatus{ReadyReplicas: readyReplicas, UpdatedReplicas: 3, ObservedGeneration: 1},
				}
				fakeClient.GetStub = func(ctx context.Context, key client.ObjectKey, obj client.Object) error {
					if s, ok := obj.(*appsv1.StatefulSet); ok {
						*s = *sts
					}
					return nil
				}
			}

			mgr.CheckStatefulSetHealth(context.Background(), "test", "ns", "StatefulSetAvailable")

			cond := mgr.conditions["StatefulSetAvailable"]
			if cond.Status != tt.expectedStatus {
				t.Errorf("Expected %v, got %v", tt.expectedStatus, cond.Status)
			}
		})
	}
}

func TestCheckDaemonSetHealth(t *testing.T) {
	tests := []struct {
		name           string
		getError       error
		healthy        bool
		expectedStatus metav1.ConditionStatus
	}{
		{name: "not found", getError: errors.New("not found"), expectedStatus: metav1.ConditionFalse},
		{name: "healthy", healthy: true, expectedStatus: metav1.ConditionTrue},
		{name: "not ready", healthy: false, expectedStatus: metav1.ConditionFalse},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fakeClient := &fakes.FakeCustomCtrlClient{}
			mgr := NewManager(fakeClient)

			if tt.getError != nil {
				fakeClient.GetReturns(tt.getError)
			} else {
				numberReady := int32(1)
				if tt.healthy {
					numberReady = 3
				}
				ds := &appsv1.DaemonSet{
					ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "ns", Generation: 1},
					Status:     appsv1.DaemonSetStatus{DesiredNumberScheduled: 3, NumberReady: numberReady, UpdatedNumberScheduled: 3, NumberAvailable: numberReady, ObservedGeneration: 1},
				}
				fakeClient.GetStub = func(ctx context.Context, key client.ObjectKey, obj client.Object) error {
					if d, ok := obj.(*appsv1.DaemonSet); ok {
						*d = *ds
					}
					return nil
				}
			}

			mgr.CheckDaemonSetHealth(context.Background(), "test", "ns", "DaemonSetAvailable")

			cond := mgr.conditions["DaemonSetAvailable"]
			if cond.Status != tt.expectedStatus {
				t.Errorf("Expected %v, got %v", tt.expectedStatus, cond.Status)
			}
		})
	}
}

func TestCheckDeploymentHealth(t *testing.T) {
	tests := []struct {
		name           string
		getError       error
		healthy        bool
		expectedStatus metav1.ConditionStatus
	}{
		{name: "not found", getError: errors.New("not found"), expectedStatus: metav1.ConditionFalse},
		{name: "healthy", healthy: true, expectedStatus: metav1.ConditionTrue},
		{name: "not ready", healthy: false, expectedStatus: metav1.ConditionFalse},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fakeClient := &fakes.FakeCustomCtrlClient{}
			mgr := NewManager(fakeClient)

			if tt.getError != nil {
				fakeClient.GetReturns(tt.getError)
			} else {
				readyReplicas := int32(1)
				if tt.healthy {
					readyReplicas = 3
				}
				deploy := &appsv1.Deployment{
					ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "ns", Generation: 1},
					Spec:       appsv1.DeploymentSpec{Replicas: pointer.Int32(3)},
					Status:     appsv1.DeploymentStatus{ReadyReplicas: readyReplicas, UpdatedReplicas: 3, AvailableReplicas: readyReplicas, ObservedGeneration: 1},
				}
				fakeClient.GetStub = func(ctx context.Context, key client.ObjectKey, obj client.Object) error {
					if d, ok := obj.(*appsv1.Deployment); ok {
						*d = *deploy
					}
					return nil
				}
			}

			mgr.CheckDeploymentHealth(context.Background(), "test", "ns", "DeploymentAvailable")

			cond := mgr.conditions["DeploymentAvailable"]
			if cond.Status != tt.expectedStatus {
				t.Errorf("Expected %v, got %v", tt.expectedStatus, cond.Status)
			}
		})
	}
}

// TestGetDaemonSetStatusMessage_AllScenarios tests all scenarios for GetDaemonSetStatusMessage
func TestGetDaemonSetStatusMessage_AllScenarios(t *testing.T) {
	tests := []struct {
		name            string
		ds              *appsv1.DaemonSet
		expectedContain string
	}{
		{
			name:            "nil DaemonSet",
			ds:              nil,
			expectedContain: "nil",
		},
		{
			name: "no pods scheduled",
			ds: &appsv1.DaemonSet{
				Status: appsv1.DaemonSetStatus{
					DesiredNumberScheduled: 0,
				},
			},
			expectedContain: "no pods scheduled",
		},
		{
			name: "generation mismatch",
			ds: &appsv1.DaemonSet{
				ObjectMeta: metav1.ObjectMeta{Generation: 2},
				Status: appsv1.DaemonSetStatus{
					DesiredNumberScheduled: 3,
					ObservedGeneration:     1,
				},
			},
			expectedContain: "update in progress",
		},
		{
			name: "not all pods ready",
			ds: &appsv1.DaemonSet{
				ObjectMeta: metav1.ObjectMeta{Generation: 1},
				Status: appsv1.DaemonSetStatus{
					DesiredNumberScheduled: 3,
					NumberReady:            2,
					ObservedGeneration:     1,
				},
			},
			expectedContain: "pods ready",
		},
		{
			name: "not all pods updated",
			ds: &appsv1.DaemonSet{
				ObjectMeta: metav1.ObjectMeta{Generation: 1},
				Status: appsv1.DaemonSetStatus{
					DesiredNumberScheduled: 3,
					NumberReady:            3,
					UpdatedNumberScheduled: 2,
					ObservedGeneration:     1,
				},
			},
			expectedContain: "pods updated",
		},
		{
			name: "not all pods available",
			ds: &appsv1.DaemonSet{
				ObjectMeta: metav1.ObjectMeta{Generation: 1},
				Status: appsv1.DaemonSetStatus{
					DesiredNumberScheduled: 3,
					NumberReady:            3,
					UpdatedNumberScheduled: 3,
					NumberAvailable:        2,
					ObservedGeneration:     1,
				},
			},
			expectedContain: "pods available",
		},
		{
			name: "pods unavailable",
			ds: &appsv1.DaemonSet{
				ObjectMeta: metav1.ObjectMeta{Generation: 1},
				Status: appsv1.DaemonSetStatus{
					DesiredNumberScheduled: 3,
					NumberReady:            3,
					UpdatedNumberScheduled: 3,
					NumberAvailable:        3,
					NumberUnavailable:      1,
					ObservedGeneration:     1,
				},
			},
			expectedContain: "unavailable",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg := GetDaemonSetStatusMessage(tt.ds)
			if !strings.Contains(msg, tt.expectedContain) {
				t.Errorf("Expected message to contain '%s', got '%s'", tt.expectedContain, msg)
			}
		})
	}
}

// TestGetDeploymentStatusMessage_AllScenarios tests all scenarios for GetDeploymentStatusMessage
func TestGetDeploymentStatusMessage_AllScenarios(t *testing.T) {
	tests := []struct {
		name            string
		deploy          *appsv1.Deployment
		expectedContain string
	}{
		{
			name:            "nil Deployment",
			deploy:          nil,
			expectedContain: "nil",
		},
		{
			name: "no replicas configured",
			deploy: &appsv1.Deployment{
				Spec: appsv1.DeploymentSpec{Replicas: nil},
			},
			expectedContain: "nil",
		},
		{
			name: "generation mismatch",
			deploy: &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{Generation: 2},
				Spec:       appsv1.DeploymentSpec{Replicas: pointer.Int32(3)},
				Status: appsv1.DeploymentStatus{
					ObservedGeneration: 1,
				},
			},
			expectedContain: "update in progress",
		},
		{
			name: "not all replicas ready",
			deploy: &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{Generation: 1},
				Spec:       appsv1.DeploymentSpec{Replicas: pointer.Int32(3)},
				Status: appsv1.DeploymentStatus{
					ReadyReplicas:      2,
					ObservedGeneration: 1,
				},
			},
			expectedContain: "replicas ready",
		},
		{
			name: "not all replicas updated",
			deploy: &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{Generation: 1},
				Spec:       appsv1.DeploymentSpec{Replicas: pointer.Int32(3)},
				Status: appsv1.DeploymentStatus{
					ReadyReplicas:      3,
					UpdatedReplicas:    2,
					ObservedGeneration: 1,
				},
			},
			expectedContain: "replicas updated",
		},
		{
			name: "not all replicas available",
			deploy: &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{Generation: 1},
				Spec:       appsv1.DeploymentSpec{Replicas: pointer.Int32(3)},
				Status: appsv1.DeploymentStatus{
					ReadyReplicas:      3,
					UpdatedReplicas:    3,
					AvailableReplicas:  2,
					ObservedGeneration: 1,
				},
			},
			expectedContain: "replicas available",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg := GetDeploymentStatusMessage(tt.deploy)
			if !strings.Contains(msg, tt.expectedContain) {
				t.Errorf("Expected message to contain '%s', got '%s'", tt.expectedContain, msg)
			}
		})
	}
}

// TestGetStatefulSetStatusMessage_AllScenarios tests all scenarios for GetStatefulSetStatusMessage
func TestGetStatefulSetStatusMessage_AllScenarios(t *testing.T) {
	tests := []struct {
		name            string
		sts             *appsv1.StatefulSet
		expectedContain string
	}{
		{
			name:            "nil StatefulSet",
			sts:             nil,
			expectedContain: "nil",
		},
		{
			name: "no replicas configured",
			sts: &appsv1.StatefulSet{
				Spec: appsv1.StatefulSetSpec{Replicas: nil},
			},
			expectedContain: "nil",
		},
		{
			name: "generation mismatch",
			sts: &appsv1.StatefulSet{
				ObjectMeta: metav1.ObjectMeta{Generation: 2},
				Spec:       appsv1.StatefulSetSpec{Replicas: pointer.Int32(3)},
				Status: appsv1.StatefulSetStatus{
					ObservedGeneration: 1,
				},
			},
			expectedContain: "update in progress",
		},
		{
			name: "not all replicas ready",
			sts: &appsv1.StatefulSet{
				ObjectMeta: metav1.ObjectMeta{Generation: 1},
				Spec:       appsv1.StatefulSetSpec{Replicas: pointer.Int32(3)},
				Status: appsv1.StatefulSetStatus{
					ReadyReplicas:      2,
					ObservedGeneration: 1,
				},
			},
			expectedContain: "replicas ready",
		},
		{
			name: "not all replicas updated",
			sts: &appsv1.StatefulSet{
				ObjectMeta: metav1.ObjectMeta{Generation: 1},
				Spec:       appsv1.StatefulSetSpec{Replicas: pointer.Int32(3)},
				Status: appsv1.StatefulSetStatus{
					ReadyReplicas:      3,
					UpdatedReplicas:    2,
					ObservedGeneration: 1,
				},
			},
			expectedContain: "replicas updated",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg := GetStatefulSetStatusMessage(tt.sts)
			if !strings.Contains(msg, tt.expectedContain) {
				t.Errorf("Expected message to contain '%s', got '%s'", tt.expectedContain, msg)
			}
		})
	}
}
