package repositories

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/lib/pq"

	"github.com/toddbartholow/kootenai/api/internal/models"
)

// -----------------------------------------------------------------------------
// FeatureRepo Tests
// -----------------------------------------------------------------------------

func TestFeatureRepo_GetByID(t *testing.T) {
	fixedTime := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)

	tests := []struct {
		name    string
		id      string
		mockFn  func(mock sqlmock.Sqlmock)
		wantNil bool
		wantErr bool
	}{
		{
			name: "existing feature",
			id:   "feature-analytics",
			mockFn: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{
					"id", "name", "description", "editions", "is_global", "default_settings", "created_at",
				}).AddRow(
					"feature-analytics", "Analytics", "Advanced analytics",
					pq.StringArray{"professional", "enterprise"}, false, []byte(`{}`), fixedTime,
				)
				mock.ExpectQuery(`SELECT .+ FROM feature_flags WHERE id = \$1`).
					WithArgs("feature-analytics").
					WillReturnRows(rows)
			},
			wantNil: false,
			wantErr: false,
		},
		{
			name: "non-existent feature",
			id:   "non-existent",
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`SELECT .+ FROM feature_flags WHERE id = \$1`).
					WithArgs("non-existent").
					WillReturnError(sql.ErrNoRows)
			},
			wantNil: true,
			wantErr: false,
		},
		{
			name: "database error",
			id:   "feature-err",
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`SELECT .+ FROM feature_flags WHERE id = \$1`).
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

			repo := NewFeatureRepo(db)
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

func TestFeatureRepo_List(t *testing.T) {
	fixedTime := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)

	tests := []struct {
		name      string
		mockFn    func(mock sqlmock.Sqlmock)
		wantCount int
		wantErr   bool
	}{
		{
			name: "list all features",
			mockFn: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{
					"id", "name", "description", "editions", "is_global", "default_settings", "created_at",
				}).AddRow(
					"feature-1", "Feature 1", "Description 1",
					pq.StringArray{"professional"}, false, []byte(`{}`), fixedTime,
				).AddRow(
					"feature-2", "Feature 2", "Description 2",
					pq.StringArray{}, true, []byte(`{}`), fixedTime,
				)
				mock.ExpectQuery(`SELECT .+ FROM feature_flags ORDER BY id`).
					WillReturnRows(rows)
			},
			wantCount: 2,
			wantErr:   false,
		},
		{
			name: "empty list",
			mockFn: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{
					"id", "name", "description", "editions", "is_global", "default_settings", "created_at",
				})
				mock.ExpectQuery(`SELECT .+ FROM feature_flags ORDER BY id`).
					WillReturnRows(rows)
			},
			wantCount: 0,
			wantErr:   false,
		},
		{
			name: "database error",
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`SELECT .+ FROM feature_flags`).
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

			repo := NewFeatureRepo(db)
			tt.mockFn(mock)

			result, err := repo.List(context.Background())

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

func TestFeatureRepo_ListByEdition(t *testing.T) {
	fixedTime := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)

	tests := []struct {
		name      string
		edition   models.Edition
		mockFn    func(mock sqlmock.Sqlmock)
		wantCount int
		wantErr   bool
	}{
		{
			name:    "list features for edition",
			edition: models.EditionProfessional,
			mockFn: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{
					"id", "name", "description", "editions", "is_global", "default_settings", "created_at",
				}).AddRow(
					"feature-1", "Feature 1", "Description 1",
					pq.StringArray{"professional"}, false, []byte(`{}`), fixedTime,
				)
				mock.ExpectQuery(`SELECT .+ FROM feature_flags WHERE is_global = true OR \$1 = ANY\(editions\)`).
					WithArgs(models.EditionProfessional).
					WillReturnRows(rows)
			},
			wantCount: 1,
			wantErr:   false,
		},
		{
			name:    "database error",
			edition: models.EditionCommunity,
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`SELECT .+ FROM feature_flags`).
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

			repo := NewFeatureRepo(db)
			tt.mockFn(mock)

			result, err := repo.ListByEdition(context.Background(), tt.edition)

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

