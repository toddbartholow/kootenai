package repositories

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/toddbartholow/kootenai/api/internal/models"
)

// QuestionResponseRepo implements question response storage
type QuestionResponseRepo struct {
	db DBTX
}

// NewQuestionResponseRepo creates a new question response repository
func NewQuestionResponseRepo(db DBTX) *QuestionResponseRepo {
	return &QuestionResponseRepo{db: db}
}

// Create inserts a new question response record
func (r *QuestionResponseRepo) Create(ctx context.Context, response *models.QuestionResponse) error {
	if response.ID == "" {
		response.ID = uuid.New().String()
	}

	selectedOptionsJSON, err := json.Marshal(response.SelectedOptions)
	if err != nil {
		return fmt.Errorf("marshaling selected options: %w", err)
	}

	query := `
		INSERT INTO question_responses (
			id, session_id, question_id, question_type, points,
			response_text, selected_options, status, earned_points, is_correct,
			attempt_count, first_attempt_at, last_attempt_at, correct_at, feedback,
			hint_level_shown, hint_penalty_applied
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17)
		RETURNING created_at, updated_at`

	err = r.db.QueryRowContext(ctx, query,
		response.ID,
		response.SessionID,
		response.QuestionID,
		response.QuestionType,
		response.Points,
		nullString(response.ResponseText),
		selectedOptionsJSON,
		response.Status,
		response.EarnedPoints,
		response.IsCorrect,
		response.AttemptCount,
		response.FirstAttemptAt,
		response.LastAttemptAt,
		response.CorrectAt,
		nullString(response.Feedback),
		response.HintLevelShown,
		response.HintPenaltyApplied,
	).Scan(&response.CreatedAt, &response.UpdatedAt)

	if err != nil {
		return fmt.Errorf("inserting question response: %w", err)
	}

	return nil
}

// GetBySessionID retrieves all question responses for a session
func (r *QuestionResponseRepo) GetBySessionID(ctx context.Context, sessionID string) ([]*models.QuestionResponse, error) {
	query := `
		SELECT id, session_id, question_id, question_type, points,
		       response_text, selected_options, status, earned_points, is_correct,
		       attempt_count, first_attempt_at, last_attempt_at, correct_at,
		       feedback, hint_level_shown, hint_penalty_applied, created_at, updated_at
		FROM question_responses
		WHERE session_id = $1
		ORDER BY created_at ASC`

	rows, err := r.db.QueryContext(ctx, query, sessionID)
	if err != nil {
		return nil, fmt.Errorf("querying question responses: %w", err)
	}
	defer rows.Close()

	return r.scanMultiple(rows)
}

// GetBySessionAndQuestion retrieves a specific question response
func (r *QuestionResponseRepo) GetBySessionAndQuestion(ctx context.Context, sessionID, questionID string) (*models.QuestionResponse, error) {
	query := `
		SELECT id, session_id, question_id, question_type, points,
		       response_text, selected_options, status, earned_points, is_correct,
		       attempt_count, first_attempt_at, last_attempt_at, correct_at,
		       feedback, hint_level_shown, hint_penalty_applied, created_at, updated_at
		FROM question_responses
		WHERE session_id = $1 AND question_id = $2`

	row := r.db.QueryRowContext(ctx, query, sessionID, questionID)
	response, err := r.scanResponse(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return response, nil
}

// Update updates a question response record
func (r *QuestionResponseRepo) Update(ctx context.Context, response *models.QuestionResponse) error {
	selectedOptionsJSON, err := json.Marshal(response.SelectedOptions)
	if err != nil {
		return fmt.Errorf("marshaling selected options: %w", err)
	}

	query := `
		UPDATE question_responses
		SET response_text = $3, selected_options = $4, status = $5,
		    earned_points = $6, is_correct = $7, attempt_count = $8,
		    last_attempt_at = $9, correct_at = $10, feedback = $11,
		    hint_level_shown = $12, hint_penalty_applied = $13
		WHERE session_id = $1 AND question_id = $2`

	_, err = r.db.ExecContext(ctx, query,
		response.SessionID,
		response.QuestionID,
		nullString(response.ResponseText),
		selectedOptionsJSON,
		response.Status,
		response.EarnedPoints,
		response.IsCorrect,
		response.AttemptCount,
		response.LastAttemptAt,
		response.CorrectAt,
		nullString(response.Feedback),
		response.HintLevelShown,
		response.HintPenaltyApplied,
	)

	if err != nil {
		return fmt.Errorf("updating question response: %w", err)
	}

	return nil
}

// MarkCorrect marks a question response as correct
func (r *QuestionResponseRepo) MarkCorrect(ctx context.Context, sessionID, questionID string, earnedPoints int, feedback string) error {
	now := time.Now()
	query := `
		UPDATE question_responses
		SET status = 'correct',
		    earned_points = $3,
		    is_correct = true,
		    correct_at = $4,
		    attempt_count = attempt_count + 1,
		    last_attempt_at = $4,
		    feedback = $5
		WHERE session_id = $1 AND question_id = $2`

	_, err := r.db.ExecContext(ctx, query, sessionID, questionID, earnedPoints, now, nullString(feedback))
	if err != nil {
		return fmt.Errorf("marking question correct: %w", err)
	}

	return nil
}

// MarkIncorrect marks a question response as incorrect
func (r *QuestionResponseRepo) MarkIncorrect(ctx context.Context, sessionID, questionID, feedback string) error {
	now := time.Now()
	query := `
		UPDATE question_responses
		SET status = 'incorrect',
		    is_correct = false,
		    attempt_count = attempt_count + 1,
		    last_attempt_at = $3,
		    feedback = $4
		WHERE session_id = $1 AND question_id = $2`

	_, err := r.db.ExecContext(ctx, query, sessionID, questionID, now, nullString(feedback))
	if err != nil {
		return fmt.Errorf("marking question incorrect: %w", err)
	}

	return nil
}

// ShowHintLevel records that a hint level was shown and applies the penalty
func (r *QuestionResponseRepo) ShowHintLevel(ctx context.Context, sessionID, questionID string, level, penalty int) error {
	query := `
		UPDATE question_responses
		SET hint_level_shown = GREATEST(hint_level_shown, $3),
		    hint_penalty_applied = hint_penalty_applied + $4
		WHERE session_id = $1 AND question_id = $2
		  AND hint_level_shown < $3`

	_, err := r.db.ExecContext(ctx, query, sessionID, questionID, level, penalty)
	if err != nil {
		return fmt.Errorf("updating hint level: %w", err)
	}

	return nil
}

// GetHintLevel returns the current hint level shown for a question
func (r *QuestionResponseRepo) GetHintLevel(ctx context.Context, sessionID, questionID string) (level int, penalty int, err error) {
	query := `
		SELECT hint_level_shown, hint_penalty_applied
		FROM question_responses
		WHERE session_id = $1 AND question_id = $2`

	err = r.db.QueryRowContext(ctx, query, sessionID, questionID).Scan(&level, &penalty)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return 0, 0, nil
		}
		return 0, 0, fmt.Errorf("getting hint level: %w", err)
	}

	return level, penalty, nil
}

