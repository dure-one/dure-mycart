//go:build sqlc

package queries

// The sqlc build replaces a query group's plain file (tagged !sqlc) with a
// <group>_sqlc.go file that keeps the same types and exported methods. Each
// method runs the generated query of the connection's engine through pick or
// run below.
//
// Queries are written so both engines generate identical row structs, which
// lets a SQLite row convert into its PostgreSQL twin (pggen.X(row)) and one
// mapper serve both. If the engines ever infer different types, that
// conversion stops compiling.

import (
	"database/sql"

	"github.com/shurco/mycart/internal/database"
	pggen "github.com/shurco/mycart/internal/queries/sqlcgen/postgres"
	sqlitegen "github.com/shurco/mycart/internal/queries/sqlcgen/sqlite"
)

// dbtx is what the generated code executes on: a *database.Conn or a
// *database.Tx. Their rebinding leaves sqlc's SQL alone — SQLite keeps `?` and
// the PostgreSQL queries are already numbered.
type dbtx = pggen.DBTX

// isPostgres reports whether conn talks to PostgreSQL. Every other engine the
// project supports is SQLite.
func isPostgres(conn *database.Conn) bool {
	return conn.Dialect().Name() == "postgres"
}

// pick runs the closure for conn's engine against x and returns its result.
func pick[T any](conn *database.Conn, x dbtx, pg func(*pggen.Queries) (T, error), lite func(*sqlitegen.Queries) (T, error)) (T, error) {
	if isPostgres(conn) {
		return pg(pggen.New(x))
	}
	return lite(sqlitegen.New(x))
}

// run is pick for statements that return nothing but an error.
func run(conn *database.Conn, x dbtx, pg func(*pggen.Queries) error, lite func(*sqlitegen.Queries) error) error {
	if isPostgres(conn) {
		return pg(pggen.New(x))
	}
	return lite(sqlitegen.New(x))
}

// pageBounds turns limit and offset into LIMIT/OFFSET values the way paginate
// does: a limit of zero or less means no limit, and then no offset either. The
// "no limit" value is -1, which SQLite reads as unbounded; PostgreSQL wants
// NULL instead, see pgLimit.
func pageBounds(limit, offset int) (lim, off int64) {
	lim = -1
	if limit > 0 {
		lim = int64(limit)
		if offset > 0 {
			off = int64(offset)
		}
	}
	return lim, off
}

// pgLimit is pageBounds' limit for PostgreSQL, which reads LIMIT NULL as unbounded.
func pgLimit(lim int64) sql.NullInt64 {
	return sql.NullInt64{Int64: lim, Valid: lim >= 0}
}

// convertAll converts SQLite rows into their identical PostgreSQL row type.
func convertAll[L, P any](rows []L, err error, conv func(L) P) ([]P, error) {
	if err != nil {
		return nil, err
	}
	out := make([]P, len(rows))
	for i := range rows {
		out[i] = conv(rows[i])
	}
	return out, nil
}
