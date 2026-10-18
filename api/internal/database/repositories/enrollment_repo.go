package repositories

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/toddbartholow/kootenai/api/internal/models"
)

// EnrollmentRepository defines operations for pathway enrollments and progress
type EnrollmentRepository interface {
	// Enrollment CRUD
	Create(ctx context.Context, enrollment *models.PathwayEnrollment) error
	GetByID(ctx context.Context, id string) (*models.PathwayEnrollment, error)
	GetByUserAndPathway(ctx context.Context, userID, pathwayID string) (*models.PathwayEnrollment, error)
	List(ctx context.Context, opts models.EnrollmentListOptions) ([]*models.PathwayEnrollment, error)
	Update(ctx context.Context, enrollment *models.PathwayEnrollment) error
	Delete(ctx context.Context, id string) error
	UpdateStatus(ctx context.Context, id string, status models.EnrollmentStatus) error

	// Enrollment with progress
	GetWithProgress(ctx context.Context, id string) (*models.PathwayEnrollment, error)

	// Module progress
	InitializeModuleProgress(ctx context.Context, enrollmentID, pathwayID string) error
	GetModuleProgress(ctx context.Context, enrollmentID, moduleID string) (*models.ModuleProgress, error)
	ListModuleProgress(ctx context.Context, enrollmentID string) ([]*models.ModuleProgress, error)
	UpdateModuleProgress(ctx context.Context, progress *models.ModuleProgress) error
	UnlockModule(ctx context.Context, enrollmentID, moduleID string) error
	UnlockNextModules(ctx context.Context, enrollmentID, completedModuleID string) ([]string, error)

	// Lab progress
	GetLabProgress(ctx context.Context, enrollmentID, labTemplateID string) (*models.LabProgress, error)
	ListLabProgress(ctx context.Context, enrollmentID, moduleID string) ([]*models.LabProgress, error)
	UpdateLabProgress(ctx context.Context, progress *models.LabProgress) error
	RecordLabAttempt(ctx context.Context, enrollmentID, moduleID, labTemplateID, sessionID string, score int, passed bool) error

	// Progress calculations (trigger updates)
	RecalculateModuleProgress(ctx context.Context, enrollmentID, moduleID string) error
	RecalculateEnrollmentProgress(ctx context.Context, enrollmentID string) error
}

// EnrollmentRepo implements EnrollmentRepository
type EnrollmentRepo struct {
	db DBTX
}

// NewEnrollmentRepo creates a new enrollment repository
func NewEnrollmentRepo(db DBTX) *EnrollmentRepo {
	return &EnrollmentRepo{db: db}
}

// Create inserts a new enrollment
func (r *EnrollmentRepo) Create(ctx context.Context, enrollment *models.PathwayEnrollment) error {
	query := `
		INSERT INTO pathway_enrollments (
			id, user_id, pathway_id, status,
			completed_modules, total_modules,
			earned_points, max_points, percentage,
			enrolled_at, organization_id
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		RETURNING enrolled_at`

	err := r.db.QueryRowContext(ctx, query,
		enrollment.ID,
		enrollment.UserID,
		enrollment.PathwayID,
		enrollment.Status,
		enrollment.CompletedModules,
		enrollment.TotalModules,
		enrollment.EarnedPoints,
		enrollment.MaxPoints,
		enrollment.Percentage,
		time.Now(),
		nullStringPtr(enrollment.OrganizationID),
	).Scan(&enrollment.EnrolledAt)

	if err != nil {
		return fmt.Errorf("inserting enrollment: %w", err)
	}

	return nil
}

