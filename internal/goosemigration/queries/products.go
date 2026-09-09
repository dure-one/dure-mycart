package queries

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/shurco/mycart/internal/store/db/postgres"
	"github.com/shurco/mycart/internal/store/db/sqlite"
	"github.com/shurco/mycart/internal/models"
	"github.com/shurco/mycart/pkg/errors"
	"github.com/shurco/mycart/pkg/security"
	"github.com/shurco/mycart/pkg/slugify"
)

// ProductQueries is a struct that embeds a pointer to an sql.DB.
// This allows for direct access to the database methods via the ProductQueries struct,
// effectively extending it with all the functionality of *sql.DB.
type ProductQueries struct {
	*sql.DB
}

// ListProducts retrieves a list of products from the database using sqlc where possible.
// If cartID is provided, it will also include digital products that were purchased in that cart.
func (q *ProductQueries) ListProducts(ctx context.Context, private bool, limit, offset int, cartID string, idList ...models.CartProduct) (*models.Products, error) {
	currency, err := db.GetSettingByKey(ctx, "currency")
	if err != nil {
		return nil, err
	}

	products := &models.Products{
		Currency: currency["currency"].Value.(string),
	}

	// For simple cases (no cartID, no idList), use sqlc queries
	if cartID == "" && len(idList) == 0 {
		return q.listProductsSimple(ctx, products, private, int64(limit), int64(offset))
	}

	// For complex cases (with cartID or idList), use hand-written SQL with dynamic filtering
	return q.listProductsComplex(ctx, products, private, limit, offset, cartID, idList...)
}

// listProductsSimple handles simple product listing using sqlc queries
func (q *ProductQueries) listProductsSimple(ctx context.Context, products *models.Products, private bool, limit, offset int64) (*models.Products, error) {
	queries := getSQLCQueries()

	if private {
		var rows interface{}
		var err error

		if DBType() == "postgres" {
			pgQueries := queries.(*postgres.Queries)
			params := postgres.ListProductsPrivateParams{
				Limit:  int32(limit),
				Offset: int32(offset),
			}
			rows, err = pgQueries.ListProductsPrivate(ctx, params)
		} else {
			sqliteQueries := queries.(*sqlite.Queries)
			params := sqlite.ListProductsPrivateParams{
				Limit:  limit,
				Offset: offset,
			}
			rows, err = sqliteQueries.ListProductsPrivate(ctx, params)
		}

		if err != nil {
			return nil, err
		}

		if err := q.mapProductListRows(rows, products, private); err != nil {
			return nil, err
		}

		// Get total count
		if DBType() == "postgres" {
			pgQueries := queries.(*postgres.Queries)
			count, err := pgQueries.CountProducts(ctx)
			if err != nil {
				return nil, err
			}
			products.Total = int(count)
		} else {
			sqliteQueries := queries.(*sqlite.Queries)
			count, err := sqliteQueries.CountProducts(ctx)
			if err != nil {
				return nil, err
			}
			products.Total = int(count)
		}
	} else {
		var rows interface{}
		var err error

		if DBType() == "postgres" {
			pgQueries := queries.(*postgres.Queries)
			params := postgres.ListProductsPublicParams{
				Limit:  int32(limit),
				Offset: int32(offset),
			}
			rows, err = pgQueries.ListProductsPublic(ctx, params)
		} else {
			sqliteQueries := queries.(*sqlite.Queries)
			params := sqlite.ListProductsPublicParams{
				Limit:  limit,
				Offset: offset,
			}
			rows, err = sqliteQueries.ListProductsPublic(ctx, params)
		}

		if err != nil {
			return nil, err
		}

		if err := q.mapProductListRows(rows, products, private); err != nil {
			return nil, err
		}

		// Get total count (only active, non-deleted products for public)
		countQuery := `SELECT COUNT(*) FROM product WHERE deleted = 0 AND active = 1`
		err = q.DB.QueryRowContext(ctx, countQuery).Scan(&products.Total)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return nil, err
		}
	}

	return products, nil
}

