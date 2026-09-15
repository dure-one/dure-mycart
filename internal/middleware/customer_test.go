package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"

	"github.com/shurco/mycart/internal/models"
	"github.com/shurco/mycart/internal/queries"
	"github.com/shurco/mycart/internal/testutil"
)

// enableCabinet turns the cabinet on and returns its signing key.
func enableCabinet(t *testing.T) string {
	t.Helper()

	if err := queries.DB().UpdateSettingByGroup(t.Context(), &models.Account{Enabled: true, ExpireHours: 24}); err != nil {
		t.Fatalf("enable cabinet: %v", err)
	}

	secret, err := queries.DB().AccountSecret(t.Context())
	if err != nil {
		t.Fatalf("account secret: %v", err)
	}
	return secret
}

// deleteCustomer creates an account, deletes it, and returns its id, so a case
// can present a session that outlived the account it points at.
func deleteCustomer(t *testing.T, email string) string {
	t.Helper()

	customer := addCustomer(t, email, true)
	if err := queries.DB().DeleteCustomer(t.Context(), customer.ID); err != nil {
		t.Fatalf("delete customer: %v", err)
	}
	return customer.ID
}

// bearer spells a token the way a client sends it. Without the scheme the
// request never reaches the checks under test: the extractor rejects it first,
// and every case would pass for the wrong reason.
func bearer(token string) string {
	return "Bearer " + token
}

// addSession writes a session row of the given role and returns its id.
func addSession(t *testing.T, role, subject string) string {
	t.Helper()

	id := uuid.NewString()
	expires := time.Now().Add(time.Hour).Unix()
	if err := queries.DB().AddSession(t.Context(), id, queries.SessionValue(role, subject), expires); err != nil {
		t.Fatalf("add session: %v", err)
	}
	return id
}

// addCustomer creates an account and returns it, optionally already blocked.
//
// A session row is no longer enough to open the cabinet: the middleware also
// asks whether the account behind the session exists and is unblocked, so a
// positive case needs a real customer, not just an id written into a session.
func addCustomer(t *testing.T, email string, active bool) *models.Customer {
	t.Helper()

	customer, err := queries.DB().CreateCustomer(t.Context(), email, "correct-horse-battery", "Buyer")
	if err != nil {
		t.Fatalf("create customer: %v", err)
	}
	if !active {
		if err := queries.DB().SetCustomerActive(t.Context(), customer.ID, false); err != nil {
			t.Fatalf("block customer: %v", err)
		}
	}
	return customer
}

