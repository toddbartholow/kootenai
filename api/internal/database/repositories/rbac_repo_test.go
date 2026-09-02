package repositories

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"

	"github.com/toddbartholow/kootenai/api/internal/models"
)

func TestRBACRepo_GetPermission(t *testing.T) {
	fixedTime := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)

	tests := []struct {
		name    string
		id      string
		mockFn  func(mock sqlmock.Sqlmock)
		wantNil bool
		wantErr bool
		errMsg  string
	}{
		{
			name: "existing permission",
			id:   "pods.create",
			mockFn: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{
					"id", "name", "description", "resource", "action", "is_system", "created_at",
				}).AddRow(
					"pods.create", "Create Pod", "Create new pods", "pods", "create", true, fixedTime,
				)
				mock.ExpectQuery(`SELECT .+ FROM permissions WHERE id = \$1`).
					WithArgs("pods.create").
					WillReturnRows(rows)
			},
			wantNil: false,
			wantErr: false,
		},
		{
			name: "non-existent permission",
			id:   "unknown.permission",
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`SELECT .+ FROM permissions WHERE id = \$1`).
					WithArgs("unknown.permission").
					WillReturnError(sql.ErrNoRows)
			},
			wantNil: true,
			wantErr: false,
		},
		{
			name: "database error",
			id:   "error.permission",
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`SELECT .+ FROM permissions WHERE id = \$1`).
					WithArgs("error.permission").
					WillReturnError(errors.New("connection refused"))
			},
			wantNil: true,
			wantErr: true,
			errMsg:  "getting permission",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatalf("failed to create sqlmock: %v", err)
			}
			defer db.Close()

			repo := NewRBACRepo(db)
			tt.mockFn(mock)

			got, err := repo.GetPermission(context.Background(), tt.id)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				} else if tt.errMsg != "" && !containsString(err.Error(), tt.errMsg) {
					t.Errorf("error %q should contain %q", err.Error(), tt.errMsg)
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if tt.wantNil && got != nil {
					t.Errorf("expected nil, got %+v", got)
				}
				if !tt.wantNil && got == nil {
					t.Error("expected permission, got nil")
				}
			}

			if err := mock.ExpectationsWereMet(); err != nil {
				t.Errorf("unfulfilled expectations: %v", err)
			}
		})
	}
}

func TestRBACRepo_ListPermissions(t *testing.T) {
	fixedTime := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)

	tests := []struct {
		name      string
		resource  string
		mockFn    func(mock sqlmock.Sqlmock)
		wantCount int
		wantErr   bool
	}{
		{
			name:     "list all permissions",
			resource: "",
			mockFn: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{
					"id", "name", "description", "resource", "action", "is_system", "created_at",
				}).
					AddRow("pods.create", "Create Pod", "Create pods", "pods", "create", true, fixedTime).
					AddRow("pods.read", "Read Pod", "Read pods", "pods", "read", true, fixedTime).
					AddRow("labs.create", "Create Lab", "Create labs", "labs", "create", true, fixedTime)
				mock.ExpectQuery(`SELECT .+ FROM permissions WHERE .+ ORDER BY resource, action`).
					WillReturnRows(rows)
			},
			wantCount: 3,
			wantErr:   false,
		},
		{
			name:     "list permissions by resource",
			resource: "pods",
			mockFn: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{
					"id", "name", "description", "resource", "action", "is_system", "created_at",
				}).
					AddRow("pods.create", "Create Pod", "Create pods", "pods", "create", true, fixedTime).
					AddRow("pods.read", "Read Pod", "Read pods", "pods", "read", true, fixedTime)
				mock.ExpectQuery(`SELECT .+ FROM permissions WHERE .+ resource = \$1`).
					WithArgs("pods").
					WillReturnRows(rows)
			},
			wantCount: 2,
			wantErr:   false,
		},
		{
			name:     "no permissions found",
			resource: "unknown",
			mockFn: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{
					"id", "name", "description", "resource", "action", "is_system", "created_at",
				})
				mock.ExpectQuery(`SELECT .+ FROM permissions WHERE .+ resource = \$1`).
					WithArgs("unknown").
					WillReturnRows(rows)
			},
			wantCount: 0,
			wantErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatalf("failed to create sqlmock: %v", err)
			}
			defer db.Close()

			repo := NewRBACRepo(db)
			tt.mockFn(mock)

			got, err := repo.ListPermissions(context.Background(), tt.resource)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if len(got) != tt.wantCount {
					t.Errorf("got %d permissions, want %d", len(got), tt.wantCount)
				}
			}

			if err := mock.ExpectationsWereMet(); err != nil {
				t.Errorf("unfulfilled expectations: %v", err)
			}
		})
	}
}

