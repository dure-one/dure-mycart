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

	// Build query
	var query string
	var params []any
	paramCount := 0

	if private {
		query = `SELECT id, name, brief, desc, slug, amount, quantity, sku, has_variants, active, digital, metadata, attribute, created, updated
		         FROM product WHERE deleted = 0`
	} else {
		if db.Type() == "postgres" {
			query = `SELECT id, name, brief, desc, slug, amount, quantity, sku, has_variants, active, digital, metadata, attribute, created, updated
			         FROM product WHERE deleted = false AND active = true`
		} else {
			query = `SELECT id, name, brief, desc, slug, amount, quantity, sku, has_variants, active, digital, metadata, attribute, created, updated
			         FROM product WHERE deleted = 0 AND active = 1`
		}
	}

	// Add pagination
	if limit > 0 {
		paramCount++
		if db.Type() == "postgres" {
			query += fmt.Sprintf(" LIMIT $%d", paramCount)
		} else {
			query += " LIMIT ?"
		}
		params = append(params, limit)

		if offset > 0 {
			paramCount++
			if db.Type() == "postgres" {
				query += fmt.Sprintf(" OFFSET $%d", paramCount)
			} else {
				query += " OFFSET ?"
			}
			params = append(params, offset)
		}
	}

	rows, err := db.DB().QueryContext(ctx, query, params...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var p models.Product
		var metadata, attributes, digitalType sql.NullString
		var quantity sql.NullInt64
		var sku sql.NullString
		var hasVariants sql.NullBool
		var created, updated sql.NullTime

		err := rows.Scan(&p.ID, &p.Name, &p.Brief, &p.Description, &p.Slug, &p.Amount, &quantity, &sku,
			&hasVariants, &p.Active, &digitalType, &metadata, &attributes, &created, &updated)
		if err != nil {
			return nil, err
		}

		if quantity.Valid {
			p.Quantity = int(quantity.Int64)
		}
		if sku.Valid {
			p.SKU = sku.String
		}
		if hasVariants.Valid {
			p.HasVariants = hasVariants.Bool
		}
		if digitalType.Valid {
			p.Digital.Type = digitalType.String
		}
		if created.Valid {
			p.Created = created.Time.Unix()
		}
		if updated.Valid {
			p.Updated = updated.Time.Unix()
		}
		if metadata.Valid {
			json.Unmarshal([]byte(metadata.String), &p.Metadata)
		}
		if attributes.Valid {
			json.Unmarshal([]byte(attributes.String), &p.Attributes)
		}

		products.Products = append(products.Products, p)
	}

	return products, nil
}

// Product retrieves a single product by ID
func Product(ctx context.Context, private bool, productID string) (*models.Product, error) {
	var query string
	if private {
		if db.Type() == "postgres" {
			query = `SELECT id, name, brief, desc, slug, amount, quantity, sku, has_variants, active, digital, metadata, attribute, seo, created, updated
			         FROM product WHERE id = $1 AND deleted = false`
		} else {
			query = `SELECT id, name, brief, desc, slug, amount, quantity, sku, has_variants, active, digital, metadata, attribute, seo, created, updated
			         FROM product WHERE id = ? AND deleted = 0`
		}
	} else {
		if db.Type() == "postgres" {
			query = `SELECT id, name, brief, desc, slug, amount, quantity, sku, has_variants, active, digital, metadata, attribute, seo, created, updated
			         FROM product WHERE id = $1 AND deleted = false AND active = true`
		} else {
			query = `SELECT id, name, brief, desc, slug, amount, quantity, sku, has_variants, active, digital, metadata, attribute, seo, created, updated
			         FROM product WHERE id = ? AND deleted = 0 AND active = 1`
		}
	}

	var p models.Product
	var metadata, attributes, digitalType, seo sql.NullString
	var quantity sql.NullInt64
	var sku sql.NullString
	var hasVariants sql.NullBool
	var created, updated sql.NullTime

	err := db.DB().QueryRowContext(ctx, query, productID).Scan(
		&p.ID, &p.Name, &p.Brief, &p.Description, &p.Slug, &p.Amount, &quantity, &sku,
		&hasVariants, &p.Active, &digitalType, &metadata, &attributes, &seo, &created, &updated)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("product not found")
		}
		return nil, err
	}

	if quantity.Valid {
		p.Quantity = int(quantity.Int64)
	}
	if sku.Valid {
		p.SKU = sku.String
	}
	if hasVariants.Valid {
		p.HasVariants = hasVariants.Bool
	}
	if digitalType.Valid {
		p.Digital.Type = digitalType.String
	}
	if created.Valid {
		p.Created = created.Time.Unix()
	}
	if updated.Valid {
		p.Updated = updated.Time.Unix()
	}
	if metadata.Valid {
		json.Unmarshal([]byte(metadata.String), &p.Metadata)
	}
	if attributes.Valid {
		json.Unmarshal([]byte(attributes.String), &p.Attributes)
	}
	if seo.Valid {
		json.Unmarshal([]byte(seo.String), &p.Seo)
	}

	// Load images
	if err := loadProductImages(ctx, &p); err != nil {
		return nil, err
	}

	// Load variants if has_variants
	if p.HasVariants {
		if err := loadProductVariants(ctx, &p); err != nil {
			return nil, err
		}
	}

	return &p, nil
}

