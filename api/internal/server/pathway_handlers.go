package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"slices"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/toddbartholow/kootenai/api/internal/auth"
	"github.com/toddbartholow/kootenai/api/internal/database/repositories"
	"github.com/toddbartholow/kootenai/api/internal/models"
	"github.com/toddbartholow/kootenai/api/internal/pathway"
)

// getUserIDFromContext extracts the user ID from the request context
func getUserIDFromContext(ctx context.Context) (string, bool) {
	user, ok := auth.UserFromContext(ctx)
	if !ok || user == nil {
		return "", false
	}
	return user.ID, true
}

// -----------------------------------------------------------------------------
// Pathway Handlers
// -----------------------------------------------------------------------------

func (m *PathwayManager) handleListPathways() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if m.pathwayRepo == nil {
			m.responder.JSONResponse(w, http.StatusOK, map[string]any{
				"pathways":   []any{},
				"count":      0,
				"pagination": NewPaginationMeta(0, 50, 0),
			})
			return
		}

		ctx := r.Context()

		// Parse pagination params
		pagination := ParsePagination(r, 50, 500)

		// Parse query parameters
		opts := models.PathwayListOptions{
			IncludeStats: r.URL.Query().Get("include_stats") == "true",
			Limit:        pagination.Limit,
			Offset:       pagination.Offset,
		}

		if status := r.URL.Query().Get("status"); status != "" {
			opts.Status = models.PathwayStatus(status)
		}
		if visibility := r.URL.Query().Get("visibility"); visibility != "" {
			opts.Visibility = models.LabVisibility(visibility)
		}
		if difficulty := r.URL.Query().Get("difficulty"); difficulty != "" {
			opts.Difficulty = difficulty
		}
		if featured := r.URL.Query().Get("featured"); featured == "true" {
			f := true
			opts.IsFeatured = &f
		}
		if search := r.URL.Query().Get("search"); search != "" {
			opts.Search = search
		}

		// Default to published pathways for non-admin users
		if opts.Status == "" {
			opts.Status = models.PathwayStatusPublished
		}

		pathways, err := m.pathwayRepo.List(ctx, opts)
		if err != nil {
			m.logger.Error("Failed to list pathways", "error", err)
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusInternalServerError, "pathway.errors.listFailed", nil)
			return
		}

		// Calculate total - heuristic since we don't have a separate count query
		total := len(pathways)
		if pagination.Offset == 0 && len(pathways) < pagination.Limit {
			// No pagination needed, total is exact
		} else if len(pathways) == pagination.Limit {
			// There may be more records
			total = pagination.Offset + len(pathways) + 1
		} else {
			// Partial page, so this is the last page
			total = pagination.Offset + len(pathways)
		}

		paginationMeta := NewPaginationMeta(total, pagination.Limit, pagination.Offset)

		m.responder.JSONResponse(w, http.StatusOK, map[string]any{
			"pathways":   pathways,
			"count":      len(pathways),
			"total":      total,
			"pagination": paginationMeta,
		})
	}
}

func (m *PathwayManager) handleGetPathway() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		pathwayIDOrSlug := chi.URLParam(r, "pathwayID")

		if m.pathwayRepo == nil {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusNotFound, "pathway.errors.repoNotAvailable", nil)
			return
		}

		var pathway *models.Pathway
		var err error

		// Check if it looks like a UUID
		if _, uuidErr := uuid.Parse(pathwayIDOrSlug); uuidErr == nil {
			// Try by ID first
			pathway, err = m.pathwayRepo.GetWithModules(r.Context(), pathwayIDOrSlug)
			if err != nil {
				m.logger.Error("Failed to get pathway", "error", err, "pathwayId", pathwayIDOrSlug)
				m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusInternalServerError, "pathway.errors.getFailed", nil)
				return
			}
		}

		// If not found by ID (or wasn't a UUID), try by slug
		if pathway == nil {
			pathway, err = m.pathwayRepo.GetWithModulesBySlug(r.Context(), pathwayIDOrSlug)
			if err != nil {
				m.logger.Error("Failed to get pathway by slug", "error", err, "slug", pathwayIDOrSlug)
				m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusInternalServerError, "pathway.errors.getFailed", nil)
				return
			}
		}

		if pathway == nil {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusNotFound, "pathway.errors.notFound", nil)
			return
		}

		m.responder.JSONResponse(w, http.StatusOK, pathway)
	}
}

