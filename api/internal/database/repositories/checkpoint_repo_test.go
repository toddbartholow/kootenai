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

func TestCheckpointProgressRepo_Create(t *testing.T) {
	fixedTime := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)

	tests := []struct {
		name     string
		progress *models.CheckpointProgress
		mockFn   func(mock sqlmock.Sqlmock, progress *models.CheckpointProgress)
		wantErr  bool
		errMsg   string
	}{
		{
			name: "success",
			progress: &models.CheckpointProgress{
				ID:           "cp-progress-1",
				SessionID:    "session-1",
				CheckpointID: "checkpoint-1",
				Status:       models.CheckpointStatusPending,
				Points:       10,
				EarnedPoints: 0,
				Feedback:     "",
			},
			mockFn: func(mock sqlmock.Sqlmock, progress *models.CheckpointProgress) {
				rows := sqlmock.NewRows([]string{"created_at", "updated_at"}).
					AddRow(fixedTime, fixedTime)
				mock.ExpectQuery(`INSERT INTO checkpoint_progress`).
					WithArgs(
						progress.ID,
						progress.SessionID,
						progress.CheckpointID,
						progress.Status,
						progress.Points,
						progress.EarnedPoints,
						sqlmock.AnyArg(), // feedback (NullString)
					).
					WillReturnRows(rows)
			},
			wantErr: false,
		},
		{
			name: "success with feedback",
			progress: &models.CheckpointProgress{
				ID:           "cp-progress-2",
				SessionID:    "session-1",
				CheckpointID: "checkpoint-2",
				Status:       models.CheckpointStatusPassed,
				Points:       20,
				EarnedPoints: 20,
				Feedback:     "Great job!",
			},
			mockFn: func(mock sqlmock.Sqlmock, progress *models.CheckpointProgress) {
				rows := sqlmock.NewRows([]string{"created_at", "updated_at"}).
					AddRow(fixedTime, fixedTime)
				mock.ExpectQuery(`INSERT INTO checkpoint_progress`).
					WithArgs(
						progress.ID,
						progress.SessionID,
						progress.CheckpointID,
						progress.Status,
						progress.Points,
						progress.EarnedPoints,
						sqlmock.AnyArg(),
					).
					WillReturnRows(rows)
			},
			wantErr: false,
		},
		{
			name: "database error",
			progress: &models.CheckpointProgress{
				ID:           "cp-progress-err",
				SessionID:    "session-1",
				CheckpointID: "checkpoint-1",
				Status:       models.CheckpointStatusPending,
				Points:       10,
				EarnedPoints: 0,
			},
			mockFn: func(mock sqlmock.Sqlmock, progress *models.CheckpointProgress) {
				mock.ExpectQuery(`INSERT INTO checkpoint_progress`).
					WillReturnError(errors.New("duplicate key"))
			},
			wantErr: true,
			errMsg:  "inserting checkpoint progress",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatalf("failed to create sqlmock: %v", err)
			}
			defer db.Close()

			repo := NewCheckpointProgressRepo(db)
			tt.mockFn(mock, tt.progress)

			err = repo.Create(context.Background(), tt.progress)

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

func TestCheckpointProgressRepo_GetBySessionID(t *testing.T) {
	fixedTime := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)

	tests := []struct {
		name      string
		sessionID string
		mockFn    func(mock sqlmock.Sqlmock)
		wantCount int
		wantErr   bool
		errMsg    string
	}{
		{
			name:      "multiple checkpoints",
			sessionID: "session-1",
			mockFn: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{
					"id", "session_id", "checkpoint_id", "status", "points", "earned_points",
					"passed_at", "triggered_by_event_id", "attempt_count", "last_attempt_at",
					"feedback", "created_at", "updated_at",
				}).
					AddRow("cp1", "session-1", "checkpoint-1", "passed", 10, 10, &fixedTime, nil, 1, &fixedTime, "Good!", fixedTime, fixedTime).
					AddRow("cp2", "session-1", "checkpoint-2", "pending", 20, 0, nil, nil, 0, nil, nil, fixedTime, fixedTime)
				mock.ExpectQuery(`SELECT .+ FROM checkpoint_progress WHERE session_id = \$1`).
					WithArgs("session-1").
					WillReturnRows(rows)
			},
			wantCount: 2,
			wantErr:   false,
		},
		{
			name:      "no checkpoints",
			sessionID: "session-empty",
			mockFn: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{
					"id", "session_id", "checkpoint_id", "status", "points", "earned_points",
					"passed_at", "triggered_by_event_id", "attempt_count", "last_attempt_at",
					"feedback", "created_at", "updated_at",
				})
				mock.ExpectQuery(`SELECT .+ FROM checkpoint_progress WHERE session_id = \$1`).
					WithArgs("session-empty").
					WillReturnRows(rows)
			},
			wantCount: 0,
			wantErr:   false,
		},
		{
			name:      "database error",
			sessionID: "session-err",
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`SELECT .+ FROM checkpoint_progress WHERE session_id = \$1`).
					WithArgs("session-err").
					WillReturnError(errors.New("connection lost"))
			},
			wantCount: 0,
			wantErr:   true,
			errMsg:    "querying checkpoint progress",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatalf("failed to create sqlmock: %v", err)
			}
			defer db.Close()

			repo := NewCheckpointProgressRepo(db)
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
				if len(got) != tt.wantCount {
					t.Errorf("got %d checkpoints, want %d", len(got), tt.wantCount)
				}
			}

			if err := mock.ExpectationsWereMet(); err != nil {
				t.Errorf("unfulfilled expectations: %v", err)
			}
		})
	}
}

