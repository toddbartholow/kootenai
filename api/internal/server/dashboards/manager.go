// Package dashboards provides the dashboard and analytics HTTP handlers.
package dashboards

import (
	"log/slog"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/toddbartholow/kootenai/api/internal/auth"
	"github.com/toddbartholow/kootenai/api/internal/dashboard"
	"github.com/toddbartholow/kootenai/api/internal/database/repositories"
	redisclient "github.com/toddbartholow/kootenai/api/internal/redis"
	"github.com/toddbartholow/kootenai/api/internal/server/httputil"
	"github.com/toddbartholow/kootenai/api/internal/server/serverutil"
)

// ErrorResponse is an alias for serverutil.ErrorResponse.
//
// swag resolves an unqualified name in an annotation against the package that
// annotation lives in, so the name has to exist here. An alias rather than a
// copy keeps one canonical struct, and unlike a doc comment it counts as a use
// of the import. Same idiom as server.ErrorResponse in auth_handlers.go.
type ErrorResponse = serverutil.ErrorResponse

// Manager manages dashboard and analytics operations.
type Manager struct {
	dashboardService *dashboard.Service
	dashboardCache   redisclient.DashboardCacheClient
	sessionRepo      repositories.SessionRepository
	labTemplateRepo  repositories.LabTemplateRepository
	logger           *slog.Logger
	responder        *httputil.Responder
}

// Config configures the Manager.
type Config struct {
	DashboardService *dashboard.Service
	DashboardCache   redisclient.DashboardCacheClient
	SessionRepo      repositories.SessionRepository
	LabTemplateRepo  repositories.LabTemplateRepository
	Logger           *slog.Logger
	Responder        *httputil.Responder
}

// NewManager creates a new dashboard Manager.
func NewManager(cfg Config) *Manager {
	return &Manager{
		dashboardService: cfg.DashboardService,
		dashboardCache:   cfg.DashboardCache,
		sessionRepo:      cfg.SessionRepo,
		labTemplateRepo:  cfg.LabTemplateRepo,
		logger:           cfg.Logger,
		responder:        cfg.Responder,
	}
}

// SetupRoutes registers dashboard and analytics routes on the router.
func (m *Manager) SetupRoutes(r chi.Router) {
	r.Get("/dashboard", m.handleGetDashboard)
	r.Get("/dashboard/insights", m.handleGetInsights)
	r.Get("/activity", m.handleGetActivity)
	r.Get("/leaderboard", m.handleGetLeaderboard)
	r.Get("/analytics/time", m.handleGetTimeAnalytics)
	r.Get("/users/{userID}/analytics/time", m.handleGetUserTimeAnalytics)
}

// handleGetDashboard godoc
// @Summary Get Dashboard
// @Description Get aggregated dashboard data for the authenticated user
// @Tags dashboard
// @Accept json
// @Produce json
// @Success 200 {object} dashboard.DashboardData
// @Failure 401 {object} ErrorResponse "Not authenticated"
// @Failure 500 {object} ErrorResponse
// @Security BearerAuth
// @Router /dashboard [get]
func (m *Manager) handleGetDashboard(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user, ok := auth.UserFromContext(ctx)
	if !ok || user == nil {
		m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusUnauthorized, "authz.authRequired", nil)
		return
	}

	if m.dashboardCache != nil {
		var cached dashboard.DashboardData
		if err := m.dashboardCache.GetDashboard(ctx, user.ID, &cached); err == nil {
			cached.User.ID = user.ID
			cached.User.DisplayName = user.Name
			cached.User.Email = user.Email
			m.responder.JSONResponse(w, http.StatusOK, cached)
			return
		}
	}

	result, err := m.dashboardService.GetDashboard(ctx, user.ID, user.Name, user.Email)
	if err != nil {
		m.logger.Error("Failed to build dashboard", "error", err, "userId", user.ID)
		m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusInternalServerError, "dashboard.errors.getDashboardFailed", nil)
		return
	}

	if m.dashboardCache != nil {
		if err := m.dashboardCache.SetDashboard(ctx, user.ID, result); err != nil {
			m.logger.Warn("Failed to cache dashboard", "error", err, "userId", user.ID)
		}
	}

	m.responder.JSONResponse(w, http.StatusOK, result)
}

