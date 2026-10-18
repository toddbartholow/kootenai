package simulation

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"

	"github.com/toddbartholow/kootenai/api/internal/models"
)

// -----------------------------------------------------------------------------
// Service Constructor Tests
// -----------------------------------------------------------------------------

func TestNewService(t *testing.T) {
	db, _, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	logger := slog.Default()
	service := NewService(db, nil, nil, nil, logger)

	if service == nil {
		t.Fatal("expected non-nil service")
	}
	if service.db != db {
		t.Error("expected db to be set")
	}
	if service.logger != logger {
		t.Error("expected logger to be set")
	}
}

// -----------------------------------------------------------------------------
// CreateTestStudent Tests
// -----------------------------------------------------------------------------

func TestService_CreateTestStudent(t *testing.T) {
	tests := []struct {
		name    string
		email   string
		mockFn  func(mock sqlmock.Sqlmock)
		wantErr bool
		errMsg  string
	}{
		{
			name:  "success",
			email: "test@example.com",
			mockFn: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{
					"id", "email", "display_name", "role", "is_active", "created_at", "updated_at",
				}).AddRow(
					"user-123", "test@example.com", "Test Student", "student", true,
					time.Now(), time.Now(),
				)
				mock.ExpectQuery(`INSERT INTO users`).
					WillReturnRows(rows)
			},
			wantErr: false,
		},
		{
			name:  "database error",
			email: "test@example.com",
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`INSERT INTO users`).
					WillReturnError(errors.New("insert failed"))
			},
			wantErr: true,
			errMsg:  "creating test student",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatalf("failed to create sqlmock: %v", err)
			}
			defer db.Close()

			service := NewService(db, nil, nil, nil, slog.Default())
			tt.mockFn(mock)

			result, err := service.CreateTestStudent(context.Background(), tt.name, tt.email)

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
				if result == nil {
					t.Error("expected non-nil user")
				}
			}

			if err := mock.ExpectationsWereMet(); err != nil {
				t.Errorf("unfulfilled expectations: %v", err)
			}
		})
	}
}

// -----------------------------------------------------------------------------
// ListTestStudents Tests
// -----------------------------------------------------------------------------

func TestService_ListTestStudents(t *testing.T) {
	fixedTime := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)

	tests := []struct {
		name      string
		mockFn    func(mock sqlmock.Sqlmock)
		wantCount int
		wantErr   bool
	}{
		{
			name: "list students successfully",
			mockFn: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{
					"id", "email", "display_name", "role", "is_active", "created_at", "updated_at",
				}).
					AddRow("user-1", "test1@example.com", "Test Student 1", "student", true, fixedTime, fixedTime).
					AddRow("user-2", "test2@example.com", "Test Student 2", "student", true, fixedTime, fixedTime)
				mock.ExpectQuery(`SELECT id, email, display_name, role, is_active, created_at, updated_at FROM users WHERE`).
					WillReturnRows(rows)
			},
			wantCount: 2,
			wantErr:   false,
		},
		{
			name: "empty list",
			mockFn: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{
					"id", "email", "display_name", "role", "is_active", "created_at", "updated_at",
				})
				mock.ExpectQuery(`SELECT id, email, display_name, role, is_active, created_at, updated_at FROM users WHERE`).
					WillReturnRows(rows)
			},
			wantCount: 0,
			wantErr:   false,
		},
		{
			name: "database error",
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`SELECT id, email, display_name, role, is_active, created_at, updated_at FROM users WHERE`).
					WillReturnError(errors.New("query failed"))
			},
			wantCount: 0,
			wantErr:   true,
		},
		{
			name: "scan error",
			mockFn: func(mock sqlmock.Sqlmock) {
				// Return wrong number of columns to cause scan error
				rows := sqlmock.NewRows([]string{"id"}).AddRow("user-1")
				mock.ExpectQuery(`SELECT id, email, display_name, role, is_active, created_at, updated_at FROM users WHERE`).
					WillReturnRows(rows)
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

			service := NewService(db, nil, nil, nil, slog.Default())
			tt.mockFn(mock)

			result, err := service.ListTestStudents(context.Background())

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if len(result) != tt.wantCount {
					t.Errorf("expected %d students, got %d", tt.wantCount, len(result))
				}
			}

			if err := mock.ExpectationsWereMet(); err != nil {
				t.Errorf("unfulfilled expectations: %v", err)
			}
		})
	}
}

// -----------------------------------------------------------------------------
// calculateScore Tests
// -----------------------------------------------------------------------------

func TestService_calculateScore(t *testing.T) {
	service := NewService(nil, nil, nil, nil, slog.Default())

	tests := []struct {
		name      string
		maxPoints int
		profile   StudentProfile
		minPct    float64
		maxPct    float64
	}{
		{
			name:      "excellent profile",
			maxPoints: 100,
			profile:   ProfileExcellent,
			minPct:    0.95,
			maxPct:    1.0,
		},
		{
			name:      "good profile",
			maxPoints: 100,
			profile:   ProfileGood,
			minPct:    0.80,
			maxPct:    0.94,
		},
		{
			name:      "average profile",
			maxPoints: 100,
			profile:   ProfileAverage,
			minPct:    0.65,
			maxPct:    0.79,
		},
		{
			name:      "struggling profile",
			maxPoints: 100,
			profile:   ProfileStruggling,
			minPct:    0.50,
			maxPct:    0.70,
		},
		{
			name:      "unknown profile - uses default",
			maxPoints: 100,
			profile:   StudentProfile("unknown"),
			minPct:    0.70,
			maxPct:    0.85,
		},
		{
			name:      "different max points",
			maxPoints: 50,
			profile:   ProfileExcellent,
			// Note: with small max points (50), integer truncation can cause scores
			// at the edge of the range to round down, so we test with wider bounds
			minPct: 0.94, // Allow for int truncation (47/50 = 0.94)
			maxPct: 1.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Run multiple times to test randomness
			for i := 0; i < 10; i++ {
				score, max := service.calculateScore(tt.maxPoints, tt.profile)

				if max != tt.maxPoints {
					t.Errorf("expected max=%d, got %d", tt.maxPoints, max)
				}

				pct := float64(score) / float64(max)
				if pct < tt.minPct || pct > tt.maxPct {
					t.Errorf("score %d (%.2f%%) outside expected range [%.2f%%, %.2f%%]",
						score, pct*100, tt.minPct*100, tt.maxPct*100)
				}
			}
		})
	}
}

