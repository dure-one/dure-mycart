-- name: ProductExists :one
SELECT EXISTS(SELECT 1 FROM product WHERE slug = $1 AND deleted = FALSE);

-- name: GetProduct :one
SELECT id, name, "desc", slug, amount, digital, active, deleted, created, updated
FROM product
WHERE id = $1 AND deleted = FALSE
LIMIT 1;

-- name: InsertProduct :one
INSERT INTO product (id, name, "desc", slug, amount, digital, active)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING id, name, "desc", slug, amount, digital, active, deleted, created, updated;

-- name: UpdateProduct :exec
UPDATE product
SET name = $1, "desc" = $2, slug = $3, amount = $4, digital = $5, active = $6, updated = CURRENT_TIMESTAMP
WHERE id = $7;

-- name: DeleteProduct :exec
UPDATE product
SET deleted = TRUE, updated = CURRENT_TIMESTAMP
WHERE id = $1;

-- name: UpdateProductActive :exec
UPDATE product
SET active = NOT active, updated = CURRENT_TIMESTAMP
WHERE id = $1;
