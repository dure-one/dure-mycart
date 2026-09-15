package handlers

import (
	"encoding/json"
	"io"
	"net/http"
	"testing"

	"github.com/shurco/mycart/internal/testutil"
)

func TestProducts(t *testing.T) {
	app, _, cleanup := testutil.SetupTestApp(t)
	defer cleanup()

	app.Get("/api/products", Products)

	tests := []struct {
		name       string
		query      string
		wantStatus int
	}{
		{"default list (fixtures have active products)", "", http.StatusOK},
		{"custom pagination", "?page=1&limit=5", http.StatusOK},
		{"high page (empty result)", "?page=999", http.StatusOK},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := testutil.DoRequest(t, app, http.MethodGet, "/api/products"+tt.query, "", "")
			testutil.AssertStatus(t, resp, tt.wantStatus)
		})
	}

	t.Run("includes active products without digital inventory", func(t *testing.T) {
		resp := testutil.DoRequest(t, app, http.MethodGet, "/api/products", "", "")
		defer func() { _ = resp.Body.Close() }()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
		}

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatalf("read body: %v", err)
		}

		var payload struct {
			Result struct {
				Total    int `json:"total"`
				Products []struct {
					Slug string `json:"slug"`
				} `json:"products"`
			} `json:"result"`
		}
		if err := json.Unmarshal(body, &payload); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}

		if payload.Result.Total < 7 {
			t.Fatalf("expected all active fixture products, got total=%d", payload.Result.Total)
		}

		found := false
		for _, product := range payload.Result.Products {
			if product.Slug == "url3" {
				found = true
				break
			}
		}
		if !found {
			t.Fatal("expected fixture product url3 without digital inventory in public list")
		}
	})
}

func TestProduct(t *testing.T) {
	app, _, cleanup := testutil.SetupTestApp(t)
	defer cleanup()

	app.Get("/api/products/:product_slug", Product)

	// A product the shop does not have, or has switched off, is not there: the
	// visitor gets a 404 and the storefront's own not-found page, not a server
	// error. The assertions used to accept either, which is how a 500 on every
	// mistyped address went unnoticed.
	tests := []struct {
		name        string
		productSlug string
		wantStatus  int
	}{
		{"active product with digital inventory", "url1", http.StatusOK},
		{"active product without digital inventory", "url3", http.StatusOK},
		{"inactive product", "url6", http.StatusNotFound},
		{"non-existent product", "nonexistent12345", http.StatusNotFound},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := testutil.DoRequest(t, app, http.MethodGet, "/api/products/"+tt.productSlug, "", "")
			testutil.AssertStatus(t, resp, tt.wantStatus)
		})
	}
}