func TestRBACRepo_CreateRole(t *testing.T) {
	fixedTime := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)

	tests := []struct {
		name    string
		role    *models.Role
		mockFn  func(mock sqlmock.Sqlmock, role *models.Role)
		wantErr bool
		errMsg  string
	}{
		{
			name: "success with all fields",
			role: &models.Role{
				ID:          "role-123",
				Name:        "Custom Role",
				Slug:        "custom-role",
				Description: "A custom role",
				IsSystem:    false,
				IsDefault:   false,
				Priority:    50,
			},
			mockFn: func(mock sqlmock.Sqlmock, role *models.Role) {
				rows := sqlmock.NewRows([]string{"created_at", "updated_at"}).
					AddRow(fixedTime, fixedTime)
				mock.ExpectQuery(`INSERT INTO roles`).
					WillReturnRows(rows)
			},
			wantErr: false,
		},
		{
			name: "generates UUID if not provided",
			role: &models.Role{
				Name:     "No ID Role",
				Slug:     "no-id-role",
				IsSystem: false,
				Priority: 10,
			},
			mockFn: func(mock sqlmock.Sqlmock, role *models.Role) {
				rows := sqlmock.NewRows([]string{"created_at", "updated_at"}).
					AddRow(fixedTime, fixedTime)
				mock.ExpectQuery(`INSERT INTO roles`).
					WillReturnRows(rows)
			},
			wantErr: false,
		},
		{
			name: "database error - duplicate slug",
			role: &models.Role{
				ID:   "role-dup",
				Name: "Duplicate Role",
				Slug: "duplicate-role",
			},
			mockFn: func(mock sqlmock.Sqlmock, role *models.Role) {
				mock.ExpectQuery(`INSERT INTO roles`).
					WillReturnError(errors.New("duplicate key value"))
			},
			wantErr: true,
			errMsg:  "creating role",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatalf("failed to create sqlmock: %v", err)
			}
			defer db.Close()

			repo := NewRBACRepo(db)
			tt.mockFn(mock, tt.role)

			err = repo.CreateRole(context.Background(), tt.role)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				} else if tt.errMsg != "" && !containsString(err.Error(), tt.errMsg) {
					t.Errorf("error %q should contain %q", err.Error(), tt.errMsg)
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			}

			if err := mock.ExpectationsWereMet(); err != nil {
				t.Errorf("unfulfilled expectations: %v", err)
			}
		})
	}
}

func TestRBACRepo_GetRole(t *testing.T) {
	fixedTime := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)

	tests := []struct {
		name    string
		id      string
		mockFn  func(mock sqlmock.Sqlmock)
		wantNil bool
		wantErr bool
		errMsg  string
	}{
		{
			name: "existing role",
			id:   "role-123",
			mockFn: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{
					"id", "name", "slug", "description", "organization_id",
					"is_system", "is_default", "priority", "created_at", "updated_at",
				}).AddRow(
					"role-123", "Test Role", "test-role", "A test role", nil,
					false, false, 50, fixedTime, fixedTime,
				)
				mock.ExpectQuery(`SELECT .+ FROM roles WHERE id = \$1`).
					WithArgs("role-123").
					WillReturnRows(rows)
			},
			wantNil: false,
			wantErr: false,
		},
		{
			name: "non-existent role",
			id:   "role-nonexistent",
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`SELECT .+ FROM roles WHERE id = \$1`).
					WithArgs("role-nonexistent").
					WillReturnError(sql.ErrNoRows)
			},
			wantNil: true,
			wantErr: false,
		},
		{
			name: "database error",
			id:   "role-err",
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`SELECT .+ FROM roles WHERE id = \$1`).
					WithArgs("role-err").
					WillReturnError(errors.New("connection refused"))
			},
			wantNil: true,
			wantErr: true,
			errMsg:  "getting role",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatalf("failed to create sqlmock: %v", err)
			}
			defer db.Close()

			repo := NewRBACRepo(db)
			tt.mockFn(mock)

			got, err := repo.GetRole(context.Background(), tt.id)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				} else if tt.errMsg != "" && !containsString(err.Error(), tt.errMsg) {
					t.Errorf("error %q should contain %q", err.Error(), tt.errMsg)
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if tt.wantNil && got != nil {
					t.Errorf("expected nil, got %+v", got)
				}
				if !tt.wantNil && got == nil {
					t.Error("expected role, got nil")
				}
			}

			if err := mock.ExpectationsWereMet(); err != nil {
				t.Errorf("unfulfilled expectations: %v", err)
			}
		})
	}
}

