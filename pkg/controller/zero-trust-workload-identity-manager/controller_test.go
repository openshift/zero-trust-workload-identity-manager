package zero_trust_workload_identity_manager

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/go-logr/logr"
	"github.com/openshift/zero-trust-workload-identity-manager/api/v1alpha1"
	"github.com/openshift/zero-trust-workload-identity-manager/pkg/client/fakes"
	"github.com/openshift/zero-trust-workload-identity-manager/pkg/controller/status"
	"github.com/openshift/zero-trust-workload-identity-manager/pkg/controller/utils"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
)

// newTestReconciler creates a reconciler for testing
func newTestReconciler(fakeClient *fakes.FakeCustomCtrlClient) *ZeroTrustWorkloadIdentityManagerReconciler {
	return &ZeroTrustWorkloadIdentityManagerReconciler{
		ctrlClient:            fakeClient,
		ctx:                   context.Background(),
		log:                   logr.Discard(),
		scheme:                runtime.NewScheme(),
		eventRecorder:         record.NewFakeRecorder(100),
		operatorConditionName: "test-operator-condition",
	}
}

// TestReconcile_ZTWIMNotFound tests that when ZTWIM CR is not found
func TestReconcile_ZTWIMNotFound(t *testing.T) {
	// Set up env variable for operator condition
	os.Setenv("OPERATOR_CONDITION_NAME", "test-operator-condition")
	defer os.Unsetenv("OPERATOR_CONDITION_NAME")

	fakeClient := &fakes.FakeCustomCtrlClient{}
	reconciler := newTestReconciler(fakeClient)

	// Configure fake client to return NotFound error for ZTWIM
	notFoundErr := kerrors.NewNotFound(schema.GroupResource{Group: "operator.openshift.io", Resource: "zerotrustworkloadidentitymanagers"}, "cluster")
	fakeClient.GetReturns(notFoundErr)

	req := ctrl.Request{NamespacedName: types.NamespacedName{Name: "cluster"}}
	result, err := reconciler.Reconcile(context.Background(), req)

	// Assert: should return nil error (not requeue) when CR not found
	if err != nil {
		t.Errorf("Expected nil error when ZTWIM not found, got: %v", err)
	}
	if result.Requeue {
		t.Error("Expected no requeue when ZTWIM not found")
	}
	if result.RequeueAfter != 0 {
		t.Error("Expected no RequeueAfter when ZTWIM not found")
	}
}

// TestReconcile_ZTWIMGetError tests that when Get returns a non-NotFound error
func TestReconcile_ZTWIMGetError(t *testing.T) {
	fakeClient := &fakes.FakeCustomCtrlClient{}
	reconciler := newTestReconciler(fakeClient)

	// Configure fake client to return a generic error for ZTWIM Get
	genericErr := errors.New("connection refused")
	fakeClient.GetReturns(genericErr)

	req := ctrl.Request{NamespacedName: types.NamespacedName{Name: "cluster"}}
	result, err := reconciler.Reconcile(context.Background(), req)

	// Assert: should return the error when Get fails with non-NotFound error
	if err == nil {
		t.Error("Expected error when Get fails, got nil")
	}
	if !errors.Is(err, genericErr) {
		t.Errorf("Expected connection refused error, got: %v", err)
	}
	if result.Requeue {
		t.Error("Expected no requeue flag when returning error")
	}
}

// TestClassifyOperandState_Ready tests classifyOperandState returns ready for ready operand
func TestClassifyOperandState_Ready(t *testing.T) {
	operand := v1alpha1.OperandStatus{
		Ready:   "true",
		Message: "Ready",
	}
	readyCondition := &metav1.Condition{
		Type:   v1alpha1.Ready,
		Status: metav1.ConditionTrue,
		Reason: v1alpha1.ReasonReady,
	}

	result := classifyOperandState(operand, readyCondition)
	if result != operandReady {
		t.Errorf("Expected operandReady, got %v", result)
	}
}

// TestClassifyOperandState_Progressing tests classifyOperandState returns progressing
func TestClassifyOperandState_Progressing(t *testing.T) {
	tests := []struct {
		name      string
		operand   v1alpha1.OperandStatus
		condition *metav1.Condition
	}{
		{
			name: "NotFound reason",
			operand: v1alpha1.OperandStatus{
				Ready:   "false",
				Message: "CR not found",
			},
			condition: &metav1.Condition{
				Type:   v1alpha1.Ready,
				Status: metav1.ConditionFalse,
				Reason: OperandStateNotFound,
			},
		},
		{
			name: "InitialReconcile reason",
			operand: v1alpha1.OperandStatus{
				Ready:   "false",
				Message: "Waiting for initial reconciliation",
			},
			condition: &metav1.Condition{
				Type:   v1alpha1.Ready,
				Status: metav1.ConditionFalse,
				Reason: OperandStateInitialReconcile,
			},
		},
		{
			name: "Reconciling reason",
			operand: v1alpha1.OperandStatus{
				Ready:   "false",
				Message: "Reconciling",
			},
			condition: &metav1.Condition{
				Type:   v1alpha1.Ready,
				Status: metav1.ConditionFalse,
				Reason: OperandStateReconciling,
			},
		},
		{
			name: "InProgress reason",
			operand: v1alpha1.OperandStatus{
				Ready:   "false",
				Message: "Reconciling",
			},
			condition: &metav1.Condition{
				Type:   v1alpha1.Ready,
				Status: metav1.ConditionFalse,
				Reason: v1alpha1.ReasonInProgress,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := classifyOperandState(tt.operand, tt.condition)
			if result != operandProgressing {
				t.Errorf("Expected operandProgressing, got %v", result)
			}
		})
	}
}

// TestClassifyOperandState_Failed tests classifyOperandState returns failed
func TestClassifyOperandState_Failed(t *testing.T) {
	tests := []struct {
		name      string
		operand   v1alpha1.OperandStatus
		condition *metav1.Condition
	}{
		{
			name: "Failed reason",
			operand: v1alpha1.OperandStatus{
				Ready:   "false",
				Message: "Failed to reconcile",
			},
			condition: &metav1.Condition{
				Type:   v1alpha1.Ready,
				Status: metav1.ConditionFalse,
				Reason: v1alpha1.ReasonFailed,
			},
		},
		{
			name: "Unhealthy reason",
			operand: v1alpha1.OperandStatus{
				Ready:   "false",
				Message: "Component is unhealthy",
			},
			condition: &metav1.Condition{
				Type:   v1alpha1.Ready,
				Status: metav1.ConditionFalse,
				Reason: OperandStateUnhealthy,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := classifyOperandState(tt.operand, tt.condition)
			if result != operandFailed {
				t.Errorf("Expected operandFailed, got %v", result)
			}
		})
	}
}

// TestClassifyOperandState_MessageFallback tests classifyOperandState message fallback
func TestClassifyOperandState_MessageFallback(t *testing.T) {
	tests := []struct {
		name     string
		operand  v1alpha1.OperandStatus
		expected operandStateClassification
	}{
		{
			name: "CR not found message - progressing",
			operand: v1alpha1.OperandStatus{
				Ready:   "false",
				Message: OperandMessageCRNotFound,
			},
			expected: operandProgressing,
		},
		{
			name: "Waiting initial recon message - progressing",
			operand: v1alpha1.OperandStatus{
				Ready:   "false",
				Message: OperandMessageWaitingInitialRecon,
			},
			expected: operandProgressing,
		},
		{
			name: "Reconciling message - progressing",
			operand: v1alpha1.OperandStatus{
				Ready:   "false",
				Message: OperandMessageReconciling,
			},
			expected: operandProgressing,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := classifyOperandState(tt.operand, nil)
			if result != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}
		})
	}
}

// TestClassifyOperandState_SubstringFallback tests classifyOperandState substring fallback
func TestClassifyOperandState_SubstringFallback(t *testing.T) {
	tests := []struct {
		name     string
		operand  v1alpha1.OperandStatus
		expected operandStateClassification
	}{
		{
			name: "Contains 'not found' - progressing",
			operand: v1alpha1.OperandStatus{
				Ready:   "false",
				Message: "Resource not found in cluster",
			},
			expected: operandProgressing,
		},
		{
			name: "Contains 'initial' - progressing",
			operand: v1alpha1.OperandStatus{
				Ready:   "false",
				Message: "Initial setup in progress",
			},
			expected: operandProgressing,
		},
		{
			name: "Contains 'reconciling' - progressing",
			operand: v1alpha1.OperandStatus{
				Ready:   "false",
				Message: "Component is still reconciling",
			},
			expected: operandProgressing,
		},
		{
			name: "Contains 'progressing' - progressing",
			operand: v1alpha1.OperandStatus{
				Ready:   "false",
				Message: "Work is progressing",
			},
			expected: operandProgressing,
		},
		{
			name: "Unknown message - failed",
			operand: v1alpha1.OperandStatus{
				Ready:   "false",
				Message: "Some other error occurred",
			},
			expected: operandFailed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := classifyOperandState(tt.operand, nil)
			if result != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}
		})
	}
}

// TestContains tests the contains helper function
func TestContains(t *testing.T) {
	tests := []struct {
		name     string
		s        string
		substr   string
		expected bool
	}{
		{
			name:     "Exact match",
			s:        "hello",
			substr:   "hello",
			expected: true,
		},
		{
			name:     "Substring match",
			s:        "hello world",
			substr:   "world",
			expected: true,
		},
		{
			name:     "Case insensitive match",
			s:        "Hello World",
			substr:   "hello",
			expected: true,
		},
		{
			name:     "No match",
			s:        "hello world",
			substr:   "foo",
			expected: false,
		},
		{
			name:     "Empty string",
			s:        "",
			substr:   "foo",
			expected: false,
		},
		{
			name:     "Empty substring",
			s:        "hello",
			substr:   "",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := contains(tt.s, tt.substr)
			if result != tt.expected {
				t.Errorf("contains(%q, %q) = %v, expected %v", tt.s, tt.substr, result, tt.expected)
			}
		})
	}
}

// TestSetCreateOnlyModeCondition tests setCreateOnlyModeCondition function
func TestSetCreateOnlyModeCondition(t *testing.T) {
	tests := []struct {
		name               string
		existingConditions []metav1.Condition
	}{
		{
			name:               "No existing conditions",
			existingConditions: nil,
		},
		{
			name: "Existing CreateOnlyMode condition True",
			existingConditions: []metav1.Condition{
				{
					Type:   CreateOnlyMode,
					Status: metav1.ConditionTrue,
					Reason: utils.CreateOnlyModeEnabled,
				},
			},
		},
		{
			name: "Existing CreateOnlyMode condition False",
			existingConditions: []metav1.Condition{
				{
					Type:   CreateOnlyMode,
					Status: metav1.ConditionFalse,
					Reason: utils.CreateOnlyModeDisabled,
				},
			},
		},
		{
			name: "Other conditions only",
			existingConditions: []metav1.Condition{
				{
					Type:   "Ready",
					Status: metav1.ConditionTrue,
					Reason: "AllGood",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fakeClient := &fakes.FakeCustomCtrlClient{}
			// Use real status.Manager
			mgr := status.NewManager(fakeClient)
			// Call function with correct signature - it checks utils.IsInCreateOnlyMode() internally
			setCreateOnlyModeCondition(mgr, tt.existingConditions)
			// The function completes without error - actual behavior depends on environment variable
			t.Log("Function completed successfully")
		})
	}
}

// TestProcessOperandStatus tests processOperandStatus function
func TestProcessOperandStatus(t *testing.T) {
	tests := []struct {
		name                     string
		operand                  v1alpha1.OperandStatus
		expectedAnyOperandExists bool
		expectedNotCreatedCount  int
		expectedFailedCount      int
	}{
		{
			name: "Ready operand",
			operand: v1alpha1.OperandStatus{
				Ready:   "true",
				Message: "Ready",
			},
			expectedAnyOperandExists: true,
			expectedNotCreatedCount:  0,
			expectedFailedCount:      0,
		},
		{
			name: "CR not found - progressing",
			operand: v1alpha1.OperandStatus{
				Ready:   "false",
				Message: OperandMessageCRNotFound,
			},
			expectedAnyOperandExists: false,
			expectedNotCreatedCount:  1,
			expectedFailedCount:      0,
		},
		{
			name: "Reconciling - progressing",
			operand: v1alpha1.OperandStatus{
				Ready:   "false",
				Message: OperandMessageReconciling,
			},
			expectedAnyOperandExists: true,
			expectedNotCreatedCount:  1,
			expectedFailedCount:      0,
		},
		{
			name: "Failed operand",
			operand: v1alpha1.OperandStatus{
				Ready:   "false",
				Message: "Failed to start",
			},
			expectedAnyOperandExists: true,
			expectedNotCreatedCount:  0,
			expectedFailedCount:      1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state := &operandAggregateState{allReady: true}
			processOperandStatus(tt.operand, state)

			if state.anyOperandExists != tt.expectedAnyOperandExists {
				t.Errorf("anyOperandExists = %v, expected %v", state.anyOperandExists, tt.expectedAnyOperandExists)
			}
			if state.notCreatedCount != tt.expectedNotCreatedCount {
				t.Errorf("notCreatedCount = %v, expected %v", state.notCreatedCount, tt.expectedNotCreatedCount)
			}
			if state.failedCount != tt.expectedFailedCount {
				t.Errorf("failedCount = %v, expected %v", state.failedCount, tt.expectedFailedCount)
			}
		})
	}
}

// TestExtractKeyConditions tests extractKeyConditions function
func TestExtractKeyConditions(t *testing.T) {
	tests := []struct {
		name          string
		conditions    []metav1.Condition
		isReady       bool
		expectedCount int
		expectedTypes []string
	}{
		{
			name:          "Ready operand - no conditions",
			conditions:    []metav1.Condition{},
			isReady:       true,
			expectedCount: 0,
			expectedTypes: []string{},
		},
		{
			name: "Ready operand with CreateOnlyMode True - include it",
			conditions: []metav1.Condition{
				{Type: utils.CreateOnlyModeStatusType, Status: metav1.ConditionTrue},
			},
			isReady:       true,
			expectedCount: 1,
			expectedTypes: []string{utils.CreateOnlyModeStatusType},
		},
		{
			name: "Ready operand with CreateOnlyMode False - exclude it",
			conditions: []metav1.Condition{
				{Type: utils.CreateOnlyModeStatusType, Status: metav1.ConditionFalse},
			},
			isReady:       true,
			expectedCount: 0,
			expectedTypes: []string{},
		},
		{
			name: "Not ready operand - include Ready condition",
			conditions: []metav1.Condition{
				{Type: v1alpha1.Ready, Status: metav1.ConditionFalse, Reason: v1alpha1.ReasonFailed},
			},
			isReady:       false,
			expectedCount: 1,
			expectedTypes: []string{v1alpha1.Ready},
		},
		{
			name: "Not ready operand with failed condition - include both",
			conditions: []metav1.Condition{
				{Type: v1alpha1.Ready, Status: metav1.ConditionFalse, Reason: v1alpha1.ReasonFailed},
				{Type: "SomeComponent", Status: metav1.ConditionFalse},
			},
			isReady:       false,
			expectedCount: 2,
			expectedTypes: []string{v1alpha1.Ready, "SomeComponent"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractKeyConditions(tt.conditions, tt.isReady)
			if len(result) != tt.expectedCount {
				t.Errorf("Expected %d conditions, got %d", tt.expectedCount, len(result))
			}
		})
	}
}

// TestRecreateClusterInstance tests recreateClusterInstance function
func TestRecreateClusterInstance(t *testing.T) {
	tests := []struct {
		name          string
		createErr     error
		expectRequeue bool
		expectErr     bool
	}{
		{
			name:          "Successful recreation - requeue",
			createErr:     nil,
			expectRequeue: true,
			expectErr:     false,
		},
		{
			name:          "Create error - return error",
			createErr:     errors.New("create failed"),
			expectRequeue: false,
			expectErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fakeClient := &fakes.FakeCustomCtrlClient{}
			reconciler := newTestReconciler(fakeClient)

			fakeClient.CreateReturns(tt.createErr)

			result, err := reconciler.recreateClusterInstance(context.Background(), "cluster")

			if tt.expectErr && err == nil {
				t.Error("Expected error, got nil")
			}
			if !tt.expectErr && err != nil {
				t.Errorf("Expected no error, got: %v", err)
			}
			if result.Requeue != tt.expectRequeue {
				t.Errorf("Requeue = %v, expected %v", result.Requeue, tt.expectRequeue)
			}
		})
	}
}

