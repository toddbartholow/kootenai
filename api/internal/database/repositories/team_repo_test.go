package repositories

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"

	"github.com/toddbartholow/kootenai/api/internal/models"
)

// -----------------------------------------------------------------------------
// TeamRepo Tests
// -----------------------------------------------------------------------------

func TestTeamRepo_Create(t *testing.T) {
	fixedTime := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)

	tests := []struct {
		name    string
		team    *models.Team
		mockFn  func(mock sqlmock.Sqlmock)
		wantErr bool
		errMsg  string
	}{
		{
			name: "success with all fields",
			team: &models.Team{
				ID:              "team-123",
				OrganizationID:  "org-123",
				Name:            "Development Team",
				Slug:            "dev-team",
				Description:     "Main development team",
				ParentTeamID:    strPtr("parent-123"),
				CanvasSectionID: strPtr("canvas-456"),
				Settings:        json.RawMessage(`{"maxMembers":10}`),
				IsActive:        true,
			},
			mockFn: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{"created_at", "updated_at"}).
					AddRow(fixedTime, fixedTime)
				mock.ExpectQuery(`INSERT INTO teams`).
					WillReturnRows(rows)
			},
			wantErr: false,
		},
		{
			name: "success with minimal fields",
			team: &models.Team{
				OrganizationID: "org-123",
				Name:           "Basic Team",
				Slug:           "basic-team",
				IsActive:       true,
			},
			mockFn: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{"created_at", "updated_at"}).
					AddRow(fixedTime, fixedTime)
				mock.ExpectQuery(`INSERT INTO teams`).
					WillReturnRows(rows)
			},
			wantErr: false,
		},
		{
			name: "database error",
			team: &models.Team{
				ID:             "team-err",
				OrganizationID: "org-123",
				Name:           "Error Team",
				Slug:           "error-team",
			},
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`INSERT INTO teams`).
					WillReturnError(errors.New("duplicate key"))
			},
			wantErr: true,
			errMsg:  "inserting team",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatalf("failed to create sqlmock: %v", err)
			}
			defer db.Close()

			repo := NewTeamRepo(db)
			tt.mockFn(mock)

			err = repo.Create(context.Background(), tt.team)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				} else if tt.errMsg != "" && !teamContainsString(err.Error(), tt.errMsg) {
					t.Errorf("error %q should contain %q", err.Error(), tt.errMsg)
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				// Verify ID was generated if not provided
				if tt.team.ID == "" {
					t.Error("expected ID to be generated")
				}
			}

			if err := mock.ExpectationsWereMet(); err != nil {
				t.Errorf("unfulfilled expectations: %v", err)
			}
		})
	}
}

func TestTeamRepo_GetByID(t *testing.T) {
	fixedTime := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)
	settingsJSON := json.RawMessage(`{"maxMembers":10}`)

	tests := []struct {
		name    string
		id      string
		mockFn  func(mock sqlmock.Sqlmock)
		wantNil bool
		wantErr bool
	}{
		{
			name: "existing team",
			id:   "team-123",
			mockFn: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{
					"id", "organization_id", "name", "slug", "description",
					"parent_team_id", "canvas_section_id", "settings", "is_active", "created_at", "updated_at",
				}).AddRow(
					"team-123", "org-123", "Dev Team", "dev-team", "Main team",
					"parent-123", "canvas-456", settingsJSON, true, fixedTime, fixedTime,
				)
				mock.ExpectQuery(`SELECT .+ FROM teams WHERE id = \$1`).
					WithArgs("team-123").
					WillReturnRows(rows)
			},
			wantNil: false,
			wantErr: false,
		},
		{
			name: "non-existent team",
			id:   "non-existent",
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`SELECT .+ FROM teams WHERE id = \$1`).
					WithArgs("non-existent").
					WillReturnError(sql.ErrNoRows)
			},
			wantNil: true,
			wantErr: false,
		},
		{
			name: "database error",
			id:   "team-err",
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`SELECT .+ FROM teams WHERE id = \$1`).
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

			repo := NewTeamRepo(db)
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

