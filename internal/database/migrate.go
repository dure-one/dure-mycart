package database

import (
	"context"
	"database/sql"
	"fmt"
	"io/fs"
	"strings"

	"github.com/pressly/goose/v3"
	goosedb "github.com/pressly/goose/v3/database"
)

// MigrateTable is the goose bookkeeping table. The name predates this package
// and is load-bearing: changing it would make goose re-run every migration on
// existing installations.
const MigrateTable = "migrate_db_version"

// Migrate brings the schema up to date with the embedded migrations.
//
// The migration set is shared by both engines: the SQL is written to be
// accepted by SQLite and PostgreSQL alike, so there is exactly one directory to
// maintain.
func Migrate(cfg Config, migrations fs.FS) error {
	d, err := DialectFor(cfg.Driver)
	if err != nil {
		return err
	}

	raw, err := openForGoose(cfg, d)
	if err != nil {
		return err
	}
	defer func() { _ = raw.Close() }()

	return MigrateOn(Wrap(raw, d), migrations)
}

// MigrateOn runs the migrations on an already open handle. It exists for
// callers whose connection cannot be reopened — in-memory SQLite in tests, for
// instance, where the schema only exists on the one connection that created it.
func MigrateOn(conn *Conn, migrations fs.FS) error {
	if migrations == nil {
		return fmt.Errorf("no migrations provided")
	}

	d := conn.Dialect()
	store, err := goosedb.NewStore(goosedb.Dialect(d.GooseDialect()), MigrateTable)
	if err != nil {
		return fmt.Errorf("create migration store: %w", err)
	}
	// An explicit store keeps goose's package-level state (SetBaseFS,
	// SetTableName, SetDialect) out of this: it is global, so callers running
	// concurrently would race and could migrate under each other's dialect.
	provider, err := goose.NewProvider(goose.DialectCustom, conn.Raw(), migrations,
		goose.WithStore(store),
		goose.WithDisableGlobalRegistry(true))
	if err != nil {
		return fmt.Errorf("create migration provider: %w", err)
	}

	if _, err := provider.Up(context.Background()); err != nil {
		return fmt.Errorf("run migrations: %w", err)
	}
	return nil
}

// openForGoose opens a dedicated connection for goose, because goose's
// statement handling is incompatible with `_txlock=immediate`.
func openForGoose(cfg Config, d Dialect) (*sql.DB, error) {
	dsn := cfg.DSN
	if d.Name() == DriverSQLite {
		// goose cannot create the file itself, and a fresh installation has
		// no lc_base directory yet.
		if err := ensureSQLiteFile(dsn); err != nil {
			return nil, err
		}
		dsn = addParams(dsn, sqliteGooseParam)
	} else {
		var err error
		if dsn, err = NormalizePostgresDSN(dsn); err != nil {
			return nil, err
		}
	}

	raw, err := sql.Open(d.Driver(), dsn)
	if err != nil {
		return nil, fmt.Errorf("open %s for migrations: %w", d.Name(), err)
	}
	return raw, nil
}

// addParams appends a raw query string to a DSN, choosing the right separator.
func addParams(dsn, params string) string {
	if strings.Contains(dsn, "?") {
		return dsn + "&" + params
	}
	return dsn + "?" + params
}
