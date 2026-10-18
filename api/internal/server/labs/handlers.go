package labs

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"gopkg.in/yaml.v3"

	"github.com/toddbartholow/kootenai/api/internal/auth"
	"github.com/toddbartholow/kootenai/api/internal/database/repositories"
	custommiddleware "github.com/toddbartholow/kootenai/api/internal/middleware"
	"github.com/toddbartholow/kootenai/api/internal/models"
	"github.com/toddbartholow/kootenai/api/internal/server/serverutil"
)

// -----------------------------------------------------------------------------
// DTOs
// -----------------------------------------------------------------------------
//
// These exist to give the swagger annotations below a resolvable type. swag
// resolves an unqualified name against the package the annotation lives in, so
// they have to be declared here rather than in the parent server package --
// which labs cannot import anyway, since server imports labs.
//
// None of these is constructed in Go: the handlers write map[string]any. They
// describe the wire shape for the generated clients only, so they must be kept
// in step with the literals below by hand.
//
// Note they are deliberately not models.LabTemplate. That type is the *YAML
// document* shape -- apiVersion, kind, metadata, spec. What these endpoints
// return is a models.LabTemplateRecord flattened into a summary, which is a
// different thing with different field names.

// LabSummary is the per-lab projection written by handleListLabs, and the base
// of what handleGetLab returns.
type LabSummary struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Slug        string `json:"slug"`
	Description string `json:"description"`
	Version     string `json:"version"`
	// Typed rather than plain string so generated clients keep the enum.
	Platform        models.Platform      `json:"platform"`
	DurationMinutes int                  `json:"durationMinutes"`
	Difficulty      string               `json:"difficulty"`
	Category        string               `json:"category"`
	Tags            []string             `json:"tags"`
	MaxPoints       int                  `json:"maxPoints"`
	PassThreshold   int                  `json:"passThreshold"`
	IsActive        bool                 `json:"isActive"`
	Visibility      models.LabVisibility `json:"visibility"`
	CreatedAt       time.Time            `json:"createdAt"`
	UpdatedAt       time.Time            `json:"updatedAt"`
	// Both written only when the record has them.
	OrganizationID string `json:"organizationId,omitempty"`
	// The web client gates the edit action on this, so omitting it locked
	// creators out of their own labs.
	CreatedBy string `json:"createdBy,omitempty"`
}

// ListLabsResponse represents the response for listing lab templates.
type ListLabsResponse struct {
	Labs  []LabSummary `json:"labs"`
	Count int          `json:"count"`
	Total int          `json:"total"`
	// Always written, so not omitempty.
	Pagination serverutil.PaginationMeta `json:"pagination"`
}

// LabResponse is a single lab template. The summary fields are written flat --
// there is no enclosing "lab" object -- plus spec and checkpoints when the
// caller passes include_spec=true.
// Both spec and checkpoints need an explicit swaggertype: swag resolves
// json.RawMessage to its underlying []byte and would otherwise describe them as
// arrays of integers. They are not the same shape as each other -- spec is the
// lab spec object, checkpoints is json.Marshal of []Checkpoint, an array -- so
// they get different tags.
type LabResponse struct {
	LabSummary
	Spec        json.RawMessage `json:"spec,omitempty" swaggertype:"object"`
	Checkpoints json.RawMessage `json:"checkpoints,omitempty" swaggertype:"array,object"`
}

// -----------------------------------------------------------------------------
// Lab Handlers
// -----------------------------------------------------------------------------

