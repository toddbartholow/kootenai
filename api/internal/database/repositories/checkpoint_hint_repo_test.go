package repositories

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"

	"github.com/toddbartholow/kootenai/api/internal/models"
)

func TestCheckpointHintRepo_GetBySessionID(t *testing.T) {
	fixedTime := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)

	tests := []struct {
		name      string
		sessionID string
		mockFn    func(mock sqlmock.Sqlmock)
		want      int // expected number of results
		wantErr   bool
	}{
		{
			name:      "returns multiple checkpoint hints",
			sessionID: "session-1",
			mockFn: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{
					"id", "session_id", "checkpoint_id", "hint_level_shown",
					"hint_penalty_applied", "created_at", "updated_at",
				}).
					AddRow("hint-1", "session-1", "cp-1", 2, 3, fixedTime, fixedTime).
					AddRow("hint-2", "session-1", "cp-2", 1, 0, fixedTime, fixedTime)
				mock.ExpectQuery(`SELECT .+ FROM checkpoint_hint_progress WHERE session_id = \$1`).
					WithArgs("session-1").
					WillReturnRows(rows)
			},
			want:    2,
			wantErr: false,
		},
		{
			name:      "returns empty for session with no hints",
			sessionID: "session-empty",
			mockFn: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{
					"id", "session_id", "checkpoint_id", "hint_level_shown",
					"hint_penalty_applied", "created_at", "updated_at",
				})
				mock.ExpectQuery(`SELECT .+ FROM checkpoint_hint_progress WHERE session_id = \$1`).
					WithArgs("session-empty").
					WillReturnRows(rows)
			},
			want:    0,
			wantErr: false,
		},
		{
			name:      "database error",
			sessionID: "session-error",
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`SELECT .+ FROM checkpoint_hint_progress WHERE session_id = \$1`).
					WithArgs("session-error").
					WillReturnError(sql.ErrConnDone)
			},
			want:    0,
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

			tt.mockFn(mock)

			repo := NewCheckpointHintRepo(db)
			results, err := repo.GetBySessionID(context.Background(), tt.sessionID)

			if (err != nil) != tt.wantErr {
				t.Errorf("GetBySessionID() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && len(results) != tt.want {
				t.Errorf("GetBySessionID() got %d results, want %d", len(results), tt.want)
			}

			if err := mock.ExpectationsWereMet(); err != nil {
				t.Errorf("unfulfilled expectations: %s", err)
			}
		})
	}
}

func TestCheckpointHintRepo_GetHintLevel(t *testing.T) {
	tests := []struct {
		name         string
		sessionID    string
		checkpointID string
		mockFn       func(mock sqlmock.Sqlmock)
		wantLevel    int
		wantPenalty  int
		wantErr      bool
	}{
		{
			name:         "returns existing hint level",
			sessionID:    "session-1",
			checkpointID: "cp-1",
			mockFn: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{"hint_level_shown", "hint_penalty_applied"}).
					AddRow(2, 5)
				mock.ExpectQuery(`SELECT hint_level_shown, hint_penalty_applied FROM checkpoint_hint_progress WHERE session_id = \$1 AND checkpoint_id = \$2`).
					WithArgs("session-1", "cp-1").
					WillReturnRows(rows)
			},
			wantLevel:   2,
			wantPenalty: 5,
			wantErr:     false,
		},
		{
			name:         "returns zero for no hint shown",
			sessionID:    "session-new",
			checkpointID: "cp-1",
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`SELECT hint_level_shown, hint_penalty_applied FROM checkpoint_hint_progress WHERE session_id = \$1 AND checkpoint_id = \$2`).
					WithArgs("session-new", "cp-1").
					WillReturnError(sql.ErrNoRows)
			},
			wantLevel:   0,
			wantPenalty: 0,
			wantErr:     false,
		},
		{
			name:         "database error",
			sessionID:    "session-error",
			checkpointID: "cp-1",
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`SELECT hint_level_shown, hint_penalty_applied FROM checkpoint_hint_progress WHERE session_id = \$1 AND checkpoint_id = \$2`).
					WithArgs("session-error", "cp-1").
					WillReturnError(sql.ErrConnDone)
			},
			wantLevel:   0,
			wantPenalty: 0,
			wantErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatalf("failed to create sqlmock: %v", err)
			}
			defer db.Close()

			tt.mockFn(mock)

			repo := NewCheckpointHintRepo(db)
			level, penalty, err := repo.GetHintLevel(context.Background(), tt.sessionID, tt.checkpointID)

			if (err != nil) != tt.wantErr {
				t.Errorf("GetHintLevel() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if level != tt.wantLevel {
				t.Errorf("GetHintLevel() level = %v, want %v", level, tt.wantLevel)
			}

			if penalty != tt.wantPenalty {
				t.Errorf("GetHintLevel() penalty = %v, want %v", penalty, tt.wantPenalty)
			}

			if err := mock.ExpectationsWereMet(); err != nil {
				t.Errorf("unfulfilled expectations: %s", err)
			}
		})
	}
}

