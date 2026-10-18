package wazuh

import (
	"context"
	"log/slog"
	"net/http/httptest"
	"os"
	"testing"
	"time"
)

func TestDefaultSecurityConfig(t *testing.T) {
	config := DefaultSecurityConfig()

	if len(config.AllowedIPs) != 0 {
		t.Errorf("expected empty AllowedIPs, got %v", config.AllowedIPs)
	}
	if config.RateLimitPerMinute != 100 {
		t.Errorf("expected RateLimitPerMinute=100, got %d", config.RateLimitPerMinute)
	}
	if config.MaxTimestampDrift != 5*time.Minute {
		t.Errorf("expected MaxTimestampDrift=5m, got %v", config.MaxTimestampDrift)
	}
	if config.RequireSignature {
		t.Error("expected RequireSignature=false")
	}
}

func TestNewWebhookValidator(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	tests := []struct {
		name        string
		config      SecurityConfig
		expectError bool
	}{
		{
			name:        "default config",
			config:      DefaultSecurityConfig(),
			expectError: false,
		},
		{
			name: "with valid CIDR",
			config: SecurityConfig{
				AllowedIPs:         []string{"192.168.1.0/24", "10.0.0.0/8"},
				RateLimitPerMinute: 100,
			},
			expectError: false,
		},
		{
			name: "with valid IP",
			config: SecurityConfig{
				AllowedIPs:         []string{"192.168.1.100", "10.0.0.1"},
				RateLimitPerMinute: 100,
			},
			expectError: false,
		},
		{
			name: "with invalid CIDR",
			config: SecurityConfig{
				AllowedIPs:         []string{"192.168.1.0/invalid"},
				RateLimitPerMinute: 100,
			},
			expectError: true,
		},
		{
			name: "with invalid IP",
			config: SecurityConfig{
				AllowedIPs:         []string{"not.an.ip.address"},
				RateLimitPerMinute: 100,
			},
			expectError: true,
		},
		{
			name: "empty allowlist entries are skipped",
			config: SecurityConfig{
				AllowedIPs:         []string{"", "  ", "192.168.1.1"},
				RateLimitPerMinute: 100,
			},
			expectError: false,
		},
		{
			name: "no rate limiting",
			config: SecurityConfig{
				AllowedIPs:         []string{},
				RateLimitPerMinute: 0,
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			validator, err := NewWebhookValidator(t.Context(), tt.config, logger)
			if tt.expectError {
				if err == nil {
					t.Error("expected error but got nil")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if validator == nil {
					t.Error("expected non-nil validator")
				}
			}
		})
	}
}

func TestWebhookValidator_IPAllowlist(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	config := SecurityConfig{
		AllowedIPs:         []string{"192.168.1.0/24", "10.0.0.50"},
		RateLimitPerMinute: 1000, // High limit to avoid rate limiting during tests
	}

	validator, err := NewWebhookValidator(t.Context(), config, logger)
	if err != nil {
		t.Fatalf("failed to create validator: %v", err)
	}

	tests := []struct {
		name        string
		remoteAddr  string
		expectError bool
	}{
		{
			name:        "allowed IP in CIDR range",
			remoteAddr:  "192.168.1.100:12345",
			expectError: false,
		},
		{
			name:        "allowed specific IP",
			remoteAddr:  "10.0.0.50:12345",
			expectError: false,
		},
		{
			name:        "disallowed IP",
			remoteAddr:  "172.16.0.1:12345",
			expectError: true,
		},
		{
			name:        "IP outside CIDR range",
			remoteAddr:  "192.168.2.1:12345",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("POST", "/api/v1/events/wazuh", nil)
			req.RemoteAddr = tt.remoteAddr

			err := validator.ValidateRequest(req, nil)
			if tt.expectError {
				if err == nil {
					t.Error("expected error but got nil")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			}
		})
	}
}

func TestWebhookValidator_NoIPRestriction(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	config := SecurityConfig{
		AllowedIPs:         []string{}, // No IP restriction
		RateLimitPerMinute: 1000,
	}

	validator, err := NewWebhookValidator(t.Context(), config, logger)
	if err != nil {
		t.Fatalf("failed to create validator: %v", err)
	}

	// Any IP should be allowed
	req := httptest.NewRequest("POST", "/api/v1/events/wazuh", nil)
	req.RemoteAddr = "1.2.3.4:12345"

	err = validator.ValidateRequest(req, nil)
	if err != nil {
		t.Errorf("unexpected error with no IP restriction: %v", err)
	}
}

func TestWebhookValidator_RateLimit(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	config := SecurityConfig{
		AllowedIPs:         []string{},
		RateLimitPerMinute: 3, // Low limit for testing
	}

	validator, err := NewWebhookValidator(t.Context(), config, logger)
	if err != nil {
		t.Fatalf("failed to create validator: %v", err)
	}

	// Make requests up to the limit
	for i := 0; i < 3; i++ {
		req := httptest.NewRequest("POST", "/api/v1/events/wazuh", nil)
		req.RemoteAddr = "192.168.1.1:12345"

		err := validator.ValidateRequest(req, nil)
		if err != nil {
			t.Errorf("request %d should have succeeded: %v", i+1, err)
		}
	}

	// This request should be rate limited
	req := httptest.NewRequest("POST", "/api/v1/events/wazuh", nil)
	req.RemoteAddr = "192.168.1.1:12345"

	err = validator.ValidateRequest(req, nil)
	if err == nil {
		t.Error("expected rate limit error but got nil")
	}
}

func TestWebhookValidator_RateLimitPerIP(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	config := SecurityConfig{
		AllowedIPs:         []string{},
		RateLimitPerMinute: 2,
	}

	validator, err := NewWebhookValidator(t.Context(), config, logger)
	if err != nil {
		t.Fatalf("failed to create validator: %v", err)
	}

	// Exhaust limit for IP1
	for i := 0; i < 2; i++ {
		req := httptest.NewRequest("POST", "/api/v1/events/wazuh", nil)
		req.RemoteAddr = "192.168.1.1:12345"
		_ = validator.ValidateRequest(req, nil)
	}

	// IP1 should be rate limited
	req1 := httptest.NewRequest("POST", "/api/v1/events/wazuh", nil)
	req1.RemoteAddr = "192.168.1.1:12345"
	if err := validator.ValidateRequest(req1, nil); err == nil {
		t.Error("IP1 should be rate limited")
	}

	// IP2 should still work
	req2 := httptest.NewRequest("POST", "/api/v1/events/wazuh", nil)
	req2.RemoteAddr = "192.168.1.2:12345"
	if err := validator.ValidateRequest(req2, nil); err != nil {
		t.Errorf("IP2 should not be rate limited: %v", err)
	}
}

func TestWebhookValidator_SecretValidation(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	config := SecurityConfig{
		WebhookSecret:      "test-secret-12345",
		RateLimitPerMinute: 1000,
		RequireSignature:   true,
	}

	validator, err := NewWebhookValidator(t.Context(), config, logger)
	if err != nil {
		t.Fatalf("failed to create validator: %v", err)
	}

	tests := []struct {
		name        string
		secret      string
		expectError bool
	}{
		{
			name:        "valid secret",
			secret:      "test-secret-12345",
			expectError: false,
		},
		{
			name:        "invalid secret",
			secret:      "wrong-secret",
			expectError: true,
		},
		{
			name:        "empty secret",
			secret:      "",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("POST", "/api/v1/events/wazuh", nil)
			req.RemoteAddr = "127.0.0.1:12345"
			if tt.secret != "" {
				req.Header.Set("X-Wazuh-Webhook-Secret", tt.secret)
			}

			err := validator.ValidateRequest(req, nil)
			if tt.expectError {
				if err == nil {
					t.Error("expected error but got nil")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			}
		})
	}
}

func TestWebhookValidator_RequireSignatureNotConfigured(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	config := SecurityConfig{
		WebhookSecret:      "", // Not configured
		RateLimitPerMinute: 1000,
		RequireSignature:   true,
	}

	validator, err := NewWebhookValidator(t.Context(), config, logger)
	if err != nil {
		t.Fatalf("failed to create validator: %v", err)
	}

	req := httptest.NewRequest("POST", "/api/v1/events/wazuh", nil)
	req.RemoteAddr = "127.0.0.1:12345"

	err = validator.ValidateRequest(req, nil)
	if err == nil {
		t.Error("expected error when signature required but not configured")
	}
}

func TestWebhookValidator_SecretNotRequiredButProvided(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	config := SecurityConfig{
		WebhookSecret:      "test-secret",
		RateLimitPerMinute: 1000,
		RequireSignature:   false, // Not required
	}

	validator, err := NewWebhookValidator(t.Context(), config, logger)
	if err != nil {
		t.Fatalf("failed to create validator: %v", err)
	}

	// Valid secret should work
	req := httptest.NewRequest("POST", "/api/v1/events/wazuh", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	req.Header.Set("X-Wazuh-Webhook-Secret", "test-secret")

	err = validator.ValidateRequest(req, nil)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	// Invalid secret should fail (when secret is configured, it's validated)
	req2 := httptest.NewRequest("POST", "/api/v1/events/wazuh", nil)
	req2.RemoteAddr = "127.0.0.1:12345"
	req2.Header.Set("X-Wazuh-Webhook-Secret", "wrong-secret")

	err = validator.ValidateRequest(req2, nil)
	if err == nil {
		t.Error("expected error for invalid secret")
	}
}

func TestWebhookValidator_ValidateTimestamp(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	config := SecurityConfig{
		MaxTimestampDrift:  5 * time.Minute,
		RateLimitPerMinute: 1000,
	}

	validator, err := NewWebhookValidator(t.Context(), config, logger)
	if err != nil {
		t.Fatalf("failed to create validator: %v", err)
	}

	tests := []struct {
		name        string
		eventTime   time.Time
		expectError bool
	}{
		{
			name:        "current time",
			eventTime:   time.Now(),
			expectError: false,
		},
		{
			name:        "1 minute ago",
			eventTime:   time.Now().Add(-1 * time.Minute),
			expectError: false,
		},
		{
			name:        "4 minutes ago",
			eventTime:   time.Now().Add(-4 * time.Minute),
			expectError: false,
		},
		{
			name:        "10 minutes ago - too old",
			eventTime:   time.Now().Add(-10 * time.Minute),
			expectError: true,
		},
		{
			name:        "1 minute in future",
			eventTime:   time.Now().Add(1 * time.Minute),
			expectError: false,
		},
		{
			name:        "10 minutes in future - too far",
			eventTime:   time.Now().Add(10 * time.Minute),
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator.ValidateTimestamp(tt.eventTime)
			if tt.expectError {
				if err == nil {
					t.Error("expected error but got nil")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			}
		})
	}
}

func TestWebhookValidator_TimestampValidationDisabled(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	config := SecurityConfig{
		MaxTimestampDrift:  0, // Disabled
		RateLimitPerMinute: 1000,
	}

	validator, err := NewWebhookValidator(t.Context(), config, logger)
	if err != nil {
		t.Fatalf("failed to create validator: %v", err)
	}

	// Even very old timestamps should pass
	err = validator.ValidateTimestamp(time.Now().Add(-24 * time.Hour))
	if err != nil {
		t.Errorf("timestamp validation should be disabled: %v", err)
	}
}

func TestWebhookValidator_GetClientIP(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	config := SecurityConfig{
		AllowedIPs:         []string{"192.168.1.100", "10.0.0.1"},
		RateLimitPerMinute: 1000,
	}

	validator, err := NewWebhookValidator(t.Context(), config, logger)
	if err != nil {
		t.Fatalf("failed to create validator: %v", err)
	}

	tests := []struct {
		name       string
		remoteAddr string
		xff        string
		xri        string
		expectedIP string
	}{
		{
			name:       "RemoteAddr only",
			remoteAddr: "192.168.1.100:12345",
			expectedIP: "192.168.1.100",
		},
		{
			name:       "X-Forwarded-For ignored (spoofable)",
			remoteAddr: "127.0.0.1:12345",
			xff:        "192.168.1.100",
			expectedIP: "127.0.0.1",
		},
		{
			name:       "X-Real-IP",
			remoteAddr: "127.0.0.1:12345",
			xri:        "192.168.1.100",
			expectedIP: "192.168.1.100",
		},
		{
			name:       "X-Real-IP takes precedence over X-Forwarded-For",
			remoteAddr: "127.0.0.1:12345",
			xff:        "192.168.1.100",
			xri:        "10.0.0.1",
			expectedIP: "10.0.0.1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("POST", "/api/v1/events/wazuh", nil)
			req.RemoteAddr = tt.remoteAddr
			if tt.xff != "" {
				req.Header.Set("X-Forwarded-For", tt.xff)
			}
			if tt.xri != "" {
				req.Header.Set("X-Real-IP", tt.xri)
			}

			ip := validator.getClientIP(req)
			if ip != tt.expectedIP {
				t.Errorf("expected IP %s, got %s", tt.expectedIP, ip)
			}
		})
	}
}

func TestWebhookValidator_IPv6(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	config := SecurityConfig{
		AllowedIPs:         []string{"2001:db8::/32", "::1"},
		RateLimitPerMinute: 1000,
	}

	validator, err := NewWebhookValidator(t.Context(), config, logger)
	if err != nil {
		t.Fatalf("failed to create validator: %v", err)
	}

	tests := []struct {
		name        string
		remoteAddr  string
		expectError bool
	}{
		{
			name:        "allowed IPv6 localhost",
			remoteAddr:  "[::1]:12345",
			expectError: false,
		},
		{
			name:        "allowed IPv6 in range",
			remoteAddr:  "[2001:db8::1]:12345",
			expectError: false,
		},
		{
			name:        "disallowed IPv6",
			remoteAddr:  "[2001:db9::1]:12345",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("POST", "/api/v1/events/wazuh", nil)
			req.RemoteAddr = tt.remoteAddr

			err := validator.ValidateRequest(req, nil)
			if tt.expectError {
				if err == nil {
					t.Error("expected error but got nil")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			}
		})
	}
}

func TestComputeHMAC(t *testing.T) {
	tests := []struct {
		name     string
		payload  []byte
		secret   string
		expected string
	}{
		{
			name:     "simple payload",
			payload:  []byte("test payload"),
			secret:   "secret",
			expected: "f1f1fc517bb886ad22c56e51dae135aad082b2e3337bed35e2e44cd299324bd8",
		},
		{
			name:     "empty payload",
			payload:  []byte(""),
			secret:   "secret",
			expected: "f9e66e179b6747ae54108f82f8ade8b3c25d76fd30afde6c395822c530196169",
		},
		{
			name:     "JSON payload",
			payload:  []byte(`{"rule":{"id":"100200"},"agent":{"id":"001"}}`),
			secret:   "webhook-secret-123",
			expected: "2ce4bb7a32f014238807447236d31e6deb64d98f7167e28ae613151d412c1637",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ComputeHMAC(tt.payload, tt.secret)
			if result != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, result)
			}
		})
	}
}