// -----------------------------------------------------------------------------
// recordSimulatedLabProgress Tests
// -----------------------------------------------------------------------------

func TestService_recordSimulatedLabProgress(t *testing.T) {
	tests := []struct {
		name         string
		enrollmentID string
		moduleID     string
		labID        string
		score        int
		maxPoints    int
		passed       bool
		mockFn       func(mock sqlmock.Sqlmock)
		wantErr      bool
	}{
		{
			name:         "success - passed lab",
			enrollmentID: "enrollment-1",
			moduleID:     "module-1",
			labID:        "lab-1",
			score:        85,
			maxPoints:    100,
			passed:       true,
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(`INSERT INTO lab_progress`).
					WithArgs("enrollment-1", "module-1", "lab-1", 85, 100, true, sqlmock.AnyArg()).
					WillReturnResult(sqlmock.NewResult(0, 1))
				mock.ExpectExec(`UPDATE pathway_enrollments SET last_activity_at`).
					WithArgs("enrollment-1", sqlmock.AnyArg()).
					WillReturnResult(sqlmock.NewResult(0, 1))
			},
			wantErr: false,
		},
		{
			name:         "success - failed lab",
			enrollmentID: "enrollment-1",
			moduleID:     "module-1",
			labID:        "lab-1",
			score:        50,
			maxPoints:    100,
			passed:       false,
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(`INSERT INTO lab_progress`).
					WithArgs("enrollment-1", "module-1", "lab-1", 50, 100, false, sqlmock.AnyArg()).
					WillReturnResult(sqlmock.NewResult(0, 1))
				mock.ExpectExec(`UPDATE pathway_enrollments SET last_activity_at`).
					WithArgs("enrollment-1", sqlmock.AnyArg()).
					WillReturnResult(sqlmock.NewResult(0, 1))
			},
			wantErr: false,
		},
		{
			name:         "insert error",
			enrollmentID: "enrollment-1",
			moduleID:     "module-1",
			labID:        "lab-1",
			score:        85,
			maxPoints:    100,
			passed:       true,
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(`INSERT INTO lab_progress`).
					WillReturnError(errors.New("insert failed"))
			},
			wantErr: true,
		},
		{
			name:         "update enrollment error",
			enrollmentID: "enrollment-1",
			moduleID:     "module-1",
			labID:        "lab-1",
			score:        85,
			maxPoints:    100,
			passed:       true,
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(`INSERT INTO lab_progress`).
					WillReturnResult(sqlmock.NewResult(0, 1))
				mock.ExpectExec(`UPDATE pathway_enrollments SET last_activity_at`).
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

			service := NewService(db, nil, nil, nil, slog.Default())
			tt.mockFn(mock)

			err = service.recordSimulatedLabProgress(
				context.Background(),
				tt.enrollmentID, tt.moduleID, tt.labID,
				tt.score, tt.maxPoints, tt.passed,
			)

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
// checkAchievementCriteria Tests
// -----------------------------------------------------------------------------

func TestService_checkAchievementCriteria(t *testing.T) {
	tests := []struct {
		name            string
		userID          string
		achievementType string
		criteria        []byte
		mockFn          func(mock sqlmock.Sqlmock)
		want            bool
		wantErr         bool
	}{
		{
			name:            "lab_completion - met",
			userID:          "user-123",
			achievementType: "lab_completion",
			criteria:        []byte(`{}`),
			mockFn: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{"count"}).AddRow(1)
				mock.ExpectQuery(`SELECT COUNT\(DISTINCT lp.lab_template_id\)`).
					WithArgs("user-123").
					WillReturnRows(rows)
			},
			want:    true,
			wantErr: false,
		},
		{
			name:            "lab_completion - not met",
			userID:          "user-123",
			achievementType: "lab_completion",
			criteria:        []byte(`{}`),
			mockFn: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{"count"}).AddRow(0)
				mock.ExpectQuery(`SELECT COUNT\(DISTINCT lp.lab_template_id\)`).
					WithArgs("user-123").
					WillReturnRows(rows)
			},
			want:    false,
			wantErr: false,
		},
		{
			name:            "lab_completion - database error",
			userID:          "user-123",
			achievementType: "lab_completion",
			criteria:        []byte(`{}`),
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`SELECT COUNT\(DISTINCT lp.lab_template_id\)`).
					WillReturnError(errors.New("query failed"))
			},
			want:    false,
			wantErr: true,
		},
		{
			name:            "milestone - met at 5 labs",
			userID:          "user-123",
			achievementType: "milestone",
			criteria:        []byte(`{}`),
			mockFn: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{"count"}).AddRow(5)
				mock.ExpectQuery(`SELECT COUNT\(DISTINCT lp.lab_template_id\)`).
					WithArgs("user-123").
					WillReturnRows(rows)
			},
			want:    true,
			wantErr: false,
		},
		{
			name:            "milestone - met at 10 labs",
			userID:          "user-123",
			achievementType: "milestone",
			criteria:        []byte(`{}`),
			mockFn: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{"count"}).AddRow(10)
				mock.ExpectQuery(`SELECT COUNT\(DISTINCT lp.lab_template_id\)`).
					WithArgs("user-123").
					WillReturnRows(rows)
			},
			want:    true,
			wantErr: false,
		},
		{
			name:            "milestone - met at 25 labs",
			userID:          "user-123",
			achievementType: "milestone",
			criteria:        []byte(`{}`),
			mockFn: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{"count"}).AddRow(25)
				mock.ExpectQuery(`SELECT COUNT\(DISTINCT lp.lab_template_id\)`).
					WithArgs("user-123").
					WillReturnRows(rows)
			},
			want:    true,
			wantErr: false,
		},
		{
			name:            "milestone - not met",
			userID:          "user-123",
			achievementType: "milestone",
			criteria:        []byte(`{}`),
			mockFn: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{"count"}).AddRow(3)
				mock.ExpectQuery(`SELECT COUNT\(DISTINCT lp.lab_template_id\)`).
					WithArgs("user-123").
					WillReturnRows(rows)
			},
			want:    false,
			wantErr: false,
		},
		{
			name:            "perfect_score - met",
			userID:          "user-123",
			achievementType: "perfect_score",
			criteria:        []byte(`{}`),
			mockFn: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{"count"}).AddRow(1)
				mock.ExpectQuery(`SELECT COUNT\(\*\)`).
					WithArgs("user-123").
					WillReturnRows(rows)
			},
			want:    true,
			wantErr: false,
		},
		{
			name:            "perfect_score - not met",
			userID:          "user-123",
			achievementType: "perfect_score",
			criteria:        []byte(`{}`),
			mockFn: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{"count"}).AddRow(0)
				mock.ExpectQuery(`SELECT COUNT\(\*\)`).
					WithArgs("user-123").
					WillReturnRows(rows)
			},
			want:    false,
			wantErr: false,
		},
		{
			name:            "pathway - met at 2 modules",
			userID:          "user-123",
			achievementType: "pathway",
			criteria:        []byte(`{}`),
			mockFn: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{"completed_modules"}).AddRow(2)
				mock.ExpectQuery(`SELECT COALESCE\(MAX\(completed_modules\), 0\) FROM pathway_enrollments`).
					WithArgs("user-123").
					WillReturnRows(rows)
			},
			want:    true,
			wantErr: false,
		},
		{
			name:            "pathway - met at 6 modules",
			userID:          "user-123",
			achievementType: "pathway",
			criteria:        []byte(`{}`),
			mockFn: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{"completed_modules"}).AddRow(6)
				mock.ExpectQuery(`SELECT COALESCE\(MAX\(completed_modules\), 0\) FROM pathway_enrollments`).
					WithArgs("user-123").
					WillReturnRows(rows)
			},
			want:    true,
			wantErr: false,
		},
		{
			name:            "pathway_completion - met",
			userID:          "user-123",
			achievementType: "pathway_completion",
			criteria:        []byte(`{}`),
			mockFn: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{"count"}).AddRow(1)
				mock.ExpectQuery(`SELECT COUNT\(\*\) FROM pathway_enrollments WHERE user_id = \$1 AND status = 'completed'`).
					WithArgs("user-123").
					WillReturnRows(rows)
			},
			want:    true,
			wantErr: false,
		},
		{
			name:            "pathway_completion - not met",
			userID:          "user-123",
			achievementType: "pathway_completion",
			criteria:        []byte(`{}`),
			mockFn: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{"count"}).AddRow(0)
				mock.ExpectQuery(`SELECT COUNT\(\*\) FROM pathway_enrollments WHERE user_id = \$1 AND status = 'completed'`).
					WithArgs("user-123").
					WillReturnRows(rows)
			},
			want:    false,
			wantErr: false,
		},
		{
			name:            "unknown type - returns false",
			userID:          "user-123",
			achievementType: "unknown_type",
			criteria:        []byte(`{}`),
			mockFn:          func(mock sqlmock.Sqlmock) {},
			want:            false,
			wantErr:         false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatalf("failed to create sqlmock: %v", err)
			}
			defer db.Close()

			service := NewService(db, nil, nil, nil, slog.Default())
			tt.mockFn(mock)

			got, err := service.checkAchievementCriteria(
				context.Background(),
				tt.userID, tt.achievementType, tt.criteria,
			)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if got != tt.want {
					t.Errorf("expected %v, got %v", tt.want, got)
				}
			}

			if err := mock.ExpectationsWereMet(); err != nil {
				t.Errorf("unfulfilled expectations: %v", err)
			}
		})
	}
}

