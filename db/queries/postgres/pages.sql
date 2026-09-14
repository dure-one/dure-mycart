-- name: PageExists :one
SELECT EXISTS(SELECT 1 FROM page WHERE slug = $1);

-- name: GetPageBySlug :one
SELECT id, name, slug, position, content, active, created, updated
FROM page
WHERE slug = $1
LIMIT 1;

-- name: GetPageByID :one
SELECT id, name, slug, position, content, active, created, updated
FROM page
WHERE id = $1
LIMIT 1;

-- name: InsertPage :one
INSERT INTO page (id, name, slug, position, content, active)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING id, name, slug, position, content, active, created, updated;

-- name: UpdatePage :exec
UPDATE page
SET name = $1, slug = $2, position = $3, content = $4, active = $5, updated = CURRENT_TIMESTAMP
WHERE id = $6;

-- name: DeletePage :exec
DELETE FROM page WHERE id = $1;

-- name: UpdatePageActive :exec
UPDATE page
SET active = NOT active, updated = CURRENT_TIMESTAMP
WHERE id = $1;
