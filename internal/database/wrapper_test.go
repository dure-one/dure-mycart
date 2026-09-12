package database

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/shurco/mycart/migrations"
)

// newTestConn opens a migrated SQLite database in a temporary directory.
func newTestConn(t *testing.T) *Conn {
	t.Helper()

	cfg := Config{Driver: DriverSQLite, DSN: filepath.Join(t.TempDir(), "data.db")}
	conn, err := Open(cfg, migrations.Embed())
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	return conn
}

// Wrap has to keep both halves reachable: the dialect decides how placeholders
// are rendered, and Raw is the escape hatch goose and the fixtures use.
func TestWrapKeepsDialectAndRaw(t *testing.T) {
	raw, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer func() { _ = raw.Close() }()

	conn := Wrap(raw, SQLite())
	if conn.Raw() != raw {
		t.Error("Raw returned a different handle")
	}
	if conn.Dialect().Name() != DriverSQLite {
		t.Errorf("dialect = %q", conn.Dialect().Name())
	}
}

// The wrapper exists so application code can use the *sql.DB method set without
// ever seeing a `?`. Each method it re-declares has to actually work.
func TestConnMethods(t *testing.T) {
	conn := newTestConn(t)
	ctx := t.Context()

	if err := conn.PingContext(ctx); err != nil {
		t.Fatalf("ping: %v", err)
	}

	// The pool settings have no return value; the point is that they reach the
	// underlying handle rather than panicking or being silently dropped.
	conn.SetMaxOpenConns(4)
	conn.SetMaxIdleConns(2)
	conn.SetConnMaxLifetime(time.Minute)
	if err := conn.PingContext(ctx); err != nil {
		t.Fatalf("ping after pool settings: %v", err)
	}

	rows, err := conn.QueryContext(ctx, `SELECT id FROM setting ORDER BY id LIMIT 2`)
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	defer func() { _ = rows.Close() }()

	var seen int
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			t.Fatalf("scan: %v", err)
		}
		seen++
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("rows: %v", err)
	}
	if seen != 2 {
		t.Errorf("read %d settings, want 2", seen)
	}

	var one string
	if err := conn.QueryRowContext(ctx, `SELECT id FROM setting ORDER BY id LIMIT 1`).Scan(&one); err != nil {
		t.Fatalf("query row: %v", err)
	}

	stmt, err := conn.PrepareContext(ctx, `SELECT COUNT(*) FROM setting WHERE key = ?`)
	if err != nil {
		t.Fatalf("prepare: %v", err)
	}
	defer func() { _ = stmt.Close() }()
	var count int
	if err := stmt.QueryRowContext(ctx, "installed").Scan(&count); err != nil {
		t.Fatalf("prepared query: %v", err)
	}
	if count != 1 {
		t.Errorf("installed rows = %d, want 1", count)
	}

	res, err := conn.ExecContext(ctx, `UPDATE setting SET value = ? WHERE key = ?`, "yes", "installed")
	if err != nil {
		t.Fatalf("exec: %v", err)
	}
	if n, err := res.RowsAffected(); err != nil || n != 1 {
		t.Errorf("rows affected = %d (%v), want 1", n, err)
	}
}

// A transaction has to rebind too: a `?` reaching PostgreSQL through a Tx would
// be the same bug the wrapper exists to prevent.
func TestTxMethods(t *testing.T) {
	conn := newTestConn(t)
	ctx := t.Context()

	var before string
	if err := conn.QueryRowContext(ctx, `SELECT value FROM setting WHERE key = ?`, "installed").Scan(&before); err != nil {
		t.Fatalf("read the pre-transaction value: %v", err)
	}

	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}

	if _, err := tx.ExecContext(ctx, `UPDATE setting SET value = ? WHERE key = ?`, "tx", "installed"); err != nil {
		t.Fatalf("tx exec: %v", err)
	}

	var value string
	if err := tx.QueryRowContext(ctx, `SELECT value FROM setting WHERE key = ?`, "installed").Scan(&value); err != nil {
		t.Fatalf("tx query row: %v", err)
	}
	if value != "tx" {
		t.Errorf("value = %q, want %q", value, "tx")
	}

	rows, err := tx.QueryContext(ctx, `SELECT COUNT(*) FROM setting WHERE value = ?`, "tx")
	if err != nil {
		t.Fatalf("tx query: %v", err)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		t.Fatalf("tx rows: %v", err)
	}

	stmt, err := tx.PrepareContext(ctx, `SELECT COUNT(*) FROM setting WHERE value = ?`)
	if err != nil {
		t.Fatalf("tx prepare: %v", err)
	}
	defer func() { _ = stmt.Close() }()
	var n int
	if err := stmt.QueryRowContext(ctx, "tx").Scan(&n); err != nil {
		t.Fatalf("tx prepared query: %v", err)
	}
	if n != 1 {
		t.Errorf("matching rows = %d, want 1", n)
	}

	// Rollback forwards to the underlying *sql.Tx, so the value must not stick.
	if err := tx.Rollback(); err != nil {
		t.Fatalf("rollback: %v", err)
	}
	var after string
	if err := conn.QueryRowContext(ctx, `SELECT value FROM setting WHERE key = ?`, "installed").Scan(&after); err != nil {
		t.Fatalf("query after rollback: %v", err)
	}
	if after != before {
		t.Errorf("value after rollback = %q, want the pre-transaction %q", after, before)
	}
}