// AddProductWithVariants creates a new product with variants
func AddProductWithVariants(ctx context.Context, product *models.Product) (*models.Product, error) {
	if product.ID == "" {
		product.ID = security.RandomString()
	}

	metadata, err := json.Marshal(product.Metadata)
	if err != nil {
		return nil, err
	}

	attributes, err := json.Marshal(product.Attributes)
	if err != nil {
		return nil, err
	}

	seoJSON, err := json.Marshal(product.Seo)
	if err != nil {
		return nil, err
	}

	var query string
	if db.Type() == "postgres" {
		query = `INSERT INTO product (id, name, amount, slug, metadata, attribute, brief, desc, digital, active, has_variants, quantity, sku, seo)
		         VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
		         RETURNING EXTRACT(EPOCH FROM created)::bigint`
	} else {
		query = `INSERT INTO product (id, name, amount, slug, metadata, attribute, brief, desc, digital, active, has_variants, quantity, sku, seo)
		         VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		         RETURNING strftime('%s', created)`
	}

	err = db.DB().QueryRowContext(ctx, query,
		product.ID, product.Name, product.Amount, product.Slug,
		metadata, attributes, product.Brief, product.Description,
		product.Digital.Type, product.Active, product.HasVariants, product.Quantity, product.SKU, seoJSON,
	).Scan(&product.Created)
	if err != nil {
		return nil, err
	}

	// Insert variants if present
	if len(product.Variants) > 0 {
		for _, variant := range product.Variants {
			if variant.ID == "" {
				variant.ID = security.RandomString()
			}

			optionValues, err := json.Marshal(variant.OptionValues)
			if err != nil {
				continue
			}

			var variantQuery string
			if db.Type() == "postgres" {
				variantQuery = `INSERT INTO product_variant (id, product_id, sku, quantity, price_surcharge, option_values, active)
				                VALUES ($1, $2, $3, $4, $5, $6, $7)`
			} else {
				variantQuery = `INSERT INTO product_variant (id, product_id, sku, quantity, price_surcharge, option_values, active)
				                VALUES (?, ?, ?, ?, ?, ?, ?)`
			}

			_, err = db.DB().ExecContext(ctx, variantQuery,
				variant.ID, product.ID, variant.SKU, variant.Quantity, variant.PriceSurcharge, optionValues, variant.Active)
			if err != nil {
				return nil, err
			}
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

// UpdateProduct updates an existing product
func UpdateProduct(ctx context.Context, product *models.Product) error {
	metadata, _ := json.Marshal(product.Metadata)
	attributes, _ := json.Marshal(product.Attributes)
	seoJSON, _ := json.Marshal(product.Seo)

	var query string
	if db.Type() == "postgres" {
		query = `UPDATE product SET name = $1, amount = $2, slug = $3, metadata = $4, attribute = $5,
		         brief = $6, desc = $7, digital = $8, active = $9, quantity = $10, sku = $11, seo = $12
		         WHERE id = $13`
	} else {
		query = `UPDATE product SET name = ?, amount = ?, slug = ?, metadata = ?, attribute = ?,
		         brief = ?, desc = ?, digital = ?, active = ?, quantity = ?, sku = ?, seo = ?
		         WHERE id = ?`
	}

	_, err := db.DB().ExecContext(ctx, query, product.Name, product.Amount, product.Slug,
		metadata, attributes, product.Brief, product.Description, product.Digital.Type,
		product.Active, product.Quantity, product.SKU, seoJSON, product.ID)
	return err
}

// DeleteProduct deletes a product by ID (soft delete)
func DeleteProduct(ctx context.Context, productID string) error {
	var query string
	if db.Type() == "postgres" {
		query = `UPDATE product SET deleted = true WHERE id = $1`
	} else {
		query = `UPDATE product SET deleted = 1 WHERE id = ?`
	}
	_, err := db.DB().ExecContext(ctx, query, productID)
	return err
}

// UpdateActive toggles product active status
func UpdateActive(ctx context.Context, productID string) error {
	var query string
	if db.Type() == "postgres" {
		query = `UPDATE product SET active = NOT active WHERE id = $1`
	} else {
		query = `UPDATE product SET active = NOT active WHERE id = ?`
	}
	_, err := db.DB().ExecContext(ctx, query, productID)
	return err
}

// ProductImages retrieves all images for a product
func ProductImages(ctx context.Context, productID string) (*[]models.File, error) {
	var images []models.File
	var query string
	if db.Type() == "postgres" {
		query = `SELECT id, name, ext FROM product_image WHERE product_id = $1 ORDER BY created`
	} else {
		query = `SELECT id, name, ext FROM product_image WHERE product_id = ? ORDER BY created`
	}

	rows, err := db.DB().QueryContext(ctx, query, productID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var img models.File
		if err := rows.Scan(&img.ID, &img.Name, &img.Ext); err != nil {
			return nil, err
		}
		images = append(images, img)
	}

	return &images, nil
}

// AddImage adds an image to a product
func AddImage(ctx context.Context, productID, fileUUID, fileExt, fileOrigName string) (*models.File, error) {
	imageID := security.RandomString()

	var query string
	if db.Type() == "postgres" {
		query = `INSERT INTO product_image (id, product_id, name, ext) VALUES ($1, $2, $3, $4)`
	} else {
		query = `INSERT INTO product_image (id, product_id, name, ext) VALUES (?, ?, ?, ?)`
	}

	_, err := db.DB().ExecContext(ctx, query, imageID, productID, fileUUID, fileExt)
	if err != nil {
		return nil, err
	}

	return &models.File{
		ID:   imageID,
		Name: fileUUID,
		Ext:  fileExt,
	}, nil
}

// DeleteImage deletes an image from a product
func DeleteImage(ctx context.Context, productID, imageID string) error {
	var query string
	if db.Type() == "postgres" {
		query = `DELETE FROM product_image WHERE id = $1 AND product_id = $2`
	} else {
		query = `DELETE FROM product_image WHERE id = ? AND product_id = ?`
	}
	_, err := db.DB().ExecContext(ctx, query, imageID, productID)
	return err
}

// ProductDigital retrieves digital content for a product
func ProductDigital(ctx context.Context, productID string) (*models.Digital, error) {
	digital := &models.Digital{}

	// Get digital type from product
	var digitalType sql.NullString
	var query string
	if db.Type() == "postgres" {
		query = `SELECT digital FROM product WHERE id = $1`
	} else {
		query = `SELECT digital FROM product WHERE id = ?`
	}
	err := db.DB().QueryRowContext(ctx, query, productID).Scan(&digitalType)
	if err != nil {
		return nil, err
	}
	if digitalType.Valid {
		digital.Type = digitalType.String
	}

	// Get digital files
	var filesQuery string
	if db.Type() == "postgres" {
		filesQuery = `SELECT id, name, ext FROM digital_file WHERE product_id = $1`
	} else {
		filesQuery = `SELECT id, name, ext FROM digital_file WHERE product_id = ?`
	}
	rows, err := db.DB().QueryContext(ctx, filesQuery, productID)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var f models.File
			if err := rows.Scan(&f.ID, &f.Name, &f.Ext); err == nil {
				digital.Files = append(digital.Files, f)
			}
		}
	}

	// Get digital data
	var dataQuery string
	if db.Type() == "postgres" {
		dataQuery = `SELECT id, content FROM digital_data WHERE product_id = $1 AND cart_id IS NULL`
	} else {
		dataQuery = `SELECT id, content FROM digital_data WHERE product_id = ? AND cart_id IS NULL`
	}
	rows, err = db.DB().QueryContext(ctx, dataQuery, productID)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var d models.Data
			if err := rows.Scan(&d.ID, &d.Content); err == nil {
				digital.Data = append(digital.Data, d)
			}
		}
	}

	return digital, nil
}