// -----------------------------------------------------------------------------
// awardAchievement Tests
// -----------------------------------------------------------------------------

func TestService_awardAchievement(t *testing.T) {
	tests := []struct {
		name          string
		userID        string
		achievementID string
		mockFn        func(mock sqlmock.Sqlmock)
		wantErr       bool
	}{
		{
			name:          "success",
			userID:        "user-123",
			achievementID: "achievement-1",
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(`INSERT INTO user_achievements`).
					WillReturnResult(sqlmock.NewResult(0, 1))
			},
			wantErr: false,
		},
		{
			name:          "conflict - already awarded",
			userID:        "user-123",
			achievementID: "achievement-1",
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(`INSERT INTO user_achievements`).
					WillReturnResult(sqlmock.NewResult(0, 0)) // No rows affected due to conflict
			},
			wantErr: false, // ON CONFLICT DO NOTHING doesn't return error
		},
		{
			name:          "database error",
			userID:        "user-123",
			achievementID: "achievement-1",
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(`INSERT INTO user_achievements`).
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

			service := NewService(db, nil, nil, nil, slog.Default())
			tt.mockFn(mock)

			err = service.awardAchievement(context.Background(), tt.userID, tt.achievementID)

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
// ResetStudentProgress Tests
// -----------------------------------------------------------------------------

func TestService_ResetStudentProgress(t *testing.T) {
	tests := []struct {
		name    string
		userID  string
		mockFn  func(mock sqlmock.Sqlmock)
		wantErr bool
		errMsg  string
	}{
		{
			name:   "success - single enrollment",
			userID: "user-123",
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectBegin()
				mock.ExpectExec(`DELETE FROM user_achievements WHERE user_id = \$1`).
					WithArgs("user-123").
					WillReturnResult(sqlmock.NewResult(0, 2))
				rows := sqlmock.NewRows([]string{"id"}).AddRow("enrollment-1")
				mock.ExpectQuery(`SELECT id FROM pathway_enrollments WHERE user_id = \$1`).
					WithArgs("user-123").
					WillReturnRows(rows)
				mock.ExpectExec(`DELETE FROM lab_progress WHERE enrollment_id = \$1`).
					WithArgs("enrollment-1").
					WillReturnResult(sqlmock.NewResult(0, 5))
				mock.ExpectExec(`DELETE FROM module_progress WHERE enrollment_id = \$1`).
					WithArgs("enrollment-1").
					WillReturnResult(sqlmock.NewResult(0, 3))
				mock.ExpectExec(`DELETE FROM pathway_enrollments WHERE user_id = \$1`).
					WithArgs("user-123").
					WillReturnResult(sqlmock.NewResult(0, 1))
				mock.ExpectCommit()
			},
			wantErr: false,
		},
		{
			name:   "success - multiple enrollments",
			userID: "user-123",
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectBegin()
				mock.ExpectExec(`DELETE FROM user_achievements WHERE user_id = \$1`).
					WillReturnResult(sqlmock.NewResult(0, 0))
				rows := sqlmock.NewRows([]string{"id"}).
					AddRow("enrollment-1").
					AddRow("enrollment-2")
				mock.ExpectQuery(`SELECT id FROM pathway_enrollments WHERE user_id = \$1`).
					WillReturnRows(rows)
				// For each enrollment
				mock.ExpectExec(`DELETE FROM lab_progress WHERE enrollment_id = \$1`).
					WithArgs("enrollment-1").
					WillReturnResult(sqlmock.NewResult(0, 3))
				mock.ExpectExec(`DELETE FROM module_progress WHERE enrollment_id = \$1`).
					WithArgs("enrollment-1").
					WillReturnResult(sqlmock.NewResult(0, 2))
				mock.ExpectExec(`DELETE FROM lab_progress WHERE enrollment_id = \$1`).
					WithArgs("enrollment-2").
					WillReturnResult(sqlmock.NewResult(0, 3))
				mock.ExpectExec(`DELETE FROM module_progress WHERE enrollment_id = \$1`).
					WithArgs("enrollment-2").
					WillReturnResult(sqlmock.NewResult(0, 2))
				mock.ExpectExec(`DELETE FROM pathway_enrollments WHERE user_id = \$1`).
					WillReturnResult(sqlmock.NewResult(0, 2))
				mock.ExpectCommit()
			},
			wantErr: false,
		},
		{
			name:   "success - no enrollments",
			userID: "user-123",
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectBegin()
				mock.ExpectExec(`DELETE FROM user_achievements WHERE user_id = \$1`).
					WillReturnResult(sqlmock.NewResult(0, 0))
				rows := sqlmock.NewRows([]string{"id"})
				mock.ExpectQuery(`SELECT id FROM pathway_enrollments WHERE user_id = \$1`).
					WillReturnRows(rows)
				mock.ExpectExec(`DELETE FROM pathway_enrollments WHERE user_id = \$1`).
					WillReturnResult(sqlmock.NewResult(0, 0))
				mock.ExpectCommit()
			},
			wantErr: false,
		},
		{
			name:   "begin transaction error",
			userID: "user-123",
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectBegin().WillReturnError(errors.New("begin failed"))
			},
			wantErr: true,
			errMsg:  "beginning transaction",
		},
		{
			name:   "delete achievements error",
			userID: "user-123",
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectBegin()
				mock.ExpectExec(`DELETE FROM user_achievements WHERE user_id = \$1`).
					WillReturnError(errors.New("delete failed"))
				mock.ExpectRollback()
			},
			wantErr: true,
			errMsg:  "deleting achievements",
		},
		{
			name:   "query enrollments error",
			userID: "user-123",
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectBegin()
				mock.ExpectExec(`DELETE FROM user_achievements WHERE user_id = \$1`).
					WillReturnResult(sqlmock.NewResult(0, 0))
				mock.ExpectQuery(`SELECT id FROM pathway_enrollments WHERE user_id = \$1`).
					WillReturnError(errors.New("query failed"))
				mock.ExpectRollback()
			},
			wantErr: true,
			errMsg:  "querying enrollments",
		},
		{
			name:   "delete lab progress error",
			userID: "user-123",
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectBegin()
				mock.ExpectExec(`DELETE FROM user_achievements WHERE user_id = \$1`).
					WillReturnResult(sqlmock.NewResult(0, 0))
				rows := sqlmock.NewRows([]string{"id"}).AddRow("enrollment-1")
				mock.ExpectQuery(`SELECT id FROM pathway_enrollments WHERE user_id = \$1`).
					WillReturnRows(rows)
				mock.ExpectExec(`DELETE FROM lab_progress WHERE enrollment_id = \$1`).
					WillReturnError(errors.New("delete failed"))
				mock.ExpectRollback()
			},
			wantErr: true,
			errMsg:  "deleting lab progress",
		},
		{
			name:   "delete module progress error",
			userID: "user-123",
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectBegin()
				mock.ExpectExec(`DELETE FROM user_achievements WHERE user_id = \$1`).
					WillReturnResult(sqlmock.NewResult(0, 0))
				rows := sqlmock.NewRows([]string{"id"}).AddRow("enrollment-1")
				mock.ExpectQuery(`SELECT id FROM pathway_enrollments WHERE user_id = \$1`).
					WillReturnRows(rows)
				mock.ExpectExec(`DELETE FROM lab_progress WHERE enrollment_id = \$1`).
					WillReturnResult(sqlmock.NewResult(0, 0))
				mock.ExpectExec(`DELETE FROM module_progress WHERE enrollment_id = \$1`).
					WillReturnError(errors.New("delete failed"))
				mock.ExpectRollback()
			},
			wantErr: true,
			errMsg:  "deleting module progress",
		},
		{
			name:   "delete enrollments error",
			userID: "user-123",
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectBegin()
				mock.ExpectExec(`DELETE FROM user_achievements WHERE user_id = \$1`).
					WillReturnResult(sqlmock.NewResult(0, 0))
				rows := sqlmock.NewRows([]string{"id"})
				mock.ExpectQuery(`SELECT id FROM pathway_enrollments WHERE user_id = \$1`).
					WillReturnRows(rows)
				mock.ExpectExec(`DELETE FROM pathway_enrollments WHERE user_id = \$1`).
					WillReturnError(errors.New("delete failed"))
				mock.ExpectRollback()
			},
			wantErr: true,
			errMsg:  "deleting enrollments",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatalf("failed to create sqlmock: %v", err)
			}
			defer db.Close()

			service := NewService(db, nil, nil, nil, slog.Default())
			tt.mockFn(mock)

			err = service.ResetStudentProgress(context.Background(), tt.userID)

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

