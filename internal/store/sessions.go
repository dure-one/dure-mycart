package store

import (
	"context"

	"github.com/shurco/mycart/internal/store/db"
)

// AddSession upserts a session record using the function pointer.
func AddSession(ctx context.Context, key, value string, expires int64) error {
	return db.UpsertSessionFunc(ctx, db.UpsertSessionParams{
		Key:     key,
		Value:   value,
		Expires: expires,
	})
}

// GetSession retrieves a session by key.
func GetSession(ctx context.Context, key string) (db.Session, error) {
	return db.GetSessionFunc(ctx, key)
}

// DeleteSession removes a session from the database using the function pointer.
func DeleteSession(ctx context.Context, key string) error {
	return db.DeleteSessionFunc(ctx, key)
}
