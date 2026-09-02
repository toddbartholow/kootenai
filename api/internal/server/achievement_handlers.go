package server

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/toddbartholow/kootenai/api/internal/achievements"
	"github.com/toddbartholow/kootenai/api/internal/auth"
	"github.com/toddbartholow/kootenai/api/internal/database/repositories"
	"github.com/toddbartholow/kootenai/api/internal/models"
	"github.com/toddbartholow/kootenai/api/internal/server/httputil"
)

// -----------------------------------------------------------------------------
// Achievement Manager
// -----------------------------------------------------------------------------

// AchievementManager manages achievement-related HTTP handlers.
type AchievementManager struct {
	achievementRepo    repositories.AchievementRepository
	achievementService *achievements.Service
	logger             *slog.Logger
	responder          *httputil.Responder
}

// AchievementManagerConfig configures AchievementManager.
type AchievementManagerConfig struct {
	AchievementRepo    repositories.AchievementRepository
	AchievementService *achievements.Service
	Logger             *slog.Logger
}

// NewAchievementManager creates a new AchievementManager.
func NewAchievementManager(cfg AchievementManagerConfig) *AchievementManager {
	return &AchievementManager{
		achievementRepo:    cfg.AchievementRepo,
		achievementService: cfg.AchievementService,
		logger:             cfg.Logger,
		responder:          httputil.NewResponder(cfg.Logger),
	}
}

// SetupRoutes registers achievement routes on the router.
func (m *AchievementManager) SetupRoutes(r chi.Router) {
	r.Route("/achievements", func(r chi.Router) {
		r.Get("/", m.handleListAchievements)
		r.Get("/recent", m.handleGetRecentAchievements)
		r.Get("/{achievementID}", m.handleGetAchievement)
	})

	// User achievements live under /users/{userID}/achievements for REST-ish
	// nesting; they still belong to this manager.
	r.Route("/users/{userID}/achievements", func(r chi.Router) {
		r.Get("/", m.handleListUserAchievements)
		r.Get("/summary", m.handleGetUserAchievementSummary)
	})
}

func (m *AchievementManager) handleListAchievements(w http.ResponseWriter, r *http.Request) {
	if m.achievementRepo == nil {
		m.responder.JSONResponse(w, http.StatusOK, map[string]any{
			"achievements": []any{},
		})
		return
	}

	ctx := r.Context()

	// Parse query parameters
	typeFilter := r.URL.Query().Get("type")
	tierFilter := r.URL.Query().Get("tier")
	activeOnly := r.URL.Query().Get("active") == "true"

	filter := repositories.AchievementFilter{}
	if typeFilter != "" {
		filter.Type = models.AchievementType(typeFilter)
	}
	if tierFilter != "" {
		filter.Tier = models.AchievementTier(tierFilter)
	}
	if activeOnly {
		active := true
		filter.IsActive = &active
	}

	achievements, err := m.achievementRepo.ListAchievements(ctx, filter)
	if err != nil {
		m.logger.Error("Failed to list achievements", "error", err)
		m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusInternalServerError, "achievement.errors.listFailed", nil)
		return
	}

	m.responder.JSONResponse(w, http.StatusOK, map[string]any{
		"achievements": achievements,
		"count":        len(achievements),
	})
}

func (m *AchievementManager) handleGetAchievement(w http.ResponseWriter, r *http.Request) {
	achievementID := chi.URLParam(r, "achievementID")

	if m.achievementRepo == nil {
		m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusNotFound, "achievement.errors.repoNotAvailable", nil)
		return
	}

	achievement, err := m.achievementRepo.GetAchievementByID(r.Context(), achievementID)
	if err != nil {
		m.logger.Error("Failed to get achievement", "error", err, "achievementId", achievementID)
		m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusInternalServerError, "achievement.errors.getFailed", nil)
		return
	}

	if achievement == nil {
		m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusNotFound, "achievement.errors.notFound", nil)
		return
	}

	m.responder.JSONResponse(w, http.StatusOK, achievement)
}

func (m *AchievementManager) handleListUserAchievements(w http.ResponseWriter, r *http.Request) {
	userID := chi.URLParam(r, "userID")

	// Authorize: users can view their own achievements; admins/instructors can view anyone's
	currentUser, ok := auth.UserFromContext(r.Context())
	if !ok {
		m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusUnauthorized, "authz.authRequired", nil)
		return
	}
	if currentUser.ID != userID && !isAdminOrInstructor(currentUser) {
		m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusForbidden, "authz.accessDenied", nil)
		return
	}

	if m.achievementService == nil {
		m.responder.JSONResponse(w, http.StatusOK, map[string]any{
			"achievements": []any{},
		})
		return
	}

	achievements, err := m.achievementService.GetUserAchievements(r.Context(), userID)
	if err != nil {
		m.logger.Error("Failed to get user achievements", "error", err, "userId", userID)
		m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusInternalServerError, "achievement.errors.userFailed", nil)
		return
	}

	m.responder.JSONResponse(w, http.StatusOK, map[string]any{
		"achievements": achievements,
		"count":        len(achievements),
	})
}

func (m *AchievementManager) handleGetUserAchievementSummary(w http.ResponseWriter, r *http.Request) {
	userID := chi.URLParam(r, "userID")

	// Authorize: users can view their own summary; admins/instructors can view anyone's
	currentUser, ok := auth.UserFromContext(r.Context())
	if !ok {
		m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusUnauthorized, "authz.authRequired", nil)
		return
	}
	if currentUser.ID != userID && !isAdminOrInstructor(currentUser) {
		m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusForbidden, "authz.accessDenied", nil)
		return
	}

	if m.achievementService == nil {
		m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusNotFound, "achievement.errors.serviceNotAvailable", nil)
		return
	}

	summary, err := m.achievementService.GetUserSummary(r.Context(), userID)
	if err != nil {
		m.logger.Error("Failed to get achievement summary", "error", err, "userId", userID)
		m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusInternalServerError, "achievement.errors.summaryFailed", nil)
		return
	}

	m.responder.JSONResponse(w, http.StatusOK, summary)
}

func (m *AchievementManager) handleGetRecentAchievements(w http.ResponseWriter, r *http.Request) {
	if m.achievementRepo == nil {
		m.responder.JSONResponse(w, http.StatusOK, map[string]any{
			"achievements": []any{},
		})
		return
	}

	limit := 10
	achievements, err := m.achievementRepo.GetRecentAchievements(r.Context(), limit)
	if err != nil {
		m.logger.Error("Failed to get recent achievements", "error", err)
		m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusInternalServerError, "achievement.errors.recentFailed", nil)
		return
	}

	m.responder.JSONResponse(w, http.StatusOK, map[string]any{
		"achievements": achievements,
		"count":        len(achievements),
	})
}

// ServerOption for achievement service
func WithAchievementService(service *achievements.Service) ServerOption {
	return func(s *Server) {
		s.ensureDeps().achievementService = service
	}
}

// ServerOption for achievement repository
func WithAchievementRepo(repo repositories.AchievementRepository) ServerOption {
	return func(s *Server) {
		s.ensureDeps().achievementRepo = repo
	}
}
