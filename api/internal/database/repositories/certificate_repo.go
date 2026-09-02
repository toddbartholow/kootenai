package repositories

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/toddbartholow/kootenai/api/internal/models"
)

// CertificateRepository defines operations for pathway completion certificates
type CertificateRepository interface {
	// Create inserts a new certificate
	Create(ctx context.Context, cert *models.Certificate) error

	// GetByID retrieves a certificate by ID
	GetByID(ctx context.Context, id string) (*models.Certificate, error)

	// GetByEnrollmentID retrieves a certificate by enrollment ID
	GetByEnrollmentID(ctx context.Context, enrollmentID string) (*models.Certificate, error)

	// GetByVerificationCode retrieves a certificate by verification code
	GetByVerificationCode(ctx context.Context, code string) (*models.Certificate, error)

	// GetWithDetailsByVerificationCode retrieves a certificate with user/pathway details
	GetWithDetailsByVerificationCode(ctx context.Context, code string) (*models.CertificateWithDetails, error)

	// ListByUserID retrieves all certificates for a user
	ListByUserID(ctx context.Context, userID string) ([]*models.Certificate, error)

	// ListWithDetailsByUserID retrieves all certificates with details for a user
	ListWithDetailsByUserID(ctx context.Context, userID string) ([]*models.CertificateWithDetails, error)

	// Revoke marks a certificate as revoked
	Revoke(ctx context.Context, id string, reason string) error
}

// CertificateRepo implements CertificateRepository
type CertificateRepo struct {
	db DBTX
}

// NewCertificateRepo creates a new certificate repository
func NewCertificateRepo(db DBTX) *CertificateRepo {
	return &CertificateRepo{db: db}
}

// Create inserts a new certificate
func (r *CertificateRepo) Create(ctx context.Context, cert *models.Certificate) error {
	query := `
		INSERT INTO certificates (
			id, enrollment_id, user_id, pathway_id,
			verification_code, issued_at, expires_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING created_at`

	err := r.db.QueryRowContext(ctx, query,
		cert.ID,
		cert.EnrollmentID,
		cert.UserID,
		cert.PathwayID,
		cert.VerificationCode,
		cert.IssuedAt,
		cert.ExpiresAt,
	).Scan(&cert.CreatedAt)

	if err != nil {
		return fmt.Errorf("inserting certificate: %w", err)
	}

	return nil
}

// GetByID retrieves a certificate by ID
func (r *CertificateRepo) GetByID(ctx context.Context, id string) (*models.Certificate, error) {
	query := `
		SELECT id, enrollment_id, user_id, pathway_id,
		       verification_code, issued_at, expires_at,
		       revoked_at, revocation_reason, created_at
		FROM certificates
		WHERE id = $1`

	cert, err := r.scanCertificate(r.db.QueryRowContext(ctx, query, id))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("getting certificate by ID: %w", err)
	}

	return cert, nil
}

// GetByEnrollmentID retrieves a certificate by enrollment ID
func (r *CertificateRepo) GetByEnrollmentID(ctx context.Context, enrollmentID string) (*models.Certificate, error) {
	query := `
		SELECT id, enrollment_id, user_id, pathway_id,
		       verification_code, issued_at, expires_at,
		       revoked_at, revocation_reason, created_at
		FROM certificates
		WHERE enrollment_id = $1`

	cert, err := r.scanCertificate(r.db.QueryRowContext(ctx, query, enrollmentID))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("getting certificate by enrollment ID: %w", err)
	}

	return cert, nil
}

// GetByVerificationCode retrieves a certificate by verification code
func (r *CertificateRepo) GetByVerificationCode(ctx context.Context, code string) (*models.Certificate, error) {
	query := `
		SELECT id, enrollment_id, user_id, pathway_id,
		       verification_code, issued_at, expires_at,
		       revoked_at, revocation_reason, created_at
		FROM certificates
		WHERE verification_code = $1`

	cert, err := r.scanCertificate(r.db.QueryRowContext(ctx, query, code))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("getting certificate by verification code: %w", err)
	}

	return cert, nil
}

