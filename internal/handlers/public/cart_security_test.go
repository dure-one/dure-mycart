package handlers

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"

	"github.com/shurco/mycart/internal/models"
	"github.com/shurco/mycart/internal/queries"
	"github.com/shurco/mycart/internal/testutil"
	"github.com/shurco/mycart/pkg/litepay"
)

// seedCart inserts a cart with the given state and returns its ID.
func seedCart(t *testing.T, paymentID string, status litepay.Status, system litepay.PaymentSystem, amountTotal int) string {
	t.Helper()

	cartID := "testcart0000001" // len 15
	err := queries.DB().AddCart(context.Background(), &models.Cart{
		Core:          models.Core{ID: cartID},
		Email:         "buyer@example.com",
		Cart:          []models.CartProduct{},
		AmountTotal:   amountTotal,
		Currency:      "USD",
		PaymentID:     paymentID,
		PaymentStatus: status,
		PaymentSystem: system,
	})
	if err != nil {
		t.Fatalf("seed cart: %v", err)
	}
	return cartID
}

func TestVerifyCartAmount(t *testing.T) {
	t.Parallel()

	cart := &models.Cart{AmountTotal: 5000, Currency: "USD"}

	tests := []struct {
		name    string
		payment litepay.Payment
		wantErr bool
	}{
		{"matching amount and currency", litepay.Payment{AmountTotal: 5000, Currency: "USD"}, false},
		{"currency case-insensitive", litepay.Payment{AmountTotal: 5000, Currency: "usd"}, false},
		{"amount mismatch", litepay.Payment{AmountTotal: 100, Currency: "USD"}, true},
		{"currency mismatch", litepay.Payment{AmountTotal: 5000, Currency: "EUR"}, true},
		{"both mismatch", litepay.Payment{AmountTotal: 0, Currency: ""}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := verifyCartAmount(&tt.payment, cart)
			if (err != nil) != tt.wantErr {
				t.Errorf("verifyCartAmount(%+v) error = %v, wantErr %v", tt.payment, err, tt.wantErr)
			}
		})
	}
}

// A provider object that does not belong to the cart must be rejected before
// any provider API call is made (cross-cart replay defense).
func TestPaymentSuccess_SessionMismatchRejected(t *testing.T) {
	app, _, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	app.Get("/cart/payment/success", PaymentSuccess)

	cartID := seedCart(t, "cs_live_owned_session", litepay.NEW, litepay.STRIPE, 5000)

	tests := []struct {
		name   string
		params string
	}{
		{"foreign session", "?cart_id=" + cartID + "&payment_system=stripe&session=cs_attacker_session"},
		{"missing session", "?cart_id=" + cartID + "&payment_system=stripe"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := testutil.DoRequest(t, app, http.MethodGet, "/cart/payment/success"+tt.params, "", "")
			testutil.AssertStatus(t, resp, http.StatusBadRequest)
		})
	}
}

func TestPaymentSuccess_PaypalTokenMismatchRejected(t *testing.T) {
	app, _, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	app.Get("/cart/payment/success", PaymentSuccess)

	cartID := seedCart(t, "PAYID-owned-order", litepay.NEW, litepay.PAYPAL, 5000)

	resp := testutil.DoRequest(t, app, http.MethodGet,
		"/cart/payment/success?cart_id="+cartID+"&payment_system=paypal&token=PAYID-attacker", "", "")
	testutil.AssertStatus(t, resp, http.StatusBadRequest)
}

func TestPaymentSuccess_CoinbaseChargeMismatchRejected(t *testing.T) {
	app, _, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	app.Get("/cart/payment/success", PaymentSuccess)

	cartID := seedCart(t, "charge-owned-666", litepay.NEW, litepay.COINBASE, 5000)

	resp := testutil.DoRequest(t, app, http.MethodGet,
		"/cart/payment/success?cart_id="+cartID+"&payment_system=coinbase&charge_id=charge-attacker", "", "")
	testutil.AssertStatus(t, resp, http.StatusBadRequest)
}

