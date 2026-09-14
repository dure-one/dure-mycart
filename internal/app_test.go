package app

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"

	"github.com/shurco/mycart/internal/database"
	"github.com/shurco/mycart/internal/queries"
	"github.com/shurco/mycart/migrations"
	"github.com/shurco/mycart/pkg/logging"
)

func TestDetermineSchemaAndAddr(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		httpAddr     string
		httpsAddr    string
		wantSchema   string
		wantMainAddr string
	}{
		{"https wins", ":80", ":443", "https", ":443"},
		{"http fallback", ":80", "", "http", ":80"},
		{"empty http", "", "", "http", ""},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			schema, addr := determineSchemaAndAddr(tt.httpAddr, tt.httpsAddr)
			if schema != tt.wantSchema || addr != tt.wantMainAddr {
				t.Errorf("got (%s,%s), want (%s,%s)",
					schema, addr, tt.wantSchema, tt.wantMainAddr)
			}
		})
	}
}

func TestExtractHostOnly(t *testing.T) {
	t.Parallel()
	tests := []struct {
		in, want string
	}{
		{"example.com", "example.com"},
		{"example.com:443", "example.com"},
		{":443", ""},
		{"[::1]:443", "::1"},
		{"invalid:port:thing", "invalid:port:thing"},
	}
	for _, tc := range tests {
		if got := extractHostOnly(tc.in); got != tc.want {
			t.Errorf("extractHostOnly(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestIsInstallPath(t *testing.T) {
	t.Parallel()
	tests := []struct {
		path string
		want bool
	}{
		{"/_/install", true},
		{"/_/install/step1", true},
		{"/_/assets/logo.png", true},
		{"/_/_app/chunk.js", true},
		{"/_app/chunk.js", true},
		{"/api/install", true},
		{"/api/install/status", true},
		{"/api/settings", true},
		{"/api/products", true},
		{"/api/_/version", false},
		{"/api/sign/in", false},
		{"/uploads/1.png", true},
		{"/", false},
		{"/random", false},
		{"/_/", false},
	}
	for _, tc := range tests {
		if got := isInstallPath(tc.path); got != tc.want {
			t.Errorf("isInstallPath(%q) = %v, want %v", tc.path, got, tc.want)
		}
	}
}

func TestSetupFiberApp_BuildsAppWithLimits(t *testing.T) {
	// log is a package-level pointer referenced by middleware.Fiber.
	setLogger(logging.New())

	app, err := setupFiberApp(false)
	if err != nil {
		t.Fatalf("setupFiberApp: %v", err)
	}
	if app == nil {
		t.Fatal("nil app returned")
	}
	t.Cleanup(func() { _ = app.Shutdown() })
}

func TestInstallCheck_RedirectsWhenNotInstalled(t *testing.T) {
	// Build a fresh DB in a temp CWD so the global queries.DB() is populated
	// with an uninitialised (installed='') row.
	prev, _ := os.Getwd()
	tmp := t.TempDir()
	if err := os.Chdir(tmp); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	_ = os.MkdirAll("lc_base", 0o775)
	t.Cleanup(func() { _ = os.Chdir(prev) })
	if err := queries.New(database.Config{Driver: database.DriverSQLite, DSN: database.DefaultSQLiteDSN}, migrations.Embed()); err != nil {
		t.Fatalf("queries.New: %v", err)
	}

	app := fiber.New()
	app.Get("/dashboard", InstallCheck, func(c fiber.Ctx) error { return c.SendStatus(http.StatusOK) })
	app.Get("/_/install", InstallCheck, func(c fiber.Ctx) error { return c.SendStatus(http.StatusOK) })

	// An unrelated URL should be redirected to /_/install.
	req := httptest.NewRequest(http.MethodGet, "/dashboard", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	if resp.StatusCode != http.StatusSeeOther && resp.StatusCode != http.StatusFound {
		t.Errorf("expected redirect, got %d", resp.StatusCode)
	}

	// Install paths should pass the guard.
	req = httptest.NewRequest(http.MethodGet, "/_/install", nil)
	resp, err = app.Test(req)
	if err != nil {
		t.Fatalf("app.Test install: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("install path got %d", resp.StatusCode)
	}
}

func TestStartServer_RespectsContextCancel(t *testing.T) {
	setLogger(logging.New())
	app := fiber.New()
	app.Get("/ping", func(c fiber.Ctx) error { return c.SendString("pong") })

	// StartServer takes an address rather than a listener, so the address has to
	// be free before it is handed over.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	addr := ln.Addr().String()
	_ = ln.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan error, 1)
	go func() { done <- StartServer(ctx, addr, app) }()

	// Wait for the server to answer before cancelling. Handing StartServer a
	// context that is already cancelled would have Shutdown find no listener to
	// close yet, so Listen would create one afterwards and serve on it for the
	// rest of the test binary — printing Fiber's banner to stdout at a moment of
	// its own choosing, which lands in the middle of another test's capture of
	// it. Waiting for the first answer proves Listen is past its banner already.
	waitForServer(t, "http://"+addr+"/ping")
	cancel()

	select {
	case err := <-done:
		// a.Shutdown returning nil is the expected path.
		if err != nil {
			t.Logf("StartServer exit: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("StartServer did not return after the context was cancelled")
	}
}

func TestInit_CreatesDirsAndDB(t *testing.T) {
	prev, _ := os.Getwd()
	tmp := t.TempDir()
	if err := os.Chdir(tmp); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(prev) })

	if err := Init(database.Config{Driver: database.DriverSQLite, DSN: database.DefaultSQLiteDSN}); err != nil {
		t.Fatalf("Init: %v", err)
	}
	for _, d := range []string{"lc_uploads", "lc_digitals", "lc_base"} {
		if fi, err := os.Stat(d); err != nil || !fi.IsDir() {
			t.Errorf("expected dir %q: %v", d, err)
		}
	}
}

func TestAppStartup_NotInstalled(t *testing.T) {
	// Setup: empty in-memory database
	t.Setenv("DB_TYPE", "sqlite")
	t.Setenv("SQLITE_PATH", ":memory:")

	// Connect and verify not installed
	err := db.Connect()
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}

	installed, err := db.IsInstalled()
	if err != nil {
		t.Fatalf("IsInstalled: %v", err)
	}
	if installed {
		t.Error("fresh database should not be installed")
	}

	// Verify installRequired flag is set
	if !db.InstallRequired() {
		t.Error("installRequired should be true for fresh database")
	}

	t.Cleanup(func() {
		db.Close()
	})
}

func TestAppStartup_Installed(t *testing.T) {
	// Setup: database with migrations
	t.Setenv("DB_TYPE", "sqlite")

	// Use temp file for this test
	tmpFile, err := os.CreateTemp("", "test-installed-*.db")
	if err != nil {
		t.Fatalf("CreateTemp: %v", err)
	}
	tmpPath := tmpFile.Name()
	tmpFile.Close()
	defer os.Remove(tmpPath)

	t.Setenv("SQLITE_PATH", tmpPath)

	// First: connect and migrate to simulate installed state
	err = db.Connect()
	if err != nil {
		t.Fatalf("Connect (first): %v", err)
	}

	err = db.Migrate(migrations.Embed())
	if err != nil {
		t.Fatalf("Migrate: %v", err)
	}

	// Close and reset
	db.Close()

	// Second: reconnect as if app is restarting
	err = db.Connect()
	if err != nil {
		t.Fatalf("Connect (second): %v", err)
	}

	installed, err := db.IsInstalled()
	if err != nil {
		t.Fatalf("IsInstalled: %v", err)
	}
	if !installed {
		t.Error("database with migrations should be installed")
	}

	// Verify installRequired flag is not set
	if db.InstallRequired() {
		t.Error("installRequired should be false for installed database")
	}

	t.Cleanup(func() {
		db.Close()
	})
}
