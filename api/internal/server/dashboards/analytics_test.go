package dashboards

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/toddbartholow/kootenai/api/internal/auth"
	"github.com/toddbartholow/kootenai/api/internal/database/repositories"
	"github.com/toddbartholow/kootenai/api/internal/models"
	"github.com/toddbartholow/kootenai/api/internal/server/httputil"
	"github.com/toddbartholow/kootenai/api/internal/testutil/mocks"

	"log/slog"
	"os"
)

func newTestLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
}

func newTestDashboardManager(sessionRepo repositories.SessionRepository, labTemplateRepo repositories.LabTemplateRepository) *Manager {
	logger := newTestLogger()
	return NewManager(Config{
		SessionRepo:     sessionRepo,
		LabTemplateRepo: labTemplateRepo,
		Logger:          logger,
		Responder:       httputil.NewResponder(logger),
	})
}

func TestBuildTimeAnalytics(t *testing.T) {
	t.Run("empty session repo returns empty analytics", func(t *testing.T) {
		mgr := newTestDashboardManager(nil, nil)

		analytics := mgr.buildTimeAnalytics(context.Background(), "user-123", "month")

		if analytics.UserID != "user-123" {
			t.Errorf("expected userId = user-123, got %s", analytics.UserID)
		}
		if analytics.Period != "month" {
			t.Errorf("expected period = month, got %s", analytics.Period)
		}
		if analytics.TotalTimeMinutes != 0 {
			t.Errorf("expected totalTimeMinutes = 0, got %d", analytics.TotalTimeMinutes)
		}
	})

	t.Run("calculates analytics from sessions", func(t *testing.T) {
		now := time.Now()
		yesterday := now.Add(-24 * time.Hour)
		twoDaysAgo := now.Add(-48 * time.Hour)

		sessionRepo := mocks.NewFakeSessionRepository()
		sessionRepo.AddSession(&models.Session{
			ID:            "session-1",
			UserID:        "user-123",
			LabTemplateID: "lab-1",
			StartedAt:     yesterday.Add(-30 * time.Minute),
			EndedAt:       &yesterday,
			Percentage:    85.0,
			Passed:        true,
			EarnedPoints:  85,
		})
		sessionRepo.AddSession(&models.Session{
			ID:            "session-2",
			UserID:        "user-123",
			LabTemplateID: "lab-1",
			StartedAt:     twoDaysAgo.Add(-45 * time.Minute),
			EndedAt:       &twoDaysAgo,
			Percentage:    90.0,
			Passed:        true,
			EarnedPoints:  90,
		})
		mgr := newTestDashboardManager(sessionRepo, nil)

		analytics := mgr.buildTimeAnalytics(context.Background(), "user-123", "week")

		if analytics.SessionCount != 2 {
			t.Errorf("expected sessionCount = 2, got %d", analytics.SessionCount)
		}
		if analytics.TotalTimeMinutes != 75 {
			t.Errorf("expected totalTimeMinutes = 75, got %d", analytics.TotalTimeMinutes)
		}
		if len(analytics.LabBreakdown) != 1 {
			t.Errorf("expected 1 lab in breakdown, got %d", len(analytics.LabBreakdown))
		}
	})

	t.Run("filters by period", func(t *testing.T) {
		now := time.Now()
		oneMonthAgo := now.Add(-35 * 24 * time.Hour)

		sessionRepo := mocks.NewFakeSessionRepository()
		sessionRepo.AddSession(&models.Session{
			ID:            "session-1",
			UserID:        "user-123",
			LabTemplateID: "lab-1",
			StartedAt:     now.Add(-30 * time.Minute),
			EndedAt:       &now,
			Percentage:    80.0,
			Passed:        true,
		})
		sessionRepo.AddSession(&models.Session{
			ID:            "session-2",
			UserID:        "user-123",
			LabTemplateID: "lab-2",
			StartedAt:     oneMonthAgo.Add(-60 * time.Minute),
			EndedAt:       &oneMonthAgo,
			Percentage:    70.0,
			Passed:        true,
		})
		mgr := newTestDashboardManager(sessionRepo, nil)

		analytics := mgr.buildTimeAnalytics(context.Background(), "user-123", "month")
		if analytics.SessionCount != 1 {
			t.Errorf("expected sessionCount = 1 for month period, got %d", analytics.SessionCount)
		}

		analyticsAll := mgr.buildTimeAnalytics(context.Background(), "user-123", "all")
		if analyticsAll.SessionCount != 2 {
			t.Errorf("expected sessionCount = 2 for all period, got %d", analyticsAll.SessionCount)
		}
	})
}

