# Optional sqlc Support

myCart builds with one of two query implementations, chosen at build time:

| Build | Command | Queries |
|---|---|---|
| Plain (default) | `go build ./cmd` | Hand-written SQL in `internal/queries/*.go`, one portable set for both engines. This is the upstream code, unchanged. |
| sqlc | `go build -tags sqlc ./cmd` | sqlc-generated queries, checked against the schema at generation time. |

Both builds support SQLite and PostgreSQL, keep the same `queries.DB()` API and
pass the same test suite. Nothing outside `internal/queries` knows which one is
compiled in.

## How the switch works

The switch lives in `internal/queries`, one file at a time:

```
internal/queries/pages.go          //go:build !sqlc   plain implementation (upstream file)
internal/queries/pages_sqlc.go     //go:build sqlc    same types and methods, on sqlc
internal/queries/queries.go        no tag             shared: Base, New, SetConn, DB
internal/queries/cart_validate.go  no tag             shared: pure Go, no SQL
```

- Every plain file that runs SQL carries `//go:build !sqlc` and has a
  `<name>_sqlc.go` twin. Files without SQL (validation, cart items, purchase
  assembly, JSON helpers, pagination) stay shared and compile in both builds.
- A twin keeps the plain file's exported API and any helpers other shared files
  call. Pure-logic helpers that live in a tagged plain file are copied into the
  twin.
- Logic outside the package that runs its own SQL is re-implemented in the
  twin over a generated query, so the sqlc build sends no hand-written SQL:
  `GenerateUniqueSlug` uses `SlugTaken` instead of `pkg/slugify`.
- `sqlc_engine.go` picks the generated querier for the connection's engine
  (`pick` / `run`), and both engines run on `database.Conn` / `database.Tx`, so
  the UTC pin, `Config.Redacted()` and pgtestdb apply to the sqlc build as well.

## Releases

Shipped binaries are sqlc builds:

- `.goreleaser.yml` builds with `tags: [sqlc]`, and a post-build hook fails the
  release when `go version -m` does not show `build -tags=sqlc` for a binary.
- Onsite images are built with `-tags sqlc` and run the same check in the
  prosody-mycart stack.

`go build ./cmd` and the CI plain jobs keep building the plain layer, so the
upstream code stays tested.

## Layout

| Path | What |
|---|---|
| `sqlc.yaml` | Generator config. |
| `internal/queries/sqlcgen/queries/{sqlite,postgres}/*.sql` | The queries, one file per group and engine. |
| `internal/queries/sqlcgen/{sqlite,postgres}/` | Generated code (committed). Packages `sqlitegen` and `pggen`. |
| `internal/queries/sqlcgen/schema/sqlite.sql` | SQLite schema dump the generator reads, produced from `migrations/`. PostgreSQL reads `migrations/` directly. |
| `internal/queries/sqlc_engine.go` | Engine dispatch and paging helpers. |
| `internal/queries/sqlc_coverage_test.go` | Merge guards (see below). |

## Writing a query

Write the query in both engine files under the same name. Write it so both
engines generate **identical Go structs**: the twin maps the PostgreSQL row type
and converts the SQLite row into it (`pggen.XRow(sqliteRow)`), which only
compiles while the two match.

| Need | SQLite | PostgreSQL |
|---|---|---|
| Unix time | `CAST(COALESCE(strftime('%s', col), 0) AS INTEGER)` | `COALESCE(EXTRACT(EPOCH FROM col)::bigint, 0)::bigint` |
| `NUMERIC` / `INTEGER` column or param as `int64` | `CAST(x AS INTEGER)` | `x::bigint` |
| Boolean expression | `CAST(x AS BOOLEAN)` | `x::boolean` |
| ID list, only parameter | `IN (sqlc.slice(ids))` | `IN (SELECT jsonb_array_elements_text(sqlc.arg(ids)::jsonb))` |
| ID list among other parameters | `IN (SELECT value FROM json_each(CAST(sqlc.arg(ids) AS TEXT)))` | as above |
| Optional filter | `(CAST(sqlc.arg(all) AS BOOLEAN) OR col = sqlc.arg(v))` | `(sqlc.arg(all)::boolean OR col = sqlc.arg(v))` |
| No LIMIT | pass `-1` | pass `NULL` (`sqlc.narg(lim)`) — use `pageBounds` / `pgLimit` |
| `LIKE … ESCAPE '\'` | wrap in parentheses, cast the parameter to `TEXT` | wrap in parentheses |

Known sqlc limitations behind those rules:

- `= ANY($1::text[])` makes sqlc generate a `github.com/lib/pq` dependency.
- sqlc numbers repeated SQLite parameters (`?1`, `?2`), and a `sqlc.slice`
  expanded among numbered parameters binds the wrong values.
- sqlc's SQLite parser rejects `LIKE … ESCAPE` outside parentheses and cannot
  infer parameter types from a column of a `UNION` subquery.
- sqlc's SQLite parser cannot replay the migration history, hence the dump.

Regenerate after changing a query or a migration:

```bash
make sqlc-generate   # dumps the SQLite schema, then runs sqlc generate
make sqlc-verify     # fails on a stale dump or stale generated code, builds and vets -tags sqlc
```

## Testing

```bash
make test-queries-sqlc-sqlite
make test-queries-sqlc-postgres   # needs TEST_POSTGRES_DSN, see AGENTS.md
go test -tags sqlc ./... -count=1 -race
```

`internal/queries/*_test.go` run unchanged against both builds — that is the
parity check. `sql_golden_test.go` freezes the plain build's SQL text and is
tagged `!sqlc`; the `.sql` files plus `sqlc diff` cover the sqlc build.

## Merging upstream

Upstream (`main`) only ever changes the plain files. After a merge:

1. `go build -tags sqlc ./...` — a duplicate-method error means an upstream file
   now defines something a twin defines: tag it and update the twin.
2. `go test -tags sqlc ./internal/queries -run TestSQLC` —
   - `TestSQLCPortedFilesUnchanged` lists every tagged plain file whose content
     changed (by git blob hash). Port the change into its `_sqlc.go` twin, then
     update the hash in `ported` (`git hash-object <file>`). Files outside the
     package that a twin mirrors (`pkg/slugify/slug.go`) are in `mirrored`.
   - `TestSQLCCoverage` lists untagged files that run SQL: new upstream files,
     which the sqlc build would otherwise compile and run as plain SQL. Port
     them, or park them in `unported` for the time being.
3. New migrations: `make sqlc-generate`.
4. The full matrix: plain and sqlc, on SQLite and PostgreSQL.
