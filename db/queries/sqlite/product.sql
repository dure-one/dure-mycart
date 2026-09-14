-- name: ProductExists :one
SELECT EXISTS(SELECT 1 FROM product WHERE slug = ? AND deleted = FALSE);

-- name: GetProduct :one
SELECT id, name, "desc", slug, amount, digital, active, deleted, created, updated
FROM product
WHERE id = ? AND deleted = FALSE
LIMIT 1;

-- name: InsertProduct :one
INSERT INTO product (id, name, "desc", slug, amount, digital, active)
VALUES (?, ?, ?, ?, ?, ?, ?)
RETURNING id, name, "desc", slug, amount, digital, active, deleted, created, updated;

-- name: UpdateProduct :exec
UPDATE product
SET name = ?, "desc" = ?, slug = ?, amount = ?, digital = ?, active = ?, updated = CURRENT_TIMESTAMP
WHERE id = ?;

-- name: DeleteProduct :exec
UPDATE product
SET deleted = TRUE, updated = CURRENT_TIMESTAMP
WHERE id = ?;

-- name: UpdateProductActive :exec
UPDATE product
SET active = NOT active, updated = CURRENT_TIMESTAMP
WHERE id = ?;
