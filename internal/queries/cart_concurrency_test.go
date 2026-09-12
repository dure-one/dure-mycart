package queries_test

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/shurco/mycart/internal/models"
	"github.com/shurco/mycart/internal/queries"
	"github.com/shurco/mycart/internal/testutil"
	"github.com/shurco/mycart/pkg/litepay"
	"github.com/shurco/mycart/pkg/security"
)

// TestDigitalKeysAreClaimedOnce is the regression test for the key-allocation
// race in claimDigitalData.
//
// Digital keys are rows of digital_data with cart_id IS NULL; a purchase claims
// one per unit. Reading a candidate row and then writing it back — which is
// what the code used to do — lets two concurrent purchases read the same row
// and hand the same license key to two buyers. SQLite hid this behind
// `_txlock=immediate`, which serialises writers; PostgreSQL does not, so the
// claim has to be an UPDATE guarded on `cart_id IS NULL` whose row count
// decides the winner.
//
// The test runs on both engines. On the in-memory SQLite database the pool is
// capped at one connection, so the calls cannot really overlap there — the
// PostgreSQL run under TEST_DB_DRIVER=postgres is the one that exercises the
// race.
func TestDigitalKeysAreClaimedOnce(t *testing.T) {
	cleanup := testutil.SetupCleanDB(t)
	defer cleanup()

	ctx := context.Background()
	db := queries.DB()

	const buyers = 8

	productID := seedDigitalProduct(t, "data", buyers)
	for range buyers {
		if _, err := db.AddDigitalData(ctx, productID, "KEY-"+security.RandomString()); err != nil {
			t.Fatalf("add digital data: %v", err)
		}
	}

	carts := make([]string, buyers)
	for i := range carts {
		carts[i] = seedPaidCart(t, productID)
	}

	// Release every goroutine at once so the transactions really overlap.
	var (
		wg      sync.WaitGroup
		start   = make(chan struct{})
		mu      sync.Mutex
		keys    []string
		failure error
	)
	for _, cartID := range carts {
		wg.Add(1)
		go func(cartID string) {
			defer wg.Done()
			<-start

			mail, err := db.CartLetterPurchase(ctx, cartID)

			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				if failure == nil {
					failure = fmt.Errorf("purchase for %s: %w", cartID, err)
				}
				return
			}
			// Purchases is "Keys:\n1: KEY\n"; each cart buys one unit.
			for _, line := range strings.Split(mail.Data["Purchases"], "\n") {
				if _, key, ok := strings.Cut(line, ": "); ok {
					keys = append(keys, key)
				}
			}
		}(cartID)
	}
	close(start)
	wg.Wait()

	if failure != nil {
		t.Fatalf("concurrent purchases failed: %v", failure)
	}
	if len(keys) != buyers {
		t.Fatalf("collected %d keys, want %d: %v", len(keys), buyers, keys)
	}

	seen := map[string]bool{}
	for _, key := range keys {
		if seen[key] {
			t.Fatalf("key %s was handed to two buyers: %v", key, keys)
		}
		seen[key] = true
	}

	// The claimed rows are the other half of the invariant: every key handed
	// out belongs to exactly one cart, and no key is left unclaimed.
	var claimed int
	if err := db.CartQueries.DB.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM digital_data WHERE product_id = ? AND cart_id IS NOT NULL`,
		productID).Scan(&claimed); err != nil {
		t.Fatalf("count claimed keys: %v", err)
	}
	if claimed != buyers {
		t.Errorf("%d keys claimed, want %d", claimed, buyers)
	}

	// A retried mail must return the keys the buyer already owns rather than
	// claiming fresh ones.
	again, err := db.CartLetterPurchase(ctx, carts[0])
	if err != nil {
		t.Fatalf("repeat purchase: %v", err)
	}
	if !strings.Contains(again.Data["Purchases"], "1: ") {
		t.Errorf("repeat purchase lost its key: %q", again.Data["Purchases"])
	}
	if err := db.CartQueries.DB.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM digital_data WHERE product_id = ? AND cart_id IS NOT NULL`,
		productID).Scan(&claimed); err != nil {
		t.Fatalf("count claimed keys after retry: %v", err)
	}
	if claimed != buyers {
		t.Errorf("a repeated purchase claimed another key: %d claimed, want %d", claimed, buyers)
	}
}

// seedDigitalProduct inserts a sellable product of the given digital type and
// returns its id.
func seedDigitalProduct(t *testing.T, digital string, quantity int) string {
	t.Helper()

	ctx := context.Background()
	productID := security.RandomString()

	if _, err := queries.DB().ProductQueries.DB.ExecContext(ctx, `
		INSERT INTO product (id, name, brief, "desc", slug, amount, quantity, digital, active, deleted)
		VALUES (?, ?, ?, 'description', ?, 1000, ?, ?, TRUE, FALSE)
	`, productID, "Digital "+productID, "brief", "digital-"+productID, quantity, digital); err != nil {
		t.Fatalf("insert product: %v", err)
	}
	return productID
}

// seedPaidCart inserts a paid cart holding one unit of productID.
func seedPaidCart(t *testing.T, productID string) string {
	t.Helper()

	cartID := security.RandomString()

	if err := queries.DB().AddCart(context.Background(), &models.Cart{
		Core:          models.Core{ID: cartID},
		Email:         cartID + "@example.com",
		Cart:          []models.CartProduct{{ProductID: productID, Quantity: 1, UnitPrice: 1000}},
		AmountTotal:   1000,
		Currency:      "USD",
		PaymentStatus: litepay.PAID,
		PaymentSystem: litepay.STRIPE,
	}); err != nil {
		t.Fatalf("insert cart: %v", err)
	}
	return cartID
}
