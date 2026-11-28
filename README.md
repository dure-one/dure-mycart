# dure-mycart

E-commerce platform with integrated XMPP customer support — Go + SvelteKit single-binary application.

## Overview

dure-mycart is a production-ready e-commerce system built as a downstream fork of [nikescar/mycart:main_dure](https://github.com/nikescar/mycart/tree/main_dure), which derives from [shurco/mycart:main](https://github.com/shurco/mycart). It runs from a single binary with embedded frontends, supports dual database backends (SQLite/PostgreSQL), and includes a complete XMPP-based customer support system powered by Prosody.

## Key Features

**Single Binary Deployment:**
Embedded admin and store frontends via `go:embed`. No separate frontend build or Node.js runtime required in production.

**Dual Database Support:**
SQLite (zero-config default) and PostgreSQL with identical feature parity. Switch at runtime via environment variables.

**XMPP Customer Support:**
Complete responder system with message archive sync (MAM), workflow automation, and admin panel. Integrates Prosody 13.0 with QUIC and Direct TLS support.

**Optional Type Safety:**
Choose between raw SQL (fast builds) or sqlc-generated type-safe queries at build time. Both backends pass identical test suites.

**Product Variants:**
Advanced product configuration inherited from upstream with extended workflow and responder integrations.

## Installation

**Docker (Recommended):**

Most users deploy via the `prosody-mycart` stack, which includes dure-mycart, xmpp-proxy, and Prosody in a production-ready configuration.

```bash
cd prosody-mycart-stack

# Configure environment
cp .env.example .env
# Edit .env: set XMPP_DOMAIN, MYCART_DOMAIN, XMPP_ADMIN

# Production deployment
docker compose up -d
```

**Images used:**
- `ghcr.io/dure-one/prosody-mycart:latest` - Combined stack (dure-mycart + xmpp-proxy + fail2ban-rs)
- `prosodyim/prosody:13.0` - XMPP server

**Build from Source (Linux):**

```bash
git clone https://github.com/dure-one/dure-mycart.git
cd dure-mycart
go build -o dure-mycart
./dure-mycart init
./dure-mycart serve
```

**Download Binary:**

```bash
curl -L https://github.com/dure-one/dure-mycart/releases/latest/download/dure-mycart-linux-amd64 -o dure-mycart
chmod +x dure-mycart
./dure-mycart init
./dure-mycart serve
```

## Fork Hierarchy

This repository is downstream of `nikescar/mycart:main_dure`, which is derived from `shurco/mycart:main`.

| Repository | Branch | Added Features |
|------------|--------|----------------|
| `shurco/mycart` | `main` | • Product variants<br>• Product image ordering |
| `nikescar/mycart` | `main_dure` | • Optional sqlc support with build tags<br>• sqlc-generated type-safe query layer<br>• SEO settings (sitemap, robots.txt, llms.txt, ai.txt)<br>• JWT compliance hardening<br>• Enhanced i18n translations |
| `dure-one/dure-mycart` | `main` | • Seller info admin/site pages and API<br>• Responder (workflows, messages) admin/site pages and API<br>• Prosody docker-compose integrations<br>• XMPP customer support system |

**Technical Note (sqlc imports):**

The sqlc-generated packages exist only in `dure-one/dure-mycart` and do not exist upstream. Import paths in generated code reference `github.com/dure-one/dure-mycart/internal/queries/sqlcgen/*` while the upstream `github.com/shurco/mycart` is imported as a dependency for base types. See commit `199b29c0` for the import path resolution.

## Quick Start

| Command | Purpose |
|---------|---------|
| `./dure-mycart init` | Initialize database and config |
| `./dure-mycart serve` | Start HTTP server (port 8080) |
| `./dure-mycart install --email <email> --password <pass> --domain <domain>` | CLI-based installation |
| `go test ./... -count=1 -race` | Run SQLite test suite |
| `make test-queries-all` | Test all query backends (raw+sqlc × SQLite+PostgreSQL) |

**First-time setup:**
Visit `http://localhost:8080/_/install` or use the CLI install command above.

## Database Configuration

**SQLite (Default):**

Zero configuration. Database stored at `./lc_base/data.db`.

**PostgreSQL:**

```bash
# Via environment variables (recommended)
export MYCART_DB_DRIVER=postgres
export MYCART_DB_DSN='postgres://user:password@host:5432/dure-mycart?sslmode=disable'
./dure-mycart serve

# Via command line
./dure-mycart serve --db postgres --db-dsn 'postgres://...'

# Docker deployment
cd prosody-mycart-stack
# Add to .env:
# MYCART_DB_DRIVER=postgres
# MYCART_DB_DSN=postgres://...
docker compose up -d
```

## Optional sqlc Support

Choose SQL backend at build time:

1. **Raw SQL (default)** - Hand-written queries, fast builds
2. **sqlc (optional)** - Compile-time type-safe generated queries

Both support SQLite + PostgreSQL with identical test coverage.

```bash
# Install sqlc
make install-sqlc

# Generate type-safe code from SQL
make sqlc-generate

# Build with sqlc backend
make build-sqlc

# Test all combinations
make test-queries-all
```

**Details:** See [./docs/optional_sqlc_support.md](./docs/optional_sqlc_support.md) for when to use sqlc vs raw SQL, adding new queries, and PostgreSQL test setup.

## XMPP Stack Architecture

The `prosody-mycart-stack` Docker Compose deployment provides a production-ready 3-container architecture:

**Services:**
- **prosody-mycart** (merged container): dure-mycart HTTP server + xmpp-proxy + fail2ban-rs
- **prosody**: XMPP server backend (172.19.0.2 on bridge network)

**Protocol Support:**
- XMPP QUIC (XEP-0467): 443/udp
- XMPP Direct TLS: 5222, 5223, 5269/tcp
- HTTP/HTTPS: 80, 443/tcp

**Features:**
- TLS certificates issued by dure-mycart (ACME), shared via `/certs` volume
- PROXY protocol v1 for preserving real client IPs
- S2S federation via Direct TLS (5269/tcp) and QUIC (443/udp)
- Message Archive Management (MAM) sync for customer support responder

**Configuration:**
See `prosody-mycart-stack/.env.example` for required environment variables.

## Development

**Frontend development:**

```bash
cd web/admin
bun run dev
```

**Testing:**

```bash
# SQLite tests
go test ./... -count=1 -race

# PostgreSQL tests
TEST_DB_DRIVER=postgres TEST_POSTGRES_DSN='postgres://...' go test ./... -count=1 -race
```

**Documentation:**
- `AGENTS.md` - Comprehensive development guide
- `internal/database/AGENTS.md` - Database layer architecture
- `migrations/AGENTS.md` - Portable SQL migration patterns
