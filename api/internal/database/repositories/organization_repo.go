package repositories

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/lib/pq"

	"github.com/google/uuid"

	"github.com/toddbartholow/kootenai/api/internal/models"
)

// OrganizationRepo implements OrganizationRepository
type OrganizationRepo struct {
	db DBTX
}

// NewOrganizationRepo creates a new organization repository
func NewOrganizationRepo(db DBTX) *OrganizationRepo {
	return &OrganizationRepo{db: db}
}

// Create inserts a new organization
func (r *OrganizationRepo) Create(ctx context.Context, org *models.Organization) error {
	if org.ID == "" {
		org.ID = uuid.New().String()
	}

	settingsJSON, err := json.Marshal(org.Settings)
	if err != nil {
		return fmt.Errorf("marshaling settings: %w", err)
	}

	metadataJSON := []byte("{}")
	if org.Metadata != nil {
		metadataJSON = org.Metadata
	}

	query := `
		INSERT INTO organizations (
			id, name, slug, type, edition, license_key, license_expires_at,
			settings, max_users, max_concurrent_pods, max_storage_gb,
			logo_url, contact_email, is_active, metadata
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)
		RETURNING created_at, updated_at`

	err = r.db.QueryRowContext(ctx, query,
		org.ID,
		org.Name,
		org.Slug,
		org.Type,
		org.Edition,
		nullStringOrg(org.LicenseKey),
		nullTimePtr(org.LicenseExpiresAt),
		settingsJSON,
		nullIntPtr(org.MaxUsers),
		nullIntPtr(org.MaxConcurrentPods),
		nullIntPtr(org.MaxStorageGB),
		nullStringOrg(org.LogoURL),
		nullStringOrg(org.ContactEmail),
		org.IsActive,
		metadataJSON,
	).Scan(&org.CreatedAt, &org.UpdatedAt)

	if err != nil {
		return fmt.Errorf("inserting organization: %w", err)
	}

	return nil
}

// GetByID retrieves an organization by ID
func (r *OrganizationRepo) GetByID(ctx context.Context, id string) (*models.Organization, error) {
	query := `
		SELECT id, name, slug, type, edition, license_key, license_expires_at,
		       settings, max_users, max_concurrent_pods, max_storage_gb,
		       logo_url, contact_email, is_active, created_at, updated_at, metadata
		FROM organizations
		WHERE id = $1`

	return r.scanOrganization(ctx, query, id)
}

// GetBySlug retrieves an organization by slug
func (r *OrganizationRepo) GetBySlug(ctx context.Context, slug string) (*models.Organization, error) {
	query := `
		SELECT id, name, slug, type, edition, license_key, license_expires_at,
		       settings, max_users, max_concurrent_pods, max_storage_gb,
		       logo_url, contact_email, is_active, created_at, updated_at, metadata
		FROM organizations
		WHERE slug = $1`

	return r.scanOrganization(ctx, query, slug)
}

// scanOrganization is a helper to scan a single organization row
func (r *OrganizationRepo) scanOrganization(ctx context.Context, query string, arg any) (*models.Organization, error) {
	var org models.Organization
	var settingsJSON, metadataJSON []byte
	var licenseKey, logoURL, contactEmail sql.NullString
	var licenseExpiresAt sql.NullTime
	var maxUsers, maxConcurrentPods, maxStorageGB sql.NullInt32

	err := r.db.QueryRowContext(ctx, query, arg).Scan(
		&org.ID,
		&org.Name,
		&org.Slug,
		&org.Type,
		&org.Edition,
		&licenseKey,
		&licenseExpiresAt,
		&settingsJSON,
		&maxUsers,
		&maxConcurrentPods,
		&maxStorageGB,
		&logoURL,
		&contactEmail,
		&org.IsActive,
		&org.CreatedAt,
		&org.UpdatedAt,
		&metadataJSON,
	)

	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("querying organization: %w", err)
	}

	// Handle nullable fields
	if licenseKey.Valid {
		org.LicenseKey = licenseKey.String
	}
	if licenseExpiresAt.Valid {
		org.LicenseExpiresAt = &licenseExpiresAt.Time
	}
	if maxUsers.Valid {
		v := int(maxUsers.Int32)
		org.MaxUsers = &v
	}
	if maxConcurrentPods.Valid {
		v := int(maxConcurrentPods.Int32)
		org.MaxConcurrentPods = &v
	}
	if maxStorageGB.Valid {
		v := int(maxStorageGB.Int32)
		org.MaxStorageGB = &v
	}
	if logoURL.Valid {
		org.LogoURL = logoURL.String
	}
	if contactEmail.Valid {
		org.ContactEmail = contactEmail.String
	}

	// Unmarshal JSON fields
	if len(settingsJSON) > 0 {
		if err := json.Unmarshal(settingsJSON, &org.Settings); err != nil {
			return nil, fmt.Errorf("unmarshaling settings: %w", err)
		}
	}
	if len(metadataJSON) > 0 {
		org.Metadata = metadataJSON
	}

	return &org, nil
}

