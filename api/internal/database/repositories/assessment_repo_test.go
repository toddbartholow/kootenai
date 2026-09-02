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

func TestAssessmentResultRepo_Create(t *testing.T) {
	fixedTime := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)

	tests := []struct {
		name    string
		result  *models.AssessmentResult
		mockFn  func(mock sqlmock.Sqlmock, result *models.AssessmentResult)
		wantErr bool
		errMsg  string
	}{
		{
			name: "success with components and devices",
			result: &models.AssessmentResult{
				SessionID:   "session-123",
				Score:       85,
				MaxScore:    100,
				Percentage:  85.0,
				ItemCount:   10,
				PassedCount: 8,
				Status:      "in_progress",
				StartedAt:   fixedTime,
				LastChecked: fixedTime,
				Components: []models.ComponentResult{
					{ID: "comp-1", Description: "Component 1", TotalItems: 5, PassedItems: 4},
				},
				Devices: []models.DeviceResult{
					{Name: "Device 1", Type: "router", TotalItems: 5, PassedItems: 4, EarnedPoints: 80, MaxPoints: 100},
				},
			},
			mockFn: func(mock sqlmock.Sqlmock, result *models.AssessmentResult) {
				mock.ExpectExec(`INSERT INTO assessment_results`).
					WithArgs(
						result.SessionID,
						result.Score,
						result.MaxScore,
						result.Percentage,
						result.ItemCount,
						result.PassedCount,
						result.Status,
						result.StartedAt,
						result.LastChecked,
						sqlmock.AnyArg(), // components JSON
						sqlmock.AnyArg(), // devices JSON
					).
					WillReturnResult(sqlmock.NewResult(0, 1))
			},
			wantErr: false,
		},
		{
			name: "success with empty components",
			result: &models.AssessmentResult{
				SessionID:   "session-456",
				Score:       0,
				MaxScore:    100,
				Status:      "in_progress",
				StartedAt:   fixedTime,
				LastChecked: fixedTime,
				Components:  []models.ComponentResult{},
				Devices:     []models.DeviceResult{},
			},
			mockFn: func(mock sqlmock.Sqlmock, result *models.AssessmentResult) {
				mock.ExpectExec(`INSERT INTO assessment_results`).
					WillReturnResult(sqlmock.NewResult(0, 1))
			},
			wantErr: false,
		},
		{
			name: "database error",
			result: &models.AssessmentResult{
				SessionID:   "session-err",
				Score:       0,
				MaxScore:    100,
				Status:      "in_progress",
				StartedAt:   fixedTime,
				LastChecked: fixedTime,
			},
			mockFn: func(mock sqlmock.Sqlmock, result *models.AssessmentResult) {
				mock.ExpectExec(`INSERT INTO assessment_results`).
					WillReturnError(errors.New("insertion failed"))
			},
			wantErr: true,
			errMsg:  "inserting assessment result",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatalf("failed to create sqlmock: %v", err)
			}
			defer db.Close()

			repo := NewAssessmentResultRepo(db)
			tt.mockFn(mock, tt.result)

			err = repo.Create(context.Background(), tt.result)

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

func TestAssessmentResultRepo_GetBySessionID(t *testing.T) {
	fixedTime := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)

	tests := []struct {
		name      string
		sessionID string
		mockFn    func(mock sqlmock.Sqlmock)
		wantNil   bool
		wantErr   bool
		errMsg    string
	}{
		{
			name:      "existing result",
			sessionID: "session-123",
			mockFn: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{
					"session_id", "score", "max_score", "percentage",
					"item_count", "passed_count", "status",
					"started_at", "last_checked", "completed_at",
					"components", "devices",
				}).AddRow(
					"session-123", 85, 100, 85.0,
					10, 8, "in_progress",
					fixedTime, fixedTime, nil,
					[]byte(`[{"id":"comp-1","description":"Component 1","totalItems":5,"passedItems":4}]`),
					[]byte(`[{"id":"dev-1","name":"Device 1","score":80,"maxScore":100}]`),
				)
				mock.ExpectQuery(`SELECT .+ FROM assessment_results WHERE session_id = \$1`).
					WithArgs("session-123").
					WillReturnRows(rows)
			},
			wantNil: false,
			wantErr: false,
		},
		{
			name:      "non-existent result",
			sessionID: "session-nonexistent",
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`SELECT .+ FROM assessment_results WHERE session_id = \$1`).
					WithArgs("session-nonexistent").
					WillReturnError(sql.ErrNoRows)
			},
			wantNil: true,
			wantErr: false,
		},
		{
			name:      "database error",
			sessionID: "session-err",
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`SELECT .+ FROM assessment_results WHERE session_id = \$1`).
					WithArgs("session-err").
					WillReturnError(errors.New("connection refused"))
			},
			wantNil: true,
			wantErr: true,
			errMsg:  "querying assessment result",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatalf("failed to create sqlmock: %v", err)
			}
			defer db.Close()

			repo := NewAssessmentResultRepo(db)
			tt.mockFn(mock)

			got, err := repo.GetBySessionID(context.Background(), tt.sessionID)

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
					t.Error("expected result, got nil")
				}
			}

			if err := mock.ExpectationsWereMet(); err != nil {
				t.Errorf("unfulfilled expectations: %v", err)
			}
		})
	}
}