// -----------------------------------------------------------------------------
// GetStudentProgress Tests
// -----------------------------------------------------------------------------

func TestService_GetStudentProgress(t *testing.T) {
	fixedTime := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)

	tests := []struct {
		name    string
		userID  string
		mockFn  func(mock sqlmock.Sqlmock)
		wantErr bool
		errMsg  string
		checkFn func(t *testing.T, result map[string]interface{})
	}{
		{
			name:   "success - full progress",
			userID: "user-123",
			mockFn: func(mock sqlmock.Sqlmock) {
				// Get user info
				userRow := sqlmock.NewRows([]string{"display_name", "email"}).
					AddRow("Test User", "test@example.com")
				mock.ExpectQuery(`SELECT display_name, email FROM users WHERE id = \$1`).
					WithArgs("user-123").
					WillReturnRows(userRow)

				// Get achievements
				achievementRows := sqlmock.NewRows([]string{"id", "name", "tier", "points", "earned_at"}).
					AddRow("ach-1", "First Lab", "bronze", 10, fixedTime).
					AddRow("ach-2", "Lab Master", "silver", 25, fixedTime)
				mock.ExpectQuery(`SELECT a.id, a.name, a.tier, a.points, ua.earned_at FROM user_achievements ua`).
					WithArgs("user-123").
					WillReturnRows(achievementRows)

				// Get lab summary
				summaryRow := sqlmock.NewRows([]string{"total", "passed", "points", "max"}).
					AddRow(10, 8, 750, 1000)
				mock.ExpectQuery(`SELECT COUNT\(\*\), COUNT\(\*\) FILTER \(WHERE lp.passed\)`).
					WithArgs("user-123").
					WillReturnRows(summaryRow)
			},
			wantErr: false,
			checkFn: func(t *testing.T, result map[string]interface{}) {
				// Check user
				user, ok := result["user"].(map[string]string)
				if !ok {
					t.Error("expected user to be map[string]string")
					return
				}
				if user["id"] != "user-123" {
					t.Errorf("expected user id 'user-123', got '%s'", user["id"])
				}
				if user["email"] != "test@example.com" {
					t.Errorf("expected email 'test@example.com', got '%s'", user["email"])
				}

				// Check achievements
				achievements, ok := result["achievements"].([]map[string]interface{})
				if !ok {
					t.Error("expected achievements to be []map[string]interface{}")
					return
				}
				if len(achievements) != 2 {
					t.Errorf("expected 2 achievements, got %d", len(achievements))
				}

				// Check lab summary
				labSummary, ok := result["labSummary"].(map[string]interface{})
				if !ok {
					t.Error("expected labSummary to be map[string]interface{}")
					return
				}
				if labSummary["totalLabs"] != 10 {
					t.Errorf("expected totalLabs 10, got %v", labSummary["totalLabs"])
				}
			},
		},
		{
			name:   "user not found",
			userID: "non-existent",
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`SELECT display_name, email FROM users WHERE id = \$1`).
					WithArgs("non-existent").
					WillReturnError(sql.ErrNoRows)
			},
			wantErr: true,
			errMsg:  "getting user",
		},
		{
			name:   "achievements query error",
			userID: "user-123",
			mockFn: func(mock sqlmock.Sqlmock) {
				userRow := sqlmock.NewRows([]string{"display_name", "email"}).
					AddRow("Test User", "test@example.com")
				mock.ExpectQuery(`SELECT display_name, email FROM users WHERE id = \$1`).
					WillReturnRows(userRow)

				mock.ExpectQuery(`SELECT a.id, a.name, a.tier, a.points, ua.earned_at FROM user_achievements ua`).
					WillReturnError(errors.New("query failed"))
			},
			wantErr: true,
			errMsg:  "querying achievements",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatalf("failed to create sqlmock: %v", err)
			}
			defer db.Close()

			// Create a mock enrollment repo that returns empty list
			mockEnrollmentRepo := &mockEnrollmentRepository{}
			service := NewService(db, mockEnrollmentRepo, nil, nil, slog.Default())
			tt.mockFn(mock)

			result, err := service.GetStudentProgress(context.Background(), tt.userID)

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
				if tt.checkFn != nil && result != nil {
					tt.checkFn(t, result)
				}
			}

			if err := mock.ExpectationsWereMet(); err != nil {
				t.Errorf("unfulfilled expectations: %v", err)
			}
		})
	}
}

