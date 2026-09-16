package database

import (
	"os"
	"strings"
	"testing"
)

// The session timezone must be pinned to UTC. myCart stores plain TIMESTAMP
// columns and PostgreSQL reads those as UTC when converting to epoch, while
// CURRENT_TIMESTAMP writes them in the session timezone — so any other session
// timezone silently shifts every stored date by the server's offset. The DSN
// pins it; this test is what makes the pin non-optional.
func TestConnectPostgresPinsTimezone(t *testing.T) {
	dsn := postgresTestDSN(t)

	conn, err := Connect(Config{Driver: DriverPostgres, DSN: dsn})
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer func() { _ = conn.Close() }()

	var tz string
	if err := conn.QueryRowContext(t.Context(), "SHOW timezone").Scan(&tz); err != nil {
		t.Fatalf("show timezone: %v", err)
	}
	if !strings.EqualFold(tz, "UTC") {
		t.Errorf("session timezone = %q, want UTC", tz)
	}
}

// connectPostgres refuses a connection whose session timezone is not UTC,
// rather than letting dates drift.
func TestConnectPostgresRejectsNonUTCTimezone(t *testing.T) {
	dsn := postgresTestDSN(t)
	if strings.Contains(dsn, "timezone=") {
		t.Skip("the test DSN pins the timezone itself")
	}

	// An explicit non-UTC timezone survives NormalizePostgresDSN, so the
	// start-up check is what has to catch it.
	conn, err := Connect(Config{Driver: DriverPostgres, DSN: withParam(dsn, "timezone=Asia/Seoul")})
	if err == nil {
		_ = conn.Close()
		t.Fatal("expected the connection to be refused")
	}
	if !strings.Contains(err.Error(), "UTC") {
		t.Errorf("error = %q, want it to mention UTC", err)
	}
}

// postgresTestDSN returns the DSN of the test server, skipping the test when
// the suite is not running against PostgreSQL.
func postgresTestDSN(t *testing.T) string {
	t.Helper()

	driver := os.Getenv("TEST_DB_DRIVER")
	if driver != "" && !strings.EqualFold(driver, DriverPostgres) {
		t.Skip("PostgreSQL test, skipping in SQLite-only mode")
	}
	dsn := os.Getenv("TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("TEST_POSTGRES_DSN is not set")
	}
	// The administrative connection itself, not a pgtestdb one: this test is
	// about Connect's session settings, and the admin connection is the only
	// one whose query string the test controls.
	return dsn
}

// withParam appends a keyword/value parameter, in the form the DSN already uses.
func withParam(dsn, param string) string {
	if strings.Contains(dsn, "?") {
		return dsn + "&" + param
	}
	return dsn + " " + param
}

func TestEnsureSQLiteFileCreatesParentDirectories(t *testing.T) {
	path := t.TempDir() + "/nested/lc_base/data.db"

	if err := ensureSQLiteFile(path); err != nil {
		t.Fatalf("ensureSQLiteFile: %v", err)
	}
	if info, err := os.Stat(path); err != nil || info.IsDir() {
		t.Fatalf("expected a file at %s: %v", path, err)
	}

	// Idempotent: an existing file is left alone.
	if err := ensureSQLiteFile(path); err != nil {
		t.Fatalf("second ensureSQLiteFile: %v", err)
	}
}
