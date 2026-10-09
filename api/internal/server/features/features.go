// Package features provides feature flag API handlers
package features

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/toddbartholow/kootenai/api/internal/auth"
	"github.com/toddbartholow/kootenai/api/internal/database/repositories"
	custommiddleware "github.com/toddbartholow/kootenai/api/internal/middleware"
	"github.com/toddbartholow/kootenai/api/internal/models"
)

// Feature IDs for common feature checks
const (
	FeatureTeams             = "teams"
	FeatureCustomLabs        = "labs.custom"
	FeatureFullAPI           = "api.full"
	FeatureFullAchievements  = "achievements.full"
	FeatureCanvasFull        = "canvas.full"
	FeatureSAMLSSO           = "sso.saml"
	FeatureOIDCSSO           = "sso.oidc"
	FeatureLDAPSSO           = "sso.ldap"
	FeatureCustomRoles       = "rbac.custom"
	FeatureABAC              = "rbac.abac"
	FeatureAdvancedAnalytics = "analytics.advanced"
	FeatureWebhooks          = "api.webhooks"
	FeatureMultiOrg          = "multi_org"
)

// Service handles feature flag operations
type Service struct {
	repo   repositories.FeatureRepository
	logger *slog.Logger
}

// NewService creates a new features service
func NewService(repo repositories.FeatureRepository, logger *slog.Logger) *Service {
	return &Service{
		repo:   repo,
		logger: logger,
	}
}

// ResponseWriter is an interface for HTTP response writing (allows dependency injection)
type ResponseWriter interface {
	JSONResponse(w http.ResponseWriter, status int, data any)
	ErrorResponse(w http.ResponseWriter, status int, message string)
	LocalizedErrorResponse(ctx context.Context, w http.ResponseWriter, status int, messageID string, templateData map[string]any)
}

// FeatureResponse represents a feature in API responses
type FeatureResponse struct {
	ID          string           `json:"id"`
	Name        string           `json:"name"`
	Description string           `json:"description,omitempty"`
	Editions    []models.Edition `json:"editions"`
	IsGlobal    bool             `json:"isGlobal"`
}

// OrgFeatureResponse represents a feature with enabled status for an organization
type OrgFeatureResponse struct {
	ID          string           `json:"id"`
	Name        string           `json:"name"`
	Description string           `json:"description,omitempty"`
	Enabled     bool             `json:"enabled"`
	Editions    []models.Edition `json:"editions"`
	IsGlobal    bool             `json:"isGlobal"`
}

// SetFeatureRequest represents a request to set a feature override
type SetFeatureRequest struct {
	Enabled   bool       `json:"enabled"`
	ExpiresAt *time.Time `json:"expiresAt,omitempty"`
}

// HandleListFeatures returns all available feature flags
func (svc *Service) HandleListFeatures(rw ResponseWriter) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if svc.repo == nil {
			rw.LocalizedErrorResponse(r.Context(), w, http.StatusInternalServerError, "features.errors.notConfigured", nil)
			return
		}

		features, err := svc.repo.List(r.Context())
		if err != nil {
			svc.logger.Error("Failed to list features", "error", err)
			rw.LocalizedErrorResponse(r.Context(), w, http.StatusInternalServerError, "features.errors.listFailed", nil)
			return
		}

		// Transform to response format (hide internal details)
		response := make([]FeatureResponse, len(features))
		for i, f := range features {
			response[i] = FeatureResponse{
				ID:          f.ID,
				Name:        f.Name,
				Description: f.Description,
				Editions:    f.Editions,
				IsGlobal:    f.IsGlobal,
			}
		}

		rw.JSONResponse(w, http.StatusOK, response)
	}
}

// HandleGetOrganizationFeatures returns features enabled for an organization
func (svc *Service) HandleGetOrganizationFeatures(rw ResponseWriter) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if svc.repo == nil {
			rw.LocalizedErrorResponse(r.Context(), w, http.StatusInternalServerError, "features.errors.notConfigured", nil)
			return
		}

		tc, ok := custommiddleware.TenantFromContext(r.Context())
		if !ok || tc == nil {
			rw.LocalizedErrorResponse(r.Context(), w, http.StatusBadRequest, "features.errors.orgRequired", nil)
			return
		}

		// Get all features with their enabled status
		features, err := svc.repo.List(r.Context())
		if err != nil {
			svc.logger.Error("Failed to list features", "error", err)
			rw.LocalizedErrorResponse(r.Context(), w, http.StatusInternalServerError, "features.errors.listFailed", nil)
			return
		}

		// Build response with enabled status from tenant context
		response := make([]OrgFeatureResponse, len(features))
		for i, f := range features {
			response[i] = OrgFeatureResponse{
				ID:          f.ID,
				Name:        f.Name,
				Description: f.Description,
				Enabled:     tc.HasFeature(f.ID),
				Editions:    f.Editions,
				IsGlobal:    f.IsGlobal,
			}
		}

		rw.JSONResponse(w, http.StatusOK, response)
	}
}

