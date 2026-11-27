-- name: PageExists :one
SELECT EXISTS(SELECT 1 FROM page WHERE slug = sqlc.arg(slug))::boolean AS page_exists;

-- name: ListPages :many
SELECT id, name, slug, position, active, seo,
       COALESCE(EXTRACT(EPOCH FROM created)::bigint, 0)::bigint AS created,
       COALESCE(EXTRACT(EPOCH FROM updated)::bigint, 0)::bigint AS updated
FROM page
WHERE (sqlc.arg(private)::boolean OR active = TRUE)
ORDER BY page.position, page.id
LIMIT sqlc.narg(lim)::bigint OFFSET sqlc.arg(off)::bigint;

-- name: CountPages :one
SELECT COUNT(*) FROM page WHERE (sqlc.arg(private)::boolean OR active = TRUE);

-- name: PublishedPageBySlug :one
SELECT id, name, content, active, seo FROM page WHERE slug = sqlc.arg(slug) AND active = TRUE;

-- name: PageByID :one
SELECT id, name, slug, position, content, active, seo,
       COALESCE(EXTRACT(EPOCH FROM created)::bigint, 0)::bigint AS created,
       COALESCE(EXTRACT(EPOCH FROM updated)::bigint, 0)::bigint AS updated
FROM page WHERE id = sqlc.arg(id);

-- name: InsertPage :one
INSERT INTO page (id, name, slug, position)
VALUES (sqlc.arg(id), sqlc.arg(name), sqlc.arg(slug), sqlc.arg(position))
RETURNING COALESCE(EXTRACT(EPOCH FROM created)::bigint, 0)::bigint AS created;

-- name: UpdatePage :exec
UPDATE page SET name = sqlc.arg(name), slug = sqlc.arg(slug), position = sqlc.arg(position),
       content = sqlc.narg(content), seo = sqlc.arg(seo), updated = CURRENT_TIMESTAMP
WHERE id = sqlc.arg(id);

-- name: DeletePage :exec
DELETE FROM page WHERE id = sqlc.arg(id);

-- name: UpdatePageContent :exec
UPDATE page SET content = sqlc.narg(content), updated = CURRENT_TIMESTAMP WHERE id = sqlc.arg(id);

-- name: TogglePageActive :exec
UPDATE page SET active = NOT active, updated = CURRENT_TIMESTAMP WHERE id = sqlc.arg(id);
