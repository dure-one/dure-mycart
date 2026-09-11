-- name: GetCart :one
SELECT id, email, amount_total, currency, payment_id, payment_status, cart::text as cart, payment_system, created, updated
FROM cart WHERE id = $1 LIMIT 1;

-- name: ListCarts :many
SELECT id, email, amount_total, currency, payment_id, payment_status, cart::text as cart, payment_system, created, updated
FROM cart
ORDER BY created DESC
LIMIT $1 OFFSET $2;

-- name: CountCarts :one
SELECT COUNT(*) FROM cart;

-- name: CreateCart :one
INSERT INTO cart (id, email, amount_total, currency, payment_id, payment_status, cart, payment_system, created)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, NOW())
RETURNING id, email, amount_total, currency, payment_id, payment_status, cart::text as cart, payment_system, created, updated;

-- name: UpdateCart :exec
UPDATE cart
SET email = $1, amount_total = $2, currency = $3, payment_id = $4, payment_status = $5, cart = $6, payment_system = $7, updated = NOW()
WHERE id = $8;

-- name: UpdateCartPaymentStatus :exec
UPDATE cart
SET payment_status = $1, updated = NOW()
WHERE id = $2;

-- name: UpdateCartPaymentID :exec
UPDATE cart
SET payment_id = $1, updated = NOW()
WHERE id = $2;

-- name: UpdateCartPaymentFields :exec
UPDATE cart
SET payment_id = $1, payment_status = $2, updated = NOW()
WHERE id = $3;

-- name: GetCartByStatusAndID :one
SELECT email, cart::text as cart FROM cart
WHERE payment_status = $1 AND id = $2
LIMIT 1;

-- name: DeleteCart :exec
DELETE FROM cart WHERE id = $1;

-- name: ListAllCarts :many
SELECT id, email, amount_total, currency, payment_id, payment_status, cart::text as cart, payment_system, created, updated
FROM cart ORDER BY created;
