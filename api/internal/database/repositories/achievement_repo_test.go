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
// Achievement CRUD Tests
// -----------------------------------------------------------------------------

func TestAchievementRepo_CreateAchievement(t *testing.T) {
	fixedTime := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)

	tests := []struct {
		name        string
		achievement *models.Achievement
		mockFn      func(mock sqlmock.Sqlmock)
		wantErr     bool
		errMsg      string
	}{
		{
			name: "success",
			achievement: &models.Achievement{
				ID:          "ach-123",
				Name:        "First Lab",
				Description: "Complete your first lab",
				Type:        models.AchievementTypeMilestone,
				Tier:        models.AchievementTierBronze,
				Points:      10,
				IsActive:    true,
				Criteria:    models.AchievementCriteria{TotalLabs: 1},
			},
			mockFn: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{"created_at", "updated_at"}).
					AddRow(fixedTime, fixedTime)
				mock.ExpectQuery(`INSERT INTO achievements`).
					WillReturnRows(rows)
			},
			wantErr: false,
		},
		{
			name: "database error",
			achievement: &models.Achievement{
				ID:   "ach-err",
				Name: "Error Achievement",
			},
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`INSERT INTO achievements`).
					WillReturnError(errors.New("duplicate key"))
			},
			wantErr: true,
			errMsg:  "inserting achievement",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatalf("failed to create sqlmock: %v", err)
			}
			defer db.Close()

			repo := NewAchievementRepo(db)
			tt.mockFn(mock)

			err = repo.CreateAchievement(context.Background(), tt.achievement)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				} else if tt.errMsg != "" && !achievementContainsString(err.Error(), tt.errMsg) {
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

func TestAchievementRepo_GetAchievementByID(t *testing.T) {
	fixedTime := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)
	criteriaJSON := json.RawMessage(`{"type":"lab_count","value":1}`)

	tests := []struct {
		name    string
		id      string
		mockFn  func(mock sqlmock.Sqlmock)
		wantNil bool
		wantErr bool
	}{
		{
			name: "existing achievement",
			id:   "ach-123",
			mockFn: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{
					"id", "name", "description", "type", "tier", "icon_url", "points",
					"is_secret", "is_active", "criteria", "created_at", "updated_at",
				}).AddRow(
					"ach-123", "First Lab", "Complete your first lab",
					"progress", "bronze", "icon.png", 10,
					false, true, criteriaJSON, fixedTime, fixedTime,
				)
				mock.ExpectQuery(`SELECT .+ FROM achievements WHERE id = \$1`).
					WithArgs("ach-123").
					WillReturnRows(rows)
			},
			wantNil: false,
			wantErr: false,
		},
		{
			name: "non-existent achievement",
			id:   "non-existent",
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`SELECT .+ FROM achievements WHERE id = \$1`).
					WithArgs("non-existent").
					WillReturnError(sql.ErrNoRows)
			},
			wantNil: true,
			wantErr: false,
		},
		{
			name: "database error",
			id:   "ach-err",
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`SELECT .+ FROM achievements WHERE id = \$1`).
					WillReturnError(errors.New("connection error"))
			},
			wantNil: true, // nil result when error
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

			repo := NewAchievementRepo(db)
			tt.mockFn(mock)

			result, err := repo.GetAchievementByID(context.Background(), tt.id)

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

