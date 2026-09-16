package database_test

import (
	"os"
	"strings"
	"testing"

	"github.com/shurco/mycart/internal/database"
	"github.com/shurco/mycart/internal/testutil/pgtest"
	"github.com/shurco/mycart/migrations"
)

// requirePostgres skips the test unless the suite is pointed at a test server.
// The provisioning itself is pgtestdb's, through internal/testutil/pgtest.
func requirePostgres(t *testing.T) {
	t.Helper()

	driver := os.Getenv("TEST_DB_DRIVER")
	if driver != "" && !strings.EqualFold(driver, database.DriverPostgres) {
		t.Skip("PostgreSQL test, skipping in SQLite-only mode")
	}

	if os.Getenv("TEST_POSTGRES_DSN") == "" {
		t.Skip("set TEST_POSTGRES_DSN to run PostgreSQL tests")
	}
}

// The production migration path has to work on an empty PostgreSQL database,
// not just on one a test helper prepared — this is what runs on every start of
// a real installation.
func TestMigratePostgresFromEmpty(t *testing.T) {
	requirePostgres(t)

	cfg := database.Config{Driver: database.DriverPostgres, DSN: pgtest.EmptyDSN(t)}
	if err := database.Migrate(cfg, migrations.Embed()); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	conn, err := database.Connect(cfg)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer func() { _ = conn.Close() }()

	var versions int
	if err := conn.QueryRowContext(t.Context(),
		`SELECT COUNT(*) FROM `+database.MigrateTable).Scan(&versions); err != nil {
		t.Fatalf("count versions: %v", err)
	}
	// One row per applied migration plus goose's own baseline row.
	if versions < 2 {
		t.Errorf("recorded %d versions, want the whole migration set", versions)
	}

	// A second run must be a no-op rather than an error or a re-apply.
	if err := database.Migrate(cfg, migrations.Embed()); err != nil {
		t.Fatalf("second migrate: %v", err)
	}
	var again int
	if err := conn.QueryRowContext(t.Context(),
		`SELECT COUNT(*) FROM `+database.MigrateTable).Scan(&again); err != nil {
		t.Fatalf("count versions again: %v", err)
	}
	if again != versions {
		t.Errorf("version table grew from %d to %d rows", versions, again)
	}
}

// Open is what a real installation calls: migrate, then connect.
func TestOpenPostgresReturnsUsableHandle(t *testing.T) {
	requirePostgres(t)

	cfg := database.Config{Driver: database.DriverPostgres, DSN: pgtest.EmptyDSN(t)}
	conn, err := database.Open(cfg, migrations.Embed())
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer func() { _ = conn.Close() }()

	if err := conn.PingContext(t.Context()); err != nil {
		t.Fatalf("ping: %v", err)
	}

	var installed string
	if err := conn.QueryRowContext(t.Context(),
		`SELECT value FROM setting WHERE key = ?`, "installed").Scan(&installed); err != nil {
		t.Fatalf("query through the dialect wrapper: %v", err)
	}
	if installed == "" {
		t.Error("the seeded `installed` setting is empty")
	}
}

// Migrate validates the connection string itself, before it ever reaches the
// server: a bad one must not be reported as a connection failure.
func TestMigratePostgresRejectsBadDSN(t *testing.T) {
	requirePostgres(t)

	tests := []struct {
		name, dsn, want string
	}{
		{"empty", "", "connection string is empty"},
		{"unparseable", "postgres://%zz", "parse postgres connection string"},
		{"pgxpool option", "postgres://user@localhost/db?pool_max_conns=5", "pool_max_conns"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := database.Config{Driver: database.DriverPostgres, DSN: tt.dsn}
			err := database.Migrate(cfg, migrations.Embed())
			if err == nil {
				t.Fatal("expected an error")
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Errorf("error = %q, want it to mention %q", err, tt.want)
			}
		})
	}
}

// An unreachable server must be reported as a connection failure. The password
// must not be in the message: this error is logged by the install wizard, and
// the wizard is reachable before the cart exists. The user, database and host
// are in it — pgx names them in its dial errors — and it is the handler that
// strips them before answering a caller; see
// internal/handlers/private.TestDescribeConnectFailure.
func TestConnectPostgresUnreachableServer(t *testing.T) {
	requirePostgres(t)

	_, err := database.Connect(database.Config{
		Driver: database.DriverPostgres,
		DSN:    "postgres://someone:hunter2@127.0.0.1:1/cart?sslmode=disable",
	})
	if err == nil {
		t.Fatal("expected the connection to be refused")
	}
	if strings.Contains(err.Error(), "hunter2") {
		t.Errorf("error %q leaks the password", err)
	}
	if !strings.Contains(err.Error(), "connect to postgres") {
		t.Errorf("error = %q, want it to say what failed", err)
	}
}

func TestPostgresPoolIsSizedFromTheEnvironment(t *testing.T) {
	requirePostgres(t)

	t.Setenv("MYCART_DB_MAX_OPEN_CONNS", "6")
	t.Setenv("MYCART_DB_MAX_IDLE_CONNS", "3")

	conn, err := database.Connect(database.Config{Driver: database.DriverPostgres, DSN: pgtest.MigratedDSN(t)})
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer func() { _ = conn.Close() }()

	if got := conn.Raw().Stats().MaxOpenConnections; got != 6 {
		t.Errorf("max open connections = %d, want 6", got)
	}
}
