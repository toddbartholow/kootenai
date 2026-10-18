package dashboards

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/toddbartholow/kootenai/api/internal/auth"
	"github.com/toddbartholow/kootenai/api/internal/dashboard"
	"github.com/toddbartholow/kootenai/api/internal/server/httputil"
	"github.com/toddbartholow/kootenai/api/internal/testutil/mocks"
)

func dashboardTestLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func newDashboardMgrForTest() *Manager {
	logger := dashboardTestLogger()
	return NewManager(Config{
		DashboardService: dashboard.NewService(logger),
		Logger:           logger,
		Responder:        httputil.NewResponder(logger),
	})
}

func TestHandleGetDashboard(t *testing.T) {
	t.Run("unauthorized - no user in context", func(t *testing.T) {
		mgr := newDashboardMgrForTest()

		req := httptest.NewRequest(http.MethodGet, "/api/v1/dashboard", nil)
		rr := httptest.NewRecorder()

		mgr.handleGetDashboard(rr, req)

		if rr.Code != http.StatusUnauthorized {
			t.Errorf("expected status %d, got %d", http.StatusUnauthorized, rr.Code)
		}
	})

	t.Run("success - minimal data", func(t *testing.T) {
		mgr := newDashboardMgrForTest()

		user := &auth.User{
			ID:    "user-123",
			Name:  "Test User",
			Email: "test@example.com",
		}
		ctx := auth.ContextWithUser(context.Background(), user)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/dashboard", nil)
		req = req.WithContext(ctx)
		rr := httptest.NewRecorder()

		mgr.handleGetDashboard(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d: %s", http.StatusOK, rr.Code, rr.Body.String())
		}
	})
}

func TestHandleGetActivity(t *testing.T) {
	t.Run("unauthorized - no user in context", func(t *testing.T) {
		mgr := newDashboardMgrForTest()

		req := httptest.NewRequest(http.MethodGet, "/api/v1/activity", nil)
		rr := httptest.NewRecorder()

		mgr.handleGetActivity(rr, req)

		if rr.Code != http.StatusUnauthorized {
			t.Errorf("expected status %d, got %d", http.StatusUnauthorized, rr.Code)
		}
	})

	t.Run("success - empty activities", func(t *testing.T) {
		mgr := newDashboardMgrForTest()

		user := &auth.User{ID: "user-456", Name: "Test User", Email: "test@example.com"}
		ctx := auth.ContextWithUser(context.Background(), user)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/activity", nil)
		req = req.WithContext(ctx)
		rr := httptest.NewRecorder()

		mgr.handleGetActivity(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d: %s", http.StatusOK, rr.Code, rr.Body.String())
		}
	})

	t.Run("success - with limit parameter", func(t *testing.T) {
		mgr := newDashboardMgrForTest()

		user := &auth.User{ID: "user-456", Name: "Test User", Email: "test@example.com"}
		ctx := auth.ContextWithUser(context.Background(), user)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/activity?limit=5", nil)
		req = req.WithContext(ctx)
		rr := httptest.NewRecorder()

		mgr.handleGetActivity(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d: %s", http.StatusOK, rr.Code, rr.Body.String())
		}
	})
}

func TestHandleGetLeaderboard(t *testing.T) {
	t.Run("success - no data", func(t *testing.T) {
		mgr := newDashboardMgrForTest()

		req := httptest.NewRequest(http.MethodGet, "/api/v1/leaderboard", nil)
		rr := httptest.NewRecorder()

		mgr.handleGetLeaderboard(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d: %s", http.StatusOK, rr.Code, rr.Body.String())
		}
	})

	t.Run("success - with authenticated user", func(t *testing.T) {
		mgr := newDashboardMgrForTest()

		user := &auth.User{ID: "user-789", Name: "Test User", Email: "test@example.com"}
		ctx := auth.ContextWithUser(context.Background(), user)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/leaderboard", nil)
		req = req.WithContext(ctx)
		rr := httptest.NewRecorder()

		mgr.handleGetLeaderboard(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d: %s", http.StatusOK, rr.Code, rr.Body.String())
		}
	})
}

