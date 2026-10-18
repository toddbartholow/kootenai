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

func TestQuestionResponseRepo_Create(t *testing.T) {
	fixedTime := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)

	tests := []struct {
		name     string
		response *models.QuestionResponse
		mockFn   func(mock sqlmock.Sqlmock, response *models.QuestionResponse)
		wantErr  bool
		errMsg   string
	}{
		{
			name: "success text question",
			response: &models.QuestionResponse{
				ID:           "qr-1",
				SessionID:    "session-1",
				QuestionID:   "q1",
				QuestionType: "text",
				Points:       10,
				Status:       models.QuestionStatusPending,
			},
			mockFn: func(mock sqlmock.Sqlmock, response *models.QuestionResponse) {
				rows := sqlmock.NewRows([]string{"created_at", "updated_at"}).
					AddRow(fixedTime, fixedTime)
				mock.ExpectQuery(`INSERT INTO question_responses`).
					WithArgs(
						response.ID,
						response.SessionID,
						response.QuestionID,
						response.QuestionType,
						response.Points,
						sqlmock.AnyArg(), // response_text
						sqlmock.AnyArg(), // selected_options
						response.Status,
						response.EarnedPoints,
						response.IsCorrect,
						response.AttemptCount,
						sqlmock.AnyArg(), // first_attempt_at
						sqlmock.AnyArg(), // last_attempt_at
						sqlmock.AnyArg(), // correct_at
						sqlmock.AnyArg(), // feedback
						response.HintLevelShown,
						response.HintPenaltyApplied,
					).
					WillReturnRows(rows)
			},
			wantErr: false,
		},
		{
			name: "success multiple choice question",
			response: &models.QuestionResponse{
				ID:              "qr-2",
				SessionID:       "session-1",
				QuestionID:      "q2",
				QuestionType:    "multiple_choice",
				Points:          15,
				SelectedOptions: []string{"A", "C"},
				Status:          models.QuestionStatusPending,
			},
			mockFn: func(mock sqlmock.Sqlmock, response *models.QuestionResponse) {
				rows := sqlmock.NewRows([]string{"created_at", "updated_at"}).
					AddRow(fixedTime, fixedTime)
				mock.ExpectQuery(`INSERT INTO question_responses`).
					WithArgs(
						response.ID,
						response.SessionID,
						response.QuestionID,
						response.QuestionType,
						response.Points,
						sqlmock.AnyArg(),
						sqlmock.AnyArg(),
						response.Status,
						response.EarnedPoints,
						response.IsCorrect,
						response.AttemptCount,
						sqlmock.AnyArg(),
						sqlmock.AnyArg(),
						sqlmock.AnyArg(),
						sqlmock.AnyArg(),
						response.HintLevelShown,
						response.HintPenaltyApplied,
					).
					WillReturnRows(rows)
			},
			wantErr: false,
		},
		{
			name: "database error",
			response: &models.QuestionResponse{
				ID:           "qr-err",
				SessionID:    "session-1",
				QuestionID:   "q1",
				QuestionType: "text",
				Points:       10,
				Status:       models.QuestionStatusPending,
			},
			mockFn: func(mock sqlmock.Sqlmock, response *models.QuestionResponse) {
				mock.ExpectQuery(`INSERT INTO question_responses`).
					WillReturnError(errors.New("duplicate key"))
			},
			wantErr: true,
			errMsg:  "inserting question response",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatalf("failed to create sqlmock: %v", err)
			}
			defer db.Close()

			repo := NewQuestionResponseRepo(db)
			tt.mockFn(mock, tt.response)

			err = repo.Create(context.Background(), tt.response)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error but got nil")
				} else if tt.errMsg != "" && !strContains(err.Error(), tt.errMsg) {
					t.Errorf("error message should contain %q, got %q", tt.errMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if tt.response.CreatedAt.IsZero() {
					t.Error("CreatedAt should be set")
				}
			}

			if err := mock.ExpectationsWereMet(); err != nil {
				t.Errorf("unfulfilled expectations: %v", err)
			}
		})
	}
}

