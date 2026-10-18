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

// -----------------------------------------------------------------------------
// OrganizationMembershipRepo Tests
// -----------------------------------------------------------------------------

func TestOrganizationMembershipRepo_Create(t *testing.T) {
	fixedTime := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)
	invitedBy := "admin-123"
	token := "invite-token-123"

	tests := []struct {
		name       string
		membership *models.OrganizationMembership
		mockFn     func(mock sqlmock.Sqlmock)
		wantErr    bool
		errMsg     string
	}{
		{
			name: "success with all fields",
			membership: &models.OrganizationMembership{
				ID:              "mem-123",
				OrganizationID:  "org-123",
				UserID:          "user-123",
				Role:            models.OrgRoleMember,
				IsPrimary:       true,
				InvitedBy:       &invitedBy,
				InvitationToken: &token,
				InvitedAt:       fixedTime,
			},
			mockFn: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{"created_at", "updated_at"}).
					AddRow(fixedTime, fixedTime)
				mock.ExpectQuery(`INSERT INTO organization_memberships`).
					WillReturnRows(rows)
			},
			wantErr: false,
		},
		{
			name: "success - auto-generate ID",
			membership: &models.OrganizationMembership{
				OrganizationID: "org-123",
				UserID:         "user-456",
				Role:           models.OrgRoleAdmin,
			},
			mockFn: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{"created_at", "updated_at"}).
					AddRow(fixedTime, fixedTime)
				mock.ExpectQuery(`INSERT INTO organization_memberships`).
					WillReturnRows(rows)
			},
			wantErr: false,
		},
		{
			name: "database error",
			membership: &models.OrganizationMembership{
				ID:             "mem-err",
				OrganizationID: "org-123",
				UserID:         "user-123",
				Role:           models.OrgRoleMember,
			},
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`INSERT INTO organization_memberships`).
					WillReturnError(errors.New("duplicate key"))
			},
			wantErr: true,
			errMsg:  "inserting organization membership",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatalf("failed to create sqlmock: %v", err)
			}
			defer db.Close()

			repo := NewOrganizationMembershipRepo(db)
			tt.mockFn(mock)

			err = repo.Create(context.Background(), tt.membership)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				} else if tt.errMsg != "" && !membershipContainsString(err.Error(), tt.errMsg) {
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

func TestOrganizationMembershipRepo_GetByID(t *testing.T) {
	fixedTime := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)

	tests := []struct {
		name    string
		id      string
		mockFn  func(mock sqlmock.Sqlmock)
		wantNil bool
		wantErr bool
	}{
		{
			name: "existing membership",
			id:   "mem-123",
			mockFn: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{
					"id", "organization_id", "user_id", "role", "is_primary",
					"invited_by", "invitation_token", "invited_at", "accepted_at", "created_at", "updated_at",
				}).AddRow(
					"mem-123", "org-123", "user-123", "member", true,
					"admin-123", nil, fixedTime, fixedTime, fixedTime, fixedTime,
				)
				mock.ExpectQuery(`SELECT .+ FROM organization_memberships WHERE id = \$1`).
					WithArgs("mem-123").
					WillReturnRows(rows)
			},
			wantNil: false,
			wantErr: false,
		},
		{
			name: "non-existent membership",
			id:   "non-existent",
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`SELECT .+ FROM organization_memberships WHERE id = \$1`).
					WithArgs("non-existent").
					WillReturnError(sql.ErrNoRows)
			},
			wantNil: true,
			wantErr: false,
		},
		{
			name: "database error",
			id:   "mem-err",
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`SELECT .+ FROM organization_memberships WHERE id = \$1`).
					WillReturnError(errors.New("connection error"))
			},
			wantNil: true,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatalf("failed to create sqlmock: %v", err)
			}
			defer db.Close()

			repo := NewOrganizationMembershipRepo(db)
			tt.mockFn(mock)

			result, err := repo.GetByID(context.Background(), tt.id)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if tt.wantNil && result != nil {
					t.Error("expected nil result")
				}
				if !tt.wantNil && result == nil {
					t.Error("expected non-nil result")
				}
			}

			if err := mock.ExpectationsWereMet(); err != nil {
				t.Errorf("unfulfilled expectations: %v", err)
			}
		})
	}
}

