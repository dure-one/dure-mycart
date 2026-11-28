-- name: ActiveProductsForSEO :many
SELECT name, slug, brief, "desc",
       CAST(strftime('%s', COALESCE(updated, created)) AS INTEGER) AS updated
FROM product
WHERE active = TRUE
ORDER BY name
LIMIT sqlc.arg(max_rows);

-- name: ActivePagesForSEO :many
SELECT name, slug,
       CAST(strftime('%s', COALESCE(updated, created)) AS INTEGER) AS updated
FROM page
WHERE active = TRUE
ORDER BY name
LIMIT sqlc.arg(max_rows);