// listProductsComplex handles complex product listing with cartID or idList filtering (hand-written SQL)
func (q *ProductQueries) listProductsComplex(ctx context.Context, products *models.Products, private bool, limit, offset int, cartID string, idList ...models.CartProduct) (*models.Products, error) {
	// Build variants subquery based on private mode
	variantsFilter := ""
	if !private {
		variantsFilter = " AND active = 1"
	}

	query := fmt.Sprintf(`
			SELECT DISTINCT
			  product.id,
				product.name,
				product.brief,
				product.slug,
				product.amount,
				product.quantity,
				product.has_variants,
				product.active,
				product.digital,
				EXISTS(SELECT 1 FROM digital_data WHERE digital_data.product_id = product.id AND digital_data.cart_id IS NULL) OR
				EXISTS(SELECT 1 FROM digital_file WHERE digital_file.product_id = product.id) AS digital_filled,
				(SELECT json_group_array(json_object('id', product_image.id, 'name', product_image.name, 'ext', product_image.ext)) as images FROM product_image WHERE product_id = product.id GROUP BY id LIMIT 1) as image,
				(SELECT json_group_array(json_object('id', product_variant.id, 'sku', product_variant.sku, 'quantity', product_variant.quantity, 'price_surcharge', product_variant.price_surcharge, 'option_values', json(product_variant.option_values), 'active', CASE WHEN product_variant.active = 1 THEN json('true') ELSE json('false') END)) FROM product_variant WHERE product_id = product.id%s) as variants,
				strftime('%%s', created)
			FROM product
		`, variantsFilter)

	var queryPublic string
	var params []any
	var countParams []any

	if !private {
		queryPublic, params = publicProductFilter(cartID)
		countParams = append(countParams, params...)
	} else {
		queryPublic = " WHERE product.deleted = 0 "
	}

	var queryAddon string

	if len(idList) > 0 {
		for _, item := range idList {
			params = append(params, item.ProductID)
			countParams = append(countParams, item.ProductID)
		}
		queryAddon = fmt.Sprintf("AND product.id IN (%s)", BuildPlaceholders(len(idList)))
	}

	query += queryPublic

	// Add pagination
	if limit > 0 {
		paramIndex := len(params) + 1
		query += " LIMIT " + BuildPlaceholder(paramIndex)
		params = append(params, limit)
		if offset > 0 {
			paramIndex++
			query += " OFFSET " + BuildPlaceholder(paramIndex)
			params = append(params, offset)
		}
	}

	rows, err := q.DB.QueryContext(ctx, query+queryAddon, params...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	for rows.Next() {
		var image, variants, digitalType sql.NullString
		var digitalFilled, hasVariants sql.NullBool
		product := models.Product{}
		err := rows.Scan(
			&product.ID,
			&product.Name,
			&product.Brief,
			&product.Slug,
			&product.Amount,
			&product.Quantity,
			&hasVariants,
			&product.Active,
			&digitalType,
			&digitalFilled,
			&image,
			&variants,
			&product.Created,
		)
		if err != nil {
			return nil, err
		}

		if image.Valid && image.String != `[{"id":null,"name":null,"ext":null}]` {
			if err := json.Unmarshal([]byte(image.String), &product.Images); err != nil {
				return nil, err
			}
		}

		if hasVariants.Valid {
			product.HasVariants = hasVariants.Bool
		}

		if variants.Valid && variants.String != `[{"id":null}]` && variants.String != "[]" {
			if err := json.Unmarshal([]byte(variants.String), &product.Variants); err != nil {
				return nil, err
			}
		}

		product.Digital.Type = digitalType.String
		if private && digitalType.Valid {
			if digitalFilled.Valid {
				product.Digital.Filled = digitalFilled.Bool
			} else {
				product.Digital.Filled = false
			}
		}

		products.Products = append(products.Products, product)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Count total records (without pagination params)
	countQuery := `SELECT COUNT(DISTINCT product.id) FROM product`
	countQuery += queryPublic
	err = q.DB.QueryRowContext(ctx, countQuery+queryAddon, countParams...).Scan(&products.Total)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}

	return products, nil
}

// mapProductListRows maps sqlc list query results to products
func (q *ProductQueries) mapProductListRows(rows interface{}, products *models.Products, private bool) error {
	switch v := rows.(type) {
	case []postgres.ListProductsPrivateRow:
		for _, row := range v {
			product := models.Product{}
			product.ID = row.ID
			product.Name = row.Name
			product.Brief = row.Brief
			product.Slug = row.Slug
			product.Active = row.Active

			if amount, err := strconv.Atoi(row.Amount); err == nil {
				product.Amount = amount
			}
			if row.Quantity.Valid {
				product.Quantity = int(row.Quantity.Int32)
			}
			if row.HasVariants.Valid {
				product.HasVariants = row.HasVariants.Bool
			}
			if row.Digital.Valid {
				product.Digital.Type = row.Digital.String
			}
			if row.DigitalFilled.Valid {
				product.Digital.Filled = row.DigitalFilled.Bool
			}

			// Parse image JSON
			imageStr := string(row.Image)
			if imageStr != "" && imageStr != `[{"id":null,"name":null,"ext":null}]` {
				if err := json.Unmarshal(row.Image, &product.Images); err != nil {
					return err
				}
			}

			// Parse variants JSON
			variantsStr := string(row.Variants)
			if variantsStr != "" && variantsStr != `[{"id":null}]` && variantsStr != "[]" {
				if err := json.Unmarshal(row.Variants, &product.Variants); err != nil {
					return err
				}
			}

			product.Created = row.Created

			products.Products = append(products.Products, product)
		}

	case []sqlite.ListProductsPrivateRow:
		for _, row := range v {
			product := models.Product{}
			product.ID = row.ID
			product.Name = row.Name
			product.Brief = row.Brief
			product.Slug = row.Slug
			product.Active = row.Active

			if amount, ok := row.Amount.(int64); ok {
				product.Amount = int(amount)
			} else if amount, ok := row.Amount.(float64); ok {
				product.Amount = int(amount)
			}
			if row.Quantity.Valid {
				product.Quantity = int(row.Quantity.Int64)
			}
			if row.HasVariants.Valid {
				product.HasVariants = row.HasVariants.Bool
			}
			if row.Digital.Valid {
				product.Digital.Type = row.Digital.String
			}
			if row.DigitalFilled.Valid {
				product.Digital.Filled = row.DigitalFilled.Bool
			}

			// Parse image JSON (interface{} in sqlite)
			if imageStr, ok := row.Image.(string); ok {
				if imageStr != "" && imageStr != `[{"id":null,"name":null,"ext":null}]` {
					if err := json.Unmarshal([]byte(imageStr), &product.Images); err != nil {
						return err
					}
				}
			}

			// Parse variants JSON (interface{} in sqlite)
			if variantsStr, ok := row.Variants.(string); ok {
				if variantsStr != "" && variantsStr != `[{"id":null}]` && variantsStr != "[]" {
					if err := json.Unmarshal([]byte(variantsStr), &product.Variants); err != nil {
						return err
					}
				}
			}

			if created, ok := row.Created.(int64); ok {
				product.Created = created
			}

			products.Products = append(products.Products, product)
		}

	case []postgres.ListProductsPublicRow:
		for _, row := range v {
			product := models.Product{}
			product.ID = row.ID
			product.Name = row.Name
			product.Brief = row.Brief
			product.Slug = row.Slug
			product.Active = row.Active

			if amount, err := strconv.Atoi(row.Amount); err == nil {
				product.Amount = amount
			}
			if row.Quantity.Valid {
				product.Quantity = int(row.Quantity.Int32)
			}
			if row.HasVariants.Valid {
				product.HasVariants = row.HasVariants.Bool
			}
			if row.Digital.Valid {
				product.Digital.Type = row.Digital.String
			}
			if row.DigitalFilled.Valid {
				product.Digital.Filled = row.DigitalFilled.Bool
			}

			imageStr := string(row.Image)
			if imageStr != "" && imageStr != `[{"id":null,"name":null,"ext":null}]` {
				if err := json.Unmarshal(row.Image, &product.Images); err != nil {
					return err
				}
			}

			variantsStr := string(row.Variants)
			if variantsStr != "" && variantsStr != `[{"id":null}]` && variantsStr != "[]" {
				if err := json.Unmarshal(row.Variants, &product.Variants); err != nil {
					return err
				}
			}

			product.Created = row.Created

			products.Products = append(products.Products, product)
		}

	case []sqlite.ListProductsPublicRow:
		for _, row := range v {
			product := models.Product{}
			product.ID = row.ID
			product.Name = row.Name
			product.Brief = row.Brief
			product.Slug = row.Slug
			product.Active = row.Active

			if amount, ok := row.Amount.(int64); ok {
				product.Amount = int(amount)
			} else if amount, ok := row.Amount.(float64); ok {
				product.Amount = int(amount)
			}
			if row.Quantity.Valid {
				product.Quantity = int(row.Quantity.Int64)
			}
			if row.HasVariants.Valid {
				product.HasVariants = row.HasVariants.Bool
			}
			if row.Digital.Valid {
				product.Digital.Type = row.Digital.String
			}
			if row.DigitalFilled.Valid {
				product.Digital.Filled = row.DigitalFilled.Bool
			}

			if imageStr, ok := row.Image.(string); ok {
				if imageStr != "" && imageStr != `[{"id":null,"name":null,"ext":null}]` {
					if err := json.Unmarshal([]byte(imageStr), &product.Images); err != nil {
						return err
					}
				}
			}

			if variantsStr, ok := row.Variants.(string); ok {
				if variantsStr != "" && variantsStr != `[{"id":null}]` && variantsStr != "[]" {
					if err := json.Unmarshal([]byte(variantsStr), &product.Variants); err != nil {
						return err
					}
				}
			}

			if created, ok := row.Created.(int64); ok {
				product.Created = created
			}

			products.Products = append(products.Products, product)
		}
	}

	return nil
}

// Product retrieves a product by its ID, with the option to fetch private or public data using sqlc.
func (q *ProductQueries) Product(ctx context.Context, private bool, id string) (*models.Product, error) {
	queries := getSQLCQueries()
	product := &models.Product{}

	// Use sqlc query for base product data
	if private {
		// Private mode: fetch by ID
		var row interface{}
		var err error

		if DBType() == "postgres" {
			pgQueries := queries.(*postgres.Queries)
			row, err = pgQueries.GetProductDetailByID(ctx, id)
		} else {
			sqliteQueries := queries.(*sqlite.Queries)
			row, err = sqliteQueries.GetProductDetailByID(ctx, id)
		}

		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, errors.ErrProductNotFound
			}
			return nil, err
		}

		// Map sqlc result to product model
		if err := q.mapProductDetail(row, product, private); err != nil {
			return nil, err
		}
	} else {
		// Public mode: fetch by slug
		var row interface{}
		var err error

		if DBType() == "postgres" {
			pgQueries := queries.(*postgres.Queries)
			row, err = pgQueries.GetProductDetailBySlug(ctx, id)
		} else {
			sqliteQueries := queries.(*sqlite.Queries)
			row, err = sqliteQueries.GetProductDetailBySlug(ctx, id)
		}

		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, errors.ErrProductNotFound
			}
			return nil, err
		}

		// Map sqlc result to product model
		if err := q.mapProductDetail(row, product, private); err != nil {
			return nil, err
		}
	}

	// Load options and variants if product has variants (keep existing logic)
	return q.loadProductOptionsAndVariants(ctx, product)
}

