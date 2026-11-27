//go:build sqlc

package queries

import (
	"context"
	"database/sql"
	"strings"
	"time"

	pggen "github.com/dure-one/dure-mycart/internal/queries/sqlcgen/postgres"
	sqlitegen "github.com/dure-one/dure-mycart/internal/queries/sqlcgen/sqlite"
)

// GetSession retrieves the session value for a given key if it hasn't expired.
// It takes a context and key as arguments and returns the session value and an error if any.
func (q *SettingQueries) GetSession(ctx context.Context, key string) (string, error) {
	now := time.Now().Unix()
	value, err := pick(q.DB, q.DB,
		func(g *pggen.Queries) (sql.NullString, error) {
			return g.GetSession(ctx, pggen.GetSessionParams{Key: key, Now: now})
		},
		func(g *sqlitegen.Queries) (sql.NullString, error) {
			return g.GetSession(ctx, sqlitegen.GetSessionParams{Key: key, Now: now})
		})
	if err != nil {
		return "", err
	}
	return value.String, nil
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
	val := sql.NullString{String: value, Valid: true}
	return run(q.DB, q.DB,
		func(g *pggen.Queries) error {
			if err := g.SweepSessions(ctx, pggen.SweepSessionsParams{Now: now, Key: key}); err != nil {
				return err
			}
			return g.UpsertSession(ctx, pggen.UpsertSessionParams{Key: key, Value: val, Expires: expires})
		},
		func(g *sqlitegen.Queries) error {
			if err := g.SweepSessions(ctx, sqlitegen.SweepSessionsParams{Now: now, Key: key}); err != nil {
				return err
			}
			return g.UpsertSession(ctx, sqlitegen.UpsertSessionParams{Key: key, Value: val, Expires: expires})
		})
}

// UpdateSession updates the session with a new value and expiration time for a given key.
// It takes a context, a session key, the new value to be set, and the new expiration time as arguments.
func (q *SettingQueries) UpdateSession(ctx context.Context, key, value string, expires int64) error {
	val := sql.NullString{String: value, Valid: true}
	return run(q.DB, q.DB,
		func(g *pggen.Queries) error {
			return g.UpdateSession(ctx, pggen.UpdateSessionParams{Value: val, Expires: expires, Key: key})
		},
		func(g *sqlitegen.Queries) error {
			return g.UpdateSession(ctx, sqlitegen.UpdateSessionParams{Value: val, Expires: expires, Key: key})
		})
}

// DeleteSession removes a session from the database based on the provided key.
func (q *SettingQueries) DeleteSession(ctx context.Context, key string) error {
	return run(q.DB, q.DB,
		func(g *pggen.Queries) error { return g.DeleteSession(ctx, key) },
		func(g *sqlitegen.Queries) error { return g.DeleteSession(ctx, key) })
}

// The helpers below are copied unchanged from session.go, which the sqlc
// build leaves out. Keep them in step with it.

// The two authenticated surfaces this application serves.
const (
	// SessionRoleAdmin is the role of the single admin account.
	SessionRoleAdmin = "admin"
	// SessionRoleCustomer is the role of a storefront cabinet session.
	SessionRoleCustomer = "customer"
)

// SessionValue formats the value column of a session row as "<role>:<subject>".
//
// The role is what keeps the surfaces apart. Both the admin and the storefront
// middleware accept a token on the strength of a row in this table, and the
// tokens themselves carry nothing but a session id — so without the role a row
// created for one surface would be enough to be admitted by the other. The
// subject names who the session belongs to: a customer id for the cabinet,
// empty for the admin, who is singular and unnamed.
func SessionValue(role, subject string) string {
	if subject == "" {
		return role
	}
	return role + ":" + subject
}

// ParseSessionValue splits a session value into its role and subject. A value
// with no separator is a bare role, which is how admin sessions were written
// before subjects existed; the middleware has to keep accepting those.
func ParseSessionValue(value string) (role, subject string) {
	role, subject, _ = strings.Cut(value, ":")
	return role, subject
}
