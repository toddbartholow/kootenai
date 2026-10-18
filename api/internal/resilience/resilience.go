// Package resilience provides circuit breaker and retry patterns for external API calls
package resilience

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math/rand"
	"net"
	"net/http"
	"sync"
	"time"
)

// CircuitState represents the current state of a circuit breaker
type CircuitState int

const (
	StateClosed   CircuitState = iota // Normal operation, requests pass through
	StateOpen                         // Circuit is open, requests are rejected
	StateHalfOpen                     // Testing if service has recovered
)

func (s CircuitState) String() string {
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

// CircuitBreakerConfig configures the circuit breaker behavior
type CircuitBreakerConfig struct {
	// FailureThreshold is the number of failures before opening the circuit
	FailureThreshold int
	// SuccessThreshold is the number of successes in half-open state to close the circuit
	SuccessThreshold int
	// Timeout is how long the circuit stays open before transitioning to half-open
	Timeout time.Duration
	// OnStateChange is called when the circuit state changes
	OnStateChange func(from, to CircuitState)
}

// DefaultCircuitBreakerConfig returns sensible defaults
func DefaultCircuitBreakerConfig() CircuitBreakerConfig {
	return CircuitBreakerConfig{
		FailureThreshold: 5,
		SuccessThreshold: 2,
		Timeout:          30 * time.Second,
	}
}

// CircuitBreaker implements the circuit breaker pattern
type CircuitBreaker struct {
	name   string
	config CircuitBreakerConfig
	logger *slog.Logger

	mu              sync.RWMutex
	state           CircuitState
	failures        int
	successes       int
	lastFailure     time.Time
	lastStateChange time.Time
}

// NewCircuitBreaker creates a new circuit breaker
func NewCircuitBreaker(name string, config CircuitBreakerConfig, logger *slog.Logger) *CircuitBreaker {
	if logger == nil {
		logger = slog.Default()
	}
	if config.FailureThreshold == 0 {
		config.FailureThreshold = 5
	}
	if config.SuccessThreshold == 0 {
		config.SuccessThreshold = 2
	}
	if config.Timeout == 0 {
		config.Timeout = 30 * time.Second
	}

	return &CircuitBreaker{
		name:            name,
		config:          config,
		logger:          logger,
		state:           StateClosed,
		lastStateChange: time.Now(),
	}
}

// ErrCircuitOpen is returned when the circuit is open
var ErrCircuitOpen = errors.New("circuit breaker is open")

// Execute runs the given function through the circuit breaker
func (cb *CircuitBreaker) Execute(ctx context.Context, fn func() error) error {
	if !cb.allowRequest() {
		cb.logger.Warn("circuit breaker rejected request",
			slog.String("name", cb.name),
			slog.String("state", cb.state.String()),
		)
		return ErrCircuitOpen
	}

	err := fn()

	if err != nil {
		cb.recordFailure()
		return err
	}

	cb.recordSuccess()
	return nil
}

// allowRequest determines if a request should be allowed through
func (cb *CircuitBreaker) allowRequest() bool {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	switch cb.state {
	case StateClosed:
		return true
	case StateOpen:
		// Check if timeout has passed to transition to half-open
		if time.Since(cb.lastFailure) > cb.config.Timeout {
			cb.transitionTo(StateHalfOpen)
			return true
		}
		return false
	case StateHalfOpen:
		// Allow limited requests in half-open state
		return true
	default:
		return false
	}
}

// recordSuccess records a successful request
func (cb *CircuitBreaker) recordSuccess() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	switch cb.state {
	case StateHalfOpen:
		cb.successes++
		if cb.successes >= cb.config.SuccessThreshold {
			cb.transitionTo(StateClosed)
		}
	case StateClosed:
		// Reset failure count on success
		cb.failures = 0
	}
}

// recordFailure records a failed request
func (cb *CircuitBreaker) recordFailure() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.failures++
	cb.lastFailure = time.Now()

	switch cb.state {
	case StateClosed:
		if cb.failures >= cb.config.FailureThreshold {
			cb.transitionTo(StateOpen)
		}
	case StateHalfOpen:
		// Any failure in half-open state opens the circuit
		cb.transitionTo(StateOpen)
	}
}

// transitionTo changes the circuit state (caller must hold lock)
func (cb *CircuitBreaker) transitionTo(newState CircuitState) {
	if cb.state == newState {
		return
	}

	oldState := cb.state
	cb.state = newState
	cb.lastStateChange = time.Now()
	cb.failures = 0
	cb.successes = 0

	cb.logger.Info("circuit breaker state changed",
		slog.String("name", cb.name),
		slog.String("from", oldState.String()),
		slog.String("to", newState.String()),
	)

	if cb.config.OnStateChange != nil {
		go cb.config.OnStateChange(oldState, newState)
	}
}