// handleListLabs godoc
// @Summary List Lab Templates
// @Description Get all lab templates, optionally filtered by platform or active status
// @Tags labs
// @Accept json
// @Produce json
// @Param platform query string false "Filter by platform (proxmox, cloudstack)"
// @Param active query boolean false "Filter by active status (default: true)"
// @Param visibility query string false "Filter by visibility (global, organization, private)"
// @Param all query boolean false "Show all labs (admin only, bypasses org filtering)"
// @Success 200 {object} ListLabsResponse
// @Failure 500 {object} serverutil.ErrorResponse
// @Failure 503 {object} serverutil.ErrorResponse "Lab template repository not configured"
// @Security BearerAuth
// @Router /labs [get]
func (m *Manager) handleListLabs() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if m.labTemplateRepo == nil {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusServiceUnavailable, "labTemplate.errors.repoNotConfigured", nil)
			return
		}

		// Get user from context for org filtering
		user, _ := auth.UserFromContext(r.Context())

		// Parse query parameters for filtering
		platform := r.URL.Query().Get("platform")
		activeParam := r.URL.Query().Get("active")
		visibility := r.URL.Query().Get("visibility")
		showAll := r.URL.Query().Get("all") == "true"

		// Parse pagination params
		pagination := serverutil.ParsePagination(r, 50, 500)

		filter := repositories.LabTemplateFilter{
			Platform: platform,
			Limit:    pagination.Limit,
			Offset:   pagination.Offset,
		}

		// Handle active filter: "all" shows both, "false" shows inactive, default shows active only
		if activeParam != "all" {
			activeOnly := activeParam != "false"
			filter.Active = &activeOnly
		}

		// Apply visibility filter if specified
		if visibility != "" {
			filter.Visibility = models.LabVisibility(visibility)
		}

		// Apply org-scoped filtering based on tenant context (from X-Organization
		// header or URL path), falling back to user's default org.
		// Admins with ?all=true can bypass org filtering.
		tc := tenantFromCtx(r.Context())
		if showAll && user != nil && serverutil.IsAdminOrInstructor(user) {
			// Admin requesting all labs - no org filter
		} else if tc != nil && tc.Organization != nil {
			filter.OrganizationID = tc.Organization.ID
			filter.IncludeGlobal = true
		} else if user != nil && user.DefaultOrganizationID != "" {
			filter.OrganizationID = user.DefaultOrganizationID
			filter.IncludeGlobal = true
		} else {
			// No org context - only show global labs
			filter.Visibility = models.LabVisibilityGlobal
		}

		records, err := m.labTemplateRepo.List(r.Context(), filter)
		if err != nil {
			m.logger.Error("Failed to list lab templates", "error", err)
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusInternalServerError, "labTemplate.errors.listFailed", nil)
			return
		}

		// Convert records to API response format
		labs := make([]map[string]any, 0, len(records))
		for _, record := range records {
			lab := map[string]any{
				"id":              record.ID,
				"name":            record.Name,
				"slug":            record.Slug,
				"description":     record.Description,
				"version":         record.Version,
				"platform":        record.Platform,
				"durationMinutes": record.DurationMinutes,
				"difficulty":      record.Difficulty,
				"category":        record.Category,
				"tags":            record.Tags,
				"maxPoints":       record.MaxPoints,
				"passThreshold":   record.PassThreshold,
				"isActive":        record.IsActive,
				"visibility":      record.Visibility,
				"createdAt":       record.CreatedAt,
				"updatedAt":       record.UpdatedAt,
			}
			// Include organization ID if present
			if record.OrganizationID != nil {
				lab["organizationId"] = *record.OrganizationID
			}
			if record.CreatedBy != nil {
				lab["createdBy"] = *record.CreatedBy
			}
			labs = append(labs, lab)
		}

		// Calculate total - for now we use len(records) since we don't have a separate count query
		// When count < limit, we know we have all records; otherwise there may be more
		total := len(labs)
		if pagination.Offset == 0 && len(labs) < pagination.Limit {
			// No pagination needed, total is exact
		} else if len(labs) == pagination.Limit {
			// There may be more records, indicate that we don't know the true total
			// For now, use a simple heuristic: total = offset + count + 1 (to show hasMore=true)
			total = pagination.Offset + len(labs) + 1
		} else {
			// Partial page, so this is the last page
			total = pagination.Offset + len(labs)
		}

		paginationMeta := serverutil.NewPaginationMeta(total, pagination.Limit, pagination.Offset)

		m.responder.JSONResponse(w, http.StatusOK, map[string]any{
			"labs":       labs,
			"count":      len(labs),
			"total":      total,
			"pagination": paginationMeta,
		})
	}
}

// handleGetLab godoc
// @Summary Get Lab Template
// @Description Get detailed information about a specific lab template
// @Tags labs
// @Accept json
// @Produce json
// @Param labID path string true "Lab template ID or name"
// @Param include_spec query boolean false "Include full lab specification"
// @Success 200 {object} LabResponse
// @Failure 400 {object} serverutil.ErrorResponse "Invalid lab ID"
// @Failure 403 {object} serverutil.ErrorResponse "Access denied"
// @Failure 404 {object} serverutil.ErrorResponse "Lab template not found"
// @Failure 500 {object} serverutil.ErrorResponse
// @Failure 503 {object} serverutil.ErrorResponse "Lab template repository not configured"
// @Security BearerAuth
// @Router /labs/{labID} [get]
func (m *Manager) handleGetLab() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if m.labTemplateRepo == nil {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusServiceUnavailable, "labTemplate.errors.repoNotConfigured", nil)
			return
		}

		labID := chi.URLParam(r, "labID")

		// Validate the labID parameter
		labID, err := serverutil.ValidateID("labID", labID)
		if err != nil {
			m.responder.LocalizedErrorResponseFromErr(r.Context(), w, http.StatusBadRequest, err)
			return
		}

		// Try to get by ID first, then by name
		record, err := m.labTemplateRepo.GetByID(r.Context(), labID)
		if err != nil {
			m.logger.Error("Failed to get lab template", "error", err, "labID", labID)
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusInternalServerError, "labTemplate.errors.getFailed", nil)
			return
		}

		if record == nil {
			// Try by name
			record, err = m.labTemplateRepo.GetByName(r.Context(), labID)
			if err != nil {
				m.logger.Error("Failed to get lab template by name", "error", err, "name", labID)
				m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusInternalServerError, "labTemplate.errors.getFailed", nil)
				return
			}
		}

		if record == nil {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusNotFound, "labTemplate.errors.notFound", nil)
			return
		}

		// Check access based on visibility
		user, _ := auth.UserFromContext(r.Context())
		tc, _ := custommiddleware.TenantFromContext(r.Context())
		if !CanAccessLabWithTenant(user, tc, record) {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusForbidden, "labTemplate.errors.accessDeniedToTemplate", nil)
			return
		}

		// Include full spec if requested
		includeSpec := r.URL.Query().Get("include_spec") == "true"

		response := map[string]any{
			"id":              record.ID,
			"name":            record.Name,
			"slug":            record.Slug,
			"description":     record.Description,
			"version":         record.Version,
			"platform":        record.Platform,
			"durationMinutes": record.DurationMinutes,
			"difficulty":      record.Difficulty,
			"category":        record.Category,
			"tags":            record.Tags,
			"maxPoints":       record.MaxPoints,
			"passThreshold":   record.PassThreshold,
			"isActive":        record.IsActive,
			"visibility":      record.Visibility,
			"createdAt":       record.CreatedAt,
			"updatedAt":       record.UpdatedAt,
		}

		// Include organization ID if present
		if record.OrganizationID != nil {
			response["organizationId"] = *record.OrganizationID
		}
		if record.CreatedBy != nil {
			response["createdBy"] = *record.CreatedBy
		}

		if includeSpec {
			response["spec"] = record.Spec
			response["checkpoints"] = record.Checkpoints
		}

		m.responder.JSONResponse(w, http.StatusOK, response)
	}
}

