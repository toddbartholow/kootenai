package repositories

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/lib/pq"

	"github.com/toddbartholow/kootenai/api/internal/models"
)

func TestLicenseRepo_Create(t *testing.T) {
	fixedTime := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)
	expiresAt := fixedTime.AddDate(1, 0, 0) // 1 year from now
	maxUsers := 100
	maxPods := 50
	maxStorageGB := 500

	tests := []struct {
		name    string
		license *models.License
		mockFn  func(mock sqlmock.Sqlmock, license *models.License)
		wantErr bool
		errMsg  string
	}{
		{
			name: "success with all fields",
			license: &models.License{
				ID:               "lic-123",
				OrganizationID:   "org-456",
				LicenseKey:       "LIC-KEY-ABC123",
				Edition:          models.EditionEnterprise,
				IssuedAt:         fixedTime,
				ExpiresAt:        &expiresAt,
				MaxUsers:         &maxUsers,
				MaxPods:          &maxPods,
				MaxStorageGB:     &maxStorageGB,
				Features:         []string{"lti", "sso", "api"},
				IsActive:         true,
				ValidationStatus: models.LicenseStatusValid,
				Metadata:         json.RawMessage(`{"plan":"annual"}`),
			},
			mockFn: func(mock sqlmock.Sqlmock, license *models.License) {
				mock.ExpectExec(`INSERT INTO licenses`).
					WithArgs(
						license.ID,
						license.OrganizationID,
						license.LicenseKey,
						license.Edition,
						sqlmock.AnyArg(), // issued_at (may be set to now if zero)
						license.ExpiresAt,
						license.MaxUsers,
						license.MaxPods,
						license.MaxStorageGB,
						pq.Array(license.Features),
						license.IsActive,
						license.ValidationStatus,
						sqlmock.AnyArg(), // metadata
						sqlmock.AnyArg(), // created_at
						sqlmock.AnyArg(), // updated_at
					).
					WillReturnResult(sqlmock.NewResult(1, 1))
			},
			wantErr: false,
		},
		{
			name: "success with minimal fields",
			license: &models.License{
				ID:               "lic-456",
				OrganizationID:   "org-789",
				LicenseKey:       "LIC-KEY-DEF456",
				Edition:          models.EditionCommunity,
				IsActive:         true,
				ValidationStatus: models.LicenseStatusPending,
			},
			mockFn: func(mock sqlmock.Sqlmock, license *models.License) {
				mock.ExpectExec(`INSERT INTO licenses`).
					WillReturnResult(sqlmock.NewResult(1, 1))
			},
			wantErr: false,
		},
		{
			name: "database error - duplicate key",
			license: &models.License{
				ID:               "lic-dup",
				OrganizationID:   "org-dup",
				LicenseKey:       "LIC-KEY-DUP",
				Edition:          models.EditionProfessional,
				IsActive:         true,
				ValidationStatus: models.LicenseStatusValid,
			},
			mockFn: func(mock sqlmock.Sqlmock, license *models.License) {
				mock.ExpectExec(`INSERT INTO licenses`).
					WillReturnError(errors.New("duplicate key violation"))
			},
			wantErr: true,
			errMsg:  "creating license",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatalf("failed to create sqlmock: %v", err)
			}
			defer db.Close()

			repo := NewLicenseRepo(db)
			tt.mockFn(mock, tt.license)

			err = repo.Create(context.Background(), tt.license)

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

func TestLicenseRepo_GetByID(t *testing.T) {
	fixedTime := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)
	expiresAt := fixedTime.AddDate(1, 0, 0)
	maxUsers := int64(100)
	maxPods := int64(50)
	maxStorageGB := int64(500)

	tests := []struct {
		name    string
		id      string
		mockFn  func(mock sqlmock.Sqlmock)
		wantNil bool
		wantErr bool
		errMsg  string
	}{
		{
			name: "existing license",
			id:   "lic-123",
			mockFn: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{
					"id", "organization_id", "license_key", "edition", "issued_at", "expires_at",
					"max_users", "max_pods", "max_storage_gb", "features", "is_active",
					"validation_status", "last_validated_at", "validation_error",
					"created_at", "updated_at", "metadata",
				}).AddRow(
					"lic-123", "org-456", "LIC-KEY-ABC123", "enterprise", fixedTime, expiresAt,
					maxUsers, maxPods, maxStorageGB, pq.Array([]string{"lti", "sso"}), true,
					"valid", fixedTime, nil,
					fixedTime, fixedTime, []byte(`{"plan":"annual"}`),
				)
				mock.ExpectQuery(`SELECT .+ FROM licenses WHERE id = \$1`).
					WithArgs("lic-123").
					WillReturnRows(rows)
			},
			wantNil: false,
			wantErr: false,
		},
		{
			name: "license not found",
			id:   "nonexistent",
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`SELECT .+ FROM licenses WHERE id = \$1`).
					WithArgs("nonexistent").
					WillReturnError(sql.ErrNoRows)
			},
			wantNil: true,
			wantErr: false,
		},
		{
			name: "database error",
			id:   "lic-err",
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`SELECT .+ FROM licenses WHERE id = \$1`).
					WithArgs("lic-err").
					WillReturnError(errors.New("connection refused"))
			},
			wantNil: false,
			wantErr: true,
			errMsg:  "scanning license",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatalf("failed to create sqlmock: %v", err)
			}
			defer db.Close()

			repo := NewLicenseRepo(db)
			tt.mockFn(mock)

			license, err := repo.GetByID(context.Background(), tt.id)

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
				if tt.wantNil && license != nil {
					t.Errorf("expected nil license, got %+v", license)
				}
				if !tt.wantNil && license == nil {
					t.Error("expected license, got nil")
				}
			}

			if err := mock.ExpectationsWereMet(); err != nil {
				t.Errorf("unfulfilled expectations: %v", err)
			}
		})
	}
}

