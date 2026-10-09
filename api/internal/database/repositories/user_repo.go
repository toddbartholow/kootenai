package repositories

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"

	"github.com/toddbartholow/kootenai/api/internal/models"
)

// UserRepo implements UserRepository
type UserRepo struct {
	db DBTX
}

// NewUserRepo creates a new user repository
func NewUserRepo(db DBTX) *UserRepo {
	return &UserRepo{db: db}
}

// Create inserts a new user
func (r *UserRepo) Create(ctx context.Context, user *models.User) error {
	metadataJSON, err := json.Marshal(user.Metadata)
	if err != nil {
		return fmt.Errorf("marshaling metadata: %w", err)
	}

	query := `
		INSERT INTO users (id, external_id, username, email, display_name, role, is_active, metadata)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING created_at, updated_at`

	err = r.db.QueryRowContext(ctx, query,
		user.ID,
		user.ExternalID,
		user.Username,
		user.Email,
		user.DisplayName,
		user.Role,
		user.IsActive,
		metadataJSON,
	).Scan(&user.CreatedAt, &user.UpdatedAt)

	if err != nil {
		return fmt.Errorf("inserting user: %w", err)
	}

	return nil
}

// GetByID retrieves a user by ID
func (r *UserRepo) GetByID(ctx context.Context, id string) (*models.User, error) {
	query := `
		SELECT id, external_id, username, email, display_name, role, is_active,
		       created_at, updated_at, last_login_at, metadata
		FROM users
		WHERE id = $1`

	return r.scanUser(ctx, query, id)
}

// GetByIDs retrieves multiple users by their IDs in a single query
// This avoids N+1 queries when loading multiple users
func (r *UserRepo) GetByIDs(ctx context.Context, ids []string) ([]*models.User, error) {
	if len(ids) == 0 {
		return []*models.User{}, nil
	}

	query := `
		SELECT id, external_id, username, email, display_name, role, is_active,
		       created_at, updated_at, last_login_at, metadata
		FROM users
		WHERE id = ANY($1)`

	rows, err := r.db.QueryContext(ctx, query, pq.Array(ids))
	if err != nil {
		return nil, fmt.Errorf("querying users by ids: %w", err)
	}
	defer rows.Close()

	var users []*models.User
	for rows.Next() {
		var user models.User
		var metadataJSON []byte
		var lastLoginAt sql.NullTime
		var externalID, email, displayName sql.NullString

		err := rows.Scan(
			&user.ID,
			&externalID,
			&user.Username,
			&email,
			&displayName,
			&user.Role,
			&user.IsActive,
			&user.CreatedAt,
			&user.UpdatedAt,
			&lastLoginAt,
			&metadataJSON,
		)
		if err != nil {
			return nil, fmt.Errorf("scanning user: %w", err)
		}

		if externalID.Valid {
			user.ExternalID = externalID.String
		}
		if email.Valid {
			user.Email = email.String
		}
		if displayName.Valid {
			user.DisplayName = displayName.String
		}
		if lastLoginAt.Valid {
			user.LastLoginAt = &lastLoginAt.Time
		}
		if len(metadataJSON) > 0 {
			if err := json.Unmarshal(metadataJSON, &user.Metadata); err != nil {
				return nil, fmt.Errorf("unmarshaling metadata: %w", err)
			}
		}

		users = append(users, &user)
	}

	return users, rows.Err()
}

// GetByExternalID retrieves a user by external ID (FreeIPA uid)
func (r *UserRepo) GetByExternalID(ctx context.Context, externalID string) (*models.User, error) {
	query := `
		SELECT id, external_id, username, email, display_name, role, is_active,
		       created_at, updated_at, last_login_at, metadata
		FROM users
		WHERE external_id = $1`

	return r.scanUser(ctx, query, externalID)
}

// GetByUsername retrieves a user by username
func (r *UserRepo) GetByUsername(ctx context.Context, username string) (*models.User, error) {
	query := `
		SELECT id, external_id, username, email, display_name, role, is_active,
		       created_at, updated_at, last_login_at, metadata
		FROM users
		WHERE username = $1`

	return r.scanUser(ctx, query, username)
}

