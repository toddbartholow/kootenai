package features

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/toddbartholow/kootenai/api/internal/auth"
	"github.com/toddbartholow/kootenai/api/internal/middleware"
	"github.com/toddbartholow/kootenai/api/internal/models"
)

// -----------------------------------------------------------------------------
// Mock Feature Repository
// -----------------------------------------------------------------------------

type mockFeatureRepo struct {
	features    []*models.FeatureFlag
	orgFeatures map[string]map[string]bool // orgID -> featureID -> enabled
	setError    error
	removeError error
	listError   error
	getError    error
}

func newMockFeatureRepo() *mockFeatureRepo {
	return &mockFeatureRepo{
		features: []*models.FeatureFlag{
			{
				ID:          "teams",
				Name:        "Team Management",
				Description: "Create and manage teams",
				Editions:    []models.Edition{models.EditionProfessional, models.EditionEnterprise},
				IsGlobal:    false,
				CreatedAt:   time.Now(),
			},
			{
				ID:          "labs.basic",
				Name:        "Basic Lab Access",
				Description: "Access to run labs",
				Editions:    []models.Edition{models.EditionCommunity, models.EditionProfessional, models.EditionEnterprise},
				IsGlobal:    true,
				CreatedAt:   time.Now(),
			},
			{
				ID:          "sso.saml",
				Name:        "SAML SSO",
				Description: "Single Sign-On via SAML",
				Editions:    []models.Edition{models.EditionEnterprise},
				IsGlobal:    false,
				CreatedAt:   time.Now(),
			},
		},
		orgFeatures: make(map[string]map[string]bool),
	}
}

func (r *mockFeatureRepo) GetByID(ctx context.Context, id string) (*models.FeatureFlag, error) {
	if r.getError != nil {
		return nil, r.getError
	}
	for _, f := range r.features {
		if f.ID == id {
			return f, nil
		}
	}
	return nil, nil
}

func (r *mockFeatureRepo) List(ctx context.Context) ([]*models.FeatureFlag, error) {
	if r.listError != nil {
		return nil, r.listError
	}
	return r.features, nil
}

func (r *mockFeatureRepo) ListByEdition(ctx context.Context, edition models.Edition) ([]*models.FeatureFlag, error) {
	if r.listError != nil {
		return nil, r.listError
	}
	var result []*models.FeatureFlag
	for _, f := range r.features {
		if f.IsGlobal {
			result = append(result, f)
			continue
		}
		for _, e := range f.Editions {
			if e == edition {
				result = append(result, f)
				break
			}
		}
	}
	return result, nil
}

func (r *mockFeatureRepo) IsFeatureEnabled(ctx context.Context, orgID, featureID string) (bool, error) {
	if orgFeatures, ok := r.orgFeatures[orgID]; ok {
		if enabled, ok := orgFeatures[featureID]; ok {
			return enabled, nil
		}
	}
	for _, f := range r.features {
		if f.ID == featureID {
			return f.IsGlobal, nil
		}
	}
	return false, nil
}

func (r *mockFeatureRepo) GetOrganizationFeatures(ctx context.Context, orgID string) (map[string]bool, error) {
	result := make(map[string]bool)
	for _, f := range r.features {
		result[f.ID] = f.IsGlobal
	}
	if orgFeatures, ok := r.orgFeatures[orgID]; ok {
		for k, v := range orgFeatures {
			result[k] = v
		}
	}
	return result, nil
}

func (r *mockFeatureRepo) SetOrganizationFeature(ctx context.Context, feature *models.OrganizationFeature) error {
	if r.setError != nil {
		return r.setError
	}
	if r.orgFeatures[feature.OrganizationID] == nil {
		r.orgFeatures[feature.OrganizationID] = make(map[string]bool)
	}
	r.orgFeatures[feature.OrganizationID][feature.FeatureID] = feature.Enabled
	return nil
}