func TestOrganizationMembershipRepo_GetByOrgAndUser(t *testing.T) {
	fixedTime := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)

	tests := []struct {
		name    string
		orgID   string
		userID  string
		mockFn  func(mock sqlmock.Sqlmock)
		wantNil bool
		wantErr bool
	}{
		{
			name:   "existing membership",
			orgID:  "org-123",
			userID: "user-123",
			mockFn: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{
					"id", "organization_id", "user_id", "role", "is_primary",
					"invited_by", "invitation_token", "invited_at", "accepted_at", "created_at", "updated_at",
				}).AddRow(
					"mem-123", "org-123", "user-123", "member", true,
					nil, nil, fixedTime, fixedTime, fixedTime, fixedTime,
				)
				mock.ExpectQuery(`SELECT .+ FROM organization_memberships WHERE organization_id = \$1 AND user_id = \$2`).
					WithArgs("org-123", "user-123").
					WillReturnRows(rows)
			},
			wantNil: false,
			wantErr: false,
		},
		{
			name:   "non-existent membership",
			orgID:  "org-123",
			userID: "user-999",
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`SELECT .+ FROM organization_memberships WHERE organization_id = \$1 AND user_id = \$2`).
					WithArgs("org-123", "user-999").
					WillReturnError(sql.ErrNoRows)
			},
			wantNil: true,
			wantErr: false,
		},
		{
			name:   "database error",
			orgID:  "org-err",
			userID: "user-err",
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`SELECT .+ FROM organization_memberships WHERE organization_id = \$1 AND user_id = \$2`).
					WillReturnError(errors.New("connection error"))
			},
			wantNil: true,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatalf("failed to create sqlmock: %v", err)
			}
			defer db.Close()

			repo := NewOrganizationMembershipRepo(db)
			tt.mockFn(mock)

			result, err := repo.GetByOrgAndUser(context.Background(), tt.orgID, tt.userID)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if tt.wantNil && result != nil {
					t.Error("expected nil result")
				}
				if !tt.wantNil && result == nil {
					t.Error("expected non-nil result")
				}
			}

			if err := mock.ExpectationsWereMet(); err != nil {
				t.Errorf("unfulfilled expectations: %v", err)
			}
		})
	}
}

func TestOrganizationMembershipRepo_ListByOrganization(t *testing.T) {
	fixedTime := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)

	tests := []struct {
		name      string
		orgID     string
		filter    MembershipFilter
		mockFn    func(mock sqlmock.Sqlmock)
		wantCount int
		wantErr   bool
	}{
		{
			name:   "list all accepted memberships",
			orgID:  "org-123",
			filter: MembershipFilter{},
			mockFn: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{
					"id", "organization_id", "user_id", "role", "is_primary",
					"invited_by", "invitation_token", "invited_at", "accepted_at", "created_at", "updated_at",
				}).AddRow(
					"mem-1", "org-123", "user-1", "admin", true,
					nil, nil, fixedTime, fixedTime, fixedTime, fixedTime,
				).AddRow(
					"mem-2", "org-123", "user-2", "member", false,
					nil, nil, fixedTime, fixedTime, fixedTime, fixedTime,
				)
				mock.ExpectQuery(`SELECT .+ FROM organization_memberships WHERE .+ AND accepted_at IS NOT NULL .+ LIMIT \$2`).
					WithArgs("org-123", 100).
					WillReturnRows(rows)
			},
			wantCount: 2,
			wantErr:   false,
		},
		{
			name:  "include pending",
			orgID: "org-123",
			filter: MembershipFilter{
				IncludePending: true,
			},
			mockFn: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{
					"id", "organization_id", "user_id", "role", "is_primary",
					"invited_by", "invitation_token", "invited_at", "accepted_at", "created_at", "updated_at",
				}).AddRow(
					"mem-1", "org-123", "user-1", "member", false,
					"admin-1", "token-123", fixedTime, nil, fixedTime, fixedTime,
				)
				mock.ExpectQuery(`SELECT .+ FROM organization_memberships WHERE .+ organization_id = \$1 .+ LIMIT \$2`).
					WithArgs("org-123", 100).
					WillReturnRows(rows)
			},
			wantCount: 1,
			wantErr:   false,
		},
		{
			name:  "filter by role",
			orgID: "org-123",
			filter: MembershipFilter{
				Role: models.OrgRoleAdmin,
			},
			mockFn: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{
					"id", "organization_id", "user_id", "role", "is_primary",
					"invited_by", "invitation_token", "invited_at", "accepted_at", "created_at", "updated_at",
				})
				mock.ExpectQuery(`SELECT .+ FROM organization_memberships WHERE .+ AND role = \$2 .+ LIMIT \$3`).
					WithArgs("org-123", models.OrgRoleAdmin, 100).
					WillReturnRows(rows)
			},
			wantCount: 0,
			wantErr:   false,
		},
		{
			name:   "database error",
			orgID:  "org-err",
			filter: MembershipFilter{},
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`SELECT .+ FROM organization_memberships`).
					WillReturnError(errors.New("connection error"))
			},
			wantCount: 0,
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatalf("failed to create sqlmock: %v", err)
			}
			defer db.Close()

			repo := NewOrganizationMembershipRepo(db)
			tt.mockFn(mock)

			result, err := repo.ListByOrganization(context.Background(), tt.orgID, tt.filter)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if len(result) != tt.wantCount {
					t.Errorf("expected %d results, got %d", tt.wantCount, len(result))
				}
			}

			if err := mock.ExpectationsWereMet(); err != nil {
				t.Errorf("unfulfilled expectations: %v", err)
			}
		})
	}
}

