# dure-mycart

## How to Run

### 1. Build from Source (Linux)

```bash
# Clone
git clone https://github.com/dure-one/dure-mycart.git
cd dure-mycart

# Build
go build -o dure-mycart

# Initialize
./dure-mycart init

# Run
./dure-mycart serve
```

Access at http://localhost:8080

### 2. Download Binary

```bash
# Download latest release
curl -L https://github.com/dure-one/dure-mycart/releases/latest/download/dure-mycart-linux-amd64 -o dure-mycart
chmod +x dure-mycart

# Initialize
./dure-mycart init

# Run
./dure-mycart serve
```

### 3. Docker

#### Without Prosody

```bash
docker run -v ./lc_base:/lc_base -v ./lc_digitals:/lc_digitals -v ./lc_uploads:/lc_uploads --rm ghcr.io/dure-one/dure-mycart:latest init

docker run --name dure-mycart --restart unless-stopped -p 8080:8080 -v ./lc_base:/lc_base -v ./lc_digitals:/lc_digitals -v ./lc_uploads:/lc_uploads ghcr.io/dure-one/dure-mycart:latest
```

**Image**: Built from `Dockerfile` → `ghcr.io/dure-one/dure-mycart:latest`

#### With XMPP Stack (Prosody + xmpp-proxy)

**3-Container Architecture:**
- `dure-mycart`: Pure HTTP server (port 80/tcp)
- `xmpp-proxy`: XMPP traffic - QUIC (443/udp), Direct TLS (5222, 5223, 5269/tcp)
- `prosody`: XMPP server backend

```bash
cd prosody-mycart-stack

# Copy and configure environment
cp .env.example .env
# Edit .env with your domain and settings

# Production (pull images)
docker compose up -d

# Development (build from source)
docker compose -f docker-compose.yml -f docker-compose.dev.yml up -d

# Testing (use test images)
docker compose -f docker-compose.yml -f docker-compose.test.yml up -d
```

**Environment Variables** (`.env`):
- `MYCART_IMAGE_TAG` - Image tag (latest/test/custom)
- `XMPP_PROXY_IMAGE_TAG` - xmpp-proxy image tag
- `XMPP_DOMAIN` - Your domain (required)
- `MYCART_DOMAIN` - Must match XMPP_DOMAIN for shared SSL
- `XMPP_ADMIN` - Admin user (e.g., admin@example.com)

**Images:**
- `ghcr.io/dure-one/dure-mycart:latest` - Main application
- `ghcr.io/nikescar/xmpp-proxy:latest` - XMPP reverse proxy
- `prosodyim/prosody:13.0` - XMPP server

---

## Database

### SQLite (Default)

No configuration needed. Database stored in `./lc_base/data.db`.

### PostgreSQL

#### Installation

```bash
./dure-mycart install \
  --email admin@example.com \
  --password yourpass \
  --domain localhost \
  --db postgres \
  --db-dsn 'postgres://user:password@host:5432/dure-mycart?sslmode=disable'
```

#### Runtime

```bash
# Via command line
./dure-mycart serve --db postgres --db-dsn 'postgres://...'

# Via environment variables
export MYCART_DB_DRIVER=postgres
export MYCART_DB_DSN='postgres://user:password@host:5432/dure-mycart?sslmode=disable'
./dure-mycart serve
```

#### Docker

```bash
docker run --name dure-mycart --restart unless-stopped -p 8080:8080 \
  -e MYCART_DB_DRIVER=postgres \
  -e MYCART_DB_DSN='postgres://user:password@host:5432/dure-mycart?sslmode=disable' \
  -v ./lc_base:/lc_base \
  -v ./lc_digitals:/lc_digitals \
  -v ./lc_uploads:/lc_uploads \
  ghcr.io/dure-one/dure-mycart:latest
```

---

## Optional sqlc Support

dure-mycart supports two SQL backends (choose at build time):

1. **Raw SQL (default)** - Hand-written queries, fast builds
2. **sqlc (optional)** - Compile-time type-safe queries

Both support SQLite + PostgreSQL and pass identical tests.

### Quick Start

```bash
# Install sqlc
make install-sqlc

# Generate type-safe code from SQL
make sqlc-generate

# Build with sqlc backend
make build-sqlc

# Test all combinations (raw+sqlc × SQLite+PostgreSQL)
make test-queries-all
```

**See [./optional_sqlc_support.md](./optional_sqlc_support.md) for:**
- When to use sqlc vs raw SQL
- Adding new queries
- PostgreSQL test setup

---

## First-time Setup

Visit http://localhost:8080/_/install or run:

```bash
./dure-mycart install --email admin@example.com --password yourpass --domain localhost
```

---

## Fork Hierarchy

This repository is a three-tier fork with the following feature additions:

| Repository | Branch | Added Features |
|------------|--------|----------------|
| `shurco/mycart` | `main` | • Product variants<br>• Product image ordering |
| `nikescar/mycart` | `main_dure` | • Optional sqlc support with build tag |
| `dure-one/dure-mycart` | `main` | • Seller info admin/site pages and API<br>• Responder (workflows, messages) admin/site pages and API<br>• Prosody docker-compose integrations |