func (m *PathwayManager) handleCreatePathway() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if m.pathwayRepo == nil {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusServiceUnavailable, "pathway.errors.repoNotAvailable", nil)
			return
		}

		var req models.CreatePathwayRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusBadRequest, "error.invalidRequestBody", nil)
			return
		}

		if req.Name == "" {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusBadRequest, "pathway.errors.nameRequired", nil)
			return
		}

		// Generate slug if not provided
		slug := req.Slug
		if slug == "" {
			slug = repositories.GenerateSlug(req.Name)
		}

		// Check for slug uniqueness
		existing, err := m.pathwayRepo.GetBySlug(r.Context(), slug)
		if err != nil {
			m.logger.Error("Failed to check slug uniqueness", "error", err)
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusInternalServerError, "pathway.errors.createFailed", nil)
			return
		}
		if existing != nil {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusConflict, "pathway.errors.slugConflict", nil)
			return
		}

		pathway := &models.Pathway{
			ID:               uuid.New().String(),
			Name:             req.Name,
			Slug:             slug,
			Description:      req.Description,
			ShortDescription: req.ShortDescription,
			Difficulty:       req.Difficulty,
			EstimatedHours:   req.EstimatedHours,
			Status:           models.PathwayStatusDraft,
			Tags:             req.Tags,
			Icon:             req.Icon,
			Color:            req.Color,
			CoverImageURL:    req.CoverImageURL,
			Visibility:       req.Visibility,
		}

		if pathway.Visibility == "" {
			pathway.Visibility = models.LabVisibilityGlobal
		}

		// Set creator from auth context if available
		if userID, ok := getUserIDFromContext(r.Context()); ok {
			pathway.CreatedBy = &userID
		}

		if err := m.pathwayRepo.Create(r.Context(), pathway); err != nil {
			m.logger.Error("Failed to create pathway", "error", err)
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusInternalServerError, "pathway.errors.createFailed", nil)
			return
		}

		m.responder.JSONResponse(w, http.StatusCreated, pathway)
	}
}

func (m *PathwayManager) handleUpdatePathway() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		pathwayID := chi.URLParam(r, "pathwayID")

		if m.pathwayRepo == nil {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusServiceUnavailable, "pathway.errors.repoNotAvailable", nil)
			return
		}

		pathway, err := m.pathwayRepo.GetByID(r.Context(), pathwayID)
		if err != nil {
			m.logger.Error("Failed to get pathway", "error", err)
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusInternalServerError, "pathway.errors.getFailed", nil)
			return
		}
		if pathway == nil {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusNotFound, "pathway.errors.notFound", nil)
			return
		}

		var req models.UpdatePathwayRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusBadRequest, "error.invalidRequestBody", nil)
			return
		}

		// Apply updates
		if req.Name != nil {
			pathway.Name = *req.Name
		}
		if req.Description != nil {
			pathway.Description = *req.Description
		}
		if req.ShortDescription != nil {
			pathway.ShortDescription = *req.ShortDescription
		}
		if req.Difficulty != nil {
			pathway.Difficulty = *req.Difficulty
		}
		if req.EstimatedHours != nil {
			pathway.EstimatedHours = *req.EstimatedHours
		}
		if req.DisplayOrder != nil {
			pathway.DisplayOrder = *req.DisplayOrder
		}
		if req.Status != nil {
			pathway.Status = *req.Status
		}
		if req.IsFeatured != nil {
			pathway.IsFeatured = *req.IsFeatured
		}
		if req.Tags != nil {
			pathway.Tags = req.Tags
		}
		if req.Icon != nil {
			pathway.Icon = *req.Icon
		}
		if req.Color != nil {
			pathway.Color = *req.Color
		}
		if req.CoverImageURL != nil {
			pathway.CoverImageURL = *req.CoverImageURL
		}
		if req.Visibility != nil {
			pathway.Visibility = *req.Visibility
		}

		if err := m.pathwayRepo.Update(r.Context(), pathway); err != nil {
			m.logger.Error("Failed to update pathway", "error", err)
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusInternalServerError, "pathway.errors.updateFailed", nil)
			return
		}

		m.responder.JSONResponse(w, http.StatusOK, pathway)
	}
}

