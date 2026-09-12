package handlers

import (
	"net/http"
	"testing"

	"github.com/gofiber/fiber/v3"

	"github.com/shurco/mycart/internal/testutil"
)

// setupCleanDB returns a Fiber app backed by a migrated but uninstalled
// database, so the install endpoint starts from first-run state.
func setupCleanDB(t *testing.T) (*fiber.App, func()) {
	t.Helper()

	dbCleanup := testutil.SetupCleanDB(t)
	app := fiber.New()

	return app, func() {
		_ = app.Shutdown()
		dbCleanup()
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
