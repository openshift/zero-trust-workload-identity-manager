package status

import (
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"

	"github.com/openshift/zero-trust-workload-identity-manager/api/v1alpha1"
)

func TestAddCondition(t *testing.T) {
	mgr := &Manager{
		customClient: nil,
		conditions:   make(map[string]Condition),
	}

	tests := []struct {
		name          string
		conditionType string
		reason        string
		message       string
		status        metav1.ConditionStatus
	}{
		{
			name:          "Add True condition",
			conditionType: "TestReady",
			reason:        "AllGood",
			message:       "Everything is working",
			status:        metav1.ConditionTrue,
		},
		{
			name:          "Add False condition",
			conditionType: "TestFailed",
			reason:        "SomethingWrong",
			message:       "An error occurred",
			status:        metav1.ConditionFalse,
		},
		{
			name:          "Add Unknown condition",
			conditionType: "TestUnknown",
			reason:        "NotSure",
			message:       "Status is unknown",
			status:        metav1.ConditionUnknown,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mgr.AddCondition(tt.conditionType, tt.reason, tt.message, tt.status)

			cond, exists := mgr.conditions[tt.conditionType]
			if !exists {
				t.Errorf("Expected condition %s to be added", tt.conditionType)
				return
			}

			if cond.Type != tt.conditionType {
				t.Errorf("Expected Type %s, got %s", tt.conditionType, cond.Type)
			}

			if cond.Reason != tt.reason {
				t.Errorf("Expected Reason %s, got %s", tt.reason, cond.Reason)
			}

			if cond.Message != tt.message {
				t.Errorf("Expected Message %s, got %s", tt.message, cond.Message)
			}

			if cond.Status != tt.status {
				t.Errorf("Expected Status %s, got %s", tt.status, cond.Status)
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
			name: "All conditions true - should be Ready",
			existingConditions: map[string]Condition{
				"Component1": {Type: "Component1", Status: metav1.ConditionTrue, Reason: "OK", Message: "Good"},
				"Component2": {Type: "Component2", Status: metav1.ConditionTrue, Reason: "OK", Message: "Good"},
			},
			expectedStatus: metav1.ConditionTrue,
			expectedReason: v1alpha1.ReasonReady,
		},
		{
			name: "One condition false with actual failure - should be Failed",
			existingConditions: map[string]Condition{
				"Component1": {Type: "Component1", Status: metav1.ConditionTrue, Reason: "OK", Message: "Good"},
				"Component2": {Type: "Component2", Status: metav1.ConditionFalse, Reason: "Failed", Message: "Bad"},
			},
			expectedStatus: metav1.ConditionFalse,
			expectedReason: v1alpha1.ReasonFailed,
		},
		{
			name: "StatefulSet starting up - should be Progressing",
			existingConditions: map[string]Condition{
				"StatefulSetAvailable": {Type: "StatefulSetAvailable", Status: metav1.ConditionFalse, Reason: "StatefulSetNotReady", Message: "StatefulSet has 0/1 replicas ready"},
			},
			expectedStatus: metav1.ConditionFalse,
			expectedReason: v1alpha1.ReasonInProgress,
		},
		{
			name: "DaemonSet starting up - should be Progressing",
			existingConditions: map[string]Condition{
				"DaemonSetAvailable": {Type: "DaemonSetAvailable", Status: metav1.ConditionFalse, Reason: "DaemonSetNotReady", Message: "DaemonSet has 0/3 pods ready"},
			},
			expectedStatus: metav1.ConditionFalse,
			expectedReason: v1alpha1.ReasonInProgress,
		},
		{
			name: "Deployment rolling out - should be Progressing",
			existingConditions: map[string]Condition{
				"DeploymentAvailable": {Type: "DeploymentAvailable", Status: metav1.ConditionFalse, Reason: "DeploymentNotReady", Message: "Deployment has 1/3 replicas ready"},
			},
			expectedStatus: metav1.ConditionFalse,
			expectedReason: v1alpha1.ReasonInProgress,
		},
		{
			name: "Mixed progressing and ready - should be Progressing",
			existingConditions: map[string]Condition{
				"Component1":           {Type: "Component1", Status: metav1.ConditionTrue, Reason: "OK", Message: "Good"},
				"StatefulSetAvailable": {Type: "StatefulSetAvailable", Status: metav1.ConditionFalse, Reason: "StatefulSetNotReady", Message: "StatefulSet has 0/1 replicas ready"},
			},
			expectedStatus: metav1.ConditionFalse,
			expectedReason: v1alpha1.ReasonInProgress,
		},
		{
			name: "Failure takes precedence over progressing - should be Failed",
			existingConditions: map[string]Condition{
				"StatefulSetAvailable": {Type: "StatefulSetAvailable", Status: metav1.ConditionFalse, Reason: "StatefulSetNotReady", Message: "StatefulSet has 0/1 replicas ready"},
				"ConfigValid":          {Type: "ConfigValid", Status: metav1.ConditionFalse, Reason: "InvalidConfig", Message: "Config is invalid"},
			},
			expectedStatus: metav1.ConditionFalse,
			expectedReason: v1alpha1.ReasonFailed,
		},
		{
			name:               "No conditions - should be Ready",
			existingConditions: map[string]Condition{},
			expectedStatus:     metav1.ConditionTrue,
			expectedReason:     v1alpha1.ReasonReady,
		},
		{
			name: "Ready and Degraded conditions ignored",
			existingConditions: map[string]Condition{
				v1alpha1.Ready:    {Type: v1alpha1.Ready, Status: metav1.ConditionFalse, Reason: "OldStatus", Message: "Old"},
				v1alpha1.Degraded: {Type: v1alpha1.Degraded, Status: metav1.ConditionTrue, Reason: "OldDegraded", Message: "Old"},
				"Component1":      {Type: "Component1", Status: metav1.ConditionTrue, Reason: "OK", Message: "Good"},
			},
			expectedStatus: metav1.ConditionTrue,
			expectedReason: v1alpha1.ReasonReady,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mgr := &Manager{
				customClient: nil,
				conditions:   tt.existingConditions,
			}

			mgr.SetReadyCondition()

			readyCond, exists := mgr.conditions[v1alpha1.Ready]
			if !exists {
				t.Error("Expected Ready condition to be set")
				return
			}

			if readyCond.Status != tt.expectedStatus {
				t.Errorf("Expected Ready status %s, got %s", tt.expectedStatus, readyCond.Status)
			}

			if readyCond.Reason != tt.expectedReason {
				t.Errorf("Expected Ready reason %s, got %s", tt.expectedReason, readyCond.Reason)
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
		{
			name:     "Nil StatefulSet",
			sts:      nil,
			expected: false,
		},
		{
			name: "StatefulSet with nil replicas",
			sts: &appsv1.StatefulSet{
				Spec: appsv1.StatefulSetSpec{
					Replicas: nil,
				},
			},
			expected: false,
		},
		{
			name: "Healthy StatefulSet - all replicas ready",
			sts: &appsv1.StatefulSet{
				ObjectMeta: metav1.ObjectMeta{
					Generation: 5,
				},
				Spec: appsv1.StatefulSetSpec{
					Replicas: pointer.Int32(3),
				},
				Status: appsv1.StatefulSetStatus{
					ReadyReplicas:      3,
					UpdatedReplicas:    3,
					ObservedGeneration: 5,
				},
			},
			expected: true,
		},
		{
			name: "Unhealthy - not all replicas ready",
			sts: &appsv1.StatefulSet{
				ObjectMeta: metav1.ObjectMeta{
					Generation: 5,
				},
				Spec: appsv1.StatefulSetSpec{
					Replicas: pointer.Int32(3),
				},
				Status: appsv1.StatefulSetStatus{
					ReadyReplicas:      1,
					UpdatedReplicas:    3,
					ObservedGeneration: 5,
				},
			},
			expected: false,
		},
		{
			name: "Unhealthy - not all replicas updated",
			sts: &appsv1.StatefulSet{
				ObjectMeta: metav1.ObjectMeta{
					Generation: 5,
				},
				Spec: appsv1.StatefulSetSpec{
					Replicas: pointer.Int32(3),
				},
				Status: appsv1.StatefulSetStatus{
					ReadyReplicas:      3,
					UpdatedReplicas:    2,
					ObservedGeneration: 5,
				},
			},
			expected: false,
		},
		{
			name: "Unhealthy - observed generation mismatch",
			sts: &appsv1.StatefulSet{
				ObjectMeta: metav1.ObjectMeta{
					Generation: 5,
				},
				Spec: appsv1.StatefulSetSpec{
					Replicas: pointer.Int32(3),
				},
				Status: appsv1.StatefulSetStatus{
					ReadyReplicas:      3,
					UpdatedReplicas:    3,
					ObservedGeneration: 4,
				},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsStatefulSetHealthy(tt.sts)
			if result != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, result)
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
		{
			name:     "Nil DaemonSet",
			ds:       nil,
			expected: false,
		},
		{
			name: "DaemonSet with no pods scheduled",
			ds: &appsv1.DaemonSet{
				Status: appsv1.DaemonSetStatus{
					DesiredNumberScheduled: 0,
				},
			},
			expected: false,
		},
		{
			name: "Healthy DaemonSet - all pods ready",
			ds: &appsv1.DaemonSet{
				ObjectMeta: metav1.ObjectMeta{
					Generation: 3,
				},
				Status: appsv1.DaemonSetStatus{
					DesiredNumberScheduled: 10,
					NumberReady:            10,
					UpdatedNumberScheduled: 10,
					NumberAvailable:        10,
					ObservedGeneration:     3,
				},
			},
			expected: true,
		},
		{
			name: "Unhealthy - not all pods ready",
			ds: &appsv1.DaemonSet{
				ObjectMeta: metav1.ObjectMeta{
					Generation: 3,
				},
				Status: appsv1.DaemonSetStatus{
					DesiredNumberScheduled: 10,
					NumberReady:            7,
					UpdatedNumberScheduled: 10,
					NumberAvailable:        10,
					ObservedGeneration:     3,
				},
			},
			expected: false,
		},
		{
			name: "Unhealthy - not all pods updated",
			ds: &appsv1.DaemonSet{
				ObjectMeta: metav1.ObjectMeta{
					Generation: 3,
				},
				Status: appsv1.DaemonSetStatus{
					DesiredNumberScheduled: 10,
					NumberReady:            10,
					UpdatedNumberScheduled: 8,
					NumberAvailable:        10,
					ObservedGeneration:     3,
				},
			},
			expected: false,
		},
		{
			name: "Unhealthy - not all pods available",
			ds: &appsv1.DaemonSet{
				ObjectMeta: metav1.ObjectMeta{
					Generation: 3,
				},
				Status: appsv1.DaemonSetStatus{
					DesiredNumberScheduled: 10,
					NumberReady:            10,
					UpdatedNumberScheduled: 10,
					NumberAvailable:        9,
					ObservedGeneration:     3,
				},
			},
			expected: false,
		},
		{
			name: "Unhealthy - observed generation mismatch",
			ds: &appsv1.DaemonSet{
				ObjectMeta: metav1.ObjectMeta{
					Generation: 5,
				},
				Status: appsv1.DaemonSetStatus{
					DesiredNumberScheduled: 10,
					NumberReady:            10,
					UpdatedNumberScheduled: 10,
					NumberAvailable:        10,
					ObservedGeneration:     4,
				},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsDaemonSetHealthy(tt.ds)
			if result != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, result)
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
		{
			name:     "Nil Deployment",
			deploy:   nil,
			expected: false,
		},
		{
			name: "Deployment with nil replicas",
			deploy: &appsv1.Deployment{
				Spec: appsv1.DeploymentSpec{
					Replicas: nil,
				},
			},
			expected: false,
		},
		{
			name: "Healthy Deployment - all replicas ready",
			deploy: &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Generation: 7,
				},
				Spec: appsv1.DeploymentSpec{
					Replicas: pointer.Int32(5),
				},
				Status: appsv1.DeploymentStatus{
					ReadyReplicas:      5,
					UpdatedReplicas:    5,
					AvailableReplicas:  5,
					ObservedGeneration: 7,
				},
			},
			expected: true,
		},
		{
			name: "Unhealthy - not all replicas ready",
			deploy: &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Generation: 7,
				},
				Spec: appsv1.DeploymentSpec{
					Replicas: pointer.Int32(5),
				},
				Status: appsv1.DeploymentStatus{
					ReadyReplicas:      3,
					UpdatedReplicas:    5,
					AvailableReplicas:  5,
					ObservedGeneration: 7,
				},
			},
			expected: false,
		},
		{
			name: "Unhealthy - not all replicas updated",
			deploy: &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Generation: 7,
				},
				Spec: appsv1.DeploymentSpec{
					Replicas: pointer.Int32(5),
				},
				Status: appsv1.DeploymentStatus{
					ReadyReplicas:      5,
					UpdatedReplicas:    4,
					AvailableReplicas:  5,
					ObservedGeneration: 7,
				},
			},
			expected: false,
		},
		{
			name: "Unhealthy - not all replicas available",
			deploy: &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Generation: 7,
				},
				Spec: appsv1.DeploymentSpec{
					Replicas: pointer.Int32(5),
				},
				Status: appsv1.DeploymentStatus{
					ReadyReplicas:      5,
					UpdatedReplicas:    5,
					AvailableReplicas:  4,
					ObservedGeneration: 7,
				},
			},
			expected: false,
		},
		{
			name: "Unhealthy - observed generation mismatch",
			deploy: &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Generation: 7,
				},
				Spec: appsv1.DeploymentSpec{
					Replicas: pointer.Int32(5),
				},
				Status: appsv1.DeploymentStatus{
					ReadyReplicas:      5,
					UpdatedReplicas:    5,
					AvailableReplicas:  5,
					ObservedGeneration: 6,
				},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsDeploymentHealthy(tt.deploy)
			if result != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestGetStatefulSetStatusMessage(t *testing.T) {
	tests := []struct {
		name            string
		sts             *appsv1.StatefulSet
		expectedMessage string
	}{
		{
			name:            "Nil StatefulSet",
			sts:             nil,
			expectedMessage: "StatefulSet is nil or has no replicas configured",
		},
		{
			name: "StatefulSet with nil replicas",
			sts: &appsv1.StatefulSet{
				Spec: appsv1.StatefulSetSpec{
					Replicas: nil,
				},
			},
			expectedMessage: "StatefulSet is nil or has no replicas configured",
		},
		{
			name: "Generation mismatch",
			sts: &appsv1.StatefulSet{
				ObjectMeta: metav1.ObjectMeta{
					Generation: 5,
				},
				Spec: appsv1.StatefulSetSpec{
					Replicas: pointer.Int32(3),
				},
				Status: appsv1.StatefulSetStatus{
					ObservedGeneration: 4,
				},
			},
			expectedMessage: "StatefulSet update in progress (generation 5, observed 4)",
		},
		{
			name: "Not all replicas ready",
			sts: &appsv1.StatefulSet{
				ObjectMeta: metav1.ObjectMeta{
					Generation: 5,
				},
				Spec: appsv1.StatefulSetSpec{
					Replicas: pointer.Int32(3),
				},
				Status: appsv1.StatefulSetStatus{
					ReadyReplicas:      1,
					ObservedGeneration: 5,
				},
			},
			expectedMessage: "StatefulSet has 1/3 replicas ready",
		},
		{
			name: "Not all replicas updated",
			sts: &appsv1.StatefulSet{
				ObjectMeta: metav1.ObjectMeta{
					Generation: 5,
				},
				Spec: appsv1.StatefulSetSpec{
					Replicas: pointer.Int32(3),
				},
				Status: appsv1.StatefulSetStatus{
					ReadyReplicas:      3,
					UpdatedReplicas:    2,
					ObservedGeneration: 5,
				},
			},
			expectedMessage: "StatefulSet has 2/3 replicas updated",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			message := GetStatefulSetStatusMessage(tt.sts)
			if message != tt.expectedMessage {
				t.Errorf("Expected message '%s', got '%s'", tt.expectedMessage, message)
			}
		})
	}
}

