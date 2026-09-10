-- name: GetDigitalData :one
SELECT id, product_id, content, cart_id
FROM digital_data WHERE id = $1 LIMIT 1;

-- name: GetDigitalDataByProduct :one
SELECT id, product_id, content, cart_id
FROM digital_data WHERE product_id = $1 LIMIT 1;

-- name: ListDigitalDataByCart :many
SELECT id, product_id, content, cart_id
FROM digital_data WHERE cart_id = $1;

-- name: ListUnassignedDigitalDataByProduct :many
SELECT id, product_id, content, cart_id
FROM digital_data WHERE product_id = $1 AND cart_id IS NULL;

-- name: CreateDigitalData :one
INSERT INTO digital_data (id, product_id, content, cart_id)
VALUES ($1, $2, $3, $4)
RETURNING id, product_id, content, cart_id;

-- name: UpdateDigitalData :exec
UPDATE digital_data
SET content = $1
WHERE id = $2;

-- name: DeleteDigitalData :exec
DELETE FROM digital_data WHERE id = $1;

-- name: DeleteDigitalDataByProduct :exec
DELETE FROM digital_data WHERE product_id = $1;

-- name: ListAllDigitalData :many
SELECT id, product_id, content, cart_id
FROM digital_data ORDER BY id;
