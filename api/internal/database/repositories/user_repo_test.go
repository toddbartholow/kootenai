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

func TestUserRepo_Create(t *testing.T) {
	fixedTime := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)

	tests := []struct {
		name    string
		user    *models.User
		mockFn  func(mock sqlmock.Sqlmock, user *models.User)
		wantErr bool
		errMsg  string
	}{
		{
			name: "success with all fields",
			user: &models.User{
				ID:          "user-123",
				ExternalID:  "uid-123",
				Username:    "testuser",
				Email:       "test@example.com",
				DisplayName: "Test User",
				Role:        "student",
				IsActive:    true,
				Metadata:    map[string]string{"key": "value"},
			},
			mockFn: func(mock sqlmock.Sqlmock, user *models.User) {
				rows := sqlmock.NewRows([]string{"created_at", "updated_at"}).
					AddRow(fixedTime, fixedTime)
				mock.ExpectQuery(`INSERT INTO users`).
					WithArgs(
						user.ID,
						user.ExternalID,
						user.Username,
						user.Email,
						user.DisplayName,
						user.Role,
						user.IsActive,
						sqlmock.AnyArg(), // metadata JSON
					).
					WillReturnRows(rows)
			},
			wantErr: false,
		},
		{
			name: "success with minimal fields",
			user: &models.User{
				ID:         "user-456",
				ExternalID: "uid-456",
				Username:   "minuser",
				Role:       "student",
				IsActive:   true,
			},
			mockFn: func(mock sqlmock.Sqlmock, user *models.User) {
				rows := sqlmock.NewRows([]string{"created_at", "updated_at"}).
					AddRow(fixedTime, fixedTime)
				mock.ExpectQuery(`INSERT INTO users`).
					WillReturnRows(rows)
			},
			wantErr: false,
		},
		{
			name: "database error",
			user: &models.User{
				ID:         "user-err",
				ExternalID: "uid-err",
				Username:   "erruser",
				Role:       "student",
				IsActive:   true,
			},
			mockFn: func(mock sqlmock.Sqlmock, user *models.User) {
				mock.ExpectQuery(`INSERT INTO users`).
					WillReturnError(errors.New("duplicate key"))
			},
			wantErr: true,
			errMsg:  "inserting user",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatalf("failed to create sqlmock: %v", err)
			}
			defer db.Close()

			repo := NewUserRepo(db)
			tt.mockFn(mock, tt.user)

			err = repo.Create(context.Background(), tt.user)

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

func TestUserRepo_GetByID(t *testing.T) {
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
			name: "existing user",
			id:   "user-123",
			mockFn: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{
					"id", "external_id", "username", "email", "display_name", "role", "is_active",
					"created_at", "updated_at", "last_login_at", "metadata",
				}).AddRow(
					"user-123", "uid-123", "testuser", "test@example.com", "Test User", "student", true,
					fixedTime, fixedTime, &fixedTime, []byte(`{"key":"value"}`),
				)
				mock.ExpectQuery(`SELECT .+ FROM users WHERE id = \$1`).
					WithArgs("user-123").
					WillReturnRows(rows)
			},
			wantNil: false,
			wantErr: false,
		},
		{
			name: "user without last login",
			id:   "user-456",
			mockFn: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{
					"id", "external_id", "username", "email", "display_name", "role", "is_active",
					"created_at", "updated_at", "last_login_at", "metadata",
				}).AddRow(
					"user-456", "uid-456", "newuser", "new@example.com", "New User", "student", true,
					fixedTime, fixedTime, nil, []byte(`{}`),
				)
				mock.ExpectQuery(`SELECT .+ FROM users WHERE id = \$1`).
					WithArgs("user-456").
					WillReturnRows(rows)
			},
			wantNil: false,
			wantErr: false,
		},
		{
			name: "non-existent user",
			id:   "user-nonexistent",
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`SELECT .+ FROM users WHERE id = \$1`).
					WithArgs("user-nonexistent").
					WillReturnError(sql.ErrNoRows)
			},
			wantNil: true,
			wantErr: false, // UserRepo correctly handles ErrNoRows
		},
		{
			name: "database error",
			id:   "user-err",
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`SELECT .+ FROM users WHERE id = \$1`).
					WithArgs("user-err").
					WillReturnError(errors.New("connection refused"))
			},
			wantNil: true,
			wantErr: true,
			errMsg:  "querying user",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatalf("failed to create sqlmock: %v", err)
			}
			defer db.Close()

			repo := NewUserRepo(db)
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
					t.Error("expected user, got nil")
				}
			}

			if err := mock.ExpectationsWereMet(); err != nil {
				t.Errorf("unfulfilled expectations: %v", err)
			}
		})
	}
}