func TestOrganizationMembershipRepo_ListByUser(t *testing.T) {
	fixedTime := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)

	tests := []struct {
		name      string
		userID    string
		mockFn    func(mock sqlmock.Sqlmock)
		wantCount int
		wantErr   bool
	}{
		{
			name:   "list user memberships",
			userID: "user-123",
			mockFn: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{
					"id", "organization_id", "user_id", "role", "is_primary",
					"invited_by", "invitation_token", "invited_at", "accepted_at", "created_at", "updated_at",
				}).AddRow(
					"mem-1", "org-1", "user-123", "admin", true,
					nil, nil, fixedTime, fixedTime, fixedTime, fixedTime,
				).AddRow(
					"mem-2", "org-2", "user-123", "member", false,
					nil, nil, fixedTime, fixedTime, fixedTime, fixedTime,
				)
				mock.ExpectQuery(`SELECT .+ FROM organization_memberships WHERE user_id = \$1`).
					WithArgs("user-123").
					WillReturnRows(rows)
			},
			wantCount: 2,
			wantErr:   false,
		},
		{
			name:   "database error",
			userID: "user-err",
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`SELECT .+ FROM organization_memberships WHERE user_id = \$1`).
					WillReturnError(errors.New("connection error"))
			},
			wantCount: 0,
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatalf("failed to create sqlmock: %v", err)
			}
			defer db.Close()

			repo := NewOrganizationMembershipRepo(db)
			tt.mockFn(mock)

			result, err := repo.ListByUser(context.Background(), tt.userID)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if len(result) != tt.wantCount {
					t.Errorf("expected %d results, got %d", tt.wantCount, len(result))
				}
			}

			if err := mock.ExpectationsWereMet(); err != nil {
				t.Errorf("unfulfilled expectations: %v", err)
			}
		})
	}
}