func TestTeamRepo_GetBySlug(t *testing.T) {
	fixedTime := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)
	settingsJSON := json.RawMessage(`{"maxMembers":10}`)

	tests := []struct {
		name    string
		orgID   string
		slug    string
		mockFn  func(mock sqlmock.Sqlmock)
		wantNil bool
		wantErr bool
	}{
		{
			name:  "existing team",
			orgID: "org-123",
			slug:  "dev-team",
			mockFn: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{
					"id", "organization_id", "name", "slug", "description",
					"parent_team_id", "canvas_section_id", "settings", "is_active", "created_at", "updated_at",
				}).AddRow(
					"team-123", "org-123", "Dev Team", "dev-team", "Main team",
					"parent-123", "canvas-456", settingsJSON, true, fixedTime, fixedTime,
				)
				mock.ExpectQuery(`SELECT .+ FROM teams WHERE organization_id = \$1 AND slug = \$2`).
					WithArgs("org-123", "dev-team").
					WillReturnRows(rows)
			},
			wantNil: false,
			wantErr: false,
		},
		{
			name:  "non-existent team",
			orgID: "org-123",
			slug:  "non-existent",
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`SELECT .+ FROM teams WHERE organization_id = \$1 AND slug = \$2`).
					WithArgs("org-123", "non-existent").
					WillReturnError(sql.ErrNoRows)
			},
			wantNil: true,
			wantErr: false,
		},
		{
			name:  "database error",
			orgID: "org-123",
			slug:  "error-team",
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`SELECT .+ FROM teams WHERE organization_id = \$1 AND slug = \$2`).
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

			repo := NewTeamRepo(db)
			tt.mockFn(mock)

			result, err := repo.GetBySlug(context.Background(), tt.orgID, tt.slug)

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

func TestTeamRepo_ListByOrganization(t *testing.T) {
	fixedTime := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)
	settingsJSON := json.RawMessage(`{}`)
	parentID := "parent-123"
	isActive := true

	tests := []struct {
		name      string
		orgID     string
		filter    TeamFilter
		mockFn    func(mock sqlmock.Sqlmock)
		wantCount int
		wantErr   bool
	}{
		{
			name:   "list all teams",
			orgID:  "org-123",
			filter: TeamFilter{},
			mockFn: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{
					"id", "organization_id", "name", "slug", "description",
					"parent_team_id", "canvas_section_id", "settings", "is_active", "created_at", "updated_at",
				}).AddRow(
					"team-1", "org-123", "Team 1", "team-1", "Description 1",
					nil, nil, settingsJSON, true, fixedTime, fixedTime,
				).AddRow(
					"team-2", "org-123", "Team 2", "team-2", "Description 2",
					nil, nil, settingsJSON, true, fixedTime, fixedTime,
				)
				mock.ExpectQuery(`SELECT .+ FROM teams WHERE .+ organization_id = \$1 .+ LIMIT \$2`).
					WithArgs("org-123", 100).
					WillReturnRows(rows)
			},
			wantCount: 2,
			wantErr:   false,
		},
		{
			name:  "filter by parent team",
			orgID: "org-123",
			filter: TeamFilter{
				ParentTeamID: &parentID,
			},
			mockFn: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{
					"id", "organization_id", "name", "slug", "description",
					"parent_team_id", "canvas_section_id", "settings", "is_active", "created_at", "updated_at",
				}).AddRow(
					"team-1", "org-123", "Sub Team 1", "sub-team-1", "Child team",
					"parent-123", nil, settingsJSON, true, fixedTime, fixedTime,
				)
				mock.ExpectQuery(`SELECT .+ FROM teams WHERE .+ organization_id = \$1 AND parent_team_id = \$2 .+ LIMIT \$3`).
					WithArgs("org-123", "parent-123", 100).
					WillReturnRows(rows)
			},
			wantCount: 1,
			wantErr:   false,
		},
		{
			name:  "filter by active status",
			orgID: "org-123",
			filter: TeamFilter{
				Active: &isActive,
			},
			mockFn: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{
					"id", "organization_id", "name", "slug", "description",
					"parent_team_id", "canvas_section_id", "settings", "is_active", "created_at", "updated_at",
				})
				mock.ExpectQuery(`SELECT .+ FROM teams WHERE .+ organization_id = \$1 AND is_active = \$2 .+ LIMIT \$3`).
					WithArgs("org-123", true, 100).
					WillReturnRows(rows)
			},
			wantCount: 0,
			wantErr:   false,
		},
		{
			name:  "with limit and offset",
			orgID: "org-123",
			filter: TeamFilter{
				Limit:  10,
				Offset: 5,
			},
			mockFn: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{
					"id", "organization_id", "name", "slug", "description",
					"parent_team_id", "canvas_section_id", "settings", "is_active", "created_at", "updated_at",
				})
				// QueryBuilder parameterizes LIMIT/OFFSET ($N placeholders) instead
				// of inlining them, so limit/offset values arrive through args.
				mock.ExpectQuery(`SELECT .+ FROM teams WHERE .+ LIMIT \$\d+ OFFSET \$\d+`).
					WithArgs("org-123", 10, 5).
					WillReturnRows(rows)
			},
			wantCount: 0,
			wantErr:   false,
		},
		{
			name:   "database error",
			orgID:  "org-err",
			filter: TeamFilter{},
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`SELECT .+ FROM teams WHERE`).
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

			repo := NewTeamRepo(db)
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

