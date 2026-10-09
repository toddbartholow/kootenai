package repositories

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/google/uuid"

	"github.com/toddbartholow/kootenai/api/internal/models"
)

// TeamRepo implements TeamRepository
type TeamRepo struct {
	db DBTX
}

// NewTeamRepo creates a new team repository
func NewTeamRepo(db DBTX) *TeamRepo {
	return &TeamRepo{db: db}
}

// Create inserts a new team
func (r *TeamRepo) Create(ctx context.Context, team *models.Team) error {
	if team.ID == "" {
		team.ID = uuid.New().String()
	}

	settingsJSON := []byte("{}")
	if team.Settings != nil {
		settingsJSON = team.Settings
	}

	query := `
		INSERT INTO teams (
			id, organization_id, name, slug, description,
			parent_team_id, canvas_section_id, settings, is_active
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING created_at, updated_at`

	err := r.db.QueryRowContext(ctx, query,
		team.ID,
		team.OrganizationID,
		team.Name,
		team.Slug,
		nullStringOrg(team.Description),
		nullStringOrg(stringFromPtr(team.ParentTeamID)),
		nullStringOrg(stringFromPtr(team.CanvasSectionID)),
		settingsJSON,
		team.IsActive,
	).Scan(&team.CreatedAt, &team.UpdatedAt)

	if err != nil {
		return fmt.Errorf("inserting team: %w", err)
	}

	return nil
}

// GetByID retrieves a team by ID
func (r *TeamRepo) GetByID(ctx context.Context, id string) (*models.Team, error) {
	query := `
		SELECT id, organization_id, name, slug, description,
		       parent_team_id, canvas_section_id, settings, is_active, created_at, updated_at
		FROM teams
		WHERE id = $1`

	return r.scanTeam(ctx, query, id)
}

// GetBySlug retrieves a team by organization ID and slug
func (r *TeamRepo) GetBySlug(ctx context.Context, orgID, slug string) (*models.Team, error) {
	query := `
		SELECT id, organization_id, name, slug, description,
		       parent_team_id, canvas_section_id, settings, is_active, created_at, updated_at
		FROM teams
		WHERE organization_id = $1 AND slug = $2`

	var team models.Team
	var description, parentTeamID, canvasSectionID sql.NullString
	var settingsJSON []byte

	err := r.db.QueryRowContext(ctx, query, orgID, slug).Scan(
		&team.ID,
		&team.OrganizationID,
		&team.Name,
		&team.Slug,
		&description,
		&parentTeamID,
		&canvasSectionID,
		&settingsJSON,
		&team.IsActive,
		&team.CreatedAt,
		&team.UpdatedAt,
	)

	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("querying team: %w", err)
	}

	if description.Valid {
		team.Description = description.String
	}
	if parentTeamID.Valid {
		team.ParentTeamID = &parentTeamID.String
	}
	if canvasSectionID.Valid {
		team.CanvasSectionID = &canvasSectionID.String
	}
	if len(settingsJSON) > 0 {
		team.Settings = settingsJSON
	}

	return &team, nil
}

// scanTeam is a helper to scan a single team row
func (r *TeamRepo) scanTeam(ctx context.Context, query string, arg any) (*models.Team, error) {
	var team models.Team
	var description, parentTeamID, canvasSectionID sql.NullString
	var settingsJSON []byte

	err := r.db.QueryRowContext(ctx, query, arg).Scan(
		&team.ID,
		&team.OrganizationID,
		&team.Name,
		&team.Slug,
		&description,
		&parentTeamID,
		&canvasSectionID,
		&settingsJSON,
		&team.IsActive,
		&team.CreatedAt,
		&team.UpdatedAt,
	)

	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("querying team: %w", err)
	}

	if description.Valid {
		team.Description = description.String
	}
	if parentTeamID.Valid {
		team.ParentTeamID = &parentTeamID.String
	}
	if canvasSectionID.Valid {
		team.CanvasSectionID = &canvasSectionID.String
	}
	if len(settingsJSON) > 0 {
		team.Settings = settingsJSON
	}

	return &team, nil
}

// ListByOrganization retrieves all teams for an organization
func (r *TeamRepo) ListByOrganization(ctx context.Context, orgID string, filter TeamFilter) ([]*models.Team, error) {
	qb := NewQueryBuilder(`
		SELECT id, organization_id, name, slug, description,
		       parent_team_id, canvas_section_id, settings, is_active, created_at, updated_at
		FROM teams
		WHERE 1=1`)

	qb.AddCondition("organization_id = $%d", orgID)

	if filter.ParentTeamID != nil {
		if *filter.ParentTeamID == "" {
			qb.AddRawCondition("parent_team_id IS NULL")
		} else {
			qb.AddCondition("parent_team_id = $%d", *filter.ParentTeamID)
		}
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
		return nil, fmt.Errorf("listing teams: %w", err)
	}
	defer rows.Close()

	return r.scanTeamRows(rows)
}