func TestUserRepo_GetByExternalID(t *testing.T) {
	fixedTime := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)

	tests := []struct {
		name       string
		externalID string
		mockFn     func(mock sqlmock.Sqlmock)
		wantNil    bool
		wantErr    bool
	}{
		{
			name:       "existing user",
			externalID: "uid-123",
			mockFn: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{
					"id", "external_id", "username", "email", "display_name", "role", "is_active",
					"created_at", "updated_at", "last_login_at", "metadata",
				}).AddRow(
					"user-123", "uid-123", "testuser", "test@example.com", "Test User", "student", true,
					fixedTime, fixedTime, nil, []byte(`{}`),
				)
				mock.ExpectQuery(`SELECT .+ FROM users WHERE external_id = \$1`).
					WithArgs("uid-123").
					WillReturnRows(rows)
			},
			wantNil: false,
			wantErr: false,
		},
		{
			name:       "non-existent user",
			externalID: "uid-nonexistent",
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`SELECT .+ FROM users WHERE external_id = \$1`).
					WithArgs("uid-nonexistent").
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

			repo := NewUserRepo(db)
			tt.mockFn(mock)

			got, err := repo.GetByExternalID(context.Background(), tt.externalID)

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
					t.Error("expected user, got nil")
				}
			}

			if err := mock.ExpectationsWereMet(); err != nil {
				t.Errorf("unfulfilled expectations: %v", err)
			}
		})
	}
}

func TestUserRepo_GetByUsername(t *testing.T) {
	fixedTime := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)

	tests := []struct {
		name     string
		username string
		mockFn   func(mock sqlmock.Sqlmock)
		wantNil  bool
		wantErr  bool
	}{
		{
			name:     "existing user",
			username: "testuser",
			mockFn: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{
					"id", "external_id", "username", "email", "display_name", "role", "is_active",
					"created_at", "updated_at", "last_login_at", "metadata",
				}).AddRow(
					"user-123", "uid-123", "testuser", "test@example.com", "Test User", "student", true,
					fixedTime, fixedTime, nil, []byte(`{}`),
				)
				mock.ExpectQuery(`SELECT .+ FROM users WHERE username = \$1`).
					WithArgs("testuser").
					WillReturnRows(rows)
			},
			wantNil: false,
			wantErr: false,
		},
		{
			name:     "non-existent user",
			username: "unknownuser",
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`SELECT .+ FROM users WHERE username = \$1`).
					WithArgs("unknownuser").
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

			repo := NewUserRepo(db)
			tt.mockFn(mock)

			got, err := repo.GetByUsername(context.Background(), tt.username)

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
					t.Error("expected user, got nil")
				}
			}

			if err := mock.ExpectationsWereMet(); err != nil {
				t.Errorf("unfulfilled expectations: %v", err)
			}
		})
	}
}