func TestTeamRepo_Update(t *testing.T) {
	tests := []struct {
		name    string
		team    *models.Team
		mockFn  func(mock sqlmock.Sqlmock)
		wantErr bool
		errMsg  string
	}{
		{
			name: "success",
			team: &models.Team{
				ID:              "team-123",
				Name:            "Updated Team",
				Slug:            "updated-team",
				Description:     "Updated description",
				ParentTeamID:    strPtr("new-parent"),
				CanvasSectionID: strPtr("canvas-789"),
				Settings:        json.RawMessage(`{"maxMembers":20}`),
				IsActive:        true,
			},
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(`UPDATE teams`).
					WillReturnResult(sqlmock.NewResult(0, 1))
			},
			wantErr: false,
		},
		{
			name: "team not found",
			team: &models.Team{
				ID:   "non-existent",
				Name: "Ghost Team",
				Slug: "ghost-team",
			},
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(`UPDATE teams`).
					WillReturnResult(sqlmock.NewResult(0, 0))
			},
			wantErr: true,
			errMsg:  "team not found",
		},
		{
			name: "database error",
			team: &models.Team{
				ID:   "team-err",
				Name: "Error Team",
				Slug: "error-team",
			},
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(`UPDATE teams`).
					WillReturnError(errors.New("update failed"))
			},
			wantErr: true,
			errMsg:  "updating team",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatalf("failed to create sqlmock: %v", err)
			}
			defer db.Close()

			repo := NewTeamRepo(db)
			tt.mockFn(mock)

			err = repo.Update(context.Background(), tt.team)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				} else if tt.errMsg != "" && !teamContainsString(err.Error(), tt.errMsg) {
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

func TestTeamRepo_Delete(t *testing.T) {
	tests := []struct {
		name    string
		id      string
		mockFn  func(mock sqlmock.Sqlmock)
		wantErr bool
		errMsg  string
	}{
		{
			name: "success",
			id:   "team-123",
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(`DELETE FROM teams WHERE id = \$1`).
					WithArgs("team-123").
					WillReturnResult(sqlmock.NewResult(0, 1))
			},
			wantErr: false,
		},
		{
			name: "team not found",
			id:   "non-existent",
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(`DELETE FROM teams WHERE id = \$1`).
					WithArgs("non-existent").
					WillReturnResult(sqlmock.NewResult(0, 0))
			},
			wantErr: true,
			errMsg:  "team not found",
		},
		{
			name: "database error",
			id:   "team-err",
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(`DELETE FROM teams WHERE id = \$1`).
					WillReturnError(errors.New("delete failed"))
			},
			wantErr: true,
			errMsg:  "deleting team",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatalf("failed to create sqlmock: %v", err)
			}
			defer db.Close()

			repo := NewTeamRepo(db)
			tt.mockFn(mock)

			err = repo.Delete(context.Background(), tt.id)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				} else if tt.errMsg != "" && !teamContainsString(err.Error(), tt.errMsg) {
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

func TestTeamRepo_SetActive(t *testing.T) {
	tests := []struct {
		name    string
		id      string
		active  bool
		mockFn  func(mock sqlmock.Sqlmock)
		wantErr bool
		errMsg  string
	}{
		{
			name:   "success - set active",
			id:     "team-123",
			active: true,
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(`UPDATE teams SET is_active = \$2`).
					WithArgs("team-123", true).
					WillReturnResult(sqlmock.NewResult(0, 1))
			},
			wantErr: false,
		},
		{
			name:   "success - set inactive",
			id:     "team-123",
			active: false,
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(`UPDATE teams SET is_active = \$2`).
					WithArgs("team-123", false).
					WillReturnResult(sqlmock.NewResult(0, 1))
			},
			wantErr: false,
		},
		{
			name:   "team not found",
			id:     "non-existent",
			active: true,
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(`UPDATE teams SET is_active = \$2`).
					WillReturnResult(sqlmock.NewResult(0, 0))
			},
			wantErr: true,
			errMsg:  "team not found",
		},
		{
			name:   "database error",
			id:     "team-err",
			active: true,
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(`UPDATE teams SET is_active = \$2`).
					WillReturnError(errors.New("update failed"))
			},
			wantErr: true,
			errMsg:  "setting team active status",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatalf("failed to create sqlmock: %v", err)
			}
			defer db.Close()

			repo := NewTeamRepo(db)
			tt.mockFn(mock)

			err = repo.SetActive(context.Background(), tt.id, tt.active)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				} else if tt.errMsg != "" && !teamContainsString(err.Error(), tt.errMsg) {
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

func TestTeamRepo_GetMemberCount(t *testing.T) {
	tests := []struct {
		name      string
		id        string
		mockFn    func(mock sqlmock.Sqlmock)
		wantCount int
		wantErr   bool
	}{
		{
			name: "success with members",
			id:   "team-123",
			mockFn: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{"count"}).AddRow(5)
				mock.ExpectQuery(`SELECT COUNT\(\*\) FROM team_memberships WHERE team_id = \$1`).
					WithArgs("team-123").
					WillReturnRows(rows)
			},
			wantCount: 5,
			wantErr:   false,
		},
		{
			name: "success with no members",
			id:   "team-empty",
			mockFn: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{"count"}).AddRow(0)
				mock.ExpectQuery(`SELECT COUNT\(\*\) FROM team_memberships WHERE team_id = \$1`).
					WithArgs("team-empty").
					WillReturnRows(rows)
			},
			wantCount: 0,
			wantErr:   false,
		},
		{
			name: "database error",
			id:   "team-err",
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`SELECT COUNT\(\*\) FROM team_memberships WHERE team_id = \$1`).
					WillReturnError(errors.New("count failed"))
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

			repo := NewTeamRepo(db)
			tt.mockFn(mock)

			count, err := repo.GetMemberCount(context.Background(), tt.id)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if count != tt.wantCount {
					t.Errorf("expected count %d, got %d", tt.wantCount, count)
				}
			}

			if err := mock.ExpectationsWereMet(); err != nil {
				t.Errorf("unfulfilled expectations: %v", err)
			}
		})
	}
}