func TestAchievementRepo_ListAchievements(t *testing.T) {
	fixedTime := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)
	criteriaJSON := json.RawMessage(`{"type":"lab_count","value":1}`)
	isActive := true

	tests := []struct {
		name      string
		filter    AchievementFilter
		mockFn    func(mock sqlmock.Sqlmock)
		wantCount int
		wantErr   bool
	}{
		{
			name:   "list all",
			filter: AchievementFilter{},
			mockFn: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{
					"id", "name", "description", "type", "tier", "icon_url", "points",
					"is_secret", "is_active", "criteria", "created_at", "updated_at",
				}).AddRow(
					"ach-1", "First Lab", "Complete your first lab",
					"progress", "bronze", "icon1.png", 10,
					false, true, criteriaJSON, fixedTime, fixedTime,
				).AddRow(
					"ach-2", "Speed Demon", "Complete a lab in under 10 minutes",
					"speed", "silver", "icon2.png", 25,
					false, true, criteriaJSON, fixedTime, fixedTime,
				)
				mock.ExpectQuery(`SELECT .+ FROM achievements WHERE`).
					WillReturnRows(rows)
			},
			wantCount: 2,
			wantErr:   false,
		},
		{
			name: "filter by type",
			filter: AchievementFilter{
				Type: models.AchievementTypeMilestone,
			},
			mockFn: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{
					"id", "name", "description", "type", "tier", "icon_url", "points",
					"is_secret", "is_active", "criteria", "created_at", "updated_at",
				}).AddRow(
					"ach-1", "First Lab", "Complete your first lab",
					"progress", "bronze", "icon1.png", 10,
					false, true, criteriaJSON, fixedTime, fixedTime,
				)
				mock.ExpectQuery(`SELECT .+ FROM achievements WHERE`).
					WillReturnRows(rows)
			},
			wantCount: 1,
			wantErr:   false,
		},
		{
			name: "filter by active",
			filter: AchievementFilter{
				IsActive: &isActive,
			},
			mockFn: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{
					"id", "name", "description", "type", "tier", "icon_url", "points",
					"is_secret", "is_active", "criteria", "created_at", "updated_at",
				})
				mock.ExpectQuery(`SELECT .+ FROM achievements WHERE`).
					WillReturnRows(rows)
			},
			wantCount: 0,
			wantErr:   false,
		},
		{
			name:   "database error",
			filter: AchievementFilter{},
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`SELECT .+ FROM achievements WHERE`).
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

			repo := NewAchievementRepo(db)
			tt.mockFn(mock)

			result, err := repo.ListAchievements(context.Background(), tt.filter)

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

func TestAchievementRepo_UpdateAchievement(t *testing.T) {
	tests := []struct {
		name        string
		achievement *models.Achievement
		mockFn      func(mock sqlmock.Sqlmock)
		wantErr     bool
	}{
		{
			name: "success",
			achievement: &models.Achievement{
				ID:          "ach-123",
				Name:        "Updated Achievement",
				Description: "Updated description",
				Type:        models.AchievementTypeMilestone,
				Tier:        models.AchievementTierSilver,
				Points:      20,
				IsActive:    true,
				Criteria:    models.AchievementCriteria{TotalLabs: 5},
			},
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(`UPDATE achievements`).
					WillReturnResult(sqlmock.NewResult(0, 1))
			},
			wantErr: false,
		},
		{
			name: "database error",
			achievement: &models.Achievement{
				ID:   "ach-err",
				Name: "Error Update",
			},
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(`UPDATE achievements`).
					WillReturnError(errors.New("update failed"))
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

			repo := NewAchievementRepo(db)
			tt.mockFn(mock)

			err = repo.UpdateAchievement(context.Background(), tt.achievement)

			if tt.wantErr && err == nil {
				t.Error("expected error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			if err := mock.ExpectationsWereMet(); err != nil {
				t.Errorf("unfulfilled expectations: %v", err)
			}
		})
	}
}

func TestAchievementRepo_DeleteAchievement(t *testing.T) {
	tests := []struct {
		name    string
		id      string
		mockFn  func(mock sqlmock.Sqlmock)
		wantErr bool
	}{
		{
			name: "success",
			id:   "ach-123",
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(`DELETE FROM achievements WHERE id = \$1`).
					WithArgs("ach-123").
					WillReturnResult(sqlmock.NewResult(0, 1))
			},
			wantErr: false,
		},
		{
			name: "database error",
			id:   "ach-err",
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(`DELETE FROM achievements`).
					WillReturnError(errors.New("foreign key constraint"))
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

			repo := NewAchievementRepo(db)
			tt.mockFn(mock)

			err = repo.DeleteAchievement(context.Background(), tt.id)

			if tt.wantErr && err == nil {
				t.Error("expected error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			if err := mock.ExpectationsWereMet(); err != nil {
				t.Errorf("unfulfilled expectations: %v", err)
			}
		})
	}
}