func TestCheckpointHintRepo_ShowHintLevel(t *testing.T) {
	tests := []struct {
		name         string
		sessionID    string
		checkpointID string
		level        int
		penalty      int
		mockFn       func(mock sqlmock.Sqlmock)
		wantErr      bool
	}{
		{
			name:         "inserts new hint progress",
			sessionID:    "session-1",
			checkpointID: "cp-1",
			level:        1,
			penalty:      0,
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(`INSERT INTO checkpoint_hint_progress .+ ON CONFLICT .+ DO UPDATE SET`).
					WithArgs(
						sqlmock.AnyArg(), // id (UUID)
						"session-1",
						"cp-1",
						1, // level
						0, // penalty
					).
					WillReturnResult(sqlmock.NewResult(1, 1))
			},
			wantErr: false,
		},
		{
			name:         "updates existing hint progress with higher level",
			sessionID:    "session-1",
			checkpointID: "cp-1",
			level:        2,
			penalty:      3,
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(`INSERT INTO checkpoint_hint_progress .+ ON CONFLICT .+ DO UPDATE SET`).
					WithArgs(
						sqlmock.AnyArg(),
						"session-1",
						"cp-1",
						2,
						3,
					).
					WillReturnResult(sqlmock.NewResult(1, 1))
			},
			wantErr: false,
		},
		{
			name:         "database error",
			sessionID:    "session-error",
			checkpointID: "cp-1",
			level:        1,
			penalty:      0,
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(`INSERT INTO checkpoint_hint_progress .+ ON CONFLICT .+ DO UPDATE SET`).
					WithArgs(
						sqlmock.AnyArg(),
						"session-error",
						"cp-1",
						1,
						0,
					).
					WillReturnError(sql.ErrConnDone)
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

			tt.mockFn(mock)

			repo := NewCheckpointHintRepo(db)
			err = repo.ShowHintLevel(context.Background(), tt.sessionID, tt.checkpointID, tt.level, tt.penalty)

			if (err != nil) != tt.wantErr {
				t.Errorf("ShowHintLevel() error = %v, wantErr %v", err, tt.wantErr)
			}

			if err := mock.ExpectationsWereMet(); err != nil {
				t.Errorf("unfulfilled expectations: %s", err)
			}
		})
	}
}

func TestCheckpointHintRepo_Create(t *testing.T) {
	fixedTime := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)

	tests := []struct {
		name     string
		progress *models.CheckpointHintProgress
		mockFn   func(mock sqlmock.Sqlmock, progress *models.CheckpointHintProgress)
		wantErr  bool
	}{
		{
			name: "creates new checkpoint hint progress",
			progress: &models.CheckpointHintProgress{
				SessionID:          "session-1",
				CheckpointID:       "cp-1",
				HintLevelShown:     1,
				HintPenaltyApplied: 0,
			},
			mockFn: func(mock sqlmock.Sqlmock, progress *models.CheckpointHintProgress) {
				rows := sqlmock.NewRows([]string{"created_at", "updated_at"}).
					AddRow(fixedTime, fixedTime)
				mock.ExpectQuery(`INSERT INTO checkpoint_hint_progress`).
					WithArgs(
						sqlmock.AnyArg(), // ID (generated if empty)
						progress.SessionID,
						progress.CheckpointID,
						progress.HintLevelShown,
						progress.HintPenaltyApplied,
					).
					WillReturnRows(rows)
			},
			wantErr: false,
		},
		{
			name: "creates with provided ID",
			progress: &models.CheckpointHintProgress{
				ID:                 "hint-123",
				SessionID:          "session-1",
				CheckpointID:       "cp-2",
				HintLevelShown:     2,
				HintPenaltyApplied: 5,
			},
			mockFn: func(mock sqlmock.Sqlmock, progress *models.CheckpointHintProgress) {
				rows := sqlmock.NewRows([]string{"created_at", "updated_at"}).
					AddRow(fixedTime, fixedTime)
				mock.ExpectQuery(`INSERT INTO checkpoint_hint_progress`).
					WithArgs(
						"hint-123",
						progress.SessionID,
						progress.CheckpointID,
						progress.HintLevelShown,
						progress.HintPenaltyApplied,
					).
					WillReturnRows(rows)
			},
			wantErr: false,
		},
		{
			name: "database error",
			progress: &models.CheckpointHintProgress{
				SessionID:    "session-error",
				CheckpointID: "cp-1",
			},
			mockFn: func(mock sqlmock.Sqlmock, progress *models.CheckpointHintProgress) {
				mock.ExpectQuery(`INSERT INTO checkpoint_hint_progress`).
					WithArgs(
						sqlmock.AnyArg(),
						progress.SessionID,
						progress.CheckpointID,
						progress.HintLevelShown,
						progress.HintPenaltyApplied,
					).
					WillReturnError(sql.ErrConnDone)
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

			tt.mockFn(mock, tt.progress)

			repo := NewCheckpointHintRepo(db)
			err = repo.Create(context.Background(), tt.progress)

			if (err != nil) != tt.wantErr {
				t.Errorf("Create() error = %v, wantErr %v", err, tt.wantErr)
			}

			if !tt.wantErr {
				if tt.progress.ID == "" {
					t.Error("Create() should set ID if empty")
				}
				if tt.progress.CreatedAt.IsZero() {
					t.Error("Create() should set CreatedAt")
				}
			}

			if err := mock.ExpectationsWereMet(); err != nil {
				t.Errorf("unfulfilled expectations: %s", err)
			}
		})
	}
}
