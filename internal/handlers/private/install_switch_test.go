package handlers

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shurco/mycart/internal/database"
	"github.com/shurco/mycart/internal/models"
	"github.com/shurco/mycart/internal/queries"
	"github.com/shurco/mycart/migrations"
)

// installBody builds the wizard's payload for a database selection.
func installBody(driver, dsn string) string {
	return `{"email":"wizard@example.com","password":"secret","domain":"example.com",` +
		`"database":{"driver":"` + driver + `","dsn":"` + dsn + `"}}`
}

// A switch that cannot reach the chosen database must fail with a 4xx the
// operator can act on, and must leave the running installation alone.
func TestInstallSwitchToUnreachableDatabase(t *testing.T) {
	app, cleanup := setupCleanDB(t)
	defer cleanup()
	app.Post("/api/install", Install)

	current := database.Config{Driver: database.DriverSQLite, DSN: database.DefaultSQLiteDSN, Source: database.SourceDefault}
	withActiveConfig(t, current)

	code, env := postJSON(t, app, "/api/install",
		installBody(database.DriverPostgres, "postgres://someone:hunter2@127.0.0.1:1/cart?sslmode=disable"))
	if code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 (body %s)", code, env.detail())
	}
	if !strings.Contains(env.detail(), "connect to the selected database") {
		t.Errorf("message = %q", env.detail())
	}
	// The wizard calls this before anyone has authenticated, so nothing about
	// the server it tried to reach may come back.
	for _, secret := range []string{"hunter2", "someone", "127.0.0.1", "/cart"} {
		if strings.Contains(env.detail(), secret) {
			t.Errorf("message %q leaks %q to an unauthenticated caller", env.detail(), secret)
		}
	}

	// The process is still on the database it started on.
	if got := database.Active(); got != current {
		t.Errorf("active configuration changed to %+v", got)
	}
	// Nothing was written, so a restart would not pick up the failed choice.
	if _, err := os.Stat(filepath.Join(database.DirBase, "config.json")); err == nil {
		t.Error("config.json was written for a database that could not be reached")
	}
}

// A cart that is already installed somewhere else must not be overwritten by a
// second installation.
func TestInstallSwitchToOccupiedDatabase(t *testing.T) {
	app, cleanup := setupCleanDB(t)
	defer cleanup()
	app.Post("/api/install", Install)

	withActiveConfig(t, database.Config{
		Driver: database.DriverSQLite,
		DSN:    database.DefaultSQLiteDSN,
		Source: database.SourceDefault,
	})

	// A second database that already holds an installed cart.
	occupied := filepath.Join(t.TempDir(), "occupied.db")
	seedInstalled(t, occupied)

	code, env := postJSON(t, app, "/api/install", installBody(database.DriverSQLite, occupied))
	if code != http.StatusConflict {
		t.Fatalf("status = %d, want 409 (body %s)", code, env.detail())
	}
	if !strings.Contains(env.detail(), "already holds an installed cart") {
		t.Errorf("message = %q", env.detail())
	}
}

// The order in installInto is deliberate: the new database is prepared first and
// config.json is written last, so a failure while writing it has to put the
// process back on the database it was already using.
func TestInstallSwitchRollsBackWhenTheChoiceCannotBePersisted(t *testing.T) {
	app, cleanup := setupCleanDB(t)
	defer cleanup()
	app.Post("/api/install", Install)

	current := database.Config{
		Driver: database.DriverSQLite,
		DSN:    filepath.Join(t.TempDir(), "current.db"),
		Source: database.SourceDefault,
	}
	withActiveConfig(t, current)

	// The test's working directory is the installation directory. Making
	// `lc_base` a file rather than a directory is the realistic way the
	// configuration cannot be written.
	if err := os.WriteFile(database.DirBase, []byte("in the way"), 0o600); err != nil {
		t.Fatalf("block %s: %v", database.DirBase, err)
	}

	target := filepath.Join(t.TempDir(), "chosen.db")
	code, env := postJSON(t, app, "/api/install", installBody(database.DriverSQLite, target))
	if code == http.StatusOK {
		t.Fatalf("install unexpectedly succeeded: %s", env.detail())
	}

	if got := database.Active(); got != current {
		t.Errorf("active configuration = %+v, want the original %+v", got, current)
	}
	// The connection the process was already using has to still work, which is
	// what the rollback exists for.
	if _, err := queries.DB().IsInstalled(t.Context()); err != nil {
		t.Errorf("the original database is no longer usable: %v", err)
	}
}

// A switch is only for first-time setup. The endpoint is reachable without a
// session, so once the running cart holds an installation an anonymous request
// must not be able to move the process to a database of its own choosing — and
// must not leave that choice in config.json for the next restart.
func TestInstallSwitchRefusedOnceInstalled(t *testing.T) {
	app, cleanup := setupCleanDB(t)
	defer cleanup()
	app.Post("/api/install", Install)

	// First-time setup of the running database, exactly as the wizard does it
	// when it has no database of its own to send.
	code, env := postJSON(t, app, "/api/install",
		`{"email":"owner@example.com","password":"secret","domain":"example.com"}`)
	if code != http.StatusOK {
		t.Fatalf("install status = %d, body %s", code, env.detail())
	}

	target := filepath.Join(t.TempDir(), "attacker.db")
	code, env = postJSON(t, app, "/api/install", installBody(database.DriverSQLite, target))
	if code != http.StatusBadRequest {
		t.Fatalf("switch status = %d, want 400 (body %s)", code, env.detail())
	}
	if !strings.Contains(env.detail(), "already installed") {
		t.Errorf("message = %q", env.detail())
	}

	// Nothing was created and nothing was persisted: the refused request had no
	// effect beyond being refused.
	if _, err := os.Stat(target); err == nil {
		t.Error("the requested database was created")
	}
	if _, err := os.Stat(filepath.Join(database.DirBase, "config.json")); err == nil {
		t.Error("config.json was written by a refused switch")
	}
}

// seedInstalled performs a real installation into a fresh SQLite file.
func seedInstalled(t *testing.T, path string) {
	t.Helper()

	conn, err := database.Open(database.Config{Driver: database.DriverSQLite, DSN: path}, migrations.Embed())
	if err != nil {
		t.Fatalf("open %s: %v", path, err)
	}
	defer func() { _ = conn.Close() }()

	if err := queries.NewBase(conn).Install(t.Context(), &models.Install{
		Email:    "occupied@example.com",
		Password: "secret",
		Domain:   "example.com",
	}); err != nil {
		t.Fatalf("install into %s: %v", path, err)
	}
}
