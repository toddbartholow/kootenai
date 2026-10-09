// Package wazuh provides Wazuh integration for the Kootenai platform
package wazuh

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

// SecurityConfig holds security configuration for the Wazuh webhook
type SecurityConfig struct {
	// WebhookSecret for validating incoming webhooks (required in production)
	WebhookSecret string `yaml:"webhook_secret"`

	// AllowedIPs is a list of IP addresses/CIDRs allowed to send webhook requests
	// If empty, IP validation is disabled (not recommended for production)
	AllowedIPs []string `yaml:"allowed_ips"`

	// RateLimitPerMinute limits webhook requests per source IP
	RateLimitPerMinute int `yaml:"rate_limit_per_minute"`

	// MaxTimestampDrift is the maximum allowed difference between event timestamp and server time
	MaxTimestampDrift time.Duration `yaml:"max_timestamp_drift"`

	// RequireSignature enforces webhook signature validation (should be true in production)
	RequireSignature bool `yaml:"require_signature"`
}

// DefaultSecurityConfig returns secure default settings
func DefaultSecurityConfig() SecurityConfig {
	return SecurityConfig{
		AllowedIPs:         []string{}, // Empty = allow all (configure in production!)
		RateLimitPerMinute: 100,        // 100 requests per minute per IP
		MaxTimestampDrift:  5 * time.Minute,
		RequireSignature:   false, // Set to true once webhook secret is configured
	}
}

// WebhookValidator validates incoming Wazuh webhook requests
type WebhookValidator struct {
	config       SecurityConfig
	logger       *slog.Logger
	allowedNets  []*net.IPNet
	allowedAddrs []net.IP
	rateLimiter  *webhookRateLimiter
}

// NewWebhookValidator creates a new webhook validator. The provided context
// governs the lifetime of background goroutines (e.g. rate limiter cleanup);
// cancelling it stops them and avoids leaks at shutdown.
func NewWebhookValidator(ctx context.Context, config SecurityConfig, logger *slog.Logger) (*WebhookValidator, error) {
	v := &WebhookValidator{
		config: config,
		logger: logger,
	}

	// Parse allowed IPs/CIDRs
	for _, ipStr := range config.AllowedIPs {
		ipStr = strings.TrimSpace(ipStr)
		if ipStr == "" {
			continue
		}

		// Try parsing as CIDR first
		if strings.Contains(ipStr, "/") {
			_, network, err := net.ParseCIDR(ipStr)
			if err != nil {
				return nil, fmt.Errorf("invalid CIDR %q: %w", ipStr, err)
			}
			v.allowedNets = append(v.allowedNets, network)
		} else {
			// Parse as single IP
			ip := net.ParseIP(ipStr)
			if ip == nil {
				return nil, fmt.Errorf("invalid IP address %q", ipStr)
			}
			v.allowedAddrs = append(v.allowedAddrs, ip)
		}
	}

	// Initialize rate limiter
	if config.RateLimitPerMinute > 0 {
		v.rateLimiter = newWebhookRateLimiter(ctx, config.RateLimitPerMinute, time.Minute)
	}

	logger.Info("Webhook validator initialized",
		"allowed_ips", len(config.AllowedIPs),
		"rate_limit", config.RateLimitPerMinute,
		"require_signature", config.RequireSignature,
	)

	return v, nil
}

// ValidateRequest validates an incoming webhook request.
// The body parameter is required for HMAC signature verification.
// Returns an error if the request should be rejected.
func (v *WebhookValidator) ValidateRequest(r *http.Request, body []byte) error {
	clientIP := v.getClientIP(r)

	// 1. Check IP allowlist (if configured)
	if len(v.allowedNets) > 0 || len(v.allowedAddrs) > 0 {
		if !v.isIPAllowed(clientIP) {
			v.logger.Warn("Webhook request from unauthorized IP",
				"ip", clientIP,
				"path", r.URL.Path,
			)
			return fmt.Errorf("unauthorized source IP: %s", clientIP)
		}
	}

	// 2. Check rate limit
	if v.rateLimiter != nil {
		if !v.rateLimiter.Allow(clientIP) {
			v.logger.Warn("Webhook rate limit exceeded",
				"ip", clientIP,
				"path", r.URL.Path,
			)
			return fmt.Errorf("rate limit exceeded for IP: %s", clientIP)
		}
	}

	// 3. Validate webhook signature/secret
	if v.config.RequireSignature || v.config.WebhookSecret != "" {
		if err := v.validateSignature(r, body); err != nil {
			v.logger.Warn("Webhook signature validation failed",
				"ip", clientIP,
				"error", err,
			)
			return err
		}
	}

	return nil
}