// scanUser is a helper to scan a single user row
func (r *UserRepo) scanUser(ctx context.Context, query string, arg any) (*models.User, error) {
	var user models.User
	var metadataJSON []byte
	var lastLoginAt sql.NullTime

	err := r.db.QueryRowContext(ctx, query, arg).Scan(
		&user.ID,
		&user.ExternalID,
		&user.Username,
		&user.Email,
		&user.DisplayName,
		&user.Role,
		&user.IsActive,
		&user.CreatedAt,
		&user.UpdatedAt,
		&lastLoginAt,
		&metadataJSON,
	)

	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("querying user: %w", err)
	}

	if lastLoginAt.Valid {
		user.LastLoginAt = &lastLoginAt.Time
	}

	if len(metadataJSON) > 0 {
		if err := json.Unmarshal(metadataJSON, &user.Metadata); err != nil {
			return nil, fmt.Errorf("unmarshaling metadata: %w", err)
		}
	}

	return &user, nil
}

// Update updates an existing user
func (r *UserRepo) Update(ctx context.Context, user *models.User) error {
	metadataJSON, err := json.Marshal(user.Metadata)
	if err != nil {
		return fmt.Errorf("marshaling metadata: %w", err)
	}

	query := `
		UPDATE users
		SET external_id = $2, username = $3, email = $4, display_name = $5,
		    role = $6, is_active = $7, metadata = $8, updated_at = NOW()
		WHERE id = $1`

	res, err := r.db.ExecContext(ctx, query,
		user.ID,
		user.ExternalID,
		user.Username,
		user.Email,
		user.DisplayName,
		user.Role,
		user.IsActive,
		metadataJSON,
	)

	if err != nil {
		return fmt.Errorf("updating user: %w", err)
	}

	rows, _ := res.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("user not found: %s", user.ID)
	}

	return nil
}

// GetOrCreateByUsername retrieves a user by username or creates a new one.
// Uses INSERT ... ON CONFLICT to avoid TOCTOU race conditions.
func (r *UserRepo) GetOrCreateByUsername(ctx context.Context, username string) (*models.User, error) {
	// First try a quick lookup
	user, err := r.GetByUsername(ctx, username)
	if err != nil {
		return nil, err
	}
	if user != nil {
		return user, nil
	}

	// Also try to find by email if username looks like an email
	if strings.Contains(username, "@") {
		user, err = r.GetByEmail(ctx, username)
		if err != nil {
			return nil, err
		}
		if user != nil {
			return user, nil
		}
	}

	// Atomically insert or get existing user to avoid race conditions
	externalID := fmt.Sprintf("local-%s", uuid.New().String()[:8])
	newID := uuid.New().String()

	query := `
		INSERT INTO users (id, external_id, username, email, display_name, role, is_active)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (username) DO NOTHING
		RETURNING id, external_id, username, email, display_name, role, is_active, created_at, updated_at`

	var created models.User
	err = r.db.QueryRowContext(ctx, query,
		newID, externalID, username, username, username, "student", true,
	).Scan(
		&created.ID, &created.ExternalID, &created.Username,
		&created.Email, &created.DisplayName, &created.Role,
		&created.IsActive, &created.CreatedAt, &created.UpdatedAt,
	)
	if err == nil {
		return &created, nil
	}

	// ON CONFLICT DO NOTHING returns no rows — another goroutine created it first
	// Fetch the existing user
	user, err = r.GetByUsername(ctx, username)
	if err != nil {
		return nil, fmt.Errorf("fetching existing user after conflict: %w", err)
	}
	if user != nil {
		return user, nil
	}

	return nil, fmt.Errorf("failed to get or create user: %s", username)
}

// UpdateLastLogin updates the user's last login timestamp
func (r *UserRepo) UpdateLastLogin(ctx context.Context, id string) error {
	query := `UPDATE users SET last_login_at = $2, updated_at = NOW() WHERE id = $1`

	res, err := r.db.ExecContext(ctx, query, id, time.Now())
	if err != nil {
		return fmt.Errorf("updating last login: %w", err)
	}

	rows, _ := res.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("user not found: %s", id)
	}

	return nil
}

// UserListOptions contains filtering options for listing users
type UserListOptions struct {
	Role     string
	IsActive *bool
	Search   string
	Limit    int
	Offset   int
}

