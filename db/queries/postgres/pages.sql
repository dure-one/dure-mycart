-- name: GetPageByID :one
SELECT id, name, slug, content, position, active, created, updated
FROM page WHERE id = $1 LIMIT 1;

-- name: GetPageBySlug :one
SELECT id, name, slug, content, position, active, created, updated
FROM page WHERE slug = $1 LIMIT 1;

-- name: ListPages :many
SELECT id, name, slug, content, position, active, created, updated
FROM page
ORDER BY created DESC
LIMIT $1 OFFSET $2;

-- name: ListPagesByPosition :many
SELECT id, name, slug, content, position, active, created, updated
FROM page
WHERE position = $1 AND active = $2
ORDER BY created DESC;

-- name: CountPages :one
SELECT COUNT(*) FROM page;

-- name: CreatePage :one
INSERT INTO page (id, name, slug, content, position, active, created)
VALUES ($1, $2, $3, $4, $5, $6, NOW())
RETURNING id, name, slug, content, position, active, created, updated;

-- name: UpdatePage :exec
UPDATE page
SET name = $1, slug = $2, content = $3, position = $4, active = $5, updated = NOW()
WHERE id = $6;

-- name: UpdatePageContent :exec
UPDATE page
SET content = $1, updated = NOW()
WHERE id = $2;

-- name: UpdatePageActive :exec
UPDATE page
SET active = NOT active, updated = NOW()
WHERE id = $1;

-- name: DeletePage :exec
DELETE FROM page WHERE id = $1;

-- name: PageExists :one
SELECT EXISTS(SELECT 1 FROM page WHERE slug = $1);

-- name: ListAllPages :many
SELECT id, name, slug, content, position, active, created, updated
FROM page ORDER BY created;

-- name: GetPageSeo :one
SELECT seo FROM page WHERE id = $1;
