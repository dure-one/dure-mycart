package handlers

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/gofiber/fiber/v3"

	"github.com/shurco/mycart/internal/models"
	"github.com/shurco/mycart/internal/queries"
	"github.com/shurco/mycart/internal/testutil"
	"github.com/shurco/mycart/internal/webhook"
	"github.com/shurco/mycart/pkg/litepay"
)

// freeCartID is a 15-character cart id, the length litepay.Payment.Validate
// requires, for carts seeded by these tests.
const freeCartID = "freecart0000001"

// setSetting writes one setting, for tests that need a configuration the
// fixtures do not provide.
func setSetting(t *testing.T, key, value string) {
	t.Helper()

	if err := queries.DB().UpdateSettingByKey(context.Background(), &models.SettingName{
		Key:   key,
		Value: value,
	}); err != nil {
		t.Fatalf("set %s=%q: %v", key, value, err)
	}
}

// silenceSMTP removes the SMTP configuration the fixtures install.
//
// The fixtures point SMTP at localhost:1025, a mail catcher that is not running
// during tests, so every handler that sends a letter would fail at the very
// last step. The mailer skips sending when SMTP is unconfigured, which is the
// documented dev/test behaviour and what these tests want: they are about the
// payment flow, not about SMTP. The mailer's own tests cover the sending.
func silenceSMTP(t *testing.T) {
	t.Helper()

	for _, key := range []string{"smtp_host", "smtp_port", "smtp_username", "smtp_password"} {
		setSetting(t, key, "")
	}
}

// captureWebhooks points the payment webhook at a local server and records
// what arrives, so the tests can assert the notification was actually
// delivered rather than merely attempted.
func captureWebhooks(t *testing.T) *webhookRecorder {
	t.Helper()

	rec := &webhookRecorder{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		var event webhook.Payment
		if err := json.Unmarshal(body, &event); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		rec.add(event)
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)

	setSetting(t, "webhook_url", srv.URL)
	return rec
}

// webhookRecorder collects the webhook payloads a test provoked. The handler
// delivers them synchronously, but the write happens on the server's goroutine
// and the read on the test's, so it is guarded for -race.
type webhookRecorder struct {
	mu     sync.Mutex
	events []webhook.Payment
}

func (r *webhookRecorder) add(event webhook.Payment) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.events = append(r.events, event)
}

func (r *webhookRecorder) all() []webhook.Payment {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]webhook.Payment(nil), r.events...)
}

// expectEvent asserts that exactly one webhook of the given event was
// delivered, and returns it.
func (r *webhookRecorder) expectEvent(t *testing.T, event webhook.Event) webhook.Payment {
	t.Helper()

	events := r.all()
	if len(events) != 1 {
		t.Fatalf("received %d webhooks, want 1: %+v", len(events), events)
	}
	if events[0].Event != event {
		t.Errorf("event = %q, want %q", events[0].Event, event)
	}
	if events[0].TimeStamp == 0 {
		t.Error("webhook carries no timestamp")
	}
	return events[0]
}

// resultString decodes a response whose result is a plain string.
func resultString(t *testing.T, env portoneEnvelope) string {
	t.Helper()

	var s string
	if err := json.Unmarshal(env.Result, &s); err != nil {
		t.Fatalf("result is not a string: %s (%v)", env.Result, err)
	}
	return s
}

// countCarts returns how many carts the database holds, so a test can assert
// that a request did not get as far as creating one.
func countCarts(t *testing.T) int {
	t.Helper()

	var n int
	err := queries.DB().CartQueries.DB.QueryRowContext(context.Background(), `SELECT COUNT(*) FROM cart`).Scan(&n)
	if err != nil {
		t.Fatalf("count carts: %v", err)
	}
	return n
}

