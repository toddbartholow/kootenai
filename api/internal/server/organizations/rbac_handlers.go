package organizations

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/toddbartholow/kootenai/api/internal/auth"
	"github.com/toddbartholow/kootenai/api/internal/database/repositories"
	"github.com/toddbartholow/kootenai/api/internal/middleware"
	"github.com/toddbartholow/kootenai/api/internal/models"
)

// blockedPermissionPrefixes defines permission prefixes that cannot be assigned to custom roles.
var blockedPermissionPrefixes = []string{"admin.", "rbac."}

// blockedPermissionError is returned by validateCustomRolePermissions when a
// permission ID matches a blocked prefix. Implements httputil.LocalizableError
// so handlers can render it via Responder.LocalizedErrorResponseFromErr.
type blockedPermissionError struct {
	Permission string
}

func (e *blockedPermissionError) Error() string {
	return fmt.Sprintf("permission %q cannot be assigned to custom roles", e.Permission)
}

func (e *blockedPermissionError) MessageID() string { return "rbac.errors.permissionBlocked" }

func (e *blockedPermissionError) TemplateData() map[string]any {
	return map[string]any{"Permission": e.Permission}
}

// validateCustomRolePermissions checks that no blocked permissions are included in custom roles.
func validateCustomRolePermissions(permIDs []string) error {
	for _, pid := range permIDs {
		for _, prefix := range blockedPermissionPrefixes {
			if strings.HasPrefix(pid, prefix) {
				return &blockedPermissionError{Permission: pid}
			}
		}
	}
	return nil
}

// verifyRoleOrgAccess checks that a role is either a system role or belongs to the given org.
func verifyRoleOrgAccess(role *models.Role, orgID string) bool {
	return role.OrganizationID == nil || *role.OrganizationID == orgID
}

// checkCustomRoleLimit verifies that the org hasn't exceeded its custom role
// limit for the edition. Returns (ok, messageID); messageID is a catalog key
// when ok is false, "" otherwise.
func checkCustomRoleLimit(rbacRepo repositories.RBACRepository, ctx context.Context, orgID string, edition models.Edition) (bool, string) {
	existingRoles, err := rbacRepo.ListRoles(ctx, &orgID, false)
	if err != nil {
		return false, "rbac.errors.checkRoleLimitsFailed"
	}
	customCount := 0
	for _, role := range existingRoles {
		if !role.IsSystem {
			customCount++
		}
	}
	maxCustomRoles := customRoleLimit(edition)
	if maxCustomRoles >= 0 && customCount >= maxCustomRoles {
		return false, "rbac.errors.customRoleLimitReached"
	}
	return true, ""
}

// -----------------------------------------------------------------------------
// RBAC Role & Permission Handlers
// -----------------------------------------------------------------------------

// handleListRoles lists roles available in an organization (system + custom)
func (m *Manager) handleListRoles() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		rbacRepo := m.rbacRepo
		if rbacRepo == nil {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusServiceUnavailable, "rbac.errors.notConfigured", nil)
			return
		}

		tc, ok := middleware.TenantFromContext(r.Context())
		if !ok || tc.Organization == nil {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusBadRequest, "rbac.errors.contextRequired", nil)
			return
		}

		orgID := tc.Organization.ID
		roles, err := rbacRepo.ListRoles(r.Context(), &orgID, true)
		if err != nil {
			m.logger.Error("Failed to list roles", slog.Any("error", err))
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusInternalServerError, "rbac.errors.listRolesFailed", nil)
			return
		}

		m.responder.JSONResponse(w, http.StatusOK, map[string]any{
			"roles": roles,
			"count": len(roles),
		})
	}
}

