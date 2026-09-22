// Package labs provides lab template HTTP handlers.
package labs

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/toddbartholow/kootenai/api/internal/database/repositories"
	custommiddleware "github.com/toddbartholow/kootenai/api/internal/middleware"
	"github.com/toddbartholow/kootenai/api/internal/server/httputil"
)

// Manager manages lab template operations (CRUD)
type Manager struct {
	labTemplateRepo   repositories.LabTemplateRepository
	logger            *slog.Logger
	responder         *httputil.Responder
	permissionService *custommiddleware.PermissionService
}

// Config configures Manager
type Config struct {
	LabTemplateRepo   repositories.LabTemplateRepository
	Logger            *slog.Logger
	Responder         *httputil.Responder
	PermissionService *custommiddleware.PermissionService
}

// NewManager creates a new Manager
func NewManager(cfg Config) *Manager {
	return &Manager{
		labTemplateRepo:   cfg.LabTemplateRepo,
		logger:            cfg.Logger,
		responder:         cfg.Responder,
		permissionService: cfg.PermissionService,
	}
}

// SetupRoutes registers lab routes on the router
// These routes are under /api/v1/labs
func (m *Manager) SetupRoutes(r chi.Router) {
	ps := m.permissionService

	r.Route("/labs", func(r chi.Router) {
		r.Get("/", m.handleListLabs())
		r.With(requirePerm(ps, custommiddleware.PermLabsCreate)).Post("/", m.handleCreateLab())
		r.With(requirePerm(ps, custommiddleware.PermLabsCreate)).Post("/import", m.handleImportLab())
		r.Get("/{labID}", m.handleGetLab())
		r.With(requirePerm(ps, custommiddleware.PermLabsUpdate)).Put("/{labID}", m.handleUpdateLab())
		r.With(requirePerm(ps, custommiddleware.PermLabsDelete)).Delete("/{labID}", m.handleDeleteLab())
		r.With(requirePerm(ps, custommiddleware.PermLabsUpdate)).Put("/{labID}/active", m.handleSetLabActive())
		r.Get("/{labID}/instructions", m.handleGetLabInstructions())
		r.Get("/{labID}/export", m.handleExportLab())
		r.Get("/{labID}/versions", m.handleListVersions())
		r.Get("/{labID}/versions/{versionNumber}", m.handleGetVersion())
		r.With(requirePerm(ps, custommiddleware.PermLabsUpdate)).Post("/{labID}/versions/{versionNumber}/restore", m.handleRestoreVersion())
	})
}

// requirePerm returns a middleware that checks for a specific permission.
// If ps is nil (RBAC not configured), it returns a no-op passthrough middleware.
func requirePerm(ps *custommiddleware.PermissionService, perm string) func(http.Handler) http.Handler {
	if ps == nil {
		return func(next http.Handler) http.Handler { return next }
	}
	return ps.RequirePermission(perm)
}
