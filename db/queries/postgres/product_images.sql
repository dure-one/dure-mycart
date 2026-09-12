-- name: GetProductImage :one
SELECT id, product_id, name, ext, orig_name
FROM product_image WHERE id = $1 LIMIT 1;

-- name: ListProductImages :many
SELECT id, product_id, name, ext, orig_name, position
FROM product_image WHERE product_id = $1
ORDER BY position ASC;

-- name: CreateProductImage :one
INSERT INTO product_image (id, product_id, name, ext, orig_name)
VALUES ($1, $2, $3, $4, $5)
RETURNING id, product_id, name, ext, orig_name;

-- name: DeleteProductImage :exec
DELETE FROM product_image WHERE id = $1;

-- name: DeleteProductImages :exec
DELETE FROM product_image WHERE product_id = $1;

-- name: ListAllProductImages :many
SELECT id, product_id, name, ext, orig_name
FROM product_image ORDER BY id;

-- name: UpdateProductImagePosition :exec
UPDATE product_image SET position = $1 WHERE id = $2 AND product_id = $3;

-- name: GetProductRepImageBySlug :one
SELECT pi.id, pi.name, pi.ext, pi.orig_name
FROM product_image pi
JOIN product p ON pi.product_id = p.id
WHERE p.slug = $1 AND p.deleted = 0
ORDER BY pi.position ASC
LIMIT 1;