// CanAccessLab checks if a user has access to a lab template based on visibility rules
func CanAccessLab(user *auth.User, lab *models.LabTemplateRecord) bool {
	return CanAccessLabWithTenant(user, nil, lab)
}

// CanAccessLabWithTenant checks if a user has access to a lab template based on
// visibility rules. When a TenantContext is available (from the tenant middleware),
// the org membership check is authoritative because the middleware already validated
// the user's membership. Falls back to user.DefaultOrganizationID when tc is nil.
func CanAccessLabWithTenant(user *auth.User, tc *models.TenantContext, lab *models.LabTemplateRecord) bool {
	// Global labs (or unset visibility -- backwards compat) are accessible to everyone
	if lab.Visibility == models.LabVisibilityGlobal || lab.Visibility == "" {
		return true
	}

	// No user context - only global labs are accessible
	if user == nil {
		return false
	}

	// Admins can access all labs
	if serverutil.IsAdminOrInstructor(user) {
		return true
	}

	// Organization labs - user must be in the same organization
	if lab.Visibility == models.LabVisibilityOrganization {
		if lab.OrganizationID != nil {
			// Prefer tenant context (validated membership)
			if tc != nil && tc.Organization != nil {
				return tc.Organization.ID == *lab.OrganizationID
			}
			// Fallback for backwards compat (no tenant middleware)
			return user.DefaultOrganizationID == *lab.OrganizationID
		}
		return false
	}

	// Private labs - only the creator can access
	if lab.Visibility == models.LabVisibilityPrivate {
		if lab.CreatedBy != nil && user.ID == *lab.CreatedBy {
			return true
		}
		return false
	}

	return false
}

// CanModifyLab checks if a user can modify a lab template (create/update/delete)
func CanModifyLab(user *auth.User, lab *models.LabTemplateRecord) bool {
	// No user context - cannot modify
	if user == nil {
		return false
	}

	// Admins can modify all labs
	if serverutil.IsAdminOrInstructor(user) {
		return true
	}

	// Users can modify their own labs (any visibility level)
	if lab.CreatedBy != nil && user.ID == *lab.CreatedBy {
		return true
	}

	// Organization instructors can modify organization labs in their org
	// (future enhancement: check for instructor role within org)

	return false
}

func (m *Manager) handleGetLabInstructions() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if m.labTemplateRepo == nil {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusServiceUnavailable, "labTemplate.errors.repoNotConfigured", nil)
			return
		}

		labID := chi.URLParam(r, "labID")

		// Validate the labID parameter
		labID, err := serverutil.ValidateID("labID", labID)
		if err != nil {
			m.responder.LocalizedErrorResponseFromErr(r.Context(), w, http.StatusBadRequest, err)
			return
		}

		// Try to get by ID first, then by name
		record, err := m.labTemplateRepo.GetByID(r.Context(), labID)
		if err != nil {
			m.logger.Error("Failed to get lab template", "error", err, "labID", labID)
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusInternalServerError, "labTemplate.errors.getFailed", nil)
			return
		}

		if record == nil {
			// Try by name
			record, err = m.labTemplateRepo.GetByName(r.Context(), labID)
			if err != nil {
				m.logger.Error("Failed to get lab template by name", "error", err, "name", labID)
				m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusInternalServerError, "labTemplate.errors.getFailed", nil)
				return
			}
		}

		if record == nil {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusNotFound, "labTemplate.errors.notFound", nil)
			return
		}

		// Check access based on visibility
		user, _ := auth.UserFromContext(r.Context())
		tc, _ := custommiddleware.TenantFromContext(r.Context())
		if !CanAccessLabWithTenant(user, tc, record) {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusForbidden, "labTemplate.errors.accessDeniedToTemplate", nil)
			return
		}

		// Convert record to LabTemplate to access instructions
		labTemplate, err := record.ToLabTemplate()
		if err != nil {
			m.logger.Error("Failed to parse lab template spec", "error", err, "labID", labID)
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusInternalServerError, "labTemplate.errors.parseFailed", nil)
			return
		}

		// Extract instructions from metadata
		instructions := labTemplate.Metadata.Instructions
		if instructions == nil {
			// Return empty instructions structure if none defined
			instructions = &models.LabInstructions{}
		}

		response := map[string]any{
			"labId":               record.ID,
			"labName":             record.Name,
			"overview":            instructions.Overview,
			"learning_objectives": instructions.LearningObjectives,
			"prerequisites":       instructions.Prerequisites,
			"steps":               instructions.Steps,
			"summary":             instructions.Summary,
			"tips":                instructions.Tips,
			"resources":           instructions.Resources,
		}

		m.responder.JSONResponse(w, http.StatusOK, response)
	}
}

// -----------------------------------------------------------------------------
// Lab Template CRUD Handlers (Admin)
// -----------------------------------------------------------------------------

// CreateLabTemplateRequest is the request body for creating a lab template
type CreateLabTemplateRequest struct {
	Name            string   `json:"name"`
	Description     string   `json:"description,omitempty"`
	Version         string   `json:"version"`
	Platform        string   `json:"platform"`
	DurationMinutes int      `json:"durationMinutes,omitempty"`
	Difficulty      string   `json:"difficulty,omitempty"`
	Category        string   `json:"category,omitempty"`
	Tags            []string `json:"tags,omitempty"`
	MaxPoints       int      `json:"maxPoints"`
	PassThreshold   int      `json:"passThreshold"`
	Spec            string   `json:"spec"` // YAML or JSON spec
	IsActive        bool     `json:"isActive"`
	Visibility      string   `json:"visibility,omitempty"` // global, organization, private
	OrganizationID  string   `json:"organizationId,omitempty"`
}

