package repositories

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/toddbartholow/kootenai/api/internal/models"
)

// AssessmentResultRepo implements AssessmentResultRepository
type AssessmentResultRepo struct {
	db DBTX
}

// NewAssessmentResultRepo creates a new assessment result repository
func NewAssessmentResultRepo(db DBTX) *AssessmentResultRepo {
	return &AssessmentResultRepo{db: db}
}

// Create inserts a new assessment result
func (r *AssessmentResultRepo) Create(ctx context.Context, result *models.AssessmentResult) error {
	componentsJSON, err := json.Marshal(result.Components)
	if err != nil {
		return fmt.Errorf("marshaling components: %w", err)
	}

	devicesJSON, err := json.Marshal(result.Devices)
	if err != nil {
		return fmt.Errorf("marshaling devices: %w", err)
	}

	query := `
		INSERT INTO assessment_results (
			session_id, score, max_score, percentage,
			item_count, passed_count, status,
			started_at, last_checked, components, devices
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		ON CONFLICT (session_id) DO UPDATE SET
			score = EXCLUDED.score,
			max_score = EXCLUDED.max_score,
			percentage = EXCLUDED.percentage,
			item_count = EXCLUDED.item_count,
			passed_count = EXCLUDED.passed_count,
			status = EXCLUDED.status,
			last_checked = EXCLUDED.last_checked,
			components = EXCLUDED.components,
			devices = EXCLUDED.devices,
			updated_at = NOW()`

	_, err = r.db.ExecContext(ctx, query,
		result.SessionID,
		result.Score,
		result.MaxScore,
		result.Percentage,
		result.ItemCount,
		result.PassedCount,
		result.Status,
		result.StartedAt,
		result.LastChecked,
		componentsJSON,
		devicesJSON,
	)

	if err != nil {
		return fmt.Errorf("inserting assessment result: %w", err)
	}

	return nil
}

// GetBySessionID retrieves an assessment result by session ID
func (r *AssessmentResultRepo) GetBySessionID(ctx context.Context, sessionID string) (*models.AssessmentResult, error) {
	query := `
		SELECT session_id, score, max_score, percentage,
		       item_count, passed_count, status,
		       started_at, last_checked, completed_at,
		       components, devices
		FROM assessment_results
		WHERE session_id = $1`

	var result models.AssessmentResult
	var completedAt sql.NullTime
	var componentsJSON, devicesJSON []byte

	err := r.db.QueryRowContext(ctx, query, sessionID).Scan(
		&result.SessionID,
		&result.Score,
		&result.MaxScore,
		&result.Percentage,
		&result.ItemCount,
		&result.PassedCount,
		&result.Status,
		&result.StartedAt,
		&result.LastChecked,
		&completedAt,
		&componentsJSON,
		&devicesJSON,
	)

	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("querying assessment result: %w", err)
	}

	if len(componentsJSON) > 0 {
		if err := json.Unmarshal(componentsJSON, &result.Components); err != nil {
			return nil, fmt.Errorf("unmarshaling components: %w", err)
		}
	}

	if len(devicesJSON) > 0 {
		if err := json.Unmarshal(devicesJSON, &result.Devices); err != nil {
			return nil, fmt.Errorf("unmarshaling devices: %w", err)
		}
	}

	// Calculate time elapsed
	if !result.StartedAt.IsZero() {
		elapsed := result.LastChecked.Sub(result.StartedAt)
		result.TimeElapsed = formatDuration(elapsed)
	}

	return &result, nil
}

// Update updates an existing assessment result
func (r *AssessmentResultRepo) Update(ctx context.Context, result *models.AssessmentResult) error {
	componentsJSON, err := json.Marshal(result.Components)
	if err != nil {
		return fmt.Errorf("marshaling components: %w", err)
	}

	devicesJSON, err := json.Marshal(result.Devices)
	if err != nil {
		return fmt.Errorf("marshaling devices: %w", err)
	}

	query := `
		UPDATE assessment_results
		SET score = $2, max_score = $3, percentage = $4,
		    item_count = $5, passed_count = $6, status = $7,
		    last_checked = $8, components = $9, devices = $10
		WHERE session_id = $1`

	res, err := r.db.ExecContext(ctx, query,
		result.SessionID,
		result.Score,
		result.MaxScore,
		result.Percentage,
		result.ItemCount,
		result.PassedCount,
		result.Status,
		result.LastChecked,
		componentsJSON,
		devicesJSON,
	)

	if err != nil {
		return fmt.Errorf("updating assessment result: %w", err)
	}

	rows, _ := res.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("assessment result not found for session: %s", result.SessionID)
	}

	return nil
}

// UpdateStatus updates just the status of an assessment result
func (r *AssessmentResultRepo) UpdateStatus(ctx context.Context, sessionID string, status string) error {
	query := `UPDATE assessment_results SET status = $2, last_checked = NOW() WHERE session_id = $1`

	if status == "completed" || status == "graded" {
		query = `UPDATE assessment_results SET status = $2, last_checked = NOW(), completed_at = NOW() WHERE session_id = $1`
	}

	res, err := r.db.ExecContext(ctx, query, sessionID, status)
	if err != nil {
		return fmt.Errorf("updating status: %w", err)
	}

	rows, _ := res.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("assessment result not found for session: %s", sessionID)
	}

	return nil
}

// Delete removes an assessment result by session ID
func (r *AssessmentResultRepo) Delete(ctx context.Context, sessionID string) error {
	query := `DELETE FROM assessment_results WHERE session_id = $1`

	_, err := r.db.ExecContext(ctx, query, sessionID)
	if err != nil {
		return fmt.Errorf("deleting assessment result: %w", err)
	}

	return nil
}

// DeleteExpired removes assessment results older than the specified time
func (r *AssessmentResultRepo) DeleteExpired(ctx context.Context, olderThan time.Time) (int64, error) {
	query := `DELETE FROM assessment_results WHERE updated_at < $1`

	res, err := r.db.ExecContext(ctx, query, olderThan)
	if err != nil {
		return 0, fmt.Errorf("deleting expired assessment results: %w", err)
	}

	count, _ := res.RowsAffected()
	return count, nil
}

// formatDuration formats a duration as HH:MM:SS
func formatDuration(d time.Duration) string {
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	s := int(d.Seconds()) % 60
	return fmt.Sprintf("%02d:%02d:%02d", h, m, s)
}
