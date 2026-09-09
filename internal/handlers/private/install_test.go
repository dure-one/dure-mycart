package handlers

import (
	"net/http"
	"os"
	"testing"

	"github.com/gofiber/fiber/v3"

	"github.com/shurco/mycart/db/migrations"
	"github.com/shurco/mycart/internal/store"
	"github.com/shurco/mycart/internal/store/db"
	"github.com/shurco/mycart/internal/testutil"
)

func setupCleanDB(t *testing.T) (*fiber.App, func()) {
	t.Helper()
	dirCleanup := testutil.WithCmdTestDir(t)

	// Set up environment for SQLite
	os.Setenv("DB_TYPE", "sqlite")
	os.Setenv("SQLITE_PATH", ":memory:")

	// Initialize database with new pattern
	if err := queries.New(migrations.Embed()); err != nil {
		t.Fatal(err)
	}

	// Initialize store layer
	if err := db.Init(queries.Adapter().DB(), "sqlite"); err != nil {
		t.Fatal(err)
	}
	store.InitStore(queries.Adapter().DB())

	app := fiber.New()

	return app, func() {
		_ = app.Shutdown()
		queries.Close()
		dirCleanup()
	}
}

func TestInstall(t *testing.T) {
	app, cleanup := setupCleanDB(t)
	defer cleanup()

	app.Post("/api/install", Install)

	tests := []struct {
		name       string
		body       string
		wantStatus int
	}{
		{
			"invalid email",
			`{"email":"bad","password":"secret","domain":"example.com"}`,
			http.StatusBadRequest,
		},
		{
			"short password",
			`{"email":"admin@example.com","password":"12","domain":"example.com"}`,
			http.StatusBadRequest,
		},
		{
			"empty body",
			`{}`,
			http.StatusBadRequest,
		},
		{
			"valid install (last — mutates DB)",
			`{"email":"admin@example.com","password":"secret","domain":"example.com"}`,
			http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := testutil.DoRequest(t, app, http.MethodPost, "/api/install", tt.body, "")
			testutil.AssertStatus(t, resp, tt.wantStatus)
		})
	}
}
