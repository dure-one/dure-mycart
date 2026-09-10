-- +goose Up
-- +goose StatementBegin
-- Convert JSON to JSONB for seo columns to enable GROUP BY operations
-- PostgreSQL error: "could not identify an equality operator for type json" (42883)
-- JSON type lacks equality operators needed for GROUP BY, JSONB has them
ALTER TABLE product ALTER COLUMN seo TYPE JSONB USING seo::jsonb;
ALTER TABLE page ALTER COLUMN seo TYPE JSONB USING seo::jsonb;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
-- Revert JSONB back to JSON (safe downgrade, no data loss)
ALTER TABLE page ALTER COLUMN seo TYPE JSON USING seo::json;
ALTER TABLE product ALTER COLUMN seo TYPE JSON USING seo::json;
-- +goose StatementEnd
