package repositories

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/toddbartholow/kootenai/api/internal/models"
)

// LTIAssignmentRepo implements LTI assignment repository
type LTIAssignmentRepo struct {
	db DBTX
}

// NewLTIAssignmentRepo creates a new LTI assignment repository
func NewLTIAssignmentRepo(db DBTX) *LTIAssignmentRepo {
	return &LTIAssignmentRepo{db: db}
}

// Create inserts a new LTI assignment mapping
func (r *LTIAssignmentRepo) Create(ctx context.Context, assignment *models.LTIAssignment) error {
	query := `
		INSERT INTO lti_assignments (
			id, canvas_course_id, resource_link_id, lab_template_id,
			custom_title, max_points, organization_id, created_by,
			deployment_id, lti_version
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		RETURNING created_at, updated_at`

	err := r.db.QueryRowContext(ctx, query,
		assignment.ID,
		assignment.CanvasCourseID,
		assignment.ResourceLinkID,
		assignment.LabTemplateID,
		nullString(assignment.CustomTitle),
		assignment.MaxPoints,
		nullStringPtr(assignment.OrganizationID),
		nullStringPtr(assignment.CreatedBy),
		nullString(assignment.DeploymentID),
		nullString(assignment.LTIVersion),
	).Scan(&assignment.CreatedAt, &assignment.UpdatedAt)

	if err != nil {
		return fmt.Errorf("inserting LTI assignment: %w", err)
	}

	return nil
}

