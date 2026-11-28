//go:build sqlc

package queries

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/dure-one/dure-mycart/internal/database"
	"github.com/dure-one/dure-mycart/internal/models"
	pggen "github.com/dure-one/dure-mycart/internal/queries/sqlcgen/postgres"
	sqlitegen "github.com/dure-one/dure-mycart/internal/queries/sqlcgen/sqlite"
	"github.com/dure-one/dure-mycart/pkg/security"
)

// AddProduct inserts a new product into the database and returns the product with the created timestamp.
func (q *ProductQueries) AddProduct(ctx context.Context, product *models.Product) (*models.Product, error) {
	product.ID = security.RandomString()

	metadata, attributes, seo, err := q.marshalProductJSON(product)
	if err != nil {
		return nil, err
	}

	params := pggen.InsertProductParams{
		ID:          product.ID,
		Name:        product.Name,
		Amount:      int64(product.Amount),
		Slug:        product.Slug,
		Metadata:    string(metadata),
		Attribute:   string(attributes),
		Brief:       product.Brief,
		Description: product.Description,
		Digital:     digitalType(product.Digital.Type),
		Quantity:    int64(product.Quantity),
		Sku:         skuOrNull(product.SKU),
		Seo:         string(seo),
		Active:      product.Active,
	}
	created, err := pick(q.DB, q.DB,
		func(g *pggen.Queries) (int64, error) { return g.InsertProduct(ctx, params) },
		func(g *sqlitegen.Queries) (int64, error) {
			return g.InsertProduct(ctx, sqlitegen.InsertProductParams(params))
		})
	if err != nil {
		return nil, err
	}
	product.Created = created

	return product, nil
}

// skuOrNull keeps the unique index on sku usable: it applies to every value
// that is not NULL, so rows without a SKU have to store NULL rather than the
// empty string — otherwise the second one would collide with the first.
func skuOrNull(sku string) sql.NullString {
	return sql.NullString{String: sku, Valid: sku != ""}
}

// digitalType passes the delivery type through as the plain build does: an
// empty type is stored as ” rather than NULL.
func digitalType(t string) sql.NullString {
	return sql.NullString{String: t, Valid: true}
}

// UpdateProduct updates an existing product in the database with new values.
func (q *ProductQueries) UpdateProduct(ctx context.Context, product *models.Product) error {
	metadata, attributes, seo, err := q.marshalProductJSON(product)
	if err != nil {
		return err
	}

	tx, err := q.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	if err = q.updateProductMainFields(ctx, tx, product, metadata, attributes, seo); err != nil {
		return err
	}

	if err = q.syncProductVariants(ctx, tx, product); err != nil {
		return err
	}

	return tx.Commit()
}

// marshalProductJSON marshals product metadata, attributes, and SEO to JSON
func (q *ProductQueries) marshalProductJSON(product *models.Product) (metadata, attributes, seo []byte, err error) {
	metadata, err = json.Marshal(product.Metadata)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("marshal metadata: %w", err)
	}

	attributes, err = json.Marshal(product.Attributes)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("marshal attributes: %w", err)
	}

	seo, err = json.Marshal(product.Seo)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("marshal seo: %w", err)
	}

	return metadata, attributes, seo, nil
}

// updateProductMainFields executes the UPDATE statement for main product fields
func (q *ProductQueries) updateProductMainFields(ctx context.Context, tx *database.Tx, product *models.Product, metadata, attributes, seo []byte) error {
	params := pggen.UpdateProductFieldsParams{
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
		ID:          product.ID,
	}
	if err := run(q.DB, tx,
		func(g *pggen.Queries) error { return g.UpdateProductFields(ctx, params) },
		func(g *sqlitegen.Queries) error {
			return g.UpdateProductFields(ctx, sqlitegen.UpdateProductFieldsParams(params))
		}); err != nil {
		return fmt.Errorf("execute update: %w", err)
	}
	return nil
}

// deleteVariantTree removes every option (cascading to their values) and
// variant of a product.
func (q *ProductQueries) deleteVariantTree(ctx context.Context, tx *database.Tx, productID string, optionsMsg, variantsMsg string) error {
	if err := run(q.DB, tx,
		func(g *pggen.Queries) error { return g.DeleteProductOptions(ctx, productID) },
		func(g *sqlitegen.Queries) error { return g.DeleteProductOptions(ctx, productID) }); err != nil {
		return fmt.Errorf("%s: %w", optionsMsg, err)
	}
	if err := run(q.DB, tx,
		func(g *pggen.Queries) error { return g.DeleteProductVariants(ctx, productID) },
		func(g *sqlitegen.Queries) error { return g.DeleteProductVariants(ctx, productID) }); err != nil {
		return fmt.Errorf("%s: %w", variantsMsg, err)
	}
	return nil
}

// syncProductVariants handles all variant-related CRUD operations
func (q *ProductQueries) syncProductVariants(ctx context.Context, tx *database.Tx, product *models.Product) error {
	if !product.HasVariants {
		// If has_variants is false, clean up any existing variant data
		return q.deleteVariantTree(ctx, tx, product.ID, "cleanup options", "cleanup variants")
	}

	if err := q.deleteVariantTree(ctx, tx, product.ID, "delete options", "delete variants"); err != nil {
		return err
	}

	if err := q.replaceOptions(ctx, tx, product); err != nil {
		return err
	}

	return q.replaceVariants(ctx, tx, product)
}

// replaceOptions inserts the options of product and their values.
func (q *ProductQueries) replaceOptions(ctx context.Context, tx *database.Tx, product *models.Product) error {
	for _, option := range product.Options {
		optionID := security.RandomString()

		if err := q.insertOption(ctx, tx, pggen.InsertProductOptionParams{
			ID: optionID, ProductID: product.ID, Name: option.Name, Position: int64(option.Position),
		}); err != nil {
			return fmt.Errorf("insert option: %w", err)
		}

		for _, value := range option.Values {
			if err := q.insertOptionValue(ctx, tx, pggen.InsertOptionValueParams{
				ID: security.RandomString(), OptionID: optionID, Value: value.Value, Position: int64(value.Position),
			}); err != nil {
				return fmt.Errorf("insert option value: %w", err)
			}
		}
	}

	return nil
}

// replaceVariants inserts the variants of product.
func (q *ProductQueries) replaceVariants(ctx context.Context, tx *database.Tx, product *models.Product) error {
	for _, variant := range product.Variants {
		optionValuesJSON, err := json.Marshal(variant.OptionValues)
		if err != nil {
			return fmt.Errorf("marshal option values: %w", err)
		}

		if err := q.insertVariantRow(ctx, tx, pggen.InsertVariantParams{
			ID:             security.RandomString(),
			ProductID:      product.ID,
			Sku:            skuOrNull(variant.SKU),
			PriceSurcharge: int64(variant.PriceSurcharge),
			Quantity:       int64(variant.Quantity),
			OptionValues:   string(optionValuesJSON),
			Active:         variant.Active,
		}); err != nil {
			return fmt.Errorf("insert variant: %w", err)
		}
	}

	return nil
}