func TestOrganizationMembershipRepo_Update(t *testing.T) {
	tests := []struct {
		name       string
		membership *models.OrganizationMembership
		mockFn     func(mock sqlmock.Sqlmock)
		wantErr    bool
		errMsg     string
	}{
		{
			name: "success",
			membership: &models.OrganizationMembership{
				ID:        "mem-123",
				Role:      models.OrgRoleAdmin,
				IsPrimary: true,
			},
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(`UPDATE organization_memberships`).
					WillReturnResult(sqlmock.NewResult(0, 1))
			},
			wantErr: false,
		},
		{
			name: "membership not found",
			membership: &models.OrganizationMembership{
				ID:   "non-existent",
				Role: models.OrgRoleMember,
			},
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(`UPDATE organization_memberships`).
					WillReturnResult(sqlmock.NewResult(0, 0))
			},
			wantErr: true,
			errMsg:  "membership not found",
		},
		{
			name: "database error",
			membership: &models.OrganizationMembership{
				ID:   "mem-err",
				Role: models.OrgRoleMember,
			},
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(`UPDATE organization_memberships`).
					WillReturnError(errors.New("update failed"))
			},
			wantErr: true,
			errMsg:  "updating organization membership",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatalf("failed to create sqlmock: %v", err)
			}
			defer db.Close()

			repo := NewOrganizationMembershipRepo(db)
			tt.mockFn(mock)

			err = repo.Update(context.Background(), tt.membership)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				} else if tt.errMsg != "" && !membershipContainsString(err.Error(), tt.errMsg) {
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

func TestOrganizationMembershipRepo_UpdateRole(t *testing.T) {
	tests := []struct {
		name    string
		id      string
		role    models.OrgRole
		mockFn  func(mock sqlmock.Sqlmock)
		wantErr bool
		errMsg  string
	}{
		{
			name: "success",
			id:   "mem-123",
			role: models.OrgRoleAdmin,
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(`UPDATE organization_memberships SET role = \$2`).
					WithArgs("mem-123", models.OrgRoleAdmin).
					WillReturnResult(sqlmock.NewResult(0, 1))
			},
			wantErr: false,
		},
		{
			name: "membership not found",
			id:   "non-existent",
			role: models.OrgRoleMember,
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(`UPDATE organization_memberships SET role = \$2`).
					WillReturnResult(sqlmock.NewResult(0, 0))
			},
			wantErr: true,
			errMsg:  "membership not found",
		},
		{
			name: "database error",
			id:   "mem-err",
			role: models.OrgRoleMember,
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(`UPDATE organization_memberships SET role = \$2`).
					WillReturnError(errors.New("update failed"))
			},
			wantErr: true,
			errMsg:  "updating membership role",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatalf("failed to create sqlmock: %v", err)
			}
			defer db.Close()

			repo := NewOrganizationMembershipRepo(db)
			tt.mockFn(mock)

			err = repo.UpdateRole(context.Background(), tt.id, tt.role)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				} else if tt.errMsg != "" && !membershipContainsString(err.Error(), tt.errMsg) {
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

func TestOrganizationMembershipRepo_Delete(t *testing.T) {
	tests := []struct {
		name    string
		id      string
		mockFn  func(mock sqlmock.Sqlmock)
		wantErr bool
		errMsg  string
	}{
		{
			name: "success",
			id:   "mem-123",
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(`DELETE FROM organization_memberships WHERE id = \$1`).
					WithArgs("mem-123").
					WillReturnResult(sqlmock.NewResult(0, 1))
			},
			wantErr: false,
		},
		{
			name: "membership not found",
			id:   "non-existent",
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(`DELETE FROM organization_memberships WHERE id = \$1`).
					WithArgs("non-existent").
					WillReturnResult(sqlmock.NewResult(0, 0))
			},
			wantErr: true,
			errMsg:  "membership not found",
		},
		{
			name: "database error",
			id:   "mem-err",
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(`DELETE FROM organization_memberships WHERE id = \$1`).
					WillReturnError(errors.New("delete failed"))
			},
			wantErr: true,
			errMsg:  "deleting organization membership",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatalf("failed to create sqlmock: %v", err)
			}
			defer db.Close()

			repo := NewOrganizationMembershipRepo(db)
			tt.mockFn(mock)

			err = repo.Delete(context.Background(), tt.id)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				} else if tt.errMsg != "" && !membershipContainsString(err.Error(), tt.errMsg) {
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

func TestOrganizationMembershipRepo_AcceptInvitation(t *testing.T) {
	fixedTime := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)

	tests := []struct {
		name    string
		token   string
		mockFn  func(mock sqlmock.Sqlmock)
		wantNil bool
		wantErr bool
		errMsg  string
	}{
		{
			name:  "success",
			token: "valid-token",
			mockFn: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{
					"id", "organization_id", "user_id", "role", "is_primary",
					"invited_by", "invitation_token", "invited_at", "accepted_at", "created_at", "updated_at",
				}).AddRow(
					"mem-123", "org-123", "user-123", "member", false,
					"admin-123", nil, fixedTime, fixedTime, fixedTime, fixedTime,
				)
				mock.ExpectQuery(`UPDATE organization_memberships SET accepted_at = \$2`).
					WillReturnRows(rows)
			},
			wantNil: false,
			wantErr: false,
		},
		{
			name:  "invalid token",
			token: "invalid-token",
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`UPDATE organization_memberships SET accepted_at = \$2`).
					WillReturnError(sql.ErrNoRows)
			},
			wantNil: true,
			wantErr: true,
			errMsg:  "invalid or expired invitation token",
		},
		{
			name:  "database error",
			token: "error-token",
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`UPDATE organization_memberships SET accepted_at = \$2`).
					WillReturnError(errors.New("db error"))
			},
			wantNil: true,
			wantErr: true,
			errMsg:  "accepting invitation",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatalf("failed to create sqlmock: %v", err)
			}
			defer db.Close()

			repo := NewOrganizationMembershipRepo(db)
			tt.mockFn(mock)

			result, err := repo.AcceptInvitation(context.Background(), tt.token)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				} else if tt.errMsg != "" && !membershipContainsString(err.Error(), tt.errMsg) {
					t.Errorf("error %q should contain %q", err.Error(), tt.errMsg)
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if tt.wantNil && result != nil {
					t.Error("expected nil result")
				}
				if !tt.wantNil && result == nil {
					t.Error("expected non-nil result")
				}
			}

			if err := mock.ExpectationsWereMet(); err != nil {
				t.Errorf("unfulfilled expectations: %v", err)
			}
		})
	}
}

