-- name: PageExists :one
SELECT CAST(EXISTS(SELECT 1 FROM page WHERE slug = sqlc.arg(slug)) AS BOOLEAN) AS page_exists;

-- name: ListPages :many
SELECT id, name, slug, position, active, seo,
       CAST(COALESCE(strftime('%s', created), 0) AS INTEGER) AS created,
       CAST(COALESCE(strftime('%s', updated), 0) AS INTEGER) AS updated
FROM page
WHERE (CAST(sqlc.arg(private) AS BOOLEAN) OR active = TRUE)
ORDER BY page.position, page.id
LIMIT CAST(sqlc.arg(lim) AS INTEGER) OFFSET CAST(sqlc.arg(off) AS INTEGER);

-- name: CountPages :one
SELECT COUNT(*) FROM page WHERE (CAST(sqlc.arg(private) AS BOOLEAN) OR active = TRUE);

-- name: PublishedPageBySlug :one
SELECT id, name, content, active, seo FROM page WHERE slug = sqlc.arg(slug) AND active = TRUE;

-- name: PageByID :one
SELECT id, name, slug, position, content, active, seo,
       CAST(COALESCE(strftime('%s', created), 0) AS INTEGER) AS created,
       CAST(COALESCE(strftime('%s', updated), 0) AS INTEGER) AS updated
FROM page WHERE id = sqlc.arg(id);

-- name: InsertPage :one
INSERT INTO page (id, name, slug, position)
VALUES (sqlc.arg(id), sqlc.arg(name), sqlc.arg(slug), sqlc.arg(position))
RETURNING CAST(COALESCE(strftime('%s', created), 0) AS INTEGER) AS created;

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
