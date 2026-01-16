package utils

import (
	"errors"
	"testing"

	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func TestErrorCreation(t *testing.T) {
	tests := []struct {
		name           string
		createFunc     func() *ReconcileError
		expectNil      bool
		expectedReason ErrorReason
		expectedMsg    string
	}{
		{
			name:           "NewIrrecoverableError with error",
			createFunc:     func() *ReconcileError { return NewIrrecoverableError(errors.New("base"), "test error") },
			expectedReason: IrrecoverableError,
			expectedMsg:    "test error",
		},
		{
			name:       "NewIrrecoverableError with nil",
			createFunc: func() *ReconcileError { return NewIrrecoverableError(nil, "test") },
			expectNil:  true,
		},
		{
			name:           "NewIrrecoverableError with format args",
			createFunc:     func() *ReconcileError { return NewIrrecoverableError(errors.New("base"), "error: %s", "details") },
			expectedReason: IrrecoverableError,
			expectedMsg:    "error: details",
		},
		{
			name:           "NewMultipleInstanceError with error",
			createFunc:     func() *ReconcileError { return NewMultipleInstanceError(errors.New("multiple")) },
			expectedReason: MultipleInstanceError,
		},
		{
			name:       "NewMultipleInstanceError with nil",
			createFunc: func() *ReconcileError { return NewMultipleInstanceError(nil) },
			expectNil:  true,
		},
		{
			name:           "NewRetryRequiredError with error",
			createFunc:     func() *ReconcileError { return NewRetryRequiredError(errors.New("transient"), "retry") },
			expectedReason: RetryRequiredError,
			expectedMsg:    "retry",
		},
		{
			name:       "NewRetryRequiredError with nil",
			createFunc: func() *ReconcileError { return NewRetryRequiredError(nil, "retry") },
			expectNil:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.createFunc()
			if tt.expectNil {
				if err != nil {
					t.Error("Expected nil error")
				}
				return
			}
			if err == nil {
				t.Fatal("Expected non-nil error")
			}
			if err.Reason != tt.expectedReason {
				t.Errorf("Expected reason %s, got %s", tt.expectedReason, err.Reason)
			}
			if tt.expectedMsg != "" && err.Message != tt.expectedMsg {
				t.Errorf("Expected message '%s', got '%s'", tt.expectedMsg, err.Message)
			}
		})
	}
}

func TestFromClientError(t *testing.T) {
	tests := []struct {
		name           string
		err            error
		expectedReason ErrorReason
		expectNil      bool
	}{
		{name: "nil error", err: nil, expectNil: true},
		{name: "not found is retryable", err: kerrors.NewNotFound(schema.GroupResource{}, "test"), expectedReason: RetryRequiredError},
		{name: "conflict is retryable", err: kerrors.NewConflict(schema.GroupResource{}, "test", errors.New("c")), expectedReason: RetryRequiredError},
		{name: "unauthorized is irrecoverable", err: kerrors.NewUnauthorized("no"), expectedReason: IrrecoverableError},
		{name: "forbidden is irrecoverable", err: kerrors.NewForbidden(schema.GroupResource{}, "t", errors.New("f")), expectedReason: IrrecoverableError},
		{name: "bad request is irrecoverable", err: kerrors.NewBadRequest("bad"), expectedReason: IrrecoverableError},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FromClientError(tt.err, "context")
			if tt.expectNil {
				if result != nil {
					t.Error("Expected nil")
				}
				return
			}
			if result == nil {
				t.Fatal("Expected non-nil")
			}
			if result.Reason != tt.expectedReason {
				t.Errorf("Expected %s, got %s", tt.expectedReason, result.Reason)
			}
		})
	}
}

func TestFromError(t *testing.T) {
	tests := []struct {
		name           string
		err            error
		expectedReason ErrorReason
		expectNil      bool
	}{
		{name: "nil error", err: nil, expectNil: true},
		{name: "regular error is retryable", err: errors.New("test"), expectedReason: RetryRequiredError},
		{name: "irrecoverable stays irrecoverable", err: NewIrrecoverableError(errors.New("b"), "i"), expectedReason: IrrecoverableError},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FromError(tt.err, "context")
			if tt.expectNil {
				if result != nil {
					t.Error("Expected nil")
				}
				return
			}
			if result == nil {
				t.Fatal("Expected non-nil")
			}
			if result.Reason != tt.expectedReason {
				t.Errorf("Expected %s, got %s", tt.expectedReason, result.Reason)
			}
		})
	}
}

func TestErrorTypeChecks(t *testing.T) {
	irrecoverableErr := NewIrrecoverableError(errors.New("b"), "i")
	retryErr := NewRetryRequiredError(errors.New("b"), "r")
	multiErr := NewMultipleInstanceError(errors.New("m"))
	regularErr := errors.New("regular")

	tests := []struct {
		name     string
		err      error
		checkFn  func(error) bool
		expected bool
	}{
		{name: "IsIrrecoverableError true", err: irrecoverableErr, checkFn: IsIrrecoverableError, expected: true},
		{name: "IsIrrecoverableError false for regular", err: regularErr, checkFn: IsIrrecoverableError, expected: false},
		{name: "IsIrrecoverableError false for nil", err: nil, checkFn: IsIrrecoverableError, expected: false},
		{name: "IsRetryRequiredError true", err: retryErr, checkFn: IsRetryRequiredError, expected: true},
		{name: "IsRetryRequiredError false for regular", err: regularErr, checkFn: IsRetryRequiredError, expected: false},
		{name: "IsRetryRequiredError false for nil", err: nil, checkFn: IsRetryRequiredError, expected: false},
		{name: "IsMultipleInstanceError true", err: multiErr, checkFn: IsMultipleInstanceError, expected: true},
		{name: "IsMultipleInstanceError false for regular", err: regularErr, checkFn: IsMultipleInstanceError, expected: false},
		{name: "IsMultipleInstanceError false for nil", err: nil, checkFn: IsMultipleInstanceError, expected: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.checkFn(tt.err); got != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, got)
			}
		})
	}
}

func TestReconcileErrorError(t *testing.T) {
	err := NewIrrecoverableError(errors.New("base error"), "test message")
	expected := "test message: base error"
	if err.Error() != expected {
		t.Errorf("Expected '%s', got '%s'", expected, err.Error())
	}
}