func (m *PathwayManager) handleDeletePathway() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		pathwayID := chi.URLParam(r, "pathwayID")

		if m.pathwayRepo == nil {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusServiceUnavailable, "pathway.errors.repoNotAvailable", nil)
			return
		}

		pathway, err := m.pathwayRepo.GetByID(r.Context(), pathwayID)
		if err != nil {
			m.logger.Error("Failed to get pathway", "error", err)
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusInternalServerError, "pathway.errors.getFailed", nil)
			return
		}
		if pathway == nil {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusNotFound, "pathway.errors.notFound", nil)
			return
		}

		if err := m.pathwayRepo.Delete(r.Context(), pathwayID); err != nil {
			m.logger.Error("Failed to delete pathway", "error", err)
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusInternalServerError, "pathway.errors.deleteFailed", nil)
			return
		}

		m.responder.JSONResponse(w, http.StatusOK, map[string]string{"status": "deleted"})
	}
}

func (m *PathwayManager) handlePublishPathway() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		pathwayID := chi.URLParam(r, "pathwayID")

		if m.pathwayRepo == nil {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusServiceUnavailable, "pathway.errors.repoNotAvailable", nil)
			return
		}

		if err := m.pathwayRepo.UpdateStatus(r.Context(), pathwayID, models.PathwayStatusPublished); err != nil {
			m.logger.Error("Failed to publish pathway", "error", err)
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusInternalServerError, "pathway.errors.publishFailed", nil)
			return
		}

		m.responder.JSONResponse(w, http.StatusOK, map[string]string{"status": "published"})
	}
}

func (m *PathwayManager) handleArchivePathway() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		pathwayID := chi.URLParam(r, "pathwayID")

		if m.pathwayRepo == nil {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusServiceUnavailable, "pathway.errors.repoNotAvailable", nil)
			return
		}

		if err := m.pathwayRepo.UpdateStatus(r.Context(), pathwayID, models.PathwayStatusArchived); err != nil {
			m.logger.Error("Failed to archive pathway", "error", err)
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusInternalServerError, "pathway.errors.archiveFailed", nil)
			return
		}

		m.responder.JSONResponse(w, http.StatusOK, map[string]string{"status": "archived"})
	}
}

func (m *PathwayManager) handleGetPathwayStats() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		pathwayID := chi.URLParam(r, "pathwayID")

		if m.pathwayRepo == nil {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusServiceUnavailable, "pathway.errors.repoNotAvailable", nil)
			return
		}

		stats, err := m.pathwayRepo.GetStats(r.Context(), pathwayID)
		if err != nil {
			m.logger.Error("Failed to get pathway stats", "error", err)
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusInternalServerError, "pathway.errors.statsFailed", nil)
			return
		}

		m.responder.JSONResponse(w, http.StatusOK, stats)
	}
}

// -----------------------------------------------------------------------------
// Module Handlers
// -----------------------------------------------------------------------------