func (r *mockFeatureRepo) RemoveOrganizationFeature(ctx context.Context, orgID, featureID string) error {
	if r.removeError != nil {
		return r.removeError
	}
	if orgFeatures, ok := r.orgFeatures[orgID]; ok {
		delete(orgFeatures, featureID)
	}
	return nil
}

// -----------------------------------------------------------------------------
// Mock Response Writer
// -----------------------------------------------------------------------------

type mockResponseWriter struct {
	lastStatus int
	lastData   any
}

func (rw *mockResponseWriter) JSONResponse(w http.ResponseWriter, status int, data any) {
	rw.lastStatus = status
	rw.lastData = data
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func (rw *mockResponseWriter) ErrorResponse(w http.ResponseWriter, status int, message string) {
	rw.JSONResponse(w, status, map[string]string{"error": message})
}

func (rw *mockResponseWriter) LocalizedErrorResponse(_ context.Context, w http.ResponseWriter, status int, messageID string, _ map[string]any) {
	rw.ErrorResponse(w, status, messageID)
}

// -----------------------------------------------------------------------------
// Test Helpers
// -----------------------------------------------------------------------------

func newTestLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelError}))
}

func createTenantContext(orgID string) *models.TenantContext {
	return &models.TenantContext{
		Organization: &models.Organization{
			ID:      orgID,
			Name:    "Test Org",
			Edition: models.EditionProfessional,
		},
		Features: map[string]bool{
			"teams":      true,
			"labs.basic": true,
			"sso.saml":   false,
		},
	}
}

func withTenantContext(req *http.Request, tc *models.TenantContext) *http.Request {
	ctx := context.WithValue(req.Context(), middleware.TenantContextKey{}, tc)
	return req.WithContext(ctx)
}

func withChiURLParam(req *http.Request, key, value string) *http.Request {
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add(key, value)
	return req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
}

// -----------------------------------------------------------------------------
// HandleListFeatures Tests
// -----------------------------------------------------------------------------

func TestHandleListFeatures(t *testing.T) {
	t.Run("success - list all features", func(t *testing.T) {
		featureRepo := newMockFeatureRepo()
		rw := &mockResponseWriter{}
		svc := NewService(featureRepo, newTestLogger())

		req := httptest.NewRequest(http.MethodGet, "/api/v1/features", nil)
		rec := httptest.NewRecorder()

		svc.HandleListFeatures(rw)(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d", http.StatusOK, rec.Code)
		}

		var response []map[string]any
		if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		if len(response) != 3 {
			t.Errorf("expected 3 features, got %d", len(response))
		}
	})

	t.Run("error - repo not configured", func(t *testing.T) {
		rw := &mockResponseWriter{}
		svc := NewService(nil, newTestLogger())

		req := httptest.NewRequest(http.MethodGet, "/api/v1/features", nil)
		rec := httptest.NewRecorder()

		svc.HandleListFeatures(rw)(rec, req)

		if rec.Code != http.StatusInternalServerError {
			t.Errorf("expected status %d, got %d", http.StatusInternalServerError, rec.Code)
		}
	})

	t.Run("error - repo list fails", func(t *testing.T) {
		featureRepo := newMockFeatureRepo()
		featureRepo.listError = errors.New("database error")
		rw := &mockResponseWriter{}
		svc := NewService(featureRepo, newTestLogger())

		req := httptest.NewRequest(http.MethodGet, "/api/v1/features", nil)
		rec := httptest.NewRecorder()

		svc.HandleListFeatures(rw)(rec, req)

		if rec.Code != http.StatusInternalServerError {
			t.Errorf("expected status %d, got %d", http.StatusInternalServerError, rec.Code)
		}
	})
}

// -----------------------------------------------------------------------------
// HandleGetOrganizationFeatures Tests
// -----------------------------------------------------------------------------

