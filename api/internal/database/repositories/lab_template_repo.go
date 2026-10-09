package repositories

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"

	"github.com/lib/pq"

	"github.com/toddbartholow/kootenai/api/internal/models"
)

// LabTemplateRepo implements LabTemplateRepository
type LabTemplateRepo struct {
	db DBTX
}

// NewLabTemplateRepo creates a new lab template repository
func NewLabTemplateRepo(db DBTX) *LabTemplateRepo {
	return &LabTemplateRepo{db: db}
}

// Create inserts a new lab template
func (r *LabTemplateRepo) Create(ctx context.Context, record *models.LabTemplateRecord) error {
	query := `
		INSERT INTO lab_templates (
			name, slug, description, version, platform, duration_minutes,
			difficulty, category, tags, max_points, pass_threshold, spec,
			checkpoints, instructions, is_active, organization_id, visibility,
			created_by
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14,
		        $15, $16, $17, $18)
		RETURNING id, created_at, updated_at`

	// Default visibility to global if not set
	visibility := record.Visibility
	if visibility == "" {
		visibility = models.LabVisibilityGlobal
	}

	err := r.db.QueryRowContext(ctx, query,
		record.Name,
		record.Slug,
		nullString(record.Description),
		record.Version,
		record.Platform,
		nullInt(record.DurationMinutes),
		nullString(record.Difficulty),
		nullString(record.Category),
		pq.Array(record.Tags),
		record.MaxPoints,
		record.PassThreshold,
		record.Spec,
		nullJSONRaw(record.Checkpoints),
		nullJSONRaw(record.Instructions),
		record.IsActive,
		nullStringPtr(record.OrganizationID),
		visibility,
		nullStringPtr(record.CreatedBy),
	).Scan(&record.ID, &record.CreatedAt, &record.UpdatedAt)

	if err != nil {
		return fmt.Errorf("inserting lab template: %w", err)
	}

	record.Visibility = visibility
	return nil
}

// GetByID retrieves a lab template by ID
func (r *LabTemplateRepo) GetByID(ctx context.Context, id string) (*models.LabTemplateRecord, error) {
	query := `
		SELECT id, name, slug, description, version, platform, duration_minutes,
		       difficulty, category, tags, max_points, pass_threshold, spec, checkpoints, instructions,
		       created_at, updated_at, is_active,
		       organization_id, visibility, created_by, COALESCE(min_edition, 'community') as min_edition
		FROM lab_templates
		WHERE id = $1`

	record, err := r.scanRecord(r.db.QueryRowContext(ctx, query, id))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("getting lab template by ID: %w", err)
	}
	return record, nil
}

// GetByName retrieves a lab template by name
func (r *LabTemplateRepo) GetByName(ctx context.Context, name string) (*models.LabTemplateRecord, error) {
	query := `
		SELECT id, name, slug, description, version, platform, duration_minutes,
		       difficulty, category, tags, max_points, pass_threshold, spec, checkpoints, instructions,
		       created_at, updated_at, is_active,
		       organization_id, visibility, created_by, COALESCE(min_edition, 'community') as min_edition
		FROM lab_templates
		WHERE name = $1`

	record, err := r.scanRecord(r.db.QueryRowContext(ctx, query, name))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("getting lab template by name: %w", err)
	}
	return record, nil
}

