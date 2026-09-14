package testutil

import (
	"context"
	"database/sql"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/pressly/goose/v3"
	goosedb "github.com/pressly/goose/v3/database"

	"github.com/shurco/mycart/internal/database"
	"github.com/shurco/mycart/internal/queries"
	"github.com/shurco/mycart/internal/testutil/pgtest"
	"github.com/shurco/mycart/migrations"
	"github.com/shurco/mycart/pkg/jwtutil"
)

const (
	FixtureJWTSecret = "d58ca30c8e5ca96695451fa27af949d9"
	FixtureEmail     = "user@mail.com"
	FixturePassword  = "Pass123"
)

// Environment variables selecting the dialect the suite runs against. When
// neither is set, tests run on in-memory SQLite, which is what the project has
// always done.
const (
	EnvTestDriver = "TEST_DB_DRIVER"
	// EnvTestDSN names the PostgreSQL test server. It is the same variable
	// pgtestdb administers that server through — see pgtest.AdminDSN.
	EnvTestDSN = pgtest.AdminDSN
)

// TestingPostgres reports whether the suite is configured to run against
// PostgreSQL. Tests that only make sense there use it to skip otherwise.
func TestingPostgres() bool {
	return strings.EqualFold(os.Getenv(EnvTestDriver), database.DriverPostgres)
}

// SetupTestDB creates a migrated, fixture-loaded database and installs it as
// the process-wide handle. Returns the cleanup function.
//
// SQLite gets a fresh in-memory database. PostgreSQL gets a database of its own
// on the server named by TEST_POSTGRES_DSN, cloned by pgtestdb from a template
// carrying the schema and the fixtures — see internal/testutil/pgtest. Tests
// never see each other's rows on either engine.
func SetupTestDB(t *testing.T) func() {
	t.Helper()
	return setupDB(t, true)
}

// SetupCleanDB is SetupTestDB without the fixtures: a migrated but otherwise
// empty database, for tests that exercise first-run behaviour.
func SetupCleanDB(t *testing.T) func() {
	t.Helper()
	return setupDB(t, false)
}

// setupDB creates the per-test database, loads the fixtures when asked for and
// installs it as the process-wide handle.
func setupDB(t *testing.T, withFixtures bool) func() {
	t.Helper()

	dirCleanup := WithCmdTestDir(t)
	conn, cfg, dbCleanup := openTestDB(t, withFixtures)

	queries.SetConn(conn)
	// Production sets both when it connects (queries.New). Leaving the active
	// configuration at the built-in default would make the install wizard
	// describe a database the test is not talking to.
	database.SetActive(cfg)

	return func() {
		dbCleanup()
		dirCleanup()
	}
}

// openTestDB returns the per-test database — freshly migrated, and carrying the
// fixtures when they were asked for — and the configuration it is reachable
// through.
func openTestDB(t *testing.T, withFixtures bool) (*database.Conn, database.Config, func()) {
	t.Helper()

	if TestingPostgres() {
		return openTestPostgres(t, withFixtures)
	}
	return openTestSQLite(t, withFixtures)
}

// testSQLiteDSN is the in-memory database the SQLite half of the suite runs
// against. The pragma is part of the DSN because a second connection would see
// an empty database otherwise.
const testSQLiteDSN = ":memory:?_pragma=foreign_keys(ON)"

// openTestSQLite opens an in-memory SQLite database with the schema — and, when
// asked for, the fixtures — applied.
func openTestSQLite(t *testing.T, withFixtures bool) (*database.Conn, database.Config, func()) {
	t.Helper()

	raw, err := sql.Open("sqlite", testSQLiteDSN)
	if err != nil {
		t.Fatalf("open in-memory sqlite: %v", err)
	}
	// A single connection keeps the in-memory database alive for the lifetime
	// of the pool: a second connection would see an empty database.
	raw.SetMaxOpenConns(1)

	conn := database.Wrap(raw, database.SQLite())
	if err := database.MigrateOn(conn, migrations.Embed()); err != nil {
		t.Fatalf("run schema migrations: %v", err)
	}
	if withFixtures {
		applyFixtures(t, conn)
	}

	cfg := database.Config{Driver: database.DriverSQLite, DSN: testSQLiteDSN, Source: database.SourceDefault}
	return conn, cfg, func() { _ = raw.Close() }
}