// -----------------------------------------------------------------------------
// User Achievement Tests
// -----------------------------------------------------------------------------

func TestAchievementRepo_AwardAchievement(t *testing.T) {
	fixedTime := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)

	tests := []struct {
		name            string
		userAchievement *models.UserAchievement
		mockFn          func(mock sqlmock.Sqlmock)
		wantErr         bool
	}{
		{
			name: "success",
			userAchievement: &models.UserAchievement{
				ID:            "ua-123",
				UserID:        "user-123",
				AchievementID: "ach-123",
				EarnedAt:      fixedTime,
				Progress:      100.0,
				Notified:      false,
			},
			mockFn: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{"created_at"}).AddRow(fixedTime)
				mock.ExpectQuery(`INSERT INTO user_achievements`).
					WillReturnRows(rows)
			},
			wantErr: false,
		},
		{
			name: "already earned - no error",
			userAchievement: &models.UserAchievement{
				ID:            "ua-dup",
				UserID:        "user-123",
				AchievementID: "ach-123",
				EarnedAt:      fixedTime,
			},
			mockFn: func(mock sqlmock.Sqlmock) {
				// sql.ErrNoRows means ON CONFLICT DO NOTHING happened
				mock.ExpectQuery(`INSERT INTO user_achievements`).
					WillReturnError(sql.ErrNoRows)
			},
			wantErr: false,
		},
		{
			name: "database error",
			userAchievement: &models.UserAchievement{
				ID:            "ua-err",
				UserID:        "user-123",
				AchievementID: "ach-err",
			},
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`INSERT INTO user_achievements`).
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

			repo := NewAchievementRepo(db)
			tt.mockFn(mock)

			err = repo.AwardAchievement(context.Background(), tt.userAchievement)

			if tt.wantErr && err == nil {
				t.Error("expected error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			if err := mock.ExpectationsWereMet(); err != nil {
				t.Errorf("unfulfilled expectations: %v", err)
			}
		})
	}
}

func TestAchievementRepo_GetUserAchievement(t *testing.T) {
	fixedTime := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)

	tests := []struct {
		name          string
		userID        string
		achievementID string
		mockFn        func(mock sqlmock.Sqlmock)
		wantNil       bool
		wantErr       bool
	}{
		{
			name:          "existing user achievement",
			userID:        "user-123",
			achievementID: "ach-123",
			mockFn: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{
					"id", "user_id", "achievement_id", "earned_at",
					"session_id", "progress", "notified", "created_at",
				}).AddRow(
					"ua-123", "user-123", "ach-123", fixedTime,
					"session-123", 100.0, true, fixedTime,
				)
				mock.ExpectQuery(`SELECT .+ FROM user_achievements WHERE user_id = \$1 AND achievement_id = \$2`).
					WithArgs("user-123", "ach-123").
					WillReturnRows(rows)
			},
			wantNil: false,
			wantErr: false,
		},
		{
			name:          "not found",
			userID:        "user-123",
			achievementID: "ach-999",
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`SELECT .+ FROM user_achievements`).
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

			repo := NewAchievementRepo(db)
			tt.mockFn(mock)

			result, err := repo.GetUserAchievement(context.Background(), tt.userID, tt.achievementID)

			if tt.wantErr && err == nil {
				t.Error("expected error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if tt.wantNil && result != nil {
				t.Error("expected nil result")
			}
			if !tt.wantNil && result == nil {
				t.Error("expected non-nil result")
			}

			if err := mock.ExpectationsWereMet(); err != nil {
				t.Errorf("unfulfilled expectations: %v", err)
			}
		})
	}
}