func (m *PathwayManager) handleListModules() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		pathwayID := chi.URLParam(r, "pathwayID")

		if m.pathwayRepo == nil {
			m.responder.JSONResponse(w, http.StatusOK, map[string]any{
				"modules": []any{},
				"count":   0,
			})
			return
		}

		modules, err := m.pathwayRepo.ListModules(r.Context(), pathwayID)
		if err != nil {
			m.logger.Error("Failed to list modules", "error", err)
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusInternalServerError, "pathway.errors.moduleListFailed", nil)
			return
		}

		m.responder.JSONResponse(w, http.StatusOK, map[string]any{
			"modules": modules,
			"count":   len(modules),
		})
	}
}

func (m *PathwayManager) handleCreateModule() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		pathwayID := chi.URLParam(r, "pathwayID")

		if m.pathwayRepo == nil {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusServiceUnavailable, "pathway.errors.repoNotAvailable", nil)
			return
		}

		var req models.CreateModuleRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusBadRequest, "error.invalidRequestBody", nil)
			return
		}

		if req.Name == "" {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusBadRequest, "pathway.errors.nameRequired", nil)
			return
		}

		slug := req.Slug
		if slug == "" {
			slug = repositories.GenerateSlug(req.Name)
		}

		module := &models.PathwayModule{
			ID:               uuid.New().String(),
			PathwayID:        pathwayID,
			Name:             req.Name,
			Slug:             slug,
			Description:      req.Description,
			UnlockType:       req.UnlockType,
			IsActive:         true,
			Icon:             req.Icon,
			EstimatedMinutes: req.EstimatedMinutes,
		}

		if module.UnlockType == "" {
			module.UnlockType = models.UnlockTypeSequential
		}

		// Get current module count to set display order
		modules, _ := m.pathwayRepo.ListModules(r.Context(), pathwayID)
		module.DisplayOrder = len(modules)

		if err := m.pathwayRepo.CreateModule(r.Context(), module); err != nil {
			m.logger.Error("Failed to create module", "error", err)
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusInternalServerError, "pathway.errors.moduleCreateFailed", nil)
			return
		}

		m.responder.JSONResponse(w, http.StatusCreated, module)
	}
}

func (m *PathwayManager) handleUpdateModule() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		moduleID := chi.URLParam(r, "moduleID")

		if m.pathwayRepo == nil {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusServiceUnavailable, "pathway.errors.repoNotAvailable", nil)
			return
		}

		module, err := m.pathwayRepo.GetModuleByID(r.Context(), moduleID)
		if err != nil {
			m.logger.Error("Failed to get module", "error", err)
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusInternalServerError, "pathway.errors.moduleGetFailed", nil)
			return
		}
		if module == nil {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusNotFound, "pathway.errors.moduleNotFound", nil)
			return
		}

		var req models.UpdateModuleRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusBadRequest, "error.invalidRequestBody", nil)
			return
		}

		if req.Name != nil {
			module.Name = *req.Name
		}
		if req.Description != nil {
			module.Description = *req.Description
		}
		if req.DisplayOrder != nil {
			module.DisplayOrder = *req.DisplayOrder
		}
		if req.UnlockType != nil {
			module.UnlockType = *req.UnlockType
		}
		if req.IsActive != nil {
			module.IsActive = *req.IsActive
		}
		if req.Icon != nil {
			module.Icon = *req.Icon
		}
		if req.EstimatedMinutes != nil {
			module.EstimatedMinutes = *req.EstimatedMinutes
		}

		if err := m.pathwayRepo.UpdateModule(r.Context(), module); err != nil {
			m.logger.Error("Failed to update module", "error", err)
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusInternalServerError, "pathway.errors.moduleUpdateFailed", nil)
			return
		}

		m.responder.JSONResponse(w, http.StatusOK, module)
	}
}

func (m *PathwayManager) handleDeleteModule() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		moduleID := chi.URLParam(r, "moduleID")

		if m.pathwayRepo == nil {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusServiceUnavailable, "pathway.errors.repoNotAvailable", nil)
			return
		}

		if err := m.pathwayRepo.DeleteModule(r.Context(), moduleID); err != nil {
			m.logger.Error("Failed to delete module", "error", err)
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusInternalServerError, "pathway.errors.moduleDeleteFailed", nil)
			return
		}

		m.responder.JSONResponse(w, http.StatusOK, map[string]string{"status": "deleted"})
	}
}