// TestCustomerJWTProtected_KeysAndRolesApart is the guard for the pair of
// properties that keep the two authenticated surfaces from becoming one:
// a storefront token is only accepted here when it is signed with the cabinet
// key *and* its session row says it belongs to a customer.
func TestCustomerJWTProtected_KeysAndRolesApart(t *testing.T) {
	cleanup := testutil.SetupTestDB(t)
	defer cleanup()

	cabinetSecret := enableCabinet(t)

	app := fiber.New()
	app.Get("/api/customer/me", CustomerJWTProtected(), func(c fiber.Ctx) error {
		return c.JSON(fiber.Map{"id": CustomerID(c), "session": CustomerSessionID(c)})
	})

	customer := addCustomer(t, "buyer@example.com", true)
	blocked := addCustomer(t, "blocked@example.com", false)

	customerSession := addSession(t, queries.SessionRoleCustomer, customer.ID)
	adminSession := addSession(t, queries.SessionRoleAdmin, "")

	tests := []struct {
		name       string
		token      string
		wantStatus []int
	}{
		{"no token", "", []int{http.StatusUnauthorized}},
		{"garbage", "Bearer not-a-token", []int{http.StatusUnauthorized}},
		{"customer session", bearer(mustGenerateToken(t, cabinetSecret, customerSession)), []int{http.StatusOK}},
		// The same session row, signed with the admin key: the signature check
		// alone would not tell the surfaces apart, so the key has to.
		{"signed with the admin key", bearer(mustGenerateToken(t, testutil.FixtureJWTSecret, customerSession)), []int{http.StatusUnauthorized}},
		{"session that does not exist", bearer(mustGenerateToken(t, cabinetSecret, uuid.NewString())), []int{http.StatusUnauthorized}},
		// A row that exists but belongs to the other surface must not open the
		// cabinet, even with a signature the cabinet key accepts.
		{"admin session", bearer(mustGenerateToken(t, cabinetSecret, adminSession)), []int{http.StatusUnauthorized}},
		// A valid session whose account has since been blocked. The admin API
		// revokes sessions when it blocks, so this is the second lock: it holds
		// for a session that survived, and for a flag flipped in the database.
		{
			"blocked account",
			bearer(mustGenerateToken(t, cabinetSecret, addSession(t, queries.SessionRoleCustomer, blocked.ID))),
			[]int{http.StatusUnauthorized},
		},
		// A session whose account was deleted outright.
		{
			"deleted account",
			bearer(mustGenerateToken(t, cabinetSecret, addSession(t, queries.SessionRoleCustomer, deleteCustomer(t, "gone@example.com")))),
			[]int{http.StatusUnauthorized},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/api/customer/me", nil)
			if tt.token != "" {
				req.Header.Set("Authorization", tt.token)
			}
			resp, err := app.Test(req)
			if err != nil {
				t.Fatalf("app.Test: %v", err)
			}
			testutil.AssertStatus(t, resp, tt.wantStatus...)
		})
	}
}

// TestJWTProtected_RejectsCustomerSessions is the regression guard for the hole
// the cabinet would otherwise open. Both surfaces store their session in one
// table, and both tokens carry nothing but a session id; before the role check,
// a row in that table was the only thing the admin middleware asked for, so a
// storefront session was enough to reach the admin API.
func TestJWTProtected_RejectsCustomerSessions(t *testing.T) {
	cleanup := testutil.SetupTestDB(t)
	defer cleanup()

	app := fiber.New()
	app.Get("/api/test", JWTProtected(), func(c fiber.Ctx) error { return c.SendStatus(http.StatusOK) })

	customerSession := addSession(t, queries.SessionRoleCustomer, "cust-1")

	tests := []struct {
		name       string
		token      string
		wantStatus []int
	}{
		// Signed with the admin key and backed by a real row: everything the
		// middleware checked before the fix is satisfied.
		{"customer session signed with the admin key", mustGenerateToken(t, testutil.FixtureJWTSecret, customerSession), []int{http.StatusUnauthorized}},
		{"admin session", mustGenerateToken(t, testutil.FixtureJWTSecret, addSession(t, queries.SessionRoleAdmin, "")), []int{http.StatusOK}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
			req.Header.Set("Authorization", "Bearer "+tt.token)
			resp, err := app.Test(req)
			if err != nil {
				t.Fatalf("app.Test: %v", err)
			}
			testutil.AssertStatus(t, resp, tt.wantStatus...)
		})
	}
}

// TestAccountEnabled checks the switch that takes the cabinet off a shop that
// does not want one, and that it takes effect per request rather than needing a
// restart.
func TestAccountEnabled(t *testing.T) {
	cleanup := testutil.SetupTestDB(t)
	defer cleanup()

	app := fiber.New()
	app.Get("/api/customer/me", AccountEnabled(), func(c fiber.Ctx) error { return c.SendStatus(http.StatusOK) })

	// The migration seeds it off.
	resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/api/customer/me", nil))
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	// 404, not 403: an installation without a cabinet should not advertise
	// that one could exist.
	testutil.AssertStatus(t, resp, http.StatusNotFound)

	enableCabinet(t)

	resp, err = app.Test(httptest.NewRequest(http.MethodGet, "/api/customer/me", nil))
	if err != nil {
		t.Fatalf("app.Test after enable: %v", err)
	}
	testutil.AssertStatus(t, resp, http.StatusOK)
}
