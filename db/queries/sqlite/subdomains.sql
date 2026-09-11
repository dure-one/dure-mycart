-- name: GetSubdomain :one
SELECT id, name, "desc"
FROM subdomain WHERE id = ? LIMIT 1;

-- name: GetSubdomainByName :one
SELECT id, name, "desc"
FROM subdomain WHERE name = ? LIMIT 1;

-- name: ListSubdomains :many
SELECT id, name, "desc"
FROM subdomain
ORDER BY name;

-- name: CreateSubdomain :one
INSERT INTO subdomain (id, name, "desc")
VALUES (?, ?, ?)
RETURNING id, name, "desc";

-- name: UpdateSubdomain :exec
UPDATE subdomain
SET name = ?, "desc" = ?
WHERE id = ?;

-- name: DeleteSubdomain :exec
DELETE FROM subdomain WHERE id = ?;

-- name: SubdomainExists :one
SELECT EXISTS(SELECT 1 FROM subdomain WHERE name = ?);

-- name: ListAllSubdomains :many
SELECT id, name, "desc" FROM subdomain ORDER BY name;