func (m *PathwayManager) handleAddLabToModule() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		moduleID := chi.URLParam(r, "moduleID")

		if m.pathwayRepo == nil {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusServiceUnavailable, "pathway.errors.repoNotAvailable", nil)
			return
		}

		var req models.AddLabToModuleRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusBadRequest, "error.invalidRequestBody", nil)
			return
		}

		if req.LabTemplateID == "" {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusBadRequest, "pathway.errors.labTemplateIdRequired", nil)
			return
		}

		isRequired := true
		if req.IsRequired != nil {
			isRequired = *req.IsRequired
		}

		displayOrder := 0
		if req.DisplayOrder != nil {
			displayOrder = *req.DisplayOrder
		} else {
			// Get current lab count
			labs, _ := m.pathwayRepo.ListModuleLabs(r.Context(), moduleID)
			displayOrder = len(labs)
		}

		moduleLab := &models.ModuleLab{
			ID:                    uuid.New().String(),
			ModuleID:              moduleID,
			LabTemplateID:         req.LabTemplateID,
			DisplayOrder:          displayOrder,
			IsRequired:            isRequired,
			PassThresholdOverride: req.PassThresholdOverride,
		}

		if err := m.pathwayRepo.AddLabToModule(r.Context(), moduleLab); err != nil {
			m.logger.Error("Failed to add lab to module", "error", err)
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusInternalServerError, "pathway.errors.moduleLabAddFailed", nil)
			return
		}

		m.responder.JSONResponse(w, http.StatusCreated, moduleLab)
	}
}

func (m *PathwayManager) handleRemoveLabFromModule() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		moduleID := chi.URLParam(r, "moduleID")
		labTemplateID := chi.URLParam(r, "labTemplateID")

		if m.pathwayRepo == nil {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusServiceUnavailable, "pathway.errors.repoNotAvailable", nil)
			return
		}

		if err := m.pathwayRepo.RemoveLabFromModule(r.Context(), moduleID, labTemplateID); err != nil {
			m.logger.Error("Failed to remove lab from module", "error", err)
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusInternalServerError, "pathway.errors.moduleLabRemoveFailed", nil)
			return
		}

		m.responder.JSONResponse(w, http.StatusOK, map[string]string{"status": "removed"})
	}
}

// -----------------------------------------------------------------------------
// Enrollment Handlers
// -----------------------------------------------------------------------------

func (m *PathwayManager) handleEnrollInPathway() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		pathwayIDOrSlug := chi.URLParam(r, "pathwayID")

		ctx := r.Context()
		userID, ok := getUserIDFromContext(ctx)
		if !ok || userID == "" {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusUnauthorized, "pathway.errors.authRequired", nil)
			return
		}

		result, err := m.pathwayService.Enroll(ctx, userID, pathwayIDOrSlug)
		if err != nil {
			status := http.StatusInternalServerError
			msgID := "pathway.errors.enrollFailed"
			if errors.Is(err, pathway.ErrPathwayNotFound) {
				status = http.StatusNotFound
				msgID = "pathway.errors.notFound"
			} else if errors.Is(err, pathway.ErrRepoNotAvailable) {
				status = http.StatusServiceUnavailable
				msgID = "pathway.errors.repoNotAvailable"
			}
			m.responder.LocalizedErrorResponse(r.Context(), w, status, msgID, nil)
			return
		}

		// Return 200 if already enrolled, 201 if newly created
		status := http.StatusOK
		if result.IsNew {
			status = http.StatusCreated
		}
		m.responder.JSONResponse(w, status, result.Enrollment)
	}
}

