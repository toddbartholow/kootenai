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

func TestOrganizationRepo_Create(t *testing.T) {
	fixedTime := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)

	tests := []struct {
		name    string
		org     *models.Organization
		mockFn  func(mock sqlmock.Sqlmock, org *models.Organization)
		wantErr bool
		errMsg  string
	}{
		{
			name: "success with all fields",
			org: &models.Organization{
				ID:           "org-123",
				Name:         "Test Organization",
				Slug:         "test-org",
				Type:         models.OrgTypeStandard,
				Edition:      models.EditionCommunity,
				ContactEmail: "admin@test.org",
				IsActive:     true,
				Settings: models.OrgSettings{
					AllowPublicSignup: true,
					DefaultUserRole:   models.OrgRoleMember,
				},
			},
			mockFn: func(mock sqlmock.Sqlmock, org *models.Organization) {
				rows := sqlmock.NewRows([]string{"created_at", "updated_at"}).
					AddRow(fixedTime, fixedTime)
				mock.ExpectQuery(`INSERT INTO organizations`).
					WithArgs(
						org.ID,
						org.Name,
						org.Slug,
						org.Type,
						org.Edition,
						sqlmock.AnyArg(), // license_key
						sqlmock.AnyArg(), // license_expires_at
						sqlmock.AnyArg(), // settings JSON
						sqlmock.AnyArg(), // max_users
						sqlmock.AnyArg(), // max_concurrent_pods
						sqlmock.AnyArg(), // max_storage_gb
						sqlmock.AnyArg(), // logo_url
						sqlmock.AnyArg(), // contact_email
						org.IsActive,
						sqlmock.AnyArg(), // metadata
					).
					WillReturnRows(rows)
			},
			wantErr: false,
		},
		{
			name: "generates UUID if not provided",
			org: &models.Organization{
				Name:     "No ID Org",
				Slug:     "no-id-org",
				Type:     models.OrgTypeStandard,
				Edition:  models.EditionCommunity,
				IsActive: true,
			},
			mockFn: func(mock sqlmock.Sqlmock, org *models.Organization) {
				rows := sqlmock.NewRows([]string{"created_at", "updated_at"}).
					AddRow(fixedTime, fixedTime)
				mock.ExpectQuery(`INSERT INTO organizations`).
					WillReturnRows(rows)
			},
			wantErr: false,
		},
		{
			name: "database error",
			org: &models.Organization{
				ID:       "org-err",
				Name:     "Error Org",
				Slug:     "error-org",
				Type:     models.OrgTypeStandard,
				Edition:  models.EditionCommunity,
				IsActive: true,
			},
			mockFn: func(mock sqlmock.Sqlmock, org *models.Organization) {
				mock.ExpectQuery(`INSERT INTO organizations`).
					WillReturnError(errors.New("duplicate key"))
			},
			wantErr: true,
			errMsg:  "inserting organization",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatalf("failed to create sqlmock: %v", err)
			}
			defer db.Close()

			repo := NewOrganizationRepo(db)
			tt.mockFn(mock, tt.org)

			err = repo.Create(context.Background(), tt.org)

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

func TestOrganizationRepo_GetByID(t *testing.T) {
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
			name: "existing organization",
			id:   "org-123",
			mockFn: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{
					"id", "name", "slug", "type", "edition", "license_key", "license_expires_at",
					"settings", "max_users", "max_concurrent_pods", "max_storage_gb",
					"logo_url", "contact_email", "is_active", "created_at", "updated_at", "metadata",
				}).AddRow(
					"org-123", "Test Org", "test-org", "standard", "community", nil, nil,
					[]byte(`{"allowPublicSignup":true}`), 50, 10, 100,
					"https://example.com/logo.png", "admin@test.org", true, fixedTime, fixedTime, []byte(`{}`),
				)
				mock.ExpectQuery(`SELECT .+ FROM organizations WHERE id = \$1`).
					WithArgs("org-123").
					WillReturnRows(rows)
			},
			wantNil: false,
			wantErr: false,
		},
		{
			name: "non-existent organization",
			id:   "org-nonexistent",
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`SELECT .+ FROM organizations WHERE id = \$1`).
					WithArgs("org-nonexistent").
					WillReturnError(sql.ErrNoRows)
			},
			wantNil: true,
			wantErr: false,
		},
		{
			name: "database error",
			id:   "org-err",
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`SELECT .+ FROM organizations WHERE id = \$1`).
					WithArgs("org-err").
					WillReturnError(errors.New("connection refused"))
			},
			wantNil: true,
			wantErr: true,
			errMsg:  "querying organization",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatalf("failed to create sqlmock: %v", err)
			}
			defer db.Close()

			repo := NewOrganizationRepo(db)
			tt.mockFn(mock)

			got, err := repo.GetByID(context.Background(), tt.id)

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
					t.Error("expected organization, got nil")
				}
			}

			if err := mock.ExpectationsWereMet(); err != nil {
				t.Errorf("unfulfilled expectations: %v", err)
			}
		})
	}
}

