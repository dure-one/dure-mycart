-- name: GetSubdomain :one
SELECT id, name, "desc"
FROM subdomain WHERE id = $1 LIMIT 1;

-- name: GetSubdomainByName :one
SELECT id, name, "desc"
FROM subdomain WHERE name = $1 LIMIT 1;

-- name: ListSubdomains :many
SELECT id, name, "desc"
FROM subdomain
ORDER BY name;

-- name: CreateSubdomain :one
INSERT INTO subdomain (id, name, "desc")
VALUES ($1, $2, $3)
RETURNING id, name, "desc";

-- name: UpdateSubdomain :exec
UPDATE subdomain
SET name = $1, "desc" = $2
WHERE id = $3;

-- name: DeleteSubdomain :exec
DELETE FROM subdomain WHERE id = $1;

-- name: SubdomainExists :one
SELECT EXISTS(SELECT 1 FROM subdomain WHERE name = $1);

-- name: ListAllSubdomains :many
SELECT id, name, "desc" FROM subdomain ORDER BY name;
