package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"

	"github.com/shurco/mycart/internal/models"
	"github.com/shurco/mycart/internal/store/db"
	"github.com/shurco/mycart/pkg/security"
	"github.com/shurco/mycart/pkg/slugify"
)

// ListProducts retrieves a list of products with optional filtering
func ListProducts(ctx context.Context, private bool, limit, offset int, cartID string, idList ...models.CartProduct) (*models.Products, error) {
	// Get currency setting
	currencySetting, err := db.GetSettingByKeyFunc(ctx, "currency")
	if err != nil {
		return nil, fmt.Errorf("get currency: %w", err)
	}

	products := &models.Products{
		Currency: currencySetting.Value.String,
	}

	// Set default limit if not specified
	if limit <= 0 {
		limit = 999999
	}

	// Use sqlc function pointers based on private flag
	var dbProducts []db.ProductListRow
	if private {
		dbProducts, err = db.ListProductsPrivateFunc(ctx, db.ListProductsPrivateParams{
			Limit:  int32(limit),
			Offset: int32(offset),
		})
	} else {
		dbProducts, err = db.ListProductsPublicFunc(ctx, db.ListProductsPublicParams{
			Limit:  int32(limit),
			Offset: int32(offset),
		})
	}
	if err != nil {
		return nil, fmt.Errorf("list products: %w", err)
	}

	// Convert DB rows to models
	for _, dbProd := range dbProducts {
		p := convertProductListRowToModel(dbProd)
		products.Products = append(products.Products, *p)
	}

	// Get total count of all products (not just current page)
	totalCount, err := db.CountProductsFunc(ctx)
	if err != nil {
		return nil, fmt.Errorf("count products: %w", err)
	}
	products.Total = int(totalCount)

	return products, nil
}

// Product retrieves a single product by ID or slug
func Product(ctx context.Context, private bool, productIDOrSlug string) (*models.Product, error) {
	// Try fetching by ID first, then by slug
	dbProduct, err := db.GetProductDetailByIDFunc(ctx, productIDOrSlug)
	if err != nil {
		// If not found by ID, try by slug
		if err == sql.ErrNoRows {
			dbProduct, err = db.GetProductDetailBySlugFunc(ctx, productIDOrSlug)
			if err != nil {
				if err == sql.ErrNoRows {
					return nil, fmt.Errorf("product not found")
				}
				return nil, err
			}
		} else {
			return nil, err
		}
	}

	// Apply private/public filtering
	if !private && !dbProduct.Active {
		return nil, fmt.Errorf("product not found")
	}

	// Convert DB product to model
	p := convertProductDetailToModel(dbProduct)

	// Load images
	if err := loadProductImages(ctx, p); err != nil {
		return nil, err
	}

	// Load variants if has_variants
	if p.HasVariants {
		if err := loadProductVariants(ctx, p); err != nil {
			return nil, err
		}
	}

	return p, nil
}

// buildCreateParams creates CreateProductParams from a product model
func buildCreateParams(product *models.Product) (db.CreateProductParams, error) {
	metadata, err := json.Marshal(product.Metadata)
	if err != nil {
		return db.CreateProductParams{}, fmt.Errorf("marshal metadata: %w", err)
	}

	attributes, err := json.Marshal(product.Attributes)
	if err != nil {
		return db.CreateProductParams{}, fmt.Errorf("marshal attributes: %w", err)
	}

	seoJSON, err := json.Marshal(product.Seo)
	if err != nil {
		return db.CreateProductParams{}, fmt.Errorf("marshal seo: %w", err)
	}
	// Default to empty JSON object if seo is null/empty
	if len(seoJSON) == 0 || string(seoJSON) == "null" {
		seoJSON = []byte("{}")
	}

	// Convert empty SKU to NULL to avoid unique constraint violations
	var sku sql.NullString
	if product.SKU != "" {
		sku = sql.NullString{String: product.SKU, Valid: true}
	}

	// Convert quantity to NullInt64
	var quantity sql.NullInt64
	if product.Quantity > 0 {
		quantity = sql.NullInt64{Int64: int64(product.Quantity), Valid: true}
	}

	// Convert digital type to NullString
	var digital sql.NullString
	if product.Digital.Type != "" {
		digital = sql.NullString{String: product.Digital.Type, Valid: true}
	}

	return db.CreateProductParams{
		ID:          product.ID,
		Name:        product.Name,
		Brief:       product.Brief,
		Desc:        product.Description,
		Slug:        product.Slug,
		Amount:      fmt.Sprintf("%d", product.Amount),
		Metadata:    metadata,
		Attribute:   attributes,
		Digital:     digital,
		Active:      product.Active,
		HasVariants: product.HasVariants,
		Quantity:    quantity,
		SKU:         sku,
		Seo:         seoJSON,
	}, nil
}

