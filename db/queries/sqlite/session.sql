-- name: GetSession :one
SELECT key, value, expires
FROM session
WHERE key = ? AND expires > ?
LIMIT 1;

-- name: CleanupExpiredSessions :exec
DELETE FROM session
WHERE expires < ? AND key != ?;

-- name: UpsertSession :exec
INSERT INTO session (key, value, expires)
VALUES (?, ?, ?)
ON CONFLICT (key) DO UPDATE
SET value = excluded.value, expires = excluded.expires;

-- name: UpdateSession :exec
UPDATE session
SET value = ?, expires = ?
WHERE key = ?;

-- name: DeleteSession :exec
DELETE FROM session WHERE key = ?;
