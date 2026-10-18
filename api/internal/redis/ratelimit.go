package redis

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// RateLimitResult contains the result of a rate limit check
type RateLimitResult struct {
	Allowed   bool
	Remaining int64
	ResetAt   time.Time
	RetryIn   time.Duration
}

// RateLimiter provides rate limiting functionality
type RateLimiter struct {
	client *Client
	logger *slog.Logger
}

// NewRateLimiter creates a new rate limiter
func NewRateLimiter(client *Client, logger *slog.Logger) *RateLimiter {
	if logger == nil {
		logger = slog.Default()
	}
	return &RateLimiter{
		client: client,
		logger: logger,
	}
}

// Allow checks if a request is allowed under the rate limit
// Uses a sliding window algorithm
func (rl *RateLimiter) Allow(ctx context.Context, key string, limit int64, window time.Duration) (*RateLimitResult, error) {
	now := time.Now()
	windowStart := now.Add(-window).UnixMilli()
	fullKey := rl.client.Key(PrefixRateLimit + key)

	// Lua script for sliding window rate limiting
	script := `
		local key = KEYS[1]
		local now = tonumber(ARGV[1])
		local window_start = tonumber(ARGV[2])
		local limit = tonumber(ARGV[3])
		local window_ms = tonumber(ARGV[4])

		-- Remove old entries
		redis.call("zremrangebyscore", key, "-inf", window_start)

		-- Count current requests
		local count = redis.call("zcard", key)

		if count < limit then
			-- Allow request, add to set
			redis.call("zadd", key, now, now .. "-" .. math.random(1000000))
			redis.call("pexpire", key, window_ms)
			return {1, limit - count - 1}
		else
			-- Get oldest entry to calculate retry time
			local oldest = redis.call("zrange", key, 0, 0, "WITHSCORES")
			if oldest[2] then
				return {0, 0, oldest[2]}
			end
			return {0, 0, now}
		end
	`

	result, err := rl.client.rdb.Eval(ctx, script, []string{fullKey},
		now.UnixMilli(),
		windowStart,
		limit,
		int64(window/time.Millisecond),
	).Slice()

	if err != nil {
		return nil, fmt.Errorf("rate limit check failed: %w", err)
	}

	allowed, _ := result[0].(int64)
	remaining, _ := result[1].(int64)

	res := &RateLimitResult{
		Allowed:   allowed == 1,
		Remaining: remaining,
		ResetAt:   now.Add(window),
	}

	if !res.Allowed && len(result) > 2 {
		oldestMs, _ := result[2].(int64)
		retryAt := time.UnixMilli(oldestMs).Add(window)
		res.RetryIn = retryAt.Sub(now)
	}

	return res, nil
}

// AllowN checks if N requests are allowed
func (rl *RateLimiter) AllowN(ctx context.Context, key string, n int64, limit int64, window time.Duration) (*RateLimitResult, error) {
	now := time.Now()
	windowStart := now.Add(-window).UnixMilli()
	fullKey := rl.client.Key(PrefixRateLimit + key)

	script := `
		local key = KEYS[1]
		local now = tonumber(ARGV[1])
		local window_start = tonumber(ARGV[2])
		local limit = tonumber(ARGV[3])
		local window_ms = tonumber(ARGV[4])
		local n = tonumber(ARGV[5])

		-- Remove old entries
		redis.call("zremrangebyscore", key, "-inf", window_start)

		-- Count current requests
		local count = redis.call("zcard", key)

		if count + n <= limit then
			-- Allow requests, add to set
			for i = 1, n do
				redis.call("zadd", key, now, now .. "-" .. i .. "-" .. math.random(1000000))
			end
			redis.call("pexpire", key, window_ms)
			return {1, limit - count - n}
		else
			return {0, limit - count}
		end
	`

	result, err := rl.client.rdb.Eval(ctx, script, []string{fullKey},
		now.UnixMilli(),
		windowStart,
		limit,
		int64(window/time.Millisecond),
		n,
	).Slice()

	if err != nil {
		return nil, fmt.Errorf("rate limit check failed: %w", err)
	}

	allowed, _ := result[0].(int64)
	remaining, _ := result[1].(int64)

	return &RateLimitResult{
		Allowed:   allowed == 1,
		Remaining: remaining,
		ResetAt:   now.Add(window),
	}, nil
}

