package testutil

import (
	"context"
	"database/sql"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/pressly/goose/v3"

	"github.com/shurco/mycart/db/migrations"
	"github.com/shurco/mycart/internal/store"
	"github.com/shurco/mycart/internal/store/db"
	"github.com/shurco/mycart/pkg/jwtutil"
	_ "modernc.org/sqlite"
)

const (
	FixtureJWTSecret = "d58ca30c8e5ca96695451fa27af949d9"
	FixtureEmail     = "user@mail.com"
	FixturePassword  = "Pass123"
)

// projectRoot returns the absolute path to the repository root,
// derived from this source file's location at compile time.
func projectRoot() string {
	_, src, _, _ := runtime.Caller(0)
	// src = <root>/internal/testutil/testdb.go → root = ../../
	return filepath.Join(filepath.Dir(src), "..", "..")
}

// SetupTestDB creates in-memory SQLite, runs schema migrations + fixtures,
// sets queries.DB global. Returns cleanup function.
func SetupTestDB(t *testing.T) func() {
	t.Helper()

	dirCleanup := WithCmdTestDir(t)

	// Use file::memory:?mode=memory&cache=shared to share in-memory DB across connections
	// Plain :memory: creates separate DB per connection!
	sqlite, err := sql.Open("sqlite", "file::memory:?mode=memory&cache=shared&_pragma=foreign_keys(ON)")
	if err != nil {
		t.Fatalf("open in-memory sqlite: %v", err)
	}
	// Allow multiple connections to prevent deadlock with nested queries
	sqlite.SetMaxOpenConns(5)

	if err := goose.SetDialect("sqlite3"); err != nil {
		t.Fatalf("set goose dialect: %v", err)
	}

	// Extract sqlite subdirectory from embedded filesystem
	migrationsSubFS, err := fs.Sub(migrations.Embed(), "sqlite")
	if err != nil {
		t.Fatalf("access sqlite migrations: %v", err)
	}

	goose.SetBaseFS(migrationsSubFS)
	goose.SetTableName("migrate_db_version")
	if err := goose.Up(sqlite, "."); err != nil {
		t.Fatalf("run schema migrations: %v", err)
	}

	fixturesDir := filepath.Join(projectRoot(), "fixtures")
	goose.SetBaseFS(os.DirFS(fixturesDir))
	goose.SetTableName("migrate_fixtures_version")
	if err := goose.Up(sqlite, "migration"); err != nil {
		t.Fatalf("run fixtures: %v", err)
	}

	store.InitStore(sqlite)

	if err := db.InitFromDB(sqlite, "sqlite"); err != nil {
		t.Fatalf("init store function pointers: %v", err)
	}

	return func() {
		_ = sqlite.Close()
		dirCleanup()
	}
}

// SetupTestApp creates in-memory DB with fixtures, Fiber app, and JWT cookie.
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

	if err := store.AddSession(context.Background(), "test-user-id", "admin", exp); err != nil {
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