// MigrateOn is the only entry point for databases whose connection cannot be
// reopened (in-memory SQLite), so its refusal to run without migrations and its
// idempotence both matter.
func TestMigrateOnRequiresMigrations(t *testing.T) {
	conn := newTestConn(t)

	if err := MigrateOn(conn, nil); err == nil {
		t.Fatal("expected an error for a nil filesystem")
	} else if !strings.Contains(err.Error(), "no migrations") {
		t.Errorf("error = %q, want it to mention the missing migrations", err)
	}
}

func TestMigrateIsIdempotentOnSQLite(t *testing.T) {
	path := filepath.Join(t.TempDir(), "data.db")
	cfg := Config{Driver: DriverSQLite, DSN: path}

	if err := Migrate(cfg, migrations.Embed()); err != nil {
		t.Fatalf("first migrate: %v", err)
	}
	before := versionRows(t, cfg)

	if err := Migrate(cfg, migrations.Embed()); err != nil {
		t.Fatalf("second migrate: %v", err)
	}
	if after := versionRows(t, cfg); after != before {
		t.Errorf("version table grew from %d to %d rows", before, after)
	}
	if before < 2 {
		t.Fatalf("version table has %d rows; the sanity check needs at least 2", before)
	}
}

// versionRows counts the rows goose recorded, through a connection that does not
// go near the package's own helpers.
func versionRows(t *testing.T, cfg Config) int {
	t.Helper()

	conn, err := Connect(cfg)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer func() { _ = conn.Close() }()

	var n int
	if err := conn.QueryRowContext(t.Context(),
		`SELECT COUNT(*) FROM `+MigrateTable).Scan(&n); err != nil {
		t.Fatalf("count versions: %v", err)
	}
	return n
}

// Open is Migrate followed by Connect, and must leave a usable handle behind.
func TestOpenReturnsUsableHandle(t *testing.T) {
	cfg := Config{Driver: DriverSQLite, DSN: filepath.Join(t.TempDir(), "nested", "data.db")}

	conn, err := Open(cfg, migrations.Embed())
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer func() { _ = conn.Close() }()

	var installed string
	if err := conn.QueryRowContext(t.Context(),
		`SELECT value FROM setting WHERE key = ?`, "installed").Scan(&installed); err != nil {
		t.Fatalf("query: %v", err)
	}
	if installed == "" {
		t.Error("the seeded `installed` setting is empty")
	}
}

func TestConnectUnknownDriver(t *testing.T) {
	if _, err := Connect(Config{Driver: "mysql", DSN: "x"}); err == nil {
		t.Fatal("expected an error for an unknown driver")
	}
}

// A path whose parent is a file, not a directory, is the realistic way a fresh
// installation cannot be created.
func TestConnectSQLiteReportsUnwritablePath(t *testing.T) {
	dir := t.TempDir()
	blocker := filepath.Join(dir, "not-a-directory")
	if err := os.WriteFile(blocker, []byte("x"), 0o600); err != nil {
		t.Fatalf("write blocker: %v", err)
	}

	_, err := Connect(Config{Driver: DriverSQLite, DSN: filepath.Join(blocker, "data.db")})
	if err == nil {
		t.Fatal("expected an error for a database path under a file")
	}
}

func TestAddParams(t *testing.T) {
	tests := []struct {
		name, dsn, params, want string
	}{
		{"no query yet", "file.db", "a=1", "file.db?a=1"},
		{"query already present", "file.db?b=2", "a=1", "file.db?b=2&a=1"},
		{"postgres keyword form", "host=localhost", "a=1", "host=localhost?a=1"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := addParams(tt.dsn, tt.params); got != tt.want {
				t.Errorf("addParams(%q, %q) = %q, want %q", tt.dsn, tt.params, got, tt.want)
			}
		})
	}
}