func (m *PathwayManager) handleUnenrollFromPathway() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		pathwayIDOrSlug := chi.URLParam(r, "pathwayID")

		ctx := r.Context()
		userID, ok := getUserIDFromContext(ctx)
		if !ok || userID == "" {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusUnauthorized, "pathway.errors.authRequired", nil)
			return
		}

		err := m.pathwayService.Unenroll(ctx, userID, pathwayIDOrSlug)
		if err != nil {
			status := http.StatusInternalServerError
			msgID := "pathway.errors.unenrollFailed"
			if errors.Is(err, pathway.ErrPathwayNotFound) {
				status = http.StatusNotFound
				msgID = "pathway.errors.notFound"
			} else if errors.Is(err, pathway.ErrNotEnrolled) {
				status = http.StatusNotFound
				msgID = "pathway.errors.notEnrolled"
			} else if errors.Is(err, pathway.ErrRepoNotAvailable) {
				status = http.StatusServiceUnavailable
				msgID = "pathway.errors.enrollmentRepoNotAvailable"
			}
			m.responder.LocalizedErrorResponse(r.Context(), w, status, msgID, nil)
			return
		}

		m.responder.JSONResponse(w, http.StatusOK, map[string]string{"status": "unenrolled"})
	}
}

func (m *PathwayManager) handleListEnrollments() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if m.enrollmentRepo == nil {
			m.responder.JSONResponse(w, http.StatusOK, map[string]any{
				"enrollments": []any{},
				"count":       0,
			})
			return
		}

		ctx := r.Context()
		userID, ok := getUserIDFromContext(ctx)
		if !ok || userID == "" {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusUnauthorized, "pathway.errors.authRequired", nil)
			return
		}

		opts := models.EnrollmentListOptions{
			UserID: userID,
		}

		if status := r.URL.Query().Get("status"); status != "" {
			opts.Status = models.EnrollmentStatus(status)
		}
		if limit := r.URL.Query().Get("limit"); limit != "" {
			if l, err := strconv.Atoi(limit); err == nil {
				opts.Limit = l
			}
		}

		enrollments, err := m.enrollmentRepo.List(ctx, opts)
		if err != nil {
			m.logger.Error("Failed to list enrollments", "error", err)
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusInternalServerError, "pathway.errors.enrollmentListFailed", nil)
			return
		}

		m.responder.JSONResponse(w, http.StatusOK, map[string]any{
			"enrollments": enrollments,
			"count":       len(enrollments),
		})
	}
}

func (m *PathwayManager) handleGetEnrollment() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		enrollmentID := chi.URLParam(r, "enrollmentID")

		if m.enrollmentRepo == nil {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusServiceUnavailable, "pathway.errors.enrollmentRepoNotAvailable", nil)
			return
		}

		enrollment, err := m.enrollmentRepo.GetByID(r.Context(), enrollmentID)
		if err != nil {
			m.logger.Error("Failed to get enrollment", "error", err)
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusInternalServerError, "pathway.errors.enrollmentGetFailed", nil)
			return
		}
		if enrollment == nil {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusNotFound, "pathway.errors.enrollmentNotFound", nil)
			return
		}

		m.responder.JSONResponse(w, http.StatusOK, enrollment)
	}
}

func (m *PathwayManager) handleGetEnrollmentProgress() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		enrollmentID := chi.URLParam(r, "enrollmentID")

		if m.enrollmentRepo == nil {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusServiceUnavailable, "pathway.errors.enrollmentRepoNotAvailable", nil)
			return
		}

		enrollment, err := m.enrollmentRepo.GetWithProgress(r.Context(), enrollmentID)
		if err != nil {
			m.logger.Error("Failed to get enrollment progress", "error", err)
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusInternalServerError, "pathway.errors.enrollmentProgressFailed", nil)
			return
		}
		if enrollment == nil {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusNotFound, "pathway.errors.enrollmentNotFound", nil)
			return
		}

		m.responder.JSONResponse(w, http.StatusOK, enrollment)
	}
}