// mapProductDetail maps sqlc query result to models.Product
func (q *ProductQueries) mapProductDetail(row interface{}, product *models.Product, private bool) error {
	switch v := row.(type) {
	case postgres.GetProductDetailByIDRow:
		product.ID = v.ID
		product.Name = v.Name
		product.Brief = v.Brief
		product.Description = v.Desc
		product.Slug = v.Slug
		// Amount is string in postgres (numeric type)
		if amount, err := strconv.Atoi(v.Amount); err == nil {
			product.Amount = amount
		}
		if v.Quantity.Valid {
			product.Quantity = int(v.Quantity.Int32)
		}
		if v.Sku.Valid {
			product.SKU = v.Sku.String
		}
		if v.HasVariants.Valid {
			product.HasVariants = v.HasVariants.Bool
		}
		product.Active = v.Active

		if err := json.Unmarshal(v.Metadata, &product.Metadata); err != nil {
			return err
		}
		if err := json.Unmarshal(v.Attribute, &product.Attributes); err != nil {
			return err
		}
		if v.Digital.Valid {
			product.Digital.Type = v.Digital.String
		}
		if err := json.Unmarshal(v.Seo, &product.Seo); err != nil {
			return err
		}
		// Images is json.RawMessage in postgres
		imagesStr := string(v.Images)
		if imagesStr != "" && imagesStr != `[{"id":null,"name":null,"ext":null}]` {
			if err := json.Unmarshal(v.Images, &product.Images); err != nil {
				return err
			}
		}
		if private && v.DigitalFilled.Valid {
			product.Digital.Filled = v.DigitalFilled.Bool
		}
		product.Created = v.Created
		product.Updated = v.Updated

	case sqlite.GetProductDetailByIDRow:
		product.ID = v.ID
		product.Name = v.Name
		product.Brief = v.Brief
		product.Description = v.Desc
		product.Slug = v.Slug
		// Amount is interface{} in sqlite
		if amount, ok := v.Amount.(int64); ok {
			product.Amount = int(amount)
		} else if amount, ok := v.Amount.(float64); ok {
			product.Amount = int(amount)
		}
		if v.Quantity.Valid {
			product.Quantity = int(v.Quantity.Int64)
		}
		if v.Sku.Valid {
			product.SKU = v.Sku.String
		}
		if v.HasVariants.Valid {
			product.HasVariants = v.HasVariants.Bool
		}
		product.Active = v.Active

		if err := json.Unmarshal(v.Metadata, &product.Metadata); err != nil {
			return err
		}
		if err := json.Unmarshal(v.Attribute, &product.Attributes); err != nil {
			return err
		}
		if v.Digital.Valid {
			product.Digital.Type = v.Digital.String
		}
		if err := json.Unmarshal(v.Seo, &product.Seo); err != nil {
			return err
		}
		// Images is interface{} in sqlite
		if imagesStr, ok := v.Images.(string); ok {
			if imagesStr != "" && imagesStr != `[{"id":null,"name":null,"ext":null}]` {
				if err := json.Unmarshal([]byte(imagesStr), &product.Images); err != nil {
					return err
				}
			}
		}
		if private && v.DigitalFilled.Valid {
			product.Digital.Filled = v.DigitalFilled.Bool
		}
		// Created and Updated are interface{} in sqlite
		if created, ok := v.Created.(int64); ok {
			product.Created = created
		}
		if updated, ok := v.Updated.(int64); ok {
			product.Updated = updated
		}

	case postgres.GetProductDetailBySlugRow:
		product.ID = v.ID
		product.Name = v.Name
		product.Brief = v.Brief
		product.Description = v.Desc
		product.Slug = v.Slug
		// Amount is string in postgres
		if amount, err := strconv.Atoi(v.Amount); err == nil {
			product.Amount = amount
		}
		if v.Quantity.Valid {
			product.Quantity = int(v.Quantity.Int32)
		}
		if v.Sku.Valid {
			product.SKU = v.Sku.String
		}
		if v.HasVariants.Valid {
			product.HasVariants = v.HasVariants.Bool
		}
		product.Active = v.Active

		if err := json.Unmarshal(v.Metadata, &product.Metadata); err != nil {
			return err
		}
		if err := json.Unmarshal(v.Attribute, &product.Attributes); err != nil {
			return err
		}
		if v.Digital.Valid {
			product.Digital.Type = v.Digital.String
		}
		if err := json.Unmarshal(v.Seo, &product.Seo); err != nil {
			return err
		}
		// Images is json.RawMessage in postgres
		imagesStr := string(v.Images)
		if imagesStr != "" && imagesStr != `[{"id":null,"name":null,"ext":null}]` {
			if err := json.Unmarshal(v.Images, &product.Images); err != nil {
				return err
			}
		}
		product.Created = v.Created
		product.Updated = v.Updated

	case sqlite.GetProductDetailBySlugRow:
		product.ID = v.ID
		product.Name = v.Name
		product.Brief = v.Brief
		product.Description = v.Desc
		product.Slug = v.Slug
		// Amount is interface{} in sqlite
		if amount, ok := v.Amount.(int64); ok {
			product.Amount = int(amount)
		} else if amount, ok := v.Amount.(float64); ok {
			product.Amount = int(amount)
		}
		if v.Quantity.Valid {
			product.Quantity = int(v.Quantity.Int64)
		}
		if v.Sku.Valid {
			product.SKU = v.Sku.String
		}
		if v.HasVariants.Valid {
			product.HasVariants = v.HasVariants.Bool
		}
		product.Active = v.Active

		if err := json.Unmarshal(v.Metadata, &product.Metadata); err != nil {
			return err
		}
		if err := json.Unmarshal(v.Attribute, &product.Attributes); err != nil {
			return err
		}
		if v.Digital.Valid {
			product.Digital.Type = v.Digital.String
		}
		if err := json.Unmarshal(v.Seo, &product.Seo); err != nil {
			return err
		}
		// Images is interface{} in sqlite
		if imagesStr, ok := v.Images.(string); ok {
			if imagesStr != "" && imagesStr != `[{"id":null,"name":null,"ext":null}]` {
				if err := json.Unmarshal([]byte(imagesStr), &product.Images); err != nil {
					return err
				}
			}
		}
		// Created and Updated are interface{} in sqlite
		if created, ok := v.Created.(int64); ok {
			product.Created = created
		}
		if updated, ok := v.Updated.(int64); ok {
			product.Updated = updated
		}
	}

	return nil
}

// loadProductOptionsAndVariants loads options and variants for a product
func (q *ProductQueries) loadProductOptionsAndVariants(ctx context.Context, product *models.Product) (*models.Product, error) {
	if !product.HasVariants {
		return product, nil
	}

	// Load all options first
	if product.HasVariants {
		// Load all options first
		optionsQuery := `
			SELECT id, name, position
			FROM product_option
			WHERE product_id = ` + BuildPlaceholder(1) + `
			ORDER BY position`

		optionRows, err := q.DB.QueryContext(ctx, optionsQuery, product.ID)
		if err != nil {
			return nil, err
		}

		optionIDs := []string{}
		optionMap := make(map[string]*models.ProductOption)

		for optionRows.Next() {
			option := models.ProductOption{ProductID: product.ID}
			if err := optionRows.Scan(&option.ID, &option.Name, &option.Position); err != nil {
				optionRows.Close()
				return nil, err
			}
			optionIDs = append(optionIDs, option.ID)
			optionMap[option.ID] = &option
			product.Options = append(product.Options, option)
		}
		optionRows.Close()

		// Load all option values
		if len(optionIDs) > 0 {
			valuesQuery := `
				SELECT id, option_id, value, position
				FROM product_option_value
				WHERE option_id IN (SELECT id FROM product_option WHERE product_id = ` + BuildPlaceholder(1) + `)
				ORDER BY option_id, position`

			valuesRows, err := q.DB.QueryContext(ctx, valuesQuery, product.ID)
			if err != nil {
				return nil, err
			}

			for valuesRows.Next() {
				value := models.ProductOptionValue{}
				if err := valuesRows.Scan(&value.ID, &value.OptionID, &value.Value, &value.Position); err != nil {
					valuesRows.Close()
					return nil, err
				}

				// Find the option and append the value
				for i := range product.Options {
					if product.Options[i].ID == value.OptionID {
						product.Options[i].Values = append(product.Options[i].Values, value)
						break
					}
				}
			}
			valuesRows.Close()
		}

		// Load variants
		variantsQuery := `
			SELECT id, sku, price_surcharge, quantity, option_values, active
			FROM product_variant
			WHERE product_id = ` + BuildPlaceholder(1) + ` AND deleted = 0`

		variantRows, err := q.DB.QueryContext(ctx, variantsQuery, product.ID)
		if err != nil {
			return nil, err
		}

		for variantRows.Next() {
			variant := models.ProductVariant{ProductID: product.ID}
			var optionValues string
			var sku sql.NullString

			if err := variantRows.Scan(&variant.ID, &sku, &variant.PriceSurcharge, &variant.Quantity, &optionValues, &variant.Active); err != nil {
				variantRows.Close()
				return nil, err
			}

			if sku.Valid {
				variant.SKU = sku.String
			}

			if err := json.Unmarshal([]byte(optionValues), &variant.OptionValues); err != nil {
				variantRows.Close()
				return nil, err
			}

			product.Variants = append(product.Variants, variant)
		}
		variantRows.Close()
	}

	return product, nil
}