func TestWebhookRateLimiter_RemainingRequests(t *testing.T) {
	rl := newWebhookRateLimiter(context.Background(), 5, time.Minute)

	ip := "192.168.1.1"

	// Initially should have all requests available
	remaining := rl.RemainingRequests(ip)
	if remaining != 5 {
		t.Errorf("expected 5 remaining, got %d", remaining)
	}

	// Use some requests
	rl.Allow(ip)
	rl.Allow(ip)

	remaining = rl.RemainingRequests(ip)
	if remaining != 3 {
		t.Errorf("expected 3 remaining, got %d", remaining)
	}

	// Use all remaining
	rl.Allow(ip)
	rl.Allow(ip)
	rl.Allow(ip)

	remaining = rl.RemainingRequests(ip)
	if remaining != 0 {
		t.Errorf("expected 0 remaining, got %d", remaining)
	}
}

func TestWebhookRateLimiter_AllowDeniesOverLimit(t *testing.T) {
	rl := newWebhookRateLimiter(context.Background(), 2, time.Minute)

	ip := "192.168.1.1"

	// First two should succeed
	if !rl.Allow(ip) {
		t.Error("first request should be allowed")
	}
	if !rl.Allow(ip) {
		t.Error("second request should be allowed")
	}

	// Third should fail
	if rl.Allow(ip) {
		t.Error("third request should be denied")
	}
}