// GetByID retrieves an enrollment by ID
func (r *EnrollmentRepo) GetByID(ctx context.Context, id string) (*models.PathwayEnrollment, error) {
	query := `
		SELECT id, user_id, pathway_id, status,
		       completed_modules, total_modules,
		       earned_points, max_points, percentage,
		       enrolled_at, started_at, completed_at, last_activity_at,
		       organization_id, certificate_issued, certificate_url
		FROM pathway_enrollments
		WHERE id = $1`

	enrollment, err := r.scanEnrollment(r.db.QueryRowContext(ctx, query, id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return enrollment, err
}

// GetByUserAndPathway retrieves an enrollment by user and pathway
func (r *EnrollmentRepo) GetByUserAndPathway(ctx context.Context, userID, pathwayID string) (*models.PathwayEnrollment, error) {
	query := `
		SELECT id, user_id, pathway_id, status,
		       completed_modules, total_modules,
		       earned_points, max_points, percentage,
		       enrolled_at, started_at, completed_at, last_activity_at,
		       organization_id, certificate_issued, certificate_url
		FROM pathway_enrollments
		WHERE user_id = $1 AND pathway_id = $2`

	enrollment, err := r.scanEnrollment(r.db.QueryRowContext(ctx, query, userID, pathwayID))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return enrollment, err
}

// List retrieves enrollments matching the options
func (r *EnrollmentRepo) List(ctx context.Context, opts models.EnrollmentListOptions) ([]*models.PathwayEnrollment, error) {
	baseQuery := `
		SELECT e.id, e.user_id, e.pathway_id, e.status,
		       e.completed_modules, e.total_modules,
		       e.earned_points, e.max_points, e.percentage,
		       e.enrolled_at, e.started_at, e.completed_at, e.last_activity_at,
		       e.organization_id, e.certificate_issued, e.certificate_url
		FROM pathway_enrollments e
		WHERE 1=1`

	qb := NewQueryBuilder(baseQuery)

	if opts.UserID != "" {
		qb.AddCondition("e.user_id = $%d", opts.UserID)
	}

	if opts.PathwayID != "" {
		qb.AddCondition("e.pathway_id = $%d", opts.PathwayID)
	}

	if opts.Status != "" {
		qb.AddCondition("e.status = $%d", opts.Status)
	}

	if opts.OrganizationID != nil {
		qb.AddCondition("e.organization_id = $%d", *opts.OrganizationID)
	}

	// Use OrderByRaw for complex ORDER BY with multiple columns (hardcoded, safe)
	qb.OrderByRaw("e.last_activity_at DESC NULLS LAST, e.enrolled_at DESC")
	qb.DefaultLimit(opts.Limit, 100)
	qb.Offset(opts.Offset)

	query, args := qb.Build()

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("querying enrollments: %w", err)
	}
	defer rows.Close()

	return r.scanMultipleEnrollments(rows)
}

// Update updates an enrollment
func (r *EnrollmentRepo) Update(ctx context.Context, enrollment *models.PathwayEnrollment) error {
	query := `
		UPDATE pathway_enrollments
		SET status = $2, completed_modules = $3, total_modules = $4,
		    earned_points = $5, max_points = $6, percentage = $7,
		    started_at = $8, completed_at = $9, last_activity_at = $10,
		    certificate_issued = $11, certificate_url = $12
		WHERE id = $1`

	_, err := r.db.ExecContext(ctx, query,
		enrollment.ID,
		enrollment.Status,
		enrollment.CompletedModules,
		enrollment.TotalModules,
		enrollment.EarnedPoints,
		enrollment.MaxPoints,
		enrollment.Percentage,
		enrollment.StartedAt,
		enrollment.CompletedAt,
		enrollment.LastActivityAt,
		enrollment.CertificateIssued,
		nullString(enrollment.CertificateURL),
	)

	if err != nil {
		return fmt.Errorf("updating enrollment: %w", err)
	}

	return nil
}

// Delete deletes an enrollment
func (r *EnrollmentRepo) Delete(ctx context.Context, id string) error {
	query := `DELETE FROM pathway_enrollments WHERE id = $1`
	_, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("deleting enrollment: %w", err)
	}
	return nil
}

// UpdateStatus updates just the enrollment status
func (r *EnrollmentRepo) UpdateStatus(ctx context.Context, id string, status models.EnrollmentStatus) error {
	query := `UPDATE pathway_enrollments SET status = $2, last_activity_at = NOW() WHERE id = $1`
	_, err := r.db.ExecContext(ctx, query, id, status)
	if err != nil {
		return fmt.Errorf("updating enrollment status: %w", err)
	}
	return nil
}

// GetWithProgress retrieves an enrollment with all module and lab progress
func (r *EnrollmentRepo) GetWithProgress(ctx context.Context, id string) (*models.PathwayEnrollment, error) {
	enrollment, err := r.GetByID(ctx, id)
	if err != nil || enrollment == nil {
		return enrollment, err
	}

	moduleProgress, err := r.ListModuleProgress(ctx, id)
	if err != nil {
		return nil, err
	}

	for _, mp := range moduleProgress {
		labProgress, err := r.ListLabProgress(ctx, id, mp.ModuleID)
		if err != nil {
			return nil, err
		}
		mp.LabProgress = labProgress
	}

	enrollment.ModuleProgress = moduleProgress
	return enrollment, nil
}

