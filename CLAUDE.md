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

**Branch**: `main`

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

**Prosody-myCart Stack** (`prosody-mycart-stack/`):
- All-in-one Docker image: dure-mycart + fail2ban-rs + Horust
- **ALPN Router**: Routes XMPP/HTTP on port 443 based on TLS ALPN protocol
- PROXY protocol v1 for preserving real client IPs in Prosody logs
- Bridge network with static IPs (172.19.0.0/16)
- Prosody static IP: 172.19.0.2 (configured in docker-compose.yml)

**Hybrid S2S Federation**:
- **Modern servers** (XEP-0368): Port 443 with `xmpp-server` ALPN → Prosody 5270
- **Legacy servers**: Direct S2S on port 5269 (exposed from Prosody container)
- DNS SRV: `_xmpps-server._tcp` → port 443, `_xmpp-server._tcp` → port 5269

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
