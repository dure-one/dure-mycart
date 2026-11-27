-- name: GetInstalled :one
SELECT value FROM setting WHERE key = 'installed';

-- name: SetSettingValue :exec
UPDATE setting SET value = sqlc.arg(value) WHERE key = sqlc.arg(key);
