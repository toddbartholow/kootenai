// Package audit provides audit log API handlers
package audit

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/toddbartholow/kootenai/api/internal/auth"
	"github.com/toddbartholow/kootenai/api/internal/database/repositories"
	custommiddleware "github.com/toddbartholow/kootenai/api/internal/middleware"
	"github.com/toddbartholow/kootenai/api/internal/models"
)

// Service handles audit log operations
type Service struct {
	repo   repositories.AuditLogRepository
	logger *slog.Logger
}

// NewService creates a new audit service
func NewService(repo repositories.AuditLogRepository, logger *slog.Logger) *Service {
	return &Service{
		repo:   repo,
		logger: logger,
	}
}

// ListResponse represents the response for listing audit logs
type ListResponse struct {
	Entries    []*models.AuditEntry `json:"entries"`
	TotalCount int                  `json:"totalCount"`
	Limit      int                  `json:"limit"`
	Offset     int                  `json:"offset"`
}

// ExportRequest represents a request to export audit logs
type ExportRequest struct {
	StartTime *time.Time `json:"startTime,omitempty"`
	EndTime   *time.Time `json:"endTime,omitempty"`
	Format    string     `json:"format,omitempty"` // "json" or "csv"
}

// ResponseWriter is an interface for HTTP response writing (allows dependency injection)
type ResponseWriter interface {
	JSONResponse(w http.ResponseWriter, status int, data any)
	ErrorResponse(w http.ResponseWriter, status int, message string)
	LocalizedErrorResponse(ctx context.Context, w http.ResponseWriter, status int, messageID string, templateData map[string]any)
}

// HandleList lists audit log entries for an organization
// @Summary List audit log entries
// @Description Returns audit log entries for the organization, filtered by optional query parameters
// @Tags Audit
// @Accept json
// @Produce json
// @Param orgID path string true "Organization ID"
// @Param actor query string false "Filter by actor ID"
// @Param action query string false "Filter by action type"
// @Param resource_type query string false "Filter by resource type"
// @Param resource_id query string false "Filter by resource ID"
// @Param start_time query string false "Filter by start time (RFC3339)"
// @Param end_time query string false "Filter by end time (RFC3339)"
// @Param limit query int false "Maximum number of entries (default 50, max 500)"
// @Param offset query int false "Offset for pagination"
// @Success 200 {object} ListResponse
// @Failure 400 {object} map[string]string "Invalid parameters"
// @Failure 401 {object} map[string]string "Unauthorized"
// @Failure 403 {object} map[string]string "Forbidden - requires admin role"
// @Failure 500 {object} map[string]string "Internal server error"
// @Security BearerAuth
// @Router /organizations/{orgID}/audit [get]
func (svc *Service) HandleList(rw ResponseWriter) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		// Get tenant context for org-scoped audit
		tenantCtx, ok := custommiddleware.TenantFromContext(ctx)
		if !ok || tenantCtx == nil || tenantCtx.Organization == nil {
			rw.LocalizedErrorResponse(ctx, w, http.StatusForbidden, "audit.errors.orgRequired", nil)
			return
		}

		// Verify audit repository is available
		if svc.repo == nil {
			rw.LocalizedErrorResponse(ctx, w, http.StatusServiceUnavailable, "audit.errors.notConfigured", nil)
			return
		}

		// Parse query parameters
		filter := repositories.AuditFilter{
			ActorID:      r.URL.Query().Get("actor"),
			Action:       r.URL.Query().Get("action"),
			ResourceType: r.URL.Query().Get("resource_type"),
			ResourceID:   r.URL.Query().Get("resource_id"),
		}

		// Parse time filters
		if startStr := r.URL.Query().Get("start_time"); startStr != "" {
			t, err := time.Parse(time.RFC3339, startStr)
			if err != nil {
				rw.LocalizedErrorResponse(ctx, w, http.StatusBadRequest, "audit.errors.invalidStartTime", nil)
				return
			}
			filter.StartTime = &t
		}

		if endStr := r.URL.Query().Get("end_time"); endStr != "" {
			t, err := time.Parse(time.RFC3339, endStr)
			if err != nil {
				rw.LocalizedErrorResponse(ctx, w, http.StatusBadRequest, "audit.errors.invalidEndTime", nil)
				return
			}
			filter.EndTime = &t
		}

		// Parse pagination
		filter.Limit = 50 // default
		if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
			limit, err := strconv.Atoi(limitStr)
			if err != nil || limit < 1 {
				rw.LocalizedErrorResponse(ctx, w, http.StatusBadRequest, "audit.errors.invalidLimit", nil)
				return
			}
			if limit > 500 {
				limit = 500 // max limit
			}
			filter.Limit = limit
		}

		if offsetStr := r.URL.Query().Get("offset"); offsetStr != "" {
			offset, err := strconv.Atoi(offsetStr)
			if err != nil || offset < 0 {
				rw.LocalizedErrorResponse(ctx, w, http.StatusBadRequest, "audit.errors.invalidOffset", nil)
				return
			}
			filter.Offset = offset
		}

		// Query audit logs
		entries, err := svc.repo.Query(ctx, filter)
		if err != nil {
			svc.logger.Error("Failed to query audit logs", "error", err)
			rw.LocalizedErrorResponse(ctx, w, http.StatusInternalServerError, "audit.errors.queryFailed", nil)
			return
		}

		response := ListResponse{
			Entries:    entries,
			TotalCount: len(entries), // Note: for proper total, we'd need a Count query
			Limit:      filter.Limit,
			Offset:     filter.Offset,
		}

		rw.JSONResponse(w, http.StatusOK, response)
	}
}

