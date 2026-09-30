# Project Context

myCart e-commerce platform - Go + SvelteKit single-binary application.

## Repository Structure

Three-tier fork:
- **Upstream**: `github.com/shurco/mycart/main` (original project)
- **Fork**: `github.com/nikescar/mycart/main` + `main_dure` (personal integration)
- **Origin**: `github.com/dure-one/dure-mycart/main` (team production)

**Branching:**
- Feature branches → based on `origin/main`
- PRs → target `origin/main`
- Never directly compare to `main` branch (use `origin/main`)

## Tech Stack

- **Backend**: Go 1.26, Fiber v3, SQLite/PostgreSQL
- **Frontend**: SvelteKit, Svelte 5 (runes), TailwindCSS v4
- **Build**: Single binary with embedded frontends (`go:embed`)

## Key Files

- `AGENTS.md` - Comprehensive development guide
- `internal/database/AGENTS.md` - Database layer guide
- Feature branches in `feat/*` pattern

## Current Work

**Branch**: `feat/responder-system` (22 commits ahead of `origin/main`)

XMPP-based customer support responder system:
- Database schema: `migrations/20261001000000_responder_tables.sql`
- Backend: `internal/responder/` (XMPP worker, cron runner)
- Queries: `internal/queries/responder*.go`
- Handlers: `internal/handlers/private/responder.go`
- Frontend: `web/admin/src/routes/responder/*`
- Settings: XMPP connection config in admin panel

**Architecture:**
1. XMPP worker syncs messages via MAM (Message Archive Management)
2. Cron runner executes scheduled workflows
3. Contact/message/workflow queries with SQLite/PostgreSQL dual support
4. Admin panel for configuration and message viewing

## Development Commands

```bash
# Test both engines
go test ./... -count=1 -race
TEST_DB_DRIVER=postgres TEST_POSTGRES_DSN='...' go test ./... -count=1 -race

# Frontend
cd web/admin && bun run dev

# Run locally
go run ./cmd serve
```

## Notes

- Always use `origin/main` as base branch (not `main`)
- Write portable SQL (see `migrations/AGENTS.md`)
- Test on both SQLite and PostgreSQL
- Follow Svelte 5 runes syntax (`$state`, `$derived`, etc.)
