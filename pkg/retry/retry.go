// Package retry provides retry mechanisms with exponential backoff for GOMP services.
package retry

import (
	"context"
	"math"
	"math/rand"
	"time"

	"github.com/bardlex/gomp/pkg/errors"
)

// Config holds retry configuration
type Config struct {
	MaxAttempts int
	BaseDelay   time.Duration
	MaxDelay    time.Duration
	Multiplier  float64
	Jitter      bool
}

// DefaultConfig returns a sensible default retry configuration
func DefaultConfig() *Config {
	return &Config{
		MaxAttempts: 3,
		BaseDelay:   100 * time.Millisecond,
		MaxDelay:    5 * time.Second,
		Multiplier:  2.0,
		Jitter:      true,
	}
}

// NetworkConfig returns retry configuration optimized for network operations
func NetworkConfig() *Config {
	return &Config{
		MaxAttempts: 5,
		BaseDelay:   50 * time.Millisecond,
		MaxDelay:    2 * time.Second,
		Multiplier:  1.5,
		Jitter:      true,
	}
}

// DatabaseConfig returns retry configuration optimized for database operations
func DatabaseConfig() *Config {
	return &Config{
		MaxAttempts: 3,
		BaseDelay:   200 * time.Millisecond,
		MaxDelay:    3 * time.Second,
		Multiplier:  2.0,
		Jitter:      true,
	}
}

// RetryableFunc is a function that can be retried
type RetryableFunc func() error

// Do executes a function with retry logic
func Do(ctx context.Context, config *Config, fn RetryableFunc) error {
	if config == nil {
		config = DefaultConfig()
	}

	var lastErr error
	
	for attempt := range config.MaxAttempts {
		// Execute the function
		err := fn()
		if err == nil {
			return nil // Success
		}

		lastErr = err

		// Check if we should retry
		if !errors.IsRetryable(err) {
			return err // Don't retry non-retryable errors
		}

		// Check if we've reached max attempts
		if attempt == config.MaxAttempts-1 {
			break // Don't delay after the last attempt
		}

		// Calculate delay with exponential backoff
		delay := config.calculateDelay(attempt)

		// Check context before sleeping
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(delay):
			// Continue to next attempt
		}
	}

	// Wrap the final error with retry context
	return errors.Wrap(lastErr, errors.ErrorTypeInternal, "retry", 
		"operation failed after maximum retry attempts").
		WithContext("max_attempts", config.MaxAttempts)
}

// DoWithResult executes a function with retry logic and returns a result
func DoWithResult[T any](ctx context.Context, config *Config, fn func() (T, error)) (T, error) {
	var zero T
	var lastErr error

	if config == nil {
		config = DefaultConfig()
	}

	for attempt := range config.MaxAttempts {
		// Execute the function
		res, err := fn()
		if err == nil {
			return res, nil // Success
		}

		lastErr = err

		// Check if we should retry
		if !errors.IsRetryable(err) {
			return zero, err // Don't retry non-retryable errors
		}

		// Check if we've reached max attempts
		if attempt == config.MaxAttempts-1 {
			break // Don't delay after the last attempt
		}

		// Calculate delay with exponential backoff
		delay := config.calculateDelay(attempt)

		// Check context before sleeping
		select {
		case <-ctx.Done():
			return zero, ctx.Err()
		case <-time.After(delay):
			// Continue to next attempt
		}
	}

	// Return zero value and wrapped error
	wrappedErr := errors.Wrap(lastErr, errors.ErrorTypeInternal, "retry", 
		"operation failed after maximum retry attempts").
		WithContext("max_attempts", config.MaxAttempts)
		
	return zero, wrappedErr
}

// calculateDelay calculates the delay for the given attempt using exponential backoff
func (c *Config) calculateDelay(attempt int) time.Duration {
	delay := float64(c.BaseDelay) * math.Pow(c.Multiplier, float64(attempt))
	
	// Cap at maximum delay
	delay = min(delay, float64(c.MaxDelay))

	// Add jitter if enabled
	if c.Jitter {
		// Add random jitter up to 10% of the delay
		jitter := delay * 0.1 * rand.Float64()
		delay += jitter
	}

	return time.Duration(delay)
}