// The storefront asks which providers are available. Only the providers that
// could actually be used may be reported as active.
func TestPaymentList(t *testing.T) {
	app, _, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	app.Get("/api/cart/payment", PaymentList)

	list := func(t *testing.T) map[string]bool {
		t.Helper()

		status, _, raw := readEnvelope(t, testutil.DoRequest(t, app, http.MethodGet, "/api/cart/payment", "", ""))
		if status != http.StatusOK {
			t.Fatalf("status = %d, want 200; body: %s", status, raw)
		}

		var payload struct {
			Result map[string]bool `json:"result"`
		}
		if err := json.Unmarshal(raw, &payload); err != nil {
			t.Fatalf("decode response %s: %v", raw, err)
		}
		return payload.Result
	}

	t.Run("shipped providers are off and the dummy is always on", func(t *testing.T) {
		got := list(t)

		for _, name := range []string{"stripe", "paypal", "spectrocoin", "coinbase", "portone"} {
			if got[name] {
				t.Errorf("%s is reported active, but nothing configures it", name)
			}
		}
		// The dummy has no setting: it is free-cart-only and always offered.
		if !got["dummy"] {
			t.Error("dummy is reported inactive; it is always available for free carts")
		}
	})

	t.Run("an activated provider is reported", func(t *testing.T) {
		if err := queries.DB().UpdateSettingByGroup(context.Background(),
			&models.Stripe{SecretKey: "sk_test", Active: true}); err != nil {
			t.Fatalf("activate stripe: %v", err)
		}

		if !list(t)["stripe"] {
			t.Error("stripe is reported inactive after being activated")
		}
	})

	// The storefront is meant to hide PortOne when the channel does not support
	// the store's currency. It never can: the guard reads
	// models.Portone.SupportedCurrencies, and queries.GroupFieldMap has no entry
	// for portone_supported_currencies, so GetSettingByGroup leaves the slice
	// empty however the setting is stored. This pins today's behaviour — PortOne
	// stays listed — so that a fix has a test to change. The seed value is
	// ["KRW"] while the store currency is USD, which is exactly the case the
	// guard was written for.
	//
	// The setting is written through the raw queries handle rather than
	// UpdateSettingByGroup for the same reason: that call would drop the field
	// too.
	t.Run("portone is listed even when the store currency is unsupported by the channel", func(t *testing.T) {
		if err := queries.DB().UpdateSettingByGroup(context.Background(),
			&models.Portone{StoreID: "store", ChannelKey: "channel", ApiSecret: "secret", Active: true}); err != nil {
			t.Fatalf("activate portone: %v", err)
		}
		setSetting(t, "portone_supported_currencies", `["KRW"]`)

		got := list(t)
		if !got["portone"] {
			t.Error("portone was filtered out; if the currency filter was just wired up, update this test")
		}

		// The currency really is unsupported, so the assertion above is about
		// the guard being inert rather than about the data agreeing with it.
		currency, err := queries.DB().GetSettingByKey(context.Background(), "currency")
		if err != nil {
			t.Fatalf("read currency: %v", err)
		}
		if usd := currency["currency"].Value.(string); usd == "KRW" {
			t.Fatalf("store currency is %q, so this test proves nothing", usd)
		}
	})
}

func TestPayment(t *testing.T) {
	app, _, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	app.Post("/cart/payment", Payment)

	// A provider that is switched off must not be contacted: the buyer is sent
	// back to the site instead. Nothing is created or sent, so this is also the
	// proof that the handler returns before it touches the database or the mail
	// server.
	t.Run("inactive providers send the buyer back to the site", func(t *testing.T) {
		assertInactiveProvidersAreNotContacted(t, app)
	})

	t.Run("dummy provider completes a free cart", func(t *testing.T) {
		assertDummyCompletesAFreeCart(t, app)
	})

	t.Run("dummy provider is refused for a cart that costs money", func(t *testing.T) {
		assertDummyIsRefusedForACartThatCostsMoney(t, app)
	})

	t.Run("an out-of-stock cart is reported to the buyer", func(t *testing.T) {
		assertAnOutOfStockCartIsReported(t, app)
	})
}

// assertInactiveProvidersAreNotContacted walks the providers the fixtures leave
// switched off. Each has to answer with the site's cart page, and none of them
// may leave a cart behind: the handler must return before it writes anything.
func assertInactiveProvidersAreNotContacted(t *testing.T, app *fiber.App) {
	t.Helper()

	for _, provider := range []string{"stripe", "paypal", "spectrocoin", "coinbase"} {
		t.Run(provider, func(t *testing.T) {
			before := countCarts(t)

			body := `{"provider":"` + provider + `","email":"buyer@example.com","products":[]}`
			status, env, raw := readEnvelope(t, testutil.DoRequest(t, app, http.MethodPost, "/cart/payment", body, ""))
			if status != http.StatusOK {
				t.Fatalf("status = %d, want 200; body: %s", status, raw)
			}
			if got := resultString(t, env); got != "http://site.com/cart" {
				t.Errorf("payment url = %q, want the site cart page", got)
			}
			if after := countCarts(t); after != before {
				t.Errorf("cart count went from %d to %d; an inactive provider must not create a cart", before, after)
			}
		})
	}
}

