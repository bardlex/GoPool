// Package circuit provides circuit breaker implementation for GOMP services.
package circuit

import (
	"context"
	"sync"
	"time"

	"github.com/bardlex/gomp/pkg/errors"
)

// State represents the circuit breaker state
type State int

const (
	// StateClosed - circuit is closed, requests are allowed
	StateClosed State = iota
	// StateOpen - circuit is open, requests are rejected
	StateOpen
	// StateHalfOpen - circuit allows limited requests to test recovery
	StateHalfOpen
)

// String returns string representation of the state
func (s State) String() string {
	switch s {
	case StateClosed:
		return "closed"
	case StateOpen:
		return "open"
	case StateHalfOpen:
		return "half-open"
	default:
		return "unknown"
	}
}

// Config holds circuit breaker configuration
type Config struct {
	MaxFailures     int           // Maximum failures before opening
	SuccessRequired int           // Successful calls required to close from half-open
	Timeout         time.Duration // How long to wait before going to half-open
	ResetTimeout    time.Duration // How long to reset failure count in closed state
}

// DefaultConfig returns a sensible default configuration
func DefaultConfig() *Config {
	return &Config{
		MaxFailures:     5,
		SuccessRequired: 3,
		Timeout:         30 * time.Second,
		ResetTimeout:    60 * time.Second,
	}
}

// Breaker implements the circuit breaker pattern
type Breaker struct {
	config *Config
	mutex  sync.RWMutex

	state         State
	failures      int
	successes     int
	lastFailTime  time.Time
	lastResetTime time.Time
}

// New creates a new circuit breaker
func New(config *Config) *Breaker {
	if config == nil {
		config = DefaultConfig()
	}

	return &Breaker{
		config:        config,
		state:         StateClosed,
		lastResetTime: time.Now(),
	}
}

// Execute runs a function with circuit breaker protection
func (cb *Breaker) Execute(_ context.Context, fn func() error) error {
	// Check if request should be allowed
	if !cb.allowRequest() {
		return errors.New(errors.ErrorTypeInternal, "circuit_breaker", 
			"circuit breaker is open").
			WithContext("state", cb.GetState().String())
	}

	// Execute the function
	err := fn()

	// Record the result
	cb.recordResult(err)

	return err
}

// ExecuteWithResult runs a function with circuit breaker protection and returns result
func ExecuteWithResult[T any](ctx context.Context, cb *Breaker, fn func() (T, error)) (T, error) {
	var zero T

	// Check if request should be allowed
	if !cb.allowRequest() {
		return zero, errors.New(errors.ErrorTypeInternal, "circuit_breaker", 
			"circuit breaker is open").
			WithContext("state", cb.GetState().String())
	}

	// Execute the function
	result, err := fn()

	// Record the result
	cb.recordResult(err)

	return result, err
}

// allowRequest determines if a request should be allowed based on current state
func (cb *Breaker) allowRequest() bool {
	cb.mutex.Lock()
	defer cb.mutex.Unlock()

	now := time.Now()

	switch cb.state {
	case StateClosed:
		// Reset failure count if reset timeout has passed
		if now.Sub(cb.lastResetTime) > cb.config.ResetTimeout {
			cb.failures = 0
			cb.lastResetTime = now
		}
		return true

	case StateOpen:
		// Check if we should transition to half-open
		if now.Sub(cb.lastFailTime) > cb.config.Timeout {
			cb.state = StateHalfOpen
			cb.successes = 0
			return true
		}
		return false

	case StateHalfOpen:
		return true

	default:
		return false
	}
}

// recordResult records the result of a function execution
func (cb *Breaker) recordResult(err error) {
	cb.mutex.Lock()
	defer cb.mutex.Unlock()

	if err != nil {
		// Record failure
		cb.failures++
		cb.lastFailTime = time.Now()

		// Check if we should open the circuit
		if cb.state == StateClosed && cb.failures >= cb.config.MaxFailures {
			cb.state = StateOpen
			cb.successes = 0 // Reset on state change
		} else if cb.state == StateHalfOpen {
			cb.state = StateOpen
			cb.successes = 0 // Reset on state change
		}
	} else {
		// Record success
		if cb.state == StateHalfOpen {
			cb.successes++
			if cb.successes >= cb.config.SuccessRequired {
				cb.state = StateClosed
				cb.failures = 0
				cb.successes = 0
				cb.lastResetTime = time.Now()
			}
		} else if cb.state == StateClosed {
			// Track successes in closed state for stats
			cb.successes++
		}
	}
}

// GetState returns the current state of the circuit breaker
func (cb *Breaker) GetState() State {
	cb.mutex.RLock()
	defer cb.mutex.RUnlock()
	return cb.state
}

// GetStats returns statistics about the circuit breaker
func (cb *Breaker) GetStats() Stats {
	cb.mutex.RLock()
	defer cb.mutex.RUnlock()

	return Stats{
		State:        cb.state,
		Failures:     cb.failures,
		Successes:    cb.successes,
		LastFailTime: cb.lastFailTime,
	}
}

// Stats represents circuit breaker statistics
type Stats struct {
	State        State
	Failures     int
	Successes    int
	LastFailTime time.Time
}

// Reset manually resets the circuit breaker to closed state
func (cb *Breaker) Reset() {
	cb.mutex.Lock()
	defer cb.mutex.Unlock()

	cb.state = StateClosed
	cb.failures = 0
	cb.successes = 0
	cb.lastResetTime = time.Now()
}