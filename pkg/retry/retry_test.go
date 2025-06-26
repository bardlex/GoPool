package retry

import (
	"context"
	"errors"
	"testing"
	"time"

	gompErrors "github.com/bardlex/gomp/pkg/errors"
)

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()

	if config.MaxAttempts != 3 {
		t.Errorf("Expected MaxAttempts = 3, got %d", config.MaxAttempts)
	}

	if config.BaseDelay != 100*time.Millisecond {
		t.Errorf("Expected BaseDelay = 100ms, got %v", config.BaseDelay)
	}

	if config.MaxDelay != 5*time.Second {
		t.Errorf("Expected MaxDelay = 5s, got %v", config.MaxDelay)
	}

	if config.Multiplier != 2.0 {
		t.Errorf("Expected Multiplier = 2.0, got %f", config.Multiplier)
	}

	if !config.Jitter {
		t.Error("Expected Jitter = true")
	}
}

func TestNetworkConfig(t *testing.T) {
	config := NetworkConfig()

	if config.MaxAttempts != 5 {
		t.Errorf("Expected MaxAttempts = 5, got %d", config.MaxAttempts)
	}

	if config.BaseDelay != 50*time.Millisecond {
		t.Errorf("Expected BaseDelay = 50ms, got %v", config.BaseDelay)
	}

	if config.MaxDelay != 2*time.Second {
		t.Errorf("Expected MaxDelay = 2s, got %v", config.MaxDelay)
	}

	if config.Multiplier != 1.5 {
		t.Errorf("Expected Multiplier = 1.5, got %f", config.Multiplier)
	}
}

func TestDatabaseConfig(t *testing.T) {
	config := DatabaseConfig()

	if config.MaxAttempts != 3 {
		t.Errorf("Expected MaxAttempts = 3, got %d", config.MaxAttempts)
	}

	if config.BaseDelay != 200*time.Millisecond {
		t.Errorf("Expected BaseDelay = 200ms, got %v", config.BaseDelay)
	}

	if config.MaxDelay != 3*time.Second {
		t.Errorf("Expected MaxDelay = 3s, got %v", config.MaxDelay)
	}

	if config.Multiplier != 2.0 {
		t.Errorf("Expected Multiplier = 2.0, got %f", config.Multiplier)
	}
}

func TestDo_Success(t *testing.T) {
	ctx := context.Background()
	config := &Config{
		MaxAttempts: 3,
		BaseDelay:   10 * time.Millisecond,
		MaxDelay:    100 * time.Millisecond,
		Multiplier:  2.0,
		Jitter:      false,
	}

	callCount := 0
	fn := func() error {
		callCount++
		if callCount == 1 {
			return gompErrors.New(gompErrors.ErrorTypeNetwork, "test", "retryable error")
		}
		return nil // Success on second attempt
	}

	err := Do(ctx, config, fn)
	if err != nil {
		t.Errorf("Expected success, got error: %v", err)
	}

	if callCount != 2 {
		t.Errorf("Expected 2 calls, got %d", callCount)
	}
}

func TestDo_MaxAttemptsReached(t *testing.T) {
	ctx := context.Background()
	config := &Config{
		MaxAttempts: 2,
		BaseDelay:   1 * time.Millisecond,
		MaxDelay:    10 * time.Millisecond,
		Multiplier:  2.0,
		Jitter:      false,
	}

	callCount := 0
	fn := func() error {
		callCount++
		return gompErrors.New(gompErrors.ErrorTypeNetwork, "test", "persistent error")
	}

	err := Do(ctx, config, fn)
	if err == nil {
		t.Error("Expected error after max attempts")
	}

	if callCount != 2 {
		t.Errorf("Expected 2 calls, got %d", callCount)
	}

	// Check that the error contains retry context
	if !gompErrors.IsType(err, gompErrors.ErrorTypeInternal) {
		t.Error("Expected wrapped error to be internal type")
	}
}

func TestDo_NonRetryableError(t *testing.T) {
	ctx := context.Background()
	config := DefaultConfig()

	callCount := 0
	fn := func() error {
		callCount++
		return gompErrors.New(gompErrors.ErrorTypeValidation, "test", "validation error")
	}

	err := Do(ctx, config, fn)
	if err == nil {
		t.Error("Expected error")
	}

	if callCount != 1 {
		t.Errorf("Expected 1 call (no retry), got %d", callCount)
	}

	// Should be the original validation error, not wrapped
	if !gompErrors.IsType(err, gompErrors.ErrorTypeValidation) {
		t.Error("Expected original validation error type")
	}
}

func TestDo_ContextCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	config := &Config{
		MaxAttempts: 5,
		BaseDelay:   100 * time.Millisecond,
		MaxDelay:    1 * time.Second,
		Multiplier:  2.0,
		Jitter:      false,
	}

	callCount := 0
	fn := func() error {
		callCount++
		if callCount == 2 {
			cancel() // Cancel context during retry delay
		}
		return gompErrors.New(gompErrors.ErrorTypeNetwork, "test", "network error")
	}

	err := Do(ctx, config, fn)
	if err != context.Canceled {
		t.Errorf("Expected context.Canceled, got %v", err)
	}

	// Should have been called twice (once before cancel, once after)
	if callCount < 1 || callCount > 2 {
		t.Errorf("Expected 1-2 calls, got %d", callCount)
	}
}

