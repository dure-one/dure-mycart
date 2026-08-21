package base

import (
	"database/sql"
	"embed"
	"fmt"

	"github.com/pressly/goose/v3"
	"github.com/shurco/mycart/pkg/fsutil"
)

// buildDSN builds a SQLite connection string with optimized parameters.
// _txlock=immediate makes every transaction take its write lock up front,
// which prevents deferred-transaction lock-upgrade deadlocks that
// busy_timeout cannot rescue.
//
// Note: _txlock must NOT be present on connections used by goose (it breaks
// its statement handling), so migrations run on a separate DSN.
func buildDSN(dbPath string) string {
	return fmt.Sprintf("%s?_pragma=busy_timeout(10000)&_pragma=journal_mode(WAL)&_pragma=journal_size_limit(200000000)&_pragma=synchronous(NORMAL)&_pragma=foreign_keys(ON)&_txlock=immediate", dbPath)
}

// New creates a new database connection and brings the schema up to date.
func New(dbPath string, migrations embed.FS) (*sql.DB, error) {
	if !fsutil.IsFile(dbPath) {
		if _, err := fsutil.OpenFile(dbPath, fsutil.FsCWFlags, 0o666); err != nil {
			return nil, err
		}
	}

	// Always run migrations on startup: goose.Up is idempotent and cheap,
	// so a binary upgrade followed by `serve` can never talk to a stale
	// schema (previously migrations only ran on freshly created databases).
	if err := Migrate(dbPath, migrations); err != nil {
		return nil, fmt.Errorf("run migrations: %w", err)
	}

	db, err := sql.Open("sqlite", buildDSN(dbPath))
	if err != nil {
		return nil, err
	}

	if _, err := db.Exec("PRAGMA auto_vacuum"); err != nil {
		return nil, err
	}

	return db, nil
}

// Migrate performs database migrations
func Migrate(dbPath string, migrations embed.FS) error {
	goose.SetBaseFS(migrations)
	db, err := goose.OpenDBWithDriver("sqlite", dbPath+"?_pragma=busy_timeout(10000)")
	if err != nil {
		return err
	}
	defer func() { _ = db.Close() }()

	goose.SetTableName("migrate_db_version")

	return goose.Up(db, ".")
}