// HandleSetOrganizationFeature sets a feature override for an organization
func (svc *Service) HandleSetOrganizationFeature(rw ResponseWriter) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if svc.repo == nil {
			rw.LocalizedErrorResponse(r.Context(), w, http.StatusInternalServerError, "features.errors.notConfigured", nil)
			return
		}

		tc, ok := custommiddleware.TenantFromContext(r.Context())
		if !ok || tc == nil {
			rw.LocalizedErrorResponse(r.Context(), w, http.StatusBadRequest, "features.errors.orgRequired", nil)
			return
		}

		featureID := chi.URLParam(r, "featureID")
		if featureID == "" {
			rw.LocalizedErrorResponse(r.Context(), w, http.StatusBadRequest, "features.errors.idRequired", nil)
			return
		}

		// Verify feature exists
		feature, err := svc.repo.GetByID(r.Context(), featureID)
		if err != nil {
			svc.logger.Error("Failed to get feature", "error", err, "featureId", featureID)
			rw.LocalizedErrorResponse(r.Context(), w, http.StatusInternalServerError, "features.errors.getFailed", nil)
			return
		}
		if feature == nil {
			rw.LocalizedErrorResponse(r.Context(), w, http.StatusNotFound, "features.errors.notFound", nil)
			return
		}

		// Parse request
		var req SetFeatureRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			rw.LocalizedErrorResponse(r.Context(), w, http.StatusBadRequest, "features.errors.invalidRequestBody", nil)
			return
		}

		// Get current user for audit
		user, _ := auth.UserFromContext(r.Context())
		var grantedBy *string
		if user != nil {
			grantedBy = &user.ID
		}

		// Set the feature override
		orgFeature := &models.OrganizationFeature{
			OrganizationID: tc.Organization.ID,
			FeatureID:      featureID,
			Enabled:        req.Enabled,
			ExpiresAt:      req.ExpiresAt,
			GrantedBy:      grantedBy,
			GrantedAt:      time.Now(),
		}

		if err := svc.repo.SetOrganizationFeature(r.Context(), orgFeature); err != nil {
			svc.logger.Error("Failed to set feature", "error", err, "featureId", featureID)
			rw.LocalizedErrorResponse(r.Context(), w, http.StatusInternalServerError, "features.errors.setFailed", nil)
			return
		}

		rw.JSONResponse(w, http.StatusOK, map[string]any{
			"featureId": featureID,
			"enabled":   req.Enabled,
			"expiresAt": req.ExpiresAt,
		})
	}
}

// HandleRemoveOrganizationFeature removes a feature override for an organization
func (svc *Service) HandleRemoveOrganizationFeature(rw ResponseWriter) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if svc.repo == nil {
			rw.LocalizedErrorResponse(r.Context(), w, http.StatusInternalServerError, "features.errors.notConfigured", nil)
			return
		}

		tc, ok := custommiddleware.TenantFromContext(r.Context())
		if !ok || tc == nil {
			rw.LocalizedErrorResponse(r.Context(), w, http.StatusBadRequest, "features.errors.orgRequired", nil)
			return
		}

		featureID := chi.URLParam(r, "featureID")
		if featureID == "" {
			rw.LocalizedErrorResponse(r.Context(), w, http.StatusBadRequest, "features.errors.idRequired", nil)
			return
		}

		if err := svc.repo.RemoveOrganizationFeature(r.Context(), tc.Organization.ID, featureID); err != nil {
			svc.logger.Error("Failed to remove feature override", "error", err, "featureId", featureID)
			rw.LocalizedErrorResponse(r.Context(), w, http.StatusInternalServerError, "features.errors.removeFailed", nil)
			return
		}

		rw.JSONResponse(w, http.StatusOK, map[string]any{
			"featureId": featureID,
			"message":   "feature override removed",
		})
	}
}
