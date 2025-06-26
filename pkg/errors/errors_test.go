package errors

import (
	"context"
	"errors"
	"testing"
)

func TestServiceError_Error(t *testing.T) {
	tests := []struct {
		name     string
		err      *ServiceError
		expected string
	}{
		{
			name: "error with cause",
			err: &ServiceError{
				Type:      ErrorTypeNetwork,
				Operation: "test_operation",
				Message:   "test message",
				Cause:     errors.New("underlying error"),
			},
			expected: "network operation 'test_operation' failed: test message (caused by: underlying error)",
		},
		{
			name: "error without cause",
			err: &ServiceError{
				Type:      ErrorTypeValidation,
				Operation: "validation_check",
				Message:   "invalid input",
				Cause:     nil,
			},
			expected: "validation operation 'validation_check' failed: invalid input",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.err.Error(); got != tt.expected {
				t.Errorf("ServiceError.Error() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestServiceError_Unwrap(t *testing.T) {
	cause := errors.New("underlying error")
	err := &ServiceError{
		Type:      ErrorTypeNetwork,
		Operation: "test",
		Message:   "test",
		Cause:     cause,
	}

	if unwrapped := err.Unwrap(); unwrapped != cause {
		t.Errorf("ServiceError.Unwrap() = %v, want %v", unwrapped, cause)
	}

	errNoCause := &ServiceError{
		Type:      ErrorTypeNetwork,
		Operation: "test",
		Message:   "test",
		Cause:     nil,
	}

	if unwrapped := errNoCause.Unwrap(); unwrapped != nil {
		t.Errorf("ServiceError.Unwrap() = %v, want nil", unwrapped)
	}
}

func TestServiceError_WithContext(t *testing.T) {
	err := &ServiceError{
		Type:      ErrorTypeDatabase,
		Operation: "test",
		Message:   "test",
	}

	err = err.WithContext("key1", "value1").WithContext("key2", 42)

	if len(err.Context) != 2 {
		t.Errorf("Expected 2 context items, got %d", len(err.Context))
	}

	if err.Context["key1"] != "value1" {
		t.Errorf("Expected key1 = 'value1', got %v", err.Context["key1"])
	}

	if err.Context["key2"] != 42 {
		t.Errorf("Expected key2 = 42, got %v", err.Context["key2"])
	}
}

func TestNew(t *testing.T) {
	err := New(ErrorTypeValidation, "test_operation", "test message")

	if err.Type != ErrorTypeValidation {
		t.Errorf("Expected type %v, got %v", ErrorTypeValidation, err.Type)
	}

	if err.Operation != "test_operation" {
		t.Errorf("Expected operation 'test_operation', got '%s'", err.Operation)
	}

	if err.Message != "test message" {
		t.Errorf("Expected message 'test message', got '%s'", err.Message)
	}

	if err.Timestamp.IsZero() {
		t.Error("Expected timestamp to be set")
	}

	// Validation errors should not be retryable by default
	if err.Retryable {
		t.Error("Expected validation error to not be retryable")
	}
}

func TestWrap(t *testing.T) {
	cause := errors.New("original error")
	err := Wrap(cause, ErrorTypeNetwork, "test_operation", "wrapped message")

	if err.Type != ErrorTypeNetwork {
		t.Errorf("Expected type %v, got %v", ErrorTypeNetwork, err.Type)
	}

	if err.Cause != cause {
		t.Errorf("Expected cause %v, got %v", cause, err.Cause)
	}

	// Test wrapping nil error
	nilErr := Wrap(nil, ErrorTypeNetwork, "test", "test")
	if nilErr != nil {
		t.Errorf("Expected nil when wrapping nil error, got %v", nilErr)
	}

	// Test wrapping ServiceError
	serviceErr := &ServiceError{Type: ErrorTypeDatabase, Operation: "db_op", Message: "db error"}
	wrappedServiceErr := Wrap(serviceErr, ErrorTypeNetwork, "network_op", "network error")

	if wrappedServiceErr.Cause != serviceErr {
		t.Errorf("Expected wrapped ServiceError as cause")
	}
}

func TestIsType(t *testing.T) {
	err := New(ErrorTypeNetwork, "test", "test")

	if !IsType(err, ErrorTypeNetwork) {
		t.Error("Expected IsType to return true for matching type")
	}

	if IsType(err, ErrorTypeDatabase) {
		t.Error("Expected IsType to return false for non-matching type")
	}

	// Test with regular error
	regularErr := errors.New("regular error")
	if IsType(regularErr, ErrorTypeNetwork) {
		t.Error("Expected IsType to return false for regular error")
	}
}

func TestIsRetryable(t *testing.T) {
	// Test retryable error type
	networkErr := New(ErrorTypeNetwork, "test", "test")
	if !IsRetryable(networkErr) {
		t.Error("Expected network error to be retryable")
	}

	// Test non-retryable error type
	validationErr := New(ErrorTypeValidation, "test", "test")
	if IsRetryable(validationErr) {
		t.Error("Expected validation error to not be retryable")
	}

	// Test context cancellation (should not be retryable)
	if IsRetryable(context.Canceled) {
		t.Error("Expected context.Canceled to not be retryable")
	}

	// Test timeout (should not be retryable)
	if IsRetryable(context.DeadlineExceeded) {
		t.Error("Expected context.DeadlineExceeded to not be retryable")
	}

	// Test connection error patterns
	connRefusedErr := errors.New("connection refused")
	if !IsRetryable(connRefusedErr) {
		t.Error("Expected 'connection refused' error to be retryable")
	}

	// Test other errors
	unknownErr := errors.New("unknown error")
	if IsRetryable(unknownErr) {
		t.Error("Expected unknown error to not be retryable")
	}
}

func TestGetContext(t *testing.T) {
	err := New(ErrorTypeDatabase, "test", "test").
		WithContext("key1", "value1").
		WithContext("key2", 42)

	context := GetContext(err)
	if len(context) != 2 {
		t.Errorf("Expected 2 context items, got %d", len(context))
	}

	if context["key1"] != "value1" {
		t.Errorf("Expected key1 = 'value1', got %v", context["key1"])
	}

	// Test with regular error
	regularErr := errors.New("regular error")
	context = GetContext(regularErr)
	if context != nil {
		t.Errorf("Expected nil context for regular error, got %v", context)
	}
}

func TestErrorTypeString(t *testing.T) {
	tests := []struct {
		errorType ErrorType
		expected  string
	}{
		{ErrorTypeNetwork, "network"},
		{ErrorTypeValidation, "validation"},
		{ErrorTypeDatabase, "database"},
		{ErrorTypeBitcoin, "bitcoin"},
		{ErrorTypeKafka, "kafka"},
		{ErrorTypeTimeout, "timeout"},
		{ErrorTypeInternal, "internal"},
	}

	for _, tt := range tests {
		t.Run(string(tt.errorType), func(t *testing.T) {
			if string(tt.errorType) != tt.expected {
				t.Errorf("Expected %q, got %q", tt.expected, string(tt.errorType))
			}
		})
	}
}

func TestIsRetryableByDefault(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{"nil error", nil, false},
		{"context canceled", context.Canceled, false},
		{"context timeout", context.DeadlineExceeded, false},
		{"connection refused", errors.New("connection refused"), true},
		{"connection reset", errors.New("connection reset by peer"), true},
		{"network unreachable", errors.New("network unreachable"), true},
		{"timeout error", errors.New("timeout occurred"), true},
		{"temporary failure", errors.New("temporary failure in name resolution"), true},
		{"too many connections", errors.New("too many connections"), true},
		{"unknown error", errors.New("unknown error"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isRetryableByDefault(tt.err); got != tt.expected {
				t.Errorf("isRetryableByDefault() = %v, want %v", got, tt.expected)
			}
		})
	}
}