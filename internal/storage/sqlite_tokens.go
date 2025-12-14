package storage

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/good-yellow-bee/blazelog/internal/models"
)

// sqliteTokenRepo implements TokenRepository using SQLite.
type sqliteTokenRepo struct {
	db *sql.DB
}

// Create inserts a new refresh token.
func (r *sqliteTokenRepo) Create(ctx context.Context, token *models.RefreshToken) error {
	if token.ID == "" {
		token.ID = uuid.New().String()
	}

	query := `
		INSERT INTO refresh_tokens (id, user_id, token_hash, expires_at, created_at, revoked)
		VALUES (?, ?, ?, ?, ?, ?)
	`

	_, err := r.db.ExecContext(ctx, query,
		token.ID,
		token.UserID,
		token.TokenHash,
		token.ExpiresAt,
		token.CreatedAt,
		boolToInt(token.Revoked),
	)
	if err != nil {
		return fmt.Errorf("insert refresh token: %w", err)
	}

	return nil
}

// GetByTokenHash retrieves a refresh token by its hash.
func (r *sqliteTokenRepo) GetByTokenHash(ctx context.Context, tokenHash string) (*models.RefreshToken, error) {
	query := `
		SELECT id, user_id, token_hash, expires_at, created_at, revoked, revoked_at
		FROM refresh_tokens
		WHERE token_hash = ?
	`

	var token models.RefreshToken
	var revokedAt sql.NullTime
	var revoked int

	err := r.db.QueryRowContext(ctx, query, tokenHash).Scan(
		&token.ID,
		&token.UserID,
		&token.TokenHash,
		&token.ExpiresAt,
		&token.CreatedAt,
		&revoked,
		&revokedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("query refresh token: %w", err)
	}

	token.Revoked = revoked != 0
	if revokedAt.Valid {
		token.RevokedAt = &revokedAt.Time
	}

	return &token, nil
}

// Revoke marks a token as revoked.
func (r *sqliteTokenRepo) Revoke(ctx context.Context, id string) error {
	query := `
		UPDATE refresh_tokens
		SET revoked = 1, revoked_at = ?
		WHERE id = ?
	`

	result, err := r.db.ExecContext(ctx, query, time.Now(), id)
	if err != nil {
		return fmt.Errorf("revoke token: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("token not found")
	}

	return nil
}

// RevokeByTokenHash revokes a token by its hash.
func (r *sqliteTokenRepo) RevokeByTokenHash(ctx context.Context, tokenHash string) error {
	query := `
		UPDATE refresh_tokens
		SET revoked = 1, revoked_at = ?
		WHERE token_hash = ?
	`

	_, err := r.db.ExecContext(ctx, query, time.Now(), tokenHash)
	if err != nil {
		return fmt.Errorf("revoke token by hash: %w", err)
	}

	return nil
}

// RevokeAllForUser revokes all tokens for a user.
func (r *sqliteTokenRepo) RevokeAllForUser(ctx context.Context, userID string) error {
	query := `
		UPDATE refresh_tokens
		SET revoked = 1, revoked_at = ?
		WHERE user_id = ? AND revoked = 0
	`

	_, err := r.db.ExecContext(ctx, query, time.Now(), userID)
	if err != nil {
		return fmt.Errorf("revoke all tokens for user: %w", err)
	}

	return nil
}

// DeleteExpired removes expired tokens from the database.
func (r *sqliteTokenRepo) DeleteExpired(ctx context.Context) (int64, error) {
	query := `
		DELETE FROM refresh_tokens
		WHERE expires_at < ?
	`

	result, err := r.db.ExecContext(ctx, query, time.Now())
	if err != nil {
		return 0, fmt.Errorf("delete expired tokens: %w", err)
	}

	return result.RowsAffected()
}