// AddProductWithVariants creates a new product with variants
func AddProductWithVariants(ctx context.Context, product *models.Product) (*models.Product, error) {
	if product.ID == "" {
		product.ID = security.RandomString()
	}

	// Build create params
	params, err := buildCreateParams(product)
	if err != nil {
		return nil, err
	}

	// Create product
	dbProduct, err := db.CreateProductFunc(ctx, params)
	if err != nil {
		return nil, err
	}

	// Set created timestamp from returned value
	if dbProduct.Created.Valid {
		product.Created = dbProduct.Created.Time.Unix()
	}

	// Insert options and variants if present
	if len(product.Options) > 0 {
		if err := syncProductOptions(ctx, product.ID, product.Options); err != nil {
			return nil, err
		}
	}

	if len(product.Variants) > 0 {
		if err := syncProductVariants(ctx, product.ID, product.Variants); err != nil {
			return nil, err
		}
	}

	return product, nil
}

// AddProductStub - wrapper for compatibility
func AddProductStub(ctx context.Context, product *models.Product) error {
	_, err := AddProductWithVariants(ctx, product)
	return err
}

// AddProduct creates a new product without variants
func AddProduct(ctx context.Context, product *models.Product) (*models.Product, error) {
	product.HasVariants = false
	return AddProductWithVariants(ctx, product)
}

// mergeProductFields merges non-zero fields from updates into existing product
func mergeProductFields(existing, updates *models.Product) {
	if updates.Name != "" {
		existing.Name = updates.Name
	}
	if updates.Brief != "" {
		existing.Brief = updates.Brief
	}
	if updates.Description != "" {
		existing.Description = updates.Description
	}
	if updates.Slug != "" {
		existing.Slug = updates.Slug
	}
	if updates.Amount != 0 {
		existing.Amount = updates.Amount
	}
	if updates.Quantity != 0 {
		existing.Quantity = updates.Quantity
	}
	if updates.SKU != "" {
		existing.SKU = updates.SKU
	}
	if updates.Digital.Type != "" {
		existing.Digital.Type = updates.Digital.Type
	}
	if len(updates.Metadata) > 0 {
		existing.Metadata = updates.Metadata
	}
	if len(updates.Attributes) > 0 {
		existing.Attributes = updates.Attributes
	}
	if updates.Seo != nil {
		existing.Seo = updates.Seo
	}
	// Active is a boolean, so we always use the update value
	existing.Active = updates.Active
}

// buildUpdateParams creates UpdateProductFullParams from a product model
func buildUpdateParams(product *models.Product) (db.UpdateProductFullParams, error) {
	metadata, err := json.Marshal(product.Metadata)
	if err != nil {
		return db.UpdateProductFullParams{}, fmt.Errorf("marshal metadata: %w", err)
	}

	attributes, err := json.Marshal(product.Attributes)
	if err != nil {
		return db.UpdateProductFullParams{}, fmt.Errorf("marshal attributes: %w", err)
	}

	seoJSON, err := json.Marshal(product.Seo)
	if err != nil {
		return db.UpdateProductFullParams{}, fmt.Errorf("marshal seo: %w", err)
	}

	return db.UpdateProductFullParams{
		Name:      product.Name,
		Brief:     product.Brief,
		Desc:      product.Description,
		Slug:      product.Slug,
		Amount:    fmt.Sprintf("%d", product.Amount),
		Quantity:  sql.NullInt64{Int64: int64(product.Quantity), Valid: product.Quantity != 0},
		Sku:       sql.NullString{String: product.SKU, Valid: product.SKU != ""},
		Metadata:  metadata,
		Attribute: attributes,
		Seo:       seoJSON,
		ID:        product.ID,
	}, nil
}

