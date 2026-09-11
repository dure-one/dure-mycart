-- name: CreateNewCart :exec
INSERT INTO new_carts (id, session_id, status, total, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?);

-- name: GetNewCartByID :one
SELECT id, session_id, status, total, created_at, updated_at
FROM new_carts
WHERE id = ?
LIMIT 1;

-- name: GetNewCartBySessionID :one
SELECT id, session_id, status, total, created_at, updated_at
FROM new_carts
WHERE session_id = ?
ORDER BY created_at DESC
LIMIT 1;

-- name: UpdateNewCart :exec
UPDATE new_carts
SET status = ?, total = ?, updated_at = ?
WHERE id = ?;

-- name: DeleteNewCart :exec
DELETE FROM new_carts WHERE id = ?;

-- name: CreateCartItem :exec
INSERT INTO cart_items (id, cart_id, product_id, quantity, price, created_at)
VALUES (?, ?, ?, ?, ?, ?);

-- name: GetCartItem :one
SELECT id, cart_id, product_id, quantity, price, created_at
FROM cart_items
WHERE id = ?
LIMIT 1;

-- name: ListCartItems :many
SELECT id, cart_id, product_id, quantity, price, created_at
FROM cart_items
WHERE cart_id = ?
ORDER BY created_at ASC;

-- name: UpdateCartItem :exec
UPDATE cart_items
SET quantity = ?, price = ?
WHERE id = ?;

-- name: DeleteCartItem :exec
DELETE FROM cart_items WHERE id = ?;

-- name: DeleteCartItemsByCartID :exec
DELETE FROM cart_items WHERE cart_id = ?;