func TestRBACRepo_GetRoleBySlug(t *testing.T) {
	fixedTime := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)

	tests := []struct {
		name    string
		slug    string
		mockFn  func(mock sqlmock.Sqlmock)
		wantNil bool
		wantErr bool
	}{
		{
			name: "existing role",
			slug: "instructor",
			mockFn: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{
					"id", "name", "slug", "description", "organization_id",
					"is_system", "is_default", "priority", "created_at", "updated_at",
				}).AddRow(
					"role-instr", "Instructor", "instructor", "Instructor role", nil,
					true, false, 50, fixedTime, fixedTime,
				)
				mock.ExpectQuery(`SELECT .+ FROM roles WHERE slug = \$1`).
					WithArgs("instructor").
					WillReturnRows(rows)
			},
			wantNil: false,
			wantErr: false,
		},
		{
			name: "non-existent role",
			slug: "unknown-role",
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`SELECT .+ FROM roles WHERE slug = \$1`).
					WithArgs("unknown-role").
					WillReturnError(sql.ErrNoRows)
			},
			wantNil: true,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatalf("failed to create sqlmock: %v", err)
			}
			defer db.Close()

			repo := NewRBACRepo(db)
			tt.mockFn(mock)

			got, err := repo.GetRoleBySlug(context.Background(), tt.slug)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if tt.wantNil && got != nil {
					t.Errorf("expected nil, got %+v", got)
				}
				if !tt.wantNil && got == nil {
					t.Error("expected role, got nil")
				}
			}

			if err := mock.ExpectationsWereMet(); err != nil {
				t.Errorf("unfulfilled expectations: %v", err)
			}
		})
	}
}

func TestRBACRepo_UpdateRole(t *testing.T) {
	fixedTime := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)

	tests := []struct {
		name    string
		role    *models.Role
		mockFn  func(mock sqlmock.Sqlmock, role *models.Role)
		wantErr bool
		errMsg  string
	}{
		{
			name: "success",
			role: &models.Role{
				ID:          "role-123",
				Name:        "Updated Role",
				Slug:        "updated-role",
				Description: "Updated description",
				IsDefault:   true,
				Priority:    75,
			},
			mockFn: func(mock sqlmock.Sqlmock, role *models.Role) {
				rows := sqlmock.NewRows([]string{"updated_at"}).AddRow(fixedTime)
				mock.ExpectQuery(`UPDATE roles`).
					WillReturnRows(rows)
			},
			wantErr: false,
		},
		{
			name: "role not found or system role",
			role: &models.Role{
				ID:   "role-nonexistent",
				Name: "No Role",
				Slug: "no-role",
			},
			mockFn: func(mock sqlmock.Sqlmock, role *models.Role) {
				mock.ExpectQuery(`UPDATE roles`).
					WillReturnError(sql.ErrNoRows)
			},
			wantErr: true,
			errMsg:  "role not found or is a system role",
		},
		{
			name: "database error",
			role: &models.Role{
				ID:   "role-err",
				Name: "Error Role",
				Slug: "error-role",
			},
			mockFn: func(mock sqlmock.Sqlmock, role *models.Role) {
				mock.ExpectQuery(`UPDATE roles`).
					WillReturnError(errors.New("connection lost"))
			},
			wantErr: true,
			errMsg:  "updating role",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatalf("failed to create sqlmock: %v", err)
			}
			defer db.Close()

			repo := NewRBACRepo(db)
			tt.mockFn(mock, tt.role)

			err = repo.UpdateRole(context.Background(), tt.role)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				} else if tt.errMsg != "" && !containsString(err.Error(), tt.errMsg) {
					t.Errorf("error %q should contain %q", err.Error(), tt.errMsg)
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			}

			if err := mock.ExpectationsWereMet(); err != nil {
				t.Errorf("unfulfilled expectations: %v", err)
			}
		})
	}
}

