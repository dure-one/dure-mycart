package handlers

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"

	"github.com/shurco/mycart/internal/database"
	"github.com/shurco/mycart/internal/queries"
	"github.com/shurco/mycart/internal/testutil"
	"github.com/shurco/mycart/migrations"
)

// envelope mirrors webutil.HTTPResponse without pulling in its result type.
type envelope struct {
	Success bool            `json:"success"`
	Message string          `json:"message"`
	Result  json.RawMessage `json:"result"`
}

// detail is the human-readable part of a response: the message on success,
// the payload on failure, where webutil.StatusBadRequest puts it.
// detail returns the human-readable part of the response. Handlers report
// failures two ways — webutil.StatusBadRequest puts the text in result, while
// webutil.Response puts it in message — so both are checked.
func (e envelope) detail() string {
	if text := strings.Trim(string(e.Result), `"`); text != "" {
		return text
	}
	return strings.Trim(e.Message, `"`)
}

// postJSON sends a request and decodes the standard response envelope.
func postJSON(t *testing.T, app *fiber.App, path, body string) (int, envelope) {
	t.Helper()
	return requestJSON(t, app, http.MethodPost, path, body)
}

// requestJSON sends a request and decodes the standard response envelope.
//
// The body is read before the status is checked: testutil.AssertStatus closes
// it, so the two cannot both be used on one response.
func requestJSON(t *testing.T, app *fiber.App, method, path, body string) (int, envelope) {
	t.Helper()

	resp := testutil.DoRequest(t, app, method, path, body, "")
	defer func() { _ = resp.Body.Close() }()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read response: %v", err)
	}

	var env envelope
	if err := json.Unmarshal(raw, &env); err != nil {
		t.Fatalf("decode response %q: %v", raw, err)
	}
	return resp.StatusCode, env
}

func decodeResult[T any](t *testing.T, env envelope) T {
	t.Helper()

	var out T
	if len(env.Result) == 0 {
		return out
	}
	if err := json.Unmarshal(env.Result, &out); err != nil {
		t.Fatalf("decode result %s: %v", env.Result, err)
	}
	return out
}

// withActiveConfig temporarily replaces the configuration the process reports,
// so the wizard's view of the database can be tested without restarting.
func withActiveConfig(t *testing.T, cfg database.Config) {
	t.Helper()

	previous := database.Active()
	database.SetActive(cfg)
	t.Cleanup(func() { database.SetActive(previous) })
}

func TestInstallStatusReportsDatabase(t *testing.T) {
	app, cleanup := setupCleanDB(t)
	defer cleanup()
	app.Get("/api/install/status", InstallStatus)

	t.Run("default configuration", func(t *testing.T) {
		withActiveConfig(t, database.Config{
			Driver: database.DriverSQLite,
			DSN:    database.DefaultSQLiteDSN,
			Source: database.SourceDefault,
		})

		code, env := requestJSON(t, app, http.MethodGet, "/api/install/status", "")
		testutil.AssertStatusCode(t, code, http.StatusOK)

		status := decodeResult[installStatus](t, env)

		if status.Installed {
			t.Error("a clean database reported as installed")
		}
		if status.Database.Driver != database.DriverSQLite {
			t.Errorf("driver = %q, want sqlite", status.Database.Driver)
		}
		if status.Database.Source != database.SourceDefault {
			t.Errorf("source = %q, want default", status.Database.Source)
		}
		if status.Database.Locked {
			t.Error("the built-in default must not lock the wizard")
		}
	})

	t.Run("pinned configuration is locked and redacted", func(t *testing.T) {
		withActiveConfig(t, database.Config{
			Driver: database.DriverPostgres,
			DSN:    "postgres://mycart:secret@localhost:55432/mycart?sslmode=disable",
			Source: database.SourceFlag,
		})

		code, env := requestJSON(t, app, http.MethodGet, "/api/install/status", "")
		testutil.AssertStatusCode(t, code, http.StatusOK)

		status := decodeResult[installStatus](t, env)

		if !status.Database.Locked {
			t.Error("a database given on the command line must lock the wizard")
		}
		if !strings.Contains(status.Database.DSN, "***") {
			t.Errorf("dsn %q is not redacted", status.Database.DSN)
		}
		if strings.Contains(status.Database.DSN, "secret") {
			t.Errorf("dsn %q leaks the password", status.Database.DSN)
		}
	})
}

