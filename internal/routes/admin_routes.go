package routes

import (
	"io/fs"

	"github.com/gofiber/fiber/v3"

	"github.com/shurco/mycart/web"
)

func AdminRoutes(c *fiber.App) {
	embedAdmin, _ := fs.Sub(web.EmbedAdmin(), web.AdminBuildPath)

	// The admin panel is not a page of the shop, and the only robots.txt a
	// crawler reads is the site's — the copy inside the admin build sits at
	// /_/robots.txt, where nothing looks for it. The header is what keeps the
	// panel out of search results, and with it the address that carries the
	// sign-in form.
	c.Use("/_", func(c fiber.Ctx) error {
		c.Set(fiber.HeaderXRobotsTag, "noindex, nofollow")
		return c.Next()
	})

	c.Use("/_", setupSPAHandler(embedAdmin, func(string) bool { return false }, "/_"))
}
