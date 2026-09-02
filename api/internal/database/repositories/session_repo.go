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

// maxListAllResults is the safety cap on ListAll to prevent unbounded queries.
const maxListAllResults = 10000

// SessionRepo implements SessionRepository
type SessionRepo struct {
	db DBTX
}

// NewSessionRepo creates a new session repository
func NewSessionRepo(db DBTX) *SessionRepo {
	return &SessionRepo{db: db}
}

// Create inserts a new session
func (r *SessionRepo) Create(ctx context.Context, session *models.Session) error {
	metadataJSON, err := json.Marshal(session.Metadata)
	if err != nil {
		return fmt.Errorf("marshaling metadata: %w", err)
	}

	query := `
		INSERT INTO lab_sessions (
			id, pod_id, user_id, lab_template_id, max_points,
			canvas_course_id, canvas_assignment_id, canvas_user_id,
			due_at, metadata, organization_id, team_id,
			enrollment_id, module_id, passing_threshold
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)
		RETURNING started_at`

	passingThreshold := session.PassingThreshold
	if passingThreshold <= 0 {
		passingThreshold = 70
	}

	err = r.db.QueryRowContext(ctx, query,
		session.ID,
		session.PodID,
		session.UserID,
		session.LabTemplateID,
		session.MaxPoints,
		nullString(session.CanvasCourseID),
		nullString(session.CanvasAssignmentID),
		nullString(session.CanvasUserID),
		session.DueAt,
		metadataJSON,
		nullStringPtr(session.OrganizationID),
		nullStringPtr(session.TeamID),
		nullStringPtr(session.EnrollmentID),
		nullStringPtr(session.ModuleID),
		passingThreshold,
	).Scan(&session.StartedAt)

	if err != nil {
		return fmt.Errorf("inserting session: %w", err)
	}

	return nil
}

