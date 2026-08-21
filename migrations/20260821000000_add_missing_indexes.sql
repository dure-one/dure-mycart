-- +goose Up
-- Performance indexes for hot admin/storefront queries.

-- CartLetterPurchase and public product filters look up digital rows by cart.
CREATE INDEX IF NOT EXISTS idx_digital_data_cart_id ON digital_data (cart_id);

-- Admin cart listing sorts/paginates by creation time; COUNT(*) scans stay,
-- but ORDER BY created DESC LIMIT stops being a full sort.
CREATE INDEX IF NOT EXISTS idx_cart_created ON cart (created);

-- Drop redundant indexes that duplicate UNIQUE constraints / the primary key
-- (pure write amplification).
DROP INDEX IF EXISTS idx_session_key;
DROP INDEX IF EXISTS idx_setting_key;
DROP INDEX IF EXISTS idx_product_id;

-- +goose Down
-- Non-destructive: restore the dropped indexes, leave the new ones in place
-- (extra indexes are harmless).
CREATE INDEX IF NOT EXISTS idx_session_key ON session (key);
CREATE INDEX IF NOT EXISTS idx_setting_key ON setting (key);
CREATE INDEX IF NOT EXISTS idx_product_id ON product (id);