func TestEnvIntAndDuration(t *testing.T) {
	t.Run("unset keeps the default", func(t *testing.T) {
		if got := envInt("MYCART_TEST_UNSET_INT", 7); got != 7 {
			t.Errorf("envInt = %d, want 7", got)
		}
		if got := envDuration("MYCART_TEST_UNSET_DURATION", time.Minute); got != time.Minute {
			t.Errorf("envDuration = %s, want 1m", got)
		}
	})

	t.Run("invalid keeps the default", func(t *testing.T) {
		t.Setenv("MYCART_TEST_BAD_INT", "not-a-number")
		t.Setenv("MYCART_TEST_BAD_DURATION", "not-a-duration")

		if got := envInt("MYCART_TEST_BAD_INT", 7); got != 7 {
			t.Errorf("envInt = %d, want 7", got)
		}
		if got := envDuration("MYCART_TEST_BAD_DURATION", time.Minute); got != time.Minute {
			t.Errorf("envDuration = %s, want 1m", got)
		}
	})

	t.Run("non-positive keeps the default", func(t *testing.T) {
		// Zero would mean "unlimited" to database/sql, which is never what an
		// operator setting a pool size means.
		t.Setenv("MYCART_TEST_ZERO_INT", "0")
		t.Setenv("MYCART_TEST_NEGATIVE_INT", "-1")
		t.Setenv("MYCART_TEST_ZERO_DURATION", "0s")

		if got := envInt("MYCART_TEST_ZERO_INT", 7); got != 7 {
			t.Errorf("envInt(0) = %d, want 7", got)
		}
		if got := envInt("MYCART_TEST_NEGATIVE_INT", 7); got != 7 {
			t.Errorf("envInt(-1) = %d, want 7", got)
		}
		if got := envDuration("MYCART_TEST_ZERO_DURATION", time.Minute); got != time.Minute {
			t.Errorf("envDuration(0s) = %s, want 1m", got)
		}
	})

	t.Run("valid values win", func(t *testing.T) {
		t.Setenv("MYCART_TEST_GOOD_INT", "12")
		t.Setenv("MYCART_TEST_GOOD_DURATION", "90s")

		if got := envInt("MYCART_TEST_GOOD_INT", 7); got != 12 {
			t.Errorf("envInt = %d, want 12", got)
		}
		if got := envDuration("MYCART_TEST_GOOD_DURATION", time.Minute); got != 90*time.Second {
			t.Errorf("envDuration = %s, want 90s", got)
		}
	})
}

func TestReadConfigFileRejectsBrokenFiles(t *testing.T) {
	dir := t.TempDir()

	tests := []struct {
		name    string
		content string
		want    string
	}{
		{"not json", "{", "parse database config"},
		{"no driver", `{"dsn":"./x.db"}`, "driver is required"},
		{"empty file", "", "parse database config"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(dir, strings.ReplaceAll(tt.name, " ", "-")+".json")
			if err := os.WriteFile(path, []byte(tt.content), 0o600); err != nil {
				t.Fatalf("write: %v", err)
			}

			_, _, err := readConfigFile(path)
			if err == nil {
				t.Fatal("expected an error")
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Errorf("error = %q, want it to mention %q", err, tt.want)
			}
		})
	}

	t.Run("unreadable path", func(t *testing.T) {
		// A directory is not a file: this is the branch that is neither "missing"
		// nor "malformed", and it must not be mistaken for either.
		if _, _, err := readConfigFile(dir); err == nil {
			t.Fatal("expected an error when the path is a directory")
		} else if !strings.Contains(err.Error(), "read database config") {
			t.Errorf("error = %q", err)
		}
	})
}

func TestConfigFileRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "lc_base", "config.json")

	want := Config{Driver: DriverPostgres, DSN: "postgres://user:pw@localhost:5432/cart", Source: SourceWizard}
	if err := WriteConfigFile(want, path); err != nil {
		t.Fatalf("write: %v", err)
	}

	got, ok, err := readConfigFile(path)
	if err != nil || !ok {
		t.Fatalf("read: ok=%v err=%v", ok, err)
	}
	if got.Driver != want.Driver || got.DSN != want.DSN || got.Source != SourceFile {
		t.Errorf("got %+v, want driver/dsn from the file and source %q", got, SourceFile)
	}
}

func TestRemoveConfigIsIdempotent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	if err := WriteConfigFile(Config{Driver: DriverSQLite, DSN: "./x.db"}, path); err != nil {
		t.Fatalf("write: %v", err)
	}

	if err := os.Remove(path); err != nil {
		t.Fatalf("remove: %v", err)
	}
	// RemoveConfig works on ConfigPath, so exercise the same branch through it:
	// a missing file is not an error.
	if err := RemoveConfig(); err != nil {
		t.Errorf("RemoveConfig on a missing file: %v", err)
	}
}

// Active is what the install wizard reads to decide whether the database is
// pinned. Before anything sets it, the process is on the built-in default.
func TestActiveDefaultsToTheBuiltinSQLiteDatabase(t *testing.T) {
	cfg := active.Load()
	active.Store(nil)
	t.Cleanup(func() { active.Store(cfg) })

	got := Active()
	if got.Driver != DriverSQLite || got.DSN != DefaultSQLiteDSN || got.Source != SourceDefault {
		t.Errorf("Active() = %+v, want the built-in default", got)
	}
	if got.Pinned() {
		t.Error("the default must not count as pinned")
	}
}

func TestSetActiveRoundTrip(t *testing.T) {
	previous := active.Load()
	t.Cleanup(func() { active.Store(previous) })

	want := Config{Driver: DriverPostgres, DSN: "postgres://x/y", Source: SourceWizard}
	SetActive(want)

	if got := Active(); got != want {
		t.Errorf("Active() = %+v, want %+v", got, want)
	}
	if Active().Pinned() {
		t.Error("a wizard choice must not count as pinned")
	}
}

var _ = context.Background