func TestAchievementRepo_ListUserAchievements(t *testing.T) {
	fixedTime := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)

	tests := []struct {
		name      string
		userID    string
		mockFn    func(mock sqlmock.Sqlmock)
		wantCount int
		wantErr   bool
	}{
		{
			name:   "multiple achievements",
			userID: "user-123",
			mockFn: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{
					"id", "user_id", "achievement_id", "earned_at",
					"session_id", "progress", "notified", "created_at",
				}).AddRow(
					"ua-1", "user-123", "ach-1", fixedTime,
					"session-1", 100.0, true, fixedTime,
				).AddRow(
					"ua-2", "user-123", "ach-2", fixedTime,
					nil, 100.0, false, fixedTime,
				)
				mock.ExpectQuery(`SELECT .+ FROM user_achievements WHERE user_id = \$1`).
					WithArgs("user-123").
					WillReturnRows(rows)
			},
			wantCount: 2,
			wantErr:   false,
		},
		{
			name:   "empty result",
			userID: "user-new",
			mockFn: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{
					"id", "user_id", "achievement_id", "earned_at",
					"session_id", "progress", "notified", "created_at",
				})
				mock.ExpectQuery(`SELECT .+ FROM user_achievements`).
					WillReturnRows(rows)
			},
			wantCount: 0,
			wantErr:   false,
		},
		{
			name:   "database error",
			userID: "user-err",
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`SELECT .+ FROM user_achievements`).
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

			repo := NewAchievementRepo(db)
			tt.mockFn(mock)

			result, err := repo.ListUserAchievements(context.Background(), tt.userID)

			if tt.wantErr && err == nil {
				t.Error("expected error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if len(result) != tt.wantCount {
				t.Errorf("expected %d results, got %d", tt.wantCount, len(result))
			}

			if err := mock.ExpectationsWereMet(); err != nil {
				t.Errorf("unfulfilled expectations: %v", err)
			}
		})
	}
}

func TestAchievementRepo_GetRecentAchievements(t *testing.T) {
	fixedTime := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)

	tests := []struct {
		name      string
		limit     int
		mockFn    func(mock sqlmock.Sqlmock)
		wantCount int
		wantErr   bool
	}{
		{
			name:  "success with limit",
			limit: 5,
			mockFn: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{
					"id", "user_id", "achievement_id", "earned_at",
					"session_id", "progress", "notified", "created_at",
				}).AddRow(
					"ua-1", "user-1", "ach-1", fixedTime,
					nil, 100.0, true, fixedTime,
				).AddRow(
					"ua-2", "user-2", "ach-1", fixedTime,
					nil, 100.0, false, fixedTime,
				)
				mock.ExpectQuery(`SELECT .+ FROM user_achievements ORDER BY earned_at DESC LIMIT \$1`).
					WithArgs(5).
					WillReturnRows(rows)
			},
			wantCount: 2,
			wantErr:   false,
		},
		{
			name:  "default limit when zero",
			limit: 0,
			mockFn: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{
					"id", "user_id", "achievement_id", "earned_at",
					"session_id", "progress", "notified", "created_at",
				})
				mock.ExpectQuery(`SELECT .+ FROM user_achievements ORDER BY earned_at DESC LIMIT \$1`).
					WithArgs(10). // default limit
					WillReturnRows(rows)
			},
			wantCount: 0,
			wantErr:   false,
		},
		{
			name:  "database error",
			limit: 5,
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`SELECT .+ FROM user_achievements`).
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

			repo := NewAchievementRepo(db)
			tt.mockFn(mock)

			result, err := repo.GetRecentAchievements(context.Background(), tt.limit)

			if tt.wantErr && err == nil {
				t.Error("expected error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if len(result) != tt.wantCount {
				t.Errorf("expected %d results, got %d", tt.wantCount, len(result))
			}

			if err := mock.ExpectationsWereMet(); err != nil {
				t.Errorf("unfulfilled expectations: %v", err)
			}
		})
	}
}

func TestAchievementRepo_MarkNotified(t *testing.T) {
	tests := []struct {
		name              string
		userAchievementID string
		mockFn            func(mock sqlmock.Sqlmock)
		wantErr           bool
	}{
		{
			name:              "success",
			userAchievementID: "ua-123",
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(`UPDATE user_achievements SET notified = true WHERE id = \$1`).
					WithArgs("ua-123").
					WillReturnResult(sqlmock.NewResult(0, 1))
			},
			wantErr: false,
		},
		{
			name:              "database error",
			userAchievementID: "ua-err",
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(`UPDATE user_achievements`).
					WillReturnError(errors.New("update failed"))
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

			repo := NewAchievementRepo(db)
			tt.mockFn(mock)

			err = repo.MarkNotified(context.Background(), tt.userAchievementID)

			if tt.wantErr && err == nil {
				t.Error("expected error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			if err := mock.ExpectationsWereMet(); err != nil {
				t.Errorf("unfulfilled expectations: %v", err)
			}
		})
	}
}

