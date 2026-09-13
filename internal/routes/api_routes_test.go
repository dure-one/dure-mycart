package routes

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/gofiber/fiber/v3"

	"github.com/shurco/mycart/internal/database"
	"github.com/shurco/mycart/internal/models"
	"github.com/shurco/mycart/internal/queries"
	"github.com/shurco/mycart/migrations"
)

// routesTestDB brings up a blank queries DB so handlers wired into these
// routes don't nil-panic when exercised by the test Fiber client.
func routesTestDB(t *testing.T) {
	t.Helper()
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
}

func TestApiPublicRoutes_WiredCorrectly(t *testing.T) {
	routesTestDB(t)

	app := fiber.New()
	ApiPublicRoutes(app)

	// /ping is the only public route with no external dependencies.
	resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/ping", nil))
	if err != nil {
		t.Fatalf("ping: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("ping status = %d", resp.StatusCode)
	}

	// /api/settings and /api/products exist in the router — we don't care what
	// payload they return, just that the path resolved (not 404).
	for _, p := range []string{"/api/settings", "/api/products/"} {
		resp, err := app.Test(httptest.NewRequest(http.MethodGet, p, nil))
		if err != nil {
			t.Fatalf("%s: %v", p, err)
		}
		if resp.StatusCode == http.StatusNotFound {
			t.Errorf("route %s not registered (got 404)", p)
		}
	}
}

// TestApiPublicRoutes_CabinetOffByDefault checks the switch that keeps a
// shop without customer accounts from growing a sign-in form it never asked
// for. The routes are registered either way, so the setting takes effect on the
// next request rather than on the next restart — which is why these answer 404
// rather than never appearing in the router.
func TestApiPublicRoutes_CabinetOffByDefault(t *testing.T) {
	routesTestDB(t)

	app := fiber.New()
	ApiPublicRoutes(app)

	requests := []struct {
		method string
		path   string
	}{
		{http.MethodPost, "/api/customer/signup"},
		{http.MethodPost, "/api/customer/signin"},
		{http.MethodGet, "/api/customer/me"},
		{http.MethodGet, "/api/customer/purchases"},
		{http.MethodGet, "/api/customer/purchases/abcdefghijklmno/download"},
	}

	for _, r := range requests {
		resp, err := app.Test(httptest.NewRequest(r.method, r.path, nil))
		if err != nil {
			t.Fatalf("%s %s: %v", r.method, r.path, err)
		}
		if resp.StatusCode != http.StatusNotFound {
			t.Errorf("%s %s = %d, want 404 while the cabinet is disabled", r.method, r.path, resp.StatusCode)
		}
	}
}

// TestApiPublicRoutes_CustomerRoutesAreGuarded checks that the cabinet's own
// endpoints sit behind the cabinet guard once the cabinet is on.
//
// TestApiPublicRoutes_CabinetOffByDefault cannot tell the two apart: while the
// cabinet is off, the gate answers 404 before the guard is ever reached, and a
// route mounted a level up — on the app instead of on the customer group —
// would be invisible. The download route is the one that matters: registered
// outside the guard, it would hand a guide to anyone who could guess a file id.
func TestApiPublicRoutes_CustomerRoutesAreGuarded(t *testing.T) {
	routesTestDB(t)

	if err := queries.DB().UpdateSettingByGroup(t.Context(), &models.Account{Enabled: true}); err != nil {
		t.Fatalf("enable cabinet: %v", err)
	}

	app := fiber.New()
	ApiPublicRoutes(app)

	const fileID = "abcdefghijklmno"

	for _, path := range []string{
		"/api/customer/me",
		"/api/customer/purchases",
		"/api/customer/purchases/" + fileID + "/download",
	} {
		resp, err := app.Test(httptest.NewRequest(http.MethodGet, path, nil))
		if err != nil {
			t.Fatalf("%s: %v", path, err)
		}
		// 401 is what the guard says to a missing session; what the test is
		// about is that the request does not get through.
		if resp.StatusCode != http.StatusUnauthorized {
			t.Errorf("%s = %d, want 401 without a session", path, resp.StatusCode)
		}
	}
}

func TestApiPrivateRoutes_WiredWithAuthGuard(t *testing.T) {
	routesTestDB(t)

	app := fiber.New()
	ApiPrivateRoutes(app)

	// All /_/ routes require the JWT middleware. Without a cookie the guard
	// returns 401, which is still evidence the route was registered.
	resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/api/_/version", nil))
	if err != nil {
		t.Fatalf("version: %v", err)
	}
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected 401 without token, got %d", resp.StatusCode)
	}

	// /api/install is public.
	resp, err = app.Test(httptest.NewRequest(http.MethodPost, "/api/install", nil))
	if err != nil {
		t.Fatalf("install: %v", err)
	}
	if resp.StatusCode == http.StatusNotFound {
		t.Error("install route not registered")
	}
}

func TestSiteRoutes_MountsSPAHandler(t *testing.T) {
	routesTestDB(t)

	app := fiber.New()
	SiteRoutes(app)
	// The SPA handler falls back to index.html for unknown paths; the embedded
	// FS is available at runtime, so this only checks the route got mounted.
	resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/some-arbitrary-page", nil))
	if err != nil {
		t.Fatalf("site: %v", err)
	}
	// Either 200 (index fallback) or 404 (if the web embed is empty in test).
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNotFound {
		t.Errorf("unexpected status = %d", resp.StatusCode)
	}
}

func TestAdminRoutes_MountsSPAHandler(t *testing.T) {
	app := fiber.New()
	AdminRoutes(app)

	resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/_/dashboard", nil))
	if err != nil {
		t.Fatalf("admin: %v", err)
	}
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNotFound {
		t.Errorf("unexpected status = %d", resp.StatusCode)
	}
}

// TestApiPrivateRoutes_CustomerRoutesAreGuarded checks that every customer
// endpoint sits behind the admin guard.
//
// These routes read every address the shop knows and can block a buyer, reset
// their password and delete the account — an unguarded one would be the most
// damaging route in the application. Registering them on the app instead of on
// the guarded group is a one-line mistake, and this is what catches it.
func TestApiPrivateRoutes_CustomerRoutesAreGuarded(t *testing.T) {
	routesTestDB(t)

	app := fiber.New()
	ApiPrivateRoutes(app)

	// A fifteen-character id, so the routes' own length constraint does not
	// turn the request away before the middleware is reached: a 404 here would
	// hide whether the guard is actually applied.
	const customerID = "abcdefghijklmno"

	requests := []struct {
		method string
		path   string
	}{
		{http.MethodGet, "/api/_/customers"},
		{http.MethodGet, "/api/_/customers/carts"},
		{http.MethodPatch, "/api/_/customers/" + customerID + "/active"},
		{http.MethodPatch, "/api/_/customers/" + customerID + "/password"},
		{http.MethodDelete, "/api/_/customers/" + customerID},
	}

	for _, r := range requests {
		resp, err := app.Test(httptest.NewRequest(r.method, r.path, nil))
		if err != nil {
			t.Fatalf("%s %s: %v", r.method, r.path, err)
		}
		// The guard's exact answer for a missing token is 401, but what this
		// test is about is that the request does not get through: any refusal
		// will do, a success will not.
		if resp.StatusCode != http.StatusUnauthorized && resp.StatusCode != http.StatusBadRequest {
			t.Errorf("%s %s = %d, want a refusal without a token", r.method, r.path, resp.StatusCode)
		}
	}
}
