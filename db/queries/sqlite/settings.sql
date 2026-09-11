-- name: GetSettingByKey :one
SELECT id, key, value
FROM setting
WHERE key = ? LIMIT 1;

-- name: UpdateSetting :exec
UPDATE setting
SET value = ?
WHERE key = ?;

-- name: ListSettings :many
SELECT id, key, value
FROM setting
ORDER BY key;

-- name: CreateSetting :one
INSERT INTO setting (id, key, value)
VALUES (?, ?, ?)
RETURNING id, key, value;

-- name: DeleteSetting :exec
DELETE FROM setting WHERE key = ?;

-- name: GetPaymentSettings :many
SELECT key, value FROM setting
WHERE key IN ('stripe_active', 'paypal_active', 'spectrocoin_active', 'coinbase_active', 'portone_active');

-- name: UpsertSetting :exec
INSERT INTO setting (id, key, value)
VALUES (?, ?, ?)
ON CONFLICT(key) DO UPDATE SET value = excluded.value;
