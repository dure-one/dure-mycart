//go:build sqlc

package queries

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/dure-one/dure-mycart/internal/database"
	"github.com/dure-one/dure-mycart/internal/models"
	pggen "github.com/dure-one/dure-mycart/internal/queries/sqlcgen/postgres"
	sqlitegen "github.com/dure-one/dure-mycart/internal/queries/sqlcgen/sqlite"
)

// AddProductWithVariants adds a product with its options and variants in a transaction
func (q *ProductQueries) AddProductWithVariants(ctx context.Context, product *models.Product) (*models.Product, error) {
	tx, err := q.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if err = q.insertProductRow(ctx, tx, product); err != nil {
		return nil, err
	}
	if err = q.insertProductImages(ctx, tx, product.ID, product.Images); err != nil {
		return nil, err
	}
	if err = q.insertProductOptions(ctx, tx, product.ID, product.Options); err != nil {
		return nil, err
	}
	if err = q.insertProductVariants(ctx, tx, product); err != nil {
		return nil, err
	}

	if err = tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit transaction: %w", err)
	}
	return product, nil
}

// insertProductRow inserts the product row itself within a transaction
func (q *ProductQueries) insertProductRow(ctx context.Context, tx *database.Tx, product *models.Product) error {
	metadata, attributes, seo, err := q.marshalProductJSON(product)
	if err != nil {
		return err
	}

	params := pggen.InsertProductFullParams{
		ID:          product.ID,
		Name:        product.Name,
		Brief:       product.Brief,
		Description: product.Description,
		Slug:        product.Slug,
		Amount:      int64(product.Amount),
		Quantity:    int64(product.Quantity),
		Sku:         skuOrNull(product.SKU),
		HasVariants: product.HasVariants,
		Metadata:    string(metadata),
		Attribute:   string(attributes),
		Seo:         string(seo),
		Digital:     digitalType(product.Digital.Type),
		Active:      product.Active,
	}
	if err := run(q.DB, tx,
		func(g *pggen.Queries) error { return g.InsertProductFull(ctx, params) },
		func(g *sqlitegen.Queries) error {
			return g.InsertProductFull(ctx, sqlitegen.InsertProductFullParams(params))
		}); err != nil {
		return fmt.Errorf("insert product: %w", err)
	}
	return nil
}

// insertProductImages inserts all product images within a transaction
func (q *ProductQueries) insertProductImages(ctx context.Context, tx *database.Tx, productID string, images []models.File) error {
	for i, img := range images {
		params := pggen.InsertProductImageAtParams{
			ID: img.ID, ProductID: productID, Name: img.Name, Ext: img.Ext, OrigName: img.OrigName, Position: int64(i),
		}
		if err := run(q.DB, tx,
			func(g *pggen.Queries) error { return g.InsertProductImageAt(ctx, params) },
			func(g *sqlitegen.Queries) error {
				return g.InsertProductImageAt(ctx, sqlitegen.InsertProductImageAtParams(params))
			}); err != nil {
			return fmt.Errorf("insert image %d: %w", i, err)
		}
	}
	return nil
}

// insertOption inserts one option row.
func (q *ProductQueries) insertOption(ctx context.Context, tx *database.Tx, params pggen.InsertProductOptionParams) error {
	return run(q.DB, tx,
		func(g *pggen.Queries) error { return g.InsertProductOption(ctx, params) },
		func(g *sqlitegen.Queries) error {
			return g.InsertProductOption(ctx, sqlitegen.InsertProductOptionParams(params))
		})
}

// insertOptionValue inserts one option value row.
func (q *ProductQueries) insertOptionValue(ctx context.Context, tx *database.Tx, params pggen.InsertOptionValueParams) error {
	return run(q.DB, tx,
		func(g *pggen.Queries) error { return g.InsertOptionValue(ctx, params) },
		func(g *sqlitegen.Queries) error {
			return g.InsertOptionValue(ctx, sqlitegen.InsertOptionValueParams(params))
		})
}

// insertVariantRow inserts one variant row.
func (q *ProductQueries) insertVariantRow(ctx context.Context, tx *database.Tx, params pggen.InsertVariantParams) error {
	return run(q.DB, tx,
		func(g *pggen.Queries) error { return g.InsertVariant(ctx, params) },
		func(g *sqlitegen.Queries) error { return g.InsertVariant(ctx, sqlitegen.InsertVariantParams(params)) })
}