// -----------------------------------------------------------------------------
// TeamMembershipRepo Tests
// -----------------------------------------------------------------------------

func TestTeamMembershipRepo_Create(t *testing.T) {
	fixedTime := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)

	tests := []struct {
		name       string
		membership *models.TeamMembership
		mockFn     func(mock sqlmock.Sqlmock)
		wantErr    bool
		errMsg     string
	}{
		{
			name: "success",
			membership: &models.TeamMembership{
				ID:     "mem-123",
				TeamID: "team-123",
				UserID: "user-123",
				Role:   models.TeamRoleMember,
			},
			mockFn: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{"created_at"}).AddRow(fixedTime)
				mock.ExpectQuery(`INSERT INTO team_memberships`).
					WillReturnRows(rows)
			},
			wantErr: false,
		},
		{
			name: "success - auto-generate ID",
			membership: &models.TeamMembership{
				TeamID: "team-123",
				UserID: "user-456",
				Role:   models.TeamRoleLead,
			},
			mockFn: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{"created_at"}).AddRow(fixedTime)
				mock.ExpectQuery(`INSERT INTO team_memberships`).
					WillReturnRows(rows)
			},
			wantErr: false,
		},
		{
			name: "database error",
			membership: &models.TeamMembership{
				ID:     "mem-err",
				TeamID: "team-123",
				UserID: "user-123",
				Role:   models.TeamRoleMember,
			},
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`INSERT INTO team_memberships`).
					WillReturnError(errors.New("duplicate key"))
			},
			wantErr: true,
			errMsg:  "inserting team membership",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatalf("failed to create sqlmock: %v", err)
			}
			defer db.Close()

			repo := NewTeamMembershipRepo(db)
			tt.mockFn(mock)

			err = repo.Create(context.Background(), tt.membership)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				} else if tt.errMsg != "" && !teamContainsString(err.Error(), tt.errMsg) {
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

func TestTeamMembershipRepo_GetByID(t *testing.T) {
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
					"id", "team_id", "user_id", "role", "created_at",
				}).AddRow(
					"mem-123", "team-123", "user-123", "member", fixedTime,
				)
				mock.ExpectQuery(`SELECT .+ FROM team_memberships WHERE id = \$1`).
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
				mock.ExpectQuery(`SELECT .+ FROM team_memberships WHERE id = \$1`).
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
				mock.ExpectQuery(`SELECT .+ FROM team_memberships WHERE id = \$1`).
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

			repo := NewTeamMembershipRepo(db)
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