func TestPaymentCallback_UnsupportedSystemRejected(t *testing.T) {
	app, _, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	app.Post("/cart/payment/callback", PaymentCallback)

	body := `{"status": 3}`
	resp := testutil.DoRequest(t, app, http.MethodPost,
		"/cart/payment/callback?cart_id=testcart0000001&payment_system=stripe", body, "")
	testutil.AssertStatus(t, resp, http.StatusBadRequest)
}

// The callback is unauthenticated input: a payload whose RSA signature does
// not verify must never reach the cart update.
func TestPaymentCallback_InvalidSignatureRejected(t *testing.T) {
	app, _, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	app.Post("/cart/payment/callback", PaymentCallback)

	db := queries.DB()
	ctx := context.Background()

	// SpectroCoin must be active, otherwise the handler short-circuits with 404.
	if err := db.UpdateSettingByGroup(ctx, &models.Spectrocoin{
		MerchantID: "0f8fad7b-c931-41c4-a111-8b80651c9d01",
		ProjectID:  "0f8fad7b-c931-41c4-a222-8b80651c9d02",
		PrivateKey: "unused-for-verification",
		Active:     true,
	}); err != nil {
		t.Fatalf("activate spectrocoin: %v", err)
	}

	cartID := seedCart(t, "", litepay.NEW, litepay.SPECTROCOIN, 10000)

	form := url.Values{}
	form.Set("merchantId", "12345")
	form.Set("apiId", "1")
	form.Set("orderId", cartID)
	form.Set("payCurrency", "BTC")
	form.Set("payAmount", "0.1")
	form.Set("receiveCurrency", "USD")
	form.Set("receiveAmount", "100.0")
	form.Set("description", "")
	form.Set("orderRequestId", "11")
	form.Set("status", "3") // forged PAID
	form.Set("sign", "Zm9yZ2VkLXNpZ25hdHVyZQ==")

	req, err := http.NewRequest(http.MethodPost,
		"/cart/payment/callback?cart_id="+cartID+"&payment_system=spectrocoin",
		strings.NewReader(form.Encode()))
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := app.Test(req, fiber.TestConfig{Timeout: 10 * time.Second})
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	testutil.AssertStatus(t, resp, http.StatusBadRequest)

	// The forged PAID must NOT have been persisted.
	cart, err := db.Cart(ctx, cartID)
	if err != nil {
		t.Fatalf("load cart: %v", err)
	}
	if cart.PaymentStatus == litepay.PAID {
		t.Fatal("forged callback must not mark the cart as paid")
	}
}

func TestPaymentCancel_TokenRequired(t *testing.T) {
	app, _, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	app.Get("/cart/payment/cancel", PaymentCancel)

	db := queries.DB()
	ctx := context.Background()
	cartID := seedCart(t, "", litepay.NEW, litepay.STRIPE, 5000)

	redirects := []int{http.StatusFound, http.StatusSeeOther}

	tests := []struct {
		name   string
		params string
	}{
		{"no token", "?cart_id=" + cartID + "&payment_system=stripe"},
		{"empty token", "?cart_id=" + cartID + "&payment_system=stripe&cancel_token="},
		{"wrong token", "?cart_id=" + cartID + "&payment_system=stripe&cancel_token=deadbeef"},
		{"token for another cart", "?cart_id=" + cartID + "&payment_system=stripe&cancel_token=" + cancelToken(jwtSecretForCancel(t), "othercart000001")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := testutil.DoRequest(t, app, http.MethodGet, "/cart/payment/cancel"+tt.params, "", "")
			testutil.AssertStatus(t, resp, redirects...)

			cart, err := db.Cart(ctx, cartID)
			if err != nil {
				t.Fatalf("load cart: %v", err)
			}
			if cart.PaymentStatus == litepay.CANCELED {
				t.Fatal("cart must not be canceled without a valid capability token")
			}
		})
	}
}

