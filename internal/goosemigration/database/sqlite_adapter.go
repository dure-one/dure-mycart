package database

import (
	"context"
	"database/sql"

	"github.com/shurco/mycart/internal/store/db/sqlite"
	_ "modernc.org/sqlite"
)

// SQLiteAdapter implements the Database interface for SQLite.
type SQLiteAdapter struct {
	db      *sql.DB
	queries *sqlite.Queries
}

// NewSQLiteAdapter creates a new SQLite adapter.
func NewSQLiteAdapter(db *sql.DB) *SQLiteAdapter {
	return &SQLiteAdapter{
		db:      db,
		queries: sqlite.New(db),
	}
}

// DB returns the underlying *sql.DB connection.
func (a *SQLiteAdapter) DB() *sql.DB {
	return a.db
}

// Close closes the database connection.
func (a *SQLiteAdapter) Close() error {
	return a.db.Close()
}

// BeginTx starts a database transaction.
func (a *SQLiteAdapter) BeginTx(ctx context.Context, opts *sql.TxOptions) (*sql.Tx, error) {
	return a.db.BeginTx(ctx, opts)
}

// Queries returns the sqlite.Queries object for database operations.
func (a *SQLiteAdapter) Queries() interface{} {
	return a.queries
}

// Ping verifies the connection to the database is still alive.
func (a *SQLiteAdapter) Ping(ctx context.Context) error {
	return a.db.PingContext(ctx)
}

// Stats returns database statistics.
func (a *SQLiteAdapter) Stats() sql.DBStats {
	return a.db.Stats()
}

// Type returns the database type.
func (a *SQLiteAdapter) Type() string {
	return "sqlite"
}
