package handlers

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"

	"github.com/shurco/mycart/internal/models"
	"github.com/shurco/mycart/internal/queries"
	"github.com/shurco/mycart/internal/testutil"
	"github.com/shurco/mycart/pkg/litepay"
	"github.com/shurco/mycart/pkg/logging"
)

// portoneSecret is the API secret the tests install for the PortOne settings.
// The webhook signs its payloads with it, so a test that forgets to install it
// fails the signature check rather than something further downstream.
const portoneSecret = "portone-test-api-secret-0123456789"

// portoneStub points the package's PortOne base URL at a local server for the
// duration of the test.
//
// The base URL is a package-level variable precisely so this is possible; it is
// also why none of the tests here run in parallel with each other.
func portoneStub(t *testing.T, handle http.HandlerFunc) *httptest.Server {
	t.Helper()

	srv := httptest.NewServer(handle)
	previous := portoneAPIURL
	portoneAPIURL = srv.URL
	t.Cleanup(func() {
		portoneAPIURL = previous
		srv.Close()
	})
	return srv
}

// enablePortone installs the PortOne settings the handlers read.
func enablePortone(t *testing.T, secret string) {
	t.Helper()

	if err := queries.DB().UpdateSettingByGroup(context.Background(), &models.Portone{
		StoreID:    "store-0123456789abcdefghij",
		ChannelKey: "channel-key-0123456789",
		ApiSecret:  secret,
	}); err != nil {
		t.Fatalf("install portone settings: %v", err)
	}
}

// portoneCart inserts a cart the PortOne flow can pay for, in the shape the
// create endpoint leaves it: unpaid, with the provider already recorded. The id
// must be unique within a test; 15 characters matches what the real generator
// emits.
func portoneCart(t *testing.T, id string, amountTotal int) string {
	t.Helper()

	if err := queries.DB().AddCart(context.Background(), &models.Cart{
		Core:          models.Core{ID: id},
		Email:         "buyer@example.com",
		Cart:          []models.CartProduct{},
		AmountTotal:   amountTotal,
		Currency:      "USD",
		PaymentStatus: litepay.NEW,
		PaymentSystem: "portone",
	}); err != nil {
		t.Fatalf("seed cart %s: %v", id, err)
	}
	return id
}

// portoneEnvelope is the webutil response shape. Both halves are kept because
// the two helpers disagree: StatusBadRequest puts the text in result,
// webutil.Response puts it in message.
type portoneEnvelope struct {
	Success bool            `json:"success"`
	Message string          `json:"message"`
	Result  json.RawMessage `json:"result"`
}

// readEnvelope consumes a JSON response and returns its status and envelope.
func readEnvelope(t *testing.T, resp *http.Response) (int, portoneEnvelope, []byte) {
	t.Helper()

	raw, err := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if err != nil {
		t.Fatalf("read response body: %v", err)
	}

	var env portoneEnvelope
	_ = json.Unmarshal(raw, &env)
	return resp.StatusCode, env, raw
}

// reason returns the handler's message, wherever it was put.
func (e portoneEnvelope) reason() string {
	if len(e.Result) > 0 {
		var s string
		if err := json.Unmarshal(e.Result, &s); err == nil {
			return s
		}
	}
	return e.Message
}

// upstreamResponse is what the fake PortOne API answers with.
type upstreamResponse struct {
	status int
	body   string
}

// serve answers every PortOne API request with the response.
func (u upstreamResponse) serve() http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(u.status)
		_, _ = io.WriteString(w, u.body)
	}
}

// unreachablePortone stands in for a stub that must never be called, so a
// handler reaching the network before it validated its input fails the test
// with a readable message instead of a silent success.
func unreachablePortone(t *testing.T) http.HandlerFunc {
	t.Helper()
	return func(http.ResponseWriter, *http.Request) {
		t.Error("the PortOne API must not be called for this request")
	}
}

// portonePaymentBody renders the payment payload the PortOne API returns.
// customData is itself a JSON document, hence the quoting.
func portonePaymentBody(status string, total int, currency, customData string) string {
	return fmt.Sprintf(
		`{"id":"pay_1","status":%q,"amount":{"total":%d,"currency":%q},"customData":%q}`,
		status, total, currency, customData)
}

