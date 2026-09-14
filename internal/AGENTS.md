# internal/AGENTS.md

Private application code. Not importable outside this module.

## Layout

| Path | Role |
|------|------|
| `app.go` / `init.go` | Bootstraps Fiber, DB, mailer, routes. |
| `database/` | Connections, configuration and the SQL dialect layer — the only code that knows which engine is in use. |
| `dbtransfer/` | Backup, restore and SQLite → PostgreSQL copy: the dump format, its own pgx connection, `COPY` both ways. |
| `dbops.go` | `mycart db …`: files, gzip, migrating the target before a restore. |
| `install.go` | First-run installation, whichever database the wizard chose. |
| `handlers/private/` | Admin API handlers (`/api/_/…`). |
| `handlers/public/` | Public storefront API handlers (`/api/…`). |
| `middleware/` | Fiber middlewares (JWT auth, CORS, logging). |
| `models/` | Plain data structs shared by queries + handlers. |
| `queries/` | All SQL lives here. One file per logical domain. Written in one portable dialect; the wrapper translates it. |
| `routes/` | Router wiring + SPA fallback. |
| `mailer/` | SMTP + templated email rendering. |
| `webhook/` | Outbound webhook dispatch. |
| `testutil/` | Database fixture (SQLite or PostgreSQL) + Fiber harness. |

## Rules

1. **Handlers are thin.** They parse input, call `store` methods, format the
   response. Business logic belongs in `store/` which delegates to `store/db` operations.
2. **No direct database access in handlers.** Always use `internal/store` facade,
   never call `store/db` functions or database directly from handlers.
3. **No `http.DefaultClient`.** Webhooks use `webhook.sharedClient` which
   is built from `pkg/httpclient`.
4. **Pagination** uses `webutil.ParsePagination`. Clamp is `[1, 100]`.
5. **Setting keys** map to models via `setting_registry.go`. Adding a new
   setting group is a one-line change there — do not bring back the
   per-switch branches.
6. **Error taxonomy.** Return sentinel errors from `store` layer (e.g.
   `store.ErrAlreadyInstalled`), translate to HTTP codes in handlers
   with `errors.Is`.
6. **Sessions table** is idempotent (`INSERT … ON CONFLICT … DO UPDATE`) —
   callers may refresh the same key without a prior delete. `INSERT OR
   REPLACE` is a SQLite-only spelling and must not come back.
7. **Tests** use `internal/testutil` for a fresh, migrated DB per test and
   `t.Cleanup` for teardown. `TEST_DB_DRIVER=postgres` switches the whole
   suite to PostgreSQL — where pgtestdb clones a migrated template instead of
   running migrations per test — so a test must never open its own connection
   or assume SQLite. Always run with `-race`, and run the PostgreSQL pass
   before touching SQL or anything date-related.
8. **Never reach the raw handle in `queries/`.** `q.DB` is the dialect-aware
   wrapper; `.Raw()` is for goose and test fixtures only, and
   `internal/queries/wrapper_invariant_test.go` fails the build if it appears
   in the package.
9. **`app.go`'s process-wide state is atomic.** Development mode and the logger
   are `DevMode()`/`logger()` to read and `setDevMode`/`setLogger` to write.
   Never bring back plain package-level variables for them: server goroutines
   outlive `NewApp` and read both, which `-race` reports.