func TestPaymentCancel_ValidTokenCancelsCart(t *testing.T) {
	app, _, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	app.Get("/cart/payment/cancel", PaymentCancel)

	db := queries.DB()
	ctx := context.Background()
	cartID := seedCart(t, "", litepay.NEW, litepay.STRIPE, 5000)

	token := cancelToken(jwtSecretForCancel(t), cartID)
	resp := testutil.DoRequest(t, app, http.MethodGet,
		"/cart/payment/cancel?cart_id="+cartID+"&payment_system=stripe&cancel_token="+token, "", "")
	testutil.AssertStatus(t, resp, http.StatusFound, http.StatusSeeOther)

	cart, err := db.Cart(ctx, cartID)
	if err != nil {
		t.Fatalf("load cart: %v", err)
	}
	if cart.PaymentStatus != litepay.CANCELED {
		t.Errorf("payment status = %q, want canceled", cart.PaymentStatus)
	}
}

func TestPaymentCancel_MissingCartRedirectsWithoutMutation(t *testing.T) {
	app, _, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	app.Get("/cart/payment/cancel", PaymentCancel)

	resp := testutil.DoRequest(t, app, http.MethodGet, "/cart/payment/cancel", "", "")
	testutil.AssertStatus(t, resp, http.StatusFound, http.StatusSeeOther)
}

// jwtSecretForCancel returns the fixture JWT secret used by the handler to
// derive cancel capability tokens.
func jwtSecretForCancel(t *testing.T) string {
	t.Helper()
	settingJWT, err := queries.GetSettingByGroup[models.JWT](context.Background(), queries.DB())
	if err != nil {
		t.Fatalf("load JWT settings: %v", err)
	}
	return settingJWT.Secret
}

func TestCreateCart(t *testing.T) {
	app, _, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	app.Post("/api/cart/create", CreateCart)

	t.Run("bad body returns 400", func(t *testing.T) {
		resp := testutil.DoRequest(t, app, http.MethodPost, "/api/cart/create", "{not-json", "")
		testutil.AssertStatus(t, resp, http.StatusBadRequest)
	})

	t.Run("happy path creates cart", func(t *testing.T) {
		// fv6c9s9cqzf36sc is a fixture product priced at 2000 cents; the
		// fixture ships it with quantity=0, so stock some first.
		ctx := context.Background()
		if _, err := queries.DB().ProductQueries.ExecContext(ctx,
			`UPDATE product SET quantity = 5 WHERE id = 'fv6c9s9cqzf36sc'`); err != nil {
			t.Fatalf("stock fixture product: %v", err)
		}

		body := `{"email":"buyer@example.com","provider":"dummy","products":[{"id":"fv6c9s9cqzf36sc","quantity":1,"unit_price":2000}]}`
		resp := testutil.DoRequest(t, app, http.MethodPost, "/api/cart/create", body, "")

		raw, err := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if err != nil {
			t.Fatalf("read response: %v", err)
		}
		testutil.AssertStatusCode(t, resp.StatusCode, http.StatusOK)
		if resp.StatusCode != http.StatusOK {
			t.Logf("create cart body: %s", raw)
		}

		var payload struct {
			Result struct {
				CartID    string `json:"cart_id"`
				AmountTot int    `json:"amount_total"`
				Currency  string `json:"currency"`
			} `json:"result"`
		}
		if err := json.Unmarshal(raw, &payload); err != nil {
			t.Fatalf("decode response %s: %v", raw, err)
		}

		if len(payload.Result.CartID) != 15 {
			t.Fatalf("expected 15-char cart_id, got %q", payload.Result.CartID)
		}

		cart, err := queries.DB().Cart(context.Background(), payload.Result.CartID)
		if err != nil {
			t.Fatalf("cart not persisted: %v", err)
		}
		if cart.Currency != payload.Result.Currency || cart.AmountTotal != payload.Result.AmountTot {
			t.Errorf("persisted cart %+v does not match response amount=%d currency=%s",
				cart, payload.Result.AmountTot, payload.Result.Currency)
		}
	})
}
