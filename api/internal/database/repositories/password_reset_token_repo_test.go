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
// PasswordResetTokenRepo Tests
// -----------------------------------------------------------------------------

func TestPasswordResetTokenRepo_Create(t *testing.T) {
	fixedTime := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)
	expiresAt := fixedTime.Add(1 * time.Hour)

	tests := []struct {
		name    string
		token   *models.PasswordResetToken
		mockFn  func(mock sqlmock.Sqlmock)
		wantErr bool
		errMsg  string
	}{
		{
			name: "success",
			token: &models.PasswordResetToken{
				ID:        "token-123",
				UserID:    "user-123",
				TokenHash: "hash-abc123",
				ExpiresAt: expiresAt,
				CreatedAt: fixedTime,
			},
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(`INSERT INTO password_reset_tokens`).
					WithArgs("token-123", "user-123", "hash-abc123", expiresAt, fixedTime).
					WillReturnResult(sqlmock.NewResult(0, 1))
			},
			wantErr: false,
		},
		{
			name: "database error",
			token: &models.PasswordResetToken{
				ID:        "token-err",
				UserID:    "user-123",
				TokenHash: "hash-err",
				ExpiresAt: expiresAt,
				CreatedAt: fixedTime,
			},
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(`INSERT INTO password_reset_tokens`).
					WillReturnError(errors.New("insert failed"))
			},
			wantErr: true,
			errMsg:  "creating password reset token",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatalf("failed to create sqlmock: %v", err)
			}
			defer db.Close()

			repo := NewPasswordResetTokenRepo(db)
			tt.mockFn(mock)

			err = repo.Create(context.Background(), tt.token)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				} else if tt.errMsg != "" && !passwordResetContainsString(err.Error(), tt.errMsg) {
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

func TestPasswordResetTokenRepo_GetByTokenHash(t *testing.T) {
	fixedTime := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)
	expiresAt := fixedTime.Add(1 * time.Hour)

	tests := []struct {
		name      string
		tokenHash string
		mockFn    func(mock sqlmock.Sqlmock)
		wantNil   bool
		wantErr   bool
	}{
		{
			name:      "existing token",
			tokenHash: "hash-abc123",
			mockFn: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{
					"id", "user_id", "token_hash", "expires_at", "used_at", "created_at",
				}).AddRow(
					"token-123", "user-123", "hash-abc123", expiresAt, nil, fixedTime,
				)
				mock.ExpectQuery(`SELECT .+ FROM password_reset_tokens WHERE token_hash = \$1`).
					WithArgs("hash-abc123").
					WillReturnRows(rows)
			},
			wantNil: false,
			wantErr: false,
		},
		{
			name:      "used token",
			tokenHash: "hash-used",
			mockFn: func(mock sqlmock.Sqlmock) {
				usedAt := fixedTime.Add(30 * time.Minute)
				rows := sqlmock.NewRows([]string{
					"id", "user_id", "token_hash", "expires_at", "used_at", "created_at",
				}).AddRow(
					"token-123", "user-123", "hash-used", expiresAt, usedAt, fixedTime,
				)
				mock.ExpectQuery(`SELECT .+ FROM password_reset_tokens WHERE token_hash = \$1`).
					WithArgs("hash-used").
					WillReturnRows(rows)
			},
			wantNil: false,
			wantErr: false,
		},
		{
			name:      "non-existent token",
			tokenHash: "non-existent",
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`SELECT .+ FROM password_reset_tokens WHERE token_hash = \$1`).
					WithArgs("non-existent").
					WillReturnError(sql.ErrNoRows)
			},
			wantNil: true,
			wantErr: false,
		},
		{
			name:      "database error",
			tokenHash: "hash-err",
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`SELECT .+ FROM password_reset_tokens WHERE token_hash = \$1`).
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

			repo := NewPasswordResetTokenRepo(db)
			tt.mockFn(mock)

			result, err := repo.GetByTokenHash(context.Background(), tt.tokenHash)

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

