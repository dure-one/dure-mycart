package store

import (
	"context"

	"github.com/shurco/mycart/internal/store/db"
)

// AddSession upserts a session record with database-specific UPSERT syntax.
// Uses direct SQL because postgres and sqlite have different UPSERT syntax.
func AddSession(ctx context.Context, key, value string, expires int64) error {
	// Use direct SQL for UPSERT - different syntax for postgres vs sqlite
	if db.Type() == "postgres" {
		_, err := db.DB().ExecContext(ctx,
			`INSERT INTO session (key, value, expires) VALUES ($1, $2, $3)
			 ON CONFLICT (key) DO UPDATE SET value = $2, expires = $3`,
			key, value, expires)
		return err
	}

	// SQLite: INSERT OR REPLACE
	_, err := db.DB().ExecContext(ctx,
		`INSERT OR REPLACE INTO session (key, value, expires) VALUES (?, ?, ?)`,
		key, value, expires)
	return err
}

// GetSession retrieves a session by key.
func GetSession(ctx context.Context, key string) (db.Session, error) {
	return db.GetSessionFunc(ctx, key)
}

// DeleteSession removes a session from the database using the function pointer.
func DeleteSession(ctx context.Context, key string) error {
	return db.DeleteSessionFunc(ctx, key)
}
