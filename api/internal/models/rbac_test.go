package models

import (
	"testing"
	"time"
)

func TestUserRole_IsExpired(t *testing.T) {
	tests := []struct {
		name        string
		userRole    *UserRole
		wantExpired bool
	}{
		{
			name: "no expiration set",
			userRole: &UserRole{
				ID:        "ur-1",
				UserID:    "user-1",
				RoleID:    "role-1",
				ExpiresAt: nil,
			},
			wantExpired: false,
		},
		{
			name: "expiration in the future",
			userRole: &UserRole{
				ID:        "ur-2",
				UserID:    "user-2",
				RoleID:    "role-2",
				ExpiresAt: timePtr(time.Now().Add(24 * time.Hour)),
			},
			wantExpired: false,
		},
		{
			name: "expiration in the past",
			userRole: &UserRole{
				ID:        "ur-3",
				UserID:    "user-3",
				RoleID:    "role-3",
				ExpiresAt: timePtr(time.Now().Add(-24 * time.Hour)),
			},
			wantExpired: true,
		},
		{
			name: "expiration just passed",
			userRole: &UserRole{
				ID:        "ur-4",
				UserID:    "user-4",
				RoleID:    "role-4",
				ExpiresAt: timePtr(time.Now().Add(-1 * time.Second)),
			},
			wantExpired: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.userRole.IsExpired()
			if got != tt.wantExpired {
				t.Errorf("IsExpired() = %v, want %v", got, tt.wantExpired)
			}
		})
	}
}

func TestPermission_Fields(t *testing.T) {
	p := Permission{
		ID:          "pods.create",
		Name:        "Create Pod",
		Description: "Allows creating new pods",
		Resource:    "pods",
		Action:      "create",
		IsSystem:    true,
	}

	if p.ID != "pods.create" {
		t.Errorf("ID = %v, want pods.create", p.ID)
	}
	if p.Resource != "pods" {
		t.Errorf("Resource = %v, want pods", p.Resource)
	}
	if p.Action != "create" {
		t.Errorf("Action = %v, want create", p.Action)
	}
	if p.Name != "Create Pod" {
		t.Errorf("Name = %v, want Create Pod", p.Name)
	}
	if p.Description != "Allows creating new pods" {
		t.Errorf("Description = %v, want Allows creating new pods", p.Description)
	}
	if !p.IsSystem {
		t.Error("IsSystem should be true")
	}
}

func TestRole_Fields(t *testing.T) {
	orgID := "org-123"
	r := Role{
		ID:             "role-123",
		Name:           "Custom Role",
		Slug:           "custom-role",
		Description:    "A custom role",
		OrganizationID: &orgID,
		IsSystem:       false,
		IsDefault:      false,
		Priority:       50,
	}

	if r.ID != "role-123" {
		t.Errorf("ID = %v, want role-123", r.ID)
	}
	if r.Name != "Custom Role" {
		t.Errorf("Name = %v, want Custom Role", r.Name)
	}
	if r.Description != "A custom role" {
		t.Errorf("Description = %v, want A custom role", r.Description)
	}
	if r.Slug != "custom-role" {
		t.Errorf("Slug = %v, want custom-role", r.Slug)
	}
	if r.IsDefault {
		t.Error("IsDefault should be false")
	}
	if r.OrganizationID == nil || *r.OrganizationID != "org-123" {
		t.Errorf("OrganizationID = %v, want org-123", r.OrganizationID)
	}
	if r.IsSystem {
		t.Error("IsSystem should be false")
	}
	if r.Priority != 50 {
		t.Errorf("Priority = %v, want 50", r.Priority)
	}
}

func TestRolePermission_GrantedFlag(t *testing.T) {
	tests := []struct {
		name    string
		rp      RolePermission
		granted bool
	}{
		{
			name: "permission granted",
			rp: RolePermission{
				RoleID:       "role-1",
				PermissionID: "pods.create",
				Granted:      true,
			},
			granted: true,
		},
		{
			name: "permission denied",
			rp: RolePermission{
				RoleID:       "role-2",
				PermissionID: "admin.system",
				Granted:      false,
			},
			granted: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.rp.Granted != tt.granted {
				t.Errorf("Granted = %v, want %v", tt.rp.Granted, tt.granted)
			}
		})
	}
}

func TestCreateRoleRequest_Validation(t *testing.T) {
	// Test that request structs can be properly populated
	req := CreateRoleRequest{
		Name:           "New Role",
		Slug:           "new-role",
		Description:    "A new role",
		IsDefault:      false,
		Priority:       25,
		PermissionSets: []string{"ps-1", "ps-2"},
		Permissions:    []string{"perm-1"},
	}

	if req.Name != "New Role" {
		t.Errorf("Name = %v, want New Role", req.Name)
	}
	if req.Slug != "new-role" {
		t.Errorf("Slug = %v, want new-role", req.Slug)
	}
	if req.Description != "A new role" {
		t.Errorf("Description = %v, want A new role", req.Description)
	}
	if req.IsDefault {
		t.Error("IsDefault should be false")
	}
	if req.Priority != 25 {
		t.Errorf("Priority = %v, want 25", req.Priority)
	}
	if len(req.PermissionSets) != 2 {
		t.Errorf("PermissionSets count = %v, want 2", len(req.PermissionSets))
	}
	if len(req.Permissions) != 1 {
		t.Errorf("Permissions count = %v, want 1", len(req.Permissions))
	}
}