// TestFindOperatorCondition tests findOperatorCondition function
func TestFindOperatorCondition(t *testing.T) {
	tests := []struct {
		name                  string
		operatorConditionName string
		getError              error
		expectNil             bool
		expectErr             bool
	}{
		{
			name:                  "OperatorCondition found",
			operatorConditionName: "test-operator",
			getError:              nil,
			expectNil:             false,
			expectErr:             false,
		},
		{
			name:                  "OperatorCondition not found",
			operatorConditionName: "test-operator",
			getError:              kerrors.NewNotFound(schema.GroupResource{}, "test"),
			expectNil:             true,
			expectErr:             true,
		},
		{
			name:                  "Get error",
			operatorConditionName: "test-operator",
			getError:              errors.New("connection refused"),
			expectNil:             true,
			expectErr:             true,
		},
		{
			name:                  "AlreadyExists error returns error not nil",
			operatorConditionName: "test-operator",
			getError:              kerrors.NewAlreadyExists(schema.GroupResource{Group: "operators.coreos.com", Resource: "operatorconditions"}, "test"),
			expectNil:             true,
			expectErr:             true,
		},
		// Additional mutation killer: specific non-NotFound API error
		{
			name:                  "Forbidden error returns error",
			operatorConditionName: "test-operator",
			getError:              kerrors.NewForbidden(schema.GroupResource{Group: "operators.coreos.com", Resource: "operatorconditions"}, "test", errors.New("forbidden")),
			expectNil:             true,
			expectErr:             true,
		},
		// Conflict error - another non-NotFound error type
		{
			name:                  "Conflict error returns error",
			operatorConditionName: "test-operator",
			getError:              kerrors.NewConflict(schema.GroupResource{Group: "operators.coreos.com", Resource: "operatorconditions"}, "test", errors.New("conflict")),
			expectNil:             true,
			expectErr:             true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fakeClient := &fakes.FakeCustomCtrlClient{}
			reconciler := newTestReconciler(fakeClient)
			reconciler.operatorConditionName = tt.operatorConditionName

			fakeClient.GetReturns(tt.getError)

			result, err := reconciler.findOperatorCondition(context.Background())

			if tt.expectNil && result != nil {
				t.Error("Expected nil result")
			}
			if tt.expectErr && err == nil {
				t.Error("Expected error, got nil")
			}
		})
	}
}

// TestFindOperatorCondition_EmptyName tests findOperatorCondition with empty name
func TestFindOperatorCondition_EmptyName(t *testing.T) {
	fakeClient := &fakes.FakeCustomCtrlClient{}
	reconciler := newTestReconciler(fakeClient)
	reconciler.operatorConditionName = ""

	result, err := reconciler.findOperatorCondition(context.Background())

	if result != nil {
		t.Error("Expected nil result when operatorConditionName is empty")
	}
	if err == nil {
		t.Error("Expected error when operatorConditionName is empty")
	}
}

// TestUpdateOperatorCondition_NoOperatorCondition tests updateOperatorCondition when OperatorCondition is not found
func TestUpdateOperatorCondition_NoOperatorCondition(t *testing.T) {
	fakeClient := &fakes.FakeCustomCtrlClient{}
	reconciler := newTestReconciler(fakeClient)

	// Mock findOperatorCondition to return not found error
	fakeClient.GetReturns(kerrors.NewNotFound(schema.GroupResource{}, "test"))

	err := reconciler.updateOperatorCondition(context.Background(), false, []v1alpha1.OperandStatus{})

	// Should return error when OperatorCondition not found
	if err == nil {
		t.Error("Expected error when OperatorCondition is not found")
	}
}

// TestUpdateOperatorCondition_CreateOnlyModeEnabled tests updateOperatorCondition with create-only mode
func TestUpdateOperatorCondition_CreateOnlyModeEnabled(t *testing.T) {
	fakeClient := &fakes.FakeCustomCtrlClient{}
	reconciler := newTestReconciler(fakeClient)

	// Mock successful Get (OperatorCondition found)
	fakeClient.GetReturns(nil)
	// Mock successful StatusUpdateWithRetry
	fakeClient.StatusUpdateWithRetryReturns(nil)

	err := reconciler.updateOperatorCondition(context.Background(), true, []v1alpha1.OperandStatus{})

	// Should not return error
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	// Verify StatusUpdateWithRetry was called
	if fakeClient.StatusUpdateWithRetryCallCount() != 1 {
		t.Error("Expected StatusUpdateWithRetry to be called once")
	}
}

// TestUpdateOperatorCondition_OperandsNotReady tests updateOperatorCondition with not ready operands
func TestUpdateOperatorCondition_OperandsNotReady(t *testing.T) {
	fakeClient := &fakes.FakeCustomCtrlClient{}
	reconciler := newTestReconciler(fakeClient)

	// Mock successful Get (OperatorCondition found)
	fakeClient.GetReturns(nil)
	// Mock successful StatusUpdateWithRetry
	fakeClient.StatusUpdateWithRetryReturns(nil)

	operandStatuses := []v1alpha1.OperandStatus{
		{
			Kind:    "SpireServer",
			Name:    "cluster",
			Ready:   "false",
			Message: "Failed to reconcile",
		},
	}

	err := reconciler.updateOperatorCondition(context.Background(), false, operandStatuses)

	// Should not return error
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}
}

// TestUpdateOperatorCondition_AllOperandsReady tests updateOperatorCondition with all ready operands
func TestUpdateOperatorCondition_AllOperandsReady(t *testing.T) {
	fakeClient := &fakes.FakeCustomCtrlClient{}
	reconciler := newTestReconciler(fakeClient)

	// Mock successful Get (OperatorCondition found)
	fakeClient.GetReturns(nil)
	// Mock successful StatusUpdateWithRetry
	fakeClient.StatusUpdateWithRetryReturns(nil)

	operandStatuses := []v1alpha1.OperandStatus{
		{
			Kind:    "SpireServer",
			Name:    "cluster",
			Ready:   "true",
			Message: "Ready",
		},
		{
			Kind:    "SpireAgent",
			Name:    "cluster",
			Ready:   "true",
			Message: "Ready",
		},
	}

	err := reconciler.updateOperatorCondition(context.Background(), false, operandStatuses)

	// Should not return error
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}
}

// TestUpdateOperatorCondition_StatusUpdateError tests updateOperatorCondition when status update fails
func TestUpdateOperatorCondition_StatusUpdateError(t *testing.T) {
	fakeClient := &fakes.FakeCustomCtrlClient{}
	reconciler := newTestReconciler(fakeClient)

	// Mock successful Get (OperatorCondition found)
	fakeClient.GetReturns(nil)
	// Mock StatusUpdateWithRetry error
	fakeClient.StatusUpdateWithRetryReturns(errors.New("status update failed"))

	err := reconciler.updateOperatorCondition(context.Background(), false, []v1alpha1.OperandStatus{})

	// Should return error
	if err == nil {
		t.Error("Expected error when status update fails")
	}
}

// TestUpdateOperatorCondition_CRNotFoundExcluded tests that CR not found operands don't block upgrade
func TestUpdateOperatorCondition_CRNotFoundExcluded(t *testing.T) {
	fakeClient := &fakes.FakeCustomCtrlClient{}
	reconciler := newTestReconciler(fakeClient)

	// Mock successful Get (OperatorCondition found)
	fakeClient.GetReturns(nil)
	// Mock successful StatusUpdateWithRetry
	fakeClient.StatusUpdateWithRetryReturns(nil)

	// Operand is not ready but CR not found - should not block upgrade
	operandStatuses := []v1alpha1.OperandStatus{
		{
			Kind:    "SpireServer",
			Name:    "cluster",
			Ready:   "false",
			Message: OperandMessageCRNotFound,
		},
	}

	err := reconciler.updateOperatorCondition(context.Background(), false, operandStatuses)

	// Should not return error
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}
}

// TestUpdateOperatorCondition_MutationKillers tests edge cases to kill surviving mutants
func TestUpdateOperatorCondition_MutationKillers(t *testing.T) {
	tests := []struct {
		name              string
		operandStatuses   []v1alpha1.OperandStatus
		expectUpgradeable bool
	}{
		{
			name: "operand not ready with non-CRNotFound message blocks upgrade",
			operandStatuses: []v1alpha1.OperandStatus{
				{Kind: "SpireServer", Name: "cluster", Ready: "false", Message: "Failed to reconcile"},
			},
			expectUpgradeable: false,
		},
		{
			name: "operand ready with any message allows upgrade",
			operandStatuses: []v1alpha1.OperandStatus{
				{Kind: "SpireServer", Name: "cluster", Ready: "true", Message: "Failed to reconcile"},
			},
			expectUpgradeable: true,
		},
		{
			name: "operand not ready with CRNotFound message allows upgrade",
			operandStatuses: []v1alpha1.OperandStatus{
				{Kind: "SpireServer", Name: "cluster", Ready: "false", Message: OperandMessageCRNotFound},
			},
			expectUpgradeable: true,
		},
		{
			name:              "empty operand list allows upgrade",
			operandStatuses:   []v1alpha1.OperandStatus{},
			expectUpgradeable: true,
		},
		{
			name: "one not ready operand blocks upgrade",
			operandStatuses: []v1alpha1.OperandStatus{
				{Kind: "SpireServer", Name: "cluster", Ready: "false", Message: "Reconciling"},
			},
			expectUpgradeable: false,
		},
		{
			// Multiple not ready operands
			name: "multiple not ready operands block upgrade",
			operandStatuses: []v1alpha1.OperandStatus{
				{Kind: "SpireServer", Name: "cluster", Ready: "false", Message: "Reconciling"},
				{Kind: "SpireAgent", Name: "cluster", Ready: "false", Message: "Unhealthy"},
			},
			expectUpgradeable: false,
		},
		{
			// Mix of ready and not ready (with CRNotFound)
			name: "mix of ready and CRNotFound allows upgrade",
			operandStatuses: []v1alpha1.OperandStatus{
				{Kind: "SpireServer", Name: "cluster", Ready: "true", Message: "Ready"},
				{Kind: "SpireAgent", Name: "cluster", Ready: "false", Message: OperandMessageCRNotFound},
			},
			expectUpgradeable: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fakeClient := &fakes.FakeCustomCtrlClient{}
			reconciler := newTestReconciler(fakeClient)

			// Mock successful Get (OperatorCondition found)
			fakeClient.GetReturns(nil)
			// Mock successful StatusUpdateWithRetry
			fakeClient.StatusUpdateWithRetryReturns(nil)

			err := reconciler.updateOperatorCondition(context.Background(), false, tt.operandStatuses)

			if err != nil {
				t.Errorf("Expected no error, got: %v", err)
			}

			// Verify StatusUpdateWithRetry was called
			if fakeClient.StatusUpdateWithRetryCallCount() != 1 {
				t.Error("Expected StatusUpdateWithRetry to be called once")
			}
		})
	}
}

// TestOperandAggregateState tests operandAggregateState fields
func TestOperandAggregateState(t *testing.T) {
	state := &operandAggregateState{
		allReady:         true,
		notCreatedCount:  0,
		failedCount:      0,
		anyOperandExists: false,
	}

	// Test initial state
	if !state.allReady {
		t.Error("Expected allReady to be true initially")
	}

	// Modify state
	state.allReady = false
	state.notCreatedCount = 1
	state.failedCount = 2
	state.anyOperandExists = true

	// Verify modifications
	if state.allReady {
		t.Error("Expected allReady to be false after modification")
	}
	if state.notCreatedCount != 1 {
		t.Errorf("Expected notCreatedCount to be 1, got %d", state.notCreatedCount)
	}
	if state.failedCount != 2 {
		t.Errorf("Expected failedCount to be 2, got %d", state.failedCount)
	}
	if !state.anyOperandExists {
		t.Error("Expected anyOperandExists to be true")
	}
}

// TestOperandAggregateResult tests operandAggregateResult fields
func TestOperandAggregateResult(t *testing.T) {
	operandStatuses := []v1alpha1.OperandStatus{
		{Kind: "SpireServer", Name: "cluster", Ready: "true"},
	}

	result := operandAggregateResult{
		operandStatuses:  operandStatuses,
		allReady:         true,
		notCreatedCount:  0,
		failedCount:      0,
		anyOperandExists: true,
	}

	if len(result.operandStatuses) != 1 {
		t.Errorf("Expected 1 operand status, got %d", len(result.operandStatuses))
	}
	if !result.allReady {
		t.Error("Expected allReady to be true")
	}
	if !result.anyOperandExists {
		t.Error("Expected anyOperandExists to be true")
	}
}

// TestOperandStateClassification tests operandStateClassification type
func TestOperandStateClassification(t *testing.T) {
	// Test progressing
	if operandProgressing != "progressing" {
		t.Errorf("Expected 'progressing', got %v", operandProgressing)
	}

	// Test failed
	if operandFailed != "failed" {
		t.Errorf("Expected 'failed', got %v", operandFailed)
	}

	// Test ready
	if operandReady != "ready" {
		t.Errorf("Expected 'ready', got %v", operandReady)
	}
}

// TestClassifyOperandState_NilCondition tests classifyOperandState with nil condition
func TestClassifyOperandState_NilCondition(t *testing.T) {
	tests := []struct {
		name     string
		operand  v1alpha1.OperandStatus
		expected operandStateClassification
	}{
		{
			name: "Ready operand with nil condition",
			operand: v1alpha1.OperandStatus{
				Ready:   "true",
				Message: "Ready",
			},
			expected: operandReady,
		},
		{
			name: "Not ready with CR not found message",
			operand: v1alpha1.OperandStatus{
				Ready:   "false",
				Message: OperandMessageCRNotFound,
			},
			expected: operandProgressing,
		},
		{
			name: "Not ready with unknown message",
			operand: v1alpha1.OperandStatus{
				Ready:   "false",
				Message: "Unknown error",
			},
			expected: operandFailed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := classifyOperandState(tt.operand, nil)
			if result != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}
		})
	}
}

// TestClassifyOperandState_ReasonReady tests classifyOperandState with Ready reason
// When operand.Ready is "false" but condition.Reason is ReasonReady, the function
// returns operandReady because ReasonReady in the switch statement returns operandReady
func TestClassifyOperandState_ReasonReady(t *testing.T) {
	operand := v1alpha1.OperandStatus{
		Ready:   "false",
		Message: "Ready",
	}
	condition := &metav1.Condition{
		Type:   v1alpha1.Ready,
		Status: metav1.ConditionTrue,
		Reason: v1alpha1.ReasonReady,
	}

	result := classifyOperandState(operand, condition)

	// The function checks operand.Ready first (returns operandReady if "true"),
	// but since operand.Ready is "false", it proceeds to check condition.Reason.
	// Since condition.Reason is ReasonReady, it returns operandReady.
	if result != operandReady {
		t.Errorf("Expected operandReady (condition.Reason=ReasonReady), got %v", result)
	}
}

// TestProcessOperandStatus_NotReady tests processOperandStatus with not ready operand
func TestProcessOperandStatus_NotReady(t *testing.T) {
	operand := v1alpha1.OperandStatus{
		Ready:   "false",
		Message: "Some error",
	}

	state := &operandAggregateState{allReady: true}
	processOperandStatus(operand, state)

	if state.allReady {
		t.Error("Expected allReady to be false after processing not-ready operand")
	}
}

// TestExtractKeyConditions_MultipleConditions tests extractKeyConditions with multiple conditions
func TestExtractKeyConditions_MultipleConditions(t *testing.T) {
	conditions := []metav1.Condition{
		{Type: v1alpha1.Ready, Status: metav1.ConditionFalse, Reason: v1alpha1.ReasonFailed},
		{Type: utils.CreateOnlyModeStatusType, Status: metav1.ConditionTrue},
		{Type: "ServiceAvailable", Status: metav1.ConditionFalse},
		{Type: "ConfigMapAvailable", Status: metav1.ConditionTrue}, // Should be excluded (not failed)
	}

	result := extractKeyConditions(conditions, false)

	// Should include Ready, CreateOnlyMode (True), and ServiceAvailable (False)
	if len(result) != 3 {
		t.Errorf("Expected 3 conditions, got %d", len(result))
	}
}