func TestOrganizationRepo_GetBySlug(t *testing.T) {
	fixedTime := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)

	tests := []struct {
		name    string
		slug    string
		mockFn  func(mock sqlmock.Sqlmock)
		wantNil bool
		wantErr bool
	}{
		{
			name: "existing organization",
			slug: "test-org",
			mockFn: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{
					"id", "name", "slug", "type", "edition", "license_key", "license_expires_at",
					"settings", "max_users", "max_concurrent_pods", "max_storage_gb",
					"logo_url", "contact_email", "is_active", "created_at", "updated_at", "metadata",
				}).AddRow(
					"org-123", "Test Org", "test-org", "standard", "community", nil, nil,
					[]byte(`{}`), nil, nil, nil, nil, nil, true, fixedTime, fixedTime, []byte(`{}`),
				)
				mock.ExpectQuery(`SELECT .+ FROM organizations WHERE slug = \$1`).
					WithArgs("test-org").
					WillReturnRows(rows)
			},
			wantNil: false,
			wantErr: false,
		},
		{
			name: "non-existent organization",
			slug: "unknown-org",
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`SELECT .+ FROM organizations WHERE slug = \$1`).
					WithArgs("unknown-org").
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

			repo := NewOrganizationRepo(db)
			tt.mockFn(mock)

			got, err := repo.GetBySlug(context.Background(), tt.slug)

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
					t.Error("expected organization, got nil")
				}
			}

			if err := mock.ExpectationsWereMet(); err != nil {
				t.Errorf("unfulfilled expectations: %v", err)
			}
		})
	}
}

func TestOrganizationRepo_Update(t *testing.T) {
	tests := []struct {
		name    string
		org     *models.Organization
		mockFn  func(mock sqlmock.Sqlmock, org *models.Organization)
		wantErr bool
		errMsg  string
	}{
		{
			name: "success",
			org: &models.Organization{
				ID:       "org-123",
				Name:     "Updated Org",
				Slug:     "updated-org",
				Type:     models.OrgTypeEducational,
				Edition:  models.EditionProfessional,
				IsActive: true,
			},
			mockFn: func(mock sqlmock.Sqlmock, org *models.Organization) {
				mock.ExpectExec(`UPDATE organizations SET`).
					WillReturnResult(sqlmock.NewResult(0, 1))
			},
			wantErr: false,
		},
		{
			name: "organization not found",
			org: &models.Organization{
				ID:       "org-nonexistent",
				Name:     "No Org",
				Slug:     "no-org",
				Type:     models.OrgTypeStandard,
				Edition:  models.EditionCommunity,
				IsActive: true,
			},
			mockFn: func(mock sqlmock.Sqlmock, org *models.Organization) {
				mock.ExpectExec(`UPDATE organizations SET`).
					WillReturnResult(sqlmock.NewResult(0, 0))
			},
			wantErr: true,
			errMsg:  "organization not found",
		},
		{
			name: "database error",
			org: &models.Organization{
				ID:       "org-err",
				Name:     "Error Org",
				Slug:     "error-org",
				Type:     models.OrgTypeStandard,
				Edition:  models.EditionCommunity,
				IsActive: true,
			},
			mockFn: func(mock sqlmock.Sqlmock, org *models.Organization) {
				mock.ExpectExec(`UPDATE organizations SET`).
					WillReturnError(errors.New("connection lost"))
			},
			wantErr: true,
			errMsg:  "updating organization",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatalf("failed to create sqlmock: %v", err)
			}
			defer db.Close()

			repo := NewOrganizationRepo(db)
			tt.mockFn(mock, tt.org)

			err = repo.Update(context.Background(), tt.org)

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

func TestOrganizationRepo_Delete(t *testing.T) {
	tests := []struct {
		name    string
		id      string
		mockFn  func(mock sqlmock.Sqlmock)
		wantErr bool
		errMsg  string
	}{
		{
			name: "success",
			id:   "org-123",
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(`DELETE FROM organizations WHERE id = \$1`).
					WithArgs("org-123").
					WillReturnResult(sqlmock.NewResult(0, 1))
			},
			wantErr: false,
		},
		{
			name: "organization not found",
			id:   "org-nonexistent",
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(`DELETE FROM organizations WHERE id = \$1`).
					WithArgs("org-nonexistent").
					WillReturnResult(sqlmock.NewResult(0, 0))
			},
			wantErr: true,
			errMsg:  "organization not found",
		},
		{
			name: "database error",
			id:   "org-err",
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(`DELETE FROM organizations WHERE id = \$1`).
					WithArgs("org-err").
					WillReturnError(errors.New("foreign key constraint"))
			},
			wantErr: true,
			errMsg:  "deleting organization",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatalf("failed to create sqlmock: %v", err)
			}
			defer db.Close()

			repo := NewOrganizationRepo(db)
			tt.mockFn(mock)

			err = repo.Delete(context.Background(), tt.id)

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

func TestOrganizationRepo_SetActive(t *testing.T) {
	tests := []struct {
		name    string
		id      string
		active  bool
		mockFn  func(mock sqlmock.Sqlmock)
		wantErr bool
		errMsg  string
	}{
		{
			name:   "set active true",
			id:     "org-123",
			active: true,
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(`UPDATE organizations SET is_active = \$2`).
					WithArgs("org-123", true).
					WillReturnResult(sqlmock.NewResult(0, 1))
			},
			wantErr: false,
		},
		{
			name:   "set active false",
			id:     "org-456",
			active: false,
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(`UPDATE organizations SET is_active = \$2`).
					WithArgs("org-456", false).
					WillReturnResult(sqlmock.NewResult(0, 1))
			},
			wantErr: false,
		},
		{
			name:   "organization not found",
			id:     "org-nonexistent",
			active: true,
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(`UPDATE organizations SET is_active = \$2`).
					WillReturnResult(sqlmock.NewResult(0, 0))
			},
			wantErr: true,
			errMsg:  "organization not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatalf("failed to create sqlmock: %v", err)
			}
			defer db.Close()

			repo := NewOrganizationRepo(db)
			tt.mockFn(mock)

			err = repo.SetActive(context.Background(), tt.id, tt.active)

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

