package repositories

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/toddbartholow/kootenai/api/internal/models"
)

// CheckpointProgressRepo implements CheckpointProgressRepository
type CheckpointProgressRepo struct {
	db DBTX
}

// NewCheckpointProgressRepo creates a new checkpoint progress repository
func NewCheckpointProgressRepo(db DBTX) *CheckpointProgressRepo {
	return &CheckpointProgressRepo{db: db}
}

// Create inserts a new checkpoint progress record
func (r *CheckpointProgressRepo) Create(ctx context.Context, progress *models.CheckpointProgress) error {
	query := `
		INSERT INTO checkpoint_progress (
			id, session_id, checkpoint_id, status, points, earned_points, feedback
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING created_at, updated_at`

	err := r.db.QueryRowContext(ctx, query,
		progress.ID,
		progress.SessionID,
		progress.CheckpointID,
		progress.Status,
		progress.Points,
		progress.EarnedPoints,
		nullString(progress.Feedback),
	).Scan(&progress.CreatedAt, &progress.UpdatedAt)

	if err != nil {
		return fmt.Errorf("inserting checkpoint progress: %w", err)
	}

	return nil
}

// GetBySessionID retrieves all checkpoint progress for a session
func (r *CheckpointProgressRepo) GetBySessionID(ctx context.Context, sessionID string) ([]*models.CheckpointProgress, error) {
	query := `
		SELECT id, session_id, checkpoint_id, status, points, earned_points,
		       passed_at, triggered_by_event_id, attempt_count, last_attempt_at,
		       feedback, created_at, updated_at
		FROM checkpoint_progress
		WHERE session_id = $1
		ORDER BY created_at ASC`

	rows, err := r.db.QueryContext(ctx, query, sessionID)
	if err != nil {
		return nil, fmt.Errorf("querying checkpoint progress: %w", err)
	}
	defer rows.Close()

	return r.scanMultiple(rows)
}

// GetBySessionAndCheckpoint retrieves a specific checkpoint progress
func (r *CheckpointProgressRepo) GetBySessionAndCheckpoint(ctx context.Context, sessionID, checkpointID string) (*models.CheckpointProgress, error) {
	query := `
		SELECT id, session_id, checkpoint_id, status, points, earned_points,
		       passed_at, triggered_by_event_id, attempt_count, last_attempt_at,
		       feedback, created_at, updated_at
		FROM checkpoint_progress
		WHERE session_id = $1 AND checkpoint_id = $2`

	row := r.db.QueryRowContext(ctx, query, sessionID, checkpointID)
	progress, err := r.scanProgress(row)
	if err != nil {
		// Check for no rows using errors.Is (handles wrapped errors)
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return progress, nil
}

// Update updates a checkpoint progress record
func (r *CheckpointProgressRepo) Update(ctx context.Context, progress *models.CheckpointProgress) error {
	query := `
		UPDATE checkpoint_progress
		SET status = $3, earned_points = $4, passed_at = $5,
		    triggered_by_event_id = $6, attempt_count = $7,
		    last_attempt_at = $8, feedback = $9
		WHERE session_id = $1 AND checkpoint_id = $2`

	res, err := r.db.ExecContext(ctx, query,
		progress.SessionID,
		progress.CheckpointID,
		progress.Status,
		progress.EarnedPoints,
		progress.PassedAt,
		progress.TriggerEventID,
		progress.AttemptCount,
		progress.LastAttemptAt,
		nullString(progress.Feedback),
	)

	if err != nil {
		return fmt.Errorf("updating checkpoint progress: %w", err)
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("checkpoint progress not found: session=%s checkpoint=%s", progress.SessionID, progress.CheckpointID)
	}

	return nil
}

// MarkPassed marks a checkpoint as passed
func (r *CheckpointProgressRepo) MarkPassed(ctx context.Context, sessionID, checkpointID string, triggerEventID *string) error {
	now := time.Now()
	query := `
		UPDATE checkpoint_progress
		SET status = 'passed',
		    earned_points = points,
		    passed_at = $3,
		    triggered_by_event_id = $4,
		    attempt_count = attempt_count + 1,
		    last_attempt_at = $3
		WHERE session_id = $1 AND checkpoint_id = $2`

	var eventID sql.NullInt64
	if triggerEventID != nil {
		if id, err := strconv.ParseInt(*triggerEventID, 10, 64); err == nil {
			eventID = sql.NullInt64{Int64: id, Valid: true}
		}
	}

	res, err := r.db.ExecContext(ctx, query, sessionID, checkpointID, now, eventID)
	if err != nil {
		return fmt.Errorf("marking checkpoint passed: %w", err)
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("checkpoint progress not found: session=%s checkpoint=%s", sessionID, checkpointID)
	}

	return nil
}

// MarkFailed marks a checkpoint as failed
func (r *CheckpointProgressRepo) MarkFailed(ctx context.Context, sessionID, checkpointID string) error {
	now := time.Now()
	query := `
		UPDATE checkpoint_progress
		SET status = 'failed',
		    attempt_count = attempt_count + 1,
		    last_attempt_at = $3
		WHERE session_id = $1 AND checkpoint_id = $2`

	res, err := r.db.ExecContext(ctx, query, sessionID, checkpointID, now)
	if err != nil {
		return fmt.Errorf("marking checkpoint failed: %w", err)
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("checkpoint progress not found: session=%s checkpoint=%s", sessionID, checkpointID)
	}

	return nil
}

// ResetForSession resets all checkpoints for a session
func (r *CheckpointProgressRepo) ResetForSession(ctx context.Context, sessionID string) error {
	query := `
		UPDATE checkpoint_progress
		SET status = 'pending',
		    earned_points = 0,
		    passed_at = NULL,
		    triggered_by_event_id = NULL,
		    attempt_count = 0,
		    last_attempt_at = NULL,
		    feedback = NULL
		WHERE session_id = $1`

	_, err := r.db.ExecContext(ctx, query, sessionID)
	if err != nil {
		return fmt.Errorf("resetting checkpoints: %w", err)
	}

	return nil
}

// Helper methods

func (r *CheckpointProgressRepo) scanMultiple(rows *sql.Rows) ([]*models.CheckpointProgress, error) {
	var progressList []*models.CheckpointProgress
	for rows.Next() {
		progress, err := r.scanProgressRow(rows)
		if err != nil {
			return nil, err
		}
		progressList = append(progressList, progress)
	}
	return progressList, rows.Err()
}

type scannableProgress interface {
	Scan(dest ...any) error
}

func (r *CheckpointProgressRepo) scanProgress(row scannableProgress) (*models.CheckpointProgress, error) {
	var progress models.CheckpointProgress
	var triggeredByEventID sql.NullInt64
	var feedback sql.NullString

	err := row.Scan(
		&progress.ID,
		&progress.SessionID,
		&progress.CheckpointID,
		&progress.Status,
		&progress.Points,
		&progress.EarnedPoints,
		&progress.PassedAt,
		&triggeredByEventID,
		&progress.AttemptCount,
		&progress.LastAttemptAt,
		&feedback,
		&progress.CreatedAt,
		&progress.UpdatedAt,
	)

	if err != nil {
		return nil, fmt.Errorf("scanning checkpoint progress: %w", err)
	}

	if triggeredByEventID.Valid {
		id := fmt.Sprintf("%d", triggeredByEventID.Int64)
		progress.TriggerEventID = &id
	}
	progress.Feedback = feedback.String

	return &progress, nil
}

func (r *CheckpointProgressRepo) scanProgressRow(rows *sql.Rows) (*models.CheckpointProgress, error) {
	return r.scanProgress(rows)
}