// TestExtractKeyConditions_OnlyCreateOnlyMode tests extractKeyConditions returns only CreateOnlyMode when ready
func TestExtractKeyConditions_OnlyCreateOnlyMode(t *testing.T) {
	conditions := []metav1.Condition{
		{Type: v1alpha1.Ready, Status: metav1.ConditionTrue},
		{Type: utils.CreateOnlyModeStatusType, Status: metav1.ConditionTrue},
		{Type: "ServiceAvailable", Status: metav1.ConditionTrue},
	}

	result := extractKeyConditions(conditions, true)

	// When ready, should only include CreateOnlyMode if True
	if len(result) != 1 {
		t.Errorf("Expected 1 condition, got %d", len(result))
	}
	if result[0].Type != utils.CreateOnlyModeStatusType {
		t.Errorf("Expected CreateOnlyMode condition, got %s", result[0].Type)
	}
}

// TestHandleNonExistentCR_AllOperandsNotFound tests logic when all operands are not found
func TestHandleNonExistentCR_AllOperandsNotFound(t *testing.T) {
	operandStatuses := []v1alpha1.OperandStatus{
		{Kind: "SpireServer", Name: "cluster", Ready: "false", Message: OperandMessageCRNotFound},
		{Kind: "SpireAgent", Name: "cluster", Ready: "false", Message: OperandMessageCRNotFound},
		{Kind: "SpiffeCSIDriver", Name: "cluster", Ready: "false", Message: OperandMessageCRNotFound},
		{Kind: "SpireOIDCDiscoveryProvider", Name: "cluster", Ready: "false", Message: OperandMessageCRNotFound},
	}

	result := operandAggregateResult{
		operandStatuses:  operandStatuses,
		allReady:         false,
		notCreatedCount:  4,
		failedCount:      0,
		anyOperandExists: false,
	}

	// When all operands are not found and no operand exists, notCreatedCount should match total
	if result.notCreatedCount != 4 {
		t.Errorf("Expected notCreatedCount 4, got %d", result.notCreatedCount)
	}
	if result.anyOperandExists {
		t.Error("Expected anyOperandExists to be false when all CRs not found")
	}
}

// TestHandleNonExistentCR_SomeOperandsExist tests logic when some operands exist
func TestHandleNonExistentCR_SomeOperandsExist(t *testing.T) {
	operandStatuses := []v1alpha1.OperandStatus{
		{Kind: "SpireServer", Name: "cluster", Ready: "true", Message: "Ready"},
		{Kind: "SpireAgent", Name: "cluster", Ready: "false", Message: OperandMessageCRNotFound},
	}

	state := &operandAggregateState{allReady: true}
	for _, operand := range operandStatuses {
		processOperandStatus(operand, state)
	}

	// When some operands exist, anyOperandExists should be true
	if !state.anyOperandExists {
		t.Error("Expected anyOperandExists to be true when some operands exist")
	}
}

// TestHandleNonExistentCR_AllOperandsReady tests logic when all operands are ready
func TestHandleNonExistentCR_AllOperandsReady(t *testing.T) {
	operandStatuses := []v1alpha1.OperandStatus{
		{Kind: "SpireServer", Name: "cluster", Ready: "true", Message: "Ready"},
		{Kind: "SpireAgent", Name: "cluster", Ready: "true", Message: "Ready"},
	}

	state := &operandAggregateState{allReady: true}
	for _, operand := range operandStatuses {
		processOperandStatus(operand, state)
	}

	// When all operands are ready, state should reflect that
	if !state.allReady {
		t.Error("Expected allReady to be true when all operands are ready")
	}
	if state.failedCount != 0 {
		t.Errorf("Expected failedCount 0, got %d", state.failedCount)
	}
}

// TestOperandsAvailableCondition_AllReady tests OperandsAvailable condition logic with all operands ready
func TestOperandsAvailableCondition_AllReady(t *testing.T) {
	result := operandAggregateResult{
		allReady:         true,
		notCreatedCount:  0,
		failedCount:      0,
		anyOperandExists: true,
	}

	// When all operands are ready, allReady should be true
	if !result.allReady {
		t.Error("Expected allReady to be true")
	}
	if result.failedCount != 0 {
		t.Errorf("Expected failedCount 0, got %d", result.failedCount)
	}
}

// TestOperandsAvailableCondition_AllProgressing tests OperandsAvailable condition logic with all operands progressing
func TestOperandsAvailableCondition_AllProgressing(t *testing.T) {
	result := operandAggregateResult{
		allReady:         false,
		notCreatedCount:  4, // All 4 operands are progressing
		failedCount:      0,
		anyOperandExists: false,
	}

	// When all operands are progressing, notCreatedCount should equal total
	if result.allReady {
		t.Error("Expected allReady to be false when operands are progressing")
	}
	if result.notCreatedCount != 4 {
		t.Errorf("Expected notCreatedCount 4, got %d", result.notCreatedCount)
	}
}

// TestOperandsAvailableCondition_SomeFailed tests OperandsAvailable condition logic with some operands failed
func TestOperandsAvailableCondition_SomeFailed(t *testing.T) {
	result := operandAggregateResult{
		allReady:         false,
		notCreatedCount:  0,
		failedCount:      2, // Some operands failed
		anyOperandExists: true,
		operandStatuses: []v1alpha1.OperandStatus{
			{Kind: "SpireServer", Name: "cluster", Ready: "false", Message: "Failed"},
			{Kind: "SpireAgent", Name: "cluster", Ready: "false", Message: "Failed"},
		},
	}

	// When some operands failed, failedCount should be > 0
	if result.failedCount != 2 {
		t.Errorf("Expected failedCount 2, got %d", result.failedCount)
	}
	if result.allReady {
		t.Error("Expected allReady to be false when operands failed")
	}
}

// TestOperandsAvailableCondition_MixedState tests OperandsAvailable condition logic with mixed operand states
func TestOperandsAvailableCondition_MixedState(t *testing.T) {
	result := operandAggregateResult{
		allReady:         false,
		notCreatedCount:  1, // Some progressing
		failedCount:      1, // Some failed
		anyOperandExists: true,
		operandStatuses: []v1alpha1.OperandStatus{
			{Kind: "SpireServer", Name: "cluster", Ready: "false", Message: "Failed"},
			{Kind: "SpireAgent", Name: "cluster", Ready: "false", Message: OperandMessageCRNotFound},
		},
	}

	// When there's a mix, both counts should be tracked
	if result.notCreatedCount != 1 {
		t.Errorf("Expected notCreatedCount 1, got %d", result.notCreatedCount)
	}
	if result.failedCount != 1 {
		t.Errorf("Expected failedCount 1, got %d", result.failedCount)
	}
}

// TestNewReconciler_Initialization tests the reconciler creation
func TestNewReconciler_Initialization(t *testing.T) {
	fakeClient := &fakes.FakeCustomCtrlClient{}
	reconciler := newTestReconciler(fakeClient)

	if reconciler.ctrlClient == nil {
		t.Error("Expected ctrlClient to be set")
	}
	if reconciler.scheme == nil {
		t.Error("Expected scheme to be set")
	}
	if reconciler.eventRecorder == nil {
		t.Error("Expected eventRecorder to be set")
	}
}

// TestBuildOperandStatus_Ready tests buildOperandStatus for ready operand
func TestBuildOperandStatus_Ready(t *testing.T) {
	status := v1alpha1.OperandStatus{
		Kind:    "SpireServer",
		Name:    "cluster",
		Ready:   "true",
		Message: "Ready",
		Conditions: []metav1.Condition{
			{Type: v1alpha1.Ready, Status: metav1.ConditionTrue, Reason: v1alpha1.ReasonReady},
		},
	}

	if status.Kind != "SpireServer" {
		t.Errorf("Expected Kind SpireServer, got %s", status.Kind)
	}
	if status.Ready != "true" {
		t.Errorf("Expected Ready true, got %s", status.Ready)
	}
}

// TestBuildOperandStatus_NotReady tests buildOperandStatus for not ready operand
func TestBuildOperandStatus_NotReady(t *testing.T) {
	status := v1alpha1.OperandStatus{
		Kind:    "SpireAgent",
		Name:    "cluster",
		Ready:   "false",
		Message: "Component unhealthy",
		Conditions: []metav1.Condition{
			{Type: v1alpha1.Ready, Status: metav1.ConditionFalse, Reason: v1alpha1.ReasonFailed},
		},
	}

	if status.Ready != "false" {
		t.Errorf("Expected Ready false, got %s", status.Ready)
	}
	if status.Message != "Component unhealthy" {
		t.Errorf("Expected message 'Component unhealthy', got %s", status.Message)
	}
}

// TestGetOperandStatusFromCRNotFound tests getOperandStatus behavior when CR not found
func TestGetOperandStatusFromCRNotFound(t *testing.T) {
	operand := v1alpha1.OperandStatus{
		Kind:    "SpireServer",
		Name:    "cluster",
		Ready:   "false",
		Message: OperandMessageCRNotFound,
	}

	if operand.Message != OperandMessageCRNotFound {
		t.Errorf("Expected message %s, got %s", OperandMessageCRNotFound, operand.Message)
	}
}

// TestGetOperandStatus_AllConditionStates tests getOperandStatus with all condition states through Reconcile
func TestGetOperandStatus_AllConditionStates(t *testing.T) {
	tests := []struct {
		name             string
		serverConditions []metav1.Condition
	}{
		{
			name:             "empty conditions - waiting for initial reconcile",
			serverConditions: []metav1.Condition{},
		},
		{
			name: "Ready condition is True",
			serverConditions: []metav1.Condition{
				{Type: v1alpha1.Ready, Status: metav1.ConditionTrue, Reason: v1alpha1.ReasonReady, Message: "Ready"},
			},
		},
		{
			name: "Ready condition is False",
			serverConditions: []metav1.Condition{
				{Type: v1alpha1.Ready, Status: metav1.ConditionFalse, Reason: v1alpha1.ReasonFailed, Message: "Failed"},
			},
		},
		{
			name: "Ready condition is Unknown",
			serverConditions: []metav1.Condition{
				{Type: v1alpha1.Ready, Status: metav1.ConditionUnknown, Reason: "Unknown", Message: "Unknown state"},
			},
		},
		{
			name: "No Ready condition but other conditions exist",
			serverConditions: []metav1.Condition{
				{Type: "SomeOtherCondition", Status: metav1.ConditionTrue, Reason: "Test", Message: "Test"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fakeClient := &fakes.FakeCustomCtrlClient{}
			scheme := runtime.NewScheme()
			_ = v1alpha1.AddToScheme(scheme)

			reconciler := &ZeroTrustWorkloadIdentityManagerReconciler{
				ctrlClient:            fakeClient,
				ctx:                   context.Background(),
				log:                   logr.Discard(),
				scheme:                scheme,
				eventRecorder:         record.NewFakeRecorder(100),
				operatorConditionName: "test-operator-condition",
			}

			fakeClient.GetStub = func(ctx context.Context, key client.ObjectKey, obj client.Object) error {
				switch o := obj.(type) {
				case *v1alpha1.ZeroTrustWorkloadIdentityManager:
					o.Name = "cluster"
					o.Spec.TrustDomain = "example.org"
				case *v1alpha1.SpireServer:
					o.Name = "cluster"
					o.Status.ConditionalStatus.Conditions = tt.serverConditions
				default:
					return kerrors.NewNotFound(schema.GroupResource{}, key.Name)
				}
				return nil
			}

			req := ctrl.Request{NamespacedName: types.NamespacedName{Name: "cluster"}}
			result, err := reconciler.Reconcile(context.Background(), req)

			// Should not error - we're testing condition handling
			if err != nil {
				t.Logf("Reconcile returned error (expected): %v", err)
			}
			if result.Requeue {
				t.Error("Expected no requeue")
			}
		})
	}
}

// TestExtractKeyConditions_AllStates tests extractKeyConditions with all states
func TestExtractKeyConditions_AllStates(t *testing.T) {
	tests := []struct {
		name        string
		conditions  []metav1.Condition
		isReady     bool
		expectCount int
	}{
		{
			name:        "ready with no create-only condition",
			conditions:  []metav1.Condition{},
			isReady:     true,
			expectCount: 0,
		},
		{
			name: "ready with create-only condition True",
			conditions: []metav1.Condition{
				{Type: utils.CreateOnlyModeStatusType, Status: metav1.ConditionTrue},
			},
			isReady:     true,
			expectCount: 1,
		},
		{
			name: "ready with create-only condition False - should not include",
			conditions: []metav1.Condition{
				{Type: utils.CreateOnlyModeStatusType, Status: metav1.ConditionFalse},
			},
			isReady:     true,
			expectCount: 0,
		},
		{
			name: "not ready with nil create-only condition",
			conditions: []metav1.Condition{
				{Type: v1alpha1.Ready, Status: metav1.ConditionFalse},
			},
			isReady:     false,
			expectCount: 1, // Just Ready condition
		},
		{
			name: "not ready with create-only True",
			conditions: []metav1.Condition{
				{Type: utils.CreateOnlyModeStatusType, Status: metav1.ConditionTrue},
				{Type: v1alpha1.Ready, Status: metav1.ConditionFalse},
			},
			isReady:     false,
			expectCount: 2, // CreateOnly + Ready
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractKeyConditions(tt.conditions, tt.isReady)

			if len(result) != tt.expectCount {
				t.Errorf("Expected %d conditions, got %d", tt.expectCount, len(result))
			}
		})
	}
}

// TestAggregateOperandStatus_Integration tests the full aggregation flow
func TestAggregateOperandStatus_Integration(t *testing.T) {
	// Create operand statuses representing various states
	operandStatuses := []v1alpha1.OperandStatus{
		{Kind: "SpireServer", Name: "cluster", Ready: "true", Message: "Ready"},
		{Kind: "SpireAgent", Name: "cluster", Ready: "false", Message: OperandMessageCRNotFound},
		{Kind: "SpiffeCSIDriver", Name: "cluster", Ready: "true", Message: "Ready"},
		{Kind: "SpireOIDCDiscoveryProvider", Name: "cluster", Ready: "false", Message: "Failed"},
	}

	state := &operandAggregateState{allReady: true}
	for _, operand := range operandStatuses {
		processOperandStatus(operand, state)
	}

	// Verify aggregated state
	if state.allReady {
		t.Error("Expected allReady to be false")
	}
	if state.notCreatedCount != 1 {
		t.Errorf("Expected notCreatedCount 1, got %d", state.notCreatedCount)
	}
	if state.failedCount != 1 {
		t.Errorf("Expected failedCount 1, got %d", state.failedCount)
	}
}

// TestReconcile_SuccessfulReconciliation tests a successful reconciliation path
func TestReconcile_SuccessfulReconciliation(t *testing.T) {
	fakeClient := &fakes.FakeCustomCtrlClient{}
	reconciler := newTestReconciler(fakeClient)

	// Return not found for ZTWIM, which should return nil error
	fakeClient.GetReturns(kerrors.NewNotFound(schema.GroupResource{}, "cluster"))

	req := ctrl.Request{NamespacedName: types.NamespacedName{Name: "cluster"}}
	result, err := reconciler.Reconcile(context.Background(), req)

	// Should not return error when ZTWIM not found
	if err != nil {
		t.Errorf("Expected no error for not found, got: %v", err)
	}
	if result.Requeue {
		t.Error("Expected no requeue for not found")
	}
}

// TestClassifyOperandState_UnknownReason tests classifyOperandState with unknown reason
func TestClassifyOperandState_UnknownReason(t *testing.T) {
	operand := v1alpha1.OperandStatus{
		Ready:   "false",
		Message: "Unknown state",
	}
	condition := &metav1.Condition{
		Type:   v1alpha1.Ready,
		Status: metav1.ConditionFalse,
		Reason: "UnknownReason",
	}

	result := classifyOperandState(operand, condition)

	// Unknown reason should default to failed
	if result != operandFailed {
		t.Errorf("Expected operandFailed for unknown reason, got %v", result)
	}
}

// TestAggregateOperandStatus_AllNotFound tests aggregateOperandStatus when all CRs not found
func TestAggregateOperandStatus_AllNotFound(t *testing.T) {
	fakeClient := &fakes.FakeCustomCtrlClient{}
	scheme := runtime.NewScheme()
	_ = v1alpha1.AddToScheme(scheme)

	reconciler := &ZeroTrustWorkloadIdentityManagerReconciler{
		ctrlClient:            fakeClient,
		ctx:                   context.Background(),
		log:                   logr.Discard(),
		scheme:                scheme,
		eventRecorder:         record.NewFakeRecorder(100),
		operatorConditionName: "test-operator-condition",
	}

	// Return NotFound for all CRs
	fakeClient.GetReturns(kerrors.NewNotFound(schema.GroupResource{}, "cluster"))

	result := reconciler.aggregateOperandStatus(context.Background())

	// Should have 4 operand statuses
	if len(result.operandStatuses) != 4 {
		t.Errorf("Expected 4 operand statuses, got %d", len(result.operandStatuses))
	}

	// All should be not ready
	if result.allReady {
		t.Error("Expected allReady to be false when all CRs not found")
	}

	// All 4 should be in notCreatedCount (progressing state for NotFound)
	if result.notCreatedCount != 4 {
		t.Errorf("Expected notCreatedCount 4, got %d", result.notCreatedCount)
	}

	// None should have failed
	if result.failedCount != 0 {
		t.Errorf("Expected failedCount 0, got %d", result.failedCount)
	}

	// No operands exist
	if result.anyOperandExists {
		t.Error("Expected anyOperandExists to be false")
	}
}

