-- +goose Up
-- +goose StatementBegin
-- Storefront customer accounts.
--
-- The cabinet is opt-in: `account_enabled` defaults to FALSE, so an
-- installation that has no use for buyer logins keeps the storefront it had
-- before this migration. While it is off, every /api/customer route answers
-- 404 as if it did not exist — the routes themselves are always registered.
INSERT INTO setting VALUES ('90s86f85m5rmoqe', 'account_enabled', 'FALSE')
	ON CONFLICT (key) DO NOTHING;
-- Filled in on first use rather than seeded here: a signing key written into a
-- public migration is a published signing key. See queries.CustomerQueries.
INSERT INTO setting VALUES ('78bftcwgd19zuv4', 'account_jwt_secret', '')
	ON CONFLICT (key) DO NOTHING;
INSERT INTO setting VALUES ('vthmd9bqaossps4', 'account_jwt_expire_hours', '720')
	ON CONFLICT (key) DO NOTHING;

-- email is UNIQUE, which already gives it an index: a second one on the same
-- column would only add writes. Same reasoning as the drops in
-- 20260821000000_add_missing_indexes.sql.
CREATE TABLE customer (
	id        TEXT PRIMARY KEY NOT NULL,
	email     TEXT UNIQUE NOT NULL,
	password  TEXT NOT NULL,
	name      TEXT DEFAULT NULL,
	active    BOOLEAN DEFAULT TRUE NOT NULL,
	created   TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
	updated   TIMESTAMP
);

-- The cabinet looks a cart up by the address the buyer typed at checkout, and
-- compares it trimmed and lower-cased. A plain index on the column does not
-- serve that comparison, and an index on the expression cannot be used in this
-- schema: TestSchemaConformance reads each index back per column and compares
-- the two engines, but neither can report an expression — SQLite returns a NULL
-- column name for one, PostgreSQL an attnum of 0. The index is kept as the
-- conventional one on the column; the scan it does not save is the price of
-- keeping the two schemas checkable against each other.
CREATE INDEX IF NOT EXISTS idx_cart_email ON cart (email);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS idx_cart_email;
-- IF EXISTS so that a half-applied Up, which stops before creating the table,
-- still rolls back cleanly.
DROP TABLE IF EXISTS customer;
-- By key, not by id: the ids above are also searched for in this tree, but a
-- future migration that reuses one of them must not lose its row here.
DELETE FROM setting WHERE key IN ('account_enabled', 'account_jwt_secret', 'account_jwt_expire_hours');
-- +goose StatementEnd
