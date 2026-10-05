-- +goose Up
-- +goose StatementBegin
INSERT INTO setting (id, key, value) VALUES
    ('resp_xmpp_en', 'responder_xmpp_enabled', 'false');
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DELETE FROM setting WHERE key = 'responder_xmpp_enabled';
-- +goose StatementEnd
