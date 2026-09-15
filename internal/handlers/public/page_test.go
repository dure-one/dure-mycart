package handlers

import (
	"context"
	"net/http"
	"testing"

	"github.com/shurco/mycart/internal/queries"
	"github.com/shurco/mycart/internal/testutil"
)

func TestPage(t *testing.T) {
	app, _, cleanup := testutil.SetupTestApp(t)
	defer cleanup()

	app.Get("/api/pages/:page_slug", Page)

	// A page the operator has written and not switched on. The storefront asks
	// this endpoint for every address it has no route of its own for, so a
	// draft answered here is published at its slug the moment it is created,
	// long before the operator decides it is ready — and it is published
	// silently, because the list the navigation is built from leaves it out.
	if _, err := queries.DB().PageQueries.DB.ExecContext(context.Background(), `
		INSERT INTO page (id, name, slug, position, content, active)
		VALUES ('draftpage000001', 'Delivery Draft', 'delivery-draft', 'footer', '<p>draft</p>', FALSE)`); err != nil {
		t.Fatalf("seed the page that is not switched on: %v", err)
	}

	tests := []struct {
		name       string
		slug       string
		wantStatus []int
	}{
		{"terms page from fixtures", "terms", []int{http.StatusOK}},
		{"privacy page from fixtures", "privacy", []int{http.StatusOK}},
		{"cookies page from fixtures", "cookies", []int{http.StatusOK}},
		{"page not switched on", "delivery-draft", []int{http.StatusNotFound}},
		{"non-existent page", "nonexistent", []int{http.StatusNotFound}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := testutil.DoRequest(t, app, http.MethodGet, "/api/pages/"+tt.slug, "", "")
			testutil.AssertStatus(t, resp, tt.wantStatus...)
		})
	}
}
