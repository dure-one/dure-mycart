package handlers

import (
	"database/sql"
	"errors"
	"fmt"

	"github.com/disintegration/imaging"
	"github.com/gofiber/fiber/v3"

	_ "github.com/shurco/mycart/internal/models"
	"github.com/shurco/mycart/internal/store"
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
	log := logging.New()
	p := webutil.ParsePagination(c)

	products, err := store.ListProducts(c.Context(), false, p.Limit, p.Offset, "")
	if err != nil {
		log.ErrorStack(err)
		return webutil.StatusInternalServerError(c)
	}

	return webutil.Response(c, fiber.StatusOK, "Products", products)
}

// Product returns a single active product by ID for public access.
//
// @Summary      Get active product
// @Description  Get a single active product by its ID
// @Tags         Public
// @Produce      json
// @Param        product_id path string true "Product ID"
// @Success      200 {object} webutil.HTTPResponse{result=models.Product} "Product details"
// @Failure      500 {object} webutil.HTTPResponse "Internal server error"
// @Router       /api/products/{product_id} [get]
func Product(c fiber.Ctx) error {
	productID := c.Params("product_id")
	log := logging.New()

	product, err := store.Product(c.Context(), false, productID)
	if err != nil {
		log.ErrorStack(err)
		return webutil.StatusInternalServerError(c)
	}

	return webutil.Response(c, fiber.StatusOK, "Product info", product)
}

// GetProductRepresentativeImage serves the first product image as PNG.
// Converts JPEG to PNG on-the-fly if needed.
//
// @Summary      Get product representative image
// @Description  Serves first product image (by position) as PNG with JPEG conversion
// @Tags         Public
// @Produce      png
// @Param        slug path string true "Product slug"
// @Success      200 {file} image/png "Product image"
// @Failure      500 {object} webutil.HTTPResponse "Internal server error"
// @Router       /products/{slug}.png [get]
func GetProductRepresentativeImage(c fiber.Ctx) error {
	slug := c.Params("slug")
	log := logging.New()

	image, err := store.GetProductRepImage(c.Context(), slug)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			// Serve placeholder
			placeholderPath := "./cmd/lc_uploads/product-placeholder.png"
			c.Set("Content-Type", "image/png")
			return c.SendFile(placeholderPath)
		}
		log.ErrorStack(err)
		return webutil.StatusInternalServerError(c)
	}

	filePath := fmt.Sprintf("./cmd/lc_uploads/%s.%s", image.Name, image.Ext)

	// If already PNG, serve directly
	if image.Ext == "png" {
		c.Set("Content-Type", "image/png")
		c.Set("Cache-Control", "public, max-age=3600")
		return c.SendFile(filePath)
	}

	// Convert JPEG to PNG
	if image.Ext == "jpg" || image.Ext == "jpeg" {
		src, err := imaging.Open(filePath)
		if err != nil {
			log.ErrorStack(err)
			return webutil.StatusInternalServerError(c)
		}

		c.Set("Content-Type", "image/png")
		c.Set("Cache-Control", "public, max-age=3600")

		if err := imaging.Encode(c.Response().BodyWriter(), src, imaging.PNG); err != nil {
			log.ErrorStack(err)
			return webutil.StatusInternalServerError(c)
		}
		return nil
	}

	// Unsupported format, serve as-is
	return c.SendFile(filePath)
}
