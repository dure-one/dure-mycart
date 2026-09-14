-- name: PageExists :one
SELECT EXISTS(SELECT 1 FROM page WHERE slug = ?);

-- name: GetPageBySlug :one
SELECT id, name, slug, position, content, active, created, updated
FROM page
WHERE slug = ?
LIMIT 1;

-- name: GetPageByID :one
SELECT id, name, slug, position, content, active, created, updated
FROM page
WHERE id = ?
LIMIT 1;

-- name: InsertPage :one
INSERT INTO page (id, name, slug, position, content, active)
VALUES (?, ?, ?, ?, ?, ?)
RETURNING id, name, slug, position, content, active, created, updated;

-- name: UpdatePage :exec
UPDATE page
SET name = ?, slug = ?, position = ?, content = ?, active = ?, updated = CURRENT_TIMESTAMP
WHERE id = ?;

-- name: DeletePage :exec
DELETE FROM page WHERE id = ?;

-- name: UpdatePageActive :exec
UPDATE page
SET active = NOT active, updated = CURRENT_TIMESTAMP
WHERE id = ?;