// -----------------------------------------------------------------------------
// StudentProfile Tests
// -----------------------------------------------------------------------------

func TestStudentProfile_Constants(t *testing.T) {
	tests := []struct {
		profile StudentProfile
		value   string
	}{
		{ProfileExcellent, "excellent"},
		{ProfileGood, "good"},
		{ProfileAverage, "average"},
		{ProfileStruggling, "struggling"},
	}

	for _, tt := range tests {
		if string(tt.profile) != tt.value {
			t.Errorf("expected %s, got %s", tt.value, tt.profile)
		}
	}
}

// -----------------------------------------------------------------------------
// SimulationResult Tests
// -----------------------------------------------------------------------------

func TestSimulationResult_Structure(t *testing.T) {
	result := &SimulationResult{
		StudentID:        "student-1",
		StudentName:      "Test Student",
		PathwayID:        "pathway-1",
		PathwayName:      "Test Pathway",
		EnrollmentID:     "enrollment-1",
		Status:           "enrolled",
		ModulesCompleted: 5,
		TotalModules:     10,
		EarnedPoints:     500,
		MaxPoints:        1000,
		Percentage:       50.0,
		Achievements:     []AchievementResult{},
		LabResults:       []LabResult{},
		Duration:         time.Minute,
	}

	if result.StudentID != "student-1" {
		t.Errorf("expected StudentID 'student-1', got '%s'", result.StudentID)
	}
	if result.ModulesCompleted != 5 {
		t.Errorf("expected ModulesCompleted 5, got %d", result.ModulesCompleted)
	}
	if result.Percentage != 50.0 {
		t.Errorf("expected Percentage 50.0, got %f", result.Percentage)
	}
}

