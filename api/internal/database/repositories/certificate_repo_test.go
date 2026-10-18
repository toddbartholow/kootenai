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

func TestCertificateRepo_Create(t *testing.T) {
	fixedTime := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)
	expiresAt := fixedTime.AddDate(1, 0, 0) // 1 year from now

	tests := []struct {
		name    string
		cert    *models.Certificate
		mockFn  func(mock sqlmock.Sqlmock, cert *models.Certificate)
		wantErr bool
		errMsg  string
	}{
		{
			name: "success with all fields",
			cert: &models.Certificate{
				ID:               "cert-123",
				EnrollmentID:     "enroll-456",
				UserID:           "user-789",
				PathwayID:        "pathway-001",
				VerificationCode: "VERIFY-ABC123",
				IssuedAt:         fixedTime,
				ExpiresAt:        &expiresAt,
			},
			mockFn: func(mock sqlmock.Sqlmock, cert *models.Certificate) {
				rows := sqlmock.NewRows([]string{"created_at"}).
					AddRow(fixedTime)
				mock.ExpectQuery(`INSERT INTO certificates`).
					WithArgs(
						cert.ID,
						cert.EnrollmentID,
						cert.UserID,
						cert.PathwayID,
						cert.VerificationCode,
						cert.IssuedAt,
						cert.ExpiresAt,
					).
					WillReturnRows(rows)
			},
			wantErr: false,
		},
		{
			name: "success without expiry",
			cert: &models.Certificate{
				ID:               "cert-456",
				EnrollmentID:     "enroll-789",
				UserID:           "user-012",
				PathwayID:        "pathway-002",
				VerificationCode: "VERIFY-DEF456",
				IssuedAt:         fixedTime,
			},
			mockFn: func(mock sqlmock.Sqlmock, cert *models.Certificate) {
				rows := sqlmock.NewRows([]string{"created_at"}).
					AddRow(fixedTime)
				mock.ExpectQuery(`INSERT INTO certificates`).
					WillReturnRows(rows)
			},
			wantErr: false,
		},
		{
			name: "database error - duplicate key",
			cert: &models.Certificate{
				ID:               "cert-dup",
				EnrollmentID:     "enroll-dup",
				UserID:           "user-dup",
				PathwayID:        "pathway-dup",
				VerificationCode: "VERIFY-DUP",
				IssuedAt:         fixedTime,
			},
			mockFn: func(mock sqlmock.Sqlmock, cert *models.Certificate) {
				mock.ExpectQuery(`INSERT INTO certificates`).
					WillReturnError(errors.New("duplicate key"))
			},
			wantErr: true,
			errMsg:  "inserting certificate",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatalf("failed to create sqlmock: %v", err)
			}
			defer db.Close()

			repo := NewCertificateRepo(db)
			tt.mockFn(mock, tt.cert)

			err = repo.Create(context.Background(), tt.cert)

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

func TestCertificateRepo_GetByID(t *testing.T) {
	fixedTime := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)
	expiresAt := fixedTime.AddDate(1, 0, 0)

	tests := []struct {
		name    string
		id      string
		mockFn  func(mock sqlmock.Sqlmock)
		wantNil bool
		wantErr bool
		errMsg  string
	}{
		{
			name: "existing certificate",
			id:   "cert-123",
			mockFn: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{
					"id", "enrollment_id", "user_id", "pathway_id",
					"verification_code", "issued_at", "expires_at",
					"revoked_at", "revocation_reason", "created_at",
				}).AddRow(
					"cert-123", "enroll-456", "user-789", "pathway-001",
					"VERIFY-ABC123", fixedTime, expiresAt,
					nil, nil, fixedTime,
				)
				mock.ExpectQuery(`SELECT .+ FROM certificates WHERE id = \$1`).
					WithArgs("cert-123").
					WillReturnRows(rows)
			},
			wantNil: false,
			wantErr: false,
		},
		{
			name: "certificate not found",
			id:   "nonexistent",
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`SELECT .+ FROM certificates WHERE id = \$1`).
					WithArgs("nonexistent").
					WillReturnError(sql.ErrNoRows)
			},
			wantNil: true,
			wantErr: false,
		},
		{
			name: "database error",
			id:   "cert-err",
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`SELECT .+ FROM certificates WHERE id = \$1`).
					WithArgs("cert-err").
					WillReturnError(errors.New("connection refused"))
			},
			wantNil: false,
			wantErr: true,
			errMsg:  "getting certificate by ID",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatalf("failed to create sqlmock: %v", err)
			}
			defer db.Close()

			repo := NewCertificateRepo(db)
			tt.mockFn(mock)

			cert, err := repo.GetByID(context.Background(), tt.id)

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
				if tt.wantNil && cert != nil {
					t.Errorf("expected nil certificate, got %+v", cert)
				}
				if !tt.wantNil && cert == nil {
					t.Error("expected certificate, got nil")
				}
			}

			if err := mock.ExpectationsWereMet(); err != nil {
				t.Errorf("unfulfilled expectations: %v", err)
			}
		})
	}
}