func TestHandleGetDashboard_WithMockCache(t *testing.T) {
	t.Run("returns cached data on cache hit", func(t *testing.T) {
		mockCache := mocks.NewFakeDashboardCache()
		logger := dashboardTestLogger()
		mgr := NewManager(Config{
			DashboardService: dashboard.NewService(logger),
			DashboardCache:   mockCache,
			Logger:           logger,
			Responder:        httputil.NewResponder(logger),
		})

		cachedData := dashboard.DashboardData{
			User: dashboard.UserInfo{
				ID:          "user-123",
				DisplayName: "Cached User",
				Email:       "cached@example.com",
			},
			Stats: dashboard.StatsInfo{
				TotalLabsCompleted: 5,
				CurrentStreak:      3,
				TotalTimeSpentMins: 330,
				AverageScore:       85.5,
			},
		}
		if err := mockCache.PreloadDashboard("user-123", cachedData); err != nil {
			t.Fatalf("failed to preload cache: %v", err)
		}

		user := &auth.User{ID: "user-123", Name: "Test User", Email: "test@example.com"}
		ctx := auth.ContextWithUser(context.Background(), user)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/dashboard", nil)
		req = req.WithContext(ctx)
		rr := httptest.NewRecorder()

		mgr.handleGetDashboard(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d: %s", http.StatusOK, rr.Code, rr.Body.String())
		}

		if len(mockCache.GetDashboardCalls) != 1 {
			t.Errorf("expected 1 GetDashboard call, got %d", len(mockCache.GetDashboardCalls))
		}

		var response dashboard.DashboardData
		if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		if response.User.DisplayName != "Test User" {
			t.Errorf("expected DisplayName 'Test User', got '%s'", response.User.DisplayName)
		}
		if response.Stats.TotalLabsCompleted != 5 {
			t.Errorf("expected TotalLabsCompleted 5, got %d", response.Stats.TotalLabsCompleted)
		}
	})

	t.Run("builds dashboard and caches on cache miss", func(t *testing.T) {
		mockCache := mocks.NewFakeDashboardCache()
		logger := dashboardTestLogger()
		mgr := NewManager(Config{
			DashboardService: dashboard.NewService(logger),
			DashboardCache:   mockCache,
			Logger:           logger,
			Responder:        httputil.NewResponder(logger),
		})

		user := &auth.User{ID: "user-456", Name: "New User", Email: "new@example.com"}
		ctx := auth.ContextWithUser(context.Background(), user)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/dashboard", nil)
		req = req.WithContext(ctx)
		rr := httptest.NewRecorder()

		mgr.handleGetDashboard(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d: %s", http.StatusOK, rr.Code, rr.Body.String())
		}

		if len(mockCache.SetDashboardCalls) != 1 {
			t.Errorf("expected 1 SetDashboard call, got %d", len(mockCache.SetDashboardCalls))
		}
	})

	t.Run("handles cache set error gracefully", func(t *testing.T) {
		mockCache := mocks.NewFakeDashboardCache()
		mockCache.SetDashboardErr = errors.New("cache write error")
		logger := dashboardTestLogger()
		mgr := NewManager(Config{
			DashboardService: dashboard.NewService(logger),
			DashboardCache:   mockCache,
			Logger:           logger,
			Responder:        httputil.NewResponder(logger),
		})

		user := &auth.User{ID: "user-789", Name: "Test User", Email: "test@example.com"}
		ctx := auth.ContextWithUser(context.Background(), user)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/dashboard", nil)
		req = req.WithContext(ctx)
		rr := httptest.NewRecorder()

		mgr.handleGetDashboard(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d: %s", http.StatusOK, rr.Code, rr.Body.String())
		}
	})
}

func TestHandleGetLeaderboard_WithMockCache(t *testing.T) {
	t.Run("returns cached leaderboard on cache hit", func(t *testing.T) {
		mockCache := mocks.NewFakeDashboardCache()
		logger := dashboardTestLogger()
		mgr := NewManager(Config{
			DashboardService: dashboard.NewService(logger),
			DashboardCache:   mockCache,
			Logger:           logger,
			Responder:        httputil.NewResponder(logger),
		})

		cachedData := dashboard.LeaderboardData{
			Entries: []dashboard.LeaderboardEntry{
				{Rank: 1, UserID: "user-1", DisplayName: "Top User", TotalPoints: 500},
				{Rank: 2, UserID: "user-2", DisplayName: "Second User", TotalPoints: 400},
			},
		}
		if err := mockCache.PreloadLeaderboard(10, cachedData); err != nil {
			t.Fatalf("failed to preload cache: %v", err)
		}

		req := httptest.NewRequest(http.MethodGet, "/api/v1/leaderboard", nil)
		rr := httptest.NewRecorder()

		mgr.handleGetLeaderboard(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d: %s", http.StatusOK, rr.Code, rr.Body.String())
		}

		var response dashboard.LeaderboardData
		if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		if len(response.Entries) != 2 {
			t.Errorf("expected 2 entries, got %d", len(response.Entries))
		}
	})

	t.Run("marks current user in cached leaderboard", func(t *testing.T) {
		mockCache := mocks.NewFakeDashboardCache()
		logger := dashboardTestLogger()
		mgr := NewManager(Config{
			DashboardService: dashboard.NewService(logger),
			DashboardCache:   mockCache,
			Logger:           logger,
			Responder:        httputil.NewResponder(logger),
		})

		cachedData := dashboard.LeaderboardData{
			Entries: []dashboard.LeaderboardEntry{
				{Rank: 1, UserID: "user-1", DisplayName: "Top User", TotalPoints: 500},
				{Rank: 2, UserID: "current-user", DisplayName: "Current", TotalPoints: 400},
			},
		}
		if err := mockCache.PreloadLeaderboard(10, cachedData); err != nil {
			t.Fatalf("failed to preload cache: %v", err)
		}

		user := &auth.User{ID: "current-user", Name: "Current User", Email: "current@example.com"}
		ctx := auth.ContextWithUser(context.Background(), user)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/leaderboard", nil)
		req = req.WithContext(ctx)
		rr := httptest.NewRecorder()

		mgr.handleGetLeaderboard(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d: %s", http.StatusOK, rr.Code, rr.Body.String())
		}

		var response dashboard.LeaderboardData
		if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		if !response.Entries[1].IsCurrentUser {
			t.Error("expected second entry to be marked as current user")
		}
		if response.CurrentUser == nil || response.CurrentUser.UserID != "current-user" {
			t.Error("expected CurrentUser to be set correctly")
		}
	})
}
