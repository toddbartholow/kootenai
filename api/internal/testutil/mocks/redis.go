// Package mocks provides mock implementations for testing
package mocks

import (
	"context"
	"encoding/json"
	"errors"
	"sync"

	redisclient "github.com/toddbartholow/kootenai/api/internal/redis"
)

// Compile-time checks that mocks implement interfaces
var (
	_ redisclient.DashboardCacheClient = (*FakeDashboardCache)(nil)
	_ redisclient.LTIStateCacheClient  = (*FakeLTIStateStore)(nil)
	_ redisclient.HealthChecker        = (*FakeRedisHealthChecker)(nil)
)

// FakeDashboardCache is a mock implementation of DashboardCacheClient for testing
type FakeDashboardCache struct {
	mu sync.RWMutex

	// Storage for cached data
	dashboards   map[string][]byte
	leaderboards map[int][]byte

	// Error injection
	GetDashboardErr            error
	SetDashboardErr            error
	GetLeaderboardErr          error
	SetLeaderboardErr          error
	InvalidateUserDashboardErr error
	InvalidateLeaderboardErr   error

	// Call tracking
	GetDashboardCalls            []string
	SetDashboardCalls            []SetDashboardCall
	GetLeaderboardCalls          []int
	SetLeaderboardCalls          []SetLeaderboardCall
	InvalidateUserDashboardCalls []string
	InvalidateLeaderboardCalls   int
}

// SetDashboardCall records a call to SetDashboard
type SetDashboardCall struct {
	UserID string
	Data   any
}

// SetLeaderboardCall records a call to SetLeaderboard
type SetLeaderboardCall struct {
	Limit int
	Data  any
}

// NewFakeDashboardCache creates a new mock dashboard cache
func NewFakeDashboardCache() *FakeDashboardCache {
	return &FakeDashboardCache{
		dashboards:   make(map[string][]byte),
		leaderboards: make(map[int][]byte),
	}
}

// GetDashboard retrieves cached dashboard data
func (m *FakeDashboardCache) GetDashboard(ctx context.Context, userID string, dest any) error {
	m.mu.Lock()
	m.GetDashboardCalls = append(m.GetDashboardCalls, userID)
	m.mu.Unlock()

	if m.GetDashboardErr != nil {
		return m.GetDashboardErr
	}

	m.mu.RLock()
	data, ok := m.dashboards[userID]
	m.mu.RUnlock()

	if !ok {
		return redisclient.ErrCacheMiss
	}

	return json.Unmarshal(data, dest)
}

// SetDashboard caches dashboard data
func (m *FakeDashboardCache) SetDashboard(ctx context.Context, userID string, data any) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.SetDashboardCalls = append(m.SetDashboardCalls, SetDashboardCall{UserID: userID, Data: data})

	if m.SetDashboardErr != nil {
		return m.SetDashboardErr
	}

	jsonData, err := json.Marshal(data)
	if err != nil {
		return err
	}

	m.dashboards[userID] = jsonData
	return nil
}

// GetLeaderboard retrieves cached leaderboard data
func (m *FakeDashboardCache) GetLeaderboard(ctx context.Context, limit int, dest any) error {
	m.mu.Lock()
	m.GetLeaderboardCalls = append(m.GetLeaderboardCalls, limit)
	m.mu.Unlock()

	if m.GetLeaderboardErr != nil {
		return m.GetLeaderboardErr
	}

	m.mu.RLock()
	data, ok := m.leaderboards[limit]
	m.mu.RUnlock()

	if !ok {
		return redisclient.ErrCacheMiss
	}

	return json.Unmarshal(data, dest)
}

// SetLeaderboard caches leaderboard data
func (m *FakeDashboardCache) SetLeaderboard(ctx context.Context, limit int, data any) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.SetLeaderboardCalls = append(m.SetLeaderboardCalls, SetLeaderboardCall{Limit: limit, Data: data})

	if m.SetLeaderboardErr != nil {
		return m.SetLeaderboardErr
	}

	jsonData, err := json.Marshal(data)
	if err != nil {
		return err
	}

	m.leaderboards[limit] = jsonData
	return nil
}

