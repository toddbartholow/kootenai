package repositories

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"

	"github.com/toddbartholow/kootenai/api/internal/models"
)

func TestGradeSyncRepo_Create(t *testing.T) {
	fixedTime := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)

	tests := []struct {
		name    string
		entry   *models.GradeSyncEntry
		mockFn  func(mock sqlmock.Sqlmock, entry *models.GradeSyncEntry)
		wantErr bool
		errMsg  string
	}{
		{
			name: "success",
			entry: &models.GradeSyncEntry{
				ID:                 "grade-123",
				SessionID:          "session-1",
				Status:             "pending",
				EarnedPoints:       85,
				MaxPoints:          100,
				Percentage:         85.0,
				CanvasCourseID:     "course-1",
				CanvasAssignmentID: "assign-1",
				CanvasUserID:       "canvas-user-1",
				Attempts:           0,
			},
			mockFn: func(mock sqlmock.Sqlmock, entry *models.GradeSyncEntry) {
				rows := sqlmock.NewRows([]string{"created_at"}).AddRow(fixedTime)
				mock.ExpectQuery(`INSERT INTO grade_sync_queue`).
					WithArgs(
						entry.ID,
						entry.SessionID,
						entry.Status,
						entry.EarnedPoints,
						entry.MaxPoints,
						entry.Percentage,
						entry.CanvasCourseID,
						entry.CanvasAssignmentID,
						entry.CanvasUserID,
						entry.Attempts,
					).
					WillReturnRows(rows)
			},
			wantErr: false,
		},
		{
			name: "database error",
			entry: &models.GradeSyncEntry{
				ID:        "grade-err",
				SessionID: "session-1",
				Status:    "pending",
			},
			mockFn: func(mock sqlmock.Sqlmock, entry *models.GradeSyncEntry) {
				mock.ExpectQuery(`INSERT INTO grade_sync_queue`).
					WillReturnError(errors.New("duplicate key"))
			},
			wantErr: true,
			errMsg:  "inserting grade sync entry",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatalf("failed to create sqlmock: %v", err)
			}
			defer db.Close()

			repo := NewGradeSyncRepo(db)
			tt.mockFn(mock, tt.entry)

			err = repo.Create(context.Background(), tt.entry)

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

func TestGradeSyncRepo_GetPending(t *testing.T) {
	fixedTime := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)

	tests := []struct {
		name      string
		limit     int
		mockFn    func(mock sqlmock.Sqlmock)
		wantCount int
		wantErr   bool
		errMsg    string
	}{
		{
			name:  "has pending entries",
			limit: 10,
			mockFn: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{
					"id", "session_id", "status", "earned_points", "max_points", "percentage",
					"canvas_course_id", "canvas_assignment_id", "canvas_user_id",
					"attempts", "last_attempt_at", "completed_at", "error_message", "created_at",
				}).
					AddRow("grade-1", "session-1", "pending", 85, 100, 85.0, "c1", "a1", "u1", 0, nil, nil, nil, fixedTime).
					AddRow("grade-2", "session-2", "pending", 90, 100, 90.0, "c2", "a2", "u2", 1, &fixedTime, nil, nil, fixedTime)
				mock.ExpectQuery(`SELECT .+ FROM grade_sync_queue WHERE status = 'pending'`).
					WithArgs(10).
					WillReturnRows(rows)
			},
			wantCount: 2,
			wantErr:   false,
		},
		{
			name:  "no pending entries",
			limit: 10,
			mockFn: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{
					"id", "session_id", "status", "earned_points", "max_points", "percentage",
					"canvas_course_id", "canvas_assignment_id", "canvas_user_id",
					"attempts", "last_attempt_at", "completed_at", "error_message", "created_at",
				})
				mock.ExpectQuery(`SELECT .+ FROM grade_sync_queue WHERE status = 'pending'`).
					WithArgs(10).
					WillReturnRows(rows)
			},
			wantCount: 0,
			wantErr:   false,
		},
		{
			name:  "database error",
			limit: 10,
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`SELECT .+ FROM grade_sync_queue WHERE status = 'pending'`).
					WillReturnError(errors.New("connection refused"))
			},
			wantCount: 0,
			wantErr:   true,
			errMsg:    "querying pending entries",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatalf("failed to create sqlmock: %v", err)
			}
			defer db.Close()

			repo := NewGradeSyncRepo(db)
			tt.mockFn(mock)

			got, err := repo.GetPending(context.Background(), tt.limit)

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
				if len(got) != tt.wantCount {
					t.Errorf("got %d entries, want %d", len(got), tt.wantCount)
				}
			}

			if err := mock.ExpectationsWereMet(); err != nil {
				t.Errorf("unfulfilled expectations: %v", err)
			}
		})
	}
}