// GetWithDetailsByVerificationCode retrieves a certificate with user/pathway details
func (r *CertificateRepo) GetWithDetailsByVerificationCode(ctx context.Context, code string) (*models.CertificateWithDetails, error) {
	query := `
		SELECT c.id, c.enrollment_id, c.user_id, c.pathway_id,
		       c.verification_code, c.issued_at, c.expires_at,
		       c.revoked_at, c.revocation_reason, c.created_at,
		       p.name AS pathway_name,
		       COALESCE(u.display_name, u.username) AS user_name,
		       u.email AS user_email,
		       e.earned_points, e.max_points, e.percentage, e.completed_at
		FROM certificates c
		JOIN pathways p ON c.pathway_id = p.id
		JOIN users u ON c.user_id = u.id
		JOIN pathway_enrollments e ON c.enrollment_id = e.id
		WHERE c.verification_code = $1`

	cert, err := r.scanCertificateWithDetails(r.db.QueryRowContext(ctx, query, code))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("getting certificate with details: %w", err)
	}

	return cert, nil
}

// ListByUserID retrieves all certificates for a user
func (r *CertificateRepo) ListByUserID(ctx context.Context, userID string) ([]*models.Certificate, error) {
	query := `
		SELECT id, enrollment_id, user_id, pathway_id,
		       verification_code, issued_at, expires_at,
		       revoked_at, revocation_reason, created_at
		FROM certificates
		WHERE user_id = $1
		ORDER BY issued_at DESC`

	rows, err := r.db.QueryContext(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("listing certificates: %w", err)
	}
	defer rows.Close()

	var certs []*models.Certificate
	for rows.Next() {
		cert, err := r.scanCertificateRow(rows)
		if err != nil {
			return nil, fmt.Errorf("scanning certificate: %w", err)
		}
		certs = append(certs, cert)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating certificates: %w", err)
	}

	return certs, nil
}

// ListWithDetailsByUserID retrieves all certificates with details for a user
func (r *CertificateRepo) ListWithDetailsByUserID(ctx context.Context, userID string) ([]*models.CertificateWithDetails, error) {
	query := `
		SELECT c.id, c.enrollment_id, c.user_id, c.pathway_id,
		       c.verification_code, c.issued_at, c.expires_at,
		       c.revoked_at, c.revocation_reason, c.created_at,
		       p.name AS pathway_name,
		       COALESCE(u.display_name, u.username) AS user_name,
		       u.email AS user_email,
		       e.earned_points, e.max_points, e.percentage, e.completed_at
		FROM certificates c
		JOIN pathways p ON c.pathway_id = p.id
		JOIN users u ON c.user_id = u.id
		JOIN pathway_enrollments e ON c.enrollment_id = e.id
		WHERE c.user_id = $1
		ORDER BY c.issued_at DESC`

	rows, err := r.db.QueryContext(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("listing certificates with details: %w", err)
	}
	defer rows.Close()

	var certs []*models.CertificateWithDetails
	for rows.Next() {
		cert, err := r.scanCertificateWithDetailsRow(rows)
		if err != nil {
			return nil, fmt.Errorf("scanning certificate with details: %w", err)
		}
		certs = append(certs, cert)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating certificates: %w", err)
	}

	return certs, nil
}

