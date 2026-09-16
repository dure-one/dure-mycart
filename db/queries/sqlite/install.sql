-- name: GetSettingValue :one
SELECT value
FROM setting
WHERE key = ?
LIMIT 1;

-- name: UpdateSettingValue :exec
UPDATE setting
SET value = ?
WHERE key = ?;
