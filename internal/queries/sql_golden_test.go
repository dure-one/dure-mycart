package queries

import (
	"context"
	"flag"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"

	"github.com/shurco/mycart/internal/database"
	"github.com/shurco/mycart/internal/models"
	"github.com/shurco/mycart/migrations"
)

var updateGolden = flag.Bool("update", false, "rewrite testdata/sql_golden.txt")

// recordingDialect captures every statement on its way to the database.
//
// Wrapping the dialect records the statements exactly as the query layer builds
// them, before the connection translates them for the driver.
type recordingDialect struct {
	database.Dialect

	mu    sync.Mutex
	stmts []string
}

func (r *recordingDialect) Rebind(query string) string {
	r.mu.Lock()
	r.stmts = append(r.stmts, query)
	r.mu.Unlock()
	return r.Dialect.Rebind(query)
}

func (r *recordingDialect) statements() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]string(nil), r.stmts...)
}

// TestSQLGolden freezes the SQL the query layer emits.
//
// It records the SQLite rendering: the source SQL is shared between both
// engines, and only the file's dialect fragments (epoch conversion, JSON
// aggregation) differ, which TestDialectFragments in internal/database covers.
// This test is the regression net for the rest — a reviewable diff whenever a
// query changes shape.
func TestSQLGolden(t *testing.T) {
	cleanup := withTempBase(t)
	defer cleanup()

	if err := New(database.Config{Driver: database.DriverSQLite, DSN: database.DefaultSQLiteDSN}, migrations.Embed()); err != nil {
		t.Fatalf("init queries: %v", err)
	}

	prev := Conn()
	if prev.Dialect().Name() != database.DriverSQLite {
		t.Skip("the golden file records the SQLite rendering of the shared SQL")
	}

	rec := &recordingDialect{Dialect: prev.Dialect()}
	SetConn(database.Wrap(prev.Raw(), rec))
	defer SetConn(prev)

	ctx := context.Background()
	db := DB()

	// Reads. Statements are recorded whether or not the call finds a row, so
	// the test does not need fixtures — it needs each query to run.
	ignoreErr(db.IsInstalled(ctx))
	ignoreErr(db.GetSettingByKey(ctx, "site_name", "mail_letter_purchase"))
	ignoreErr(db.ListProducts(ctx, true, 0, 0, ""))
	ignoreErr(db.ListProducts(ctx, false, 10, 0, "cart-id"))
	ignoreErr(db.Product(ctx, true, "golden-product-id"))
	ignoreErr(db.Product(ctx, false, "golden-product-slug"))
	ignoreErr(db.ProductImages(ctx, "golden-product-id"))
	ignoreErr(db.ProductDigital(ctx, "golden-product-id"))
	ignoreErr(db.ListPages(ctx, true, 0, 0))
	ignoreErr(db.ListPages(ctx, false, 10, 0, "page-id"))
	ignoreErr(db.Page(ctx, "terms"))
	ignoreErr(db.PageByID(ctx, "golden-page-id"))
	ignoreErr(db.Carts(ctx, 10, 0))
	ignoreErr(db.Cart(ctx, "golden-cart-id"))

	// Writes.
	product := &models.Product{
		Core:        models.Core{ID: "golden-product-id"},
		Name:        "Golden Product",
		Slug:        "golden-product-slug",
		Brief:       "brief",
		Description: "description",
		Amount:      1000,
		Quantity:    5,
		SKU:         "GOLDEN-1",
		Active:      true,
		Metadata:    []models.Metadata{{Key: "key", Value: "value"}},
		Attributes:  []string{"color"},
		Digital:     models.Digital{Type: "data"},
		Images:      []models.File{{ID: "golden-image-id", Name: "uuid-name", Ext: "png", OrigName: "a.png"}},
	}
	if _, err := db.AddProductWithVariants(ctx, product); err != nil {
		t.Fatalf("AddProductWithVariants: %v", err)
	}
	if _, err := db.AddProduct(ctx, product); err != nil && !isUniqueViolation(err) {
		t.Fatalf("AddProduct: %v", err)
	}
	if err := db.UpdateProduct(ctx, product); err != nil {
		t.Fatalf("UpdateProduct: %v", err)
	}
	ignoreErr(db.UpdateActive(ctx, product.ID))
	ignoreErr(db.UpdateDigital(ctx, &models.Data{ID: "golden-digital-id", Content: "key"}))

	cart := &models.Cart{
		Core:          models.Core{ID: "golden-cart-id"},
		Email:         "golden@example.com",
		Cart:          []models.CartProduct{},
		AmountTotal:   1000,
		Currency:      "USD",
		PaymentID:     "pi_1",
		PaymentStatus: "paid",
	}
	if err := db.AddCart(ctx, cart); err != nil {
		t.Fatalf("AddCart: %v", err)
	}
	if err := db.UpdateCart(ctx, cart); err != nil {
		t.Fatalf("UpdateCart: %v", err)
	}

	if err := db.AddSession(ctx, "golden-key", "golden-value", 0); err != nil {
		t.Fatalf("AddSession: %v", err)
	}
	ignoreErr(db.GetSession(ctx, "golden-key"))
	ignoreErr(db.UpdateSession(ctx, "golden-key", "golden-value-2", 0))
	ignoreErr(db.DeleteSession(ctx, "golden-key"))

	// Storefront customer accounts. These queries arrived after the file was
	// first frozen and are recorded here so that a change to any of them shows
	// up in the golden diff like the rest, rather than escaping the net.
	customer, err := db.CreateCustomer(ctx, "golden-customer@example.com", "golden-password", "Golden")
	if err != nil {
		t.Fatalf("CreateCustomer: %v", err)
	}
	ignoreErr(db.CustomerByEmail(ctx, "golden-customer@example.com"))
	ignoreErr(db.CustomerByID(ctx, customer.ID))
	ignoreErr(db.Account(ctx))
	ignoreErr(db.AccountSecret(ctx))
	ignoreErr(db.Customers(ctx, CustomerFilter{Search: "golden", Currency: "USD"}, 10, 0))
	ignoreErr(db.Customers(ctx, CustomerFilter{RegisteredOnly: true, Currency: "USD"}, 10, 20))
	ignoreErr(db.CartsByEmail(ctx, "golden-customer@example.com"))

	// The cabinet's purchase list resolves an order's lines against the
	// catalogue, and the statements that read a line's deliverables are reached
	// only by the kind of product that has them. The golden cart above is
	// empty, so the pieces are called directly: what is frozen is each
	// statement's shape, and this way all of them are in the file whether or
	// not the fixtures happen to carry a file product.
	ignoreErr(db.PaidCartsByEmail(ctx, "golden@example.com"))
	ignoreErr(db.CustomerPurchases(ctx, "golden@example.com"))
	ignoreErr(db.CustomerOwnsProduct(ctx, "golden@example.com", product.ID))
	ignoreErr(db.EntitledDigitalFile(ctx, "golden@example.com", "golden-file-id"))
	ignoreErr(db.purchaseProducts(ctx, []string{product.ID}))
	ignoreErr(db.purchaseFiles(ctx, []string{product.ID}))
	ignoreErr(db.purchaseCodes(ctx, []string{cart.ID}))

	ignoreErr(db.SetCustomerActive(ctx, customer.ID, false))
	ignoreErr(db.SetCustomerPassword(ctx, customer.ID, "golden-hash"))
	ignoreErr(db.RevokeCustomerSessions(ctx, customer.ID))
	ignoreErr(db.DeleteCustomer(ctx, customer.ID))

	got := strings.Join(rec.statements(), "\n---\n")

	// The test chdirs into a temporary base directory, so the golden file is
	// located relative to this source file rather than to the working directory.
	_, thisFile, _, _ := runtime.Caller(0)
	path := filepath.Join(filepath.Dir(thisFile), "testdata", "sql_golden.txt")
	if *updateGolden {
		if err := os.MkdirAll(filepath.Dir(path), 0o775); err != nil {
			t.Fatalf("create testdata: %v", err)
		}
		if err := os.WriteFile(path, []byte(got+"\n"), 0o644); err != nil {
			t.Fatalf("write golden: %v", err)
		}
		t.Logf("wrote %s (%d statements)", path, len(rec.statements()))
		return
	}

	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read golden (run `go test ./internal/queries -run TestSQLGolden -update` to create it): %v", err)
	}
	if strings.TrimSuffix(string(want), "\n") != got {
		t.Error("the emitted SQL changed; review it and run `go test ./internal/queries -run TestSQLGolden -update` if the change is intentional")
	}
}

// ignoreErr keeps the golden test readable: rows may legitimately be missing,
// and what is asserted is each statement's shape, not its result.
func ignoreErr(_ ...any) {}

// isUniqueViolation reports whether the insert collided with an existing row,
// which is how SQLite reports a duplicate primary key.
func isUniqueViolation(err error) bool {
	return err != nil && strings.Contains(err.Error(), "UNIQUE constraint failed")
}