// AddProduct inserts a new product into the database and returns the product with the created timestamp using sqlc.
func (q *ProductQueries) AddProduct(ctx context.Context, product *models.Product) (*models.Product, error) {
	queries := getSQLCQueries()
	product.ID = security.RandomString()

	metadata, err := json.Marshal(product.Metadata)
	if err != nil {
		return nil, err
	}

	attributes, err := json.Marshal(product.Attributes)
	if err != nil {
		return nil, err
	}

	var digitalType sql.NullString
	if product.Digital.Type != "" {
		digitalType = sql.NullString{String: product.Digital.Type, Valid: true}
	}

	if DBType() == "postgres" {
		pgQueries := queries.(*postgres.Queries)
		params := postgres.CreateProductParams{
			ID:        product.ID,
			Name:      product.Name,
			Brief:     product.Brief,
			Desc:      product.Description,
			Slug:      product.Slug,
			Amount:    strconv.Itoa(product.Amount),
			Metadata:  metadata,
			Attribute: attributes,
			Digital:   digitalType,
			Active:    false,
		}
		result, err := pgQueries.CreateProduct(ctx, params)
		if err != nil {
			return nil, err
		}
		if result.Created.Valid {
			product.Created = result.Created.Time.Unix()
		}
	} else {
		sqliteQueries := queries.(*sqlite.Queries)
		params := sqlite.CreateProductParams{
			ID:        product.ID,
			Name:      product.Name,
			Brief:     product.Brief,
			Desc:      product.Description,
			Slug:      product.Slug,
			Amount:    product.Amount,
			Metadata:  metadata,
			Attribute: attributes,
			Digital:   digitalType,
			Active:    false,
		}
		result, err := sqliteQueries.CreateProduct(ctx, params)
		if err != nil {
			return nil, err
		}
		if result.Created.Valid {
			product.Created = result.Created.Time.Unix()
		}
	}

	return product, nil
}

// UpdateProduct updates an existing product in the database with new values.
func (q *ProductQueries) UpdateProduct(ctx context.Context, product *models.Product) error {
	// Marshal JSON fields
	metadata, attributes, seo, err := q.marshalProductJSON(product)
	if err != nil {
		return err
	}

	// Start transaction for atomic updates
	tx, err := q.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	// Update main product fields
	if err = q.updateProductMainFields(ctx, tx, product, metadata, attributes, seo); err != nil {
		return err
	}

	// Sync variant data
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

// updateProductMainFields executes the UPDATE statement for main product fields using sqlc
func (q *ProductQueries) updateProductMainFields(ctx context.Context, tx *sql.Tx, product *models.Product, metadata, attributes, seo []byte) error {
	// Handle empty SKU as NULL
	var productSKU sql.NullString
	if product.SKU != "" {
		productSKU = sql.NullString{String: product.SKU, Valid: true}
	}

	// Handle has_variants as NULL if false
	var hasVariants sql.NullBool
	if product.HasVariants {
		hasVariants = sql.NullBool{Bool: product.HasVariants, Valid: true}
	}

	if DBType() == "postgres" {
		txQueries := postgres.New(tx)

		// Postgres uses NullInt32 for quantity
		var quantityInt32 sql.NullInt32
		if product.Quantity > 0 {
			quantityInt32 = sql.NullInt32{Int32: int32(product.Quantity), Valid: true}
		}

		params := postgres.UpdateProductFullParams{
			Name:        product.Name,
			Brief:       product.Brief,
			Desc:        product.Description,
			Slug:        product.Slug,
			Amount:      strconv.Itoa(product.Amount),
			Quantity:    quantityInt32,
			Sku:         productSKU,
			HasVariants: hasVariants,
			Metadata:    metadata,
			Attribute:   attributes,
			Seo:         seo,
			ID:          product.ID,
		}
		return txQueries.UpdateProductFull(ctx, params)
	}

	txQueries := sqlite.New(tx)

	// SQLite uses NullInt64 for quantity
	var quantityInt64 sql.NullInt64
	if product.Quantity > 0 {
		quantityInt64 = sql.NullInt64{Int64: int64(product.Quantity), Valid: true}
	}

	params := sqlite.UpdateProductFullParams{
		Name:        product.Name,
		Brief:       product.Brief,
		Desc:        product.Description,
		Slug:        product.Slug,
		Amount:      float64(product.Amount),
		Quantity:    quantityInt64,
		Sku:         productSKU,
		HasVariants: hasVariants,
		Metadata:    metadata,
		Attribute:   attributes,
		Seo:         seo,
		ID:          product.ID,
	}
	return txQueries.UpdateProductFull(ctx, params)
}

// syncProductVariants handles all variant-related CRUD operations
func (q *ProductQueries) syncProductVariants(ctx context.Context, tx *sql.Tx, product *models.Product) error {
	// Build database-specific queries
	var (
		deleteOptionQuery  string
		deleteVariantQuery string
		insertOptionQuery  string
		insertValueQuery   string
		insertVariantQuery string
	)

	if DBType() == "postgres" {
		deleteOptionQuery = `DELETE FROM product_option WHERE product_id = $1`
		deleteVariantQuery = `DELETE FROM product_variant WHERE product_id = $1`
		insertOptionQuery = `INSERT INTO product_option (id, product_id, name, position) VALUES ($1, $2, $3, $4)`
		insertValueQuery = `INSERT INTO product_option_value (id, option_id, value, position) VALUES ($1, $2, $3, $4)`
		insertVariantQuery = `INSERT INTO product_variant (id, product_id, sku, price_surcharge, quantity, option_values, active) VALUES ($1, $2, $3, $4, $5, $6, $7)`
	} else {
		deleteOptionQuery = `DELETE FROM product_option WHERE product_id = ?`
		deleteVariantQuery = `DELETE FROM product_variant WHERE product_id = ?`
		insertOptionQuery = `INSERT INTO product_option (id, product_id, name, position) VALUES (?, ?, ?, ?)`
		insertValueQuery = `INSERT INTO product_option_value (id, option_id, value, position) VALUES (?, ?, ?, ?)`
		insertVariantQuery = `INSERT INTO product_variant (id, product_id, sku, price_surcharge, quantity, option_values, active) VALUES (?, ?, ?, ?, ?, ?, ?)`
	}

	if product.HasVariants {
		// Delete existing options (cascades to option values)
		_, err := tx.ExecContext(ctx, deleteOptionQuery, product.ID)
		if err != nil {
			return fmt.Errorf("delete options: %w", err)
		}

		// Delete existing variants
		_, err = tx.ExecContext(ctx, deleteVariantQuery, product.ID)
		if err != nil {
			return fmt.Errorf("delete variants: %w", err)
		}

		// Insert new options and their values
		for _, option := range product.Options {
			optionID := security.RandomString()

			_, err = tx.ExecContext(ctx, insertOptionQuery,
				optionID, product.ID, option.Name, option.Position,
			)
			if err != nil {
				return fmt.Errorf("insert option: %w", err)
			}

			// Insert option values
			for _, value := range option.Values {
				valueID := security.RandomString()
				_, err = tx.ExecContext(ctx, insertValueQuery,
					valueID, optionID, value.Value, value.Position,
				)
				if err != nil {
					return fmt.Errorf("insert option value: %w", err)
				}
			}
		}

		// Insert new variants
		for _, variant := range product.Variants {
			variantID := security.RandomString()

			// Marshal option_values map to JSON
			optionValuesJSON, err := json.Marshal(variant.OptionValues)
			if err != nil {
				return fmt.Errorf("marshal option values: %w", err)
			}

			// Handle empty SKU as NULL to avoid unique constraint violations
			var skuValue sql.NullString
			if variant.SKU != "" {
				skuValue = sql.NullString{String: variant.SKU, Valid: true}
			}

			_, err = tx.ExecContext(ctx, insertVariantQuery,
				variantID, product.ID, skuValue, variant.PriceSurcharge, variant.Quantity, string(optionValuesJSON), variant.Active,
			)
			if err != nil {
				return fmt.Errorf("insert variant: %w", err)
			}
		}
	} else {
		// If has_variants is false, clean up any existing variant data
		_, err := tx.ExecContext(ctx, deleteOptionQuery, product.ID)
		if err != nil {
			return fmt.Errorf("cleanup options: %w", err)
		}
		_, err = tx.ExecContext(ctx, deleteVariantQuery, product.ID)
		if err != nil {
			return fmt.Errorf("cleanup variants: %w", err)
		}
	}

	return nil
}

// DeleteProduct removes a product from the database based on its ID.
// DeleteProduct deletes a product from the database using sqlc.
func (q *ProductQueries) DeleteProduct(ctx context.Context, id string) error {
	queries := getSQLCQueries()

	if DBType() == "postgres" {
		pgQueries := queries.(*postgres.Queries)
		return pgQueries.DeleteProduct(ctx, id)
	}

	sqliteQueries := queries.(*sqlite.Queries)
	return sqliteQueries.DeleteProduct(ctx, id)
}

// publicProductFilter builds the WHERE clause for storefront product queries.
// When cartID is set, purchased items remain visible even if the product was deactivated.
func publicProductFilter(cartID string) (string, []any) {
	if cartID != "" {
		return `
			WHERE product.deleted = 0 AND (
				product.active = 1
				OR EXISTS (
					SELECT 1 FROM digital_data
					WHERE digital_data.product_id = product.id
					AND digital_data.cart_id = ?
				)
			)
		`, []any{cartID}
	}
	return ` WHERE product.deleted = 0 AND product.active = 1 `, nil
}

// IsProduct checks if a product with the given slug exists and is active using sqlc.
func (q *ProductQueries) IsProduct(ctx context.Context, slug string) bool {
	queries := getSQLCQueries()

	var exists bool
	var err error

	if DBType() == "postgres" {
		pgQueries := queries.(*postgres.Queries)
		exists, err = pgQueries.ProductExists(ctx, slug)
	} else {
		sqliteQueries := queries.(*sqlite.Queries)
		exists, err = sqliteQueries.ProductExists(ctx, slug)
	}

	return err == nil && exists
}

// UpdateActive toggles the 'active' status of a product and updates its 'updated' timestamp using sqlc.
func (q *ProductQueries) UpdateActive(ctx context.Context, id string) error {
	queries := getSQLCQueries()

	if DBType() == "postgres" {
		pgQueries := queries.(*postgres.Queries)
		return pgQueries.UpdateProductActive(ctx, id)
	}

	sqliteQueries := queries.(*sqlite.Queries)
	return sqliteQueries.UpdateProductActive(ctx, id)
}

// productImage represents the database schema for product images.
// It serves as a data transfer object between the database and domain layer,
// keeping database concerns separate from the domain model.
type productImage struct {
	ID   string
	Name string
	Ext  string
}

// ProductImages orchestrates overall image retrieval process
func (q *ProductQueries) ProductImages(ctx context.Context, id string) (*[]models.File, error) {
	images, err := q.fetchProductImages(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("fetching product images: %w", err)
	}
	return q.mapToModelFiles(images), nil
}

// fetchProductImages performs database query to get image(s) for a product using sqlc
func (q *ProductQueries) fetchProductImages(ctx context.Context, id string) ([]productImage, error) {
	queries := getSQLCQueries()

	var results interface{}
	var err error

	if DBType() == "postgres" {
		pgQueries := queries.(*postgres.Queries)
		results, err = pgQueries.ListProductImages(ctx, id)
	} else {
		sqliteQueries := queries.(*sqlite.Queries)
		results, err = sqliteQueries.ListProductImages(ctx, id)
	}

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, errors.ErrProductNotFound
		}
		return nil, err
	}

	// Convert sqlc results to productImage slice
	var images []productImage
	switch v := results.(type) {
	case []postgres.ProductImage:
		for _, img := range v {
			images = append(images, productImage{
				ID:   img.ID,
				Name: img.Name,
				Ext:  img.Ext,
			})
		}
	case []sqlite.ProductImage:
		for _, img := range v {
			images = append(images, productImage{
				ID:   img.ID,
				Name: img.Name,
				Ext:  img.Ext,
			})
		}
	}

	return images, nil
}

