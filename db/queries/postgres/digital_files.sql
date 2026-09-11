-- name: GetDigitalFile :one
SELECT id, product_id, name, ext, orig_name
FROM digital_file WHERE id = $1 LIMIT 1;

-- name: ListDigitalFiles :many
SELECT id, product_id, name, ext, orig_name
FROM digital_file WHERE product_id = $1;

-- name: CreateDigitalFile :one
INSERT INTO digital_file (id, product_id, name, ext, orig_name)
VALUES ($1, $2, $3, $4, $5)
RETURNING id, product_id, name, ext, orig_name;

-- name: DeleteDigitalFile :exec
DELETE FROM digital_file WHERE id = $1;

-- name: DeleteDigitalFiles :exec
DELETE FROM digital_file WHERE product_id = $1;

-- name: ListAllDigitalFiles :many
SELECT id, product_id, name, ext, orig_name
FROM digital_file ORDER BY id;
