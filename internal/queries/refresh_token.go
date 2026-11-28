package queries

import (
	"context"
	"fmt"
	"time"
)

// AddRefreshToken stores a new refresh token.
func (q *SettingQueries) AddRefreshToken(ctx context.Context, id, userID, role string, expiresAt int64) error {
	now := time.Now().Unix()

	_, err := q.DB.ExecContext(ctx,
		`INSERT INTO refresh_tokens (id, user_id, role, expires_at, created_at) VALUES (?, ?, ?, ?, ?)`,
		id, userID, role, expiresAt, now)
	return err
}

// GetRefreshToken returns the user_id and role for a valid, non-expired refresh token.
// Returns an error if the token doesn't exist or is expired.
func (q *SettingQueries) GetRefreshToken(ctx context.Context, id string) (userID, role string, err error) {
	now := time.Now().Unix()

	err = q.DB.QueryRowContext(ctx,
		`SELECT user_id, role FROM refresh_tokens WHERE id = ? AND expires_at > ?`,
		id, now).Scan(&userID, &role)
	if err != nil {
		return "", "", fmt.Errorf("refresh token not found or expired: %w", err)
	}

	return userID, role, nil
}

// DeleteRefreshToken removes a refresh token by id. Idempotent.
func (q *SettingQueries) DeleteRefreshToken(ctx context.Context, id string) error {
	_, err := q.DB.ExecContext(ctx,
		`DELETE FROM refresh_tokens WHERE id = ?`, id)
	return err
}

// DeleteRefreshTokensByUser removes all refresh tokens for a given user_id and role.
// Called on sign-out to revoke all refresh tokens.
func (q *SettingQueries) DeleteRefreshTokensByUser(ctx context.Context, userID, role string) error {
	_, err := q.DB.ExecContext(ctx,
		`DELETE FROM refresh_tokens WHERE user_id = ? AND role = ?`,
		userID, role)
	return err
}
