package server

import (
	"log/slog"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/toddbartholow/kootenai/api/internal/auth"
	"github.com/toddbartholow/kootenai/api/internal/recommendation"
	"github.com/toddbartholow/kootenai/api/internal/server/httputil"
)

// ---------------------------------------------------------------------------
// RecommendationManager
// ---------------------------------------------------------------------------

// RecommendationManagerConfig holds configuration for creating a RecommendationManager.
type RecommendationManagerConfig struct {
	RecommendationService *recommendation.Service
	Logger                *slog.Logger
}

// RecommendationManager owns all recommendation HTTP handlers.
type RecommendationManager struct {
	responder             *httputil.Responder
	recommendationService *recommendation.Service
	logger                *slog.Logger
}

// NewRecommendationManager creates a RecommendationManager from the given config.
func NewRecommendationManager(cfg RecommendationManagerConfig) *RecommendationManager {
	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}
	return &RecommendationManager{
		responder:             httputil.NewResponder(logger),
		recommendationService: cfg.RecommendationService,
		logger:                logger,
	}
}

// SetupRoutes registers recommendation routes: a top-level personalized feed
// at /recommendations and the per-pathway "next lab" endpoint that lives
// alongside /pathways/{pathwayID}.
func (m *RecommendationManager) SetupRoutes(r chi.Router) {
	r.Get("/recommendations", m.handleGetRecommendations)
	r.Get("/pathways/{pathwayID}/next-lab", m.handleGetPathwayNextLab)
}

// handleGetRecommendations returns personalized recommendations for the authenticated user
// @Summary Get Recommendations
// @Description Get personalized lab and pathway recommendations for the authenticated user
// @Tags recommendations
// @Produce json
// @Param limit query int false "Maximum number of recommendations per category (default: 5)"
// @Success 200 {object} recommendation.RecommendationsResponse
// @Failure 401 {object} ErrorResponse "Not authenticated"
// @Failure 500 {object} ErrorResponse
// @Security BearerAuth
// @Router /recommendations [get]
func (m *RecommendationManager) handleGetRecommendations(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user, ok := auth.UserFromContext(ctx)
	if !ok || user == nil {
		m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusUnauthorized, "authz.authRequired", nil)
		return
	}

	if m.recommendationService == nil {
		// Return empty recommendations if service not available
		m.responder.JSONResponse(w, http.StatusOK, map[string]any{
			"labs":     []any{},
			"pathways": []any{},
		})
		return
	}

	limit := 5
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
			limit = l
		}
	}

	recommendations, err := m.recommendationService.GetRecommendations(ctx, user.ID, limit)
	if err != nil {
		m.logger.Error("Failed to get recommendations", "error", err, "userId", user.ID)
		m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusInternalServerError, "recommendation.errors.getFailed", nil)
		return
	}

	m.responder.JSONResponse(w, http.StatusOK, recommendations)
}

// handleGetPathwayNextLab returns the next lab to complete in a specific pathway
// @Summary Get Next Lab in Pathway
// @Description Get the next recommended lab to complete in a specific pathway
// @Tags recommendations
// @Produce json
// @Param pathwayID path string true "Pathway ID or slug"
// @Success 200 {object} recommendation.LabRecommendation
// @Failure 401 {object} ErrorResponse "Not authenticated"
// @Failure 404 {object} ErrorResponse "No next lab found or not enrolled"
// @Failure 500 {object} ErrorResponse
// @Security BearerAuth
// @Router /pathways/{pathwayID}/next-lab [get]
func (m *RecommendationManager) handleGetPathwayNextLab(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user, ok := auth.UserFromContext(ctx)
	if !ok || user == nil {
		m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusUnauthorized, "authz.authRequired", nil)
		return
	}

	pathwayID := chi.URLParam(r, "pathwayID")

	if m.recommendationService == nil {
		m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusServiceUnavailable, "recommendation.errors.serviceUnavailable", nil)
		return
	}

	nextLab, err := m.recommendationService.GetNextLabInPathway(ctx, user.ID, pathwayID)
	if err != nil {
		m.logger.Error("Failed to get next lab", "error", err, "userId", user.ID, "pathwayId", pathwayID)
		m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusInternalServerError, "recommendation.errors.nextLabFailed", nil)
		return
	}

	if nextLab == nil {
		m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusNotFound, "recommendation.errors.noNextLab", nil)
		return
	}

	m.responder.JSONResponse(w, http.StatusOK, nextLab)
}
