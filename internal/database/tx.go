package database

import (
	"context"
	"database/sql"
)

// Tx is a *sql.Tx that speaks the configured dialect.
//
// Like Conn it deliberately does not embed *sql.Tx: embedding would leave the
// unbound Exec, Query and QueryRow reachable through the embedded field and
// silently send `?` placeholders to PostgreSQL. Commit and Rollback are
// declared explicitly because they have no dialect dimension.
type Tx struct {
	raw *sql.Tx
	d   Dialect
}

func (t *Tx) QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	return t.raw.QueryContext(ctx, t.d.Rebind(query), args...)
}

func (t *Tx) QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row {
	return t.raw.QueryRowContext(ctx, t.d.Rebind(query), args...)
}

func (t *Tx) ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error) {
	return t.raw.ExecContext(ctx, t.d.Rebind(query), args...)
}

func (t *Tx) PrepareContext(ctx context.Context, query string) (*sql.Stmt, error) {
	return t.raw.PrepareContext(ctx, t.d.Rebind(query))
}

func (t *Tx) Commit() error { return t.raw.Commit() }

func (t *Tx) Rollback() error { return t.raw.Rollback() }
