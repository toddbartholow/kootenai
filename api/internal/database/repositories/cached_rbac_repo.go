package repositories

import (
	"context"
	"log/slog"

	"github.com/toddbartholow/kootenai/api/internal/models"
	redisclient "github.com/toddbartholow/kootenai/api/internal/redis"
)

// Compile-time interface assertion
var _ RBACRepository = (*CachedRBACRepo)(nil)

// CachedRBACRepo wraps RBACRepository with Redis caching for permission lookups
type CachedRBACRepo struct {
	repo   RBACRepository
	cache  *redisclient.PermissionCache
	logger *slog.Logger
}

// NewCachedRBACRepo creates a new cached RBAC repository
func NewCachedRBACRepo(repo RBACRepository, cache *redisclient.PermissionCache, logger *slog.Logger) *CachedRBACRepo {
	if logger == nil {
		logger = slog.Default()
	}
	return &CachedRBACRepo{
		repo:   repo,
		cache:  cache,
		logger: logger,
	}
}

// --- Permission operations (pass-through) ---

func (r *CachedRBACRepo) GetPermission(ctx context.Context, id string) (*models.Permission, error) {
	return r.repo.GetPermission(ctx, id)
}

func (r *CachedRBACRepo) ListPermissions(ctx context.Context, resource string) ([]*models.Permission, error) {
	return r.repo.ListPermissions(ctx, resource)
}

// --- Role operations (pass-through, invalidate on mutation) ---

func (r *CachedRBACRepo) CreateRole(ctx context.Context, role *models.Role) error {
	return r.repo.CreateRole(ctx, role)
}

func (r *CachedRBACRepo) GetRole(ctx context.Context, id string) (*models.Role, error) {
	return r.repo.GetRole(ctx, id)
}

func (r *CachedRBACRepo) GetRoleBySlug(ctx context.Context, slug string) (*models.Role, error) {
	return r.repo.GetRoleBySlug(ctx, slug)
}

func (r *CachedRBACRepo) GetRoleWithPermissions(ctx context.Context, id string) (*models.Role, error) {
	return r.repo.GetRoleWithPermissions(ctx, id)
}

func (r *CachedRBACRepo) ListRoles(ctx context.Context, orgID *string, includeSystem bool) ([]*models.Role, error) {
	return r.repo.ListRoles(ctx, orgID, includeSystem)
}

func (r *CachedRBACRepo) UpdateRole(ctx context.Context, role *models.Role) error {
	if err := r.repo.UpdateRole(ctx, role); err != nil {
		return err
	}
	// Role change affects all users with this role
	r.invalidateAll(ctx)
	return nil
}

func (r *CachedRBACRepo) DeleteRole(ctx context.Context, id string) error {
	if err := r.repo.DeleteRole(ctx, id); err != nil {
		return err
	}
	r.invalidateAll(ctx)
	return nil
}

func (r *CachedRBACRepo) SetRolePermissionSets(ctx context.Context, roleID string, permissionSetIDs []string) error {
	if err := r.repo.SetRolePermissionSets(ctx, roleID, permissionSetIDs); err != nil {
		return err
	}
	r.invalidateAll(ctx)
	return nil
}

func (r *CachedRBACRepo) SetRolePermissions(ctx context.Context, roleID string, permissions []models.RolePermission) error {
	if err := r.repo.SetRolePermissions(ctx, roleID, permissions); err != nil {
		return err
	}
	r.invalidateAll(ctx)
	return nil
}

func (r *CachedRBACRepo) GetRolePermissions(ctx context.Context, roleID string) ([]string, error) {
	return r.repo.GetRolePermissions(ctx, roleID)
}

// --- User role operations (invalidate affected user) ---

func (r *CachedRBACRepo) AssignRole(ctx context.Context, userRole *models.UserRole) error {
	if err := r.repo.AssignRole(ctx, userRole); err != nil {
		return err
	}
	r.invalidateUser(ctx, userRole.UserID)
	return nil
}