func TestFeatureRepo_IsFeatureEnabled(t *testing.T) {
	futureTime := time.Now().Add(24 * time.Hour)
	pastTime := time.Now().Add(-24 * time.Hour)

	tests := []struct {
		name        string
		orgID       string
		featureID   string
		mockFn      func(mock sqlmock.Sqlmock)
		wantEnabled bool
		wantErr     bool
	}{
		{
			name:      "enabled via override",
			orgID:     "org-123",
			featureID: "feature-1",
			mockFn: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{"enabled", "expires_at"}).
					AddRow(true, futureTime)
				mock.ExpectQuery(`SELECT enabled, expires_at FROM organization_features`).
					WithArgs("org-123", "feature-1").
					WillReturnRows(rows)
			},
			wantEnabled: true,
			wantErr:     false,
		},
		{
			name:      "disabled via override",
			orgID:     "org-123",
			featureID: "feature-1",
			mockFn: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{"enabled", "expires_at"}).
					AddRow(false, nil)
				mock.ExpectQuery(`SELECT enabled, expires_at FROM organization_features`).
					WithArgs("org-123", "feature-1").
					WillReturnRows(rows)
			},
			wantEnabled: false,
			wantErr:     false,
		},
		{
			name:      "expired override - falls through to edition check",
			orgID:     "org-123",
			featureID: "feature-1",
			mockFn: func(mock sqlmock.Sqlmock) {
				// First query returns expired override
				overrideRows := sqlmock.NewRows([]string{"enabled", "expires_at"}).
					AddRow(true, pastTime)
				mock.ExpectQuery(`SELECT enabled, expires_at FROM organization_features`).
					WithArgs("org-123", "feature-1").
					WillReturnRows(overrideRows)
				// Second query checks edition
				checkRows := sqlmock.NewRows([]string{"enabled"}).AddRow(false)
				mock.ExpectQuery(`SELECT CASE`).
					WithArgs("feature-1", "org-123").
					WillReturnRows(checkRows)
			},
			wantEnabled: false,
			wantErr:     false,
		},
		{
			name:      "no override - enabled via edition",
			orgID:     "org-123",
			featureID: "feature-1",
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`SELECT enabled, expires_at FROM organization_features`).
					WillReturnError(sql.ErrNoRows)
				checkRows := sqlmock.NewRows([]string{"enabled"}).AddRow(true)
				mock.ExpectQuery(`SELECT CASE`).
					WithArgs("feature-1", "org-123").
					WillReturnRows(checkRows)
			},
			wantEnabled: true,
			wantErr:     false,
		},
		{
			name:      "database error on override check",
			orgID:     "org-123",
			featureID: "feature-1",
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`SELECT enabled, expires_at FROM organization_features`).
					WillReturnError(errors.New("connection error"))
			},
			wantEnabled: false,
			wantErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatalf("failed to create sqlmock: %v", err)
			}
			defer db.Close()

			repo := NewFeatureRepo(db)
			tt.mockFn(mock)

			enabled, err := repo.IsFeatureEnabled(context.Background(), tt.orgID, tt.featureID)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if enabled != tt.wantEnabled {
					t.Errorf("expected enabled=%v, got %v", tt.wantEnabled, enabled)
				}
			}

			if err := mock.ExpectationsWereMet(); err != nil {
				t.Errorf("unfulfilled expectations: %v", err)
			}
		})
	}
}

func TestFeatureRepo_GetOrganizationFeatures(t *testing.T) {
	tests := []struct {
		name      string
		orgID     string
		mockFn    func(mock sqlmock.Sqlmock)
		wantCount int
		wantErr   bool
	}{
		{
			name:  "get features for org",
			orgID: "org-123",
			mockFn: func(mock sqlmock.Sqlmock) {
				// Get edition
				editionRows := sqlmock.NewRows([]string{"edition"}).AddRow("professional")
				mock.ExpectQuery(`SELECT edition FROM organizations WHERE id = \$1`).
					WithArgs("org-123").
					WillReturnRows(editionRows)
				// Get features
				featureRows := sqlmock.NewRows([]string{"id", "enabled"}).
					AddRow("feature-1", true).
					AddRow("feature-2", false)
				mock.ExpectQuery(`SELECT ff.id, CASE`).
					WithArgs("org-123", "professional").
					WillReturnRows(featureRows)
			},
			wantCount: 2,
			wantErr:   false,
		},
		{
			name:  "org not found",
			orgID: "non-existent",
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`SELECT edition FROM organizations WHERE id = \$1`).
					WillReturnError(sql.ErrNoRows)
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

			repo := NewFeatureRepo(db)
			tt.mockFn(mock)

			result, err := repo.GetOrganizationFeatures(context.Background(), tt.orgID)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if len(result) != tt.wantCount {
					t.Errorf("expected %d features, got %d", tt.wantCount, len(result))
				}
			}

			if err := mock.ExpectationsWereMet(); err != nil {
				t.Errorf("unfulfilled expectations: %v", err)
			}
		})
	}
}

