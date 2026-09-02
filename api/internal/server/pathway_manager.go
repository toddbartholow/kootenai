package server

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/toddbartholow/kootenai/api/internal/database/repositories"
	custommiddleware "github.com/toddbartholow/kootenai/api/internal/middleware"
	"github.com/toddbartholow/kootenai/api/internal/pathway"
	"github.com/toddbartholow/kootenai/api/internal/server/httputil"
)

// PathwayManager manages learning pathway operations
type PathwayManager struct {
	// Service layer (preferred for new code)
	pathwayService *pathway.Service

	// Repositories (kept for handlers not yet migrated to service)
	pathwayRepo       repositories.PathwayRepository
	enrollmentRepo    repositories.EnrollmentRepository
	logger            *slog.Logger
	responder         *httputil.Responder
	permissionService *custommiddleware.PermissionService
}

// PathwayManagerConfig configures PathwayManager
type PathwayManagerConfig struct {
	// Service layer (preferred for new code)
	PathwayService *pathway.Service

	// Repositories (kept for handlers not yet migrated to service)
	PathwayRepo       repositories.PathwayRepository
	EnrollmentRepo    repositories.EnrollmentRepository
	Logger            *slog.Logger
	Responder         *httputil.Responder
	PermissionService *custommiddleware.PermissionService
}

// NewPathwayManager creates a new PathwayManager
func NewPathwayManager(cfg PathwayManagerConfig) *PathwayManager {
	return &PathwayManager{
		pathwayService:    cfg.PathwayService,
		pathwayRepo:       cfg.PathwayRepo,
		enrollmentRepo:    cfg.EnrollmentRepo,
		logger:            cfg.Logger,
		responder:         cfg.Responder,
		permissionService: cfg.PermissionService,
	}
}

// PathwayExternalHandlers holds handler functions from other managers/Server
// that are registered alongside pathway routes but not owned by PathwayManager.
// Recommendation routes (/recommendations and /pathways/{pathwayID}/next-lab)
// were moved out — they now live in RecommendationManager.SetupRoutes.
type PathwayExternalHandlers struct {
	IssueCertificate     http.HandlerFunc
	GetCertificate       http.HandlerFunc
	ListUserCertificates http.HandlerFunc
	VerifyCertificate    http.HandlerFunc
}

// SetupRoutes registers pathway-related routes on the router
func (m *PathwayManager) SetupRoutes(r chi.Router, ext PathwayExternalHandlers) {
	ps := m.permissionService

	// Pathways
	r.Route("/pathways", func(r chi.Router) {
		r.Get("/", m.handleListPathways())
		r.With(requirePerm(ps, custommiddleware.PermPathwaysCreate)).Post("/", m.handleCreatePathway())
		r.Get("/{pathwayID}", m.handleGetPathway())
		r.With(requirePerm(ps, custommiddleware.PermPathwaysUpdate)).Put("/{pathwayID}", m.handleUpdatePathway())
		r.With(requirePerm(ps, custommiddleware.PermPathwaysDelete)).Delete("/{pathwayID}", m.handleDeletePathway())
		r.With(requirePerm(ps, custommiddleware.PermPathwaysUpdate)).Post("/{pathwayID}/publish", m.handlePublishPathway())
		r.With(requirePerm(ps, custommiddleware.PermPathwaysUpdate)).Post("/{pathwayID}/archive", m.handleArchivePathway())
		r.Get("/{pathwayID}/stats", m.handleGetPathwayStats())

		// Enrollment
		r.Post("/{pathwayID}/enroll", m.handleEnrollInPathway())
		r.Delete("/{pathwayID}/enroll", m.handleUnenrollFromPathway())

		// Modules
		r.Get("/{pathwayID}/modules", m.handleListModules())
		r.With(requirePerm(ps, custommiddleware.PermPathwaysUpdate)).Post("/{pathwayID}/modules", m.handleCreateModule())
	})

	// Modules (standalone routes for updates/deletes)
	r.Route("/modules", func(r chi.Router) {
		r.With(requirePerm(ps, custommiddleware.PermPathwaysUpdate)).Put("/{moduleID}", m.handleUpdateModule())
		r.With(requirePerm(ps, custommiddleware.PermPathwaysDelete)).Delete("/{moduleID}", m.handleDeleteModule())
		r.With(requirePerm(ps, custommiddleware.PermPathwaysUpdate)).Post("/{moduleID}/labs", m.handleAddLabToModule())
		r.With(requirePerm(ps, custommiddleware.PermPathwaysUpdate)).Delete("/{moduleID}/labs/{labTemplateID}", m.handleRemoveLabFromModule())
	})

	// Enrollments
	r.Route("/enrollments", func(r chi.Router) {
		r.Get("/", m.handleListEnrollments())
		r.Get("/{enrollmentID}", m.handleGetEnrollment())
		r.Get("/{enrollmentID}/progress", m.handleGetEnrollmentProgress())

		// Module unlock endpoints
		r.Get("/{enrollmentID}/modules/{moduleID}/unlock-requirements", m.handleGetUnlockRequirements())
		r.Post("/{enrollmentID}/modules/{moduleID}/unlock", m.handleManuallyUnlockModule())

		// Certificate endpoints (owned by Server, not PathwayManager)
		r.Post("/{enrollmentID}/certificate", ext.IssueCertificate)
		r.Get("/{enrollmentID}/certificate", ext.GetCertificate)
	})

	// Certificates (owned by Server)
	r.Route("/certificates", func(r chi.Router) {
		r.Get("/", ext.ListUserCertificates)
		r.Get("/verify/{code}", ext.VerifyCertificate)
	})
}
