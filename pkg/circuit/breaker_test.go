package circuit

import (
	"context"
	"errors"
	"testing"
	"time"

	gompErrors "github.com/bardlex/gomp/pkg/errors"
)

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()

	if config.MaxFailures != 5 {
		t.Errorf("Expected MaxFailures = 5, got %d", config.MaxFailures)
	}

	if config.SuccessRequired != 3 {
		t.Errorf("Expected SuccessRequired = 3, got %d", config.SuccessRequired)
	}

	if config.Timeout != 30*time.Second {
		t.Errorf("Expected Timeout = 30s, got %v", config.Timeout)
	}

	if config.ResetTimeout != 60*time.Second {
		t.Errorf("Expected ResetTimeout = 60s, got %v", config.ResetTimeout)
	}
}

func TestNew(t *testing.T) {
	config := &Config{
		MaxFailures:     3,
		SuccessRequired: 2,
		Timeout:         10 * time.Second,
		ResetTimeout:    30 * time.Second,
	}

	breaker := New(config)

	if breaker.config != config {
		t.Error("Expected config to be set")
	}

	if breaker.GetState() != StateClosed {
		t.Errorf("Expected initial state to be Closed, got %s", breaker.GetState())
	}
}

func TestNew_NilConfig(t *testing.T) {
	breaker := New(nil)

	if breaker.config == nil {
		t.Error("Expected default config when nil is passed")
	}

	if breaker.GetState() != StateClosed {
		t.Error("Expected initial state to be Closed")
	}
}