func TestUserRepo_Update(t *testing.T) {
	tests := []struct {
		name    string
		user    *models.User
		mockFn  func(mock sqlmock.Sqlmock, user *models.User)
		wantErr bool
		errMsg  string
	}{
		{
			name: "success",
			user: &models.User{
				ID:          "user-123",
				ExternalID:  "uid-123",
				Username:    "updateduser",
				Email:       "updated@example.com",
				DisplayName: "Updated User",
				Role:        "instructor",
				IsActive:    true,
				Metadata:    map[string]string{"updated": "true"},
			},
			mockFn: func(mock sqlmock.Sqlmock, user *models.User) {
				mock.ExpectExec(`UPDATE users SET`).
					WithArgs(
						user.ID,
						user.ExternalID,
						user.Username,
						user.Email,
						user.DisplayName,
						user.Role,
						user.IsActive,
						sqlmock.AnyArg(), // metadata JSON
					).
					WillReturnResult(sqlmock.NewResult(0, 1))
			},
			wantErr: false,
		},
		{
			name: "user not found",
			user: &models.User{
				ID:         "user-nonexistent",
				ExternalID: "uid-none",
				Username:   "nouser",
				Role:       "student",
				IsActive:   true,
			},
			mockFn: func(mock sqlmock.Sqlmock, user *models.User) {
				mock.ExpectExec(`UPDATE users SET`).
					WillReturnResult(sqlmock.NewResult(0, 0))
			},
			wantErr: true,
			errMsg:  "user not found",
		},
		{
			name: "database error",
			user: &models.User{
				ID:         "user-err",
				ExternalID: "uid-err",
				Username:   "erruser",
				Role:       "student",
				IsActive:   true,
			},
			mockFn: func(mock sqlmock.Sqlmock, user *models.User) {
				mock.ExpectExec(`UPDATE users SET`).
					WillReturnError(errors.New("connection lost"))
			},
			wantErr: true,
			errMsg:  "updating user",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatalf("failed to create sqlmock: %v", err)
			}
			defer db.Close()

			repo := NewUserRepo(db)
			tt.mockFn(mock, tt.user)

			err = repo.Update(context.Background(), tt.user)

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

func TestUserRepo_UpdateLastLogin(t *testing.T) {
	tests := []struct {
		name    string
		id      string
		mockFn  func(mock sqlmock.Sqlmock)
		wantErr bool
		errMsg  string
	}{
		{
			name: "success",
			id:   "user-123",
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(`UPDATE users SET last_login_at = \$2, updated_at = NOW\(\) WHERE id = \$1`).
					WithArgs("user-123", sqlmock.AnyArg()).
					WillReturnResult(sqlmock.NewResult(0, 1))
			},
			wantErr: false,
		},
		{
			name: "user not found",
			id:   "user-nonexistent",
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(`UPDATE users SET last_login_at = \$2, updated_at = NOW\(\) WHERE id = \$1`).
					WithArgs("user-nonexistent", sqlmock.AnyArg()).
					WillReturnResult(sqlmock.NewResult(0, 0))
			},
			wantErr: true,
			errMsg:  "user not found",
		},
		{
			name: "database error",
			id:   "user-err",
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(`UPDATE users SET last_login_at = \$2, updated_at = NOW\(\) WHERE id = \$1`).
					WithArgs("user-err", sqlmock.AnyArg()).
					WillReturnError(errors.New("database error"))
			},
			wantErr: true,
			errMsg:  "updating last login",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatalf("failed to create sqlmock: %v", err)
			}
			defer db.Close()

			repo := NewUserRepo(db)
			tt.mockFn(mock)

			err = repo.UpdateLastLogin(context.Background(), tt.id)

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

func TestUserRepo_GetByEmailForAuth(t *testing.T) {
	fixedTime := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)

	tests := []struct {
		name    string
		email   string
		mockFn  func(mock sqlmock.Sqlmock)
		wantNil bool
		wantErr bool
		errMsg  string
	}{
		{
			name:  "existing active user with password",
			email: "test@example.com",
			mockFn: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{
					"id", "external_id", "username", "email", "display_name", "role", "is_active",
					"password_hash", "must_change_password", "password_updated_at",
					"created_at", "updated_at", "last_login_at", "metadata",
				}).AddRow(
					"user-123", "uid-123", "testuser", "test@example.com", "Test User", "student", true,
					"$2a$12$hashedpassword", false, fixedTime,
					fixedTime, fixedTime, nil, []byte(`{}`),
				)
				mock.ExpectQuery(`SELECT .+ FROM users WHERE email = \$1 AND is_active = true`).
					WithArgs("test@example.com").
					WillReturnRows(rows)
			},
			wantNil: false,
			wantErr: false,
		},
		{
			name:  "user without password hash",
			email: "nopassword@example.com",
			mockFn: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{
					"id", "external_id", "username", "email", "display_name", "role", "is_active",
					"password_hash", "must_change_password", "password_updated_at",
					"created_at", "updated_at", "last_login_at", "metadata",
				}).AddRow(
					"user-456", "uid-456", "nopassuser", "nopassword@example.com", "No Pass User", "student", true,
					nil, false, nil,
					fixedTime, fixedTime, nil, []byte(`{}`),
				)
				mock.ExpectQuery(`SELECT .+ FROM users WHERE email = \$1 AND is_active = true`).
					WithArgs("nopassword@example.com").
					WillReturnRows(rows)
			},
			wantNil: false,
			wantErr: false,
		},
		{
			name:  "non-existent user",
			email: "nonexistent@example.com",
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`SELECT .+ FROM users WHERE email = \$1 AND is_active = true`).
					WithArgs("nonexistent@example.com").
					WillReturnError(sql.ErrNoRows)
			},
			wantNil: true,
			wantErr: false,
		},
		{
			name:  "database error",
			email: "error@example.com",
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`SELECT .+ FROM users WHERE email = \$1 AND is_active = true`).
					WithArgs("error@example.com").
					WillReturnError(errors.New("connection refused"))
			},
			wantNil: true,
			wantErr: true,
			errMsg:  "querying user for auth",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatalf("failed to create sqlmock: %v", err)
			}
			defer db.Close()

			repo := NewUserRepo(db)
			tt.mockFn(mock)

			got, err := repo.GetByEmailForAuth(context.Background(), tt.email)

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
					t.Error("expected user, got nil")
				}
			}

			if err := mock.ExpectationsWereMet(); err != nil {
				t.Errorf("unfulfilled expectations: %v", err)
			}
		})
	}
}

