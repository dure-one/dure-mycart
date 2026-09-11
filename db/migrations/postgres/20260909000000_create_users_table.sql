-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS users (
    id         TEXT PRIMARY KEY NOT NULL,
    email      TEXT UNIQUE NOT NULL,
    password   TEXT NOT NULL,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_users_email ON users (email);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS users;
-- +goose StatementEnd