func TestGradeSyncRepo_MarkProcessing(t *testing.T) {
	tests := []struct {
		name    string
		id      string
		mockFn  func(mock sqlmock.Sqlmock)
		wantErr bool
		errMsg  string
	}{
		{
			name: "success",
			id:   "grade-123",
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(`UPDATE grade_sync_queue SET status = 'processing'`).
					WithArgs("grade-123", sqlmock.AnyArg()).
					WillReturnResult(sqlmock.NewResult(0, 1))
			},
			wantErr: false,
		},
		{
			name: "entry not found",
			id:   "grade-nonexistent",
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(`UPDATE grade_sync_queue SET status = 'processing'`).
					WithArgs("grade-nonexistent", sqlmock.AnyArg()).
					WillReturnResult(sqlmock.NewResult(0, 0))
			},
			wantErr: true,
			errMsg:  "grade sync entry not found",
		},
		{
			name: "database error",
			id:   "grade-err",
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(`UPDATE grade_sync_queue SET status = 'processing'`).
					WillReturnError(errors.New("database error"))
			},
			wantErr: true,
			errMsg:  "marking entry as processing",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatalf("failed to create sqlmock: %v", err)
			}
			defer db.Close()

			repo := NewGradeSyncRepo(db)
			tt.mockFn(mock)

			err = repo.MarkProcessing(context.Background(), tt.id)

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

func TestGradeSyncRepo_MarkCompleted(t *testing.T) {
	tests := []struct {
		name    string
		id      string
		mockFn  func(mock sqlmock.Sqlmock)
		wantErr bool
		errMsg  string
	}{
		{
			name: "success",
			id:   "grade-123",
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(`UPDATE grade_sync_queue SET status = 'completed'`).
					WithArgs("grade-123", sqlmock.AnyArg()).
					WillReturnResult(sqlmock.NewResult(0, 1))
			},
			wantErr: false,
		},
		{
			name: "entry not found",
			id:   "grade-nonexistent",
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(`UPDATE grade_sync_queue SET status = 'completed'`).
					WithArgs("grade-nonexistent", sqlmock.AnyArg()).
					WillReturnResult(sqlmock.NewResult(0, 0))
			},
			wantErr: true,
			errMsg:  "grade sync entry not found",
		},
		{
			name: "database error",
			id:   "grade-err",
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(`UPDATE grade_sync_queue SET status = 'completed'`).
					WillReturnError(errors.New("database error"))
			},
			wantErr: true,
			errMsg:  "marking entry as completed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatalf("failed to create sqlmock: %v", err)
			}
			defer db.Close()

			repo := NewGradeSyncRepo(db)
			tt.mockFn(mock)

			err = repo.MarkCompleted(context.Background(), tt.id)

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

func TestGradeSyncRepo_MarkFailed(t *testing.T) {
	tests := []struct {
		name     string
		id       string
		errorMsg string
		mockFn   func(mock sqlmock.Sqlmock)
		wantErr  bool
		errText  string
	}{
		{
			name:     "success",
			id:       "grade-123",
			errorMsg: "Canvas API timeout",
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(`UPDATE grade_sync_queue SET status = 'failed'`).
					WithArgs("grade-123", "Canvas API timeout").
					WillReturnResult(sqlmock.NewResult(0, 1))
			},
			wantErr: false,
		},
		{
			name:     "entry not found",
			id:       "grade-nonexistent",
			errorMsg: "Some error",
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(`UPDATE grade_sync_queue SET status = 'failed'`).
					WithArgs("grade-nonexistent", "Some error").
					WillReturnResult(sqlmock.NewResult(0, 0))
			},
			wantErr: true,
			errText: "grade sync entry not found",
		},
		{
			name:     "database error",
			id:       "grade-err",
			errorMsg: "Error msg",
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(`UPDATE grade_sync_queue SET status = 'failed'`).
					WillReturnError(errors.New("database error"))
			},
			wantErr: true,
			errText: "marking entry as failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatalf("failed to create sqlmock: %v", err)
			}
			defer db.Close()

			repo := NewGradeSyncRepo(db)
			tt.mockFn(mock)

			err = repo.MarkFailed(context.Background(), tt.id, tt.errorMsg)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				} else if tt.errText != "" && !containsString(err.Error(), tt.errText) {
					t.Errorf("error %q should contain %q", err.Error(), tt.errText)
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