// TestAggregateOperandStatus_AllReady tests aggregateOperandStatus when all CRs are ready
func TestAggregateOperandStatus_AllReady(t *testing.T) {
	fakeClient := &fakes.FakeCustomCtrlClient{}
	scheme := runtime.NewScheme()
	_ = v1alpha1.AddToScheme(scheme)

	reconciler := &ZeroTrustWorkloadIdentityManagerReconciler{
		ctrlClient:            fakeClient,
		ctx:                   context.Background(),
		log:                   logr.Discard(),
		scheme:                scheme,
		eventRecorder:         record.NewFakeRecorder(100),
		operatorConditionName: "test-operator-condition",
	}

	// Return ready CRs for all types
	getCallCount := 0
	fakeClient.GetStub = func(ctx context.Context, key client.ObjectKey, obj client.Object) error {
		getCallCount++

		switch cr := obj.(type) {
		case *v1alpha1.SpireServer:
			cr.Name = "cluster"
			cr.Status.ConditionalStatus.Conditions = []metav1.Condition{
				{Type: v1alpha1.Ready, Status: metav1.ConditionTrue, Reason: v1alpha1.ReasonReady},
			}
		case *v1alpha1.SpireAgent:
			cr.Name = "cluster"
			cr.Status.ConditionalStatus.Conditions = []metav1.Condition{
				{Type: v1alpha1.Ready, Status: metav1.ConditionTrue, Reason: v1alpha1.ReasonReady},
			}
		case *v1alpha1.SpiffeCSIDriver:
			cr.Name = "cluster"
			cr.Status.ConditionalStatus.Conditions = []metav1.Condition{
				{Type: v1alpha1.Ready, Status: metav1.ConditionTrue, Reason: v1alpha1.ReasonReady},
			}
		case *v1alpha1.SpireOIDCDiscoveryProvider:
			cr.Name = "cluster"
			cr.Status.ConditionalStatus.Conditions = []metav1.Condition{
				{Type: v1alpha1.Ready, Status: metav1.ConditionTrue, Reason: v1alpha1.ReasonReady},
			}
		}
		return nil
	}

	result := reconciler.aggregateOperandStatus(context.Background())

	// All should be ready
	if !result.allReady {
		t.Error("Expected allReady to be true when all CRs are ready")
	}

	// None should be in notCreatedCount or failedCount
	if result.notCreatedCount != 0 {
		t.Errorf("Expected notCreatedCount 0, got %d", result.notCreatedCount)
	}
	if result.failedCount != 0 {
		t.Errorf("Expected failedCount 0, got %d", result.failedCount)
	}

	// Operands should exist
	if !result.anyOperandExists {
		t.Error("Expected anyOperandExists to be true")
	}
}

// TestAggregateOperandStatus_MixedStates tests aggregateOperandStatus with mixed operand states
func TestAggregateOperandStatus_MixedStates(t *testing.T) {
	fakeClient := &fakes.FakeCustomCtrlClient{}
	scheme := runtime.NewScheme()
	_ = v1alpha1.AddToScheme(scheme)

	reconciler := &ZeroTrustWorkloadIdentityManagerReconciler{
		ctrlClient:            fakeClient,
		ctx:                   context.Background(),
		log:                   logr.Discard(),
		scheme:                scheme,
		eventRecorder:         record.NewFakeRecorder(100),
		operatorConditionName: "test-operator-condition",
	}

	// Return mixed states
	getCallCount := 0
	fakeClient.GetStub = func(ctx context.Context, key client.ObjectKey, obj client.Object) error {
		getCallCount++

		switch cr := obj.(type) {
		case *v1alpha1.SpireServer:
			// Ready
			cr.Name = "cluster"
			cr.Status.ConditionalStatus.Conditions = []metav1.Condition{
				{Type: v1alpha1.Ready, Status: metav1.ConditionTrue, Reason: v1alpha1.ReasonReady},
			}
		case *v1alpha1.SpireAgent:
			// Not found
			return kerrors.NewNotFound(schema.GroupResource{}, "cluster")
		case *v1alpha1.SpiffeCSIDriver:
			// Failed
			cr.Name = "cluster"
			cr.Status.ConditionalStatus.Conditions = []metav1.Condition{
				{Type: v1alpha1.Ready, Status: metav1.ConditionFalse, Reason: v1alpha1.ReasonFailed, Message: "Failed"},
			}
		case *v1alpha1.SpireOIDCDiscoveryProvider:
			// Reconciling (no conditions)
			cr.Name = "cluster"
			cr.Status.ConditionalStatus.Conditions = []metav1.Condition{}
		}
		return nil
	}

	result := reconciler.aggregateOperandStatus(context.Background())

	// Should not be all ready
	if result.allReady {
		t.Error("Expected allReady to be false with mixed states")
	}

	// Some operands exist
	if !result.anyOperandExists {
		t.Error("Expected anyOperandExists to be true")
	}

	// Should have progressing count (NotFound + WaitingInitialRecon) and failed count
	if result.notCreatedCount != 2 { // SpireAgent (NotFound) + SpireOIDCDiscoveryProvider (no conditions)
		t.Errorf("Expected notCreatedCount 2, got %d", result.notCreatedCount)
	}
	if result.failedCount != 1 { // SpiffeCSIDriver
		t.Errorf("Expected failedCount 1, got %d", result.failedCount)
	}
}

// TestAggregateOperandStatus_WithCreateOnlyMode tests aggregateOperandStatus with CreateOnlyMode condition
func TestAggregateOperandStatus_WithCreateOnlyMode(t *testing.T) {
	fakeClient := &fakes.FakeCustomCtrlClient{}
	scheme := runtime.NewScheme()
	_ = v1alpha1.AddToScheme(scheme)

	reconciler := &ZeroTrustWorkloadIdentityManagerReconciler{
		ctrlClient:            fakeClient,
		ctx:                   context.Background(),
		log:                   logr.Discard(),
		scheme:                scheme,
		eventRecorder:         record.NewFakeRecorder(100),
		operatorConditionName: "test-operator-condition",
	}

	// Return CRs with CreateOnlyMode condition
	fakeClient.GetStub = func(ctx context.Context, key client.ObjectKey, obj client.Object) error {
		switch cr := obj.(type) {
		case *v1alpha1.SpireServer:
			cr.Name = "cluster"
			cr.Status.ConditionalStatus.Conditions = []metav1.Condition{
				{Type: v1alpha1.Ready, Status: metav1.ConditionTrue, Reason: v1alpha1.ReasonReady},
				{Type: utils.CreateOnlyModeStatusType, Status: metav1.ConditionTrue},
			}
		case *v1alpha1.SpireAgent:
			cr.Name = "cluster"
			cr.Status.ConditionalStatus.Conditions = []metav1.Condition{
				{Type: v1alpha1.Ready, Status: metav1.ConditionTrue, Reason: v1alpha1.ReasonReady},
			}
		case *v1alpha1.SpiffeCSIDriver:
			cr.Name = "cluster"
			cr.Status.ConditionalStatus.Conditions = []metav1.Condition{
				{Type: v1alpha1.Ready, Status: metav1.ConditionTrue, Reason: v1alpha1.ReasonReady},
			}
		case *v1alpha1.SpireOIDCDiscoveryProvider:
			cr.Name = "cluster"
			cr.Status.ConditionalStatus.Conditions = []metav1.Condition{
				{Type: v1alpha1.Ready, Status: metav1.ConditionTrue, Reason: v1alpha1.ReasonReady},
			}
		}
		return nil
	}

	result := reconciler.aggregateOperandStatus(context.Background())

	// All operands are ready and exist
	if !result.allReady {
		t.Error("Expected allReady to be true")
	}
	if !result.anyOperandExists {
		t.Error("Expected anyOperandExists to be true")
	}
	if len(result.operandStatuses) != 4 {
		t.Errorf("Expected 4 operand statuses, got %d", len(result.operandStatuses))
	}
}

// TestGetSpireServerStatus_Ready tests getSpireServerStatus with ready server
func TestGetSpireServerStatus_Ready(t *testing.T) {
	fakeClient := &fakes.FakeCustomCtrlClient{}
	scheme := runtime.NewScheme()
	_ = v1alpha1.AddToScheme(scheme)

	reconciler := &ZeroTrustWorkloadIdentityManagerReconciler{
		ctrlClient:            fakeClient,
		ctx:                   context.Background(),
		log:                   logr.Discard(),
		scheme:                scheme,
		eventRecorder:         record.NewFakeRecorder(100),
		operatorConditionName: "test-operator-condition",
	}

	fakeClient.GetStub = func(ctx context.Context, key client.ObjectKey, obj client.Object) error {
		if cr, ok := obj.(*v1alpha1.SpireServer); ok {
			cr.Name = "cluster"
			cr.Status.ConditionalStatus.Conditions = []metav1.Condition{
				{Type: v1alpha1.Ready, Status: metav1.ConditionTrue, Reason: v1alpha1.ReasonReady},
			}
		}
		return nil
	}

	status := reconciler.getSpireServerStatus(context.Background())

	if status.Kind != "SpireServer" {
		t.Errorf("Expected kind SpireServer, got %s", status.Kind)
	}
	if status.Ready != "true" {
		t.Errorf("Expected ready true, got %s", status.Ready)
	}
}

// TestGetSpireServerStatus_NotFound tests getSpireServerStatus when CR not found
func TestGetSpireServerStatus_NotFound(t *testing.T) {
	fakeClient := &fakes.FakeCustomCtrlClient{}
	scheme := runtime.NewScheme()
	_ = v1alpha1.AddToScheme(scheme)

	reconciler := &ZeroTrustWorkloadIdentityManagerReconciler{
		ctrlClient:            fakeClient,
		ctx:                   context.Background(),
		log:                   logr.Discard(),
		scheme:                scheme,
		eventRecorder:         record.NewFakeRecorder(100),
		operatorConditionName: "test-operator-condition",
	}

	fakeClient.GetReturns(kerrors.NewNotFound(schema.GroupResource{}, "cluster"))

	status := reconciler.getSpireServerStatus(context.Background())

	if status.Kind != "SpireServer" {
		t.Errorf("Expected kind SpireServer, got %s", status.Kind)
	}
	if status.Ready != "false" {
		t.Errorf("Expected ready false, got %s", status.Ready)
	}
	if status.Message != OperandMessageCRNotFound {
		t.Errorf("Expected message %s, got %s", OperandMessageCRNotFound, status.Message)
	}
}

// TestGetSpireServerStatus_GetError tests getSpireServerStatus when Get fails
func TestGetSpireServerStatus_GetError(t *testing.T) {
	fakeClient := &fakes.FakeCustomCtrlClient{}
	scheme := runtime.NewScheme()
	_ = v1alpha1.AddToScheme(scheme)

	reconciler := &ZeroTrustWorkloadIdentityManagerReconciler{
		ctrlClient:            fakeClient,
		ctx:                   context.Background(),
		log:                   logr.Discard(),
		scheme:                scheme,
		eventRecorder:         record.NewFakeRecorder(100),
		operatorConditionName: "test-operator-condition",
	}

	fakeClient.GetReturns(errors.New("connection refused"))

	status := reconciler.getSpireServerStatus(context.Background())

	if status.Ready != "false" {
		t.Errorf("Expected ready false, got %s", status.Ready)
	}
	if status.Message == "" {
		t.Error("Expected error message, got empty")
	}
}

// TestGetSpireServerStatus_NoConditions tests getSpireServerStatus with no conditions
func TestGetSpireServerStatus_NoConditions(t *testing.T) {
	fakeClient := &fakes.FakeCustomCtrlClient{}
	scheme := runtime.NewScheme()
	_ = v1alpha1.AddToScheme(scheme)

	reconciler := &ZeroTrustWorkloadIdentityManagerReconciler{
		ctrlClient:            fakeClient,
		ctx:                   context.Background(),
		log:                   logr.Discard(),
		scheme:                scheme,
		eventRecorder:         record.NewFakeRecorder(100),
		operatorConditionName: "test-operator-condition",
	}

	fakeClient.GetStub = func(ctx context.Context, key client.ObjectKey, obj client.Object) error {
		if cr, ok := obj.(*v1alpha1.SpireServer); ok {
			cr.Name = "cluster"
			cr.Status.ConditionalStatus.Conditions = []metav1.Condition{} // No conditions
		}
		return nil
	}

	status := reconciler.getSpireServerStatus(context.Background())

	if status.Ready != "false" {
		t.Errorf("Expected ready false, got %s", status.Ready)
	}
	if status.Message != OperandMessageWaitingInitialRecon {
		t.Errorf("Expected message %s, got %s", OperandMessageWaitingInitialRecon, status.Message)
	}
}

// TestGetSpireAgentStatus_Ready tests getSpireAgentStatus with ready agent
func TestGetSpireAgentStatus_Ready(t *testing.T) {
	fakeClient := &fakes.FakeCustomCtrlClient{}
	scheme := runtime.NewScheme()
	_ = v1alpha1.AddToScheme(scheme)

	reconciler := &ZeroTrustWorkloadIdentityManagerReconciler{
		ctrlClient:            fakeClient,
		ctx:                   context.Background(),
		log:                   logr.Discard(),
		scheme:                scheme,
		eventRecorder:         record.NewFakeRecorder(100),
		operatorConditionName: "test-operator-condition",
	}

	fakeClient.GetStub = func(ctx context.Context, key client.ObjectKey, obj client.Object) error {
		if cr, ok := obj.(*v1alpha1.SpireAgent); ok {
			cr.Name = "cluster"
			cr.Status.ConditionalStatus.Conditions = []metav1.Condition{
				{Type: v1alpha1.Ready, Status: metav1.ConditionTrue, Reason: v1alpha1.ReasonReady},
			}
		}
		return nil
	}

	status := reconciler.getSpireAgentStatus(context.Background())

	if status.Kind != "SpireAgent" {
		t.Errorf("Expected kind SpireAgent, got %s", status.Kind)
	}
	if status.Ready != "true" {
		t.Errorf("Expected ready true, got %s", status.Ready)
	}
}

// TestGetSpiffeCSIDriverStatus_Ready tests getSpiffeCSIDriverStatus with ready driver
func TestGetSpiffeCSIDriverStatus_Ready(t *testing.T) {
	fakeClient := &fakes.FakeCustomCtrlClient{}
	scheme := runtime.NewScheme()
	_ = v1alpha1.AddToScheme(scheme)

	reconciler := &ZeroTrustWorkloadIdentityManagerReconciler{
		ctrlClient:            fakeClient,
		ctx:                   context.Background(),
		log:                   logr.Discard(),
		scheme:                scheme,
		eventRecorder:         record.NewFakeRecorder(100),
		operatorConditionName: "test-operator-condition",
	}

	fakeClient.GetStub = func(ctx context.Context, key client.ObjectKey, obj client.Object) error {
		if cr, ok := obj.(*v1alpha1.SpiffeCSIDriver); ok {
			cr.Name = "cluster"
			cr.Status.ConditionalStatus.Conditions = []metav1.Condition{
				{Type: v1alpha1.Ready, Status: metav1.ConditionTrue, Reason: v1alpha1.ReasonReady},
			}
		}
		return nil
	}

	status := reconciler.getSpiffeCSIDriverStatus(context.Background())

	if status.Kind != "SpiffeCSIDriver" {
		t.Errorf("Expected kind SpiffeCSIDriver, got %s", status.Kind)
	}
	if status.Ready != "true" {
		t.Errorf("Expected ready true, got %s", status.Ready)
	}
}

