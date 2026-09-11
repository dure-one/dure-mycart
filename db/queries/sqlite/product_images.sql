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
