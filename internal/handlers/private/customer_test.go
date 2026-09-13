package handlers

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/gofiber/fiber/v3"

	"github.com/shurco/mycart/internal/models"
	"github.com/shurco/mycart/internal/queries"
	"github.com/shurco/mycart/internal/testutil"
	"github.com/shurco/mycart/pkg/litepay"
	"github.com/shurco/mycart/pkg/security"
)

// cabinetApp wires the customer endpoints onto a bare app, the way the other
// handler tests do: the middleware is exercised on its own, and what is under
// test here is the handler.
func cabinetApp(t *testing.T) (*fiber.App, func()) {
	t.Helper()

	app, _, cleanup := testutil.SetupTestApp(t)
	app.Get("/api/_/customers", Customers)
	app.Get("/api/_/customers/carts", CustomerCarts)
	app.Patch("/api/_/customers/:customer_id/active", UpdateCustomerActive)
	app.Patch("/api/_/customers/:customer_id/password", UpdateCustomerPassword)
	app.Delete("/api/_/customers/:customer_id", DeleteCustomer)

	return app, cleanup
}

// testExec runs a statement the test needs for its own staging, through the
// same dialect-aware handle the queries use.
func testExec(t *testing.T, query string, args ...any) {
	t.Helper()

	if _, err := queries.Conn().ExecContext(t.Context(), query, args...); err != nil {
		t.Fatalf("%s: %v", query, err)
	}
}

// seedSession writes a session row so a test can watch it being revoked.
func seedSession(t *testing.T, key, customerID string) {
	t.Helper()

	testExec(t, `INSERT INTO session (key, value, expires) VALUES (?, ?, ?)`,
		key, queries.SessionValue(queries.SessionRoleCustomer, customerID), 0)
}

// cabinetCustomer registers an account, blocked when active is false.
func cabinetCustomer(t *testing.T, email, name string, active bool) *models.Customer {
	t.Helper()

	customer, err := queries.DB().CreateCustomer(t.Context(), email, "correct-horse-battery", name)
	if err != nil {
		t.Fatalf("create customer %s: %v", email, err)
	}
	if !active {
		if err := queries.DB().SetCustomerActive(t.Context(), customer.ID, false); err != nil {
			t.Fatalf("block customer %s: %v", email, err)
		}
	}
	return customer
}

// cabinetCart writes a cart with an explicit creation date.
//
// The date is passed in rather than left to the column default because the list
// is ordered by when the customer last bought, and the default has one-second
// resolution: two carts written in the same test would tie and the assertion
// would be about the tie-break instead of about recency.
func cabinetCart(t *testing.T, email string, amount int, currency string, status litepay.Status, created string) {
	t.Helper()

	id := security.RandomString()
	cart := &models.Cart{
		Core:          models.Core{ID: id},
		Email:         email,
		Cart:          []models.CartProduct{},
		AmountTotal:   amount,
		Currency:      currency,
		PaymentStatus: status,
		PaymentSystem: litepay.STRIPE,
	}
	if err := queries.DB().AddCart(t.Context(), cart); err != nil {
		t.Fatalf("add cart for %s: %v", email, err)
	}
	testExec(t, `UPDATE cart SET created = ? WHERE id = ?`, created, id)
}

// customersResponse is the list payload.
type customersResponse struct {
	Result struct {
		Customers []models.CustomerSummary `json:"customers"`
		Total     int                      `json:"total"`
		Page      int                      `json:"page"`
		Limit     int                      `json:"limit"`
	} `json:"result"`
}

// listCustomers calls the list endpoint and returns the decoded payload.
func listCustomers(t *testing.T, app *fiber.App, query string) customersResponse {
	t.Helper()

	resp := testutil.DoRequest(t, app, http.MethodGet, "/api/_/customers"+query, "", "")
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}

	var decoded customersResponse
	if err := json.NewDecoder(resp.Body).Decode(&decoded); err != nil {
		t.Fatalf("decode: %v", err)
	}
	return decoded
}

