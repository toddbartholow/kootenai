package server

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"

	"github.com/toddbartholow/kootenai/api/internal/auth"
)

func recTestLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
}

func TestHandleGetRecommendations_Unauthorized(t *testing.T) {
	mgr := NewRecommendationManager(RecommendationManagerConfig{
		Logger: recTestLogger(),
	})

	router := chi.NewRouter()
	router.Get("/recommendations", mgr.handleGetRecommendations)

	req := httptest.NewRequest(http.MethodGet, "/recommendations", nil)
	rr := httptest.NewRecorder()

	router.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

func TestHandleGetRecommendations_NoService(t *testing.T) {
	mgr := NewRecommendationManager(RecommendationManagerConfig{
		RecommendationService: nil,
		Logger:                recTestLogger(),
	})

	router := chi.NewRouter()
	router.Get("/recommendations", mgr.handleGetRecommendations)

	req := httptest.NewRequest(http.MethodGet, "/recommendations", nil)
	ctx := auth.ContextWithUser(req.Context(), &auth.User{ID: "user-123", Roles: []string{"student"}})
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()

	router.ServeHTTP(rr, req)

	// Should return empty recommendations, not error
	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Contains(t, rr.Body.String(), `"labs":[]`)
	assert.Contains(t, rr.Body.String(), `"pathways":[]`)
}

func TestHandleGetRecommendations_WithLimit(t *testing.T) {
	mgr := NewRecommendationManager(RecommendationManagerConfig{
		RecommendationService: nil,
		Logger:                recTestLogger(),
	})

	router := chi.NewRouter()
	router.Get("/recommendations", mgr.handleGetRecommendations)

	req := httptest.NewRequest(http.MethodGet, "/recommendations?limit=10", nil)
	ctx := auth.ContextWithUser(req.Context(), &auth.User{ID: "user-123", Roles: []string{"student"}})
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()

	router.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestHandleGetPathwayNextLab_Unauthorized(t *testing.T) {
	mgr := NewRecommendationManager(RecommendationManagerConfig{
		Logger: recTestLogger(),
	})

	router := chi.NewRouter()
	router.Get("/pathways/{pathwayID}/next-lab", mgr.handleGetPathwayNextLab)

	req := httptest.NewRequest(http.MethodGet, "/pathways/pathway-123/next-lab", nil)
	rr := httptest.NewRecorder()

	router.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

func TestHandleGetPathwayNextLab_ServiceUnavailable(t *testing.T) {
	mgr := NewRecommendationManager(RecommendationManagerConfig{
		RecommendationService: nil,
		Logger:                recTestLogger(),
	})

	router := chi.NewRouter()
	router.Get("/pathways/{pathwayID}/next-lab", mgr.handleGetPathwayNextLab)

	req := httptest.NewRequest(http.MethodGet, "/pathways/pathway-123/next-lab", nil)
	ctx := auth.ContextWithUser(req.Context(), &auth.User{ID: "user-123", Roles: []string{"student"}})
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()

	router.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusServiceUnavailable, rr.Code)
}
