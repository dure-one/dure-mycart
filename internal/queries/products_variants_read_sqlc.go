//go:build sqlc

package queries

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/shurco/mycart/internal/models"
	pggen "github.com/dure-one/dure-mycart/internal/queries/sqlcgen/postgres"
	sqlitegen "github.com/dure-one/dure-mycart/internal/queries/sqlcgen/sqlite"
)

// GetProductWithVariants retrieves a product with all its options and variants
func (q *ProductQueries) GetProductWithVariants(ctx context.Context, productID string) (*models.Product, error) {
	product, err := q.loadProductBase(ctx, productID)
	if err != nil {
		return nil, err
	}

	if err = q.loadProductImages(ctx, product); err != nil {
		return nil, err
	}

	// Skip options/variants if product doesn't have variants
	if !product.HasVariants {
		return product, nil
	}

	return q.loadProductOptions(ctx, product)
}

// loadProductBase reads the product row and decodes its JSON columns
func (q *ProductQueries) loadProductBase(ctx context.Context, productID string) (*models.Product, error) {
	r, err := pick(q.DB, q.DB,
		func(g *pggen.Queries) (pggen.ProductBaseRow, error) { return g.ProductBase(ctx, productID) },
		func(g *sqlitegen.Queries) (pggen.ProductBaseRow, error) {
			r, err := g.ProductBase(ctx, productID)
			return pggen.ProductBaseRow(r), err
		})
	if err != nil {
		return nil, fmt.Errorf("query product: %w", err)
	}

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
	product.ID = r.ID

	if err = json.Unmarshal([]byte(r.Metadata), &product.Metadata); err != nil {
		return nil, fmt.Errorf("unmarshal metadata: %w", err)
	}
	if err = json.Unmarshal([]byte(r.Attribute), &product.Attributes); err != nil {
		return nil, fmt.Errorf("unmarshal attributes: %w", err)
	}
	if r.Digital.Valid {
		product.Digital.Type = r.Digital.String
	}

	return product, nil
}

// loadProductImages reads the images of the product in position order
func (q *ProductQueries) loadProductImages(ctx context.Context, product *models.Product) error {
	rows, err := pick(q.DB, q.DB,
		func(g *pggen.Queries) ([]pggen.ProductImagesOrderedRow, error) {
			return g.ProductImagesOrdered(ctx, product.ID)
		},
		func(g *sqlitegen.Queries) ([]pggen.ProductImagesOrderedRow, error) {
			r, err := g.ProductImagesOrdered(ctx, product.ID)
			return convertAll(r, err, func(x sqlitegen.ProductImagesOrderedRow) pggen.ProductImagesOrderedRow {
				return pggen.ProductImagesOrderedRow(x)
			})
		})
	if err != nil {
		return fmt.Errorf("query product images: %w", err)
	}

	for _, r := range rows {
		product.Images = append(product.Images, models.File{ID: r.ID, Name: r.Name, Ext: r.Ext, OrigName: r.OrigName})
	}
	return nil
}

// loadProductOptions loads options, option values, and variants for a product
func (q *ProductQueries) loadProductOptions(ctx context.Context, product *models.Product) (*models.Product, error) {
	options, err := q.productOptions(ctx, product.ID)
	if err != nil {
		return nil, fmt.Errorf("query options: %w", err)
	}
	product.Options = append(product.Options, options...)

	if err = q.loadOptionValues(ctx, &product.Options); err != nil {
		return nil, err
	}

	variants, err := q.loadProductVariants(ctx, product.ID)
	if err != nil {
		return nil, err
	}
	product.Variants = variants

	return product, nil
}