func TestOrganizationRepo_GetMemberCount(t *testing.T) {
	tests := []struct {
		name      string
		id        string
		mockFn    func(mock sqlmock.Sqlmock)
		wantCount int
		wantErr   bool
	}{
		{
			name: "organization with members",
			id:   "org-123",
			mockFn: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{"count"}).AddRow(25)
				mock.ExpectQuery(`SELECT COUNT\(\*\) FROM organization_memberships WHERE organization_id = \$1`).
					WithArgs("org-123").
					WillReturnRows(rows)
			},
			wantCount: 25,
			wantErr:   false,
		},
		{
			name: "organization with no members",
			id:   "org-empty",
			mockFn: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{"count"}).AddRow(0)
				mock.ExpectQuery(`SELECT COUNT\(\*\) FROM organization_memberships WHERE organization_id = \$1`).
					WithArgs("org-empty").
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

			repo := NewOrganizationRepo(db)
			tt.mockFn(mock)

			got, err := repo.GetMemberCount(context.Background(), tt.id)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if got != tt.wantCount {
					t.Errorf("got count %d, want %d", got, tt.wantCount)
				}
			}

			if err := mock.ExpectationsWereMet(); err != nil {
				t.Errorf("unfulfilled expectations: %v", err)
			}
		})
	}
}

func TestOrganizationRepo_GetTeamCount(t *testing.T) {
	tests := []struct {
		name      string
		id        string
		mockFn    func(mock sqlmock.Sqlmock)
		wantCount int
		wantErr   bool
	}{
		{
			name: "organization with teams",
			id:   "org-123",
			mockFn: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{"count"}).AddRow(5)
				mock.ExpectQuery(`SELECT COUNT\(\*\) FROM teams WHERE organization_id = \$1`).
					WithArgs("org-123").
					WillReturnRows(rows)
			},
			wantCount: 5,
			wantErr:   false,
		},
		{
			name: "organization with no teams",
			id:   "org-noteams",
			mockFn: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{"count"}).AddRow(0)
				mock.ExpectQuery(`SELECT COUNT\(\*\) FROM teams WHERE organization_id = \$1`).
					WithArgs("org-noteams").
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

			repo := NewOrganizationRepo(db)
			tt.mockFn(mock)

			got, err := repo.GetTeamCount(context.Background(), tt.id)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if got != tt.wantCount {
					t.Errorf("got count %d, want %d", got, tt.wantCount)
				}
			}

			if err := mock.ExpectationsWereMet(); err != nil {
				t.Errorf("unfulfilled expectations: %v", err)
			}
		})
	}
}
