-- +goose Up
-- +goose StatementBegin
INSERT INTO setting (id, key, value) VALUES ('dt9fmt0z2q1x8vw', 'date_time_format', 'eu');
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DELETE FROM setting WHERE key = 'date_time_format';
-- +goose StatementEnd
