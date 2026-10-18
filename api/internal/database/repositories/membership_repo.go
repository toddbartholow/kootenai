package repositories

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/toddbartholow/kootenai/api/internal/models"
)

// OrganizationMembershipRepo implements OrganizationMembershipRepository
type OrganizationMembershipRepo struct {
	db DBTX
}

// NewOrganizationMembershipRepo creates a new organization membership repository
func NewOrganizationMembershipRepo(db DBTX) *OrganizationMembershipRepo {
	return &OrganizationMembershipRepo{db: db}
}

// Create inserts a new organization membership
func (r *OrganizationMembershipRepo) Create(ctx context.Context, membership *models.OrganizationMembership) error {
	if membership.ID == "" {
		membership.ID = uuid.New().String()
	}

	query := `
		INSERT INTO organization_memberships (
			id, organization_id, user_id, role, is_primary,
			invited_by, invitation_token, invited_at, accepted_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING created_at, updated_at`

	err := r.db.QueryRowContext(ctx, query,
		membership.ID,
		membership.OrganizationID,
		membership.UserID,
		membership.Role,
		membership.IsPrimary,
		nullStringOrg(stringFromPtr(membership.InvitedBy)),
		nullStringOrg(stringFromPtr(membership.InvitationToken)),
		membership.InvitedAt,
		nullTimePtr(membership.AcceptedAt),
	).Scan(&membership.CreatedAt, &membership.UpdatedAt)

	if err != nil {
		return fmt.Errorf("inserting organization membership: %w", err)
	}

	return nil
}

// GetByID retrieves a membership by ID
func (r *OrganizationMembershipRepo) GetByID(ctx context.Context, id string) (*models.OrganizationMembership, error) {
	query := `
		SELECT id, organization_id, user_id, role, is_primary,
		       invited_by, invitation_token, invited_at, accepted_at, created_at, updated_at
		FROM organization_memberships
		WHERE id = $1`

	return r.scanMembership(ctx, query, id)
}

// GetByOrgAndUser retrieves a membership by organization and user IDs
func (r *OrganizationMembershipRepo) GetByOrgAndUser(ctx context.Context, orgID, userID string) (*models.OrganizationMembership, error) {
	query := `
		SELECT id, organization_id, user_id, role, is_primary,
		       invited_by, invitation_token, invited_at, accepted_at, created_at, updated_at
		FROM organization_memberships
		WHERE organization_id = $1 AND user_id = $2`

	var membership models.OrganizationMembership
	var invitedBy, invitationToken sql.NullString
	var acceptedAt sql.NullTime

	err := r.db.QueryRowContext(ctx, query, orgID, userID).Scan(
		&membership.ID,
		&membership.OrganizationID,
		&membership.UserID,
		&membership.Role,
		&membership.IsPrimary,
		&invitedBy,
		&invitationToken,
		&membership.InvitedAt,
		&acceptedAt,
		&membership.CreatedAt,
		&membership.UpdatedAt,
	)

	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("querying organization membership: %w", err)
	}

	if invitedBy.Valid {
		membership.InvitedBy = &invitedBy.String
	}
	if invitationToken.Valid {
		membership.InvitationToken = &invitationToken.String
	}
	if acceptedAt.Valid {
		membership.AcceptedAt = &acceptedAt.Time
	}

	return &membership, nil
}

// scanMembership is a helper to scan a single membership row
func (r *OrganizationMembershipRepo) scanMembership(ctx context.Context, query string, arg any) (*models.OrganizationMembership, error) {
	var membership models.OrganizationMembership
	var invitedBy, invitationToken sql.NullString
	var acceptedAt sql.NullTime

	err := r.db.QueryRowContext(ctx, query, arg).Scan(
		&membership.ID,
		&membership.OrganizationID,
		&membership.UserID,
		&membership.Role,
		&membership.IsPrimary,
		&invitedBy,
		&invitationToken,
		&membership.InvitedAt,
		&acceptedAt,
		&membership.CreatedAt,
		&membership.UpdatedAt,
	)

	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("querying organization membership: %w", err)
	}

	if invitedBy.Valid {
		membership.InvitedBy = &invitedBy.String
	}
	if invitationToken.Valid {
		membership.InvitationToken = &invitationToken.String
	}
	if acceptedAt.Valid {
		membership.AcceptedAt = &acceptedAt.Time
	}

	return &membership, nil
}

