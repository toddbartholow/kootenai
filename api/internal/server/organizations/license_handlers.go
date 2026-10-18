package organizations

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"

	"github.com/toddbartholow/kootenai/api/internal/middleware"
	"github.com/toddbartholow/kootenai/api/internal/models"
	"github.com/toddbartholow/kootenai/api/internal/server/serverutil"
)

// -----------------------------------------------------------------------------
// License Handlers
// -----------------------------------------------------------------------------

// ErrorResponse is an alias for serverutil.ErrorResponse.
//
// swag resolves an unqualified name in an annotation against the package that
// annotation lives in, so the name has to exist here. An alias rather than a
// copy keeps one canonical struct, and unlike a doc comment it counts as a use
// of the import. Same idiom as server.ErrorResponse in auth_handlers.go.
type ErrorResponse = serverutil.ErrorResponse

// ActivateLicenseRequest represents the request body for activating a license
type ActivateLicenseRequest struct {
	LicenseKey string `json:"licenseKey"`
}

// LicenseResponse represents the public license information
type LicenseResponse struct {
	ID               string               `json:"id"`
	Edition          models.Edition       `json:"edition"`
	MaskedKey        string               `json:"maskedKey"`
	IssuedAt         time.Time            `json:"issuedAt"`
	ExpiresAt        *time.Time           `json:"expiresAt,omitempty"`
	MaxUsers         *int                 `json:"maxUsers,omitempty"`
	MaxPods          *int                 `json:"maxPods,omitempty"`
	MaxStorageGB     *int                 `json:"maxStorageGb,omitempty"`
	Features         []string             `json:"features,omitempty"`
	IsActive         bool                 `json:"isActive"`
	ValidationStatus models.LicenseStatus `json:"validationStatus"`
	LastValidatedAt  *time.Time           `json:"lastValidatedAt,omitempty"`
	IsValid          bool                 `json:"isValid"`
	DaysRemaining    *int                 `json:"daysRemaining,omitempty"`
}

// handleGetLicense gets the current license for the organization
// @Summary Get organization license
// @Description Returns the current active license for the organization
// @Tags licenses
// @Accept json
// @Produce json
// @Security BearerAuth
// @Success 200 {object} LicenseResponse
// @Failure 401 {object} ErrorResponse "Unauthorized"
// @Failure 403 {object} ErrorResponse "Forbidden"
// @Failure 404 {object} ErrorResponse "No active license"
// @Router /organizations/{orgId}/license [get]
func (m *Manager) handleGetLicense() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tc, ok := middleware.TenantFromContext(r.Context())
		if !ok || tc.Organization == nil {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusBadRequest, "license.errors.orgContextRequired", nil)
			return
		}

		// Require admin role
		if !tc.HasRole(models.OrgRoleAdmin) {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusForbidden, "license.errors.adminRequired", nil)
			return
		}

		if m.licenseRepo == nil {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusServiceUnavailable, "license.errors.serviceNotConfigured", nil)
			return
		}

		license, err := m.licenseRepo.GetActiveByOrganization(r.Context(), tc.Organization.ID)
		if err != nil {
			m.logger.Error("Failed to get license", "error", err, "orgId", tc.Organization.ID)
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusInternalServerError, "license.errors.getFailed", nil)
			return
		}

		if license == nil {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusNotFound, "license.errors.noActiveLicense", nil)
			return
		}

		m.responder.JSONResponse(w, http.StatusOK, toLicenseResponse(license))
	}
}

