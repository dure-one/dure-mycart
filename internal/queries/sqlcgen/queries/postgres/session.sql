-- name: GetSession :one
SELECT value FROM session WHERE key = sqlc.arg(key) AND expires > sqlc.arg(now)::bigint;

-- name: SweepSessions :exec
DELETE FROM session WHERE expires < sqlc.arg(now)::bigint AND key != sqlc.arg(key);

-- name: UpsertSession :exec
INSERT INTO session (key, value, expires) VALUES (sqlc.arg(key), sqlc.arg(value), sqlc.arg(expires)::bigint)
ON CONFLICT (key) DO UPDATE SET value = excluded.value, expires = excluded.expires;

-- name: UpdateSession :exec
UPDATE session SET value = sqlc.arg(value), expires = sqlc.arg(expires)::bigint WHERE key = sqlc.arg(key);

-- name: DeleteSession :exec
DELETE FROM session WHERE key = sqlc.arg(key);