// ListByOrganization retrieves all memberships for an organization
func (r *OrganizationMembershipRepo) ListByOrganization(ctx context.Context, orgID string, filter MembershipFilter) ([]*models.OrganizationMembership, error) {
	qb := NewQueryBuilder(`
		SELECT id, organization_id, user_id, role, is_primary,
		       invited_by, invitation_token, invited_at, accepted_at, created_at, updated_at
		FROM organization_memberships
		WHERE 1=1`)

	qb.AddCondition("organization_id = $%d", orgID)

	if filter.Role != "" {
		qb.AddCondition("role = $%d", filter.Role)
	}

	if !filter.IncludePending {
		qb.AddRawCondition("accepted_at IS NOT NULL")
	}

	qb.OrderByRaw("role, created_at")
	qb.DefaultLimit(filter.Limit, 100)
	qb.Offset(filter.Offset)

	query, args := qb.Build()

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("listing organization memberships: %w", err)
	}
	defer rows.Close()

	return r.scanMembershipRows(rows)
}

// ListByUser retrieves all organization memberships for a user
func (r *OrganizationMembershipRepo) ListByUser(ctx context.Context, userID string) ([]*models.OrganizationMembership, error) {
	query := `
		SELECT id, organization_id, user_id, role, is_primary,
		       invited_by, invitation_token, invited_at, accepted_at, created_at, updated_at
		FROM organization_memberships
		WHERE user_id = $1 AND accepted_at IS NOT NULL
		ORDER BY is_primary DESC, created_at`

	rows, err := r.db.QueryContext(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("listing user memberships: %w", err)
	}
	defer rows.Close()

	return r.scanMembershipRows(rows)
}

// scanMembershipRows scans multiple membership rows
func (r *OrganizationMembershipRepo) scanMembershipRows(rows *sql.Rows) ([]*models.OrganizationMembership, error) {
	var memberships []*models.OrganizationMembership

	for rows.Next() {
		var membership models.OrganizationMembership
		var invitedBy, invitationToken sql.NullString
		var acceptedAt sql.NullTime

		err := rows.Scan(
			&membership.ID,
			&membership.OrganizationID,
			&membership.UserID,
			&membership.Role,
			&membership.IsPrimary,
			&invitedBy,
			&invitationToken,
			&membership.InvitedAt,
			&acceptedAt,
			&membership.CreatedAt,
			&membership.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scanning membership row: %w", err)
		}

		if invitedBy.Valid {
			membership.InvitedBy = &invitedBy.String
		}
		if invitationToken.Valid {
			membership.InvitationToken = &invitationToken.String
		}
		if acceptedAt.Valid {
			membership.AcceptedAt = &acceptedAt.Time
		}

		memberships = append(memberships, &membership)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating membership rows: %w", err)
	}

	return memberships, nil
}

// Update updates an existing membership
func (r *OrganizationMembershipRepo) Update(ctx context.Context, membership *models.OrganizationMembership) error {
	query := `
		UPDATE organization_memberships
		SET role = $2, is_primary = $3, updated_at = NOW()
		WHERE id = $1`

	res, err := r.db.ExecContext(ctx, query,
		membership.ID,
		membership.Role,
		membership.IsPrimary,
	)

	if err != nil {
		return fmt.Errorf("updating organization membership: %w", err)
	}

	rows, _ := res.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("membership not found: %s", membership.ID)
	}

	return nil
}

// UpdateRole updates a membership's role
func (r *OrganizationMembershipRepo) UpdateRole(ctx context.Context, id string, role models.OrgRole) error {
	query := `UPDATE organization_memberships SET role = $2, updated_at = NOW() WHERE id = $1`

	res, err := r.db.ExecContext(ctx, query, id, role)
	if err != nil {
		return fmt.Errorf("updating membership role: %w", err)
	}

	rows, _ := res.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("membership not found: %s", id)
	}

	return nil
}