// handleGetRole gets a single role with its permissions
func (m *Manager) handleGetRole() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		rbacRepo := m.rbacRepo
		if rbacRepo == nil {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusServiceUnavailable, "rbac.errors.notConfigured", nil)
			return
		}

		tc, ok := middleware.TenantFromContext(r.Context())
		if !ok || tc.Organization == nil {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusBadRequest, "rbac.errors.contextRequired", nil)
			return
		}

		roleID := chi.URLParam(r, "roleID")
		role, err := rbacRepo.GetRoleWithPermissions(r.Context(), roleID)
		if err != nil {
			m.logger.Error("Failed to get role", slog.Any("error", err))
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusInternalServerError, "rbac.errors.getFailed", nil)
			return
		}
		if role == nil {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusNotFound, "rbac.errors.roleNotFound", nil)
			return
		}

		if !verifyRoleOrgAccess(role, tc.Organization.ID) {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusNotFound, "rbac.errors.roleNotFound", nil)
			return
		}

		m.responder.JSONResponse(w, http.StatusOK, role)
	}
}

// handleListUserRoles lists roles assigned to a user within an organization
func (m *Manager) handleListUserRoles() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		rbacRepo := m.rbacRepo
		if rbacRepo == nil {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusServiceUnavailable, "rbac.errors.notConfigured", nil)
			return
		}

		tc, ok := middleware.TenantFromContext(r.Context())
		if !ok || tc.Organization == nil {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusBadRequest, "rbac.errors.contextRequired", nil)
			return
		}

		userID := chi.URLParam(r, "userID")
		orgID := tc.Organization.ID

		roles, err := rbacRepo.GetUserRoles(r.Context(), userID, &orgID)
		if err != nil {
			m.logger.Error("Failed to get user roles", slog.Any("error", err))
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusInternalServerError, "rbac.errors.getUserRolesFailed", nil)
			return
		}

		m.responder.JSONResponse(w, http.StatusOK, map[string]any{
			"roles": roles,
			"count": len(roles),
		})
	}
}

// AssignRoleRequest represents a request to assign a role to a user
type AssignRoleRequest struct {
	RoleID    string     `json:"roleId"`
	ExpiresAt *time.Time `json:"expiresAt,omitempty"`
}

// handleAssignRole assigns a role to a user within an organization
func (m *Manager) handleAssignRole() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		rbacRepo := m.rbacRepo
		if rbacRepo == nil {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusServiceUnavailable, "rbac.errors.notConfigured", nil)
			return
		}

		tc, ok := middleware.TenantFromContext(r.Context())
		if !ok || tc.Organization == nil {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusBadRequest, "rbac.errors.contextRequired", nil)
			return
		}

		currentUser, ok := auth.UserFromContext(r.Context())
		if !ok {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusUnauthorized, "authz.authRequired", nil)
			return
		}

		userID := chi.URLParam(r, "userID")
		orgID := tc.Organization.ID

		var req AssignRoleRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusBadRequest, "error.invalidRequestBody", nil)
			return
		}

		if req.RoleID == "" {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusBadRequest, "rbac.errors.roleIdRequired", nil)
			return
		}

		// Verify role exists
		role, err := rbacRepo.GetRole(r.Context(), req.RoleID)
		if err != nil || role == nil {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusNotFound, "rbac.errors.roleNotFound", nil)
			return
		}

		// Check role priority: assigner cannot assign roles >= their own priority
		assignerRoles, err := rbacRepo.GetUserRoles(r.Context(), currentUser.ID, &orgID)
		if err != nil {
			m.logger.Error("Failed to get assigner roles", slog.Any("error", err))
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusInternalServerError, "rbac.errors.verifyPermissionsFailed", nil)
			return
		}
		maxPriority := 0
		for _, ur := range assignerRoles {
			if ur.Role != nil && ur.Role.Priority > maxPriority {
				maxPriority = ur.Role.Priority
			}
		}
		if role.Priority >= maxPriority && !slices.Contains(currentUser.Roles, "admin") {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusForbidden, "rbac.errors.cannotAssignHigherRole", nil)
			return
		}
		// Block self-elevation
		if userID == currentUser.ID && role.Priority > maxPriority {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusForbidden, "rbac.errors.cannotElevateOwnRole", nil)
			return
		}

		userRole := &models.UserRole{
			ID:             uuid.New().String(),
			UserID:         userID,
			RoleID:         req.RoleID,
			OrganizationID: &orgID,
			GrantedBy:      &currentUser.ID,
			GrantedAt:      time.Now(),
			ExpiresAt:      req.ExpiresAt,
		}

		if err := rbacRepo.AssignRole(r.Context(), userRole); err != nil {
			m.logger.Error("Failed to assign role", slog.Any("error", err))
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusInternalServerError, "rbac.errors.assignFailed", nil)
			return
		}

		// Audit log
		m.logAdminAction(r, "role.assigned", "user_role", userRole.ID, map[string]any{
			"userId":         userID,
			"roleId":         req.RoleID,
			"roleName":       role.Name,
			"organizationId": orgID,
		})

		m.responder.JSONResponse(w, http.StatusCreated, map[string]any{
			"id":      userRole.ID,
			"userId":  userID,
			"roleId":  req.RoleID,
			"message": "role assigned",
		})
	}
}

