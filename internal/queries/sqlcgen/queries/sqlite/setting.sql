-- name: GetSettingValues :many
SELECT key, value FROM setting WHERE key IN (sqlc.slice(keys));

-- name: GetSettingsWithID :many
SELECT id, key, value FROM setting WHERE key IN (sqlc.slice(keys));

-- name: UpsertSetting :exec
INSERT INTO setting (id, key, value) VALUES (sqlc.arg(id), sqlc.arg(key), sqlc.arg(value))
ON CONFLICT (key) DO UPDATE SET value = excluded.value;

-- name: GetPasswordHash :one
SELECT value FROM setting WHERE key = 'password';

-- name: SetPasswordHash :exec
UPDATE setting SET value = sqlc.arg(value) WHERE key = 'password';
