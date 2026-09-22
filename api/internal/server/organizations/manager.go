// Package organizations provides organization, team, membership, feature,
// license, and RBAC HTTP handlers.
package organizations

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/toddbartholow/kootenai/api/internal/database/repositories"
	custommiddleware "github.com/toddbartholow/kootenai/api/internal/middleware"
	"github.com/toddbartholow/kootenai/api/internal/models"
	"github.com/toddbartholow/kootenai/api/internal/server/audit"
	"github.com/toddbartholow/kootenai/api/internal/server/features"
	"github.com/toddbartholow/kootenai/api/internal/server/httputil"
	"github.com/toddbartholow/kootenai/api/internal/server/labs"
)

// Manager manages organization, team, feature, and license operations
type Manager struct {
	orgRepo            repositories.OrganizationRepository
	orgMembershipRepo  repositories.OrganizationMembershipRepository
	teamRepo           repositories.TeamRepository
	teamMembershipRepo repositories.TeamMembershipRepository
	featureRepo        repositories.FeatureRepository
	licenseRepo        repositories.LicenseRepository
	userRepo           repositories.UserRepository
	rbacRepo           repositories.RBACRepository
	labMgr             *labs.Manager
	tenantService      *custommiddleware.TenantService
	featuresService    *features.Service
	auditService       *audit.Service
	logger             *slog.Logger
	responder          *httputil.Responder
	permissionService  *custommiddleware.PermissionService
}

// Config configures Manager
type Config struct {
	OrgRepo            repositories.OrganizationRepository
	OrgMembershipRepo  repositories.OrganizationMembershipRepository
	TeamRepo           repositories.TeamRepository
	TeamMembershipRepo repositories.TeamMembershipRepository
	FeatureRepo        repositories.FeatureRepository
	LicenseRepo        repositories.LicenseRepository
	UserRepo           repositories.UserRepository
	RBACRepo           repositories.RBACRepository
	LabMgr             *labs.Manager
	TenantService      *custommiddleware.TenantService
	FeaturesService    *features.Service
	AuditService       *audit.Service
	Logger             *slog.Logger
	Responder          *httputil.Responder
	PermissionService  *custommiddleware.PermissionService
}

// NewManager creates a new Manager
func NewManager(cfg Config) *Manager {
	return &Manager{
		orgRepo:            cfg.OrgRepo,
		orgMembershipRepo:  cfg.OrgMembershipRepo,
		teamRepo:           cfg.TeamRepo,
		teamMembershipRepo: cfg.TeamMembershipRepo,
		featureRepo:        cfg.FeatureRepo,
		licenseRepo:        cfg.LicenseRepo,
		userRepo:           cfg.UserRepo,
		rbacRepo:           cfg.RBACRepo,
		labMgr:             cfg.LabMgr,
		tenantService:      cfg.TenantService,
		featuresService:    cfg.FeaturesService,
		auditService:       cfg.AuditService,
		logger:             cfg.Logger,
		responder:          cfg.Responder,
		permissionService:  cfg.PermissionService,
	}
}

// logAdminAction logs an admin action to the audit service if available.
func (m *Manager) logAdminAction(r *http.Request, action, resourceType, resourceID string, details map[string]any) {
	if m.auditService != nil {
		m.auditService.LogAction(r, action, resourceType, resourceID, details)
	}
}