// HandleGet gets a single audit log entry
// @Summary Get audit log entry
// @Description Returns a single audit log entry by ID
// @Tags Audit
// @Accept json
// @Produce json
// @Param orgID path string true "Organization ID"
// @Param entryID path string true "Audit entry ID"
// @Success 200 {object} models.AuditEntry
// @Failure 401 {object} map[string]string "Unauthorized"
// @Failure 403 {object} map[string]string "Forbidden - requires admin role"
// @Failure 404 {object} map[string]string "Entry not found"
// @Failure 500 {object} map[string]string "Internal server error"
// @Security BearerAuth
// @Router /organizations/{orgID}/audit/{entryID} [get]
func (svc *Service) HandleGet(rw ResponseWriter) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		entryID := chi.URLParam(r, "entryID")

		// Get tenant context
		tenantCtx, ok := custommiddleware.TenantFromContext(ctx)
		if !ok || tenantCtx == nil || tenantCtx.Organization == nil {
			rw.LocalizedErrorResponse(ctx, w, http.StatusForbidden, "audit.errors.orgRequired", nil)
			return
		}

		if svc.repo == nil {
			rw.LocalizedErrorResponse(ctx, w, http.StatusServiceUnavailable, "audit.errors.notConfigured", nil)
			return
		}

		// Query by ID
		filter := repositories.AuditFilter{
			ResourceID: entryID, // Using ResourceID as a simple filter - we might want a GetByID method
			Limit:      1,
		}

		entries, err := svc.repo.Query(ctx, filter)
		if err != nil {
			svc.logger.Error("Failed to get audit entry", "error", err, "entryID", entryID)
			rw.LocalizedErrorResponse(ctx, w, http.StatusInternalServerError, "audit.errors.getFailed", nil)
			return
		}

		if len(entries) == 0 {
			rw.LocalizedErrorResponse(ctx, w, http.StatusNotFound, "audit.errors.entryNotFound", nil)
			return
		}

		rw.JSONResponse(w, http.StatusOK, entries[0])
	}
}