// handleGetActivity godoc
// @Summary Get Activity Feed
// @Description Get the activity feed for the authenticated user
// @Tags dashboard
// @Accept json
// @Produce json
// @Param limit query int false "Maximum number of activity items (default: 20)"
// @Success 200 {object} dashboard.ActivityData
// @Failure 401 {object} ErrorResponse "Not authenticated"
// @Failure 500 {object} ErrorResponse
// @Security BearerAuth
// @Router /activity [get]
func (m *Manager) handleGetActivity(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user, ok := auth.UserFromContext(ctx)
	if !ok || user == nil {
		m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusUnauthorized, "authz.authRequired", nil)
		return
	}

	limit := 20
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if parsed, err := strconv.Atoi(limitStr); err == nil && parsed > 0 {
			limit = parsed
		}
	}

	result, err := m.dashboardService.GetActivity(ctx, user.ID, limit)
	if err != nil {
		m.logger.Error("Failed to get activity", "error", err, "userId", user.ID)
		m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusInternalServerError, "dashboard.errors.getActivityFailed", nil)
		return
	}

	m.responder.JSONResponse(w, http.StatusOK, result)
}

// handleGetLeaderboard godoc
// @Summary Get Leaderboard
// @Description Get the platform leaderboard
// @Tags dashboard
// @Accept json
// @Produce json
// @Param limit query int false "Number of entries to return, max 100 (default: 10)"
// @Success 200 {object} dashboard.LeaderboardData
// @Failure 500 {object} ErrorResponse
// @Security BearerAuth
// @Router /leaderboard [get]
func (m *Manager) handleGetLeaderboard(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user, _ := auth.UserFromContext(ctx)

	limit := 10
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if parsed, err := strconv.Atoi(limitStr); err == nil && parsed > 0 && parsed <= 100 {
			limit = parsed
		}
	}

	if m.dashboardCache != nil {
		var cached dashboard.LeaderboardData
		if err := m.dashboardCache.GetLeaderboard(ctx, limit, &cached); err == nil {
			if user != nil {
				for i := range cached.Entries {
					if cached.Entries[i].UserID == user.ID {
						cached.Entries[i].IsCurrentUser = true
						cached.CurrentUser = &cached.Entries[i]
						break
					}
				}
			}
			m.responder.JSONResponse(w, http.StatusOK, cached)
			return
		}
	}

	currentUserID := ""
	if user != nil {
		currentUserID = user.ID
	}
	result, err := m.dashboardService.GetLeaderboard(ctx, currentUserID, limit)
	if err != nil {
		m.logger.Error("Failed to get leaderboard", "error", err)
		m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusInternalServerError, "dashboard.errors.getLeaderboardFailed", nil)
		return
	}

	if m.dashboardCache != nil {
		if err := m.dashboardCache.SetLeaderboard(ctx, limit, result); err != nil {
			m.logger.Warn("Failed to cache leaderboard", "error", err)
		}
	}

	m.responder.JSONResponse(w, http.StatusOK, result)
}

// handleGetInsights godoc
// @Summary Get Learning Insights
// @Description Get learning insights for the authenticated user
// @Tags dashboard
// @Accept json
// @Produce json
// @Success 200 {object} dashboard.InsightsData
// @Failure 401 {object} ErrorResponse "Not authenticated"
// @Failure 500 {object} ErrorResponse
// @Security BearerAuth
// @Router /dashboard/insights [get]
func (m *Manager) handleGetInsights(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user, ok := auth.UserFromContext(ctx)
	if !ok || user == nil {
		m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusUnauthorized, "authz.authRequired", nil)
		return
	}

	result, err := m.dashboardService.GetInsights(ctx, user.ID)
	if err != nil {
		m.logger.Error("Failed to get insights", "error", err, "userId", user.ID)
		m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusInternalServerError, "dashboard.errors.getInsightsFailed", nil)
		return
	}

	m.responder.JSONResponse(w, http.StatusOK, result)
}