func TestQuestionResponseRepo_GetBySessionID(t *testing.T) {
	fixedTime := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)
	selectedOptions, _ := json.Marshal([]string{"A", "B"})

	tests := []struct {
		name      string
		sessionID string
		mockFn    func(mock sqlmock.Sqlmock)
		wantCount int
		wantErr   bool
	}{
		{
			name:      "success with results",
			sessionID: "session-1",
			mockFn: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{
					"id", "session_id", "question_id", "question_type", "points",
					"response_text", "selected_options", "status", "earned_points",
					"is_correct", "attempt_count", "first_attempt_at", "last_attempt_at",
					"correct_at", "feedback", "hint_level_shown", "hint_penalty_applied", "created_at", "updated_at",
				}).
					AddRow(
						"qr-1", "session-1", "q1", "text", 10,
						sql.NullString{String: "root", Valid: true}, nil, "correct", 10,
						true, 1, fixedTime, fixedTime,
						fixedTime, sql.NullString{String: "Correct!", Valid: true}, 0, 0, fixedTime, fixedTime,
					).
					AddRow(
						"qr-2", "session-1", "q2", "multiple_choice", 15,
						sql.NullString{}, selectedOptions, "pending", 0,
						false, 0, nil, nil,
						nil, sql.NullString{}, 0, 0, fixedTime, fixedTime,
					)
				mock.ExpectQuery(`SELECT .+ FROM question_responses WHERE session_id = \$1`).
					WithArgs("session-1").
					WillReturnRows(rows)
			},
			wantCount: 2,
			wantErr:   false,
		},
		{
			name:      "success empty results",
			sessionID: "session-empty",
			mockFn: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{
					"id", "session_id", "question_id", "question_type", "points",
					"response_text", "selected_options", "status", "earned_points",
					"is_correct", "attempt_count", "first_attempt_at", "last_attempt_at",
					"correct_at", "feedback", "hint_level_shown", "hint_penalty_applied", "created_at", "updated_at",
				})
				mock.ExpectQuery(`SELECT .+ FROM question_responses WHERE session_id = \$1`).
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
				mock.ExpectQuery(`SELECT .+ FROM question_responses WHERE session_id = \$1`).
					WillReturnError(errors.New("connection refused"))
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

			repo := NewQuestionResponseRepo(db)
			tt.mockFn(mock)

			responses, err := repo.GetBySessionID(context.Background(), tt.sessionID)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error but got nil")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if len(responses) != tt.wantCount {
					t.Errorf("expected %d responses, got %d", tt.wantCount, len(responses))
				}
			}

			if err := mock.ExpectationsWereMet(); err != nil {
				t.Errorf("unfulfilled expectations: %v", err)
			}
		})
	}
}

