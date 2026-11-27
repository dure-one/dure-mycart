//go:build sqlc

package queries

import (
	"context"
	"database/sql"
	"encoding/json"

	"github.com/shurco/mycart/internal/models"
	pggen "github.com/dure-one/dure-mycart/internal/queries/sqlcgen/postgres"
	sqlitegen "github.com/dure-one/dure-mycart/internal/queries/sqlcgen/sqlite"
	"github.com/shurco/mycart/pkg/errors"
)

// Product retrieves a product by its ID, with the option to fetch private or public data.
// Private reads look the product up by id, public ones by slug among the
// published products.
func (q *ProductQueries) Product(ctx context.Context, private bool, id string) (*models.Product, error) {
	row, err := pick(q.DB, q.DB,
		func(g *pggen.Queries) (pggen.ProductDetailByIDRow, error) {
			if private {
				return g.ProductDetailByID(ctx, id)
			}
			r, err := g.ProductDetailBySlug(ctx, id)
			return pggen.ProductDetailByIDRow(r), err
		},
		func(g *sqlitegen.Queries) (pggen.ProductDetailByIDRow, error) {
			if private {
				r, err := g.ProductDetailByID(ctx, id)
				return pggen.ProductDetailByIDRow(r), err
			}
			r, err := g.ProductDetailBySlug(ctx, id)
			return pggen.ProductDetailByIDRow(r), err
		})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, errors.ErrProductNotFound
		}
		return nil, err
	}

	product, err := productFromDetailRow(row, private)
	if err != nil {
		return nil, err
	}

	// Load options and variants if product has variants
	if product.HasVariants {
		if err := q.loadDetailVariantData(ctx, product); err != nil {
			return nil, err
		}
	}

	return product, nil
}

// productFromDetailRow decodes the single-product row into a models.Product.
func productFromDetailRow(r pggen.ProductDetailByIDRow, private bool) (*models.Product, error) {
	product := &models.Product{
		Name:        r.Name,
		Brief:       r.Brief,
		Description: r.Desc,
		Slug:        r.Slug,
		Amount:      int(r.Amount),
		Quantity:    int(r.Quantity),
		SKU:         r.Sku.String,
		HasVariants: r.HasVariants,
		Active:      r.Active,
	}
	product.ID, product.Created, product.Updated = r.ID, r.Created, r.Updated

	if r.Images != "" {
		if err := decodeFiles(r.Images, &product.Images); err != nil {
			return nil, err
		}
	}
	if err := json.Unmarshal([]byte(r.Attribute), &product.Attributes); err != nil {
		return nil, err
	}
	if err := json.Unmarshal([]byte(r.Metadata), &product.Metadata); err != nil {
		return nil, err
	}

	product.Digital.Type = r.Digital.String
	if private && r.Digital.Valid {
		product.Digital.Filled = r.DigitalFilled
	}

	if err := json.Unmarshal([]byte(r.Seo), &product.Seo); err != nil {
		return nil, err
	}

	return product, nil
}