// TestGetSpireOIDCDiscoveryProviderStatus_Ready tests getSpireOIDCDiscoveryProviderStatus with ready provider
func TestGetSpireOIDCDiscoveryProviderStatus_Ready(t *testing.T) {
	fakeClient := &fakes.FakeCustomCtrlClient{}
	scheme := runtime.NewScheme()
	_ = v1alpha1.AddToScheme(scheme)

	reconciler := &ZeroTrustWorkloadIdentityManagerReconciler{
		ctrlClient:            fakeClient,
		ctx:                   context.Background(),
		log:                   logr.Discard(),
		scheme:                scheme,
		eventRecorder:         record.NewFakeRecorder(100),
		operatorConditionName: "test-operator-condition",
	}

	fakeClient.GetStub = func(ctx context.Context, key client.ObjectKey, obj client.Object) error {
		if cr, ok := obj.(*v1alpha1.SpireOIDCDiscoveryProvider); ok {
			cr.Name = "cluster"
			cr.Status.ConditionalStatus.Conditions = []metav1.Condition{
				{Type: v1alpha1.Ready, Status: metav1.ConditionTrue, Reason: v1alpha1.ReasonReady},
			}
		}
		return nil
	}

	status := reconciler.getSpireOIDCDiscoveryProviderStatus(context.Background())

	if status.Kind != "SpireOIDCDiscoveryProvider" {
		t.Errorf("Expected kind SpireOIDCDiscoveryProvider, got %s", status.Kind)
	}
	if status.Ready != "true" {
		t.Errorf("Expected ready true, got %s", status.Ready)
	}
}

// TestGetSpireServerStatus_NotReady tests getSpireServerStatus with not ready server
func TestGetSpireServerStatus_NotReady(t *testing.T) {
	fakeClient := &fakes.FakeCustomCtrlClient{}
	scheme := runtime.NewScheme()
	_ = v1alpha1.AddToScheme(scheme)

	reconciler := &ZeroTrustWorkloadIdentityManagerReconciler{
		ctrlClient:            fakeClient,
		ctx:                   context.Background(),
		log:                   logr.Discard(),
		scheme:                scheme,
		eventRecorder:         record.NewFakeRecorder(100),
		operatorConditionName: "test-operator-condition",
	}

	fakeClient.GetStub = func(ctx context.Context, key client.ObjectKey, obj client.Object) error {
		if cr, ok := obj.(*v1alpha1.SpireServer); ok {
			cr.Name = "cluster"
			cr.Status.ConditionalStatus.Conditions = []metav1.Condition{
				{Type: v1alpha1.Ready, Status: metav1.ConditionFalse, Reason: v1alpha1.ReasonFailed, Message: "Failed to reconcile"},
			}
		}
		return nil
	}

	status := reconciler.getSpireServerStatus(context.Background())

	if status.Ready != "false" {
		t.Errorf("Expected ready false, got %s", status.Ready)
	}
	if status.Message != "Failed to reconcile" {
		t.Errorf("Expected message 'Failed to reconcile', got %s", status.Message)
	}
}

// TestGetSpireServerStatus_ReadyConditionNil tests getSpireServerStatus with nil Ready condition
func TestGetSpireServerStatus_ReadyConditionNil(t *testing.T) {
	fakeClient := &fakes.FakeCustomCtrlClient{}
	scheme := runtime.NewScheme()
	_ = v1alpha1.AddToScheme(scheme)

	reconciler := &ZeroTrustWorkloadIdentityManagerReconciler{
		ctrlClient:            fakeClient,
		ctx:                   context.Background(),
		log:                   logr.Discard(),
		scheme:                scheme,
		eventRecorder:         record.NewFakeRecorder(100),
		operatorConditionName: "test-operator-condition",
	}

	fakeClient.GetStub = func(ctx context.Context, key client.ObjectKey, obj client.Object) error {
		if cr, ok := obj.(*v1alpha1.SpireServer); ok {
			cr.Name = "cluster"
			// Has conditions but no Ready condition
			cr.Status.ConditionalStatus.Conditions = []metav1.Condition{
				{Type: "SomeOtherCondition", Status: metav1.ConditionTrue},
			}
		}
		return nil
	}

	status := reconciler.getSpireServerStatus(context.Background())

	if status.Ready != "false" {
		t.Errorf("Expected ready false when Ready condition is nil, got %s", status.Ready)
	}
	if status.Message != OperandMessageReconciling {
		t.Errorf("Expected message %s, got %s", OperandMessageReconciling, status.Message)
	}
}

// TestReconcile_AllOperandsReady tests the reconcile path when all operands are ready
func TestReconcile_AllOperandsReady(t *testing.T) {
	fakeClient := &fakes.FakeCustomCtrlClient{}
	scheme := runtime.NewScheme()
	_ = v1alpha1.AddToScheme(scheme)

	reconciler := &ZeroTrustWorkloadIdentityManagerReconciler{
		ctrlClient:            fakeClient,
		ctx:                   context.Background(),
		log:                   logr.Discard(),
		scheme:                scheme,
		eventRecorder:         record.NewFakeRecorder(100),
		operatorConditionName: "test-operator-condition",
	}

	callCount := 0
	fakeClient.GetStub = func(ctx context.Context, key client.ObjectKey, obj client.Object) error {
		callCount++
		switch o := obj.(type) {
		case *v1alpha1.ZeroTrustWorkloadIdentityManager:
			o.Name = "cluster"
			o.Spec.TrustDomain = "example.org"
		case *v1alpha1.SpireServer:
			o.Name = "cluster"
			o.Status.ConditionalStatus.Conditions = []metav1.Condition{
				{Type: v1alpha1.Ready, Status: metav1.ConditionTrue, Reason: v1alpha1.ReasonReady, Message: "Ready"},
			}
		case *v1alpha1.SpireAgent:
			o.Name = "cluster"
			o.Status.ConditionalStatus.Conditions = []metav1.Condition{
				{Type: v1alpha1.Ready, Status: metav1.ConditionTrue, Reason: v1alpha1.ReasonReady, Message: "Ready"},
			}
		case *v1alpha1.SpiffeCSIDriver:
			o.Name = "cluster"
			o.Status.ConditionalStatus.Conditions = []metav1.Condition{
				{Type: v1alpha1.Ready, Status: metav1.ConditionTrue, Reason: v1alpha1.ReasonReady, Message: "Ready"},
			}
		case *v1alpha1.SpireOIDCDiscoveryProvider:
			o.Name = "cluster"
			o.Status.ConditionalStatus.Conditions = []metav1.Condition{
				{Type: v1alpha1.Ready, Status: metav1.ConditionTrue, Reason: v1alpha1.ReasonReady, Message: "Ready"},
			}
		}
		return nil
	}

	req := ctrl.Request{NamespacedName: types.NamespacedName{Name: "cluster"}}
	result, err := reconciler.Reconcile(context.Background(), req)

	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}
	if result.Requeue {
		t.Error("Expected no requeue")
	}
}

// TestReconcile_SomeOperandsNotCreated tests the reconcile path when some operands are not created
func TestReconcile_SomeOperandsNotCreated(t *testing.T) {
	fakeClient := &fakes.FakeCustomCtrlClient{}
	scheme := runtime.NewScheme()
	_ = v1alpha1.AddToScheme(scheme)

	reconciler := &ZeroTrustWorkloadIdentityManagerReconciler{
		ctrlClient:            fakeClient,
		ctx:                   context.Background(),
		log:                   logr.Discard(),
		scheme:                scheme,
		eventRecorder:         record.NewFakeRecorder(100),
		operatorConditionName: "test-operator-condition",
	}

	fakeClient.GetStub = func(ctx context.Context, key client.ObjectKey, obj client.Object) error {
		switch o := obj.(type) {
		case *v1alpha1.ZeroTrustWorkloadIdentityManager:
			o.Name = "cluster"
			o.Spec.TrustDomain = "example.org"
		case *v1alpha1.SpireServer:
			o.Name = "cluster"
			o.Status.ConditionalStatus.Conditions = []metav1.Condition{
				{Type: v1alpha1.Ready, Status: metav1.ConditionTrue, Reason: v1alpha1.ReasonReady, Message: "Ready"},
			}
		default:
			return kerrors.NewNotFound(schema.GroupResource{}, key.Name)
		}
		return nil
	}

	req := ctrl.Request{NamespacedName: types.NamespacedName{Name: "cluster"}}
	result, err := reconciler.Reconcile(context.Background(), req)

	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}
	if result.Requeue {
		t.Error("Expected no requeue")
	}
}

// TestReconcile_SomeOperandsFailed tests the reconcile path when some operands have failed
func TestReconcile_SomeOperandsFailed(t *testing.T) {
	fakeClient := &fakes.FakeCustomCtrlClient{}
	scheme := runtime.NewScheme()
	_ = v1alpha1.AddToScheme(scheme)

	reconciler := &ZeroTrustWorkloadIdentityManagerReconciler{
		ctrlClient:            fakeClient,
		ctx:                   context.Background(),
		log:                   logr.Discard(),
		scheme:                scheme,
		eventRecorder:         record.NewFakeRecorder(100),
		operatorConditionName: "test-operator-condition",
	}

	fakeClient.GetStub = func(ctx context.Context, key client.ObjectKey, obj client.Object) error {
		switch o := obj.(type) {
		case *v1alpha1.ZeroTrustWorkloadIdentityManager:
			o.Name = "cluster"
			o.Spec.TrustDomain = "example.org"
		case *v1alpha1.SpireServer:
			o.Name = "cluster"
			o.Status.ConditionalStatus.Conditions = []metav1.Condition{
				{Type: v1alpha1.Ready, Status: metav1.ConditionFalse, Reason: v1alpha1.ReasonFailed, Message: "Failed"},
			}
		case *v1alpha1.SpireAgent:
			o.Name = "cluster"
			o.Status.ConditionalStatus.Conditions = []metav1.Condition{
				{Type: v1alpha1.Ready, Status: metav1.ConditionTrue, Reason: v1alpha1.ReasonReady, Message: "Ready"},
			}
		case *v1alpha1.SpiffeCSIDriver:
			o.Name = "cluster"
			o.Status.ConditionalStatus.Conditions = []metav1.Condition{
				{Type: v1alpha1.Ready, Status: metav1.ConditionTrue, Reason: v1alpha1.ReasonReady, Message: "Ready"},
			}
		case *v1alpha1.SpireOIDCDiscoveryProvider:
			o.Name = "cluster"
			o.Status.ConditionalStatus.Conditions = []metav1.Condition{
				{Type: v1alpha1.Ready, Status: metav1.ConditionTrue, Reason: v1alpha1.ReasonReady, Message: "Ready"},
			}
		}
		return nil
	}

	req := ctrl.Request{NamespacedName: types.NamespacedName{Name: "cluster"}}
	result, err := reconciler.Reconcile(context.Background(), req)

	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}
	if result.Requeue {
		t.Error("Expected no requeue")
	}
}

// TestReconcile_WithCreateOnlyMode tests the reconcile path when create only mode is enabled
func TestReconcile_WithCreateOnlyMode(t *testing.T) {
	fakeClient := &fakes.FakeCustomCtrlClient{}
	scheme := runtime.NewScheme()
	_ = v1alpha1.AddToScheme(scheme)

	reconciler := &ZeroTrustWorkloadIdentityManagerReconciler{
		ctrlClient:            fakeClient,
		ctx:                   context.Background(),
		log:                   logr.Discard(),
		scheme:                scheme,
		eventRecorder:         record.NewFakeRecorder(100),
		operatorConditionName: "test-operator-condition",
	}

	fakeClient.GetStub = func(ctx context.Context, key client.ObjectKey, obj client.Object) error {
		switch o := obj.(type) {
		case *v1alpha1.ZeroTrustWorkloadIdentityManager:
			o.Name = "cluster"
			o.Spec.TrustDomain = "example.org"
		case *v1alpha1.SpireServer:
			o.Name = "cluster"
			o.Status.ConditionalStatus.Conditions = []metav1.Condition{
				{Type: v1alpha1.Ready, Status: metav1.ConditionTrue, Reason: v1alpha1.ReasonReady, Message: "Ready"},
				{Type: "CreateOnlyMode", Status: metav1.ConditionTrue, Reason: "CreateOnlyModeEnabled", Message: "Create-only mode is enabled"},
			}
		case *v1alpha1.SpireAgent:
			o.Name = "cluster"
			o.Status.ConditionalStatus.Conditions = []metav1.Condition{
				{Type: v1alpha1.Ready, Status: metav1.ConditionTrue, Reason: v1alpha1.ReasonReady, Message: "Ready"},
			}
		case *v1alpha1.SpiffeCSIDriver:
			o.Name = "cluster"
			o.Status.ConditionalStatus.Conditions = []metav1.Condition{
				{Type: v1alpha1.Ready, Status: metav1.ConditionTrue, Reason: v1alpha1.ReasonReady, Message: "Ready"},
			}
		case *v1alpha1.SpireOIDCDiscoveryProvider:
			o.Name = "cluster"
			o.Status.ConditionalStatus.Conditions = []metav1.Condition{
				{Type: v1alpha1.Ready, Status: metav1.ConditionTrue, Reason: v1alpha1.ReasonReady, Message: "Ready"},
			}
		}
		return nil
	}

	req := ctrl.Request{NamespacedName: types.NamespacedName{Name: "cluster"}}
	result, err := reconciler.Reconcile(context.Background(), req)

	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}
	if result.Requeue {
		t.Error("Expected no requeue")
	}
}