func TestUserRepo_UpdatePassword(t *testing.T) {
	tests := []struct {
		name       string
		id         string
		hash       string
		mustChange bool
		mockFn     func(mock sqlmock.Sqlmock)
		wantErr    bool
		errMsg     string
	}{
		{
			name:       "success",
			id:         "user-123",
			hash:       "$2a$12$newhash",
			mustChange: false,
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(`UPDATE users SET password_hash = \$2`).
					WithArgs("user-123", "$2a$12$newhash", false).
					WillReturnResult(sqlmock.NewResult(0, 1))
			},
			wantErr: false,
		},
		{
			name:       "success with must_change_password",
			id:         "user-456",
			hash:       "$2a$12$temphash",
			mustChange: true,
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(`UPDATE users SET password_hash = \$2`).
					WithArgs("user-456", "$2a$12$temphash", true).
					WillReturnResult(sqlmock.NewResult(0, 1))
			},
			wantErr: false,
		},
		{
			name:       "user not found",
			id:         "user-nonexistent",
			hash:       "$2a$12$anyhash",
			mustChange: false,
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(`UPDATE users SET password_hash = \$2`).
					WithArgs("user-nonexistent", "$2a$12$anyhash", false).
					WillReturnResult(sqlmock.NewResult(0, 0))
			},
			wantErr: true,
			errMsg:  "user not found",
		},
		{
			name:       "database error",
			id:         "user-err",
			hash:       "$2a$12$hash",
			mustChange: false,
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(`UPDATE users SET password_hash = \$2`).
					WithArgs("user-err", "$2a$12$hash", false).
					WillReturnError(errors.New("database error"))
			},
			wantErr: true,
			errMsg:  "updating password",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatalf("failed to create sqlmock: %v", err)
			}
			defer db.Close()

			repo := NewUserRepo(db)
			tt.mockFn(mock)

			err = repo.UpdatePassword(context.Background(), tt.id, tt.hash, tt.mustChange)

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

