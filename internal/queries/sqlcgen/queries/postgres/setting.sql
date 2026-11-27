-- Key lists travel as a JSON array: `= ANY($1::text[])` would make sqlc
-- generate a github.com/lib/pq dependency for database/sql.

-- name: GetSettingValues :many
SELECT key, value FROM setting WHERE key IN (SELECT jsonb_array_elements_text(sqlc.arg(keys)::jsonb));

-- name: GetSettingsWithID :many
SELECT id, key, value FROM setting WHERE key IN (SELECT jsonb_array_elements_text(sqlc.arg(keys)::jsonb));

-- name: UpsertSetting :exec
INSERT INTO setting (id, key, value) VALUES (sqlc.arg(id), sqlc.arg(key), sqlc.arg(value))
ON CONFLICT (key) DO UPDATE SET value = excluded.value;

-- name: GetPasswordHash :one
SELECT value FROM setting WHERE key = 'password';

-- name: SetPasswordHash :exec
UPDATE setting SET value = sqlc.arg(value) WHERE key = 'password';
