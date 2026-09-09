-- name: GetSession :one
SELECT key, value, expires FROM session WHERE key = ? LIMIT 1;

-- name: CreateSession :exec
INSERT INTO session (key, value, expires) VALUES (?, ?, ?);

-- name: UpdateSession :exec
UPDATE session SET value = ?, expires = ? WHERE key = ?;

-- name: DeleteSession :exec
DELETE FROM session WHERE key = ?;

-- name: DeleteExpiredSessions :exec
DELETE FROM session WHERE expires < ?;

-- name: ListAllSessions :many
SELECT key, value, expires FROM session;

-- name: UpsertSession :exec
INSERT OR REPLACE INTO session (key, value, expires)
VALUES (?, ?, ?);
