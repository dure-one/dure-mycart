-- name: GetCart :one
SELECT id, customer_id, email, status, stripe_id, amount, created, updated
FROM cart WHERE id = ? LIMIT 1;

-- name: InsertCart :one
INSERT INTO cart (id, customer_id, email, status, amount)
VALUES (?, ?, ?, ?, ?)
RETURNING id, customer_id, email, status, stripe_id, amount, created, updated;

-- name: UpdateCart :exec
UPDATE cart SET customer_id = ?, email = ?, status = ?, amount = ?, updated = CURRENT_TIMESTAMP
WHERE id = ?;

-- name: DeleteCart :exec
DELETE FROM cart WHERE id = ?;