// TestCustomers covers the list: which addresses become rows, what the totals
// count, and in what order.
//
// The shop's fixtures already hold one paid cart from user@gmail.com, so every
// assertion here searches for @example.com to look at the customers this test
// created and nothing else.
func TestCustomers(t *testing.T) {
	app, cleanup := cabinetApp(t)
	defer cleanup()

	cabinetCustomer(t, "registered@example.com", "Anna Smith", true)
	// Registered and paying: the common case of a returning buyer, and the one
	// where the two halves of the union have to collapse into a single row
	// rather than appear as an account and a buyer.
	cabinetCustomer(t, "both@example.com", "", true)
	cabinetCart(t, "both@example.com", 100, "USD", litepay.PAID, "2021-01-01 00:00:00")
	cabinetCart(t, "early-ord@example.com", 500, "USD", litepay.PAID, "2020-01-01 00:00:00")
	cabinetCart(t, "late-ord@example.com", 700, "USD", litepay.PAID, "2025-01-01 00:00:00")
	cabinetCart(t, "euro@example.com", 900, "EUR", litepay.PAID, "2024-01-01 00:00:00")
	// One buyer, two carts, two spellings of one address, and a cart that was
	// never paid. They have to collapse into a single row that counts only the
	// paid one.
	cabinetCart(t, "Guest@Example.COM ", 300, "USD", litepay.PAID, "2023-01-01 00:00:00")
	cabinetCart(t, "guest@example.com", 999, "USD", litepay.CANCELED, "2023-06-01 00:00:00")

	t.Run("one row per address, buyers first, newest purchase first", func(t *testing.T) {
		got := listCustomers(t, app, "?search=%40example.com")
		if got.Result.Total != 6 {
			t.Fatalf("total = %d, want 6", got.Result.Total)
		}

		want := []string{
			"late-ord@example.com",
			"euro@example.com",
			"guest@example.com",
			"both@example.com",
			"early-ord@example.com",
			"registered@example.com",
		}
		if len(got.Result.Customers) != len(want) {
			t.Fatalf("got %d rows, want %d", len(got.Result.Customers), len(want))
		}
		for i, email := range want {
			if got.Result.Customers[i].Email != email {
				t.Errorf("row %d = %s, want %s", i, got.Result.Customers[i].Email, email)
			}
		}
	})

	t.Run("totals count paid carts in the shop currency", func(t *testing.T) {
		rows := map[string]models.CustomerSummary{}
		for _, customer := range listCustomers(t, app, "?search=%40example.com").Result.Customers {
			rows[customer.Email] = customer
		}

		tests := []struct {
			email      string
			purchases  int
			spent      int
			registered bool
		}{
			{"late-ord@example.com", 1, 700, false},
			{"early-ord@example.com", 1, 500, false},
			// Paid, so it counts as a purchase — but the shop sells in USD, and
			// adding EUR cents to USD cents would produce a number that means
			// nothing.
			{"euro@example.com", 1, 0, false},
			// The unpaid cart is not a purchase.
			{"guest@example.com", 1, 300, false},
			{"both@example.com", 1, 100, true},
			{"registered@example.com", 0, 0, true},
		}

		for _, tt := range tests {
			row, ok := rows[tt.email]
			if !ok {
				t.Errorf("%s: missing from the list", tt.email)
				continue
			}
			if row.Purchases != tt.purchases {
				t.Errorf("%s: purchases = %d, want %d", tt.email, row.Purchases, tt.purchases)
			}
			if row.Spent != tt.spent {
				t.Errorf("%s: spent = %d, want %d", tt.email, row.Spent, tt.spent)
			}
			if row.Registered != tt.registered {
				t.Errorf("%s: registered = %v, want %v", tt.email, row.Registered, tt.registered)
			}
			if row.Registered && row.ID == "" {
				t.Errorf("%s: registered row has no id", tt.email)
			}
			if !row.Registered && row.ID != "" {
				t.Errorf("%s: guest row carries id %q", tt.email, row.ID)
			}
			if row.Currency != "USD" {
				t.Errorf("%s: currency = %q, want USD", tt.email, row.Currency)
			}
		}
	})

	t.Run("registered filter drops the guests", func(t *testing.T) {
		got := listCustomers(t, app, "?search=%40example.com&registered=true")
		if got.Result.Total != 2 {
			t.Fatalf("total = %d, want 2", got.Result.Total)
		}
		for _, customer := range got.Result.Customers {
			if !customer.Registered || customer.ID == "" {
				t.Errorf("%s: registered = %v, id = %q", customer.Email, customer.Registered, customer.ID)
			}
		}
	})

	t.Run("search matches the name too", func(t *testing.T) {
		got := listCustomers(t, app, "?search=smith")
		if got.Result.Total != 1 {
			t.Fatalf("total = %d, want 1", got.Result.Total)
		}
		if got.Result.Customers[0].Email != "registered@example.com" {
			t.Errorf("email = %s", got.Result.Customers[0].Email)
		}
	})

	t.Run("pagination", func(t *testing.T) {
		got := listCustomers(t, app, "?search=%40example.com&limit=2")
		if got.Result.Total != 6 {
			t.Errorf("total = %d, want 6 (the whole set, not the page)", got.Result.Total)
		}
		if len(got.Result.Customers) != 2 {
			t.Errorf("rows = %d, want 2", len(got.Result.Customers))
		}
	})

	t.Run("no match", func(t *testing.T) {
		got := listCustomers(t, app, "?search=nobody%40nowhere.invalid")
		if got.Result.Total != 0 || len(got.Result.Customers) != 0 {
			t.Errorf("total = %d, rows = %d, want 0 and 0", got.Result.Total, len(got.Result.Customers))
		}
	})
}

