package redis

import (
	"context"
	"log/slog"
	"os"
	"testing"
	"time"
)

func TestNoOpTokenBlacklist(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	blacklist := NewNoOpTokenBlacklist(logger)

	ctx := context.Background()

	t.Run("Blacklist does not error", func(t *testing.T) {
		err := blacklist.Blacklist(ctx, "test-token-hash", 1*time.Hour)
		if err != nil {
			t.Errorf("NoOpTokenBlacklist.Blacklist() error = %v, want nil", err)
		}
	})

	t.Run("IsBlacklisted always returns false", func(t *testing.T) {
		// Even after "blacklisting", should return false
		_ = blacklist.Blacklist(ctx, "test-token-hash", 1*time.Hour)

		blacklisted, err := blacklist.IsBlacklisted(ctx, "test-token-hash")
		if err != nil {
			t.Errorf("NoOpTokenBlacklist.IsBlacklisted() error = %v, want nil", err)
		}
		if blacklisted {
			t.Error("NoOpTokenBlacklist.IsBlacklisted() = true, want false")
		}
	})
}

func TestTokenBlacklist_WithMockRedis(t *testing.T) {
	// Skip if running in short mode (no Redis available)
	if testing.Short() {
		t.Skip("Skipping Redis integration test in short mode")
	}

	// This test requires a running Redis instance
	// For unit tests without Redis, use NoOpTokenBlacklist
	t.Run("Interface compliance", func(t *testing.T) {
		// Verify both types implement the interface
		var _ TokenBlacklistClient = (*TokenBlacklist)(nil)
		var _ TokenBlacklistClient = (*NoOpTokenBlacklist)(nil)
	})
}

func TestNewNoOpTokenBlacklist_NilLogger(t *testing.T) {
	// Should not panic with nil logger
	blacklist := NewNoOpTokenBlacklist(nil)
	if blacklist == nil {
		t.Fatal("NewNoOpTokenBlacklist(nil) returned nil")
	}
	if blacklist.logger == nil {
		t.Error("NewNoOpTokenBlacklist should set default logger when nil provided")
	}
}

func TestTokenBlacklist_ZeroTTL(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	blacklist := NewNoOpTokenBlacklist(logger)

	ctx := context.Background()

	// Zero TTL should be a no-op (token already expired)
	err := blacklist.Blacklist(ctx, "expired-token", 0)
	if err != nil {
		t.Errorf("Blacklist with zero TTL should not error: %v", err)
	}

	// Negative TTL should also be a no-op
	err = blacklist.Blacklist(ctx, "expired-token", -1*time.Hour)
	if err != nil {
		t.Errorf("Blacklist with negative TTL should not error: %v", err)
	}
}
