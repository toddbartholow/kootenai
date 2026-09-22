package classroom

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// Simulation represents a classroom simulation run.
type Simulation struct {
	ID             string          `json:"id"`
	Name           string          `json:"name"`
	Status         string          `json:"status"`
	CanvasCourseID *string         `json:"canvasCourseId,omitempty"`
	PathwayID      *string         `json:"pathwayId,omitempty"`
	Config         json.RawMessage `json:"config"`
	StartedAt      *time.Time      `json:"startedAt,omitempty"`
	CompletedAt    *time.Time      `json:"completedAt,omitempty"`
	CreatedAt      time.Time       `json:"createdAt"`
	UpdatedAt      time.Time       `json:"updatedAt"`
	Students       []AIStudent     `json:"students,omitempty"`
}

// AIStudent represents a simulated AI student.
type AIStudent struct {
	ID               string          `json:"id"`
	SimulationID     string          `json:"simulationId"`
	UserID           *string         `json:"userId,omitempty"`
	CanvasUserID     *string         `json:"canvasUserId,omitempty"`
	Name             string          `json:"name"`
	Personality      PersonalityType `json:"personality"`
	Traits           json.RawMessage `json:"traits"`
	TechSkills       json.RawMessage `json:"techSkills"`
	BehavioralConfig json.RawMessage `json:"behavioralConfig"`
	State            json.RawMessage `json:"state"`
	CreatedAt        time.Time       `json:"createdAt"`
}

// Activity represents an AI student activity log entry.
type Activity struct {
	ID           int64           `json:"id"`
	SimulationID string          `json:"simulationId"`
	StudentID    string          `json:"studentId"`
	ActivityType string          `json:"activityType"`
	TargetID     *string         `json:"targetId,omitempty"`
	TargetName   *string         `json:"targetName,omitempty"`
	Status       string          `json:"status"`
	Content      *string         `json:"content,omitempty"`
	Score        *int            `json:"score,omitempty"`
	MaxScore     *int            `json:"maxScore,omitempty"`
	Metadata     json.RawMessage `json:"metadata,omitempty"`
	CreatedAt    time.Time       `json:"createdAt"`
}

// Feedback represents structured feedback from an AI student about a lab or pathway.
type Feedback struct {
	ID                  string          `json:"id"`
	SimulationID        string          `json:"simulationId"`
	StudentID           string          `json:"studentId"`
	LabTemplateID       *string         `json:"labTemplateId,omitempty"`
	PathwayID           *string         `json:"pathwayId,omitempty"`
	SessionID           *string         `json:"sessionId,omitempty"`
	FeedbackType        string          `json:"feedbackType"`
	Summary             string          `json:"summary"`
	Rating              *int            `json:"rating,omitempty"`
	Details             json.RawMessage `json:"details"`
	ExecutionDurationMs *int            `json:"executionDurationMs,omitempty"`
	CheckpointsPassed   *int            `json:"checkpointsPassed,omitempty"`
	CheckpointsTotal    *int            `json:"checkpointsTotal,omitempty"`
	Score               *int            `json:"score,omitempty"`
	MaxScore            *int            `json:"maxScore,omitempty"`
	ExecutionLog        *string         `json:"executionLog,omitempty"`
	CreatedAt           time.Time       `json:"createdAt"`
}

// FeedbackFilter holds filter options for listing feedback.
type FeedbackFilter struct {
	FeedbackType  string
	LabTemplateID string
	Limit         int
	Offset        int
}

// FeedbackSummary holds aggregated feedback statistics for a simulation.
type FeedbackSummary struct {
	TotalFeedback        int                    `json:"totalFeedback"`
	AvgRating            float64                `json:"avgRating"`
	ByType               map[string]TypeSummary `json:"byType"`
	ByLab                []LabFeedbackSummary   `json:"byLab"`
	InfrastructureIssues int                    `json:"infrastructureIssues"`
}

// TypeSummary holds per-type aggregated stats.
type TypeSummary struct {
	Count     int     `json:"count"`
	AvgRating float64 `json:"avgRating"`
}

// LabFeedbackSummary holds per-lab aggregated stats.
type LabFeedbackSummary struct {
	LabTemplateID string  `json:"labTemplateId"`
	Count         int     `json:"count"`
	AvgRating     float64 `json:"avgRating"`
}