func TestState_String(t *testing.T) {
	tests := []struct {
		state    State
		expected string
	}{
		{StateClosed, "closed"},
		{StateOpen, "open"},
		{StateHalfOpen, "half-open"},
		{State(999), "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			if got := tt.state.String(); got != tt.expected {
				t.Errorf("State.String() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestBreaker_Execute_Success(t *testing.T) {
	breaker := New(&Config{
		MaxFailures:     3,
		SuccessRequired: 2,
		Timeout:         10 * time.Second,
		ResetTimeout:    30 * time.Second,
	})

	ctx := context.Background()
	callCount := 0

	fn := func() error {
		callCount++
		return nil
	}

	err := breaker.Execute(ctx, fn)
	if err != nil {
		t.Errorf("Expected success, got error: %v", err)
	}

	if callCount != 1 {
		t.Errorf("Expected 1 call, got %d", callCount)
	}

	if breaker.GetState() != StateClosed {
		t.Errorf("Expected state to remain Closed, got %s", breaker.GetState())
	}
}

func TestBreaker_Execute_OpenCircuit(t *testing.T) {
	breaker := New(&Config{
		MaxFailures:     2,
		SuccessRequired: 1,
		Timeout:         10 * time.Second,
		ResetTimeout:    30 * time.Second,
	})

	ctx := context.Background()
	callCount := 0

	failingFn := func() error {
		callCount++
		return errors.New("test error")
	}

	// Execute enough times to open the circuit
	for i := 0; i < 2; i++ {
		err := breaker.Execute(ctx, failingFn)
		if err == nil {
			t.Error("Expected error")
		}
	}

	if breaker.GetState() != StateOpen {
		t.Errorf("Expected state to be Open, got %s", breaker.GetState())
	}

	if callCount != 2 {
		t.Errorf("Expected 2 calls, got %d", callCount)
	}

	// Now try to execute again - should be rejected without calling function
	err := breaker.Execute(ctx, failingFn)
	if err == nil {
		t.Error("Expected circuit breaker to reject call")
	}

	if !gompErrors.IsType(err, gompErrors.ErrorTypeInternal) {
		t.Error("Expected circuit breaker error to be internal type")
	}

	if callCount != 2 {
		t.Error("Expected function not to be called when circuit is open")
	}
}

func TestBreaker_Execute_HalfOpen(t *testing.T) {
	breaker := New(&Config{
		MaxFailures:     2,
		SuccessRequired: 1,
		Timeout:         1 * time.Millisecond, // Very short timeout
		ResetTimeout:    30 * time.Second,
	})

	ctx := context.Background()

	// Open the circuit
	for i := 0; i < 2; i++ {
		_ = breaker.Execute(ctx, func() error {
			return errors.New("test error")
		})
	}

	if breaker.GetState() != StateOpen {
		t.Error("Expected circuit to be open")
	}

	// Wait for timeout to allow half-open state
	time.Sleep(2 * time.Millisecond)

	// Next call should move to half-open
	callCount := 0
	err := breaker.Execute(ctx, func() error {
		callCount++
		return nil // Success
	})

	if err != nil {
		t.Errorf("Expected success in half-open state, got: %v", err)
	}

	if callCount != 1 {
		t.Errorf("Expected 1 call, got %d", callCount)
	}

	if breaker.GetState() != StateClosed {
		t.Errorf("Expected circuit to close after success, got %s", breaker.GetState())
	}
}

func TestBreaker_Execute_HalfOpenFailure(t *testing.T) {
	breaker := New(&Config{
		MaxFailures:     2,
		SuccessRequired: 1,
		Timeout:         1 * time.Millisecond,
		ResetTimeout:    30 * time.Second,
	})

	ctx := context.Background()

	// Open the circuit
	for i := 0; i < 2; i++ {
		_ = breaker.Execute(ctx, func() error {
			return errors.New("test error")
		})
	}

	// Wait for timeout
	time.Sleep(2 * time.Millisecond)

	// Failure in half-open should reopen circuit
	callCount := 0
	err := breaker.Execute(ctx, func() error {
		callCount++
		return errors.New("half-open failure")
	})

	if err == nil {
		t.Error("Expected error")
	}

	if callCount != 1 {
		t.Errorf("Expected 1 call, got %d", callCount)
	}

	if breaker.GetState() != StateOpen {
		t.Errorf("Expected circuit to reopen after failure, got %s", breaker.GetState())
	}
}

func TestExecuteWithResult_Success(t *testing.T) {
	breaker := New(DefaultConfig())
	ctx := context.Background()

	result, err := ExecuteWithResult(ctx, breaker, func() (string, error) {
		return "success", nil
	})

	if err != nil {
		t.Errorf("Expected success, got error: %v", err)
	}

	if result != "success" {
		t.Errorf("Expected result 'success', got '%s'", result)
	}
}

func TestExecuteWithResult_CircuitOpen(t *testing.T) {
	breaker := New(&Config{
		MaxFailures:     1,
		SuccessRequired: 1,
		Timeout:         10 * time.Second,
		ResetTimeout:    30 * time.Second,
	})

	ctx := context.Background()

	// Open the circuit
	_, _ = ExecuteWithResult(ctx, breaker, func() (string, error) {
		return "", errors.New("test error")
	})

	// Should be rejected
	result, err := ExecuteWithResult(ctx, breaker, func() (string, error) {
		return "should not execute", nil
	})

	if err == nil {
		t.Error("Expected circuit breaker to reject call")
	}

	if result != "" {
		t.Errorf("Expected empty result when circuit is open, got '%s'", result)
	}
}

func TestBreaker_GetStats(t *testing.T) {
	breaker := New(&Config{
		MaxFailures:     3,
		SuccessRequired: 2,
		Timeout:         10 * time.Second,
		ResetTimeout:    30 * time.Second,
	})

	ctx := context.Background()

	// Execute some operations
	_ = breaker.Execute(ctx, func() error { return nil })                  // Success
	_ = breaker.Execute(ctx, func() error { return errors.New("error") }) // Failure

	stats := breaker.GetStats()

	if stats.State != StateClosed {
		t.Errorf("Expected state Closed, got %s", stats.State)
	}

	if stats.Failures != 1 {
		t.Errorf("Expected 1 failure, got %d", stats.Failures)
	}

	if stats.Successes != 1 {
		t.Errorf("Expected 1 success, got %d", stats.Successes)
	}

	if stats.LastFailTime.IsZero() {
		t.Error("Expected LastFailTime to be set")
	}
}

func TestBreaker_Reset(t *testing.T) {
	breaker := New(&Config{
		MaxFailures:     1,
		SuccessRequired: 1,
		Timeout:         10 * time.Second,
		ResetTimeout:    30 * time.Second,
	})

	ctx := context.Background()

	// Open the circuit
	breaker.Execute(ctx, func() error {
		return errors.New("test error")
	})

	if breaker.GetState() != StateOpen {
		t.Error("Expected circuit to be open")
	}

	stats := breaker.GetStats()
	if stats.Failures != 1 {
		t.Error("Expected failures to be recorded")
	}

	// Reset the circuit breaker
	breaker.Reset()

	if breaker.GetState() != StateClosed {
		t.Errorf("Expected state to be Closed after reset, got %s", breaker.GetState())
	}

	statsAfterReset := breaker.GetStats()
	if statsAfterReset.Failures != 0 {
		t.Errorf("Expected failures to be reset to 0, got %d", statsAfterReset.Failures)
	}

	if statsAfterReset.Successes != 0 {
		t.Errorf("Expected successes to be reset to 0, got %d", statsAfterReset.Successes)
	}
}

func TestBreaker_SuccessRequiredInHalfOpen(t *testing.T) {
	breaker := New(&Config{
		MaxFailures:     2,
		SuccessRequired: 3, // Require 3 successes to close
		Timeout:         1 * time.Millisecond,
		ResetTimeout:    30 * time.Second,
	})

	ctx := context.Background()

	// Open the circuit
	for i := 0; i < 2; i++ {
		_ = breaker.Execute(ctx, func() error {
			return errors.New("test error")
		})
	}

	// Wait for timeout
	time.Sleep(2 * time.Millisecond)

	// Execute 2 successful operations (not enough to close)
	for i := 0; i < 2; i++ {
		err := breaker.Execute(ctx, func() error {
			return nil
		})
		if err != nil {
			t.Errorf("Expected success in half-open, got: %v", err)
		}
	}

	// Should still be half-open
	if breaker.GetState() != StateHalfOpen {
		t.Errorf("Expected state to be HalfOpen, got %s", breaker.GetState())
	}

	// Execute one more successful operation
	err := breaker.Execute(ctx, func() error {
		return nil
	})
	if err != nil {
		t.Errorf("Expected success, got: %v", err)
	}

	// Now should be closed
	if breaker.GetState() != StateClosed {
		t.Errorf("Expected state to be Closed after required successes, got %s", breaker.GetState())
	}
}

func TestBreaker_ResetTimeout(t *testing.T) {
	breaker := New(&Config{
		MaxFailures:     2,
		SuccessRequired: 1,
		Timeout:         10 * time.Second,
		ResetTimeout:    1 * time.Millisecond, // Very short reset timeout
	})

	ctx := context.Background()

	// Record one failure
	breaker.Execute(ctx, func() error {
		return errors.New("test error")
	})

	// Wait for reset timeout
	time.Sleep(2 * time.Millisecond)

	// Execute successful operation
	err := breaker.Execute(ctx, func() error {
		return nil
	})
	if err != nil {
		t.Errorf("Expected success, got: %v", err)
	}

	// Check that failure count was reset
	stats := breaker.GetStats()
	if stats.Failures != 0 {
		t.Errorf("Expected failures to be reset after timeout, got %d", stats.Failures)
	}
}