func TestDoWithResult_Success(t *testing.T) {
	ctx := context.Background()
	config := &Config{
		MaxAttempts: 3,
		BaseDelay:   1 * time.Millisecond,
		MaxDelay:    10 * time.Millisecond,
		Multiplier:  2.0,
		Jitter:      false,
	}

	callCount := 0
	fn := func() (string, error) {
		callCount++
		if callCount == 1 {
			return "", gompErrors.New(gompErrors.ErrorTypeNetwork, "test", "retryable error")
		}
		return "success", nil
	}

	result, err := DoWithResult(ctx, config, fn)
	if err != nil {
		t.Errorf("Expected success, got error: %v", err)
	}

	if result != "success" {
		t.Errorf("Expected result 'success', got '%s'", result)
	}

	if callCount != 2 {
		t.Errorf("Expected 2 calls, got %d", callCount)
	}
}

func TestDoWithResult_Failure(t *testing.T) {
	ctx := context.Background()
	config := &Config{
		MaxAttempts: 2,
		BaseDelay:   1 * time.Millisecond,
		MaxDelay:    10 * time.Millisecond,
		Multiplier:  2.0,
		Jitter:      false,
	}

	callCount := 0
	fn := func() (int, error) {
		callCount++
		return 0, gompErrors.New(gompErrors.ErrorTypeNetwork, "test", "persistent error")
	}

	result, err := DoWithResult(ctx, config, fn)
	if err == nil {
		t.Error("Expected error after max attempts")
	}

	if result != 0 {
		t.Errorf("Expected zero value result, got %d", result)
	}

	if callCount != 2 {
		t.Errorf("Expected 2 calls, got %d", callCount)
	}
}

func TestConfig_calculateDelay(t *testing.T) {
	config := &Config{
		BaseDelay:  100 * time.Millisecond,
		MaxDelay:   1 * time.Second,
		Multiplier: 2.0,
		Jitter:     false,
	}

	tests := []struct {
		attempt  int
		expected time.Duration
	}{
		{0, 100 * time.Millisecond},
		{1, 200 * time.Millisecond},
		{2, 400 * time.Millisecond},
		{3, 800 * time.Millisecond},
		{4, 1 * time.Second}, // Capped at MaxDelay
		{5, 1 * time.Second}, // Still capped
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			delay := config.calculateDelay(tt.attempt)
			if delay != tt.expected {
				t.Errorf("For attempt %d, expected delay %v, got %v", tt.attempt, tt.expected, delay)
			}
		})
	}
}

func TestConfig_calculateDelay_WithJitter(t *testing.T) {
	config := &Config{
		BaseDelay:  100 * time.Millisecond,
		MaxDelay:   1 * time.Second,
		Multiplier: 2.0,
		Jitter:     true,
	}

	baseDelay := config.calculateDelay(0)
	
	// With jitter, the delay should be at least the base delay
	// and at most base delay + 10% jitter
	expectedMin := 100 * time.Millisecond
	expectedMax := time.Duration(110 * time.Millisecond) // 100ms + 10% jitter

	if baseDelay < expectedMin {
		t.Errorf("Delay with jitter too small: %v < %v", baseDelay, expectedMin)
	}

	if baseDelay > expectedMax {
		t.Errorf("Delay with jitter too large: %v > %v", baseDelay, expectedMax)
	}
}

func TestDo_NilConfig(t *testing.T) {
	ctx := context.Background()

	callCount := 0
	fn := func() error {
		callCount++
		if callCount == 1 {
			return gompErrors.New(gompErrors.ErrorTypeNetwork, "test", "retryable error")
		}
		return nil
	}

	err := Do(ctx, nil, fn) // Pass nil config
	if err != nil {
		t.Errorf("Expected success with default config, got error: %v", err)
	}

	if callCount != 2 {
		t.Errorf("Expected 2 calls with default config, got %d", callCount)
	}
}

func TestDoWithResult_NilConfig(t *testing.T) {
	ctx := context.Background()

	callCount := 0
	fn := func() (string, error) {
		callCount++
		if callCount == 1 {
			return "", gompErrors.New(gompErrors.ErrorTypeNetwork, "test", "retryable error")
		}
		return "success", nil
	}

	result, err := DoWithResult(ctx, nil, fn) // Pass nil config
	if err != nil {
		t.Errorf("Expected success with default config, got error: %v", err)
	}

	if result != "success" {
		t.Errorf("Expected result 'success', got '%s'", result)
	}

	if callCount != 2 {
		t.Errorf("Expected 2 calls with default config, got %d", callCount)
	}
}

func TestDo_RegularError(t *testing.T) {
	ctx := context.Background()
	config := DefaultConfig()

	callCount := 0
	fn := func() error {
		callCount++
		return errors.New("regular error") // Not a ServiceError
	}

	err := Do(ctx, config, fn)
	if err == nil {
		t.Error("Expected error")
	}

	// Regular errors are not retryable by default
	if callCount != 1 {
		t.Errorf("Expected 1 call (no retry for regular error), got %d", callCount)
	}
}