// buildUpdateParamsWithVariants creates UpdateProductFullParams with HasVariants field
func buildUpdateParamsWithVariants(product *models.Product) (db.UpdateProductFullParams, error) {
	params, err := buildUpdateParams(product)
	if err != nil {
		return db.UpdateProductFullParams{}, err
	}
	params.HasVariants = sql.NullBool{Bool: product.HasVariants, Valid: true}
	return params, nil
}

// syncProductOptions synchronizes product options and their values
func syncProductOptions(ctx context.Context, productID string, options []models.ProductOption) error {
	// Delete existing options (cascades to option_values via foreign key)
	if err := db.DeleteProductOptionsByProductFunc(ctx, productID); err != nil {
		return fmt.Errorf("delete existing options: %w", err)
	}

	// Re-create options and option values
	for i, option := range options {
		if option.ID == "" {
			option.ID = security.RandomString()
		}

		_, err := db.CreateProductOptionFunc(ctx, db.CreateProductOptionParams{
			ID:        option.ID,
			ProductID: productID,
			Name:      option.Name,
			Position:  sql.NullInt64{Int64: int64(i), Valid: true},
		})
		if err != nil {
			return fmt.Errorf("create option: %w", err)
		}

		for j, value := range option.Values {
			if value.ID == "" {
				value.ID = security.RandomString()
			}

			_, err = db.CreateProductOptionValueFunc(ctx, db.CreateProductOptionValueParams{
				ID:       value.ID,
				OptionID: option.ID,
				Value:    value.Value,
				Position: sql.NullInt64{Int64: int64(j), Valid: true},
			})
			if err != nil {
				return fmt.Errorf("create option value: %w", err)
			}
		}
	}

	return nil
}

// syncProductVariants synchronizes product variants
func syncProductVariants(ctx context.Context, productID string, variants []models.ProductVariant) error {
	// Delete existing variants
	if err := db.DeleteProductVariantsByProductFunc(ctx, productID); err != nil {
		return fmt.Errorf("delete existing variants: %w", err)
	}

	// Re-create variants
	for _, variant := range variants {
		if variant.ID == "" {
			variant.ID = security.RandomString()
		}

		optionValues, err := json.Marshal(variant.OptionValues)
		if err != nil {
			return fmt.Errorf("marshal option values: %w", err)
		}

		var variantSKU sql.NullString
		if variant.SKU != "" {
			variantSKU = sql.NullString{String: variant.SKU, Valid: true}
		}

		_, err = db.CreateProductVariantFunc(ctx, db.CreateProductVariantParams{
			ID:             variant.ID,
			ProductID:      productID,
			Sku:            variantSKU,
			PriceSurcharge: sql.NullString{String: fmt.Sprintf("%d", variant.PriceSurcharge), Valid: true},
			Quantity:       sql.NullInt64{Int64: int64(variant.Quantity), Valid: true},
			OptionValues:   string(optionValues),
		})
		if err != nil {
			return fmt.Errorf("create variant: %w", err)
		}
	}

	return nil
}

// UpdateProduct updates an existing product
func UpdateProduct(ctx context.Context, product *models.Product) error {
	// Fetch existing product to merge updates (prevents zero values from overwriting existing data)
	existing, err := Product(ctx, true, product.ID)
	if err != nil {
		return fmt.Errorf("fetch existing product: %w", err)
	}

	// Merge non-zero fields from request into existing product
	mergeProductFields(existing, product)

	// Prepare JSON fields and build update params
	params, err := buildUpdateParams(existing)
	if err != nil {
		return fmt.Errorf("build update params: %w", err)
	}

	return db.UpdateProductFullFunc(ctx, params)
}