// mapToModelFiles creates a representation of the database results as a domain object
func (q *ProductQueries) mapToModelFiles(images []productImage) *[]models.File {
	result := make([]models.File, len(images))
	for i, img := range images {
		result[i] = models.File{
			ID:   img.ID,
			Name: img.Name,
			Ext:  img.Ext,
		}
	}
	return &result
}

// AddImage attaches an image to a product by inserting a new record in the product_image table using sqlc.
func (q *ProductQueries) AddImage(ctx context.Context, productID, fileUUID, fileExt, origName string) (*models.File, error) {
	queries := getSQLCQueries()
	imageID := security.RandomString()

	if DBType() == "postgres" {
		pgQueries := queries.(*postgres.Queries)
		params := postgres.CreateProductImageParams{
			ID:        imageID,
			ProductID: productID,
			Name:      fileUUID,
			Ext:       fileExt,
			OrigName:  origName,
		}
		_, err := pgQueries.CreateProductImage(ctx, params)
		if err != nil {
			return nil, err
		}
	} else {
		sqliteQueries := queries.(*sqlite.Queries)
		params := sqlite.CreateProductImageParams{
			ID:        imageID,
			ProductID: productID,
			Name:      fileUUID,
			Ext:       fileExt,
			OrigName:  origName,
		}
		_, err := sqliteQueries.CreateProductImage(ctx, params)
		if err != nil {
			return nil, err
		}
	}

	// Convert result to models.File
	file := &models.File{
		ID:   imageID,
		Name: fileUUID,
		Ext:  fileExt,
	}

	return file, nil
}

// DeleteImage handles the deletion of a product image.
func (q *ProductQueries) DeleteImage(ctx context.Context, productID, imageID string) error {
	// Get image info and delete from database
	imageInfo, err := q.deleteImageRecord(ctx, productID, imageID)
	if err != nil {
		return fmt.Errorf("deleting image record: %w", err)
	}

	// Delete physical files
	if err := q.deleteImageFiles(imageInfo.name, imageInfo.ext); err != nil {
		return fmt.Errorf("deleting image files: %w", err)
	}

	return nil
}

type imageInfo struct {
	name string
	ext  string
}