func TestCheckpointProgressRepo_GetBySessionAndCheckpoint(t *testing.T) {
	fixedTime := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)

	tests := []struct {
		name         string
		sessionID    string
		checkpointID string
		mockFn       func(mock sqlmock.Sqlmock)
		wantNil      bool
		wantErr      bool
		errMsg       string
	}{
		{
			name:         "existing checkpoint",
			sessionID:    "session-1",
			checkpointID: "checkpoint-1",
			mockFn: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{
					"id", "session_id", "checkpoint_id", "status", "points", "earned_points",
					"passed_at", "triggered_by_event_id", "attempt_count", "last_attempt_at",
					"feedback", "created_at", "updated_at",
				}).AddRow("cp1", "session-1", "checkpoint-1", "passed", 10, 10, &fixedTime, int64(123), 1, &fixedTime, "Done!", fixedTime, fixedTime)
				mock.ExpectQuery(`SELECT .+ FROM checkpoint_progress WHERE session_id = \$1 AND checkpoint_id = \$2`).
					WithArgs("session-1", "checkpoint-1").
					WillReturnRows(rows)
			},
			wantNil: false,
			wantErr: false,
		},
		{
			name:         "non-existent checkpoint",
			sessionID:    "session-1",
			checkpointID: "checkpoint-missing",
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`SELECT .+ FROM checkpoint_progress WHERE session_id = \$1 AND checkpoint_id = \$2`).
					WithArgs("session-1", "checkpoint-missing").
					WillReturnError(sql.ErrNoRows)
			},
			wantNil: true,
			wantErr: false, // Repository returns nil, nil for non-existent records (consistent with other repos)
		},
		{
			name:         "database error",
			sessionID:    "session-1",
			checkpointID: "checkpoint-err",
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`SELECT .+ FROM checkpoint_progress WHERE session_id = \$1 AND checkpoint_id = \$2`).
					WithArgs("session-1", "checkpoint-err").
					WillReturnError(errors.New("timeout"))
			},
			wantNil: true,
			wantErr: true,
			errMsg:  "scanning checkpoint progress",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatalf("failed to create sqlmock: %v", err)
			}
			defer db.Close()

			repo := NewCheckpointProgressRepo(db)
			tt.mockFn(mock)

			got, err := repo.GetBySessionAndCheckpoint(context.Background(), tt.sessionID, tt.checkpointID)

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
					t.Error("expected progress, got nil")
				}
			}

			if err := mock.ExpectationsWereMet(); err != nil {
				t.Errorf("unfulfilled expectations: %v", err)
			}
		})
	}
}