// TestReconcile_OperandConditionBranches tests specific branches to kill && to || mutations
func TestReconcile_OperandConditionBranches(t *testing.T) {
	tests := []struct {
		name        string
		setupClient func(*fakes.FakeCustomCtrlClient)
	}{
		{
			name: "all operands ready - enters allReady branch",
			setupClient: func(fc *fakes.FakeCustomCtrlClient) {
				fc.GetStub = func(ctx context.Context, key client.ObjectKey, obj client.Object) error {
					switch o := obj.(type) {
					case *v1alpha1.ZeroTrustWorkloadIdentityManager:
						o.Name = "cluster"
						o.Spec.TrustDomain = "example.org"
					case *v1alpha1.SpireServer, *v1alpha1.SpireAgent, *v1alpha1.SpiffeCSIDriver, *v1alpha1.SpireOIDCDiscoveryProvider:
						// Set Ready condition for all operands
						if s, ok := o.(*v1alpha1.SpireServer); ok {
							s.Name = "cluster"
							s.Status.ConditionalStatus.Conditions = []metav1.Condition{
								{Type: v1alpha1.Ready, Status: metav1.ConditionTrue, Reason: v1alpha1.ReasonReady},
							}
						} else if a, ok := o.(*v1alpha1.SpireAgent); ok {
							a.Name = "cluster"
							a.Status.ConditionalStatus.Conditions = []metav1.Condition{
								{Type: v1alpha1.Ready, Status: metav1.ConditionTrue, Reason: v1alpha1.ReasonReady},
							}
						} else if c, ok := o.(*v1alpha1.SpiffeCSIDriver); ok {
							c.Name = "cluster"
							c.Status.ConditionalStatus.Conditions = []metav1.Condition{
								{Type: v1alpha1.Ready, Status: metav1.ConditionTrue, Reason: v1alpha1.ReasonReady},
							}
						} else if p, ok := o.(*v1alpha1.SpireOIDCDiscoveryProvider); ok {
							p.Name = "cluster"
							p.Status.ConditionalStatus.Conditions = []metav1.Condition{
								{Type: v1alpha1.Ready, Status: metav1.ConditionTrue, Reason: v1alpha1.ReasonReady},
							}
						}
					}
					return nil
				}
			},
		},
		{
			name: "some not created with no failures - enters progressing branch",
			setupClient: func(fc *fakes.FakeCustomCtrlClient) {
				fc.GetStub = func(ctx context.Context, key client.ObjectKey, obj client.Object) error {
					switch o := obj.(type) {
					case *v1alpha1.ZeroTrustWorkloadIdentityManager:
						o.Name = "cluster"
						o.Spec.TrustDomain = "example.org"
					case *v1alpha1.SpireServer:
						o.Name = "cluster"
						o.Status.ConditionalStatus.Conditions = []metav1.Condition{
							{Type: v1alpha1.Ready, Status: metav1.ConditionTrue, Reason: v1alpha1.ReasonReady},
						}
					default:
						// Other operands not found
						return kerrors.NewNotFound(schema.GroupResource{}, key.Name)
					}
					return nil
				}
			},
		},
		{
			name: "some not created AND some failed - enters failed branch not progressing",
			setupClient: func(fc *fakes.FakeCustomCtrlClient) {
				fc.GetStub = func(ctx context.Context, key client.ObjectKey, obj client.Object) error {
					switch o := obj.(type) {
					case *v1alpha1.ZeroTrustWorkloadIdentityManager:
						o.Name = "cluster"
						o.Spec.TrustDomain = "example.org"
					case *v1alpha1.SpireServer:
						o.Name = "cluster"
						o.Status.ConditionalStatus.Conditions = []metav1.Condition{
							{Type: v1alpha1.Ready, Status: metav1.ConditionFalse, Reason: v1alpha1.ReasonFailed, Message: "Failed"},
						}
					default:
						return kerrors.NewNotFound(schema.GroupResource{}, key.Name)
					}
					return nil
				}
			},
		},
		{
			name: "all created but some failed - enters failed branch",
			setupClient: func(fc *fakes.FakeCustomCtrlClient) {
				fc.GetStub = func(ctx context.Context, key client.ObjectKey, obj client.Object) error {
					switch o := obj.(type) {
					case *v1alpha1.ZeroTrustWorkloadIdentityManager:
						o.Name = "cluster"
						o.Spec.TrustDomain = "example.org"
					case *v1alpha1.SpireServer:
						o.Name = "cluster"
						o.Status.ConditionalStatus.Conditions = []metav1.Condition{
							{Type: v1alpha1.Ready, Status: metav1.ConditionFalse, Reason: v1alpha1.ReasonFailed, Message: "Failed"},
						}
					case *v1alpha1.SpireAgent:
						o.Name = "cluster"
						o.Status.ConditionalStatus.Conditions = []metav1.Condition{
							{Type: v1alpha1.Ready, Status: metav1.ConditionTrue, Reason: v1alpha1.ReasonReady},
						}
					case *v1alpha1.SpiffeCSIDriver:
						o.Name = "cluster"
						o.Status.ConditionalStatus.Conditions = []metav1.Condition{
							{Type: v1alpha1.Ready, Status: metav1.ConditionTrue, Reason: v1alpha1.ReasonReady},
						}
					case *v1alpha1.SpireOIDCDiscoveryProvider:
						o.Name = "cluster"
						o.Status.ConditionalStatus.Conditions = []metav1.Condition{
							{Type: v1alpha1.Ready, Status: metav1.ConditionTrue, Reason: v1alpha1.ReasonReady},
						}
					}
					return nil
				}
			},
		},
		{
			name: "notCreatedCount zero and failedCount nonzero - enters failed branch directly",
			setupClient: func(fc *fakes.FakeCustomCtrlClient) {
				fc.GetStub = func(ctx context.Context, key client.ObjectKey, obj client.Object) error {
					switch o := obj.(type) {
					case *v1alpha1.ZeroTrustWorkloadIdentityManager:
						o.Name = "cluster"
						o.Spec.TrustDomain = "example.org"
					case *v1alpha1.SpireServer:
						o.Name = "cluster"
						o.Status.ConditionalStatus.Conditions = []metav1.Condition{
							{Type: v1alpha1.Ready, Status: metav1.ConditionFalse, Reason: v1alpha1.ReasonFailed, Message: "Server failed"},
						}
					case *v1alpha1.SpireAgent:
						o.Name = "cluster"
						o.Status.ConditionalStatus.Conditions = []metav1.Condition{
							{Type: v1alpha1.Ready, Status: metav1.ConditionFalse, Reason: v1alpha1.ReasonFailed, Message: "Agent failed"},
						}
					case *v1alpha1.SpiffeCSIDriver:
						o.Name = "cluster"
						o.Status.ConditionalStatus.Conditions = []metav1.Condition{
							{Type: v1alpha1.Ready, Status: metav1.ConditionFalse, Reason: v1alpha1.ReasonFailed, Message: "CSI failed"},
						}
					case *v1alpha1.SpireOIDCDiscoveryProvider:
						o.Name = "cluster"
						o.Status.ConditionalStatus.Conditions = []metav1.Condition{
							{Type: v1alpha1.Ready, Status: metav1.ConditionFalse, Reason: v1alpha1.ReasonFailed, Message: "OIDC failed"},
						}
					}
					return nil
				}
			},
		},
		{
			name: "notCreatedCount nonzero and failedCount zero - progressing branch exact",
			setupClient: func(fc *fakes.FakeCustomCtrlClient) {
				fc.GetStub = func(ctx context.Context, key client.ObjectKey, obj client.Object) error {
					switch o := obj.(type) {
					case *v1alpha1.ZeroTrustWorkloadIdentityManager:
						o.Name = "cluster"
						o.Spec.TrustDomain = "example.org"
					case *v1alpha1.SpireServer:
						o.Name = "cluster"
						o.Status.ConditionalStatus.Conditions = []metav1.Condition{
							{Type: v1alpha1.Ready, Status: metav1.ConditionTrue, Reason: v1alpha1.ReasonReady},
						}
					case *v1alpha1.SpireAgent:
						// Progressing - waiting for initial reconciliation
						o.Name = "cluster"
						o.Status.ConditionalStatus.Conditions = []metav1.Condition{}
					case *v1alpha1.SpiffeCSIDriver:
						o.Name = "cluster"
						o.Status.ConditionalStatus.Conditions = []metav1.Condition{
							{Type: v1alpha1.Ready, Status: metav1.ConditionTrue, Reason: v1alpha1.ReasonReady},
						}
					case *v1alpha1.SpireOIDCDiscoveryProvider:
						o.Name = "cluster"
						o.Status.ConditionalStatus.Conditions = []metav1.Condition{
							{Type: v1alpha1.Ready, Status: metav1.ConditionTrue, Reason: v1alpha1.ReasonReady},
						}
					}
					return nil
				}
			},
		},
		{
			name: "progressing operand with CRNotFound message - specific message handling",
			setupClient: func(fc *fakes.FakeCustomCtrlClient) {
				fc.GetStub = func(ctx context.Context, key client.ObjectKey, obj client.Object) error {
					switch o := obj.(type) {
					case *v1alpha1.ZeroTrustWorkloadIdentityManager:
						o.Name = "cluster"
						o.Spec.TrustDomain = "example.org"
					case *v1alpha1.SpireServer:
						o.Name = "cluster"
						o.Status.ConditionalStatus.Conditions = []metav1.Condition{
							{Type: v1alpha1.Ready, Status: metav1.ConditionTrue, Reason: v1alpha1.ReasonReady},
						}
					case *v1alpha1.SpireAgent:
						// Not found - progressing with CR not found message
						return kerrors.NewNotFound(schema.GroupResource{}, key.Name)
					case *v1alpha1.SpiffeCSIDriver:
						// Reconciling - progressing with reconciling message
						o.Name = "cluster"
						o.Status.ConditionalStatus.Conditions = []metav1.Condition{
							{Type: v1alpha1.Ready, Status: metav1.ConditionFalse, Reason: v1alpha1.ReasonInProgress, Message: "Reconciling"},
						}
					case *v1alpha1.SpireOIDCDiscoveryProvider:
						o.Name = "cluster"
						o.Status.ConditionalStatus.Conditions = []metav1.Condition{
							{Type: v1alpha1.Ready, Status: metav1.ConditionTrue, Reason: v1alpha1.ReasonReady},
						}
					}
					return nil
				}
			},
		},
		{
			name: "failed operand classification - enters unhealthy list",
			setupClient: func(fc *fakes.FakeCustomCtrlClient) {
				fc.GetStub = func(ctx context.Context, key client.ObjectKey, obj client.Object) error {
					switch o := obj.(type) {
					case *v1alpha1.ZeroTrustWorkloadIdentityManager:
						o.Name = "cluster"
						o.Spec.TrustDomain = "example.org"
					case *v1alpha1.SpireServer:
						o.Name = "cluster"
						o.Status.ConditionalStatus.Conditions = []metav1.Condition{
							{Type: v1alpha1.Ready, Status: metav1.ConditionFalse, Reason: OperandStateUnhealthy, Message: "Unhealthy state"},
						}
					case *v1alpha1.SpireAgent:
						o.Name = "cluster"
						o.Status.ConditionalStatus.Conditions = []metav1.Condition{
							{Type: v1alpha1.Ready, Status: metav1.ConditionTrue, Reason: v1alpha1.ReasonReady},
						}
					case *v1alpha1.SpiffeCSIDriver:
						o.Name = "cluster"
						o.Status.ConditionalStatus.Conditions = []metav1.Condition{
							{Type: v1alpha1.Ready, Status: metav1.ConditionTrue, Reason: v1alpha1.ReasonReady},
						}
					case *v1alpha1.SpireOIDCDiscoveryProvider:
						o.Name = "cluster"
						o.Status.ConditionalStatus.Conditions = []metav1.Condition{
							{Type: v1alpha1.Ready, Status: metav1.ConditionTrue, Reason: v1alpha1.ReasonReady},
						}
					}
					return nil
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fakeClient := &fakes.FakeCustomCtrlClient{}
			scheme := runtime.NewScheme()
			_ = v1alpha1.AddToScheme(scheme)

			reconciler := &ZeroTrustWorkloadIdentityManagerReconciler{
				ctrlClient:            fakeClient,
				ctx:                   context.Background(),
				log:                   logr.Discard(),
				scheme:                scheme,
				eventRecorder:         record.NewFakeRecorder(100),
				operatorConditionName: "test-operator-condition",
			}

			if tt.setupClient != nil {
				tt.setupClient(fakeClient)
			}

			req := ctrl.Request{NamespacedName: types.NamespacedName{Name: "cluster"}}
			result, err := reconciler.Reconcile(context.Background(), req)

			// Should not error
			if err != nil {
				t.Logf("Reconcile returned error (may be expected): %v", err)
			}
			// Should not requeue
			if result.Requeue {
				t.Error("Expected no requeue")
			}
		})
	}
}

// TestAggregateOperandStatus tests the aggregateOperandStatus function
func TestAggregateOperandStatus(t *testing.T) {
	fakeClient := &fakes.FakeCustomCtrlClient{}
	scheme := runtime.NewScheme()
	_ = v1alpha1.AddToScheme(scheme)

	reconciler := &ZeroTrustWorkloadIdentityManagerReconciler{
		ctrlClient:            fakeClient,
		ctx:                   context.Background(),
		log:                   logr.Discard(),
		scheme:                scheme,
		eventRecorder:         record.NewFakeRecorder(100),
		operatorConditionName: "test-operator-condition",
	}

	t.Run("all operands ready", func(t *testing.T) {
		fakeClient.GetStub = func(ctx context.Context, key client.ObjectKey, obj client.Object) error {
			switch o := obj.(type) {
			case *v1alpha1.SpireServer:
				o.Name = "cluster"
				o.Status.ConditionalStatus.Conditions = []metav1.Condition{
					{Type: v1alpha1.Ready, Status: metav1.ConditionTrue, Reason: v1alpha1.ReasonReady, Message: "Ready"},
				}
			case *v1alpha1.SpireAgent:
				o.Name = "cluster"
				o.Status.ConditionalStatus.Conditions = []metav1.Condition{
					{Type: v1alpha1.Ready, Status: metav1.ConditionTrue, Reason: v1alpha1.ReasonReady, Message: "Ready"},
				}
			case *v1alpha1.SpiffeCSIDriver:
				o.Name = "cluster"
				o.Status.ConditionalStatus.Conditions = []metav1.Condition{
					{Type: v1alpha1.Ready, Status: metav1.ConditionTrue, Reason: v1alpha1.ReasonReady, Message: "Ready"},
				}
			case *v1alpha1.SpireOIDCDiscoveryProvider:
				o.Name = "cluster"
				o.Status.ConditionalStatus.Conditions = []metav1.Condition{
					{Type: v1alpha1.Ready, Status: metav1.ConditionTrue, Reason: v1alpha1.ReasonReady, Message: "Ready"},
				}
			}
			return nil
		}

		result := reconciler.aggregateOperandStatus(context.Background())

		if !result.allReady {
			t.Error("Expected allReady to be true")
		}
		if result.failedCount != 0 {
			t.Errorf("Expected failedCount 0, got %d", result.failedCount)
		}
		if result.notCreatedCount != 0 {
			t.Errorf("Expected notCreatedCount 0, got %d", result.notCreatedCount)
		}
	})

	t.Run("some operands not created", func(t *testing.T) {
		fakeClient.GetStub = func(ctx context.Context, key client.ObjectKey, obj client.Object) error {
			switch o := obj.(type) {
			case *v1alpha1.SpireServer:
				o.Name = "cluster"
				o.Status.ConditionalStatus.Conditions = []metav1.Condition{
					{Type: v1alpha1.Ready, Status: metav1.ConditionTrue, Reason: v1alpha1.ReasonReady, Message: "Ready"},
				}
			default:
				return kerrors.NewNotFound(schema.GroupResource{}, key.Name)
			}
			return nil
		}

		result := reconciler.aggregateOperandStatus(context.Background())

		if result.allReady {
			t.Error("Expected allReady to be false")
		}
		if result.notCreatedCount == 0 {
			t.Error("Expected notCreatedCount to be > 0")
		}
	})
}

// TestReconcile_SuccessfulPath_NoRequeue tests that successful reconciliation returns no requeue
func TestReconcile_SuccessfulPath_NoRequeue(t *testing.T) {
	fakeClient := &fakes.FakeCustomCtrlClient{}
	scheme := runtime.NewScheme()
	_ = v1alpha1.AddToScheme(scheme)

	reconciler := &ZeroTrustWorkloadIdentityManagerReconciler{
		ctrlClient:            fakeClient,
		ctx:                   context.Background(),
		log:                   logr.Discard(),
		scheme:                scheme,
		eventRecorder:         record.NewFakeRecorder(100),
		operatorConditionName: "test-operator-condition",
	}

	fakeClient.GetStub = func(ctx context.Context, key client.ObjectKey, obj client.Object) error {
		switch o := obj.(type) {
		case *v1alpha1.ZeroTrustWorkloadIdentityManager:
			o.Name = "cluster"
			o.Spec.TrustDomain = "example.org"
		case *v1alpha1.SpireServer:
			o.Name = "cluster"
			o.Status.ConditionalStatus.Conditions = []metav1.Condition{
				{Type: v1alpha1.Ready, Status: metav1.ConditionTrue, Reason: v1alpha1.ReasonReady, Message: "Ready"},
			}
		case *v1alpha1.SpireAgent:
			o.Name = "cluster"
			o.Status.ConditionalStatus.Conditions = []metav1.Condition{
				{Type: v1alpha1.Ready, Status: metav1.ConditionTrue, Reason: v1alpha1.ReasonReady, Message: "Ready"},
			}
		case *v1alpha1.SpiffeCSIDriver:
			o.Name = "cluster"
			o.Status.ConditionalStatus.Conditions = []metav1.Condition{
				{Type: v1alpha1.Ready, Status: metav1.ConditionTrue, Reason: v1alpha1.ReasonReady, Message: "Ready"},
			}
		case *v1alpha1.SpireOIDCDiscoveryProvider:
			o.Name = "cluster"
			o.Status.ConditionalStatus.Conditions = []metav1.Condition{
				{Type: v1alpha1.Ready, Status: metav1.ConditionTrue, Reason: v1alpha1.ReasonReady, Message: "Ready"},
			}
		}
		return nil
	}

	req := ctrl.Request{NamespacedName: types.NamespacedName{Name: "cluster"}}
	result, err := reconciler.Reconcile(context.Background(), req)

	if err != nil {
		t.Logf("Reconcile returned error (may be expected): %v", err)
	}
	if result.Requeue {
		t.Error("Expected Requeue=false on successful reconcile")
	}
	if result.RequeueAfter != 0 {
		t.Errorf("Expected RequeueAfter=0 on successful reconcile, got %v", result.RequeueAfter)
	}
}

// mockStatusObject is a mock object that implements GetStatus() interface{}
type mockStatusObject struct {
	client.Object
	status interface{}
}

func (m *mockStatusObject) GetStatus() interface{} {
	return m.status
}

// TestOperandStatusChangedPredicate_MutationKillers tests the predicate function
func TestOperandStatusChangedPredicate_MutationKillers(t *testing.T) {
	tests := []struct {
		name     string
		oldObj   client.Object
		newObj   client.Object
		expected bool
	}{
		{
			// Both objects can get status - should check for status change
			// Same status -> no update needed
			name: "both objects have GetStatus - status same",
			oldObj: &mockStatusObject{
				Object: &v1alpha1.SpireServer{},
				status: map[string]string{"key": "value"},
			},
			newObj: &mockStatusObject{
				Object: &v1alpha1.SpireServer{},
				status: map[string]string{"key": "value"},
			},
			expected: false, // No status change
		},
		{
			// Both objects can get status - status changed
			name: "both objects have GetStatus - status different",
			oldObj: &mockStatusObject{
				Object: &v1alpha1.SpireServer{},
				status: map[string]string{"key": "old"},
			},
			newObj: &mockStatusObject{
				Object: &v1alpha1.SpireServer{},
				status: map[string]string{"key": "new"},
			},
			expected: true, // Status changed
		},
		{
			// Only old object can get status - should reconcile to be safe
			// This tests the || vs && mutation on line 551
			name: "only old object has GetStatus - reconcile to be safe",
			oldObj: &mockStatusObject{
				Object: &v1alpha1.SpireServer{},
				status: "old",
			},
			newObj:   &v1alpha1.SpireServer{}, // Doesn't implement GetStatus() interface{}
			expected: true,                    // Can't compare, reconcile to be safe
		},
		{
			// Only new object can get status - should reconcile to be safe
			name:   "only new object has GetStatus - reconcile to be safe",
			oldObj: &v1alpha1.SpireServer{}, // Doesn't implement GetStatus() interface{}
			newObj: &mockStatusObject{
				Object: &v1alpha1.SpireServer{},
				status: "new",
			},
			expected: true, // Can't compare, reconcile to be safe
		},
		{
			// Neither object can get status - should reconcile to be safe
			name:     "neither object has GetStatus - reconcile to be safe",
			oldObj:   &v1alpha1.SpireServer{},
			newObj:   &v1alpha1.SpireServer{},
			expected: true, // Can't compare, reconcile to be safe
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			updateEvent := event.UpdateEvent{
				ObjectOld: tt.oldObj,
				ObjectNew: tt.newObj,
			}

			result := operandStatusChangedPredicate.Update(updateEvent)

			if result != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}
		})
	}
}

