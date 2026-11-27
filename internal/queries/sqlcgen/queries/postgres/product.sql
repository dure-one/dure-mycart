-- Aggregated JSON columns are COALESCEd to '' and read as "NULL" by the Go
-- side: json_agg over no rows yields NULL here, exactly as the plain build
-- sees it. ID lists travel as a JSON array; see setting.sql.

-- name: CountSoldDigitalData :one
SELECT COUNT(1) FROM digital_data WHERE product_id = sqlc.arg(id) AND cart_id IS NOT NULL;

-- name: DeleteProductRow :exec
DELETE FROM product WHERE id = sqlc.arg(id);

-- name: ActiveProductExists :one
SELECT EXISTS (
    SELECT 1 FROM product
    WHERE product.slug = sqlc.arg(slug) AND product.deleted = FALSE AND product.active = TRUE
)::boolean AS product_exists;

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
    product.amount::bigint AS amount,
    COALESCE(product.quantity, 0)::bigint AS quantity,
    COALESCE(product.has_variants, FALSE)::boolean AS has_variants,
    product.active,
    product.digital,
    (EXISTS(SELECT 1 FROM digital_data WHERE digital_data.product_id = product.id AND digital_data.cart_id IS NULL) OR
     EXISTS(SELECT 1 FROM digital_file WHERE digital_file.product_id = product.id))::boolean AS digital_filled,
    COALESCE((SELECT json_agg(json_build_object(
        'id', pi.id, 'name', pi.name, 'ext', pi.ext, 'position', pi.position,
        'is_representative', pi.is_representative))
        FROM (SELECT * FROM product_image WHERE product_id = product.id ORDER BY position, id) pi)::text, '')::text AS image,
    COALESCE((SELECT json_agg(json_build_object(
        'id', product_variant.id, 'sku', product_variant.sku, 'quantity', product_variant.quantity,
        'price_surcharge', product_variant.price_surcharge,
        'option_values', (product_variant.option_values)::json,
        'active', product_variant.active))
        FROM product_variant
        WHERE product_id = product.id AND (sqlc.arg(private)::boolean OR active = TRUE))::text, '')::text AS variants,
    COALESCE(EXTRACT(EPOCH FROM product.created)::bigint, 0)::bigint AS created
FROM product
WHERE product.deleted = FALSE
  AND (sqlc.arg(private)::boolean
       OR product.active = TRUE
       OR (sqlc.arg(cart_id)::text <> '' AND EXISTS (
            SELECT 1 FROM digital_data
            WHERE digital_data.product_id = product.id AND digital_data.cart_id = sqlc.arg(cart_id)::text)))
  AND (sqlc.arg(all_ids)::boolean OR product.id IN (SELECT jsonb_array_elements_text(sqlc.arg(ids)::jsonb)))
ORDER BY product.created DESC, product.id
LIMIT sqlc.narg(lim)::bigint OFFSET sqlc.arg(off)::bigint;

-- name: CountListedProducts :one
SELECT COUNT(DISTINCT product.id) FROM product
WHERE product.deleted = FALSE
  AND (sqlc.arg(private)::boolean
       OR product.active = TRUE
       OR (sqlc.arg(cart_id)::text <> '' AND EXISTS (
            SELECT 1 FROM digital_data
            WHERE digital_data.product_id = product.id AND digital_data.cart_id = sqlc.arg(cart_id)::text)))
  AND (sqlc.arg(all_ids)::boolean OR product.id IN (SELECT jsonb_array_elements_text(sqlc.arg(ids)::jsonb)));

-- name: ProductDetailByID :one
SELECT
    product.id, product.name, product.brief, product."desc", product.slug,
    product.amount::bigint AS amount,
    COALESCE(product.quantity, 0)::bigint AS quantity,
    product.sku,
    COALESCE(product.has_variants, FALSE)::boolean AS has_variants,
    product.active, product.metadata, product.attribute, product.digital, product.seo,
    COALESCE((SELECT json_agg(json_build_object(
        'id', pi.id, 'name', pi.name, 'ext', pi.ext, 'position', pi.position,
        'is_representative', pi.is_representative))
        FROM (SELECT * FROM product_image WHERE product_id = product.id ORDER BY position, id) pi)::text, '')::text AS images,
    COALESCE(EXTRACT(EPOCH FROM product.created)::bigint, 0)::bigint AS created,
    COALESCE(EXTRACT(EPOCH FROM product.updated)::bigint, 0)::bigint AS updated,
    (EXISTS(SELECT 1 FROM digital_data WHERE digital_data.product_id = product.id AND digital_data.cart_id IS NULL) OR
     EXISTS(SELECT 1 FROM digital_file WHERE digital_file.product_id = product.id))::boolean AS digital_filled
FROM product
WHERE product.id = sqlc.arg(id);

-- name: ProductDetailBySlug :one
SELECT
    product.id, product.name, product.brief, product."desc", product.slug,
    product.amount::bigint AS amount,
    COALESCE(product.quantity, 0)::bigint AS quantity,
    product.sku,
    COALESCE(product.has_variants, FALSE)::boolean AS has_variants,
    product.active, product.metadata, product.attribute, product.digital, product.seo,
    COALESCE((SELECT json_agg(json_build_object(
        'id', pi.id, 'name', pi.name, 'ext', pi.ext, 'position', pi.position,
        'is_representative', pi.is_representative))
        FROM (SELECT * FROM product_image WHERE product_id = product.id ORDER BY position, id) pi)::text, '')::text AS images,
    COALESCE(EXTRACT(EPOCH FROM product.created)::bigint, 0)::bigint AS created,
    COALESCE(EXTRACT(EPOCH FROM product.updated)::bigint, 0)::bigint AS updated,
    FALSE::boolean AS digital_filled