// UpdateProductWithVariants updates a product with variants (options and variants data)
func UpdateProductWithVariants(ctx context.Context, product *models.Product) error {
	// Fetch existing product to preserve fields not in the update request
	existing, err := Product(ctx, true, product.ID)
	if err != nil {
		return fmt.Errorf("fetch existing product: %w", err)
	}

	// Merge non-zero fields from request into existing product
	mergeProductFields(existing, product)

	// Handle variant data
	existing.HasVariants = product.HasVariants
	if product.HasVariants {
		existing.Options = product.Options
		existing.Variants = product.Variants
	}

	// Build and execute base product update
	params, err := buildUpdateParamsWithVariants(existing)
	if err != nil {
		return fmt.Errorf("build update params: %w", err)
	}

	if err := db.UpdateProductFullFunc(ctx, params); err != nil {
		return fmt.Errorf("update product: %w", err)
	}

	// Sync options and variants based on has_variants flag
	if existing.HasVariants {
		if err := syncProductOptions(ctx, existing.ID, existing.Options); err != nil {
			return err
		}
		if err := syncProductVariants(ctx, existing.ID, existing.Variants); err != nil {
			return err
		}
	} else {
		// Clean up any existing variant data
		if err := db.DeleteProductOptionsByProductFunc(ctx, existing.ID); err != nil {
			return fmt.Errorf("cleanup options: %w", err)
		}
		if err := db.DeleteProductVariantsByProductFunc(ctx, existing.ID); err != nil {
			return fmt.Errorf("cleanup variants: %w", err)
		}
	}

	return nil
}

// DeleteProduct deletes a product by ID (soft delete)
func DeleteProduct(ctx context.Context, productID string) error {
	// Guard: prevent deletion if product has sold digital data
	hasSold, err := db.ProductHasSoldDigitalDataFunc(ctx, productID)
	if err != nil {
		return fmt.Errorf("check sold digital data: %w", err)
	}
	if hasSold {
		return fmt.Errorf("cannot delete product with sold digital keys")
	}

	// Hard delete products without sold digital data
	return db.DeleteProductFunc(ctx, productID)
}

// UpdateActive toggles product active status
func UpdateActive(ctx context.Context, productID string) error {
	return db.UpdateProductActiveFunc(ctx, productID)
}

// ProductImages retrieves all images for a product
func ProductImages(ctx context.Context, productID string) (*[]models.File, error) {
	dbImages, err := db.ListProductImagesFunc(ctx, productID)
	if err != nil {
		return nil, err
	}

	images := make([]models.File, len(dbImages))
	for i, img := range dbImages {
		var position *int
		if img.Position.Valid {
			pos := int(img.Position.Int64)
			position = &pos
		}

		images[i] = models.File{
			ID:       img.ID,
			Name:     img.Name,
			Ext:      img.Ext,
			OrigName: img.OrigName,
			Position: position,
		}
	}

	return &images, nil
}

// AddImage adds an image to a product
func AddImage(ctx context.Context, productID, fileUUID, fileExt, fileOrigName string) (*models.File, error) {
	imageID := security.RandomString()

	dbImage, err := db.CreateProductImageFunc(ctx, db.CreateProductImageParams{
		ID:        imageID,
		ProductID: productID,
		Name:      fileUUID,
		Ext:       fileExt,
		OrigName:  fileOrigName,
	})
	if err != nil {
		return nil, err
	}

	return &models.File{
		ID:       dbImage.ID,
		Name:     dbImage.Name,
		Ext:      dbImage.Ext,
		OrigName: dbImage.OrigName,
	}, nil
}

// DeleteImage deletes an image from a product
func DeleteImage(ctx context.Context, productID, imageID string) error {
	// Note: sqlc-generated DeleteProductImage only takes imageID, not productID
	// The original query verified product_id for safety, but the generated version doesn't
	return db.DeleteProductImageFunc(ctx, imageID)
}

// ReorderImages updates position values for product images
func ReorderImages(ctx context.Context, productID string, positions []db.UpdateProductImagePositionParams) error {
	for _, p := range positions {
		if err := db.UpdateProductImagePositionFunc(ctx, p); err != nil {
			return fmt.Errorf("update position for image %s: %w", p.ID, err)
		}
	}

	return nil
}

