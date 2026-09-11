-- +goose Up
-- +goose StatementBegin
-- No-op migration for SQLite consistency with PostgreSQL
-- SQLite doesn't distinguish between JSON and JSONB types
-- SQLite queries only GROUP BY p.id (primary key), avoiding the GROUP BY issue
-- This migration exists to maintain parallel migration numbering with PostgreSQL
SELECT 1; -- no-op
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
SELECT 1; -- no-op
-- +goose StatementEnd
