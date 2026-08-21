package middleware

import (
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/limiter"
)

// authLimiter throttles brute-forceable endpoints (login, install, payment
// initiation/callbacks) per IP: 10 requests per minute is far above human
// usage while making online password guessing impractical.
var authLimiter = limiter.New(limiter.Config{
	Max:        10,
	Expiration: time.Minute,
})

// AuthLimiter returns the shared per-IP rate limiter handler.
func AuthLimiter() fiber.Handler {
	return authLimiter
}
