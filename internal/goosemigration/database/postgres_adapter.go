package database

import (
	"context"
	"database/sql"

	"github.com/shurco/mycart/internal/store/db/postgres"
	_ "github.com/lib/pq"
)

// PostgresAdapter implements the Database interface for PostgreSQL.
type PostgresAdapter struct {
	db      *sql.DB
	queries *postgres.Queries
}

// NewPostgresAdapter creates a new PostgreSQL adapter.
func NewPostgresAdapter(db *sql.DB) *PostgresAdapter {
	return &PostgresAdapter{
		db:      db,
		queries: postgres.New(db),
	}
}

// DB returns the underlying *sql.DB connection.
func (a *PostgresAdapter) DB() *sql.DB {
	return a.db
}

// Close closes the database connection.
func (a *PostgresAdapter) Close() error {
	return a.db.Close()
}

// BeginTx starts a database transaction.
func (a *PostgresAdapter) BeginTx(ctx context.Context, opts *sql.TxOptions) (*sql.Tx, error) {
	return a.db.BeginTx(ctx, opts)
}

// Queries returns the postgres.Queries object for database operations.
func (a *PostgresAdapter) Queries() interface{} {
	return a.queries
}

// Ping verifies the connection to the database is still alive.
func (a *PostgresAdapter) Ping(ctx context.Context) error {
	return a.db.PingContext(ctx)
}

// Stats returns database statistics.
func (a *PostgresAdapter) Stats() sql.DBStats {
	return a.db.Stats()
}

// Type returns the database type.
func (a *PostgresAdapter) Type() string {
	return "postgres"
}