// -----------------------------------------------------------------------------
// LabResult Tests
// -----------------------------------------------------------------------------

func TestLabResult_Structure(t *testing.T) {
	now := time.Now()
	result := &LabResult{
		ModuleName:  "Module 1",
		LabName:     "Lab 1",
		Score:       85,
		MaxPoints:   100,
		Percentage:  85.0,
		Passed:      true,
		CompletedAt: now,
	}

	if result.ModuleName != "Module 1" {
		t.Errorf("expected ModuleName 'Module 1', got '%s'", result.ModuleName)
	}
	if result.Score != 85 {
		t.Errorf("expected Score 85, got %d", result.Score)
	}
	if !result.Passed {
		t.Error("expected Passed to be true")
	}
}

// -----------------------------------------------------------------------------
// AchievementResult Tests
// -----------------------------------------------------------------------------

func TestAchievementResult_Structure(t *testing.T) {
	result := &AchievementResult{
		ID:     "ach-1",
		Name:   "First Lab",
		Tier:   "bronze",
		Points: 10,
	}

	if result.ID != "ach-1" {
		t.Errorf("expected ID 'ach-1', got '%s'", result.ID)
	}
	if result.Points != 10 {
		t.Errorf("expected Points 10, got %d", result.Points)
	}
}

// -----------------------------------------------------------------------------
// SimulationConfig Tests
// -----------------------------------------------------------------------------

func TestSimulationConfig_Structure(t *testing.T) {
	config := &SimulationConfig{
		StudentProfile:    ProfileExcellent,
		Speed:             100,
		ModulesToComplete: 5,
	}

	if config.StudentProfile != ProfileExcellent {
		t.Errorf("expected StudentProfile 'excellent', got '%s'", config.StudentProfile)
	}
	if config.Speed != 100 {
		t.Errorf("expected Speed 100, got %d", config.Speed)
	}
	if config.ModulesToComplete != 5 {
		t.Errorf("expected ModulesToComplete 5, got %d", config.ModulesToComplete)
	}
}

// -----------------------------------------------------------------------------
// Helper Types
// -----------------------------------------------------------------------------

// mockEnrollmentRepository is a simple mock for enrollment repository
type mockEnrollmentRepository struct {
	getByUserAndPathwayResult *models.PathwayEnrollment
	getByIDResult             *models.PathwayEnrollment
	finalEnrollment           *models.PathwayEnrollment
	createCalled              bool
	getByIDCallCount          int
}

func (m *mockEnrollmentRepository) Create(ctx context.Context, enrollment *models.PathwayEnrollment) error {
	m.createCalled = true
	return nil
}

func (m *mockEnrollmentRepository) GetByID(ctx context.Context, id string) (*models.PathwayEnrollment, error) {
	m.getByIDCallCount++
	// Return finalEnrollment on subsequent calls (for final state query)
	if m.getByIDCallCount > 1 && m.finalEnrollment != nil {
		return m.finalEnrollment, nil
	}
	return m.getByIDResult, nil
}

func (m *mockEnrollmentRepository) GetByUserAndPathway(ctx context.Context, userID, pathwayID string) (*models.PathwayEnrollment, error) {
	return m.getByUserAndPathwayResult, nil
}

func (m *mockEnrollmentRepository) GetWithProgress(ctx context.Context, id string) (*models.PathwayEnrollment, error) {
	return nil, nil
}

func (m *mockEnrollmentRepository) List(ctx context.Context, opts models.EnrollmentListOptions) ([]*models.PathwayEnrollment, error) {
	return []*models.PathwayEnrollment{}, nil
}