// GetByID retrieves a session by ID
func (r *SessionRepo) GetByID(ctx context.Context, id string) (*models.Session, error) {
	query := `
		SELECT id, pod_id, user_id, lab_template_id,
		       max_points, earned_points, percentage, passed,
		       canvas_course_id, canvas_assignment_id, canvas_user_id,
		       started_at, ended_at, due_at, grade_synced_at, grade_sync_error,
		       metadata, organization_id, team_id, enrollment_id, module_id, passing_threshold
		FROM lab_sessions
		WHERE id = $1`

	session, err := r.scanSession(r.db.QueryRowContext(ctx, query, id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return session, err
}

// GetByPodID retrieves sessions for a pod
func (r *SessionRepo) GetByPodID(ctx context.Context, podID string) ([]*models.Session, error) {
	query := `
		SELECT id, pod_id, user_id, lab_template_id,
		       max_points, earned_points, percentage, passed,
		       canvas_course_id, canvas_assignment_id, canvas_user_id,
		       started_at, ended_at, due_at, grade_synced_at, grade_sync_error,
		       metadata, organization_id, team_id, enrollment_id, module_id, passing_threshold
		FROM lab_sessions
		WHERE pod_id = $1
		ORDER BY started_at DESC`

	return r.queryMultiple(ctx, query, podID)
}

// GetActiveByUserID retrieves active sessions for a user
func (r *SessionRepo) GetActiveByUserID(ctx context.Context, userID string) ([]*models.Session, error) {
	query := `
		SELECT id, pod_id, user_id, lab_template_id,
		       max_points, earned_points, percentage, passed,
		       canvas_course_id, canvas_assignment_id, canvas_user_id,
		       started_at, ended_at, due_at, grade_synced_at, grade_sync_error,
		       metadata, organization_id, team_id, enrollment_id, module_id, passing_threshold
		FROM lab_sessions
		WHERE user_id = $1 AND ended_at IS NULL
		ORDER BY started_at DESC`

	return r.queryMultiple(ctx, query, userID)
}

// List retrieves sessions matching the filter
func (r *SessionRepo) List(ctx context.Context, filter SessionFilter) ([]*models.Session, error) {
	qb := NewQueryBuilder(`
		SELECT id, pod_id, user_id, lab_template_id,
		       max_points, earned_points, percentage, passed,
		       canvas_course_id, canvas_assignment_id, canvas_user_id,
		       started_at, ended_at, due_at, grade_synced_at, grade_sync_error,
		       metadata, organization_id, team_id, enrollment_id, module_id, passing_threshold
		FROM lab_sessions
		WHERE 1=1`)

	if filter.UserID != "" {
		qb.AddCondition("user_id = $%d", filter.UserID)
	}

	if filter.PodID != "" {
		qb.AddCondition("pod_id = $%d", filter.PodID)
	}

	if filter.TemplateID != "" {
		qb.AddCondition("lab_template_id = $%d", filter.TemplateID)
	}

	if filter.CanvasAssignmentID != "" {
		qb.AddCondition("canvas_assignment_id = $%d", filter.CanvasAssignmentID)
	}

	if filter.Active != nil {
		if *filter.Active {
			qb.AddRawCondition("ended_at IS NULL")
		} else {
			qb.AddRawCondition("ended_at IS NOT NULL")
		}
	}

	if filter.OrganizationID != "" {
		qb.AddCondition("organization_id = $%d", filter.OrganizationID)
	}

	if filter.TeamID != "" {
		qb.AddCondition("team_id = $%d", filter.TeamID)
	}

	qb.OrderByRaw("started_at DESC")
	qb.DefaultLimit(filter.Limit, 100)
	qb.Offset(filter.Offset)

	query, args := qb.Build()

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("querying sessions: %w", err)
	}
	defer rows.Close()

	return r.scanMultiple(rows)
}

// Update updates a session
func (r *SessionRepo) Update(ctx context.Context, session *models.Session) error {
	metadataJSON, err := json.Marshal(session.Metadata)
	if err != nil {
		return fmt.Errorf("marshaling metadata: %w", err)
	}

	query := `
		UPDATE lab_sessions
		SET earned_points = $2, percentage = $3, passed = $4,
		    ended_at = $5, grade_synced_at = $6, grade_sync_error = $7,
		    metadata = $8, organization_id = $9, team_id = $10,
		    enrollment_id = $11, module_id = $12
		WHERE id = $1`

	res, err := r.db.ExecContext(ctx, query,
		session.ID,
		session.EarnedPoints,
		session.Percentage,
		session.Passed,
		session.EndedAt,
		session.GradeSyncedAt,
		nullString(session.GradeSyncError),
		metadataJSON,
		nullStringPtr(session.OrganizationID),
		nullStringPtr(session.TeamID),
		nullStringPtr(session.EnrollmentID),
		nullStringPtr(session.ModuleID),
	)

	if err != nil {
		return fmt.Errorf("updating session: %w", err)
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("session not found: %s", session.ID)
	}

	return nil
}

// End marks a session as ended
func (r *SessionRepo) End(ctx context.Context, id string) error {
	query := `UPDATE lab_sessions SET ended_at = $2 WHERE id = $1`
	res, err := r.db.ExecContext(ctx, query, id, time.Now())
	if err != nil {
		return fmt.Errorf("ending session: %w", err)
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("session not found: %s", id)
	}
	return nil
}

// UpdateGrade updates the session grade
func (r *SessionRepo) UpdateGrade(ctx context.Context, id string, earnedPoints int, passed bool) error {
	query := `
		UPDATE lab_sessions
		SET earned_points = $2,
		    percentage = CASE WHEN max_points > 0
		                      THEN ROUND(($2::DECIMAL / max_points) * 100, 2)
		                      ELSE 0 END,
		    passed = $3
		WHERE id = $1`

	res, err := r.db.ExecContext(ctx, query, id, earnedPoints, passed)
	if err != nil {
		return fmt.Errorf("updating grade: %w", err)
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("session not found: %s", id)
	}
	return nil
}

// MarkGradeSynced marks the session grade as synced
func (r *SessionRepo) MarkGradeSynced(ctx context.Context, id string, syncedAt time.Time) error {
	query := `UPDATE lab_sessions SET grade_synced_at = $2, grade_sync_error = NULL WHERE id = $1`
	_, err := r.db.ExecContext(ctx, query, id, syncedAt)
	if err != nil {
		return fmt.Errorf("marking grade synced: %w", err)
	}
	return nil
}

// MarkGradeSyncFailed marks the grade sync as failed
func (r *SessionRepo) MarkGradeSyncFailed(ctx context.Context, id string, errorMsg string) error {
	query := `UPDATE lab_sessions SET grade_sync_error = $2 WHERE id = $1`
	_, err := r.db.ExecContext(ctx, query, id, errorMsg)
	if err != nil {
		return fmt.Errorf("marking grade sync failed: %w", err)
	}
	return nil
}

// Helper methods

func (r *SessionRepo) queryMultiple(ctx context.Context, query string, args ...any) ([]*models.Session, error) {
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("querying sessions: %w", err)
	}
	defer rows.Close()

	return r.scanMultiple(rows)
}