func TestRBACRepo_DeleteRole(t *testing.T) {
	tests := []struct {
		name    string
		id      string
		mockFn  func(mock sqlmock.Sqlmock)
		wantErr bool
		errMsg  string
	}{
		{
			name: "success",
			id:   "role-123",
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(`DELETE FROM roles WHERE id = \$1 AND is_system = false`).
					WithArgs("role-123").
					WillReturnResult(sqlmock.NewResult(0, 1))
			},
			wantErr: false,
		},
		{
			name: "cannot delete system role",
			id:   "system-role",
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(`DELETE FROM roles WHERE id = \$1 AND is_system = false`).
					WithArgs("system-role").
					WillReturnResult(sqlmock.NewResult(0, 0))
			},
			wantErr: true,
			errMsg:  "role not found or is a system role",
		},
		{
			name: "role not found",
			id:   "role-nonexistent",
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(`DELETE FROM roles WHERE id = \$1 AND is_system = false`).
					WithArgs("role-nonexistent").
					WillReturnResult(sqlmock.NewResult(0, 0))
			},
			wantErr: true,
			errMsg:  "role not found or is a system role",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatalf("failed to create sqlmock: %v", err)
			}
			defer db.Close()

			repo := NewRBACRepo(db)
			tt.mockFn(mock)

			err = repo.DeleteRole(context.Background(), tt.id)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				} else if tt.errMsg != "" && !containsString(err.Error(), tt.errMsg) {
					t.Errorf("error %q should contain %q", err.Error(), tt.errMsg)
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			}

			if err := mock.ExpectationsWereMet(); err != nil {
				t.Errorf("unfulfilled expectations: %v", err)
			}
		})
	}
}

func TestRBACRepo_AssignRole(t *testing.T) {
	fixedTime := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)

	tests := []struct {
		name     string
		userRole *models.UserRole
		mockFn   func(mock sqlmock.Sqlmock)
		wantErr  bool
		errMsg   string
	}{
		{
			name: "success",
			userRole: &models.UserRole{
				ID:        "ur-123",
				UserID:    "user-123",
				RoleID:    "role-123",
				GrantedAt: fixedTime,
			},
			mockFn: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{"granted_at"}).AddRow(fixedTime)
				mock.ExpectQuery(`INSERT INTO user_roles`).
					WillReturnRows(rows)
			},
			wantErr: false,
		},
		{
			name: "with organization context",
			userRole: &models.UserRole{
				ID:             "ur-456",
				UserID:         "user-456",
				RoleID:         "role-456",
				OrganizationID: stringPtr("org-123"),
				GrantedBy:      stringPtr("admin-user"),
				GrantedAt:      fixedTime,
			},
			mockFn: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{"granted_at"}).AddRow(fixedTime)
				mock.ExpectQuery(`INSERT INTO user_roles`).
					WillReturnRows(rows)
			},
			wantErr: false,
		},
		{
			name: "database error",
			userRole: &models.UserRole{
				ID:        "ur-dup",
				UserID:    "user-dup",
				RoleID:    "role-dup",
				GrantedAt: fixedTime,
			},
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`INSERT INTO user_roles`).
					WillReturnError(errors.New("connection error"))
			},
			wantErr: true,
			errMsg:  "assigning role",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatalf("failed to create sqlmock: %v", err)
			}
			defer db.Close()

			repo := NewRBACRepo(db)
			tt.mockFn(mock)

			err = repo.AssignRole(context.Background(), tt.userRole)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				} else if tt.errMsg != "" && !containsString(err.Error(), tt.errMsg) {
					t.Errorf("error %q should contain %q", err.Error(), tt.errMsg)
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			}

			if err := mock.ExpectationsWereMet(); err != nil {
				t.Errorf("unfulfilled expectations: %v", err)
			}
		})
	}
}