func (q *ProductQueries) deleteImageRecord(ctx context.Context, productID, imageID string) (*imageInfo, error) {
	tx, err := q.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	// Get image info before deletion using sqlc
	info := &imageInfo{}
	var image interface{}

	if DBType() == "postgres" {
		txQueries := postgres.New(tx)
		image, err = txQueries.GetProductImage(ctx, imageID)
		if err != nil {
			return nil, err
		}
		pgImage := image.(postgres.ProductImage)
		if pgImage.ProductID != productID {
			return nil, sql.ErrNoRows
		}
		info.name = pgImage.Name
		info.ext = pgImage.Ext

		// Delete the database record
		if err := txQueries.DeleteProductImage(ctx, imageID); err != nil {
			return nil, err
		}
	} else {
		txQueries := sqlite.New(tx)
		image, err = txQueries.GetProductImage(ctx, imageID)
		if err != nil {
			return nil, err
		}
		sqliteImage := image.(sqlite.ProductImage)
		if sqliteImage.ProductID != productID {
			return nil, sql.ErrNoRows
		}
		info.name = sqliteImage.Name
		info.ext = sqliteImage.Ext

		// Delete the database record
		if err := txQueries.DeleteProductImage(ctx, imageID); err != nil {
			return nil, err
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	return info, nil
}

func (q *ProductQueries) deleteImageFiles(name, ext string) error {
	filePaths := []string{
		fmt.Sprintf("./lc_uploads/%s.%s", name, ext),
		fmt.Sprintf("./lc_uploads/%s_sm.%s", name, ext),
	}

	var removeErrors []string
	for _, filePath := range filePaths {
		if err := os.Remove(filePath); err != nil && !os.IsNotExist(err) {
			removeErrors = append(removeErrors,
				fmt.Sprintf("failed to remove %s: %v", filePath, err))
		}
	}

	if len(removeErrors) > 0 {
		return fmt.Errorf("file deletion errors: %s",
			strings.Join(removeErrors, "; "))
	}

	return nil
}

// ProductDigital retrieves the digital content associated with a given product ID using sqlc.
func (q *ProductQueries) ProductDigital(ctx context.Context, productID string) (*models.Digital, error) {
	queries := getSQLCQueries()
	digital := &models.Digital{}

	var rows interface{}
	var err error

	if DBType() == "postgres" {
		pgQueries := queries.(*postgres.Queries)
		rows, err = pgQueries.GetProductDigitalContent(ctx, productID)
	} else {
		sqliteQueries := queries.(*sqlite.Queries)
		rows, err = sqliteQueries.GetProductDigitalContent(ctx, productID)
	}

	if err != nil {
		return nil, err
	}

	// Process rows based on database type
	switch v := rows.(type) {
	case []postgres.GetProductDigitalContentRow:
		for _, row := range v {
			if digital.Type == "" && row.Digital.Valid {
				digital.Type = row.Digital.String
			}

			if row.FileID.Valid {
				file := models.File{
					ID:   row.FileID.String,
					Name: row.FileName.String,
					Ext:  row.FileExt.String,
				}
				digital.Files = append(digital.Files, file)
			}
			if row.DataID.Valid {
				data := models.Data{
					ID:      row.DataID.String,
					Content: row.DataContent.String,
					CartID:  row.DataCartID.String,
				}
				digital.Data = append(digital.Data, data)
			}
		}
	case []sqlite.GetProductDigitalContentRow:
		for _, row := range v {
			if digital.Type == "" && row.Digital.Valid {
				digital.Type = row.Digital.String
			}

			if row.FileID.Valid {
				file := models.File{
					ID:   row.FileID.String,
					Name: row.FileName.String,
					Ext:  row.FileExt.String,
				}
				digital.Files = append(digital.Files, file)
			}
			if row.DataID.Valid {
				data := models.Data{
					ID:      row.DataID.String,
					Content: row.DataContent.String,
					CartID:  row.DataCartID.String,
				}
				digital.Data = append(digital.Data, data)
			}
		}
	}

	return digital, nil
}

// DigitalFile retrieves a single digital file by ID and product ID using sqlc.
func (q *ProductQueries) DigitalFile(ctx context.Context, productID, fileID string) (*models.File, error) {
	queries := getSQLCQueries()

	var digitalFile interface{}
	var err error

	if DBType() == "postgres" {
		pgQueries := queries.(*postgres.Queries)
		digitalFile, err = pgQueries.GetDigitalFile(ctx, fileID)
	} else {
		sqliteQueries := queries.(*sqlite.Queries)
		digitalFile, err = sqliteQueries.GetDigitalFile(ctx, fileID)
	}

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, errors.ErrProductNotFound
		}
		return nil, err
	}

	// Verify product_id matches and convert to models.File
	file := &models.File{}
	switch v := digitalFile.(type) {
	case postgres.DigitalFile:
		if v.ProductID != productID {
			return nil, errors.ErrProductNotFound
		}
		file.ID = v.ID
		file.Name = v.Name
		file.Ext = v.Ext
		file.OrigName = v.OrigName
	case sqlite.DigitalFile:
		if v.ProductID != productID {
			return nil, errors.ErrProductNotFound
		}
		file.ID = v.ID
		file.Name = v.Name
		file.Ext = v.Ext
		file.OrigName = v.OrigName
	}

	return file, nil
}

// AddDigitalFile associates a digital file with a product in the database using sqlc.
func (q *ProductQueries) AddDigitalFile(ctx context.Context, productID, fileUUID, fileExt, origName string) (*models.File, error) {
	queries := getSQLCQueries()
	fileID := security.RandomString()

	if DBType() == "postgres" {
		pgQueries := queries.(*postgres.Queries)
		params := postgres.CreateDigitalFileParams{
			ID:        fileID,
			ProductID: productID,
			Name:      fileUUID,
			Ext:       fileExt,
			OrigName:  origName,
		}
		result, err := pgQueries.CreateDigitalFile(ctx, params)
		if err != nil {
			return nil, err
		}
		return &models.File{
			ID:       result.ID,
			Name:     result.Name,
			Ext:      result.Ext,
			OrigName: result.OrigName,
		}, nil
	}

	sqliteQueries := queries.(*sqlite.Queries)
	params := sqlite.CreateDigitalFileParams{
		ID:        fileID,
		ProductID: productID,
		Name:      fileUUID,
		Ext:       fileExt,
		OrigName:  origName,
	}
	result, err := sqliteQueries.CreateDigitalFile(ctx, params)
	if err != nil {
		return nil, err
	}
	return &models.File{
		ID:       result.ID,
		Name:     result.Name,
		Ext:      result.Ext,
		OrigName: result.OrigName,
	}, nil
}

// AddDigitalData adds a new digital data record associated with a product.
func (q *ProductQueries) AddDigitalData(ctx context.Context, productID, content string) (*models.Data, error) {
	file := &models.Data{
		ID:      security.RandomString(),
		Content: content,
	}

	var query string
	if DBType() == "postgres" {
		query = `INSERT INTO digital_data (id, product_id, content) VALUES ($1, $2, $3)`
	} else {
		query = `INSERT INTO digital_data (id, product_id, content) VALUES (?, ?, ?)`
	}
	_, err := q.DB.ExecContext(ctx, query, file.ID, productID, file.Content)
	if err != nil {
		return nil, err
	}

	return file, nil
}

// UpdateDigital updates the content of a digital data record in the database using sqlc.
func (q *ProductQueries) UpdateDigital(ctx context.Context, digital *models.Data) error {
	queries := getSQLCQueries()

	if DBType() == "postgres" {
		pgQueries := queries.(*postgres.Queries)
		params := postgres.UpdateDigitalDataParams{
			Content: digital.Content,
			ID:      digital.ID,
		}
		return pgQueries.UpdateDigitalData(ctx, params)
	}

	sqliteQueries := queries.(*sqlite.Queries)
	params := sqlite.UpdateDigitalDataParams{
		Content: digital.Content,
		ID:      digital.ID,
	}
	return sqliteQueries.UpdateDigitalData(ctx, params)
}

func (q *ProductQueries) DeleteDigital(ctx context.Context, productID, digitalID string) error {
	queries := getSQLCQueries()
	var digitalType string
	var name, ext sql.NullString

	// Get product digital type and file info
	var selectQuery string
	if DBType() == "postgres" {
		selectQuery = `
			SELECT p.digital, df.name, df.ext
			FROM product p
			LEFT JOIN digital_file df ON df.id = $1 AND df.product_id = p.id
			WHERE p.id = $2
		`
	} else {
		selectQuery = `
			SELECT p.digital, df.name, df.ext
			FROM product p
			LEFT JOIN digital_file df ON df.id = ? AND df.product_id = p.id
			WHERE p.id = ?
		`
	}

	err := q.DB.QueryRowContext(ctx, selectQuery, digitalID, productID).Scan(&digitalType, &name, &ext)
	if err != nil {
		return err
	}

	// Delete using sqlc queries
	switch digitalType {
	case "file":
		if DBType() == "postgres" {
			pgQueries := queries.(*postgres.Queries)
			if err := pgQueries.DeleteDigitalFile(ctx, digitalID); err != nil {
				return fmt.Errorf("deleting from digital_file: %w", err)
			}
		} else {
			sqliteQueries := queries.(*sqlite.Queries)
			if err := sqliteQueries.DeleteDigitalFile(ctx, digitalID); err != nil {
				return fmt.Errorf("deleting from digital_file: %w", err)
			}
		}

		filePath := fmt.Sprintf("./lc_digitals/%s.%s", name.String, ext.String)
		if err := os.Remove(filePath); err != nil {
			return fmt.Errorf("failed to remove file %s: %w", filePath, err)
		}

	case "data":
		if DBType() == "postgres" {
			pgQueries := queries.(*postgres.Queries)
			if err := pgQueries.DeleteDigitalData(ctx, digitalID); err != nil {
				return fmt.Errorf("deleting from digital_data: %w", err)
			}
		} else {
			sqliteQueries := queries.(*sqlite.Queries)
			if err := sqliteQueries.DeleteDigitalData(ctx, digitalID); err != nil {
				return fmt.Errorf("deleting from digital_data: %w", err)
			}
		}
	}

	return nil
}