func TestCertificateRepo_GetByEnrollmentID(t *testing.T) {
	fixedTime := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)

	tests := []struct {
		name         string
		enrollmentID string
		mockFn       func(mock sqlmock.Sqlmock)
		wantNil      bool
		wantErr      bool
	}{
		{
			name:         "existing certificate",
			enrollmentID: "enroll-456",
			mockFn: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{
					"id", "enrollment_id", "user_id", "pathway_id",
					"verification_code", "issued_at", "expires_at",
					"revoked_at", "revocation_reason", "created_at",
				}).AddRow(
					"cert-123", "enroll-456", "user-789", "pathway-001",
					"VERIFY-ABC123", fixedTime, nil,
					nil, nil, fixedTime,
				)
				mock.ExpectQuery(`SELECT .+ FROM certificates WHERE enrollment_id = \$1`).
					WithArgs("enroll-456").
					WillReturnRows(rows)
			},
			wantNil: false,
			wantErr: false,
		},
		{
			name:         "enrollment not found",
			enrollmentID: "nonexistent",
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`SELECT .+ FROM certificates WHERE enrollment_id = \$1`).
					WithArgs("nonexistent").
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

			repo := NewCertificateRepo(db)
			tt.mockFn(mock)

			cert, err := repo.GetByEnrollmentID(context.Background(), tt.enrollmentID)

			if tt.wantErr && err == nil {
				t.Error("expected error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if tt.wantNil && cert != nil {
				t.Errorf("expected nil certificate, got %+v", cert)
			}
			if !tt.wantNil && cert == nil {
				t.Error("expected certificate, got nil")
			}

			if err := mock.ExpectationsWereMet(); err != nil {
				t.Errorf("unfulfilled expectations: %v", err)
			}
		})
	}
}

func TestCertificateRepo_GetByVerificationCode(t *testing.T) {
	fixedTime := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)

	tests := []struct {
		name     string
		code     string
		mockFn   func(mock sqlmock.Sqlmock)
		wantNil  bool
		wantErr  bool
		wantCode string
	}{
		{
			name: "valid verification code",
			code: "VERIFY-ABC123",
			mockFn: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{
					"id", "enrollment_id", "user_id", "pathway_id",
					"verification_code", "issued_at", "expires_at",
					"revoked_at", "revocation_reason", "created_at",
				}).AddRow(
					"cert-123", "enroll-456", "user-789", "pathway-001",
					"VERIFY-ABC123", fixedTime, nil,
					nil, nil, fixedTime,
				)
				mock.ExpectQuery(`SELECT .+ FROM certificates WHERE verification_code = \$1`).
					WithArgs("VERIFY-ABC123").
					WillReturnRows(rows)
			},
			wantNil:  false,
			wantErr:  false,
			wantCode: "VERIFY-ABC123",
		},
		{
			name: "invalid verification code",
			code: "INVALID-CODE",
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`SELECT .+ FROM certificates WHERE verification_code = \$1`).
					WithArgs("INVALID-CODE").
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

			repo := NewCertificateRepo(db)
			tt.mockFn(mock)

			cert, err := repo.GetByVerificationCode(context.Background(), tt.code)

			if tt.wantErr && err == nil {
				t.Error("expected error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if tt.wantNil && cert != nil {
				t.Errorf("expected nil certificate, got %+v", cert)
			}
			if !tt.wantNil && cert == nil {
				t.Error("expected certificate, got nil")
			}
			if cert != nil && cert.VerificationCode != tt.wantCode {
				t.Errorf("expected verification code %s, got %s", tt.wantCode, cert.VerificationCode)
			}

			if err := mock.ExpectationsWereMet(); err != nil {
				t.Errorf("unfulfilled expectations: %v", err)
			}
		})
	}
}