// GetProductRepImage retrieves the first image (by position) for a product slug
func GetProductRepImage(ctx context.Context, slug string) (*models.File, error) {
	row, err := db.GetProductRepImageBySlugFunc(ctx, slug)
	if err != nil {
		return nil, err
	}
	return &models.File{
		ID:       row.ID,
		Name:     row.Name,
		Ext:      row.Ext,
		OrigName: row.OrigName,
	}, nil
}

// ProductDigital retrieves digital content for a product
func ProductDigital(ctx context.Context, productID string) (*models.Digital, error) {
	digital := &models.Digital{}

	// Get digital type from product using GetProductByIDFunc
	dbProduct, err := db.GetProductByIDFunc(ctx, productID)
	if err != nil {
		return nil, err
	}
	if dbProduct.Digital.Valid {
		digital.Type = dbProduct.Digital.String
	}

	// Get digital files using ListDigitalFilesFunc
	dbFiles, err := db.ListDigitalFilesFunc(ctx, productID)
	if err == nil {
		for _, dbf := range dbFiles {
			digital.Files = append(digital.Files, models.File{
				ID:   dbf.ID,
				Name: dbf.Name,
				Ext:  dbf.Ext,
			})
		}
	}

	// Get unassigned digital data using ListUnassignedDigitalDataByProductFunc
	dbData, err := db.ListUnassignedDigitalDataByProductFunc(ctx, productID)
	if err == nil {
		for _, dbd := range dbData {
			digital.Data = append(digital.Data, models.Data{
				ID:      dbd.ID,
				Content: dbd.Content,
			})
		}
	}

	return digital, nil
}

// AddDigitalFile adds a digital file to a product
func AddDigitalFile(ctx context.Context, productID, fileUUID, fileExt, fileOrigName string) (*models.File, error) {
	fileID := security.RandomString()

	dbFile, err := db.CreateDigitalFileFunc(ctx, db.CreateDigitalFileParams{
		ID:        fileID,
		ProductID: productID,
		Name:      fileUUID,
		Ext:       fileExt,
		OrigName:  fileOrigName,
	})
	if err != nil {
		return nil, err
	}

	return &models.File{
		ID:       dbFile.ID,
		Name:     dbFile.Name,
		Ext:      dbFile.Ext,
		OrigName: dbFile.OrigName,
	}, nil
}

// DigitalFile retrieves a digital file by ID for a product
func DigitalFile(ctx context.Context, productID, fileID string) (*models.File, error) {
	// Note: sqlc-generated GetDigitalFile only takes fileID, not productID
	// The original query verified product_id for safety, but the generated version doesn't
	dbFile, err := db.GetDigitalFileFunc(ctx, fileID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("digital file not found")
		}
		return nil, err
	}
	return &models.File{
		ID:       dbFile.ID,
		Name:     dbFile.Name,
		Ext:      dbFile.Ext,
		OrigName: dbFile.OrigName,
	}, nil
}

// Helper functions

// convertProductDetailToModel converts sqlc ProductDetail to models.Product
func convertProductDetailToModel(detail db.ProductDetail) *models.Product {
	p := &models.Product{
		Core: models.Core{
			ID: detail.ID,
		},
		Name:        detail.Name,
		Brief:       detail.Brief,
		Description: detail.Desc,
		Slug:        detail.Slug,
		Active:      detail.Active,
		HasVariants: detail.HasVariants,
	}

	// Parse amount from string to int (cents)
	if detail.Amount != "" {
		fmt.Sscanf(detail.Amount, "%d", &p.Amount)
	}

	if detail.Quantity.Valid {
		p.Quantity = int(detail.Quantity.Int64)
	}

	if detail.Sku.Valid {
		p.SKU = detail.Sku.String
	}

	if detail.Digital.Valid {
		p.Digital.Type = detail.Digital.String
	}

	if detail.Created.Valid {
		p.Created = detail.Created.Time.Unix()
	}

	if detail.Updated.Valid {
		p.Updated = detail.Updated.Time.Unix()
	}

	// Parse metadata JSON
	if len(detail.Metadata) > 0 {
		json.Unmarshal(detail.Metadata, &p.Metadata)
	}

	// Parse attributes JSON
	if len(detail.Attribute) > 0 {
		json.Unmarshal(detail.Attribute, &p.Attributes)
	}

	// Parse SEO JSON
	if len(detail.Seo) > 0 {
		json.Unmarshal(detail.Seo, &p.Seo)
	}

	return p
}

