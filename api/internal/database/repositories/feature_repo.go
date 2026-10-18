package repositories

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/lib/pq"

	"github.com/toddbartholow/kootenai/api/internal/models"
)

// FeatureRepo implements FeatureRepository
type FeatureRepo struct {
	db DBTX
}

// NewFeatureRepo creates a new feature repository
func NewFeatureRepo(db DBTX) *FeatureRepo {
	return &FeatureRepo{db: db}
}

// GetByID retrieves a feature flag by ID
func (r *FeatureRepo) GetByID(ctx context.Context, id string) (*models.FeatureFlag, error) {
	query := `
		SELECT id, name, description, editions, is_global, default_settings, created_at
		FROM feature_flags
		WHERE id = $1`

	var feature models.FeatureFlag
	var description sql.NullString
	var defaultSettings []byte
	var editions pq.StringArray

	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&feature.ID,
		&feature.Name,
		&description,
		&editions,
		&feature.IsGlobal,
		&defaultSettings,
		&feature.CreatedAt,
	)

	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("getting feature flag: %w", err)
	}

	feature.Description = description.String
	feature.Editions = editionsFromStrings(editions)
	feature.DefaultSettings = defaultSettings

	return &feature, nil
}

// List retrieves all feature flags
func (r *FeatureRepo) List(ctx context.Context) ([]*models.FeatureFlag, error) {
	query := `
		SELECT id, name, description, editions, is_global, default_settings, created_at
		FROM feature_flags
		ORDER BY id`

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("listing feature flags: %w", err)
	}
	defer rows.Close()

	return r.scanMultiple(rows)
}

// ListByEdition retrieves feature flags available for a specific edition
func (r *FeatureRepo) ListByEdition(ctx context.Context, edition models.Edition) ([]*models.FeatureFlag, error) {
	query := `
		SELECT id, name, description, editions, is_global, default_settings, created_at
		FROM feature_flags
		WHERE is_global = true OR $1 = ANY(editions)
		ORDER BY id`

	rows, err := r.db.QueryContext(ctx, query, edition)
	if err != nil {
		return nil, fmt.Errorf("listing feature flags by edition: %w", err)
	}
	defer rows.Close()

	return r.scanMultiple(rows)
}

// IsFeatureEnabled checks if a feature is enabled for an organization
func (r *FeatureRepo) IsFeatureEnabled(ctx context.Context, orgID, featureID string) (bool, error) {
	// First check for an organization-specific override
	overrideQuery := `
		SELECT enabled, expires_at
		FROM organization_features
		WHERE organization_id = $1 AND feature_id = $2`

	var enabled bool
	var expiresAt sql.NullTime

	err := r.db.QueryRowContext(ctx, overrideQuery, orgID, featureID).Scan(&enabled, &expiresAt)
	if err == nil {
		// Override exists - check if it's expired
		if expiresAt.Valid && time.Now().After(expiresAt.Time) {
			// Override has expired, fall through to check base edition
		} else {
			return enabled, nil
		}
	} else if !errors.Is(err, sql.ErrNoRows) {
		return false, fmt.Errorf("checking feature override: %w", err)
	}

	// No override (or expired), check if feature is available for org's edition
	checkQuery := `
		SELECT CASE
			WHEN ff.is_global THEN true
			WHEN o.edition = ANY(ff.editions) THEN true
			ELSE false
		END as enabled
		FROM feature_flags ff
		CROSS JOIN organizations o
		WHERE ff.id = $1 AND o.id = $2`

	err = r.db.QueryRowContext(ctx, checkQuery, featureID, orgID).Scan(&enabled)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("checking feature enabled: %w", err)
	}

	return enabled, nil
}

// GetOrganizationFeatures returns all features and their enabled status for an organization
func (r *FeatureRepo) GetOrganizationFeatures(ctx context.Context, orgID string) (map[string]bool, error) {
	// Get org's edition first
	var edition string
	err := r.db.QueryRowContext(ctx, `SELECT edition FROM organizations WHERE id = $1`, orgID).Scan(&edition)
	if err != nil {
		return nil, fmt.Errorf("getting organization edition: %w", err)
	}

	// Get all features with their enabled status for this org
	query := `
		SELECT
			ff.id,
			CASE
				WHEN of.enabled IS NOT NULL AND (of.expires_at IS NULL OR of.expires_at > NOW()) THEN of.enabled
				WHEN ff.is_global THEN true
				WHEN $2 = ANY(ff.editions) THEN true
				ELSE false
			END as enabled
		FROM feature_flags ff
		LEFT JOIN organization_features of ON ff.id = of.feature_id AND of.organization_id = $1
		ORDER BY ff.id`

	rows, err := r.db.QueryContext(ctx, query, orgID, edition)
	if err != nil {
		return nil, fmt.Errorf("getting organization features: %w", err)
	}
	defer rows.Close()

	features := make(map[string]bool)
	for rows.Next() {
		var featureID string
		var enabled bool
		if err := rows.Scan(&featureID, &enabled); err != nil {
			return nil, fmt.Errorf("scanning feature: %w", err)
		}
		features[featureID] = enabled
	}

	return features, rows.Err()
}

// SetOrganizationFeature sets or updates an organization-specific feature override
func (r *FeatureRepo) SetOrganizationFeature(ctx context.Context, feature *models.OrganizationFeature) error {
	query := `
		INSERT INTO organization_features (organization_id, feature_id, enabled, expires_at, settings, granted_by, granted_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (organization_id, feature_id)
		DO UPDATE SET enabled = $3, expires_at = $4, settings = $5, granted_by = $6, granted_at = $7`

	settings := feature.Settings
	if settings == nil {
		settings = []byte("{}")
	}

	_, err := r.db.ExecContext(ctx, query,
		feature.OrganizationID,
		feature.FeatureID,
		feature.Enabled,
		feature.ExpiresAt,
		settings,
		feature.GrantedBy,
		feature.GrantedAt,
	)

	if err != nil {
		return fmt.Errorf("setting organization feature: %w", err)
	}

	return nil
}

// RemoveOrganizationFeature removes an organization-specific feature override
func (r *FeatureRepo) RemoveOrganizationFeature(ctx context.Context, orgID, featureID string) error {
	query := `DELETE FROM organization_features WHERE organization_id = $1 AND feature_id = $2`

	_, err := r.db.ExecContext(ctx, query, orgID, featureID)
	if err != nil {
		return fmt.Errorf("removing organization feature: %w", err)
	}

	return nil
}

// Helper methods

func (r *FeatureRepo) scanMultiple(rows *sql.Rows) ([]*models.FeatureFlag, error) {
	var features []*models.FeatureFlag
	for rows.Next() {
		var feature models.FeatureFlag
		var description sql.NullString
		var defaultSettings []byte
		var editions pq.StringArray

		err := rows.Scan(
			&feature.ID,
			&feature.Name,
			&description,
			&editions,
			&feature.IsGlobal,
			&defaultSettings,
			&feature.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scanning feature flag: %w", err)
		}

		feature.Description = description.String
		feature.Editions = editionsFromStrings(editions)
		feature.DefaultSettings = defaultSettings

		features = append(features, &feature)
	}

	return features, rows.Err()
}

// editionsFromStrings converts a string array to Edition slice
func editionsFromStrings(strs []string) []models.Edition {
	editions := make([]models.Edition, len(strs))
	for i, s := range strs {
		editions[i] = models.Edition(s)
	}
	return editions
}
