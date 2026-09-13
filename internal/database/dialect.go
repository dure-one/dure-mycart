// Package database owns every difference between the SQL engines myCart can
// talk to. The rest of the application writes one dialect of SQL — the SQLite
// one, with `?` placeholders and portable DDL — and the Conn wrapper translates
// it on the way to the driver.
package database

import (
	"fmt"

	_ "github.com/jackc/pgx/v5/stdlib" // database/sql driver "pgx"
	_ "modernc.org/sqlite"             // database/sql driver "sqlite"
)

// Supported driver names, as they appear in configuration.
const (
	DriverSQLite   = "sqlite"
	DriverPostgres = "postgres"
)

// Dialect is the irreducible set of differences between the two engines.
//
// Everything that is *not* in here is written in portable SQL that both accept,
// and is covered by migrations/schema_conformance_test.go (the cross-engine
// schema comparison) and internal/database/dialect_test.go: upserts
// (`ON CONFLICT`), boolean literals (`TRUE`/`FALSE`), `CURRENT_TIMESTAMP`,
// `json_array_length`, `ALTER TABLE ... DROP COLUMN` and partial indexes.
type Dialect interface {
	// Name is the configuration-facing name ("sqlite" | "postgres").
	Name() string
	// Driver is the database/sql driver name ("sqlite" | "pgx").
	Driver() string
	// GooseDialect is the dialect name goose expects ("sqlite3" | "postgres").
	GooseDialect() string
	// Rebind rewrites `?` placeholders into the engine's own form.
	Rebind(query string) string

	// Epoch renders an expression returning a timestamp column as unix
	// seconds. The PostgreSQL form must cast to bigint: EXTRACT returns
	// numeric, which does not scan into an int64.
	Epoch(column string) string
	// JSONObject renders a JSON object from "key, value, key, value" pairs.
	JSONObject(pairs string) string
	// JSONAgg renders an aggregate collecting rows into a JSON array.
	JSONAgg(inner string) string
	// JSONValue renders a text column holding JSON as a JSON value, so that an
	// enclosing JSONObject nests it instead of embedding it as a string.
	JSONValue(expr string) string
	// JSONBool renders a boolean column as a JSON true/false. SQLite stores
	// booleans as 0/1, which would otherwise serialise as a number and fail
	// to unmarshal into a Go bool.
	JSONBool(column string) string
}

// SQLite returns the dialect used for the embedded default database.
func SQLite() Dialect { return sqliteDialect{} }

// Postgres returns the dialect used for PostgreSQL.
func Postgres() Dialect { return postgresDialect{} }

// DialectFor maps a configuration driver name to its dialect.
func DialectFor(driver string) (Dialect, error) {
	switch driver {
	case DriverSQLite:
		return sqliteDialect{}, nil
	case DriverPostgres:
		return postgresDialect{}, nil
	default:
		return nil, fmt.Errorf("unknown database driver %q (want %q or %q)",
			driver, DriverSQLite, DriverPostgres)
	}
}