FROM product
WHERE product.slug = sqlc.arg(slug) AND product.deleted = FALSE AND product.active = TRUE;

-- name: OptionsByProduct :many
SELECT id, name, COALESCE(position, 0)::bigint AS position
FROM product_option WHERE product_id = sqlc.arg(product_id) ORDER BY product_option.position;

-- name: OptionValuesByProduct :many
SELECT id, option_id, value, COALESCE(position, 0)::bigint AS position
FROM product_option_value
WHERE option_id IN (SELECT po.id FROM product_option po WHERE po.product_id = sqlc.arg(product_id))
ORDER BY option_id, product_option_value.position;

-- name: OptionValuesByOption :many
SELECT id, value, COALESCE(position, 0)::bigint AS position
FROM product_option_value WHERE option_id = sqlc.arg(option_id) ORDER BY product_option_value.position;

-- name: LiveVariantsByProduct :many
SELECT id, sku,
       COALESCE(price_surcharge, 0)::bigint AS price_surcharge,
       COALESCE(quantity, 0)::bigint AS quantity,
       option_values,
       COALESCE(active, FALSE)::boolean AS active
FROM product_variant WHERE product_id = sqlc.arg(product_id) AND deleted = FALSE;

-- name: VariantsByProduct :many
SELECT id, sku,
       COALESCE(price_surcharge, 0)::bigint AS price_surcharge,
       COALESCE(quantity, 0)::bigint AS quantity,
       COALESCE(active, FALSE)::boolean AS active
FROM product_variant WHERE product_id = sqlc.arg(product_id);

-- name: ProductBase :one
SELECT id, name, brief, "desc", slug,
       amount::bigint AS amount,
       COALESCE(quantity, 0)::bigint AS quantity,
       sku,
       COALESCE(has_variants, FALSE)::boolean AS has_variants,
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
UPDATE product_image SET position = sqlc.arg(position)::bigint WHERE id = sqlc.arg(id);

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
    sqlc.arg(amount)::bigint, sqlc.arg(quantity)::bigint, sqlc.narg(sku),
    sqlc.arg(has_variants)::boolean, sqlc.arg(metadata), sqlc.arg(attribute), sqlc.arg(seo),
    sqlc.narg(digital), sqlc.arg(active)::boolean
);

-- name: InsertProduct :one
INSERT INTO product (
    id, name, amount, slug, metadata, attribute, brief, "desc", digital,
    quantity, sku, seo, active
) VALUES (
    sqlc.arg(id), sqlc.arg(name), sqlc.arg(amount)::bigint, sqlc.arg(slug),
    sqlc.arg(metadata), sqlc.arg(attribute), sqlc.arg(brief), sqlc.arg(description), sqlc.narg(digital),
    sqlc.arg(quantity)::bigint, sqlc.narg(sku), sqlc.arg(seo), sqlc.arg(active)::boolean
)
RETURNING COALESCE(EXTRACT(EPOCH FROM created)::bigint, 0)::bigint AS created;

-- name: UpdateProductFields :exec
UPDATE product SET
    name = sqlc.arg(name),
    brief = sqlc.arg(brief),
    "desc" = sqlc.arg(description),
    slug = sqlc.arg(slug),
    amount = sqlc.arg(amount)::bigint,
    quantity = sqlc.arg(quantity)::bigint,
    sku = sqlc.narg(sku),
    has_variants = sqlc.arg(has_variants)::boolean,
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
VALUES (sqlc.arg(id), sqlc.arg(product_id), sqlc.arg(name), sqlc.arg(ext), sqlc.arg(orig_name), sqlc.arg(position)::bigint);

-- name: InsertProductOption :exec
INSERT INTO product_option (id, product_id, name, position)
VALUES (sqlc.arg(id), sqlc.arg(product_id), sqlc.arg(name), sqlc.arg(position)::bigint);

-- name: InsertOptionValue :exec
INSERT INTO product_option_value (id, option_id, value, position)
VALUES (sqlc.arg(id), sqlc.arg(option_id), sqlc.arg(value), sqlc.arg(position)::bigint);

-- name: InsertVariant :exec
INSERT INTO product_variant (id, product_id, sku, price_surcharge, quantity, option_values, active)
VALUES (sqlc.arg(id), sqlc.arg(product_id), sqlc.narg(sku), sqlc.arg(price_surcharge)::bigint,
        sqlc.arg(quantity)::bigint, sqlc.arg(option_values), sqlc.arg(active)::boolean);

-- name: FindOptionValueID :one
SELECT pov.id
FROM product_option_value pov
JOIN product_option po ON pov.option_id = po.id
WHERE po.product_id = sqlc.arg(product_id) AND po.name = sqlc.arg(option_name) AND pov.value = sqlc.arg(option_value);

-- name: InsertVariantOption :exec
INSERT INTO product_variant_option (variant_id, option_value_id) VALUES (sqlc.arg(variant_id), sqlc.arg(option_value_id));

-- name: InsertVariantImage :exec
INSERT INTO product_variant_image (id, variant_id, name, ext, orig_name, position)
VALUES (sqlc.arg(id), sqlc.arg(variant_id), sqlc.arg(name), sqlc.arg(ext), sqlc.arg(orig_name), sqlc.arg(position)::bigint);