// Reset clears rate limit for a key
func (rl *RateLimiter) Reset(ctx context.Context, key string) error {
	return rl.client.Delete(ctx, PrefixRateLimit+key)
}

// RateLimitConfig defines rate limiting configuration
type RateLimitConfig struct {
	// Requests per window
	Limit int64
	// Time window
	Window time.Duration
	// Key function to extract rate limit key from request
	KeyFunc func(r *http.Request) string
	// Skip function to skip rate limiting for certain requests
	SkipFunc func(r *http.Request) bool
	// Error handler for rate limit exceeded
	OnLimitExceeded func(w http.ResponseWriter, r *http.Request, result *RateLimitResult)
}

// DefaultRateLimitConfig returns default config (100 requests per minute by IP)
func DefaultRateLimitConfig() RateLimitConfig {
	return RateLimitConfig{
		Limit:  100,
		Window: time.Minute,
		KeyFunc: func(r *http.Request) string {
			return "ip:" + GetRealIP(r)
		},
		OnLimitExceeded: func(w http.ResponseWriter, r *http.Request, result *RateLimitResult) {
			w.Header().Set("X-RateLimit-Limit", strconv.FormatInt(result.Remaining+1, 10))
			w.Header().Set("X-RateLimit-Remaining", "0")
			w.Header().Set("X-RateLimit-Reset", strconv.FormatInt(result.ResetAt.Unix(), 10))
			w.Header().Set("Retry-After", strconv.FormatInt(int64(result.RetryIn.Seconds()), 10))
			http.Error(w, "Rate limit exceeded", http.StatusTooManyRequests)
		},
	}
}