func (r *SessionRepo) scanMultiple(rows *sql.Rows) ([]*models.Session, error) {
	var sessions []*models.Session
	for rows.Next() {
		session, err := r.scanSessionRow(rows)
		if err != nil {
			return nil, err
		}
		sessions = append(sessions, session)
	}
	return sessions, rows.Err()
}

type scannable interface {
	Scan(dest ...any) error
}

func (r *SessionRepo) scanSession(row scannable) (*models.Session, error) {
	var session models.Session
	var canvasCourseID, canvasAssignmentID, canvasUserID, gradeSyncError sql.NullString
	var organizationID, teamID, enrollmentID, moduleID sql.NullString
	var metadataJSON []byte

	err := row.Scan(
		&session.ID,
		&session.PodID,
		&session.UserID,
		&session.LabTemplateID,
		&session.MaxPoints,
		&session.EarnedPoints,
		&session.Percentage,
		&session.Passed,
		&canvasCourseID,
		&canvasAssignmentID,
		&canvasUserID,
		&session.StartedAt,
		&session.EndedAt,
		&session.DueAt,
		&session.GradeSyncedAt,
		&gradeSyncError,
		&metadataJSON,
		&organizationID,
		&teamID,
		&enrollmentID,
		&moduleID,
		&session.PassingThreshold,
	)

	if err != nil {
		return nil, fmt.Errorf("scanning session: %w", err)
	}

	session.CanvasCourseID = canvasCourseID.String
	session.CanvasAssignmentID = canvasAssignmentID.String
	session.CanvasUserID = canvasUserID.String
	session.GradeSyncError = gradeSyncError.String
	if organizationID.Valid {
		session.OrganizationID = &organizationID.String
	}
	if teamID.Valid {
		session.TeamID = &teamID.String
	}
	if enrollmentID.Valid {
		session.EnrollmentID = &enrollmentID.String
	}
	if moduleID.Valid {
		session.ModuleID = &moduleID.String
	}

	if len(metadataJSON) > 0 {
		if err := json.Unmarshal(metadataJSON, &session.Metadata); err != nil {
			return nil, fmt.Errorf("unmarshaling metadata: %w", err)
		}
	}

	return &session, nil
}

func (r *SessionRepo) scanSessionRow(rows *sql.Rows) (*models.Session, error) {
	return r.scanSession(rows)
}

func nullString(s string) sql.NullString {
	if s == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: s, Valid: true}
}

// GetUserID returns the user ID for a session
func (r *SessionRepo) GetUserID(ctx context.Context, id string) (string, error) {
	query := `SELECT user_id FROM lab_sessions WHERE id = $1`
	var userID string
	err := r.db.QueryRowContext(ctx, query, id).Scan(&userID)
	if err != nil {
		return "", fmt.Errorf("getting session user: %w", err)
	}
	return userID, nil
}

// IsOwner checks if a user owns a session
func (r *SessionRepo) IsOwner(ctx context.Context, id, userID string) (bool, error) {
	query := `SELECT EXISTS(SELECT 1 FROM lab_sessions WHERE id = $1 AND user_id = $2)`
	var exists bool
	err := r.db.QueryRowContext(ctx, query, id, userID).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("checking session ownership: %w", err)
	}
	return exists, nil
}

// GetOrganizationID returns the organization ID for a session (if any)
func (r *SessionRepo) GetOrganizationID(ctx context.Context, id string) (*string, error) {
	query := `SELECT organization_id FROM lab_sessions WHERE id = $1`
	var orgID sql.NullString
	err := r.db.QueryRowContext(ctx, query, id).Scan(&orgID)
	if err != nil {
		return nil, fmt.Errorf("getting session organization: %w", err)
	}
	if orgID.Valid {
		return &orgID.String, nil
	}
	return nil, nil
}

// ListAll retrieves all sessions (for leaderboard/analytics)
func (r *SessionRepo) ListAll(ctx context.Context) ([]*models.Session, error) {
	query := `
		SELECT id, pod_id, user_id, lab_template_id,
		       max_points, earned_points, percentage, passed,
		       canvas_course_id, canvas_assignment_id, canvas_user_id,
		       started_at, ended_at, due_at, grade_synced_at, grade_sync_error,
		       metadata, organization_id, team_id, enrollment_id, module_id, passing_threshold
		FROM lab_sessions
		ORDER BY started_at DESC
		LIMIT $1`

	rows, err := r.db.QueryContext(ctx, query, maxListAllResults)
	if err != nil {
		return nil, fmt.Errorf("listing all sessions: %w", err)
	}
	defer rows.Close()

	return r.scanMultiple(rows)
}

