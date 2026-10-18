package repositories

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/lib/pq"

	"github.com/toddbartholow/kootenai/api/internal/models"
)

// PathwayRepository defines operations for pathways and modules
type PathwayRepository interface {
	// Pathway CRUD
	Create(ctx context.Context, pathway *models.Pathway) error
	GetByID(ctx context.Context, id string) (*models.Pathway, error)
	GetByIDs(ctx context.Context, ids []string) ([]*models.Pathway, error) // Batch fetch to avoid N+1 queries
	GetBySlug(ctx context.Context, slug string) (*models.Pathway, error)
	List(ctx context.Context, opts models.PathwayListOptions) ([]*models.Pathway, error)
	Update(ctx context.Context, pathway *models.Pathway) error
	Delete(ctx context.Context, id string) error
	UpdateStatus(ctx context.Context, id string, status models.PathwayStatus) error

	// Pathway with full details
	GetWithModules(ctx context.Context, id string) (*models.Pathway, error)
	GetWithModulesBySlug(ctx context.Context, slug string) (*models.Pathway, error)

	// Module CRUD
	CreateModule(ctx context.Context, module *models.PathwayModule) error
	GetModuleByID(ctx context.Context, id string) (*models.PathwayModule, error)
	ListModules(ctx context.Context, pathwayID string) ([]*models.PathwayModule, error)
	UpdateModule(ctx context.Context, module *models.PathwayModule) error
	DeleteModule(ctx context.Context, id string) error
	ReorderModules(ctx context.Context, pathwayID string, moduleIDs []string) error

	// Module Labs
	AddLabToModule(ctx context.Context, moduleLab *models.ModuleLab) error
	RemoveLabFromModule(ctx context.Context, moduleID, labTemplateID string) error
	ListModuleLabs(ctx context.Context, moduleID string) ([]*models.ModuleLab, error)
	ReorderModuleLabs(ctx context.Context, moduleID string, labTemplateIDs []string) error

	// Stats
	GetStats(ctx context.Context, pathwayID string) (*models.PathwayStats, error)
}

// PathwayRepo implements PathwayRepository
type PathwayRepo struct {
	db DBTX
}

// NewPathwayRepo creates a new pathway repository
func NewPathwayRepo(db DBTX) *PathwayRepo {
	return &PathwayRepo{db: db}
}

// Create inserts a new pathway
func (r *PathwayRepo) Create(ctx context.Context, pathway *models.Pathway) error {
	prereqJSON, err := json.Marshal(pathway.Prerequisites)
	if err != nil {
		prereqJSON = []byte("[]")
	}

	query := `
		INSERT INTO pathways (
			id, name, slug, description, short_description,
			difficulty, estimated_hours, display_order,
			status, is_featured, organization_id, visibility,
			created_by, prerequisites, tags, icon, color, cover_image_url
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18)
		RETURNING created_at, updated_at`

	err = r.db.QueryRowContext(ctx, query,
		pathway.ID,
		pathway.Name,
		pathway.Slug,
		nullString(pathway.Description),
		nullString(pathway.ShortDescription),
		nullString(pathway.Difficulty),
		nullInt(pathway.EstimatedHours),
		pathway.DisplayOrder,
		pathway.Status,
		pathway.IsFeatured,
		nullStringPtr(pathway.OrganizationID),
		pathway.Visibility,
		nullStringPtr(pathway.CreatedBy),
		prereqJSON,
		pq.Array(pathway.Tags),
		nullString(pathway.Icon),
		nullString(pathway.Color),
		nullString(pathway.CoverImageURL),
	).Scan(&pathway.CreatedAt, &pathway.UpdatedAt)

	if err != nil {
		return fmt.Errorf("inserting pathway: %w", err)
	}

	return nil
}