// UpdateLabTemplateRequest is the request body for updating a lab template
type UpdateLabTemplateRequest struct {
	Name            *string  `json:"name,omitempty"`
	Description     *string  `json:"description,omitempty"`
	Version         *string  `json:"version,omitempty"`
	Platform        *string  `json:"platform,omitempty"`
	DurationMinutes *int     `json:"durationMinutes,omitempty"`
	Difficulty      *string  `json:"difficulty,omitempty"`
	Category        *string  `json:"category,omitempty"`
	Tags            []string `json:"tags,omitempty"`
	MaxPoints       *int     `json:"maxPoints,omitempty"`
	PassThreshold   *int     `json:"passThreshold,omitempty"`
	Spec            *string  `json:"spec,omitempty"` // YAML or JSON spec
	IsActive        *bool    `json:"isActive,omitempty"`
	Visibility      *string  `json:"visibility,omitempty"`
}

func (m *Manager) handleCreateLab() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if m.labTemplateRepo == nil {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusServiceUnavailable, "labTemplate.errors.repoNotConfigured", nil)
			return
		}

		// Get user from context
		user, _ := auth.UserFromContext(r.Context())

		var req CreateLabTemplateRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			m.responder.SafeErrorResponseWithMessage(w, http.StatusBadRequest, "invalid request body", err, "decode create lab request")
			return
		}

		// Validate required fields
		if req.Name == "" {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusBadRequest, "labTemplate.errors.nameRequired", nil)
			return
		}
		if req.Version == "" {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusBadRequest, "labTemplate.errors.versionRequired", nil)
			return
		}
		if req.Platform == "" {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusBadRequest, "labTemplate.errors.platformRequired", nil)
			return
		}
		if req.Spec == "" {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusBadRequest, "labTemplate.errors.specRequired", nil)
			return
		}

		// Parse and validate the spec (YAML or JSON)
		specJSON, checkpointsJSON, err := parseAndValidateSpec(req.Spec)
		if err != nil {
			m.logger.Warn("Lab spec validation failed", "error", err)
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusBadRequest, "labTemplate.errors.invalidSpec", nil)
			return
		}

		// Generate slug from name
		slug := generateLabSlug(req.Name)

		// Set creator from auth context
		var createdBy *string
		if user != nil {
			createdBy = &user.ID
		}

		// Set visibility - default to global for admins, organization for non-admins
		visibility := models.LabVisibility(req.Visibility)
		if visibility == "" {
			if user != nil && serverutil.IsAdminOrInstructor(user) {
				visibility = models.LabVisibilityGlobal
			} else if user != nil && user.DefaultOrganizationID != "" {
				visibility = models.LabVisibilityOrganization
			} else {
				visibility = models.LabVisibilityGlobal
			}
		}

		// Set organization ID - use user's org if not specified
		var orgID *string
		if req.OrganizationID != "" {
			orgID = &req.OrganizationID
		} else if user != nil && user.DefaultOrganizationID != "" {
			orgID = &user.DefaultOrganizationID
		}

		// Non-admins can only create organization or private labs
		if user != nil && !serverutil.IsAdminOrInstructor(user) && visibility == models.LabVisibilityGlobal {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusForbidden, "labTemplate.errors.onlyAdminsCanCreateGlobal", nil)
			return
		}

		record := &models.LabTemplateRecord{
			Name:            req.Name,
			Slug:            slug,
			Description:     req.Description,
			Version:         req.Version,
			Platform:        models.Platform(req.Platform),
			DurationMinutes: req.DurationMinutes,
			Difficulty:      req.Difficulty,
			Category:        req.Category,
			Tags:            req.Tags,
			MaxPoints:       req.MaxPoints,
			PassThreshold:   req.PassThreshold,
			Spec:            specJSON,
			Checkpoints:     checkpointsJSON,
			IsActive:        req.IsActive,
			Visibility:      visibility,
			OrganizationID:  orgID,
			CreatedBy:       createdBy,
		}

		if err := m.labTemplateRepo.Create(r.Context(), record); err != nil {
			m.logger.Error("Failed to create lab template", "error", err)
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusInternalServerError, "labTemplate.errors.createFailed", nil)
			return
		}

		m.logger.Info("Created lab template", "id", record.ID, "name", record.Name, "visibility", visibility)

		m.responder.JSONResponse(w, http.StatusCreated, map[string]any{
			"id":         record.ID,
			"name":       record.Name,
			"slug":       record.Slug,
			"visibility": record.Visibility,
			"createdAt":  record.CreatedAt,
		})
	}
}

