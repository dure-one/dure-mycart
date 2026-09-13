package handlers

import (
	"github.com/gofiber/fiber/v3"

	_ "github.com/shurco/mycart/internal/models"
	"github.com/shurco/mycart/internal/queries"
	"github.com/shurco/mycart/pkg/errors"
	"github.com/shurco/mycart/pkg/logging"
	"github.com/shurco/mycart/pkg/webutil"
)

// Products returns a list of all active products for public access.
//
// @Summary      List active products
// @Description  Get paginated list of active products visible to customers
// @Tags         Public
// @Produce      json
// @Param        page  query int false "Page number" default(1)
// @Param        limit query int false "Items per page" default(20)
// @Success      200 {object} webutil.HTTPResponse{result=models.Products} "Products list"
// @Failure      500 {object} webutil.HTTPResponse "Internal server error"
// @Router       /api/products [get]
func Products(c fiber.Ctx) error {
	db := queries.DB()
	log := logging.New()

	p := webutil.ParsePagination(c)

	products, err := db.ListProducts(c.Context(), false, p.Limit, p.Offset, "")
	if err != nil {
		log.ErrorStack(err)
		return webutil.StatusInternalServerError(c)
	}

	return webutil.Response(c, fiber.StatusOK, "Products", products)
}

// Product returns a single active product by slug for public access.
//
// The address is the slug and not the row id: the storefront links to
// /products/<slug>, and the public query behind this handler matches on the
// slug. Asking for something the shop does not have is a 404 — the product is
// simply not there — and not the 500 an unclassified error would produce.
//
// @Summary      Get active product
// @Description  Get a single active product by its slug
// @Tags         Public
// @Produce      json
// @Param        product_slug path string true "Product slug"
// @Success      200 {object} webutil.HTTPResponse{result=models.Product} "Product details"
// @Failure      404 {object} webutil.HTTPResponse "Product not found"
// @Failure      500 {object} webutil.HTTPResponse "Internal server error"
// @Router       /api/products/{product_slug} [get]
func Product(c fiber.Ctx) error {
	productSlug := c.Params("product_slug")
	db := queries.DB()
	log := logging.New()

	product, err := db.Product(c.Context(), false, productSlug)
	if err != nil {
		if errors.Is(err, errors.ErrProductNotFound) {
			return webutil.StatusNotFound(c)
		}
		log.ErrorStack(err)
		return webutil.StatusInternalServerError(c)
	}

	return webutil.Response(c, fiber.StatusOK, "Product info", product)
}
