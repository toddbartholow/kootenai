// Package middleware provides HTTP middleware components
package middleware

import (
	"net/http"
	"sync"
	"time"
)

// InMemoryRateLimiter provides basic rate limiting without Redis
// This is a fallback for when Redis is unavailable
type InMemoryRateLimiter struct {
	mu       sync.Mutex
	requests map[string][]time.Time
	limit    int
	window   time.Duration
	cleanup  time.Duration
	done     chan struct{}
}

// NewInMemoryRateLimiter creates a new in-memory rate limiter
// limit: maximum requests allowed
// window: time window for the limit
func NewInMemoryRateLimiter(limit int, window time.Duration) *InMemoryRateLimiter {
	rl := &InMemoryRateLimiter{
		requests: make(map[string][]time.Time),
		limit:    limit,
		window:   window,
		cleanup:  window * 2,
		done:     make(chan struct{}),
	}

	// Start cleanup goroutine
	go rl.cleanupLoop()

	return rl
}

// cleanupLoop periodically removes old entries
func (rl *InMemoryRateLimiter) cleanupLoop() {
	ticker := time.NewTicker(rl.cleanup)
	defer ticker.Stop()

	for {
		select {
		case <-rl.done:
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

// Close stops the cleanup goroutine
func (rl *InMemoryRateLimiter) Close() {
	close(rl.done)
}

// Allow checks if a request from the given key should be allowed
func (rl *InMemoryRateLimiter) Allow(key string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-rl.window)

	// Get existing requests for this key
	times := rl.requests[key]

	// Filter to only requests within the window
	var valid []time.Time
	for _, t := range times {
		if t.After(cutoff) {
			valid = append(valid, t)
		}
	}

	// Check if limit exceeded
	if len(valid) >= rl.limit {
		return false
	}

	// Add this request
	valid = append(valid, now)
	rl.requests[key] = valid

	return true
}

// Middleware returns an HTTP middleware that rate limits by IP
func (rl *InMemoryRateLimiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := GetClientIP(r)

		if !rl.Allow(ip) {
			w.Header().Set("Retry-After", "60")
			http.Error(w, "Too many requests", http.StatusTooManyRequests)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// DefaultAuthRateLimiter creates a rate limiter suitable for auth endpoints
// 5 requests per minute for login/password reset (stricter than general rate limit)
func DefaultAuthRateLimiter() *InMemoryRateLimiter {
	return NewInMemoryRateLimiter(5, time.Minute)
}
