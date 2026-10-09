package repositories

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/toddbartholow/kootenai/api/internal/models"
)

// GradeSyncRepo implements GradeSyncRepository
type GradeSyncRepo struct {
	db DBTX
}

// NewGradeSyncRepo creates a new grade sync repository
func NewGradeSyncRepo(db DBTX) *GradeSyncRepo {
	return &GradeSyncRepo{db: db}
}

// Create inserts a new grade sync entry
func (r *GradeSyncRepo) Create(ctx context.Context, entry *models.GradeSyncEntry) error {
	query := `
		INSERT INTO grade_sync_queue (
			id, session_id, status, earned_points, max_points, percentage,
			canvas_course_id, canvas_assignment_id, canvas_user_id, attempts
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		RETURNING created_at`

	err := r.db.QueryRowContext(ctx, query,
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
	).Scan(&entry.CreatedAt)

	if err != nil {
		return fmt.Errorf("inserting grade sync entry: %w", err)
	}

	return nil
}

// GetPending retrieves pending grade sync entries
func (r *GradeSyncRepo) GetPending(ctx context.Context, limit int) ([]*models.GradeSyncEntry, error) {
	query := `
		SELECT id, session_id, status, earned_points, max_points, percentage,
		       canvas_course_id, canvas_assignment_id, canvas_user_id,
		       attempts, last_attempt_at, completed_at, error_message, created_at
		FROM grade_sync_queue
		WHERE status = 'pending'
		ORDER BY created_at ASC
		LIMIT $1`

	rows, err := r.db.QueryContext(ctx, query, limit)
	if err != nil {
		return nil, fmt.Errorf("querying pending entries: %w", err)
	}
	defer rows.Close()

	return r.scanEntries(rows)
}

// MarkProcessing marks an entry as processing
func (r *GradeSyncRepo) MarkProcessing(ctx context.Context, id string) error {
	query := `
		UPDATE grade_sync_queue
		SET status = 'processing', last_attempt_at = $2, attempts = attempts + 1
		WHERE id = $1`

	res, err := r.db.ExecContext(ctx, query, id, time.Now())
	if err != nil {
		return fmt.Errorf("marking entry as processing: %w", err)
	}

	rows, _ := res.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("grade sync entry not found: %s", id)
	}

	return nil
}

// MarkCompleted marks an entry as completed
func (r *GradeSyncRepo) MarkCompleted(ctx context.Context, id string) error {
	query := `
		UPDATE grade_sync_queue
		SET status = 'completed', completed_at = $2, error_message = NULL
		WHERE id = $1`

	res, err := r.db.ExecContext(ctx, query, id, time.Now())
	if err != nil {
		return fmt.Errorf("marking entry as completed: %w", err)
	}

	rows, _ := res.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("grade sync entry not found: %s", id)
	}

	return nil
}

// MarkFailed marks an entry as failed with an error message
func (r *GradeSyncRepo) MarkFailed(ctx context.Context, id string, errorMsg string) error {
	query := `
		UPDATE grade_sync_queue
		SET status = 'failed', error_message = $2
		WHERE id = $1`

	res, err := r.db.ExecContext(ctx, query, id, errorMsg)
	if err != nil {
		return fmt.Errorf("marking entry as failed: %w", err)
	}

	rows, _ := res.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("grade sync entry not found: %s", id)
	}

	return nil
}

// scanEntries scans multiple grade sync entries from rows
func (r *GradeSyncRepo) scanEntries(rows *sql.Rows) ([]*models.GradeSyncEntry, error) {
	var entries []*models.GradeSyncEntry

	for rows.Next() {
		var entry models.GradeSyncEntry
		var lastAttemptAt, completedAt sql.NullTime
		var errorMessage sql.NullString

		err := rows.Scan(
			&entry.ID,
			&entry.SessionID,
			&entry.Status,
			&entry.EarnedPoints,
			&entry.MaxPoints,
			&entry.Percentage,
			&entry.CanvasCourseID,
			&entry.CanvasAssignmentID,
			&entry.CanvasUserID,
			&entry.Attempts,
			&lastAttemptAt,
			&completedAt,
			&errorMessage,
			&entry.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scanning entry: %w", err)
		}

		if lastAttemptAt.Valid {
			entry.LastAttemptAt = &lastAttemptAt.Time
		}
		if completedAt.Valid {
			entry.CompletedAt = &completedAt.Time
		}
		if errorMessage.Valid {
			entry.ErrorMessage = errorMessage.String
		}

		entries = append(entries, &entry)
	}

	return entries, rows.Err()
}