// GetSessionQuestionStats returns the total and earned question points for a session
func (r *QuestionResponseRepo) GetSessionQuestionStats(ctx context.Context, sessionID string) (earned int, total int, err error) {
	query := `
		SELECT COALESCE(SUM(earned_points), 0), COALESCE(SUM(points), 0)
		FROM question_responses
		WHERE session_id = $1`

	err = r.db.QueryRowContext(ctx, query, sessionID).Scan(&earned, &total)
	if err != nil {
		return 0, 0, fmt.Errorf("getting question stats: %w", err)
	}

	return earned, total, nil
}

// InitializeForSession creates pending response records for all questions in a session
func (r *QuestionResponseRepo) InitializeForSession(ctx context.Context, sessionID string, questions []models.Question) error {
	for _, q := range questions {
		response := &models.QuestionResponse{
			SessionID:    sessionID,
			QuestionID:   q.ID,
			QuestionType: string(q.Type),
			Points:       q.Points,
			Status:       models.QuestionStatusPending,
		}

		if err := r.Create(ctx, response); err != nil {
			return fmt.Errorf("initializing question %s: %w", q.ID, err)
		}
	}

	return nil
}

// Helper methods

func (r *QuestionResponseRepo) scanMultiple(rows *sql.Rows) ([]*models.QuestionResponse, error) {
	var responses []*models.QuestionResponse
	for rows.Next() {
		response, err := r.scanResponseRow(rows)
		if err != nil {
			return nil, err
		}
		responses = append(responses, response)
	}
	return responses, rows.Err()
}

type scannableQuestionResponse interface {
	Scan(dest ...any) error
}

func (r *QuestionResponseRepo) scanResponse(row scannableQuestionResponse) (*models.QuestionResponse, error) {
	var response models.QuestionResponse
	var responseText sql.NullString
	var selectedOptionsJSON []byte
	var feedback sql.NullString

	err := row.Scan(
		&response.ID,
		&response.SessionID,
		&response.QuestionID,
		&response.QuestionType,
		&response.Points,
		&responseText,
		&selectedOptionsJSON,
		&response.Status,
		&response.EarnedPoints,
		&response.IsCorrect,
		&response.AttemptCount,
		&response.FirstAttemptAt,
		&response.LastAttemptAt,
		&response.CorrectAt,
		&feedback,
		&response.HintLevelShown,
		&response.HintPenaltyApplied,
		&response.CreatedAt,
		&response.UpdatedAt,
	)

	if err != nil {
		return nil, fmt.Errorf("scanning question response: %w", err)
	}

	response.ResponseText = responseText.String
	response.Feedback = feedback.String

	if len(selectedOptionsJSON) > 0 {
		if err := json.Unmarshal(selectedOptionsJSON, &response.SelectedOptions); err != nil {
			// Non-fatal: just leave SelectedOptions nil
			response.SelectedOptions = nil
		}
	}

	return &response, nil
}

func (r *QuestionResponseRepo) scanResponseRow(rows *sql.Rows) (*models.QuestionResponse, error) {
	return r.scanResponse(rows)
}