// scanTeamRows scans multiple team rows
func (r *TeamRepo) scanTeamRows(rows *sql.Rows) ([]*models.Team, error) {
	var teams []*models.Team

	for rows.Next() {
		var team models.Team
		var description, parentTeamID, canvasSectionID sql.NullString
		var settingsJSON []byte

		err := rows.Scan(
			&team.ID,
			&team.OrganizationID,
			&team.Name,
			&team.Slug,
			&description,
			&parentTeamID,
			&canvasSectionID,
			&settingsJSON,
			&team.IsActive,
			&team.CreatedAt,
			&team.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scanning team row: %w", err)
		}

		if description.Valid {
			team.Description = description.String
		}
		if parentTeamID.Valid {
			team.ParentTeamID = &parentTeamID.String
		}
		if canvasSectionID.Valid {
			team.CanvasSectionID = &canvasSectionID.String
		}
		if len(settingsJSON) > 0 {
			team.Settings = settingsJSON
		}

		teams = append(teams, &team)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating team rows: %w", err)
	}

	return teams, nil
}

// Update updates an existing team
func (r *TeamRepo) Update(ctx context.Context, team *models.Team) error {
	settingsJSON := []byte("{}")
	if team.Settings != nil {
		settingsJSON = team.Settings
	}

	query := `
		UPDATE teams
		SET name = $2, slug = $3, description = $4, parent_team_id = $5,
		    canvas_section_id = $6, settings = $7, is_active = $8, updated_at = NOW()
		WHERE id = $1`

	res, err := r.db.ExecContext(ctx, query,
		team.ID,
		team.Name,
		team.Slug,
		nullStringOrg(team.Description),
		nullStringOrg(stringFromPtr(team.ParentTeamID)),
		nullStringOrg(stringFromPtr(team.CanvasSectionID)),
		settingsJSON,
		team.IsActive,
	)

	if err != nil {
		return fmt.Errorf("updating team: %w", err)
	}

	rows, _ := res.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("team not found: %s", team.ID)
	}

	return nil
}

// Delete deletes a team by ID
func (r *TeamRepo) Delete(ctx context.Context, id string) error {
	query := `DELETE FROM teams WHERE id = $1`

	res, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("deleting team: %w", err)
	}

	rows, _ := res.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("team not found: %s", id)
	}

	return nil
}

// SetActive sets the active status of a team
func (r *TeamRepo) SetActive(ctx context.Context, id string, active bool) error {
	query := `UPDATE teams SET is_active = $2, updated_at = NOW() WHERE id = $1`

	res, err := r.db.ExecContext(ctx, query, id, active)
	if err != nil {
		return fmt.Errorf("setting team active status: %w", err)
	}

	rows, _ := res.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("team not found: %s", id)
	}

	return nil
}

// GetMemberCount returns the number of members in a team
func (r *TeamRepo) GetMemberCount(ctx context.Context, id string) (int, error) {
	query := `SELECT COUNT(*) FROM team_memberships WHERE team_id = $1`

	var count int
	err := r.db.QueryRowContext(ctx, query, id).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("counting team members: %w", err)
	}

	return count, nil
}

// TeamMembershipRepo implements TeamMembershipRepository
type TeamMembershipRepo struct {
	db DBTX
}

// NewTeamMembershipRepo creates a new team membership repository
func NewTeamMembershipRepo(db DBTX) *TeamMembershipRepo {
	return &TeamMembershipRepo{db: db}
}

// Create inserts a new team membership
func (r *TeamMembershipRepo) Create(ctx context.Context, membership *models.TeamMembership) error {
	if membership.ID == "" {
		membership.ID = uuid.New().String()
	}

	query := `
		INSERT INTO team_memberships (id, team_id, user_id, role)
		VALUES ($1, $2, $3, $4)
		RETURNING created_at`

	err := r.db.QueryRowContext(ctx, query,
		membership.ID,
		membership.TeamID,
		membership.UserID,
		membership.Role,
	).Scan(&membership.CreatedAt)

	if err != nil {
		return fmt.Errorf("inserting team membership: %w", err)
	}

	return nil
}

