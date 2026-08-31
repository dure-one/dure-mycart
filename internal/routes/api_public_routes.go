package routes

import (
	"github.com/gofiber/fiber/v3"

	handlers "github.com/shurco/mycart/internal/handlers/public"
	"github.com/shurco/mycart/internal/middleware"
)

// ApiPublicRoutes sets up public API routes accessible without authentication.
func ApiPublicRoutes(c *fiber.App) {
	c.Get("/ping", handlers.Ping)

	c.Get("/api/settings", handlers.Settings)
	c.Get("/api/pages/:page_slug", handlers.Page)

	// Seller info with captcha protection
	c.Get("/api/sellerinfo/captcha", handlers.GenerateCaptcha)
	c.Post("/api/sellerinfo/verify", handlers.VerifyCaptcha)
	c.Get("/api/sellerinfo", handlers.GetSellerInfo)

	product := c.Group("/api/products")
	product.Get("/", handlers.Products)
	// Addressed by slug: the storefront links to /products/<slug>, and the
	// public query matches on it.
	product.Get("/:product_slug", handlers.Product)

	// Every route below is driven by the cabinet's session cookie, so it is the
	// cross-site check that has to come first — the earlier it runs, the smaller
	// the surface a forged request can reach.
	c.Use("/api/customer/", middleware.CSRFProtect())

	// Storefront customer cabinet. The whole group is gated on the account
	// setting, so an installation that does not want it answers 404 here
	// exactly as it would if these routes were never registered — while a
	// change to the setting takes effect on the next request.
	//
	// Sign-in and sign-up share the auth rate limiter with the admin sign-in
	// and the payment endpoints: they are the password-guessing surface.
	customer := c.Group("/api/customer", middleware.AccountEnabled())
	customer.Post("/signup", middleware.AuthLimiter(), handlers.CustomerSignUp)
	customer.Post("/signin", middleware.AuthLimiter(), handlers.CustomerSignIn)
	customer.Post("/signout", middleware.CustomerJWTProtected(), handlers.CustomerSignOut)
	customer.Get("/me", middleware.CustomerJWTProtected(), handlers.CustomerMe)
	customer.Get("/purchases", middleware.CustomerJWTProtected(), handlers.CustomerPurchases)
	// The file id is addressed on its own rather than under its product: the
	// buyer knows which guide they clicked, not which product row it came from,
	// and the endpoint looks the product up to check the entitlement anyway.
	customer.Get("/purchases/:file_id<len(15)>/download", middleware.CustomerJWTProtected(), handlers.CustomerDownload)

	cart := c.Group("/cart")
	cart.Post("/payment", middleware.AuthLimiter(), handlers.Payment)
	cart.Post("/payment/callback", middleware.AuthLimiter(), handlers.PaymentCallback)

	c.Get("/api/cart/payment", handlers.PaymentList)
	c.Post("/api/cart/create", handlers.CreateCart)
	c.Get("/api/cart/portone-config", handlers.GetPortoneConfig)
	c.Get("/api/cart/:cart_id", handlers.GetCart)
	c.Post("/api/payment/portone/complete", handlers.CompletePortonePayment)
	c.Post("/api/payment/portone/webhook", handlers.PortoneWebhook)
}
