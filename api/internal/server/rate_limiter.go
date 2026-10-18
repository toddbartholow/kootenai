// Package server provides rate limiting middleware for the API
package server

import (
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/toddbartholow/kootenai/api/internal/middleware"
)

// RateLimiter implements a token bucket rate limiter
type RateLimiter struct {
	mu       sync.Mutex
	buckets  map[string]*tokenBucket
	rate     float64       // tokens per second
	capacity int           // max tokens
	cleanup  time.Duration // how often to clean up old buckets
	done     chan struct{} // signals cleanup goroutine to stop
}

// tokenBucket represents a single rate limit bucket
type tokenBucket struct {
	tokens     float64
	lastUpdate time.Time
}

// RateLimiterConfig configures the rate limiter
type RateLimiterConfig struct {
	// Rate is the number of requests allowed per second
	Rate float64
	// Burst is the maximum number of requests that can be made at once
	Burst int
	// CleanupInterval is how often to clean up old buckets (default: 5 minutes)
	CleanupInterval time.Duration
}

// DefaultRateLimiterConfig returns the default rate limiter configuration
func DefaultRateLimiterConfig() RateLimiterConfig {
	return RateLimiterConfig{
		Rate:            10,              // 10 requests per second
		Burst:           50,              // burst of up to 50 requests
		CleanupInterval: 5 * time.Minute, // cleanup every 5 minutes
	}
}

// NewRateLimiter creates a new rate limiter
func NewRateLimiter(cfg RateLimiterConfig) *RateLimiter {
	if cfg.Rate <= 0 {
		cfg.Rate = 10
	}
	if cfg.Burst <= 0 {
		cfg.Burst = 50
	}
	if cfg.CleanupInterval <= 0 {
		cfg.CleanupInterval = 5 * time.Minute
	}

	rl := &RateLimiter{
		buckets:  make(map[string]*tokenBucket),
		rate:     cfg.Rate,
		capacity: cfg.Burst,
		cleanup:  cfg.CleanupInterval,
		done:     make(chan struct{}),
	}

	// Start cleanup goroutine
	go rl.cleanupLoop()

	return rl
}

// Allow checks if a request from the given key is allowed
func (rl *RateLimiter) Allow(key string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	bucket, exists := rl.buckets[key]

	if !exists {
		// Create new bucket with full capacity
		rl.buckets[key] = &tokenBucket{
			tokens:     float64(rl.capacity) - 1, // consume one token
			lastUpdate: now,
		}
		return true
	}

	// Calculate tokens to add based on elapsed time
	elapsed := now.Sub(bucket.lastUpdate).Seconds()
	bucket.tokens += elapsed * rl.rate
	if bucket.tokens > float64(rl.capacity) {
		bucket.tokens = float64(rl.capacity)
	}
	bucket.lastUpdate = now

	// Try to consume a token
	if bucket.tokens >= 1 {
		bucket.tokens--
		return true
	}

	return false
}

// Remaining returns the number of remaining tokens for a key
func (rl *RateLimiter) Remaining(key string) int {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	bucket, exists := rl.buckets[key]
	if !exists {
		return rl.capacity
	}

	// Update tokens based on elapsed time
	now := time.Now()
	elapsed := now.Sub(bucket.lastUpdate).Seconds()
	tokens := bucket.tokens + elapsed*rl.rate
	if tokens > float64(rl.capacity) {
		tokens = float64(rl.capacity)
	}

	return int(tokens)
}

// Close stops the cleanup goroutine. Call during server shutdown.
func (rl *RateLimiter) Close() {
	close(rl.done)
}

// cleanupLoop periodically removes old buckets
func (rl *RateLimiter) cleanupLoop() {
	ticker := time.NewTicker(rl.cleanup)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			rl.mu.Lock()
			now := time.Now()
			for key, bucket := range rl.buckets {
				// Remove buckets that haven't been used for 2x cleanup interval
				if now.Sub(bucket.lastUpdate) > rl.cleanup*2 {
					delete(rl.buckets, key)
				}
			}
			rl.mu.Unlock()
		case <-rl.done:
			return
		}
	}
}

// RateLimitMiddleware returns middleware that enforces rate limiting
func RateLimitMiddleware(rl *RateLimiter, keyFunc func(*http.Request) string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			key := keyFunc(r)

			if !rl.Allow(key) {
				w.Header().Set("X-RateLimit-Limit", "rate limited")
				w.Header().Set("X-RateLimit-Remaining", "0")
				w.Header().Set("Retry-After", "1")
				http.Error(w, "rate limit exceeded", http.StatusTooManyRequests)
				return
			}

			// Add rate limit headers
			remaining := rl.Remaining(key)
			w.Header().Set("X-RateLimit-Remaining", strconv.Itoa(remaining))

			next.ServeHTTP(w, r)
		})
	}
}

// IPKeyFunc returns a key function that uses the client IP address
func IPKeyFunc() func(*http.Request) string {
	return func(r *http.Request) string {
		return middleware.GetClientIP(r)
	}
}
