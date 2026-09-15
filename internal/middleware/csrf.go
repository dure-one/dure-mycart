package middleware

import (
	"context"
	"net/url"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"

	"github.com/shurco/mycart/internal/models"
	"github.com/shurco/mycart/internal/queries"
	"github.com/shurco/mycart/pkg/logging"
	"github.com/shurco/mycart/pkg/webutil"
)

// CSRFProtect rejects state-changing requests that a browser reports as coming
// from another site.
//
// Both surfaces authenticate with a cookie, and a cookie is attached by the
// browser whatever page asked for the request. SameSite already keeps the
// browser from sending it on a cross-site POST, but that is a property of the
// browser's idea of a site: a sibling subdomain, or a client that predates the
// attribute, is outside it. What cannot be spoofed from a page is the Origin
// header, so the rule here is simply that a request which announces where it
// came from has to have come from this shop.
//
// It is mounted only on the API surfaces that are driven by a session cookie —
// the admin panel, the storefront cabinet and the sign-in endpoints. The
// payment callbacks are deliberately left out: they are posted by the payment
// providers' own pages, whose Origin is theirs and never this shop's.
func CSRFProtect() fiber.Handler {
	return func(c fiber.Ctx) error {
		switch c.Method() {
		case fiber.MethodGet, fiber.MethodHead, fiber.MethodOptions:
			return c.Next()
		}

		// A browser sends Origin on every unsafe request, same-origin ones
		// included, so it decides on its own when it is there. Referer is the
		// fallback for the few clients that send only that.
		origin, header := c.Get(fiber.HeaderOrigin), fiber.HeaderOrigin
		if origin == "" {
			origin, header = c.Get(fiber.HeaderReferer), fiber.HeaderReferer
		}
		// Neither header: not a browser. CSRF is a browser's trick — a script
		// has to hold the token itself to use it — so there is nothing to
		// defend against here, and rejecting would only break API clients.
		if origin == "" {
			return c.Next()
		}

		if originBelongsToShop(c, origin) {
			return c.Next()
		}

		logging.New().Warn().
			Str("method", c.Method()).
			Str("path", c.Path()).
			Str("header", header).
			Str("origin", origin).
			Str("host", requestHost(c)).
			Msg("cross-site request rejected")

		return webutil.Response(c, fiber.StatusForbidden,
			"forbidden", "cross-site request rejected")
	}
}

// originBelongsToShop reports whether an Origin or Referer value names this
// shop.
//
// The comparison is on the host alone, ignoring the port: the storefront's
// development server reaches the API through a proxy under a port of its own,
// and a shop that answers on several ports is still one shop. The configured
// site domain is accepted as well, because a reverse proxy is free to rewrite
// the Host header to the address it forwards to.
func originBelongsToShop(c fiber.Ctx, raw string) bool {
	parsed, err := url.Parse(raw)
	if err != nil {
		return false
	}

	host := strings.ToLower(parsed.Hostname())
	// An Origin of "null" — a sandboxed iframe, a data: page — has no host, and
	// is exactly the case this middleware exists for.
	if host == "" || host == "null" {
		return false
	}

	own := requestHost(c)
	if host == own {
		return true
	}

	// localhost, 127.0.0.1 and ::1 are the same machine under three names, and
	// the development frontends use a different one than the API does.
	if isLoopbackHost(host) && isLoopbackHost(own) {
		return true
	}

	return host == shopDomain(c)
}

// requestHost is the host this request was addressed to, without its port.
//
// The host is taken from the request line rather than from c.Hostname(), which
// prefers X-Forwarded-Host whenever the peer is a trusted proxy — and this app
// trusts loopback and private peers (internal/app.go). That header is written by
// whoever sent the request, so anchoring the same-origin check to it would let a
// request name the origin it claims to have come from: send X-Forwarded-Host and
// an Origin of the same value and the check agrees with itself. The Host header
// is filled in by the client because it says where the client connected, which
// is the question being asked.
func requestHost(c fiber.Ctx) string {
	raw := string(c.RequestCtx().URI().Host())
	if parsed, err := url.Parse("//" + raw); err == nil && parsed.Hostname() != "" {
		return strings.ToLower(parsed.Hostname())
	}
	return strings.ToLower(raw)
}

// shopDomain returns the host the shop is published under, as configured in
// the site settings, or "" when it cannot be read.
func shopDomain(c fiber.Ctx) string {
	ctx, cancel := context.WithTimeout(c.Context(), 5*time.Second)
	defer cancel()

	settingMain, err := queries.GetSettingByGroup[models.Main](ctx, queries.DB())
	if err != nil {
		logging.New().ErrorStack(err)
		return ""
	}

	// The setting is documented and validated as a bare host, but a value that
	// carries a scheme is read for its host rather than compared as a URL.
	domain := strings.TrimSpace(settingMain.Domain)
	if parsed, err := url.Parse(domain); err == nil && parsed.Hostname() != "" {
		domain = parsed.Hostname()
	}

	return strings.ToLower(strings.TrimSuffix(domain, "."))
}

// isLoopbackHost reports whether a hostname names the local machine.
func isLoopbackHost(host string) bool {
	switch strings.ToLower(host) {
	case "localhost", "127.0.0.1", "::1", "[::1]":
		return true
	}
	return false
}