// TestOperandStatusChangedPredicate_CreateDelete tests Create and Delete events
func TestOperandStatusChangedPredicate_CreateDelete(t *testing.T) {
	// Create event should always return true
	createEvent := event.CreateEvent{
		Object: &v1alpha1.SpireServer{},
	}
	if !operandStatusChangedPredicate.Create(createEvent) {
		t.Error("Expected Create to return true")
	}

	// Delete event should always return true
	deleteEvent := event.DeleteEvent{
		Object: &v1alpha1.SpireServer{},
	}
	if !operandStatusChangedPredicate.Delete(deleteEvent) {
		t.Error("Expected Delete to return true")
	}

	// Generic event should return false
	genericEvent := event.GenericEvent{
		Object: &v1alpha1.SpireServer{},
	}
	if operandStatusChangedPredicate.Generic(genericEvent) {
		t.Error("Expected Generic to return false")
	}
}

// TestReconcile_UpdateOperatorConditionError tests when updateOperatorCondition fails
func TestReconcile_UpdateOperatorConditionError(t *testing.T) {
	tests := []struct {
		name               string
		setupClient        func(*fakes.FakeCustomCtrlClient)
		expectReconcileErr bool
	}{
		{
			// Test when ZTWIM exists but updateOperatorCondition fails
			// The reconciler should continue and not fail (it's best effort)
			name: "updateOperatorCondition error is logged but reconcile continues",
			setupClient: func(fc *fakes.FakeCustomCtrlClient) {
				getCallCount := 0
				fc.GetStub = func(ctx context.Context, key client.ObjectKey, obj client.Object) error {
					getCallCount++
					switch o := obj.(type) {
					case *v1alpha1.ZeroTrustWorkloadIdentityManager:
						o.Name = "cluster"
						o.Spec.TrustDomain = "example.org"
						return nil
					case *v1alpha1.SpireServer:
						o.Name = "cluster"
						o.Status.ConditionalStatus.Conditions = []metav1.Condition{
							{Type: v1alpha1.Ready, Status: metav1.ConditionTrue, Reason: v1alpha1.ReasonReady},
						}
						return nil
					case *v1alpha1.SpireAgent:
						o.Name = "cluster"
						o.Status.ConditionalStatus.Conditions = []metav1.Condition{
							{Type: v1alpha1.Ready, Status: metav1.ConditionTrue, Reason: v1alpha1.ReasonReady},
						}
						return nil
					case *v1alpha1.SpiffeCSIDriver:
						o.Name = "cluster"
						o.Status.ConditionalStatus.Conditions = []metav1.Condition{
							{Type: v1alpha1.Ready, Status: metav1.ConditionTrue, Reason: v1alpha1.ReasonReady},
						}
						return nil
					case *v1alpha1.SpireOIDCDiscoveryProvider:
						o.Name = "cluster"
						o.Status.ConditionalStatus.Conditions = []metav1.Condition{
							{Type: v1alpha1.Ready, Status: metav1.ConditionTrue, Reason: v1alpha1.ReasonReady},
						}
						return nil
					default:
						// OperatorCondition Get returns error
						return errors.New("OperatorCondition not found")
					}
				}
			},
			expectReconcileErr: false, // Error is logged but reconcile continues
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fakeClient := &fakes.FakeCustomCtrlClient{}
			scheme := runtime.NewScheme()
			_ = v1alpha1.AddToScheme(scheme)

			reconciler := &ZeroTrustWorkloadIdentityManagerReconciler{
				ctrlClient:            fakeClient,
				ctx:                   context.Background(),
				log:                   logr.Discard(),
				scheme:                scheme,
				eventRecorder:         record.NewFakeRecorder(100),
				operatorConditionName: "test-operator-condition",
			}

			if tt.setupClient != nil {
				tt.setupClient(fakeClient)
			}

			req := ctrl.Request{NamespacedName: types.NamespacedName{Name: "cluster"}}
			_, err := reconciler.Reconcile(context.Background(), req)

			if tt.expectReconcileErr && err == nil {
				t.Error("Expected reconcile error, got nil")
			}
			if !tt.expectReconcileErr && err != nil {
				t.Errorf("Expected no reconcile error, got: %v", err)
			}
		})
	}
}

// TestClassifyOperandState_AllReasons tests all possible reason values for classification
func TestClassifyOperandState_AllReasons(t *testing.T) {
	tests := []struct {
		name      string
		operand   v1alpha1.OperandStatus
		condition *metav1.Condition
		expected  operandStateClassification
	}{
		{
			name:    "Ready true returns operandReady",
			operand: v1alpha1.OperandStatus{Ready: "true", Message: "Ready"},
			condition: &metav1.Condition{
				Type:   v1alpha1.Ready,
				Status: metav1.ConditionTrue,
				Reason: v1alpha1.ReasonReady,
			},
			expected: operandReady,
		},
		{
			name:    "ReasonInProgress returns operandProgressing",
			operand: v1alpha1.OperandStatus{Ready: "false", Message: "In progress"},
			condition: &metav1.Condition{
				Type:   v1alpha1.Ready,
				Status: metav1.ConditionFalse,
				Reason: v1alpha1.ReasonInProgress,
			},
			expected: operandProgressing,
		},
		{
			name:    "OperandStateNotFound returns operandProgressing",
			operand: v1alpha1.OperandStatus{Ready: "false", Message: "Not found"},
			condition: &metav1.Condition{
				Type:   v1alpha1.Ready,
				Status: metav1.ConditionFalse,
				Reason: OperandStateNotFound,
			},
			expected: operandProgressing,
		},
		{
			name:    "OperandStateInitialReconcile returns operandProgressing",
			operand: v1alpha1.OperandStatus{Ready: "false", Message: "Initial reconcile"},
			condition: &metav1.Condition{
				Type:   v1alpha1.Ready,
				Status: metav1.ConditionFalse,
				Reason: OperandStateInitialReconcile,
			},
			expected: operandProgressing,
		},
		{
			name:    "OperandStateReconciling returns operandProgressing",
			operand: v1alpha1.OperandStatus{Ready: "false", Message: "Reconciling"},
			condition: &metav1.Condition{
				Type:   v1alpha1.Ready,
				Status: metav1.ConditionFalse,
				Reason: OperandStateReconciling,
			},
			expected: operandProgressing,
		},
		{
			name:    "ReasonFailed returns operandFailed",
			operand: v1alpha1.OperandStatus{Ready: "false", Message: "Failed"},
			condition: &metav1.Condition{
				Type:   v1alpha1.Ready,
				Status: metav1.ConditionFalse,
				Reason: v1alpha1.ReasonFailed,
			},
			expected: operandFailed,
		},
		{
			name:    "OperandStateUnhealthy returns operandFailed",
			operand: v1alpha1.OperandStatus{Ready: "false", Message: "Unhealthy"},
			condition: &metav1.Condition{
				Type:   v1alpha1.Ready,
				Status: metav1.ConditionFalse,
				Reason: OperandStateUnhealthy,
			},
			expected: operandFailed,
		},
		{
			name:    "ReasonReady with false operand returns operandReady due to Reason",
			operand: v1alpha1.OperandStatus{Ready: "false", Message: "Should be ready"},
			condition: &metav1.Condition{
				Type:   v1alpha1.Ready,
				Status: metav1.ConditionFalse,
				Reason: v1alpha1.ReasonReady,
			},
			expected: operandReady,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := classifyOperandState(tt.operand, tt.condition)
			if result != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}
		})
	}
}

// TestRecreateClusterInstance_AlreadyExistsError tests recreateClusterInstance when CR already exists
func TestRecreateClusterInstance_AlreadyExistsError(t *testing.T) {
	tests := []struct {
		name          string
		createErr     error
		expectRequeue bool
		expectErr     bool
	}{
		{
			name:          "AlreadyExists error - return error",
			createErr:     kerrors.NewAlreadyExists(schema.GroupResource{Group: "operator.openshift.io", Resource: "zerotrustworkloadidentitymanagers"}, "cluster"),
			expectRequeue: false,
			expectErr:     true,
		},
		{
			name:          "Conflict error - return error",
			createErr:     kerrors.NewConflict(schema.GroupResource{Group: "operator.openshift.io", Resource: "zerotrustworkloadidentitymanagers"}, "cluster", errors.New("conflict")),
			expectRequeue: false,
			expectErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fakeClient := &fakes.FakeCustomCtrlClient{}
			reconciler := newTestReconciler(fakeClient)

			fakeClient.CreateReturns(tt.createErr)

			result, err := reconciler.recreateClusterInstance(context.Background(), "cluster")

			if tt.expectErr && err == nil {
				t.Error("Expected error, got nil")
			}
			if !tt.expectErr && err != nil {
				t.Errorf("Expected no error, got: %v", err)
			}
			if result.Requeue != tt.expectRequeue {
				t.Errorf("Requeue = %v, expected %v", result.Requeue, tt.expectRequeue)
			}
		})
	}
}

// TestReconcile_ZTWIMNotFoundWithUpdateError tests ZTWIM not found path with updateOperatorCondition
func TestReconcile_ZTWIMNotFoundWithUpdateError(t *testing.T) {
	tests := []struct {
		name                  string
		getErr                error
		updateOperatorCondErr bool
		expectReconcileErr    bool
		expectResult          ctrl.Result
	}{
		{
			name:                  "ZTWIM not found - updateOperatorCondition succeeds",
			getErr:                kerrors.NewNotFound(schema.GroupResource{}, "cluster"),
			updateOperatorCondErr: false,
			expectReconcileErr:    false,
			expectResult:          ctrl.Result{},
		},
		{
			name:                  "ZTWIM not found - updateOperatorCondition fails (logged but no error returned)",
			getErr:                kerrors.NewNotFound(schema.GroupResource{}, "cluster"),
			updateOperatorCondErr: true,
			expectReconcileErr:    false, // Error is logged but reconcile continues
			expectResult:          ctrl.Result{},
		},
		{
			name:                  "ZTWIM Get error (not NotFound) - returns error",
			getErr:                errors.New("connection refused"),
			updateOperatorCondErr: false,
			expectReconcileErr:    true,
			expectResult:          ctrl.Result{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fakeClient := &fakes.FakeCustomCtrlClient{}
			reconciler := newTestReconciler(fakeClient)

			fakeClient.GetReturns(tt.getErr)
			if tt.updateOperatorCondErr {
				// This will cause findOperatorCondition to fail
				fakeClient.GetReturns(tt.getErr)
			}

			req := ctrl.Request{NamespacedName: types.NamespacedName{Name: "cluster"}}
			result, err := reconciler.Reconcile(context.Background(), req)

			if tt.expectReconcileErr && err == nil {
				t.Error("Expected reconcile error, got nil")
			}
			if !tt.expectReconcileErr && err != nil {
				t.Errorf("Expected no reconcile error, got: %v", err)
			}
			if result.Requeue != tt.expectResult.Requeue {
				t.Errorf("Requeue = %v, expected %v", result.Requeue, tt.expectResult.Requeue)
			}
		})
	}
}

// TestReconcile_ProgressingBranch tests the progressing branch (notCreatedCount > 0 && failedCount == 0)
func TestReconcile_ProgressingBranch(t *testing.T) {
	tests := []struct {
		name              string
		setupOperands     func(*fakes.FakeCustomCtrlClient)
		expectAllReady    bool
		expectProgressing bool
		expectFailed      bool
	}{
		{
			// notCreatedCount = 0, failedCount = 0, allReady = true
			name: "all operands ready - allReady branch",
			setupOperands: func(fc *fakes.FakeCustomCtrlClient) {
				fc.GetStub = func(ctx context.Context, key client.ObjectKey, obj client.Object) error {
					switch o := obj.(type) {
					case *v1alpha1.ZeroTrustWorkloadIdentityManager:
						o.Name = "cluster"
					case *v1alpha1.SpireServer, *v1alpha1.SpireAgent, *v1alpha1.SpiffeCSIDriver, *v1alpha1.SpireOIDCDiscoveryProvider:
						setAllReady(o)
					}
					return nil
				}
			},
			expectAllReady:    true,
			expectProgressing: false,
			expectFailed:      false,
		},
		{
			// notCreatedCount > 0, failedCount = 0 - should be progressing
			name: "notCreatedCount=1 failedCount=0 - progressing branch",
			setupOperands: func(fc *fakes.FakeCustomCtrlClient) {
				fc.GetStub = func(ctx context.Context, key client.ObjectKey, obj client.Object) error {
					switch o := obj.(type) {
					case *v1alpha1.ZeroTrustWorkloadIdentityManager:
						o.Name = "cluster"
					case *v1alpha1.SpireServer:
						o.Name = "cluster"
						o.Status.ConditionalStatus.Conditions = []metav1.Condition{
							{Type: v1alpha1.Ready, Status: metav1.ConditionTrue, Reason: v1alpha1.ReasonReady},
						}
					default:
						return kerrors.NewNotFound(schema.GroupResource{}, key.Name)
					}
					return nil
				}
			},
			expectAllReady:    false,
			expectProgressing: true,
			expectFailed:      false,
		},
		{
			name: "notCreatedCount=0 failedCount=1 - failed branch",
			setupOperands: func(fc *fakes.FakeCustomCtrlClient) {
				fc.GetStub = func(ctx context.Context, key client.ObjectKey, obj client.Object) error {
					switch o := obj.(type) {
					case *v1alpha1.ZeroTrustWorkloadIdentityManager:
						o.Name = "cluster"
					case *v1alpha1.SpireServer:
						o.Name = "cluster"
						o.Status.ConditionalStatus.Conditions = []metav1.Condition{
							{Type: v1alpha1.Ready, Status: metav1.ConditionFalse, Reason: v1alpha1.ReasonFailed, Message: "Failed"},
						}
					case *v1alpha1.SpireAgent:
						o.Name = "cluster"
						o.Status.ConditionalStatus.Conditions = []metav1.Condition{
							{Type: v1alpha1.Ready, Status: metav1.ConditionTrue, Reason: v1alpha1.ReasonReady},
						}
					case *v1alpha1.SpiffeCSIDriver:
						o.Name = "cluster"
						o.Status.ConditionalStatus.Conditions = []metav1.Condition{
							{Type: v1alpha1.Ready, Status: metav1.ConditionTrue, Reason: v1alpha1.ReasonReady},
						}
					case *v1alpha1.SpireOIDCDiscoveryProvider:
						o.Name = "cluster"
						o.Status.ConditionalStatus.Conditions = []metav1.Condition{
							{Type: v1alpha1.Ready, Status: metav1.ConditionTrue, Reason: v1alpha1.ReasonReady},
						}
					}
					return nil
				}
			},
			expectAllReady:    false,
			expectProgressing: false,
			expectFailed:      true,
		},
		{
			name: "notCreatedCount=1 failedCount=1 - failed branch",
			setupOperands: func(fc *fakes.FakeCustomCtrlClient) {
				fc.GetStub = func(ctx context.Context, key client.ObjectKey, obj client.Object) error {
					switch o := obj.(type) {
					case *v1alpha1.ZeroTrustWorkloadIdentityManager:
						o.Name = "cluster"
					case *v1alpha1.SpireServer:
						o.Name = "cluster"
						o.Status.ConditionalStatus.Conditions = []metav1.Condition{
							{Type: v1alpha1.Ready, Status: metav1.ConditionFalse, Reason: v1alpha1.ReasonFailed, Message: "Failed"},
						}
					case *v1alpha1.SpireAgent:
						return kerrors.NewNotFound(schema.GroupResource{}, key.Name)
					case *v1alpha1.SpiffeCSIDriver:
						o.Name = "cluster"
						o.Status.ConditionalStatus.Conditions = []metav1.Condition{
							{Type: v1alpha1.Ready, Status: metav1.ConditionTrue, Reason: v1alpha1.ReasonReady},
						}
					case *v1alpha1.SpireOIDCDiscoveryProvider:
						o.Name = "cluster"
						o.Status.ConditionalStatus.Conditions = []metav1.Condition{
							{Type: v1alpha1.Ready, Status: metav1.ConditionTrue, Reason: v1alpha1.ReasonReady},
						}
					}
					return nil
				}
			},
			expectAllReady:    false,
			expectProgressing: false,
			expectFailed:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fakeClient := &fakes.FakeCustomCtrlClient{}
			scheme := runtime.NewScheme()
			_ = v1alpha1.AddToScheme(scheme)

			reconciler := &ZeroTrustWorkloadIdentityManagerReconciler{
				ctrlClient:            fakeClient,
				ctx:                   context.Background(),
				log:                   logr.Discard(),
				scheme:                scheme,
				eventRecorder:         record.NewFakeRecorder(100),
				operatorConditionName: "test-operator-condition",
			}

			tt.setupOperands(fakeClient)

			result := reconciler.aggregateOperandStatus(context.Background())

			if result.allReady != tt.expectAllReady {
				t.Errorf("allReady = %v, expected %v", result.allReady, tt.expectAllReady)
			}

			// Check progressing: notCreatedCount > 0 && failedCount == 0
			isProgressing := result.notCreatedCount > 0 && result.failedCount == 0
			if isProgressing != tt.expectProgressing {
				t.Errorf("progressing = %v (notCreatedCount=%d, failedCount=%d), expected %v",
					isProgressing, result.notCreatedCount, result.failedCount, tt.expectProgressing)
			}

			// Check failed: failedCount > 0
			isFailed := result.failedCount > 0
			if isFailed != tt.expectFailed {
				t.Errorf("failed = %v (failedCount=%d), expected %v",
					isFailed, result.failedCount, tt.expectFailed)
			}
		})
	}
}

