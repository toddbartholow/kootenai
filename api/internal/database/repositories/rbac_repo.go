package repositories

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/lib/pq"

	"github.com/toddbartholow/kootenai/api/internal/models"
)

// RBACRepo implements RBAC repository operations
type RBACRepo struct {
	db DBTX
}

// NewRBACRepo creates a new RBAC repository
func NewRBACRepo(db DBTX) *RBACRepo {
	return &RBACRepo{db: db}
}

// -----------------------------------------------------------------------------
// Permission Operations
// -----------------------------------------------------------------------------

// GetPermission retrieves a permission by ID
func (r *RBACRepo) GetPermission(ctx context.Context, id string) (*models.Permission, error) {
	query := `
		SELECT id, name, description, resource, action, is_system, created_at
		FROM permissions
		WHERE id = $1`

	var p models.Permission
	var desc sql.NullString

	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&p.ID, &p.Name, &desc, &p.Resource, &p.Action, &p.IsSystem, &p.CreatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("getting permission: %w", err)
	}

	p.Description = desc.String
	return &p, nil
}

// ListPermissions lists all permissions with optional filtering
func (r *RBACRepo) ListPermissions(ctx context.Context, resource string) ([]*models.Permission, error) {
	qb := NewQueryBuilder(`
		SELECT id, name, description, resource, action, is_system, created_at
		FROM permissions
		WHERE 1=1`)
	if resource != "" {
		qb.AddCondition("resource = $%d", resource)
	}
	qb.OrderByRaw("resource, action")

	query, args := qb.Build()

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("listing permissions: %w", err)
	}
	defer rows.Close()

	var permissions []*models.Permission
	for rows.Next() {
		var p models.Permission
		var desc sql.NullString
		if err := rows.Scan(&p.ID, &p.Name, &desc, &p.Resource, &p.Action, &p.IsSystem, &p.CreatedAt); err != nil {
			return nil, fmt.Errorf("scanning permission: %w", err)
		}
		p.Description = desc.String
		permissions = append(permissions, &p)
	}

	return permissions, rows.Err()
}

// -----------------------------------------------------------------------------
// Role Operations
// -----------------------------------------------------------------------------

// CreateRole creates a new role
func (r *RBACRepo) CreateRole(ctx context.Context, role *models.Role) error {
	if role.ID == "" {
		role.ID = uuid.New().String()
	}
	if role.Slug == "" {
		role.Slug = strings.ToLower(strings.ReplaceAll(role.Name, " ", "-"))
	}

	query := `
		INSERT INTO roles (id, name, slug, description, organization_id, is_system, is_default, priority)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING created_at, updated_at`

	err := r.db.QueryRowContext(ctx, query,
		role.ID,
		role.Name,
		role.Slug,
		nullString(role.Description),
		nullStringPtr(role.OrganizationID),
		role.IsSystem,
		role.IsDefault,
		role.Priority,
	).Scan(&role.CreatedAt, &role.UpdatedAt)

	if err != nil {
		return fmt.Errorf("creating role: %w", err)
	}

	return nil
}

// GetRole retrieves a role by ID
func (r *RBACRepo) GetRole(ctx context.Context, id string) (*models.Role, error) {
	query := `
		SELECT id, name, slug, description, organization_id, is_system, is_default, priority, created_at, updated_at
		FROM roles
		WHERE id = $1`

	role, err := r.scanRole(r.db.QueryRowContext(ctx, query, id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("getting role: %w", err)
	}

	return role, nil
}

// GetRoleBySlug retrieves a role by slug
func (r *RBACRepo) GetRoleBySlug(ctx context.Context, slug string) (*models.Role, error) {
	query := `
		SELECT id, name, slug, description, organization_id, is_system, is_default, priority, created_at, updated_at
		FROM roles
		WHERE slug = $1`

	role, err := r.scanRole(r.db.QueryRowContext(ctx, query, slug))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("getting role by slug: %w", err)
	}

	return role, nil
}

