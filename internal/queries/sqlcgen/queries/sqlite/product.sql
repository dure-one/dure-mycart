-- Aggregated JSON columns are COALESCEd to '' and read as "NULL" by the Go
-- side. On SQLite json_group_array over no rows already yields '[]', exactly as
-- the plain build sees it.
--
-- ID lists in queries with other parameters travel as a JSON array through
-- json_each: sqlc numbers repeated parameters (?1, ?2), and a sqlc.slice
-- expanded among numbered parameters binds the wrong values.

-- name: CountSoldDigitalData :one
SELECT COUNT(1) FROM digital_data WHERE product_id = sqlc.arg(id) AND cart_id IS NOT NULL;

-- name: DeleteProductRow :exec
DELETE FROM product WHERE id = sqlc.arg(id);

-- name: ActiveProductExists :one
SELECT CAST(EXISTS (
    SELECT 1 FROM product
    WHERE product.slug = sqlc.arg(slug) AND product.deleted = FALSE AND product.active = TRUE
) AS BOOLEAN) AS product_exists;

-- name: SlugTaken :one
SELECT COUNT(*) FROM product WHERE slug = sqlc.arg(slug) AND id != sqlc.arg(exclude_id);

-- name: ToggleProductActive :exec
UPDATE product SET active = NOT active, updated = CURRENT_TIMESTAMP WHERE id = sqlc.arg(id);

-- name: ListProducts :many
SELECT
    product.id,
    product.name,
    product.brief,
    product.slug,
    CAST(product.amount AS INTEGER) AS amount,
    CAST(COALESCE(product.quantity, 0) AS INTEGER) AS quantity,
    CAST(COALESCE(product.has_variants, FALSE) AS BOOLEAN) AS has_variants,
    product.active,
    product.digital,
    CAST((EXISTS(SELECT 1 FROM digital_data WHERE digital_data.product_id = product.id AND digital_data.cart_id IS NULL) OR
          EXISTS(SELECT 1 FROM digital_file WHERE digital_file.product_id = product.id)) AS BOOLEAN) AS digital_filled,
    CAST(COALESCE((SELECT json_group_array(json_object(
        'id', pi.id, 'name', pi.name, 'ext', pi.ext, 'position', pi.position,
        'is_representative', CASE WHEN pi.is_representative = 1 THEN json('true') ELSE json('false') END))
        FROM (SELECT * FROM product_image WHERE product_id = product.id ORDER BY position, id) pi), '') AS TEXT) AS image,
    CAST(COALESCE((SELECT json_group_array(json_object(
        'id', product_variant.id, 'sku', product_variant.sku, 'quantity', product_variant.quantity,
        'price_surcharge', product_variant.price_surcharge,
        'option_values', json(product_variant.option_values),
        'active', CASE WHEN product_variant.active = 1 THEN json('true') ELSE json('false') END))
        FROM product_variant
        WHERE product_id = product.id AND (CAST(sqlc.arg(private) AS BOOLEAN) OR active = TRUE)), '') AS TEXT) AS variants,
    CAST(COALESCE(strftime('%s', product.created), 0) AS INTEGER) AS created
FROM product
WHERE product.deleted = FALSE
  AND (CAST(sqlc.arg(private) AS BOOLEAN)
       OR product.active = TRUE
       OR (CAST(sqlc.arg(cart_id) AS TEXT) <> '' AND EXISTS (
            SELECT 1 FROM digital_data
            WHERE digital_data.product_id = product.id AND digital_data.cart_id = CAST(sqlc.arg(cart_id) AS TEXT))))
  AND (CAST(sqlc.arg(all_ids) AS BOOLEAN) OR product.id IN (SELECT value FROM json_each(CAST(sqlc.arg(ids) AS TEXT))))
ORDER BY product.created DESC, product.id
LIMIT CAST(sqlc.arg(lim) AS INTEGER) OFFSET CAST(sqlc.arg(off) AS INTEGER);

-- name: CountListedProducts :one
SELECT COUNT(DISTINCT product.id) FROM product
WHERE product.deleted = FALSE
  AND (CAST(sqlc.arg(private) AS BOOLEAN)
       OR product.active = TRUE
       OR (CAST(sqlc.arg(cart_id) AS TEXT) <> '' AND EXISTS (
            SELECT 1 FROM digital_data
            WHERE digital_data.product_id = product.id AND digital_data.cart_id = CAST(sqlc.arg(cart_id) AS TEXT))))
  AND (CAST(sqlc.arg(all_ids) AS BOOLEAN) OR product.id IN (SELECT value FROM json_each(CAST(sqlc.arg(ids) AS TEXT))));

