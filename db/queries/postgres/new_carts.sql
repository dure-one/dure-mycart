-- name: CreateNewCart :exec
INSERT INTO new_carts (id, session_id, status, total, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $6);

-- name: GetNewCartByID :one
SELECT id, session_id, status, total, created_at, updated_at
FROM new_carts
WHERE id = $1
LIMIT 1;

-- name: GetNewCartBySessionID :one
SELECT id, session_id, status, total, created_at, updated_at
FROM new_carts
WHERE session_id = $1
ORDER BY created_at DESC
LIMIT 1;

-- name: UpdateNewCart :exec
UPDATE new_carts
SET status = $1, total = $2, updated_at = $3
WHERE id = $4;

-- name: DeleteNewCart :exec
DELETE FROM new_carts WHERE id = $1;

-- name: CreateCartItem :exec
INSERT INTO cart_items (id, cart_id, product_id, quantity, price, created_at)
VALUES ($1, $2, $3, $4, $5, $6);

-- name: GetCartItem :one
SELECT id, cart_id, product_id, quantity, price, created_at
FROM cart_items
WHERE id = $1
LIMIT 1;

-- name: ListCartItems :many
SELECT id, cart_id, product_id, quantity, price, created_at
FROM cart_items
WHERE cart_id = $1
ORDER BY created_at ASC;

-- name: UpdateCartItem :exec
UPDATE cart_items
SET quantity = $1, price = $2
WHERE id = $3;

-- name: DeleteCartItem :exec
DELETE FROM cart_items WHERE id = $1;

-- name: DeleteCartItemsByCartID :exec
DELETE FROM cart_items WHERE cart_id = $1;
