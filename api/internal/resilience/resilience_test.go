package resilience

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
}

// -----------------------------------------------------------------------------
// Circuit Breaker Tests
// -----------------------------------------------------------------------------

func TestCircuitBreakerDefaultConfig(t *testing.T) {
	cfg := DefaultCircuitBreakerConfig()

	if cfg.FailureThreshold != 5 {
		t.Errorf("expected FailureThreshold 5, got %d", cfg.FailureThreshold)
	}
	if cfg.SuccessThreshold != 2 {
		t.Errorf("expected SuccessThreshold 2, got %d", cfg.SuccessThreshold)
	}
	if cfg.Timeout != 30*time.Second {
		t.Errorf("expected Timeout 30s, got %v", cfg.Timeout)
	}
}

func TestCircuitBreakerStartsClosed(t *testing.T) {
	cb := NewCircuitBreaker("test", DefaultCircuitBreakerConfig(), testLogger())

	if cb.State() != StateClosed {
		t.Errorf("expected state Closed, got %s", cb.State().String())
	}
}

func TestCircuitBreakerOpensAfterFailures(t *testing.T) {
	cfg := CircuitBreakerConfig{
		FailureThreshold: 3,
		SuccessThreshold: 1,
		Timeout:          time.Second,
	}
	cb := NewCircuitBreaker("test", cfg, testLogger())

	testErr := errors.New("test error")
	ctx := context.Background()

	// First 2 failures should keep circuit closed
	for i := 0; i < 2; i++ {
		_ = cb.Execute(ctx, func() error { return testErr })
		if cb.State() != StateClosed {
			t.Errorf("circuit should still be closed after %d failures", i+1)
		}
	}

	// Third failure should open the circuit
	_ = cb.Execute(ctx, func() error { return testErr })
	if cb.State() != StateOpen {
		t.Errorf("expected state Open after 3 failures, got %s", cb.State().String())
	}
}

func TestCircuitBreakerRejectsWhenOpen(t *testing.T) {
	cfg := CircuitBreakerConfig{
		FailureThreshold: 1,
		SuccessThreshold: 1,
		Timeout:          time.Hour, // Long timeout to stay open
	}
	cb := NewCircuitBreaker("test", cfg, testLogger())

	ctx := context.Background()

	// Trigger circuit open
	_ = cb.Execute(ctx, func() error { return errors.New("fail") })

	// Subsequent requests should be rejected
	err := cb.Execute(ctx, func() error { return nil })
	if !errors.Is(err, ErrCircuitOpen) {
		t.Errorf("expected ErrCircuitOpen, got %v", err)
	}
}

func TestCircuitBreakerTransitionsToHalfOpen(t *testing.T) {
	cfg := CircuitBreakerConfig{
		FailureThreshold: 1,
		SuccessThreshold: 1,
		Timeout:          50 * time.Millisecond,
	}
	cb := NewCircuitBreaker("test", cfg, testLogger())

	ctx := context.Background()

	// Open the circuit
	_ = cb.Execute(ctx, func() error { return errors.New("fail") })
	if cb.State() != StateOpen {
		t.Fatal("circuit should be open")
	}

	// Wait for timeout to elapse, then verify request is allowed (half-open)
	called := false
	require.Eventually(t, func() bool {
		err := cb.Execute(ctx, func() error {
			called = true
			return nil
		})
		return err == nil && called
	}, 1*time.Second, 10*time.Millisecond, "request should have been allowed in half-open state")
}

func TestCircuitBreakerClosesAfterSuccessInHalfOpen(t *testing.T) {
	cfg := CircuitBreakerConfig{
		FailureThreshold: 1,
		SuccessThreshold: 2,
		Timeout:          10 * time.Millisecond,
	}
	cb := NewCircuitBreaker("test", cfg, testLogger())

	ctx := context.Background()

	// Open the circuit
	_ = cb.Execute(ctx, func() error { return errors.New("fail") })

	// Wait for timeout to elapse and execute successful requests in half-open
	require.Eventually(t, func() bool {
		err := cb.Execute(ctx, func() error { return nil })
		return err == nil
	}, 1*time.Second, 10*time.Millisecond, "circuit should allow request in half-open")

	// Second successful request should close the circuit
	_ = cb.Execute(ctx, func() error { return nil })

	require.Equal(t, StateClosed, cb.State(), "expected state Closed")
}

