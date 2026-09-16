-- name: GetSettingValue :one
SELECT value
FROM setting
WHERE key = $1
LIMIT 1;

-- name: UpdateSettingValue :exec
UPDATE setting
SET value = $1
WHERE key = $2;
