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
// Enrollment CRUD Tests
// -----------------------------------------------------------------------------

func TestEnrollmentRepo_Create(t *testing.T) {
	fixedTime := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)

	tests := []struct {
		name    string
		enroll  *models.PathwayEnrollment
		mockFn  func(mock sqlmock.Sqlmock, enroll *models.PathwayEnrollment)
		wantErr bool
		errMsg  string
	}{
		{
			name: "success",
			enroll: &models.PathwayEnrollment{
				ID:               "enroll-123",
				UserID:           "user-123",
				PathwayID:        "pathway-123",
				Status:           models.EnrollmentStatusEnrolled,
				CompletedModules: 0,
				TotalModules:     5,
				EarnedPoints:     0,
				MaxPoints:        500,
				Percentage:       0,
			},
			mockFn: func(mock sqlmock.Sqlmock, enroll *models.PathwayEnrollment) {
				rows := sqlmock.NewRows([]string{"enrolled_at"}).
					AddRow(fixedTime)
				mock.ExpectQuery(`INSERT INTO pathway_enrollments`).
					WithArgs(
						enroll.ID,
						enroll.UserID,
						enroll.PathwayID,
						enroll.Status,
						enroll.CompletedModules,
						enroll.TotalModules,
						enroll.EarnedPoints,
						enroll.MaxPoints,
						enroll.Percentage,
						sqlmock.AnyArg(), // enrolled_at timestamp
						sqlmock.AnyArg(), // organization_id
					).
					WillReturnRows(rows)
			},
			wantErr: false,
		},
		{
			name: "database error",
			enroll: &models.PathwayEnrollment{
				ID:        "enroll-err",
				UserID:    "user-123",
				PathwayID: "pathway-123",
				Status:    models.EnrollmentStatusEnrolled,
			},
			mockFn: func(mock sqlmock.Sqlmock, enroll *models.PathwayEnrollment) {
				mock.ExpectQuery(`INSERT INTO pathway_enrollments`).
					WillReturnError(errors.New("duplicate key"))
			},
			wantErr: true,
			errMsg:  "inserting enrollment",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatalf("failed to create sqlmock: %v", err)
			}
			defer db.Close()

			repo := NewEnrollmentRepo(db)
			tt.mockFn(mock, tt.enroll)

			err = repo.Create(context.Background(), tt.enroll)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				} else if tt.errMsg != "" && !enrollmentContainsString(err.Error(), tt.errMsg) {
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

func TestEnrollmentRepo_GetByID(t *testing.T) {
	fixedTime := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)

	tests := []struct {
		name    string
		id      string
		mockFn  func(mock sqlmock.Sqlmock)
		wantNil bool
		wantErr bool
	}{
		{
			name: "existing enrollment",
			id:   "enroll-123",
			mockFn: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{
					"id", "user_id", "pathway_id", "status",
					"completed_modules", "total_modules",
					"earned_points", "max_points", "percentage",
					"enrolled_at", "started_at", "completed_at", "last_activity_at",
					"organization_id", "certificate_issued", "certificate_url",
				}).AddRow(
					"enroll-123", "user-123", "pathway-123", "enrolled",
					2, 5,
					100, 500, 20.0,
					fixedTime, &fixedTime, nil, &fixedTime,
					nil, false, nil,
				)
				mock.ExpectQuery(`SELECT .+ FROM pathway_enrollments WHERE id = \$1`).
					WithArgs("enroll-123").
					WillReturnRows(rows)
			},
			wantNil: false,
			wantErr: false,
		},
		{
			name: "non-existent enrollment",
			id:   "non-existent",
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`SELECT .+ FROM pathway_enrollments WHERE id = \$1`).
					WithArgs("non-existent").
					WillReturnError(sql.ErrNoRows)
			},
			wantNil: true,
			wantErr: false,
		},
		{
			name: "database error",
			id:   "enroll-err",
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`SELECT .+ FROM pathway_enrollments WHERE id = \$1`).
					WillReturnError(errors.New("connection error"))
			},
			wantNil: false,
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

			repo := NewEnrollmentRepo(db)
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

func TestEnrollmentRepo_GetByUserAndPathway(t *testing.T) {
	fixedTime := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)

	tests := []struct {
		name      string
		userID    string
		pathwayID string
		mockFn    func(mock sqlmock.Sqlmock)
		wantNil   bool
		wantErr   bool
	}{
		{
			name:      "existing enrollment",
			userID:    "user-123",
			pathwayID: "pathway-123",
			mockFn: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{
					"id", "user_id", "pathway_id", "status",
					"completed_modules", "total_modules",
					"earned_points", "max_points", "percentage",
					"enrolled_at", "started_at", "completed_at", "last_activity_at",
					"organization_id", "certificate_issued", "certificate_url",
				}).AddRow(
					"enroll-123", "user-123", "pathway-123", "in_progress",
					3, 5,
					150, 500, 30.0,
					fixedTime, &fixedTime, nil, &fixedTime,
					nil, false, nil,
				)
				mock.ExpectQuery(`SELECT .+ FROM pathway_enrollments WHERE user_id = \$1 AND pathway_id = \$2`).
					WithArgs("user-123", "pathway-123").
					WillReturnRows(rows)
			},
			wantNil: false,
			wantErr: false,
		},
		{
			name:      "not enrolled",
			userID:    "user-123",
			pathwayID: "pathway-999",
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`SELECT .+ FROM pathway_enrollments WHERE user_id = \$1 AND pathway_id = \$2`).
					WithArgs("user-123", "pathway-999").
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

			repo := NewEnrollmentRepo(db)
			tt.mockFn(mock)

			result, err := repo.GetByUserAndPathway(context.Background(), tt.userID, tt.pathwayID)

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

