-- name: ActiveProductsForSEO :many
SELECT name, slug, brief, "desc",
       EXTRACT(EPOCH FROM COALESCE(updated, created))::bigint AS updated
FROM product
WHERE active = TRUE
ORDER BY name
LIMIT sqlc.arg(max_rows);

-- name: ActivePagesForSEO :many
SELECT name, slug,
       EXTRACT(EPOCH FROM COALESCE(updated, created))::bigint AS updated
FROM page
WHERE active = TRUE
ORDER BY name
LIMIT sqlc.arg(max_rows);