func TestRBACRepo_RevokeRole(t *testing.T) {
	tests := []struct {
		name    string
		userID  string
		roleID  string
		orgID   *string
		mockFn  func(mock sqlmock.Sqlmock)
		wantErr bool
		errMsg  string
	}{
		{
			name:   "success without org",
			userID: "user-123",
			roleID: "role-123",
			orgID:  nil,
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(`DELETE FROM user_roles WHERE user_id = \$1 AND role_id = \$2`).
					WithArgs("user-123", "role-123").
					WillReturnResult(sqlmock.NewResult(0, 1))
			},
			wantErr: false,
		},
		{
			name:   "success with org",
			userID: "user-456",
			roleID: "role-456",
			orgID:  stringPtr("org-123"),
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(`DELETE FROM user_roles WHERE user_id = \$1 AND role_id = \$2 AND organization_id = \$3`).
					WithArgs("user-456", "role-456", "org-123").
					WillReturnResult(sqlmock.NewResult(0, 1))
			},
			wantErr: false,
		},
		{
			name:   "assignment not found",
			userID: "user-nonexistent",
			roleID: "role-nonexistent",
			orgID:  nil,
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(`DELETE FROM user_roles WHERE user_id = \$1 AND role_id = \$2`).
					WillReturnResult(sqlmock.NewResult(0, 0))
			},
			wantErr: true,
			errMsg:  "role assignment not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatalf("failed to create sqlmock: %v", err)
			}
			defer db.Close()

			repo := NewRBACRepo(db)
			tt.mockFn(mock)

			err = repo.RevokeRole(context.Background(), tt.userID, tt.roleID, tt.orgID)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				} else if tt.errMsg != "" && !containsString(err.Error(), tt.errMsg) {
					t.Errorf("error %q should contain %q", err.Error(), tt.errMsg)
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			}

			if err := mock.ExpectationsWereMet(); err != nil {
				t.Errorf("unfulfilled expectations: %v", err)
			}
		})
	}
}

func TestRBACRepo_GetUserRoles(t *testing.T) {
	fixedTime := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)

	tests := []struct {
		name      string
		userID    string
		orgID     *string
		mockFn    func(mock sqlmock.Sqlmock)
		wantCount int
		wantErr   bool
	}{
		{
			name:   "user with roles (no org filter)",
			userID: "user-123",
			orgID:  nil,
			mockFn: func(mock sqlmock.Sqlmock) {
				// Query JOINs user_roles with roles table
				rows := sqlmock.NewRows([]string{
					"ur_id", "ur_user_id", "ur_role_id", "ur_organization_id", "ur_granted_by", "ur_granted_at", "ur_expires_at",
					"r_id", "r_name", "r_slug", "r_description", "r_organization_id", "r_is_system", "r_is_default", "r_priority", "r_created_at", "r_updated_at",
				}).
					AddRow("ur-1", "user-123", "role-student", nil, nil, fixedTime, nil,
						"role-student", "Student", "student", "Student role", nil, true, true, 10, fixedTime, fixedTime).
					AddRow("ur-2", "user-123", "role-instructor", "org-123", "admin", fixedTime, nil,
						"role-instructor", "Instructor", "instructor", "Instructor role", nil, true, false, 50, fixedTime, fixedTime)
				mock.ExpectQuery(`SELECT .+ FROM user_roles ur JOIN roles r ON ur.role_id = r.id WHERE .+ ur.user_id = \$1`).
					WithArgs("user-123").
					WillReturnRows(rows)
			},
			wantCount: 2,
			wantErr:   false,
		},
		{
			name:   "user with roles (org filter)",
			userID: "user-456",
			orgID:  stringPtr("org-123"),
			mockFn: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{
					"ur_id", "ur_user_id", "ur_role_id", "ur_organization_id", "ur_granted_by", "ur_granted_at", "ur_expires_at",
					"r_id", "r_name", "r_slug", "r_description", "r_organization_id", "r_is_system", "r_is_default", "r_priority", "r_created_at", "r_updated_at",
				}).
					AddRow("ur-3", "user-456", "role-member", "org-123", nil, fixedTime, nil,
						"role-member", "Member", "member", "Member role", nil, false, false, 5, fixedTime, fixedTime)
				mock.ExpectQuery(`SELECT .+ FROM user_roles ur JOIN roles r ON ur.role_id = r.id WHERE .+ ur.user_id = \$1`).
					WithArgs("user-456", "org-123").
					WillReturnRows(rows)
			},
			wantCount: 1,
			wantErr:   false,
		},
		{
			name:   "user with no roles",
			userID: "user-noroles",
			orgID:  nil,
			mockFn: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{
					"ur_id", "ur_user_id", "ur_role_id", "ur_organization_id", "ur_granted_by", "ur_granted_at", "ur_expires_at",
					"r_id", "r_name", "r_slug", "r_description", "r_organization_id", "r_is_system", "r_is_default", "r_priority", "r_created_at", "r_updated_at",
				})
				mock.ExpectQuery(`SELECT .+ FROM user_roles ur JOIN roles r ON ur.role_id = r.id WHERE .+ ur.user_id = \$1`).
					WithArgs("user-noroles").
					WillReturnRows(rows)
			},
			wantCount: 0,
			wantErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatalf("failed to create sqlmock: %v", err)
			}
			defer db.Close()

			repo := NewRBACRepo(db)
			tt.mockFn(mock)

			got, err := repo.GetUserRoles(context.Background(), tt.userID, tt.orgID)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if len(got) != tt.wantCount {
					t.Errorf("got %d roles, want %d", len(got), tt.wantCount)
				}
			}

			if err := mock.ExpectationsWereMet(); err != nil {
				t.Errorf("unfulfilled expectations: %v", err)
			}
		})
	}
}