// InitializeModuleProgress creates module progress records for a new enrollment
func (r *EnrollmentRepo) InitializeModuleProgress(ctx context.Context, enrollmentID, pathwayID string) error {
	// Get all modules and labs for the pathway
	query := `
		INSERT INTO module_progress (id, enrollment_id, module_id, status, total_labs, max_points)
		SELECT
			uuid_generate_v4(),
			$1,
			pm.id,
			CASE WHEN pm.unlock_type = 'always' OR pm.display_order = 0 THEN 'unlocked'::module_status ELSE 'locked'::module_status END,
			(SELECT COUNT(*) FROM module_labs ml WHERE ml.module_id = pm.id),
			COALESCE((
				SELECT SUM(lt.max_points)
				FROM module_labs ml
				JOIN lab_templates lt ON lt.id = ml.lab_template_id
				WHERE ml.module_id = pm.id
			), 0)
		FROM pathway_modules pm
		WHERE pm.pathway_id = $2 AND pm.is_active = true`

	_, err := r.db.ExecContext(ctx, query, enrollmentID, pathwayID)
	if err != nil {
		return fmt.Errorf("initializing module progress: %w", err)
	}

	// Also initialize lab progress for unlocked modules
	labQuery := `
		INSERT INTO lab_progress (id, enrollment_id, module_id, lab_template_id, max_points)
		SELECT
			uuid_generate_v4(),
			$1,
			pm.id,
			ml.lab_template_id,
			lt.max_points
		FROM pathway_modules pm
		JOIN module_labs ml ON ml.module_id = pm.id
		JOIN lab_templates lt ON lt.id = ml.lab_template_id
		WHERE pm.pathway_id = $2 AND pm.is_active = true AND lt.is_active = true`

	_, err = r.db.ExecContext(ctx, labQuery, enrollmentID, pathwayID)
	if err != nil {
		return fmt.Errorf("initializing lab progress: %w", err)
	}

	// Update enrollment with total modules
	updateQuery := `
		UPDATE pathway_enrollments
		SET total_modules = (
			SELECT COUNT(*) FROM pathway_modules WHERE pathway_id = $2 AND is_active = true
		),
		max_points = (
			SELECT COALESCE(SUM(lt.max_points), 0)
			FROM pathway_modules pm
			JOIN module_labs ml ON ml.module_id = pm.id
			JOIN lab_templates lt ON lt.id = ml.lab_template_id
			WHERE pm.pathway_id = $2 AND pm.is_active = true AND lt.is_active = true
		)
		WHERE id = $1`

	_, err = r.db.ExecContext(ctx, updateQuery, enrollmentID, pathwayID)
	if err != nil {
		return fmt.Errorf("updating enrollment totals: %w", err)
	}

	return nil
}

// GetModuleProgress retrieves progress for a specific module
func (r *EnrollmentRepo) GetModuleProgress(ctx context.Context, enrollmentID, moduleID string) (*models.ModuleProgress, error) {
	query := `
		SELECT id, enrollment_id, module_id, status,
		       completed_labs, total_labs, earned_points, max_points,
		       unlocked_at, started_at, completed_at
		FROM module_progress
		WHERE enrollment_id = $1 AND module_id = $2`

	progress, err := r.scanModuleProgress(r.db.QueryRowContext(ctx, query, enrollmentID, moduleID))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return progress, err
}

// ListModuleProgress retrieves all module progress for an enrollment
func (r *EnrollmentRepo) ListModuleProgress(ctx context.Context, enrollmentID string) ([]*models.ModuleProgress, error) {
	query := `
		SELECT mp.id, mp.enrollment_id, mp.module_id, mp.status,
		       mp.completed_labs, mp.total_labs, mp.earned_points, mp.max_points,
		       mp.unlocked_at, mp.started_at, mp.completed_at
		FROM module_progress mp
		JOIN pathway_modules pm ON pm.id = mp.module_id
		WHERE mp.enrollment_id = $1
		ORDER BY pm.display_order`

	rows, err := r.db.QueryContext(ctx, query, enrollmentID)
	if err != nil {
		return nil, fmt.Errorf("querying module progress: %w", err)
	}
	defer rows.Close()

	return r.scanMultipleModuleProgress(rows)
}

