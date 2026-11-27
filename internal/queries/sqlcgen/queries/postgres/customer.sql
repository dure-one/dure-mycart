-- name: CountCustomersByEmail :one
SELECT COUNT(*) FROM customer WHERE email = sqlc.arg(email);

-- name: InsertCustomer :one
INSERT INTO customer (id, email, password, name, active)
VALUES (sqlc.arg(id), sqlc.arg(email), sqlc.arg(password), sqlc.arg(name), sqlc.arg(active))
RETURNING COALESCE(EXTRACT(EPOCH FROM created)::bigint, 0)::bigint AS created;

-- name: CustomerByEmail :one
SELECT id, email, password, name, active,
       COALESCE(EXTRACT(EPOCH FROM created)::bigint, 0)::bigint AS created,
       COALESCE(EXTRACT(EPOCH FROM updated)::bigint, 0)::bigint AS updated
FROM customer WHERE email = sqlc.arg(email);

-- name: CustomerByID :one
SELECT id, email, password, name, active,
       COALESCE(EXTRACT(EPOCH FROM created)::bigint, 0)::bigint AS created,
       COALESCE(EXTRACT(EPOCH FROM updated)::bigint, 0)::bigint AS updated
FROM customer WHERE id = sqlc.arg(id);

-- name: SetCustomerActive :exec
UPDATE customer SET active = sqlc.arg(active), updated = CURRENT_TIMESTAMP WHERE id = sqlc.arg(id);

-- name: SetCustomerPassword :exec
UPDATE customer SET password = sqlc.arg(password), updated = CURRENT_TIMESTAMP WHERE id = sqlc.arg(id);

-- name: DeleteCustomer :exec
DELETE FROM customer WHERE id = sqlc.arg(id);

-- name: DeleteSessionsByValue :exec
DELETE FROM session WHERE value = sqlc.arg(value);

-- name: GetSettingValue :one
SELECT value FROM setting WHERE key = sqlc.arg(key);

-- name: FillEmptySetting :exec
UPDATE setting SET value = sqlc.arg(value) WHERE key = sqlc.arg(key) AND value = '';

-- name: CountCustomerAddresses :one
SELECT COUNT(*)
FROM (
    SELECT email FROM customer
    UNION
    SELECT LOWER(TRIM(pc.email)) AS email FROM cart pc WHERE TRIM(pc.email) <> '' AND pc.payment_status = sqlc.arg(paid)
) e
LEFT JOIN customer c ON c.email = e.email
WHERE (sqlc.arg(any_name)::boolean
       OR (e.email LIKE sqlc.arg(pattern)::text ESCAPE '\')
       OR (LOWER(COALESCE(c.name, '')) LIKE sqlc.arg(pattern)::text ESCAPE '\'))
  AND (NOT sqlc.arg(registered_only)::boolean OR c.id IS NOT NULL);

-- name: CustomerSummaries :many
SELECT
    c.id AS account_id,
    e.email::text AS email,
    COALESCE(c.name, '')::text AS name,
    c.active,
    COALESCE(EXTRACT(EPOCH FROM c.created)::bigint, 0)::bigint AS created,
    COALESCE(EXTRACT(EPOCH FROM c.updated)::bigint, 0)::bigint AS updated,
    COALESCE(o.purchases, 0)::bigint AS purchases,
    COALESCE(o.spent, 0)::bigint AS spent,
    COALESCE(o.last_order, 0)::bigint AS last_order
FROM (
    SELECT email FROM customer
    UNION
    SELECT LOWER(TRIM(pc.email)) AS email FROM cart pc WHERE TRIM(pc.email) <> '' AND pc.payment_status = sqlc.arg(paid)
) e
LEFT JOIN customer c ON c.email = e.email
LEFT JOIN (
    SELECT
        LOWER(TRIM(oc.email)) AS email,
        COUNT(*) AS purchases,
        SUM(CASE WHEN oc.currency = sqlc.arg(currency) THEN oc.amount_total ELSE 0 END) AS spent,
        MAX(EXTRACT(EPOCH FROM oc.created)::bigint) AS last_order
    FROM cart oc
    WHERE oc.payment_status = sqlc.arg(paid) AND TRIM(oc.email) <> ''
    GROUP BY LOWER(TRIM(oc.email))
) o ON o.email = e.email
WHERE (sqlc.arg(any_name)::boolean
       OR (e.email LIKE sqlc.arg(pattern)::text ESCAPE '\')
       OR (LOWER(COALESCE(c.name, '')) LIKE sqlc.arg(pattern)::text ESCAPE '\'))
  AND (NOT sqlc.arg(registered_only)::boolean OR c.id IS NOT NULL)
ORDER BY (o.last_order IS NULL), o.last_order DESC, e.email
LIMIT sqlc.arg(lim)::bigint OFFSET sqlc.arg(off)::bigint;

-- ID lists travel as a JSON array; see setting.sql.

-- name: PurchaseProducts :many
SELECT id, name, slug, digital FROM product
WHERE id IN (SELECT jsonb_array_elements_text(sqlc.arg(ids)::jsonb));

-- name: PurchaseFiles :many
SELECT product_id, id, name, ext, orig_name FROM digital_file
WHERE product_id IN (SELECT jsonb_array_elements_text(sqlc.arg(ids)::jsonb))
ORDER BY COALESCE(NULLIF(orig_name, ''), name || '.' || ext), id;

-- name: PurchaseCodes :many
SELECT cart_id, product_id, content FROM digital_data
WHERE cart_id IN (SELECT jsonb_array_elements_text(sqlc.arg(ids)::jsonb))
ORDER BY id;

-- name: DigitalFileByID :one
SELECT id, name, ext, orig_name, product_id FROM digital_file WHERE id = sqlc.arg(id);