// Delete removes a membership
func (r *OrganizationMembershipRepo) Delete(ctx context.Context, id string) error {
	query := `DELETE FROM organization_memberships WHERE id = $1`

	res, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("deleting organization membership: %w", err)
	}

	rows, _ := res.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("membership not found: %s", id)
	}

	return nil
}

// AcceptInvitation accepts a pending invitation by token
func (r *OrganizationMembershipRepo) AcceptInvitation(ctx context.Context, token string) (*models.OrganizationMembership, error) {
	now := time.Now()

	query := `
		UPDATE organization_memberships
		SET accepted_at = $2, invitation_token = NULL, updated_at = NOW()
		WHERE invitation_token = $1 AND accepted_at IS NULL
		RETURNING id, organization_id, user_id, role, is_primary,
		          invited_by, invitation_token, invited_at, accepted_at, created_at, updated_at`

	var membership models.OrganizationMembership
	var invitedBy, invitationToken sql.NullString
	var acceptedAt sql.NullTime

	err := r.db.QueryRowContext(ctx, query, token, now).Scan(
		&membership.ID,
		&membership.OrganizationID,
		&membership.UserID,
		&membership.Role,
		&membership.IsPrimary,
		&invitedBy,
		&invitationToken,
		&membership.InvitedAt,
		&acceptedAt,
		&membership.CreatedAt,
		&membership.UpdatedAt,
	)

	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrInvitationInvalid
	}
	if err != nil {
		return nil, fmt.Errorf("accepting invitation: %w", err)
	}

	if invitedBy.Valid {
		membership.InvitedBy = &invitedBy.String
	}
	if acceptedAt.Valid {
		membership.AcceptedAt = &acceptedAt.Time
	}

	return &membership, nil
}

// SetPrimary sets an organization as the user's primary organization
func (r *OrganizationMembershipRepo) SetPrimary(ctx context.Context, userID, orgID string) error {
	tx, err := beginTx(ctx, r.db, nil)
	if err != nil {
		return fmt.Errorf("beginning transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	// Clear existing primary
	_, err = tx.ExecContext(ctx,
		`UPDATE organization_memberships SET is_primary = false, updated_at = NOW() WHERE user_id = $1 AND is_primary = true`,
		userID,
	)
	if err != nil {
		return fmt.Errorf("clearing existing primary: %w", err)
	}

	// Set new primary
	res, err := tx.ExecContext(ctx,
		`UPDATE organization_memberships SET is_primary = true, updated_at = NOW() WHERE user_id = $1 AND organization_id = $2`,
		userID, orgID,
	)
	if err != nil {
		return fmt.Errorf("setting primary organization: %w", err)
	}

	rows, _ := res.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("membership not found for user %s in org %s", userID, orgID)
	}

	// Also update the user's default_organization_id
	_, err = tx.ExecContext(ctx,
		`UPDATE users SET default_organization_id = $2, updated_at = NOW() WHERE id = $1`,
		userID, orgID,
	)
	if err != nil {
		return fmt.Errorf("updating user default organization: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("committing transaction: %w", err)
	}

	return nil
}

// GetPrimaryOrganization returns the user's primary organization
func (r *OrganizationMembershipRepo) GetPrimaryOrganization(ctx context.Context, userID string) (*models.Organization, error) {
	query := `
		SELECT o.id, o.name, o.slug, o.type, o.edition, o.license_key, o.license_expires_at,
		       o.settings, o.max_users, o.max_concurrent_pods, o.max_storage_gb,
		       o.logo_url, o.contact_email, o.is_active, o.created_at, o.updated_at, o.metadata
		FROM organizations o
		JOIN organization_memberships m ON o.id = m.organization_id
		WHERE m.user_id = $1 AND m.is_primary = true AND m.accepted_at IS NOT NULL`

	orgRepo := &OrganizationRepo{db: r.db}
	return orgRepo.scanOrganization(ctx, query, userID)
}

// Helper to safely convert *string to string
func stringFromPtr(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