// UpdateModuleProgress updates module progress
func (r *EnrollmentRepo) UpdateModuleProgress(ctx context.Context, progress *models.ModuleProgress) error {
	query := `
		UPDATE module_progress
		SET status = $2, completed_labs = $3, earned_points = $4,
		    unlocked_at = $5, started_at = $6, completed_at = $7
		WHERE id = $1`

	_, err := r.db.ExecContext(ctx, query,
		progress.ID,
		progress.Status,
		progress.CompletedLabs,
		progress.EarnedPoints,
		progress.UnlockedAt,
		progress.StartedAt,
		progress.CompletedAt,
	)

	if err != nil {
		return fmt.Errorf("updating module progress: %w", err)
	}

	return nil
}

// UnlockModule marks a module as unlocked
func (r *EnrollmentRepo) UnlockModule(ctx context.Context, enrollmentID, moduleID string) error {
	query := `
		UPDATE module_progress
		SET status = 'unlocked'::module_status, unlocked_at = NOW()
		WHERE enrollment_id = $1 AND module_id = $2 AND status = 'locked'::module_status`

	_, err := r.db.ExecContext(ctx, query, enrollmentID, moduleID)
	if err != nil {
		return fmt.Errorf("unlocking module: %w", err)
	}

	return nil
}

// UnlockNextModules checks and unlocks modules that should be unlocked after a module completion
// Returns the list of module IDs that were unlocked
func (r *EnrollmentRepo) UnlockNextModules(ctx context.Context, enrollmentID, completedModuleID string) ([]string, error) {
	// Get the enrollment to find the pathway
	enrollment, err := r.GetByID(ctx, enrollmentID)
	if err != nil {
		return nil, fmt.Errorf("getting enrollment: %w", err)
	}
	if enrollment == nil {
		return nil, fmt.Errorf("enrollment not found: %s", enrollmentID)
	}

	// Get the completed module's display order
	var completedOrder int
	err = r.db.QueryRowContext(ctx, `
		SELECT pm.display_order FROM pathway_modules pm
		WHERE pm.id = $1`, completedModuleID).Scan(&completedOrder)
	if err != nil {
		return nil, fmt.Errorf("getting completed module order: %w", err)
	}

	// Query for locked modules that could be unlocked
	// Unlock sequential modules if previous is completed
	// Unlock all_previous modules if all prior modules are completed
	// Always and manual don't need automatic unlock
	query := `
		WITH completed_modules AS (
			SELECT mp.module_id, pm.display_order
			FROM module_progress mp
			JOIN pathway_modules pm ON pm.id = mp.module_id
			WHERE mp.enrollment_id = $1 AND mp.status = 'completed'
		),
		unlockable_modules AS (
			SELECT pm.id as module_id
			FROM pathway_modules pm
			JOIN module_progress mp ON mp.module_id = pm.id AND mp.enrollment_id = $1
			WHERE pm.pathway_id = $2
			  AND mp.status = 'locked'
			  AND (
				-- Sequential: unlock if the immediately previous module is completed
				(pm.unlock_type = 'sequential' AND pm.display_order = $3 + 1 AND EXISTS (
					SELECT 1 FROM completed_modules cm WHERE cm.module_id = $4
				))
				OR
				-- All previous: unlock if all modules with lower display_order are completed
				(pm.unlock_type = 'all_previous' AND NOT EXISTS (
					SELECT 1 FROM pathway_modules pm2
					JOIN module_progress mp2 ON mp2.module_id = pm2.id AND mp2.enrollment_id = $1
					WHERE pm2.pathway_id = $2
					  AND pm2.display_order < pm.display_order
					  AND mp2.status != 'completed'
				))
			  )
		)
		UPDATE module_progress
		SET status = 'unlocked'::module_status, unlocked_at = NOW()
		WHERE enrollment_id = $1 AND module_id IN (SELECT module_id FROM unlockable_modules)
		RETURNING module_id`

	rows, err := r.db.QueryContext(ctx, query, enrollmentID, enrollment.PathwayID, completedOrder, completedModuleID)
	if err != nil {
		return nil, fmt.Errorf("unlocking next modules: %w", err)
	}
	defer rows.Close()

	var unlockedModuleIDs []string
	for rows.Next() {
		var moduleID string
		if err := rows.Scan(&moduleID); err != nil {
			return nil, fmt.Errorf("scanning unlocked module ID: %w", err)
		}
		unlockedModuleIDs = append(unlockedModuleIDs, moduleID)
	}

	return unlockedModuleIDs, rows.Err()
}

