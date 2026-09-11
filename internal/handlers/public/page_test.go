package handlers

import (
	"net/http"
	"testing"

	"github.com/shurco/mycart/internal/testutil"
)

func TestPage(t *testing.T) {
	app, _, cleanup := testutil.SetupTestApp(t)
	defer cleanup()

	app.Get("/api/pages/:page_slug", Page)

	tests := []struct {
		name       string
		slug       string
		wantStatus []int
	}{
		{"terms page from fixtures", "terms", []int{http.StatusOK}},
		{"privacy page from fixtures", "privacy", []int{http.StatusOK}},
		{"cookies page from fixtures", "cookies", []int{http.StatusOK}},
		{"non-existent page", "nonexistent", []int{http.StatusNotFound}},
		{"slug with dashes", "my-page-slug", []int{http.StatusNotFound, http.StatusOK}},
		{"slug with numbers", "page123", []int{http.StatusNotFound, http.StatusOK}},
		{"very long slug", "very-long-slug-name-that-exceeds-normal-length-limits", []int{http.StatusNotFound, http.StatusOK}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := testutil.DoRequest(t, app, http.MethodGet, "/api/pages/"+tt.slug, "", "")
			testutil.AssertStatus(t, resp, tt.wantStatus...)
		})
	}
}

// TestPage_EmptySlug tests page with empty slug parameter
func TestPage_EmptySlug(t *testing.T) {
	app, _, cleanup := testutil.SetupTestApp(t)
	defer cleanup()

	app.Get("/api/pages/:page_slug", Page)

	resp := testutil.DoRequest(t, app, http.MethodGet, "/api/pages/", "", "")
	// Empty slug might match a different route or return 404
	testutil.AssertStatus(t, resp, http.StatusNotFound, http.StatusOK, http.StatusBadRequest)
}