func TestOrganizationMembershipRepo_SetPrimary(t *testing.T) {
	tests := []struct {
		name    string
		userID  string
		orgID   string
		mockFn  func(mock sqlmock.Sqlmock)
		wantErr bool
		errMsg  string
	}{
		{
			name:   "success",
			userID: "user-123",
			orgID:  "org-123",
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectBegin()
				mock.ExpectExec(`UPDATE organization_memberships SET is_primary = false`).
					WithArgs("user-123").
					WillReturnResult(sqlmock.NewResult(0, 1))
				mock.ExpectExec(`UPDATE organization_memberships SET is_primary = true`).
					WithArgs("user-123", "org-123").
					WillReturnResult(sqlmock.NewResult(0, 1))
				mock.ExpectExec(`UPDATE users SET default_organization_id = \$2`).
					WithArgs("user-123", "org-123").
					WillReturnResult(sqlmock.NewResult(0, 1))
				mock.ExpectCommit()
			},
			wantErr: false,
		},
		{
			name:   "membership not found",
			userID: "user-123",
			orgID:  "org-999",
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectBegin()
				mock.ExpectExec(`UPDATE organization_memberships SET is_primary = false`).
					WillReturnResult(sqlmock.NewResult(0, 1))
				mock.ExpectExec(`UPDATE organization_memberships SET is_primary = true`).
					WillReturnResult(sqlmock.NewResult(0, 0))
				mock.ExpectRollback()
			},
			wantErr: true,
			errMsg:  "membership not found",
		},
		{
			name:   "begin transaction error",
			userID: "user-123",
			orgID:  "org-123",
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectBegin().WillReturnError(errors.New("begin failed"))
			},
			wantErr: true,
			errMsg:  "beginning transaction",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatalf("failed to create sqlmock: %v", err)
			}
			defer db.Close()

			repo := NewOrganizationMembershipRepo(db)
			tt.mockFn(mock)

			err = repo.SetPrimary(context.Background(), tt.userID, tt.orgID)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				} else if tt.errMsg != "" && !membershipContainsString(err.Error(), tt.errMsg) {
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

// -----------------------------------------------------------------------------
// Helper Functions
// -----------------------------------------------------------------------------

func TestNewOrganizationMembershipRepo(t *testing.T) {
	db, _, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	repo := NewOrganizationMembershipRepo(db)
	if repo == nil {
		t.Error("expected non-nil repository")
	}
}

func membershipContainsString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
