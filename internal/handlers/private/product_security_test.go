package handlers

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shurco/mycart/internal/models"
	"github.com/shurco/mycart/internal/queries"
	"github.com/shurco/mycart/internal/testutil"
	"github.com/shurco/mycart/pkg/litepay"
	"github.com/shurco/mycart/pkg/security"
)

// seedProductWithDigital inserts a product plus an attached digital file
// record and returns (productID, digitalFileID). When soldCartID is not
// empty, a purchased license key assigned to that cart is added too.
func seedProductWithDigital(t *testing.T, soldCartID string) (string, string) {
	t.Helper()

	ctx := context.Background()
	db := queries.DB()

	productID := security.RandomString()
	if _, err := db.ProductQueries.ExecContext(ctx, `
		INSERT INTO product (id, name, slug, desc, amount, quantity, digital, active, deleted)
		VALUES (?, 'Guard Product', ?, 'desc', 1000, 1, 'file', 1, 0)
	`, productID, "guard-"+productID); err != nil {
		t.Fatalf("insert product: %v", err)
	}

	fileID := security.RandomString()
	if _, err := db.ProductQueries.ExecContext(ctx,
		`INSERT INTO digital_file (id, product_id, name, ext, orig_name) VALUES (?, ?, ?, 'pdf', 'manual.pdf')`,
		fileID, productID, security.RandomString()+"-uuid"); err != nil {
		t.Fatalf("insert digital_file: %v", err)
	}

	if soldCartID != "" {
		if _, err := db.ProductQueries.ExecContext(ctx,
			`INSERT INTO digital_data (id, product_id, content, cart_id) VALUES (?, ?, 'KEY-001', ?)`,
			security.RandomString(), productID, soldCartID); err != nil {
			t.Fatalf("insert digital_data: %v", err)
		}
	}

	return productID, fileID
}

// fileUUIDFor looks up the stored uuid name of a digital file row.
func fileUUIDFor(t *testing.T, productID, fileID string) string {
	t.Helper()

	var name string
	err := queries.DB().ProductQueries.QueryRowContext(context.Background(),
		`SELECT name FROM digital_file WHERE id = ? AND product_id = ?`, fileID, productID).Scan(&name)
	if err != nil {
		t.Fatalf("load digital file name: %v", err)
	}
	return name
}

func TestDownloadProductDigital(t *testing.T) {
	app, cookie, cleanup := testutil.SetupTestApp(t)
	defer cleanup()

	productID, fileID := seedProductWithDigital(t, "")

	payload := []byte("%PDF-1.4 fake manual content")
	if err := os.MkdirAll(dirDigitals, 0o775); err != nil {
		t.Fatalf("mkdir digitals: %v", err)
	}
	fileName := fileUUIDFor(t, productID, fileID) + ".pdf"
	if err := os.WriteFile(filepath.Join(dirDigitals, fileName), payload, 0o644); err != nil {
		t.Fatalf("write digital file: %v", err)
	}

	app.Get("/api/_/products/:product_id/digital/:digital_id/download", DownloadProductDigital)

	path := "/api/_/products/" + productID + "/digital/" + fileID + "/download"

	t.Run("streams file as attachment", func(t *testing.T) {
		resp := testutil.DoRequest(t, app, http.MethodGet, path, "", cookie)
		defer func() { _ = resp.Body.Close() }()

		testutil.AssertStatus(t, resp, http.StatusOK)

		if cd := resp.Header.Get("Content-Disposition"); !strings.Contains(cd, "attachment") {
			t.Errorf("Content-Disposition = %q, want attachment", cd)
		}
		if ct := resp.Header.Get("Content-Type"); ct != "application/octet-stream" {
			t.Errorf("Content-Type = %q, want application/octet-stream", ct)
		}
	})

	t.Run("unknown file returns 404", func(t *testing.T) {
		resp := testutil.DoRequest(t, app, http.MethodGet,
			"/api/_/products/"+productID+"/digital/"+security.RandomString()+"/download", "", cookie)
		testutil.AssertStatus(t, resp, http.StatusNotFound)
	})
}

func TestDeleteProduct_SoldDigitalGuard(t *testing.T) {
	app, cookie, cleanup := testutil.SetupTestApp(t)
	defer cleanup()

	db := queries.DB()
	ctx := context.Background()

	// A cart owning purchased keys must exist to satisfy the FK.
	soldCartID := "soldcart0000001" // len 15
	if err := db.AddCart(ctx, &models.Cart{
		Core:          models.Core{ID: soldCartID},
		Email:         "buyer@example.com",
		Cart:          []models.CartProduct{},
		AmountTotal:   1000,
		Currency:      "USD",
		PaymentStatus: litepay.PAID,
		PaymentSystem: litepay.STRIPE,
	}); err != nil {
		t.Fatalf("seed cart: %v", err)
	}

	soldProductID, _ := seedProductWithDigital(t, soldCartID)
	cleanProductID, _ := seedProductWithDigital(t, "")

	app.Delete("/api/_/products/:product_id", DeleteProduct)

	t.Run("product with sold keys cannot be deleted", func(t *testing.T) {
		resp := testutil.DoRequest(t, app, http.MethodDelete, "/api/_/products/"+soldProductID, "", cookie)
		testutil.AssertStatus(t, resp, http.StatusBadRequest)

		var exists bool
		if err := db.ProductQueries.QueryRowContext(ctx,
			`SELECT EXISTS(SELECT 1 FROM product WHERE id = ?)`, soldProductID).Scan(&exists); err != nil {
			t.Fatalf("check product: %v", err)
		}
		if !exists {
			t.Fatal("guarded product was deleted")
		}
	})

	t.Run("clean product deletes fine", func(t *testing.T) {
		resp := testutil.DoRequest(t, app, http.MethodDelete, "/api/_/products/"+cleanProductID, "", cookie)
		testutil.AssertStatus(t, resp, http.StatusOK)

		var exists bool
		if err := db.ProductQueries.QueryRowContext(ctx,
			`SELECT EXISTS(SELECT 1 FROM product WHERE id = ?)`, cleanProductID).Scan(&exists); err != nil {
			t.Fatalf("check product: %v", err)
		}
		if exists {
			t.Fatal("clean product was not deleted")
		}
	})
}