// SimulationConfig is the user-provided configuration for a simulation.
type SimulationConfig struct {
	StudentCount      int                     `json:"studentCount"`
	PersonalityMix    map[PersonalityType]int `json:"personalityMix"`
	EnableCanvas      bool                    `json:"enableCanvas"`
	EnableVMLabs      bool                    `json:"enableVmLabs"`
	SpeedMultiplier   float64                 `json:"speedMultiplier"`
	MaxConcurrentPods int                     `json:"maxConcurrentPods,omitempty"`
}

// Repository defines the data access interface for classroom simulation.
type Repository interface {
	// Simulations
	CreateSimulation(ctx context.Context, sim *Simulation) error
	GetSimulation(ctx context.Context, id string) (*Simulation, error)
	ListSimulations(ctx context.Context) ([]Simulation, error)
	UpdateSimulationStatus(ctx context.Context, id, status string) error
	DeleteSimulation(ctx context.Context, id string) error

	// Students
	CreateStudent(ctx context.Context, student *AIStudent) error
	ListStudents(ctx context.Context, simulationID string) ([]AIStudent, error)
	GetStudent(ctx context.Context, id string) (*AIStudent, error)
	UpdateStudentState(ctx context.Context, id string, state json.RawMessage) error

	// Activities
	CreateActivity(ctx context.Context, activity *Activity) error
	ListActivities(ctx context.Context, simulationID string, limit, offset int) ([]Activity, int, error)
	ListStudentActivities(ctx context.Context, studentID string, limit, offset int) ([]Activity, int, error)

	// Feedback
	CreateFeedback(ctx context.Context, feedback *Feedback) error
	ListFeedback(ctx context.Context, simulationID string, opts FeedbackFilter) ([]Feedback, int, error)
	GetFeedbackSummary(ctx context.Context, simulationID string) (*FeedbackSummary, error)
}

// PostgresRepository implements Repository using PostgreSQL.
type PostgresRepository struct {
	db *sql.DB
}

// NewPostgresRepository creates a new PostgreSQL repository.
func NewPostgresRepository(db *sql.DB) *PostgresRepository {
	return &PostgresRepository{db: db}
}

func (r *PostgresRepository) CreateSimulation(ctx context.Context, sim *Simulation) error {
	if sim.ID == "" {
		sim.ID = uuid.New().String()
	}
	if sim.Config == nil {
		sim.Config = json.RawMessage(`{}`)
	}

	query := `
		INSERT INTO classroom_simulations (id, name, status, canvas_course_id, pathway_id, config, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, NOW(), NOW())
		RETURNING created_at, updated_at`

	return r.db.QueryRowContext(ctx, query,
		sim.ID, sim.Name, sim.Status, sim.CanvasCourseID, sim.PathwayID, sim.Config,
	).Scan(&sim.CreatedAt, &sim.UpdatedAt)
}

func (r *PostgresRepository) GetSimulation(ctx context.Context, id string) (*Simulation, error) {
	query := `
		SELECT id, name, status, canvas_course_id, pathway_id, config,
		       started_at, completed_at, created_at, updated_at
		FROM classroom_simulations WHERE id = $1`

	var sim Simulation
	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&sim.ID, &sim.Name, &sim.Status, &sim.CanvasCourseID, &sim.PathwayID,
		&sim.Config, &sim.StartedAt, &sim.CompletedAt, &sim.CreatedAt, &sim.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("getting simulation: %w", err)
	}

	// Load students
	students, err := r.ListStudents(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("loading students: %w", err)
	}
	sim.Students = students

	return &sim, nil
}

func (r *PostgresRepository) ListSimulations(ctx context.Context) ([]Simulation, error) {
	query := `
		SELECT id, name, status, canvas_course_id, pathway_id, config,
		       started_at, completed_at, created_at, updated_at
		FROM classroom_simulations
		ORDER BY created_at DESC
		LIMIT 1000`

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("listing simulations: %w", err)
	}
	defer rows.Close()

	var sims []Simulation
	for rows.Next() {
		var sim Simulation
		if err := rows.Scan(
			&sim.ID, &sim.Name, &sim.Status, &sim.CanvasCourseID, &sim.PathwayID,
			&sim.Config, &sim.StartedAt, &sim.CompletedAt, &sim.CreatedAt, &sim.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scanning simulation: %w", err)
		}
		sims = append(sims, sim)
	}
	return sims, rows.Err()
}