func (m *Manager) handleUpdateLab() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if m.labTemplateRepo == nil {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusServiceUnavailable, "labTemplate.errors.repoNotConfigured", nil)
			return
		}

		// Get user from context
		user, _ := auth.UserFromContext(r.Context())

		labID := chi.URLParam(r, "labID")
		labID, err := serverutil.ValidateID("labID", labID)
		if err != nil {
			m.responder.LocalizedErrorResponseFromErr(r.Context(), w, http.StatusBadRequest, err)
			return
		}

		// Get existing record
		record, err := m.labTemplateRepo.GetByID(r.Context(), labID)
		if err != nil {
			m.logger.Error("Failed to get lab template", "error", err, "labID", labID)
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusInternalServerError, "labTemplate.errors.getFailed", nil)
			return
		}
		if record == nil {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusNotFound, "labTemplate.errors.notFound", nil)
			return
		}

		// Check ownership - admins can edit any, others can only edit their own
		if !CanModifyLab(user, record) {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusForbidden, "labTemplate.errors.modifyPermissionDenied", nil)
			return
		}

		var req UpdateLabTemplateRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			m.responder.SafeErrorResponseWithMessage(w, http.StatusBadRequest, "invalid request body", err, "decode update lab request")
			return
		}

		// Non-admins cannot change visibility to global
		if req.Visibility != nil && *req.Visibility == string(models.LabVisibilityGlobal) {
			if user != nil && !serverutil.IsAdminOrInstructor(user) {
				m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusForbidden, "labTemplate.errors.onlyAdminsCanSetGlobal", nil)
				return
			}
		}

		// Update fields if provided
		if req.Name != nil {
			record.Name = *req.Name
			record.Slug = generateLabSlug(*req.Name) // Update slug when name changes
		}
		if req.Description != nil {
			record.Description = *req.Description
		}
		if req.Version != nil {
			record.Version = *req.Version
		}
		if req.Platform != nil {
			record.Platform = models.Platform(*req.Platform)
		}
		if req.DurationMinutes != nil {
			record.DurationMinutes = *req.DurationMinutes
		}
		if req.Difficulty != nil {
			record.Difficulty = *req.Difficulty
		}
		if req.Category != nil {
			record.Category = *req.Category
		}
		if req.Tags != nil {
			record.Tags = req.Tags
		}
		if req.MaxPoints != nil {
			record.MaxPoints = *req.MaxPoints
		}
		if req.PassThreshold != nil {
			record.PassThreshold = *req.PassThreshold
		}
		if req.IsActive != nil {
			record.IsActive = *req.IsActive
		}
		if req.Visibility != nil {
			record.Visibility = models.LabVisibility(*req.Visibility)
		}

		// Update spec if provided
		if req.Spec != nil {
			specJSON, checkpointsJSON, err := parseAndValidateSpec(*req.Spec)
			if err != nil {
				m.logger.Warn("Lab spec validation failed", "error", err)
				m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusBadRequest, "labTemplate.errors.invalidSpec", nil)
				return
			}
			record.Spec = specJSON
			record.Checkpoints = checkpointsJSON
		}

		if err := m.labTemplateRepo.Update(r.Context(), record); err != nil {
			m.logger.Error("Failed to update lab template", "error", err, "labID", labID)
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusInternalServerError, "labTemplate.errors.updateFailed", nil)
			return
		}

		m.logger.Info("Updated lab template", "id", record.ID, "name", record.Name)

		m.responder.JSONResponse(w, http.StatusOK, map[string]any{
			"id":        record.ID,
			"name":      record.Name,
			"slug":      record.Slug,
			"updatedAt": record.UpdatedAt,
		})
	}
}

func (m *Manager) handleDeleteLab() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if m.labTemplateRepo == nil {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusServiceUnavailable, "labTemplate.errors.repoNotConfigured", nil)
			return
		}

		// Get user from context
		user, _ := auth.UserFromContext(r.Context())

		labID := chi.URLParam(r, "labID")
		labID, err := serverutil.ValidateID("labID", labID)
		if err != nil {
			m.responder.LocalizedErrorResponseFromErr(r.Context(), w, http.StatusBadRequest, err)
			return
		}

		// Check if template exists
		record, err := m.labTemplateRepo.GetByID(r.Context(), labID)
		if err != nil {
			m.logger.Error("Failed to get lab template", "error", err, "labID", labID)
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusInternalServerError, "labTemplate.errors.getFailed", nil)
			return
		}
		if record == nil {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusNotFound, "labTemplate.errors.notFound", nil)
			return
		}

		// Check ownership - admins can delete any, others can only delete their own
		if !CanModifyLab(user, record) {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusForbidden, "labTemplate.errors.deletePermissionDenied", nil)
			return
		}

		// Delete the template
		if err := m.labTemplateRepo.Delete(r.Context(), labID); err != nil {
			m.logger.Error("Failed to delete lab template", "error", err, "labID", labID)
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusInternalServerError, "labTemplate.errors.deleteFailed", nil)
			return
		}

		m.logger.Info("Deleted lab template", "id", labID, "name", record.Name)

		m.responder.JSONResponse(w, http.StatusOK, map[string]string{
			"message": "lab template deleted successfully",
		})
	}
}