// Revoke marks a certificate as revoked
func (r *CertificateRepo) Revoke(ctx context.Context, id string, reason string) error {
	query := `
		UPDATE certificates
		SET revoked_at = $2, revocation_reason = $3
		WHERE id = $1 AND revoked_at IS NULL`

	result, err := r.db.ExecContext(ctx, query, id, time.Now(), reason)
	if err != nil {
		return fmt.Errorf("revoking certificate: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("checking rows affected: %w", err)
	}

	if rows == 0 {
		return errors.New("certificate not found or already revoked")
	}

	return nil
}

// scanCertificate scans a single certificate from a row
func (r *CertificateRepo) scanCertificate(row *sql.Row) (*models.Certificate, error) {
	var cert models.Certificate
	var expiresAt, revokedAt sql.NullTime
	var revocationReason sql.NullString

	err := row.Scan(
		&cert.ID,
		&cert.EnrollmentID,
		&cert.UserID,
		&cert.PathwayID,
		&cert.VerificationCode,
		&cert.IssuedAt,
		&expiresAt,
		&revokedAt,
		&revocationReason,
		&cert.CreatedAt,
	)
	if err != nil {
		return nil, err
	}

	if expiresAt.Valid {
		cert.ExpiresAt = &expiresAt.Time
	}
	if revokedAt.Valid {
		cert.RevokedAt = &revokedAt.Time
	}
	if revocationReason.Valid {
		cert.RevocationReason = revocationReason.String
	}

	return &cert, nil
}

// scanCertificateRow scans a certificate from a rows result
func (r *CertificateRepo) scanCertificateRow(rows *sql.Rows) (*models.Certificate, error) {
	var cert models.Certificate
	var expiresAt, revokedAt sql.NullTime
	var revocationReason sql.NullString

	err := rows.Scan(
		&cert.ID,
		&cert.EnrollmentID,
		&cert.UserID,
		&cert.PathwayID,
		&cert.VerificationCode,
		&cert.IssuedAt,
		&expiresAt,
		&revokedAt,
		&revocationReason,
		&cert.CreatedAt,
	)
	if err != nil {
		return nil, err
	}

	if expiresAt.Valid {
		cert.ExpiresAt = &expiresAt.Time
	}
	if revokedAt.Valid {
		cert.RevokedAt = &revokedAt.Time
	}
	if revocationReason.Valid {
		cert.RevocationReason = revocationReason.String
	}

	return &cert, nil
}

// scanCertificateWithDetails scans a certificate with details from a row
func (r *CertificateRepo) scanCertificateWithDetails(row *sql.Row) (*models.CertificateWithDetails, error) {
	var cert models.CertificateWithDetails
	var expiresAt, revokedAt, completedAt sql.NullTime
	var revocationReason, userEmail sql.NullString

	err := row.Scan(
		&cert.ID,
		&cert.EnrollmentID,
		&cert.UserID,
		&cert.PathwayID,
		&cert.VerificationCode,
		&cert.IssuedAt,
		&expiresAt,
		&revokedAt,
		&revocationReason,
		&cert.CreatedAt,
		&cert.PathwayName,
		&cert.UserName,
		&userEmail,
		&cert.EarnedPoints,
		&cert.MaxPoints,
		&cert.Percentage,
		&completedAt,
	)
	if err != nil {
		return nil, err
	}

	if expiresAt.Valid {
		cert.ExpiresAt = &expiresAt.Time
	}
	if revokedAt.Valid {
		cert.RevokedAt = &revokedAt.Time
	}
	if revocationReason.Valid {
		cert.RevocationReason = revocationReason.String
	}
	if userEmail.Valid {
		cert.UserEmail = userEmail.String
	}
	if completedAt.Valid {
		cert.CompletedAt = &completedAt.Time
	}

	return &cert, nil
}

// scanCertificateWithDetailsRow scans a certificate with details from rows
func (r *CertificateRepo) scanCertificateWithDetailsRow(rows *sql.Rows) (*models.CertificateWithDetails, error) {
	var cert models.CertificateWithDetails
	var expiresAt, revokedAt, completedAt sql.NullTime
	var revocationReason, userEmail sql.NullString

	err := rows.Scan(
		&cert.ID,
		&cert.EnrollmentID,
		&cert.UserID,
		&cert.PathwayID,
		&cert.VerificationCode,
		&cert.IssuedAt,
		&expiresAt,
		&revokedAt,
		&revocationReason,
		&cert.CreatedAt,
		&cert.PathwayName,
		&cert.UserName,
		&userEmail,
		&cert.EarnedPoints,
		&cert.MaxPoints,
		&cert.Percentage,
		&completedAt,
	)
	if err != nil {
		return nil, err
	}

	if expiresAt.Valid {
		cert.ExpiresAt = &expiresAt.Time
	}
	if revokedAt.Valid {
		cert.RevokedAt = &revokedAt.Time
	}
	if revocationReason.Valid {
		cert.RevocationReason = revocationReason.String
	}
	if userEmail.Valid {
		cert.UserEmail = userEmail.String
	}
	if completedAt.Valid {
		cert.CompletedAt = &completedAt.Time
	}

	return &cert, nil
}
