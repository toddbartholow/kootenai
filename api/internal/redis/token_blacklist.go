package redis

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/redis/go-redis/v9"
)

// TokenBlacklist provides a Redis-backed blacklist for invalidated JWT tokens.
// Used to prevent token reuse after refresh operations.
type TokenBlacklist struct {
	client *Client
	logger *slog.Logger
}

// NewTokenBlacklist creates a new token blacklist
func NewTokenBlacklist(client *Client, logger *slog.Logger) *TokenBlacklist {
	if logger == nil {
		logger = slog.Default()
	}
	return &TokenBlacklist{
		client: client,
		logger: logger,
	}
}

// Blacklist adds a token to the blacklist with automatic expiration.
// The TTL should match the token's remaining lifetime to avoid indefinite storage.
func (tb *TokenBlacklist) Blacklist(ctx context.Context, tokenHash string, ttl time.Duration) error {
	if ttl <= 0 {
		// If token is already expired, no need to blacklist
		return nil
	}

	key := "token:blacklist:" + tokenHash
	if err := tb.client.Set(ctx, key, []byte("1"), ttl); err != nil {
		tb.logger.Error("failed to blacklist token", slog.String("error", err.Error()))
		return fmt.Errorf("failed to blacklist token: %w", err)
	}

	tb.logger.Debug("token blacklisted", slog.String("hash", tokenHash[:8]+"..."), slog.Duration("ttl", ttl))
	return nil
}

// IsBlacklisted checks if a token is in the blacklist
func (tb *TokenBlacklist) IsBlacklisted(ctx context.Context, tokenHash string) (bool, error) {
	key := "token:blacklist:" + tokenHash
	_, err := tb.client.GetBytes(ctx, key)
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return false, nil
		}
		// Log but don't fail - if Redis is down, allow the token
		// This is a security tradeoff: availability over strict security
		tb.logger.Warn("failed to check token blacklist", slog.String("error", err.Error()))
		return false, nil
	}
	return true, nil
}

// TokenBlacklistClient defines the interface for token blacklist operations.
// This enables dependency injection and testing.
type TokenBlacklistClient interface {
	// Blacklist adds a token to the blacklist
	Blacklist(ctx context.Context, tokenHash string, ttl time.Duration) error
	// IsBlacklisted checks if a token is blacklisted
	IsBlacklisted(ctx context.Context, tokenHash string) (bool, error)
}

// Compile-time check that TokenBlacklist implements TokenBlacklistClient
var _ TokenBlacklistClient = (*TokenBlacklist)(nil)

// NoOpTokenBlacklist is a no-op implementation for when Redis is not available.
// This allows the system to function without Redis but without token blacklisting.
type NoOpTokenBlacklist struct {
	logger *slog.Logger
}

// NewNoOpTokenBlacklist creates a no-op token blacklist
func NewNoOpTokenBlacklist(logger *slog.Logger) *NoOpTokenBlacklist {
	if logger == nil {
		logger = slog.Default()
	}
	return &NoOpTokenBlacklist{logger: logger}
}

// Blacklist is a no-op that logs a warning
func (tb *NoOpTokenBlacklist) Blacklist(ctx context.Context, tokenHash string, ttl time.Duration) error {
	tb.logger.Warn("token blacklist disabled (Redis not configured)")
	return nil
}

// IsBlacklisted always returns false when Redis is not available
func (tb *NoOpTokenBlacklist) IsBlacklisted(ctx context.Context, tokenHash string) (bool, error) {
	return false, nil
}

// Compile-time check that NoOpTokenBlacklist implements TokenBlacklistClient
var _ TokenBlacklistClient = (*NoOpTokenBlacklist)(nil)