func TestRBACRepo_HasPermission(t *testing.T) {
	tests := []struct {
		name       string
		userID     string
		permission string
		orgID      *string
		mockFn     func(mock sqlmock.Sqlmock)
		want       bool
		wantErr    bool
	}{
		{
			name:       "user has permission",
			userID:     "user-123",
			permission: "pods.create",
			orgID:      nil,
			mockFn: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{"has_permission"}).AddRow(true)
				mock.ExpectQuery(`SELECT has_permission\(\$1, \$2, \$3\)`).
					WithArgs("user-123", "pods.create", nil).
					WillReturnRows(rows)
			},
			want:    true,
			wantErr: false,
		},
		{
			name:       "user does not have permission",
			userID:     "user-456",
			permission: "admin.system",
			orgID:      nil,
			mockFn: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{"has_permission"}).AddRow(false)
				mock.ExpectQuery(`SELECT has_permission\(\$1, \$2, \$3\)`).
					WithArgs("user-456", "admin.system", nil).
					WillReturnRows(rows)
			},
			want:    false,
			wantErr: false,
		},
		{
			name:       "permission with org context",
			userID:     "user-789",
			permission: "org.manage_members",
			orgID:      stringPtr("org-123"),
			mockFn: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{"has_permission"}).AddRow(true)
				mock.ExpectQuery(`SELECT has_permission\(\$1, \$2, \$3\)`).
					WithArgs("user-789", "org.manage_members", "org-123").
					WillReturnRows(rows)
			},
			want:    true,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatalf("failed to create sqlmock: %v", err)
			}
			defer db.Close()

			repo := NewRBACRepo(db)
			tt.mockFn(mock)

			got, err := repo.HasPermission(context.Background(), tt.userID, tt.permission, tt.orgID)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if got != tt.want {
					t.Errorf("got %v, want %v", got, tt.want)
				}
			}

			if err := mock.ExpectationsWereMet(); err != nil {
				t.Errorf("unfulfilled expectations: %v", err)
			}
		})
	}
}