func (m *mockEnrollmentRepository) Update(ctx context.Context, enrollment *models.PathwayEnrollment) error {
	return nil
}

func (m *mockEnrollmentRepository) UpdateStatus(ctx context.Context, id string, status models.EnrollmentStatus) error {
	return nil
}

func (m *mockEnrollmentRepository) Delete(ctx context.Context, id string) error {
	return nil
}

func (m *mockEnrollmentRepository) InitializeModuleProgress(ctx context.Context, enrollmentID, pathwayID string) error {
	return nil
}

func (m *mockEnrollmentRepository) UnlockModule(ctx context.Context, enrollmentID, moduleID string) error {
	return nil
}

func (m *mockEnrollmentRepository) UnlockNextModules(ctx context.Context, enrollmentID, completedModuleID string) ([]string, error) {
	return nil, nil
}

func (m *mockEnrollmentRepository) RecalculateModuleProgress(ctx context.Context, enrollmentID, moduleID string) error {
	return nil
}

func (m *mockEnrollmentRepository) RecalculateEnrollmentProgress(ctx context.Context, enrollmentID string) error {
	return nil
}

func (m *mockEnrollmentRepository) GetModuleProgress(ctx context.Context, enrollmentID, moduleID string) (*models.ModuleProgress, error) {
	return nil, nil
}

func (m *mockEnrollmentRepository) ListModuleProgress(ctx context.Context, enrollmentID string) ([]*models.ModuleProgress, error) {
	return nil, nil
}

func (m *mockEnrollmentRepository) UpdateModuleProgress(ctx context.Context, progress *models.ModuleProgress) error {
	return nil
}

func (m *mockEnrollmentRepository) GetLabProgress(ctx context.Context, enrollmentID, labTemplateID string) (*models.LabProgress, error) {
	return nil, nil
}

func (m *mockEnrollmentRepository) ListLabProgress(ctx context.Context, enrollmentID, moduleID string) ([]*models.LabProgress, error) {
	return nil, nil
}

func (m *mockEnrollmentRepository) UpdateLabProgress(ctx context.Context, progress *models.LabProgress) error {
	return nil
}

func (m *mockEnrollmentRepository) RecordLabAttempt(ctx context.Context, enrollmentID, moduleID, labTemplateID, sessionID string, score int, passed bool) error {
	return nil
}

// -----------------------------------------------------------------------------
// checkAndAwardAchievements Tests
// -----------------------------------------------------------------------------

func TestService_checkAndAwardAchievements(t *testing.T) {
	tests := []struct {
		name    string
		userID  string
		mockFn  func(mock sqlmock.Sqlmock)
		wantLen int
		wantErr bool
	}{
		{
			name:   "success - no achievements to check",
			userID: "user-123",
			mockFn: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{"id", "name", "tier", "points", "criteria", "type"})
				mock.ExpectQuery(`SELECT a.id, a.name, a.tier, a.points, a.criteria, a.type FROM achievements a`).
					WithArgs("user-123").
					WillReturnRows(rows)
			},
			wantLen: 0,
			wantErr: false,
		},
		{
			name:   "success - achievement earned",
			userID: "user-123",
			mockFn: func(mock sqlmock.Sqlmock) {
				// Query achievements
				rows := sqlmock.NewRows([]string{"id", "name", "tier", "points", "criteria", "type"}).
					AddRow("ach-1", "First Lab", "bronze", 10, []byte(`{}`), "lab_completion")
				mock.ExpectQuery(`SELECT a.id, a.name, a.tier, a.points, a.criteria, a.type FROM achievements a`).
					WithArgs("user-123").
					WillReturnRows(rows)

				// Check lab_completion criteria - met
				countRows := sqlmock.NewRows([]string{"count"}).AddRow(1)
				mock.ExpectQuery(`SELECT COUNT\(DISTINCT lp.lab_template_id\)`).
					WithArgs("user-123").
					WillReturnRows(countRows)

				// Award achievement
				mock.ExpectExec(`INSERT INTO user_achievements`).
					WillReturnResult(sqlmock.NewResult(0, 1))
			},
			wantLen: 1,
			wantErr: false,
		},
		{
			name:   "success - achievement criteria not met",
			userID: "user-123",
			mockFn: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{"id", "name", "tier", "points", "criteria", "type"}).
					AddRow("ach-1", "First Lab", "bronze", 10, []byte(`{}`), "lab_completion")
				mock.ExpectQuery(`SELECT a.id, a.name, a.tier, a.points, a.criteria, a.type FROM achievements a`).
					WithArgs("user-123").
					WillReturnRows(rows)

				// Check lab_completion criteria - not met
				countRows := sqlmock.NewRows([]string{"count"}).AddRow(0)
				mock.ExpectQuery(`SELECT COUNT\(DISTINCT lp.lab_template_id\)`).
					WithArgs("user-123").
					WillReturnRows(countRows)
			},
			wantLen: 0,
			wantErr: false,
		},
		{
			name:   "error - query fails",
			userID: "user-123",
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`SELECT a.id, a.name, a.tier, a.points, a.criteria, a.type FROM achievements a`).
					WillReturnError(errors.New("query failed"))
			},
			wantLen: 0,
			wantErr: true,
		},
		{
			name:   "error - scan fails",
			userID: "user-123",
			mockFn: func(mock sqlmock.Sqlmock) {
				// Return wrong number of columns to cause scan error
				rows := sqlmock.NewRows([]string{"id"}).AddRow("ach-1")
				mock.ExpectQuery(`SELECT a.id, a.name, a.tier, a.points, a.criteria, a.type FROM achievements a`).
					WillReturnRows(rows)
			},
			wantLen: 0,
			wantErr: true,
		},
		{
			name:   "success - multiple achievements, some earned",
			userID: "user-123",
			mockFn: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{"id", "name", "tier", "points", "criteria", "type"}).
					AddRow("ach-1", "First Lab", "bronze", 10, []byte(`{}`), "lab_completion").
					AddRow("ach-2", "Lab Master", "silver", 25, []byte(`{}`), "milestone")
				mock.ExpectQuery(`SELECT a.id, a.name, a.tier, a.points, a.criteria, a.type FROM achievements a`).
					WithArgs("user-123").
					WillReturnRows(rows)

				// First achievement - lab_completion - met
				countRows1 := sqlmock.NewRows([]string{"count"}).AddRow(1)
				mock.ExpectQuery(`SELECT COUNT\(DISTINCT lp.lab_template_id\)`).
					WithArgs("user-123").
					WillReturnRows(countRows1)
				mock.ExpectExec(`INSERT INTO user_achievements`).
					WillReturnResult(sqlmock.NewResult(0, 1))

				// Second achievement - milestone (requires 5) - not met
				countRows2 := sqlmock.NewRows([]string{"count"}).AddRow(1)
				mock.ExpectQuery(`SELECT COUNT\(DISTINCT lp.lab_template_id\)`).
					WithArgs("user-123").
					WillReturnRows(countRows2)
			},
			wantLen: 1,
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

			service := NewService(db, nil, nil, nil, slog.Default())
			tt.mockFn(mock)

			result, err := service.checkAndAwardAchievements(context.Background(), tt.userID)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if len(result) != tt.wantLen {
					t.Errorf("expected %d achievements, got %d", tt.wantLen, len(result))
				}
			}

			if err := mock.ExpectationsWereMet(); err != nil {
				t.Errorf("unfulfilled expectations: %v", err)
			}
		})
	}
}