// GetByID retrieves an LTI assignment by ID
func (r *LTIAssignmentRepo) GetByID(ctx context.Context, id string) (*models.LTIAssignment, error) {
	query := `
		SELECT id, canvas_course_id, resource_link_id, lab_template_id,
		       custom_title, max_points, organization_id, created_by,
		       deployment_id, lti_version, created_at, updated_at
		FROM lti_assignments
		WHERE id = $1`

	assignment, err := r.scanAssignment(r.db.QueryRowContext(ctx, query, id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return assignment, err
}

// GetByResourceLink retrieves an LTI assignment by course ID and resource link ID
func (r *LTIAssignmentRepo) GetByResourceLink(ctx context.Context, courseID, resourceLinkID string) (*models.LTIAssignment, error) {
	query := `
		SELECT id, canvas_course_id, resource_link_id, lab_template_id,
		       custom_title, max_points, organization_id, created_by,
		       deployment_id, lti_version, created_at, updated_at
		FROM lti_assignments
		WHERE canvas_course_id = $1 AND resource_link_id = $2`

	assignment, err := r.scanAssignment(r.db.QueryRowContext(ctx, query, courseID, resourceLinkID))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return assignment, err
}

// GetByCourseID retrieves all LTI assignments for a course
func (r *LTIAssignmentRepo) GetByCourseID(ctx context.Context, courseID string) ([]*models.LTIAssignment, error) {
	query := `
		SELECT id, canvas_course_id, resource_link_id, lab_template_id,
		       custom_title, max_points, organization_id, created_by,
		       deployment_id, lti_version, created_at, updated_at
		FROM lti_assignments
		WHERE canvas_course_id = $1
		ORDER BY created_at DESC`

	return r.queryMultiple(ctx, query, courseID)
}

// GetByTemplateID retrieves all LTI assignments for a template
func (r *LTIAssignmentRepo) GetByTemplateID(ctx context.Context, templateID string) ([]*models.LTIAssignment, error) {
	query := `
		SELECT id, canvas_course_id, resource_link_id, lab_template_id,
		       custom_title, max_points, organization_id, created_by,
		       deployment_id, lti_version, created_at, updated_at
		FROM lti_assignments
		WHERE lab_template_id = $1
		ORDER BY created_at DESC`

	return r.queryMultiple(ctx, query, templateID)
}

// Update updates an LTI assignment
func (r *LTIAssignmentRepo) Update(ctx context.Context, assignment *models.LTIAssignment) error {
	query := `
		UPDATE lti_assignments
		SET custom_title = $2, max_points = $3, updated_at = NOW()
		WHERE id = $1
		RETURNING updated_at`

	err := r.db.QueryRowContext(ctx, query,
		assignment.ID,
		nullString(assignment.CustomTitle),
		assignment.MaxPoints,
	).Scan(&assignment.UpdatedAt)

	if err != nil {
		return fmt.Errorf("updating LTI assignment: %w", err)
	}

	return nil
}

// Delete removes an LTI assignment by ID
func (r *LTIAssignmentRepo) Delete(ctx context.Context, id string) error {
	query := `DELETE FROM lti_assignments WHERE id = $1`

	result, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("deleting LTI assignment: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("getting rows affected: %w", err)
	}

	if rows == 0 {
		return sql.ErrNoRows
	}

	return nil
}

// scanAssignment scans a row into an LTIAssignment struct
func (r *LTIAssignmentRepo) scanAssignment(row *sql.Row) (*models.LTIAssignment, error) {
	var assignment models.LTIAssignment
	var customTitle, organizationID, createdBy, deploymentID, ltiVersion sql.NullString

	err := row.Scan(
		&assignment.ID,
		&assignment.CanvasCourseID,
		&assignment.ResourceLinkID,
		&assignment.LabTemplateID,
		&customTitle,
		&assignment.MaxPoints,
		&organizationID,
		&createdBy,
		&deploymentID,
		&ltiVersion,
		&assignment.CreatedAt,
		&assignment.UpdatedAt,
	)

	if err != nil {
		return nil, err
	}

	assignment.CustomTitle = customTitle.String
	if organizationID.Valid {
		assignment.OrganizationID = &organizationID.String
	}
	if createdBy.Valid {
		assignment.CreatedBy = &createdBy.String
	}
	assignment.DeploymentID = deploymentID.String
	assignment.LTIVersion = ltiVersion.String

	return &assignment, nil
}

// scanAssignments scans a row into an LTIAssignment struct (for queryMultiple)
func (r *LTIAssignmentRepo) scanAssignments(rows *sql.Rows) (*models.LTIAssignment, error) {
	var assignment models.LTIAssignment
	var customTitle, organizationID, createdBy, deploymentID, ltiVersion sql.NullString

	err := rows.Scan(
		&assignment.ID,
		&assignment.CanvasCourseID,
		&assignment.ResourceLinkID,
		&assignment.LabTemplateID,
		&customTitle,
		&assignment.MaxPoints,
		&organizationID,
		&createdBy,
		&deploymentID,
		&ltiVersion,
		&assignment.CreatedAt,
		&assignment.UpdatedAt,
	)

	if err != nil {
		return nil, err
	}

	assignment.CustomTitle = customTitle.String
	if organizationID.Valid {
		assignment.OrganizationID = &organizationID.String
	}
	if createdBy.Valid {
		assignment.CreatedBy = &createdBy.String
	}
	assignment.DeploymentID = deploymentID.String
	assignment.LTIVersion = ltiVersion.String

	return &assignment, nil
}

// queryMultiple executes a query and returns multiple assignments
func (r *LTIAssignmentRepo) queryMultiple(ctx context.Context, query string, args ...any) ([]*models.LTIAssignment, error) {
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("querying LTI assignments: %w", err)
	}
	defer rows.Close()

	var assignments []*models.LTIAssignment
	for rows.Next() {
		assignment, err := r.scanAssignments(rows)
		if err != nil {
			return nil, fmt.Errorf("scanning LTI assignment: %w", err)
		}
		assignments = append(assignments, assignment)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating LTI assignments: %w", err)
	}

	return assignments, nil
}

// Helper to count assignments for a course
func (r *LTIAssignmentRepo) CountByCourse(ctx context.Context, courseID string) (int, error) {
	var count int
	query := `SELECT COUNT(*) FROM lti_assignments WHERE canvas_course_id = $1`
	err := r.db.QueryRowContext(ctx, query, courseID).Scan(&count)
	return count, err
}

// GetWithTemplate retrieves an LTI assignment with its associated template info
type LTIAssignmentWithTemplate struct {
	Assignment *models.LTIAssignment
	Template   *models.LTITemplateInfo
}

func (r *LTIAssignmentRepo) GetWithTemplateByResourceLink(ctx context.Context, courseID, resourceLinkID string) (*LTIAssignmentWithTemplate, error) {
	query := `
		SELECT
			a.id, a.canvas_course_id, a.resource_link_id, a.lab_template_id,
			a.custom_title, a.max_points, a.organization_id, a.created_by,
			a.deployment_id, a.lti_version, a.created_at, a.updated_at,
			t.id, t.name, t.description, t.difficulty, t.duration_minutes, t.max_points,
			COALESCE(array_length(ARRAY(SELECT jsonb_array_elements(t.checkpoints)), 1), 0) as checkpoint_count,
			t.tags
		FROM lti_assignments a
		JOIN lab_templates t ON a.lab_template_id = t.id
		WHERE a.canvas_course_id = $1 AND a.resource_link_id = $2`

	var assignment models.LTIAssignment
	var template models.LTITemplateInfo
	var customTitle, organizationID, createdBy, deploymentID, ltiVersion sql.NullString
	var description, difficulty sql.NullString
	var durationMinutes sql.NullInt32
	var tagsJSON []byte

	err := r.db.QueryRowContext(ctx, query, courseID, resourceLinkID).Scan(
		&assignment.ID,
		&assignment.CanvasCourseID,
		&assignment.ResourceLinkID,
		&assignment.LabTemplateID,
		&customTitle,
		&assignment.MaxPoints,
		&organizationID,
		&createdBy,
		&deploymentID,
		&ltiVersion,
		&assignment.CreatedAt,
		&assignment.UpdatedAt,
		&template.ID,
		&template.Name,
		&description,
		&difficulty,
		&durationMinutes,
		&template.MaxPoints,
		&template.CheckpointCount,
		&tagsJSON,
	)

	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("querying LTI assignment with template: %w", err)
	}

	assignment.CustomTitle = customTitle.String
	if organizationID.Valid {
		assignment.OrganizationID = &organizationID.String
	}
	if createdBy.Valid {
		assignment.CreatedBy = &createdBy.String
	}
	assignment.DeploymentID = deploymentID.String
	assignment.LTIVersion = ltiVersion.String

	template.Description = description.String
	template.Difficulty = difficulty.String
	if durationMinutes.Valid {
		template.DurationMinutes = int(durationMinutes.Int32)
	}

	return &LTIAssignmentWithTemplate{
		Assignment: &assignment,
		Template:   &template,
	}, nil
}