// handleRevokeRole revokes a role from a user within an organization
func (m *Manager) handleRevokeRole() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		rbacRepo := m.rbacRepo
		if rbacRepo == nil {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusServiceUnavailable, "rbac.errors.notConfigured", nil)
			return
		}

		tc, ok := middleware.TenantFromContext(r.Context())
		if !ok || tc.Organization == nil {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusBadRequest, "rbac.errors.contextRequired", nil)
			return
		}

		userID := chi.URLParam(r, "userID")
		roleID := chi.URLParam(r, "roleID")
		orgID := tc.Organization.ID

		if err := rbacRepo.RevokeRole(r.Context(), userID, roleID, &orgID); err != nil {
			m.logger.Error("Failed to revoke role", slog.Any("error", err))
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusInternalServerError, "rbac.errors.revokeFailed", nil)
			return
		}

		// Audit log
		m.logAdminAction(r, "role.revoked", "user_role", roleID, map[string]any{
			"userId":         userID,
			"roleId":         roleID,
			"organizationId": orgID,
		})

		m.responder.JSONResponse(w, http.StatusOK, map[string]any{
			"message": "role revoked",
		})
	}
}

// handleListPermissions lists all available permissions
func (m *Manager) handleListPermissions() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		rbacRepo := m.rbacRepo
		if rbacRepo == nil {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusServiceUnavailable, "rbac.errors.notConfigured", nil)
			return
		}

		resource := r.URL.Query().Get("resource")
		permissions, err := rbacRepo.ListPermissions(r.Context(), resource)
		if err != nil {
			m.logger.Error("Failed to list permissions", slog.Any("error", err))
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusInternalServerError, "rbac.errors.listPermissionsFailed", nil)
			return
		}

		m.responder.JSONResponse(w, http.StatusOK, map[string]any{
			"permissions": permissions,
			"count":       len(permissions),
		})
	}
}

// -----------------------------------------------------------------------------
// Custom Role Management (Enterprise)
// -----------------------------------------------------------------------------

// CreateCustomRoleRequest represents a request to create a custom role
type CreateCustomRoleRequest struct {
	Name             string   `json:"name"`
	Slug             string   `json:"slug"`
	Description      string   `json:"description,omitempty"`
	PermissionIDs    []string `json:"permissionIds,omitempty"`
	PermissionSetIDs []string `json:"permissionSetIds,omitempty"`
}

// UpdateCustomRoleRequest represents a request to update a custom role
type UpdateCustomRoleRequest struct {
	Name             *string  `json:"name,omitempty"`
	Description      *string  `json:"description,omitempty"`
	PermissionIDs    []string `json:"permissionIds,omitempty"`
	PermissionSetIDs []string `json:"permissionSetIds,omitempty"`
}

