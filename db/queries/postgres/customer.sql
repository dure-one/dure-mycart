-- name: GetCustomer :one
SELECT id, email, name, created, updated
FROM customer WHERE id = $1 LIMIT 1;

-- name: InsertCustomer :one
INSERT INTO customer (id, email, name)
VALUES ($1, $2, $3)
RETURNING id, email, name, created, updated;

-- name: UpdateCustomer :exec
UPDATE customer SET email = $1, name = $2, updated = CURRENT_TIMESTAMP
WHERE id = $3;

-- name: DeleteCustomer :exec
DELETE FROM customer WHERE id = $1;