// AddDigitalFile adds a digital file to a product
func AddDigitalFile(ctx context.Context, productID, fileUUID, fileExt, fileOrigName string) (*models.File, error) {
	fileID := security.RandomString()

	var query string
	if db.Type() == "postgres" {
		query = `INSERT INTO digital_file (id, product_id, name, ext, orig_name) VALUES ($1, $2, $3, $4, $5)`
	} else {
		query = `INSERT INTO digital_file (id, product_id, name, ext, orig_name) VALUES (?, ?, ?, ?, ?)`
	}

	_, err := db.DB().ExecContext(ctx, query, fileID, productID, fileUUID, fileExt, fileOrigName)
	if err != nil {
		return nil, err
	}

	return &models.File{
		ID:       fileID,
		Name:     fileUUID,
		Ext:      fileExt,
		OrigName: fileOrigName,
	}, nil
}

// DigitalFile retrieves a digital file by ID for a product
func DigitalFile(ctx context.Context, productID, fileID string) (*models.File, error) {
	file := &models.File{}
	var query string
	if db.Type() == "postgres" {
		query = `SELECT id, name, ext, orig_name FROM digital_file WHERE id = $1 AND product_id = $2`
	} else {
		query = `SELECT id, name, ext, orig_name FROM digital_file WHERE id = ? AND product_id = ?`
	}
	err := db.DB().QueryRowContext(ctx, query, fileID, productID).
		Scan(&file.ID, &file.Name, &file.Ext, &file.OrigName)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("digital file not found")
		}
		return nil, err
	}
	return file, nil
}