// GetLabProgress retrieves progress for a specific lab
func (r *EnrollmentRepo) GetLabProgress(ctx context.Context, enrollmentID, labTemplateID string) (*models.LabProgress, error) {
	query := `
		SELECT id, enrollment_id, module_id, lab_template_id,
		       best_session_id, attempt_count, best_score, max_points,
		       passed, first_attempt_at, completed_at
		FROM lab_progress
		WHERE enrollment_id = $1 AND lab_template_id = $2`

	progress, err := r.scanLabProgress(r.db.QueryRowContext(ctx, query, enrollmentID, labTemplateID))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return progress, err
}

// ListLabProgress retrieves all lab progress for a module in an enrollment
func (r *EnrollmentRepo) ListLabProgress(ctx context.Context, enrollmentID, moduleID string) ([]*models.LabProgress, error) {
	query := `
		SELECT lp.id, lp.enrollment_id, lp.module_id, lp.lab_template_id,
		       lp.best_session_id, lp.attempt_count, lp.best_score, lp.max_points,
		       lp.passed, lp.first_attempt_at, lp.completed_at
		FROM lab_progress lp
		JOIN module_labs ml ON ml.lab_template_id = lp.lab_template_id AND ml.module_id = lp.module_id
		WHERE lp.enrollment_id = $1 AND lp.module_id = $2
		ORDER BY ml.display_order`

	rows, err := r.db.QueryContext(ctx, query, enrollmentID, moduleID)
	if err != nil {
		return nil, fmt.Errorf("querying lab progress: %w", err)
	}
	defer rows.Close()

	return r.scanMultipleLabProgress(rows)
}

// UpdateLabProgress updates lab progress
func (r *EnrollmentRepo) UpdateLabProgress(ctx context.Context, progress *models.LabProgress) error {
	query := `
		UPDATE lab_progress
		SET best_session_id = $2, attempt_count = $3, best_score = $4,
		    passed = $5, first_attempt_at = $6, completed_at = $7
		WHERE id = $1`

	_, err := r.db.ExecContext(ctx, query,
		progress.ID,
		nullStringPtr(progress.BestSessionID),
		progress.AttemptCount,
		progress.BestScore,
		progress.Passed,
		progress.FirstAttemptAt,
		progress.CompletedAt,
	)

	if err != nil {
		return fmt.Errorf("updating lab progress: %w", err)
	}

	return nil
}