// handleActivateLicense activates a new license for the organization
// @Summary Activate license
// @Description Activates a new license key for the organization
// @Tags licenses
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body ActivateLicenseRequest true "License key"
// @Success 200 {object} LicenseResponse
// @Failure 400 {object} ErrorResponse "Invalid request"
// @Failure 401 {object} ErrorResponse "Unauthorized"
// @Failure 403 {object} ErrorResponse "Forbidden"
// @Failure 409 {object} ErrorResponse "License already in use"
// @Router /organizations/{orgId}/license [post]
func (m *Manager) handleActivateLicense() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tc, ok := middleware.TenantFromContext(r.Context())
		if !ok || tc.Organization == nil {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusBadRequest, "license.errors.orgContextRequired", nil)
			return
		}

		// Require owner role for license activation
		if !tc.HasRole(models.OrgRoleOwner) {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusForbidden, "license.errors.ownerRequired", nil)
			return
		}

		if m.licenseRepo == nil {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusServiceUnavailable, "license.errors.serviceNotConfigured", nil)
			return
		}

		var req ActivateLicenseRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusBadRequest, "error.invalidRequestBody", nil)
			return
		}

		if req.LicenseKey == "" {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusBadRequest, "license.errors.keyRequired", nil)
			return
		}

		// Check if license key is already used
		existing, err := m.licenseRepo.GetByLicenseKey(r.Context(), req.LicenseKey)
		if err != nil {
			m.logger.Error("Failed to check license key", "error", err)
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusInternalServerError, "license.errors.activateFailed", nil)
			return
		}
		if existing != nil && existing.OrganizationID != tc.Organization.ID {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusConflict, "license.errors.keyInUse", nil)
			return
		}

		// Validate license key format and extract info
		// In a real system, this would validate against a license server
		licenseInfo, err := validateLicenseKey(req.LicenseKey)
		if err != nil {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusBadRequest, "license.errors.invalidKey", nil)
			return
		}

		// Deactivate any existing license for this org
		existingOrgLicense, _ := m.licenseRepo.GetActiveByOrganization(r.Context(), tc.Organization.ID)
		if existingOrgLicense != nil {
			if err := m.licenseRepo.Deactivate(r.Context(), existingOrgLicense.ID); err != nil {
				m.logger.Warn("Failed to deactivate old license", "error", err)
			}
		}

		now := time.Now()
		license := &models.License{
			ID:               uuid.New().String(),
			OrganizationID:   tc.Organization.ID,
			LicenseKey:       req.LicenseKey,
			Edition:          licenseInfo.Edition,
			IssuedAt:         now,
			ExpiresAt:        licenseInfo.ExpiresAt,
			MaxUsers:         licenseInfo.MaxUsers,
			MaxPods:          licenseInfo.MaxPods,
			MaxStorageGB:     licenseInfo.MaxStorageGB,
			Features:         licenseInfo.Features,
			IsActive:         true,
			ValidationStatus: models.LicenseStatusValid,
			LastValidatedAt:  &now,
		}

		if err := m.licenseRepo.Create(r.Context(), license); err != nil {
			m.logger.Error("Failed to create license", "error", err)
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusInternalServerError, "license.errors.activateFailed", nil)
			return
		}

		// Update organization's edition and limits
		if m.orgRepo != nil {
			tc.Organization.Edition = license.Edition
			tc.Organization.LicenseExpiresAt = license.ExpiresAt
			tc.Organization.MaxUsers = license.MaxUsers
			tc.Organization.MaxConcurrentPods = license.MaxPods
			tc.Organization.MaxStorageGB = license.MaxStorageGB

			if err := m.orgRepo.Update(r.Context(), tc.Organization); err != nil {
				m.logger.Warn("Failed to update organization limits", "error", err)
			}
		}

		m.logger.Info("License activated",
			"orgId", tc.Organization.ID,
			"edition", license.Edition,
			"expiresAt", license.ExpiresAt,
		)

		m.responder.JSONResponse(w, http.StatusOK, toLicenseResponse(license))
	}
}

