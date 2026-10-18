// Package middleware provides HTTP middleware for the Kootenai API
package middleware

import (
	"context"
	"log/slog"
	"net/http"
	"slices"

	"github.com/toddbartholow/kootenai/api/internal/auth"
	"github.com/toddbartholow/kootenai/api/internal/database/repositories"
)

// PermissionContextKey is the context key for cached user permissions
type PermissionContextKey struct{}

// CachedPermissions holds permissions loaded for the current request
type CachedPermissions struct {
	UserID      string
	OrgID       *string
	Permissions map[string]bool
}

// PermissionAuditLogger logs permission check outcomes for compliance
type PermissionAuditLogger interface {
	LogPermissionCheck(ctx context.Context, entry PermissionAuditEntry)
}

// PermissionAuditEntry represents a permission check event
type PermissionAuditEntry struct {
	UserID     string
	Permission string
	Decision   string // "allowed" or "denied"
	Resource   string // request path
	Method     string
	OrgID      *string
}

// PermissionService provides RBAC permission checking functionality
type PermissionService struct {
	rbacRepo    repositories.RBACRepository
	logger      *slog.Logger
	auditLogger PermissionAuditLogger
}

// NewPermissionService creates a new permission service
func NewPermissionService(rbacRepo repositories.RBACRepository, logger *slog.Logger) *PermissionService {
	if logger == nil {
		logger = slog.Default()
	}
	return &PermissionService{
		rbacRepo: rbacRepo,
		logger:   logger,
	}
}

// WithAuditLogger sets the audit logger for permission checks
func (s *PermissionService) WithAuditLogger(al PermissionAuditLogger) *PermissionService {
	s.auditLogger = al
	return s
}

// LoadPermissions creates middleware that loads user permissions into context
// This is useful to run once early in the chain to cache permissions for later checks
func (s *PermissionService) LoadPermissions(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		// Get authenticated user
		user, ok := auth.UserFromContext(ctx)
		if !ok {
			next.ServeHTTP(w, r)
			return
		}

		// Get organization context if available
		var orgID *string
		if tc, ok := TenantFromContext(ctx); ok && tc != nil && tc.Organization != nil {
			orgID = &tc.Organization.ID
		}

		// Load all user permissions
		permissions, err := s.rbacRepo.GetUserPermissions(ctx, user.ID, orgID)
		if err != nil {
			s.logger.Warn("Failed to load user permissions",
				slog.String("user_id", user.ID),
				slog.Any("error", err))
			// Continue without cached permissions - individual checks will still work
			next.ServeHTTP(w, r)
			return
		}

		// Convert to map for O(1) lookups
		permMap := make(map[string]bool, len(permissions))
		for _, p := range permissions {
			permMap[p] = true
		}

		cached := &CachedPermissions{
			UserID:      user.ID,
			OrgID:       orgID,
			Permissions: permMap,
		}

		ctx = context.WithValue(ctx, PermissionContextKey{}, cached)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// PermissionsFromContext retrieves cached permissions from context
func PermissionsFromContext(ctx context.Context) (*CachedPermissions, bool) {
	cached, ok := ctx.Value(PermissionContextKey{}).(*CachedPermissions)
	return cached, ok
}

// RequirePermission creates middleware that requires a specific permission
// Permission IDs follow the format "resource.action" (e.g., "pods.create", "labs.delete")
func (s *PermissionService) RequirePermission(permissionID string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()

			// Get authenticated user
			user, ok := auth.UserFromContext(ctx)
			if !ok {
				writeJSONError(w, http.StatusUnauthorized, "Authentication required")
				return
			}

			// Check for system admin in JWT roles (bypasses RBAC)
			if hasJWTRole(user, "admin") {
				next.ServeHTTP(w, r)
				return
			}

			// Check cached permissions first
			if cached, ok := PermissionsFromContext(ctx); ok {
				if cached.Permissions[permissionID] {
					next.ServeHTTP(w, r)
					return
				}
				// Permission not in cache means denied
				s.logDenied(r, user.ID, permissionID)
				writeJSONError(w, http.StatusForbidden, "Permission denied")
				return
			}

			// No cached permissions - check database directly
			var orgID *string
			if tc, ok := TenantFromContext(ctx); ok && tc != nil && tc.Organization != nil {
				orgID = &tc.Organization.ID
			}

			hasPermission, err := s.rbacRepo.HasPermission(ctx, user.ID, permissionID, orgID)
			if err != nil {
				s.logger.Error("Permission check failed",
					slog.String("user_id", user.ID),
					slog.String("permission", permissionID),
					slog.Any("error", err))
				writeJSONError(w, http.StatusInternalServerError, "Permission check failed")
				return
			}

			if !hasPermission {
				s.logDenied(r, user.ID, permissionID)
				writeJSONError(w, http.StatusForbidden, "Permission denied")
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// RequireAnyPermission creates middleware that requires at least one of the permissions
func (s *PermissionService) RequireAnyPermission(permissionIDs ...string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()

			// Get authenticated user
			user, ok := auth.UserFromContext(ctx)
			if !ok {
				writeJSONError(w, http.StatusUnauthorized, "Authentication required")
				return
			}

			// Check for system admin in JWT roles (bypasses RBAC)
			if hasJWTRole(user, "admin") {
				next.ServeHTTP(w, r)
				return
			}

			// Check cached permissions first
			if cached, ok := PermissionsFromContext(ctx); ok {
				for _, permissionID := range permissionIDs {
					if cached.Permissions[permissionID] {
						next.ServeHTTP(w, r)
						return
					}
				}
				// No matching permission in cache
				s.logger.Debug("Permission denied (any)",
					slog.String("user_id", user.ID),
					slog.Any("required_permissions", permissionIDs))
				writeJSONError(w, http.StatusForbidden, "Permission denied")
				return
			}

			// No cached permissions - check database
			var orgID *string
			if tc, ok := TenantFromContext(ctx); ok && tc != nil && tc.Organization != nil {
				orgID = &tc.Organization.ID
			}

			results, err := s.rbacRepo.CheckPermissions(ctx, user.ID, permissionIDs, orgID)
			if err != nil {
				s.logger.Error("Permission check failed",
					slog.String("user_id", user.ID),
					slog.Any("permissions", permissionIDs),
					slog.Any("error", err))
				writeJSONError(w, http.StatusInternalServerError, "Permission check failed")
				return
			}

			for _, hasPermission := range results {
				if hasPermission {
					next.ServeHTTP(w, r)
					return
				}
			}

			s.logger.Debug("Permission denied (any)",
				slog.String("user_id", user.ID),
				slog.Any("required_permissions", permissionIDs))
			writeJSONError(w, http.StatusForbidden, "Permission denied")
		})
	}
}