func TestQuestionResponseRepo_GetBySessionAndQuestion(t *testing.T) {
	fixedTime := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)

	tests := []struct {
		name       string
		sessionID  string
		questionID string
		mockFn     func(mock sqlmock.Sqlmock)
		wantNil    bool
		wantErr    bool
	}{
		{
			name:       "success found",
			sessionID:  "session-1",
			questionID: "q1",
			mockFn: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{
					"id", "session_id", "question_id", "question_type", "points",
					"response_text", "selected_options", "status", "earned_points",
					"is_correct", "attempt_count", "first_attempt_at", "last_attempt_at",
					"correct_at", "feedback", "hint_level_shown", "hint_penalty_applied", "created_at", "updated_at",
				}).AddRow(
					"qr-1", "session-1", "q1", "text", 10,
					sql.NullString{String: "root", Valid: true}, nil, "correct", 10,
					true, 1, fixedTime, fixedTime,
					fixedTime, sql.NullString{}, 0, 0, fixedTime, fixedTime,
				)
				mock.ExpectQuery(`SELECT .+ FROM question_responses WHERE session_id = \$1 AND question_id = \$2`).
					WithArgs("session-1", "q1").
					WillReturnRows(rows)
			},
			wantNil: false,
			wantErr: false,
		},
		{
			name:       "not found",
			sessionID:  "session-1",
			questionID: "q-not-found",
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`SELECT .+ FROM question_responses WHERE session_id = \$1 AND question_id = \$2`).
					WithArgs("session-1", "q-not-found").
					WillReturnError(sql.ErrNoRows)
			},
			wantNil: true,
			wantErr: false,
		},
		{
			name:       "database error",
			sessionID:  "session-1",
			questionID: "q1",
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`SELECT .+ FROM question_responses WHERE session_id = \$1 AND question_id = \$2`).
					WillReturnError(errors.New("connection refused"))
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

			repo := NewQuestionResponseRepo(db)
			tt.mockFn(mock)

			response, err := repo.GetBySessionAndQuestion(context.Background(), tt.sessionID, tt.questionID)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error but got nil")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if tt.wantNil && response != nil {
					t.Error("expected nil response")
				}
				if !tt.wantNil && response == nil {
					t.Error("expected non-nil response")
				}
			}

			if err := mock.ExpectationsWereMet(); err != nil {
				t.Errorf("unfulfilled expectations: %v", err)
			}
		})
	}
}

