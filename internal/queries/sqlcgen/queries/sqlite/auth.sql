-- name: GetAuthSettings :many
SELECT key, value FROM setting WHERE key IN ('email', 'password');
