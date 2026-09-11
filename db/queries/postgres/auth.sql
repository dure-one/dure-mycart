-- name: GetPasswordByEmail :one
SELECT id, key, value
FROM setting
WHERE key = 'password'
LIMIT 1;