func TestCircuitBreakerReopensOnFailureInHalfOpen(t *testing.T) {
	cfg := CircuitBreakerConfig{
		FailureThreshold: 1,
		SuccessThreshold: 2,
		Timeout:          10 * time.Millisecond,
	}
	cb := NewCircuitBreaker("test", cfg, testLogger())

	ctx := context.Background()

	// Open the circuit
	_ = cb.Execute(ctx, func() error { return errors.New("fail") })

	// Wait for timeout to elapse and send a failure in half-open
	require.Eventually(t, func() bool {
		// Try to execute -- if circuit is still open, we get ErrCircuitOpen
		// Once half-open, it allows through and we send a failure
		err := cb.Execute(ctx, func() error { return errors.New("fail again") })
		return err != nil && !errors.Is(err, ErrCircuitOpen)
	}, 1*time.Second, 10*time.Millisecond, "circuit should transition to half-open and accept request")

	require.Equal(t, StateOpen, cb.State(), "expected state Open after failure in half-open")
}

func TestCircuitBreakerResetsFailuresOnSuccess(t *testing.T) {
	cfg := CircuitBreakerConfig{
		FailureThreshold: 3,
		SuccessThreshold: 1,
		Timeout:          time.Second,
	}
	cb := NewCircuitBreaker("test", cfg, testLogger())

	ctx := context.Background()

	// Two failures
	_ = cb.Execute(ctx, func() error { return errors.New("fail") })
	_ = cb.Execute(ctx, func() error { return errors.New("fail") })

	// Success resets counter
	_ = cb.Execute(ctx, func() error { return nil })

	// Two more failures should not open (since we reset)
	_ = cb.Execute(ctx, func() error { return errors.New("fail") })
	_ = cb.Execute(ctx, func() error { return errors.New("fail") })

	if cb.State() != StateClosed {
		t.Errorf("expected state Closed, got %s", cb.State().String())
	}
}

func TestCircuitBreakerStats(t *testing.T) {
	cb := NewCircuitBreaker("test-stats", DefaultCircuitBreakerConfig(), testLogger())

	stats := cb.Stats()
	if stats.Name != "test-stats" {
		t.Errorf("expected name 'test-stats', got %s", stats.Name)
	}
	if stats.State != "closed" {
		t.Errorf("expected state 'closed', got %s", stats.State)
	}
}

func TestCircuitBreakerOnStateChange(t *testing.T) {
	stateChanges := make(chan struct {
		from, to CircuitState
	}, 10)

	cfg := CircuitBreakerConfig{
		FailureThreshold: 1,
		SuccessThreshold: 1,
		Timeout:          10 * time.Millisecond,
		OnStateChange: func(from, to CircuitState) {
			stateChanges <- struct{ from, to CircuitState }{from, to}
		},
	}
	cb := NewCircuitBreaker("test", cfg, testLogger())

	ctx := context.Background()

	// Trigger state change
	_ = cb.Execute(ctx, func() error { return errors.New("fail") })

	select {
	case change := <-stateChanges:
		if change.from != StateClosed || change.to != StateOpen {
			t.Errorf("unexpected state change: %s -> %s", change.from, change.to)
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("expected state change callback")
	}
}

func TestCircuitBreakerConcurrency(t *testing.T) {
	cfg := CircuitBreakerConfig{
		FailureThreshold: 100,
		SuccessThreshold: 1,
		Timeout:          time.Second,
	}
	cb := NewCircuitBreaker("test", cfg, testLogger())

	ctx := context.Background()
	var wg sync.WaitGroup
	numGoroutines := 50

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < 10; j++ {
				if id%2 == 0 {
					_ = cb.Execute(ctx, func() error { return nil })
				} else {
					_ = cb.Execute(ctx, func() error { return errors.New("fail") })
				}
			}
		}(i)
	}

	wg.Wait()

	// Circuit should still be functional
	err := cb.Execute(ctx, func() error { return nil })
	if err != nil && !errors.Is(err, ErrCircuitOpen) {
		t.Errorf("unexpected error: %v", err)
	}
}

// -----------------------------------------------------------------------------
// Retry Tests
// -----------------------------------------------------------------------------

func TestRetryDefaultConfig(t *testing.T) {
	cfg := DefaultRetryConfig()

	if cfg.MaxAttempts != 3 {
		t.Errorf("expected MaxAttempts 3, got %d", cfg.MaxAttempts)
	}
	if cfg.InitialDelay != 100*time.Millisecond {
		t.Errorf("expected InitialDelay 100ms, got %v", cfg.InitialDelay)
	}
	if cfg.MaxDelay != 5*time.Second {
		t.Errorf("expected MaxDelay 5s, got %v", cfg.MaxDelay)
	}
	if cfg.Multiplier != 2.0 {
		t.Errorf("expected Multiplier 2.0, got %f", cfg.Multiplier)
	}
}

func TestRetrySucceedsOnFirstAttempt(t *testing.T) {
	ctx := context.Background()
	attempts := 0

	err := Retry(ctx, DefaultRetryConfig(), func() error {
		attempts++
		return nil
	})

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if attempts != 1 {
		t.Errorf("expected 1 attempt, got %d", attempts)
	}
}

