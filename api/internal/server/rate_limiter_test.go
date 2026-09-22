package server

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"
)

func TestRateLimiter_Allow(t *testing.T) {
	cfg := RateLimiterConfig{
		Rate:            10,
		Burst:           5,
		CleanupInterval: time.Minute,
	}
	rl := NewRateLimiter(cfg)

	key := "test-key"

	// First 5 requests should be allowed (burst)
	for i := 0; i < 5; i++ {
		if !rl.Allow(key) {
			t.Errorf("request %d should be allowed", i+1)
		}
	}

	// 6th request should be denied (burst exceeded)
	if rl.Allow(key) {
		t.Error("6th request should be denied (burst exceeded)")
	}
}

func TestRateLimiter_TokenRefill(t *testing.T) {
	cfg := RateLimiterConfig{
		Rate:            1000, // 1000 tokens per second for fast testing
		Burst:           1,
		CleanupInterval: time.Minute,
	}
	rl := NewRateLimiter(cfg)

	key := "test-key"

	// Use the burst
	if !rl.Allow(key) {
		t.Error("first request should be allowed")
	}

	// Immediately should be denied
	if rl.Allow(key) {
		t.Error("second immediate request should be denied")
	}

	// Wait for token refill
	time.Sleep(5 * time.Millisecond)

	// Should be allowed now
	if !rl.Allow(key) {
		t.Error("request after refill should be allowed")
	}
}

func TestRateLimiter_Remaining(t *testing.T) {
	cfg := RateLimiterConfig{
		Rate:            10,
		Burst:           10,
		CleanupInterval: time.Minute,
	}
	rl := NewRateLimiter(cfg)

	key := "test-key"

	// Initially should have full capacity
	remaining := rl.Remaining(key)
	if remaining != 10 {
		t.Errorf("expected 10 remaining, got %d", remaining)
	}

	// After consuming one token
	rl.Allow(key)
	remaining = rl.Remaining(key)
	if remaining != 9 {
		t.Errorf("expected 9 remaining after one request, got %d", remaining)
	}
}

func TestRateLimiter_MultipleKeys(t *testing.T) {
	cfg := RateLimiterConfig{
		Rate:            10,
		Burst:           2,
		CleanupInterval: time.Minute,
	}
	rl := NewRateLimiter(cfg)

	key1 := "user1"
	key2 := "user2"

	// Both users should get their own buckets
	if !rl.Allow(key1) {
		t.Error("first request from user1 should be allowed")
	}
	if !rl.Allow(key1) {
		t.Error("second request from user1 should be allowed")
	}
	if rl.Allow(key1) {
		t.Error("third request from user1 should be denied")
	}

	// User2 should still have their full burst
	if !rl.Allow(key2) {
		t.Error("first request from user2 should be allowed")
	}
	if !rl.Allow(key2) {
		t.Error("second request from user2 should be allowed")
	}
}

func TestDefaultRateLimiterConfig(t *testing.T) {
	cfg := DefaultRateLimiterConfig()

	if cfg.Rate != 10 {
		t.Errorf("expected rate 10, got %f", cfg.Rate)
	}
	if cfg.Burst != 50 {
		t.Errorf("expected burst 50, got %d", cfg.Burst)
	}
	if cfg.CleanupInterval != 5*time.Minute {
		t.Errorf("expected cleanup interval 5m, got %v", cfg.CleanupInterval)
	}
}

func TestRateLimitMiddleware(t *testing.T) {
	cfg := RateLimiterConfig{
		Rate:            10,
		Burst:           2,
		CleanupInterval: time.Minute,
	}
	rl := NewRateLimiter(cfg)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	middleware := RateLimitMiddleware(rl, IPKeyFunc())
	wrappedHandler := middleware(handler)

	// First two requests should succeed
	for i := 0; i < 2; i++ {
		req := httptest.NewRequest("GET", "/test", nil)
		req.RemoteAddr = "192.168.1.1:12345"
		rr := httptest.NewRecorder()

		wrappedHandler.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("request %d: expected status 200, got %d", i+1, rr.Code)
		}
	}

	// Third request should be rate limited
	req := httptest.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "192.168.1.1:12345"
	rr := httptest.NewRecorder()

	wrappedHandler.ServeHTTP(rr, req)

	if rr.Code != http.StatusTooManyRequests {
		t.Errorf("expected status 429, got %d", rr.Code)
	}
}

func TestIPKeyFunc(t *testing.T) {
	keyFunc := IPKeyFunc()

	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "192.168.1.100:54321"

	key := keyFunc(req)
	if key != "192.168.1.100" {
		t.Errorf("expected IP key '192.168.1.100', got %q", key)
	}
}

func TestRateLimitMiddleware_RemainingHeader_IsNumericString(t *testing.T) {
	cfg := RateLimiterConfig{
		Rate:            10,
		Burst:           5,
		CleanupInterval: time.Minute,
	}
	rl := NewRateLimiter(cfg)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	middleware := RateLimitMiddleware(rl, IPKeyFunc())
	wrappedHandler := middleware(handler)

	req := httptest.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "10.0.0.1:12345"
	rr := httptest.NewRecorder()

	wrappedHandler.ServeHTTP(rr, req)

	remaining := rr.Header().Get("X-RateLimit-Remaining")
	if remaining == "" {
		t.Fatal("expected X-RateLimit-Remaining header to be set")
	}

	// The value must be a valid integer string, not a unicode codepoint
	val, err := strconv.Atoi(remaining)
	if err != nil {
		t.Errorf("X-RateLimit-Remaining %q is not a valid integer: %v", remaining, err)
	}
	if val < 0 {
		t.Errorf("X-RateLimit-Remaining should be non-negative, got %d", val)
	}
}
