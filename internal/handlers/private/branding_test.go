package handlers

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"

	"github.com/shurco/mycart/internal/models"
	"github.com/shurco/mycart/internal/queries"
	"github.com/shurco/mycart/internal/testutil"
)

// uploadBranding posts a file to one of the two branding endpoints.
func uploadBranding(t *testing.T, app *fiber.App, path, mime, filename string) *http.Response {
	t.Helper()

	body, contentType := createTestImage(t, mime, filename)
	req := httptest.NewRequest(http.MethodPost, path, body)
	req.Header.Set("Content-Type", contentType)

	resp, err := app.Test(req, fiber.TestConfig{Timeout: 10 * time.Second})
	if err != nil {
		t.Fatalf("POST %s: %v", path, err)
	}
	return resp
}

// TestUploadBrandingLogo stores a logo, points the setting at it and puts the
// file where the storefront will look for it.
func TestUploadBrandingLogo(t *testing.T) {
	app, _, cleanup := testutil.SetupTestApp(t)
	defer cleanup()

	app.Post("/api/_/settings/branding/logo", UploadBrandingLogo)

	resp := uploadBranding(t, app, "/api/_/settings/branding/logo", "image/png", "logo.png")
	testutil.AssertStatus(t, resp, http.StatusOK)

	branding, err := queries.GetSettingByGroup[models.Branding](t.Context(), queries.DB())
	if err != nil {
		t.Fatalf("read branding: %v", err)
	}
	if branding.Logo == "" {
		t.Fatal("logo setting is empty after a successful upload")
	}

	// The name is the storefront's to build an address from, so it has to name
	// a file that is really there.
	if _, err := os.Stat(filepath.Join(dirUploads, branding.Logo)); err != nil {
		t.Errorf("stored logo %q is not in %s: %v", branding.Logo, dirUploads, err)
	}
}

// TestUploadBranding_ReplacesStoredFile checks that uploading twice leaves one
// file behind: lc_uploads is copied and backed up as a whole, and a logo
// nothing points at is weight every backup carries forever.
func TestUploadBranding_ReplacesStoredFile(t *testing.T) {
	app, _, cleanup := testutil.SetupTestApp(t)
	defer cleanup()

	app.Post("/api/_/settings/branding/logo", UploadBrandingLogo)

	first := uploadBranding(t, app, "/api/_/settings/branding/logo", "image/png", "first.png")
	testutil.AssertStatus(t, first, http.StatusOK)
	before, err := queries.GetSettingByGroup[models.Branding](t.Context(), queries.DB())
	if err != nil {
		t.Fatalf("read branding: %v", err)
	}

	second := uploadBranding(t, app, "/api/_/settings/branding/logo", "image/png", "second.png")
	testutil.AssertStatus(t, second, http.StatusOK)
	after, err := queries.GetSettingByGroup[models.Branding](t.Context(), queries.DB())
	if err != nil {
		t.Fatalf("read branding: %v", err)
	}

	if after.Logo == before.Logo {
		t.Fatal("the second upload kept the first file name")
	}
	if _, err := os.Stat(filepath.Join(dirUploads, before.Logo)); !os.IsNotExist(err) {
		t.Errorf("the replaced file %q is still in %s", before.Logo, dirUploads)
	}
	if _, err := os.Stat(filepath.Join(dirUploads, after.Logo)); err != nil {
		t.Errorf("the replacement %q is not in %s: %v", after.Logo, dirUploads, err)
	}
}

// TestUploadBranding_RejectsNonImage keeps the upload on the same terms as a
// product image: the storefront serves whatever lands in lc_uploads from its
// own origin, so only the formats it can be sure of are stored.
func TestUploadBranding_RejectsNonImage(t *testing.T) {
	app, _, cleanup := testutil.SetupTestApp(t)
	defer cleanup()

	app.Post("/api/_/settings/branding/logo", UploadBrandingLogo)

	body, contentType := createTestImageBadMIME(t)
	req := httptest.NewRequest(http.MethodPost, "/api/_/settings/branding/logo", body)
	req.Header.Set("Content-Type", contentType)

	resp, err := app.Test(req, fiber.TestConfig{Timeout: 10 * time.Second})
	if err != nil {
		t.Fatalf("POST branding/logo: %v", err)
	}
	testutil.AssertStatus(t, resp, http.StatusBadRequest)

	branding, err := queries.GetSettingByGroup[models.Branding](t.Context(), queries.DB())
	if err != nil {
		t.Fatalf("read branding: %v", err)
	}
	if branding.Logo != "" {
		t.Errorf("logo = %q after a refused upload, want it left empty", branding.Logo)
	}
}