// RequireAllPermissions creates middleware that requires all specified permissions
func (s *PermissionService) RequireAllPermissions(permissionIDs ...string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()

			// Get authenticated user
			user, ok := auth.UserFromContext(ctx)
			if !ok {
				writeJSONError(w, http.StatusUnauthorized, "Authentication required")
				return
			}

			// Check for system admin in JWT roles (bypasses RBAC)
			if hasJWTRole(user, "admin") {
				next.ServeHTTP(w, r)
				return
			}

			// Check cached permissions first
			if cached, ok := PermissionsFromContext(ctx); ok {
				for _, permissionID := range permissionIDs {
					if !cached.Permissions[permissionID] {
						s.logger.Debug("Permission denied (all)",
							slog.String("user_id", user.ID),
							slog.String("missing", permissionID))
						writeJSONError(w, http.StatusForbidden, "Permission denied")
						return
					}
				}
				next.ServeHTTP(w, r)
				return
			}

			// No cached permissions - check database
			var orgID *string
			if tc, ok := TenantFromContext(ctx); ok && tc != nil && tc.Organization != nil {
				orgID = &tc.Organization.ID
			}

			results, err := s.rbacRepo.CheckPermissions(ctx, user.ID, permissionIDs, orgID)
			if err != nil {
				s.logger.Error("Permission check failed",
					slog.String("user_id", user.ID),
					slog.Any("permissions", permissionIDs),
					slog.Any("error", err))
				writeJSONError(w, http.StatusInternalServerError, "Permission check failed")
				return
			}

			for permissionID, hasPermission := range results {
				if !hasPermission {
					s.logger.Debug("Permission denied (all)",
						slog.String("user_id", user.ID),
						slog.String("missing", permissionID))
					writeJSONError(w, http.StatusForbidden, "Permission denied")
					return
				}
			}

			next.ServeHTTP(w, r)
		})
	}
}