// Middleware returns an HTTP middleware for rate limiting
func (rl *RateLimiter) Middleware(cfg RateLimitConfig) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Skip if configured
			if cfg.SkipFunc != nil && cfg.SkipFunc(r) {
				next.ServeHTTP(w, r)
				return
			}

			// Get rate limit key
			key := cfg.KeyFunc(r)

			// Check rate limit
			result, err := rl.Allow(r.Context(), key, cfg.Limit, cfg.Window)
			if err != nil {
				rl.logger.Error("rate limit check failed",
					slog.String("key", key),
					slog.Any("error", err),
				)
				// On error, allow the request but log it
				next.ServeHTTP(w, r)
				return
			}

			// Set rate limit headers
			w.Header().Set("X-RateLimit-Limit", strconv.FormatInt(cfg.Limit, 10))
			w.Header().Set("X-RateLimit-Remaining", strconv.FormatInt(result.Remaining, 10))
			w.Header().Set("X-RateLimit-Reset", strconv.FormatInt(result.ResetAt.Unix(), 10))

			if !result.Allowed {
				rl.logger.Info("rate limit exceeded",
					slog.String("key", key),
					slog.Duration("retry_in", result.RetryIn),
				)
				cfg.OnLimitExceeded(w, r, result)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// --- Specialized Rate Limiters ---

// APIRateLimiter provides rate limiting for API endpoints
type APIRateLimiter struct {
	limiter *RateLimiter
}

// NewAPIRateLimiter creates an API rate limiter
func NewAPIRateLimiter(client *Client, logger *slog.Logger) *APIRateLimiter {
	return &APIRateLimiter{
		limiter: NewRateLimiter(client, logger),
	}
}

// PerIPMiddleware returns middleware that limits by IP address
func (arl *APIRateLimiter) PerIPMiddleware(limit int64, window time.Duration) func(http.Handler) http.Handler {
	cfg := DefaultRateLimitConfig()
	cfg.Limit = limit
	cfg.Window = window
	return arl.limiter.Middleware(cfg)
}

// PerUserMiddleware returns middleware that limits by user ID
func (arl *APIRateLimiter) PerUserMiddleware(limit int64, window time.Duration, getUserID func(r *http.Request) string) func(http.Handler) http.Handler {
	cfg := DefaultRateLimitConfig()
	cfg.Limit = limit
	cfg.Window = window
	cfg.KeyFunc = func(r *http.Request) string {
		userID := getUserID(r)
		if userID == "" {
			return "anonymous:" + GetRealIP(r)
		}
		return "user:" + userID
	}
	return arl.limiter.Middleware(cfg)
}

// PerEndpointMiddleware returns middleware that limits by endpoint + IP
func (arl *APIRateLimiter) PerEndpointMiddleware(limit int64, window time.Duration) func(http.Handler) http.Handler {
	cfg := DefaultRateLimitConfig()
	cfg.Limit = limit
	cfg.Window = window
	cfg.KeyFunc = func(r *http.Request) string {
		return "endpoint:" + r.Method + ":" + r.URL.Path + ":" + GetRealIP(r)
	}
	return arl.limiter.Middleware(cfg)
}

// PodOperationRateLimiter limits pod-related operations
type PodOperationRateLimiter struct {
	limiter *RateLimiter
}

// NewPodOperationRateLimiter creates a pod operation rate limiter
func NewPodOperationRateLimiter(client *Client, logger *slog.Logger) *PodOperationRateLimiter {
	return &PodOperationRateLimiter{
		limiter: NewRateLimiter(client, logger),
	}
}

// AllowPodCreate checks if a user can create another pod
func (prl *PodOperationRateLimiter) AllowPodCreate(ctx context.Context, userID string, maxPods int64, window time.Duration) (*RateLimitResult, error) {
	return prl.limiter.Allow(ctx, "pod:create:"+userID, maxPods, window)
}

// AllowSnapshot checks if a user can take another snapshot
func (prl *PodOperationRateLimiter) AllowSnapshot(ctx context.Context, userID string, limit int64, window time.Duration) (*RateLimitResult, error) {
	return prl.limiter.Allow(ctx, "pod:snapshot:"+userID, limit, window)
}

// AllowRevert checks if a user can revert again
func (prl *PodOperationRateLimiter) AllowRevert(ctx context.Context, podID string, limit int64, window time.Duration) (*RateLimitResult, error) {
	return prl.limiter.Allow(ctx, "pod:revert:"+podID, limit, window)
}

// AuthRateLimiter provides rate limiting for authentication endpoints
// These endpoints are particularly sensitive and need stricter limits to prevent:
// - Brute force password attacks
// - Email bombing (password reset)
// - Account enumeration
type AuthRateLimiter struct {
	limiter *RateLimiter
	logger  *slog.Logger
}

// NewAuthRateLimiter creates a new auth rate limiter
func NewAuthRateLimiter(client *Client, logger *slog.Logger) *AuthRateLimiter {
	if logger == nil {
		logger = slog.Default()
	}
	return &AuthRateLimiter{
		limiter: NewRateLimiter(client, logger),
		logger:  logger,
	}
}

// AuthRateLimitConfig holds configuration for auth rate limiting
type AuthRateLimitConfig struct {
	// Login attempts per IP per window (default: 5 per minute)
	LoginAttemptsPerIP int64
	LoginWindowPerIP   time.Duration

	// Login attempts per email per window (default: 10 per hour)
	LoginAttemptsPerEmail int64
	LoginWindowPerEmail   time.Duration

	// Password reset requests per IP per window (default: 3 per hour)
	PasswordResetPerIP       int64
	PasswordResetWindowPerIP time.Duration

	// Password reset requests per email per window (default: 3 per hour)
	PasswordResetPerEmail       int64
	PasswordResetWindowPerEmail time.Duration

	// Token refresh per user per window (default: 30 per hour)
	TokenRefreshPerUser int64
	TokenRefreshWindow  time.Duration
}

// DefaultAuthRateLimitConfig returns sensible defaults for auth rate limiting
func DefaultAuthRateLimitConfig() AuthRateLimitConfig {
	return AuthRateLimitConfig{
		LoginAttemptsPerIP:          5,
		LoginWindowPerIP:            time.Minute,
		LoginAttemptsPerEmail:       10,
		LoginWindowPerEmail:         time.Hour,
		PasswordResetPerIP:          3,
		PasswordResetWindowPerIP:    time.Hour,
		PasswordResetPerEmail:       3,
		PasswordResetWindowPerEmail: time.Hour,
		TokenRefreshPerUser:         30,
		TokenRefreshWindow:          time.Hour,
	}
}

// hashForLog creates a privacy-preserving hash of sensitive data for logging
// Returns the first 12 characters of the SHA-256 hash (enough for correlation)
func hashForLog(data string) string {
	if data == "" {
		return ""
	}
	data = strings.ToLower(strings.TrimSpace(data))
	hash := sha256.Sum256([]byte(data))
	return hex.EncodeToString(hash[:])[:12]
}

// AllowLogin checks if a login attempt is allowed
// Applies both per-IP and per-email limits
func (arl *AuthRateLimiter) AllowLogin(ctx context.Context, ip, email string, cfg AuthRateLimitConfig) (*RateLimitResult, error) {
	// Check per-IP limit first (cheaper, catches distributed attacks)
	ipResult, err := arl.limiter.Allow(ctx, "auth:login:ip:"+ip, cfg.LoginAttemptsPerIP, cfg.LoginWindowPerIP)
	if err != nil {
		arl.logger.Error("auth rate limit check failed (IP)",
			slog.String("ip", ip),
			slog.Any("error", err),
		)
		// On error, allow but log
		return &RateLimitResult{Allowed: true}, nil
	}
	if !ipResult.Allowed {
		arl.logger.Warn("login rate limit exceeded (IP)",
			slog.String("ip", ip),
			slog.Duration("retry_in", ipResult.RetryIn),
		)
		return ipResult, nil
	}

	// Check per-email limit (prevents targeted attacks against specific accounts)
	if email != "" {
		emailResult, err := arl.limiter.Allow(ctx, "auth:login:email:"+email, cfg.LoginAttemptsPerEmail, cfg.LoginWindowPerEmail)
		if err != nil {
			arl.logger.Error("auth rate limit check failed (email)",
				slog.String("email_hash", hashForLog(email)), // Hash for privacy
				slog.Any("error", err),
			)
			return ipResult, nil
		}
		if !emailResult.Allowed {
			arl.logger.Warn("login rate limit exceeded (email)",
				slog.String("email_hash", hashForLog(email)), // Hash for privacy
				slog.Duration("retry_in", emailResult.RetryIn),
			)
			return emailResult, nil
		}
	}

	return ipResult, nil
}

// AllowPasswordReset checks if a password reset request is allowed
func (arl *AuthRateLimiter) AllowPasswordReset(ctx context.Context, ip, email string, cfg AuthRateLimitConfig) (*RateLimitResult, error) {
	// Check per-IP limit first
	ipResult, err := arl.limiter.Allow(ctx, "auth:reset:ip:"+ip, cfg.PasswordResetPerIP, cfg.PasswordResetWindowPerIP)
	if err != nil {
		arl.logger.Error("password reset rate limit check failed (IP)",
			slog.String("ip", ip),
			slog.Any("error", err),
		)
		return &RateLimitResult{Allowed: true}, nil
	}
	if !ipResult.Allowed {
		arl.logger.Warn("password reset rate limit exceeded (IP)",
			slog.String("ip", ip),
			slog.Duration("retry_in", ipResult.RetryIn),
		)
		return ipResult, nil
	}

	// Check per-email limit (prevents email bombing)
	if email != "" {
		emailResult, err := arl.limiter.Allow(ctx, "auth:reset:email:"+email, cfg.PasswordResetPerEmail, cfg.PasswordResetWindowPerEmail)
		if err != nil {
			arl.logger.Error("password reset rate limit check failed (email)",
				slog.String("email_hash", hashForLog(email)), // Hash for privacy
				slog.Any("error", err),
			)
			return ipResult, nil
		}
		if !emailResult.Allowed {
			arl.logger.Warn("password reset rate limit exceeded (email)",
				slog.String("email_hash", hashForLog(email)), // Hash for privacy
				slog.Duration("retry_in", emailResult.RetryIn),
			)
			return emailResult, nil
		}
	}

	return ipResult, nil
}

// AllowTokenRefresh checks if a token refresh is allowed
func (arl *AuthRateLimiter) AllowTokenRefresh(ctx context.Context, userID string, cfg AuthRateLimitConfig) (*RateLimitResult, error) {
	result, err := arl.limiter.Allow(ctx, "auth:refresh:"+userID, cfg.TokenRefreshPerUser, cfg.TokenRefreshWindow)
	if err != nil {
		arl.logger.Error("token refresh rate limit check failed",
			slog.String("user_id", userID),
			slog.Any("error", err),
		)
		return &RateLimitResult{Allowed: true}, nil
	}
	if !result.Allowed {
		arl.logger.Warn("token refresh rate limit exceeded",
			slog.String("user_id", userID),
			slog.Duration("retry_in", result.RetryIn),
		)
	}
	return result, nil
}

// GetRealIP extracts the real client IP address from a request.
// It prefers X-Real-IP (set by nginx, harder to spoof) over RemoteAddr.
// X-Forwarded-For is NOT used as it can be trivially spoofed by clients.
func GetRealIP(r *http.Request) string {
	// Prefer X-Real-IP header (set by nginx, single value)
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return strings.TrimSpace(xri)
	}

	// Fallback to RemoteAddr (strip port if present)
	addr := r.RemoteAddr
	if idx := strings.LastIndex(addr, ":"); idx != -1 {
		// Check if this is an IPv6 address with brackets
		if strings.Contains(addr, "[") {
			// IPv6: [::1]:8080 -> extract IP without brackets
			if bracketEnd := strings.Index(addr, "]"); bracketEnd != -1 {
				return addr[1:bracketEnd]
			}
		}
		return addr[:idx]
	}
	return addr
}

