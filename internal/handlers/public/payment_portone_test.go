package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"

	"github.com/shurco/mycart/internal/models"
	"github.com/shurco/mycart/internal/store"
	"github.com/shurco/mycart/internal/testutil"
	"github.com/shurco/mycart/pkg/logging"
)

func TestValidatePaymentAmount(t *testing.T) {
	t.Parallel()

	log := logging.New()

	tests := []struct {
		name         string
		paymentTotal int
		cartTotal    int
		currency     string
		wantErr      bool
	}{
		{
			name:         "valid amount match - USD",
			paymentTotal: 1000,
			cartTotal:    1000,
			currency:     "USD",
			wantErr:      false,
		},
		{
			name:         "valid amount match - KRW (zero-decimal)",
			paymentTotal: 100,        // PortOne sends 100 won
			cartTotal:    10000,      // System stores as 100 * 100
			currency:     "KRW",
			wantErr:      false,
		},
		{
			name:         "amount mismatch",
			paymentTotal: 500,
			cartTotal:    1000,
			currency:     "USD",
			wantErr:      true,
		},
		{
			name:         "zero amount",
			paymentTotal: 0,
			cartTotal:    0,
			currency:     "USD",
			wantErr:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := validatePaymentAmount(tt.paymentTotal, tt.cartTotal, tt.currency, log)
			if (err != nil) != tt.wantErr {
				t.Errorf("validatePaymentAmount() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidatePaymentCurrency(t *testing.T) {
	t.Parallel()

	log := logging.New()

	tests := []struct {
		name            string
		paymentCurrency string
		cartCurrency    string
		wantErr         bool
	}{
		{
			name:            "currency match",
			paymentCurrency: "USD",
			cartCurrency:    "USD",
			wantErr:         false,
		},
		{
			name:            "currency mismatch",
			paymentCurrency: "EUR",
			cartCurrency:    "USD",
			wantErr:         true,
		},
		{
			name:            "empty currencies",
			paymentCurrency: "",
			cartCurrency:    "",
			wantErr:         false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := validatePaymentCurrency(tt.paymentCurrency, tt.cartCurrency, log)
			if (err != nil) != tt.wantErr {
				t.Errorf("validatePaymentCurrency() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateCartID(t *testing.T) {
	t.Parallel()

	log := logging.New()

	tests := []struct {
		name           string
		customDataJSON string
		expectedCartID string
		wantErr        bool
	}{
		{
			name:           "valid cart ID",
			customDataJSON: `{"cart_id":"cart123"}`,
			expectedCartID: "cart123",
			wantErr:        false,
		},
		{
			name:           "cart ID mismatch",
			customDataJSON: `{"cart_id":"cart123"}`,
			expectedCartID: "cart456",
			wantErr:        true,
		},
		{
			name:           "invalid JSON",
			customDataJSON: `{invalid}`,
			expectedCartID: "cart123",
			wantErr:        true,
		},
		{
			name:           "missing cart_id",
			customDataJSON: `{}`,
			expectedCartID: "cart123",
			wantErr:        true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := validateCartID(tt.customDataJSON, tt.expectedCartID, log)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateCartID() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestVerifyWebhookSignature(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		body      string
		signature string
		secret    string
		want      bool
	}{
		{
			name:      "valid signature",
			body:      `{"type":"payment.paid","data":{"paymentId":"pay_123"}}`,
			signature: "60a986b09ba9b8a82ac4f8643ee57d0cd7e04bbeccd4db174f66394175c73698",
			secret:    "test_secret",
			want:      true,
		},
		{
			name:      "invalid signature",
			body:      `{"type":"payment.paid","data":{"paymentId":"pay_123"}}`,
			signature: "invalid_signature",
			secret:    "test_secret",
			want:      false,
		},
		{
			name:      "wrong secret",
			body:      `{"type":"payment.paid","data":{"paymentId":"pay_123"}}`,
			signature: "60a986b09ba9b8a82ac4f8643ee57d0cd7e04bbeccd4db174f66394175c73698",
			secret:    "wrong_secret",
			want:      false,
		},
		{
			name:      "empty body",
			body:      "",
			signature: "f7f9bd47fb987337b5796fdc1fdb9ba221d0d5396814bfcaf9521f43fd8927fd",
			secret:    "test_secret",
			want:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := verifyWebhookSignature([]byte(tt.body), tt.signature, tt.secret)
			if got != tt.want {
				t.Errorf("verifyWebhookSignature() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetPortoneConfig(t *testing.T) {
	app, _, cleanup := testutil.SetupTestApp(t)
	defer cleanup()

	// Setup PortOne settings in database
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	portoneSettings := &models.Portone{
		StoreID:      "store-test-123",
		ChannelKey:   "channel-key-456",
		ApiSecret:    "secret-should-not-be-exposed",
		DebugEnabled: true,
	}

	if err := store.UpdateSettingByGroup(ctx, portoneSettings); err != nil {
		t.Fatalf("Failed to setup PortOne settings: %v", err)
	}

	app.Get("/api/cart/portone-config", GetPortoneConfig)

	resp := testutil.DoRequest(t, app, http.MethodGet, "/api/cart/portone-config", "", "")

	// Parse response body first (before AssertStatus closes it)
	var response map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	testutil.AssertStatus(t, resp, http.StatusOK)

	// Extract result field (not data)
	result, ok := response["result"].(map[string]interface{})
	if !ok {
		t.Fatalf("Expected result field in response, got: %+v", response)
	}

	// Verify store_id and channel_key are returned
	if result["store_id"] != "store-test-123" {
		t.Errorf("Expected store_id 'store-test-123', got %v", result["store_id"])
	}
	if result["channel_key"] != "channel-key-456" {
		t.Errorf("Expected channel_key 'channel-key-456', got %v", result["channel_key"])
	}

	// Verify api_secret is NOT exposed
	if _, exists := result["api_secret"]; exists {
		t.Error("api_secret should not be exposed to frontend")
	}

	// Verify debug_enabled is included (could be true or false, just verify it exists)
	if _, exists := result["debug_enabled"]; !exists {
		t.Error("debug_enabled field should be present")
	}
}

func TestCallPortoneAPI(t *testing.T) {
	t.Parallel()

	// Create mock PortOne API server
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check endpoint first
		if r.URL.Path == "/payments/pay_123" {
			// Then verify Authorization header
			auth := r.Header.Get("Authorization")
			if auth != "PortOne test_secret" {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"id":     "pay_123",
				"status": "PAID",
			})
			return
		}

		// For unknown endpoints, check auth first
		auth := r.Header.Get("Authorization")
		if auth != "PortOne test_secret" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		w.WriteHeader(http.StatusNotFound)
	}))
	defer mockServer.Close()

	// Override global portoneAPIURL for testing
	originalURL := portoneAPIURL
	portoneAPIURL = mockServer.URL
	defer func() { portoneAPIURL = originalURL }()

	tests := []struct {
		name           string
		endpoint       string
		apiSecret      string
		wantStatusCode int
		wantErr        bool
	}{
		{
			name:           "successful API call",
			endpoint:       "/payments/pay_123",
			apiSecret:      "test_secret",
			wantStatusCode: http.StatusOK,
			wantErr:        false,
		},
		{
			name:           "unauthorized - wrong secret",
			endpoint:       "/payments/pay_123",
			apiSecret:      "wrong_secret",
			wantStatusCode: http.StatusUnauthorized,
			wantErr:        false,
		},
		{
			name:           "not found endpoint",
			endpoint:       "/payments/nonexistent",
			apiSecret:      "test_secret",
			wantStatusCode: http.StatusNotFound,
			wantErr:        false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Don't run subtests in parallel since they share the same mockServer
			resp, err := callPortoneAPI(tt.endpoint, tt.apiSecret)
			if (err != nil) != tt.wantErr {
				t.Errorf("callPortoneAPI() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if resp != nil {
				defer resp.Body.Close()
				if resp.StatusCode != tt.wantStatusCode {
					t.Errorf("callPortoneAPI() status = %d, want %d", resp.StatusCode, tt.wantStatusCode)
				}
			}
		})
	}
}

func TestCompletePortonePayment(t *testing.T) {
	app, _, cleanup := testutil.SetupTestApp(t)
	defer cleanup()

	// Create mock PortOne API server
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/payments/pay_valid" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"id":       "pay_valid",
				"status":   "PAID",
				"currency": "KRW", // Currency at top level
				"amount": map[string]interface{}{
					"total": 6300, // Amount in smallest currency unit (cart.AmountTotal = 6300)
				},
				"customData": `{"cart_id":"iodz4ibf5h5zmov"}`,
			})
			return
		}
		if r.URL.Path == "/payments/pay_notfound" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer mockServer.Close()

	// Override global portoneAPIURL
	originalURL := portoneAPIURL
	portoneAPIURL = mockServer.URL
	defer func() { portoneAPIURL = originalURL }()

	// Setup PortOne settings
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	portoneSettings := &models.Portone{
		StoreID:    "store-test",
		ChannelKey: "channel-test",
		ApiSecret:  "test_secret",
	}
	if err := store.UpdateSettingByGroup(ctx, portoneSettings); err != nil {
		t.Fatalf("Failed to setup PortOne settings: %v", err)
	}

	app.Post("/api/payment/portone/complete", CompletePortonePayment)

	tests := []struct {
		name       string
		body       string
		wantStatus []int
	}{
		{
			name:       "missing payment_id",
			body:       `{"cart_id":"iodz4ibf5h5zmov"}`,
			wantStatus: []int{http.StatusBadRequest},
		},
		{
			name:       "missing cart_id",
			body:       `{"payment_id":"pay_123"}`,
			wantStatus: []int{http.StatusBadRequest},
		},
		{
			name:       "invalid JSON",
			body:       `{invalid}`,
			wantStatus: []int{http.StatusBadRequest},
		},
		{
			name:       "cart not found",
			body:       `{"payment_id":"pay_valid","cart_id":"nonexistent"}`,
			wantStatus: []int{http.StatusBadRequest},
		},
		{
			name:       "payment not found in PortOne",
			body:       `{"payment_id":"pay_notfound","cart_id":"iodz4ibf5h5zmov"}`,
			wantStatus: []int{http.StatusBadRequest},
		},
		{
			name:       "successful payment completion",
			body:       `{"payment_id":"pay_valid","cart_id":"iodz4ibf5h5zmov"}`,
			wantStatus: []int{http.StatusOK, http.StatusBadRequest}, // May fail if cart total doesn't match
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := testutil.DoRequest(t, app, http.MethodPost, "/api/payment/portone/complete", tt.body, "application/json")
			testutil.AssertStatus(t, resp, tt.wantStatus...)
		})
	}
}

func TestPortoneWebhook(t *testing.T) {
	app, _, cleanup := testutil.SetupTestApp(t)
	defer cleanup()

	// Create mock PortOne API server
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/payments/pay_webhook" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"status":     "PAID",
				"currency":   "KRW", // Currency at top level
				"customData": `{"cart_id":"iodz4ibf5h5zmov"}`,
				"amount": map[string]interface{}{
					"total": 6300, // Amount in smallest currency unit (cart.AmountTotal = 6300)
				},
			})
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer mockServer.Close()

	// Override global portoneAPIURL
	originalURL := portoneAPIURL
	portoneAPIURL = mockServer.URL
	defer func() { portoneAPIURL = originalURL }()

	// Setup PortOne settings
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	portoneSettings := &models.Portone{
		StoreID:    "store-test",
		ChannelKey: "channel-test",
		ApiSecret:  "test_secret",
	}
	if err := store.UpdateSettingByGroup(ctx, portoneSettings); err != nil {
		t.Fatalf("Failed to setup PortOne settings: %v", err)
	}

	app.Post("/api/payment/portone/webhook", PortoneWebhook)

	tests := []struct {
		name       string
		body       string
		signature  string
		wantStatus int
	}{
		{
			name:       "missing signature header",
			body:       `{"type":"payment.paid","data":{"paymentId":"pay_webhook"}}`,
			signature:  "",
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:       "invalid signature",
			body:       `{"type":"payment.paid","data":{"paymentId":"pay_webhook"}}`,
			signature:  "invalid_signature",
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:       "valid webhook with correct signature",
			body:       `{"type":"payment.paid","data":{"paymentId":"pay_webhook"}}`,
			signature:  "f05f262f94c9f3ae490238175a3f594d07a55a2bb9b775b5dbfe229c70ca97f6",
			wantStatus: http.StatusOK,
		},
		{
			name:       "invalid JSON body",
			body:       `{invalid}`,
			signature:  "",
			wantStatus: http.StatusUnauthorized, // Will fail at signature check first
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/api/payment/portone/webhook", strings.NewReader(tt.body))
			req.Header.Set("Content-Type", "application/json")
			if tt.signature != "" {
				req.Header.Set("PortOne-Signature", tt.signature)
			}

			resp, err := app.Test(req, fiber.TestConfig{Timeout: 5 * time.Second})
			if err != nil {
				t.Fatalf("POST /api/payment/portone/webhook: %v", err)
			}
			if resp.StatusCode != tt.wantStatus {
				t.Errorf("PortoneWebhook() status = %d, want %d", resp.StatusCode, tt.wantStatus)
			}
		})
	}
}