// assertDummyCompletesAFreeCart pays for a cart that costs nothing and checks
// both halves of what the buyer gets: the redirect to the success page, and the
// cart the redirect names, which has to be in the database already.
func assertDummyCompletesAFreeCart(t *testing.T, app *fiber.App) {
	t.Helper()

	silenceSMTP(t)
	hooks := captureWebhooks(t)

	body := `{"provider":"dummy","email":"buyer@example.com","products":[]}`
	status, env, raw := readEnvelope(t, testutil.DoRequest(t, app, http.MethodPost, "/cart/payment", body, ""))
	if status != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", status, raw)
	}

	var payload struct {
		URL string `json:"url"`
	}
	if err := json.Unmarshal(env.Result, &payload); err != nil {
		t.Fatalf("result is not a payment url object: %s (%v)", env.Result, err)
	}
	url := payload.URL
	if !strings.HasPrefix(url, "http://site.com/cart/payment/success?") {
		t.Fatalf("payment url = %q, want a redirect to the success page", url)
	}
	if !strings.Contains(url, "payment_system=dummy") {
		t.Errorf("payment url = %q, want it to name the provider", url)
	}

	// The cart id in the redirect URL is the one that must now exist.
	cartID := url[strings.Index(url, "cart_id=")+len("cart_id="):]
	if len(cartID) != 15 {
		t.Fatalf("cart id %q is not 15 characters", cartID)
	}

	assertFreeCartWasStored(t, cartID)
	assertPaymentInitiationWasAnnounced(t, hooks, cartID)
}

// assertFreeCartWasStored checks the cart the dummy provider's redirect names:
// the buyer, a total of nothing, and the states that say nothing has been paid
// yet.
func assertFreeCartWasStored(t *testing.T, cartID string) {
	t.Helper()

	cart, err := queries.DB().Cart(context.Background(), cartID)
	if err != nil {
		t.Fatalf("cart not persisted: %v", err)
	}
	if cart.Email != "buyer@example.com" {
		t.Errorf("email = %q, want buyer@example.com", cart.Email)
	}
	if cart.AmountTotal != 0 || cart.Currency != "USD" {
		t.Errorf("cart amount = %d %s, want 0 USD", cart.AmountTotal, cart.Currency)
	}
	if cart.PaymentStatus != litepay.NEW {
		t.Errorf("payment status = %q, want %q: nothing has been paid yet", cart.PaymentStatus, litepay.NEW)
	}
	if cart.PaymentSystem != litepay.DUMMY {
		t.Errorf("payment system = %q, want %q", cart.PaymentSystem, litepay.DUMMY)
	}
}

// assertPaymentInitiationWasAnnounced checks the webhook the shop's backend is
// told about: one initiation for this cart, naming the provider and the amount.
func assertPaymentInitiationWasAnnounced(t *testing.T, hooks *webhookRecorder, cartID string) {
	t.Helper()

	event := hooks.expectEvent(t, webhook.PAYMENT_INITIATION)
	if event.Data.CartID != cartID {
		t.Errorf("webhook cart id = %q, want %q", event.Data.CartID, cartID)
	}
	if event.Data.PaymentSystem != litepay.DUMMY || event.Data.PaymentStatus != litepay.NEW {
		t.Errorf("webhook data = %+v, want a dummy payment in the new state", event.Data)
	}
	if event.Data.Currency != "USD" {
		t.Errorf("webhook currency = %q, want USD", event.Data.Currency)
	}
}