// GetByID retrieves a team membership by ID
func (r *TeamMembershipRepo) GetByID(ctx context.Context, id string) (*models.TeamMembership, error) {
	query := `
		SELECT id, team_id, user_id, role, created_at
		FROM team_memberships
		WHERE id = $1`

	var membership models.TeamMembership
	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&membership.ID,
		&membership.TeamID,
		&membership.UserID,
		&membership.Role,
		&membership.CreatedAt,
	)

	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("querying team membership: %w", err)
	}

	return &membership, nil
}

// GetByTeamAndUser retrieves a membership by team and user IDs
func (r *TeamMembershipRepo) GetByTeamAndUser(ctx context.Context, teamID, userID string) (*models.TeamMembership, error) {
	query := `
		SELECT id, team_id, user_id, role, created_at
		FROM team_memberships
		WHERE team_id = $1 AND user_id = $2`

	var membership models.TeamMembership
	err := r.db.QueryRowContext(ctx, query, teamID, userID).Scan(
		&membership.ID,
		&membership.TeamID,
		&membership.UserID,
		&membership.Role,
		&membership.CreatedAt,
	)

	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("querying team membership: %w", err)
	}

	return &membership, nil
}

// ListByTeam retrieves all memberships for a team
func (r *TeamMembershipRepo) ListByTeam(ctx context.Context, teamID string) ([]*models.TeamMembership, error) {
	query := `
		SELECT id, team_id, user_id, role, created_at
		FROM team_memberships
		WHERE team_id = $1
		ORDER BY role, created_at`

	rows, err := r.db.QueryContext(ctx, query, teamID)
	if err != nil {
		return nil, fmt.Errorf("listing team memberships: %w", err)
	}
	defer rows.Close()

	return r.scanTeamMembershipRows(rows)
}

// ListByUser retrieves all team memberships for a user
func (r *TeamMembershipRepo) ListByUser(ctx context.Context, userID string) ([]*models.TeamMembership, error) {
	query := `
		SELECT id, team_id, user_id, role, created_at
		FROM team_memberships
		WHERE user_id = $1
		ORDER BY created_at`

	rows, err := r.db.QueryContext(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("listing user team memberships: %w", err)
	}
	defer rows.Close()

	return r.scanTeamMembershipRows(rows)
}

// ListByTeamAndRole retrieves all memberships for a team with a specific role
func (r *TeamMembershipRepo) ListByTeamAndRole(ctx context.Context, teamID string, role models.TeamRole) ([]*models.TeamMembership, error) {
	query := `
		SELECT id, team_id, user_id, role, created_at
		FROM team_memberships
		WHERE team_id = $1 AND role = $2
		ORDER BY created_at`

	rows, err := r.db.QueryContext(ctx, query, teamID, role)
	if err != nil {
		return nil, fmt.Errorf("listing team memberships by role: %w", err)
	}
	defer rows.Close()

	return r.scanTeamMembershipRows(rows)
}

// scanTeamMembershipRows scans multiple team membership rows
func (r *TeamMembershipRepo) scanTeamMembershipRows(rows *sql.Rows) ([]*models.TeamMembership, error) {
	var memberships []*models.TeamMembership

	for rows.Next() {
		var membership models.TeamMembership
		err := rows.Scan(
			&membership.ID,
			&membership.TeamID,
			&membership.UserID,
			&membership.Role,
			&membership.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scanning team membership row: %w", err)
		}
		memberships = append(memberships, &membership)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating team membership rows: %w", err)
	}

	return memberships, nil
}

// Update updates an existing team membership
func (r *TeamMembershipRepo) Update(ctx context.Context, membership *models.TeamMembership) error {
	query := `UPDATE team_memberships SET role = $2 WHERE id = $1`

	res, err := r.db.ExecContext(ctx, query, membership.ID, membership.Role)
	if err != nil {
		return fmt.Errorf("updating team membership: %w", err)
	}

	rows, _ := res.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("team membership not found: %s", membership.ID)
	}

	return nil
}

// UpdateRole updates a team membership's role
func (r *TeamMembershipRepo) UpdateRole(ctx context.Context, id string, role models.TeamRole) error {
	query := `UPDATE team_memberships SET role = $2 WHERE id = $1`

	res, err := r.db.ExecContext(ctx, query, id, role)
	if err != nil {
		return fmt.Errorf("updating team membership role: %w", err)
	}

	rows, _ := res.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("team membership not found: %s", id)
	}

	return nil
}

// Delete removes a team membership
func (r *TeamMembershipRepo) Delete(ctx context.Context, id string) error {
	query := `DELETE FROM team_memberships WHERE id = $1`

	res, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("deleting team membership: %w", err)
	}

	rows, _ := res.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("team membership not found: %s", id)
	}

	return nil
}