func TestTeamMembershipRepo_GetByTeamAndUser(t *testing.T) {
	fixedTime := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)

	tests := []struct {
		name    string
		teamID  string
		userID  string
		mockFn  func(mock sqlmock.Sqlmock)
		wantNil bool
		wantErr bool
	}{
		{
			name:   "existing membership",
			teamID: "team-123",
			userID: "user-123",
			mockFn: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{
					"id", "team_id", "user_id", "role", "created_at",
				}).AddRow(
					"mem-123", "team-123", "user-123", "member", fixedTime,
				)
				mock.ExpectQuery(`SELECT .+ FROM team_memberships WHERE team_id = \$1 AND user_id = \$2`).
					WithArgs("team-123", "user-123").
					WillReturnRows(rows)
			},
			wantNil: false,
			wantErr: false,
		},
		{
			name:   "non-existent membership",
			teamID: "team-123",
			userID: "user-999",
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`SELECT .+ FROM team_memberships WHERE team_id = \$1 AND user_id = \$2`).
					WithArgs("team-123", "user-999").
					WillReturnError(sql.ErrNoRows)
			},
			wantNil: true,
			wantErr: false,
		},
		{
			name:   "database error",
			teamID: "team-err",
			userID: "user-err",
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`SELECT .+ FROM team_memberships WHERE team_id = \$1 AND user_id = \$2`).
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

			repo := NewTeamMembershipRepo(db)
			tt.mockFn(mock)

			result, err := repo.GetByTeamAndUser(context.Background(), tt.teamID, tt.userID)

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