-- name: ProductDetailByID :one
SELECT
    product.id, product.name, product.brief, product."desc", product.slug,
    CAST(product.amount AS INTEGER) AS amount,
    CAST(COALESCE(product.quantity, 0) AS INTEGER) AS quantity,
    product.sku,
    CAST(COALESCE(product.has_variants, FALSE) AS BOOLEAN) AS has_variants,
    product.active, product.metadata, product.attribute, product.digital, product.seo,
    CAST(COALESCE((SELECT json_group_array(json_object(
        'id', pi.id, 'name', pi.name, 'ext', pi.ext, 'position', pi.position,
        'is_representative', CASE WHEN pi.is_representative = 1 THEN json('true') ELSE json('false') END))
        FROM (SELECT * FROM product_image WHERE product_id = product.id ORDER BY position, id) pi), '') AS TEXT) AS images,
    CAST(COALESCE(strftime('%s', product.created), 0) AS INTEGER) AS created,
    CAST(COALESCE(strftime('%s', product.updated), 0) AS INTEGER) AS updated,
    CAST((EXISTS(SELECT 1 FROM digital_data WHERE digital_data.product_id = product.id AND digital_data.cart_id IS NULL) OR
          EXISTS(SELECT 1 FROM digital_file WHERE digital_file.product_id = product.id)) AS BOOLEAN) AS digital_filled
FROM product
WHERE product.id = sqlc.arg(id);

-- name: ProductDetailBySlug :one
SELECT
    product.id, product.name, product.brief, product."desc", product.slug,
    CAST(product.amount AS INTEGER) AS amount,
    CAST(COALESCE(product.quantity, 0) AS INTEGER) AS quantity,
    product.sku,
    CAST(COALESCE(product.has_variants, FALSE) AS BOOLEAN) AS has_variants,
    product.active, product.metadata, product.attribute, product.digital, product.seo,
    CAST(COALESCE((SELECT json_group_array(json_object(
        'id', pi.id, 'name', pi.name, 'ext', pi.ext, 'position', pi.position,
        'is_representative', CASE WHEN pi.is_representative = 1 THEN json('true') ELSE json('false') END))
        FROM (SELECT * FROM product_image WHERE product_id = product.id ORDER BY position, id) pi), '') AS TEXT) AS images,
    CAST(COALESCE(strftime('%s', product.created), 0) AS INTEGER) AS created,
    CAST(COALESCE(strftime('%s', product.updated), 0) AS INTEGER) AS updated,
    CAST(FALSE AS BOOLEAN) AS digital_filled
FROM product
WHERE product.slug = sqlc.arg(slug) AND product.deleted = FALSE AND product.active = TRUE;

-- name: OptionsByProduct :many
SELECT id, name, CAST(COALESCE(position, 0) AS INTEGER) AS position
FROM product_option WHERE product_id = sqlc.arg(product_id) ORDER BY product_option.position;

-- name: OptionValuesByProduct :many
SELECT id, option_id, value, CAST(COALESCE(position, 0) AS INTEGER) AS position
FROM product_option_value
WHERE option_id IN (SELECT po.id FROM product_option po WHERE po.product_id = sqlc.arg(product_id))
ORDER BY option_id, product_option_value.position;

-- name: OptionValuesByOption :many
SELECT id, value, CAST(COALESCE(position, 0) AS INTEGER) AS position
FROM product_option_value WHERE option_id = sqlc.arg(option_id) ORDER BY product_option_value.position;

-- name: LiveVariantsByProduct :many
SELECT id, sku,
       CAST(COALESCE(price_surcharge, 0) AS INTEGER) AS price_surcharge,
       CAST(COALESCE(quantity, 0) AS INTEGER) AS quantity,
       option_values,
       CAST(COALESCE(active, FALSE) AS BOOLEAN) AS active
FROM product_variant WHERE product_id = sqlc.arg(product_id) AND deleted = FALSE;

-- name: VariantsByProduct :many
SELECT id, sku,
       CAST(COALESCE(price_surcharge, 0) AS INTEGER) AS price_surcharge,
       CAST(COALESCE(quantity, 0) AS INTEGER) AS quantity,
       CAST(COALESCE(active, FALSE) AS BOOLEAN) AS active
