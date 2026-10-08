package middleware

import (
	"github.com/gofiber/fiber/v3"
)

// CacheControl sets Cache-Control and Vary headers based on route patterns.
// Runs after handlers to inspect final response status and Set-Cookie headers.
func CacheControl() fiber.Handler {
	return func(c fiber.Ctx) error {
		// Only cache GET/HEAD requests
		method := c.Method()
		if method != fiber.MethodGet && method != fiber.MethodHead {
			return c.Next()
		}

		// Get policy for this path before running handler
		path := c.Path()
		policy := match(path)

		// Execute handler first to get final response status
		err := c.Next()

		// After handler runs, check if handler set Cache-Control (allow override)
		if len(c.Response().Header.Peek(fiber.HeaderCacheControl)) > 0 {
			return err
		}

		// Force no-store on error responses (non-2xx)
		status := c.Response().StatusCode()
		if status < 200 || status >= 300 {
			c.Set(fiber.HeaderCacheControl, "no-store")
			return err
		}

		// Force no-store, private if Set-Cookie header present
		setCookieHeaders := c.Response().Header.Peek("Set-Cookie")
		if len(setCookieHeaders) > 0 {
			c.Set(fiber.HeaderCacheControl, "no-store, private")
			c.Set(fiber.HeaderVary, "Cookie, Authorization")
			return err
		}

		// Set cache headers
		c.Set(fiber.HeaderCacheControl, policy.Header())
		if vary := policy.VaryHeader(); vary != "" {
			c.Set(fiber.HeaderVary, vary)
		}

		return err
	}
}