// -----------------------------------------------------------------------------
// Progress Tests
// -----------------------------------------------------------------------------

func TestAchievementRepo_UpdateProgress(t *testing.T) {
	fixedTime := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)

	tests := []struct {
		name     string
		progress *models.AchievementProgress
		mockFn   func(mock sqlmock.Sqlmock)
		wantErr  bool
	}{
		{
			name: "success",
			progress: &models.AchievementProgress{
				UserID:        "user-123",
				AchievementID: "ach-123",
				Progress:      50.0,
				Current:       5,
				Required:      10,
				Metadata:      map[string]interface{}{"last_lab": "lab-5"},
			},
			mockFn: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{"updated_at"}).AddRow(fixedTime)
				mock.ExpectQuery(`INSERT INTO achievement_progress`).
					WillReturnRows(rows)
			},
			wantErr: false,
		},
		{
			name: "database error",
			progress: &models.AchievementProgress{
				UserID:        "user-err",
				AchievementID: "ach-123",
			},
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`INSERT INTO achievement_progress`).
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

			repo := NewAchievementRepo(db)
			tt.mockFn(mock)

			err = repo.UpdateProgress(context.Background(), tt.progress)

			if tt.wantErr && err == nil {
				t.Error("expected error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			if err := mock.ExpectationsWereMet(); err != nil {
				t.Errorf("unfulfilled expectations: %v", err)
			}
		})
	}
}

func TestAchievementRepo_GetProgress(t *testing.T) {
	fixedTime := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)
	metadataJSON := json.RawMessage(`{"last_lab":"lab-5"}`)

	tests := []struct {
		name          string
		userID        string
		achievementID string
		mockFn        func(mock sqlmock.Sqlmock)
		wantNil       bool
		wantErr       bool
	}{
		{
			name:          "existing progress",
			userID:        "user-123",
			achievementID: "ach-123",
			mockFn: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{
					"user_id", "achievement_id", "progress",
					"current_value", "required_value", "metadata", "updated_at",
				}).AddRow(
					"user-123", "ach-123", 50.0,
					5, 10, metadataJSON, fixedTime,
				)
				mock.ExpectQuery(`SELECT .+ FROM achievement_progress WHERE user_id = \$1 AND achievement_id = \$2`).
					WithArgs("user-123", "ach-123").
					WillReturnRows(rows)
			},
			wantNil: false,
			wantErr: false,
		},
		{
			name:          "not found",
			userID:        "user-123",
			achievementID: "ach-999",
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`SELECT .+ FROM achievement_progress`).
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

			repo := NewAchievementRepo(db)
			tt.mockFn(mock)

			result, err := repo.GetProgress(context.Background(), tt.userID, tt.achievementID)

			if tt.wantErr && err == nil {
				t.Error("expected error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if tt.wantNil && result != nil {
				t.Error("expected nil result")
			}
			if !tt.wantNil && result == nil {
				t.Error("expected non-nil result")
			}

			if err := mock.ExpectationsWereMet(); err != nil {
				t.Errorf("unfulfilled expectations: %v", err)
			}
		})
	}
}