func (m *Manager) handleSetLabActive() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if m.labTemplateRepo == nil {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusServiceUnavailable, "labTemplate.errors.repoNotConfigured", nil)
			return
		}

		// Get user from context
		user, _ := auth.UserFromContext(r.Context())

		labID := chi.URLParam(r, "labID")
		labID, err := serverutil.ValidateID("labID", labID)
		if err != nil {
			m.responder.LocalizedErrorResponseFromErr(r.Context(), w, http.StatusBadRequest, err)
			return
		}

		var req struct {
			IsActive bool `json:"isActive"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			m.responder.SafeErrorResponseWithMessage(w, http.StatusBadRequest, "invalid request body", err, "decode set lab active request")
			return
		}

		// Check if template exists
		record, err := m.labTemplateRepo.GetByID(r.Context(), labID)
		if err != nil {
			m.logger.Error("Failed to get lab template", "error", err, "labID", labID)
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusInternalServerError, "labTemplate.errors.getFailed", nil)
			return
		}
		if record == nil {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusNotFound, "labTemplate.errors.notFound", nil)
			return
		}

		// Check ownership - admins can modify any, others can only modify their own
		if !CanModifyLab(user, record) {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusForbidden, "labTemplate.errors.modifyPermissionDenied", nil)
			return
		}

		if err := m.labTemplateRepo.SetActive(r.Context(), labID, req.IsActive); err != nil {
			m.logger.Error("Failed to set lab template active status", "error", err, "labID", labID)
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusInternalServerError, "labTemplate.errors.updateFailed", nil)
			return
		}

		status := "deactivated"
		if req.IsActive {
			status = "activated"
		}

		m.logger.Info("Set lab template active status", "id", labID, "active", req.IsActive)

		m.responder.JSONResponse(w, http.StatusOK, map[string]string{
			"message": "lab template " + status,
		})
	}
}

// -----------------------------------------------------------------------------
// Version History & Import/Export Handlers
// -----------------------------------------------------------------------------

func (m *Manager) handleListVersions() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if m.labTemplateRepo == nil {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusServiceUnavailable, "labTemplate.errors.repoNotConfigured", nil)
			return
		}

		labID := chi.URLParam(r, "labID")
		labID, err := serverutil.ValidateID("labID", labID)
		if err != nil {
			m.responder.LocalizedErrorResponseFromErr(r.Context(), w, http.StatusBadRequest, err)
			return
		}

		// Check template exists
		record, err := m.labTemplateRepo.GetByID(r.Context(), labID)
		if err != nil {
			m.logger.Error("Failed to get lab template", "error", err, "labID", labID)
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusInternalServerError, "labTemplate.errors.getFailed", nil)
			return
		}
		if record == nil {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusNotFound, "labTemplate.errors.notFound", nil)
			return
		}

		user, _ := auth.UserFromContext(r.Context())
		tc, _ := custommiddleware.TenantFromContext(r.Context())
		if !CanAccessLabWithTenant(user, tc, record) {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusForbidden, "labTemplate.errors.accessDenied", nil)
			return
		}

		pagination := serverutil.ParsePagination(r, 50, 200)

		versions, err := m.labTemplateRepo.ListVersions(r.Context(), labID, pagination.Limit, pagination.Offset)
		if err != nil {
			m.logger.Error("Failed to list versions", "error", err, "labID", labID)
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusInternalServerError, "labTemplate.errors.listVersionsFailed", nil)
			return
		}

		total, _ := m.labTemplateRepo.CountVersions(r.Context(), labID)

		// Return summary only (no spec/checkpoints/instructions)
		summaries := make([]map[string]any, 0, len(versions))
		for _, v := range versions {
			summaries = append(summaries, map[string]any{
				"id":            v.ID,
				"versionNumber": v.VersionNumber,
				"name":          v.Name,
				"version":       v.Version,
				"changeSummary": v.ChangeSummary,
				"createdAt":     v.CreatedAt,
			})
		}

		m.responder.JSONResponse(w, http.StatusOK, map[string]any{
			"versions": summaries,
			"total":    total,
		})
	}
}

func (m *Manager) handleGetVersion() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if m.labTemplateRepo == nil {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusServiceUnavailable, "labTemplate.errors.repoNotConfigured", nil)
			return
		}

		labID := chi.URLParam(r, "labID")
		labID, err := serverutil.ValidateID("labID", labID)
		if err != nil {
			m.responder.LocalizedErrorResponseFromErr(r.Context(), w, http.StatusBadRequest, err)
			return
		}

		numStr := chi.URLParam(r, "versionNumber")
		var versionNumber int
		if _, err := fmt.Sscanf(numStr, "%d", &versionNumber); err != nil || versionNumber < 1 {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusBadRequest, "labTemplate.errors.invalidVersionNumber", nil)
			return
		}

		// Check template exists and access
		record, err := m.labTemplateRepo.GetByID(r.Context(), labID)
		if err != nil {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusInternalServerError, "labTemplate.errors.getFailed", nil)
			return
		}
		if record == nil {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusNotFound, "labTemplate.errors.notFound", nil)
			return
		}
		user, _ := auth.UserFromContext(r.Context())
		tc, _ := custommiddleware.TenantFromContext(r.Context())
		if !CanAccessLabWithTenant(user, tc, record) {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusForbidden, "labTemplate.errors.accessDenied", nil)
			return
		}

		version, err := m.labTemplateRepo.GetVersionByNumber(r.Context(), labID, versionNumber)
		if err != nil {
			m.logger.Error("Failed to get version", "error", err)
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusInternalServerError, "labTemplate.errors.getVersionFailed", nil)
			return
		}
		if version == nil {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusNotFound, "labTemplate.errors.versionNotFound", nil)
			return
		}

		m.responder.JSONResponse(w, http.StatusOK, version)
	}
}

func (m *Manager) handleRestoreVersion() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if m.labTemplateRepo == nil {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusServiceUnavailable, "labTemplate.errors.repoNotConfigured", nil)
			return
		}

		labID := chi.URLParam(r, "labID")
		labID, err := serverutil.ValidateID("labID", labID)
		if err != nil {
			m.responder.LocalizedErrorResponseFromErr(r.Context(), w, http.StatusBadRequest, err)
			return
		}

		numStr := chi.URLParam(r, "versionNumber")
		var versionNumber int
		if _, err := fmt.Sscanf(numStr, "%d", &versionNumber); err != nil || versionNumber < 1 {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusBadRequest, "labTemplate.errors.invalidVersionNumber", nil)
			return
		}

		// Check template exists and permission
		record, err := m.labTemplateRepo.GetByID(r.Context(), labID)
		if err != nil {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusInternalServerError, "labTemplate.errors.getFailed", nil)
			return
		}
		if record == nil {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusNotFound, "labTemplate.errors.notFound", nil)
			return
		}
		user, _ := auth.UserFromContext(r.Context())
		if !CanModifyLab(user, record) {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusForbidden, "labTemplate.errors.permissionDenied", nil)
			return
		}

		// Parse optional change summary
		var body struct {
			ChangeSummary string `json:"changeSummary"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)

		// Get the version to restore
		version, err := m.labTemplateRepo.GetVersionByNumber(r.Context(), labID, versionNumber)
		if err != nil {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusInternalServerError, "labTemplate.errors.getVersionFailed", nil)
			return
		}
		if version == nil {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusNotFound, "labTemplate.errors.versionNotFound", nil)
			return
		}

		// Convert version to record and update (Update auto-snapshots current state)
		restored := version.ToLabTemplateRecord()
		if err := m.labTemplateRepo.Update(r.Context(), restored); err != nil {
			m.logger.Error("Failed to restore version", "error", err)
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusInternalServerError, "labTemplate.errors.restoreVersionFailed", nil)
			return
		}

		m.logger.Info("Restored lab template version", "labID", labID, "versionNumber", versionNumber)

		m.responder.JSONResponse(w, http.StatusOK, map[string]any{
			"message":         "version restored successfully",
			"restoredVersion": versionNumber,
			"updatedAt":       restored.UpdatedAt,
		})
	}
}