func TestHandleGetTimeAnalytics(t *testing.T) {
	t.Run("requires authentication", func(t *testing.T) {
		mgr := newTestDashboardManager(nil, nil)
		router := chi.NewRouter()
		router.Get("/analytics/time", mgr.handleGetTimeAnalytics)

		req := httptest.NewRequest(http.MethodGet, "/analytics/time", nil)
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		if rr.Code != http.StatusUnauthorized {
			t.Errorf("expected status %d, got %d", http.StatusUnauthorized, rr.Code)
		}
	})

	t.Run("returns analytics for authenticated user", func(t *testing.T) {
		sessionRepo := mocks.NewFakeSessionRepository()
		mgr := newTestDashboardManager(sessionRepo, nil)

		router := chi.NewRouter()
		router.Get("/analytics/time", mgr.handleGetTimeAnalytics)

		req := httptest.NewRequest(http.MethodGet, "/analytics/time?period=week", nil)
		ctx := auth.ContextWithUser(req.Context(), &auth.User{ID: "user-123", Email: "test@example.com"})
		req = req.WithContext(ctx)

		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d", http.StatusOK, rr.Code)
		}
	})
}

func TestHandleGetUserTimeAnalytics(t *testing.T) {
	t.Run("requires authentication", func(t *testing.T) {
		mgr := newTestDashboardManager(nil, nil)
		router := chi.NewRouter()
		router.Get("/users/{userID}/analytics/time", mgr.handleGetUserTimeAnalytics)

		req := httptest.NewRequest(http.MethodGet, "/users/user-123/analytics/time", nil)
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		if rr.Code != http.StatusUnauthorized {
			t.Errorf("expected status %d, got %d", http.StatusUnauthorized, rr.Code)
		}
	})

	t.Run("allows user to access own analytics", func(t *testing.T) {
		sessionRepo := mocks.NewFakeSessionRepository()
		mgr := newTestDashboardManager(sessionRepo, nil)

		router := chi.NewRouter()
		router.Get("/users/{userID}/analytics/time", mgr.handleGetUserTimeAnalytics)

		req := httptest.NewRequest(http.MethodGet, "/users/user-123/analytics/time", nil)
		ctx := auth.ContextWithUser(req.Context(), &auth.User{ID: "user-123", Email: "test@example.com"})
		req = req.WithContext(ctx)

		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d: %s", http.StatusOK, rr.Code, rr.Body.String())
		}
	})

	t.Run("admin can access other user analytics", func(t *testing.T) {
		sessionRepo := mocks.NewFakeSessionRepository()
		mgr := newTestDashboardManager(sessionRepo, nil)

		router := chi.NewRouter()
		router.Get("/users/{userID}/analytics/time", mgr.handleGetUserTimeAnalytics)

		req := httptest.NewRequest(http.MethodGet, "/users/user-456/analytics/time", nil)
		ctx := auth.ContextWithUser(req.Context(), &auth.User{
			ID:    "admin-123",
			Email: "admin@example.com",
			Roles: []string{"admin"},
		})
		req = req.WithContext(ctx)

		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d: %s", http.StatusOK, rr.Code, rr.Body.String())
		}
	})

	t.Run("non-admin cannot access other user analytics", func(t *testing.T) {
		sessionRepo := mocks.NewFakeSessionRepository()
		mgr := newTestDashboardManager(sessionRepo, nil)

		router := chi.NewRouter()
		router.Get("/users/{userID}/analytics/time", mgr.handleGetUserTimeAnalytics)

		req := httptest.NewRequest(http.MethodGet, "/users/user-456/analytics/time", nil)
		ctx := auth.ContextWithUser(req.Context(), &auth.User{
			ID:    "user-123",
			Email: "test@example.com",
			Roles: []string{"student"},
		})
		req = req.WithContext(ctx)

		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		if rr.Code != http.StatusForbidden {
			t.Errorf("expected status %d, got %d: %s", http.StatusForbidden, rr.Code, rr.Body.String())
		}
	})
}

func TestTimeAnalyticsTypes(t *testing.T) {
	t.Run("TimeAnalyticsResponse initializes correctly", func(t *testing.T) {
		analytics := TimeAnalyticsResponse{
			UserID:         "user-123",
			Period:         "month",
			DailyBreakdown: []DailyTimeStats{},
			LabBreakdown:   []LabTimeStats{},
			WeeklyTrend:    []WeeklyTimeStats{},
		}

		if analytics.UserID != "user-123" {
			t.Error("UserID not set correctly")
		}
		if analytics.DailyBreakdown == nil {
			t.Error("DailyBreakdown should not be nil")
		}
	})

	t.Run("DailyTimeStats fields", func(t *testing.T) {
		daily := DailyTimeStats{
			Date:         "2024-01-15",
			TimeMinutes:  120,
			SessionCount: 3,
		}

		if daily.Date != "2024-01-15" {
			t.Error("Date not set correctly")
		}
		if daily.TimeMinutes != 120 {
			t.Errorf("TimeMinutes = %d, want 120", daily.TimeMinutes)
		}
	})

	t.Run("LabTimeStats with average calculation", func(t *testing.T) {
		lab := LabTimeStats{
			LabID:        "lab-1",
			TotalMinutes: 300,
			SessionCount: 5,
		}
		lab.AverageMinutes = lab.TotalMinutes / lab.SessionCount

		if lab.AverageMinutes != 60 {
			t.Errorf("AverageMinutes = %d, want 60", lab.AverageMinutes)
		}
	})
}
