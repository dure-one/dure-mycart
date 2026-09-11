-- name: GetUserByEmail :one
SELECT id, email, password, created_at, updated_at
FROM users
WHERE email = $1
LIMIT 1;

-- name: CreateUser :exec
INSERT INTO users (id, email, password, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5);

-- name: UpdateUserPassword :exec
UPDATE users
SET password = $1, updated_at = $2
WHERE email = $3;
