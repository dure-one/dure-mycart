package handlers

import (
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

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
			`{"email":"bad","password":"secret","domain":"example.com","dbType":"sqlite","sqlitePath":"lc_base/data.db"}`,
			http.StatusBadRequest,
		},
		{
			"short password",
			`{"email":"admin@example.com","password":"12","domain":"example.com","dbType":"sqlite","sqlitePath":"lc_base/data.db"}`,
			http.StatusBadRequest,
		},
		{
			"empty body",
			`{}`,
			http.StatusBadRequest,
		},
		{
			"valid install (last — mutates DB)",
			`{"email":"admin@example.com","password":"secret","domain":"example.com","dbType":"sqlite","sqlitePath":"lc_base/data.db"}`,
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

func TestInstall_WithDatabaseConfig(t *testing.T) {
	tests := []struct {
		name     string
		payload  string
		wantCode int
	}{
		{
			name: "successful installation with sqlite",
			payload: `{
				"email": "admin@example.com",
				"password": "Pass123",
				"domain": "example.com",
				"dbType": "sqlite",
				"sqlitePath": ":memory:"
			}`,
			wantCode: 200,
		},
		{
			name: "invalid dbType",
			payload: `{
				"email": "admin@example.com",
				"password": "Pass123",
				"domain": "example.com",
				"dbType": "mysql",
				"sqlitePath": "./data.db"
			}`,
			wantCode: 400,
		},
		{
			name: "postgres missing databaseUrl",
			payload: `{
				"email": "admin@example.com",
				"password": "Pass123",
				"domain": "example.com",
				"dbType": "postgres"
			}`,
			wantCode: 400,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set up clean environment for each test
			dirCleanup := testutil.WithCmdTestDir(t)
			defer dirCleanup()

			// Create required directories
			_ = os.MkdirAll("lc_base", 0o775)

			// Clean up database connection after test to prevent resource leaks
			defer db.Close()

			app := fiber.New()
			app.Post("/api/install", Install)

			req := httptest.NewRequest("POST", "/api/install", strings.NewReader(tt.payload))
			req.Header.Set("Content-Type", "application/json")

			// Increase timeout to 10 seconds to allow full migration suite to complete
			// The Install handler runs db.MigrateWithConfig which can take several seconds
			resp, err := app.Test(req, fiber.TestConfig{Timeout: 10 * time.Second})
			require.NoError(t, err)
			require.Equal(t, tt.wantCode, resp.StatusCode)
		})
	}
}