// CheckPermission performs a permission check and returns the result without blocking
// Useful for conditional logic in handlers
func (s *PermissionService) CheckPermission(ctx context.Context, permissionID string) bool {
	user, ok := auth.UserFromContext(ctx)
	if !ok {
		return false
	}

	// Check for system admin in JWT roles
	if hasJWTRole(user, "admin") {
		return true
	}

	// Check cached permissions
	if cached, ok := PermissionsFromContext(ctx); ok {
		return cached.Permissions[permissionID]
	}

	// Fallback to database check
	var orgID *string
	if tc, ok := TenantFromContext(ctx); ok && tc != nil && tc.Organization != nil {
		orgID = &tc.Organization.ID
	}

	hasPermission, err := s.rbacRepo.HasPermission(ctx, user.ID, permissionID, orgID)
	if err != nil {
		s.logger.Warn("Permission check failed",
			slog.String("user_id", user.ID),
			slog.String("permission", permissionID),
			slog.Any("error", err))
		return false
	}

	return hasPermission
}

// GetUserPermissions returns all permissions for the current user
func (s *PermissionService) GetUserPermissions(ctx context.Context) ([]string, error) {
	user, ok := auth.UserFromContext(ctx)
	if !ok {
		return nil, nil
	}

	// Check cached permissions
	if cached, ok := PermissionsFromContext(ctx); ok {
		permissions := make([]string, 0, len(cached.Permissions))
		for p := range cached.Permissions {
			permissions = append(permissions, p)
		}
		return permissions, nil
	}

	// Fallback to database
	var orgID *string
	if tc, ok := TenantFromContext(ctx); ok && tc != nil && tc.Organization != nil {
		orgID = &tc.Organization.ID
	}

	return s.rbacRepo.GetUserPermissions(ctx, user.ID, orgID)
}

// logDenied logs a permission denial
func (s *PermissionService) logDenied(r *http.Request, userID, permissionID string) {
	s.logger.Debug("Permission denied",
		slog.String("user_id", userID),
		slog.String("permission", permissionID))

	if s.auditLogger != nil {
		var orgID *string
		if tc, ok := TenantFromContext(r.Context()); ok && tc != nil && tc.Organization != nil {
			orgID = &tc.Organization.ID
		}
		go s.auditLogger.LogPermissionCheck(context.WithoutCancel(r.Context()), PermissionAuditEntry{
			UserID:     userID,
			Permission: permissionID,
			Decision:   "denied",
			Resource:   r.URL.Path,
			Method:     r.Method,
			OrgID:      orgID,
		})
	}
}

// hasJWTRole checks if the user has a specific role in their JWT claims
func hasJWTRole(user *auth.User, role string) bool {
	return slices.Contains(user.Roles, role)
}

// Common permission constants for convenience
const (
	// Lab permissions
	PermLabsRead   = "labs.read"
	PermLabsCreate = "labs.create"
	PermLabsUpdate = "labs.update"
	PermLabsDelete = "labs.delete"

	// Pod permissions
	PermPodsRead   = "pods.read"
	PermPodsCreate = "pods.create"
	PermPodsStart  = "pods.start"
	PermPodsStop   = "pods.stop"
	PermPodsDelete = "pods.delete"

	// Session permissions
	PermSessionsRead   = "sessions.read"
	PermSessionsCreate = "sessions.create"
	PermSessionsUpdate = "sessions.update"
	PermSessionsEnd    = "sessions.end"

	// Team permissions
	PermTeamsRead   = "teams.read"
	PermTeamsCreate = "teams.create"
	PermTeamsUpdate = "teams.update"
	PermTeamsDelete = "teams.delete"
	PermTeamsManage = "teams.manage_members"

	// Organization permissions
	PermOrgRead     = "org.read"
	PermOrgUpdate   = "org.update"
	PermOrgManage   = "org.manage_members"
	PermOrgRoles    = "org.manage_roles"
	PermOrgFeatures = "org.manage_features"
	PermOrgBilling  = "org.billing"

	// Pathway permissions
	PermPathwaysRead   = "pathways.read"
	PermPathwaysCreate = "pathways.create"
	PermPathwaysUpdate = "pathways.update"
	PermPathwaysDelete = "pathways.delete"

	// RBAC permissions
	PermRBACRolesRead   = "rbac.roles.read"
	PermRBACRolesCreate = "rbac.roles.create"
	PermRBACRolesUpdate = "rbac.roles.update"
	PermRBACRolesDelete = "rbac.roles.delete"
	PermRBACAssign      = "rbac.assign"

	// Analytics permissions
	PermAnalyticsRead   = "analytics.read"
	PermAnalyticsExport = "analytics.export"

	// Audit permissions
	PermAuditRead   = "audit.read"
	PermAuditExport = "audit.export"

	// Admin permissions
	PermAdminUsers    = "admin.users"
	PermAdminSystem   = "admin.system"
	PermAdminLicenses = "admin.licenses"
)