func TestAssessmentResultRepo_Update(t *testing.T) {
	fixedTime := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)

	tests := []struct {
		name    string
		result  *models.AssessmentResult
		mockFn  func(mock sqlmock.Sqlmock, result *models.AssessmentResult)
		wantErr bool
		errMsg  string
	}{
		{
			name: "success",
			result: &models.AssessmentResult{
				SessionID:   "session-123",
				Score:       95,
				MaxScore:    100,
				Percentage:  95.0,
				ItemCount:   10,
				PassedCount: 9,
				Status:      "completed",
				LastChecked: fixedTime,
				Components:  []models.ComponentResult{},
				Devices:     []models.DeviceResult{},
			},
			mockFn: func(mock sqlmock.Sqlmock, result *models.AssessmentResult) {
				mock.ExpectExec(`UPDATE assessment_results SET`).
					WithArgs(
						result.SessionID,
						result.Score,
						result.MaxScore,
						result.Percentage,
						result.ItemCount,
						result.PassedCount,
						result.Status,
						result.LastChecked,
						sqlmock.AnyArg(), // components JSON
						sqlmock.AnyArg(), // devices JSON
					).
					WillReturnResult(sqlmock.NewResult(0, 1))
			},
			wantErr: false,
		},
		{
			name: "result not found",
			result: &models.AssessmentResult{
				SessionID:   "session-nonexistent",
				Score:       50,
				MaxScore:    100,
				Status:      "in_progress",
				LastChecked: fixedTime,
			},
			mockFn: func(mock sqlmock.Sqlmock, result *models.AssessmentResult) {
				mock.ExpectExec(`UPDATE assessment_results SET`).
					WillReturnResult(sqlmock.NewResult(0, 0))
			},
			wantErr: true,
			errMsg:  "assessment result not found",
		},
		{
			name: "database error",
			result: &models.AssessmentResult{
				SessionID:   "session-err",
				Status:      "in_progress",
				LastChecked: fixedTime,
			},
			mockFn: func(mock sqlmock.Sqlmock, result *models.AssessmentResult) {
				mock.ExpectExec(`UPDATE assessment_results SET`).
					WillReturnError(errors.New("database error"))
			},
			wantErr: true,
			errMsg:  "updating assessment result",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatalf("failed to create sqlmock: %v", err)
			}
			defer db.Close()

			repo := NewAssessmentResultRepo(db)
			tt.mockFn(mock, tt.result)

			err = repo.Update(context.Background(), tt.result)

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

func TestAssessmentResultRepo_UpdateStatus(t *testing.T) {
	tests := []struct {
		name      string
		sessionID string
		status    string
		mockFn    func(mock sqlmock.Sqlmock)
		wantErr   bool
		errMsg    string
	}{
		{
			name:      "success - in_progress",
			sessionID: "session-123",
			status:    "in_progress",
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(`UPDATE assessment_results SET status = \$2, last_checked = NOW\(\) WHERE session_id = \$1`).
					WithArgs("session-123", "in_progress").
					WillReturnResult(sqlmock.NewResult(0, 1))
			},
			wantErr: false,
		},
		{
			name:      "success - completed (sets completed_at)",
			sessionID: "session-456",
			status:    "completed",
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(`UPDATE assessment_results SET status = \$2, last_checked = NOW\(\), completed_at = NOW\(\) WHERE session_id = \$1`).
					WithArgs("session-456", "completed").
					WillReturnResult(sqlmock.NewResult(0, 1))
			},
			wantErr: false,
		},
		{
			name:      "success - graded (sets completed_at)",
			sessionID: "session-789",
			status:    "graded",
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(`UPDATE assessment_results SET status = \$2, last_checked = NOW\(\), completed_at = NOW\(\) WHERE session_id = \$1`).
					WithArgs("session-789", "graded").
					WillReturnResult(sqlmock.NewResult(0, 1))
			},
			wantErr: false,
		},
		{
			name:      "result not found",
			sessionID: "session-nonexistent",
			status:    "in_progress",
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(`UPDATE assessment_results SET status = \$2`).
					WillReturnResult(sqlmock.NewResult(0, 0))
			},
			wantErr: true,
			errMsg:  "assessment result not found",
		},
		{
			name:      "database error",
			sessionID: "session-err",
			status:    "in_progress",
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(`UPDATE assessment_results SET status = \$2`).
					WillReturnError(errors.New("database error"))
			},
			wantErr: true,
			errMsg:  "updating status",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatalf("failed to create sqlmock: %v", err)
			}
			defer db.Close()

			repo := NewAssessmentResultRepo(db)
			tt.mockFn(mock)

			err = repo.UpdateStatus(context.Background(), tt.sessionID, tt.status)

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

