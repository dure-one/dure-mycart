-- name: GetProductByID :one
SELECT id, name, "desc", slug, amount, metadata, attribute, digital, active, deleted, created, updated
FROM product WHERE id = $1 AND deleted = FALSE LIMIT 1;

-- name: GetProductBySlug :one
SELECT id, name, "desc", slug, amount, metadata, attribute, digital, active, deleted, created, updated
FROM product WHERE slug = $1 AND deleted = FALSE LIMIT 1;

-- name: ListProducts :many
SELECT id, name, "desc", slug, amount, metadata, attribute, digital, active, deleted, created, updated
FROM product
WHERE deleted = FALSE
ORDER BY created DESC
LIMIT $1 OFFSET $2;

-- name: ListActiveProducts :many
SELECT id, name, "desc", slug, amount, metadata, attribute, digital, active, deleted, created, updated
FROM product
WHERE active = TRUE AND deleted = FALSE
ORDER BY created DESC
LIMIT $1 OFFSET $2;

-- name: CountProducts :one
SELECT COUNT(*) FROM product WHERE deleted = FALSE;

-- name: CreateProduct :one
INSERT INTO product (id, name, brief, "desc", slug, amount, metadata, attribute, digital, active, has_variants, quantity, sku, seo, created)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, NOW())
RETURNING id, name, brief, "desc", slug, amount, metadata, attribute, digital, active, has_variants, quantity, sku, seo, deleted, EXTRACT(EPOCH FROM created)::bigint as created, updated;

-- name: UpdateProduct :exec
UPDATE product
SET name = $1, "desc" = $2, slug = $3, amount = $4, metadata = $5, attribute = $6, digital = $7, active = $8, updated = NOW()
WHERE id = $9;

-- name: UpdateProductFull :exec
UPDATE product
SET name = $1, brief = $2, "desc" = $3, slug = $4, amount = $5, quantity = $6, sku = $7,
    has_variants = $8, metadata = $9, attribute = $10, seo = $11, updated = NOW()
WHERE id = $12;

-- name: UpdateProductActive :exec
UPDATE product
SET active = NOT active, updated = NOW()
WHERE id = $1;

-- name: ProductHasSoldDigitalData :one
SELECT EXISTS(
	SELECT 1 FROM digital_data dd
	INNER JOIN cart c ON dd.cart_id = c.id
	WHERE dd.product_id = $1 AND c.payment_status = 'paid'
);

-- name: SoftDeleteProduct :exec
UPDATE product
SET deleted = TRUE, updated = NOW()
WHERE id = $1;

-- name: DeleteProduct :exec
DELETE FROM product WHERE id = $1;

-- name: ProductExists :one
SELECT EXISTS(SELECT 1 FROM product WHERE slug = $1 AND deleted = FALSE);

-- name: CheckSlugExists :one
SELECT COUNT(*) FROM product WHERE slug = $1 AND id != $2 AND deleted = FALSE;

-- name: ListAllProducts :many
SELECT id, name, "desc", slug, amount, metadata, attribute, digital, active, deleted, created, updated
FROM product ORDER BY created;

-- Product Variant Queries

-- name: GetProductWithVariants :many
SELECT
    p.id as product_id,
    p.name as product_name,
    pv.id as variant_id,
    pv.sku as variant_sku,
    pv.price_surcharge as variant_price_surcharge,
    pv.quantity as variant_quantity,
    pv.option_values as variant_option_values
FROM product p
LEFT JOIN product_variant pv ON p.id = pv.product_id
WHERE p.id = $1;

-- name: CreateProductVariant :one
INSERT INTO product_variant (id, product_id, sku, price_surcharge, quantity, option_values)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING id, product_id, sku, price_surcharge, quantity, option_values, active, deleted, created, updated;

-- name: UpdateProductVariant :exec
UPDATE product_variant
SET sku = $2, price_surcharge = $3, quantity = $4, option_values = $5, updated = NOW()
WHERE id = $1;

-- name: DeleteProductVariant :exec
DELETE FROM product_variant WHERE id = $1;

-- name: ListProductVariantsByProduct :many
SELECT id, product_id, sku, price_surcharge, quantity, option_values, active, deleted, created, updated
FROM product_variant
WHERE product_id = $1;

-- name: GetProductOption :one
SELECT id, name, product_id, position, created
FROM product_option
WHERE id = $1 LIMIT 1;

-- name: CreateProductOption :one
INSERT INTO product_option (id, name, product_id, position)
VALUES ($1, $2, $3, $4)
RETURNING id, name, product_id, position, created;

-- name: DeleteProductOption :exec
DELETE FROM product_option WHERE id = $1;

-- name: ListProductOptionsByProduct :many
SELECT id, name, product_id, position, created
FROM product_option
WHERE product_id = $1
ORDER BY position;

-- name: CreateProductOptionValue :one
INSERT INTO product_option_value (id, option_id, value, position)
VALUES ($1, $2, $3, $4)
RETURNING id, option_id, value, position;

-- name: ListProductOptionValuesByOption :many
SELECT id, option_id, value, position
FROM product_option_value
WHERE option_id = $1
ORDER BY position;

-- name: DeleteProductOptionValue :exec
DELETE FROM product_option_value WHERE id = $1;

-- Advanced Product Queries

-- name: BulkDeleteProductImages :exec
DELETE FROM product_image
WHERE product_id = $1;

-- name: GetProductsWithImages :many
SELECT
    p.id as product_id,
    p.name as product_name,
    pi.id as image_id,
    pi.name as image_name,
    pi.ext as image_ext
