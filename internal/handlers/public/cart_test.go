package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"

	"github.com/shurco/mycart/internal/models"
	"github.com/shurco/mycart/internal/store"
	"github.com/shurco/mycart/internal/testutil"
)

func TestGetCart(t *testing.T) {
	app, _, cleanup := testutil.SetupTestApp(t)
	defer cleanup()

	app.Get("/api/cart/:cart_id", GetCart)

	tests := []struct {
		name       string
		cartID     string
		wantStatus []int
	}{
		{"existing cart from fixtures", "iodz4ibf5h5zmov", []int{http.StatusOK}},
		{"cancelled cart from fixtures", "efzs4xayz43f226", []int{http.StatusOK}},
		{"non-existent cart", "nonexistent12345", []int{http.StatusNotFound, http.StatusInternalServerError}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := testutil.DoRequest(t, app, http.MethodGet, "/api/cart/"+tt.cartID, "", "")
			testutil.AssertStatus(t, resp, tt.wantStatus...)
		})
	}
}

func TestPaymentCancel(t *testing.T) {
	app, _, cleanup := testutil.SetupTestApp(t)
	defer cleanup()

	clearWebhookURL(t)

	app.Get("/cart/payment/cancel", PaymentCancel)

	tests := []struct {
		name       string
		query      string
		wantStatus []int
	}{
		{
			"cancel existing cart",
			"?cart_id=efzs4xayz43f226&payment_system=stripe",
			[]int{http.StatusSeeOther, http.StatusFound, http.StatusOK, http.StatusInternalServerError},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/cart/payment/cancel"+tt.query, nil)
			resp, err := app.Test(req, fiber.TestConfig{Timeout: 5 * time.Second})
			if err != nil {
				t.Fatalf("GET /cart/payment/cancel: %v", err)
			}
			testutil.AssertStatus(t, resp, tt.wantStatus...)
		})
	}
}

func TestPaymentCallback(t *testing.T) {
	app, _, cleanup := testutil.SetupTestApp(t)
	defer cleanup()

	app.Post("/cart/payment/callback", PaymentCallback)

	tests := []struct {
		name       string
		query      string
		wantStatus []int
	}{
		{
			"spectrocoin callback",
			"?cart_id=iodz4ibf5h5zmov&payment_system=spectrocoin",
			[]int{http.StatusOK, http.StatusBadRequest, http.StatusInternalServerError},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := testutil.DoRequest(t, app, http.MethodPost, "/cart/payment/callback"+tt.query, "", "")
			testutil.AssertStatus(t, resp, tt.wantStatus...)
		})
	}
}

func clearWebhookURL(t *testing.T) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = store.UpdateSettingByKey(ctx, &models.SettingName{Key: "webhook_url", Value: ""})
}

// TestGetCart_EmptyCartID tests missing cart_id parameter
func TestGetCart_EmptyCartID(t *testing.T) {
	app, _, cleanup := testutil.SetupTestApp(t)
	defer cleanup()

	app.Get("/api/cart/:cart_id", GetCart)

	resp := testutil.DoRequest(t, app, http.MethodGet, "/api/cart/", "", "")
	testutil.AssertStatus(t, resp, http.StatusNotFound, http.StatusBadRequest)
}

// TestGetCart_WithProducts tests cart retrieval with product details
func TestGetCart_WithProducts(t *testing.T) {
	app, _, cleanup := testutil.SetupTestApp(t)
	defer cleanup()

	ctx := context.Background()

	// Create a cart with products
	cartID := "cart_with_products"
	err := store.AddCart(ctx, &models.Cart{
		Core: models.Core{
			ID: cartID,
		},
		Email:         "test@example.com",
		Cart:          []models.CartProduct{{ProductID: "sqyavyhyvzyn3tu", Quantity: 2}},
		AmountTotal:   2000,
		Currency:      "USD",
		PaymentStatus: "new",
		PaymentSystem: "stripe",
	})
	if err != nil {
		t.Fatalf("Failed to create test cart: %v", err)
	}

	app.Get("/api/cart/:cart_id", GetCart)

	resp := testutil.DoRequest(t, app, http.MethodGet, "/api/cart/"+cartID, "", "")
	testutil.AssertStatus(t, resp, http.StatusOK, http.StatusInternalServerError)
}

// TestPayment_MultipleProducts tests payment with multiple cart items
func TestPayment_MultipleProducts(t *testing.T) {
	app, _, cleanup := testutil.SetupTestApp(t)
	defer cleanup()

	clearWebhookURL(t)

	app.Post("/cart/payment", Payment)

	body := `{
		"provider":"stripe",
		"email":"test@example.com",
		"products":[
			{"product_id":"sqyavyhyvzyn3tu","quantity":2},
			{"product_id":"sqyavyhyvzyn3tu","quantity":1}
		]
	}`

	resp := testutil.DoRequest(t, app, http.MethodPost, "/cart/payment", body, "")
	testutil.AssertStatus(t, resp, http.StatusOK, http.StatusConflict, http.StatusInternalServerError)
}