// AddProductWithVariants adds a product with its options and variants in a transaction
func (q *ProductQueries) AddProductWithVariants(ctx context.Context, product *models.Product) (*models.Product, error) {
	tx, err := q.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback()

	// 1. Insert product
	var query string
	if DBType() == "postgres" {
		query = `
			INSERT INTO product (
				id, name, brief, "desc", slug, amount, quantity, sku,
				has_variants, metadata, attribute, digital, active
			) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
		`
	} else {
		query = `
			INSERT INTO product (
				id, name, brief, "desc", slug, amount, quantity, sku,
				has_variants, metadata, attribute, digital, active
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		`
	}

	metadata, err := json.Marshal(product.Metadata)
	if err != nil {
		return nil, fmt.Errorf("marshal metadata: %w", err)
	}

	attributes, err := json.Marshal(product.Attributes)
	if err != nil {
		return nil, fmt.Errorf("marshal attributes: %w", err)
	}

	_, err = tx.ExecContext(ctx, query,
		product.ID,
		product.Name,
		product.Brief,
		product.Description,
		product.Slug,
		product.Amount,
		product.Quantity,
		product.SKU,
		product.HasVariants,
		string(metadata),
		string(attributes),
		product.Digital.Type,
		product.Active,
	)
	if err != nil {
		return nil, fmt.Errorf("insert product: %w", err)
	}

	// 2. Insert product images
	if err = q.insertProductImages(ctx, tx, product.ID, product.Images); err != nil {
		return nil, err
	}

	// 3. Insert options and option values
	if err = q.insertProductOptions(ctx, tx, product.ID, product.Options); err != nil {
		return nil, err
	}

	// 4. Insert variants with relationships and images
	if err = q.insertProductVariants(ctx, tx, product); err != nil {
		return nil, err
	}

	if err = tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit transaction: %w", err)
	}

	return product, nil
}

// insertProductImages inserts all product images within a transaction
func (q *ProductQueries) insertProductImages(ctx context.Context, tx *sql.Tx, productID string, images []models.File) error {
	var query string
	if DBType() == "postgres" {
		query = `
			INSERT INTO product_image (id, product_id, name, ext, orig_name, position)
			VALUES ($1, $2, $3, $4, $5, $6)
		`
	} else {
		query = `
			INSERT INTO product_image (id, product_id, name, ext, orig_name, position)
			VALUES (?, ?, ?, ?, ?, ?)
		`
	}

	for i, img := range images {
		_, err := tx.ExecContext(ctx, query, img.ID, productID, img.Name, img.Ext, img.OrigName, i)
		if err != nil {
			return fmt.Errorf("insert image %d: %w", i, err)
		}
	}
	return nil
}

// insertProductOptions inserts options and their values within a transaction
func (q *ProductQueries) insertProductOptions(ctx context.Context, tx *sql.Tx, productID string, options []models.ProductOption) error {
	var optionQuery, valueQuery string
	if DBType() == "postgres" {
		optionQuery = `
			INSERT INTO product_option (id, product_id, name, position)
			VALUES ($1, $2, $3, $4)
		`
		valueQuery = `
			INSERT INTO product_option_value (id, option_id, value, position)
			VALUES ($1, $2, $3, $4)
		`
	} else {
		optionQuery = `
			INSERT INTO product_option (id, product_id, name, position)
			VALUES (?, ?, ?, ?)
		`
		valueQuery = `
			INSERT INTO product_option_value (id, option_id, value, position)
			VALUES (?, ?, ?, ?)
		`
	}

	for _, option := range options {
		_, err := tx.ExecContext(ctx, optionQuery, option.ID, productID, option.Name, option.Position)
		if err != nil {
			return fmt.Errorf("insert option %s: %w", option.Name, err)
		}

		for _, value := range option.Values {
			_, err = tx.ExecContext(ctx, valueQuery, value.ID, option.ID, value.Value, value.Position)
			if err != nil {
				return fmt.Errorf("insert option value %s: %w", value.Value, err)
			}
		}
	}
	return nil
}

// insertProductVariants inserts variants with relationships and images within a transaction
func (q *ProductQueries) insertProductVariants(ctx context.Context, tx *sql.Tx, product *models.Product) error {
	var variantQuery, selectQuery, relationQuery, imageQuery string
	if DBType() == "postgres" {
		variantQuery = `
			INSERT INTO product_variant (
				id, product_id, sku, price_surcharge, quantity, option_values, active
			) VALUES ($1, $2, $3, $4, $5, $6, $7)
		`
		selectQuery = `
			SELECT pov.id
			FROM product_option_value pov
			JOIN product_option po ON pov.option_id = po.id
			WHERE po.product_id = $1 AND po.name = $2 AND pov.value = $3
		`
		relationQuery = `
			INSERT INTO product_variant_option (variant_id, option_value_id)
			VALUES ($1, $2)
		`
		imageQuery = `
			INSERT INTO product_variant_image (id, variant_id, name, ext, orig_name, position)
			VALUES ($1, $2, $3, $4, $5, $6)
		`
	} else {
		variantQuery = `
			INSERT INTO product_variant (
				id, product_id, sku, price_surcharge, quantity, option_values, active
			) VALUES (?, ?, ?, ?, ?, ?, ?)
		`
		selectQuery = `
			SELECT pov.id
			FROM product_option_value pov
			JOIN product_option po ON pov.option_id = po.id
			WHERE po.product_id = ? AND po.name = ? AND pov.value = ?
		`
		relationQuery = `
			INSERT INTO product_variant_option (variant_id, option_value_id)
			VALUES (?, ?)
		`
		imageQuery = `
			INSERT INTO product_variant_image (id, variant_id, name, ext, orig_name, position)
			VALUES (?, ?, ?, ?, ?, ?)
		`
	}

	for _, variant := range product.Variants {
		// Marshal option values to JSON
		optionValuesJSON, err := json.Marshal(variant.OptionValues)
		if err != nil {
			return fmt.Errorf("marshal option values: %w", err)
		}

		_, err = tx.ExecContext(ctx, variantQuery, variant.ID, product.ID, variant.SKU, variant.PriceSurcharge, variant.Quantity, string(optionValuesJSON), variant.Active)
		if err != nil {
			return fmt.Errorf("insert variant %s: %w", variant.SKU, err)
		}

		// Insert variant-option relationships
		for optionName, optionValue := range variant.OptionValues {
			// Find the option value ID
			var optionValueID string
			err = tx.QueryRowContext(ctx, selectQuery, product.ID, optionName, optionValue).Scan(&optionValueID)
			if err != nil {
				return fmt.Errorf("find option value ID for %s=%s: %w", optionName, optionValue, err)
			}

			_, err = tx.ExecContext(ctx, relationQuery, variant.ID, optionValueID)
			if err != nil {
				return fmt.Errorf("insert variant-option relationship: %w", err)
			}
		}

		// Insert variant images
		for i, img := range variant.Images {
			_, err = tx.ExecContext(ctx, imageQuery, img.ID, variant.ID, img.Name, img.Ext, img.OrigName, i)
			if err != nil {
				return fmt.Errorf("insert variant image %d: %w", i, err)
			}
		}
	}
	return nil
}