FROM product p
LEFT JOIN product_image pi ON p.id = pi.product_id
WHERE p.active = TRUE AND p.deleted = FALSE
ORDER BY p.created DESC;

-- name: ListProductsPrivate :many
SELECT DISTINCT
  p.id,
  p.name,
  p.brief,
  p.slug,
  p.amount,
  p.quantity,
  p.has_variants,
  p.active,
  p.digital,
  EXISTS(SELECT 1 FROM digital_data WHERE digital_data.product_id = p.id AND digital_data.cart_id IS NULL) OR
  EXISTS(SELECT 1 FROM digital_file WHERE digital_file.product_id = p.id) AS digital_filled,
  COALESCE((SELECT jsonb_agg(jsonb_build_object('id', product_image.id, 'name', product_image.name, 'ext', product_image.ext))
   FROM product_image WHERE product_id = p.id LIMIT 1), '[]'::jsonb) as image,
  COALESCE((SELECT jsonb_agg(jsonb_build_object('id', pv.id, 'sku', pv.sku, 'quantity', pv.quantity, 'price_surcharge', pv.price_surcharge, 'option_values', pv.option_values::jsonb, 'active', CASE WHEN pv.active THEN 'true'::jsonb ELSE 'false'::jsonb END))
   FROM product_variant pv WHERE pv.product_id = p.id), '[]'::jsonb) as variants,
  EXTRACT(EPOCH FROM p.created)::bigint as created
FROM product p
WHERE p.deleted = FALSE
LIMIT $1 OFFSET $2;

-- name: ListProductsPublic :many
SELECT DISTINCT
  p.id,
  p.name,
  p.brief,
  p.slug,
  p.amount,
  p.quantity,
  p.has_variants,
  p.active,
  p.digital,
  EXISTS(SELECT 1 FROM digital_data WHERE digital_data.product_id = p.id AND digital_data.cart_id IS NULL) OR
  EXISTS(SELECT 1 FROM digital_file WHERE digital_file.product_id = p.id) AS digital_filled,
  COALESCE((SELECT jsonb_agg(jsonb_build_object('id', product_image.id, 'name', product_image.name, 'ext', product_image.ext))
   FROM product_image WHERE product_id = p.id LIMIT 1), '[]'::jsonb) as image,
  COALESCE((SELECT jsonb_agg(jsonb_build_object('id', pv.id, 'sku', pv.sku, 'quantity', pv.quantity, 'price_surcharge', pv.price_surcharge, 'option_values', pv.option_values::jsonb, 'active', CASE WHEN pv.active THEN 'true'::jsonb ELSE 'false'::jsonb END))
   FROM product_variant pv WHERE pv.product_id = p.id AND pv.active = TRUE), '[]'::jsonb) as variants,
  EXTRACT(EPOCH FROM p.created)::bigint as created
FROM product p
WHERE p.deleted = FALSE AND p.active = TRUE
LIMIT $1 OFFSET $2;

-- name: GetProductDetailByID :one
SELECT DISTINCT
  p.id,
  p.name,
  p.brief,
  p.desc,
  p.slug,
  p.amount,
  p.quantity,
  p.sku,
  p.has_variants,
  p.active,
  p.metadata,
  p.attribute,
  p.digital,
  p.seo,
  jsonb_agg(jsonb_build_object('id', pi.id, 'name', pi.name, 'ext', pi.ext)) as images,
  EXISTS(SELECT 1 FROM digital_data WHERE digital_data.product_id = p.id AND digital_data.cart_id IS NULL) OR
  EXISTS(SELECT 1 FROM digital_file WHERE digital_file.product_id = p.id) AS digital_filled,
  EXTRACT(EPOCH FROM p.created)::bigint as created,
  EXTRACT(EPOCH FROM p.updated)::bigint as updated
FROM product p
LEFT JOIN product_image pi ON p.id = pi.product_id
WHERE p.id = $1
GROUP BY p.id, p.name, p.brief, p.desc, p.slug, p.amount, p.quantity, p.sku, p.has_variants, p.active, p.metadata, p.attribute, p.digital, p.seo, p.created, p.updated;

-- name: GetProductDetailBySlug :one
SELECT DISTINCT
  p.id,
  p.name,
  p.brief,
  p.desc,
  p.slug,
  p.amount,
  p.quantity,
  p.sku,
  p.has_variants,
  p.active,
  p.metadata,
  p.attribute,
  p.digital,
  p.seo,
  jsonb_agg(jsonb_build_object('id', pi.id, 'name', pi.name, 'ext', pi.ext)) as images,
  EXTRACT(EPOCH FROM p.created)::bigint as created,
  EXTRACT(EPOCH FROM p.updated)::bigint as updated
FROM product p
LEFT JOIN product_image pi ON p.id = pi.product_id
WHERE p.slug = $1 AND p.deleted = FALSE AND p.active = TRUE
GROUP BY p.id, p.name, p.brief, p.desc, p.slug, p.amount, p.quantity, p.sku, p.has_variants, p.active, p.metadata, p.attribute, p.digital, p.seo, p.created, p.updated;

-- name: GetProductDigitalContent :many
SELECT
    p.digital,
    df.id as file_id,
    df.name as file_name,
    df.ext as file_ext,
    dd.id as data_id,
    dd.content as data_content,
    dd.cart_id as data_cart_id
FROM product p
LEFT JOIN digital_file df ON p.id = df.product_id
LEFT JOIN digital_data dd ON p.id = dd.product_id
WHERE p.id = $1;
