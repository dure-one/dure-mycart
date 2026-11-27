-- name: GetSession :one
SELECT value FROM session WHERE key = sqlc.arg(key) AND expires > CAST(sqlc.arg(now) AS INTEGER);

-- name: SweepSessions :exec
DELETE FROM session WHERE expires < CAST(sqlc.arg(now) AS INTEGER) AND key != sqlc.arg(key);

-- name: UpsertSession :exec
INSERT INTO session (key, value, expires) VALUES (sqlc.arg(key), sqlc.arg(value), CAST(sqlc.arg(expires) AS INTEGER))
ON CONFLICT (key) DO UPDATE SET value = excluded.value, expires = excluded.expires;

-- name: UpdateSession :exec
UPDATE session SET value = sqlc.arg(value), expires = CAST(sqlc.arg(expires) AS INTEGER) WHERE key = sqlc.arg(key);

-- name: DeleteSession :exec
DELETE FROM session WHERE key = sqlc.arg(key);