// Helper functions

func loadProductImages(ctx context.Context, product *models.Product) error {
	var query string
	if db.Type() == "postgres" {
		query = `SELECT id, name, ext FROM product_image WHERE product_id = $1 ORDER BY created`
	} else {
		query = `SELECT id, name, ext FROM product_image WHERE product_id = ? ORDER BY created`
	}

	rows, err := db.DB().QueryContext(ctx, query, product.ID)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var img models.File
		if err := rows.Scan(&img.ID, &img.Name, &img.Ext); err != nil {
			return err
		}
		product.Images = append(product.Images, img)
	}

	return nil
}

func loadProductVariants(ctx context.Context, product *models.Product) error {
	var query string
	if db.Type() == "postgres" {
		query = `SELECT id, sku, quantity, price_surcharge, option_values, active FROM product_variant WHERE product_id = $1`
	} else {
		query = `SELECT id, sku, quantity, price_surcharge, option_values, active FROM product_variant WHERE product_id = ?`
	}

	rows, err := db.DB().QueryContext(ctx, query, product.ID)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var v models.ProductVariant
		var optionValues sql.NullString

		if err := rows.Scan(&v.ID, &v.SKU, &v.Quantity, &v.PriceSurcharge, &optionValues, &v.Active); err != nil {
			return err
		}

		if optionValues.Valid {
			json.Unmarshal([]byte(optionValues.String), &v.OptionValues)
		}

		product.Variants = append(product.Variants, v)
	}

	return nil
}

// AddDigitalData adds digital data (license key) to a product
func AddDigitalData(ctx context.Context, productID, content string) (*models.Data, error) {
	file := &models.Data{
		ID:      security.RandomString(),
		Content: content,
	}

	var query string
	if db.Type() == "postgres" {
		query = `INSERT INTO digital_data (id, product_id, content) VALUES ($1, $2, $3)`
	} else {
		query = `INSERT INTO digital_data (id, product_id, content) VALUES (?, ?, ?)`
	}
	_, err := db.DB().ExecContext(ctx, query, file.ID, productID, file.Content)
	if err != nil {
		return nil, err
	}

	return file, nil
}

// UpdateDigital updates the content of a digital data record
func UpdateDigital(ctx context.Context, digital *models.Data) error {
	var query string
	if db.Type() == "postgres" {
		query = `UPDATE digital_data SET content = $1 WHERE id = $2`
	} else {
		query = `UPDATE digital_data SET content = ? WHERE id = ?`
	}
	_, err := db.DB().ExecContext(ctx, query, digital.Content, digital.ID)
	return err
}

// DeleteDigital deletes digital content (file or data) from a product
func DeleteDigital(ctx context.Context, productID, digitalID string) error {
	var digitalType string
	var name, ext sql.NullString

	var selectQuery, deleteFileQuery, deleteDataQuery string
	if db.Type() == "postgres" {
		selectQuery = `
			SELECT p.digital, df.name, df.ext
			FROM product p
			LEFT JOIN digital_file df ON df.id = $1 AND df.product_id = p.id
			WHERE p.id = $2
		`
		deleteFileQuery = `DELETE FROM digital_file WHERE id = $1 AND product_id = $2`
		deleteDataQuery = `DELETE FROM digital_data WHERE id = $1 AND product_id = $2`
	} else {
		selectQuery = `
			SELECT p.digital, df.name, df.ext
			FROM product p
			LEFT JOIN digital_file df ON df.id = ? AND df.product_id = p.id
			WHERE p.id = ?
		`
		deleteFileQuery = `DELETE FROM digital_file WHERE id = ? AND product_id = ?`
		deleteDataQuery = `DELETE FROM digital_data WHERE id = ? AND product_id = ?`
	}

	err := db.DB().QueryRowContext(ctx, selectQuery, digitalID, productID).Scan(&digitalType, &name, &ext)
	if err != nil {
		return err
	}

	switch digitalType {
	case "file":
		if _, err := db.DB().ExecContext(ctx, deleteFileQuery, digitalID, productID); err != nil {
			return fmt.Errorf("deleting from digital_file: %w", err)
		}

		filePath := fmt.Sprintf("./lc_digitals/%s.%s", name.String, ext.String)
		if err := os.Remove(filePath); err != nil {
			return fmt.Errorf("failed to remove file %s: %w", filePath, err)
		}

	case "data":
		if _, err := db.DB().ExecContext(ctx, deleteDataQuery, digitalID, productID); err != nil {
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