// openTestPostgres returns a fresh database on the PostgreSQL test server,
// provisioned by pgtestdb. The template it clones from already holds the schema
// (and, when asked for, the fixtures), so nothing is migrated here.
func openTestPostgres(t *testing.T, withFixtures bool) (*database.Conn, database.Config, func()) {
	t.Helper()

	dsn := pgtest.MigratedDSN(t)
	if withFixtures {
		dsn = pgtest.FixturesDSN(t)
	}

	// Connect, not Open: the database is migrated already, and this is the
	// production path that pins the session timezone and sizes the pool.
	cfg := database.Config{Driver: database.DriverPostgres, DSN: dsn, Source: database.SourceDefault}

	conn, err := database.Connect(cfg)
	if err != nil {
		t.Fatalf("connect to postgres test database: %v", err)
	}

	return conn, cfg, func() { _ = conn.Close() }
}

// applyFixtures loads fixtures/migration into an already migrated SQLite
// database.
//
// PostgreSQL does not come through here: the template it clones from already
// contains the fixtures, so the fixture script runs once for the whole suite
// instead of once per test.
func applyFixtures(t *testing.T, conn *database.Conn) {
	t.Helper()

	store, err := goosedb.NewStore(goosedb.Dialect(conn.Dialect().GooseDialect()), pgtest.FixturesTable)
	if err != nil {
		t.Fatalf("fixture store: %v", err)
	}
	// An explicit store keeps the package-level goose state (SetBaseFS,
	// SetTableName, SetDialect) out of this: it is global, and tests in a
	// package run in parallel.
	provider, err := goose.NewProvider("", conn.Raw(), pgtest.FixturesFS(),
		goose.WithStore(store),
		goose.WithDisableGlobalRegistry(true))
	if err != nil {
		t.Fatalf("fixture provider: %v", err)
	}
	if _, err := provider.Up(t.Context()); err != nil {
		t.Fatalf("run fixtures: %v", err)
	}
}

// SetupTestApp creates a database with fixtures, a Fiber app, and a JWT cookie.
// Fixtures already contain installed state + JWT secret. A matching session
// row is created so the token passes the middleware revocation check.
func SetupTestApp(t *testing.T) (app *fiber.App, cookie string, cleanup func()) {
	t.Helper()

	dbCleanup := SetupTestDB(t)
	app = fiber.New()

	exp := time.Now().Add(time.Hour).Unix()
	tok, err := jwtutil.GenerateNewToken(FixtureJWTSecret, "test-user-id", exp, nil)
	if err != nil {
		t.Fatalf("generate jwt: %v", err)
	}

	if err := queries.DB().AddSession(context.Background(), "test-user-id",
		queries.SessionValue(queries.SessionRoleAdmin, ""), exp); err != nil {
		t.Fatalf("add session: %v", err)
	}

	return app, "token=" + tok, func() {
		_ = app.Shutdown()
		dbCleanup()
	}
}

// DoRequest is a DRY helper for table-driven HTTP handler tests.
//
// The internal Fiber `app.Test` default timeout is 1 second. That is not
// enough when a handler runs bcrypt at DefaultCost (≈70 ms on modern
// hardware, ×2 for install → token generation) and the test binary is built
// with -race (another ~5× slowdown). We use a 10 s ceiling so the timeout
// continues to surface genuine hangs while tolerating realistic hashing work.
func DoRequest(t *testing.T, app *fiber.App, method, path, body, cookie string) *http.Response {
	t.Helper()

	var reader *strings.Reader
	if body != "" {
		reader = strings.NewReader(body)
	} else {
		reader = strings.NewReader("")
	}

	req := httptest.NewRequest(method, path, reader)
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	if cookie != "" {
		req.Header.Set("Cookie", cookie)
	}

	resp, err := app.Test(req, fiber.TestConfig{Timeout: 10 * time.Second})
	if err != nil {
		t.Fatalf("%s %s: %v", method, path, err)
	}
	return resp
}

// AssertStatus checks HTTP status and closes body.
func AssertStatus(t *testing.T, resp *http.Response, want ...int) {
	t.Helper()
	defer func() { _ = resp.Body.Close() }()

	for _, w := range want {
		if resp.StatusCode == w {
			return
		}
	}
	t.Errorf("status = %d, want one of %v", resp.StatusCode, want)
}

// AssertStatusCode checks a status code that was read before the body was
// consumed (e.g. via io.ReadAll).
func AssertStatusCode(t *testing.T, got, want int) {
	t.Helper()
	if got != want {
		t.Errorf("status = %d, want %d", got, want)
	}
}