// List retrieves organizations matching the filter
func (r *OrganizationRepo) List(ctx context.Context, filter OrganizationFilter) ([]*models.Organization, error) {
	qb := NewQueryBuilder(`
		SELECT id, name, slug, type, edition, license_key, license_expires_at,
		       settings, max_users, max_concurrent_pods, max_storage_gb,
		       logo_url, contact_email, is_active, created_at, updated_at, metadata
		FROM organizations
		WHERE 1=1`)

	if filter.Type != "" {
		qb.AddCondition("type = $%d", filter.Type)
	}
	if filter.Edition != "" {
		qb.AddCondition("edition = $%d", filter.Edition)
	}
	if filter.Active != nil {
		qb.AddCondition("is_active = $%d", *filter.Active)
	}

	qb.OrderBy("name")
	qb.DefaultLimit(filter.Limit, 100)
	qb.Offset(filter.Offset)

	query, args := qb.Build()

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("listing organizations: %w", err)
	}
	defer rows.Close()

	var orgs []*models.Organization
	for rows.Next() {
		org, err := r.scanOrganizationRow(rows)
		if err != nil {
			return nil, err
		}
		orgs = append(orgs, org)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating organization rows: %w", err)
	}

	return orgs, nil
}

// scanOrganizationRow scans a single row from a rows result set
func (r *OrganizationRepo) scanOrganizationRow(rows *sql.Rows) (*models.Organization, error) {
	var org models.Organization
	var settingsJSON, metadataJSON []byte
	var licenseKey, logoURL, contactEmail sql.NullString
	var licenseExpiresAt sql.NullTime
	var maxUsers, maxConcurrentPods, maxStorageGB sql.NullInt32

	err := rows.Scan(
		&org.ID,
		&org.Name,
		&org.Slug,
		&org.Type,
		&org.Edition,
		&licenseKey,
		&licenseExpiresAt,
		&settingsJSON,
		&maxUsers,
		&maxConcurrentPods,
		&maxStorageGB,
		&logoURL,
		&contactEmail,
		&org.IsActive,
		&org.CreatedAt,
		&org.UpdatedAt,
		&metadataJSON,
	)
	if err != nil {
		return nil, fmt.Errorf("scanning organization row: %w", err)
	}

	// Handle nullable fields
	if licenseKey.Valid {
		org.LicenseKey = licenseKey.String
	}
	if licenseExpiresAt.Valid {
		org.LicenseExpiresAt = &licenseExpiresAt.Time
	}
	if maxUsers.Valid {
		v := int(maxUsers.Int32)
		org.MaxUsers = &v
	}
	if maxConcurrentPods.Valid {
		v := int(maxConcurrentPods.Int32)
		org.MaxConcurrentPods = &v
	}
	if maxStorageGB.Valid {
		v := int(maxStorageGB.Int32)
		org.MaxStorageGB = &v
	}
	if logoURL.Valid {
		org.LogoURL = logoURL.String
	}
	if contactEmail.Valid {
		org.ContactEmail = contactEmail.String
	}

	// Unmarshal JSON fields
	if len(settingsJSON) > 0 {
		if err := json.Unmarshal(settingsJSON, &org.Settings); err != nil {
			return nil, fmt.Errorf("unmarshaling settings: %w", err)
		}
	}
	if len(metadataJSON) > 0 {
		org.Metadata = metadataJSON
	}

	return &org, nil
}

// Update updates an existing organization
func (r *OrganizationRepo) Update(ctx context.Context, org *models.Organization) error {
	settingsJSON, err := json.Marshal(org.Settings)
	if err != nil {
		return fmt.Errorf("marshaling settings: %w", err)
	}

	metadataJSON := []byte("{}")
	if org.Metadata != nil {
		metadataJSON = org.Metadata
	}

	query := `
		UPDATE organizations
		SET name = $2, slug = $3, type = $4, edition = $5, license_key = $6,
		    license_expires_at = $7, settings = $8, max_users = $9,
		    max_concurrent_pods = $10, max_storage_gb = $11, logo_url = $12,
		    contact_email = $13, is_active = $14, metadata = $15, updated_at = NOW()
		WHERE id = $1`

	res, err := r.db.ExecContext(ctx, query,
		org.ID,
		org.Name,
		org.Slug,
		org.Type,
		org.Edition,
		nullStringOrg(org.LicenseKey),
		nullTimePtr(org.LicenseExpiresAt),
		settingsJSON,
		nullIntPtr(org.MaxUsers),
		nullIntPtr(org.MaxConcurrentPods),
		nullIntPtr(org.MaxStorageGB),
		nullStringOrg(org.LogoURL),
		nullStringOrg(org.ContactEmail),
		org.IsActive,
		metadataJSON,
	)

	if err != nil {
		return fmt.Errorf("updating organization: %w", err)
	}

	rows, _ := res.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("organization not found: %s", org.ID)
	}

	return nil
}