// List retrieves users with optional filtering
func (r *UserRepo) List(ctx context.Context, opts UserListOptions) ([]*models.User, int, error) {
	// applyFilters centralizes the filter logic so the COUNT and the paged
	// SELECT share the same WHERE without duplicating the conditional chain.
	// Both queries produce identical argument lists.
	applyFilters := func(qb *QueryBuilder) {
		if opts.Role != "" {
			qb.AddCondition("role = $%d", opts.Role)
		}
		if opts.IsActive != nil {
			qb.AddCondition("is_active = $%d", *opts.IsActive)
		}
		if opts.Search != "" {
			like := "%" + opts.Search + "%"
			// Three separate placeholders pointing at the same value — the
			// previous hand-built query reused a single placeholder, which
			// QueryBuilder doesn't support without breaking its 1:1 arg/
			// placeholder invariant. Passing the value three times is
			// equivalent semantically.
			qb.AddConditionMulti(
				"(username ILIKE $%d OR email ILIKE $%d OR display_name ILIKE $%d)",
				like, like, like,
			)
		}
	}

	// Count query — no ORDER/LIMIT/OFFSET.
	countQB := NewQueryBuilder("SELECT COUNT(*) FROM users WHERE 1=1")
	applyFilters(countQB)
	countQuery, countArgs := countQB.Build()

	var total int
	if err := r.db.QueryRowContext(ctx, countQuery, countArgs...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("counting users: %w", err)
	}

	// Data query — same filters + pagination.
	qb := NewQueryBuilder(`
		SELECT id, external_id, username, email, display_name, role, is_active,
		       created_at, updated_at, last_login_at, metadata
		FROM users
		WHERE 1=1`)
	applyFilters(qb)
	qb.OrderByRaw("created_at DESC")
	qb.DefaultLimit(opts.Limit, 100)
	qb.Offset(opts.Offset)
	query, args := qb.Build()

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("querying users: %w", err)
	}
	defer rows.Close()

	var users []*models.User
	for rows.Next() {
		var user models.User
		var metadataJSON []byte
		var lastLoginAt sql.NullTime
		var externalID, email, displayName sql.NullString

		err := rows.Scan(
			&user.ID,
			&externalID,
			&user.Username,
			&email,
			&displayName,
			&user.Role,
			&user.IsActive,
			&user.CreatedAt,
			&user.UpdatedAt,
			&lastLoginAt,
			&metadataJSON,
		)
		if err != nil {
			return nil, 0, fmt.Errorf("scanning user: %w", err)
		}

		if externalID.Valid {
			user.ExternalID = externalID.String
		}
		if email.Valid {
			user.Email = email.String
		}
		if displayName.Valid {
			user.DisplayName = displayName.String
		}
		if lastLoginAt.Valid {
			user.LastLoginAt = &lastLoginAt.Time
		}
		if len(metadataJSON) > 0 {
			if err := json.Unmarshal(metadataJSON, &user.Metadata); err != nil {
				return nil, 0, fmt.Errorf("unmarshaling metadata: %w", err)
			}
		}

		users = append(users, &user)
	}

	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterating users: %w", err)
	}

	return users, total, nil
}

// Delete removes a user by ID
func (r *UserRepo) Delete(ctx context.Context, id string) error {
	query := `DELETE FROM users WHERE id = $1`

	res, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("deleting user: %w", err)
	}

	rows, _ := res.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("user not found: %s", id)
	}

	return nil
}

// GetByEmail retrieves a user by email
func (r *UserRepo) GetByEmail(ctx context.Context, email string) (*models.User, error) {
	query := `
		SELECT id, external_id, username, email, display_name, role, is_active,
		       created_at, updated_at, last_login_at, metadata
		FROM users
		WHERE email = $1`

	return r.scanUser(ctx, query, email)
}

// GetByEmailForAuth retrieves a user by email including password hash for authentication.
// Only returns active users.
func (r *UserRepo) GetByEmailForAuth(ctx context.Context, email string) (*models.User, error) {
	query := `
		SELECT id, external_id, username, email, display_name, role, is_active,
		       password_hash, must_change_password, password_updated_at,
		       created_at, updated_at, last_login_at, metadata
		FROM users
		WHERE email = $1 AND is_active = true`

	var user models.User
	var metadataJSON []byte
	var lastLoginAt, passwordUpdatedAt sql.NullTime
	var passwordHash sql.NullString

	err := r.db.QueryRowContext(ctx, query, email).Scan(
		&user.ID,
		&user.ExternalID,
		&user.Username,
		&user.Email,
		&user.DisplayName,
		&user.Role,
		&user.IsActive,
		&passwordHash,
		&user.MustChangePassword,
		&passwordUpdatedAt,
		&user.CreatedAt,
		&user.UpdatedAt,
		&lastLoginAt,
		&metadataJSON,
	)

	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("querying user for auth: %w", err)
	}

	if passwordHash.Valid {
		user.PasswordHash = passwordHash.String
	}
	if passwordUpdatedAt.Valid {
		user.PasswordUpdatedAt = &passwordUpdatedAt.Time
	}
	if lastLoginAt.Valid {
		user.LastLoginAt = &lastLoginAt.Time
	}
	if len(metadataJSON) > 0 {
		if err := json.Unmarshal(metadataJSON, &user.Metadata); err != nil {
			return nil, fmt.Errorf("unmarshaling metadata: %w", err)
		}
	}

	return &user, nil
}