func TestUserRepo_ClearMustChangePassword(t *testing.T) {
	tests := []struct {
		name    string
		id      string
		mockFn  func(mock sqlmock.Sqlmock)
		wantErr bool
		errMsg  string
	}{
		{
			name: "success",
			id:   "user-123",
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(`UPDATE users SET must_change_password = false`).
					WithArgs("user-123").
					WillReturnResult(sqlmock.NewResult(0, 1))
			},
			wantErr: false,
		},
		{
			name: "user not found",
			id:   "user-nonexistent",
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(`UPDATE users SET must_change_password = false`).
					WithArgs("user-nonexistent").
					WillReturnResult(sqlmock.NewResult(0, 0))
			},
			wantErr: true,
			errMsg:  "user not found",
		},
		{
			name: "database error",
			id:   "user-err",
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(`UPDATE users SET must_change_password = false`).
					WithArgs("user-err").
					WillReturnError(errors.New("database error"))
			},
			wantErr: true,
			errMsg:  "clearing must_change_password",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatalf("failed to create sqlmock: %v", err)
			}
			defer db.Close()

			repo := NewUserRepo(db)
			tt.mockFn(mock)

			err = repo.ClearMustChangePassword(context.Background(), tt.id)

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

func TestUserRepo_GetByIDWithPassword(t *testing.T) {
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
			name: "existing user with password",
			id:   "user-123",
			mockFn: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{
					"id", "external_id", "username", "email", "display_name", "role", "is_active",
					"password_hash", "must_change_password", "password_updated_at",
					"created_at", "updated_at", "last_login_at", "metadata",
				}).AddRow(
					"user-123", "uid-123", "testuser", "test@example.com", "Test User", "student", true,
					"$2a$12$hashedpassword", true, fixedTime,
					fixedTime, fixedTime, nil, []byte(`{}`),
				)
				mock.ExpectQuery(`SELECT .+ FROM users WHERE id = \$1`).
					WithArgs("user-123").
					WillReturnRows(rows)
			},
			wantNil: false,
			wantErr: false,
		},
		{
			name: "user without password",
			id:   "user-456",
			mockFn: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{
					"id", "external_id", "username", "email", "display_name", "role", "is_active",
					"password_hash", "must_change_password", "password_updated_at",
					"created_at", "updated_at", "last_login_at", "metadata",
				}).AddRow(
					"user-456", "uid-456", "nopassuser", "nopass@example.com", "No Pass User", "student", true,
					nil, false, nil,
					fixedTime, fixedTime, nil, []byte(`{}`),
				)
				mock.ExpectQuery(`SELECT .+ FROM users WHERE id = \$1`).
					WithArgs("user-456").
					WillReturnRows(rows)
			},
			wantNil: false,
			wantErr: false,
		},
		{
			name: "non-existent user",
			id:   "user-nonexistent",
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`SELECT .+ FROM users WHERE id = \$1`).
					WithArgs("user-nonexistent").
					WillReturnError(sql.ErrNoRows)
			},
			wantNil: true,
			wantErr: false,
		},
		{
			name: "database error",
			id:   "user-err",
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`SELECT .+ FROM users WHERE id = \$1`).
					WithArgs("user-err").
					WillReturnError(errors.New("connection refused"))
			},
			wantNil: true,
			wantErr: true,
			errMsg:  "querying user",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatalf("failed to create sqlmock: %v", err)
			}
			defer db.Close()

			repo := NewUserRepo(db)
			tt.mockFn(mock)

			got, err := repo.GetByIDWithPassword(context.Background(), tt.id)

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
					t.Error("expected user, got nil")
				}
			}

			if err := mock.ExpectationsWereMet(); err != nil {
				t.Errorf("unfulfilled expectations: %v", err)
			}
		})
	}
}