func TestRetrySucceedsOnSecondAttempt(t *testing.T) {
	ctx := context.Background()
	attempts := 0

	cfg := RetryConfig{
		MaxAttempts:  3,
		InitialDelay: 10 * time.Millisecond,
		MaxDelay:     100 * time.Millisecond,
		Multiplier:   2.0,
		RetryableErrors: func(err error) bool {
			return true // Retry all errors
		},
	}

	err := Retry(ctx, cfg, func() error {
		attempts++
		if attempts < 2 {
			return errors.New("transient error")
		}
		return nil
	})

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if attempts != 2 {
		t.Errorf("expected 2 attempts, got %d", attempts)
	}
}

func TestRetryExhaustsAttempts(t *testing.T) {
	ctx := context.Background()
	attempts := 0

	cfg := RetryConfig{
		MaxAttempts:  3,
		InitialDelay: 10 * time.Millisecond,
		MaxDelay:     100 * time.Millisecond,
		Multiplier:   2.0,
		RetryableErrors: func(err error) bool {
			return true
		},
	}

	err := Retry(ctx, cfg, func() error {
		attempts++
		return errors.New("persistent error")
	})

	if err == nil {
		t.Error("expected error after max retries")
	}
	if attempts != 3 {
		t.Errorf("expected 3 attempts, got %d", attempts)
	}
}

func TestRetryRespectsContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	attempts := 0

	cfg := RetryConfig{
		MaxAttempts:  10,
		InitialDelay: 50 * time.Millisecond,
		MaxDelay:     time.Second,
		Multiplier:   2.0,
		RetryableErrors: func(err error) bool {
			return true
		},
	}

	go func() {
		time.Sleep(100 * time.Millisecond)
		cancel()
	}()

	err := Retry(ctx, cfg, func() error {
		attempts++
		return errors.New("keep retrying")
	})

	if !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled, got %v", err)
	}
}

func TestRetryDoesNotRetryNonRetryableErrors(t *testing.T) {
	ctx := context.Background()
	attempts := 0

	cfg := RetryConfig{
		MaxAttempts:  3,
		InitialDelay: 10 * time.Millisecond,
		MaxDelay:     100 * time.Millisecond,
		Multiplier:   2.0,
		RetryableErrors: func(err error) bool {
			return false // Don't retry any errors
		},
	}

	err := Retry(ctx, cfg, func() error {
		attempts++
		return errors.New("non-retryable")
	})

	if err == nil {
		t.Error("expected error")
	}
	if attempts != 1 {
		t.Errorf("expected 1 attempt, got %d", attempts)
	}
}

func TestRetryExponentialBackoff(t *testing.T) {
	ctx := context.Background()
	var timestamps []time.Time

	cfg := RetryConfig{
		MaxAttempts:  4,
		InitialDelay: 50 * time.Millisecond,
		MaxDelay:     500 * time.Millisecond,
		Multiplier:   2.0,
		RetryableErrors: func(err error) bool {
			return true
		},
	}

	_ = Retry(ctx, cfg, func() error {
		timestamps = append(timestamps, time.Now())
		return errors.New("fail")
	})

	// Verify delays are increasing
	for i := 1; i < len(timestamps); i++ {
		delay := timestamps[i].Sub(timestamps[i-1])
		multiplier := 1
		for j := 1; j < i; j++ {
			multiplier *= 2
		}
		expectedMin := time.Duration(float64(cfg.InitialDelay) * float64(multiplier))
		if expectedMin > cfg.MaxDelay {
			expectedMin = cfg.MaxDelay
		}

		// Allow some tolerance for timing
		if delay < expectedMin-20*time.Millisecond {
			t.Errorf("delay %d was %v, expected at least %v", i, delay, expectedMin)
		}
	}
}

// -----------------------------------------------------------------------------
// IsRetryableError Tests
// -----------------------------------------------------------------------------

func TestIsRetryableError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "nil error",
			err:      nil,
			expected: false,
		},
		{
			name:     "circuit open error",
			err:      ErrCircuitOpen,
			expected: false,
		},
		{
			name:     "regular error",
			err:      errors.New("some error"),
			expected: false,
		},
		{
			name:     "DNS error",
			err:      &net.DNSError{IsTimeout: true, IsTemporary: true},
			expected: true, // DNS errors with Temporary() true are retryable
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsRetryableError(tt.err)
			if result != tt.expected {
				t.Errorf("IsRetryableError(%v) = %v, expected %v", tt.err, result, tt.expected)
			}
		})
	}
}