// GetByID retrieves a pathway by ID
func (r *PathwayRepo) GetByID(ctx context.Context, id string) (*models.Pathway, error) {
	query := `
		SELECT id, name, slug, description, short_description,
		       difficulty, estimated_hours, display_order,
		       status, is_featured, organization_id, visibility,
		       created_by, prerequisites, tags, icon, color, cover_image_url,
		       created_at, updated_at
		FROM pathways
		WHERE id = $1`

	pathway, err := r.scanPathway(r.db.QueryRowContext(ctx, query, id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return pathway, err
}

// GetByIDs retrieves multiple pathways by their IDs in a single query
// This avoids N+1 queries when loading multiple pathways
func (r *PathwayRepo) GetByIDs(ctx context.Context, ids []string) ([]*models.Pathway, error) {
	if len(ids) == 0 {
		return []*models.Pathway{}, nil
	}

	query := `
		SELECT id, name, slug, description, short_description,
		       difficulty, estimated_hours, display_order,
		       status, is_featured, organization_id, visibility,
		       created_by, prerequisites, tags, icon, color, cover_image_url,
		       created_at, updated_at
		FROM pathways
		WHERE id = ANY($1)`

	rows, err := r.db.QueryContext(ctx, query, pq.Array(ids))
	if err != nil {
		return nil, fmt.Errorf("querying pathways by ids: %w", err)
	}
	defer rows.Close()

	return r.scanMultiplePathways(rows, false)
}

// GetBySlug retrieves a pathway by slug
func (r *PathwayRepo) GetBySlug(ctx context.Context, slug string) (*models.Pathway, error) {
	query := `
		SELECT id, name, slug, description, short_description,
		       difficulty, estimated_hours, display_order,
		       status, is_featured, organization_id, visibility,
		       created_by, prerequisites, tags, icon, color, cover_image_url,
		       created_at, updated_at
		FROM pathways
		WHERE slug = $1`

	pathway, err := r.scanPathway(r.db.QueryRowContext(ctx, query, slug))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return pathway, err
}

// List retrieves pathways matching the options
func (r *PathwayRepo) List(ctx context.Context, opts models.PathwayListOptions) ([]*models.Pathway, error) {
	// Build the base SELECT + FROM/JOIN depending on IncludeStats before
	// handing off to QueryBuilder. The WHERE 1=1 sentinel lets QueryBuilder
	// append conditions uniformly regardless of which JOIN path was taken.
	var base strings.Builder
	base.WriteString(`
		SELECT p.id, p.name, p.slug, p.description, p.short_description,
		       p.difficulty, p.estimated_hours, p.display_order,
		       p.status, p.is_featured, p.organization_id, p.visibility,
		       p.created_by, p.prerequisites, p.tags, p.icon, p.color, p.cover_image_url,
		       p.created_at, p.updated_at`)
	if opts.IncludeStats {
		base.WriteString(`,
			COALESCE(ps.module_count, 0) AS module_count,
			COALESCE(ps.lab_count, 0) AS lab_count,
			COALESCE(ps.total_points, 0) AS total_points,
			COALESCE(ps.total_duration_minutes, 0) AS total_duration_minutes,
			COALESCE(ps.enrollment_count, 0) AS enrollment_count,
			COALESCE(ps.completion_count, 0) AS completion_count`)
	}
	base.WriteString(`
		FROM pathways p`)
	if opts.IncludeStats {
		base.WriteString(`
		LEFT JOIN pathway_stats ps ON ps.id = p.id`)
	}
	base.WriteString(`
		WHERE 1=1`)

	qb := NewQueryBuilder(base.String())

	if opts.Status != "" {
		qb.AddCondition("p.status = $%d", opts.Status)
	}
	if opts.Visibility != "" {
		qb.AddCondition("p.visibility = $%d", opts.Visibility)
	}
	if opts.Difficulty != "" {
		qb.AddCondition("p.difficulty = $%d", opts.Difficulty)
	}
	if opts.IsFeatured != nil {
		qb.AddCondition("p.is_featured = $%d", *opts.IsFeatured)
	}
	if opts.OrganizationID != nil {
		qb.AddCondition("(p.organization_id = $%d OR p.visibility = 'global')", *opts.OrganizationID)
	}
	if len(opts.Tags) > 0 {
		qb.AddCondition("p.tags && $%d", pq.Array(opts.Tags))
	}
	if opts.Search != "" {
		searchTerm := "%" + opts.Search + "%"
		qb.AddConditionMulti(
			"(p.name ILIKE $%d OR p.description ILIKE $%d)",
			searchTerm, searchTerm,
		)
	}

	qb.OrderByRaw("p.display_order, p.name")
	qb.DefaultLimit(opts.Limit, 100)
	qb.Offset(opts.Offset)

	query, args := qb.Build()

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("querying pathways: %w", err)
	}
	defer rows.Close()

	return r.scanMultiplePathways(rows, opts.IncludeStats)
}

// Update updates a pathway
func (r *PathwayRepo) Update(ctx context.Context, pathway *models.Pathway) error {
	prereqJSON, err := json.Marshal(pathway.Prerequisites)
	if err != nil {
		prereqJSON = []byte("[]")
	}

	query := `
		UPDATE pathways
		SET name = $2, description = $3, short_description = $4,
		    difficulty = $5, estimated_hours = $6, display_order = $7,
		    status = $8, is_featured = $9, visibility = $10,
		    tags = $11, icon = $12, color = $13, cover_image_url = $14,
		    prerequisites = $15
		WHERE id = $1
		RETURNING updated_at`

	err = r.db.QueryRowContext(ctx, query,
		pathway.ID,
		pathway.Name,
		nullString(pathway.Description),
		nullString(pathway.ShortDescription),
		nullString(pathway.Difficulty),
		nullInt(pathway.EstimatedHours),
		pathway.DisplayOrder,
		pathway.Status,
		pathway.IsFeatured,
		pathway.Visibility,
		pq.Array(pathway.Tags),
		nullString(pathway.Icon),
		nullString(pathway.Color),
		nullString(pathway.CoverImageURL),
		prereqJSON,
	).Scan(&pathway.UpdatedAt)

	if err != nil {
		return fmt.Errorf("updating pathway: %w", err)
	}

	return nil
}

// Delete deletes a pathway
func (r *PathwayRepo) Delete(ctx context.Context, id string) error {
	query := `DELETE FROM pathways WHERE id = $1`
	_, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("deleting pathway: %w", err)
	}
	return nil
}

