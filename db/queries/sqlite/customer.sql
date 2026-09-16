-- name: GetCustomer :one
SELECT id, email, name, created, updated
FROM customer WHERE id = ? LIMIT 1;

-- name: InsertCustomer :one
INSERT INTO customer (id, email, name)
VALUES (?, ?, ?)
RETURNING id, email, name, created, updated;

-- name: UpdateCustomer :exec
UPDATE customer SET email = ?, name = ?, updated = CURRENT_TIMESTAMP
WHERE id = ?;

-- name: DeleteCustomer :exec
DELETE FROM customer WHERE id = ?;