// InvalidateUserDashboard removes user dashboard from cache
func (m *FakeDashboardCache) InvalidateUserDashboard(ctx context.Context, userID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.InvalidateUserDashboardCalls = append(m.InvalidateUserDashboardCalls, userID)

	if m.InvalidateUserDashboardErr != nil {
		return m.InvalidateUserDashboardErr
	}

	delete(m.dashboards, userID)
	return nil
}

// InvalidateLeaderboard removes all leaderboard caches
func (m *FakeDashboardCache) InvalidateLeaderboard(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.InvalidateLeaderboardCalls++

	if m.InvalidateLeaderboardErr != nil {
		return m.InvalidateLeaderboardErr
	}

	m.leaderboards = make(map[int][]byte)
	return nil
}

// PreloadDashboard preloads dashboard data for testing
func (m *FakeDashboardCache) PreloadDashboard(userID string, data any) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	jsonData, err := json.Marshal(data)
	if err != nil {
		return err
	}

	m.dashboards[userID] = jsonData
	return nil
}

// PreloadLeaderboard preloads leaderboard data for testing
func (m *FakeDashboardCache) PreloadLeaderboard(limit int, data any) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	jsonData, err := json.Marshal(data)
	if err != nil {
		return err
	}

	m.leaderboards[limit] = jsonData
	return nil
}

// FakeLTIStateStore is a mock implementation of LTIStateCacheClient for testing
type FakeLTIStateStore struct {
	mu sync.RWMutex

	// Storage for LTI states
	states map[string]redisclient.LTIState

	// Error injection
	StoreErr  error
	GetErr    error
	DeleteErr error

	// Call tracking
	StoreCalls  []StoreLTIStateCall
	GetCalls    []string
	DeleteCalls []string
}

// StoreLTIStateCall records a call to Store
type StoreLTIStateCall struct {
	State string
	Data  redisclient.LTIState
}

// NewFakeLTIStateStore creates a new mock LTI state store
func NewFakeLTIStateStore() *FakeLTIStateStore {
	return &FakeLTIStateStore{
		states: make(map[string]redisclient.LTIState),
	}
}

// Store saves an LTI state
func (m *FakeLTIStateStore) Store(ctx context.Context, state string, data redisclient.LTIState) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.StoreCalls = append(m.StoreCalls, StoreLTIStateCall{State: state, Data: data})

	if m.StoreErr != nil {
		return m.StoreErr
	}

	m.states[state] = data
	return nil
}

// Get retrieves an LTI state
func (m *FakeLTIStateStore) Get(ctx context.Context, state string) (*redisclient.LTIState, error) {
	m.mu.Lock()
	m.GetCalls = append(m.GetCalls, state)
	m.mu.Unlock()

	if m.GetErr != nil {
		return nil, m.GetErr
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	data, ok := m.states[state]
	if !ok {
		return nil, errors.New("state not found")
	}

	return &data, nil
}

// Delete removes an LTI state
func (m *FakeLTIStateStore) Delete(ctx context.Context, state string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.DeleteCalls = append(m.DeleteCalls, state)

	if m.DeleteErr != nil {
		return m.DeleteErr
	}

	delete(m.states, state)
	return nil
}

// FakeRedisHealthChecker is a mock implementation of HealthChecker for testing
type FakeRedisHealthChecker struct {
	mu sync.Mutex

	// Error injection
	HealthCheckErr error

	// Call tracking
	HealthCheckCalls int
}

// NewFakeRedisHealthChecker creates a new mock Redis health checker
func NewFakeRedisHealthChecker() *FakeRedisHealthChecker {
	return &FakeRedisHealthChecker{}
}

// HealthCheck performs a mock health check
func (m *FakeRedisHealthChecker) HealthCheck(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.HealthCheckCalls++
	return m.HealthCheckErr
}
