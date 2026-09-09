-- name: GetPageByID :one
SELECT id, name, slug, content, position, active, created, updated
FROM page WHERE id = ? LIMIT 1;

-- name: GetPageBySlug :one
SELECT id, name, slug, content, position, active, created, updated
FROM page WHERE slug = ? LIMIT 1;

-- name: ListPages :many
SELECT id, name, slug, content, position, active, created, updated
FROM page
ORDER BY created DESC
LIMIT ? OFFSET ?;

-- name: ListPagesByPosition :many
SELECT id, name, slug, content, position, active, created, updated
FROM page
WHERE position = ? AND active = ?
ORDER BY created DESC;

-- name: CountPages :one
SELECT COUNT(*) FROM page;

-- name: CreatePage :one
INSERT INTO page (id, name, slug, content, position, active, created)
VALUES (?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
RETURNING id, name, slug, content, position, active, created, updated;

-- name: UpdatePage :exec
UPDATE page
SET name = ?, slug = ?, content = ?, position = ?, active = ?, updated = CURRENT_TIMESTAMP
WHERE id = ?;

-- name: UpdatePageContent :exec
UPDATE page
SET content = ?, updated = CURRENT_TIMESTAMP
WHERE id = ?;

-- name: UpdatePageActive :exec
UPDATE page
SET active = NOT active, updated = CURRENT_TIMESTAMP
WHERE id = ?;

-- name: DeletePage :exec
DELETE FROM page WHERE id = ?;

-- name: PageExists :one
SELECT EXISTS(SELECT 1 FROM page WHERE slug = ?);

-- name: ListAllPages :many
SELECT id, name, slug, content, position, active, created, updated
FROM page ORDER BY created;

-- name: GetPageSeo :one
SELECT seo FROM page WHERE id = ?;