func (r *PostgresRepository) UpdateSimulationStatus(ctx context.Context, id, status string) error {
	var query string
	switch status {
	case "running":
		query = `UPDATE classroom_simulations SET status = $2, started_at = NOW(), updated_at = NOW() WHERE id = $1`
	case "completed", "failed":
		query = `UPDATE classroom_simulations SET status = $2, completed_at = NOW(), updated_at = NOW() WHERE id = $1`
	default:
		query = `UPDATE classroom_simulations SET status = $2, updated_at = NOW() WHERE id = $1`
	}

	result, err := r.db.ExecContext(ctx, query, id, status)
	if err != nil {
		return fmt.Errorf("updating simulation status: %w", err)
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return fmt.Errorf("simulation not found: %s", id)
	}
	return nil
}

func (r *PostgresRepository) DeleteSimulation(ctx context.Context, id string) error {
	result, err := r.db.ExecContext(ctx, `DELETE FROM classroom_simulations WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("deleting simulation: %w", err)
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return fmt.Errorf("simulation not found: %s", id)
	}
	return nil
}

func (r *PostgresRepository) CreateStudent(ctx context.Context, student *AIStudent) error {
	if student.ID == "" {
		student.ID = uuid.New().String()
	}

	query := `
		INSERT INTO ai_students (id, simulation_id, user_id, canvas_user_id, name, personality, traits, tech_skills, behavioral_config, state, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, NOW())
		RETURNING created_at`

	return r.db.QueryRowContext(ctx, query,
		student.ID, student.SimulationID, student.UserID, student.CanvasUserID,
		student.Name, string(student.Personality), student.Traits,
		student.TechSkills, student.BehavioralConfig, student.State,
	).Scan(&student.CreatedAt)
}

func (r *PostgresRepository) ListStudents(ctx context.Context, simulationID string) ([]AIStudent, error) {
	query := `
		SELECT id, simulation_id, user_id, canvas_user_id, name, personality,
		       traits, tech_skills, behavioral_config, state, created_at
		FROM ai_students WHERE simulation_id = $1
		ORDER BY created_at`

	rows, err := r.db.QueryContext(ctx, query, simulationID)
	if err != nil {
		return nil, fmt.Errorf("listing students: %w", err)
	}
	defer rows.Close()

	var students []AIStudent
	for rows.Next() {
		var s AIStudent
		if err := rows.Scan(
			&s.ID, &s.SimulationID, &s.UserID, &s.CanvasUserID, &s.Name,
			&s.Personality, &s.Traits, &s.TechSkills, &s.BehavioralConfig,
			&s.State, &s.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scanning student: %w", err)
		}
		students = append(students, s)
	}
	return students, rows.Err()
}

func (r *PostgresRepository) GetStudent(ctx context.Context, id string) (*AIStudent, error) {
	query := `
		SELECT id, simulation_id, user_id, canvas_user_id, name, personality,
		       traits, tech_skills, behavioral_config, state, created_at
		FROM ai_students WHERE id = $1`

	var s AIStudent
	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&s.ID, &s.SimulationID, &s.UserID, &s.CanvasUserID, &s.Name,
		&s.Personality, &s.Traits, &s.TechSkills, &s.BehavioralConfig,
		&s.State, &s.CreatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("getting student: %w", err)
	}
	return &s, nil
}

func (r *PostgresRepository) UpdateStudentState(ctx context.Context, id string, state json.RawMessage) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE ai_students SET state = $2 WHERE id = $1`, id, state)
	return err
}

func (r *PostgresRepository) CreateActivity(ctx context.Context, activity *Activity) error {
	query := `
		INSERT INTO ai_student_activities
			(simulation_id, student_id, activity_type, target_id, target_name, status, content, score, max_score, metadata, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, NOW())
		RETURNING id, created_at`

	if activity.Metadata == nil {
		activity.Metadata = json.RawMessage(`{}`)
	}

	return r.db.QueryRowContext(ctx, query,
		activity.SimulationID, activity.StudentID, activity.ActivityType,
		activity.TargetID, activity.TargetName, activity.Status,
		activity.Content, activity.Score, activity.MaxScore, activity.Metadata,
	).Scan(&activity.ID, &activity.CreatedAt)
}