func TestUpdateRoleRequest_OptionalFields(t *testing.T) {
	// Test that optional fields can be nil
	name := "Updated Name"
	priority := 75
	req := UpdateRoleRequest{
		Name:     &name,
		Priority: &priority,
	}

	if req.Name == nil || *req.Name != "Updated Name" {
		t.Errorf("Name = %v, want Updated Name", req.Name)
	}
	if req.Description != nil {
		t.Error("Description should be nil")
	}
	if req.IsDefault != nil {
		t.Error("IsDefault should be nil")
	}
	if req.Priority == nil || *req.Priority != 75 {
		t.Errorf("Priority = %v, want 75", req.Priority)
	}
}

func TestAssignRoleRequest_Fields(t *testing.T) {
	orgID := "org-123"
	expiresAt := time.Now().Add(30 * 24 * time.Hour)

	req := AssignRoleRequest{
		UserID:         "user-123",
		RoleID:         "role-123",
		OrganizationID: &orgID,
		ExpiresAt:      &expiresAt,
	}

	if req.UserID != "user-123" {
		t.Errorf("UserID = %v, want user-123", req.UserID)
	}
	if req.RoleID != "role-123" {
		t.Errorf("RoleID = %v, want role-123", req.RoleID)
	}
	if req.OrganizationID == nil || *req.OrganizationID != "org-123" {
		t.Errorf("OrganizationID = %v, want org-123", req.OrganizationID)
	}
	if req.ExpiresAt == nil {
		t.Error("ExpiresAt should not be nil")
	}
}

func TestRevokeRoleRequest_Fields(t *testing.T) {
	req := RevokeRoleRequest{
		UserID: "user-123",
		RoleID: "role-123",
	}

	if req.UserID != "user-123" {
		t.Errorf("UserID = %v, want user-123", req.UserID)
	}
	if req.RoleID != "role-123" {
		t.Errorf("RoleID = %v, want role-123", req.RoleID)
	}
	if req.OrganizationID != nil {
		t.Error("OrganizationID should be nil")
	}
}

func TestPermissionCheck_Result(t *testing.T) {
	tests := []struct {
		name  string
		check PermissionCheck
	}{
		{
			name: "permission granted",
			check: PermissionCheck{
				UserID:       "user-1",
				PermissionID: "pods.create",
				Granted:      true,
				Reason:       "via role: instructor",
			},
		},
		{
			name: "permission denied",
			check: PermissionCheck{
				UserID:       "user-2",
				PermissionID: "admin.system",
				Granted:      false,
				Reason:       "no matching role",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.check.UserID == "" {
				t.Error("UserID should not be empty")
			}
			if tt.check.PermissionID == "" {
				t.Error("PermissionID should not be empty")
			}
		})
	}
}

func TestUserPermissions_Fields(t *testing.T) {
	up := UserPermissions{
		UserID: "user-123",
		Permissions: []string{
			"pods.read",
			"pods.create",
			"labs.read",
		},
	}

	if up.UserID != "user-123" {
		t.Errorf("UserID = %v, want user-123", up.UserID)
	}
	if len(up.Permissions) != 3 {
		t.Errorf("Permissions count = %v, want 3", len(up.Permissions))
	}
}

func TestPermissionSet_WithPermissions(t *testing.T) {
	fixedTime := time.Now()
	ps := PermissionSet{
		ID:          "ps-1",
		Name:        "Pod Manager",
		Description: "Full pod management",
		IsSystem:    true,
		CreatedAt:   fixedTime,
		UpdatedAt:   fixedTime,
		Permissions: []Permission{
			{ID: "pods.read", Resource: "pods", Action: "read"},
			{ID: "pods.create", Resource: "pods", Action: "create"},
			{ID: "pods.delete", Resource: "pods", Action: "delete"},
		},
	}

	if ps.ID != "ps-1" {
		t.Errorf("ID = %v, want ps-1", ps.ID)
	}
	if ps.Name != "Pod Manager" {
		t.Errorf("Name = %v, want Pod Manager", ps.Name)
	}
	if ps.Description != "Full pod management" {
		t.Errorf("Description = %v, want Full pod management", ps.Description)
	}
	if !ps.CreatedAt.Equal(fixedTime) {
		t.Errorf("CreatedAt = %v, want %v", ps.CreatedAt, fixedTime)
	}
	if !ps.UpdatedAt.Equal(fixedTime) {
		t.Errorf("UpdatedAt = %v, want %v", ps.UpdatedAt, fixedTime)
	}
	if len(ps.Permissions) != 3 {
		t.Errorf("Permissions count = %v, want 3", len(ps.Permissions))
	}
	if !ps.IsSystem {
		t.Error("IsSystem should be true")
	}
}

// Helper function
func timePtr(t time.Time) *time.Time {
	return &t
}
