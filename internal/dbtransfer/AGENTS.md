# internal/dbtransfer

Moving a cart between a database and a file: `mycart db backup`, `db restore`
and `db copy`. PostgreSQL is read and written through its own wire protocol
(`COPY`, via `pgx`), never through `database/sql`: a transfer holds one
connection for the whole operation and needs the protocol, which is not SQL.
`internal/dbops.go` is the thin layer above this package — files, gzip,
migrations — and `cmd/main.go` is only flags and printing.

## The dump format

One file, valid SQL, readable by a person and by `psql`:

```
-- myCart database dump
-- {"magic":"mycart-dump","format":1,…}
--   psql -1 "$DSN" -c 'TRUNCATE TABLE … CASCADE;' -f <this file>
COPY "product" ("id", "name", "desc", …) FROM stdin;
p1	Shoes	Leather
\.
-- {"magic":"mycart-dump-trailer","tables":[…],"rows":48}
```

- The interchange format is **COPY text**, not CSV: it is line-oriented (a row
  is a line, a table ends at `\.`), it escapes the newlines and tabs inside a
  value, and PostgreSQL's own parser reads it back. `copytext.go` renders what
  SQLite's driver hands over (`0`/`1` for `BOOLEAN`, `time.Time` for
  `TIMESTAMP`) the way PostgreSQL renders it.
- The header and the trailer are JSON behind `-- `, so the file stays a SQL
  script. The trailer carries the row count of every table and is **required**:
  it is written last, and it is the only thing that notices a file that lost
  its end.
- Every identifier is quoted (`quoteIdent`), because the schema has a column
  named `desc`.
- `FormatVersion` is the format's own version and is separate from the schema
  version in the same header; `Load` refuses a dump in a newer format, or one
  whose schema version is ahead of what this build migrates to.

## Rules

- **Never copy the bookkeeping tables** (`migrate_db_version`,
  `migrate_fixtures_version`). The migration history belongs to the target,
  which rebuilds it by running the migrations; a copied one claims migrations
  that never ran there.
- **A load is one transaction**: `TRUNCATE` every non-bookkeeping table of the
  target, then `COPY` each section, then `COUNT(*)` per table, then `COMMIT`.
  A migrated database is *not* empty (the migrations seed `setting` and
  `page`), so the `TRUNCATE` is part of restoring, not a setting.
- **The guard is `setting.installed`**: a target holding an installation is
  refused unless `Replace` is set, and the refusal happens before anything is
  written.
- Tables are written in foreign-key order (`orderTables`, deterministic; a
  cycle is an error, not a guess).
- A transfer's session is pinned to UTC in `connectPostgres`, more strictly
  than the application's: a dump is text, and PostgreSQL prints a `TIMESTAMP`
  in the session's zone.
- Tests come in two halves: the format (parsing, rendering, ordering) runs
  anywhere, and everything that touches a server skips itself unless
  `TEST_DB_DRIVER=postgres` and `TEST_POSTGRES_DSN` are set — see
  `internal/testutil/pgtest`. Run both passes before changing anything here.
