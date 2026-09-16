package database

import (
	"context"
	"database/sql"
	"fmt"
	"io/fs"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/shurco/mycart/pkg/fsutil"
)

// SQLite DSN pragmas. These are the values myCart has always used and they are
// reproduced verbatim; changing any of them changes on-disk behaviour.
//
// Note: _txlock must NOT be present on connections used by goose (it breaks its
// statement handling), so migrations run on a separate DSN — see gooseDSN.
const (
	sqlitePragmas    = "?_pragma=busy_timeout(10000)&_pragma=journal_mode(WAL)&_pragma=journal_size_limit(200000000)&_pragma=synchronous(NORMAL)&_pragma=foreign_keys(ON)&_txlock=immediate"
	sqliteGooseParam = "_pragma=busy_timeout(10000)"
)

// PostgreSQL pool defaults.
const (
	defaultPGMaxOpenConns    = 20
	defaultPGMaxIdleConns    = 10
	defaultPGConnMaxLifetime = 30 * time.Minute
)

// Environment variables overriding the PostgreSQL pool settings.
const (
	EnvMaxOpenConns    = "MYCART_DB_MAX_OPEN_CONNS"
	EnvMaxIdleConns    = "MYCART_DB_MAX_IDLE_CONNS"
	EnvConnMaxLifetime = "MYCART_DB_CONN_MAX_LIFETIME"
)

// Connect opens a connection pool for cfg without running migrations.
//
// A new SQLite database file is created if it does not exist, so callers do not
// have to care whether the installation is fresh.
func Connect(cfg Config) (*Conn, error) {
	switch cfg.Driver {
	case DriverSQLite:
		return connectSQLite(cfg.DSN)
	case DriverPostgres:
		return connectPostgres(cfg.DSN)
	default:
		return nil, fmt.Errorf("unknown database driver %q", cfg.Driver)
	}
}

// Open connects to cfg, brings the schema up to date and returns the ready to
// use handle. Migrations always run: goose.Up is idempotent and cheap, so a
// binary upgrade followed by `serve` can never talk to a stale schema.
func Open(cfg Config, migrations fs.FS) (*Conn, error) {
	if err := Migrate(cfg, migrations); err != nil {
		return nil, err
	}
	return Connect(cfg)
}

// ensureSQLiteFile creates the database file and its directory when missing, so
// a fresh installation has somewhere to write. It is idempotent.
func ensureSQLiteFile(path string) error {
	if fsutil.IsFile(path) {
		return nil
	}
	if err := fsutil.MkDirs(0o775, filepath.Dir(path)); err != nil {
		return fmt.Errorf("create database directory: %w", err)
	}
	if _, err := fsutil.OpenFile(path, fsutil.FsCWFlags, 0o666); err != nil {
		return err
	}
	return nil
}

func connectSQLite(path string) (*Conn, error) {
	if err := ensureSQLiteFile(path); err != nil {
		return nil, err
	}

	raw, err := sql.Open("sqlite", path+sqlitePragmas)
	if err != nil {
		return nil, fmt.Errorf("open sqlite %s: %w", path, err)
	}

	// Touch the connection so a bad path or an unreadable file fails here
	// rather than on the first request.
	if _, err := raw.Exec("PRAGMA auto_vacuum"); err != nil {
		_ = raw.Close()
		return nil, fmt.Errorf("open sqlite %s: %w", path, err)
	}

	// Pool settings are deliberately left at their defaults: SQLite is a local
	// file and the current behaviour must not change.
	return Wrap(raw, SQLite()), nil
}

