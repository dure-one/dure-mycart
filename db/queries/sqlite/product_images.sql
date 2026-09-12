-- name: GetProductImage :one
SELECT id, product_id, name, ext, orig_name
FROM product_image WHERE id = ? LIMIT 1;

-- name: ListProductImages :many
SELECT id, product_id, name, ext, orig_name
FROM product_image WHERE product_id = ?
ORDER BY id;

-- name: CreateProductImage :one
INSERT INTO product_image (id, product_id, name, ext, orig_name)
VALUES (?, ?, ?, ?, ?)
RETURNING id, product_id, name, ext, orig_name;

-- name: DeleteProductImage :exec
DELETE FROM product_image WHERE id = ?;

-- name: DeleteProductImages :exec
DELETE FROM product_image WHERE product_id = ?;

-- name: ListAllProductImages :many
SELECT id, product_id, name, ext, orig_name
FROM product_image ORDER BY id;

-- name: UpdateProductImagePosition :exec
UPDATE product_image SET position = ? WHERE id = ? AND product_id = ?;

-- name: GetProductRepImageBySlug :one
SELECT pi.id, pi.name, pi.ext, pi.orig_name
FROM product_image pi
JOIN product p ON pi.product_id = p.id
WHERE p.slug = ? AND p.deleted = 0
ORDER BY pi.position ASC
LIMIT 1;
