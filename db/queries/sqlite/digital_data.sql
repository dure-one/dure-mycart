-- name: GetDigitalData :one
SELECT id, product_id, content, cart_id
FROM digital_data WHERE id = ? LIMIT 1;

-- name: GetDigitalDataByProduct :one
SELECT id, product_id, content, cart_id
FROM digital_data WHERE product_id = ? LIMIT 1;

-- name: ListDigitalDataByCart :many
SELECT id, product_id, content, cart_id
FROM digital_data WHERE cart_id = ?;

-- name: ListUnassignedDigitalDataByProduct :many
SELECT id, product_id, content, cart_id
FROM digital_data WHERE product_id = ? AND cart_id IS NULL;

-- name: CreateDigitalData :one
INSERT INTO digital_data (id, product_id, content, cart_id)
VALUES (?, ?, ?, ?)
RETURNING id, product_id, content, cart_id;

-- name: UpdateDigitalData :exec
UPDATE digital_data
SET content = ?
WHERE id = ?;

-- name: DeleteDigitalData :exec
DELETE FROM digital_data WHERE id = ?;

-- name: DeleteDigitalDataByProduct :exec
DELETE FROM digital_data WHERE product_id = ?;

-- name: ListAllDigitalData :many
SELECT id, product_id, content, cart_id
FROM digital_data ORDER BY id;