// CountCompletedLabsByUser returns a map of userID → completed lab count using a SQL aggregate
// instead of loading all sessions into memory.
func (r *SessionRepo) CountCompletedLabsByUser(ctx context.Context) (map[string]int, error) {
	query := `
		SELECT user_id, COUNT(*)
		FROM lab_sessions
		WHERE ended_at IS NOT NULL AND passed = true
		GROUP BY user_id`

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("counting completed labs by user: %w", err)
	}
	defer rows.Close()

	result := make(map[string]int)
	for rows.Next() {
		var userID string
		var count int
		if err := rows.Scan(&userID, &count); err != nil {
			return nil, fmt.Errorf("scanning completed lab count: %w", err)
		}
		result[userID] = count
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating completed lab counts: %w", err)
	}
	return result, nil
}

// UserSessionStats contains aggregated session statistics for a user
type UserSessionStats struct {
	TotalLabsCompleted int     `json:"totalLabsCompleted"`
	TotalTimeSpentMins int     `json:"totalTimeSpentMins"`
	AverageScore       float64 `json:"averageScore"`
	CurrentStreak      int     `json:"currentStreak"`
	BestStreak         int     `json:"bestStreak"`
}

// GetUserStats retrieves aggregated session statistics for a user in a single query
// This is much more efficient than loading all sessions and calculating in Go
func (r *SessionRepo) GetUserStats(ctx context.Context, userID string) (*UserSessionStats, error) {
	query := `
		WITH completed_sessions AS (
			SELECT
				id,
				started_at,
				ended_at,
				percentage,
				passed,
				EXTRACT(EPOCH FROM (ended_at - started_at))/60 as duration_mins,
				DATE(ended_at) as completion_date
			FROM lab_sessions
			WHERE user_id = $1 AND ended_at IS NOT NULL
		),
		streak_data AS (
			SELECT
				completion_date,
				ROW_NUMBER() OVER (ORDER BY completion_date) as rn,
				completion_date - (ROW_NUMBER() OVER (ORDER BY completion_date) * INTERVAL '1 day') as grp
			FROM (
				SELECT DISTINCT completion_date FROM completed_sessions WHERE passed = true
			) distinct_dates
		),
		streaks AS (
			SELECT grp, COUNT(*) as streak_length
			FROM streak_data
			GROUP BY grp
		)
		SELECT
			COALESCE(COUNT(*) FILTER (WHERE passed = true), 0) as total_completed,
			COALESCE(SUM(duration_mins)::INT, 0) as total_time_mins,
			COALESCE(AVG(percentage) FILTER (WHERE ended_at IS NOT NULL), 0) as avg_score,
			COALESCE((SELECT MAX(streak_length) FROM streaks), 0) as best_streak,
			COALESCE((
				SELECT streak_length FROM streaks
				WHERE grp = (SELECT MAX(grp) FROM streaks)
			), 0) as current_streak
		FROM completed_sessions`

	stats := &UserSessionStats{}
	err := r.db.QueryRowContext(ctx, query, userID).Scan(
		&stats.TotalLabsCompleted,
		&stats.TotalTimeSpentMins,
		&stats.AverageScore,
		&stats.BestStreak,
		&stats.CurrentStreak,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return &UserSessionStats{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("getting user stats: %w", err)
	}
	return stats, nil
}

// ListWithLabNames retrieves sessions with lab template names in a single query
// Avoids N+1 query pattern when fetching sessions for dashboard
func (r *SessionRepo) ListWithLabNames(ctx context.Context, filter SessionFilter) ([]*SessionWithLabName, error) {
	qb := NewQueryBuilder(`
		SELECT
			ls.id, ls.pod_id, ls.user_id, ls.lab_template_id,
			ls.max_points, ls.earned_points, ls.percentage, ls.passed,
			ls.started_at, ls.ended_at, ls.due_at,
			COALESCE(lt.name, 'Unknown Lab') as lab_name
		FROM lab_sessions ls
		LEFT JOIN lab_templates lt ON lt.id = ls.lab_template_id
		WHERE 1=1`)

	if filter.UserID != "" {
		qb.AddCondition("ls.user_id = $%d", filter.UserID)
	}

	if filter.Active != nil && *filter.Active {
		qb.AddRawCondition("ls.ended_at IS NULL")
	}

	qb.OrderByRaw("ls.started_at DESC")
	qb.DefaultLimit(filter.Limit, 100)
	qb.Offset(filter.Offset)

	query, args := qb.Build()

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("querying sessions with lab names: %w", err)
	}
	defer rows.Close()

	var sessions []*SessionWithLabName
	for rows.Next() {
		s := &SessionWithLabName{}
		err := rows.Scan(
			&s.ID, &s.PodID, &s.UserID, &s.LabTemplateID,
			&s.MaxPoints, &s.EarnedPoints, &s.Percentage, &s.Passed,
			&s.StartedAt, &s.EndedAt, &s.DueAt, &s.LabName,
		)
		if err != nil {
			return nil, fmt.Errorf("scanning session with lab name: %w", err)
		}
		sessions = append(sessions, s)
	}
	return sessions, rows.Err()
}