// UpdatePassword updates a user's password hash and optionally sets the must_change_password flag
func (r *UserRepo) UpdatePassword(ctx context.Context, id, passwordHash string, mustChange bool) error {
	query := `
		UPDATE users
		SET password_hash = $2,
		    must_change_password = $3,
		    password_updated_at = NOW(),
		    updated_at = NOW()
		WHERE id = $1`

	res, err := r.db.ExecContext(ctx, query, id, passwordHash, mustChange)
	if err != nil {
		return fmt.Errorf("updating password: %w", err)
	}

	rows, _ := res.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("user not found: %s", id)
	}

	return nil
}

// ClearMustChangePassword clears the must_change_password flag after user changes their password
func (r *UserRepo) ClearMustChangePassword(ctx context.Context, id string) error {
	query := `UPDATE users SET must_change_password = false, updated_at = NOW() WHERE id = $1`

	res, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("clearing must_change_password: %w", err)
	}

	rows, _ := res.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("user not found: %s", id)
	}

	return nil
}

// GetPreferredLocale returns the user's stored preferred_locale, or nil if
// the user hasn't set one. Callers should fall back to Accept-Language when
// this returns nil.
func (r *UserRepo) GetPreferredLocale(ctx context.Context, id string) (*string, error) {
	var locale sql.NullString
	err := r.db.QueryRowContext(ctx, `SELECT preferred_locale FROM users WHERE id = $1`, id).Scan(&locale)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("querying preferred_locale: %w", err)
	}
	if !locale.Valid {
		return nil, nil
	}
	return &locale.String, nil
}

// UpdatePreferredLocale sets (or clears with nil) the user's preferred_locale.
// The column's CHECK constraint validates the BCP-47 format; app-level
// callers should additionally narrow to the set of catalogs actually shipped.
func (r *UserRepo) UpdatePreferredLocale(ctx context.Context, id string, locale *string) error {
	res, err := r.db.ExecContext(ctx,
		`UPDATE users SET preferred_locale = $2, updated_at = NOW() WHERE id = $1`,
		id, locale)
	if err != nil {
		return fmt.Errorf("updating preferred_locale: %w", err)
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("user not found: %s", id)
	}
	return nil
}

// GetByIDWithPassword retrieves a user by ID including password hash
func (r *UserRepo) GetByIDWithPassword(ctx context.Context, id string) (*models.User, error) {
	query := `
		SELECT id, external_id, username, email, display_name, role, is_active,
		       password_hash, must_change_password, password_updated_at,
		       created_at, updated_at, last_login_at, metadata
		FROM users
		WHERE id = $1`

	var user models.User
	var metadataJSON []byte
	var lastLoginAt, passwordUpdatedAt sql.NullTime
	var passwordHash sql.NullString

	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&user.ID,
		&user.ExternalID,
		&user.Username,
		&user.Email,
		&user.DisplayName,
		&user.Role,
		&user.IsActive,
		&passwordHash,
		&user.MustChangePassword,
		&passwordUpdatedAt,
		&user.CreatedAt,
		&user.UpdatedAt,
		&lastLoginAt,
		&metadataJSON,
	)

	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("querying user: %w", err)
	}

	if passwordHash.Valid {
		user.PasswordHash = passwordHash.String
	}
	if passwordUpdatedAt.Valid {
		user.PasswordUpdatedAt = &passwordUpdatedAt.Time
	}
	if lastLoginAt.Valid {
		user.LastLoginAt = &lastLoginAt.Time
	}
	if len(metadataJSON) > 0 {
		if err := json.Unmarshal(metadataJSON, &user.Metadata); err != nil {
			return nil, fmt.Errorf("unmarshaling metadata: %w", err)
		}
	}

	return &user, nil
}
