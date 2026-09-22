package repositories

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/google/uuid"

	"github.com/toddbartholow/kootenai/api/internal/models"
)

// CheckpointHintRepo implements checkpoint hint progress storage
type CheckpointHintRepo struct {
	db DBTX
}

// NewCheckpointHintRepo creates a new checkpoint hint repository
func NewCheckpointHintRepo(db DBTX) *CheckpointHintRepo {
	return &CheckpointHintRepo{db: db}
}

// GetBySessionID retrieves all checkpoint hint progress for a session
func (r *CheckpointHintRepo) GetBySessionID(ctx context.Context, sessionID string) ([]*models.CheckpointHintProgress, error) {
	query := `
		SELECT id, session_id, checkpoint_id, hint_level_shown, hint_penalty_applied,
		       created_at, updated_at
		FROM checkpoint_hint_progress
		WHERE session_id = $1
		ORDER BY created_at ASC`

	rows, err := r.db.QueryContext(ctx, query, sessionID)
	if err != nil {
		return nil, fmt.Errorf("querying checkpoint hint progress: %w", err)
	}
	defer rows.Close()

	var results []*models.CheckpointHintProgress
	for rows.Next() {
		var p models.CheckpointHintProgress
		if err := rows.Scan(
			&p.ID,
			&p.SessionID,
			&p.CheckpointID,
			&p.HintLevelShown,
			&p.HintPenaltyApplied,
			&p.CreatedAt,
			&p.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scanning checkpoint hint progress: %w", err)
		}
		results = append(results, &p)
	}

	return results, rows.Err()
}

// GetHintLevel returns the current hint level shown for a checkpoint
func (r *CheckpointHintRepo) GetHintLevel(ctx context.Context, sessionID, checkpointID string) (level int, penalty int, err error) {
	query := `
		SELECT hint_level_shown, hint_penalty_applied
		FROM checkpoint_hint_progress
		WHERE session_id = $1 AND checkpoint_id = $2`

	err = r.db.QueryRowContext(ctx, query, sessionID, checkpointID).Scan(&level, &penalty)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return 0, 0, nil
		}
		return 0, 0, fmt.Errorf("getting checkpoint hint level: %w", err)
	}

	return level, penalty, nil
}

// ShowHintLevel records that a hint level was shown and applies the penalty
func (r *CheckpointHintRepo) ShowHintLevel(ctx context.Context, sessionID, checkpointID string, level, penalty int) error {
	// Upsert: insert or update the hint progress
	query := `
		INSERT INTO checkpoint_hint_progress (id, session_id, checkpoint_id, hint_level_shown, hint_penalty_applied)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (session_id, checkpoint_id)
		DO UPDATE SET
			hint_level_shown = GREATEST(checkpoint_hint_progress.hint_level_shown, EXCLUDED.hint_level_shown),
			hint_penalty_applied = CASE
				WHEN checkpoint_hint_progress.hint_level_shown < EXCLUDED.hint_level_shown
				THEN checkpoint_hint_progress.hint_penalty_applied + EXCLUDED.hint_penalty_applied
				ELSE checkpoint_hint_progress.hint_penalty_applied
			END`

	_, err := r.db.ExecContext(ctx, query, uuid.New().String(), sessionID, checkpointID, level, penalty)
	if err != nil {
		return fmt.Errorf("updating checkpoint hint level: %w", err)
	}

	return nil
}

// Create inserts a new checkpoint hint progress record
func (r *CheckpointHintRepo) Create(ctx context.Context, progress *models.CheckpointHintProgress) error {
	if progress.ID == "" {
		progress.ID = uuid.New().String()
	}

	query := `
		INSERT INTO checkpoint_hint_progress (id, session_id, checkpoint_id, hint_level_shown, hint_penalty_applied)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING created_at, updated_at`

	err := r.db.QueryRowContext(ctx, query,
		progress.ID,
		progress.SessionID,
		progress.CheckpointID,
		progress.HintLevelShown,
		progress.HintPenaltyApplied,
	).Scan(&progress.CreatedAt, &progress.UpdatedAt)

	if err != nil {
		return fmt.Errorf("inserting checkpoint hint progress: %w", err)
	}

	return nil
}