// -----------------------------------------------------------------------------
// simulateLabCompletion Tests
// -----------------------------------------------------------------------------

func TestService_simulateLabCompletion(t *testing.T) {
	tests := []struct {
		name         string
		enrollmentID string
		module       *models.PathwayModule
		lab          *models.ModuleLab
		config       SimulationConfig
		mockFn       func(mock sqlmock.Sqlmock)
		wantPassed   bool
		wantErr      bool
	}{
		{
			name:         "success - excellent profile passes",
			enrollmentID: "enrollment-1",
			module:       &models.PathwayModule{ID: "module-1", Name: "Module 1"},
			lab:          &models.ModuleLab{LabTemplateID: "lab-1", LabName: "Lab 1", LabMaxPoints: 100},
			config:       SimulationConfig{StudentProfile: ProfileExcellent},
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(`INSERT INTO lab_progress`).
					WillReturnResult(sqlmock.NewResult(0, 1))
				mock.ExpectExec(`UPDATE pathway_enrollments SET last_activity_at`).
					WillReturnResult(sqlmock.NewResult(0, 1))
			},
			wantPassed: true,
			wantErr:    false,
		},
		{
			name:         "success - good profile passes",
			enrollmentID: "enrollment-1",
			module:       &models.PathwayModule{ID: "module-1", Name: "Module 1"},
			lab:          &models.ModuleLab{LabTemplateID: "lab-1", LabName: "Lab 1", LabMaxPoints: 100},
			config:       SimulationConfig{StudentProfile: ProfileGood},
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(`INSERT INTO lab_progress`).
					WillReturnResult(sqlmock.NewResult(0, 1))
				mock.ExpectExec(`UPDATE pathway_enrollments SET last_activity_at`).
					WillReturnResult(sqlmock.NewResult(0, 1))
			},
			wantPassed: true,
			wantErr:    false,
		},
		{
			name:         "error - record progress fails",
			enrollmentID: "enrollment-1",
			module:       &models.PathwayModule{ID: "module-1", Name: "Module 1"},
			lab:          &models.ModuleLab{LabTemplateID: "lab-1", LabName: "Lab 1", LabMaxPoints: 100},
			config:       SimulationConfig{StudentProfile: ProfileExcellent},
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(`INSERT INTO lab_progress`).
					WillReturnError(errors.New("insert failed"))
			},
			wantPassed: false,
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatalf("failed to create sqlmock: %v", err)
			}
			defer db.Close()

			service := NewService(db, nil, nil, nil, slog.Default())
			tt.mockFn(mock)

			result, err := service.simulateLabCompletion(
				context.Background(),
				tt.enrollmentID,
				tt.module,
				tt.lab,
				tt.config,
			)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if result == nil {
					t.Fatal("expected non-nil result")
				}
				// Excellent and Good profiles should always pass (scores >= 70%)
				if tt.wantPassed && !result.Passed {
					t.Errorf("expected passed=true, got false (score: %d/%d = %.1f%%)",
						result.Score, result.MaxPoints, result.Percentage)
				}
				if result.ModuleName != tt.module.Name {
					t.Errorf("expected ModuleName %q, got %q", tt.module.Name, result.ModuleName)
				}
				if result.LabName != tt.lab.LabName {
					t.Errorf("expected LabName %q, got %q", tt.lab.LabName, result.LabName)
				}
			}

			if err := mock.ExpectationsWereMet(); err != nil {
				t.Errorf("unfulfilled expectations: %v", err)
			}
		})
	}
}

// Note: SimulatePathwayProgression is difficult to unit test because it expects
// concrete *repositories.PathwayRepo type, not an interface. The existing tests
// cover the helper functions it uses (simulateLabCompletion, checkAndAwardAchievements,
// recordSimulatedLabProgress, calculateScore), providing good coverage of the logic.
// Full integration testing of SimulatePathwayProgression would require a test database.

// -----------------------------------------------------------------------------
// Helper Functions
// -----------------------------------------------------------------------------

func containsString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
