package repositories

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/toddbartholow/kootenai/api/internal/models"
)

// PasswordResetTokenRepo implements PasswordResetTokenRepository
type PasswordResetTokenRepo struct {
	db DBTX
}

// NewPasswordResetTokenRepo creates a new password reset token repository
func NewPasswordResetTokenRepo(db DBTX) *PasswordResetTokenRepo {
	return &PasswordResetTokenRepo{db: db}
}

// Create stores a new password reset token
func (r *PasswordResetTokenRepo) Create(ctx context.Context, token *models.PasswordResetToken) error {
	query := `
		INSERT INTO password_reset_tokens (id, user_id, token_hash, expires_at, created_at)
		VALUES ($1, $2, $3, $4, $5)
	`

	_, err := r.db.ExecContext(ctx, query,
		token.ID,
		token.UserID,
		token.TokenHash,
		token.ExpiresAt,
		token.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("creating password reset token: %w", err)
	}

	return nil
}

// GetByTokenHash retrieves a token by its hash (for validation)
func (r *PasswordResetTokenRepo) GetByTokenHash(ctx context.Context, tokenHash string) (*models.PasswordResetToken, error) {
	query := `
		SELECT id, user_id, token_hash, expires_at, used_at, created_at
		FROM password_reset_tokens
		WHERE token_hash = $1
	`

	var token models.PasswordResetToken
	err := r.db.QueryRowContext(ctx, query, tokenHash).Scan(
		&token.ID,
		&token.UserID,
		&token.TokenHash,
		&token.ExpiresAt,
		&token.UsedAt,
		&token.CreatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("querying password reset token: %w", err)
	}

	return &token, nil
}

// MarkUsed marks a token as used
func (r *PasswordResetTokenRepo) MarkUsed(ctx context.Context, id string) error {
	query := `
		UPDATE password_reset_tokens
		SET used_at = NOW()
		WHERE id = $1
	`

	result, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("marking token used: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("checking rows affected: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("token not found: %s", id)
	}

	return nil
}

// DeleteByUserID deletes all tokens for a user (when they successfully reset)
func (r *PasswordResetTokenRepo) DeleteByUserID(ctx context.Context, userID string) error {
	query := `
		DELETE FROM password_reset_tokens
		WHERE user_id = $1
	`

	_, err := r.db.ExecContext(ctx, query, userID)
	if err != nil {
		return fmt.Errorf("deleting tokens for user: %w", err)
	}

	return nil
}

// DeleteExpired removes all expired tokens (for cleanup)
func (r *PasswordResetTokenRepo) DeleteExpired(ctx context.Context) (int64, error) {
	query := `
		DELETE FROM password_reset_tokens
		WHERE expires_at < $1
	`

	result, err := r.db.ExecContext(ctx, query, time.Now())
	if err != nil {
		return 0, fmt.Errorf("deleting expired tokens: %w", err)
	}

	return result.RowsAffected()
}
