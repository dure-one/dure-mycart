-- +goose Up
-- +goose StatementBegin
-- The shop's own marks: the logo the storefront draws in its header and the
-- favicon it hands the browser, each holding the name of a file in lc_uploads,
-- plus an optional line printed beside the logo.
--
-- Empty on a fresh installation, which is what keeps an installation that has
-- uploaded nothing drawn exactly as it was before this migration: the built-in
-- mark, and the icon the build shipped with. The files themselves are never
-- carried here — uploads are runtime data and belong in lc_uploads, which the
-- shop mounts and backs up on its own, not in a migration.
INSERT INTO setting VALUES ('8jp9hjjkw691f0u', 'branding_logo', '')
	ON CONFLICT (key) DO NOTHING;
INSERT INTO setting VALUES ('4rikxop462bewmj', 'branding_favicon', '')
	ON CONFLICT (key) DO NOTHING;
INSERT INTO setting VALUES ('ef9j6kzkfm3h8oo', 'branding_tagline', '')
	ON CONFLICT (key) DO NOTHING;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
-- By key, not by id: the ids above are also searched for in this tree, but a
-- future migration that reuses one of them must not lose its row here.
DELETE FROM setting WHERE key IN ('branding_logo', 'branding_favicon', 'branding_tagline');
-- +goose StatementEnd