// loadOptionValues loads values for all product options
func (q *ProductQueries) loadOptionValues(ctx context.Context, options *[]models.ProductOption) error {
	for i := range *options {
		optionID := (*options)[i].ID
		rows, err := pick(q.DB, q.DB,
			func(g *pggen.Queries) ([]pggen.OptionValuesByOptionRow, error) {
				return g.OptionValuesByOption(ctx, optionID)
			},
			func(g *sqlitegen.Queries) ([]pggen.OptionValuesByOptionRow, error) {
				r, err := g.OptionValuesByOption(ctx, optionID)
				return convertAll(r, err, func(x sqlitegen.OptionValuesByOptionRow) pggen.OptionValuesByOptionRow {
					return pggen.OptionValuesByOptionRow(x)
				})
			})
		if err != nil {
			return fmt.Errorf("query option values: %w", err)
		}
		for _, r := range rows {
			(*options)[i].Values = append((*options)[i].Values,
				models.ProductOptionValue{ID: r.ID, OptionID: optionID, Value: r.Value, Position: int(r.Position)})
		}
	}
	return nil
}

// loadProductVariants loads all variants with their option values and images
func (q *ProductQueries) loadProductVariants(ctx context.Context, productID string) ([]models.ProductVariant, error) {
	rows, err := pick(q.DB, q.DB,
		func(g *pggen.Queries) ([]pggen.VariantsByProductRow, error) {
			return g.VariantsByProduct(ctx, productID)
		},
		func(g *sqlitegen.Queries) ([]pggen.VariantsByProductRow, error) {
			r, err := g.VariantsByProduct(ctx, productID)
			return convertAll(r, err, func(x sqlitegen.VariantsByProductRow) pggen.VariantsByProductRow {
				return pggen.VariantsByProductRow(x)
			})
		})
	if err != nil {
		return nil, fmt.Errorf("query variants: %w", err)
	}

	var variants []models.ProductVariant
	for _, r := range rows {
		variant := models.ProductVariant{
			ID:             r.ID,
			ProductID:      productID,
			SKU:            r.Sku.String,
			PriceSurcharge: int(r.PriceSurcharge),
			Quantity:       int(r.Quantity),
			Active:         r.Active,
			OptionValues:   make(map[string]string),
		}

		if err := q.loadVariantOptionValues(ctx, &variant); err != nil {
			return nil, err
		}
		if err := q.loadVariantImages(ctx, &variant); err != nil {
			return nil, err
		}

		variants = append(variants, variant)
	}

	return variants, nil
}

// loadVariantOptionValues reads the option name and value pairs a variant is made of
func (q *ProductQueries) loadVariantOptionValues(ctx context.Context, variant *models.ProductVariant) error {
	rows, err := pick(q.DB, q.DB,
		func(g *pggen.Queries) ([]pggen.VariantOptionPairsRow, error) {
			return g.VariantOptionPairs(ctx, variant.ID)
		},
		func(g *sqlitegen.Queries) ([]pggen.VariantOptionPairsRow, error) {
			r, err := g.VariantOptionPairs(ctx, variant.ID)
			return convertAll(r, err, func(x sqlitegen.VariantOptionPairsRow) pggen.VariantOptionPairsRow {
				return pggen.VariantOptionPairsRow(x)
			})
		})
	if err != nil {
		return fmt.Errorf("query variant option values: %w", err)
	}
	for _, r := range rows {
		variant.OptionValues[r.Name] = r.Value
	}
	return nil
}

// loadVariantImages reads the images of a variant in position order
func (q *ProductQueries) loadVariantImages(ctx context.Context, variant *models.ProductVariant) error {
	rows, err := pick(q.DB, q.DB,
		func(g *pggen.Queries) ([]pggen.VariantImagesRow, error) { return g.VariantImages(ctx, variant.ID) },
		func(g *sqlitegen.Queries) ([]pggen.VariantImagesRow, error) {
			r, err := g.VariantImages(ctx, variant.ID)
			return convertAll(r, err, func(x sqlitegen.VariantImagesRow) pggen.VariantImagesRow { return pggen.VariantImagesRow(x) })
		})
	if err != nil {
		return fmt.Errorf("query variant images: %w", err)
	}
	for _, r := range rows {
		variant.Images = append(variant.Images, models.File{ID: r.ID, Name: r.Name, Ext: r.Ext, OrigName: r.OrigName})
	}
	return nil
}