func TestHandleGetOrganizationFeatures(t *testing.T) {
	t.Run("success - get org features", func(t *testing.T) {
		featureRepo := newMockFeatureRepo()
		rw := &mockResponseWriter{}
		svc := NewService(featureRepo, newTestLogger())
		tc := createTenantContext("org-1")

		req := httptest.NewRequest(http.MethodGet, "/api/v1/organizations/org-1/features", nil)
		req = withTenantContext(req, tc)
		rec := httptest.NewRecorder()

		svc.HandleGetOrganizationFeatures(rw)(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d", http.StatusOK, rec.Code)
		}

		var response []map[string]any
		if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		if len(response) != 3 {
			t.Errorf("expected 3 features, got %d", len(response))
		}
		// Check that teams is enabled
		for _, f := range response {
			if f["id"] == "teams" && f["enabled"] != true {
				t.Error("expected teams to be enabled")
			}
		}
	})

	t.Run("error - no tenant context", func(t *testing.T) {
		featureRepo := newMockFeatureRepo()
		rw := &mockResponseWriter{}
		svc := NewService(featureRepo, newTestLogger())

		req := httptest.NewRequest(http.MethodGet, "/api/v1/organizations/org-1/features", nil)
		rec := httptest.NewRecorder()

		svc.HandleGetOrganizationFeatures(rw)(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
		}
	})

	t.Run("error - repo not configured", func(t *testing.T) {
		rw := &mockResponseWriter{}
		svc := NewService(nil, newTestLogger())
		tc := createTenantContext("org-1")

		req := httptest.NewRequest(http.MethodGet, "/api/v1/organizations/org-1/features", nil)
		req = withTenantContext(req, tc)
		rec := httptest.NewRecorder()

		svc.HandleGetOrganizationFeatures(rw)(rec, req)

		if rec.Code != http.StatusInternalServerError {
			t.Errorf("expected status %d, got %d", http.StatusInternalServerError, rec.Code)
		}
	})

	t.Run("error - repo list fails", func(t *testing.T) {
		featureRepo := newMockFeatureRepo()
		featureRepo.listError = errors.New("database error")
		rw := &mockResponseWriter{}
		svc := NewService(featureRepo, newTestLogger())
		tc := createTenantContext("org-1")

		req := httptest.NewRequest(http.MethodGet, "/api/v1/organizations/org-1/features", nil)
		req = withTenantContext(req, tc)
		rec := httptest.NewRecorder()

		svc.HandleGetOrganizationFeatures(rw)(rec, req)

		if rec.Code != http.StatusInternalServerError {
			t.Errorf("expected status %d, got %d", http.StatusInternalServerError, rec.Code)
		}
	})
}

// -----------------------------------------------------------------------------
// HandleSetOrganizationFeature Tests
// -----------------------------------------------------------------------------