// ListActiveTemplates returns all active templates for LTI selection
func (r *LTIAssignmentRepo) ListActiveTemplates(ctx context.Context, orgID *string) ([]*models.LTITemplateInfo, error) {
	query := `
		SELECT
			id, name,
			COALESCE(description, '') as description,
			COALESCE(difficulty, 'intermediate') as difficulty,
			COALESCE(duration_minutes, 60) as duration_minutes,
			max_points,
			CASE
				WHEN checkpoints IS NULL THEN 0
				WHEN jsonb_typeof(checkpoints) = 'array' THEN jsonb_array_length(checkpoints)
				ELSE 0
			END as checkpoint_count,
			COALESCE(tags, '{}') as tags
		FROM lab_templates
		WHERE is_active = true
		ORDER BY name`

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("querying templates: %w", err)
	}
	defer rows.Close()

	var templates []*models.LTITemplateInfo
	for rows.Next() {
		var t models.LTITemplateInfo
		var tagsJSON []byte

		err := rows.Scan(
			&t.ID,
			&t.Name,
			&t.Description,
			&t.Difficulty,
			&t.DurationMinutes,
			&t.MaxPoints,
			&t.CheckpointCount,
			&tagsJSON,
		)
		if err != nil {
			return nil, fmt.Errorf("scanning template: %w", err)
		}

		// Parse tags from array or JSON
		// PostgreSQL array format: {tag1,tag2} or JSON: ["tag1", "tag2"]
		t.Tags = parseTags(tagsJSON)
		templates = append(templates, &t)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating templates: %w", err)
	}

	return templates, nil
}

// parseTags parses PostgreSQL array or JSON tags
func parseTags(data []byte) []string {
	if len(data) == 0 {
		return []string{}
	}

	s := string(data)
	// Handle PostgreSQL array format: {tag1,tag2,tag3}
	if len(s) > 1 && s[0] == '{' && s[len(s)-1] == '}' {
		inner := s[1 : len(s)-1]
		if inner == "" {
			return []string{}
		}
		// Simple split - doesn't handle quoted values
		tags := []string{}
		current := ""
		inQuote := false
		for _, c := range inner {
			switch c {
			case '"':
				inQuote = !inQuote
			case ',':
				if !inQuote {
					if current != "" {
						tags = append(tags, current)
					}
					current = ""
					continue
				}
				current += string(c)
			default:
				current += string(c)
			}
		}
		if current != "" {
			tags = append(tags, current)
		}
		return tags
	}

	return []string{}
}

// Helper to check if assignment exists
func (r *LTIAssignmentRepo) Exists(ctx context.Context, courseID, resourceLinkID string) (bool, error) {
	var exists bool
	query := `SELECT EXISTS(SELECT 1 FROM lti_assignments WHERE canvas_course_id = $1 AND resource_link_id = $2)`
	err := r.db.QueryRowContext(ctx, query, courseID, resourceLinkID).Scan(&exists)
	return exists, err
}