func TestTeamMembershipRepo_ListByTeam(t *testing.T) {
	fixedTime := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)

	tests := []struct {
		name      string
		teamID    string
		mockFn    func(mock sqlmock.Sqlmock)
		wantCount int
		wantErr   bool
	}{
		{
			name:   "list all memberships",
			teamID: "team-123",
			mockFn: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{
					"id", "team_id", "user_id", "role", "created_at",
				}).AddRow(
					"mem-1", "team-123", "user-1", "lead", fixedTime,
				).AddRow(
					"mem-2", "team-123", "user-2", "member", fixedTime,
				)
				mock.ExpectQuery(`SELECT .+ FROM team_memberships WHERE team_id = \$1`).
					WithArgs("team-123").
					WillReturnRows(rows)
			},
			wantCount: 2,
			wantErr:   false,
		},
		{
			name:   "empty result",
			teamID: "team-empty",
			mockFn: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{
					"id", "team_id", "user_id", "role", "created_at",
				})
				mock.ExpectQuery(`SELECT .+ FROM team_memberships WHERE team_id = \$1`).
					WithArgs("team-empty").
					WillReturnRows(rows)
			},
			wantCount: 0,
			wantErr:   false,
		},
		{
			name:   "database error",
			teamID: "team-err",
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`SELECT .+ FROM team_memberships WHERE team_id = \$1`).
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

			repo := NewTeamMembershipRepo(db)
			tt.mockFn(mock)

			result, err := repo.ListByTeam(context.Background(), tt.teamID)

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

func TestTeamMembershipRepo_ListByUser(t *testing.T) {
	fixedTime := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)

	tests := []struct {
		name      string
		userID    string
		mockFn    func(mock sqlmock.Sqlmock)
		wantCount int
		wantErr   bool
	}{
		{
			name:   "list all memberships for user",
			userID: "user-123",
			mockFn: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{
					"id", "team_id", "user_id", "role", "created_at",
				}).AddRow(
					"mem-1", "team-1", "user-123", "lead", fixedTime,
				).AddRow(
					"mem-2", "team-2", "user-123", "member", fixedTime,
				)
				mock.ExpectQuery(`SELECT .+ FROM team_memberships WHERE user_id = \$1`).
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
				mock.ExpectQuery(`SELECT .+ FROM team_memberships WHERE user_id = \$1`).
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

			repo := NewTeamMembershipRepo(db)
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

func TestTeamMembershipRepo_ListByTeamAndRole(t *testing.T) {
	fixedTime := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)

	tests := []struct {
		name      string
		teamID    string
		role      models.TeamRole
		mockFn    func(mock sqlmock.Sqlmock)
		wantCount int
		wantErr   bool
	}{
		{
			name:   "list leads",
			teamID: "team-123",
			role:   models.TeamRoleLead,
			mockFn: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{
					"id", "team_id", "user_id", "role", "created_at",
				}).AddRow(
					"mem-1", "team-123", "user-1", "lead", fixedTime,
				)
				mock.ExpectQuery(`SELECT .+ FROM team_memberships WHERE team_id = \$1 AND role = \$2`).
					WithArgs("team-123", models.TeamRoleLead).
					WillReturnRows(rows)
			},
			wantCount: 1,
			wantErr:   false,
		},
		{
			name:   "database error",
			teamID: "team-err",
			role:   models.TeamRoleMember,
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`SELECT .+ FROM team_memberships WHERE team_id = \$1 AND role = \$2`).
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

			repo := NewTeamMembershipRepo(db)
			tt.mockFn(mock)

			result, err := repo.ListByTeamAndRole(context.Background(), tt.teamID, tt.role)

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

