-- name: GetSettingByKey :one
SELECT id, key, value
FROM setting
WHERE key = $1 LIMIT 1;

-- name: UpdateSetting :exec
UPDATE setting
SET value = $1
WHERE key = $2;

-- name: ListSettings :many
SELECT id, key, value
FROM setting
ORDER BY key;

-- name: CreateSetting :one
INSERT INTO setting (id, key, value)
VALUES ($1, $2, $3)
RETURNING id, key, value;

-- name: DeleteSetting :exec
DELETE FROM setting WHERE key = $1;

-- name: GetPaymentSettings :many
SELECT key, value FROM setting
WHERE key IN ('stripe_active', 'paypal_active', 'spectrocoin_active', 'coinbase_active', 'portone_active');

-- name: UpsertSetting :exec
INSERT INTO setting (id, key, value)
VALUES ($1, $2, $3)
ON CONFLICT(key) DO UPDATE SET value = excluded.value;
