package queries

import (
	"context"
	"database/sql"
	"testing"

	"github.com/shurco/mycart/internal/models"
	"github.com/shurco/mycart/pkg/security"
)

// Creating a product has to keep what the request carries: the admin panel's
// add form, its active checkbox and the CSV importer all hand over a whole
// product in one call, and a column that is silently dropped is a checkbox
// that does nothing.
func TestAddProduct_PersistsStockSKUAndSEO(t *testing.T) {
	db, ctx := bootstrap(t)

	in := validProductInput()
	in.Quantity = 7
	in.SKU = "E2E-SKU-1"
	in.Active = true
	in.Seo = &models.Seo{Title: "title", Keywords: "keywords", Description: "description"}

	created, err := db.AddProduct(ctx, in)
	if err != nil {
		t.Fatalf("AddProduct: %v", err)
	}

	got, err := db.Product(ctx, true, created.ID)
	if err != nil {
		t.Fatalf("Product: %v", err)
	}

	if got.Quantity != 7 {
		t.Errorf("quantity: got %d, want 7", got.Quantity)
	}
	if got.SKU != "E2E-SKU-1" {
		t.Errorf("sku: got %q, want %q", got.SKU, "E2E-SKU-1")
	}
	if !got.Active {
		t.Error("a product created active came back inactive")
	}
	if got.Seo == nil || got.Seo.Title != "title" {
		t.Errorf("seo: got %+v, want a title of %q", got.Seo, "title")
	}
}

// The unique index on sku covers every value that is not NULL, so a product
// without a SKU has to leave the column empty in the SQL sense: storing the
// empty string would make the second such product a duplicate of the first.
func TestAddProduct_WithoutSKU(t *testing.T) {
	db, ctx := bootstrap(t)

	first := validProductInput()
	first.Slug = "no-sku-first"

	second := validProductInput()
	second.Name = "Mug"
	second.Slug = "no-sku-second"

	for _, in := range []*models.Product{first, second} {
		if _, err := db.AddProduct(ctx, in); err != nil {
			t.Fatalf("AddProduct(%s): %v", in.Slug, err)
		}
	}

	if sku := storedSKU(t, second.ID); sku != nil {
		t.Errorf("sku: got %q, want NULL", *sku)
	}
}

// The variant path writes the same columns as the simple one. SEO used to be
// dropped there, an absent SKU was stored as an empty string, and neither was
// covered by a test.
func TestAddProductWithVariants_PersistsSEOAndMissingSKUs(t *testing.T) {
	db, ctx := bootstrap(t)

	for _, slug := range []string{"variant-no-sku-first", "variant-no-sku-second"} {
		product := skuLessVariantProduct(slug)
		// A SKU is optional on a variant as well: two of them without one
		// belong to the same product and must both be stored.
		product.Seo = &models.Seo{Title: "variant " + slug}

		if _, err := db.AddProductWithVariants(ctx, product); err != nil {
			t.Fatalf("AddProductWithVariants(%s): %v", slug, err)
		}

		if sku := storedSKU(t, product.ID); sku != nil {
			t.Errorf("%s: product sku: got %q, want NULL", slug, *sku)
		}

		got, err := db.Product(ctx, true, product.ID)
		if err != nil {
			t.Fatalf("Product(%s): %v", slug, err)
		}
		if got.Seo == nil || got.Seo.Title != "variant "+slug {
			t.Errorf("%s: seo: got %+v, want a title of %q", slug, got.Seo, "variant "+slug)
		}

		var stored int
		if err := db.ProductQueries.DB.QueryRowContext(ctx,
			"SELECT COUNT(*) FROM product_variant WHERE product_id = ? AND sku IS NULL", product.ID).Scan(&stored); err != nil {
			t.Fatalf("count variants without a SKU: %v", err)
		}
		if stored != len(product.Variants) {
			t.Errorf("%s: %d of %d variants stored without a SKU", slug, stored, len(product.Variants))
		}
	}
}

// skuLessVariantProduct is a product with options and variants, none of which
// names a SKU — the shape the admin form sends when the fields are left empty.
func skuLessVariantProduct(slug string) *models.Product {
	return &models.Product{
		Core:        models.Core{ID: security.RandomString()},
		Name:        "Variant product " + slug,
		Description: "description",
		Slug:        slug,
		Amount:      1000,
		HasVariants: true,
		Digital:     models.Digital{Type: "file"},
		Options: []models.ProductOption{
			{
				ID:       security.RandomString(),
				Name:     "Size",
				Position: 0,
				Values: []models.ProductOptionValue{
					{ID: security.RandomString(), Value: "Small", Position: 0},
					{ID: security.RandomString(), Value: "Large", Position: 1},
				},
			},
		},
		Variants: []models.ProductVariant{
			{
				ID:             security.RandomString(),
				OptionValues:   map[string]string{"Size": "Small"},
				PriceSurcharge: 0,
				Quantity:       3,
				Active:         true,
			},
			{
				ID:             security.RandomString(),
				OptionValues:   map[string]string{"Size": "Large"},
				PriceSurcharge: 400,
				Quantity:       7,
				Active:         true,
			},
		},
	}
}

// storedSKU reads the sku column as the database has it: nil means NULL.
func storedSKU(t *testing.T, productID string) *string {
	t.Helper()

	var sku sql.NullString
	if err := DB().ProductQueries.DB.QueryRowContext(context.Background(),
		"SELECT sku FROM product WHERE id = ?", productID).Scan(&sku); err != nil {
		t.Fatalf("read sku: %v", err)
	}
	if !sku.Valid {
		return nil
	}
	return &sku.String
}
