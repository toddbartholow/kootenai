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

// LicenseRepo implements LicenseRepository
type LicenseRepo struct {
	db DBTX
}

// NewLicenseRepo creates a new license repository
func NewLicenseRepo(db DBTX) *LicenseRepo {
	return &LicenseRepo{db: db}
}

// Create creates a new license
func (r *LicenseRepo) Create(ctx context.Context, license *models.License) error {
	query := `
		INSERT INTO licenses (
			id, organization_id, license_key, edition, issued_at, expires_at,
			max_users, max_pods, max_storage_gb, features, is_active,
			validation_status, metadata, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)`

	now := time.Now()
	if license.CreatedAt.IsZero() {
		license.CreatedAt = now
	}
	license.UpdatedAt = now

	if license.IssuedAt.IsZero() {
		license.IssuedAt = now
	}

	metadata := license.Metadata
	if metadata == nil {
		metadata = []byte("{}")
	}

	_, err := r.db.ExecContext(ctx, query,
		license.ID,
		license.OrganizationID,
		license.LicenseKey,
		license.Edition,
		license.IssuedAt,
		license.ExpiresAt,
		license.MaxUsers,
		license.MaxPods,
		license.MaxStorageGB,
		pq.Array(license.Features),
		license.IsActive,
		license.ValidationStatus,
		metadata,
		license.CreatedAt,
		license.UpdatedAt,
	)

	if err != nil {
		return fmt.Errorf("creating license: %w", err)
	}

	return nil
}

// GetByID retrieves a license by ID
func (r *LicenseRepo) GetByID(ctx context.Context, id string) (*models.License, error) {
	query := `
		SELECT id, organization_id, license_key, edition, issued_at, expires_at,
			max_users, max_pods, max_storage_gb, features, is_active,
			validation_status, last_validated_at, validation_error,
			created_at, updated_at, metadata
		FROM licenses
		WHERE id = $1`

	return r.scanLicense(ctx, query, id)
}

// GetByOrganization retrieves the most recent license for an organization
func (r *LicenseRepo) GetByOrganization(ctx context.Context, orgID string) (*models.License, error) {
	query := `
		SELECT id, organization_id, license_key, edition, issued_at, expires_at,
			max_users, max_pods, max_storage_gb, features, is_active,
			validation_status, last_validated_at, validation_error,
			created_at, updated_at, metadata
		FROM licenses
		WHERE organization_id = $1
		ORDER BY created_at DESC
		LIMIT 1`

	return r.scanLicense(ctx, query, orgID)
}

// GetByLicenseKey retrieves a license by its key
func (r *LicenseRepo) GetByLicenseKey(ctx context.Context, key string) (*models.License, error) {
	query := `
		SELECT id, organization_id, license_key, edition, issued_at, expires_at,
			max_users, max_pods, max_storage_gb, features, is_active,
			validation_status, last_validated_at, validation_error,
			created_at, updated_at, metadata
		FROM licenses
		WHERE license_key = $1`

	return r.scanLicense(ctx, query, key)
}

// GetActiveByOrganization retrieves the active license for an organization
func (r *LicenseRepo) GetActiveByOrganization(ctx context.Context, orgID string) (*models.License, error) {
	query := `
		SELECT id, organization_id, license_key, edition, issued_at, expires_at,
			max_users, max_pods, max_storage_gb, features, is_active,
			validation_status, last_validated_at, validation_error,
			created_at, updated_at, metadata
		FROM licenses
		WHERE organization_id = $1
			AND is_active = true
			AND validation_status = 'valid'
			AND (expires_at IS NULL OR expires_at > NOW())
		ORDER BY created_at DESC
		LIMIT 1`

	return r.scanLicense(ctx, query, orgID)
}

// Update updates a license
func (r *LicenseRepo) Update(ctx context.Context, license *models.License) error {
	query := `
		UPDATE licenses SET
			edition = $2,
			expires_at = $3,
			max_users = $4,
			max_pods = $5,
			max_storage_gb = $6,
			features = $7,
			is_active = $8,
			validation_status = $9,
			last_validated_at = $10,
			validation_error = $11,
			metadata = $12,
			updated_at = $13
		WHERE id = $1`

	license.UpdatedAt = time.Now()

	metadata := license.Metadata
	if metadata == nil {
		metadata = []byte("{}")
	}

	result, err := r.db.ExecContext(ctx, query,
		license.ID,
		license.Edition,
		license.ExpiresAt,
		license.MaxUsers,
		license.MaxPods,
		license.MaxStorageGB,
		pq.Array(license.Features),
		license.IsActive,
		license.ValidationStatus,
		license.LastValidatedAt,
		license.ValidationError,
		metadata,
		license.UpdatedAt,
	)

	if err != nil {
		return fmt.Errorf("updating license: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("getting rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return ErrLicenseNotFound
	}

	return nil
}

// UpdateValidationStatus updates the validation status of a license
func (r *LicenseRepo) UpdateValidationStatus(ctx context.Context, id string, status models.LicenseStatus, errorMsg *string) error {
	query := `
		UPDATE licenses SET
			validation_status = $2,
			last_validated_at = $3,
			validation_error = $4,
			updated_at = $5
		WHERE id = $1`

	now := time.Now()

	result, err := r.db.ExecContext(ctx, query, id, status, now, errorMsg, now)
	if err != nil {
		return fmt.Errorf("updating validation status: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("getting rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return ErrLicenseNotFound
	}

	return nil
}

// Deactivate deactivates a license
func (r *LicenseRepo) Deactivate(ctx context.Context, id string) error {
	query := `
		UPDATE licenses SET
			is_active = false,
			updated_at = $2
		WHERE id = $1`

	result, err := r.db.ExecContext(ctx, query, id, time.Now())
	if err != nil {
		return fmt.Errorf("deactivating license: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("getting rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return ErrLicenseNotFound
	}

	return nil
}

// scanLicense scans a single license from a query
func (r *LicenseRepo) scanLicense(ctx context.Context, query string, arg any) (*models.License, error) {
	var license models.License
	var features pq.StringArray
	var expiresAt, lastValidatedAt sql.NullTime
	var validationError sql.NullString
	var maxUsers, maxPods, maxStorageGB sql.NullInt64
	var metadata []byte

	err := r.db.QueryRowContext(ctx, query, arg).Scan(
		&license.ID,
		&license.OrganizationID,
		&license.LicenseKey,
		&license.Edition,
		&license.IssuedAt,
		&expiresAt,
		&maxUsers,
		&maxPods,
		&maxStorageGB,
		&features,
		&license.IsActive,
		&license.ValidationStatus,
		&lastValidatedAt,
		&validationError,
		&license.CreatedAt,
		&license.UpdatedAt,
		&metadata,
	)

	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("scanning license: %w", err)
	}

	if expiresAt.Valid {
		license.ExpiresAt = &expiresAt.Time
	}
	if lastValidatedAt.Valid {
		license.LastValidatedAt = &lastValidatedAt.Time
	}
	if validationError.Valid {
		license.ValidationError = &validationError.String
	}
	if maxUsers.Valid {
		v := int(maxUsers.Int64)
		license.MaxUsers = &v
	}
	if maxPods.Valid {
		v := int(maxPods.Int64)
		license.MaxPods = &v
	}
	if maxStorageGB.Valid {
		v := int(maxStorageGB.Int64)
		license.MaxStorageGB = &v
	}

	license.Features = features
	license.Metadata = metadata

	return &license, nil
}