func TestWebhookValidator_CombinedValidation(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	config := SecurityConfig{
		WebhookSecret:      "test-secret",
		AllowedIPs:         []string{"192.168.1.0/24"},
		RateLimitPerMinute: 100,
		RequireSignature:   true,
	}

	validator, err := NewWebhookValidator(t.Context(), config, logger)
	if err != nil {
		t.Fatalf("failed to create validator: %v", err)
	}

	tests := []struct {
		name        string
		remoteAddr  string
		secret      string
		expectError bool
		errorMsg    string
	}{
		{
			name:        "all valid",
			remoteAddr:  "192.168.1.100:12345",
			secret:      "test-secret",
			expectError: false,
		},
		{
			name:        "wrong IP",
			remoteAddr:  "10.0.0.1:12345",
			secret:      "test-secret",
			expectError: true,
			errorMsg:    "unauthorized source IP",
		},
		{
			name:        "wrong secret",
			remoteAddr:  "192.168.1.100:12345",
			secret:      "wrong-secret",
			expectError: true,
			errorMsg:    "invalid webhook secret",
		},
		{
			name:        "missing secret",
			remoteAddr:  "192.168.1.100:12345",
			secret:      "",
			expectError: true,
			errorMsg:    "required but not provided",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("POST", "/api/v1/events/wazuh", nil)
			req.RemoteAddr = tt.remoteAddr
			if tt.secret != "" {
				req.Header.Set("X-Wazuh-Webhook-Secret", tt.secret)
			}

			err := validator.ValidateRequest(req, nil)
			if tt.expectError {
				if err == nil {
					t.Error("expected error but got nil")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			}
		})
	}
}