// SessionWithLabName includes the lab template name for display
type SessionWithLabName struct {
	ID            string     `json:"id"`
	PodID         string     `json:"podId"`
	UserID        string     `json:"userId"`
	LabTemplateID string     `json:"labTemplateId"`
	LabName       string     `json:"labName"`
	MaxPoints     int        `json:"maxPoints"`
	EarnedPoints  int        `json:"earnedPoints"`
	Percentage    float64    `json:"percentage"`
	Passed        bool       `json:"passed"`
	StartedAt     time.Time  `json:"startedAt"`
	EndedAt       *time.Time `json:"endedAt,omitempty"`
	DueAt         *time.Time `json:"dueAt,omitempty"`
}

// CountActive returns the number of sessions that have not ended. This is the
// same predicate SessionFilter{Active: true} uses.
func (r *SessionRepo) CountActive(ctx context.Context) (int64, error) {
	var count int64
	err := r.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM lab_sessions WHERE ended_at IS NULL`).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("counting active sessions: %w", err)
	}
	return count, nil
}

// EndStaleSessions marks active sessions older than maxAge as ended.
// Returns the number of sessions affected.
func (r *SessionRepo) EndStaleSessions(ctx context.Context, maxAge time.Duration) (int64, error) {
	cutoff := time.Now().Add(-maxAge)
	query := `
		UPDATE lab_sessions
		SET ended_at = NOW()
		WHERE ended_at IS NULL AND started_at < $1`

	result, err := r.db.ExecContext(ctx, query, cutoff)
	if err != nil {
		return 0, fmt.Errorf("ending stale sessions: %w", err)
	}
	return result.RowsAffected()
}

// DeleteEndedBefore deletes ended sessions whose ended_at is older than the cutoff.
// Returns the number of sessions deleted.
func (r *SessionRepo) DeleteEndedBefore(ctx context.Context, cutoff time.Time) (int64, error) {
	tx, err := beginTx(ctx, r.db, nil)
	if err != nil {
		return 0, fmt.Errorf("starting transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	// Delete checkpoint progress for sessions that will be removed
	_, err = tx.ExecContext(ctx, `
		DELETE FROM checkpoint_progress
		WHERE session_id IN (
			SELECT id FROM lab_sessions
			WHERE ended_at IS NOT NULL AND ended_at < $1
		)`, cutoff)
	if err != nil {
		return 0, fmt.Errorf("deleting checkpoint progress: %w", err)
	}

	result, err := tx.ExecContext(ctx, `
		DELETE FROM lab_sessions
		WHERE ended_at IS NOT NULL AND ended_at < $1`, cutoff)
	if err != nil {
		return 0, fmt.Errorf("deleting ended sessions: %w", err)
	}

	count, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("getting rows affected: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("committing transaction: %w", err)
	}
	return count, nil
}

// Delete removes a session and its associated checkpoint progress
func (r *SessionRepo) Delete(ctx context.Context, id string) error {
	// Use a transaction to ensure both deletes succeed or fail together
	tx, err := beginTx(ctx, r.db, nil)
	if err != nil {
		return fmt.Errorf("starting transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	// First delete checkpoint progress records
	_, err = tx.ExecContext(ctx, `DELETE FROM checkpoint_progress WHERE session_id = $1`, id)
	if err != nil {
		return fmt.Errorf("deleting checkpoint progress: %w", err)
	}

	// Then delete the session
	result, err := tx.ExecContext(ctx, `DELETE FROM lab_sessions WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("deleting session: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("getting rows affected: %w", err)
	}
	if rowsAffected == 0 {
		return sql.ErrNoRows
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("committing transaction: %w", err)
	}

	return nil
}