func TestRBACRepo_GetUserPermissions(t *testing.T) {
	tests := []struct {
		name      string
		userID    string
		orgID     *string
		mockFn    func(mock sqlmock.Sqlmock)
		wantCount int
		wantErr   bool
	}{
		{
			name:   "user with permissions",
			userID: "user-123",
			orgID:  nil,
			mockFn: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{"permission_id"}).
					AddRow("pods.read").
					AddRow("pods.create").
					AddRow("labs.read")
				mock.ExpectQuery(`SELECT permission_id FROM get_user_permissions\(\$1, \$2\)`).
					WithArgs("user-123", nil).
					WillReturnRows(rows)
			},
			wantCount: 3,
			wantErr:   false,
		},
		{
			name:   "user with no permissions",
			userID: "user-noperms",
			orgID:  nil,
			mockFn: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{"permission_id"})
				mock.ExpectQuery(`SELECT permission_id FROM get_user_permissions\(\$1, \$2\)`).
					WithArgs("user-noperms", nil).
					WillReturnRows(rows)
			},
			wantCount: 0,
			wantErr:   false,
		},
		{
			name:   "user permissions with org context",
			userID: "user-456",
			orgID:  stringPtr("org-123"),
			mockFn: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{"permission_id"}).
					AddRow("org.read").
					AddRow("org.manage_members")
				mock.ExpectQuery(`SELECT permission_id FROM get_user_permissions\(\$1, \$2\)`).
					WithArgs("user-456", "org-123").
					WillReturnRows(rows)
			},
			wantCount: 2,
			wantErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatalf("failed to create sqlmock: %v", err)
			}
			defer db.Close()

			repo := NewRBACRepo(db)
			tt.mockFn(mock)

			got, err := repo.GetUserPermissions(context.Background(), tt.userID, tt.orgID)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if len(got) != tt.wantCount {
					t.Errorf("got %d permissions, want %d", len(got), tt.wantCount)
				}
			}

			if err := mock.ExpectationsWereMet(); err != nil {
				t.Errorf("unfulfilled expectations: %v", err)
			}
		})
	}
}

func TestRBACRepo_CheckPermissions(t *testing.T) {
	tests := []struct {
		name        string
		userID      string
		permissions []string
		orgID       *string
		mockFn      func(mock sqlmock.Sqlmock)
		wantResults map[string]bool
		wantErr     bool
	}{
		{
			name:        "all permissions granted",
			userID:      "user-123",
			permissions: []string{"pods.read", "pods.create"},
			orgID:       nil,
			mockFn: func(mock sqlmock.Sqlmock) {
				// CheckPermissions internally calls GetUserPermissions
				rows := sqlmock.NewRows([]string{"permission_id"}).
					AddRow("pods.read").
					AddRow("pods.create").
					AddRow("pods.delete")
				mock.ExpectQuery(`SELECT permission_id FROM get_user_permissions`).
					WithArgs("user-123", nil).
					WillReturnRows(rows)
			},
			wantResults: map[string]bool{
				"pods.read":   true,
				"pods.create": true,
			},
			wantErr: false,
		},
		{
			name:        "some permissions denied",
			userID:      "user-456",
			permissions: []string{"pods.read", "admin.system"},
			orgID:       nil,
			mockFn: func(mock sqlmock.Sqlmock) {
				// User only has pods.read, not admin.system
				rows := sqlmock.NewRows([]string{"permission_id"}).
					AddRow("pods.read").
					AddRow("pods.create")
				mock.ExpectQuery(`SELECT permission_id FROM get_user_permissions`).
					WithArgs("user-456", nil).
					WillReturnRows(rows)
			},
			wantResults: map[string]bool{
				"pods.read":    true,
				"admin.system": false,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatalf("failed to create sqlmock: %v", err)
			}
			defer db.Close()

			repo := NewRBACRepo(db)
			tt.mockFn(mock)

			got, err := repo.CheckPermissions(context.Background(), tt.userID, tt.permissions, tt.orgID)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				for perm, want := range tt.wantResults {
					if got[perm] != want {
						t.Errorf("permission %s: got %v, want %v", perm, got[perm], want)
					}
				}
			}

			if err := mock.ExpectationsWereMet(); err != nil {
				t.Errorf("unfulfilled expectations: %v", err)
			}
		})
	}
}