// -----------------------------------------------------------------------------
// Module Unlock Handlers
// -----------------------------------------------------------------------------

// handleManuallyUnlockModule allows admins/instructors to manually unlock a module
func (m *PathwayManager) handleManuallyUnlockModule() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		enrollmentID := chi.URLParam(r, "enrollmentID")
		moduleID := chi.URLParam(r, "moduleID")

		ctx := r.Context()

		// Check if user is admin/instructor
		user, ok := auth.UserFromContext(ctx)
		if !ok || user == nil {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusUnauthorized, "pathway.errors.authRequired", nil)
			return
		}

		if !slices.Contains(user.Roles, "admin") && !slices.Contains(user.Roles, "instructor") {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusForbidden, "pathway.errors.adminOrInstructorRequired", nil)
			return
		}

		result, err := m.pathwayService.ManualUnlock(ctx, enrollmentID, moduleID)
		if err != nil {
			status := http.StatusInternalServerError
			msgID := "pathway.errors.moduleUnlockFailed"
			if errors.Is(err, pathway.ErrEnrollmentNotFound) {
				status = http.StatusNotFound
				msgID = "pathway.errors.enrollmentNotFound"
			} else if errors.Is(err, pathway.ErrModuleNotFound) {
				status = http.StatusNotFound
				msgID = "pathway.errors.moduleNotFound"
			} else if errors.Is(err, pathway.ErrModuleNotInPathway) {
				status = http.StatusBadRequest
				msgID = "pathway.errors.moduleNotInPathway"
			} else if errors.Is(err, pathway.ErrRepoNotAvailable) {
				status = http.StatusServiceUnavailable
				msgID = "pathway.errors.repoNotAvailable"
			}
			m.responder.LocalizedErrorResponse(r.Context(), w, status, msgID, nil)
			return
		}

		m.logger.Info("Module manually unlocked",
			"enrollmentId", enrollmentID,
			"moduleId", moduleID,
			"unlockedBy", user.ID,
		)

		m.responder.JSONResponse(w, http.StatusOK, result)
	}
}

// handleGetUnlockRequirements returns what's needed to unlock a specific module
func (m *PathwayManager) handleGetUnlockRequirements() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		enrollmentID := chi.URLParam(r, "enrollmentID")
		moduleID := chi.URLParam(r, "moduleID")

		ctx := r.Context()

		result, err := m.pathwayService.GetUnlockRequirements(ctx, enrollmentID, moduleID)
		if err != nil {
			status := http.StatusInternalServerError
			msgID := "pathway.errors.unlockRequirementsFailed"
			if errors.Is(err, pathway.ErrEnrollmentNotFound) {
				status = http.StatusNotFound
				msgID = "pathway.errors.enrollmentNotFound"
			} else if errors.Is(err, pathway.ErrModuleNotFound) {
				status = http.StatusNotFound
				msgID = "pathway.errors.moduleNotFound"
			} else if errors.Is(err, pathway.ErrRepoNotAvailable) {
				status = http.StatusServiceUnavailable
				msgID = "pathway.errors.repoNotAvailable"
			}
			m.responder.LocalizedErrorResponse(r.Context(), w, status, msgID, nil)
			return
		}

		m.responder.JSONResponse(w, http.StatusOK, result)
	}
}

// -----------------------------------------------------------------------------
// Server Options for Pathway Repositories
// -----------------------------------------------------------------------------

// WithPathwayRepo sets the pathway repository
func WithPathwayRepo(repo repositories.PathwayRepository) ServerOption {
	return func(s *Server) {
		s.ensureDeps().pathwayRepo = repo
	}
}

// WithEnrollmentRepo sets the enrollment repository
func WithEnrollmentRepo(repo repositories.EnrollmentRepository) ServerOption {
	return func(s *Server) {
		s.ensureDeps().enrollmentRepo = repo
	}
}
