package models

import "time"

// Permission represents an atomic permission (e.g., "pods.create")
type Permission struct {
	ID          string    `json:"id" db:"id"`
	Name        string    `json:"name" db:"name"`
	Description string    `json:"description,omitempty" db:"description"`
	Resource    string    `json:"resource" db:"resource"`
	Action      string    `json:"action" db:"action"`
	IsSystem    bool      `json:"isSystem" db:"is_system"`
	CreatedAt   time.Time `json:"createdAt" db:"created_at"`
}

// PermissionSet represents a reusable group of permissions
type PermissionSet struct {
	ID          string       `json:"id" db:"id"`
	Name        string       `json:"name" db:"name"`
	Description string       `json:"description,omitempty" db:"description"`
	IsSystem    bool         `json:"isSystem" db:"is_system"`
	CreatedAt   time.Time    `json:"createdAt" db:"created_at"`
	UpdatedAt   time.Time    `json:"updatedAt" db:"updated_at"`
	Permissions []Permission `json:"permissions,omitempty"` // Populated on demand
}

// Role represents a named collection of permissions
type Role struct {
	ID             string           `json:"id" db:"id"`
	Name           string           `json:"name" db:"name"`
	Slug           string           `json:"slug" db:"slug"`
	Description    string           `json:"description,omitempty" db:"description"`
	OrganizationID *string          `json:"organizationId,omitempty" db:"organization_id"`
	IsSystem       bool             `json:"isSystem" db:"is_system"`
	IsDefault      bool             `json:"isDefault" db:"is_default"`
	Priority       int              `json:"priority" db:"priority"`
	CreatedAt      time.Time        `json:"createdAt" db:"created_at"`
	UpdatedAt      time.Time        `json:"updatedAt" db:"updated_at"`
	PermissionSets []PermissionSet  `json:"permissionSets,omitempty"` // Populated on demand
	Permissions    []RolePermission `json:"permissions,omitempty"`    // Direct permissions
}

// RolePermission represents a direct permission assignment to a role
type RolePermission struct {
	RoleID       string `json:"roleId" db:"role_id"`
	PermissionID string `json:"permissionId" db:"permission_id"`
	Granted      bool   `json:"granted" db:"granted"` // false = explicitly denied
}

// UserRole represents a role assignment to a user
type UserRole struct {
	ID             string     `json:"id" db:"id"`
	UserID         string     `json:"userId" db:"user_id"`
	RoleID         string     `json:"roleId" db:"role_id"`
	OrganizationID *string    `json:"organizationId,omitempty" db:"organization_id"`
	GrantedBy      *string    `json:"grantedBy,omitempty" db:"granted_by"`
	GrantedAt      time.Time  `json:"grantedAt" db:"granted_at"`
	ExpiresAt      *time.Time `json:"expiresAt,omitempty" db:"expires_at"`

	// Populated on demand
	Role *Role `json:"role,omitempty"`
}

// IsExpired returns true if the user role assignment has expired
func (ur *UserRole) IsExpired() bool {
	if ur.ExpiresAt == nil {
		return false
	}
	return time.Now().After(*ur.ExpiresAt)
}

// CreateRoleRequest represents a request to create a new role
type CreateRoleRequest struct {
	Name           string   `json:"name"`
	Slug           string   `json:"slug,omitempty"`
	Description    string   `json:"description,omitempty"`
	OrganizationID *string  `json:"organizationId,omitempty"`
	IsDefault      bool     `json:"isDefault,omitempty"`
	Priority       int      `json:"priority,omitempty"`
	PermissionSets []string `json:"permissionSets,omitempty"` // IDs of permission sets
	Permissions    []string `json:"permissions,omitempty"`    // IDs of direct permissions
}

// UpdateRoleRequest represents a request to update a role
type UpdateRoleRequest struct {
	Name           *string  `json:"name,omitempty"`
	Description    *string  `json:"description,omitempty"`
	IsDefault      *bool    `json:"isDefault,omitempty"`
	Priority       *int     `json:"priority,omitempty"`
	PermissionSets []string `json:"permissionSets,omitempty"` // Replace permission sets
	Permissions    []string `json:"permissions,omitempty"`    // Replace direct permissions
}

// AssignRoleRequest represents a request to assign a role to a user
type AssignRoleRequest struct {
	UserID         string     `json:"userId"`
	RoleID         string     `json:"roleId"`
	OrganizationID *string    `json:"organizationId,omitempty"`
	ExpiresAt      *time.Time `json:"expiresAt,omitempty"`
}

// RevokeRoleRequest represents a request to revoke a role from a user
type RevokeRoleRequest struct {
	UserID         string  `json:"userId"`
	RoleID         string  `json:"roleId"`
	OrganizationID *string `json:"organizationId,omitempty"`
}

// PermissionCheck represents the result of a permission check
type PermissionCheck struct {
	UserID       string `json:"userId"`
	PermissionID string `json:"permissionId"`
	Granted      bool   `json:"granted"`
	Reason       string `json:"reason,omitempty"` // e.g., "via role: instructor"
}

// UserPermissions represents all effective permissions for a user
type UserPermissions struct {
	UserID      string   `json:"userId"`
	Permissions []string `json:"permissions"`
}