func (m *Manager) handleExportLab() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if m.labTemplateRepo == nil {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusServiceUnavailable, "labTemplate.errors.repoNotConfigured", nil)
			return
		}

		labID := chi.URLParam(r, "labID")
		labID, err := serverutil.ValidateID("labID", labID)
		if err != nil {
			m.responder.LocalizedErrorResponseFromErr(r.Context(), w, http.StatusBadRequest, err)
			return
		}

		record, err := m.labTemplateRepo.GetByID(r.Context(), labID)
		if err != nil {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusInternalServerError, "labTemplate.errors.getFailed", nil)
			return
		}
		if record == nil {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusNotFound, "labTemplate.errors.notFound", nil)
			return
		}

		user, _ := auth.UserFromContext(r.Context())
		tc, _ := custommiddleware.TenantFromContext(r.Context())
		if !CanAccessLabWithTenant(user, tc, record) {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusForbidden, "labTemplate.errors.accessDenied", nil)
			return
		}

		// Convert to LabTemplate for YAML export
		labTemplate, err := record.ToLabTemplate()
		if err != nil {
			m.logger.Error("Failed to convert lab template", "error", err)
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusInternalServerError, "labTemplate.errors.exportFailed", nil)
			return
		}

		yamlData, err := yaml.Marshal(labTemplate)
		if err != nil {
			m.logger.Error("Failed to marshal YAML", "error", err)
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusInternalServerError, "labTemplate.errors.exportFailed", nil)
			return
		}

		filename := strings.ReplaceAll(record.Name, " ", "-") + ".yaml"
		w.Header().Set("Content-Type", "application/x-yaml")
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))
		w.WriteHeader(http.StatusOK)
		w.Write(yamlData)
	}
}

// ImportLabRequest is the request body for importing a lab template from YAML
type ImportLabRequest struct {
	YAML       string `json:"yaml"`
	Visibility string `json:"visibility,omitempty"`
}

func (m *Manager) handleImportLab() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if m.labTemplateRepo == nil {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusServiceUnavailable, "labTemplate.errors.repoNotConfigured", nil)
			return
		}

		user, _ := auth.UserFromContext(r.Context())

		var req ImportLabRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			m.responder.SafeErrorResponseWithMessage(w, http.StatusBadRequest, "invalid request body", err, "decode import lab request")
			return
		}

		if req.YAML == "" {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusBadRequest, "labTemplate.errors.yamlRequired", nil)
			return
		}

		// Parse and validate using existing function
		specJSON, checkpointsJSON, err := parseAndValidateSpec(req.YAML)
		if err != nil {
			m.logger.Warn("Lab YAML validation failed", "error", err)
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusBadRequest, "labTemplate.errors.invalidSpec", nil)
			return
		}

		// Parse YAML to get metadata
		var template models.LabTemplate
		if err := yaml.Unmarshal([]byte(req.YAML), &template); err != nil {
			m.responder.SafeErrorResponseWithMessage(w, http.StatusBadRequest, "invalid YAML structure", err, "parse import YAML metadata")
			return
		}

		slug := generateLabSlug(template.Metadata.Name)

		var createdBy *string
		if user != nil {
			createdBy = &user.ID
		}

		visibility := models.LabVisibility(req.Visibility)
		if visibility == "" {
			visibility = models.LabVisibilityGlobal
		}

		var orgID *string
		if user != nil && user.DefaultOrganizationID != "" {
			orgID = &user.DefaultOrganizationID
		}

		// Non-admins can only create organization or private labs
		if user != nil && !serverutil.IsAdminOrInstructor(user) && visibility == models.LabVisibilityGlobal {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusForbidden, "labTemplate.errors.onlyAdminsCanCreateGlobal", nil)
			return
		}

		// Calculate max points
		maxPoints := 0
		for _, obj := range template.Spec.Objectives {
			maxPoints += obj.Points
		}
		passThreshold := 70
		if template.Spec.Checkpoints != nil && template.Spec.Checkpoints.PassThreshold > 0 {
			passThreshold = template.Spec.Checkpoints.PassThreshold
		}

		// Marshal instructions if present
		var instructionsJSON json.RawMessage
		if template.Metadata.Instructions != nil {
			instructionsJSON, _ = json.Marshal(template.Metadata.Instructions)
		}

		record := &models.LabTemplateRecord{
			Name:            template.Metadata.Name,
			Slug:            slug,
			Description:     template.Metadata.Description,
			Version:         template.Metadata.Version,
			Platform:        template.Spec.Platform,
			DurationMinutes: models.ParseDurationPublic(template.Metadata.Duration),
			Difficulty:      template.Metadata.Difficulty,
			MaxPoints:       maxPoints,
			PassThreshold:   passThreshold,
			Spec:            specJSON,
			Checkpoints:     checkpointsJSON,
			Instructions:    instructionsJSON,
			IsActive:        true,
			Visibility:      visibility,
			OrganizationID:  orgID,
			CreatedBy:       createdBy,
		}

		if err := m.labTemplateRepo.Create(r.Context(), record); err != nil {
			m.responder.SafeErrorResponse(w, err, "import lab template")
			return
		}

		m.logger.Info("Imported lab template", "id", record.ID, "name", record.Name)

		m.responder.JSONResponse(w, http.StatusCreated, map[string]any{
			"id":        record.ID,
			"name":      record.Name,
			"slug":      record.Slug,
			"createdAt": record.CreatedAt,
		})
	}
}

