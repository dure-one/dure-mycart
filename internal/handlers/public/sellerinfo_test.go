package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/dure-one/dure-mycart/internal/models"
	"github.com/dure-one/dure-mycart/internal/queries"
	"github.com/dure-one/dure-mycart/internal/testutil"
)

func TestGetSellerInfo_DirectAccess(t *testing.T) {
	app, _, cleanup := testutil.SetupTestApp(t)
	defer cleanup()

	app.Get("/api/sellerinfo", GetSellerInfo)

	// Enable sellerinfo with test data
	db := queries.DB()
	sellerinfo := &models.Sellerinfo{
		Enabled:           true,
		BusinessName:      "Test Business",
		Representative:    "John Doe",
		CustomerService:   "1234-5678",
		BusinessRegNumber: "123-45-67890",
		BusinessAddress:   "123 Test St",
		EcommerceLicense:  "2024-Seoul-1234",
		Email:             "contact@test.com",
	}
	if err := db.UpdateSettingByGroup(context.Background(), sellerinfo); err != nil {
		t.Fatalf("setup sellerinfo: %v", err)
	}

	// Test: GET without X-Access-Token header should work
	resp := testutil.DoRequest(t, app, http.MethodGet, "/api/sellerinfo", "", "")
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}

	var res struct {
		Success bool              `json:"success"`
		Message string            `json:"message"`
		Result  map[string]string `json:"result"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if !res.Success {
		t.Error("expected success=true")
	}
	if res.Result["business_name"] != "Test Business" {
		t.Errorf("business_name = %q, want Test Business", res.Result["business_name"])
	}
	if res.Result["email"] != "contact@test.com" {
		t.Errorf("email = %q, want contact@test.com", res.Result["email"])
	}
}

func TestGetSellerInfo_Disabled(t *testing.T) {
	app, _, cleanup := testutil.SetupTestApp(t)
	defer cleanup()

	app.Get("/api/sellerinfo", GetSellerInfo)

	// Disable sellerinfo
	db := queries.DB()
	sellerinfo := &models.Sellerinfo{
		Enabled: false,
	}
	if err := db.UpdateSettingByGroup(context.Background(), sellerinfo); err != nil {
		t.Fatalf("setup sellerinfo: %v", err)
	}

	resp := testutil.DoRequest(t, app, http.MethodGet, "/api/sellerinfo", "", "")
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", resp.StatusCode)
	}
}

func TestGenerateCaptcha(t *testing.T) {
	app, _, cleanup := testutil.SetupTestApp(t)
	defer cleanup()

	app.Get("/api/sellerinfo/captcha", GenerateCaptcha)

	resp := testutil.DoRequest(t, app, http.MethodGet, "/api/sellerinfo/captcha", "", "")
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}

	var res struct {
		Success bool `json:"success"`
		Result  struct {
			Token       string `json:"token"`
			MasterImage string `json:"master_image"`
			ThumbImage  string `json:"thumb_image"`
		} `json:"result"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if !res.Success {
		t.Error("expected success=true")
	}
	if res.Result.Token == "" {
		t.Error("expected non-empty token")
	}
	if res.Result.MasterImage == "" {
		t.Error("expected non-empty master_image")
	}
}
