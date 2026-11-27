//go:build sqlc

package queries

import (
	"context"
	"encoding/json"

	"github.com/shurco/mycart/internal/models"
	pggen "github.com/dure-one/dure-mycart/internal/queries/sqlcgen/postgres"
	sqlitegen "github.com/dure-one/dure-mycart/internal/queries/sqlcgen/sqlite"
)

// loadDetailVariantData loads the options, option values and variants of a
// product that has variants.
func (q *ProductQueries) loadDetailVariantData(ctx context.Context, product *models.Product) error {
	if err := q.loadDetailOptions(ctx, product); err != nil {
		return err
	}
	if err := q.loadDetailOptionValues(ctx, product); err != nil {
		return err
	}
	return q.loadDetailVariants(ctx, product)
}

// productOptions reads the options of a product, in position order.
func (q *ProductQueries) productOptions(ctx context.Context, productID string) ([]models.ProductOption, error) {
	rows, err := pick(q.DB, q.DB,
		func(g *pggen.Queries) ([]pggen.OptionsByProductRow, error) { return g.OptionsByProduct(ctx, productID) },
		func(g *sqlitegen.Queries) ([]pggen.OptionsByProductRow, error) {
			r, err := g.OptionsByProduct(ctx, productID)
			return convertAll(r, err, func(x sqlitegen.OptionsByProductRow) pggen.OptionsByProductRow { return pggen.OptionsByProductRow(x) })
		})
	if err != nil {
		return nil, err
	}

	var options []models.ProductOption
	for _, r := range rows {
		options = append(options, models.ProductOption{ID: r.ID, ProductID: productID, Name: r.Name, Position: int(r.Position)})
	}
	return options, nil
}

// loadDetailOptions loads the options of the product, in position order.
func (q *ProductQueries) loadDetailOptions(ctx context.Context, product *models.Product) error {
	options, err := q.productOptions(ctx, product.ID)
	if err != nil {
		return err
	}
	product.Options = append(product.Options, options...)
	return nil
}

// loadDetailOptionValues attaches the values to the options loaded before it.
func (q *ProductQueries) loadDetailOptionValues(ctx context.Context, product *models.Product) error {
	if len(product.Options) == 0 {
		return nil
	}

	rows, err := pick(q.DB, q.DB,
		func(g *pggen.Queries) ([]pggen.OptionValuesByProductRow, error) {
			return g.OptionValuesByProduct(ctx, product.ID)
		},
		func(g *sqlitegen.Queries) ([]pggen.OptionValuesByProductRow, error) {
			r, err := g.OptionValuesByProduct(ctx, product.ID)
			return convertAll(r, err, func(x sqlitegen.OptionValuesByProductRow) pggen.OptionValuesByProductRow {
				return pggen.OptionValuesByProductRow(x)
			})
		})
	if err != nil {
		return err
	}

	for _, r := range rows {
		value := models.ProductOptionValue{ID: r.ID, OptionID: r.OptionID, Value: r.Value, Position: int(r.Position)}
		for i := range product.Options {
			if product.Options[i].ID == value.OptionID {
				product.Options[i].Values = append(product.Options[i].Values, value)
				break
			}
		}
	}
	return nil
}

// loadDetailVariants loads the live variants of the product.
func (q *ProductQueries) loadDetailVariants(ctx context.Context, product *models.Product) error {
	rows, err := pick(q.DB, q.DB,
		func(g *pggen.Queries) ([]pggen.LiveVariantsByProductRow, error) {
			return g.LiveVariantsByProduct(ctx, product.ID)
		},
		func(g *sqlitegen.Queries) ([]pggen.LiveVariantsByProductRow, error) {
			r, err := g.LiveVariantsByProduct(ctx, product.ID)
			return convertAll(r, err, func(x sqlitegen.LiveVariantsByProductRow) pggen.LiveVariantsByProductRow {
				return pggen.LiveVariantsByProductRow(x)
			})
		})
	if err != nil {
		return err
	}

	for _, r := range rows {
		variant := models.ProductVariant{
			ID:             r.ID,
			ProductID:      product.ID,
			SKU:            r.Sku.String,
			PriceSurcharge: int(r.PriceSurcharge),
			Quantity:       int(r.Quantity),
			Active:         r.Active,
		}
		if err := json.Unmarshal([]byte(r.OptionValues), &variant.OptionValues); err != nil {
			return err
		}
		product.Variants = append(product.Variants, variant)
	}
	return nil
}
