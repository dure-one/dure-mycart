-- name: PaymentActiveSettings :many
SELECT key, value FROM setting
WHERE key IN ('stripe_active', 'paypal_active', 'spectrocoin_active', 'coinbase_active', 'portone_active');

-- name: CartByID :one
SELECT id, email, cart,
       CAST(amount_total AS INTEGER) AS amount_total,
       currency, payment_id, payment_status, payment_system,
       CAST(COALESCE(strftime('%s', created), 0) AS INTEGER) AS created,
       CAST(COALESCE(strftime('%s', updated), 0) AS INTEGER) AS updated
FROM cart WHERE id = sqlc.arg(id);

-- name: InsertCart :exec
INSERT INTO cart (id, email, cart, amount_total, currency, payment_id, payment_status, payment_system)
VALUES (sqlc.arg(id), sqlc.arg(email), sqlc.arg(cart), CAST(sqlc.arg(amount_total) AS INTEGER), sqlc.arg(currency),
        sqlc.arg(payment_id), sqlc.arg(payment_status), sqlc.arg(payment_system));

-- name: UpdateCartPayment :exec
UPDATE cart SET
    payment_id = COALESCE(sqlc.narg(payment_id), payment_id),
    payment_status = COALESCE(sqlc.narg(payment_status), payment_status),
    updated = CURRENT_TIMESTAMP
WHERE id = sqlc.arg(id);

-- name: ListCarts :many
SELECT id, email,
       CAST(amount_total AS INTEGER) AS amount_total,
       currency, payment_id, payment_status, payment_system,
       CAST(COALESCE(strftime('%s', created), 0) AS INTEGER) AS created,
       CAST(COALESCE(strftime('%s', updated), 0) AS INTEGER) AS updated
FROM cart
ORDER BY cart.created DESC
LIMIT CAST(sqlc.arg(lim) AS INTEGER) OFFSET CAST(sqlc.arg(off) AS INTEGER);

-- name: CountCarts :one
SELECT COUNT(*) FROM cart;

-- name: CartsByEmail :many
SELECT id, email,
       CAST(amount_total AS INTEGER) AS amount_total,
       currency, payment_id, payment_status, payment_system,
       CAST(COALESCE(strftime('%s', created), 0) AS INTEGER) AS created,
       CAST(COALESCE(strftime('%s', updated), 0) AS INTEGER) AS updated
FROM cart
WHERE LOWER(TRIM(email)) = CAST(sqlc.arg(email) AS TEXT)
ORDER BY cart.created DESC;

-- name: PaidCartsByEmail :many
SELECT id, email,
       CAST(amount_total AS INTEGER) AS amount_total,
       currency, payment_id, payment_status, payment_system,
       CAST(COALESCE(strftime('%s', created), 0) AS INTEGER) AS created,
       CAST(COALESCE(strftime('%s', updated), 0) AS INTEGER) AS updated,
       cart
FROM cart
WHERE LOWER(TRIM(email)) = CAST(sqlc.arg(email) AS TEXT) AND payment_status = sqlc.arg(paid)
ORDER BY cart.created DESC;

-- name: PaidCartLines :one
SELECT email, cart FROM cart WHERE payment_status = sqlc.arg(paid) AND id = sqlc.arg(id);

-- name: ProductDigital :one
SELECT digital FROM product WHERE id = sqlc.arg(id);

-- name: DigitalFilesByProduct :many
SELECT id, name, ext, orig_name FROM digital_file WHERE product_id = sqlc.arg(product_id);

-- name: ClaimedDigitalData :one
SELECT id, content FROM digital_data WHERE cart_id = sqlc.arg(cart_id) AND product_id = sqlc.arg(product_id);

-- name: FreeDigitalData :one
SELECT id, content FROM digital_data
WHERE cart_id IS NULL AND product_id = sqlc.arg(product_id)
ORDER BY id LIMIT 1;

-- name: ClaimDigitalData :execrows
UPDATE digital_data SET cart_id = sqlc.arg(cart_id) WHERE id = sqlc.arg(id) AND cart_id IS NULL;