func TestHandleSetOrganizationFeature(t *testing.T) {
	t.Run("success - enable feature", func(t *testing.T) {
		featureRepo := newMockFeatureRepo()
		rw := &mockResponseWriter{}
		svc := NewService(featureRepo, newTestLogger())
		tc := createTenantContext("org-123")

		req := httptest.NewRequest(http.MethodPost, "/api/v1/organizations/org-123/features/teams", strings.NewReader(`{"enabled": true}`))
		req.Header.Set("Content-Type", "application/json")
		req = withTenantContext(req, tc)
		req = withChiURLParam(req, "featureID", "teams")

		// Add auth user
		authUser := &auth.User{ID: "user-123", Roles: []string{"admin"}}
		ctx := auth.ContextWithUser(req.Context(), authUser)
		req = req.WithContext(ctx)

		rec := httptest.NewRecorder()

		svc.HandleSetOrganizationFeature(rw)(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d: %s", http.StatusOK, rec.Code, rec.Body.String())
		}
	})

	t.Run("success - disable feature", func(t *testing.T) {
		featureRepo := newMockFeatureRepo()
		rw := &mockResponseWriter{}
		svc := NewService(featureRepo, newTestLogger())
		tc := createTenantContext("org-123")

		req := httptest.NewRequest(http.MethodPost, "/api/v1/organizations/org-123/features/teams", strings.NewReader(`{"enabled": false}`))
		req.Header.Set("Content-Type", "application/json")
		req = withTenantContext(req, tc)
		req = withChiURLParam(req, "featureID", "teams")

		rec := httptest.NewRecorder()

		svc.HandleSetOrganizationFeature(rw)(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d: %s", http.StatusOK, rec.Code, rec.Body.String())
		}
	})

	t.Run("error - no tenant context", func(t *testing.T) {
		featureRepo := newMockFeatureRepo()
		rw := &mockResponseWriter{}
		svc := NewService(featureRepo, newTestLogger())

		req := httptest.NewRequest(http.MethodPost, "/api/v1/organizations/org-123/features/teams", strings.NewReader(`{"enabled": true}`))
		req.Header.Set("Content-Type", "application/json")
		req = withChiURLParam(req, "featureID", "teams")

		rec := httptest.NewRecorder()

		svc.HandleSetOrganizationFeature(rw)(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
		}
	})

	t.Run("error - invalid JSON", func(t *testing.T) {
		featureRepo := newMockFeatureRepo()
		rw := &mockResponseWriter{}
		svc := NewService(featureRepo, newTestLogger())
		tc := createTenantContext("org-123")

		req := httptest.NewRequest(http.MethodPost, "/api/v1/organizations/org-123/features/teams", strings.NewReader(`{invalid}`))
		req.Header.Set("Content-Type", "application/json")
		req = withTenantContext(req, tc)
		req = withChiURLParam(req, "featureID", "teams")

		rec := httptest.NewRecorder()

		svc.HandleSetOrganizationFeature(rw)(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
		}
	})

	t.Run("error - feature not found", func(t *testing.T) {
		featureRepo := newMockFeatureRepo()
		rw := &mockResponseWriter{}
		svc := NewService(featureRepo, newTestLogger())
		tc := createTenantContext("org-123")

		req := httptest.NewRequest(http.MethodPost, "/api/v1/organizations/org-123/features/nonexistent", strings.NewReader(`{"enabled": true}`))
		req.Header.Set("Content-Type", "application/json")
		req = withTenantContext(req, tc)
		req = withChiURLParam(req, "featureID", "nonexistent")

		rec := httptest.NewRecorder()

		svc.HandleSetOrganizationFeature(rw)(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Errorf("expected status %d, got %d", http.StatusNotFound, rec.Code)
		}
	})

	t.Run("error - repo fails", func(t *testing.T) {
		featureRepo := newMockFeatureRepo()
		featureRepo.setError = errors.New("database error")
		rw := &mockResponseWriter{}
		svc := NewService(featureRepo, newTestLogger())
		tc := createTenantContext("org-123")

		req := httptest.NewRequest(http.MethodPost, "/api/v1/organizations/org-123/features/teams", strings.NewReader(`{"enabled": true}`))
		req.Header.Set("Content-Type", "application/json")
		req = withTenantContext(req, tc)
		req = withChiURLParam(req, "featureID", "teams")

		rec := httptest.NewRecorder()

		svc.HandleSetOrganizationFeature(rw)(rec, req)

		if rec.Code != http.StatusInternalServerError {
			t.Errorf("expected status %d, got %d", http.StatusInternalServerError, rec.Code)
		}
	})

	t.Run("error - empty feature ID", func(t *testing.T) {
		featureRepo := newMockFeatureRepo()
		rw := &mockResponseWriter{}
		svc := NewService(featureRepo, newTestLogger())
		tc := createTenantContext("org-123")

		req := httptest.NewRequest(http.MethodPost, "/api/v1/organizations/org-123/features/", strings.NewReader(`{"enabled": true}`))
		req.Header.Set("Content-Type", "application/json")
		req = withTenantContext(req, tc)
		req = withChiURLParam(req, "featureID", "")

		rec := httptest.NewRecorder()

		svc.HandleSetOrganizationFeature(rw)(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
		}
	})

	t.Run("error - repo not configured", func(t *testing.T) {
		rw := &mockResponseWriter{}
		svc := NewService(nil, newTestLogger())
		tc := createTenantContext("org-123")

		req := httptest.NewRequest(http.MethodPost, "/api/v1/organizations/org-123/features/teams", strings.NewReader(`{"enabled": true}`))
		req.Header.Set("Content-Type", "application/json")
		req = withTenantContext(req, tc)
		req = withChiURLParam(req, "featureID", "teams")

		rec := httptest.NewRecorder()

		svc.HandleSetOrganizationFeature(rw)(rec, req)

		if rec.Code != http.StatusInternalServerError {
			t.Errorf("expected status %d, got %d", http.StatusInternalServerError, rec.Code)
		}
	})
}