// insertProductOptions inserts options and their values within a transaction
func (q *ProductQueries) insertProductOptions(ctx context.Context, tx *database.Tx, productID string, options []models.ProductOption) error {
	for _, option := range options {
		if err := q.insertOption(ctx, tx, pggen.InsertProductOptionParams{
			ID: option.ID, ProductID: productID, Name: option.Name, Position: int64(option.Position),
		}); err != nil {
			return fmt.Errorf("insert option %s: %w", option.Name, err)
		}

		for _, value := range option.Values {
			if err := q.insertOptionValue(ctx, tx, pggen.InsertOptionValueParams{
				ID: value.ID, OptionID: option.ID, Value: value.Value, Position: int64(value.Position),
			}); err != nil {
				return fmt.Errorf("insert option value %s: %w", value.Value, err)
			}
		}
	}
	return nil
}

// insertProductVariants inserts variants with relationships and images within a transaction
func (q *ProductQueries) insertProductVariants(ctx context.Context, tx *database.Tx, product *models.Product) error {
	for _, variant := range product.Variants {
		optionValuesJSON, err := json.Marshal(variant.OptionValues)
		if err != nil {
			return fmt.Errorf("marshal option values: %w", err)
		}

		if err := q.insertVariantRow(ctx, tx, pggen.InsertVariantParams{
			ID:             variant.ID,
			ProductID:      product.ID,
			Sku:            skuOrNull(variant.SKU),
			PriceSurcharge: int64(variant.PriceSurcharge),
			Quantity:       int64(variant.Quantity),
			OptionValues:   string(optionValuesJSON),
			Active:         variant.Active,
		}); err != nil {
			return fmt.Errorf("insert variant %s: %w", variant.SKU, err)
		}

		if err = q.insertVariantOptions(ctx, tx, product.ID, variant); err != nil {
			return err
		}
		if err = q.insertVariantImages(ctx, tx, variant); err != nil {
			return err
		}
	}
	return nil
}

// insertVariantOptions links a variant to the option values it is made of
func (q *ProductQueries) insertVariantOptions(ctx context.Context, tx *database.Tx, productID string, variant models.ProductVariant) error {
	for optionName, optionValue := range variant.OptionValues {
		find := pggen.FindOptionValueIDParams{ProductID: productID, OptionName: optionName, OptionValue: optionValue}
		optionValueID, err := pick(q.DB, tx,
			func(g *pggen.Queries) (string, error) { return g.FindOptionValueID(ctx, find) },
			func(g *sqlitegen.Queries) (string, error) {
				return g.FindOptionValueID(ctx, sqlitegen.FindOptionValueIDParams(find))
			})
		if err != nil {
			return fmt.Errorf("find option value ID for %s=%s: %w", optionName, optionValue, err)
		}

		link := pggen.InsertVariantOptionParams{VariantID: variant.ID, OptionValueID: optionValueID}
		if err := run(q.DB, tx,
			func(g *pggen.Queries) error { return g.InsertVariantOption(ctx, link) },
			func(g *sqlitegen.Queries) error {
				return g.InsertVariantOption(ctx, sqlitegen.InsertVariantOptionParams(link))
			}); err != nil {
			return fmt.Errorf("insert variant-option relationship: %w", err)
		}
	}
	return nil
}

// insertVariantImages inserts the images of a variant
func (q *ProductQueries) insertVariantImages(ctx context.Context, tx *database.Tx, variant models.ProductVariant) error {
	for i, img := range variant.Images {
		params := pggen.InsertVariantImageParams{
			ID: img.ID, VariantID: variant.ID, Name: img.Name, Ext: img.Ext, OrigName: img.OrigName, Position: int64(i),
		}
		if err := run(q.DB, tx,
			func(g *pggen.Queries) error { return g.InsertVariantImage(ctx, params) },
			func(g *sqlitegen.Queries) error {
				return g.InsertVariantImage(ctx, sqlitegen.InsertVariantImageParams(params))
			}); err != nil {
			return fmt.Errorf("insert variant image %d: %w", i, err)
		}
	}
	return nil
}