// assertDummyIsRefusedForACartThatCostsMoney checks the other side of the dummy
// provider's contract: it stands in for money that never changes hands, so a
// cart the shop can charge for has to be refused rather than paid.
func assertDummyIsRefusedForACartThatCostsMoney(t *testing.T, app *fiber.App) {
	t.Helper()

	silenceSMTP(t)

	// The fixture product ships with no stock, and a quantity of 0 is
	// itself a validation error, so stock it first.
	if _, err := queries.DB().ProductQueries.DB.ExecContext(context.Background(),
		`UPDATE product SET quantity = 5 WHERE id = 'fv6c9s9cqzf36sc'`); err != nil {
		t.Fatalf("stock fixture product: %v", err)
	}

	body := `{"provider":"dummy","email":"buyer@example.com","products":[{"id":"fv6c9s9cqzf36sc","quantity":1,"unit_price":2000}]}`
	status, env, raw := readEnvelope(t, testutil.DoRequest(t, app, http.MethodPost, "/cart/payment", body, ""))
	if status != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body: %s", status, raw)
	}
	if !strings.Contains(env.reason(), "Dummy payment provider") {
		t.Errorf("reason = %q, want it to explain that the dummy is for free items", env.reason())
	}
}

// assertAnOutOfStockCartIsReported checks what the buyer is told when the cart
// cannot be honoured: a conflict carrying the corrections, so the storefront can
// show what it would have to change.
func assertAnOutOfStockCartIsReported(t *testing.T, app *fiber.App) {
	t.Helper()

	silenceSMTP(t)

	// Back to no stock: the previous subtest stocked this product.
	if _, err := queries.DB().ProductQueries.DB.ExecContext(context.Background(),
		`UPDATE product SET quantity = 0 WHERE id = 'fv6c9s9cqzf36sc'`); err != nil {
		t.Fatalf("clear fixture product stock: %v", err)
	}

	// A quantity of 0 is what the validator rejects.
	body := `{"provider":"dummy","email":"buyer@example.com","products":[{"id":"fv6c9s9cqzf36sc","quantity":1,"unit_price":2000}]}`
	status, _, raw := readEnvelope(t, testutil.DoRequest(t, app, http.MethodPost, "/cart/payment", body, ""))
	if status != http.StatusConflict {
		t.Fatalf("status = %d, want 409; body: %s", status, raw)
	}

	var payload struct {
		Result struct {
			Errors         []models.CartValidationError `json:"validation_errors"`
			CorrectedItems []models.CorrectedCartItem   `json:"corrected_cart"`
		} `json:"result"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatalf("decode response %s: %v", raw, err)
	}
	if len(payload.Result.Errors) != 1 {
		t.Fatalf("errors = %+v, want exactly one", payload.Result.Errors)
	}
	if len(payload.Result.CorrectedItems) != 1 {
		t.Errorf("corrected cart = %+v, want the corrected item", payload.Result.CorrectedItems)
	}
}

// seedFreeCart inserts the free cart the dummy provider is allowed to pay for.
func seedFreeCart(t *testing.T, id string, status litepay.Status) string {
	t.Helper()

	if err := queries.DB().AddCart(context.Background(), &models.Cart{
		Core:          models.Core{ID: id},
		Email:         "buyer@example.com",
		Cart:          []models.CartProduct{},
		AmountTotal:   0,
		Currency:      "USD",
		PaymentStatus: status,
		PaymentSystem: litepay.DUMMY,
	}); err != nil {
		t.Fatalf("seed free cart: %v", err)
	}
	return id
}

func TestPaymentSuccess(t *testing.T) {
	app, _, cleanup := testutil.SetupTestApp(t)
	defer cleanup()

	// PaymentSuccess ends by passing control to the SPA handler that renders
	// the success page. Answering 204 from a second handler makes "control was
	// passed on" observable.
	passedOn := false
	app.Get("/cart/payment/success",
		PaymentSuccess,
		func(c fiber.Ctx) error {
			passedOn = true
			return c.SendStatus(fiber.StatusNoContent)
		})

	t.Run("a free dummy payment is completed and passed to the SPA", func(t *testing.T) {
		silenceSMTP(t)
		hooks := captureWebhooks(t)

		cartID := seedFreeCart(t, freeCartID, litepay.NEW)
		passedOn = false

		resp := testutil.DoRequest(t, app, http.MethodGet,
			"/cart/payment/success?cart_id="+cartID+"&payment_system=dummy", "", "")
		testutil.AssertStatus(t, resp, http.StatusNoContent)
		if !passedOn {
			t.Error("control was not passed to the next handler")
		}

		cart, err := queries.DB().Cart(context.Background(), cartID)
		if err != nil {
			t.Fatalf("load cart: %v", err)
		}
		if cart.PaymentStatus != litepay.PAID {
			t.Errorf("payment status = %q, want %q", cart.PaymentStatus, litepay.PAID)
		}
		if cart.PaymentID != "dummy_"+cartID {
			t.Errorf("payment id = %q, want the provider's merchant id", cart.PaymentID)
		}

		event := hooks.expectEvent(t, webhook.PAYMENT_SUCCESS)
		if event.Data.CartID != cartID || event.Data.PaymentStatus != litepay.PAID {
			t.Errorf("webhook data = %+v, want cart %s paid", event.Data, cartID)
		}
	})

	t.Run("an already paid cart is passed straight to the SPA", func(t *testing.T) {
		silenceSMTP(t)

		cartID := seedFreeCart(t, "freecart0000002", litepay.PAID)
		passedOn = false

		resp := testutil.DoRequest(t, app, http.MethodGet,
			"/cart/payment/success?cart_id="+cartID+"&payment_system=dummy", "", "")
		testutil.AssertStatus(t, resp, http.StatusNoContent)
		if !passedOn {
			t.Error("control was not passed to the next handler")
		}

		// Nothing may be re-sent for a cart that is already paid.
		cart, err := queries.DB().Cart(context.Background(), cartID)
		if err != nil {
			t.Fatalf("load cart: %v", err)
		}
		if cart.PaymentID != "" {
			t.Errorf("payment id = %q, want it untouched", cart.PaymentID)
		}
	})

	t.Run("a cart id of the wrong length is redirected home", func(t *testing.T) {
		resp := testutil.DoRequest(t, app, http.MethodGet,
			"/cart/payment/success?cart_id=short&payment_system=dummy", "", "")
		testutil.AssertStatus(t, resp, http.StatusFound, http.StatusSeeOther)
		if loc := resp.Header.Get("Location"); loc != "/" {
			t.Errorf("Location = %q, want /", loc)
		}
	})

	// A provider that is switched off cannot verify anything, and must say so
	// rather than accept the redirect at face value.
	t.Run("inactive providers cannot verify a payment", func(t *testing.T) {
		tests := []struct {
			provider string
			field    string
			value    string
		}{
			{"stripe", "session", "cs_test_owned"},
			{"paypal", "token", "PAYID-owned"},
			{"coinbase", "charge_id", "charge-owned"},
		}

		for _, tt := range tests {
			t.Run(tt.provider, func(t *testing.T) {
				cartID := "seededcart00" + tt.provider[:3]
				if err := queries.DB().AddCart(context.Background(), &models.Cart{
					Core:          models.Core{ID: cartID},
					Email:         "buyer@example.com",
					Cart:          []models.CartProduct{},
					AmountTotal:   5000,
					Currency:      "USD",
					PaymentID:     tt.value,
					PaymentStatus: litepay.NEW,
					PaymentSystem: litepay.PaymentSystem(tt.provider),
				}); err != nil {
					t.Fatalf("seed cart: %v", err)
				}

				resp := testutil.DoRequest(t, app, http.MethodGet,
					"/cart/payment/success?cart_id="+cartID+"&payment_system="+tt.provider+"&"+tt.field+"="+tt.value, "", "")
				testutil.AssertStatus(t, resp, http.StatusNotFound)

				cart, err := queries.DB().Cart(context.Background(), cartID)
				if err != nil {
					t.Fatalf("load cart: %v", err)
				}
				if cart.PaymentStatus == litepay.PAID {
					t.Error("an inactive provider must not mark the cart paid")
				}
			})
		}
	})
}

// The SpectroCoin callback is the only callback this endpoint accepts, and its
// payload must carry a signature made with SpectroCoin's own private key, which
// the test suite cannot produce — the public half is compiled in. What is left
// to test here is the state the handler checks before it reads the payload.
func TestPaymentCallback_SpectrocoinInactive(t *testing.T) {
	app, _, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	app.Post("/cart/payment/callback", PaymentCallback)

	t.Run("an inactive spectrocoin is reported as missing", func(t *testing.T) {
		status, _, _ := readEnvelope(t, testutil.DoRequest(t, app, http.MethodPost,
			"/cart/payment/callback?cart_id=testcart0000001&payment_system=spectrocoin", `{}`, ""))
		if status != http.StatusNotFound {
			t.Fatalf("status = %d, want 404", status)
		}
	})
}
