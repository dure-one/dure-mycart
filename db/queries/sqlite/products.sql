-- name: GetProductByID :one
SELECT id, name, "desc", slug, amount, metadata, attribute, digital, active, deleted, created, updated
FROM product WHERE id = ? AND deleted = FALSE LIMIT 1;

-- name: GetProductBySlug :one
SELECT id, name, "desc", slug, amount, metadata, attribute, digital, active, deleted, created, updated
FROM product WHERE slug = ? AND deleted = FALSE LIMIT 1;

-- name: ListProducts :many
SELECT id, name, "desc", slug, amount, metadata, attribute, digital, active, deleted, created, updated
FROM product
WHERE deleted = FALSE
ORDER BY created DESC
LIMIT ? OFFSET ?;

-- name: ListActiveProducts :many
SELECT id, name, "desc", slug, amount, metadata, attribute, digital, active, deleted, created, updated
FROM product
WHERE active = TRUE AND deleted = FALSE
ORDER BY created DESC
LIMIT ? OFFSET ?;

-- name: CountProducts :one
SELECT COUNT(*) FROM product WHERE deleted = FALSE;

-- name: CreateProduct :one
INSERT INTO product (id, name, brief, "desc", slug, amount, metadata, attribute, digital, active, has_variants, quantity, sku, seo, created)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
RETURNING id, name, brief, "desc", slug, amount, metadata, attribute, digital, active, has_variants, quantity, sku, seo, deleted, strftime('%s', created) as created, updated;

-- name: UpdateProduct :exec
UPDATE product
SET name = ?, "desc" = ?, slug = ?, amount = ?, metadata = ?, attribute = ?, digital = ?, active = ?, updated = CURRENT_TIMESTAMP
WHERE id = ?;

-- name: UpdateProductFull :exec
UPDATE product
SET name = ?, brief = ?, "desc" = ?, slug = ?, amount = ?, quantity = ?, sku = ?,
    has_variants = ?, metadata = ?, attribute = ?, seo = ?, updated = CURRENT_TIMESTAMP
WHERE id = ?;

-- name: UpdateProductActive :exec
UPDATE product
SET active = NOT active, updated = CURRENT_TIMESTAMP
WHERE id = ?;

-- name: ProductHasSoldDigitalData :one
SELECT EXISTS(
	SELECT 1 FROM digital_data dd
	INNER JOIN cart c ON dd.cart_id = c.id
	WHERE dd.product_id = ? AND c.payment_status = 'paid'
);

-- name: SoftDeleteProduct :exec
UPDATE product
SET deleted = TRUE, updated = CURRENT_TIMESTAMP
WHERE id = ?;

-- name: DeleteProduct :exec
DELETE FROM product WHERE id = ?;

-- name: ProductExists :one
SELECT EXISTS(SELECT 1 FROM product WHERE slug = ? AND deleted = FALSE);

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
WHERE p.id = ?;

-- name: CreateProductVariant :one
INSERT INTO product_variant (id, product_id, sku, price_surcharge, quantity, option_values)
VALUES (?, ?, ?, ?, ?, ?)
RETURNING id, product_id, sku, price_surcharge, quantity, option_values, active, deleted, created, updated;

-- name: UpdateProductVariant :exec
UPDATE product_variant
SET sku = ?, price_surcharge = ?, quantity = ?, option_values = ?, updated = CURRENT_TIMESTAMP
WHERE id = ?;

-- name: DeleteProductVariant :exec
DELETE FROM product_variant WHERE id = ?;

-- name: ListProductVariantsByProduct :many
SELECT id, product_id, sku, price_surcharge, quantity, option_values, active, deleted, created, updated
FROM product_variant
WHERE product_id = ?;

-- name: GetProductOption :one
SELECT id, name, product_id, position, created
FROM product_option
WHERE id = ? LIMIT 1;

-- name: CreateProductOption :one
INSERT INTO product_option (id, name, product_id, position)
VALUES (?, ?, ?, ?)
RETURNING id, name, product_id, position, created;

-- name: DeleteProductOption :exec
DELETE FROM product_option WHERE id = ?;

-- name: ListProductOptionsByProduct :many
SELECT id, name, product_id, position, created
FROM product_option
WHERE product_id = ?
ORDER BY position;

-- name: CreateProductOptionValue :one
INSERT INTO product_option_value (id, option_id, value, position)
VALUES (?, ?, ?, ?)
RETURNING id, option_id, value, position;

-- name: ListProductOptionValuesByOption :many
SELECT id, option_id, value, position
FROM product_option_value
WHERE option_id = ?
ORDER BY position;

-- name: DeleteProductOptionValue :exec
DELETE FROM product_option_value WHERE id = ?;

-- Advanced Product Queries

-- name: BulkDeleteProductImages :exec
DELETE FROM product_image
WHERE product_id = ?;

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

-- name: ListAllProducts :many
SELECT id, name, "desc", slug, amount, metadata, attribute, digital, active, deleted, created, updated
FROM product ORDER BY created;

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
  (SELECT json_group_array(json_object('id', product_image.id, 'name', product_image.name, 'ext', product_image.ext))
   FROM product_image WHERE product_id = p.id GROUP BY id LIMIT 1) as image,
  (SELECT json_group_array(json_object('id', pv.id, 'sku', pv.sku, 'quantity', pv.quantity, 'price_surcharge', pv.price_surcharge, 'option_values', json(pv.option_values), 'active', CASE WHEN pv.active = 1 THEN json('true') ELSE json('false') END))
   FROM product_variant pv WHERE pv.product_id = p.id) as variants,
  strftime('%s', p.created) as created
FROM product p
WHERE p.deleted = 0
LIMIT ? OFFSET ?;

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
  (SELECT json_group_array(json_object('id', product_image.id, 'name', product_image.name, 'ext', product_image.ext))
   FROM product_image WHERE product_id = p.id GROUP BY id LIMIT 1) as image,
  (SELECT json_group_array(json_object('id', pv.id, 'sku', pv.sku, 'quantity', pv.quantity, 'price_surcharge', pv.price_surcharge, 'option_values', json(pv.option_values), 'active', CASE WHEN pv.active = 1 THEN json('true') ELSE json('false') END))
   FROM product_variant pv WHERE pv.product_id = p.id AND pv.active = 1) as variants,
  strftime('%s', p.created) as created
FROM product p
WHERE p.deleted = 0 AND p.active = 1
LIMIT ? OFFSET ?;

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
  json_group_array(json_object('id', pi.id, 'name', pi.name, 'ext', pi.ext)) as images,
  EXISTS(SELECT 1 FROM digital_data WHERE digital_data.product_id = p.id AND digital_data.cart_id IS NULL) OR
  EXISTS(SELECT 1 FROM digital_file WHERE digital_file.product_id = p.id) AS digital_filled,
  strftime('%s', p.created) as created,
  strftime('%s', p.updated) as updated
FROM product p
LEFT JOIN product_image pi ON p.id = pi.product_id
WHERE p.id = ?
GROUP BY p.id;

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
  json_group_array(json_object('id', pi.id, 'name', pi.name, 'ext', pi.ext)) as images,
  strftime('%s', p.created) as created,
  strftime('%s', p.updated) as updated
FROM product p
LEFT JOIN product_image pi ON p.id = pi.product_id
WHERE p.slug = ? AND p.deleted = 0 AND p.active = 1
GROUP BY p.id;

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
WHERE p.id = ?;