func (r *CachedRBACRepo) RevokeRole(ctx context.Context, userID, roleID string, orgID *string) error {
	if err := r.repo.RevokeRole(ctx, userID, roleID, orgID); err != nil {
		return err
	}
	r.invalidateUser(ctx, userID)
	return nil
}

func (r *CachedRBACRepo) GetUserRoles(ctx context.Context, userID string, orgID *string) ([]*models.UserRole, error) {
	return r.repo.GetUserRoles(ctx, userID, orgID)
}

func (r *CachedRBACRepo) BulkAssignRole(ctx context.Context, userIDs []string, roleID string, orgID *string, grantedBy *string) error {
	if err := r.repo.BulkAssignRole(ctx, userIDs, roleID, orgID, grantedBy); err != nil {
		return err
	}
	for _, uid := range userIDs {
		r.invalidateUser(ctx, uid)
	}
	return nil
}

func (r *CachedRBACRepo) BulkRevokeRole(ctx context.Context, userIDs []string, roleID string, orgID *string) error {
	if err := r.repo.BulkRevokeRole(ctx, userIDs, roleID, orgID); err != nil {
		return err
	}
	for _, uid := range userIDs {
		r.invalidateUser(ctx, uid)
	}
	return nil
}

// --- Permission checking (cached) ---

func (r *CachedRBACRepo) HasPermission(ctx context.Context, userID, permissionID string, orgID *string) (bool, error) {
	perms, err := r.GetUserPermissions(ctx, userID, orgID)
	if err != nil {
		return r.repo.HasPermission(ctx, userID, permissionID, orgID)
	}
	for _, p := range perms {
		if p == permissionID {
			return true, nil
		}
	}
	return false, nil
}

func (r *CachedRBACRepo) GetUserPermissions(ctx context.Context, userID string, orgID *string) ([]string, error) {
	if r.cache != nil {
		cached, err := r.cache.GetUserPermissions(ctx, userID, orgID)
		if err == nil {
			r.logger.Debug("permission cache hit", "user_id", userID)
			return cached, nil
		}
	}

	perms, err := r.repo.GetUserPermissions(ctx, userID, orgID)
	if err != nil {
		return nil, err
	}

	if r.cache != nil {
		if err := r.cache.SetUserPermissions(ctx, userID, orgID, perms); err != nil {
			r.logger.Warn("failed to cache permissions", "error", err)
		}
	}

	return perms, nil
}

func (r *CachedRBACRepo) CheckPermissions(ctx context.Context, userID string, permissionIDs []string, orgID *string) (map[string]bool, error) {
	perms, err := r.GetUserPermissions(ctx, userID, orgID)
	if err != nil {
		return r.repo.CheckPermissions(ctx, userID, permissionIDs, orgID)
	}

	permSet := make(map[string]bool, len(perms))
	for _, p := range perms {
		permSet[p] = true
	}

	result := make(map[string]bool, len(permissionIDs))
	for _, pid := range permissionIDs {
		result[pid] = permSet[pid]
	}
	return result, nil
}

func (r *CachedRBACRepo) GetUsersWithPermission(ctx context.Context, permissionID string, orgID *string) ([]string, error) {
	return r.repo.GetUsersWithPermission(ctx, permissionID, orgID)
}

// --- Maintenance ---

func (r *CachedRBACRepo) CleanupExpiredRoles(ctx context.Context) (int64, error) {
	rows, err := r.repo.CleanupExpiredRoles(ctx)
	if err != nil {
		return 0, err
	}
	if rows > 0 {
		r.invalidateAll(ctx)
	}
	return rows, nil
}

// --- Cache helpers ---

func (r *CachedRBACRepo) invalidateUser(ctx context.Context, userID string) {
	if r.cache != nil {
		if err := r.cache.InvalidateUser(ctx, userID); err != nil {
			r.logger.Warn("failed to invalidate user permission cache", "error", err, "user_id", userID)
		}
	}
}

func (r *CachedRBACRepo) invalidateAll(ctx context.Context) {
	if r.cache != nil {
		if err := r.cache.InvalidateAll(ctx); err != nil {
			r.logger.Warn("failed to invalidate all permission caches", "error", err)
		}
	}
}