func TestInstallDBTest(t *testing.T) {
	app, cleanup := setupCleanDB(t)
	defer cleanup()

	app.Post("/api/install/db/test", InstallDBTest)
	app.Post("/api/install", Install)

	// The configuration the process runs on is whatever the last test left
	// behind; pin it to the default so the wizard is not "locked" here.
	withActiveConfig(t, database.Config{
		Driver: database.DriverSQLite,
		DSN:    database.DefaultSQLiteDSN,
		Source: database.SourceDefault,
	})

	t.Run("sqlite file is created on demand", func(t *testing.T) {
		assertFreshSQLiteIsCreatedOnDemand(t, app)
	})

	t.Run("existing schema is reported", func(t *testing.T) {
		migrated := filepath.Join(t.TempDir(), "migrated.db")
		conn, err := database.Open(database.Config{Driver: database.DriverSQLite, DSN: migrated}, migrations.Embed())
		if err != nil {
			t.Fatalf("prepare migrated database: %v", err)
		}
		_ = conn.Close()

		code, env := postJSON(t, app, "/api/install/db/test",
			`{"driver":"sqlite","dsn":"`+migrated+`"}`)
		if code != http.StatusOK {
			t.Fatalf("status = %d, body %s", code, env.detail())
		}
		if result := decodeResult[installDatabaseTestResult](t, env); !result.HasExistingSchema {
			t.Error("has_existing_schema = false for a migrated database")
		}
	})

	t.Run("unknown driver", func(t *testing.T) {
		code, env := postJSON(t, app, "/api/install/db/test", `{"driver":"mysql"}`)
		if code != http.StatusBadRequest {
			t.Fatalf("status = %d, want 400", code)
		}
		if !strings.Contains(env.detail(), "driver") {
			t.Errorf("message %q does not mention the driver", env.detail())
		}
	})

	t.Run("postgres without a connection string", func(t *testing.T) {
		code, env := postJSON(t, app, "/api/install/db/test", `{"driver":"postgres"}`)
		if code != http.StatusBadRequest {
			t.Fatalf("status = %d, want 400", code)
		}
		if !strings.Contains(env.detail(), "connection string is required") {
			t.Errorf("message = %q", env.detail())
		}
	})

	t.Run("unreachable server is reported without echoing the address", func(t *testing.T) {
		assertUnreachableServerIsRedacted(t, app)
	})

	t.Run("refused once installed", func(t *testing.T) {
		code, env := postJSON(t, app, "/api/install",
			`{"email":"admin@example.com","password":"secret","domain":"example.com"}`)
		if code != http.StatusOK {
			t.Fatalf("install status = %d, body %s", code, env.detail())
		}

		code, env = postJSON(t, app, "/api/install/db/test", `{}`)
		if code != http.StatusBadRequest {
			t.Fatalf("status = %d, want 400", code)
		}
		if !strings.Contains(env.detail(), "already installed") {
			t.Errorf("message = %q", env.detail())
		}
	})
}

// assertFreshSQLiteIsCreatedOnDemand drives the wizard's test button at a file
// that does not exist yet: the probe has to create it, and to report that there
// is no schema in it.
func assertFreshSQLiteIsCreatedOnDemand(t *testing.T, app *fiber.App) {
	t.Helper()

	fresh := filepath.Join(t.TempDir(), "fresh.db")
	code, env := postJSON(t, app, "/api/install/db/test",
		`{"driver":"sqlite","dsn":"`+fresh+`"}`)
	if code != http.StatusOK {
		t.Fatalf("status = %d, body %s", code, env.detail())
	}
	result := decodeResult[installDatabaseTestResult](t, env)
	if !result.OK {
		t.Error("ok = false")
	}
	if result.Driver != database.DriverSQLite {
		t.Errorf("driver = %q, want sqlite", result.Driver)
	}
	// Nothing is installed there yet, which is what the wizard warns about.
	if result.HasExistingSchema {
		t.Error("has_existing_schema = true for a database that did not exist")
	}
}

// assertUnreachableServerIsRedacted points the wizard at a server that is not
// there and checks the answer: it has to say the server could not be reached
// without echoing back any part of the connection string, which the wizard
// would then show to an unauthenticated caller.
func assertUnreachableServerIsRedacted(t *testing.T, app *fiber.App) {
	t.Helper()

	code, env := postJSON(t, app, "/api/install/db/test",
		`{"driver":"postgres","dsn":"postgres://someone:hunter2@127.0.0.1:1/cart?sslmode=disable"}`)
	if code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", code)
	}
	detail := env.detail()
	if !strings.Contains(detail, "not reachable") {
		t.Errorf("message = %q", detail)
	}
	for _, secret := range []string{"hunter2", "someone", "127.0.0.1", "/cart"} {
		if strings.Contains(detail, secret) {
			t.Errorf("message %q leaks %q to an unauthenticated caller", detail, secret)
		}
	}
}