// UpdateStatus updates just the pathway status
func (r *PathwayRepo) UpdateStatus(ctx context.Context, id string, status models.PathwayStatus) error {
	query := `UPDATE pathways SET status = $2 WHERE id = $1`
	_, err := r.db.ExecContext(ctx, query, id, status)
	if err != nil {
		return fmt.Errorf("updating pathway status: %w", err)
	}
	return nil
}

// GetWithModules retrieves a pathway with all its modules and labs
func (r *PathwayRepo) GetWithModules(ctx context.Context, id string) (*models.Pathway, error) {
	pathway, err := r.GetByID(ctx, id)
	if err != nil || pathway == nil {
		return pathway, err
	}

	modules, err := r.ListModules(ctx, id)
	if err != nil {
		return nil, err
	}

	for _, module := range modules {
		labs, err := r.ListModuleLabs(ctx, module.ID)
		if err != nil {
			return nil, err
		}
		module.Labs = labs
		module.LabCount = len(labs)
		for _, lab := range labs {
			module.TotalPoints += lab.LabMaxPoints
		}
	}

	pathway.Modules = modules
	pathway.ModuleCount = len(modules)

	return pathway, nil
}

// GetWithModulesBySlug retrieves a pathway by slug with all its modules and labs
func (r *PathwayRepo) GetWithModulesBySlug(ctx context.Context, slug string) (*models.Pathway, error) {
	pathway, err := r.GetBySlug(ctx, slug)
	if err != nil || pathway == nil {
		return pathway, err
	}

	return r.GetWithModules(ctx, pathway.ID)
}

// Module operations

// CreateModule inserts a new module
func (r *PathwayRepo) CreateModule(ctx context.Context, module *models.PathwayModule) error {
	query := `
		INSERT INTO pathway_modules (
			id, pathway_id, name, slug, description,
			display_order, unlock_type, required_module_ids,
			is_active, icon, estimated_minutes
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		RETURNING created_at`

	err := r.db.QueryRowContext(ctx, query,
		module.ID,
		module.PathwayID,
		module.Name,
		module.Slug,
		nullString(module.Description),
		module.DisplayOrder,
		module.UnlockType,
		pq.Array(module.RequiredModuleIDs),
		module.IsActive,
		nullString(module.Icon),
		nullInt(module.EstimatedMinutes),
	).Scan(&module.CreatedAt)

	if err != nil {
		return fmt.Errorf("inserting module: %w", err)
	}

	return nil
}

