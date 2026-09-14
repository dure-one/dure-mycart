# migrations/AGENTS.md

Goose-managed SQL migrations embedded into the binary via `embed.go`.

## Conventions

- Filename: `YYYYMMDDHHMMSS_short_description.sql`.
- Each file contains `-- +goose Up` and `-- +goose Down` sections, each
  wrapped in `-- +goose StatementBegin` / `-- +goose StatementEnd` for
  multi-statement scripts.
- **Down must be safe to run.** It should reverse the Up section, not
  touch rows the earlier init migration owned. See the comment in
  `20240111145752_new_sicials.sql` for a real historic footgun: the old
  Down deleted rows by id, but two ids collided with `init_db`, which
  would have wiped `mail_letter_purchase` and `smtp_host` on rollback.
  The fix is to filter by `key`, not `id`.
- **Unique IDs.** Setting rows use a pseudo-random 15-char id string.
  Before inserting a new id, search the entire `migrations/` tree to
  ensure no collision (`rg "'id_here'"`).
- Prefer `INSERT … ON CONFLICT DO NOTHING` for idempotent seed data. If the
  row must be refreshed, use `ON CONFLICT (key) DO UPDATE SET …` and document
  the reason. `INSERT OR IGNORE` / `INSERT OR REPLACE` are SQLite-only.

## One set of migrations, two engines

These files are applied to SQLite *and* to PostgreSQL, so the SQL must be
portable. `migrations/schema_conformance_test.go` runs both and compares the
schemas; `TEST_POSTGRES_DSN` makes it run. It takes an empty database from
`internal/testutil/pgtest` and migrates it through `database.Open`, so it
compares what the *production* migration path produces on each engine rather
than what a test helper produced.

- Types: `TEXT`, `INTEGER`, `NUMERIC`, `BOOLEAN`, `TIMESTAMP`. `DATETIME`,
  `VARCHAR` and `JSON` have no PostgreSQL equivalent here — **JSON columns are
  `TEXT`**, because a text parameter cannot be assigned to a PostgreSQL `json`
  column without a cast at every write site.
- Boolean literals: `TRUE` / `FALSE`, never `1` / `0`.
- Time: `CURRENT_TIMESTAMP`, never `datetime('now')`. Timestamps are read back
  with `EXTRACT(EPOCH FROM …)::bigint` on PostgreSQL, which treats them as UTC —
  so never write one with a local-zone expression.
- Identifiers: quote reserved words — `"desc"` is a column name here, and
  PostgreSQL rejects it unquoted.
- Equality in a `CHECK` is `=`, not `==`.
- `DROP COLUMN`, `ON CONFLICT`, `CREATE INDEX`, `ALTER TABLE … ADD COLUMN` and
  partial indexes work on both.
- **Append-only.** `migrations/history_test.go` freezes the list of applied
  versions: never rename, renumber, reorder or delete an existing file, and a
  new file's timestamp must sort after every existing one. Editing an applied
  migration changes new installations only, while existing carts silently keep
  the old schema.

## Running locally

```bash
./scripts/migration dev up   # apply migrations + test fixtures
./scripts/migration dev down # rollback the most recent migration
```

## Test fixtures

Fixture SQL (created by `./scripts/migration dev up`) lives outside this
folder and populates the DB with demo products, an admin user and sample
carts. Never rely on fixture state from production migrations.