// Delete deletes an organization by ID
func (r *OrganizationRepo) Delete(ctx context.Context, id string) error {
	query := `DELETE FROM organizations WHERE id = $1`

	res, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("deleting organization: %w", err)
	}

	rows, _ := res.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("organization not found: %s", id)
	}

	return nil
}

// SetActive sets the active status of an organization
func (r *OrganizationRepo) SetActive(ctx context.Context, id string, active bool) error {
	query := `UPDATE organizations SET is_active = $2, updated_at = NOW() WHERE id = $1`

	res, err := r.db.ExecContext(ctx, query, id, active)
	if err != nil {
		return fmt.Errorf("setting organization active status: %w", err)
	}

	rows, _ := res.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("organization not found: %s", id)
	}

	return nil
}

// GetMemberCount returns the number of members in an organization
func (r *OrganizationRepo) GetMemberCount(ctx context.Context, id string) (int, error) {
	query := `SELECT COUNT(*) FROM organization_memberships WHERE organization_id = $1`

	var count int
	err := r.db.QueryRowContext(ctx, query, id).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("counting organization members: %w", err)
	}

	return count, nil
}

// GetTeamCount returns the number of teams in an organization
func (r *OrganizationRepo) GetTeamCount(ctx context.Context, id string) (int, error) {
	query := `SELECT COUNT(*) FROM teams WHERE organization_id = $1`

	var count int
	err := r.db.QueryRowContext(ctx, query, id).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("counting organization teams: %w", err)
	}

	return count, nil
}

// GetByIDs returns organizations matching the given IDs
func (r *OrganizationRepo) GetByIDs(ctx context.Context, ids []string) ([]*models.Organization, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	query := `SELECT id, name, slug, description, logo_url, website, is_active,
		max_members, license_type, license_expires_at, settings,
		created_at, updated_at
		FROM organizations WHERE id = ANY($1)`

	rows, err := r.db.QueryContext(ctx, query, pq.Array(ids))
	if err != nil {
		return nil, fmt.Errorf("querying organizations by IDs: %w", err)
	}
	defer rows.Close()

	var orgs []*models.Organization
	for rows.Next() {
		org, err := r.scanOrganizationRow(rows)
		if err != nil {
			return nil, err
		}
		orgs = append(orgs, org)
	}
	return orgs, rows.Err()
}

// GetMemberCountBatch returns member counts for multiple organizations
func (r *OrganizationRepo) GetMemberCountBatch(ctx context.Context, ids []string) (map[string]int, error) {
	if len(ids) == 0 {
		return map[string]int{}, nil
	}
	query := `SELECT organization_id, COUNT(*) FROM organization_memberships
		WHERE organization_id = ANY($1) GROUP BY organization_id`

	rows, err := r.db.QueryContext(ctx, query, pq.Array(ids))
	if err != nil {
		return nil, fmt.Errorf("batch counting members: %w", err)
	}
	defer rows.Close()

	counts := make(map[string]int, len(ids))
	for rows.Next() {
		var orgID string
		var count int
		if err := rows.Scan(&orgID, &count); err != nil {
			return nil, fmt.Errorf("scanning member count: %w", err)
		}
		counts[orgID] = count
	}
	return counts, rows.Err()
}

// GetTeamCountBatch returns team counts for multiple organizations
func (r *OrganizationRepo) GetTeamCountBatch(ctx context.Context, ids []string) (map[string]int, error) {
	if len(ids) == 0 {
		return map[string]int{}, nil
	}
	query := `SELECT organization_id, COUNT(*) FROM teams
		WHERE organization_id = ANY($1) GROUP BY organization_id`

	rows, err := r.db.QueryContext(ctx, query, pq.Array(ids))
	if err != nil {
		return nil, fmt.Errorf("batch counting teams: %w", err)
	}
	defer rows.Close()

	counts := make(map[string]int, len(ids))
	for rows.Next() {
		var orgID string
		var count int
		if err := rows.Scan(&orgID, &count); err != nil {
			return nil, fmt.Errorf("scanning team count: %w", err)
		}
		counts[orgID] = count
	}
	return counts, rows.Err()
}

// Helper functions for nullable values (org-specific to avoid conflicts)

func nullStringOrg(s string) sql.NullString {
	if s == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: s, Valid: true}
}

func nullIntPtr(i *int) sql.NullInt32 {
	if i == nil {
		return sql.NullInt32{}
	}
	// Bounds check to prevent integer overflow when converting int to int32.
	// This is used for organization limits (MaxUsers, MaxConcurrentPods, MaxStorageGB)
	// which should never exceed int32 max in practice.
	val := *i
	if val > math.MaxInt32 {
		val = math.MaxInt32
	} else if val < math.MinInt32 {
		val = math.MinInt32
	}
	return sql.NullInt32{Int32: int32(val), Valid: true} // #nosec G115 -- bounds checked above
}

func nullTimePtr(t *time.Time) sql.NullTime {
	if t == nil {
		return sql.NullTime{}
	}
	return sql.NullTime{Time: *t, Valid: true}
}
