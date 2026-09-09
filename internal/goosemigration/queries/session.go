package queries

import (
	"context"
	"database/sql"
	"time"

	"github.com/shurco/mycart/internal/store/db/postgres"
	"github.com/shurco/mycart/internal/store/db/sqlite"
)

// GetSession retrieves the session value for a given key if it hasn't expired using sqlc.
func (q *SettingQueries) GetSession(ctx context.Context, key string) (string, error) {
	queries := getSQLCQueries()

	var result interface{}
	var err error

	if DBType() == "postgres" {
		pgQueries := queries.(*postgres.Queries)
		result, err = pgQueries.GetSession(ctx, key)
	} else {
		sqliteQueries := queries.(*sqlite.Queries)
		result, err = sqliteQueries.GetSession(ctx, key)
	}

	if err != nil {
		return "", err
	}

	// Check if expired and extract value
	expires := time.Now().Unix()
	switch v := result.(type) {
	case sqlite.Session:
		if v.Expires.Valid && v.Expires.Int64 < expires {
			return "", sql.ErrNoRows
		}
		if v.Value.Valid {
			return v.Value.String, nil
		}
	case postgres.Session:
		if v.Expires.Valid && int64(v.Expires.Int32) < expires {
			return "", sql.ErrNoRows
		}
		if v.Value.Valid {
			return v.Value.String, nil
		}
	}

	return "", sql.ErrNoRows
}

// AddSession upserts a session record using direct SQL for UPSERT support.
func (q *SettingQueries) AddSession(ctx context.Context, key, value string, expires int64) error {
	if DBType() == "postgres" {
		_, err := q.DB.ExecContext(ctx,
			`INSERT INTO session (key, value, expires) VALUES ($1, $2, $3)
			 ON CONFLICT (key) DO UPDATE SET value = $2, expires = $3`,
			key, value, expires)
		return err
	}

	// SQLite: INSERT OR REPLACE
	_, err := q.DB.ExecContext(ctx,
		`INSERT OR REPLACE INTO session (key, value, expires) VALUES (?, ?, ?)`,
		key, value, expires)
	return err
}

// UpdateSession updates the session with new value and expiration using sqlc.
func (q *SettingQueries) UpdateSession(ctx context.Context, key, value string, expires int64) error {
	queries := getSQLCQueries()

	if DBType() == "postgres" {
		pgQueries := queries.(*postgres.Queries)
		params := postgres.UpdateSessionParams{
			Value:   sql.NullString{String: value, Valid: true},
			Expires: sql.NullInt32{Int32: int32(expires), Valid: true},
			Key:     key,
		}
		return pgQueries.UpdateSession(ctx, params)
	}

	sqliteQueries := queries.(*sqlite.Queries)
	params := sqlite.UpdateSessionParams{
		Value:   sql.NullString{String: value, Valid: true},
		Expires: sql.NullInt64{Int64: expires, Valid: true},
		Key:     key,
	}
	return sqliteQueries.UpdateSession(ctx, params)
}

// DeleteSession removes a session from the database using sqlc.
func (q *SettingQueries) DeleteSession(ctx context.Context, key string) error {
	queries := getSQLCQueries()

	if DBType() == "postgres" {
		pgQueries := queries.(*postgres.Queries)
		return pgQueries.DeleteSession(ctx, key)
	}

	sqliteQueries := queries.(*sqlite.Queries)
	return sqliteQueries.DeleteSession(ctx, key)
}
