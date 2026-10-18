// Package testutil provides shared testing utilities
package testutil

import (
	"context"
	"log/slog"
	"os"
	"time"
)

// NewTestLogger creates a logger for tests that only outputs errors
func NewTestLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
}

// TestContext returns a context with a default test timeout
func TestContext() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), 5*time.Second)
}

// TestContextWithTimeout returns a context with a custom timeout
func TestContextWithTimeout(d time.Duration) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), d)
}

// FixedTime returns a fixed time for deterministic tests
func FixedTime() time.Time {
	return time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)
}

// FutureTime returns a time in the future (1 hour from fixed time)
func FutureTime() time.Time {
	return FixedTime().Add(time.Hour)
}

// PastTime returns a time in the past (1 hour before fixed time)
func PastTime() time.Time {
	return FixedTime().Add(-time.Hour)
}

// StringPtr returns a pointer to the given string
func StringPtr(s string) *string {
	return &s
}

// IntPtr returns a pointer to the given int
func IntPtr(i int) *int {
	return &i
}

// BoolPtr returns a pointer to the given bool
func BoolPtr(b bool) *bool {
	return &b
}

// TimePtr returns a pointer to the given time
func TimePtr(t time.Time) *time.Time {
	return &t
}