func TestCertificateRepo_ListByUserID(t *testing.T) {
	fixedTime := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)

	tests := []struct {
		name      string
		userID    string
		mockFn    func(mock sqlmock.Sqlmock)
		wantCount int
		wantErr   bool
	}{
		{
			name:   "user with multiple certificates",
			userID: "user-789",
			mockFn: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{
					"id", "enrollment_id", "user_id", "pathway_id",
					"verification_code", "issued_at", "expires_at",
					"revoked_at", "revocation_reason", "created_at",
				}).
					AddRow("cert-1", "enroll-1", "user-789", "pathway-1", "CODE-1", fixedTime, nil, nil, nil, fixedTime).
					AddRow("cert-2", "enroll-2", "user-789", "pathway-2", "CODE-2", fixedTime, nil, nil, nil, fixedTime).
					AddRow("cert-3", "enroll-3", "user-789", "pathway-3", "CODE-3", fixedTime, nil, nil, nil, fixedTime)
				mock.ExpectQuery(`SELECT .+ FROM certificates WHERE user_id = \$1`).
					WithArgs("user-789").
					WillReturnRows(rows)
			},
			wantCount: 3,
			wantErr:   false,
		},
		{
			name:   "user with no certificates",
			userID: "user-empty",
			mockFn: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{
					"id", "enrollment_id", "user_id", "pathway_id",
					"verification_code", "issued_at", "expires_at",
					"revoked_at", "revocation_reason", "created_at",
				})
				mock.ExpectQuery(`SELECT .+ FROM certificates WHERE user_id = \$1`).
					WithArgs("user-empty").
					WillReturnRows(rows)
			},
			wantCount: 0,
			wantErr:   false,
		},
		{
			name:   "database error",
			userID: "user-err",
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`SELECT .+ FROM certificates WHERE user_id = \$1`).
					WithArgs("user-err").
					WillReturnError(errors.New("connection timeout"))
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

			repo := NewCertificateRepo(db)
			tt.mockFn(mock)

			certs, err := repo.ListByUserID(context.Background(), tt.userID)

			if tt.wantErr && err == nil {
				t.Error("expected error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if !tt.wantErr && len(certs) != tt.wantCount {
				t.Errorf("expected %d certificates, got %d", tt.wantCount, len(certs))
			}

			if err := mock.ExpectationsWereMet(); err != nil {
				t.Errorf("unfulfilled expectations: %v", err)
			}
		})
	}
}

func TestCertificateRepo_Revoke(t *testing.T) {
	tests := []struct {
		name    string
		id      string
		reason  string
		mockFn  func(mock sqlmock.Sqlmock)
		wantErr bool
		errMsg  string
	}{
		{
			name:   "successful revocation",
			id:     "cert-123",
			reason: "Certificate compromised",
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(`UPDATE certificates SET revoked_at = .+ WHERE id = \$1`).
					WithArgs("cert-123", sqlmock.AnyArg(), "Certificate compromised").
					WillReturnResult(sqlmock.NewResult(0, 1))
			},
			wantErr: false,
		},
		{
			name:   "certificate not found",
			id:     "nonexistent",
			reason: "Test",
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(`UPDATE certificates SET revoked_at = .+ WHERE id = \$1`).
					WithArgs("nonexistent", sqlmock.AnyArg(), "Test").
					WillReturnResult(sqlmock.NewResult(0, 0))
			},
			wantErr: true,
			errMsg:  "not found or already revoked",
		},
		{
			name:   "already revoked",
			id:     "cert-revoked",
			reason: "Second revocation",
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(`UPDATE certificates SET revoked_at = .+ WHERE id = \$1`).
					WithArgs("cert-revoked", sqlmock.AnyArg(), "Second revocation").
					WillReturnResult(sqlmock.NewResult(0, 0))
			},
			wantErr: true,
			errMsg:  "not found or already revoked",
		},
		{
			name:   "database error",
			id:     "cert-err",
			reason: "Error test",
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(`UPDATE certificates SET revoked_at = .+ WHERE id = \$1`).
					WithArgs("cert-err", sqlmock.AnyArg(), "Error test").
					WillReturnError(errors.New("database locked"))
			},
			wantErr: true,
			errMsg:  "revoking certificate",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatalf("failed to create sqlmock: %v", err)
			}
			defer db.Close()

			repo := NewCertificateRepo(db)
			tt.mockFn(mock)

			err = repo.Revoke(context.Background(), tt.id, tt.reason)

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

