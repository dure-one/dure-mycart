package queries

import (
	"strings"
	"testing"

	"github.com/shurco/mycart/internal/models"
	"github.com/shurco/mycart/pkg/security"
)

// Product is the query behind both the admin edit form and the storefront
// product page, and it is the only place all of a product's parts are assembled:
// images, attributes, metadata, SEO, variants and their option values. This
// walks a product through it with every one of those fields populated.
func TestProduct_AssemblesEveryPart(t *testing.T) {
	db, ctx := bootstrap(t)

	product := &models.Product{
		Core:        models.Core{ID: security.RandomString()},
		Name:        "Complete product",
		Brief:       "brief",
		Description: "description",
		Slug:        "complete-product",
		Amount:      4200,
		Quantity:    9,
		SKU:         "COMPLETE-1",
		HasVariants: true,
		Digital:     models.Digital{Type: "file"},
		Metadata:    []models.Metadata{{Key: "material", Value: "cotton"}},
		Attributes:  []string{"machine washable"},
		Seo:         &models.Seo{Title: "Complete", Keywords: "product", Description: "seo description"},
		Options: []models.ProductOption{
			{
				ID:       security.RandomString(),
				Name:     "Size",
				Position: 0,
				Values: []models.ProductOptionValue{
					{ID: security.RandomString(), Value: "S", Position: 0},
					{ID: security.RandomString(), Value: "M", Position: 1},
				},
			},
		},
		Variants: []models.ProductVariant{
			{
				ID:             security.RandomString(),
				SKU:            "COMPLETE-S",
				OptionValues:   map[string]string{"Size": "S"},
				PriceSurcharge: 0,
				Quantity:       4,
				Active:         true,
			},
			{
				ID:             security.RandomString(),
				SKU:            "COMPLETE-M",
				OptionValues:   map[string]string{"Size": "M"},
				PriceSurcharge: 300,
				Quantity:       5,
				Active:         true,
			},
		},
	}

	if _, err := db.AddProductWithVariants(ctx, product); err != nil {
		t.Fatalf("AddProductWithVariants: %v", err)
	}
	// AddProductWithVariants stores the variant tree but not the fields below,
	// which only the admin form sends. Product() has to read them from the main
	// row, so they are set here.
	if _, err := db.conn.ExecContext(ctx,
		`UPDATE product SET quantity = ?, sku = ?, seo = ?, attribute = ?, metadata = ? WHERE id = ?`,
		product.Quantity, product.SKU, `{"title":"Complete","keywords":"product","description":"seo description"}`,
		`["machine washable"]`, `[{"key":"material","value":"cotton"}]`, product.ID); err != nil {
		t.Fatalf("fill in the extra columns: %v", err)
	}

	image, err := db.AddImage(ctx, product.ID, "11111111-1111-1111-1111-111111111111", "jpg", "front.jpg")
	if err != nil {
		t.Fatalf("AddImage: %v", err)
	}

	// Private: looked up by id, and it reports whether digital content is there.
	got, err := db.Product(ctx, true, product.ID)
	if err != nil {
		t.Fatalf("Product(private): %v", err)
	}
	if got.Quantity != product.Quantity {
		t.Errorf("quantity = %d, want %d", got.Quantity, product.Quantity)
	}
	if got.SKU != product.SKU {
		t.Errorf("sku = %q, want %q", got.SKU, product.SKU)
	}
	// The images subquery carries what a url needs — id, name and extension —
	// and not the uploader's original filename.
	if len(got.Images) != 1 || got.Images[0].ID != image.ID ||
		got.Images[0].Name != image.Name || got.Images[0].Ext != "jpg" {
		t.Errorf("images = %+v, want the one image just added", got.Images)
	}
	if len(got.Metadata) != 1 || got.Metadata[0].Key != "material" {
		t.Errorf("metadata = %+v", got.Metadata)
	}
	if len(got.Attributes) != 1 || got.Attributes[0] != "machine washable" {
		t.Errorf("attributes = %+v", got.Attributes)
	}
	if got.Seo == nil || got.Seo.Title != "Complete" || got.Seo.Keywords != "product" {
		t.Errorf("seo = %+v", got.Seo)
	}
	if !got.HasVariants {
		t.Error("has_variants was not read back")
	}
	if len(got.Options) != 1 || got.Options[0].Name != "Size" {
		t.Fatalf("options = %+v", got.Options)
	}
	if len(got.Options[0].Values) != 2 {
		t.Fatalf("option values = %+v", got.Options[0].Values)
	}
	if got.Options[0].Values[0].Value != "S" || got.Options[0].Values[1].Value != "M" {
		t.Errorf("option values came back out of order: %+v", got.Options[0].Values)
	}
	if len(got.Variants) != 2 {
		t.Fatalf("variants = %+v", got.Variants)
	}
	var seenSurcharge bool
	for _, v := range got.Variants {
		if v.SKU == "COMPLETE-M" && v.PriceSurcharge == 300 && v.Quantity == 5 {
			seenSurcharge = true
		}
	}
	if !seenSurcharge {
		t.Errorf("the second variant is missing or wrong: %+v", got.Variants)
	}
	// No digital rows exist yet, so the admin is told the product has none.
	if got.Digital.Filled {
		t.Error("digital.filled is true although the product has no digital content")
	}

	if _, err := db.AddDigitalFile(ctx, product.ID, "feedface", "pdf", "manual.pdf"); err != nil {
		t.Fatalf("AddDigitalFile: %v", err)
	}
	got, err = db.Product(ctx, true, product.ID)
	if err != nil {
		t.Fatalf("Product(private) after adding a file: %v", err)
	}
	if !got.Digital.Filled {
		t.Error("digital.filled is false although the product has a digital file")
	}

	// The public lookup is by slug, and only sees an active, undeleted product.
	if _, err := db.Product(ctx, false, product.Slug); err == nil {
		t.Error("an inactive product was served to the storefront")
	}
	if err := db.UpdateActive(ctx, product.ID); err != nil {
		t.Fatalf("UpdateActive: %v", err)
	}

	pub, err := db.Product(ctx, false, product.Slug)
	if err != nil {
		t.Fatalf("Product(public): %v", err)
	}
	if pub.ID != product.ID {
		t.Errorf("public lookup returned %q, want %q", pub.ID, product.ID)
	}
	if !pub.Active {
		t.Error("the storefront was served the product as inactive")
	}
	// digital.filled is a private-only field: the public query does not compute
	// it, so the storefront is not told about the shop's digital inventory.
	if pub.Digital.Filled {
		t.Error("the public query computed digital.filled")
	}

	if _, err := db.Product(ctx, false, "no-such-slug"); err == nil {
		t.Error("an unknown slug was served")
	} else if !strings.Contains(err.Error(), "not found") && err.Error() != "product not found" {
		t.Errorf("unexpected error for an unknown slug: %v", err)
	}
}