func TestIsRetryableHTTPStatus(t *testing.T) {
	tests := []struct {
		status   int
		expected bool
	}{
		{200, false},
		{201, false},
		{400, false},
		{401, false},
		{403, false},
		{404, false},
		{429, true},
		{500, true},
		{502, true},
		{503, true},
		{504, true},
	}

	for _, tt := range tests {
		t.Run(string(rune(tt.status)), func(t *testing.T) {
			result := IsRetryableHTTPStatus(tt.status)
			if result != tt.expected {
				t.Errorf("IsRetryableHTTPStatus(%d) = %v, expected %v", tt.status, result, tt.expected)
			}
		})
	}
}

// -----------------------------------------------------------------------------
// ResilientClient Tests
// -----------------------------------------------------------------------------

func TestResilientClientExecute(t *testing.T) {
	cfg := ResilientClientConfig{
		Name: "test",
		CircuitBreaker: CircuitBreakerConfig{
			FailureThreshold: 5,
			SuccessThreshold: 1,
			Timeout:          time.Second,
		},
		Retry: RetryConfig{
			MaxAttempts:  3,
			InitialDelay: 10 * time.Millisecond,
			MaxDelay:     100 * time.Millisecond,
			Multiplier:   2.0,
			RetryableErrors: func(err error) bool {
				return true
			},
		},
		Logger: testLogger(),
	}

	client := NewResilientClient(cfg)
	ctx := context.Background()

	// Successful execution
	err := client.Execute(ctx, func() error {
		return nil
	})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	// Check state
	if client.CircuitState() != StateClosed {
		t.Errorf("expected circuit closed, got %s", client.CircuitState().String())
	}
}

func TestResilientClientExecuteWithResult(t *testing.T) {
	cfg := ResilientClientConfig{
		Name: "test",
		CircuitBreaker: CircuitBreakerConfig{
			FailureThreshold: 5,
			SuccessThreshold: 1,
			Timeout:          time.Second,
		},
		Retry: RetryConfig{
			MaxAttempts:     3,
			InitialDelay:    10 * time.Millisecond,
			MaxDelay:        100 * time.Millisecond,
			Multiplier:      2.0,
			RetryableErrors: func(err error) bool { return true },
		},
		Logger: testLogger(),
	}

	client := NewResilientClient(cfg)
	ctx := context.Background()

	result, err := ExecuteWithResult(ctx, client, func() (string, error) {
		return "hello", nil
	})

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if result != "hello" {
		t.Errorf("expected 'hello', got %s", result)
	}
}

func TestResilientClientIntegration(t *testing.T) {
	cfg := ResilientClientConfig{
		Name: "integration",
		CircuitBreaker: CircuitBreakerConfig{
			FailureThreshold: 3,
			SuccessThreshold: 1,
			Timeout:          50 * time.Millisecond,
		},
		Retry: RetryConfig{
			MaxAttempts:  2,
			InitialDelay: 10 * time.Millisecond,
			MaxDelay:     50 * time.Millisecond,
			Multiplier:   2.0,
			RetryableErrors: func(err error) bool {
				return true
			},
		},
		Logger: testLogger(),
	}

	client := NewResilientClient(cfg)
	ctx := context.Background()

	var attempts int32

	// Fail enough times to open the circuit
	for i := 0; i < 6; i++ { // 3 failures * 2 retries each = 6 total attempts to open
		_ = client.Execute(ctx, func() error {
			atomic.AddInt32(&attempts, 1)
			return errors.New("fail")
		})
	}

	// Circuit should be open now
	if client.CircuitState() != StateOpen {
		t.Errorf("expected circuit open, got %s", client.CircuitState().String())
	}

	// Requests should be rejected
	beforeAttempts := atomic.LoadInt32(&attempts)
	err := client.Execute(ctx, func() error {
		atomic.AddInt32(&attempts, 1)
		return nil
	})

	if !errors.Is(err, ErrCircuitOpen) {
		t.Errorf("expected ErrCircuitOpen, got %v", err)
	}

	afterAttempts := atomic.LoadInt32(&attempts)
	if afterAttempts != beforeAttempts {
		t.Error("function should not have been called when circuit is open")
	}

	// Wait for circuit to transition to half-open and allow requests
	require.Eventually(t, func() bool {
		err := client.Execute(ctx, func() error { return nil })
		return err == nil
	}, 1*time.Second, 10*time.Millisecond, "expected success in half-open")
}

// -----------------------------------------------------------------------------
// CircuitState Tests
// -----------------------------------------------------------------------------

func TestCircuitStateString(t *testing.T) {
	tests := []struct {
		state    CircuitState
		expected string
	}{
		{StateClosed, "closed"},
		{StateOpen, "open"},
		{StateHalfOpen, "half-open"},
		{CircuitState(99), "unknown"},
	}

	for _, tt := range tests {
		if tt.state.String() != tt.expected {
			t.Errorf("State(%d).String() = %s, expected %s", tt.state, tt.state.String(), tt.expected)
		}
	}
}