func TestEnrollmentRepo_Update(t *testing.T) {
	fixedTime := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)

	tests := []struct {
		name    string
		enroll  *models.PathwayEnrollment
		mockFn  func(mock sqlmock.Sqlmock)
		wantErr bool
	}{
		{
			name: "success",
			enroll: &models.PathwayEnrollment{
				ID:               "enroll-123",
				Status:           models.EnrollmentStatusInProgress,
				CompletedModules: 3,
				TotalModules:     5,
				EarnedPoints:     150,
				MaxPoints:        500,
				Percentage:       30.0,
				StartedAt:        &fixedTime,
			},
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(`UPDATE pathway_enrollments`).
					WillReturnResult(sqlmock.NewResult(0, 1))
			},
			wantErr: false,
		},
		{
			name: "database error",
			enroll: &models.PathwayEnrollment{
				ID:     "enroll-err",
				Status: models.EnrollmentStatusInProgress,
			},
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(`UPDATE pathway_enrollments`).
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

			repo := NewEnrollmentRepo(db)
			tt.mockFn(mock)

			err = repo.Update(context.Background(), tt.enroll)

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

func TestEnrollmentRepo_Delete(t *testing.T) {
	tests := []struct {
		name    string
		id      string
		mockFn  func(mock sqlmock.Sqlmock)
		wantErr bool
	}{
		{
			name: "success",
			id:   "enroll-123",
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(`DELETE FROM pathway_enrollments WHERE id = \$1`).
					WithArgs("enroll-123").
					WillReturnResult(sqlmock.NewResult(0, 1))
			},
			wantErr: false,
		},
		{
			name: "database error",
			id:   "enroll-err",
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(`DELETE FROM pathway_enrollments`).
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

			repo := NewEnrollmentRepo(db)
			tt.mockFn(mock)

			err = repo.Delete(context.Background(), tt.id)

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

func TestEnrollmentRepo_UpdateStatus(t *testing.T) {
	tests := []struct {
		name    string
		id      string
		status  models.EnrollmentStatus
		mockFn  func(mock sqlmock.Sqlmock)
		wantErr bool
	}{
		{
			name:   "success",
			id:     "enroll-123",
			status: models.EnrollmentStatusCompleted,
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(`UPDATE pathway_enrollments SET status = \$2, last_activity_at = NOW\(\) WHERE id = \$1`).
					WithArgs("enroll-123", models.EnrollmentStatusCompleted).
					WillReturnResult(sqlmock.NewResult(0, 1))
			},
			wantErr: false,
		},
		{
			name:   "database error",
			id:     "enroll-err",
			status: models.EnrollmentStatusInProgress,
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(`UPDATE pathway_enrollments`).
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

			repo := NewEnrollmentRepo(db)
			tt.mockFn(mock)

			err = repo.UpdateStatus(context.Background(), tt.id, tt.status)

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
// Module Progress Tests
// -----------------------------------------------------------------------------

func TestEnrollmentRepo_GetModuleProgress(t *testing.T) {
	fixedTime := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)

	tests := []struct {
		name         string
		enrollmentID string
		moduleID     string
		mockFn       func(mock sqlmock.Sqlmock)
		wantNil      bool
		wantErr      bool
	}{
		{
			name:         "existing progress",
			enrollmentID: "enroll-123",
			moduleID:     "module-123",
			mockFn: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{
					"id", "enrollment_id", "module_id", "status",
					"completed_labs", "total_labs", "earned_points", "max_points",
					"unlocked_at", "started_at", "completed_at",
				}).AddRow(
					"progress-123", "enroll-123", "module-123", "in_progress",
					2, 4, 50, 100,
					&fixedTime, &fixedTime, nil,
				)
				mock.ExpectQuery(`SELECT .+ FROM module_progress WHERE enrollment_id = \$1 AND module_id = \$2`).
					WithArgs("enroll-123", "module-123").
					WillReturnRows(rows)
			},
			wantNil: false,
			wantErr: false,
		},
		{
			name:         "no progress",
			enrollmentID: "enroll-123",
			moduleID:     "module-999",
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`SELECT .+ FROM module_progress`).
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

			repo := NewEnrollmentRepo(db)
			tt.mockFn(mock)

			result, err := repo.GetModuleProgress(context.Background(), tt.enrollmentID, tt.moduleID)

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

func TestEnrollmentRepo_ListModuleProgress(t *testing.T) {
	fixedTime := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)

	tests := []struct {
		name         string
		enrollmentID string
		mockFn       func(mock sqlmock.Sqlmock)
		wantCount    int
		wantErr      bool
	}{
		{
			name:         "multiple modules",
			enrollmentID: "enroll-123",
			mockFn: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{
					"id", "enrollment_id", "module_id", "status",
					"completed_labs", "total_labs", "earned_points", "max_points",
					"unlocked_at", "started_at", "completed_at",
				}).AddRow(
					"progress-1", "enroll-123", "module-1", "completed",
					4, 4, 100, 100,
					&fixedTime, &fixedTime, &fixedTime,
				).AddRow(
					"progress-2", "enroll-123", "module-2", "in_progress",
					2, 4, 50, 100,
					&fixedTime, &fixedTime, nil,
				).AddRow(
					"progress-3", "enroll-123", "module-3", "locked",
					0, 4, 0, 100,
					nil, nil, nil,
				)
				mock.ExpectQuery(`SELECT .+ FROM module_progress .+ WHERE mp.enrollment_id = \$1`).
					WithArgs("enroll-123").
					WillReturnRows(rows)
			},
			wantCount: 3,
			wantErr:   false,
		},
		{
			name:         "empty progress",
			enrollmentID: "enroll-new",
			mockFn: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{
					"id", "enrollment_id", "module_id", "status",
					"completed_labs", "total_labs", "earned_points", "max_points",
					"unlocked_at", "started_at", "completed_at",
				})
				mock.ExpectQuery(`SELECT .+ FROM module_progress`).
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

			repo := NewEnrollmentRepo(db)
			tt.mockFn(mock)

			result, err := repo.ListModuleProgress(context.Background(), tt.enrollmentID)

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

func TestEnrollmentRepo_UpdateModuleProgress(t *testing.T) {
	fixedTime := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)

	tests := []struct {
		name     string
		progress *models.ModuleProgress
		mockFn   func(mock sqlmock.Sqlmock)
		wantErr  bool
	}{
		{
			name: "success",
			progress: &models.ModuleProgress{
				ID:            "progress-123",
				Status:        "completed",
				CompletedLabs: 4,
				EarnedPoints:  100,
				UnlockedAt:    &fixedTime,
				StartedAt:     &fixedTime,
				CompletedAt:   &fixedTime,
			},
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(`UPDATE module_progress`).
					WillReturnResult(sqlmock.NewResult(0, 1))
			},
			wantErr: false,
		},
		{
			name: "database error",
			progress: &models.ModuleProgress{
				ID:     "progress-err",
				Status: "in_progress",
			},
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(`UPDATE module_progress`).
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

			repo := NewEnrollmentRepo(db)
			tt.mockFn(mock)

			err = repo.UpdateModuleProgress(context.Background(), tt.progress)

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

func TestEnrollmentRepo_UnlockModule(t *testing.T) {
	tests := []struct {
		name         string
		enrollmentID string
		moduleID     string
		mockFn       func(mock sqlmock.Sqlmock)
		wantErr      bool
	}{
		{
			name:         "success",
			enrollmentID: "enroll-123",
			moduleID:     "module-123",
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(`UPDATE module_progress`).
					WithArgs("enroll-123", "module-123").
					WillReturnResult(sqlmock.NewResult(0, 1))
			},
			wantErr: false,
		},
		{
			name:         "already unlocked - no rows affected",
			enrollmentID: "enroll-123",
			moduleID:     "module-unlocked",
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(`UPDATE module_progress`).
					WillReturnResult(sqlmock.NewResult(0, 0)) // No rows affected
			},
			wantErr: false,
		},
		{
			name:         "database error",
			enrollmentID: "enroll-err",
			moduleID:     "module-err",
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(`UPDATE module_progress`).
					WillReturnError(errors.New("unlock failed"))
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

			repo := NewEnrollmentRepo(db)
			tt.mockFn(mock)

			err = repo.UnlockModule(context.Background(), tt.enrollmentID, tt.moduleID)

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
// Lab Progress Tests
// -----------------------------------------------------------------------------

func TestEnrollmentRepo_GetLabProgress(t *testing.T) {
	fixedTime := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)

	tests := []struct {
		name          string
		enrollmentID  string
		labTemplateID string
		mockFn        func(mock sqlmock.Sqlmock)
		wantNil       bool
		wantErr       bool
	}{
		{
			name:          "existing progress",
			enrollmentID:  "enroll-123",
			labTemplateID: "lab-123",
			mockFn: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{
					"id", "enrollment_id", "module_id", "lab_template_id",
					"best_session_id", "attempt_count", "best_score", "max_points",
					"passed", "first_attempt_at", "completed_at",
				}).AddRow(
					"progress-123", "enroll-123", "module-123", "lab-123",
					"session-123", 3, 85, 100,
					true, &fixedTime, &fixedTime,
				)
				mock.ExpectQuery(`SELECT .+ FROM lab_progress WHERE enrollment_id = \$1 AND lab_template_id = \$2`).
					WithArgs("enroll-123", "lab-123").
					WillReturnRows(rows)
			},
			wantNil: false,
			wantErr: false,
		},
		{
			name:          "no progress",
			enrollmentID:  "enroll-123",
			labTemplateID: "lab-999",
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`SELECT .+ FROM lab_progress`).
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

			repo := NewEnrollmentRepo(db)
			tt.mockFn(mock)

			result, err := repo.GetLabProgress(context.Background(), tt.enrollmentID, tt.labTemplateID)

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

func TestEnrollmentRepo_ListLabProgress(t *testing.T) {
	fixedTime := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)

	tests := []struct {
		name         string
		enrollmentID string
		moduleID     string
		mockFn       func(mock sqlmock.Sqlmock)
		wantCount    int
		wantErr      bool
	}{
		{
			name:         "multiple labs",
			enrollmentID: "enroll-123",
			moduleID:     "module-123",
			mockFn: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{
					"id", "enrollment_id", "module_id", "lab_template_id",
					"best_session_id", "attempt_count", "best_score", "max_points",
					"passed", "first_attempt_at", "completed_at",
				}).AddRow(
					"progress-1", "enroll-123", "module-123", "lab-1",
					"session-1", 2, 100, 100,
					true, &fixedTime, &fixedTime,
				).AddRow(
					"progress-2", "enroll-123", "module-123", "lab-2",
					nil, 1, 60, 100,
					false, &fixedTime, nil,
				)
				mock.ExpectQuery(`SELECT .+ FROM lab_progress .+ WHERE lp.enrollment_id = \$1 AND lp.module_id = \$2`).
					WithArgs("enroll-123", "module-123").
					WillReturnRows(rows)
			},
			wantCount: 2,
			wantErr:   false,
		},
		{
			name:         "empty progress",
			enrollmentID: "enroll-new",
			moduleID:     "module-new",
			mockFn: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{
					"id", "enrollment_id", "module_id", "lab_template_id",
					"best_session_id", "attempt_count", "best_score", "max_points",
					"passed", "first_attempt_at", "completed_at",
				})
				mock.ExpectQuery(`SELECT .+ FROM lab_progress`).
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

			repo := NewEnrollmentRepo(db)
			tt.mockFn(mock)

			result, err := repo.ListLabProgress(context.Background(), tt.enrollmentID, tt.moduleID)

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

func TestEnrollmentRepo_UpdateLabProgress(t *testing.T) {
	fixedTime := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)
	sessionID := "session-123"

	tests := []struct {
		name     string
		progress *models.LabProgress
		mockFn   func(mock sqlmock.Sqlmock)
		wantErr  bool
	}{
		{
			name: "success",
			progress: &models.LabProgress{
				ID:             "progress-123",
				BestSessionID:  &sessionID,
				AttemptCount:   3,
				BestScore:      85,
				Passed:         true,
				FirstAttemptAt: &fixedTime,
				CompletedAt:    &fixedTime,
			},
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(`UPDATE lab_progress`).
					WillReturnResult(sqlmock.NewResult(0, 1))
			},
			wantErr: false,
		},
		{
			name: "database error",
			progress: &models.LabProgress{
				ID: "progress-err",
			},
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(`UPDATE lab_progress`).
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

			repo := NewEnrollmentRepo(db)
			tt.mockFn(mock)

			err = repo.UpdateLabProgress(context.Background(), tt.progress)

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
// Progress Calculation Tests
// -----------------------------------------------------------------------------

func TestEnrollmentRepo_RecalculateModuleProgress(t *testing.T) {
	tests := []struct {
		name         string
		enrollmentID string
		moduleID     string
		mockFn       func(mock sqlmock.Sqlmock)
		wantErr      bool
	}{
		{
			name:         "success",
			enrollmentID: "enroll-123",
			moduleID:     "module-123",
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(`UPDATE module_progress mp`).
					WithArgs("enroll-123", "module-123").
					WillReturnResult(sqlmock.NewResult(0, 1))
			},
			wantErr: false,
		},
		{
			name:         "database error",
			enrollmentID: "enroll-err",
			moduleID:     "module-err",
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(`UPDATE module_progress`).
					WillReturnError(errors.New("recalculation failed"))
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

			repo := NewEnrollmentRepo(db)
			tt.mockFn(mock)

			err = repo.RecalculateModuleProgress(context.Background(), tt.enrollmentID, tt.moduleID)

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

func TestEnrollmentRepo_RecalculateEnrollmentProgress(t *testing.T) {
	tests := []struct {
		name         string
		enrollmentID string
		mockFn       func(mock sqlmock.Sqlmock)
		wantErr      bool
	}{
		{
			name:         "success",
			enrollmentID: "enroll-123",
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(`UPDATE pathway_enrollments pe`).
					WithArgs("enroll-123").
					WillReturnResult(sqlmock.NewResult(0, 1))
			},
			wantErr: false,
		},
		{
			name:         "database error",
			enrollmentID: "enroll-err",
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(`UPDATE pathway_enrollments`).
					WillReturnError(errors.New("recalculation failed"))
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

			repo := NewEnrollmentRepo(db)
			tt.mockFn(mock)

			err = repo.RecalculateEnrollmentProgress(context.Background(), tt.enrollmentID)

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
// List Tests
// -----------------------------------------------------------------------------

func TestEnrollmentRepo_List(t *testing.T) {
	fixedTime := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)

	tests := []struct {
		name      string
		opts      models.EnrollmentListOptions
		mockFn    func(mock sqlmock.Sqlmock)
		wantCount int
		wantErr   bool
	}{
		{
			name: "list by user",
			opts: models.EnrollmentListOptions{
				UserID: "user-123",
			},
			mockFn: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{
					"id", "user_id", "pathway_id", "status",
					"completed_modules", "total_modules",
					"earned_points", "max_points", "percentage",
					"enrolled_at", "started_at", "completed_at", "last_activity_at",
					"organization_id", "certificate_issued", "certificate_url",
				}).AddRow(
					"enroll-1", "user-123", "pathway-1", "in_progress",
					2, 5, 100, 500, 20.0,
					fixedTime, &fixedTime, nil, &fixedTime,
					nil, false, nil,
				).AddRow(
					"enroll-2", "user-123", "pathway-2", "completed",
					5, 5, 500, 500, 100.0,
					fixedTime, &fixedTime, &fixedTime, &fixedTime,
					nil, true, "https://certs.example.com/123",
				)
				mock.ExpectQuery(`SELECT .+ FROM pathway_enrollments e WHERE`).
					WillReturnRows(rows)
			},
			wantCount: 2,
			wantErr:   false,
		},
		{
			name: "list by pathway",
			opts: models.EnrollmentListOptions{
				PathwayID: "pathway-123",
				Limit:     10,
			},
			mockFn: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{
					"id", "user_id", "pathway_id", "status",
					"completed_modules", "total_modules",
					"earned_points", "max_points", "percentage",
					"enrolled_at", "started_at", "completed_at", "last_activity_at",
					"organization_id", "certificate_issued", "certificate_url",
				}).AddRow(
					"enroll-1", "user-1", "pathway-123", "in_progress",
					2, 5, 100, 500, 20.0,
					fixedTime, &fixedTime, nil, &fixedTime,
					nil, false, nil,
				)
				mock.ExpectQuery(`SELECT .+ FROM pathway_enrollments e WHERE`).
					WillReturnRows(rows)
			},
			wantCount: 1,
			wantErr:   false,
		},
		{
			name: "empty result",
			opts: models.EnrollmentListOptions{
				UserID: "user-no-enrollments",
			},
			mockFn: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{
					"id", "user_id", "pathway_id", "status",
					"completed_modules", "total_modules",
					"earned_points", "max_points", "percentage",
					"enrolled_at", "started_at", "completed_at", "last_activity_at",
					"organization_id", "certificate_issued", "certificate_url",
				})
				mock.ExpectQuery(`SELECT .+ FROM pathway_enrollments e WHERE`).
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

			repo := NewEnrollmentRepo(db)
			tt.mockFn(mock)

			result, err := repo.List(context.Background(), tt.opts)

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
// Helper Tests
// -----------------------------------------------------------------------------

func TestNewEnrollmentRepo(t *testing.T) {
	db, _, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	repo := NewEnrollmentRepo(db)
	if repo == nil {
		t.Error("expected non-nil repository")
	}
}

// -----------------------------------------------------------------------------
// GetWithProgress Tests
// -----------------------------------------------------------------------------

func TestEnrollmentRepo_GetWithProgress(t *testing.T) {
	fixedTime := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)

	tests := []struct {
		name    string
		id      string
		mockFn  func(mock sqlmock.Sqlmock)
		wantNil bool
		wantErr bool
	}{
		{
			name: "success with progress",
			id:   "enroll-123",
			mockFn: func(mock sqlmock.Sqlmock) {
				// First: GetByID
				enrollRows := sqlmock.NewRows([]string{
					"id", "user_id", "pathway_id", "status",
					"completed_modules", "total_modules",
					"earned_points", "max_points", "percentage",
					"enrolled_at", "started_at", "completed_at", "last_activity_at",
					"organization_id", "certificate_issued", "certificate_url",
				}).AddRow(
					"enroll-123", "user-123", "pathway-123", "in_progress",
					2, 5, 100, 500, 20.0,
					fixedTime, &fixedTime, nil, &fixedTime,
					nil, false, nil,
				)
				mock.ExpectQuery(`SELECT .+ FROM pathway_enrollments WHERE id = \$1`).
					WithArgs("enroll-123").
					WillReturnRows(enrollRows)

				// Second: ListModuleProgress
				moduleRows := sqlmock.NewRows([]string{
					"id", "enrollment_id", "module_id", "status",
					"completed_labs", "total_labs", "earned_points", "max_points",
					"unlocked_at", "started_at", "completed_at",
				}).AddRow(
					"mp-1", "enroll-123", "module-1", "completed",
					4, 4, 100, 100,
					&fixedTime, &fixedTime, &fixedTime,
				).AddRow(
					"mp-2", "enroll-123", "module-2", "in_progress",
					2, 4, 50, 100,
					&fixedTime, &fixedTime, nil,
				)
				mock.ExpectQuery(`SELECT .+ FROM module_progress .+ WHERE mp.enrollment_id = \$1`).
					WithArgs("enroll-123").
					WillReturnRows(moduleRows)

				// Third: ListLabProgress for module-1
				labRows1 := sqlmock.NewRows([]string{
					"id", "enrollment_id", "module_id", "lab_template_id",
					"best_session_id", "attempt_count", "best_score", "max_points",
					"passed", "first_attempt_at", "completed_at",
				}).AddRow(
					"lp-1", "enroll-123", "module-1", "lab-1",
					"session-1", 2, 100, 100,
					true, &fixedTime, &fixedTime,
				)
				mock.ExpectQuery(`SELECT .+ FROM lab_progress .+ WHERE lp.enrollment_id = \$1 AND lp.module_id = \$2`).
					WithArgs("enroll-123", "module-1").
					WillReturnRows(labRows1)

				// Fourth: ListLabProgress for module-2
				labRows2 := sqlmock.NewRows([]string{
					"id", "enrollment_id", "module_id", "lab_template_id",
					"best_session_id", "attempt_count", "best_score", "max_points",
					"passed", "first_attempt_at", "completed_at",
				}).AddRow(
					"lp-2", "enroll-123", "module-2", "lab-2",
					nil, 1, 50, 100,
					false, &fixedTime, nil,
				)
				mock.ExpectQuery(`SELECT .+ FROM lab_progress .+ WHERE lp.enrollment_id = \$1 AND lp.module_id = \$2`).
					WithArgs("enroll-123", "module-2").
					WillReturnRows(labRows2)
			},
			wantNil: false,
			wantErr: false,
		},
		{
			name: "enrollment not found",
			id:   "non-existent",
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`SELECT .+ FROM pathway_enrollments WHERE id = \$1`).
					WithArgs("non-existent").
					WillReturnError(sql.ErrNoRows)
			},
			wantNil: true,
			wantErr: false,
		},
		{
			name: "module progress error",
			id:   "enroll-err",
			mockFn: func(mock sqlmock.Sqlmock) {
				// GetByID succeeds
				enrollRows := sqlmock.NewRows([]string{
					"id", "user_id", "pathway_id", "status",
					"completed_modules", "total_modules",
					"earned_points", "max_points", "percentage",
					"enrolled_at", "started_at", "completed_at", "last_activity_at",
					"organization_id", "certificate_issued", "certificate_url",
				}).AddRow(
					"enroll-err", "user-123", "pathway-123", "enrolled",
					0, 5, 0, 500, 0.0,
					fixedTime, nil, nil, nil,
					nil, false, nil,
				)
				mock.ExpectQuery(`SELECT .+ FROM pathway_enrollments WHERE id = \$1`).
					WithArgs("enroll-err").
					WillReturnRows(enrollRows)

				// ListModuleProgress fails
				mock.ExpectQuery(`SELECT .+ FROM module_progress`).
					WillReturnError(errors.New("database error"))
			},
			wantNil: false,
			wantErr: true,
		},
		{
			name: "lab progress error",
			id:   "enroll-lab-err",
			mockFn: func(mock sqlmock.Sqlmock) {
				// GetByID succeeds
				enrollRows := sqlmock.NewRows([]string{
					"id", "user_id", "pathway_id", "status",
					"completed_modules", "total_modules",
					"earned_points", "max_points", "percentage",
					"enrolled_at", "started_at", "completed_at", "last_activity_at",
					"organization_id", "certificate_issued", "certificate_url",
				}).AddRow(
					"enroll-lab-err", "user-123", "pathway-123", "in_progress",
					1, 5, 50, 500, 10.0,
					fixedTime, &fixedTime, nil, &fixedTime,
					nil, false, nil,
				)
				mock.ExpectQuery(`SELECT .+ FROM pathway_enrollments WHERE id = \$1`).
					WithArgs("enroll-lab-err").
					WillReturnRows(enrollRows)

				// ListModuleProgress succeeds
				moduleRows := sqlmock.NewRows([]string{
					"id", "enrollment_id", "module_id", "status",
					"completed_labs", "total_labs", "earned_points", "max_points",
					"unlocked_at", "started_at", "completed_at",
				}).AddRow(
					"mp-1", "enroll-lab-err", "module-1", "in_progress",
					2, 4, 50, 100,
					&fixedTime, &fixedTime, nil,
				)
				mock.ExpectQuery(`SELECT .+ FROM module_progress`).
					WillReturnRows(moduleRows)

				// ListLabProgress fails
				mock.ExpectQuery(`SELECT .+ FROM lab_progress`).
					WillReturnError(errors.New("lab progress error"))
			},
			wantNil: false,
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

			repo := NewEnrollmentRepo(db)
			tt.mockFn(mock)

			result, err := repo.GetWithProgress(context.Background(), tt.id)

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

// -----------------------------------------------------------------------------
// InitializeModuleProgress Tests
// -----------------------------------------------------------------------------

func TestEnrollmentRepo_InitializeModuleProgress(t *testing.T) {
	tests := []struct {
		name         string
		enrollmentID string
		pathwayID    string
		mockFn       func(mock sqlmock.Sqlmock)
		wantErr      bool
		errMsg       string
	}{
		{
			name:         "success",
			enrollmentID: "enroll-123",
			pathwayID:    "pathway-123",
			mockFn: func(mock sqlmock.Sqlmock) {
				// Insert module progress
				mock.ExpectExec(`INSERT INTO module_progress`).
					WithArgs("enroll-123", "pathway-123").
					WillReturnResult(sqlmock.NewResult(0, 3))

				// Insert lab progress
				mock.ExpectExec(`INSERT INTO lab_progress`).
					WithArgs("enroll-123", "pathway-123").
					WillReturnResult(sqlmock.NewResult(0, 8))

				// Update enrollment totals
				mock.ExpectExec(`UPDATE pathway_enrollments`).
					WithArgs("enroll-123", "pathway-123").
					WillReturnResult(sqlmock.NewResult(0, 1))
			},
			wantErr: false,
		},
		{
			name:         "module progress insert error",
			enrollmentID: "enroll-err",
			pathwayID:    "pathway-123",
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(`INSERT INTO module_progress`).
					WillReturnError(errors.New("duplicate key"))
			},
			wantErr: true,
			errMsg:  "initializing module progress",
		},
		{
			name:         "lab progress insert error",
			enrollmentID: "enroll-lab-err",
			pathwayID:    "pathway-123",
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(`INSERT INTO module_progress`).
					WillReturnResult(sqlmock.NewResult(0, 3))

				mock.ExpectExec(`INSERT INTO lab_progress`).
					WillReturnError(errors.New("foreign key violation"))
			},
			wantErr: true,
			errMsg:  "initializing lab progress",
		},
		{
			name:         "update enrollment error",
			enrollmentID: "enroll-update-err",
			pathwayID:    "pathway-123",
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(`INSERT INTO module_progress`).
					WillReturnResult(sqlmock.NewResult(0, 3))

				mock.ExpectExec(`INSERT INTO lab_progress`).
					WillReturnResult(sqlmock.NewResult(0, 8))

				mock.ExpectExec(`UPDATE pathway_enrollments`).
					WillReturnError(errors.New("update failed"))
			},
			wantErr: true,
			errMsg:  "updating enrollment totals",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatalf("failed to create sqlmock: %v", err)
			}
			defer db.Close()

			repo := NewEnrollmentRepo(db)
			tt.mockFn(mock)

			err = repo.InitializeModuleProgress(context.Background(), tt.enrollmentID, tt.pathwayID)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				} else if tt.errMsg != "" && !enrollmentContainsString(err.Error(), tt.errMsg) {
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
// UnlockNextModules Tests
// -----------------------------------------------------------------------------

func TestEnrollmentRepo_UnlockNextModules(t *testing.T) {
	fixedTime := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)

	tests := []struct {
		name              string
		enrollmentID      string
		completedModuleID string
		mockFn            func(mock sqlmock.Sqlmock)
		wantUnlocked      int
		wantErr           bool
		errMsg            string
	}{
		{
			name:              "success - unlocks next module",
			enrollmentID:      "enroll-123",
			completedModuleID: "module-1",
			mockFn: func(mock sqlmock.Sqlmock) {
				// GetByID
				enrollRows := sqlmock.NewRows([]string{
					"id", "user_id", "pathway_id", "status",
					"completed_modules", "total_modules",
					"earned_points", "max_points", "percentage",
					"enrolled_at", "started_at", "completed_at", "last_activity_at",
					"organization_id", "certificate_issued", "certificate_url",
				}).AddRow(
					"enroll-123", "user-123", "pathway-123", "in_progress",
					1, 5, 100, 500, 20.0,
					fixedTime, &fixedTime, nil, &fixedTime,
					nil, false, nil,
				)
				mock.ExpectQuery(`SELECT .+ FROM pathway_enrollments WHERE id = \$1`).
					WithArgs("enroll-123").
					WillReturnRows(enrollRows)

				// Get completed module display order
				mock.ExpectQuery(`SELECT pm.display_order FROM pathway_modules pm WHERE pm.id = \$1`).
					WithArgs("module-1").
					WillReturnRows(sqlmock.NewRows([]string{"display_order"}).AddRow(0))

				// Unlock next modules - returns one module
				unlockedRows := sqlmock.NewRows([]string{"module_id"}).AddRow("module-2")
				mock.ExpectQuery(`WITH completed_modules AS`).
					WithArgs("enroll-123", "pathway-123", 0, "module-1").
					WillReturnRows(unlockedRows)
			},
			wantUnlocked: 1,
			wantErr:      false,
		},
		{
			name:              "success - no modules to unlock",
			enrollmentID:      "enroll-123",
			completedModuleID: "module-5",
			mockFn: func(mock sqlmock.Sqlmock) {
				// GetByID
				enrollRows := sqlmock.NewRows([]string{
					"id", "user_id", "pathway_id", "status",
					"completed_modules", "total_modules",
					"earned_points", "max_points", "percentage",
					"enrolled_at", "started_at", "completed_at", "last_activity_at",
					"organization_id", "certificate_issued", "certificate_url",
				}).AddRow(
					"enroll-123", "user-123", "pathway-123", "in_progress",
					5, 5, 500, 500, 100.0,
					fixedTime, &fixedTime, nil, &fixedTime,
					nil, false, nil,
				)
				mock.ExpectQuery(`SELECT .+ FROM pathway_enrollments WHERE id = \$1`).
					WillReturnRows(enrollRows)

				// Get completed module display order
				mock.ExpectQuery(`SELECT pm.display_order FROM pathway_modules pm WHERE pm.id = \$1`).
					WillReturnRows(sqlmock.NewRows([]string{"display_order"}).AddRow(4))

				// No modules unlocked
				mock.ExpectQuery(`WITH completed_modules AS`).
					WillReturnRows(sqlmock.NewRows([]string{"module_id"}))
			},
			wantUnlocked: 0,
			wantErr:      false,
		},
		{
			name:              "enrollment not found",
			enrollmentID:      "non-existent",
			completedModuleID: "module-1",
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`SELECT .+ FROM pathway_enrollments WHERE id = \$1`).
					WithArgs("non-existent").
					WillReturnError(sql.ErrNoRows)
			},
			wantUnlocked: 0,
			wantErr:      true,
			errMsg:       "enrollment not found",
		},
		{
			name:              "get display order error",
			enrollmentID:      "enroll-123",
			completedModuleID: "module-err",
			mockFn: func(mock sqlmock.Sqlmock) {
				enrollRows := sqlmock.NewRows([]string{
					"id", "user_id", "pathway_id", "status",
					"completed_modules", "total_modules",
					"earned_points", "max_points", "percentage",
					"enrolled_at", "started_at", "completed_at", "last_activity_at",
					"organization_id", "certificate_issued", "certificate_url",
				}).AddRow(
					"enroll-123", "user-123", "pathway-123", "in_progress",
					1, 5, 100, 500, 20.0,
					fixedTime, &fixedTime, nil, &fixedTime,
					nil, false, nil,
				)
				mock.ExpectQuery(`SELECT .+ FROM pathway_enrollments WHERE id = \$1`).
					WillReturnRows(enrollRows)

				mock.ExpectQuery(`SELECT pm.display_order FROM pathway_modules pm WHERE pm.id = \$1`).
					WillReturnError(errors.New("module not found"))
			},
			wantUnlocked: 0,
			wantErr:      true,
			errMsg:       "getting completed module order",
		},
		{
			name:              "unlock query error",
			enrollmentID:      "enroll-123",
			completedModuleID: "module-1",
			mockFn: func(mock sqlmock.Sqlmock) {
				enrollRows := sqlmock.NewRows([]string{
					"id", "user_id", "pathway_id", "status",
					"completed_modules", "total_modules",
					"earned_points", "max_points", "percentage",
					"enrolled_at", "started_at", "completed_at", "last_activity_at",
					"organization_id", "certificate_issued", "certificate_url",
				}).AddRow(
					"enroll-123", "user-123", "pathway-123", "in_progress",
					1, 5, 100, 500, 20.0,
					fixedTime, &fixedTime, nil, &fixedTime,
					nil, false, nil,
				)
				mock.ExpectQuery(`SELECT .+ FROM pathway_enrollments WHERE id = \$1`).
					WillReturnRows(enrollRows)

				mock.ExpectQuery(`SELECT pm.display_order FROM pathway_modules pm WHERE pm.id = \$1`).
					WillReturnRows(sqlmock.NewRows([]string{"display_order"}).AddRow(0))

				mock.ExpectQuery(`WITH completed_modules AS`).
					WillReturnError(errors.New("unlock query failed"))
			},
			wantUnlocked: 0,
			wantErr:      true,
			errMsg:       "unlocking next modules",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatalf("failed to create sqlmock: %v", err)
			}
			defer db.Close()

			repo := NewEnrollmentRepo(db)
			tt.mockFn(mock)

			result, err := repo.UnlockNextModules(context.Background(), tt.enrollmentID, tt.completedModuleID)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				} else if tt.errMsg != "" && !enrollmentContainsString(err.Error(), tt.errMsg) {
					t.Errorf("error %q should contain %q", err.Error(), tt.errMsg)
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if len(result) != tt.wantUnlocked {
					t.Errorf("expected %d unlocked modules, got %d", tt.wantUnlocked, len(result))
				}
			}

			if err := mock.ExpectationsWereMet(); err != nil {
				t.Errorf("unfulfilled expectations: %v", err)
			}
		})
	}
}