func TestCheckpointProgressRepo_Update(t *testing.T) {
	fixedTime := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)
	eventID := "123"

	tests := []struct {
		name     string
		progress *models.CheckpointProgress
		mockFn   func(mock sqlmock.Sqlmock, progress *models.CheckpointProgress)
		wantErr  bool
		errMsg   string
	}{
		{
			name: "success",
			progress: &models.CheckpointProgress{
				SessionID:      "session-1",
				CheckpointID:   "checkpoint-1",
				Status:         models.CheckpointStatusPassed,
				EarnedPoints:   10,
				PassedAt:       &fixedTime,
				TriggerEventID: &eventID,
				AttemptCount:   1,
				LastAttemptAt:  &fixedTime,
				Feedback:       "Well done!",
			},
			mockFn: func(mock sqlmock.Sqlmock, progress *models.CheckpointProgress) {
				mock.ExpectExec(`UPDATE checkpoint_progress SET`).
					WithArgs(
						progress.SessionID,
						progress.CheckpointID,
						progress.Status,
						progress.EarnedPoints,
						progress.PassedAt,
						progress.TriggerEventID,
						progress.AttemptCount,
						progress.LastAttemptAt,
						sqlmock.AnyArg(),
					).
					WillReturnResult(sqlmock.NewResult(0, 1))
			},
			wantErr: false,
		},
		{
			name: "database error",
			progress: &models.CheckpointProgress{
				SessionID:    "session-1",
				CheckpointID: "checkpoint-err",
				Status:       models.CheckpointStatusFailed,
			},
			mockFn: func(mock sqlmock.Sqlmock, progress *models.CheckpointProgress) {
				mock.ExpectExec(`UPDATE checkpoint_progress SET`).
					WillReturnError(errors.New("constraint violation"))
			},
			wantErr: true,
			errMsg:  "updating checkpoint progress",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatalf("failed to create sqlmock: %v", err)
			}
			defer db.Close()

			repo := NewCheckpointProgressRepo(db)
			tt.mockFn(mock, tt.progress)

			err = repo.Update(context.Background(), tt.progress)

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

func TestCheckpointProgressRepo_MarkPassed(t *testing.T) {
	validEventID := "42"
	nonNumericEventID := "event-abc"

	tests := []struct {
		name           string
		sessionID      string
		checkpointID   string
		triggerEventID *string
		mockFn         func(mock sqlmock.Sqlmock)
		wantErr        bool
		errMsg         string
	}{
		{
			name:           "success without trigger event",
			sessionID:      "session-1",
			checkpointID:   "checkpoint-1",
			triggerEventID: nil,
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(`UPDATE checkpoint_progress SET status = 'passed'`).
					WithArgs("session-1", "checkpoint-1", sqlmock.AnyArg(), sqlmock.AnyArg()).
					WillReturnResult(sqlmock.NewResult(0, 1))
			},
			wantErr: false,
		},
		{
			name:           "success with valid numeric trigger event ID",
			sessionID:      "session-1",
			checkpointID:   "checkpoint-2",
			triggerEventID: &validEventID,
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(`UPDATE checkpoint_progress SET status = 'passed'`).
					WithArgs("session-1", "checkpoint-2", sqlmock.AnyArg(), sqlmock.AnyArg()).
					WillReturnResult(sqlmock.NewResult(0, 1))
			},
			wantErr: false,
		},
		{
			name:           "success with non-numeric trigger event ID (graceful nil)",
			sessionID:      "session-1",
			checkpointID:   "checkpoint-3",
			triggerEventID: &nonNumericEventID,
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(`UPDATE checkpoint_progress SET status = 'passed'`).
					WithArgs("session-1", "checkpoint-3", sqlmock.AnyArg(), sqlmock.AnyArg()).
					WillReturnResult(sqlmock.NewResult(0, 1))
			},
			wantErr: false,
		},
		{
			name:         "database error",
			sessionID:    "session-1",
			checkpointID: "checkpoint-err",
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(`UPDATE checkpoint_progress SET status = 'passed'`).
					WillReturnError(errors.New("database error"))
			},
			wantErr: true,
			errMsg:  "marking checkpoint passed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatalf("failed to create sqlmock: %v", err)
			}
			defer db.Close()

			repo := NewCheckpointProgressRepo(db)
			tt.mockFn(mock)

			err = repo.MarkPassed(context.Background(), tt.sessionID, tt.checkpointID, tt.triggerEventID)

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

func TestCheckpointProgressRepo_MarkFailed(t *testing.T) {
	tests := []struct {
		name         string
		sessionID    string
		checkpointID string
		mockFn       func(mock sqlmock.Sqlmock)
		wantErr      bool
		errMsg       string
	}{
		{
			name:         "success",
			sessionID:    "session-1",
			checkpointID: "checkpoint-1",
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(`UPDATE checkpoint_progress SET status = 'failed'`).
					WithArgs("session-1", "checkpoint-1", sqlmock.AnyArg()).
					WillReturnResult(sqlmock.NewResult(0, 1))
			},
			wantErr: false,
		},
		{
			name:         "database error",
			sessionID:    "session-1",
			checkpointID: "checkpoint-err",
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(`UPDATE checkpoint_progress SET status = 'failed'`).
					WillReturnError(errors.New("database error"))
			},
			wantErr: true,
			errMsg:  "marking checkpoint failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatalf("failed to create sqlmock: %v", err)
			}
			defer db.Close()

			repo := NewCheckpointProgressRepo(db)
			tt.mockFn(mock)

			err = repo.MarkFailed(context.Background(), tt.sessionID, tt.checkpointID)

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

func TestCheckpointProgressRepo_ResetForSession(t *testing.T) {
	tests := []struct {
		name      string
		sessionID string
		mockFn    func(mock sqlmock.Sqlmock)
		wantErr   bool
		errMsg    string
	}{
		{
			name:      "success",
			sessionID: "session-1",
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(`UPDATE checkpoint_progress SET status = 'pending'`).
					WithArgs("session-1").
					WillReturnResult(sqlmock.NewResult(0, 3))
			},
			wantErr: false,
		},
		{
			name:      "success - no checkpoints to reset",
			sessionID: "session-empty",
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(`UPDATE checkpoint_progress SET status = 'pending'`).
					WithArgs("session-empty").
					WillReturnResult(sqlmock.NewResult(0, 0))
			},
			wantErr: false,
		},
		{
			name:      "database error",
			sessionID: "session-err",
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(`UPDATE checkpoint_progress SET status = 'pending'`).
					WillReturnError(errors.New("database error"))
			},
			wantErr: true,
			errMsg:  "resetting checkpoints",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatalf("failed to create sqlmock: %v", err)
			}
			defer db.Close()

			repo := NewCheckpointProgressRepo(db)
			tt.mockFn(mock)

			err = repo.ResetForSession(context.Background(), tt.sessionID)

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
