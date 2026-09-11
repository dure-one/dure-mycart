-- +goose Up
-- +goose StatementBegin
-- Fix two social settings that never actually materialised.
--
-- Migration 20240111145752_new_sicials.sql used `INSERT OR IGNORE` with
-- setting ids `CoDDXfxF4GZxq6b` and `AC3of7o9pS9HdB1`. Those ids were
-- already taken by the init migration for `mail_letter_purchase` and
-- `smtp_host`, so the inserts were silently skipped and the keys
-- `social_youtube` / `social_other` never existed in the DB.
--
-- Re-inserting here with fresh, unique ids. The keys themselves are
-- `UNIQUE`, so `INSERT ON CONFLICT DO NOTHING` is safe on installations that
-- already have them from 20240111145752_new_sicials.sql.
-- Using same IDs as in 20240111145752 for consistency.
INSERT INTO setting VALUES ('vs1N8xXCm9sP2Kq', 'social_youtube', '') ON CONFLICT (id) DO NOTHING;
INSERT INTO setting VALUES ('tR7mQ2wYnBvL3Hp', 'social_other', '') ON CONFLICT (id) DO NOTHING;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DELETE FROM setting WHERE key = 'social_youtube' AND id = 'yt1oK0n7cN6m9pB';
DELETE FROM setting WHERE key = 'social_other' AND id = 'oT8hZ2q9vS3rW1L';
-- +goose StatementEnd