func TestAchievementRepo_ListUserProgress(t *testing.T) {
	fixedTime := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)
	metadataJSON := json.RawMessage(`{}`)

	tests := []struct {
		name      string
		userID    string
		mockFn    func(mock sqlmock.Sqlmock)
		wantCount int
		wantErr   bool
	}{
		{
			name:   "multiple progress",
			userID: "user-123",
			mockFn: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{
					"user_id", "achievement_id", "progress",
					"current_value", "required_value", "metadata", "updated_at",
				}).AddRow(
					"user-123", "ach-1", 75.0,
					15, 20, metadataJSON, fixedTime,
				).AddRow(
					"user-123", "ach-2", 50.0,
					5, 10, metadataJSON, fixedTime,
				)
				mock.ExpectQuery(`SELECT .+ FROM achievement_progress WHERE user_id = \$1`).
					WithArgs("user-123").
					WillReturnRows(rows)
			},
			wantCount: 2,
			wantErr:   false,
		},
		{
			name:   "empty result",
			userID: "user-new",
			mockFn: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{
					"user_id", "achievement_id", "progress",
					"current_value", "required_value", "metadata", "updated_at",
				})
				mock.ExpectQuery(`SELECT .+ FROM achievement_progress`).
					WillReturnRows(rows)
			},
			wantCount: 0,
			wantErr:   false,
		},
		{
			name:   "database error",
			userID: "user-err",
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`SELECT .+ FROM achievement_progress`).
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

			repo := NewAchievementRepo(db)
			tt.mockFn(mock)

			result, err := repo.ListUserProgress(context.Background(), tt.userID)

			if tt.wantErr && err == nil {
				t.Error("expected error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if len(result) != tt.wantCount {
				t.Errorf("expected %d results, got %d", tt.wantCount, len(result))
			}

			if err := mock.ExpectationsWereMet(); err != nil {
				t.Errorf("unfulfilled expectations: %v", err)
			}
		})
	}
}

// -----------------------------------------------------------------------------
// Combined View Tests
// -----------------------------------------------------------------------------

func TestAchievementRepo_ListAchievementsWithProgress(t *testing.T) {
	fixedTime := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)
	criteriaJSON := json.RawMessage(`{"type":"lab_count","value":10}`)
	metadataJSON := json.RawMessage(`{}`)

	tests := []struct {
		name      string
		userID    string
		mockFn    func(mock sqlmock.Sqlmock)
		wantCount int
		wantErr   bool
	}{
		{
			name:   "achievements with mixed progress",
			userID: "user-123",
			mockFn: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{
					"id", "name", "description", "type", "tier", "icon_url", "points",
					"is_secret", "is_active", "criteria", "created_at", "updated_at",
					"earned_at",
					"progress", "current_value", "required_value", "metadata", "progress_updated_at",
				}).AddRow(
					"ach-1", "First Lab", "Complete your first lab",
					"progress", "bronze", "icon1.png", 10,
					false, true, criteriaJSON, fixedTime, fixedTime,
					fixedTime, // earned
					100.0, 10, 10, metadataJSON, fixedTime,
				).AddRow(
					"ach-2", "Lab Master", "Complete 100 labs",
					"progress", "gold", "icon2.png", 100,
					false, true, criteriaJSON, fixedTime, fixedTime,
					nil, // not earned
					50.0, 50, 100, metadataJSON, fixedTime,
				)
				mock.ExpectQuery(`SELECT .+ FROM achievements a LEFT JOIN user_achievements ua`).
					WithArgs("user-123").
					WillReturnRows(rows)
			},
			wantCount: 2,
			wantErr:   false,
		},
		{
			name:   "empty achievements",
			userID: "user-new",
			mockFn: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{
					"id", "name", "description", "type", "tier", "icon_url", "points",
					"is_secret", "is_active", "criteria", "created_at", "updated_at",
					"earned_at",
					"progress", "current_value", "required_value", "metadata", "progress_updated_at",
				})
				mock.ExpectQuery(`SELECT .+ FROM achievements a LEFT JOIN user_achievements ua`).
					WillReturnRows(rows)
			},
			wantCount: 0,
			wantErr:   false,
		},
		{
			name:   "database error",
			userID: "user-err",
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`SELECT .+ FROM achievements a LEFT JOIN user_achievements ua`).
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

			repo := NewAchievementRepo(db)
			tt.mockFn(mock)

			result, err := repo.ListAchievementsWithProgress(context.Background(), tt.userID)

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

// -----------------------------------------------------------------------------
// GetUserAchievementSummary Tests
// -----------------------------------------------------------------------------