// handleCreateCustomRole creates a custom role for an organization (Enterprise only)
func (m *Manager) handleCreateCustomRole() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		rbacRepo := m.rbacRepo
		if rbacRepo == nil {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusServiceUnavailable, "rbac.errors.notConfigured", nil)
			return
		}

		tc, ok := middleware.TenantFromContext(r.Context())
		if !ok || tc.Organization == nil {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusBadRequest, "rbac.errors.contextRequired", nil)
			return
		}

		var req CreateCustomRoleRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusBadRequest, "error.invalidRequestBody", nil)
			return
		}

		if req.Name == "" || req.Slug == "" {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusBadRequest, "rbac.errors.nameAndSlugRequired", nil)
			return
		}

		orgID := tc.Organization.ID

		// Validate permissions aren't blocked for custom roles
		if err := validateCustomRolePermissions(req.PermissionIDs); err != nil {
			m.responder.LocalizedErrorResponseFromErr(r.Context(), w, http.StatusBadRequest, err)
			return
		}

		// Check custom role limits based on edition
		if ok, msg := checkCustomRoleLimit(rbacRepo, r.Context(), orgID, tc.Edition); !ok {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusForbidden, msg, nil)
			return
		}

		role := &models.Role{
			ID:             uuid.New().String(),
			Name:           req.Name,
			Slug:           req.Slug,
			Description:    req.Description,
			OrganizationID: &orgID,
			IsSystem:       false,
			IsDefault:      false,
			Priority:       40, // Default priority for custom roles
		}

		if err := rbacRepo.CreateRole(r.Context(), role); err != nil {
			m.logger.Error("Failed to create custom role", slog.Any("error", err))
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusInternalServerError, "rbac.errors.createFailed", nil)
			return
		}

		// Set permissions if provided
		if len(req.PermissionSetIDs) > 0 {
			if err := rbacRepo.SetRolePermissionSets(r.Context(), role.ID, req.PermissionSetIDs); err != nil {
				m.logger.Error("Failed to set role permission sets", slog.Any("error", err))
				if delErr := rbacRepo.DeleteRole(r.Context(), role.ID); delErr != nil {
					m.logger.Error("Failed to clean up role after permission set error", slog.Any("error", delErr), slog.String("roleID", role.ID))
				}
				m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusInternalServerError, "rbac.errors.setRolePermissionSetsFailed", nil)
				return
			}
		}
		if len(req.PermissionIDs) > 0 {
			rps := make([]models.RolePermission, len(req.PermissionIDs))
			for i, pid := range req.PermissionIDs {
				rps[i] = models.RolePermission{RoleID: role.ID, PermissionID: pid, Granted: true}
			}
			if err := rbacRepo.SetRolePermissions(r.Context(), role.ID, rps); err != nil {
				m.logger.Error("Failed to set role permissions", slog.Any("error", err))
				if delErr := rbacRepo.DeleteRole(r.Context(), role.ID); delErr != nil {
					m.logger.Error("Failed to clean up role after permission error", slog.Any("error", delErr), slog.String("roleID", role.ID))
				}
				m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusInternalServerError, "rbac.errors.setRolePermissionsFailed", nil)
				return
			}
		}

		m.logAdminAction(r, "role.created", "role", role.ID, map[string]any{
			"name":           role.Name,
			"organizationId": orgID,
		})

		m.responder.JSONResponse(w, http.StatusCreated, role)
	}
}