// GetProductWithVariants retrieves a product with all its options and variants
func (q *ProductQueries) GetProductWithVariants(ctx context.Context, productID string) (*models.Product, error) {
	product := &models.Product{}

	// 1. Get product base data
	var query string
	if DBType() == "postgres" {
		query = `
			SELECT id, name, brief, "desc", slug, amount, quantity, sku,
			       has_variants, metadata, attribute, digital, active
			FROM product
			WHERE id = $1 AND deleted = FALSE
		`
	} else {
		query = `
			SELECT id, name, brief, "desc", slug, amount, quantity, sku,
			       has_variants, metadata, attribute, digital, active
			FROM product
			WHERE id = ? AND deleted = FALSE
		`
	}

	var metadata, attributes string
	var digitalType sql.NullString

	err := q.DB.QueryRowContext(ctx, query, productID).Scan(
		&product.ID,
		&product.Name,
		&product.Brief,
		&product.Description,
		&product.Slug,
		&product.Amount,
		&product.Quantity,
		&product.SKU,
		&product.HasVariants,
		&metadata,
		&attributes,
		&digitalType,
		&product.Active,
	)
	if err != nil {
		return nil, fmt.Errorf("query product: %w", err)
	}

	// Parse JSON fields
	if err = json.Unmarshal([]byte(metadata), &product.Metadata); err != nil {
		return nil, fmt.Errorf("unmarshal metadata: %w", err)
	}
	if err = json.Unmarshal([]byte(attributes), &product.Attributes); err != nil {
		return nil, fmt.Errorf("unmarshal attributes: %w", err)
	}
	if digitalType.Valid {
		product.Digital.Type = digitalType.String
	}

	// 2. Get product images
	var imageQuery string
	if DBType() == "postgres" {
		imageQuery = `
			SELECT id, name, ext, orig_name
			FROM product_image
			WHERE product_id = $1
			ORDER BY position
		`
	} else {
		imageQuery = `
			SELECT id, name, ext, orig_name
			FROM product_image
			WHERE product_id = ?
			ORDER BY position
		`
	}

	imageRows, err := q.DB.QueryContext(ctx, imageQuery, productID)
	if err != nil {
		return nil, fmt.Errorf("query product images: %w", err)
	}
	defer imageRows.Close()

	for imageRows.Next() {
		var img models.File
		if err := imageRows.Scan(&img.ID, &img.Name, &img.Ext, &img.OrigName); err != nil {
			return nil, fmt.Errorf("scan product image: %w", err)
		}
		product.Images = append(product.Images, img)
	}

	// Skip options/variants if product doesn't have variants
	if !product.HasVariants {
		return product, nil
	}

	// Continue in next part...
	return q.loadProductOptions(ctx, product)
}

// loadProductOptions loads options, option values, and variants for a product
func (q *ProductQueries) loadProductOptions(ctx context.Context, product *models.Product) (*models.Product, error) {
	// Get options
	var optionQuery string
	if DBType() == "postgres" {
		optionQuery = `
			SELECT id, name, position
			FROM product_option
			WHERE product_id = $1
			ORDER BY position
		`
	} else {
		optionQuery = `
			SELECT id, name, position
			FROM product_option
			WHERE product_id = ?
			ORDER BY position
		`
	}

	optionRows, err := q.DB.QueryContext(ctx, optionQuery, product.ID)
	if err != nil {
		return nil, fmt.Errorf("query options: %w", err)
	}
	defer optionRows.Close()

	for optionRows.Next() {
		option := models.ProductOption{ProductID: product.ID}
		if err := optionRows.Scan(&option.ID, &option.Name, &option.Position); err != nil {
			return nil, fmt.Errorf("scan option: %w", err)
		}
		product.Options = append(product.Options, option)
	}

	// Load option values
	if err = q.loadOptionValues(ctx, &product.Options); err != nil {
		return nil, err
	}

	// Load variants with their data
	variants, err := q.loadProductVariants(ctx, product.ID)
	if err != nil {
		return nil, err
	}
	product.Variants = variants

	return product, nil
}

// loadOptionValues loads values for all product options
func (q *ProductQueries) loadOptionValues(ctx context.Context, options *[]models.ProductOption) error {
	var valueQuery string
	if DBType() == "postgres" {
		valueQuery = `
			SELECT id, value, position
			FROM product_option_value
			WHERE option_id = $1
			ORDER BY position
		`
	} else {
		valueQuery = `
			SELECT id, value, position
			FROM product_option_value
			WHERE option_id = ?
			ORDER BY position
		`
	}

	for i := range *options {
		valueRows, err := q.DB.QueryContext(ctx, valueQuery, (*options)[i].ID)
		if err != nil {
			return fmt.Errorf("query option values: %w", err)
		}

		for valueRows.Next() {
			value := models.ProductOptionValue{OptionID: (*options)[i].ID}
			if err := valueRows.Scan(&value.ID, &value.Value, &value.Position); err != nil {
				valueRows.Close()
				return fmt.Errorf("scan option value: %w", err)
			}
			(*options)[i].Values = append((*options)[i].Values, value)
		}
		valueRows.Close()
	}
	return nil
}

// loadProductVariants loads all variants with their option values and images
func (q *ProductQueries) loadProductVariants(ctx context.Context, productID string) ([]models.ProductVariant, error) {
	var variantQuery string
	if DBType() == "postgres" {
		variantQuery = `
			SELECT id, sku, price_surcharge, quantity, active
			FROM product_variant
			WHERE product_id = $1
		`
	} else {
		variantQuery = `
			SELECT id, sku, price_surcharge, quantity, active
			FROM product_variant
			WHERE product_id = ?
		`
	}

	variantRows, err := q.DB.QueryContext(ctx, variantQuery, productID)
	if err != nil {
		return nil, fmt.Errorf("query variants: %w", err)
	}
	defer variantRows.Close()

	var variants []models.ProductVariant

	for variantRows.Next() {
		variant := models.ProductVariant{
			ProductID:    productID,
			OptionValues: make(map[string]string),
		}

		var sku sql.NullString
		if err := variantRows.Scan(&variant.ID, &sku, &variant.PriceSurcharge, &variant.Quantity, &variant.Active); err != nil {
			return nil, fmt.Errorf("scan variant: %w", err)
		}
		if sku.Valid {
			variant.SKU = sku.String
		}

		// Get variant option values
		var optValueQuery string
		if DBType() == "postgres" {
			optValueQuery = `
				SELECT po.name, pov.value
				FROM product_variant_option pvo
				JOIN product_option_value pov ON pvo.option_value_id = pov.id
				JOIN product_option po ON pov.option_id = po.id
				WHERE pvo.variant_id = $1
			`
		} else {
			optValueQuery = `
				SELECT po.name, pov.value
				FROM product_variant_option pvo
				JOIN product_option_value pov ON pvo.option_value_id = pov.id
				JOIN product_option po ON pov.option_id = po.id
				WHERE pvo.variant_id = ?
			`
		}

		optValueRows, err := q.DB.QueryContext(ctx, optValueQuery, variant.ID)
		if err != nil {
			return nil, fmt.Errorf("query variant option values: %w", err)
		}

		for optValueRows.Next() {
			var optionName, optionValue string
			if err := optValueRows.Scan(&optionName, &optionValue); err != nil {
				optValueRows.Close()
				return nil, fmt.Errorf("scan variant option value: %w", err)
			}
			variant.OptionValues[optionName] = optionValue
		}
		optValueRows.Close()

		// Get variant images
		var imgQuery string
		if DBType() == "postgres" {
			imgQuery = `
				SELECT id, name, ext, orig_name
				FROM product_variant_image
				WHERE variant_id = $1
				ORDER BY position
			`
		} else {
			imgQuery = `
				SELECT id, name, ext, orig_name
				FROM product_variant_image
				WHERE variant_id = ?
				ORDER BY position
			`
		}

		imgRows, err := q.DB.QueryContext(ctx, imgQuery, variant.ID)
		if err != nil {
			return nil, fmt.Errorf("query variant images: %w", err)
		}

		for imgRows.Next() {
			var img models.File
			if err := imgRows.Scan(&img.ID, &img.Name, &img.Ext, &img.OrigName); err != nil {
				imgRows.Close()
				return nil, fmt.Errorf("scan variant image: %w", err)
			}
			variant.Images = append(variant.Images, img)
		}
		imgRows.Close()

		variants = append(variants, variant)
	}

	return variants, nil
}

// GenerateUniqueSlug generates a unique URL-friendly slug from product name
func (q *ProductQueries) GenerateUniqueSlug(ctx context.Context, name string, excludeProductID string) (string, error) {
	service := slugify.NewSlugService(q.DB)
	return service.Generate(ctx, name, excludeProductID)
}