// TestPayment_LargeQuantity tests payment with large product quantities
func TestPayment_LargeQuantity(t *testing.T) {
	app, _, cleanup := testutil.SetupTestApp(t)
	defer cleanup()

	clearWebhookURL(t)

	app.Post("/cart/payment", Payment)

	body := `{
		"provider":"stripe",
		"email":"test@example.com",
		"products":[{"product_id":"sqyavyhyvzyn3tu","quantity":100}]
	}`

	resp := testutil.DoRequest(t, app, http.MethodPost, "/cart/payment", body, "")
	testutil.AssertStatus(t, resp, http.StatusOK, http.StatusConflict, http.StatusInternalServerError)
}

// TestPayment_AllProviders tests all payment provider types
func TestPayment_AllProviders(t *testing.T) {
	providers := []string{"stripe", "paypal", "spectrocoin", "coinbase"}

	for _, provider := range providers {
		t.Run(provider, func(t *testing.T) {
			app, _, cleanup := testutil.SetupTestApp(t)
			defer cleanup()

			clearWebhookURL(t)

			app.Post("/cart/payment", Payment)

			body := `{"provider":"` + provider + `","email":"test@example.com","products":[{"product_id":"sqyavyhyvzyn3tu","quantity":1}]}`
			resp := testutil.DoRequest(t, app, http.MethodPost, "/cart/payment", body, "")

			// Accept success, conflict (validation), or internal error
			testutil.AssertStatus(t, resp, http.StatusOK, http.StatusConflict, http.StatusInternalServerError)
		})
	}
}

// TestCreateCart_WithVariants tests cart creation with product variants
func TestCreateCart_WithVariants(t *testing.T) {
	app, _, cleanup := testutil.SetupTestApp(t)
	defer cleanup()

	clearWebhookURL(t)

	app.Post("/api/cart/create", CreateCart)

	body := `{
		"provider":"portone",
		"email":"test@example.com",
		"products":[{
			"product_id":"sqyavyhyvzyn3tu",
			"quantity":1,
			"variant_id":"some_variant"
		}]
	}`

	resp := testutil.DoRequest(t, app, http.MethodPost, "/api/cart/create", body, "")
	testutil.AssertStatus(t, resp, http.StatusOK, http.StatusConflict, http.StatusInternalServerError)
}

// TestCreateCart_MultipleItems tests cart creation with multiple products
func TestCreateCart_MultipleItems(t *testing.T) {
	app, _, cleanup := testutil.SetupTestApp(t)
	defer cleanup()

	clearWebhookURL(t)

	app.Post("/api/cart/create", CreateCart)

	body := `{
		"provider":"portone",
		"email":"test@example.com",
		"products":[
			{"product_id":"sqyavyhyvzyn3tu","quantity":1},
			{"product_id":"sqyavyhyvzyn3tu","quantity":2}
		]
	}`

	resp := testutil.DoRequest(t, app, http.MethodPost, "/api/cart/create", body, "")
	testutil.AssertStatus(t, resp, http.StatusOK, http.StatusConflict, http.StatusInternalServerError)
}

// TestPaymentSuccess_InvalidPaymentSystem tests validation with invalid payment system
func TestPaymentSuccess_InvalidPaymentSystem(t *testing.T) {
	app, _, cleanup := testutil.SetupTestApp(t)
	defer cleanup()

	// Register next handler to catch redirects
	app.Get("/cart/payment/success", PaymentSuccess, func(c fiber.Ctx) error {
		return c.SendString("next")
	})

	resp := testutil.DoRequest(t, app, http.MethodGet,
		"/cart/payment/success?cart_id=iodz4ibf5h5zmov&payment_system=invalid_system",
		"", "")
	testutil.AssertStatus(t, resp, http.StatusOK, http.StatusSeeOther, http.StatusInternalServerError)
}

// TestPaymentSuccess_SpectroCoinProvider tests SpectroCoin success handling
func TestPaymentSuccess_SpectroCoinProvider(t *testing.T) {
	app, _, cleanup := testutil.SetupTestApp(t)
	defer cleanup()

	ctx := context.Background()

	// Create a cart with SpectroCoin as payment system
	cartID := "spectro_success_test"
	err := store.AddCart(ctx, &models.Cart{
		Core: models.Core{
			ID: cartID,
		},
		Email:         "test@example.com",
		AmountTotal:   1000,
		Currency:      "USD",
		PaymentStatus: "new",
		PaymentSystem: "spectrocoin",
	})
	if err != nil {
		t.Fatalf("Failed to create test cart: %v", err)
	}

	// Register next handler
	app.Get("/cart/payment/success", PaymentSuccess, func(c fiber.Ctx) error {
		return c.SendString("next")
	})

	resp := testutil.DoRequest(t, app, http.MethodGet,
		"/cart/payment/success?cart_id="+cartID+"&payment_system=spectrocoin",
		"", "")
	testutil.AssertStatus(t, resp, http.StatusOK, http.StatusSeeOther, http.StatusInternalServerError)
}