func TestRBACRepo_ListRoles(t *testing.T) {
	fixedTime := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)

	tests := []struct {
		name          string
		orgID         *string
		includeSystem bool
		mockFn        func(mock sqlmock.Sqlmock)
		wantCount     int
		wantErr       bool
	}{
		{
			name:          "list all roles including system",
			orgID:         nil,
			includeSystem: true,
			mockFn: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{
					"id", "name", "slug", "description", "organization_id",
					"is_system", "is_default", "priority", "created_at", "updated_at",
				}).
					AddRow("r1", "Student", "student", "Student role", nil, true, true, 10, fixedTime, fixedTime).
					AddRow("r2", "Instructor", "instructor", "Instructor role", nil, true, false, 50, fixedTime, fixedTime).
					AddRow("r3", "Custom", "custom", "Custom role", nil, false, false, 25, fixedTime, fixedTime)
				mock.ExpectQuery(`SELECT .+ FROM roles WHERE 1=1 ORDER BY priority DESC, name`).
					WillReturnRows(rows)
			},
			wantCount: 3,
			wantErr:   false,
		},
		{
			name:          "list non-system roles only",
			orgID:         nil,
			includeSystem: false,
			mockFn: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{
					"id", "name", "slug", "description", "organization_id",
					"is_system", "is_default", "priority", "created_at", "updated_at",
				}).
					AddRow("r3", "Custom", "custom", "Custom role", nil, false, false, 25, fixedTime, fixedTime)
				mock.ExpectQuery(`SELECT .+ FROM roles WHERE 1=1 AND is_system = false ORDER BY priority DESC, name`).
					WillReturnRows(rows)
			},
			wantCount: 1,
			wantErr:   false,
		},
		{
			name:          "list roles for organization",
			orgID:         stringPtr("org-123"),
			includeSystem: true,
			mockFn: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{
					"id", "name", "slug", "description", "organization_id",
					"is_system", "is_default", "priority", "created_at", "updated_at",
				}).
					AddRow("r1", "Student", "student", "Student role", nil, true, true, 10, fixedTime, fixedTime).
					AddRow("r4", "Org Custom", "org-custom", "Org custom role", "org-123", false, false, 30, fixedTime, fixedTime)
				mock.ExpectQuery(`SELECT .+ FROM roles WHERE 1=1 AND \(organization_id = \$1 OR organization_id IS NULL\) ORDER BY priority DESC, name`).
					WithArgs("org-123").
					WillReturnRows(rows)
			},
			wantCount: 2,
			wantErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatalf("failed to create sqlmock: %v", err)
			}
			defer db.Close()

			repo := NewRBACRepo(db)
			tt.mockFn(mock)

			got, err := repo.ListRoles(context.Background(), tt.orgID, tt.includeSystem)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if len(got) != tt.wantCount {
					t.Errorf("got %d roles, want %d", len(got), tt.wantCount)
				}
			}

			if err := mock.ExpectationsWereMet(); err != nil {
				t.Errorf("unfulfilled expectations: %v", err)
			}
		})
	}
}

func TestRBACRepo_GetUsersWithPermission(t *testing.T) {
	tests := []struct {
		name       string
		permission string
		orgID      *string
		mockFn     func(mock sqlmock.Sqlmock)
		wantCount  int
		wantErr    bool
	}{
		{
			name:       "users with permission",
			permission: "pods.create",
			orgID:      nil,
			mockFn: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{"user_id"}).
					AddRow("user-1").
					AddRow("user-2").
					AddRow("user-3")
				mock.ExpectQuery(`SELECT DISTINCT ur.user_id FROM user_roles ur`).
					WillReturnRows(rows)
			},
			wantCount: 3,
			wantErr:   false,
		},
		{
			name:       "no users with permission",
			permission: "admin.system",
			orgID:      nil,
			mockFn: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{"user_id"})
				mock.ExpectQuery(`SELECT DISTINCT ur.user_id FROM user_roles ur`).
					WillReturnRows(rows)
			},
			wantCount: 0,
			wantErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatalf("failed to create sqlmock: %v", err)
			}
			defer db.Close()

			repo := NewRBACRepo(db)
			tt.mockFn(mock)

			got, err := repo.GetUsersWithPermission(context.Background(), tt.permission, tt.orgID)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if len(got) != tt.wantCount {
					t.Errorf("got %d users, want %d", len(got), tt.wantCount)
				}
			}

			if err := mock.ExpectationsWereMet(); err != nil {
				t.Errorf("unfulfilled expectations: %v", err)
			}
		})
	}
}

// stringPtr is defined in lab_template_repo_test.go
