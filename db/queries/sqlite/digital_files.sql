-- name: GetDigitalFile :one
SELECT id, product_id, name, ext, orig_name
FROM digital_file WHERE id = ? LIMIT 1;

-- name: ListDigitalFiles :many
SELECT id, product_id, name, ext, orig_name
FROM digital_file WHERE product_id = ?;

-- name: CreateDigitalFile :one
INSERT INTO digital_file (id, product_id, name, ext, orig_name)
VALUES (?, ?, ?, ?, ?)
RETURNING id, product_id, name, ext, orig_name;

-- name: DeleteDigitalFile :exec
DELETE FROM digital_file WHERE id = ?;

-- name: DeleteDigitalFiles :exec
DELETE FROM digital_file WHERE product_id = ?;

-- name: ListAllDigitalFiles :many
SELECT id, product_id, name, ext, orig_name
FROM digital_file ORDER BY id;