// State returns the current circuit state
func (cb *CircuitBreaker) State() CircuitState {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.state
}

// Stats returns current circuit breaker statistics
func (cb *CircuitBreaker) Stats() CircuitStats {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return CircuitStats{
		Name:            cb.name,
		State:           cb.state.String(),
		Failures:        cb.failures,
		Successes:       cb.successes,
		LastFailure:     cb.lastFailure,
		LastStateChange: cb.lastStateChange,
	}
}

// CircuitStats contains circuit breaker statistics
type CircuitStats struct {
	Name            string    `json:"name"`
	State           string    `json:"state"`
	Failures        int       `json:"failures"`
	Successes       int       `json:"successes"`
	LastFailure     time.Time `json:"last_failure,omitempty"`
	LastStateChange time.Time `json:"last_state_change"`
}

// RetryConfig configures retry behavior
type RetryConfig struct {
	// MaxAttempts is the maximum number of attempts (including the initial attempt)
	MaxAttempts int
	// InitialDelay is the initial delay before the first retry
	InitialDelay time.Duration
	// MaxDelay is the maximum delay between retries
	MaxDelay time.Duration
	// Multiplier is the factor by which the delay increases after each attempt
	Multiplier float64
	// RetryableErrors is a function that determines if an error is retryable
	RetryableErrors func(error) bool
}

// DefaultRetryConfig returns sensible defaults for retry configuration
func DefaultRetryConfig() RetryConfig {
	return RetryConfig{
		MaxAttempts:     3,
		InitialDelay:    100 * time.Millisecond,
		MaxDelay:        5 * time.Second,
		Multiplier:      2.0,
		RetryableErrors: IsRetryableError,
	}
}

// Retry executes a function with exponential backoff retry
func Retry(ctx context.Context, config RetryConfig, fn func() error) error {
	if config.MaxAttempts <= 0 {
		config.MaxAttempts = 3
	}
	if config.InitialDelay <= 0 {
		config.InitialDelay = 100 * time.Millisecond
	}
	if config.MaxDelay <= 0 {
		config.MaxDelay = 5 * time.Second
	}
	if config.Multiplier <= 0 {
		config.Multiplier = 2.0
	}
	if config.RetryableErrors == nil {
		config.RetryableErrors = IsRetryableError
	}

	var lastErr error
	delay := config.InitialDelay

	for attempt := 1; attempt <= config.MaxAttempts; attempt++ {
		err := fn()
		if err == nil {
			return nil
		}

		lastErr = err

		// Don't retry on context cancellation
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return err
		}

		// Check if error is retryable
		if !config.RetryableErrors(err) {
			return err
		}

		// Don't wait after the last attempt
		if attempt == config.MaxAttempts {
			break
		}

		// Wait with exponential backoff
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(delay):
		}

		// Calculate next delay with exponential backoff
		delay = time.Duration(float64(delay) * config.Multiplier)
		if delay > config.MaxDelay {
			delay = config.MaxDelay
		}
	}

	return fmt.Errorf("max retries (%d) exceeded: %w", config.MaxAttempts, lastErr)
}

// IsRetryableError determines if an error should be retried
func IsRetryableError(err error) bool {
	if err == nil {
		return false
	}

	// Check for network errors
	var netErr net.Error
	if errors.As(err, &netErr) {
		return netErr.Timeout()
	}

	// Check for specific error types
	var dnsErr *net.DNSError
	if errors.As(err, &dnsErr) {
		return dnsErr.Temporary()
	}

	// Check for connection refused
	var opErr *net.OpError
	if errors.As(err, &opErr) {
		return true
	}

	// Check for circuit breaker open (don't retry)
	if errors.Is(err, ErrCircuitOpen) {
		return false
	}

	return false
}

// IsRetryableHTTPStatus determines if an HTTP status code should trigger a retry
func IsRetryableHTTPStatus(statusCode int) bool {
	switch statusCode {
	case http.StatusTooManyRequests, // 429
		http.StatusServiceUnavailable, // 503
		http.StatusGatewayTimeout,     // 504
		http.StatusBadGateway:         // 502
		return true
	default:
		return statusCode >= 500
	}
}

// ResilientClient wraps retry and circuit breaker for external API calls
type ResilientClient struct {
	circuitBreaker *CircuitBreaker
	retryConfig    RetryConfig
	logger         *slog.Logger
}