func (r *PostgresRepository) ListActivities(ctx context.Context, simulationID string, limit, offset int) ([]Activity, int, error) {
	if limit <= 0 {
		limit = 50
	}

	var total int
	err := r.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM ai_student_activities WHERE simulation_id = $1`, simulationID,
	).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("counting activities: %w", err)
	}

	query := `
		SELECT id, simulation_id, student_id, activity_type, target_id, target_name,
		       status, content, score, max_score, metadata, created_at
		FROM ai_student_activities
		WHERE simulation_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3`

	rows, err := r.db.QueryContext(ctx, query, simulationID, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("listing activities: %w", err)
	}
	defer rows.Close()

	var activities []Activity
	for rows.Next() {
		var a Activity
		if err := rows.Scan(
			&a.ID, &a.SimulationID, &a.StudentID, &a.ActivityType,
			&a.TargetID, &a.TargetName, &a.Status, &a.Content,
			&a.Score, &a.MaxScore, &a.Metadata, &a.CreatedAt,
		); err != nil {
			return nil, 0, fmt.Errorf("scanning activity: %w", err)
		}
		activities = append(activities, a)
	}
	return activities, total, rows.Err()
}

func (r *PostgresRepository) ListStudentActivities(ctx context.Context, studentID string, limit, offset int) ([]Activity, int, error) {
	if limit <= 0 {
		limit = 50
	}

	var total int
	err := r.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM ai_student_activities WHERE student_id = $1`, studentID,
	).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("counting student activities: %w", err)
	}

	query := `
		SELECT id, simulation_id, student_id, activity_type, target_id, target_name,
		       status, content, score, max_score, metadata, created_at
		FROM ai_student_activities
		WHERE student_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3`

	rows, err := r.db.QueryContext(ctx, query, studentID, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("listing student activities: %w", err)
	}
	defer rows.Close()

	var activities []Activity
	for rows.Next() {
		var a Activity
		if err := rows.Scan(
			&a.ID, &a.SimulationID, &a.StudentID, &a.ActivityType,
			&a.TargetID, &a.TargetName, &a.Status, &a.Content,
			&a.Score, &a.MaxScore, &a.Metadata, &a.CreatedAt,
		); err != nil {
			return nil, 0, fmt.Errorf("scanning activity: %w", err)
		}
		activities = append(activities, a)
	}
	return activities, total, rows.Err()
}

func (r *PostgresRepository) CreateFeedback(ctx context.Context, fb *Feedback) error {
	if fb.ID == "" {
		fb.ID = uuid.New().String()
	}
	if fb.Details == nil {
		fb.Details = json.RawMessage(`{}`)
	}

	query := `
		INSERT INTO classroom_feedback
			(id, simulation_id, student_id, lab_template_id, pathway_id, session_id,
			 feedback_type, summary, rating, details, execution_duration_ms,
			 checkpoints_passed, checkpoints_total, score, max_score, execution_log, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, NOW())
		RETURNING created_at`

	return r.db.QueryRowContext(ctx, query,
		fb.ID, fb.SimulationID, fb.StudentID, fb.LabTemplateID, fb.PathwayID,
		fb.SessionID, fb.FeedbackType, fb.Summary, fb.Rating, fb.Details,
		fb.ExecutionDurationMs, fb.CheckpointsPassed, fb.CheckpointsTotal,
		fb.Score, fb.MaxScore, fb.ExecutionLog,
	).Scan(&fb.CreatedAt)
}

