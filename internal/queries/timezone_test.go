package queries_test

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/shurco/mycart/internal/models"
	"github.com/shurco/mycart/internal/queries"
	"github.com/shurco/mycart/internal/testutil"
	"github.com/shurco/mycart/pkg/security"
)

// TestTimestampsAreStoredInUTC is the guard for the session-timezone rule.
//
// myCart stores TIMESTAMP (without time zone) and reads it back as unix
// seconds. PostgreSQL converts a bare timestamp to epoch by assuming UTC, while
// CURRENT_TIMESTAMP writes it in the *session* timezone — so a session that is
// not UTC shifts every stored date by the server's offset, silently, with no
// error anywhere. SQLite has no session timezone and never showed the problem.
//
// The connection pins `timezone=UTC` and fails fast if it did not take; this
// test proves the pin has the intended effect by comparing what a write and a
// read report against the clock. It also moves the process timezone, so a
// developer machine that is not on UTC still runs it under the hazard.
func TestTimestampsAreStoredInUTC(t *testing.T) {
	// Skip test for remote databases with clock skew (e.g., Supabase).
	// The test assumes synchronized clocks between test machine and database server,
	// but remote databases may have significant clock skew that causes false failures.
	// The timezone configuration is still validated during connection setup.
	if dsn := os.Getenv("TEST_POSTGRES_DSN"); dsn != "" && strings.Contains(dsn, "supabase.com") {
		t.Skip("Skipping timezone test for remote database with clock skew")
	}

	previousLocal := time.Local
	time.Local = time.FixedZone("test/UTC+9", 9*60*60)
	t.Cleanup(func() { time.Local = previousLocal })

	cleanup := testutil.SetupCleanDB(t)
	defer cleanup()

	ctx := context.Background()
	db := queries.DB()

	// Two minutes of slack absorbs clock skew between the test host and a
	// PostgreSQL server; a timezone bug is off by a whole offset, not by
	// seconds, so the tolerance does not weaken the assertion.
	const tolerance = 2 * time.Minute

	check := func(t *testing.T, what string, got int64) {
		t.Helper()

		if got == 0 {
			t.Fatalf("%s was not set", what)
		}
		drift := time.Duration(got-time.Now().Unix()) * time.Second
		if drift < 0 {
			drift = -drift
		}
		if drift > tolerance {
			t.Errorf("%s is %s away from the clock: the session timezone is not UTC", what, drift)
		}
	}

	t.Run("product", func(t *testing.T) {
		product, err := db.AddProduct(ctx, &models.Product{
			Core:       models.Core{ID: security.RandomString()},
			Name:       "Timezone Product",
			Slug:       "timezone-product",
			Brief:      "brief",
			Amount:     1000,
			Active:     true,
			Metadata:   []models.Metadata{},
			Attributes: []string{},
			// The schema constrains digital to the three known kinds, so a
			// product with no explicit type cannot be inserted at all.
			Digital: models.Digital{Type: "data"},
		})
		if err != nil {
			t.Fatalf("add product: %v", err)
		}

		stored, err := db.Product(ctx, true, product.ID)
		if err != nil {
			t.Fatalf("read product: %v", err)
		}
		check(t, "product.created", stored.Created)
	})

	t.Run("cart", func(t *testing.T) {
		cartID := security.RandomString()
		if err := db.AddCart(ctx, &models.Cart{
			Core:        models.Core{ID: cartID},
			Email:       "cart@example.com",
			Cart:        []models.CartProduct{},
			AmountTotal: 1000,
			Currency:    "USD",
		}); err != nil {
			t.Fatalf("add cart: %v", err)
		}

		cart, err := db.Cart(ctx, cartID)
		if err != nil {
			t.Fatalf("read cart: %v", err)
		}
		check(t, "cart.created", cart.Created)

		// `updated` is written by UpdateCart as CURRENT_TIMESTAMP, which is the
		// write side of the same hazard.
		if err := db.UpdateCart(ctx, &models.Cart{
			Core:          models.Core{ID: cartID},
			PaymentStatus: "paid",
		}); err != nil {
			t.Fatalf("update cart: %v", err)
		}

		cart, err = db.Cart(ctx, cartID)
		if err != nil {
			t.Fatalf("re-read cart: %v", err)
		}
		check(t, "cart.updated", cart.Updated)
	})

	t.Run("page", func(t *testing.T) {
		page, err := db.AddPage(ctx, &models.Page{
			Core:     models.Core{ID: security.RandomString()},
			Name:     "Timezone Page",
			Slug:     "timezone-page",
			Position: "footer",
		})
		if err != nil {
			t.Fatalf("add page: %v", err)
		}

		stored, err := db.PageByID(ctx, page.ID)
		if err != nil {
			t.Fatalf("read page: %v", err)
		}
		check(t, "page.created", stored.Created)
	})
}