func TestTeamMembershipRepo_Update(t *testing.T) {
	tests := []struct {
		name       string
		membership *models.TeamMembership
		mockFn     func(mock sqlmock.Sqlmock)
		wantErr    bool
		errMsg     string
	}{
		{
			name: "success",
			membership: &models.TeamMembership{
				ID:   "mem-123",
				Role: models.TeamRoleLead,
			},
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(`UPDATE team_memberships SET role = \$2 WHERE id = \$1`).
					WithArgs("mem-123", models.TeamRoleLead).
					WillReturnResult(sqlmock.NewResult(0, 1))
			},
			wantErr: false,
		},
		{
			name: "membership not found",
			membership: &models.TeamMembership{
				ID:   "non-existent",
				Role: models.TeamRoleMember,
			},
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(`UPDATE team_memberships SET role = \$2 WHERE id = \$1`).
					WillReturnResult(sqlmock.NewResult(0, 0))
			},
			wantErr: true,
			errMsg:  "team membership not found",
		},
		{
			name: "database error",
			membership: &models.TeamMembership{
				ID:   "mem-err",
				Role: models.TeamRoleMember,
			},
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(`UPDATE team_memberships SET role = \$2 WHERE id = \$1`).
					WillReturnError(errors.New("update failed"))
			},
			wantErr: true,
			errMsg:  "updating team membership",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatalf("failed to create sqlmock: %v", err)
			}
			defer db.Close()

			repo := NewTeamMembershipRepo(db)
			tt.mockFn(mock)

			err = repo.Update(context.Background(), tt.membership)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				} else if tt.errMsg != "" && !teamContainsString(err.Error(), tt.errMsg) {
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

func TestTeamMembershipRepo_UpdateRole(t *testing.T) {
	tests := []struct {
		name    string
		id      string
		role    models.TeamRole
		mockFn  func(mock sqlmock.Sqlmock)
		wantErr bool
		errMsg  string
	}{
		{
			name: "success",
			id:   "mem-123",
			role: models.TeamRoleLead,
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(`UPDATE team_memberships SET role = \$2 WHERE id = \$1`).
					WithArgs("mem-123", models.TeamRoleLead).
					WillReturnResult(sqlmock.NewResult(0, 1))
			},
			wantErr: false,
		},
		{
			name: "membership not found",
			id:   "non-existent",
			role: models.TeamRoleMember,
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(`UPDATE team_memberships SET role = \$2 WHERE id = \$1`).
					WillReturnResult(sqlmock.NewResult(0, 0))
			},
			wantErr: true,
			errMsg:  "team membership not found",
		},
		{
			name: "database error",
			id:   "mem-err",
			role: models.TeamRoleMember,
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(`UPDATE team_memberships SET role = \$2 WHERE id = \$1`).
					WillReturnError(errors.New("update failed"))
			},
			wantErr: true,
			errMsg:  "updating team membership role",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatalf("failed to create sqlmock: %v", err)
			}
			defer db.Close()

			repo := NewTeamMembershipRepo(db)
			tt.mockFn(mock)

			err = repo.UpdateRole(context.Background(), tt.id, tt.role)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				} else if tt.errMsg != "" && !teamContainsString(err.Error(), tt.errMsg) {
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

func TestTeamMembershipRepo_Delete(t *testing.T) {
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
				mock.ExpectExec(`DELETE FROM team_memberships WHERE id = \$1`).
					WithArgs("mem-123").
					WillReturnResult(sqlmock.NewResult(0, 1))
			},
			wantErr: false,
		},
		{
			name: "membership not found",
			id:   "non-existent",
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(`DELETE FROM team_memberships WHERE id = \$1`).
					WithArgs("non-existent").
					WillReturnResult(sqlmock.NewResult(0, 0))
			},
			wantErr: true,
			errMsg:  "team membership not found",
		},
		{
			name: "database error",
			id:   "mem-err",
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(`DELETE FROM team_memberships WHERE id = \$1`).
					WillReturnError(errors.New("delete failed"))
			},
			wantErr: true,
			errMsg:  "deleting team membership",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatalf("failed to create sqlmock: %v", err)
			}
			defer db.Close()

			repo := NewTeamMembershipRepo(db)
			tt.mockFn(mock)

			err = repo.Delete(context.Background(), tt.id)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				} else if tt.errMsg != "" && !teamContainsString(err.Error(), tt.errMsg) {
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

func TestNewTeamRepo(t *testing.T) {
	db, _, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	repo := NewTeamRepo(db)
	if repo == nil {
		t.Error("expected non-nil repository")
	}
}

func TestNewTeamMembershipRepo(t *testing.T) {
	db, _, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	repo := NewTeamMembershipRepo(db)
	if repo == nil {
		t.Error("expected non-nil repository")
	}
}

func teamContainsString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func strPtr(s string) *string {
	return &s
}