func (r *PostgresRepository) ListFeedback(ctx context.Context, simulationID string, opts FeedbackFilter) ([]Feedback, int, error) {
	if opts.Limit <= 0 {
		opts.Limit = 50
	}

	// Build WHERE clause
	where := "WHERE simulation_id = $1"
	args := []any{simulationID}
	argIdx := 2

	if opts.FeedbackType != "" {
		where += fmt.Sprintf(" AND feedback_type = $%d", argIdx)
		args = append(args, opts.FeedbackType)
		argIdx++
	}
	if opts.LabTemplateID != "" {
		where += fmt.Sprintf(" AND lab_template_id = $%d", argIdx)
		args = append(args, opts.LabTemplateID)
		argIdx++
	}

	var total int
	countQuery := "SELECT COUNT(*) FROM classroom_feedback " + where
	err := r.db.QueryRowContext(ctx, countQuery, args...).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("counting feedback: %w", err)
	}

	query := fmt.Sprintf(`
		SELECT id, simulation_id, student_id, lab_template_id, pathway_id, session_id,
		       feedback_type, summary, rating, details, execution_duration_ms,
		       checkpoints_passed, checkpoints_total, score, max_score, execution_log, created_at
		FROM classroom_feedback
		%s
		ORDER BY created_at DESC
		LIMIT $%d OFFSET $%d`, where, argIdx, argIdx+1)

	args = append(args, opts.Limit, opts.Offset)
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("listing feedback: %w", err)
	}
	defer rows.Close()

	var feedbacks []Feedback
	for rows.Next() {
		var fb Feedback
		if err := rows.Scan(
			&fb.ID, &fb.SimulationID, &fb.StudentID, &fb.LabTemplateID, &fb.PathwayID,
			&fb.SessionID, &fb.FeedbackType, &fb.Summary, &fb.Rating, &fb.Details,
			&fb.ExecutionDurationMs, &fb.CheckpointsPassed, &fb.CheckpointsTotal,
			&fb.Score, &fb.MaxScore, &fb.ExecutionLog, &fb.CreatedAt,
		); err != nil {
			return nil, 0, fmt.Errorf("scanning feedback: %w", err)
		}
		feedbacks = append(feedbacks, fb)
	}
	return feedbacks, total, rows.Err()
}

func (r *PostgresRepository) GetFeedbackSummary(ctx context.Context, simulationID string) (*FeedbackSummary, error) {
	summary := &FeedbackSummary{
		ByType: make(map[string]TypeSummary),
	}

	// Total and avg rating
	err := r.db.QueryRowContext(ctx,
		`SELECT COUNT(*), COALESCE(AVG(rating), 0) FROM classroom_feedback WHERE simulation_id = $1`,
		simulationID,
	).Scan(&summary.TotalFeedback, &summary.AvgRating)
	if err != nil {
		return nil, fmt.Errorf("getting feedback totals: %w", err)
	}

	// By type
	typeRows, err := r.db.QueryContext(ctx,
		`SELECT feedback_type, COUNT(*), COALESCE(AVG(rating), 0)
		 FROM classroom_feedback WHERE simulation_id = $1
		 GROUP BY feedback_type`, simulationID)
	if err != nil {
		return nil, fmt.Errorf("getting feedback by type: %w", err)
	}
	defer typeRows.Close()

	for typeRows.Next() {
		var fbType string
		var ts TypeSummary
		if err := typeRows.Scan(&fbType, &ts.Count, &ts.AvgRating); err != nil {
			return nil, fmt.Errorf("scanning type summary: %w", err)
		}
		summary.ByType[fbType] = ts
	}
	if err := typeRows.Err(); err != nil {
		return nil, err
	}

	// Infrastructure issue count
	if infra, ok := summary.ByType["infrastructure"]; ok {
		summary.InfrastructureIssues = infra.Count
	}

	// By lab
	labRows, err := r.db.QueryContext(ctx,
		`SELECT lab_template_id, COUNT(*), COALESCE(AVG(rating), 0)
		 FROM classroom_feedback
		 WHERE simulation_id = $1 AND lab_template_id IS NOT NULL
		 GROUP BY lab_template_id`, simulationID)
	if err != nil {
		return nil, fmt.Errorf("getting feedback by lab: %w", err)
	}
	defer labRows.Close()

	for labRows.Next() {
		var ls LabFeedbackSummary
		if err := labRows.Scan(&ls.LabTemplateID, &ls.Count, &ls.AvgRating); err != nil {
			return nil, fmt.Errorf("scanning lab summary: %w", err)
		}
		summary.ByLab = append(summary.ByLab, ls)
	}

	return summary, labRows.Err()
}