// List retrieves lab templates matching the filter
func (r *LabTemplateRepo) List(ctx context.Context, filter LabTemplateFilter) ([]*models.LabTemplateRecord, error) {
	baseQuery := `
		SELECT id, name, slug, description, version, platform, duration_minutes,
		       difficulty, category, tags, max_points, pass_threshold, spec, checkpoints, instructions,
		       created_at, updated_at, is_active,
		       organization_id, visibility, created_by, COALESCE(min_edition, 'community') as min_edition
		FROM lab_templates
		WHERE 1=1`

	qb := NewQueryBuilder(baseQuery)

	if filter.Platform != "" {
		qb.AddCondition("platform = $%d", filter.Platform)
	}

	if filter.Active != nil {
		qb.AddCondition("is_active = $%d", *filter.Active)
	}

	// Organization and visibility filtering
	if filter.OrganizationID != "" {
		if filter.IncludeGlobal {
			// Include org-specific templates AND global templates
			qb.AddCondition("(organization_id = $%d OR visibility = 'global')", filter.OrganizationID)
		} else {
			// Only org-specific templates
			qb.AddCondition("organization_id = $%d", filter.OrganizationID)
		}
	}

	if filter.Visibility != "" {
		qb.AddCondition("visibility = $%d", filter.Visibility)
	}

	if filter.CreatedBy != "" {
		qb.AddCondition("created_by = $%d", filter.CreatedBy)
	}

	if filter.Category != "" {
		qb.AddCondition("category = $%d", filter.Category)
	}

	// Edition filtering - include labs at or below the specified edition level
	if filter.MaxEdition != "" {
		switch filter.MaxEdition {
		case models.EditionCommunity:
			qb.AddRawCondition("COALESCE(min_edition, 'community') = 'community'")
		case models.EditionProfessional:
			qb.AddRawCondition("COALESCE(min_edition, 'community') IN ('community', 'professional')")
		case models.EditionEnterprise:
			// Enterprise includes all editions, no filter needed
		}
	}

	qb.OrderByRaw("name ASC") // Hardcoded, safe
	qb.DefaultLimit(filter.Limit, 100)
	qb.Offset(filter.Offset)

	query, args := qb.Build()

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("listing lab templates: %w", err)
	}
	defer rows.Close()

	var records []*models.LabTemplateRecord
	for rows.Next() {
		record, err := r.scanRecordRow(rows)
		if err != nil {
			return nil, err
		}
		records = append(records, record)
	}

	return records, rows.Err()
}