// TestInstallDBTestPostgres exercises the test-connection button against a real
// server, which is the only way to know that pgx's errors are mapped to
// something an operator can act on.
func TestInstallDBTestPostgres(t *testing.T) {
	dsn := os.Getenv(testutil.EnvTestDSN)
	if dsn == "" {
		t.Skipf("set %s to test the connection probe against PostgreSQL", testutil.EnvTestDSN)
	}

	app, cleanup := setupCleanDB(t)
	defer cleanup()
	app.Post("/api/install/db/test", InstallDBTest)

	withActiveConfig(t, database.Config{
		Driver: database.DriverSQLite,
		DSN:    database.DefaultSQLiteDSN,
		Source: database.SourceDefault,
	})

	body, err := json.Marshal(installDatabaseTest{Driver: database.DriverPostgres, DSN: dsn})
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}

	code, env := postJSON(t, app, "/api/install/db/test", string(body))
	if code != http.StatusOK {
		t.Fatalf("status = %d, body %s", code, env.detail())
	}

	result := decodeResult[installDatabaseTestResult](t, env)
	if !result.OK || result.Driver != database.DriverPostgres {
		t.Errorf("unexpected result: %+v", result)
	}
	if !strings.Contains(result.ServerVersion, "PostgreSQL") {
		t.Errorf("server_version = %q", result.ServerVersion)
	}
}

func TestInstallRejectsSwitchWhenDatabaseIsPinned(t *testing.T) {
	app, cleanup := setupCleanDB(t)
	defer cleanup()
	app.Post("/api/install", Install)

	withActiveConfig(t, database.Config{
		Driver: database.DriverSQLite,
		DSN:    database.DefaultSQLiteDSN,
		Source: database.SourceFlag,
	})

	target := filepath.Join(t.TempDir(), "elsewhere.db")
	body := `{"email":"admin@example.com","password":"secret","domain":"example.com","database":{"driver":"sqlite","dsn":"` + target + `"}}`

	code, env := postJSON(t, app, "/api/install", body)
	if code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 (body %s)", code, env.detail())
	}
	if !strings.Contains(env.detail(), "fixed by the flag") {
		t.Errorf("message = %q", env.detail())
	}
	if _, err := os.Stat(target); err == nil {
		t.Error("a pinned installation still created the other database")
	}
}

// TestInstallSwitchesDatabase drives a switch from the wizard's point of view:
// install into a database the process did not start on, and check that the
// choice is both live and recorded.
func TestInstallSwitchesDatabase(t *testing.T) {
	app, cleanup := setupCleanDB(t)
	defer cleanup()
	app.Post("/api/install", Install)
	app.Get("/api/install/status", InstallStatus)

	withActiveConfig(t, database.Config{
		Driver: database.DriverSQLite,
		DSN:    database.DefaultSQLiteDSN,
		Source: database.SourceDefault,
	})

	target := filepath.Join(t.TempDir(), "chosen.db")
	body := `{"email":"wizard@example.com","password":"secret","domain":"example.com","database":{"driver":"sqlite","dsn":"` + target + `"}}`

	code, env := postJSON(t, app, "/api/install", body)
	if code != http.StatusOK {
		t.Fatalf("status = %d, body %s", code, env.detail())
	}

	// The running process now uses the chosen database.
	active := database.Active()
	if active.DSN != target || active.Driver != database.DriverSQLite {
		t.Fatalf("active config is %+v, want the chosen database", active)
	}
	if active.Source != database.SourceWizard {
		t.Errorf("source = %q, want %q", active.Source, database.SourceWizard)
	}

	// And the choice survives a restart: it is in config.json.
	// The test's working directory is the installation directory.
	if _, err := os.Stat(filepath.Join(database.DirBase, "config.json")); err != nil {
		t.Fatalf("config.json was not written: %v", err)
	}
	resolved, err := database.Resolve(database.Overrides{})
	if err != nil {
		t.Fatalf("resolve configuration: %v", err)
	}
	if resolved.DSN != target || resolved.Source != database.SourceFile {
		t.Errorf("resolved %+v, want the wizard's choice from the file", resolved)
	}

	// The installation really landed in the chosen database.
	conn, err := database.Connect(database.Config{Driver: database.DriverSQLite, DSN: target})
	if err != nil {
		t.Fatalf("connect to chosen database: %v", err)
	}
	defer func() { _ = conn.Close() }()

	installed, err := queries.NewBase(conn).IsInstalled(context.Background())
	if err != nil {
		t.Fatalf("check installed: %v", err)
	}
	if !installed {
		t.Error("the chosen database is not installed")
	}

	// A second install must now be refused by the new database, which is what
	// proves the switch took effect for queries as well.
	code, env = postJSON(t, app, "/api/install", body)
	if code != http.StatusBadRequest {
		t.Fatalf("second install status = %d, want 400 (body %s)", code, env.detail())
	}
	if !strings.Contains(env.detail(), "already installed") {
		t.Errorf("message = %q", env.detail())
	}
}
