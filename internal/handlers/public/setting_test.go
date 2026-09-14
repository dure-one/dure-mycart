package handlers

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/shurco/mycart/internal/testutil"
)

func TestPing(t *testing.T) {
	app, _, cleanup := testutil.SetupTestApp(t)
	defer cleanup()

	app.Get("/ping", Ping)

	resp := testutil.DoRequest(t, app, http.MethodGet, "/ping", "", "")
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}

	var res struct {
		Success bool   `json:"success"`
		Message string `json:"message"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&res)

	if !res.Success {
		t.Error("expected success=true")
	}
	if res.Message != "Pong" {
		t.Errorf("message = %q, want Pong", res.Message)
	}
}

func TestSettings(t *testing.T) {
	app, _, cleanup := testutil.SetupTestApp(t)
	defer cleanup()

	app.Get("/api/settings", Settings)

	resp := testutil.DoRequest(t, app, http.MethodGet, "/api/settings", "", "")
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}

	var res struct {
		Success bool `json:"success"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&res)

	if !res.Success {
		t.Error("expected success=true")
	}
}

// publicSettings reads /api/settings into its named blocks.
func publicSettings(t *testing.T) map[string]any {
	t.Helper()

	app, _, cleanup := testutil.SetupTestApp(t)
	t.Cleanup(cleanup)
	app.Get("/api/settings", Settings)

	resp := testutil.DoRequest(t, app, http.MethodGet, "/api/settings", "", "")
	defer func() { _ = resp.Body.Close() }()

	var res struct {
		Success bool           `json:"success"`
		Result  map[string]any `json:"result"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		t.Fatalf("decode settings: %v", err)
	}
	if !res.Success {
		t.Fatal("expected success=true")
	}
	return res.Result
}

// TestSettings_BrandingIsAddressed checks the shape the storefront is handed:
// an uploaded mark comes back as an address under /uploads, and a shop that has
// uploaded nothing gets an empty string rather than "/uploads/", which would be
// a request for the directory.
func TestSettings_BrandingIsAddressed(t *testing.T) {
	settings := publicSettings(t)

	branding, ok := settings["branding"].(map[string]any)
	if !ok {
		t.Fatalf("branding = %T, want an object", settings["branding"])
	}
	for _, key := range []string{"logo", "favicon", "tagline"} {
		if _, ok := branding[key]; !ok {
			t.Errorf("branding has no %q", key)
		}
	}

	if got := branding["logo"]; got != "" {
		t.Errorf("logo = %v on a shop that has uploaded nothing, want an empty string", got)
	}
	if got := branding["favicon"]; got != "" {
		t.Errorf("favicon = %v on a shop that has uploaded nothing, want an empty string", got)
	}
}

// TestUploadURL covers the two inputs that are not an ordinary file name.
func TestUploadURL(t *testing.T) {
	if got := uploadURL(""); got != "" {
		t.Errorf("uploadURL(%q) = %q, want an empty string", "", got)
	}
	if got := uploadURL("abc.png"); got != "/uploads/abc.png" {
		t.Errorf("uploadURL(%q) = %q, want /uploads/abc.png", "abc.png", got)
	}
}