// Update updates an existing lab template, auto-snapshotting the current state first.
// The snapshot and update are wrapped in a transaction for consistency.
func (r *LabTemplateRepo) Update(ctx context.Context, record *models.LabTemplateRecord) error {
	tx, err := beginTx(ctx, r.db, nil)
	if err != nil {
		return fmt.Errorf("beginning update transaction: %w", err)
	}
	defer tx.Rollback()

	// Snapshot current state before applying the update
	existingQuery := `
		SELECT id, name, slug, description, version, platform, duration_minutes,
		       difficulty, category, tags, max_points, pass_threshold, spec, checkpoints, instructions,
		       created_at, updated_at, is_active,
		       organization_id, visibility, created_by, COALESCE(min_edition, 'community') as min_edition
		FROM lab_templates
		WHERE id = $1
		FOR UPDATE`

	existing, err := r.scanRecord(tx.QueryRowContext(ctx, existingQuery, record.ID))
	if err == nil && existing != nil {
		version := &models.LabTemplateVersion{
			LabTemplateID:   existing.ID,
			Name:            existing.Name,
			Slug:            existing.Slug,
			Description:     existing.Description,
			Version:         existing.Version,
			Platform:        existing.Platform,
			DurationMinutes: existing.DurationMinutes,
			Difficulty:      existing.Difficulty,
			Category:        existing.Category,
			Tags:            existing.Tags,
			MaxPoints:       existing.MaxPoints,
			PassThreshold:   existing.PassThreshold,
			Spec:            existing.Spec,
			Checkpoints:     existing.Checkpoints,
			Instructions:    existing.Instructions,
			IsActive:        existing.IsActive,
			OrganizationID:  existing.OrganizationID,
			Visibility:      existing.Visibility,
			CreatedBy:       existing.CreatedBy,
			MinEdition:      existing.MinEdition,
		}
		if err := r.createVersionTx(ctx, tx, version); err != nil {
			slog.Warn("failed to snapshot template version", "template_id", existing.ID, "error", err)
		}
	}

	query := `
		UPDATE lab_templates
		SET name = $2, slug = $3, description = $4, version = $5, platform = $6,
		    duration_minutes = $7, difficulty = $8, category = $9, tags = $10,
		    max_points = $11, pass_threshold = $12, spec = $13, checkpoints = $14,
		    instructions = $15, is_active = $16, organization_id = $17,
		    visibility = $18, created_by = $19
		WHERE id = $1
		RETURNING updated_at`

	// Default visibility to global if not set
	visibility := record.Visibility
	if visibility == "" {
		visibility = models.LabVisibilityGlobal
	}

	err = tx.QueryRowContext(ctx, query,
		record.ID,
		record.Name,
		record.Slug,
		nullString(record.Description),
		record.Version,
		record.Platform,
		nullInt(record.DurationMinutes),
		nullString(record.Difficulty),
		nullString(record.Category),
		pq.Array(record.Tags),
		record.MaxPoints,
		record.PassThreshold,
		record.Spec,
		record.Checkpoints,
		nullJSONRaw(record.Instructions),
		record.IsActive,
		nullStringPtr(record.OrganizationID),
		visibility,
		nullStringPtr(record.CreatedBy),
	).Scan(&record.UpdatedAt)

	if err != nil {
		return fmt.Errorf("updating lab template: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("committing update transaction: %w", err)
	}

	return nil
}

// createVersionTx inserts a version snapshot within an existing transaction
func (r *LabTemplateRepo) createVersionTx(ctx context.Context, tx *sql.Tx, version *models.LabTemplateVersion) error {
	query := `
		INSERT INTO lab_template_versions (
			lab_template_id, version_number, name, slug, description, version, platform,
			duration_minutes, difficulty, category, tags, max_points, pass_threshold,
			spec, checkpoints, instructions, is_active, organization_id, visibility,
			created_by, min_edition, change_summary, created_by_user_id
		)
		VALUES (
			$1, get_next_version_number($1), $2, $3, $4, $5, $6,
			$7, $8, $9, $10, $11, $12,
			$13, $14, $15, $16, $17, $18,
			$19, $20, $21, $22
		)
		RETURNING id, version_number, created_at`

	return tx.QueryRowContext(ctx, query,
		version.LabTemplateID,
		version.Name,
		version.Slug,
		nullString(version.Description),
		version.Version,
		version.Platform,
		nullInt(version.DurationMinutes),
		nullString(version.Difficulty),
		nullString(version.Category),
		nullJSON(version.Tags),
		version.MaxPoints,
		version.PassThreshold,
		nullJSONRaw(version.Spec),
		nullJSONRaw(version.Checkpoints),
		nullJSONRaw(version.Instructions),
		version.IsActive,
		nullStringPtr(version.OrganizationID),
		version.Visibility,
		nullStringPtr(version.CreatedBy),
		nullString(string(version.MinEdition)),
		nullString(version.ChangeSummary),
		nullStringPtr(version.CreatedByUserID),
	).Scan(&version.ID, &version.VersionNumber, &version.CreatedAt)
}

// CreateVersion inserts a new version snapshot
func (r *LabTemplateRepo) CreateVersion(ctx context.Context, version *models.LabTemplateVersion) error {
	query := `
		INSERT INTO lab_template_versions (
			lab_template_id, version_number, name, slug, description, version, platform,
			duration_minutes, difficulty, category, tags, max_points, pass_threshold,
			spec, checkpoints, instructions, is_active, organization_id, visibility,
			created_by, min_edition, change_summary, created_by_user_id
		)
		VALUES (
			$1, get_next_version_number($1), $2, $3, $4, $5, $6,
			$7, $8, $9, $10, $11, $12,
			$13, $14, $15, $16, $17, $18,
			$19, $20, $21, $22
		)
		RETURNING id, version_number, created_at`

	err := r.db.QueryRowContext(ctx, query,
		version.LabTemplateID,
		version.Name,
		version.Slug,
		nullString(version.Description),
		version.Version,
		version.Platform,
		nullInt(version.DurationMinutes),
		nullString(version.Difficulty),
		nullString(version.Category),
		nullJSON(version.Tags),
		version.MaxPoints,
		version.PassThreshold,
		nullJSONRaw(version.Spec),
		nullJSONRaw(version.Checkpoints),
		nullJSONRaw(version.Instructions),
		version.IsActive,
		nullStringPtr(version.OrganizationID),
		version.Visibility,
		nullStringPtr(version.CreatedBy),
		nullString(string(version.MinEdition)),
		nullString(version.ChangeSummary),
		nullStringPtr(version.CreatedByUserID),
	).Scan(&version.ID, &version.VersionNumber, &version.CreatedAt)

	if err != nil {
		return fmt.Errorf("creating lab template version: %w", err)
	}
	return nil
}

// ListVersions returns version history for a template
func (r *LabTemplateRepo) ListVersions(ctx context.Context, templateID string, limit, offset int) ([]*models.LabTemplateVersion, error) {
	query := `
		SELECT id, lab_template_id, version_number, name, slug, description, version, platform,
		       duration_minutes, difficulty, category, tags, max_points, pass_threshold,
		       spec, checkpoints, instructions, is_active, organization_id, visibility,
		       created_by, COALESCE(min_edition, 'community'), change_summary, created_by_user_id, created_at
		FROM lab_template_versions
		WHERE lab_template_id = $1
		ORDER BY version_number DESC
		LIMIT $2 OFFSET $3`

	if limit <= 0 {
		limit = 50
	}

	rows, err := r.db.QueryContext(ctx, query, templateID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("listing lab template versions: %w", err)
	}
	defer rows.Close()

	var versions []*models.LabTemplateVersion
	for rows.Next() {
		v, err := scanVersion(rows)
		if err != nil {
			return nil, err
		}
		versions = append(versions, v)
	}
	return versions, rows.Err()
}

// GetVersionByNumber retrieves a specific version
func (r *LabTemplateRepo) GetVersionByNumber(ctx context.Context, templateID string, versionNumber int) (*models.LabTemplateVersion, error) {
	query := `
		SELECT id, lab_template_id, version_number, name, slug, description, version, platform,
		       duration_minutes, difficulty, category, tags, max_points, pass_threshold,
		       spec, checkpoints, instructions, is_active, organization_id, visibility,
		       created_by, COALESCE(min_edition, 'community'), change_summary, created_by_user_id, created_at
		FROM lab_template_versions
		WHERE lab_template_id = $1 AND version_number = $2`

	row := r.db.QueryRowContext(ctx, query, templateID, versionNumber)
	v, err := scanVersion(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("getting lab template version: %w", err)
	}
	return v, nil
}

// CountVersions returns the total number of versions for a template
func (r *LabTemplateRepo) CountVersions(ctx context.Context, templateID string) (int, error) {
	var count int
	err := r.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM lab_template_versions WHERE lab_template_id = $1`,
		templateID,
	).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("counting lab template versions: %w", err)
	}
	return count, nil
}

func scanVersion(row scannableLabTemplate) (*models.LabTemplateVersion, error) {
	var v models.LabTemplateVersion
	var description, difficulty, category sql.NullString
	var durationMinutes sql.NullInt64
	var checkpoints, instructions, tagsJSON []byte
	var organizationID, createdBy, changeSummary, createdByUserID sql.NullString
	var visibility, minEdition string

	err := row.Scan(
		&v.ID, &v.LabTemplateID, &v.VersionNumber,
		&v.Name, &v.Slug, &description, &v.Version, &v.Platform,
		&durationMinutes, &difficulty, &category, &tagsJSON,
		&v.MaxPoints, &v.PassThreshold,
		&v.Spec, &checkpoints, &instructions,
		&v.IsActive, &organizationID, &visibility,
		&createdBy, &minEdition, &changeSummary, &createdByUserID, &v.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("scanning lab template version: %w", err)
	}

	v.Description = description.String
	v.Difficulty = difficulty.String
	v.Category = category.String
	v.DurationMinutes = int(durationMinutes.Int64)
	v.Checkpoints = checkpoints
	v.Instructions = instructions
	v.Visibility = models.LabVisibility(visibility)
	v.MinEdition = models.Edition(minEdition)
	v.ChangeSummary = changeSummary.String
	if organizationID.Valid {
		v.OrganizationID = &organizationID.String
	}
	if createdBy.Valid {
		v.CreatedBy = &createdBy.String
	}
	if createdByUserID.Valid {
		v.CreatedByUserID = &createdByUserID.String
	}
	if len(tagsJSON) > 0 {
		_ = json.Unmarshal(tagsJSON, &v.Tags)
	}

	return &v, nil
}

// Delete removes a lab template
func (r *LabTemplateRepo) Delete(ctx context.Context, id string) error {
	query := `DELETE FROM lab_templates WHERE id = $1`

	result, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("deleting lab template: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("checking rows affected: %w", err)
	}

	if rows == 0 {
		return sql.ErrNoRows
	}

	return nil
}

// SetActive sets the active status of a lab template
func (r *LabTemplateRepo) SetActive(ctx context.Context, id string, active bool) error {
	query := `UPDATE lab_templates SET is_active = $2 WHERE id = $1`

	_, err := r.db.ExecContext(ctx, query, id, active)
	if err != nil {
		return fmt.Errorf("setting lab template active status: %w", err)
	}

	return nil
}

// Upsert creates or updates a lab template based on name or slug.
// Catches unique constraint violations to handle concurrent inserts gracefully.
func (r *LabTemplateRepo) Upsert(ctx context.Context, record *models.LabTemplateRecord) error {
	// Check if template exists by name first
	existing, err := r.GetByName(ctx, record.Name)
	if err != nil {
		return fmt.Errorf("checking existing template by name: %w", err)
	}

	// If not found by name, try by slug to handle name variations
	if existing == nil && record.Slug != "" {
		existing, err = r.GetBySlug(ctx, record.Slug)
		if err != nil {
			return fmt.Errorf("checking existing template by slug: %w", err)
		}
	}

	if existing != nil {
		// Update existing - preserve ID and slug
		record.ID = existing.ID
		record.Slug = existing.Slug // Preserve existing slug to avoid constraint violation
		return r.Update(ctx, record)
	}

	// Create new — if a concurrent insert wins, catch the unique constraint
	// violation and fall back to update
	if err := r.Create(ctx, record); err != nil {
		// Check for unique constraint violation (PostgreSQL error code 23505)
		if pqErr, ok := err.(*pq.Error); ok && pqErr.Code == "23505" {
			// Re-fetch and update instead
			existing, fetchErr := r.GetByName(ctx, record.Name)
			if fetchErr != nil {
				return fmt.Errorf("fetching after conflict: %w", fetchErr)
			}
			if existing != nil {
				record.ID = existing.ID
				record.Slug = existing.Slug
				return r.Update(ctx, record)
			}
		}
		return err
	}

	return nil
}

// GetByIDs retrieves multiple lab templates by their IDs in a single query
// This avoids N+1 queries when loading multiple templates
func (r *LabTemplateRepo) GetByIDs(ctx context.Context, ids []string) ([]*models.LabTemplateRecord, error) {
	if len(ids) == 0 {
		return []*models.LabTemplateRecord{}, nil
	}

	query := `
		SELECT id, name, slug, description, version, platform, duration_minutes,
		       difficulty, category, tags, max_points, pass_threshold, spec, checkpoints, instructions,
		       created_at, updated_at, is_active,
		       organization_id, visibility, created_by, COALESCE(min_edition, 'community') as min_edition
		FROM lab_templates
		WHERE id = ANY($1)`

	rows, err := r.db.QueryContext(ctx, query, pq.Array(ids))
	if err != nil {
		return nil, fmt.Errorf("querying lab templates by ids: %w", err)
	}
	defer rows.Close()

	var records []*models.LabTemplateRecord
	for rows.Next() {
		record, err := r.scanRecordRow(rows)
		if err != nil {
			return nil, err
		}
		records = append(records, record)
	}

	return records, rows.Err()
}

// GetBySlug retrieves a lab template by slug
func (r *LabTemplateRepo) GetBySlug(ctx context.Context, slug string) (*models.LabTemplateRecord, error) {
	query := `
		SELECT id, name, slug, description, version, platform, duration_minutes,
		       difficulty, category, tags, max_points, pass_threshold, spec, checkpoints, instructions,
		       created_at, updated_at, is_active,
		       organization_id, visibility, created_by, COALESCE(min_edition, 'community') as min_edition
		FROM lab_templates
		WHERE slug = $1`

	record, err := r.scanRecord(r.db.QueryRowContext(ctx, query, slug))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("getting lab template by slug: %w", err)
	}
	return record, nil
}

// Helper methods

type scannableLabTemplate interface {
	Scan(dest ...any) error
}

func (r *LabTemplateRepo) scanRecord(row scannableLabTemplate) (*models.LabTemplateRecord, error) {
	var record models.LabTemplateRecord
	var description, difficulty, category sql.NullString
	var durationMinutes sql.NullInt64
	var checkpoints, instructions []byte
	var organizationID, createdBy sql.NullString
	var visibility, minEdition string
	// lab_templates.tags is TEXT[] (migration 013). Let the driver decode it:
	// Postgres quotes any element containing a space, comma, quote or
	// backslash, and a hand-rolled strings.Split on "," silently corrupts
	// those -- turning one tag with a comma into two, and leaving literal
	// quotes on any tag with a space.
	var tags pq.StringArray

	err := row.Scan(
		&record.ID,
		&record.Name,
		&record.Slug,
		&description,
		&record.Version,
		&record.Platform,
		&durationMinutes,
		&difficulty,
		&category,
		&tags,
		&record.MaxPoints,
		&record.PassThreshold,
		&record.Spec,
		&checkpoints,
		&instructions,
		&record.CreatedAt,
		&record.UpdatedAt,
		&record.IsActive,
		&organizationID,
		&visibility,
		&createdBy,
		&minEdition,
	)

	if err != nil {
		return nil, fmt.Errorf("scanning lab template: %w", err)
	}

	record.Description = description.String
	record.Difficulty = difficulty.String
	record.Category = category.String
	record.DurationMinutes = int(durationMinutes.Int64)
	record.Checkpoints = checkpoints
	record.Instructions = instructions
	record.Visibility = models.LabVisibility(visibility)
	record.MinEdition = models.Edition(minEdition)
	if organizationID.Valid {
		record.OrganizationID = &organizationID.String
	}
	if createdBy.Valid {
		record.CreatedBy = &createdBy.String
	}

	record.Tags = tags

	return &record, nil
}

func (r *LabTemplateRepo) scanRecordRow(rows *sql.Rows) (*models.LabTemplateRecord, error) {
	return r.scanRecord(rows)
}

// pgArrayToSlice parses a PostgreSQL TEXT[] literal like {a,b,c} into a string slice.
// nullInt returns a sql.NullInt64 for the given value
func nullInt(v int) sql.NullInt64 {
	if v == 0 {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: int64(v), Valid: true}
}

// nullStringPtr returns a sql.NullString for a string pointer
func nullStringPtr(v *string) sql.NullString {
	if v == nil || *v == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: *v, Valid: true}
}

// nullJSONRaw returns nil (SQL NULL) for empty/nil json.RawMessage, otherwise the raw bytes
func nullJSONRaw(v json.RawMessage) any {
	if len(v) == 0 {
		return nil
	}
	return v
}

// nullJSON marshals a value to JSON bytes, returning nil for nil/empty slices
func nullJSON(v any) any {
	if v == nil {
		return nil
	}
	b, err := json.Marshal(v)
	if err != nil || len(b) == 0 || string(b) == "null" {
		return nil
	}
	return b
}