func TestGetDaemonSetStatusMessage(t *testing.T) {
	tests := []struct {
		name            string
		ds              *appsv1.DaemonSet
		expectedMessage string
	}{
		{
			name:            "Nil DaemonSet",
			ds:              nil,
			expectedMessage: "DaemonSet is nil",
		},
		{
			name: "No pods scheduled",
			ds: &appsv1.DaemonSet{
				Status: appsv1.DaemonSetStatus{
					DesiredNumberScheduled: 0,
				},
			},
			expectedMessage: "DaemonSet has no pods scheduled",
		},
		{
			name: "Generation mismatch",
			ds: &appsv1.DaemonSet{
				ObjectMeta: metav1.ObjectMeta{
					Generation: 5,
				},
				Status: appsv1.DaemonSetStatus{
					DesiredNumberScheduled: 10,
					ObservedGeneration:     4,
				},
			},
			expectedMessage: "DaemonSet update in progress (generation 5, observed 4)",
		},
		{
			name: "Not all pods ready",
			ds: &appsv1.DaemonSet{
				ObjectMeta: metav1.ObjectMeta{
					Generation: 5,
				},
				Status: appsv1.DaemonSetStatus{
					DesiredNumberScheduled: 10,
					NumberReady:            7,
					ObservedGeneration:     5,
				},
			},
			expectedMessage: "DaemonSet has 7/10 pods ready",
		},
		{
			name: "Not all pods updated",
			ds: &appsv1.DaemonSet{
				ObjectMeta: metav1.ObjectMeta{
					Generation: 5,
				},
				Status: appsv1.DaemonSetStatus{
					DesiredNumberScheduled: 10,
					NumberReady:            10,
					UpdatedNumberScheduled: 8,
					ObservedGeneration:     5,
				},
			},
			expectedMessage: "DaemonSet has 8/10 pods updated",
		},
		{
			name: "Pods unavailable",
			ds: &appsv1.DaemonSet{
				ObjectMeta: metav1.ObjectMeta{
					Generation: 5,
				},
				Status: appsv1.DaemonSetStatus{
					DesiredNumberScheduled: 10,
					NumberReady:            10,
					UpdatedNumberScheduled: 10,
					NumberAvailable:        10,
					NumberUnavailable:      2,
					ObservedGeneration:     5,
				},
			},
			expectedMessage: "DaemonSet has 2 pods unavailable",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			message := GetDaemonSetStatusMessage(tt.ds)
			if message != tt.expectedMessage {
				t.Errorf("Expected message '%s', got '%s'", tt.expectedMessage, message)
			}
		})
	}
}