func TestLicenseRepo_GetByOrganization(t *testing.T) {
	fixedTime := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)

	tests := []struct {
		name    string
		orgID   string
		mockFn  func(mock sqlmock.Sqlmock)
		wantNil bool
		wantErr bool
	}{
		{
			name:  "organization with license",
			orgID: "org-456",
			mockFn: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{
					"id", "organization_id", "license_key", "edition", "issued_at", "expires_at",
					"max_users", "max_pods", "max_storage_gb", "features", "is_active",
					"validation_status", "last_validated_at", "validation_error",
					"created_at", "updated_at", "metadata",
				}).AddRow(
					"lic-123", "org-456", "LIC-KEY-ABC123", "enterprise", fixedTime, nil,
					nil, nil, nil, pq.Array([]string{}), true,
					"valid", nil, nil,
					fixedTime, fixedTime, []byte(`{}`),
				)
				mock.ExpectQuery(`SELECT .+ FROM licenses WHERE organization_id = \$1`).
					WithArgs("org-456").
					WillReturnRows(rows)
			},
			wantNil: false,
			wantErr: false,
		},
		{
			name:  "organization without license",
			orgID: "org-no-license",
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`SELECT .+ FROM licenses WHERE organization_id = \$1`).
					WithArgs("org-no-license").
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

			repo := NewLicenseRepo(db)
			tt.mockFn(mock)

			license, err := repo.GetByOrganization(context.Background(), tt.orgID)

			if tt.wantErr && err == nil {
				t.Error("expected error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if tt.wantNil && license != nil {
				t.Errorf("expected nil license, got %+v", license)
			}
			if !tt.wantNil && license == nil {
				t.Error("expected license, got nil")
			}

			if err := mock.ExpectationsWereMet(); err != nil {
				t.Errorf("unfulfilled expectations: %v", err)
			}
		})
	}
}