func TestFeatureRepo_SetOrganizationFeature(t *testing.T) {
	fixedTime := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)
	expiresAt := fixedTime.Add(30 * 24 * time.Hour)
	grantedBy := "admin-1"

	tests := []struct {
		name    string
		feature *models.OrganizationFeature
		mockFn  func(mock sqlmock.Sqlmock)
		wantErr bool
	}{
		{
			name: "success",
			feature: &models.OrganizationFeature{
				OrganizationID: "org-123",
				FeatureID:      "feature-1",
				Enabled:        true,
				ExpiresAt:      &expiresAt,
				Settings:       []byte(`{"key":"value"}`),
				GrantedBy:      &grantedBy,
				GrantedAt:      fixedTime,
			},
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(`INSERT INTO organization_features`).
					WillReturnResult(sqlmock.NewResult(0, 1))
			},
			wantErr: false,
		},
		{
			name: "success with nil settings",
			feature: &models.OrganizationFeature{
				OrganizationID: "org-123",
				FeatureID:      "feature-2",
				Enabled:        false,
				GrantedBy:      &grantedBy,
				GrantedAt:      fixedTime,
			},
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(`INSERT INTO organization_features`).
					WillReturnResult(sqlmock.NewResult(0, 1))
			},
			wantErr: false,
		},
		{
			name: "database error",
			feature: &models.OrganizationFeature{
				OrganizationID: "org-123",
				FeatureID:      "feature-err",
				Enabled:        true,
			},
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(`INSERT INTO organization_features`).
					WillReturnError(errors.New("insert failed"))
			},
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

			repo := NewFeatureRepo(db)
			tt.mockFn(mock)

			err = repo.SetOrganizationFeature(context.Background(), tt.feature)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
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

func TestFeatureRepo_RemoveOrganizationFeature(t *testing.T) {
	tests := []struct {
		name      string
		orgID     string
		featureID string
		mockFn    func(mock sqlmock.Sqlmock)
		wantErr   bool
	}{
		{
			name:      "success",
			orgID:     "org-123",
			featureID: "feature-1",
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(`DELETE FROM organization_features WHERE organization_id = \$1 AND feature_id = \$2`).
					WithArgs("org-123", "feature-1").
					WillReturnResult(sqlmock.NewResult(0, 1))
			},
			wantErr: false,
		},
		{
			name:      "database error",
			orgID:     "org-err",
			featureID: "feature-err",
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(`DELETE FROM organization_features`).
					WillReturnError(errors.New("delete failed"))
			},
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

			repo := NewFeatureRepo(db)
			tt.mockFn(mock)

			err = repo.RemoveOrganizationFeature(context.Background(), tt.orgID, tt.featureID)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
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
// Helper Tests
// -----------------------------------------------------------------------------

func TestNewFeatureRepo(t *testing.T) {
	db, _, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	repo := NewFeatureRepo(db)
	if repo == nil {
		t.Error("expected non-nil repository")
	}
}

func TestEditionsFromStrings(t *testing.T) {
	tests := []struct {
		name     string
		input    []string
		expected []models.Edition
	}{
		{
			name:     "empty slice",
			input:    []string{},
			expected: []models.Edition{},
		},
		{
			name:     "single edition",
			input:    []string{"professional"},
			expected: []models.Edition{models.EditionProfessional},
		},
		{
			name:     "multiple editions",
			input:    []string{"community", "professional", "enterprise"},
			expected: []models.Edition{models.EditionCommunity, models.EditionProfessional, models.EditionEnterprise},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := editionsFromStrings(tt.input)
			if len(result) != len(tt.expected) {
				t.Errorf("expected length %d, got %d", len(tt.expected), len(result))
				return
			}
			for i, e := range tt.expected {
				if result[i] != e {
					t.Errorf("at index %d: expected %s, got %s", i, e, result[i])
				}
			}
		})
	}
}
