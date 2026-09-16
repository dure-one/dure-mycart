-- name: GetSession :one
SELECT key, value, expires
FROM session
WHERE key = $1 AND expires > $2
LIMIT 1;

-- name: CleanupExpiredSessions :exec
DELETE FROM session
WHERE expires < $1 AND key != $2;

-- name: UpsertSession :exec
INSERT INTO session (key, value, expires)
VALUES ($1, $2, $3)
ON CONFLICT (key) DO UPDATE
SET value = EXCLUDED.value, expires = EXCLUDED.expires;

-- name: UpdateSession :exec
UPDATE session
SET value = $1, expires = $2
WHERE key = $3;

-- name: DeleteSession :exec
DELETE FROM session WHERE key = $1;