FROM product_variant WHERE product_id = sqlc.arg(product_id);

-- name: ProductBase :one
SELECT id, name, brief, "desc", slug,
       CAST(amount AS INTEGER) AS amount,
       CAST(COALESCE(quantity, 0) AS INTEGER) AS quantity,
       sku,
       CAST(COALESCE(has_variants, FALSE) AS BOOLEAN) AS has_variants,
       metadata, attribute, digital, active
FROM product WHERE id = sqlc.arg(id) AND deleted = FALSE;

-- name: ProductImagesOrdered :many
SELECT id, name, ext, orig_name FROM product_image
WHERE product_id = sqlc.arg(product_id) ORDER BY position;

-- name: VariantOptionPairs :many
SELECT po.name, pov.value
FROM product_variant_option pvo
JOIN product_option_value pov ON pvo.option_value_id = pov.id
JOIN product_option po ON pov.option_id = po.id
WHERE pvo.variant_id = sqlc.arg(variant_id);

-- name: VariantImages :many
SELECT id, name, ext, orig_name FROM product_variant_image
WHERE variant_id = sqlc.arg(variant_id) ORDER BY position;

-- name: ProductDigitalRows :many
SELECT p.digital,
       df.id AS file_id, df.name AS file_name, df.ext AS file_ext,
       dd.id AS data_id, dd.content AS data_content, dd.cart_id AS data_cart_id
FROM product p
LEFT JOIN digital_file df ON p.id = df.product_id
LEFT JOIN digital_data dd ON p.id = dd.product_id
WHERE p.id = sqlc.arg(product_id);

-- name: DigitalFileOfProduct :one
SELECT id, name, ext, orig_name FROM digital_file WHERE id = sqlc.arg(id) AND product_id = sqlc.arg(product_id);

-- name: InsertDigitalFile :exec
INSERT INTO digital_file (id, product_id, name, ext, orig_name)
VALUES (sqlc.arg(id), sqlc.arg(product_id), sqlc.arg(name), sqlc.arg(ext), sqlc.arg(orig_name));

-- name: InsertDigitalData :exec
INSERT INTO digital_data (id, product_id, content) VALUES (sqlc.arg(id), sqlc.arg(product_id), sqlc.arg(content));

-- name: UpdateDigitalContent :exec
UPDATE digital_data SET content = sqlc.arg(content) WHERE id = sqlc.arg(id);

-- name: DigitalKindAndFile :one
SELECT p.digital, df.name AS file_name, df.ext AS file_ext
FROM product p
LEFT JOIN digital_file df ON df.id = sqlc.arg(digital_id) AND df.product_id = p.id
WHERE p.id = sqlc.arg(product_id);

-- name: DeleteDigitalFile :exec
DELETE FROM digital_file WHERE id = sqlc.arg(id) AND product_id = sqlc.arg(product_id);

-- name: DeleteDigitalData :exec
DELETE FROM digital_data WHERE id = sqlc.arg(id) AND product_id = sqlc.arg(product_id);

-- name: ProductImageRows :many
SELECT id, name, ext FROM product_image WHERE product_id = sqlc.arg(product_id);

-- name: InsertProductImage :exec
INSERT INTO product_image (id, product_id, name, ext, orig_name)
VALUES (sqlc.arg(id), sqlc.arg(product_id), sqlc.arg(name), sqlc.arg(ext), sqlc.arg(orig_name));

-- name: ImageNameExt :one
SELECT name, ext FROM product_image WHERE id = sqlc.arg(id);

-- name: DeleteProductImage :exec
DELETE FROM product_image WHERE id = sqlc.arg(id) AND product_id = sqlc.arg(product_id);

-- name: SetImagePosition :execrows
UPDATE product_image SET position = CAST(sqlc.arg(position) AS INTEGER) WHERE id = sqlc.arg(id);

-- name: ImageProduct :one
SELECT product_id FROM product_image WHERE id = sqlc.arg(id);

-- name: ClearRepresentative :exec
UPDATE product_image SET is_representative = FALSE WHERE product_id = sqlc.arg(product_id);

-- name: MarkRepresentative :exec
UPDATE product_image SET is_representative = TRUE WHERE id = sqlc.arg(id);