func TestAssessmentResultRepo_Delete(t *testing.T) {
	tests := []struct {
		name      string
		sessionID string
		mockFn    func(mock sqlmock.Sqlmock)
		wantErr   bool
		errMsg    string
	}{
		{
			name:      "success",
			sessionID: "session-123",
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(`DELETE FROM assessment_results WHERE session_id = \$1`).
					WithArgs("session-123").
					WillReturnResult(sqlmock.NewResult(0, 1))
			},
			wantErr: false,
		},
		{
			name:      "success - no rows affected (doesn't error)",
			sessionID: "session-nonexistent",
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(`DELETE FROM assessment_results WHERE session_id = \$1`).
					WithArgs("session-nonexistent").
					WillReturnResult(sqlmock.NewResult(0, 0))
			},
			wantErr: false, // Delete doesn't error on missing row
		},
		{
			name:      "database error",
			sessionID: "session-err",
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(`DELETE FROM assessment_results WHERE session_id = \$1`).
					WillReturnError(errors.New("database error"))
			},
			wantErr: true,
			errMsg:  "deleting assessment result",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatalf("failed to create sqlmock: %v", err)
			}
			defer db.Close()

			repo := NewAssessmentResultRepo(db)
			tt.mockFn(mock)

			err = repo.Delete(context.Background(), tt.sessionID)

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

func TestAssessmentResultRepo_DeleteExpired(t *testing.T) {
	fixedTime := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)

	tests := []struct {
		name      string
		olderThan time.Time
		mockFn    func(mock sqlmock.Sqlmock)
		wantCount int64
		wantErr   bool
		errMsg    string
	}{
		{
			name:      "success - deleted some",
			olderThan: fixedTime,
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(`DELETE FROM assessment_results WHERE updated_at < \$1`).
					WithArgs(fixedTime).
					WillReturnResult(sqlmock.NewResult(0, 5))
			},
			wantCount: 5,
			wantErr:   false,
		},
		{
			name:      "success - none to delete",
			olderThan: fixedTime,
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(`DELETE FROM assessment_results WHERE updated_at < \$1`).
					WithArgs(fixedTime).
					WillReturnResult(sqlmock.NewResult(0, 0))
			},
			wantCount: 0,
			wantErr:   false,
		},
		{
			name:      "database error",
			olderThan: fixedTime,
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(`DELETE FROM assessment_results WHERE updated_at < \$1`).
					WillReturnError(errors.New("database error"))
			},
			wantCount: 0,
			wantErr:   true,
			errMsg:    "deleting expired assessment results",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatalf("failed to create sqlmock: %v", err)
			}
			defer db.Close()

			repo := NewAssessmentResultRepo(db)
			tt.mockFn(mock)

			count, err := repo.DeleteExpired(context.Background(), tt.olderThan)

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
				if count != tt.wantCount {
					t.Errorf("got count %d, want %d", count, tt.wantCount)
				}
			}

			if err := mock.ExpectationsWereMet(); err != nil {
				t.Errorf("unfulfilled expectations: %v", err)
			}
		})
	}
}