// RecordLabAttempt records a lab attempt and updates progress
func (r *EnrollmentRepo) RecordLabAttempt(ctx context.Context, enrollmentID, moduleID, labTemplateID, sessionID string, score int, passed bool) error {
	tx, err := beginTx(ctx, r.db, nil)
	if err != nil {
		return fmt.Errorf("beginning transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	now := time.Now()

	// Upsert lab progress
	query := `
		INSERT INTO lab_progress (
			id, enrollment_id, module_id, lab_template_id,
			best_session_id, attempt_count, best_score, max_points,
			passed, first_attempt_at, completed_at
		)
		SELECT
			uuid_generate_v4(),
			$1, $2, $3, $4, 1, $5,
			lt.max_points,
			$6,
			$7::timestamptz,
			CASE WHEN $6 THEN $7::timestamptz ELSE NULL END
		FROM lab_templates lt
		WHERE lt.id = $3
		ON CONFLICT (enrollment_id, lab_template_id)
		DO UPDATE SET
			attempt_count = lab_progress.attempt_count + 1,
			best_session_id = CASE
				WHEN $5 > lab_progress.best_score THEN $4
				ELSE lab_progress.best_session_id
			END,
			best_score = GREATEST(lab_progress.best_score, $5),
			passed = lab_progress.passed OR $6,
			first_attempt_at = COALESCE(lab_progress.first_attempt_at, $7::timestamptz),
			completed_at = CASE
				WHEN $6 AND lab_progress.completed_at IS NULL THEN $7::timestamptz
				ELSE lab_progress.completed_at
			END`

	_, err = tx.ExecContext(ctx, query,
		enrollmentID, moduleID, labTemplateID, sessionID, score, passed, now)
	if err != nil {
		return fmt.Errorf("recording lab attempt: %w", err)
	}

	// Update enrollment last activity
	_, err = tx.ExecContext(ctx,
		`UPDATE pathway_enrollments SET last_activity_at = $2, started_at = COALESCE(started_at, $2) WHERE id = $1`,
		enrollmentID, now)
	if err != nil {
		return fmt.Errorf("updating enrollment activity: %w", err)
	}

	return tx.Commit()
}

// EnrollmentWithPathway combines enrollment data with pathway details for dashboard
type EnrollmentWithPathway struct {
	// Enrollment fields
	ID               string                  `json:"id"`
	UserID           string                  `json:"userId"`
	PathwayID        string                  `json:"pathwayId"`
	Status           models.EnrollmentStatus `json:"status"`
	CompletedModules int                     `json:"completedModules"`
	TotalModules     int                     `json:"totalModules"`
	EarnedPoints     int                     `json:"earnedPoints"`
	MaxPoints        int                     `json:"maxPoints"`
	Percentage       float64                 `json:"percentage"`
	EnrolledAt       time.Time               `json:"enrolledAt"`
	StartedAt        *time.Time              `json:"startedAt,omitempty"`
	CompletedAt      *time.Time              `json:"completedAt,omitempty"`
	LastActivityAt   *time.Time              `json:"lastActivityAt,omitempty"`

	// Pathway fields (joined)
	PathwayName        string `json:"pathwayName"`
	PathwaySlug        string `json:"pathwaySlug"`
	PathwayDescription string `json:"pathwayDescription,omitempty"`
	PathwayIcon        string `json:"pathwayIcon,omitempty"`
	PathwayCoverImage  string `json:"pathwayCoverImage,omitempty"`
	PathwayDifficulty  string `json:"pathwayDifficulty,omitempty"`
}

// ListWithPathways retrieves enrollments with pathway details in a single query
// Avoids N+1 query pattern when fetching enrollments for dashboard
func (r *EnrollmentRepo) ListWithPathways(ctx context.Context, opts models.EnrollmentListOptions) ([]*EnrollmentWithPathway, error) {
	qb := NewQueryBuilder(`
		SELECT
			e.id, e.user_id, e.pathway_id, e.status,
			e.completed_modules, e.total_modules,
			e.earned_points, e.max_points, e.percentage,
			e.enrolled_at, e.started_at, e.completed_at, e.last_activity_at,
			p.name, p.slug, COALESCE(p.description, ''), COALESCE(p.icon, ''),
			COALESCE(p.cover_image_url, ''), COALESCE(p.difficulty, '')
		FROM pathway_enrollments e
		JOIN pathways p ON p.id = e.pathway_id
		WHERE 1=1`)

	if opts.UserID != "" {
		qb.AddCondition("e.user_id = $%d", opts.UserID)
	}
	if opts.PathwayID != "" {
		qb.AddCondition("e.pathway_id = $%d", opts.PathwayID)
	}
	if opts.Status != "" {
		qb.AddCondition("e.status = $%d", opts.Status)
	}

	qb.OrderByRaw("e.last_activity_at DESC NULLS LAST, e.enrolled_at DESC")
	qb.DefaultLimit(opts.Limit, 100)
	qb.Offset(opts.Offset)

	query, args := qb.Build()

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("querying enrollments with pathways: %w", err)
	}
	defer rows.Close()

	var enrollments []*EnrollmentWithPathway
	for rows.Next() {
		e := &EnrollmentWithPathway{}
		err := rows.Scan(
			&e.ID, &e.UserID, &e.PathwayID, &e.Status,
			&e.CompletedModules, &e.TotalModules,
			&e.EarnedPoints, &e.MaxPoints, &e.Percentage,
			&e.EnrolledAt, &e.StartedAt, &e.CompletedAt, &e.LastActivityAt,
			&e.PathwayName, &e.PathwaySlug, &e.PathwayDescription, &e.PathwayIcon,
			&e.PathwayCoverImage, &e.PathwayDifficulty,
		)
		if err != nil {
			return nil, fmt.Errorf("scanning enrollment with pathway: %w", err)
		}
		enrollments = append(enrollments, e)
	}
	return enrollments, rows.Err()
}

// RecalculateModuleProgress recalculates progress for a module
func (r *EnrollmentRepo) RecalculateModuleProgress(ctx context.Context, enrollmentID, moduleID string) error {
	query := `
		UPDATE module_progress mp
		SET
			completed_labs = (
				SELECT COUNT(*) FROM lab_progress lp
				WHERE lp.enrollment_id = mp.enrollment_id AND lp.module_id = mp.module_id AND lp.passed = true
			),
			earned_points = (
				SELECT COALESCE(SUM(lp.best_score), 0) FROM lab_progress lp
				WHERE lp.enrollment_id = mp.enrollment_id AND lp.module_id = mp.module_id
			),
			status = CASE
				WHEN (SELECT COUNT(*) FROM lab_progress lp
					  JOIN module_labs ml ON ml.lab_template_id = lp.lab_template_id AND ml.module_id = lp.module_id
					  WHERE lp.enrollment_id = mp.enrollment_id AND lp.module_id = mp.module_id
					  AND ml.is_required = true AND lp.passed = false) = 0
				AND (SELECT COUNT(*) FROM module_labs ml WHERE ml.module_id = mp.module_id AND ml.is_required = true) > 0
				THEN 'completed'::module_status
				WHEN mp.status != 'locked' AND (SELECT COUNT(*) FROM lab_progress lp WHERE lp.enrollment_id = mp.enrollment_id AND lp.module_id = mp.module_id AND lp.attempt_count > 0) > 0
				THEN 'in_progress'::module_status
				ELSE mp.status
			END,
			completed_at = CASE
				WHEN mp.status != 'completed' AND (
					SELECT COUNT(*) FROM lab_progress lp
					JOIN module_labs ml ON ml.lab_template_id = lp.lab_template_id AND ml.module_id = lp.module_id
					WHERE lp.enrollment_id = mp.enrollment_id AND lp.module_id = mp.module_id
					AND ml.is_required = true AND lp.passed = false
				) = 0 AND (SELECT COUNT(*) FROM module_labs ml WHERE ml.module_id = mp.module_id AND ml.is_required = true) > 0
				THEN NOW()
				ELSE mp.completed_at
			END
		WHERE mp.enrollment_id = $1 AND mp.module_id = $2`

	_, err := r.db.ExecContext(ctx, query, enrollmentID, moduleID)
	if err != nil {
		return fmt.Errorf("recalculating module progress: %w", err)
	}

	return nil
}

// RecalculateEnrollmentProgress recalculates progress for an enrollment
func (r *EnrollmentRepo) RecalculateEnrollmentProgress(ctx context.Context, enrollmentID string) error {
	query := `
		UPDATE pathway_enrollments pe
		SET
			completed_modules = (
				SELECT COUNT(*) FROM module_progress mp
				WHERE mp.enrollment_id = pe.id AND mp.status = 'completed'
			),
			earned_points = (
				SELECT COALESCE(SUM(mp.earned_points), 0) FROM module_progress mp
				WHERE mp.enrollment_id = pe.id
			),
			percentage = CASE
				WHEN pe.max_points > 0 THEN
					ROUND((SELECT COALESCE(SUM(mp.earned_points), 0)::DECIMAL FROM module_progress mp WHERE mp.enrollment_id = pe.id) / pe.max_points * 100, 2)
				ELSE 0
			END,
			status = CASE
				WHEN (SELECT COUNT(*) FROM module_progress mp WHERE mp.enrollment_id = pe.id AND mp.status = 'completed') = pe.total_modules AND pe.total_modules > 0
				THEN 'completed'::enrollment_status
				WHEN pe.started_at IS NOT NULL
				THEN 'in_progress'::enrollment_status
				ELSE pe.status
			END,
			completed_at = CASE
				WHEN pe.status != 'completed' AND
				     (SELECT COUNT(*) FROM module_progress mp WHERE mp.enrollment_id = pe.id AND mp.status = 'completed') = pe.total_modules
				     AND pe.total_modules > 0
				THEN NOW()
				ELSE pe.completed_at
			END
		WHERE pe.id = $1`

	_, err := r.db.ExecContext(ctx, query, enrollmentID)
	if err != nil {
		return fmt.Errorf("recalculating enrollment progress: %w", err)
	}

	return nil
}

// Helper methods

func (r *EnrollmentRepo) scanEnrollment(row scannable) (*models.PathwayEnrollment, error) {
	var enrollment models.PathwayEnrollment
	var startedAt, completedAt, lastActivityAt sql.NullTime
	var orgID, certURL sql.NullString

	err := row.Scan(
		&enrollment.ID,
		&enrollment.UserID,
		&enrollment.PathwayID,
		&enrollment.Status,
		&enrollment.CompletedModules,
		&enrollment.TotalModules,
		&enrollment.EarnedPoints,
		&enrollment.MaxPoints,
		&enrollment.Percentage,
		&enrollment.EnrolledAt,
		&startedAt,
		&completedAt,
		&lastActivityAt,
		&orgID,
		&enrollment.CertificateIssued,
		&certURL,
	)

	if err != nil {
		return nil, fmt.Errorf("scanning enrollment: %w", err)
	}

	if startedAt.Valid {
		enrollment.StartedAt = &startedAt.Time
	}
	if completedAt.Valid {
		enrollment.CompletedAt = &completedAt.Time
	}
	if lastActivityAt.Valid {
		enrollment.LastActivityAt = &lastActivityAt.Time
	}
	if orgID.Valid {
		enrollment.OrganizationID = &orgID.String
	}
	enrollment.CertificateURL = certURL.String

	return &enrollment, nil
}

func (r *EnrollmentRepo) scanMultipleEnrollments(rows *sql.Rows) ([]*models.PathwayEnrollment, error) {
	var enrollments []*models.PathwayEnrollment
	for rows.Next() {
		enrollment, err := r.scanEnrollment(rows)
		if err != nil {
			return nil, err
		}
		enrollments = append(enrollments, enrollment)
	}
	return enrollments, rows.Err()
}

func (r *EnrollmentRepo) scanModuleProgress(row scannable) (*models.ModuleProgress, error) {
	var progress models.ModuleProgress
	var unlockedAt, startedAt, completedAt sql.NullTime

	err := row.Scan(
		&progress.ID,
		&progress.EnrollmentID,
		&progress.ModuleID,
		&progress.Status,
		&progress.CompletedLabs,
		&progress.TotalLabs,
		&progress.EarnedPoints,
		&progress.MaxPoints,
		&unlockedAt,
		&startedAt,
		&completedAt,
	)

	if err != nil {
		return nil, fmt.Errorf("scanning module progress: %w", err)
	}

	if unlockedAt.Valid {
		progress.UnlockedAt = &unlockedAt.Time
	}
	if startedAt.Valid {
		progress.StartedAt = &startedAt.Time
	}
	if completedAt.Valid {
		progress.CompletedAt = &completedAt.Time
	}

	return &progress, nil
}

func (r *EnrollmentRepo) scanMultipleModuleProgress(rows *sql.Rows) ([]*models.ModuleProgress, error) {
	var progressList []*models.ModuleProgress
	for rows.Next() {
		progress, err := r.scanModuleProgress(rows)
		if err != nil {
			return nil, err
		}
		progressList = append(progressList, progress)
	}
	return progressList, rows.Err()
}

func (r *EnrollmentRepo) scanLabProgress(row scannable) (*models.LabProgress, error) {
	var progress models.LabProgress
	var bestSessionID sql.NullString
	var firstAttemptAt, completedAt sql.NullTime

	err := row.Scan(
		&progress.ID,
		&progress.EnrollmentID,
		&progress.ModuleID,
		&progress.LabTemplateID,
		&bestSessionID,
		&progress.AttemptCount,
		&progress.BestScore,
		&progress.MaxPoints,
		&progress.Passed,
		&firstAttemptAt,
		&completedAt,
	)

	if err != nil {
		return nil, fmt.Errorf("scanning lab progress: %w", err)
	}

	if bestSessionID.Valid {
		progress.BestSessionID = &bestSessionID.String
	}
	if firstAttemptAt.Valid {
		progress.FirstAttemptAt = &firstAttemptAt.Time
	}
	if completedAt.Valid {
		progress.CompletedAt = &completedAt.Time
	}

	return &progress, nil
}

func (r *EnrollmentRepo) scanMultipleLabProgress(rows *sql.Rows) ([]*models.LabProgress, error) {
	var progressList []*models.LabProgress
	for rows.Next() {
		progress, err := r.scanLabProgress(rows)
		if err != nil {
			return nil, err
		}
		progressList = append(progressList, progress)
	}
	return progressList, rows.Err()
}
