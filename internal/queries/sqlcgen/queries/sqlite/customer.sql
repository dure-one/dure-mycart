-- name: CountCustomersByEmail :one
SELECT COUNT(*) FROM customer WHERE email = sqlc.arg(email);

-- name: InsertCustomer :one
INSERT INTO customer (id, email, password, name, active)
VALUES (sqlc.arg(id), sqlc.arg(email), sqlc.arg(password), sqlc.arg(name), sqlc.arg(active))
RETURNING CAST(COALESCE(strftime('%s', created), 0) AS INTEGER) AS created;

-- name: CustomerByEmail :one
SELECT id, email, password, name, active,
       CAST(COALESCE(strftime('%s', created), 0) AS INTEGER) AS created,
       CAST(COALESCE(strftime('%s', updated), 0) AS INTEGER) AS updated
FROM customer WHERE email = sqlc.arg(email);

-- name: CustomerByID :one
SELECT id, email, password, name, active,
       CAST(COALESCE(strftime('%s', created), 0) AS INTEGER) AS created,
       CAST(COALESCE(strftime('%s', updated), 0) AS INTEGER) AS updated
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
WHERE (CAST(sqlc.arg(any_name) AS BOOLEAN)
       OR (e.email LIKE CAST(sqlc.arg(pattern) AS TEXT) ESCAPE '\')
       OR (LOWER(COALESCE(c.name, '')) LIKE CAST(sqlc.arg(pattern) AS TEXT) ESCAPE '\'))
  AND (NOT CAST(sqlc.arg(registered_only) AS BOOLEAN) OR c.id IS NOT NULL);

-- name: CustomerSummaries :many
SELECT
    c.id AS account_id,
    CAST(e.email AS TEXT) AS email,
    CAST(COALESCE(c.name, '') AS TEXT) AS name,
    c.active,
    CAST(COALESCE(strftime('%s', c.created), 0) AS INTEGER) AS created,
    CAST(COALESCE(strftime('%s', c.updated), 0) AS INTEGER) AS updated,
    CAST(COALESCE(o.purchases, 0) AS INTEGER) AS purchases,
    CAST(COALESCE(o.spent, 0) AS INTEGER) AS spent,
    CAST(COALESCE(o.last_order, 0) AS INTEGER) AS last_order
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
        MAX(CAST(strftime('%s', oc.created) AS INTEGER)) AS last_order
    FROM cart oc
    WHERE oc.payment_status = sqlc.arg(paid) AND TRIM(oc.email) <> ''
    GROUP BY LOWER(TRIM(oc.email))
) o ON o.email = e.email
WHERE (CAST(sqlc.arg(any_name) AS BOOLEAN)
       OR (e.email LIKE CAST(sqlc.arg(pattern) AS TEXT) ESCAPE '\')
       OR (LOWER(COALESCE(c.name, '')) LIKE CAST(sqlc.arg(pattern) AS TEXT) ESCAPE '\'))
  AND (NOT CAST(sqlc.arg(registered_only) AS BOOLEAN) OR c.id IS NOT NULL)
ORDER BY (o.last_order IS NULL), o.last_order DESC, e.email
LIMIT CAST(sqlc.arg(lim) AS INTEGER) OFFSET CAST(sqlc.arg(off) AS INTEGER);

-- name: PurchaseProducts :many
SELECT id, name, slug, digital FROM product WHERE id IN (sqlc.slice(ids));

-- name: PurchaseFiles :many
SELECT product_id, id, name, ext, orig_name FROM digital_file
WHERE product_id IN (sqlc.slice(ids))
ORDER BY COALESCE(NULLIF(orig_name, ''), name || '.' || ext), id;

-- name: PurchaseCodes :many
SELECT cart_id, product_id, content FROM digital_data
WHERE cart_id IN (sqlc.slice(ids))
ORDER BY id;

-- name: DigitalFileByID :one
SELECT id, name, ext, orig_name, product_id FROM digital_file WHERE id = sqlc.arg(id);