func TestLicenseRepo_GetByLicenseKey(t *testing.T) {
	fixedTime := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)

	tests := []struct {
		name    string
		key     string
		mockFn  func(mock sqlmock.Sqlmock)
		wantNil bool
		wantErr bool
	}{
		{
			name: "valid license key",
			key:  "LIC-KEY-ABC123", // gitleaks:allow - test fixture, not a real key
			mockFn: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{
					"id", "organization_id", "license_key", "edition", "issued_at", "expires_at",
					"max_users", "max_pods", "max_storage_gb", "features", "is_active",
					"validation_status", "last_validated_at", "validation_error",
					"created_at", "updated_at", "metadata",
				}).AddRow(
					"lic-123", "org-456", "LIC-KEY-ABC123", "enterprise", fixedTime, nil,
					nil, nil, nil, pq.Array([]string{}), true,
					"valid", nil, nil,
					fixedTime, fixedTime, []byte(`{}`),
				)
				mock.ExpectQuery(`SELECT .+ FROM licenses WHERE license_key = \$1`).
					WithArgs("LIC-KEY-ABC123").
					WillReturnRows(rows)
			},
			wantNil: false,
			wantErr: false,
		},
		{
			name: "invalid license key",
			key:  "INVALID-KEY",
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`SELECT .+ FROM licenses WHERE license_key = \$1`).
					WithArgs("INVALID-KEY").
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

			repo := NewLicenseRepo(db)
			tt.mockFn(mock)

			license, err := repo.GetByLicenseKey(context.Background(), tt.key)

			if tt.wantErr && err == nil {
				t.Error("expected error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if tt.wantNil && license != nil {
				t.Errorf("expected nil license, got %+v", license)
			}
			if !tt.wantNil && license == nil {
				t.Error("expected license, got nil")
			}

			if err := mock.ExpectationsWereMet(); err != nil {
				t.Errorf("unfulfilled expectations: %v", err)
			}
		})
	}
}

func TestLicenseRepo_GetActiveByOrganization(t *testing.T) {
	fixedTime := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)

	tests := []struct {
		name    string
		orgID   string
		mockFn  func(mock sqlmock.Sqlmock)
		wantNil bool
		wantErr bool
	}{
		{
			name:  "organization with active license",
			orgID: "org-456",
			mockFn: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{
					"id", "organization_id", "license_key", "edition", "issued_at", "expires_at",
					"max_users", "max_pods", "max_storage_gb", "features", "is_active",
					"validation_status", "last_validated_at", "validation_error",
					"created_at", "updated_at", "metadata",
				}).AddRow(
					"lic-123", "org-456", "LIC-KEY-ABC123", "enterprise", fixedTime, nil,
					nil, nil, nil, pq.Array([]string{}), true,
					"valid", nil, nil,
					fixedTime, fixedTime, []byte(`{}`),
				)
				mock.ExpectQuery(`SELECT .+ FROM licenses WHERE organization_id = \$1 AND is_active = true`).
					WithArgs("org-456").
					WillReturnRows(rows)
			},
			wantNil: false,
			wantErr: false,
		},
		{
			name:  "organization without active license",
			orgID: "org-inactive",
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`SELECT .+ FROM licenses WHERE organization_id = \$1 AND is_active = true`).
					WithArgs("org-inactive").
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

			repo := NewLicenseRepo(db)
			tt.mockFn(mock)

			license, err := repo.GetActiveByOrganization(context.Background(), tt.orgID)

			if tt.wantErr && err == nil {
				t.Error("expected error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if tt.wantNil && license != nil {
				t.Errorf("expected nil license, got %+v", license)
			}
			if !tt.wantNil && license == nil {
				t.Error("expected license, got nil")
			}

			if err := mock.ExpectationsWereMet(); err != nil {
				t.Errorf("unfulfilled expectations: %v", err)
			}
		})
	}
}