// ResilientClientConfig configures a resilient client
type ResilientClientConfig struct {
	Name           string
	CircuitBreaker CircuitBreakerConfig
	Retry          RetryConfig
	Logger         *slog.Logger
}

// NewResilientClient creates a new resilient client
func NewResilientClient(config ResilientClientConfig) *ResilientClient {
	if config.Logger == nil {
		config.Logger = slog.Default()
	}

	return &ResilientClient{
		circuitBreaker: NewCircuitBreaker(config.Name, config.CircuitBreaker, config.Logger),
		retryConfig:    config.Retry,
		logger:         config.Logger,
	}
}

// Execute runs a function with retry and circuit breaker protection
func (rc *ResilientClient) Execute(ctx context.Context, fn func() error) error {
	return rc.circuitBreaker.Execute(ctx, func() error {
		return Retry(ctx, rc.retryConfig, fn)
	})
}

// ExecuteWithResult runs a function that returns a result with retry and circuit breaker protection
func ExecuteWithResult[T any](ctx context.Context, rc *ResilientClient, fn func() (T, error)) (T, error) {
	var result T
	var lastErr error

	err := rc.Execute(ctx, func() error {
		r, err := fn()
		if err != nil {
			lastErr = err
			return err
		}
		result = r
		return nil
	})

	if err != nil {
		return result, err
	}

	return result, lastErr
}

// CircuitState returns the current circuit breaker state
func (rc *ResilientClient) CircuitState() CircuitState {
	return rc.circuitBreaker.State()
}

// Stats returns the circuit breaker statistics
func (rc *ResilientClient) Stats() CircuitStats {
	return rc.circuitBreaker.Stats()
}

// HTTPRetryTransport wraps an http.RoundTripper with retry logic
type HTTPRetryTransport struct {
	Base   http.RoundTripper
	Config RetryConfig
	Logger *slog.Logger
}

// RoundTrip implements http.RoundTripper with retry logic
func (t *HTTPRetryTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if t.Base == nil {
		t.Base = http.DefaultTransport
	}

	var lastResp *http.Response
	var lastErr error

	config := t.Config
	if config.MaxAttempts <= 0 {
		config = DefaultRetryConfig()
	}

	// Override retryable check to include HTTP status codes
	originalRetryable := config.RetryableErrors
	if originalRetryable == nil {
		originalRetryable = IsRetryableError
	}

	delay := config.InitialDelay

	for attempt := 1; attempt <= config.MaxAttempts; attempt++ {
		// Clone the request for retry (body must be re-readable)
		reqCopy := req.Clone(req.Context())

		resp, err := t.Base.RoundTrip(reqCopy)

		if err == nil {
			// Check if we should retry based on status code
			if IsRetryableHTTPStatus(resp.StatusCode) && attempt < config.MaxAttempts {
				// Close response body before retry
				resp.Body.Close()
				lastResp = nil
				lastErr = fmt.Errorf("retryable HTTP status: %d", resp.StatusCode)
			} else {
				return resp, nil
			}
		} else {
			lastErr = err

			// Don't retry on context cancellation
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				return nil, err
			}

			// Check if error is retryable
			if !originalRetryable(err) {
				return nil, err
			}
		}

		// Don't wait after the last attempt
		if attempt == config.MaxAttempts {
			break
		}

		if t.Logger != nil {
			t.Logger.Debug("retrying HTTP request",
				slog.Int("attempt", attempt),
				slog.Duration("delay", delay),
				slog.Any("error", lastErr),
			)
		}

		// Wait with exponential backoff
		select {
		case <-req.Context().Done():
			return nil, req.Context().Err()
		case <-time.After(delay):
		}

		// Calculate next delay with exponential backoff
		delay = time.Duration(float64(delay) * config.Multiplier)
		if delay > config.MaxDelay {
			delay = config.MaxDelay
		}
	}

	if lastResp != nil {
		return lastResp, nil
	}
	return nil, fmt.Errorf("max retries (%d) exceeded: %w", config.MaxAttempts, lastErr)
}

// Jitter adds randomness to delay to prevent thundering herd
func Jitter(delay time.Duration, factor float64) time.Duration {
	if factor <= 0 || factor > 1 {
		factor = 0.1
	}
	jitter := float64(delay) * factor
	// Add or subtract up to jitter amount
	// #nosec G404 -- math/rand is acceptable for jitter calculation as this is for
	// load distribution timing, not for any security-sensitive operation.
	delta := (rand.Float64() - 0.5) * 2 * jitter
	return delay + time.Duration(delta)
}