// -----------------------------------------------------------------------------
// HandleRemoveOrganizationFeature Tests
// -----------------------------------------------------------------------------

func TestHandleRemoveOrganizationFeature(t *testing.T) {
	t.Run("success - remove feature", func(t *testing.T) {
		featureRepo := newMockFeatureRepo()
		rw := &mockResponseWriter{}
		svc := NewService(featureRepo, newTestLogger())
		tc := createTenantContext("org-123")

		req := httptest.NewRequest(http.MethodDelete, "/api/v1/organizations/org-123/features/teams", nil)
		req = withTenantContext(req, tc)
		req = withChiURLParam(req, "featureID", "teams")

		rec := httptest.NewRecorder()

		svc.HandleRemoveOrganizationFeature(rw)(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d: %s", http.StatusOK, rec.Code, rec.Body.String())
		}
	})

	t.Run("error - no tenant context", func(t *testing.T) {
		featureRepo := newMockFeatureRepo()
		rw := &mockResponseWriter{}
		svc := NewService(featureRepo, newTestLogger())

		req := httptest.NewRequest(http.MethodDelete, "/api/v1/organizations/org-123/features/teams", nil)
		req = withChiURLParam(req, "featureID", "teams")

		rec := httptest.NewRecorder()

		svc.HandleRemoveOrganizationFeature(rw)(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
		}
	})

	t.Run("error - empty feature ID", func(t *testing.T) {
		featureRepo := newMockFeatureRepo()
		rw := &mockResponseWriter{}
		svc := NewService(featureRepo, newTestLogger())
		tc := createTenantContext("org-123")

		req := httptest.NewRequest(http.MethodDelete, "/api/v1/organizations/org-123/features/", nil)
		req = withTenantContext(req, tc)
		req = withChiURLParam(req, "featureID", "")

		rec := httptest.NewRecorder()

		svc.HandleRemoveOrganizationFeature(rw)(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
		}
	})

	t.Run("error - repo fails", func(t *testing.T) {
		featureRepo := newMockFeatureRepo()
		featureRepo.removeError = errors.New("database error")
		rw := &mockResponseWriter{}
		svc := NewService(featureRepo, newTestLogger())
		tc := createTenantContext("org-123")

		req := httptest.NewRequest(http.MethodDelete, "/api/v1/organizations/org-123/features/teams", nil)
		req = withTenantContext(req, tc)
		req = withChiURLParam(req, "featureID", "teams")

		rec := httptest.NewRecorder()

		svc.HandleRemoveOrganizationFeature(rw)(rec, req)

		if rec.Code != http.StatusInternalServerError {
			t.Errorf("expected status %d, got %d", http.StatusInternalServerError, rec.Code)
		}
	})

	t.Run("error - repo not configured", func(t *testing.T) {
		rw := &mockResponseWriter{}
		svc := NewService(nil, newTestLogger())
		tc := createTenantContext("org-123")

		req := httptest.NewRequest(http.MethodDelete, "/api/v1/organizations/org-123/features/teams", nil)
		req = withTenantContext(req, tc)
		req = withChiURLParam(req, "featureID", "teams")

		rec := httptest.NewRecorder()

		svc.HandleRemoveOrganizationFeature(rw)(rec, req)

		if rec.Code != http.StatusInternalServerError {
			t.Errorf("expected status %d, got %d", http.StatusInternalServerError, rec.Code)
		}
	})
}