// convertProductListRowToModel converts sqlc ProductListRow to models.Product
func convertProductListRowToModel(row db.ProductListRow) *models.Product {
	p := &models.Product{
		Core: models.Core{
			ID: row.ID,
		},
		Name:        row.Name,
		Brief:       row.Brief,
		Description: row.Desc,
		Slug:        row.Slug,
		Active:      row.Active,
	}

	// Parse amount from string to int (cents)
	if row.Amount != "" {
		fmt.Sscanf(row.Amount, "%d", &p.Amount)
	}

	if row.Quantity.Valid {
		p.Quantity = int(row.Quantity.Int64)
	}

	if row.Sku.Valid {
		p.SKU = row.Sku.String
	}

	if row.Digital.Valid {
		p.Digital.Type = row.Digital.String
	}

	if row.Created.Valid {
		p.Created = row.Created.Time.Unix()
	}

	if row.Updated.Valid {
		p.Updated = row.Updated.Time.Unix()
	}

	// Parse metadata JSON
	if len(row.Metadata) > 0 {
		json.Unmarshal(row.Metadata, &p.Metadata)
	}

	// Parse attributes JSON
	if len(row.Attribute) > 0 {
		json.Unmarshal(row.Attribute, &p.Attributes)
	}

	// Handle HasVariants nullable boolean
	if row.HasVariants.Valid {
		p.HasVariants = row.HasVariants.Bool
	}

	// Parse images JSON array
	if len(row.Image) > 0 && string(row.Image) != "[]" {
		json.Unmarshal(row.Image, &p.Images)
	}

	// Parse variants JSON array
	if len(row.Variants) > 0 && string(row.Variants) != "[]" {
		json.Unmarshal(row.Variants, &p.Variants)
	}

	return p
}

func loadProductImages(ctx context.Context, product *models.Product) error {
	dbImages, err := db.ListProductImagesFunc(ctx, product.ID)
	if err != nil {
		return err
	}

	for _, img := range dbImages {
		var position *int
		if img.Position.Valid {
			pos := int(img.Position.Int64)
			position = &pos
		}

		product.Images = append(product.Images, models.File{
			ID:       img.ID,
			Name:     img.Name,
			Ext:      img.Ext,
			OrigName: img.OrigName,
			Position: position,
		})
	}
	return nil
}

func loadProductVariants(ctx context.Context, product *models.Product) error {
	// Load options using sqlc function
	dbOptions, err := db.ListProductOptionsByProductFunc(ctx, product.ID)
	if err != nil {
		return err
	}

	for _, dbOpt := range dbOptions {
		var option models.ProductOption
		option.ID = dbOpt.ID
		option.Name = dbOpt.Name
		option.ProductID = dbOpt.ProductID
		if dbOpt.Position.Valid {
			option.Position = int(dbOpt.Position.Int64)
		}

		// Load option values using sqlc function
		dbValues, err := db.ListProductOptionValuesByOptionFunc(ctx, option.ID)
		if err != nil {
			return err
		}

		for _, dbVal := range dbValues {
			var value models.ProductOptionValue
			value.ID = dbVal.ID
			value.OptionID = dbVal.OptionID
			value.Value = dbVal.Value
			if dbVal.Position.Valid {
				value.Position = int(dbVal.Position.Int64)
			}
			option.Values = append(option.Values, value)
		}

		product.Options = append(product.Options, option)
	}

	// Load variants
	dbVariants, err := db.ListProductVariantsByProductFunc(ctx, product.ID)
	if err != nil {
		return err
	}

	for _, dbV := range dbVariants {
		var v models.ProductVariant
		v.ID = dbV.ID
		v.SKU = dbV.Sku.String
		if dbV.Quantity.Valid {
			v.Quantity = int(dbV.Quantity.Int64)
		}
		// Parse price surcharge from string to int (cents)
		if dbV.PriceSurcharge.Valid {
			fmt.Sscanf(dbV.PriceSurcharge.String, "%d", &v.PriceSurcharge)
		}
		if dbV.Active.Valid {
			v.Active = dbV.Active.Bool
		}

		json.Unmarshal([]byte(dbV.OptionValues), &v.OptionValues)

		product.Variants = append(product.Variants, v)
	}

	return nil
}

