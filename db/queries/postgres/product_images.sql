-- name: GetProductImage :one
SELECT id, product_id, name, ext, orig_name
FROM product_image WHERE id = $1 LIMIT 1;

-- name: ListProductImages :many
SELECT id, product_id, name, ext, orig_name
FROM product_image WHERE product_id = $1
ORDER BY id;

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