// ExtractEmailFromBody reads the request body, extracts the email field,
// and restores the body for subsequent handlers to use.
func ExtractEmailFromBody(r *http.Request) string {
	if r.Body == nil {
		return ""
	}

	// Read the body
	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		return ""
	}

	// Restore the body for the next handler
	r.Body.Close()
	r.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

	// Parse email from JSON
	var body struct {
		Email    string `json:"email"`
		Username string `json:"username"` // Fallback to username if email not present
	}
	if err := json.Unmarshal(bodyBytes, &body); err != nil {
		return ""
	}

	if body.Email != "" {
		return body.Email
	}
	return body.Username
}

// LoginMiddleware returns middleware for rate limiting login endpoints
func (arl *AuthRateLimiter) LoginMiddleware(cfg AuthRateLimitConfig, getEmail func(r *http.Request) string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := GetRealIP(r)
			email := ""
			if getEmail != nil {
				email = getEmail(r)
			} else {
				// Default: extract email from request body
				email = ExtractEmailFromBody(r)
			}

			result, _ := arl.AllowLogin(r.Context(), ip, email, cfg)
			if !result.Allowed {
				w.Header().Set("X-RateLimit-Limit", strconv.FormatInt(cfg.LoginAttemptsPerIP, 10))
				w.Header().Set("X-RateLimit-Remaining", "0")
				w.Header().Set("X-RateLimit-Reset", strconv.FormatInt(result.ResetAt.Unix(), 10))
				w.Header().Set("Retry-After", strconv.FormatInt(int64(result.RetryIn.Seconds()), 10))
				http.Error(w, "Too many login attempts. Please try again later.", http.StatusTooManyRequests)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// PasswordResetMiddleware returns middleware for rate limiting password reset endpoints
func (arl *AuthRateLimiter) PasswordResetMiddleware(cfg AuthRateLimitConfig, getEmail func(r *http.Request) string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := GetRealIP(r)
			email := ""
			if getEmail != nil {
				email = getEmail(r)
			} else {
				// Default: extract email from request body
				email = ExtractEmailFromBody(r)
			}

			result, _ := arl.AllowPasswordReset(r.Context(), ip, email, cfg)
			if !result.Allowed {
				w.Header().Set("X-RateLimit-Limit", strconv.FormatInt(cfg.PasswordResetPerIP, 10))
				w.Header().Set("X-RateLimit-Remaining", "0")
				w.Header().Set("X-RateLimit-Reset", strconv.FormatInt(result.ResetAt.Unix(), 10))
				w.Header().Set("Retry-After", strconv.FormatInt(int64(result.RetryIn.Seconds()), 10))
				http.Error(w, "Too many password reset requests. Please try again later.", http.StatusTooManyRequests)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
