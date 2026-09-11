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
		{"negative page (clamped to 1)", "?page=-1", http.StatusOK},
		{"zero page (clamped to 1)", "?page=0", http.StatusOK},
		{"negative limit (clamped to default)", "?limit=-10", http.StatusOK},
		{"zero limit (clamped to default)", "?limit=0", http.StatusOK},
		{"very large limit (clamped to max)", "?limit=9999", http.StatusOK},
		{"invalid page parameter", "?page=invalid", http.StatusOK},
		{"invalid limit parameter", "?limit=invalid", http.StatusOK},
		{"page 2 with limit", "?page=2&limit=3", http.StatusOK},
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

	app.Get("/api/products/:product_id", Product)

	tests := []struct {
		name       string
		productID  string
		wantStatus []int
	}{
		{"active product with digital inventory", "url1", []int{http.StatusOK}},
		{"active product without digital inventory", "url3", []int{http.StatusOK}},
		{"inactive product", "url6", []int{http.StatusNotFound, http.StatusInternalServerError}},
		{"non-existent product", "nonexistent12345", []int{http.StatusNotFound, http.StatusInternalServerError}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := testutil.DoRequest(t, app, http.MethodGet, "/api/products/"+tt.productID, "", "")
			testutil.AssertStatus(t, resp, tt.wantStatus...)
		})
	}
}