// HandleExport exports audit logs for compliance/archival
// @Summary Export audit logs
// @Description Exports audit log entries for the specified time range
// @Tags Audit
// @Accept json
// @Produce json
// @Param orgID path string true "Organization ID"
// @Param request body ExportRequest true "Export parameters"
// @Success 200 {array} models.AuditEntry
// @Failure 400 {object} map[string]string "Invalid parameters"
// @Failure 401 {object} map[string]string "Unauthorized"
// @Failure 403 {object} map[string]string "Forbidden - requires admin role"
// @Failure 500 {object} map[string]string "Internal server error"
// @Security BearerAuth
// @Router /organizations/{orgID}/audit/export [post]
func (svc *Service) HandleExport(rw ResponseWriter) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		// Get tenant context
		tenantCtx, ok := custommiddleware.TenantFromContext(ctx)
		if !ok || tenantCtx == nil || tenantCtx.Organization == nil {
			rw.LocalizedErrorResponse(ctx, w, http.StatusForbidden, "audit.errors.orgRequired", nil)
			return
		}

		if svc.repo == nil {
			rw.LocalizedErrorResponse(ctx, w, http.StatusServiceUnavailable, "audit.errors.notConfigured", nil)
			return
		}

		// Parse request body
		var req ExportRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			rw.LocalizedErrorResponse(ctx, w, http.StatusBadRequest, "audit.errors.invalidRequestBody", nil)
			return
		}

		// Default to last 30 days if no time range specified
		if req.StartTime == nil {
			thirtyDaysAgo := time.Now().AddDate(0, 0, -30)
			req.StartTime = &thirtyDaysAgo
		}
		if req.EndTime == nil {
			now := time.Now()
			req.EndTime = &now
		}

		// Build filter - no limit for export (or use a high limit)
		filter := repositories.AuditFilter{
			StartTime: req.StartTime,
			EndTime:   req.EndTime,
			Limit:     10000, // Export limit
		}

		entries, err := svc.repo.Query(ctx, filter)
		if err != nil {
			svc.logger.Error("Failed to export audit logs", "error", err)
			rw.LocalizedErrorResponse(ctx, w, http.StatusInternalServerError, "audit.errors.exportFailed", nil)
			return
		}

		if req.Format == "csv" {
			w.Header().Set("Content-Type", "text/csv")
			w.Header().Set("Content-Disposition", "attachment; filename=audit-export.csv")
			w.WriteHeader(http.StatusOK)

			writer := csv.NewWriter(w)
			// Write header row
			writer.Write([]string{"id", "timestamp", "actorId", "actorType", "action", "resourceType", "resourceId", "ipAddress", "userAgent", "details"})

			for _, e := range entries {
				details := ""
				if e.Details != nil {
					details = string(e.Details)
				}
				writer.Write([]string{
					fmt.Sprintf("%d", e.ID),
					e.Timestamp.Format(time.RFC3339),
					e.ActorID,
					e.ActorType,
					e.Action,
					e.ResourceType,
					e.ResourceID,
					e.IPAddress,
					e.UserAgent,
					details,
				})
			}
			writer.Flush()
			return
		}

		rw.JSONResponse(w, http.StatusOK, map[string]any{
			"entries":    entries,
			"count":      len(entries),
			"startTime":  req.StartTime,
			"endTime":    req.EndTime,
			"exportedAt": time.Now(),
		})
	}
}

// LogAction logs an administrative action to the audit log
func (svc *Service) LogAction(r *http.Request, action string, resourceType string, resourceID string, details map[string]any) {
	if svc.repo == nil {
		return
	}

	ctx := r.Context()

	// Get actor from auth context
	actorID := "unknown"
	if user, ok := auth.UserFromContext(r.Context()); ok {
		actorID = user.ID
	}

	// Build details JSON
	var detailsJSON json.RawMessage
	if details != nil {
		if jsonBytes, err := json.Marshal(details); err == nil {
			detailsJSON = jsonBytes
		}
	}

	entry := &models.AuditEntry{
		ActorID:      actorID,
		ActorType:    "user",
		Action:       action,
		ResourceType: resourceType,
		ResourceID:   resourceID,
		Details:      detailsJSON,
		IPAddress:    GetClientIP(r),
		UserAgent:    r.UserAgent(),
	}

	if err := svc.repo.Create(ctx, entry); err != nil {
		svc.logger.Error("Failed to log admin action", "error", err, "action", action)
	}
}

// PermissionAuditAdapter adapts the audit Service to the middleware PermissionAuditLogger interface
type PermissionAuditAdapter struct {
	svc *Service
}

// NewPermissionAuditAdapter creates a new adapter
func NewPermissionAuditAdapter(svc *Service) *PermissionAuditAdapter {
	return &PermissionAuditAdapter{svc: svc}
}

// LogPermissionCheck logs a permission check to the audit log
func (a *PermissionAuditAdapter) LogPermissionCheck(ctx context.Context, entry custommiddleware.PermissionAuditEntry) {
	if a.svc.repo == nil {
		return
	}

	detailsJSON, _ := json.Marshal(map[string]any{
		"permission": entry.Permission,
		"decision":   entry.Decision,
		"method":     entry.Method,
		"resource":   entry.Resource,
		"orgId":      entry.OrgID,
	})

	auditEntry := &models.AuditEntry{
		ActorID:      entry.UserID,
		ActorType:    "user",
		Action:       "permission_check",
		ResourceType: "permission",
		ResourceID:   entry.Permission,
		Details:      detailsJSON,
	}

	if err := a.svc.repo.Create(ctx, auditEntry); err != nil {
		a.svc.logger.Error("Failed to log permission check", "error", err)
	}
}

// GetClientIP extracts the real client IP from the request.
// Prefers X-Real-IP (set by nginx, harder to spoof) over RemoteAddr.
// X-Forwarded-For is NOT used as it can be trivially spoofed by clients.
func GetClientIP(r *http.Request) string {
	// Prefer X-Real-IP header (set by nginx, single value)
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return strings.TrimSpace(xri)
	}
	// Fall back to RemoteAddr
	ip := r.RemoteAddr
	// Strip port if present
	if idx := strings.LastIndex(ip, ":"); idx != -1 {
		return ip[:idx]
	}
	return ip
}