// TestCustomerCarts checks the carts of one address: matched however it was
// spelled, and in every payment state, because the operator is looking at what
// the customer did rather than at the takings.
func TestCustomerCarts(t *testing.T) {
	app, cleanup := cabinetApp(t)
	defer cleanup()

	cabinetCart(t, "Buyer@Example.com", 300, "USD", litepay.PAID, "2023-01-01 00:00:00")
	cabinetCart(t, "buyer@example.com ", 999, "USD", litepay.FAILED, "2023-06-01 00:00:00")

	tests := []struct {
		name      string
		query     string
		wantCode  int
		wantCarts int
	}{
		{"lower case", "?email=buyer%40example.com", http.StatusOK, 2},
		{"mixed case and spaces", "?email=%20Buyer%40Example.COM%20", http.StatusOK, 2},
		{"unknown address", "?email=someone%40example.com", http.StatusOK, 0},
		{"missing address", "", http.StatusBadRequest, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := testutil.DoRequest(t, app, http.MethodGet, "/api/_/customers/carts"+tt.query, "", "")
			defer func() { _ = resp.Body.Close() }()

			if resp.StatusCode != tt.wantCode {
				t.Fatalf("status = %d, want %d", resp.StatusCode, tt.wantCode)
			}
			if tt.wantCode != http.StatusOK {
				return
			}

			var decoded struct {
				Result struct {
					Carts []models.Cart `json:"carts"`
					Total int           `json:"total"`
				} `json:"result"`
			}
			if err := json.NewDecoder(resp.Body).Decode(&decoded); err != nil {
				t.Fatalf("decode: %v", err)
			}
			if decoded.Result.Total != tt.wantCarts || len(decoded.Result.Carts) != tt.wantCarts {
				t.Errorf("total = %d, carts = %d, want %d",
					decoded.Result.Total, len(decoded.Result.Carts), tt.wantCarts)
			}
		})
	}
}