// -----------------------------------------------------------------------------
// RecordLabAttempt Tests
// -----------------------------------------------------------------------------

func TestEnrollmentRepo_RecordLabAttempt(t *testing.T) {
	tests := []struct {
		name          string
		enrollmentID  string
		moduleID      string
		labTemplateID string
		sessionID     string
		score         int
		passed        bool
		mockFn        func(mock sqlmock.Sqlmock)
		wantErr       bool
		errMsg        string
	}{
		{
			name:          "success - first attempt passed",
			enrollmentID:  "enroll-123",
			moduleID:      "module-123",
			labTemplateID: "lab-123",
			sessionID:     "session-123",
			score:         100,
			passed:        true,
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectBegin()
				mock.ExpectExec(`INSERT INTO lab_progress`).
					WithArgs(
						"enroll-123", "module-123", "lab-123", "session-123",
						100, true, sqlmock.AnyArg(),
					).
					WillReturnResult(sqlmock.NewResult(0, 1))

				mock.ExpectExec(`UPDATE pathway_enrollments SET last_activity_at`).
					WithArgs("enroll-123", sqlmock.AnyArg()).
					WillReturnResult(sqlmock.NewResult(0, 1))

				mock.ExpectCommit()
			},
			wantErr: false,
		},
		{
			name:          "success - attempt failed",
			enrollmentID:  "enroll-123",
			moduleID:      "module-123",
			labTemplateID: "lab-123",
			sessionID:     "session-456",
			score:         50,
			passed:        false,
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectBegin()
				mock.ExpectExec(`INSERT INTO lab_progress`).
					WillReturnResult(sqlmock.NewResult(0, 1))

				mock.ExpectExec(`UPDATE pathway_enrollments SET last_activity_at`).
					WillReturnResult(sqlmock.NewResult(0, 1))

				mock.ExpectCommit()
			},
			wantErr: false,
		},
		{
			name:          "begin transaction error",
			enrollmentID:  "enroll-err",
			moduleID:      "module-123",
			labTemplateID: "lab-123",
			sessionID:     "session-123",
			score:         100,
			passed:        true,
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectBegin().WillReturnError(errors.New("connection error"))
			},
			wantErr: true,
			errMsg:  "beginning transaction",
		},
		{
			name:          "insert lab progress error",
			enrollmentID:  "enroll-123",
			moduleID:      "module-123",
			labTemplateID: "lab-err",
			sessionID:     "session-123",
			score:         100,
			passed:        true,
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectBegin()
				mock.ExpectExec(`INSERT INTO lab_progress`).
					WillReturnError(errors.New("foreign key violation"))
				mock.ExpectRollback()
			},
			wantErr: true,
			errMsg:  "recording lab attempt",
		},
		{
			name:          "update enrollment error",
			enrollmentID:  "enroll-123",
			moduleID:      "module-123",
			labTemplateID: "lab-123",
			sessionID:     "session-123",
			score:         100,
			passed:        true,
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectBegin()
				mock.ExpectExec(`INSERT INTO lab_progress`).
					WillReturnResult(sqlmock.NewResult(0, 1))

				mock.ExpectExec(`UPDATE pathway_enrollments`).
					WillReturnError(errors.New("update failed"))
				mock.ExpectRollback()
			},
			wantErr: true,
			errMsg:  "updating enrollment activity",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatalf("failed to create sqlmock: %v", err)
			}
			defer db.Close()

			repo := NewEnrollmentRepo(db)
			tt.mockFn(mock)

			err = repo.RecordLabAttempt(
				context.Background(),
				tt.enrollmentID,
				tt.moduleID,
				tt.labTemplateID,
				tt.sessionID,
				tt.score,
				tt.passed,
			)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				} else if tt.errMsg != "" && !enrollmentContainsString(err.Error(), tt.errMsg) {
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
// ListWithPathways Tests
// -----------------------------------------------------------------------------

func TestEnrollmentRepo_ListWithPathways(t *testing.T) {
	fixedTime := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)

	tests := []struct {
		name      string
		opts      models.EnrollmentListOptions
		mockFn    func(mock sqlmock.Sqlmock)
		wantCount int
		wantErr   bool
	}{
		{
			name: "success - list by user",
			opts: models.EnrollmentListOptions{
				UserID: "user-123",
			},
			mockFn: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{
					"id", "user_id", "pathway_id", "status",
					"completed_modules", "total_modules",
					"earned_points", "max_points", "percentage",
					"enrolled_at", "started_at", "completed_at", "last_activity_at",
					"name", "slug", "description", "icon", "cover_image", "difficulty",
				}).AddRow(
					"enroll-1", "user-123", "pathway-1", "in_progress",
					2, 5, 100, 500, 20.0,
					fixedTime, &fixedTime, nil, &fixedTime,
					"Network Fundamentals", "network-fundamentals",
					"Learn network basics", "network", "cover1.jpg", "beginner",
				).AddRow(
					"enroll-2", "user-123", "pathway-2", "completed",
					5, 5, 500, 500, 100.0,
					fixedTime, &fixedTime, &fixedTime, &fixedTime,
					"Security Basics", "security-basics",
					"Security fundamentals", "security", "cover2.jpg", "intermediate",
				)
				mock.ExpectQuery(`SELECT .+ FROM pathway_enrollments e JOIN pathways p`).
					WillReturnRows(rows)
			},
			wantCount: 2,
			wantErr:   false,
		},
		{
			name: "success - list by status",
			opts: models.EnrollmentListOptions{
				Status: models.EnrollmentStatusInProgress,
			},
			mockFn: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{
					"id", "user_id", "pathway_id", "status",
					"completed_modules", "total_modules",
					"earned_points", "max_points", "percentage",
					"enrolled_at", "started_at", "completed_at", "last_activity_at",
					"name", "slug", "description", "icon", "cover_image", "difficulty",
				}).AddRow(
					"enroll-1", "user-1", "pathway-1", "in_progress",
					2, 5, 100, 500, 20.0,
					fixedTime, &fixedTime, nil, &fixedTime,
					"Network Fundamentals", "network-fundamentals",
					"Learn network basics", "network", "cover1.jpg", "beginner",
				)
				mock.ExpectQuery(`SELECT .+ FROM pathway_enrollments e JOIN pathways p`).
					WillReturnRows(rows)
			},
			wantCount: 1,
			wantErr:   false,
		},
		{
			name: "empty result",
			opts: models.EnrollmentListOptions{
				UserID: "user-no-enrollments",
			},
			mockFn: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{
					"id", "user_id", "pathway_id", "status",
					"completed_modules", "total_modules",
					"earned_points", "max_points", "percentage",
					"enrolled_at", "started_at", "completed_at", "last_activity_at",
					"name", "slug", "description", "icon", "cover_image", "difficulty",
				})
				mock.ExpectQuery(`SELECT .+ FROM pathway_enrollments e JOIN pathways p`).
					WillReturnRows(rows)
			},
			wantCount: 0,
			wantErr:   false,
		},
		{
			name: "database error",
			opts: models.EnrollmentListOptions{
				UserID: "user-err",
			},
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`SELECT .+ FROM pathway_enrollments e JOIN pathways p`).
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

			repo := NewEnrollmentRepo(db)
			tt.mockFn(mock)

			result, err := repo.ListWithPathways(context.Background(), tt.opts)

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
// Helper Functions
// -----------------------------------------------------------------------------

func enrollmentContainsString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