// ValidateTimestamp checks if an event timestamp is within acceptable drift
func (v *WebhookValidator) ValidateTimestamp(eventTime time.Time) error {
	if v.config.MaxTimestampDrift == 0 {
		return nil // Timestamp validation disabled
	}

	drift := time.Since(eventTime)
	if drift < 0 {
		drift = -drift // Absolute value
	}

	if drift > v.config.MaxTimestampDrift {
		return fmt.Errorf("event timestamp drift too large: %v (max: %v)", drift, v.config.MaxTimestampDrift)
	}

	return nil
}

// validateSignature checks the webhook signature header
func (v *WebhookValidator) validateSignature(r *http.Request, body []byte) error {
	if v.config.WebhookSecret == "" {
		if v.config.RequireSignature {
			return fmt.Errorf("webhook secret not configured but signature required")
		}
		return nil
	}

	// Check for HMAC signature first (preferred, more secure)
	signature := r.Header.Get("X-Wazuh-Signature")
	if signature != "" {
		expected := ComputeHMAC(body, v.config.WebhookSecret)
		if subtle.ConstantTimeCompare([]byte(signature), []byte(expected)) == 1 {
			return nil
		}
		return fmt.Errorf("invalid HMAC signature")
	}

	// Fall back to shared secret header (backward compatibility)
	providedSecret := r.Header.Get("X-Wazuh-Webhook-Secret")
	if providedSecret != "" {
		if subtle.ConstantTimeCompare([]byte(providedSecret), []byte(v.config.WebhookSecret)) == 1 {
			return nil
		}
		return fmt.Errorf("invalid webhook secret")
	}

	if v.config.RequireSignature {
		return fmt.Errorf("webhook signature/secret required but not provided")
	}

	return nil
}

// isIPAllowed checks if an IP is in the allowlist
func (v *WebhookValidator) isIPAllowed(ipStr string) bool {
	ip := net.ParseIP(ipStr)
	if ip == nil {
		return false
	}

	// Check against individual IPs
	for _, allowed := range v.allowedAddrs {
		if allowed.Equal(ip) {
			return true
		}
	}

	// Check against CIDRs
	for _, network := range v.allowedNets {
		if network.Contains(ip) {
			return true
		}
	}

	return false
}

// getClientIP extracts the client IP from the request.
// Prefers X-Real-IP (set by nginx, harder to spoof) over RemoteAddr.
// X-Forwarded-For is NOT used as it can be trivially spoofed by clients.
func (v *WebhookValidator) getClientIP(r *http.Request) string {
	// Prefer X-Real-IP header (set by nginx, single value)
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return strings.TrimSpace(xri)
	}

	// Fall back to RemoteAddr
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// ComputeHMAC computes an HMAC-SHA256 signature for a payload
// This can be used to sign outgoing requests or verify incoming ones
func ComputeHMAC(payload []byte, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(payload)
	return hex.EncodeToString(mac.Sum(nil))
}

// webhookRateLimiter provides per-IP rate limiting for webhook requests
type webhookRateLimiter struct {
	mu       sync.Mutex
	requests map[string][]time.Time
	limit    int
	window   time.Duration
}

func newWebhookRateLimiter(ctx context.Context, limit int, window time.Duration) *webhookRateLimiter {
	rl := &webhookRateLimiter{
		requests: make(map[string][]time.Time),
		limit:    limit,
		window:   window,
	}

	// Start cleanup goroutine with cancellation support
	go rl.cleanupLoop(ctx)

	return rl
}

func (rl *webhookRateLimiter) cleanupLoop(ctx context.Context) {
	ticker := time.NewTicker(rl.window)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			rl.mu.Lock()
			cutoff := time.Now().Add(-rl.window)
			for key, times := range rl.requests {
				var valid []time.Time
				for _, t := range times {
					if t.After(cutoff) {
						valid = append(valid, t)
					}
				}
				if len(valid) == 0 {
					delete(rl.requests, key)
				} else {
					rl.requests[key] = valid
				}
			}
			rl.mu.Unlock()
		}
	}
}

func (rl *webhookRateLimiter) Allow(ip string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-rl.window)

	times := rl.requests[ip]
	var valid []time.Time
	for _, t := range times {
		if t.After(cutoff) {
			valid = append(valid, t)
		}
	}

	if len(valid) >= rl.limit {
		return false
	}

	valid = append(valid, now)
	rl.requests[ip] = valid
	return true
}

// RemainingRequests returns the number of remaining requests for an IP
func (rl *webhookRateLimiter) RemainingRequests(ip string) int {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	cutoff := time.Now().Add(-rl.window)
	times := rl.requests[ip]

	count := 0
	for _, t := range times {
		if t.After(cutoff) {
			count++
		}
	}

	remaining := rl.limit - count
	if remaining < 0 {
		return 0
	}
	return remaining
}