func TestAchievementRepo_GetUserAchievementSummary(t *testing.T) {
	fixedTime := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)

	tests := []struct {
		name    string
		userID  string
		mockFn  func(mock sqlmock.Sqlmock)
		wantErr bool
		errMsg  string
	}{
		{
			name:   "success with achievements",
			userID: "user-123",
			mockFn: func(mock sqlmock.Sqlmock) {
				// 1. Count available achievements
				mock.ExpectQuery(`SELECT COUNT\(\*\) FROM achievements WHERE is_active = true`).
					WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(10))

				// 2. Count earned and total points
				mock.ExpectQuery(`SELECT COUNT\(ua.id\), COALESCE\(SUM\(a.points\), 0\)`).
					WithArgs("user-123").
					WillReturnRows(sqlmock.NewRows([]string{"count", "sum"}).AddRow(5, 150))

				// 3. Recent achievements
				recentRows := sqlmock.NewRows([]string{
					"id", "user_id", "achievement_id", "earned_at", "session_id", "progress", "notified", "created_at",
				}).AddRow(
					"ua-1", "user-123", "ach-1", fixedTime, "sess-1", 100.0, true, fixedTime,
				)
				mock.ExpectQuery(`SELECT id, user_id, achievement_id, earned_at, session_id, progress, notified, created_at FROM user_achievements WHERE user_id = \$1`).
					WithArgs("user-123").
					WillReturnRows(recentRows)

				// 4. Rarity breakdown
				rarityRows := sqlmock.NewRows([]string{"tier", "count"}).
					AddRow("bronze", 3).
					AddRow("silver", 2)
				mock.ExpectQuery(`SELECT a.tier, COUNT\(ua.id\) FROM user_achievements ua JOIN achievements a`).
					WithArgs("user-123").
					WillReturnRows(rarityRows)

				// 5. Type breakdown
				typeRows := sqlmock.NewRows([]string{"type", "count"}).
					AddRow("milestone", 3).
					AddRow("speed", 2)
				mock.ExpectQuery(`SELECT a.type, COUNT\(ua.id\) FROM user_achievements ua JOIN achievements a`).
					WithArgs("user-123").
					WillReturnRows(typeRows)
			},
			wantErr: false,
		},
		{
			name:   "error counting available achievements",
			userID: "user-123",
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`SELECT COUNT\(\*\) FROM achievements WHERE is_active = true`).
					WillReturnError(errors.New("db error"))
			},
			wantErr: true,
			errMsg:  "counting available achievements",
		},
		{
			name:   "error counting earned achievements",
			userID: "user-123",
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`SELECT COUNT\(\*\) FROM achievements WHERE is_active = true`).
					WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(10))
				mock.ExpectQuery(`SELECT COUNT\(ua.id\), COALESCE\(SUM\(a.points\), 0\)`).
					WillReturnError(errors.New("db error"))
			},
			wantErr: true,
			errMsg:  "counting earned achievements",
		},
		{
			name:   "error querying recent achievements",
			userID: "user-123",
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`SELECT COUNT\(\*\) FROM achievements WHERE is_active = true`).
					WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(10))
				mock.ExpectQuery(`SELECT COUNT\(ua.id\), COALESCE\(SUM\(a.points\), 0\)`).
					WillReturnRows(sqlmock.NewRows([]string{"count", "sum"}).AddRow(5, 150))
				mock.ExpectQuery(`SELECT id, user_id, achievement_id`).
					WillReturnError(errors.New("db error"))
			},
			wantErr: true,
			errMsg:  "querying recent achievements",
		},
		{
			name:   "error querying rarity breakdown",
			userID: "user-123",
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`SELECT COUNT\(\*\) FROM achievements WHERE is_active = true`).
					WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(10))
				mock.ExpectQuery(`SELECT COUNT\(ua.id\), COALESCE\(SUM\(a.points\), 0\)`).
					WillReturnRows(sqlmock.NewRows([]string{"count", "sum"}).AddRow(5, 150))
				mock.ExpectQuery(`SELECT id, user_id, achievement_id`).
					WillReturnRows(sqlmock.NewRows([]string{
						"id", "user_id", "achievement_id", "earned_at", "session_id", "progress", "notified", "created_at",
					}))
				mock.ExpectQuery(`SELECT a.tier, COUNT\(ua.id\)`).
					WillReturnError(errors.New("db error"))
			},
			wantErr: true,
			errMsg:  "querying rarity breakdown",
		},
		{
			name:   "error querying type breakdown",
			userID: "user-123",
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`SELECT COUNT\(\*\) FROM achievements WHERE is_active = true`).
					WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(10))
				mock.ExpectQuery(`SELECT COUNT\(ua.id\), COALESCE\(SUM\(a.points\), 0\)`).
					WillReturnRows(sqlmock.NewRows([]string{"count", "sum"}).AddRow(5, 150))
				mock.ExpectQuery(`SELECT id, user_id, achievement_id`).
					WillReturnRows(sqlmock.NewRows([]string{
						"id", "user_id", "achievement_id", "earned_at", "session_id", "progress", "notified", "created_at",
					}))
				mock.ExpectQuery(`SELECT a.tier, COUNT\(ua.id\)`).
					WillReturnRows(sqlmock.NewRows([]string{"tier", "count"}))
				mock.ExpectQuery(`SELECT a.type, COUNT\(ua.id\)`).
					WillReturnError(errors.New("db error"))
			},
			wantErr: true,
			errMsg:  "querying type breakdown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatalf("failed to create sqlmock: %v", err)
			}
			defer db.Close()

			repo := NewAchievementRepo(db)
			tt.mockFn(mock)

			result, err := repo.GetUserAchievementSummary(context.Background(), tt.userID)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				} else if tt.errMsg != "" && !achievementContainsString(err.Error(), tt.errMsg) {
					t.Errorf("error %q should contain %q", err.Error(), tt.errMsg)
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if result == nil {
					t.Error("expected non-nil result")
				}
			}

			if err := mock.ExpectationsWereMet(); err != nil {
				t.Errorf("unfulfilled expectations: %v", err)
			}
		})
	}
}