func TestQuestionResponseRepo_Update(t *testing.T) {
	tests := []struct {
		name     string
		response *models.QuestionResponse
		mockFn   func(mock sqlmock.Sqlmock, response *models.QuestionResponse)
		wantErr  bool
	}{
		{
			name: "success",
			response: &models.QuestionResponse{
				ID:           "qr-1",
				SessionID:    "session-1",
				QuestionID:   "q1",
				QuestionType: "text",
				Points:       10,
				ResponseText: "root",
				Status:       models.QuestionStatusCorrect,
				EarnedPoints: 10,
				IsCorrect:    true,
				AttemptCount: 1,
			},
			mockFn: func(mock sqlmock.Sqlmock, response *models.QuestionResponse) {
				mock.ExpectExec(`UPDATE question_responses`).
					WithArgs(
						response.SessionID,
						response.QuestionID,
						sqlmock.AnyArg(), // response_text
						sqlmock.AnyArg(), // selected_options
						response.Status,
						response.EarnedPoints,
						response.IsCorrect,
						response.AttemptCount,
						sqlmock.AnyArg(), // last_attempt_at
						sqlmock.AnyArg(), // correct_at
						sqlmock.AnyArg(), // feedback
						response.HintLevelShown,
						response.HintPenaltyApplied,
					).
					WillReturnResult(sqlmock.NewResult(0, 1))
			},
			wantErr: false,
		},
		{
			name: "database error",
			response: &models.QuestionResponse{
				ID:         "qr-not-found",
				SessionID:  "session-1",
				QuestionID: "q1",
				Status:     models.QuestionStatusPending,
			},
			mockFn: func(mock sqlmock.Sqlmock, response *models.QuestionResponse) {
				mock.ExpectExec(`UPDATE question_responses`).
					WillReturnError(errors.New("connection error"))
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

			repo := NewQuestionResponseRepo(db)
			tt.mockFn(mock, tt.response)

			err = repo.Update(context.Background(), tt.response)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error but got nil")
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

func TestQuestionResponseRepo_MarkCorrect(t *testing.T) {
	tests := []struct {
		name         string
		sessionID    string
		questionID   string
		earnedPoints int
		feedback     string
		mockFn       func(mock sqlmock.Sqlmock)
		wantErr      bool
	}{
		{
			name:         "success",
			sessionID:    "session-1",
			questionID:   "q1",
			earnedPoints: 10,
			feedback:     "Great job!",
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(`UPDATE question_responses SET status = 'correct'`).
					WithArgs("session-1", "q1", 10, sqlmock.AnyArg(), sqlmock.AnyArg()).
					WillReturnResult(sqlmock.NewResult(0, 1))
			},
			wantErr: false,
		},
		{
			name:         "database error",
			sessionID:    "session-1",
			questionID:   "q-not-found",
			earnedPoints: 10,
			feedback:     "",
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(`UPDATE question_responses SET status = 'correct'`).
					WillReturnError(errors.New("connection error"))
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

			repo := NewQuestionResponseRepo(db)
			tt.mockFn(mock)

			err = repo.MarkCorrect(context.Background(), tt.sessionID, tt.questionID, tt.earnedPoints, tt.feedback)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error but got nil")
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

func TestQuestionResponseRepo_ShowHintLevel(t *testing.T) {
	tests := []struct {
		name       string
		sessionID  string
		questionID string
		level      int
		penalty    int
		mockFn     func(mock sqlmock.Sqlmock)
		wantErr    bool
	}{
		{
			name:       "success",
			sessionID:  "session-1",
			questionID: "q1",
			level:      1,
			penalty:    5,
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(`UPDATE question_responses`).
					WithArgs("session-1", "q1", 1, 5).
					WillReturnResult(sqlmock.NewResult(0, 1))
			},
			wantErr: false,
		},
		{
			name:       "database error",
			sessionID:  "session-1",
			questionID: "q-not-found",
			level:      2,
			penalty:    10,
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(`UPDATE question_responses`).
					WillReturnError(errors.New("connection error"))
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

			repo := NewQuestionResponseRepo(db)
			tt.mockFn(mock)

			err = repo.ShowHintLevel(context.Background(), tt.sessionID, tt.questionID, tt.level, tt.penalty)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error but got nil")
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

func TestQuestionResponseRepo_GetSessionQuestionStats(t *testing.T) {
	tests := []struct {
		name       string
		sessionID  string
		mockFn     func(mock sqlmock.Sqlmock)
		wantEarned int
		wantTotal  int
		wantErr    bool
	}{
		{
			name:      "success with points",
			sessionID: "session-1",
			mockFn: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{"earned_points", "total_points"}).
					AddRow(30, 50)
				mock.ExpectQuery(`SELECT COALESCE\(SUM\(earned_points\), 0\), COALESCE\(SUM\(points\), 0\)`).
					WithArgs("session-1").
					WillReturnRows(rows)
			},
			wantEarned: 30,
			wantTotal:  50,
			wantErr:    false,
		},
		{
			name:      "success no responses",
			sessionID: "session-empty",
			mockFn: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{"earned_points", "total_points"}).
					AddRow(0, 0)
				mock.ExpectQuery(`SELECT COALESCE\(SUM\(earned_points\), 0\), COALESCE\(SUM\(points\), 0\)`).
					WithArgs("session-empty").
					WillReturnRows(rows)
			},
			wantEarned: 0,
			wantTotal:  0,
			wantErr:    false,
		},
		{
			name:      "database error",
			sessionID: "session-err",
			mockFn: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`SELECT COALESCE\(SUM\(earned_points\), 0\), COALESCE\(SUM\(points\), 0\)`).
					WillReturnError(errors.New("connection refused"))
			},
			wantEarned: 0,
			wantTotal:  0,
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

			repo := NewQuestionResponseRepo(db)
			tt.mockFn(mock)

			earned, total, err := repo.GetSessionQuestionStats(context.Background(), tt.sessionID)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error but got nil")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if earned != tt.wantEarned {
					t.Errorf("expected earned %d, got %d", tt.wantEarned, earned)
				}
				if total != tt.wantTotal {
					t.Errorf("expected total %d, got %d", tt.wantTotal, total)
				}
			}

			if err := mock.ExpectationsWereMet(); err != nil {
				t.Errorf("unfulfilled expectations: %v", err)
			}
		})
	}
}

// strContains is a local helper to avoid conflicts with other test files
func strContains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