func TestSecurityConfig_Fields(t *testing.T) {
	config := SecurityConfig{
		WebhookSecret:      "secret-123",
		AllowedIPs:         []string{"10.0.0.0/8", "192.168.1.1"},
		RateLimitPerMinute: 500,
		MaxTimestampDrift:  10 * time.Minute,
		RequireSignature:   true,
	}

	if config.WebhookSecret != "secret-123" {
		t.Errorf("WebhookSecret mismatch")
	}
	if len(config.AllowedIPs) != 2 {
		t.Errorf("AllowedIPs length mismatch")
	}
	if config.RateLimitPerMinute != 500 {
		t.Errorf("RateLimitPerMinute mismatch")
	}
	if config.MaxTimestampDrift != 10*time.Minute {
		t.Errorf("MaxTimestampDrift mismatch")
	}
	if !config.RequireSignature {
		t.Errorf("RequireSignature mismatch")
	}
}

func BenchmarkComputeHMAC(b *testing.B) {
	payload := []byte(`{"rule":{"id":"100200","level":15},"agent":{"id":"001","name":"test-vm"}}`)
	secret := "benchmark-secret-key-32-chars!!"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ComputeHMAC(payload, secret)
	}
}

func BenchmarkRateLimiterAllow(b *testing.B) {
	rl := newWebhookRateLimiter(context.Background(), 10000, time.Minute)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rl.Allow("192.168.1.1")
	}
}

func BenchmarkValidateRequest(b *testing.B) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	config := SecurityConfig{
		WebhookSecret:      "benchmark-secret",
		AllowedIPs:         []string{"192.168.0.0/16"},
		RateLimitPerMinute: 100000, // High to avoid rate limiting
		RequireSignature:   true,
	}

	validator, _ := NewWebhookValidator(b.Context(), config, logger)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest("POST", "/api/v1/events/wazuh", nil)
		req.RemoteAddr = "192.168.1.100:12345"
		req.Header.Set("X-Wazuh-Webhook-Secret", "benchmark-secret")
		validator.ValidateRequest(req, nil)
	}
}