// TestUpdateCustomerActive checks the block: it flips the flag, and it ends the
// sessions the blocked customer had open, because a token stays good for as
// long as its session row exists.
func TestUpdateCustomerActive(t *testing.T) {
	app, cleanup := cabinetApp(t)
	defer cleanup()

	customer := cabinetCustomer(t, "buyer@example.com", "", true)
	seedSession(t, "cabinet-session", customer.ID)

	toggle := func(id string) int {
		t.Helper()
		resp := testutil.DoRequest(t, app, http.MethodPatch, "/api/_/customers/"+id+"/active", "", "")
		defer func() { _ = resp.Body.Close() }()
		return resp.StatusCode
	}

	if code := toggle(customer.ID); code != http.StatusOK {
		t.Fatalf("block: status = %d, want 200", code)
	}

	blocked, err := queries.DB().CustomerByID(t.Context(), customer.ID)
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	if blocked.Active {
		t.Error("the customer is still active after being blocked")
	}
	if _, err := queries.DB().GetSession(t.Context(), "cabinet-session"); err == nil {
		t.Error("the blocked customer's session survived")
	}

	if code := toggle(customer.ID); code != http.StatusOK {
		t.Fatalf("unblock: status = %d, want 200", code)
	}
	unblocked, err := queries.DB().CustomerByID(t.Context(), customer.ID)
	if err != nil {
		t.Fatalf("read back after unblock: %v", err)
	}
	if !unblocked.Active {
		t.Error("the customer is still blocked after being unblocked")
	}

	if code := toggle("nonexistent12345"); code != http.StatusNotFound {
		t.Errorf("unknown id: status = %d, want 404", code)
	}
}

// TestUpdateCustomerPassword checks the reset: the returned password is the one
// that opens the account, the old one does not, nothing plaintext is stored,
// and the sessions opened with the old password are gone.
func TestUpdateCustomerPassword(t *testing.T) {
	app, cleanup := cabinetApp(t)
	defer cleanup()

	customer, err := queries.DB().CreateCustomer(t.Context(), "buyer@example.com", "old-password-1", "")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	seedSession(t, "cabinet-session", customer.ID)

	resp := testutil.DoRequest(t, app, http.MethodPatch, "/api/_/customers/"+customer.ID+"/password", "", "")
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}

	var decoded struct {
		Result struct {
			Password string `json:"password"`
		} `json:"result"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&decoded); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if decoded.Result.Password == "" {
		t.Fatal("no password in the response; the operator has nothing to hand over")
	}

	stored, err := queries.DB().CustomerByEmail(t.Context(), "buyer@example.com")
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	if stored.Password == decoded.Result.Password {
		t.Error("the password was stored in the clear")
	}
	if security.ComparePasswords(stored.Password, "old-password-1") {
		t.Error("the old password still opens the account")
	}
	if !security.ComparePasswords(stored.Password, decoded.Result.Password) {
		t.Error("the returned password does not open the account")
	}
	if _, err := queries.DB().GetSession(t.Context(), "cabinet-session"); err == nil {
		t.Error("the session opened with the old password survived the reset")
	}
}

// TestDeleteCustomer checks that removing an account leaves the record of what
// was paid alone: the carts are the shop's books, not the customer's property.
func TestDeleteCustomer(t *testing.T) {
	app, cleanup := cabinetApp(t)
	defer cleanup()

	customer := cabinetCustomer(t, "buyer@example.com", "", true)
	cabinetCart(t, "buyer@example.com", 500, "USD", litepay.PAID, "2024-01-01 00:00:00")
	seedSession(t, "cabinet-session", customer.ID)

	resp := testutil.DoRequest(t, app, http.MethodDelete, "/api/_/customers/"+customer.ID, "", "")
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}

	if _, err := queries.DB().CustomerByID(t.Context(), customer.ID); err == nil {
		t.Error("the account survived the delete")
	}
	if _, err := queries.DB().GetSession(t.Context(), "cabinet-session"); err == nil {
		t.Error("the deleted customer's session survived")
	}

	carts, err := queries.DB().CartsByEmail(t.Context(), "buyer@example.com")
	if err != nil {
		t.Fatalf("read carts: %v", err)
	}
	if len(carts) != 1 {
		t.Errorf("carts = %d, want 1: deleting an account must not erase what was paid", len(carts))
	}
}
