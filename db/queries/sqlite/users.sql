-- name: GetUserByEmail :one
SELECT id, email, password, created_at, updated_at
FROM users
WHERE email = ?
LIMIT 1;

-- name: CreateUser :exec
INSERT INTO users (id, email, password, created_at, updated_at)
VALUES (?, ?, ?, ?, ?);

-- name: UpdateUserPassword :exec
UPDATE users
SET password = ?, updated_at = ?
WHERE email = ?;