// -----------------------------------------------------------------------------
// ListAllUserAchievements Tests
// -----------------------------------------------------------------------------

func TestAchievementRepo_ListAllUserAchievements(t *testing.T) {
	fixedTime := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)

	tests := []struct {
		name      string
		mockFn    func(mock sqlmock.Sqlmock)
		wantCount int
		wantErr   bool
	}{
		{
			name: "success with multiple achievements",
			mockFn: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{
					"id", "user_id", "achievement_id", "earned_at", "session_id", "progress", "notified", "created_at",
				}).AddRow(
					"ua-1", "user-1", "ach-1", fixedTime, "sess-1", 100.0, true, fixedTime,
				).AddRow(
					"ua-2", "user-2", "ach-2", fixedTime, "sess-2", 100.0, false, fixedTime,
				)
				mock.ExpectQuery(`SELECT id, user_id, achievement_id, earned_at, session_id, progress, notified, created_at FROM user_achievements ORDER BY earned_at DESC`).
					WillReturnRows(rows)
			},
			wantCount: 2,
			wantErr:   false,
		},
		{
			name: "empty result",
			mockFn: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{
					"id", "user_id", "achievement_id", "earned_at", "session_id", "progress", "notified", "created_at",
				})
				mock.ExpectQuery(`SELECT id, user_id, achievement_id, earned_at, session_id, progress, notified, created_at FROM user_achievements ORDER BY earned_at DESC`).
					WillReturnRows(rows)
			},
			wantCount: 0,
			wantErr:   false,
		},
		{
			name: "database error",
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`SELECT id, user_id, achievement_id`).
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

			repo := NewAchievementRepo(db)
			tt.mockFn(mock)

			result, err := repo.ListAllUserAchievements(context.Background())

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

// -----------------------------------------------------------------------------
// Helper Tests
// -----------------------------------------------------------------------------

func TestNewAchievementRepo(t *testing.T) {
	db, _, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	repo := NewAchievementRepo(db)
	if repo == nil {
		t.Error("expected non-nil repository")
	}
}

// -----------------------------------------------------------------------------
// Helper Functions
// -----------------------------------------------------------------------------

func achievementContainsString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