// AddDigitalData adds digital data (license key) to a product
func AddDigitalData(ctx context.Context, productID, content string) (*models.Data, error) {
	fileID := security.RandomString()

	// Use CreateDigitalDataFunc
	dbData, err := db.CreateDigitalDataFunc(ctx, db.CreateDigitalDataParams{
		ID:        fileID,
		ProductID: productID,
		Content:   content,
		CartID:    sql.NullString{}, // NULL - unassigned
	})
	if err != nil {
		return nil, err
	}

	return &models.Data{
		ID:      dbData.ID,
		Content: dbData.Content,
	}, nil
}

// UpdateDigital updates the content of a digital data record
func UpdateDigital(ctx context.Context, digital *models.Data) error {
	// Use UpdateDigitalDataFunc
	return db.UpdateDigitalDataFunc(ctx, db.UpdateDigitalDataParams{
		Content: digital.Content,
		ID:      digital.ID,
	})
}

// DeleteDigital deletes digital content (file or data) from a product
func DeleteDigital(ctx context.Context, productID, digitalID string) error {
	// Get product to determine digital type
	dbProduct, err := db.GetProductByIDFunc(ctx, productID)
	if err != nil {
		return err
	}

	if !dbProduct.Digital.Valid {
		return fmt.Errorf("product has no digital type")
	}

	switch dbProduct.Digital.String {
	case "file":
		// Get file info before deleting
		dbFile, err := db.GetDigitalFileFunc(ctx, digitalID)
		if err != nil {
			return fmt.Errorf("getting digital file: %w", err)
		}

		// Delete from database using DeleteDigitalFileFunc
		if err := db.DeleteDigitalFileFunc(ctx, digitalID); err != nil {
			return fmt.Errorf("deleting from digital_file: %w", err)
		}

		// Delete physical file
		filePath := fmt.Sprintf("./lc_digitals/%s.%s", dbFile.Name, dbFile.Ext)
		if err := os.Remove(filePath); err != nil {
			return fmt.Errorf("failed to remove file %s: %w", filePath, err)
		}

	case "data":
		// Delete from database using DeleteDigitalDataFunc
		if err := db.DeleteDigitalDataFunc(ctx, digitalID); err != nil {
			return fmt.Errorf("deleting from digital_data: %w", err)
		}
	}

	return nil
}

// GenerateUniqueSlug creates a URL-friendly slug from name with uniqueness check
func GenerateUniqueSlug(ctx context.Context, name string, excludeProductID string) (string, error) {
	service := slugify.NewSlugService(db.DB())
	return service.Generate(ctx, name, excludeProductID)
}

// --- sqlc-based methods (new implementation) ---

// GetProductByID retrieves a product by ID using sqlc function pointer.
func GetProductByID(ctx context.Context, id string) (db.Product, error) {
	return db.GetProductByIDFunc(ctx, id)
}

// GetProductBySlug retrieves a product by slug using sqlc function pointer.
func GetProductBySlug(ctx context.Context, slug string) (db.Product, error) {
	return db.GetProductBySlugFunc(ctx, slug)
}

// CreateProduct creates a new product using sqlc function pointer.
func CreateProduct(ctx context.Context, params db.CreateProductParams) (db.Product, error) {
	return db.CreateProductFunc(ctx, params)
}

// UpdateProductSqlc updates a product using sqlc function pointer.
func UpdateProductSqlc(ctx context.Context, params db.UpdateProductParams) error {
	return db.UpdateProductFunc(ctx, params)
}

// DeleteProductSqlc deletes a product using sqlc function pointer.
func DeleteProductSqlc(ctx context.Context, id string) error {
	return db.DeleteProductFunc(ctx, id)
}