// ListRoles lists roles with optional organization filter
func (r *RBACRepo) ListRoles(ctx context.Context, orgID *string, includeSystem bool) ([]*models.Role, error) {
	qb := NewQueryBuilder(`
		SELECT id, name, slug, description, organization_id, is_system, is_default, priority, created_at, updated_at
		FROM roles
		WHERE 1=1`)

	if orgID != nil {
		qb.AddCondition("(organization_id = $%d OR organization_id IS NULL)", *orgID)
	}
	if !includeSystem {
		qb.AddRawCondition("is_system = false")
	}

	qb.OrderByRaw("priority DESC, name")

	query, args := qb.Build()

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("listing roles: %w", err)
	}
	defer rows.Close()

	var roles []*models.Role
	for rows.Next() {
		role, err := r.scanRoleRow(rows)
		if err != nil {
			return nil, err
		}
		roles = append(roles, role)
	}

	return roles, rows.Err()
}

// UpdateRole updates an existing role
func (r *RBACRepo) UpdateRole(ctx context.Context, role *models.Role) error {
	query := `
		UPDATE roles
		SET name = $2, description = $3, is_default = $4, priority = $5, updated_at = NOW()
		WHERE id = $1 AND is_system = false
		RETURNING updated_at`

	err := r.db.QueryRowContext(ctx, query,
		role.ID,
		role.Name,
		nullString(role.Description),
		role.IsDefault,
		role.Priority,
	).Scan(&role.UpdatedAt)

	if errors.Is(err, sql.ErrNoRows) {
		return ErrRoleNotFoundOrSystem
	}
	if err != nil {
		return fmt.Errorf("updating role: %w", err)
	}

	return nil
}

// DeleteRole deletes a non-system role
func (r *RBACRepo) DeleteRole(ctx context.Context, id string) error {
	query := `DELETE FROM roles WHERE id = $1 AND is_system = false`

	result, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("deleting role: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return ErrRoleNotFoundOrSystem
	}

	return nil
}

