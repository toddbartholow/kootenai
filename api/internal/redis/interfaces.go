// Package redis provides Redis client and caching functionality
package redis

import (
	"context"
)

// DashboardCacheClient defines the interface for dashboard caching operations.
// This interface enables dependency injection and testing of dashboard handlers.
type DashboardCacheClient interface {
	// GetDashboard retrieves cached full dashboard data for a user
	GetDashboard(ctx context.Context, userID string, dest any) error
	// SetDashboard caches full dashboard data for a user
	SetDashboard(ctx context.Context, userID string, data any) error
	// GetLeaderboard retrieves cached leaderboard data
	GetLeaderboard(ctx context.Context, limit int, dest any) error
	// SetLeaderboard caches leaderboard data
	SetLeaderboard(ctx context.Context, limit int, data any) error
	// InvalidateUserDashboard removes user dashboard data from cache
	InvalidateUserDashboard(ctx context.Context, userID string) error
	// InvalidateLeaderboard removes all leaderboard caches
	InvalidateLeaderboard(ctx context.Context) error
}

// LTIStateCacheClient defines the interface for LTI state management.
// Used by LTI handlers to store and validate OIDC state parameters.
type LTIStateCacheClient interface {
	// Store saves an LTI state with TTL
	Store(ctx context.Context, state string, data LTIState) error
	// Get retrieves an LTI state
	Get(ctx context.Context, state string) (*LTIState, error)
	// Delete removes an LTI state
	Delete(ctx context.Context, state string) error
}

// HealthChecker defines the interface for Redis health checks.
type HealthChecker interface {
	// HealthCheck performs a health check on the Redis connection
	HealthCheck(ctx context.Context) error
}

// Compile-time checks that concrete types implement interfaces
var (
	_ DashboardCacheClient = (*DashboardCache)(nil)
	_ LTIStateCacheClient  = (*LTIStateStore)(nil)
	_ HealthChecker        = (*Service)(nil)
)