func TestCertificateRepo_GetWithDetailsByVerificationCode(t *testing.T) {
	fixedTime := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)
	completedAt := fixedTime.Add(-24 * time.Hour)

	tests := []struct {
		name    string
		code    string
		mockFn  func(mock sqlmock.Sqlmock)
		wantNil bool
		wantErr bool
	}{
		{
			name: "certificate with all details",
			code: "VERIFY-ABC123",
			mockFn: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{
					"id", "enrollment_id", "user_id", "pathway_id",
					"verification_code", "issued_at", "expires_at",
					"revoked_at", "revocation_reason", "created_at",
					"pathway_name", "user_name", "user_email",
					"earned_points", "max_points", "percentage", "completed_at",
				}).AddRow(
					"cert-123", "enroll-456", "user-789", "pathway-001",
					"VERIFY-ABC123", fixedTime, nil,
					nil, nil, fixedTime,
					"Linux Fundamentals", "John Doe", "john@example.com",
					950, 1000, 95.0, completedAt,
				)
				mock.ExpectQuery(`SELECT .+ FROM certificates c JOIN pathways p`).
					WithArgs("VERIFY-ABC123").
					WillReturnRows(rows)
			},
			wantNil: false,
			wantErr: false,
		},
		{
			name: "certificate not found",
			code: "INVALID-CODE",
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`SELECT .+ FROM certificates c JOIN pathways p`).
					WithArgs("INVALID-CODE").
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

			repo := NewCertificateRepo(db)
			tt.mockFn(mock)

			cert, err := repo.GetWithDetailsByVerificationCode(context.Background(), tt.code)

			if tt.wantErr && err == nil {
				t.Error("expected error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if tt.wantNil && cert != nil {
				t.Errorf("expected nil certificate, got %+v", cert)
			}
			if !tt.wantNil && cert == nil {
				t.Error("expected certificate with details, got nil")
			}
			if cert != nil {
				if cert.PathwayName == "" {
					t.Error("expected pathway name to be set")
				}
				if cert.UserName == "" {
					t.Error("expected user name to be set")
				}
			}

			if err := mock.ExpectationsWereMet(); err != nil {
				t.Errorf("unfulfilled expectations: %v", err)
			}
		})
	}
}

func TestCertificateRepo_ListWithDetailsByUserID(t *testing.T) {
	fixedTime := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)
	completedAt := fixedTime.Add(-24 * time.Hour)

	tests := []struct {
		name      string
		userID    string
		mockFn    func(mock sqlmock.Sqlmock)
		wantCount int
		wantErr   bool
	}{
		{
			name:   "user with certificates and details",
			userID: "user-789",
			mockFn: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{
					"id", "enrollment_id", "user_id", "pathway_id",
					"verification_code", "issued_at", "expires_at",
					"revoked_at", "revocation_reason", "created_at",
					"pathway_name", "user_name", "user_email",
					"earned_points", "max_points", "percentage", "completed_at",
				}).
					AddRow("cert-1", "enroll-1", "user-789", "pathway-1", "CODE-1", fixedTime, nil, nil, nil, fixedTime, "Linux Fundamentals", "John Doe", "john@example.com", 950, 1000, 95.0, completedAt).
					AddRow("cert-2", "enroll-2", "user-789", "pathway-2", "CODE-2", fixedTime, nil, nil, nil, fixedTime, "Python Basics", "John Doe", "john@example.com", 800, 1000, 80.0, completedAt)
				mock.ExpectQuery(`SELECT .+ FROM certificates c JOIN pathways p`).
					WithArgs("user-789").
					WillReturnRows(rows)
			},
			wantCount: 2,
			wantErr:   false,
		},
		{
			name:   "user with no certificates",
			userID: "user-empty",
			mockFn: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{
					"id", "enrollment_id", "user_id", "pathway_id",
					"verification_code", "issued_at", "expires_at",
					"revoked_at", "revocation_reason", "created_at",
					"pathway_name", "user_name", "user_email",
					"earned_points", "max_points", "percentage", "completed_at",
				})
				mock.ExpectQuery(`SELECT .+ FROM certificates c JOIN pathways p`).
					WithArgs("user-empty").
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

			repo := NewCertificateRepo(db)
			tt.mockFn(mock)

			certs, err := repo.ListWithDetailsByUserID(context.Background(), tt.userID)

			if tt.wantErr && err == nil {
				t.Error("expected error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if !tt.wantErr && len(certs) != tt.wantCount {
				t.Errorf("expected %d certificates, got %d", tt.wantCount, len(certs))
			}

			if err := mock.ExpectationsWereMet(); err != nil {
				t.Errorf("unfulfilled expectations: %v", err)
			}
		})
	}
}
