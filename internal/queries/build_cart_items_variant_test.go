package queries

import (
	"testing"

	"github.com/shurco/mycart/internal/models"
)

// A cart line for a variant carries the variant identity, not the product's:
// two sizes of the same shirt must render as two rows with different ids and
// their own prices.
func TestBuildCartItems_Variants(t *testing.T) {
	t.Parallel()

	small, large := "var-small", "var-large"
	products := &models.Products{
		Products: []models.Product{
			{
				Core:   models.Core{ID: "p1"},
				Name:   "T-shirt",
				Slug:   "t-shirt",
				Amount: 2000,
				Images: []models.File{{ID: "i1", Name: "shirt", Ext: "jpg"}},
				Variants: []models.ProductVariant{
					{
						ID:             small,
						SKU:            "TS-S",
						OptionValues:   map[string]string{"Size": "S", "Color": "Black"},
						PriceSurcharge: 0,
						Quantity:       4,
						Active:         true,
					},
					{
						ID:             large,
						SKU:            "TS-L",
						OptionValues:   map[string]string{"Size": "L"},
						PriceSurcharge: 500,
						Quantity:       2,
						Active:         true,
					},
				},
			},
		},
	}

	cart := &models.Cart{Cart: []models.CartProduct{
		{ProductID: "p1", VariantID: &small, Quantity: 3},
		{ProductID: "p1", VariantID: &large, Quantity: 1},
		// A variant that has since been deleted from the product.
		{ProductID: "p1", VariantID: new(string), Quantity: 1},
		// A plain line on the same product.
		{ProductID: "p1", Quantity: 2},
	}}

	items := BuildCartItems(cart, products)
	if len(items) != 4 {
		t.Fatalf("items = %d, want 4: %+v", len(items), items)
	}

	// Each variant line is its own row, keyed by product and variant.
	if items[0]["id"] != "p1_var-small" || items[1]["id"] != "p1_var-large" {
		t.Errorf("variant rows are not keyed by product and variant: %v, %v", items[0]["id"], items[1]["id"])
	}
	if items[3]["id"] != "p1" {
		t.Errorf("a plain line has id %v, want the product id", items[3]["id"])
	}

	// The variant line carries what the storefront needs to price it.
	if items[0]["variant_sku"] != "TS-S" {
		t.Errorf("variant_sku = %v", items[0]["variant_sku"])
	}
	options, ok := items[0]["variant_options"].(map[string]string)
	if !ok || options["Size"] != "S" || options["Color"] != "Black" {
		t.Errorf("variant_options = %v", items[0]["variant_options"])
	}
	if items[0]["variant_price_surcharge"] != 0 || items[1]["variant_price_surcharge"] != 500 {
		t.Errorf("price surcharges = %v, %v", items[0]["variant_price_surcharge"], items[1]["variant_price_surcharge"])
	}
	if items[0]["quantity"] != 3 || items[1]["quantity"] != 1 {
		t.Errorf("quantities = %v, %v", items[0]["quantity"], items[1]["quantity"])
	}
	if items[0]["amount"] != 2000 {
		t.Errorf("amount = %v, want the base price, with the surcharge reported separately", items[0]["amount"])
	}

	// An empty variant id is not a variant: the line is the product itself, and
	// is keyed like one.
	if items[2]["id"] != "p1" {
		t.Errorf("id = %v, want the product id for an empty variant id", items[2]["id"])
	}
	if _, ok := items[2]["variant_id"]; ok {
		t.Errorf("an empty variant id was reported as a variant: %+v", items[2])
	}
	if _, ok := items[2]["variant_sku"]; ok {
		t.Errorf("a line without a variant contributed a sku: %+v", items[2])
	}

	// A variant without option values contributes no variant_options key at all,
	// rather than an empty map the storefront would have to special-case.
	if _, ok := items[1]["variant_sku"]; !ok {
		t.Error("the large variant lost its sku")
	}
}