// TestDeleteBrandingLogo clears the setting and takes the file with it.
func TestDeleteBrandingLogo(t *testing.T) {
	app, _, cleanup := testutil.SetupTestApp(t)
	defer cleanup()

	app.Post("/api/_/settings/branding/logo", UploadBrandingLogo)
	app.Delete("/api/_/settings/branding/logo", DeleteBrandingLogo)

	uploadBranding(t, app, "/api/_/settings/branding/logo", "image/png", "logo.png")
	uploaded, err := queries.GetSettingByGroup[models.Branding](t.Context(), queries.DB())
	if err != nil {
		t.Fatalf("read branding: %v", err)
	}

	resp := testutil.DoRequest(t, app, http.MethodDelete, "/api/_/settings/branding/logo", "", "")
	testutil.AssertStatus(t, resp, http.StatusOK)

	branding, err := queries.GetSettingByGroup[models.Branding](t.Context(), queries.DB())
	if err != nil {
		t.Fatalf("read branding: %v", err)
	}
	if branding.Logo != "" {
		t.Errorf("logo = %q after a delete, want it cleared", branding.Logo)
	}
	if _, err := os.Stat(filepath.Join(dirUploads, uploaded.Logo)); !os.IsNotExist(err) {
		t.Errorf("the deleted file %q is still in %s", uploaded.Logo, dirUploads)
	}
}

// TestUpdateSetting_BrandingKeysAreGroupOnly checks that the marks cannot be
// written one key at a time. The raw key/value fallback does not validate, and
// the check that a value is a bare file name lives on the group — so the
// fallback has to refuse these keys, or that check protects nothing.
func TestUpdateSetting_BrandingKeysAreGroupOnly(t *testing.T) {
	app, _, cleanup := testutil.SetupTestApp(t)
	defer cleanup()

	app.Patch("/api/_/settings/:setting_key", UpdateSetting)

	for _, key := range []string{"branding_logo", "branding_favicon", "branding_tagline"} {
		resp := testutil.DoRequest(t, app, http.MethodPatch, "/api/_/settings/"+key,
			`{"value":"../../etc/passwd"}`, "")
		testutil.AssertStatus(t, resp, http.StatusBadRequest)
	}
}

// TestUpdateSetting_BrandingGroupRejectsPath checks the validation the guard
// above exists to protect: a value that is a path rather than a file name is
// refused, because the storefront turns it into an address under /uploads.
func TestUpdateSetting_BrandingGroupRejectsPath(t *testing.T) {
	app, _, cleanup := testutil.SetupTestApp(t)
	defer cleanup()

	app.Patch("/api/_/settings/:setting_key", UpdateSetting)

	for _, body := range []string{
		`{"logo":"../../etc/passwd"}`,
		`{"favicon":"sub/dir.png"}`,
		`{"logo":".."}`,
	} {
		resp := testutil.DoRequest(t, app, http.MethodPatch, "/api/_/settings/branding", body, "")
		testutil.AssertStatus(t, resp, http.StatusBadRequest)
	}
}

// TestGetSetting_BrandingSendsEveryKey checks the shape the panel reads: a shop
// that has uploaded nothing still gets logo, favicon and tagline, each an empty
// string. A group that dropped its empty members would answer {}, and the form
// binds its fields to those values — a missing key binds a field to undefined
// rather than to an empty one, which the page cannot render.
func TestGetSetting_BrandingSendsEveryKey(t *testing.T) {
	app, _, cleanup := testutil.SetupTestApp(t)
	defer cleanup()

	app.Get("/api/_/settings/:setting_key", GetSetting)

	resp := testutil.DoRequest(t, app, http.MethodGet, "/api/_/settings/branding", "", "")
	// Read before asserting: AssertStatus closes the body.
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read branding: %v", err)
	}
	testutil.AssertStatus(t, resp, http.StatusOK)

	var res struct {
		Success bool           `json:"success"`
		Result  map[string]any `json:"result"`
	}
	if err := json.Unmarshal(raw, &res); err != nil {
		t.Fatalf("decode branding: %v", err)
	}

	for _, key := range []string{"logo", "favicon", "tagline"} {
		value, ok := res.Result[key]
		if !ok {
			t.Errorf("branding has no %q; a form binding that field gets undefined", key)
			continue
		}
		if value != "" {
			t.Errorf("%s = %v on a shop that has uploaded nothing, want an empty string", key, value)
		}
	}
}