// SetupRoutes registers organization-related routes on the router
func (m *Manager) SetupRoutes(r chi.Router) {
	ps := m.permissionService

	// Public invitation endpoint (no org context needed)
	r.Post("/invitations/{token}/accept", m.handleAcceptInvitation())

	// Organization routes (user's organizations)
	r.Route("/organizations", func(r chi.Router) {
		r.Get("/", m.handleListOrganizations())
		r.With(requirePerm(ps, custommiddleware.PermAdminSystem)).Post("/", m.handleCreateOrganization())

		// Organization-scoped routes with tenant middleware
		r.Route("/{orgID}", func(r chi.Router) {
			if m.tenantService != nil {
				r.Use(m.tenantService.Middleware)
			}

			r.Get("/", m.handleGetOrganization())
			r.With(requirePerm(ps, custommiddleware.PermOrgUpdate)).Put("/", m.handleUpdateOrganization())
			r.With(requirePerm(ps, custommiddleware.PermAdminSystem)).Delete("/", m.handleDeleteOrganization())

			// Members
			r.Route("/members", func(r chi.Router) {
				r.Get("/", m.handleListMembers())
				r.With(requirePerm(ps, custommiddleware.PermOrgManage)).Post("/", m.handleInviteMember())
				r.With(requirePerm(ps, custommiddleware.PermOrgManage)).Put("/{userID}", m.handleUpdateMember())
				r.With(requirePerm(ps, custommiddleware.PermOrgManage)).Delete("/{userID}", m.handleRemoveMember())
			})

			// Teams (requires teams feature)
			r.Route("/teams", func(r chi.Router) {
				r.Use(custommiddleware.RequireFeature(features.FeatureTeams))
				r.Get("/", m.handleListTeams())
				r.With(requirePerm(ps, custommiddleware.PermTeamsCreate)).Post("/", m.handleCreateTeam())
			})

			// Org-scoped labs — templates owned by this organization
			if m.labMgr != nil {
				r.Route("/labs", func(r chi.Router) {
					r.Get("/", m.labMgr.HandleListOrgLabs())
					r.With(requirePerm(ps, custommiddleware.PermLabsCreate)).Post("/", m.labMgr.HandleCreateOrgLab())
				})
			}

			// Features (delegated to features package)
			if m.featuresService != nil {
				features.SetupOrgRoutes(r, m.featuresService, m.responder)
			}

			// License
			r.Get("/license", m.handleGetLicense())
			r.With(requirePerm(ps, custommiddleware.PermAdminLicenses)).Post("/license", m.handleActivateLicense())
			r.With(requirePerm(ps, custommiddleware.PermAdminLicenses)).Delete("/license", m.handleDeactivateLicense())
			r.Post("/license/validate", m.handleValidateLicense())

			// RBAC roles and user role assignments
			r.Route("/roles", func(r chi.Router) {
				r.Get("/", m.handleListRoles())
				r.Get("/{roleID}", m.handleGetRole())

				// Custom role management (Professional+ edition)
				r.Group(func(r chi.Router) {
					r.Use(custommiddleware.RequireEdition(models.EditionProfessional))
					r.With(requirePerm(ps, custommiddleware.PermRBACRolesCreate)).Post("/", m.handleCreateCustomRole())
					r.With(requirePerm(ps, custommiddleware.PermRBACRolesUpdate)).Put("/{roleID}", m.handleUpdateCustomRole())
					r.With(requirePerm(ps, custommiddleware.PermRBACRolesDelete)).Delete("/{roleID}", m.handleDeleteCustomRole())
					r.With(requirePerm(ps, custommiddleware.PermRBACRolesCreate)).Post("/{roleID}/clone", m.handleCloneRole())
				})
			})

			r.Get("/permissions", m.handleListPermissions())

			r.Route("/users/{userID}/roles", func(r chi.Router) {
				r.Get("/", m.handleListUserRoles())
				r.With(requirePerm(ps, custommiddleware.PermRBACAssign)).Post("/", m.handleAssignRole())
				r.With(requirePerm(ps, custommiddleware.PermRBACAssign)).Delete("/{roleID}", m.handleRevokeRole())
			})

			// Audit logs (delegated to audit package)
			audit.SetupRoutes(r, m.auditService, m.responder)
		})
	})

	// Team routes (direct access by team ID)
	r.Route("/teams/{teamID}", func(r chi.Router) {
		r.Get("/", m.handleGetTeam())
		r.Put("/", m.handleUpdateTeam())
		r.Delete("/", m.handleDeleteTeam())

		r.Route("/members", func(r chi.Router) {
			r.Get("/", m.handleListTeamMembers())
			r.Post("/", m.handleAddTeamMember())
			r.Put("/{userID}", m.handleUpdateTeamMember())
			r.Delete("/{userID}", m.handleRemoveTeamMember())
		})
	})

	// Global features list (delegated to features package)
	if m.featuresService != nil {
		features.SetupGlobalRoutes(r, m.featuresService, m.responder)
	}
}

// requirePerm returns a middleware that checks for a specific permission.
// If ps is nil (RBAC not configured), it returns a no-op passthrough middleware.
func requirePerm(ps *custommiddleware.PermissionService, perm string) func(http.Handler) http.Handler {
	if ps == nil {
		return func(next http.Handler) http.Handler { return next }
	}
	return ps.RequirePermission(perm)
}