// GetModuleByID retrieves a module by ID
func (r *PathwayRepo) GetModuleByID(ctx context.Context, id string) (*models.PathwayModule, error) {
	query := `
		SELECT id, pathway_id, name, slug, description,
		       display_order, unlock_type, required_module_ids,
		       is_active, icon, estimated_minutes, created_at
		FROM pathway_modules
		WHERE id = $1`

	module, err := r.scanModule(r.db.QueryRowContext(ctx, query, id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return module, err
}

// ListModules retrieves all modules for a pathway
func (r *PathwayRepo) ListModules(ctx context.Context, pathwayID string) ([]*models.PathwayModule, error) {
	query := `
		SELECT id, pathway_id, name, slug, description,
		       display_order, unlock_type, required_module_ids,
		       is_active, icon, estimated_minutes, created_at
		FROM pathway_modules
		WHERE pathway_id = $1 AND is_active = true
		ORDER BY display_order, name`

	rows, err := r.db.QueryContext(ctx, query, pathwayID)
	if err != nil {
		return nil, fmt.Errorf("querying modules: %w", err)
	}
	defer rows.Close()

	return r.scanMultipleModules(rows)
}

// UpdateModule updates a module
func (r *PathwayRepo) UpdateModule(ctx context.Context, module *models.PathwayModule) error {
	query := `
		UPDATE pathway_modules
		SET name = $2, description = $3, display_order = $4,
		    unlock_type = $5, is_active = $6, icon = $7, estimated_minutes = $8
		WHERE id = $1`

	_, err := r.db.ExecContext(ctx, query,
		module.ID,
		module.Name,
		nullString(module.Description),
		module.DisplayOrder,
		module.UnlockType,
		module.IsActive,
		nullString(module.Icon),
		nullInt(module.EstimatedMinutes),
	)

	if err != nil {
		return fmt.Errorf("updating module: %w", err)
	}

	return nil
}

// DeleteModule deletes a module
func (r *PathwayRepo) DeleteModule(ctx context.Context, id string) error {
	query := `DELETE FROM pathway_modules WHERE id = $1`
	_, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("deleting module: %w", err)
	}
	return nil
}

// ReorderModules updates the display order of modules
func (r *PathwayRepo) ReorderModules(ctx context.Context, pathwayID string, moduleIDs []string) error {
	tx, err := beginTx(ctx, r.db, nil)
	if err != nil {
		return fmt.Errorf("beginning transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	for i, id := range moduleIDs {
		_, err := tx.ExecContext(ctx,
			`UPDATE pathway_modules SET display_order = $1 WHERE id = $2 AND pathway_id = $3`,
			i, id, pathwayID)
		if err != nil {
			return fmt.Errorf("updating module order: %w", err)
		}
	}

	return tx.Commit()
}

// Module Labs operations

// AddLabToModule adds a lab to a module
func (r *PathwayRepo) AddLabToModule(ctx context.Context, moduleLab *models.ModuleLab) error {
	query := `
		INSERT INTO module_labs (
			id, module_id, lab_template_id, display_order,
			is_required, pass_threshold_override
		)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING created_at`

	err := r.db.QueryRowContext(ctx, query,
		moduleLab.ID,
		moduleLab.ModuleID,
		moduleLab.LabTemplateID,
		moduleLab.DisplayOrder,
		moduleLab.IsRequired,
		nullIntPtr(moduleLab.PassThresholdOverride),
	).Scan(&sql.NullTime{})

	if err != nil {
		return fmt.Errorf("adding lab to module: %w", err)
	}

	return nil
}

// RemoveLabFromModule removes a lab from a module
func (r *PathwayRepo) RemoveLabFromModule(ctx context.Context, moduleID, labTemplateID string) error {
	query := `DELETE FROM module_labs WHERE module_id = $1 AND lab_template_id = $2`
	_, err := r.db.ExecContext(ctx, query, moduleID, labTemplateID)
	if err != nil {
		return fmt.Errorf("removing lab from module: %w", err)
	}
	return nil
}

// ListModuleLabs retrieves all labs for a module with template details
func (r *PathwayRepo) ListModuleLabs(ctx context.Context, moduleID string) ([]*models.ModuleLab, error) {
	query := `
		SELECT ml.id, ml.module_id, ml.lab_template_id, ml.display_order,
		       ml.is_required, ml.pass_threshold_override,
		       lt.name, lt.description, lt.difficulty, lt.duration_minutes, lt.max_points, lt.platform
		FROM module_labs ml
		JOIN lab_templates lt ON lt.id = ml.lab_template_id
		WHERE ml.module_id = $1 AND lt.is_active = true
		ORDER BY ml.display_order`

	rows, err := r.db.QueryContext(ctx, query, moduleID)
	if err != nil {
		return nil, fmt.Errorf("querying module labs: %w", err)
	}
	defer rows.Close()

	var labs []*models.ModuleLab
	for rows.Next() {
		lab := &models.ModuleLab{}
		var passThresholdOverride sql.NullInt64
		var labDesc sql.NullString
		var labDiff sql.NullString
		var labDur sql.NullInt64

		err := rows.Scan(
			&lab.ID, &lab.ModuleID, &lab.LabTemplateID, &lab.DisplayOrder,
			&lab.IsRequired, &passThresholdOverride,
			&lab.LabName, &labDesc, &labDiff, &labDur, &lab.LabMaxPoints, &lab.LabPlatform,
		)
		if err != nil {
			return nil, fmt.Errorf("scanning module lab: %w", err)
		}

		if passThresholdOverride.Valid {
			v := int(passThresholdOverride.Int64)
			lab.PassThresholdOverride = &v
		}
		lab.LabDescription = labDesc.String
		lab.LabDifficulty = labDiff.String
		lab.LabDuration = int(labDur.Int64)

		labs = append(labs, lab)
	}

	return labs, rows.Err()
}

// ReorderModuleLabs updates the display order of labs in a module
func (r *PathwayRepo) ReorderModuleLabs(ctx context.Context, moduleID string, labTemplateIDs []string) error {
	tx, err := beginTx(ctx, r.db, nil)
	if err != nil {
		return fmt.Errorf("beginning transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	for i, labID := range labTemplateIDs {
		_, err := tx.ExecContext(ctx,
			`UPDATE module_labs SET display_order = $1 WHERE module_id = $2 AND lab_template_id = $3`,
			i, moduleID, labID)
		if err != nil {
			return fmt.Errorf("updating lab order: %w", err)
		}
	}

	return tx.Commit()
}

// GetStats retrieves statistics for a pathway
func (r *PathwayRepo) GetStats(ctx context.Context, pathwayID string) (*models.PathwayStats, error) {
	query := `
		SELECT id, module_count, lab_count, total_points,
		       total_duration_minutes, enrollment_count, completion_count
		FROM pathway_stats
		WHERE id = $1`

	stats := &models.PathwayStats{}
	err := r.db.QueryRowContext(ctx, query, pathwayID).Scan(
		&stats.PathwayID,
		&stats.ModuleCount,
		&stats.LabCount,
		&stats.TotalPoints,
		&stats.TotalDurationMinutes,
		&stats.EnrollmentCount,
		&stats.CompletionCount,
	)

	if errors.Is(err, sql.ErrNoRows) {
		return &models.PathwayStats{PathwayID: pathwayID}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("querying pathway stats: %w", err)
	}

	stats.ComputeCompletionRate()
	return stats, nil
}

// Helper methods

func (r *PathwayRepo) scanPathway(row scannable) (*models.Pathway, error) {
	var pathway models.Pathway
	var description, shortDesc, difficulty, orgID, createdBy sql.NullString
	var icon, color, coverImage sql.NullString
	var estimatedHours sql.NullInt64
	var prereqJSON []byte
	var tags []string

	err := row.Scan(
		&pathway.ID,
		&pathway.Name,
		&pathway.Slug,
		&description,
		&shortDesc,
		&difficulty,
		&estimatedHours,
		&pathway.DisplayOrder,
		&pathway.Status,
		&pathway.IsFeatured,
		&orgID,
		&pathway.Visibility,
		&createdBy,
		&prereqJSON,
		pq.Array(&tags),
		&icon,
		&color,
		&coverImage,
		&pathway.CreatedAt,
		&pathway.UpdatedAt,
	)

	if err != nil {
		return nil, fmt.Errorf("scanning pathway: %w", err)
	}

	pathway.Description = description.String
	pathway.ShortDescription = shortDesc.String
	pathway.Difficulty = difficulty.String
	pathway.EstimatedHours = int(estimatedHours.Int64)
	pathway.Icon = icon.String
	pathway.Color = color.String
	pathway.CoverImageURL = coverImage.String
	pathway.Tags = tags

	if orgID.Valid {
		pathway.OrganizationID = &orgID.String
	}
	if createdBy.Valid {
		pathway.CreatedBy = &createdBy.String
	}

	if len(prereqJSON) > 0 {
		pathway.Prerequisites = prereqJSON
	}

	return &pathway, nil
}

func (r *PathwayRepo) scanMultiplePathways(rows *sql.Rows, includeStats bool) ([]*models.Pathway, error) {
	var pathways []*models.Pathway
	for rows.Next() {
		var pathway models.Pathway
		var description, shortDesc, difficulty, orgID, createdBy sql.NullString
		var icon, color, coverImage sql.NullString
		var estimatedHours sql.NullInt64
		var prereqJSON []byte
		var tags []string

		scanArgs := []any{
			&pathway.ID,
			&pathway.Name,
			&pathway.Slug,
			&description,
			&shortDesc,
			&difficulty,
			&estimatedHours,
			&pathway.DisplayOrder,
			&pathway.Status,
			&pathway.IsFeatured,
			&orgID,
			&pathway.Visibility,
			&createdBy,
			&prereqJSON,
			pq.Array(&tags),
			&icon,
			&color,
			&coverImage,
			&pathway.CreatedAt,
			&pathway.UpdatedAt,
		}

		if includeStats {
			scanArgs = append(scanArgs,
				&pathway.ModuleCount,
				&pathway.LabCount,
				&pathway.TotalPoints,
				&pathway.TotalDuration,
				&pathway.EnrollmentCount,
				&pathway.CompletionCount,
			)
		}

		err := rows.Scan(scanArgs...)
		if err != nil {
			return nil, fmt.Errorf("scanning pathway row: %w", err)
		}

		pathway.Description = description.String
		pathway.ShortDescription = shortDesc.String
		pathway.Difficulty = difficulty.String
		pathway.EstimatedHours = int(estimatedHours.Int64)
		pathway.Icon = icon.String
		pathway.Color = color.String
		pathway.CoverImageURL = coverImage.String
		pathway.Tags = tags

		if orgID.Valid {
			pathway.OrganizationID = &orgID.String
		}
		if createdBy.Valid {
			pathway.CreatedBy = &createdBy.String
		}

		if len(prereqJSON) > 0 {
			pathway.Prerequisites = prereqJSON
		}

		pathways = append(pathways, &pathway)
	}

	return pathways, rows.Err()
}

func (r *PathwayRepo) scanModule(row scannable) (*models.PathwayModule, error) {
	var module models.PathwayModule
	var description, icon sql.NullString
	var estimatedMinutes sql.NullInt64
	var requiredModuleIDs []string

	err := row.Scan(
		&module.ID,
		&module.PathwayID,
		&module.Name,
		&module.Slug,
		&description,
		&module.DisplayOrder,
		&module.UnlockType,
		pq.Array(&requiredModuleIDs),
		&module.IsActive,
		&icon,
		&estimatedMinutes,
		&module.CreatedAt,
	)

	if err != nil {
		return nil, fmt.Errorf("scanning module: %w", err)
	}

	module.Description = description.String
	module.Icon = icon.String
	module.EstimatedMinutes = int(estimatedMinutes.Int64)
	module.RequiredModuleIDs = requiredModuleIDs

	return &module, nil
}

func (r *PathwayRepo) scanMultipleModules(rows *sql.Rows) ([]*models.PathwayModule, error) {
	var modules []*models.PathwayModule
	for rows.Next() {
		module, err := r.scanModule(rows)
		if err != nil {
			return nil, err
		}
		modules = append(modules, module)
	}
	return modules, rows.Err()
}

// GenerateSlug creates a URL-safe slug from a name
func GenerateSlug(name string) string {
	slug := strings.ToLower(name)
	slug = strings.ReplaceAll(slug, " ", "-")
	// Remove any characters that aren't alphanumeric or hyphens
	var result strings.Builder
	for _, r := range slug {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			result.WriteRune(r)
		}
	}
	return result.String()
}