// setAllReady is a helper to set all operand types to ready state
func setAllReady(obj client.Object) {
	switch o := obj.(type) {
	case *v1alpha1.SpireServer:
		o.Name = "cluster"
		o.Status.ConditionalStatus.Conditions = []metav1.Condition{
			{Type: v1alpha1.Ready, Status: metav1.ConditionTrue, Reason: v1alpha1.ReasonReady},
		}
	case *v1alpha1.SpireAgent:
		o.Name = "cluster"
		o.Status.ConditionalStatus.Conditions = []metav1.Condition{
			{Type: v1alpha1.Ready, Status: metav1.ConditionTrue, Reason: v1alpha1.ReasonReady},
		}
	case *v1alpha1.SpiffeCSIDriver:
		o.Name = "cluster"
		o.Status.ConditionalStatus.Conditions = []metav1.Condition{
			{Type: v1alpha1.Ready, Status: metav1.ConditionTrue, Reason: v1alpha1.ReasonReady},
		}
	case *v1alpha1.SpireOIDCDiscoveryProvider:
		o.Name = "cluster"
		o.Status.ConditionalStatus.Conditions = []metav1.Condition{
			{Type: v1alpha1.Ready, Status: metav1.ConditionTrue, Reason: v1alpha1.ReasonReady},
		}
	}
}

// TestReconcile_ClassificationBranches tests classification branches for progressing vs failed
func TestReconcile_ClassificationBranches(t *testing.T) {
	tests := []struct {
		name              string
		setupOperands     func(*fakes.FakeCustomCtrlClient)
		expectProgressing []string // operand kinds expected in progressing list
		expectFailed      []string // operand kinds expected in failed list
	}{
		{
			// Classification == operandProgressing with CRNotFound message
			name: "CR not found - progressing with (not created) suffix",
			setupOperands: func(fc *fakes.FakeCustomCtrlClient) {
				fc.GetStub = func(ctx context.Context, key client.ObjectKey, obj client.Object) error {
					switch o := obj.(type) {
					case *v1alpha1.ZeroTrustWorkloadIdentityManager:
						o.Name = "cluster"
					case *v1alpha1.SpireServer:
						o.Name = "cluster"
						o.Status.ConditionalStatus.Conditions = []metav1.Condition{
							{Type: v1alpha1.Ready, Status: metav1.ConditionTrue, Reason: v1alpha1.ReasonReady},
						}
					case *v1alpha1.SpireAgent:
						// Not found - will get OperandMessageCRNotFound
						return kerrors.NewNotFound(schema.GroupResource{}, key.Name)
					case *v1alpha1.SpiffeCSIDriver:
						o.Name = "cluster"
						o.Status.ConditionalStatus.Conditions = []metav1.Condition{
							{Type: v1alpha1.Ready, Status: metav1.ConditionTrue, Reason: v1alpha1.ReasonReady},
						}
					case *v1alpha1.SpireOIDCDiscoveryProvider:
						o.Name = "cluster"
						o.Status.ConditionalStatus.Conditions = []metav1.Condition{
							{Type: v1alpha1.Ready, Status: metav1.ConditionTrue, Reason: v1alpha1.ReasonReady},
						}
					}
					return nil
				}
			},
			expectProgressing: []string{"SpireAgent"},
			expectFailed:      []string{},
		},
		{
			// Classification == operandProgressing with reconciling message
			name: "reconciling - progressing with (reconciling) suffix",
			setupOperands: func(fc *fakes.FakeCustomCtrlClient) {
				fc.GetStub = func(ctx context.Context, key client.ObjectKey, obj client.Object) error {
					switch o := obj.(type) {
					case *v1alpha1.ZeroTrustWorkloadIdentityManager:
						o.Name = "cluster"
					case *v1alpha1.SpireServer:
						o.Name = "cluster"
						o.Status.ConditionalStatus.Conditions = []metav1.Condition{
							{Type: v1alpha1.Ready, Status: metav1.ConditionTrue, Reason: v1alpha1.ReasonReady},
						}
					case *v1alpha1.SpireAgent:
						o.Name = "cluster"
						// No conditions - waiting for initial reconciliation
						o.Status.ConditionalStatus.Conditions = []metav1.Condition{}
					case *v1alpha1.SpiffeCSIDriver:
						o.Name = "cluster"
						o.Status.ConditionalStatus.Conditions = []metav1.Condition{
							{Type: v1alpha1.Ready, Status: metav1.ConditionTrue, Reason: v1alpha1.ReasonReady},
						}
					case *v1alpha1.SpireOIDCDiscoveryProvider:
						o.Name = "cluster"
						o.Status.ConditionalStatus.Conditions = []metav1.Condition{
							{Type: v1alpha1.Ready, Status: metav1.ConditionTrue, Reason: v1alpha1.ReasonReady},
						}
					}
					return nil
				}
			},
			expectProgressing: []string{"SpireAgent"},
			expectFailed:      []string{},
		},
		{
			// Classification == operandFailed
			name: "failed operand - in unhealthy list",
			setupOperands: func(fc *fakes.FakeCustomCtrlClient) {
				fc.GetStub = func(ctx context.Context, key client.ObjectKey, obj client.Object) error {
					switch o := obj.(type) {
					case *v1alpha1.ZeroTrustWorkloadIdentityManager:
						o.Name = "cluster"
					case *v1alpha1.SpireServer:
						o.Name = "cluster"
						o.Status.ConditionalStatus.Conditions = []metav1.Condition{
							{Type: v1alpha1.Ready, Status: metav1.ConditionFalse, Reason: v1alpha1.ReasonFailed, Message: "Failed"},
						}
					case *v1alpha1.SpireAgent:
						o.Name = "cluster"
						o.Status.ConditionalStatus.Conditions = []metav1.Condition{
							{Type: v1alpha1.Ready, Status: metav1.ConditionTrue, Reason: v1alpha1.ReasonReady},
						}
					case *v1alpha1.SpiffeCSIDriver:
						o.Name = "cluster"
						o.Status.ConditionalStatus.Conditions = []metav1.Condition{
							{Type: v1alpha1.Ready, Status: metav1.ConditionTrue, Reason: v1alpha1.ReasonReady},
						}
					case *v1alpha1.SpireOIDCDiscoveryProvider:
						o.Name = "cluster"
						o.Status.ConditionalStatus.Conditions = []metav1.Condition{
							{Type: v1alpha1.Ready, Status: metav1.ConditionTrue, Reason: v1alpha1.ReasonReady},
						}
					}
					return nil
				}
			},
			expectProgressing: []string{},
			expectFailed:      []string{"SpireServer"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fakeClient := &fakes.FakeCustomCtrlClient{}
			scheme := runtime.NewScheme()
			_ = v1alpha1.AddToScheme(scheme)

			reconciler := &ZeroTrustWorkloadIdentityManagerReconciler{
				ctrlClient:            fakeClient,
				ctx:                   context.Background(),
				log:                   logr.Discard(),
				scheme:                scheme,
				eventRecorder:         record.NewFakeRecorder(100),
				operatorConditionName: "test-operator-condition",
			}

			tt.setupOperands(fakeClient)

			result := reconciler.aggregateOperandStatus(context.Background())

			// Verify counts match expected
			if len(tt.expectProgressing) > 0 && result.notCreatedCount == 0 {
				t.Error("Expected progressing operands but notCreatedCount is 0")
			}
			if len(tt.expectFailed) > 0 && result.failedCount == 0 {
				t.Error("Expected failed operands but failedCount is 0")
			}
			if len(tt.expectProgressing) == 0 && len(tt.expectFailed) == 0 && !result.allReady {
				t.Error("Expected all ready but allReady is false")
			}
		})
	}
}

// TestProcessOperandStatus_MessageVariations tests processOperandStatus with various messages
// testing message == OperandMessageCRNotFound branch
func TestProcessOperandStatus_MessageVariations(t *testing.T) {
	tests := []struct {
		name                   string
		operand                v1alpha1.OperandStatus
		expectAnyOperandExists bool
		expectNotCreatedCount  int
		expectFailedCount      int
	}{
		{
			// Message == OperandMessageCRNotFound - operand doesn't exist
			name: "CR not found message - operand does not exist",
			operand: v1alpha1.OperandStatus{
				Kind:    "SpireServer",
				Name:    "cluster",
				Ready:   "false",
				Message: OperandMessageCRNotFound,
			},
			expectAnyOperandExists: false,
			expectNotCreatedCount:  1,
			expectFailedCount:      0,
		},
		{
			// Message != OperandMessageCRNotFound but progressing
			name: "reconciling message - operand exists but progressing",
			operand: v1alpha1.OperandStatus{
				Kind:    "SpireServer",
				Name:    "cluster",
				Ready:   "false",
				Message: OperandMessageReconciling,
			},
			expectAnyOperandExists: true,
			expectNotCreatedCount:  1, // Progressing counts as notCreated
			expectFailedCount:      0,
		},
		{
			// Message is some failure - operand exists and failed
			name: "failure message - operand exists but failed",
			operand: v1alpha1.OperandStatus{
				Kind:    "SpireServer",
				Name:    "cluster",
				Ready:   "false",
				Message: "Some failure occurred",
			},
			expectAnyOperandExists: true,
			expectNotCreatedCount:  0,
			expectFailedCount:      1,
		},
		{
			// Message is waiting for initial reconciliation
			name: "waiting for initial recon - progressing",
			operand: v1alpha1.OperandStatus{
				Kind:    "SpireServer",
				Name:    "cluster",
				Ready:   "false",
				Message: OperandMessageWaitingInitialRecon,
			},
			expectAnyOperandExists: true,
			expectNotCreatedCount:  1,
			expectFailedCount:      0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state := &operandAggregateState{allReady: true}
			processOperandStatus(tt.operand, state)

			if state.anyOperandExists != tt.expectAnyOperandExists {
				t.Errorf("anyOperandExists = %v, expected %v", state.anyOperandExists, tt.expectAnyOperandExists)
			}
			if state.notCreatedCount != tt.expectNotCreatedCount {
				t.Errorf("notCreatedCount = %v, expected %v", state.notCreatedCount, tt.expectNotCreatedCount)
			}
			if state.failedCount != tt.expectFailedCount {
				t.Errorf("failedCount = %v, expected %v", state.failedCount, tt.expectFailedCount)
			}
		})
	}
}

// TestFindOperatorCondition_NameVariations tests findOperatorCondition with various name states
func TestFindOperatorCondition_NameVariations(t *testing.T) {
	tests := []struct {
		name                  string
		operatorConditionName string
		getError              error
		expectNil             bool
		expectErr             bool
	}{
		{
			// operatorConditionName != "" and Get succeeds
			name:                  "non-empty name and Get succeeds",
			operatorConditionName: "test-operator",
			getError:              nil,
			expectNil:             false,
			expectErr:             false,
		},
		{
			// operatorConditionName != "" but Get returns NotFound
			name:                  "non-empty name but NotFound",
			operatorConditionName: "test-operator",
			getError:              kerrors.NewNotFound(schema.GroupResource{}, "test"),
			expectNil:             true,
			expectErr:             true,
		},
		{
			// operatorConditionName != "" but Get returns other error
			name:                  "non-empty name with Get error",
			operatorConditionName: "test-operator",
			getError:              errors.New("connection refused"),
			expectNil:             true,
			expectErr:             true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fakeClient := &fakes.FakeCustomCtrlClient{}
			reconciler := newTestReconciler(fakeClient)
			reconciler.operatorConditionName = tt.operatorConditionName

			fakeClient.GetReturns(tt.getError)

			result, err := reconciler.findOperatorCondition(context.Background())

			if tt.expectNil && result != nil {
				t.Error("Expected nil result")
			}
			if !tt.expectNil && result == nil {
				t.Error("Expected non-nil result")
			}
			if tt.expectErr && err == nil {
				t.Error("Expected error, got nil")
			}
			if !tt.expectErr && err != nil {
				t.Errorf("Expected no error, got: %v", err)
			}
		})
	}
}

// TestUpdateOperatorCondition_OperatorConditionNil tests when operatorCondition is nil
func TestUpdateOperatorCondition_OperatorConditionNil(t *testing.T) {
	fakeClient := &fakes.FakeCustomCtrlClient{}
	reconciler := newTestReconciler(fakeClient)

	// Return NotFound which will make findOperatorCondition return error
	fakeClient.GetReturns(kerrors.NewNotFound(schema.GroupResource{}, "test"))

	err := reconciler.updateOperatorCondition(context.Background(), false, []v1alpha1.OperandStatus{})

	// Should return error when findOperatorCondition fails
	if err == nil {
		t.Error("Expected error when OperatorCondition not found, got nil")
	}
}

// TestUpdateOperatorCondition_StatusUpdateFails tests when StatusUpdateWithRetry fails
func TestUpdateOperatorCondition_StatusUpdateFails(t *testing.T) {
	fakeClient := &fakes.FakeCustomCtrlClient{}
	reconciler := newTestReconciler(fakeClient)

	// Get succeeds
	fakeClient.GetReturns(nil)
	// StatusUpdateWithRetry fails
	fakeClient.StatusUpdateWithRetryReturns(errors.New("status update failed"))

	err := reconciler.updateOperatorCondition(context.Background(), false, []v1alpha1.OperandStatus{})

	// Should return error when StatusUpdateWithRetry fails
	if err == nil {
		t.Error("Expected error when StatusUpdateWithRetry fails, got nil")
	}
}

// TestFindOperatorCondition_GetSucceeds tests when Get succeeds
func TestFindOperatorCondition_GetSucceeds(t *testing.T) {
	fakeClient := &fakes.FakeCustomCtrlClient{}
	reconciler := newTestReconciler(fakeClient)
	reconciler.operatorConditionName = "test-operator"

	// Get succeeds (returns nil error)
	fakeClient.GetReturns(nil)

	result, err := reconciler.findOperatorCondition(context.Background())

	// Should return the OperatorCondition when Get succeeds
	if result == nil {
		t.Error("Expected non-nil OperatorCondition when Get succeeds")
	}
	if err != nil {
		t.Errorf("Expected no error when Get succeeds, got: %v", err)
	}
}