// handleUpdateCustomRole updates a custom role
func (m *Manager) handleUpdateCustomRole() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		rbacRepo := m.rbacRepo
		if rbacRepo == nil {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusServiceUnavailable, "rbac.errors.notConfigured", nil)
			return
		}

		tc, ok := middleware.TenantFromContext(r.Context())
		if !ok || tc.Organization == nil {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusBadRequest, "rbac.errors.contextRequired", nil)
			return
		}

		roleID := chi.URLParam(r, "roleID")

		// Get existing role
		role, err := rbacRepo.GetRole(r.Context(), roleID)
		if err != nil || role == nil {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusNotFound, "rbac.errors.roleNotFound", nil)
			return
		}
		if !verifyRoleOrgAccess(role, tc.Organization.ID) {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusNotFound, "rbac.errors.roleNotFound", nil)
			return
		}
		if role.IsSystem {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusForbidden, "rbac.errors.systemRolesCannotBeModified", nil)
			return
		}

		var req UpdateCustomRoleRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusBadRequest, "error.invalidRequestBody", nil)
			return
		}

		// Validate permissions aren't blocked for custom roles
		if req.PermissionIDs != nil {
			if err := validateCustomRolePermissions(req.PermissionIDs); err != nil {
				m.responder.LocalizedErrorResponseFromErr(r.Context(), w, http.StatusBadRequest, err)
				return
			}
		}

		if req.Name != nil {
			role.Name = *req.Name
		}
		if req.Description != nil {
			role.Description = *req.Description
		}

		if err := rbacRepo.UpdateRole(r.Context(), role); err != nil {
			m.logger.Error("Failed to update role", slog.Any("error", err))
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusInternalServerError, "rbac.errors.updateFailed", nil)
			return
		}

		// Update permissions if provided
		if req.PermissionSetIDs != nil {
			if err := rbacRepo.SetRolePermissionSets(r.Context(), role.ID, req.PermissionSetIDs); err != nil {
				m.logger.Error("Failed to update role permission sets", slog.Any("error", err))
				m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusInternalServerError, "rbac.errors.updateRolePermissionSetsFailed", nil)
				return
			}
		}
		if req.PermissionIDs != nil {
			rps := make([]models.RolePermission, len(req.PermissionIDs))
			for i, pid := range req.PermissionIDs {
				rps[i] = models.RolePermission{RoleID: role.ID, PermissionID: pid, Granted: true}
			}
			if err := rbacRepo.SetRolePermissions(r.Context(), role.ID, rps); err != nil {
				m.logger.Error("Failed to update role permissions", slog.Any("error", err))
				m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusInternalServerError, "rbac.errors.updateRolePermissionsFailed", nil)
				return
			}
		}

		m.logAdminAction(r, "role.updated", "role", role.ID, map[string]any{
			"name":           role.Name,
			"organizationId": tc.Organization.ID,
		})

		m.responder.JSONResponse(w, http.StatusOK, role)
	}
}

// handleDeleteCustomRole deletes a custom role
func (m *Manager) handleDeleteCustomRole() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		rbacRepo := m.rbacRepo
		if rbacRepo == nil {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusServiceUnavailable, "rbac.errors.notConfigured", nil)
			return
		}

		tc, ok := middleware.TenantFromContext(r.Context())
		if !ok || tc.Organization == nil {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusBadRequest, "rbac.errors.contextRequired", nil)
			return
		}

		roleID := chi.URLParam(r, "roleID")

		role, err := rbacRepo.GetRole(r.Context(), roleID)
		if err != nil || role == nil {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusNotFound, "rbac.errors.roleNotFound", nil)
			return
		}
		if !verifyRoleOrgAccess(role, tc.Organization.ID) {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusNotFound, "rbac.errors.roleNotFound", nil)
			return
		}
		if role.IsSystem {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusForbidden, "rbac.errors.systemRolesCannotBeDeleted", nil)
			return
		}

		if err := rbacRepo.DeleteRole(r.Context(), roleID); err != nil {
			m.logger.Error("Failed to delete role", slog.Any("error", err))
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusInternalServerError, "rbac.errors.deleteFailed", nil)
			return
		}

		m.logAdminAction(r, "role.deleted", "role", roleID, map[string]any{
			"name": role.Name,
		})

		m.responder.JSONResponse(w, http.StatusOK, map[string]any{"message": "role deleted"})
	}
}

