-- name: GetCart :one
SELECT id, customer_id, email, status, stripe_id, amount, created, updated
FROM cart WHERE id = $1 LIMIT 1;

-- name: InsertCart :one
INSERT INTO cart (id, customer_id, email, status, amount)
VALUES ($1, $2, $3, $4, $5)
RETURNING id, customer_id, email, status, stripe_id, amount, created, updated;

-- name: UpdateCart :exec
UPDATE cart SET customer_id = $1, email = $2, status = $3, amount = $4, updated = CURRENT_TIMESTAMP
WHERE id = $5;

-- name: DeleteCart :exec
DELETE FROM cart WHERE id = $1;