func TestLicenseRepo_Update(t *testing.T) {
	fixedTime := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)
	expiresAt := fixedTime.AddDate(1, 0, 0)
	maxUsers := 200

	tests := []struct {
		name    string
		license *models.License
		mockFn  func(mock sqlmock.Sqlmock, license *models.License)
		wantErr bool
		errMsg  string
	}{
		{
			name: "successful update",
			license: &models.License{
				ID:               "lic-123",
				Edition:          models.EditionEnterprise,
				ExpiresAt:        &expiresAt,
				MaxUsers:         &maxUsers,
				Features:         []string{"lti", "sso", "api", "advanced"},
				IsActive:         true,
				ValidationStatus: models.LicenseStatusValid,
				Metadata:         json.RawMessage(`{"plan":"enterprise-annual"}`),
			},
			mockFn: func(mock sqlmock.Sqlmock, license *models.License) {
				mock.ExpectExec(`UPDATE licenses SET`).
					WithArgs(
						license.ID,
						license.Edition,
						license.ExpiresAt,
						license.MaxUsers,
						license.MaxPods,
						license.MaxStorageGB,
						pq.Array(license.Features),
						license.IsActive,
						license.ValidationStatus,
						license.LastValidatedAt,
						license.ValidationError,
						sqlmock.AnyArg(), // metadata
						sqlmock.AnyArg(), // updated_at
					).
					WillReturnResult(sqlmock.NewResult(0, 1))
			},
			wantErr: false,
		},
		{
			name: "license not found",
			license: &models.License{
				ID:               "nonexistent",
				Edition:          models.EditionCommunity,
				IsActive:         false,
				ValidationStatus: models.LicenseStatusRevoked,
			},
			mockFn: func(mock sqlmock.Sqlmock, license *models.License) {
				mock.ExpectExec(`UPDATE licenses SET`).
					WillReturnResult(sqlmock.NewResult(0, 0))
			},
			wantErr: true,
			errMsg:  "license not found",
		},
		{
			name: "database error",
			license: &models.License{
				ID:               "lic-err",
				Edition:          models.EditionProfessional,
				IsActive:         true,
				ValidationStatus: models.LicenseStatusValid,
			},
			mockFn: func(mock sqlmock.Sqlmock, license *models.License) {
				mock.ExpectExec(`UPDATE licenses SET`).
					WillReturnError(errors.New("database locked"))
			},
			wantErr: true,
			errMsg:  "updating license",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatalf("failed to create sqlmock: %v", err)
			}
			defer db.Close()

			repo := NewLicenseRepo(db)
			tt.mockFn(mock, tt.license)

			err = repo.Update(context.Background(), tt.license)

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

func TestLicenseRepo_UpdateValidationStatus(t *testing.T) {
	validationError := "Signature verification failed"

	tests := []struct {
		name     string
		id       string
		status   models.LicenseStatus
		errorMsg *string
		mockFn   func(mock sqlmock.Sqlmock)
		wantErr  bool
		errMsg   string
	}{
		{
			name:     "successful status update to valid",
			id:       "lic-123",
			status:   models.LicenseStatusValid,
			errorMsg: nil,
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(`UPDATE licenses SET validation_status = \$2`).
					WithArgs("lic-123", models.LicenseStatusValid, sqlmock.AnyArg(), nil, sqlmock.AnyArg()).
					WillReturnResult(sqlmock.NewResult(0, 1))
			},
			wantErr: false,
		},
		{
			name:     "successful status update to expired with error",
			id:       "lic-456",
			status:   models.LicenseStatusExpired,
			errorMsg: &validationError,
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(`UPDATE licenses SET validation_status = \$2`).
					WithArgs("lic-456", models.LicenseStatusExpired, sqlmock.AnyArg(), &validationError, sqlmock.AnyArg()).
					WillReturnResult(sqlmock.NewResult(0, 1))
			},
			wantErr: false,
		},
		{
			name:     "license not found",
			id:       "nonexistent",
			status:   models.LicenseStatusRevoked,
			errorMsg: nil,
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(`UPDATE licenses SET validation_status = \$2`).
					WillReturnResult(sqlmock.NewResult(0, 0))
			},
			wantErr: true,
			errMsg:  "license not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatalf("failed to create sqlmock: %v", err)
			}
			defer db.Close()

			repo := NewLicenseRepo(db)
			tt.mockFn(mock)

			err = repo.UpdateValidationStatus(context.Background(), tt.id, tt.status, tt.errorMsg)

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

func TestLicenseRepo_Deactivate(t *testing.T) {
	tests := []struct {
		name    string
		id      string
		mockFn  func(mock sqlmock.Sqlmock)
		wantErr bool
		errMsg  string
	}{
		{
			name: "successful deactivation",
			id:   "lic-123",
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(`UPDATE licenses SET is_active = false`).
					WithArgs("lic-123", sqlmock.AnyArg()).
					WillReturnResult(sqlmock.NewResult(0, 1))
			},
			wantErr: false,
		},
		{
			name: "license not found",
			id:   "nonexistent",
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(`UPDATE licenses SET is_active = false`).
					WithArgs("nonexistent", sqlmock.AnyArg()).
					WillReturnResult(sqlmock.NewResult(0, 0))
			},
			wantErr: true,
			errMsg:  "license not found",
		},
		{
			name: "database error",
			id:   "lic-err",
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(`UPDATE licenses SET is_active = false`).
					WithArgs("lic-err", sqlmock.AnyArg()).
					WillReturnError(errors.New("permission denied"))
			},
			wantErr: true,
			errMsg:  "deactivating license",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatalf("failed to create sqlmock: %v", err)
			}
			defer db.Close()

			repo := NewLicenseRepo(db)
			tt.mockFn(mock)

			err = repo.Deactivate(context.Background(), tt.id)

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