// SetRolePermissionSets replaces the permission sets for a role
func (r *RBACRepo) SetRolePermissionSets(ctx context.Context, roleID string, permissionSetIDs []string) error {
	tx, err := beginTx(ctx, r.db, nil)
	if err != nil {
		return fmt.Errorf("starting transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	// Delete existing
	if _, err := tx.ExecContext(ctx, "DELETE FROM role_permission_sets WHERE role_id = $1", roleID); err != nil {
		return fmt.Errorf("clearing permission sets: %w", err)
	}

	// Insert new
	if len(permissionSetIDs) > 0 {
		stmt, err := tx.PrepareContext(ctx, "INSERT INTO role_permission_sets (role_id, permission_set_id) VALUES ($1, $2)")
		if err != nil {
			return fmt.Errorf("preparing statement: %w", err)
		}
		defer stmt.Close()

		for _, psID := range permissionSetIDs {
			if _, err := stmt.ExecContext(ctx, roleID, psID); err != nil {
				return fmt.Errorf("adding permission set: %w", err)
			}
		}
	}

	return tx.Commit()
}

// SetRolePermissions replaces the direct permissions for a role
func (r *RBACRepo) SetRolePermissions(ctx context.Context, roleID string, permissions []models.RolePermission) error {
	tx, err := beginTx(ctx, r.db, nil)
	if err != nil {
		return fmt.Errorf("starting transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	// Delete existing
	if _, err := tx.ExecContext(ctx, "DELETE FROM role_permissions WHERE role_id = $1", roleID); err != nil {
		return fmt.Errorf("clearing permissions: %w", err)
	}

	// Insert new
	if len(permissions) > 0 {
		stmt, err := tx.PrepareContext(ctx, "INSERT INTO role_permissions (role_id, permission_id, granted) VALUES ($1, $2, $3)")
		if err != nil {
			return fmt.Errorf("preparing statement: %w", err)
		}
		defer stmt.Close()

		for _, p := range permissions {
			if _, err := stmt.ExecContext(ctx, roleID, p.PermissionID, p.Granted); err != nil {
				return fmt.Errorf("adding permission: %w", err)
			}
		}
	}

	return tx.Commit()
}

// -----------------------------------------------------------------------------
// User Role Operations
// -----------------------------------------------------------------------------

// AssignRole assigns a role to a user
func (r *RBACRepo) AssignRole(ctx context.Context, userRole *models.UserRole) error {
	if userRole.ID == "" {
		userRole.ID = uuid.New().String()
	}

	query := `
		INSERT INTO user_roles (id, user_id, role_id, organization_id, granted_by, expires_at)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (user_id, role_id, organization_id) DO UPDATE
		SET granted_by = EXCLUDED.granted_by, expires_at = EXCLUDED.expires_at, granted_at = NOW()
		RETURNING granted_at`

	err := r.db.QueryRowContext(ctx, query,
		userRole.ID,
		userRole.UserID,
		userRole.RoleID,
		nullStringPtr(userRole.OrganizationID),
		nullStringPtr(userRole.GrantedBy),
		nullTimePtr(userRole.ExpiresAt),
	).Scan(&userRole.GrantedAt)

	if err != nil {
		return fmt.Errorf("assigning role: %w", err)
	}

	return nil
}

// RevokeRole removes a role from a user
func (r *RBACRepo) RevokeRole(ctx context.Context, userID, roleID string, orgID *string) error {
	query := `DELETE FROM user_roles WHERE user_id = $1 AND role_id = $2`
	args := []any{userID, roleID}

	if orgID != nil {
		query += " AND organization_id = $3"
		args = append(args, *orgID)
	} else {
		query += " AND organization_id IS NULL"
	}

	result, err := r.db.ExecContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("revoking role: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return ErrRoleAssignmentNotFound
	}

	return nil
}

// GetUserRoles retrieves all roles for a user
func (r *RBACRepo) GetUserRoles(ctx context.Context, userID string, orgID *string) ([]*models.UserRole, error) {
	// `user_id` and the expiry-NULL-or-future predicate are always required,
	// so they sit in the base. The org clause is optional.
	qb := NewQueryBuilder(`
		SELECT ur.id, ur.user_id, ur.role_id, ur.organization_id, ur.granted_by, ur.granted_at, ur.expires_at,
		       r.id, r.name, r.slug, r.description, r.organization_id, r.is_system, r.is_default, r.priority, r.created_at, r.updated_at
		FROM user_roles ur
		JOIN roles r ON ur.role_id = r.id
		WHERE (ur.expires_at IS NULL OR ur.expires_at > NOW())`)

	qb.AddCondition("ur.user_id = $%d", userID)
	if orgID != nil {
		qb.AddCondition("(ur.organization_id = $%d OR ur.organization_id IS NULL)", *orgID)
	}
	qb.OrderByRaw("r.priority DESC")

	query, args := qb.Build()

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("getting user roles: %w", err)
	}
	defer rows.Close()

	var userRoles []*models.UserRole
	for rows.Next() {
		ur := &models.UserRole{Role: &models.Role{}}
		var urOrgID, grantedBy, roleOrgID, roleDesc sql.NullString
		var expiresAt sql.NullTime

		err := rows.Scan(
			&ur.ID, &ur.UserID, &ur.RoleID, &urOrgID, &grantedBy, &ur.GrantedAt, &expiresAt,
			&ur.Role.ID, &ur.Role.Name, &ur.Role.Slug, &roleDesc, &roleOrgID,
			&ur.Role.IsSystem, &ur.Role.IsDefault, &ur.Role.Priority,
			&ur.Role.CreatedAt, &ur.Role.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scanning user role: %w", err)
		}

		if urOrgID.Valid {
			ur.OrganizationID = &urOrgID.String
		}
		if grantedBy.Valid {
			ur.GrantedBy = &grantedBy.String
		}
		if expiresAt.Valid {
			ur.ExpiresAt = &expiresAt.Time
		}
		if roleOrgID.Valid {
			ur.Role.OrganizationID = &roleOrgID.String
		}
		ur.Role.Description = roleDesc.String

		userRoles = append(userRoles, ur)
	}

	return userRoles, rows.Err()
}

// -----------------------------------------------------------------------------
// Permission Checking
// -----------------------------------------------------------------------------

// HasPermission checks if a user has a specific permission
func (r *RBACRepo) HasPermission(ctx context.Context, userID, permissionID string, orgID *string) (bool, error) {
	query := `SELECT has_permission($1, $2, $3)`

	var hasPermission bool
	err := r.db.QueryRowContext(ctx, query, userID, permissionID, orgID).Scan(&hasPermission)
	if err != nil {
		return false, fmt.Errorf("checking permission: %w", err)
	}

	return hasPermission, nil
}

// GetUserPermissions retrieves all effective permissions for a user
func (r *RBACRepo) GetUserPermissions(ctx context.Context, userID string, orgID *string) ([]string, error) {
	query := `SELECT permission_id FROM get_user_permissions($1, $2)`

	rows, err := r.db.QueryContext(ctx, query, userID, orgID)
	if err != nil {
		return nil, fmt.Errorf("getting user permissions: %w", err)
	}
	defer rows.Close()

	var permissions []string
	for rows.Next() {
		var p string
		if err := rows.Scan(&p); err != nil {
			return nil, fmt.Errorf("scanning permission: %w", err)
		}
		permissions = append(permissions, p)
	}

	return permissions, rows.Err()
}

// CheckPermissions checks multiple permissions at once
func (r *RBACRepo) CheckPermissions(ctx context.Context, userID string, permissionIDs []string, orgID *string) (map[string]bool, error) {
	if len(permissionIDs) == 0 {
		return make(map[string]bool), nil
	}

	// Get all user permissions
	userPerms, err := r.GetUserPermissions(ctx, userID, orgID)
	if err != nil {
		return nil, err
	}

	// Build lookup set
	permSet := make(map[string]bool)
	for _, p := range userPerms {
		permSet[p] = true
	}

	// Check each requested permission
	result := make(map[string]bool)
	for _, p := range permissionIDs {
		result[p] = permSet[p]
	}

	return result, nil
}

// GetRolePermissions gets all permissions for a role (direct + via permission sets)
func (r *RBACRepo) GetRolePermissions(ctx context.Context, roleID string) ([]string, error) {
	query := `
		SELECT DISTINCT p.id
		FROM permissions p
		WHERE p.id IN (
			-- From permission sets
			SELECT psp.permission_id
			FROM role_permission_sets rps
			JOIN permission_set_permissions psp ON rps.permission_set_id = psp.permission_set_id
			WHERE rps.role_id = $1

			UNION

			-- Direct permissions (granted)
			SELECT rp.permission_id
			FROM role_permissions rp
			WHERE rp.role_id = $1 AND rp.granted = true
		)
		-- Exclude explicitly denied
		AND p.id NOT IN (
			SELECT rp.permission_id
			FROM role_permissions rp
			WHERE rp.role_id = $1 AND rp.granted = false
		)
		ORDER BY p.id`

	rows, err := r.db.QueryContext(ctx, query, roleID)
	if err != nil {
		return nil, fmt.Errorf("getting role permissions: %w", err)
	}
	defer rows.Close()

	var permissions []string
	for rows.Next() {
		var p string
		if err := rows.Scan(&p); err != nil {
			return nil, fmt.Errorf("scanning permission: %w", err)
		}
		permissions = append(permissions, p)
	}

	return permissions, rows.Err()
}

// GetUsersWithPermission gets all users who have a specific permission in an organization
func (r *RBACRepo) GetUsersWithPermission(ctx context.Context, permissionID string, orgID *string) ([]string, error) {
	query := `
		SELECT DISTINCT ur.user_id
		FROM user_roles ur
		WHERE (ur.expires_at IS NULL OR ur.expires_at > NOW())
		AND ($2::uuid IS NULL OR ur.organization_id = $2 OR ur.organization_id IS NULL)
		AND has_permission(ur.user_id, $1, $2)`

	rows, err := r.db.QueryContext(ctx, query, permissionID, orgID)
	if err != nil {
		return nil, fmt.Errorf("getting users with permission: %w", err)
	}
	defer rows.Close()

	var userIDs []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scanning user id: %w", err)
		}
		userIDs = append(userIDs, id)
	}

	return userIDs, rows.Err()
}

// -----------------------------------------------------------------------------
// Helper Methods
// -----------------------------------------------------------------------------

type roleScanner interface {
	Scan(dest ...any) error
}

func (r *RBACRepo) scanRole(row roleScanner) (*models.Role, error) {
	var role models.Role
	var desc, orgID sql.NullString

	err := row.Scan(
		&role.ID, &role.Name, &role.Slug, &desc, &orgID,
		&role.IsSystem, &role.IsDefault, &role.Priority,
		&role.CreatedAt, &role.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	role.Description = desc.String
	if orgID.Valid {
		role.OrganizationID = &orgID.String
	}

	return &role, nil
}

func (r *RBACRepo) scanRoleRow(rows *sql.Rows) (*models.Role, error) {
	return r.scanRole(rows)
}

// GetRoleWithPermissions retrieves a role with all its permissions loaded
func (r *RBACRepo) GetRoleWithPermissions(ctx context.Context, id string) (*models.Role, error) {
	role, err := r.GetRole(ctx, id)
	if err != nil || role == nil {
		return role, err
	}

	// Load permission IDs
	permissions, err := r.GetRolePermissions(ctx, id)
	if err != nil {
		return nil, err
	}

	// Convert to RolePermission slice
	for _, p := range permissions {
		role.Permissions = append(role.Permissions, models.RolePermission{
			RoleID:       id,
			PermissionID: p,
			Granted:      true,
		})
	}

	return role, nil
}

// BulkAssignRole assigns a role to multiple users
func (r *RBACRepo) BulkAssignRole(ctx context.Context, userIDs []string, roleID string, orgID *string, grantedBy *string) error {
	if len(userIDs) == 0 {
		return nil
	}

	tx, err := beginTx(ctx, r.db, nil)
	if err != nil {
		return fmt.Errorf("starting transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	query := `
		INSERT INTO user_roles (id, user_id, role_id, organization_id, granted_by)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (user_id, role_id, organization_id) DO NOTHING`

	for _, userID := range userIDs {
		_, err := tx.ExecContext(ctx, query,
			uuid.New().String(),
			userID,
			roleID,
			nullStringPtr(orgID),
			nullStringPtr(grantedBy),
		)
		if err != nil {
			return fmt.Errorf("assigning role to user %s: %w", userID, err)
		}
	}

	return tx.Commit()
}

// CleanupExpiredRoles removes expired role assignments and returns the count deleted
func (r *RBACRepo) CleanupExpiredRoles(ctx context.Context) (int64, error) {
	result, err := r.db.ExecContext(ctx,
		"DELETE FROM user_roles WHERE expires_at IS NOT NULL AND expires_at < NOW()")
	if err != nil {
		return 0, fmt.Errorf("cleaning up expired roles: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("getting rows affected: %w", err)
	}
	return rows, nil
}

// BulkRevokeRole revokes a role from multiple users
func (r *RBACRepo) BulkRevokeRole(ctx context.Context, userIDs []string, roleID string, orgID *string) error {
	if len(userIDs) == 0 {
		return nil
	}

	query := `DELETE FROM user_roles WHERE user_id = ANY($1) AND role_id = $2`
	args := []any{pq.Array(userIDs), roleID}

	if orgID != nil {
		query += " AND organization_id = $3"
		args = append(args, *orgID)
	} else {
		query += " AND organization_id IS NULL"
	}

	_, err := r.db.ExecContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("revoking roles: %w", err)
	}

	return nil
}
