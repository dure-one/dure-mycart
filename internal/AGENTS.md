# internal/AGENTS.md

Private application code. Not importable outside this module.

## Layout

| Path | Role |
|------|------|
| `app.go` / `init.go` | Bootstraps Fiber, DB, mailer, routes. |
| `base/` | Global runtime state (version, embed flags). |
| `config/` | Runtime configuration parsing. |
| `handlers/private/` | Admin API handlers (`/api/_/…`). |
| `handlers/public/` | Public storefront API handlers (`/api/…`). |
| `middleware/` | Fiber middlewares (JWT auth, CORS, logging). |
| `models/` | Plain data structs shared across layers. |
| `store/` | Business logic facade - handlers call this layer. |
| `store/db/` | Database abstraction layer (function pointers for database operations). |
| `routes/` | Router wiring + SPA fallback. |
| `mailer/` | SMTP + templated email rendering. |
| `webhook/` | Outbound webhook dispatch. |
| `testutil/` | In-memory SQLite fixture + Fiber harness. |

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
7. **Sessions table** is idempotent (`INSERT OR REPLACE`) — callers may
   refresh the same key without a prior delete.
8. **Tests** use `internal/testutil` for a fresh SQLite DB per test and
   `t.Cleanup` for teardown. Always run with `-race`.