func TestGetDeploymentStatusMessage(t *testing.T) {
	tests := []struct {
		name            string
		deploy          *appsv1.Deployment
		expectedMessage string
	}{
		{
			name:            "Nil Deployment",
			deploy:          nil,
			expectedMessage: "Deployment is nil or has no replicas configured",
		},
		{
			name: "Deployment with nil replicas",
			deploy: &appsv1.Deployment{
				Spec: appsv1.DeploymentSpec{
					Replicas: nil,
				},
			},
			expectedMessage: "Deployment is nil or has no replicas configured",
		},
		{
			name: "Generation mismatch",
			deploy: &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Generation: 8,
				},
				Spec: appsv1.DeploymentSpec{
					Replicas: pointer.Int32(5),
				},
				Status: appsv1.DeploymentStatus{
					ObservedGeneration: 7,
				},
			},
			expectedMessage: "Deployment update in progress (generation 8, observed 7)",
		},
		{
			name: "Not all replicas ready",
			deploy: &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Generation: 8,
				},
				Spec: appsv1.DeploymentSpec{
					Replicas: pointer.Int32(5),
				},
				Status: appsv1.DeploymentStatus{
					ReadyReplicas:      3,
					ObservedGeneration: 8,
				},
			},
			expectedMessage: "Deployment has 3/5 replicas ready",
		},
		{
			name: "Not all replicas updated",
			deploy: &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Generation: 8,
				},
				Spec: appsv1.DeploymentSpec{
					Replicas: pointer.Int32(5),
				},
				Status: appsv1.DeploymentStatus{
					ReadyReplicas:      5,
					UpdatedReplicas:    4,
					ObservedGeneration: 8,
				},
			},
			expectedMessage: "Deployment has 4/5 replicas updated",
		},
		{
			name: "Replicas unavailable",
			deploy: &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Generation: 8,
				},
				Spec: appsv1.DeploymentSpec{
					Replicas: pointer.Int32(5),
				},
				Status: appsv1.DeploymentStatus{
					ReadyReplicas:       5,
					UpdatedReplicas:     5,
					AvailableReplicas:   5,
					UnavailableReplicas: 1,
					ObservedGeneration:  8,
				},
			},
			expectedMessage: "Deployment has 1 replicas unavailable",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			message := GetDeploymentStatusMessage(tt.deploy)
			if message != tt.expectedMessage {
				t.Errorf("Expected message '%s', got '%s'", tt.expectedMessage, message)
			}
		})
	}
}