-- name: InsertProductFull :exec
INSERT INTO product (
    id, name, brief, "desc", slug, amount, quantity, sku,
    has_variants, metadata, attribute, seo, digital, active
) VALUES (
    sqlc.arg(id), sqlc.arg(name), sqlc.arg(brief), sqlc.arg(description), sqlc.arg(slug),
    CAST(sqlc.arg(amount) AS INTEGER), CAST(sqlc.arg(quantity) AS INTEGER), sqlc.narg(sku),
    CAST(sqlc.arg(has_variants) AS BOOLEAN), sqlc.arg(metadata), sqlc.arg(attribute), sqlc.arg(seo),
    sqlc.narg(digital), CAST(sqlc.arg(active) AS BOOLEAN)
);

-- name: InsertProduct :one
INSERT INTO product (
    id, name, amount, slug, metadata, attribute, brief, "desc", digital,
    quantity, sku, seo, active
) VALUES (
    sqlc.arg(id), sqlc.arg(name), CAST(sqlc.arg(amount) AS INTEGER), sqlc.arg(slug),
    sqlc.arg(metadata), sqlc.arg(attribute), sqlc.arg(brief), sqlc.arg(description), sqlc.narg(digital),
    CAST(sqlc.arg(quantity) AS INTEGER), sqlc.narg(sku), sqlc.arg(seo), CAST(sqlc.arg(active) AS BOOLEAN)
)
RETURNING CAST(COALESCE(strftime('%s', created), 0) AS INTEGER) AS created;

-- name: UpdateProductFields :exec
UPDATE product SET
    name = sqlc.arg(name),
    brief = sqlc.arg(brief),
    "desc" = sqlc.arg(description),
    slug = sqlc.arg(slug),
    amount = CAST(sqlc.arg(amount) AS INTEGER),
    quantity = CAST(sqlc.arg(quantity) AS INTEGER),
    sku = sqlc.narg(sku),
    has_variants = CAST(sqlc.arg(has_variants) AS BOOLEAN),
    metadata = sqlc.arg(metadata),
    attribute = sqlc.arg(attribute),
    seo = sqlc.arg(seo),
    updated = CURRENT_TIMESTAMP
WHERE id = sqlc.arg(id);

-- name: DeleteProductOptions :exec
DELETE FROM product_option WHERE product_id = sqlc.arg(product_id);

-- name: DeleteProductVariants :exec
DELETE FROM product_variant WHERE product_id = sqlc.arg(product_id);

-- name: InsertProductImageAt :exec
INSERT INTO product_image (id, product_id, name, ext, orig_name, position)
VALUES (sqlc.arg(id), sqlc.arg(product_id), sqlc.arg(name), sqlc.arg(ext), sqlc.arg(orig_name), CAST(sqlc.arg(position) AS INTEGER));

-- name: InsertProductOption :exec
INSERT INTO product_option (id, product_id, name, position)
VALUES (sqlc.arg(id), sqlc.arg(product_id), sqlc.arg(name), CAST(sqlc.arg(position) AS INTEGER));

-- name: InsertOptionValue :exec
INSERT INTO product_option_value (id, option_id, value, position)
VALUES (sqlc.arg(id), sqlc.arg(option_id), sqlc.arg(value), CAST(sqlc.arg(position) AS INTEGER));

-- name: InsertVariant :exec
INSERT INTO product_variant (id, product_id, sku, price_surcharge, quantity, option_values, active)
VALUES (sqlc.arg(id), sqlc.arg(product_id), sqlc.narg(sku), CAST(sqlc.arg(price_surcharge) AS INTEGER),
        CAST(sqlc.arg(quantity) AS INTEGER), sqlc.arg(option_values), CAST(sqlc.arg(active) AS BOOLEAN));

-- name: FindOptionValueID :one
SELECT pov.id
FROM product_option_value pov
JOIN product_option po ON pov.option_id = po.id
WHERE po.product_id = sqlc.arg(product_id) AND po.name = sqlc.arg(option_name) AND pov.value = sqlc.arg(option_value);

-- name: InsertVariantOption :exec
INSERT INTO product_variant_option (variant_id, option_value_id) VALUES (sqlc.arg(variant_id), sqlc.arg(option_value_id));

-- name: InsertVariantImage :exec
INSERT INTO product_variant_image (id, variant_id, name, ext, orig_name, position)
VALUES (sqlc.arg(id), sqlc.arg(variant_id), sqlc.arg(name), sqlc.arg(ext), sqlc.arg(orig_name), CAST(sqlc.arg(position) AS INTEGER));
