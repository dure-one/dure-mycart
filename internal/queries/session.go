package queries

import (
	"context"
	"time"
)

// GetSession retrieves the session value for a given key if it hasn't expired.
// It takes a context and key as arguments and returns the session value and an error if any.
func (q *SettingQueries) GetSession(ctx context.Context, key string) (string, error) {
	var value string
	expires := time.Now().Unix()
	err := q.DB.QueryRowContext(ctx, `SELECT value FROM session WHERE key = ? AND expires > ?`, key, expires).Scan(&value)
	if err != nil {
		return "", err
	}
	return value, nil
}

// AddSession upserts a session record by key. The ON CONFLICT clause makes
// callers idempotent: they can write to the same key repeatedly (e.g. to
// refresh a TTL-cached value) without first deleting the previous row.
//
// Expired rows are swept opportunistically on every write so the table does
// not grow without bound. The sweep cutoff is a Go parameter rather than an
// engine-specific expression for "now".
func (q *SettingQueries) AddSession(ctx context.Context, key, value string, expires int64) error {
	now := time.Now().Unix()
	if _, err := q.DB.ExecContext(ctx,
		`DELETE FROM session WHERE expires < ? AND key != ?`, now, key); err != nil {
		return err
	}

	_, err := q.DB.ExecContext(ctx,
		`INSERT INTO session (key, value, expires) VALUES (?, ?, ?) ON CONFLICT (key) DO UPDATE SET value = excluded.value, expires = excluded.expires`,
		key, value, expires)
	return err
}

// UpdateSession updates the session with a new value and expiration time for a given key.
// It takes a context, a session key, the new value to be set, and the new expiration time as arguments.
func (q *SettingQueries) UpdateSession(ctx context.Context, key, value string, expires int64) error {
	_, err := q.DB.ExecContext(ctx, `UPDATE session SET value = ?, expires = ? WHERE key = ? `, value, expires, key)
	return err
}

// DeleteSession removes a session from the database based on the provided key.
func (q *SettingQueries) DeleteSession(ctx context.Context, key string) error {
	_, err := q.DB.ExecContext(ctx, `DELETE FROM session WHERE key = ?`, key)
	return err
}