func TestGetPortoneConfig(t *testing.T) {
	app, _, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	app.Get("/api/cart/portone-config", GetPortoneConfig)

	enablePortone(t, portoneSecret)

	status, _, raw := readEnvelope(t, testutil.DoRequest(t, app, http.MethodGet,
		"/api/cart/portone-config", "", ""))
	if status != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", status, raw)
	}

	// The browser SDK needs exactly these three fields. Anything else that
	// appears here is a leak, so the assertion is on the whole set rather than
	// on the presence of the expected keys.
	var payload struct {
		Result map[string]any `json:"result"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatalf("decode response %s: %v", raw, err)
	}

	want := map[string]any{
		"store_id":      "store-0123456789abcdefghij",
		"channel_key":   "channel-key-0123456789",
		"debug_enabled": false,
	}
	if len(payload.Result) != len(want) {
		t.Errorf("config exposes %d fields, want %d: %v", len(payload.Result), len(want), payload.Result)
	}
	for key, value := range want {
		if got := payload.Result[key]; got != value {
			t.Errorf("config[%q] = %v, want %v", key, got, value)
		}
	}

	// The API secret is the one setting that must never travel to the browser.
	if strings.Contains(string(raw), portoneSecret) {
		t.Fatal("the API secret must not appear in the public config response")
	}
}

func TestCallPortoneAPI(t *testing.T) {
	t.Run("sends the secret and returns the response", func(t *testing.T) {
		var gotAuth, gotContentType, gotPath, gotQuery string
		portoneStub(t, func(w http.ResponseWriter, r *http.Request) {
			gotAuth = r.Header.Get("Authorization")
			gotContentType = r.Header.Get("Content-Type")
			gotPath = r.URL.Path
			gotQuery = r.URL.RawQuery
			w.WriteHeader(http.StatusOK)
			_, _ = io.WriteString(w, `{"id":"pay_1"}`)
		})

		resp, err := callPortoneAPI("/payments/pay_1?fields=amount", portoneSecret)
		if err != nil {
			t.Fatalf("callPortoneAPI: %v", err)
		}
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("status = %d, want 200", resp.StatusCode)
		}
		if want := "PortOne " + portoneSecret; gotAuth != want {
			t.Errorf("Authorization = %q, want %q", gotAuth, want)
		}
		if gotContentType != "application/json" {
			t.Errorf("Content-Type = %q, want application/json", gotContentType)
		}
		if gotPath != "/payments/pay_1" || gotQuery != "fields=amount" {
			t.Errorf("requested %q?%q, want /payments/pay_1?fields=amount", gotPath, gotQuery)
		}
	})

	t.Run("unreachable server is an error", func(t *testing.T) {
		// A server that is already closed gives a port nothing listens on, so
		// the failure is deterministic rather than a DNS or firewall accident.
		srv := httptest.NewServer(http.NotFoundHandler())
		srv.Close()
		t.Cleanup(func() { portoneAPIURL = "https://api.portone.io" })
		portoneAPIURL = srv.URL

		if _, err := callPortoneAPI("/payments/pay_1", portoneSecret); err == nil {
			t.Fatal("expected an error when the PortOne API is unreachable")
		} else if !strings.Contains(err.Error(), "failed to call portone api") {
			t.Errorf("error = %v, want it to name the failed call", err)
		}
	})

	t.Run("unbuildable request is an error", func(t *testing.T) {
		// A control character in the endpoint cannot be put into a request
		// line; this is the only way the error path is reachable.
		if _, err := callPortoneAPI("/payments/bad\npath", portoneSecret); err == nil {
			t.Fatal("expected an error for an endpoint that cannot form a request")
		}
	})
}

func TestValidatePaymentAmount(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		paymentTotal int
		cartTotal    int
		wantErr      bool
	}{
		{"exact match", 500000, 5000, false},
		{"zero for zero", 0, 0, false},
		{"one cent short", 499999, 5000, true},
		{"one cent over", 500001, 5000, true},
		{"not scaled to minor units", 5000, 5000, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := validatePaymentAmount(tt.paymentTotal, tt.cartTotal, logging.New())
			if (err != nil) != tt.wantErr {
				t.Errorf("validatePaymentAmount(%d, %d) = %v, wantErr %v",
					tt.paymentTotal, tt.cartTotal, err, tt.wantErr)
			}
		})
	}
}

func TestValidatePaymentCurrency(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		payment string
		cart    string
		wantErr bool
	}{
		{"match", "USD", "USD", false},
		{"mismatch", "EUR", "USD", true},
		{"case sensitive", "usd", "USD", true},
		{"empty payment currency", "", "USD", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := validatePaymentCurrency(tt.payment, tt.cart, logging.New())
			if (err != nil) != tt.wantErr {
				t.Errorf("validatePaymentCurrency(%q, %q) = %v, wantErr %v",
					tt.payment, tt.cart, err, tt.wantErr)
			}
		})
	}
}

func TestValidateCartID(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		customData string
		expected   string
		wantErr    string
	}{
		{"match", `{"cart_id":"abc"}`, "abc", ""},
		{"mismatch", `{"cart_id":"abc"}`, "xyz", "cart ID mismatch"},
		{"missing field", `{}`, "abc", "cart ID mismatch"},
		{"not json", `{`, "abc", "failed to parse custom data"},
		{"json but not an object", `"abc"`, "abc", "failed to parse custom data"},
		{"empty custom data", ``, "abc", "failed to parse custom data"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := validateCartID(tt.customData, tt.expected, logging.New())
			switch {
			case tt.wantErr == "" && err != nil:
				t.Errorf("validateCartID(%q) = %v, want nil", tt.customData, err)
			case tt.wantErr != "" && err == nil:
				t.Errorf("validateCartID(%q) = nil, want %q", tt.customData, tt.wantErr)
			case tt.wantErr != "" && !strings.Contains(err.Error(), tt.wantErr):
				t.Errorf("validateCartID(%q) = %v, want it to contain %q", tt.customData, err, tt.wantErr)
			}
		})
	}
}

func TestVerifyWebhookSignature(t *testing.T) {
	t.Parallel()

	body := []byte(`{"type":"Transaction.Paid"}`)

	sign := func(secret string, payload []byte) string {
		mac := hmac.New(sha256.New, []byte(secret))
		mac.Write(payload)
		return hex.EncodeToString(mac.Sum(nil))
	}

	tests := []struct {
		name      string
		body      []byte
		signature string
		secret    string
		want      bool
	}{
		{"correct signature", body, sign(portoneSecret, body), portoneSecret, true},
		{"wrong secret", body, sign("another-secret", body), portoneSecret, false},
		{"signature of another body", body, sign(portoneSecret, []byte(`{}`)), portoneSecret, false},
		{"body modified after signing", append([]byte(` `), body...), sign(portoneSecret, body), portoneSecret, false},
		{"empty signature", body, "", portoneSecret, false},
		{"truncated signature", body, sign(portoneSecret, body)[:32], portoneSecret, false},
		{"uppercase hex is not accepted", body, strings.ToUpper(sign(portoneSecret, body)), portoneSecret, false},
		{"empty body and empty secret are self-consistent", nil, sign("", nil), "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := verifyWebhookSignature(tt.body, tt.signature, tt.secret); got != tt.want {
				t.Errorf("verifyWebhookSignature(%q, %q, %q) = %v, want %v",
					tt.body, tt.signature, tt.secret, got, tt.want)
			}
		})
	}
}

func TestCompletePortonePayment(t *testing.T) {
	app, _, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	app.Post("/api/payment/portone/complete", CompletePortonePayment)

	enablePortone(t, portoneSecret)

	var upstream http.HandlerFunc
	portoneStub(t, func(w http.ResponseWriter, r *http.Request) { upstream(w, r) })

	tests := []struct {
		name       string
		cartID     string
		cartAmount int
		body       string
		upstream   *upstreamResponse
		wantStatus int
		wantReason string
		check      func(t *testing.T, cart *models.Cart)
	}{
		{
			name:       "malformed request body",
			body:       "{not-json",
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "payment id missing",
			body:       `{"cart_id":"portonecart0001"}`,
			wantStatus: http.StatusBadRequest,
			wantReason: "Missing payment_id or cart_id",
		},
		{
			name:       "cart id missing",
			body:       `{"payment_id":"pay_1"}`,
			wantStatus: http.StatusBadRequest,
			wantReason: "Missing payment_id or cart_id",
		},
		{
			name:       "unknown cart",
			body:       `{"payment_id":"pay_1","cart_id":"nosuchcart000001"}`,
			wantStatus: http.StatusBadRequest,
			wantReason: "Cart not found",
		},
		{
			name:       "provider does not know the payment",
			cartID:     "portonecart0002",
			cartAmount: 5000,
			body:       `{"payment_id":"pay_1","cart_id":"portonecart0002"}`,
			upstream:   &upstreamResponse{http.StatusNotFound, `{"type":"NOT_FOUND"}`},
			wantStatus: http.StatusBadRequest,
			wantReason: "Payment not found",
		},
		{
			name:       "provider fails",
			cartID:     "portonecart0003",
			cartAmount: 5000,
			body:       `{"payment_id":"pay_1","cart_id":"portonecart0003"}`,
			upstream:   &upstreamResponse{http.StatusInternalServerError, `{"message":"boom"}`},
			wantStatus: http.StatusInternalServerError,
			check:      cartNotPaid,
		},
		{
			name:       "provider response is not json",
			cartID:     "portonecart0004",
			cartAmount: 5000,
			body:       `{"payment_id":"pay_1","cart_id":"portonecart0004"}`,
			upstream:   &upstreamResponse{http.StatusOK, `<html>maintenance</html>`},
			wantStatus: http.StatusBadRequest,
			wantReason: "Failed to decode payment",
			check:      cartNotPaid,
		},
		{
			name:       "payment not completed",
			cartID:     "portonecart0005",
			cartAmount: 5000,
			body:       `{"payment_id":"pay_1","cart_id":"portonecart0005"}`,
			upstream: &upstreamResponse{http.StatusOK,
				portonePaymentBody("FAILED", 500000, "USD", `{"cart_id":"portonecart0005"}`)},
			wantStatus: http.StatusBadRequest,
			wantReason: "Payment not completed: FAILED",
			check:      cartNotPaid,
		},
		{
			name:       "amount tampered below the cart total",
			cartID:     "portonecart0006",
			cartAmount: 5000,
			body:       `{"payment_id":"pay_1","cart_id":"portonecart0006"}`,
			upstream: &upstreamResponse{http.StatusOK,
				portonePaymentBody("PAID", 100, "USD", `{"cart_id":"portonecart0006"}`)},
			wantStatus: http.StatusBadRequest,
			wantReason: "amount mismatch",
			check:      cartNotPaid,
		},
		{
			name:       "currency mismatch",
			cartID:     "portonecart0007",
			cartAmount: 5000,
			body:       `{"payment_id":"pay_1","cart_id":"portonecart0007"}`,
			upstream: &upstreamResponse{http.StatusOK,
				portonePaymentBody("PAID", 500000, "EUR", `{"cart_id":"portonecart0007"}`)},
			wantStatus: http.StatusBadRequest,
			wantReason: "currency mismatch",
			check:      cartNotPaid,
		},
		{
			name:       "payment belongs to another cart",
			cartID:     "portonecart0008",
			cartAmount: 5000,
			body:       `{"payment_id":"pay_1","cart_id":"portonecart0008"}`,
			upstream: &upstreamResponse{http.StatusOK,
				portonePaymentBody("PAID", 500000, "USD", `{"cart_id":"someoneelse00001"}`)},
			wantStatus: http.StatusBadRequest,
			wantReason: "cart ID mismatch",
			check:      cartNotPaid,
		},
		{
			name:       "custom data is not json",
			cartID:     "portonecart0009",
			cartAmount: 5000,
			body:       `{"payment_id":"pay_1","cart_id":"portonecart0009"}`,
			upstream: &upstreamResponse{http.StatusOK,
				portonePaymentBody("PAID", 500000, "USD", "not-json")},
			wantStatus: http.StatusBadRequest,
			wantReason: "failed to parse custom data",
			check:      cartNotPaid,
		},
		{
			name:       "virtual account issued is a completed payment",
			cartID:     "portonecart0010",
			cartAmount: 5000,
			body:       `{"payment_id":"pay_1","cart_id":"portonecart0010"}`,
			upstream: &upstreamResponse{http.StatusOK,
				portonePaymentBody("VIRTUAL_ACCOUNT_ISSUED", 500000, "USD", `{"cart_id":"portonecart0010"}`)},
			wantStatus: http.StatusOK,
			check:      cartMarkedPaid,
		},
		{
			name:       "verified payment marks the cart paid",
			cartID:     "portonecart0011",
			cartAmount: 5000,
			body:       `{"payment_id":"pay_1","cart_id":"portonecart0011"}`,
			upstream: &upstreamResponse{http.StatusOK,
				portonePaymentBody("PAID", 500000, "USD", `{"cart_id":"portonecart0011"}`)},
			wantStatus: http.StatusOK,
			check:      cartMarkedPaid,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.cartID != "" {
				portoneCart(t, tt.cartID, tt.cartAmount)
			}

			if tt.upstream != nil {
				upstream = tt.upstream.serve()
			} else {
				upstream = unreachablePortone(t)
			}

			status, env, raw := readEnvelope(t, testutil.DoRequest(t, app, http.MethodPost,
				"/api/payment/portone/complete", tt.body, ""))

			if status != tt.wantStatus {
				t.Fatalf("status = %d, want %d; body: %s", status, tt.wantStatus, raw)
			}
			if tt.wantReason != "" && !strings.Contains(env.reason(), tt.wantReason) {
				t.Errorf("reason = %q, want it to contain %q", env.reason(), tt.wantReason)
			}
			if tt.check != nil {
				cart, err := queries.DB().Cart(context.Background(), tt.cartID)
				if err != nil {
					t.Fatalf("load cart: %v", err)
				}
				tt.check(t, cart)
			}
		})
	}

	t.Run("the response reports the provider status", func(t *testing.T) {
		portoneCart(t, "portonecart0012", 5000)
		upstream = upstreamResponse{http.StatusOK,
			portonePaymentBody("PAID", 500000, "USD", `{"cart_id":"portonecart0012"}`)}.serve()

		status, _, raw := readEnvelope(t, testutil.DoRequest(t, app, http.MethodPost,
			"/api/payment/portone/complete", `{"payment_id":"pay_1","cart_id":"portonecart0012"}`, ""))
		if status != http.StatusOK {
			t.Fatalf("status = %d, want 200; body: %s", status, raw)
		}

		var payload struct {
			Result struct {
				Status string `json:"status"`
			} `json:"result"`
		}
		if err := json.Unmarshal(raw, &payload); err != nil {
			t.Fatalf("decode response %s: %v", raw, err)
		}
		if payload.Result.Status != "PAID" {
			t.Errorf("status = %q, want PAID", payload.Result.Status)
		}
	})
}

// cartNotPaid asserts the cart was left untouched: a failed verification must
// never move an order forward.
func cartNotPaid(t *testing.T, cart *models.Cart) {
	t.Helper()
	if cart.PaymentStatus == litepay.PAID {
		t.Errorf("cart %s was marked paid by a request that did not verify", cart.ID)
	}
	if cart.PaymentID != "" {
		t.Errorf("cart %s carries payment id %q, want none", cart.ID, cart.PaymentID)
	}
}

// cartMarkedPaid asserts the cart was moved forward by a verified payment.
func cartMarkedPaid(t *testing.T, cart *models.Cart) {
	t.Helper()
	if cart.PaymentStatus != litepay.PAID {
		t.Errorf("payment status = %q, want %q", cart.PaymentStatus, litepay.PAID)
	}
	if cart.PaymentID != "pay_1" {
		t.Errorf("payment id = %q, want pay_1", cart.PaymentID)
	}
	// payment_system is not asserted: queries.UpdateCart writes only payment_id
	// and payment_status, so the `cart.PaymentSystem = "portone"` assignments in
	// the handlers do not reach the row. The column keeps what AddCart wrote.
}

// signWebhook produces the signature PortOne would send for a body.
func signWebhook(body, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(body))
	return hex.EncodeToString(mac.Sum(nil))
}

// postWebhook sends a webhook request with the signature header the handler
// reads. testutil.DoRequest cannot set headers, so the request is built here.
func postWebhook(t *testing.T, app *fiber.App, body, signature string) *http.Response {
	t.Helper()

	req := httptest.NewRequest(http.MethodPost, "/api/payment/portone/webhook", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if signature != "" {
		req.Header.Set("PortOne-Signature", signature)
	}

	resp, err := app.Test(req, fiber.TestConfig{Timeout: 10 * time.Second})
	if err != nil {
		t.Fatalf("webhook request: %v", err)
	}
	return resp
}

func TestPortoneWebhook(t *testing.T) {
	app, _, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	app.Post("/api/payment/portone/webhook", PortoneWebhook)

	enablePortone(t, portoneSecret)

	var upstream http.HandlerFunc
	portoneStub(t, func(w http.ResponseWriter, r *http.Request) { upstream(w, r) })

	// webhookBody is the notification itself; only data.paymentId is read from
	// it, the rest of the payment is fetched from the API.
	webhookBody := func(paymentID string) string {
		return fmt.Sprintf(`{"type":"Transaction.Paid","data":{"paymentId":%q}}`, paymentID)
	}

	tests := []struct {
		name       string
		cartID     string
		cartAmount int
		body       string
		signature  string // empty means "sign the body with the real secret"
		upstream   *upstreamResponse
		panicAPI   bool
		wantStatus int
		check      func(t *testing.T, cart *models.Cart)
	}{
		{
			name:       "no signature",
			cartID:     "webhookcart00001",
			cartAmount: 5000,
			body:       webhookBody("pay_1"),
			wantStatus: http.StatusUnauthorized,
			check:      cartNotPaid,
		},
		{
			name:       "signature that verifies against nothing",
			cartID:     "webhookcart00002",
			cartAmount: 5000,
			body:       webhookBody("pay_1"),
			signature:  "deadbeef",
			wantStatus: http.StatusUnauthorized,
			check:      cartNotPaid,
		},
		{
			name:       "signature of a different body",
			cartID:     "webhookcart00003",
			cartAmount: 5000,
			body:       webhookBody("pay_1"),
			signature:  signWebhook(`{}`, portoneSecret),
			wantStatus: http.StatusUnauthorized,
			check:      cartNotPaid,
		},
		{
			name:       "signature made with the wrong secret",
			cartID:     "webhookcart00004",
			cartAmount: 5000,
			body:       webhookBody("pay_1"),
			signature:  signWebhook(webhookBody("pay_1"), "not-the-secret"),
			wantStatus: http.StatusUnauthorized,
			check:      cartNotPaid,
		},
		{
			name:       "signed payload that is not json",
			cartID:     "webhookcart00005",
			cartAmount: 5000,
			body:       "<not json>",
			wantStatus: http.StatusBadRequest,
			check:      cartNotPaid,
		},
		{
			name:       "signed payload without a payment id",
			cartID:     "webhookcart00006",
			cartAmount: 5000,
			body:       `{"type":"Transaction.Paid","data":{}}`,
			wantStatus: http.StatusBadRequest,
			check:      cartNotPaid,
		},
		{
			// The API is down. Answering 200 keeps PortOne from retrying a
			// notification this server cannot act on either way.
			name:       "provider api is unreachable",
			cartID:     "webhookcart00007",
			cartAmount: 5000,
			body:       webhookBody("pay_1"),
			panicAPI:   true,
			wantStatus: http.StatusOK,
			check:      cartNotPaid,
		},
		{
			name:       "provider refuses to return the payment",
			cartID:     "webhookcart00008",
			cartAmount: 5000,
			body:       webhookBody("pay_1"),
			upstream:   &upstreamResponse{http.StatusForbidden, `{"message":"nope"}`},
			wantStatus: http.StatusOK,
			check:      cartNotPaid,
		},
		{
			name:       "provider response is not json",
			cartID:     "webhookcart00009",
			cartAmount: 5000,
			body:       webhookBody("pay_1"),
			upstream:   &upstreamResponse{http.StatusOK, `garbage`},
			wantStatus: http.StatusOK,
			check:      cartNotPaid,
		},
		{
			name:       "custom data is not json",
			cartID:     "webhookcart00010",
			cartAmount: 5000,
			body:       webhookBody("pay_1"),
			upstream: &upstreamResponse{http.StatusOK,
				portonePaymentBody("PAID", 500000, "USD", "not-json")},
			wantStatus: http.StatusOK,
			check:      cartNotPaid,
		},
		{
			name:       "custom data names a cart that does not exist",
			cartID:     "webhookcart00011",
			cartAmount: 5000,
			body:       webhookBody("pay_1"),
			upstream: &upstreamResponse{http.StatusOK,
				portonePaymentBody("PAID", 500000, "USD", `{"cart_id":"nosuchcart000001"}`)},
			wantStatus: http.StatusOK,
		},
		{
			name:       "amount does not match the cart",
			cartID:     "webhookcart00012",
			cartAmount: 5000,
			body:       webhookBody("pay_1"),
			upstream: &upstreamResponse{http.StatusOK,
				portonePaymentBody("PAID", 100, "USD", `{"cart_id":"webhookcart00012"}`)},
			wantStatus: http.StatusOK,
			check:      cartNotPaid,
		},
		{
			name:       "currency does not match the cart",
			cartID:     "webhookcart00013",
			cartAmount: 5000,
			body:       webhookBody("pay_1"),
			upstream: &upstreamResponse{http.StatusOK,
				portonePaymentBody("PAID", 500000, "KRW", `{"cart_id":"webhookcart00013"}`)},
			wantStatus: http.StatusOK,
			check:      cartNotPaid,
		},
		{
			name:       "payment is not completed",
			cartID:     "webhookcart00014",
			cartAmount: 5000,
			body:       webhookBody("pay_1"),
			upstream: &upstreamResponse{http.StatusOK,
				portonePaymentBody("FAILED", 500000, "USD", `{"cart_id":"webhookcart00014"}`)},
			wantStatus: http.StatusOK,
			check:      cartNotPaid,
		},
		{
			name:       "completed payment marks the cart paid",
			cartID:     "webhookcart00015",
			cartAmount: 5000,
			body:       webhookBody("pay_1"),
			upstream: &upstreamResponse{http.StatusOK,
				portonePaymentBody("PAID", 500000, "USD", `{"cart_id":"webhookcart00015"}`)},
			wantStatus: http.StatusOK,
			check:      cartMarkedPaid,
		},
		{
			name:       "virtual account issued marks the cart paid",
			cartID:     "webhookcart00016",
			cartAmount: 5000,
			body:       webhookBody("pay_1"),
			upstream: &upstreamResponse{http.StatusOK,
				portonePaymentBody("VIRTUAL_ACCOUNT_ISSUED", 500000, "USD", `{"cart_id":"webhookcart00016"}`)},
			wantStatus: http.StatusOK,
			check:      cartMarkedPaid,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.cartID != "" {
				portoneCart(t, tt.cartID, tt.cartAmount)
			}

			switch {
			case tt.panicAPI:
				// A handler that panics closes the connection without a
				// response, which is what an API that dies mid-call looks like
				// to the client.
				upstream = func(http.ResponseWriter, *http.Request) { panic("portone api exploded") }
			case tt.upstream != nil:
				upstream = tt.upstream.serve()
			default:
				upstream = unreachablePortone(t)
			}
			signature := tt.signature
			if signature == "" && tt.wantStatus != http.StatusUnauthorized {
				signature = signWebhook(tt.body, portoneSecret)
			}

			resp := postWebhook(t, app, tt.body, signature)
			if resp.StatusCode != tt.wantStatus {
				raw, _ := io.ReadAll(resp.Body)
				_ = resp.Body.Close()
				t.Fatalf("status = %d, want %d; body: %s", resp.StatusCode, tt.wantStatus, raw)
			}
			_ = resp.Body.Close()

			if tt.check != nil {
				cart, err := queries.DB().Cart(context.Background(), tt.cartID)
				if err != nil {
					t.Fatalf("load cart: %v", err)
				}
				tt.check(t, cart)
			}
		})
	}

	t.Run("the api is not called before the signature verifies", func(t *testing.T) {
		portoneCart(t, "webhookcart00017", 5000)
		upstream = unreachablePortone(t)

		body := webhookBody("pay_1")
		resp := postWebhook(t, app, body, "not-a-signature")
		_ = resp.Body.Close()
		if resp.StatusCode != http.StatusUnauthorized {
			t.Fatalf("status = %d, want 401", resp.StatusCode)
		}
	})
}