func connectPostgres(dsn string) (*Conn, error) {
	dsn, err := NormalizePostgresDSN(dsn)
	if err != nil {
		return nil, err
	}

	raw, err := sql.Open(Postgres().Driver(), dsn)
	if err != nil {
		return nil, fmt.Errorf("open postgres: %w", err)
	}
	conn := Wrap(raw, Postgres())

	conn.SetMaxOpenConns(envInt(EnvMaxOpenConns, defaultPGMaxOpenConns))
	conn.SetMaxIdleConns(envInt(EnvMaxIdleConns, defaultPGMaxIdleConns))
	conn.SetConnMaxLifetime(envDuration(EnvConnMaxLifetime, defaultPGConnMaxLifetime))

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if err := conn.PingContext(ctx); err != nil {
		_ = conn.Close()
		// The password never appears here — pgx does not put it in the error.
		// The user, database and host do, because pgx names them in its dial
		// errors; they are not secrets to the operator, who typed them, but
		// they must not reach an unauthenticated caller either. The install
		// wizard is the one place this error can get that far, and
		// describeConnectFailure replaces it with a generic reason before it
		// is returned.
		return nil, fmt.Errorf("connect to postgres: %w", err)
	}

	// The session timezone must be UTC. myCart stores TIMESTAMP (without time
	// zone), and PostgreSQL reads such a value as UTC when converting to epoch,
	// while CURRENT_TIMESTAMP writes it in the session timezone. Any non-UTC
	// session therefore shifts every stored date by the server's offset. The
	// DSN pins it; this check makes a DSN that lost the pin fail loudly instead
	// of silently corrupting dates.
	var tz string
	if err := conn.QueryRowContext(ctx, "SHOW timezone").Scan(&tz); err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("check postgres timezone: %w", err)
	}
	if !strings.EqualFold(tz, "UTC") {
		_ = conn.Close()
		return nil, fmt.Errorf(
			"postgres session timezone is %q, must be \"UTC\": dates would be stored shifted by the server offset; "+
				"add timezone=UTC to the connection string", tz)
	}

	return conn, nil
}

// NormalizePostgresDSN validates a PostgreSQL connection string and pins the
// session timezone to UTC.
//
// It is exported because the test suite has to hand PostgreSQL DSNs to pgx
// directly: the pin is not optional, so anything that builds such a DSN should
// go through this rather than restate the rule.
func NormalizePostgresDSN(dsn string) (string, error) {
	if dsn == "" {
		return "", fmt.Errorf("postgres connection string is empty")
	}

	if strings.HasPrefix(dsn, "postgres://") || strings.HasPrefix(dsn, "postgresql://") {
		u, err := url.Parse(dsn)
		if err != nil {
			return "", fmt.Errorf("parse postgres connection string: %w", err)
		}
		q := u.Query()
		if err := rejectPoolParams(q); err != nil {
			return "", err
		}
		// Use options=-c format for PgBouncer compatibility (Supabase pooler).
		// The timezone parameter doesn't work with transaction pooling mode.
		if !q.Has("timezone") && !q.Has("TimeZone") {
			if q.Has("options") {
				// Append to existing options
				opts := q.Get("options")
				if !strings.Contains(opts, "timezone") {
					q.Set("options", opts+" -c timezone=UTC")
				}
			} else {
				// Set new options parameter
				q.Set("options", "-c timezone=UTC")
			}
		}
		u.RawQuery = q.Encode()
		return u.String(), nil
	}

	parts := strings.Fields(dsn)
	for _, p := range parts {
		key, _, _ := strings.Cut(p, "=")
		if err := rejectPoolParam(key); err != nil {
			return "", err
		}
	}
	// Use options=-c format for PgBouncer compatibility
	if !hasKeyword(parts, "timezone") && !hasKeyword(parts, "TimeZone") {
		if hasKeyword(parts, "options") {
			// Append to existing options
			for i, p := range parts {
				if strings.HasPrefix(p, "options=") {
					opts := strings.TrimPrefix(p, "options=")
					if !strings.Contains(opts, "timezone") {
						parts[i] = "options=" + opts + " -c timezone=UTC"
					}
					break
				}
			}
		} else {
			// Add new options parameter
			parts = append(parts, "options=-c timezone=UTC")
		}
	}
	return strings.Join(parts, " "), nil
}

// rejectPoolParams rejects pgxpool-only parameters. They are a documented
// footgun: passed to the pgx stdlib driver they reach the server as GUCs and
// fail with `FATAL: unrecognized configuration parameter "pool_max_conns"`.
func rejectPoolParams(q url.Values) error {
	for key := range q {
		if err := rejectPoolParam(key); err != nil {
			return err
		}
	}
	return nil
}

func rejectPoolParam(key string) error {
	if strings.HasPrefix(strings.ToLower(key), "pool_") {
		return fmt.Errorf(
			"connection string parameter %q is a pgxpool option and is not supported here; "+
				"use %s to size the pool", key, EnvMaxOpenConns)
	}
	return nil
}

func hasKeyword(parts []string, key string) bool {
	for _, p := range parts {
		if k, _, ok := strings.Cut(p, "="); ok && strings.EqualFold(k, key) {
			return true
		}
	}
	return false
}

func envInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return n
		}
	}
	return def
}

func envDuration(key string, def time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		if d, err := time.ParseDuration(v); err == nil && d > 0 {
			return d
		}
	}
	return def
}
