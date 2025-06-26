// Package errors provides error handling utilities for GOMP services.
package errors

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

// ErrorType represents different categories of errors
type ErrorType string

const (
	// ErrorTypeNetwork represents network-related errors
	ErrorTypeNetwork ErrorType = "network"
	// ErrorTypeValidation represents validation errors
	ErrorTypeValidation ErrorType = "validation"
	// ErrorTypeDatabase represents database-related errors
	ErrorTypeDatabase ErrorType = "database"
	// ErrorTypeBitcoin represents Bitcoin RPC errors
	ErrorTypeBitcoin ErrorType = "bitcoin"
	// ErrorTypeKafka represents Kafka messaging errors
	ErrorTypeKafka ErrorType = "kafka"
	// ErrorTypeTimeout represents timeout errors
	ErrorTypeTimeout ErrorType = "timeout"
	// ErrorTypeInternal represents internal/unknown errors
	ErrorTypeInternal ErrorType = "internal"
)

// ServiceError represents a structured error with context
type ServiceError struct {
	Type      ErrorType
	Operation string
	Message   string
	Cause     error
	Context   map[string]interface{}
	Timestamp time.Time
	Retryable bool
}

// Error implements the error interface
func (e *ServiceError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s operation '%s' failed: %s (caused by: %v)", e.Type, e.Operation, e.Message, e.Cause)
	}
	return fmt.Sprintf("%s operation '%s' failed: %s", e.Type, e.Operation, e.Message)
}

// Unwrap returns the underlying cause for error unwrapping
func (e *ServiceError) Unwrap() error {
	return e.Cause
}

// IsRetryable returns whether this error should be retried
func (e *ServiceError) IsRetryable() bool {
	return e.Retryable
}

// WithContext adds additional context to the error
func (e *ServiceError) WithContext(key string, value interface{}) *ServiceError {
	if e.Context == nil {
		e.Context = make(map[string]interface{})
	}
	e.Context[key] = value
	return e
}

// New creates a new ServiceError
func New(errorType ErrorType, operation, message string) *ServiceError {
	return &ServiceError{
		Type:      errorType,
		Operation: operation,
		Message:   message,
		Timestamp: time.Now(),
		Retryable: isRetryableByType(errorType),
	}
}

// Wrap wraps an existing error with context
func Wrap(err error, errorType ErrorType, operation, message string) *ServiceError {
	if err == nil {
		return nil
	}

	// If it's already a ServiceError, preserve the original type unless explicitly overridden
	if se, ok := err.(*ServiceError); ok {
		return &ServiceError{
			Type:      errorType,
			Operation: operation,
			Message:   message,
			Cause:     se,
			Timestamp: time.Now(),
			Retryable: se.Retryable,
		}
	}

	return &ServiceError{
		Type:      errorType,
		Operation: operation,
		Message:   message,
		Cause:     err,
		Timestamp: time.Now(),
		Retryable: isRetryableByDefault(err),
	}
}

// isRetryableByType determines if an error type is generally retryable
func isRetryableByType(errorType ErrorType) bool {
	switch errorType {
	case ErrorTypeNetwork, ErrorTypeTimeout, ErrorTypeKafka:
		return true
	case ErrorTypeValidation:
		return false
	default:
		return false
	}
}

// isRetryableByDefault checks if an error is retryable based on common patterns
func isRetryableByDefault(err error) bool {
	if err == nil {
		return false
	}

	// Check for context cancellation/timeout (not retryable)
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}

	errStr := strings.ToLower(err.Error())
	
	// Network-related errors are usually retryable
	networkErrors := []string{
		"connection refused",
		"connection reset",
		"network unreachable",
		"timeout",
		"temporary failure",
		"too many connections",
	}

	for _, netErr := range networkErrors {
		if strings.Contains(errStr, netErr) {
			return true
		}
	}

	return false
}

// IsType checks if an error is of a specific type
func IsType(err error, errorType ErrorType) bool {
	var se *ServiceError
	if errors.As(err, &se) {
		return se.Type == errorType
	}
	return false
}

// IsRetryable checks if an error should be retried
func IsRetryable(err error) bool {
	var se *ServiceError
	if errors.As(err, &se) {
		return se.IsRetryable()
	}
	return isRetryableByDefault(err)
}

// GetContext retrieves context from a ServiceError
func GetContext(err error) map[string]interface{} {
	var se *ServiceError
	if errors.As(err, &se) {
		return se.Context
	}
	return nil
}