// handleDeactivateLicense deactivates the current license
// @Summary Deactivate license
// @Description Deactivates the current license for the organization
// @Tags licenses
// @Accept json
// @Produce json
// @Security BearerAuth
// @Success 200 {object} map[string]string
// @Failure 401 {object} ErrorResponse "Unauthorized"
// @Failure 403 {object} ErrorResponse "Forbidden"
// @Failure 404 {object} ErrorResponse "No active license"
// @Router /organizations/{orgId}/license [delete]
func (m *Manager) handleDeactivateLicense() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tc, ok := middleware.TenantFromContext(r.Context())
		if !ok || tc.Organization == nil {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusBadRequest, "license.errors.orgContextRequired", nil)
			return
		}

		// Require owner role
		if !tc.HasRole(models.OrgRoleOwner) {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusForbidden, "license.errors.ownerRequired", nil)
			return
		}

		if m.licenseRepo == nil {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusServiceUnavailable, "license.errors.serviceNotConfigured", nil)
			return
		}

		license, err := m.licenseRepo.GetActiveByOrganization(r.Context(), tc.Organization.ID)
		if err != nil {
			m.logger.Error("Failed to get license", "error", err)
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusInternalServerError, "license.errors.deactivateFailed", nil)
			return
		}

		if license == nil {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusNotFound, "license.errors.noActiveLicense", nil)
			return
		}

		if err := m.licenseRepo.Deactivate(r.Context(), license.ID); err != nil {
			m.logger.Error("Failed to deactivate license", "error", err)
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusInternalServerError, "license.errors.deactivateFailed", nil)
			return
		}

		// Downgrade organization to community edition
		if m.orgRepo != nil {
			tc.Organization.Edition = models.EditionCommunity
			tc.Organization.LicenseExpiresAt = nil
			tc.Organization.MaxUsers = nil
			tc.Organization.MaxConcurrentPods = nil
			tc.Organization.MaxStorageGB = nil

			if err := m.orgRepo.Update(r.Context(), tc.Organization); err != nil {
				m.logger.Warn("Failed to update organization", "error", err)
			}
		}

		m.logger.Info("License deactivated", "orgId", tc.Organization.ID)

		m.responder.JSONResponse(w, http.StatusOK, map[string]string{
			"message": "license deactivated",
		})
	}
}

// handleValidateLicense manually triggers license validation
// @Summary Validate license
// @Description Validates the current license against the license server
// @Tags licenses
// @Accept json
// @Produce json
// @Security BearerAuth
// @Success 200 {object} LicenseResponse
// @Failure 401 {object} ErrorResponse "Unauthorized"
// @Failure 403 {object} ErrorResponse "Forbidden"
// @Failure 404 {object} ErrorResponse "No active license"
// @Router /organizations/{orgId}/license/validate [post]
func (m *Manager) handleValidateLicense() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tc, ok := middleware.TenantFromContext(r.Context())
		if !ok || tc.Organization == nil {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusBadRequest, "license.errors.orgContextRequired", nil)
			return
		}

		// Require admin role
		if !tc.HasRole(models.OrgRoleAdmin) {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusForbidden, "license.errors.adminRequired", nil)
			return
		}

		if m.licenseRepo == nil {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusServiceUnavailable, "license.errors.serviceNotConfigured", nil)
			return
		}

		license, err := m.licenseRepo.GetActiveByOrganization(r.Context(), tc.Organization.ID)
		if err != nil {
			m.logger.Error("Failed to get license", "error", err)
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusInternalServerError, "license.errors.validateFailed", nil)
			return
		}

		if license == nil {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusNotFound, "license.errors.noActiveLicense", nil)
			return
		}

		// In a real system, this would call the license server
		// For now, just check expiration
		now := time.Now()
		var status models.LicenseStatus
		var errMsg *string

		if license.ExpiresAt != nil && now.After(*license.ExpiresAt) {
			status = models.LicenseStatusExpired
			msg := "license has expired"
			errMsg = &msg
		} else {
			status = models.LicenseStatusValid
		}

		if err := m.licenseRepo.UpdateValidationStatus(r.Context(), license.ID, status, errMsg); err != nil {
			m.logger.Error("Failed to update validation status", "error", err)
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusInternalServerError, "license.errors.validateFailed", nil)
			return
		}

		license.ValidationStatus = status
		license.LastValidatedAt = &now
		license.ValidationError = errMsg

		m.responder.JSONResponse(w, http.StatusOK, toLicenseResponse(license))
	}
}

// -----------------------------------------------------------------------------
// Helper Functions
// -----------------------------------------------------------------------------

