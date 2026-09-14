package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"

	"github.com/shurco/mycart/internal/testutil"
)

// csrfProbe mounts the middleware in front of a handler that answers 200, and
// returns the status the middleware produced.
func csrfProbe(t *testing.T, method, host string, headers map[string]string) int {
	t.Helper()

	app := fiber.New()
	app.Use(CSRFProtect())
	app.All("/", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) })
	t.Cleanup(func() { _ = app.Shutdown() })

	req := httptest.NewRequest(method, "/", nil)
	req.Host = host
	for name, value := range headers {
		req.Header.Set(name, value)
	}

	// The default one-second ceiling is too tight here: the middleware reads the
	// site settings from the database, and a run under -race on PostgreSQL
	// takes longer than that. A timeout would surface as a failure of whatever
	// case happened to be slowest.
	resp, err := app.Test(req, fiber.TestConfig{Timeout: 10 * time.Second})
	if err != nil {
		t.Fatalf("%s %s: %v", method, host, err)
	}
	defer func() { _ = resp.Body.Close() }()

	return resp.StatusCode
}

// TestCSRFProtect_Origin covers the rule the middleware exists for: a request
// that announces where it came from has to have come from this shop.
func TestCSRFProtect_Origin(t *testing.T) {
	t.Cleanup(testutil.SetupTestDB(t))

	const host = "shop.example.com"

	cases := []struct {
		name   string
		method string
		origin string
		want   int
	}{
		// A page on another site posting here is the attack: the browser
		// attaches the session cookie whatever page asked.
		{"cross-site origin", http.MethodPost, "https://evil.example", http.StatusForbidden},
		{"cross-site origin on a lookalike host", http.MethodPost, "https://shop.example.com.evil.example", http.StatusForbidden},
		// A sibling subdomain is same-site, so SameSite would have let its
		// cookie through; the host comparison is what stops it.
		{"sibling subdomain", http.MethodPost, "https://evil.shop.example.com", http.StatusForbidden},
		// Sandboxed iframes and data: pages send this, and they are exactly
		// what the check is for.
		{"null origin", http.MethodPost, "null", http.StatusForbidden},
		{"malformed origin", http.MethodPost, "://", http.StatusForbidden},

		{"same origin", http.MethodPost, "https://shop.example.com", http.StatusOK},
		// The storefront's development server reaches the API through a proxy
		// on a port of its own.
		{"same host, other port", http.MethodPost, "http://shop.example.com:5173", http.StatusOK},
		{"other scheme", http.MethodPost, "http://shop.example.com", http.StatusOK},
		// The site domain from the settings, which is what a request carries
		// when a reverse proxy rewrites the Host header to the backend.
		{"configured site domain", http.MethodPost, "https://site.com", http.StatusOK},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := csrfProbe(t, tc.method, host, map[string]string{"Origin": tc.origin})
			if got != tc.want {
				t.Errorf("POST with Origin %q = %d, want %d", tc.origin, got, tc.want)
			}
		})
	}
}

// TestCSRFProtect_SafeMethods checks that reads are left alone: they change
// nothing, and a shop that answered 403 to a cross-site GET would break every
// link into it.
func TestCSRFProtect_SafeMethods(t *testing.T) {
	t.Cleanup(testutil.SetupTestDB(t))

	for _, method := range []string{http.MethodGet, http.MethodHead, http.MethodOptions} {
		got := csrfProbe(t, method, "shop.example.com", map[string]string{"Origin": "https://evil.example"})
		if got != http.StatusOK {
			t.Errorf("%s = %d, want 200", method, got)
		}
	}
}

// TestCSRFProtect_RefererFallback covers the clients that send only a Referer:
// the check has to read it rather than see no origin and let everything past.
func TestCSRFProtect_RefererFallback(t *testing.T) {
	t.Cleanup(testutil.SetupTestDB(t))

	const host = "shop.example.com"

	cases := []struct {
		name    string
		referer string
		want    int
	}{
		{"cross-site referer", "https://evil.example/attack", http.StatusForbidden},
		{"same-site referer", "https://shop.example.com/cart", http.StatusOK},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := csrfProbe(t, http.MethodPost, host, map[string]string{"Referer": tc.referer})
			if got != tc.want {
				t.Errorf("POST with Referer %q = %d, want %d", tc.referer, got, tc.want)
			}
		})
	}
}

// TestCSRFProtect_OriginDecidesOverReferer pins the precedence: when a request
// carries both, the Origin is the one that counts, so a forgery cannot smuggle
// itself through by attaching a plausible Referer.
func TestCSRFProtect_OriginDecidesOverReferer(t *testing.T) {
	t.Cleanup(testutil.SetupTestDB(t))

	got := csrfProbe(t, http.MethodPost, "shop.example.com", map[string]string{
		"Origin":  "https://evil.example",
		"Referer": "https://shop.example.com/cart",
	})
	if got != http.StatusForbidden {
		t.Errorf("cross-site Origin with a same-site Referer = %d, want 403", got)
	}
}

// TestCSRFProtect_NoBrowserHeaders checks that a client which sends neither
// header is let through. It cannot be a browser making a cross-site request —
// every one sends at least one of the two — and it holds its own credentials
// rather than borrowing the victim's, which is the whole of what CSRF is.
func TestCSRFProtect_NoBrowserHeaders(t *testing.T) {
	t.Cleanup(testutil.SetupTestDB(t))

	got := csrfProbe(t, http.MethodPost, "shop.example.com", nil)
	if got != http.StatusOK {
		t.Errorf("POST without Origin or Referer = %d, want 200", got)
	}
}

// TestCSRFProtect_Loopback covers the development setup: the frontends call
// the API through a proxy, and localhost, 127.0.0.1 and ::1 are the same
// machine under three names.
func TestCSRFProtect_Loopback(t *testing.T) {
	t.Cleanup(testutil.SetupTestDB(t))

	cases := []struct {
		host   string
		origin string
		want   int
	}{
		{"localhost:8080", "http://localhost:5173", http.StatusOK},
		{"127.0.0.1:8080", "http://localhost:5173", http.StatusOK},
		{"localhost:8080", "http://127.0.0.1:5173", http.StatusOK},
		// A public shop is not reachable from a page on the operator's laptop,
		// however local that page is: the names have to be the same kind.
		{"shop.example.com", "http://localhost:5173", http.StatusForbidden},
	}

	for _, tc := range cases {
		t.Run(tc.host+"<-"+tc.origin, func(t *testing.T) {
			got := csrfProbe(t, http.MethodPost, tc.host, map[string]string{"Origin": tc.origin})
			if got != tc.want {
				t.Errorf("Host %q, Origin %q = %d, want %d", tc.host, tc.origin, got, tc.want)
			}
		})
	}
}
