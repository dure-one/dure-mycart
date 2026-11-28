-- +goose Up
-- +goose StatementBegin
CREATE TABLE refresh_tokens (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL,
    role TEXT NOT NULL CHECK(role IN ('admin', 'customer')),
    expires_at INTEGER NOT NULL,
    created_at INTEGER NOT NULL
);
-- +goose StatementEnd

-- +goose StatementBegin
CREATE INDEX idx_refresh_tokens_user ON refresh_tokens(user_id);
-- +goose StatementEnd

-- +goose StatementBegin
CREATE INDEX idx_refresh_tokens_expires ON refresh_tokens(expires_at);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS refresh_tokens;
-- +goose StatementEnd