// handleCloneRole clones an existing role (Enterprise only)
func (m *Manager) handleCloneRole() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		rbacRepo := m.rbacRepo
		if rbacRepo == nil {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusServiceUnavailable, "rbac.errors.notConfigured", nil)
			return
		}

		tc, ok := middleware.TenantFromContext(r.Context())
		if !ok || tc.Organization == nil {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusBadRequest, "rbac.errors.contextRequired", nil)
			return
		}

		roleID := chi.URLParam(r, "roleID")

		// Get source role with permissions
		sourceRole, err := rbacRepo.GetRoleWithPermissions(r.Context(), roleID)
		if err != nil || sourceRole == nil {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusNotFound, "rbac.errors.sourceRoleNotFound", nil)
			return
		}

		orgID := tc.Organization.ID

		if !verifyRoleOrgAccess(sourceRole, orgID) {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusNotFound, "rbac.errors.sourceRoleNotFound", nil)
			return
		}

		// Check custom role limits
		if ok, msg := checkCustomRoleLimit(rbacRepo, r.Context(), orgID, tc.Edition); !ok {
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusForbidden, msg, nil)
			return
		}

		newRole := &models.Role{
			ID:             uuid.New().String(),
			Name:           sourceRole.Name + " (Copy)",
			Slug:           sourceRole.Slug + "-copy-" + uuid.New().String()[:8],
			Description:    sourceRole.Description,
			OrganizationID: &orgID,
			IsSystem:       false,
			IsDefault:      false,
			Priority:       sourceRole.Priority,
		}

		if err := rbacRepo.CreateRole(r.Context(), newRole); err != nil {
			m.logger.Error("Failed to clone role", slog.Any("error", err))
			m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusInternalServerError, "rbac.errors.cloneFailed", nil)
			return
		}

		// Copy permission sets
		if len(sourceRole.PermissionSets) > 0 {
			psIDs := make([]string, len(sourceRole.PermissionSets))
			for i, ps := range sourceRole.PermissionSets {
				psIDs[i] = ps.ID
			}
			if err := rbacRepo.SetRolePermissionSets(r.Context(), newRole.ID, psIDs); err != nil {
				m.logger.Error("Failed to copy permission sets", slog.Any("error", err))
				if delErr := rbacRepo.DeleteRole(r.Context(), newRole.ID); delErr != nil {
					m.logger.Error("Failed to clean up cloned role after permission set error", slog.Any("error", delErr), slog.String("roleID", newRole.ID))
				}
				m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusInternalServerError, "rbac.errors.copyPermissionSetsFailed", nil)
				return
			}
		}

		// Copy direct permissions
		if len(sourceRole.Permissions) > 0 {
			rps := make([]models.RolePermission, len(sourceRole.Permissions))
			for i, p := range sourceRole.Permissions {
				rps[i] = models.RolePermission{RoleID: newRole.ID, PermissionID: p.PermissionID, Granted: p.Granted}
			}
			if err := rbacRepo.SetRolePermissions(r.Context(), newRole.ID, rps); err != nil {
				m.logger.Error("Failed to copy permissions", slog.Any("error", err))
				if delErr := rbacRepo.DeleteRole(r.Context(), newRole.ID); delErr != nil {
					m.logger.Error("Failed to clean up cloned role after permission error", slog.Any("error", delErr), slog.String("roleID", newRole.ID))
				}
				m.responder.LocalizedErrorResponse(r.Context(), w, http.StatusInternalServerError, "rbac.errors.copyPermissionsFailed", nil)
				return
			}
		}

		m.logAdminAction(r, "role.cloned", "role", newRole.ID, map[string]any{
			"name":           newRole.Name,
			"sourceRoleId":   sourceRole.ID,
			"organizationId": orgID,
		})

		m.responder.JSONResponse(w, http.StatusCreated, newRole)
	}
}

// customRoleLimit returns the maximum number of custom roles allowed for an edition.
// Returns -1 for unlimited.
func customRoleLimit(edition models.Edition) int {
	switch edition {
	case models.EditionEnterprise:
		return -1 // unlimited
	case models.EditionProfessional:
		return 3
	default:
		return 0
	}
}

// assignDefaultRole assigns the default "student" role to a user in an organization.
// This is called when a user joins an organization.
func assignDefaultRole(r *http.Request, rbacRepo repositories.RBACRepository, userID string, orgID string, grantedBy *string) {
	if rbacRepo == nil {
		return
	}

	ctx := r.Context()

	// Find the default student role
	studentRole, err := rbacRepo.GetRoleBySlug(ctx, "student")
	if err != nil || studentRole == nil {
		return
	}

	userRole := &models.UserRole{
		ID:             uuid.New().String(),
		UserID:         userID,
		RoleID:         studentRole.ID,
		OrganizationID: &orgID,
		GrantedBy:      grantedBy,
		GrantedAt:      time.Now(),
	}

	if err := rbacRepo.AssignRole(ctx, userRole); err != nil {
		slog.Warn("Failed to assign default role", "error", err, "user_id", userID, "org_id", orgID)
	}
}