func TestPasswordResetTokenRepo_MarkUsed(t *testing.T) {
	tests := []struct {
		name    string
		id      string
		mockFn  func(mock sqlmock.Sqlmock)
		wantErr bool
		errMsg  string
	}{
		{
			name: "success",
			id:   "token-123",
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(`UPDATE password_reset_tokens SET used_at = NOW\(\) WHERE id = \$1`).
					WithArgs("token-123").
					WillReturnResult(sqlmock.NewResult(0, 1))
			},
			wantErr: false,
		},
		{
			name: "token not found",
			id:   "non-existent",
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(`UPDATE password_reset_tokens SET used_at = NOW\(\) WHERE id = \$1`).
					WithArgs("non-existent").
					WillReturnResult(sqlmock.NewResult(0, 0))
			},
			wantErr: true,
			errMsg:  "token not found",
		},
		{
			name: "database error",
			id:   "token-err",
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(`UPDATE password_reset_tokens SET used_at = NOW\(\) WHERE id = \$1`).
					WillReturnError(errors.New("update failed"))
			},
			wantErr: true,
			errMsg:  "marking token used",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatalf("failed to create sqlmock: %v", err)
			}
			defer db.Close()

			repo := NewPasswordResetTokenRepo(db)
			tt.mockFn(mock)

			err = repo.MarkUsed(context.Background(), tt.id)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				} else if tt.errMsg != "" && !passwordResetContainsString(err.Error(), tt.errMsg) {
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

func TestPasswordResetTokenRepo_DeleteByUserID(t *testing.T) {
	tests := []struct {
		name    string
		userID  string
		mockFn  func(mock sqlmock.Sqlmock)
		wantErr bool
		errMsg  string
	}{
		{
			name:   "success - tokens deleted",
			userID: "user-123",
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(`DELETE FROM password_reset_tokens WHERE user_id = \$1`).
					WithArgs("user-123").
					WillReturnResult(sqlmock.NewResult(0, 3))
			},
			wantErr: false,
		},
		{
			name:   "success - no tokens to delete",
			userID: "user-no-tokens",
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(`DELETE FROM password_reset_tokens WHERE user_id = \$1`).
					WithArgs("user-no-tokens").
					WillReturnResult(sqlmock.NewResult(0, 0))
			},
			wantErr: false,
		},
		{
			name:   "database error",
			userID: "user-err",
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(`DELETE FROM password_reset_tokens WHERE user_id = \$1`).
					WillReturnError(errors.New("delete failed"))
			},
			wantErr: true,
			errMsg:  "deleting tokens for user",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatalf("failed to create sqlmock: %v", err)
			}
			defer db.Close()

			repo := NewPasswordResetTokenRepo(db)
			tt.mockFn(mock)

			err = repo.DeleteByUserID(context.Background(), tt.userID)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				} else if tt.errMsg != "" && !passwordResetContainsString(err.Error(), tt.errMsg) {
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

func TestPasswordResetTokenRepo_DeleteExpired(t *testing.T) {
	tests := []struct {
		name      string
		mockFn    func(mock sqlmock.Sqlmock)
		wantCount int64
		wantErr   bool
		errMsg    string
	}{
		{
			name: "success - deleted expired tokens",
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(`DELETE FROM password_reset_tokens WHERE expires_at < \$1`).
					WillReturnResult(sqlmock.NewResult(0, 5))
			},
			wantCount: 5,
			wantErr:   false,
		},
		{
			name: "success - no expired tokens",
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(`DELETE FROM password_reset_tokens WHERE expires_at < \$1`).
					WillReturnResult(sqlmock.NewResult(0, 0))
			},
			wantCount: 0,
			wantErr:   false,
		},
		{
			name: "database error",
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(`DELETE FROM password_reset_tokens WHERE expires_at < \$1`).
					WillReturnError(errors.New("delete failed"))
			},
			wantCount: 0,
			wantErr:   true,
			errMsg:    "deleting expired tokens",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatalf("failed to create sqlmock: %v", err)
			}
			defer db.Close()

			repo := NewPasswordResetTokenRepo(db)
			tt.mockFn(mock)

			count, err := repo.DeleteExpired(context.Background())

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				} else if tt.errMsg != "" && !passwordResetContainsString(err.Error(), tt.errMsg) {
					t.Errorf("error %q should contain %q", err.Error(), tt.errMsg)
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
// Helper Functions
// -----------------------------------------------------------------------------

func TestNewPasswordResetTokenRepo(t *testing.T) {
	db, _, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	repo := NewPasswordResetTokenRepo(db)
	if repo == nil {
		t.Error("expected non-nil repository")
	}
}

func passwordResetContainsString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
