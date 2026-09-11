-- name: GetSession :one
SELECT key, value, expires FROM session WHERE key = $1 LIMIT 1;

-- name: CreateSession :exec
INSERT INTO session (key, value, expires) VALUES ($1, $2, $3);

-- name: UpdateSession :exec
UPDATE session SET value = $1, expires = $2 WHERE key = $3;

-- name: DeleteSession :exec
DELETE FROM session WHERE key = $1;

-- name: DeleteExpiredSessions :exec
DELETE FROM session WHERE expires < $1;

-- name: ListAllSessions :many
SELECT key, value, expires FROM session;

-- name: UpsertSession :exec
INSERT INTO session (key, value, expires)
VALUES ($1, $2, $3)
ON CONFLICT (key) DO UPDATE SET value = $2, expires = $3;
