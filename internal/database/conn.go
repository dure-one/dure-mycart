package database

import (
	"context"
	"database/sql"
	"time"
)

// Conn is a *sql.DB that speaks the configured dialect.
//
// It deliberately does not embed *sql.DB: embedding would let any caller reach
// the unbound methods through the embedded field and silently send `?`
// placeholders to PostgreSQL. Raw access exists, but only through the
// explicitly named Raw, which is for goose and for building test fixtures — it
// must never be used inside internal/queries.
type Conn struct {
	raw *sql.DB
	d   Dialect
}

// Wrap adapts an already opened *sql.DB to the given dialect. It exists for
// tests that build their own connection (in-memory SQLite, a per-test
// PostgreSQL schema).
func Wrap(raw *sql.DB, d Dialect) *Conn {
	return &Conn{raw: raw, d: d}
}

// Dialect returns the dialect this connection speaks.
func (c *Conn) Dialect() Dialect { return c.d }

// Raw exposes the underlying *sql.DB. Use it only where an unbound handle is
// genuinely required (goose migrations, test fixtures), never for application
// queries.
func (c *Conn) Raw() *sql.DB { return c.raw }

func (c *Conn) QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	return c.raw.QueryContext(ctx, c.d.Rebind(query), args...)
}

func (c *Conn) QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row {
	return c.raw.QueryRowContext(ctx, c.d.Rebind(query), args...)
}

func (c *Conn) ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error) {
	return c.raw.ExecContext(ctx, c.d.Rebind(query), args...)
}

func (c *Conn) PrepareContext(ctx context.Context, query string) (*sql.Stmt, error) {
	return c.raw.PrepareContext(ctx, c.d.Rebind(query))
}

func (c *Conn) BeginTx(ctx context.Context, opts *sql.TxOptions) (*Tx, error) {
	tx, err := c.raw.BeginTx(ctx, opts)
	if err != nil {
		return nil, err
	}
	return &Tx{raw: tx, d: c.d}, nil
}

func (c *Conn) PingContext(ctx context.Context) error { return c.raw.PingContext(ctx) }

func (c *Conn) Close() error { return c.raw.Close() }

// The pool settings mirror *sql.DB so callers do not need Raw() to tune them.

func (c *Conn) SetMaxOpenConns(n int)              { c.raw.SetMaxOpenConns(n) }
func (c *Conn) SetMaxIdleConns(n int)              { c.raw.SetMaxIdleConns(n) }
func (c *Conn) SetConnMaxLifetime(d time.Duration) { c.raw.SetConnMaxLifetime(d) }