// licenseInfo contains decoded license information
type licenseInfo struct {
	Edition      models.Edition
	ExpiresAt    *time.Time
	MaxUsers     *int
	MaxPods      *int
	MaxStorageGB *int
	Features     []string
}

// validateLicenseKey validates and decodes a license key
// In a production system, this would validate against a license server
// or use cryptographic verification
func validateLicenseKey(key string) (*licenseInfo, error) {
	// Simple validation: key format is EDITION-HASH where EDITION is PRO or ENT
	// In production, this would be proper cryptographic validation
	if len(key) < 8 {
		return nil, fmt.Errorf("invalid license key format")
	}

	// Extract edition from key prefix for demo purposes
	var edition models.Edition
	switch {
	case len(key) >= 3 && key[:3] == "ENT":
		edition = models.EditionEnterprise
	case len(key) >= 3 && key[:3] == "PRO":
		edition = models.EditionProfessional
	default:
		edition = models.EditionProfessional
	}

	// Set default limits based on edition
	var maxUsers, maxPods, maxStorage int
	var features []string

	switch edition {
	case models.EditionEnterprise:
		maxUsers = 1000
		maxPods = 100
		maxStorage = 1000
		features = []string{
			"teams", "labs.custom", "achievements.full", "api.full",
			"analytics.advanced", "canvas.full", "audit.compliance",
			"sso.saml", "sso.oidc", "sso.ldap", "rbac.custom", "rbac.abac",
			"branding.full", "api.webhooks", "multi_org",
		}
	case models.EditionProfessional:
		maxUsers = 100
		maxPods = 20
		maxStorage = 100
		features = []string{
			"teams", "labs.custom", "achievements.full", "api.full",
			"analytics.standard", "canvas.full", "audit.extended",
		}
	default:
		maxUsers = 10
		maxPods = 5
		maxStorage = 10
		features = []string{"labs.basic", "achievements.basic", "api.readonly"}
	}

	// Set expiration to 1 year from now
	expires := time.Now().AddDate(1, 0, 0)

	return &licenseInfo{
		Edition:      edition,
		ExpiresAt:    &expires,
		MaxUsers:     &maxUsers,
		MaxPods:      &maxPods,
		MaxStorageGB: &maxStorage,
		Features:     features,
	}, nil
}

// toLicenseResponse converts a license to a public response
func toLicenseResponse(license *models.License) LicenseResponse {
	response := LicenseResponse{
		ID:               license.ID,
		Edition:          license.Edition,
		MaskedKey:        license.MaskedKey(),
		IssuedAt:         license.IssuedAt,
		ExpiresAt:        license.ExpiresAt,
		MaxUsers:         license.MaxUsers,
		MaxPods:          license.MaxPods,
		MaxStorageGB:     license.MaxStorageGB,
		Features:         license.Features,
		IsActive:         license.IsActive,
		ValidationStatus: license.ValidationStatus,
		LastValidatedAt:  license.LastValidatedAt,
		IsValid:          license.IsValid(),
	}

	// Calculate days remaining
	if license.ExpiresAt != nil {
		days := int(time.Until(*license.ExpiresAt).Hours() / 24)
		if days < 0 {
			days = 0
		}
		response.DaysRemaining = &days
	}

	return response
}

// generateLicenseKey generates a new license key (admin/testing only)
func generateLicenseKey(edition models.Edition) string {
	prefix := "COM"
	switch edition {
	case models.EditionEnterprise:
		prefix = "ENT"
	case models.EditionProfessional:
		prefix = "PRO"
	}

	// Generate a unique hash
	timestamp := time.Now().UnixNano()
	hash := sha256.Sum256([]byte(fmt.Sprintf("%s-%d-%s", prefix, timestamp, uuid.New().String())))
	hashStr := hex.EncodeToString(hash[:])

	// Format: PREFIX-XXXX-XXXX-XXXX-XXXX
	return fmt.Sprintf("%s-%s-%s-%s-%s",
		prefix,
		hashStr[0:4],
		hashStr[4:8],
		hashStr[8:12],
		hashStr[12:16],
	)
}