// parseAndValidateSpec parses YAML or JSON spec and returns JSON for storage
func parseAndValidateSpec(specStr string) (json.RawMessage, json.RawMessage, error) {
	// Parse as full LabTemplate (which contains APIVersion, Kind, Metadata, and Spec)
	var template models.LabTemplate

	// Try to parse as YAML first
	if err := yaml.Unmarshal([]byte(specStr), &template); err != nil {
		// Try JSON
		if err := json.Unmarshal([]byte(specStr), &template); err != nil {
			return nil, nil, fmt.Errorf("spec must be valid YAML or JSON: %w", err)
		}
	}

	// Validate required fields
	if template.APIVersion == "" {
		return nil, nil, fmt.Errorf("apiVersion is required")
	}
	if template.Metadata.Name == "" {
		return nil, nil, fmt.Errorf("metadata.name is required")
	}

	// Convert full template to JSON for storage
	specJSON, err := json.Marshal(template)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to marshal spec: %w", err)
	}

	// Extract checkpoints/objectives for separate storage
	var checkpointsJSON json.RawMessage
	if len(template.Spec.Objectives) > 0 {
		checkpointsJSON, err = json.Marshal(template.Spec.Objectives)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to marshal checkpoints: %w", err)
		}
	}

	return specJSON, checkpointsJSON, nil
}

// generateLabSlug creates a URL-friendly slug from a name (for lab templates)
func generateLabSlug(name string) string {
	slug := strings.ToLower(name)
	slug = strings.ReplaceAll(slug, " ", "-")
	// Remove special characters except hyphens
	var result strings.Builder
	for _, r := range slug {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			result.WriteRune(r)
		}
	}
	// Remove consecutive hyphens
	slug = result.String()
	for strings.Contains(slug, "--") {
		slug = strings.ReplaceAll(slug, "--", "-")
	}
	// Trim leading/trailing hyphens
	slug = strings.Trim(slug, "-")
	return slug
}

// tenantFromCtx is a nil-safe wrapper for custommiddleware.TenantFromContext.
func tenantFromCtx(ctx context.Context) *models.TenantContext {
	tc, _ := custommiddleware.TenantFromContext(ctx)
	return tc
}

// ---------------------------------------------------------------------------
// Org-scoped lab endpoints (mounted under /organizations/{orgID}/labs)
// ---------------------------------------------------------------------------

// HandleListOrgLabs returns lab templates scoped to the org from TenantContext.
// The tenant middleware (applied in the /{orgID} route group) guarantees that
// TenantFromContext returns a valid org with validated membership.
func (m *Manager) HandleListOrgLabs() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tc := tenantFromCtx(r.Context())
		if tc == nil || tc.Organization == nil {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusBadRequest, "labTemplate.errors.orgContextRequired", nil)
			return
		}

		active := r.URL.Query().Get("active") != "false"
		filter := repositories.LabTemplateFilter{
			OrganizationID: tc.Organization.ID,
			IncludeGlobal:  true,
			Active:         &active,
		}

		records, err := m.labTemplateRepo.List(r.Context(), filter)
		if err != nil {
			m.logger.Error("Failed to list org labs", "error", err, "orgId", tc.Organization.ID)
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusInternalServerError, "labTemplate.errors.listFailed", nil)
			return
		}

		m.responder.JSONResponse(w, http.StatusOK, map[string]any{"labs": records})
	}
}

// HandleCreateOrgLab creates a lab template scoped to the org from TenantContext.
// Visibility is forced to "organization" -- use the top-level POST /labs for global templates.
func (m *Manager) HandleCreateOrgLab() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tc := tenantFromCtx(r.Context())
		if tc == nil || tc.Organization == nil {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusBadRequest, "labTemplate.errors.orgContextRequired", nil)
			return
		}

		req, validationErrors := serverutil.DecodeAndValidate[CreateLabTemplateRequest](r)
		if validationErrors != nil {
			m.responder.JSONResponse(w, http.StatusBadRequest, serverutil.ValidationErrorResponse{
				Error:   "validation failed",
				Details: validationErrors,
			})
			return
		}

		// Force org-scoped visibility
		orgID := tc.Organization.ID
		user, _ := auth.UserFromContext(r.Context())
		var createdBy *string
		if user != nil {
			createdBy = &user.ID
		}

		record := &models.LabTemplateRecord{
			Name:           req.Name,
			Description:    req.Description,
			Platform:       models.Platform(req.Platform),
			Difficulty:     req.Difficulty,
			IsActive:       true,
			OrganizationID: &orgID,
			Visibility:     models.LabVisibilityOrganization,
			CreatedBy:      createdBy,
		}

		if err := m.labTemplateRepo.Create(r.Context(), record); err != nil {
			m.logger.Error("Failed to create org lab", "error", err, "orgId", orgID)
			m.responder.SafeErrorResponse(w, err, "create org lab template")
			return
		}

		m.logger.Info("Org lab template created", "id", record.ID, "name", record.Name, "orgId", orgID)
		m.responder.JSONResponse(w, http.StatusCreated, record)
	}
}
