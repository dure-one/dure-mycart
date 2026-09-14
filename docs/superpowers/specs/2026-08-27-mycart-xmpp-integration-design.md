# mycart + XMPP Integration Design

**Date:** 2026-08-27  
**Author:** Claude Sonnet 4.5  
**Status:** Design Approved

## Overview

This spec defines the architecture for integrating mycart (Go e-commerce application) with xmpp-proxy infrastructure using a merged container approach. The design eliminates nginx and acme.sh by embedding the mycart binary directly into the xmpp-proxy-stack container, leveraging mycart's built-in autocert for SSL certificate management and fail2ban-rs for rate limiting protection.

## Goals

1. **Unified deployment:** Single container combining mycart + xmpp-proxy + fail2ban-rs
2. **SSL automation:** mycart's autocert handles Let's Encrypt certificates
3. **Rate limiting:** fail2ban-rs protects both HTTP/HTTPS and XMPP traffic
4. **Single domain:** Both mycart and XMPP services on same domain
5. **Minimal dependencies:** Remove nginx and acme.sh from stack

## Architecture

### System Overview

```
┌─────────────────────────────────────────────────────────────┐
│ Internet                                                     │
└───────┬─────────────────────────────────────┬───────────────┘
        │                                     │
        │ All traffic                         │
        ▼                                     │
┌───────────────────────────────────────────┐ │
│ xmpp-proxy-stack (host network)          │ │
│                                           │ │
│ ┌─────────────────────────────────────┐  │ │
│ │ Horust (process supervisor)         │  │ │
│ │                                     │  │ │
│ │ ├─ mycart :80/:443 ◄────────────────┼──┘ │
│ │ │  • Go binary (Fiber framework)   │    │
│ │ │  • autocert (Let's Encrypt)      │    │
│ │ │  • E-commerce app                │    │
│ │ │                                  │    │
│ │ ├─ xmpp-proxy :5222/:5223/:5269   │◄───┘
│ │ │  • Rust binary                   │
│ │ │  • XMPP protocol proxy           │
│ │ │  • PROXY protocol support        │
│ │ │                                  │
│ │ └─ fail2ban-rs                     │
│ │    • Rate limiting                 │
│ │    • IP banning (nftables)         │
│ └─────────────────────────────────────┘
└──────────────┬────────────────────────────┘
               │
               │ Internal network
               ▼
         ┌─────────────┐
         │ prosody     │
         │ :5222/:5269 │
         │             │
         │ XMPP server │
         └─────────────┘

Shared Volumes:
/srv/data/certs     → SSL certificates (mycart writes, others read)
/srv/data/logs      → Application logs
/srv/data/mycart/*  → mycart data (lc_base, lc_uploads, lc_digitals)
/srv/data/prosody   → prosody XMPP data
/srv/data/fail2ban  → fail2ban-rs state
```

### Key Design Decisions

**Decision 1: Merge mycart into xmpp-proxy-stack**
- **Rationale:** Simplifies deployment, reduces container count, enables fail2ban-rs to protect mycart
- **Trade-off:** Tighter coupling (mycart updates require rebuilding entire image)

**Decision 2: Remove nginx**
- **Rationale:** mycart (Fiber framework) handles HTTP/HTTPS directly, eliminating reverse proxy overhead
- **Trade-off:** No nginx-specific features (but mycart's Fiber provides equivalent functionality)

**Decision 3: Remove acme.sh, use mycart's autocert**
- **Rationale:** Go's `autocert` package handles Let's Encrypt challenges automatically
- **Trade-off:** Tied to mycart's lifecycle (if mycart crashes, cert renewal paused)

**Decision 4: Single domain for all services**
- **Rationale:** Simplifies SSL certificate management (one cert covers everything)
- **Trade-off:** Path-based routing required (/, /_/, /api/ for mycart; dedicated ports for XMPP)

**Decision 5: Host networking mode**
- **Rationale:** Required for PROXY protocol support (preserves client IPs for XMPP), allows fail2ban-rs to manage host nftables
- **Trade-off:** Less isolation, ports bound directly to host

## Components

### 1. xmpp-proxy-stack Container

**Purpose:** Unified infrastructure container combining web application, XMPP proxy, and security.

**Processes (managed by Horust):**
1. **mycart** - Go binary (Fiber web framework)
   - Listens: `:80` (HTTP), `:443` (HTTPS)
   - Features: autocert, e-commerce app, admin panel
   - Serves: `/` (site), `/_/` (admin), `/api/` (API), `/uploads/` (static uploads)
   
2. **xmpp-proxy** - Rust binary
   - Listens: `:5222` (C2S), `:5223` (C2S Direct TLS), `:5269` (S2S), `:443/udp` (QUIC)
   - Proxies to: prosody at `127.0.0.1:15222` and `127.0.0.1:15269`
   
3. **fail2ban-rs** - Rust binary
   - Monitors: HTTP/HTTPS logs (mycart), XMPP logs (xmpp-proxy)
   - Actions: Ban abusive IPs via nftables

**Dockerfile Structure:**
```
Stage 1: frontend-builder (Node.js)
  → Build mycart admin + site frontends (Vite)

Stage 2: backend-builder (Go)
  → Build mycart binary with embedded frontend assets

Stage 3: xmpp-binaries-builder (Debian)
  → Download xmpp-proxy + fail2ban-rs from GitHub releases

Stage 4: horust-builder (Alpine)
  → Download Horust process supervisor

Stage 5: tools-builder (Debian)
  → busybox, openssl, curl, nft, envsubst

Final Stage: distroless/base-debian13
  → Copy all binaries (mycart, xmpp-proxy, fail2ban-rs, horust)
  → Copy Horust service configs
  → Copy tools (busybox, openssl, curl, nft)
  → NO nginx, NO acme.sh
```

**Network:** Host networking

**Capabilities:** `NET_ADMIN` (for fail2ban-rs nftables management)

---

### 2. prosody Container

**Purpose:** XMPP server backend.

**Image:** `prosodyim/prosody:13.0`

**Network:** Bridge network `xmpp-internal`

**Port Exposure:**
- `127.0.0.1:15222:5222` - C2S (localhost only, xmpp-proxy connects here)
- `127.0.0.1:15269:5269` - S2S (localhost only, xmpp-proxy connects here)
- `0.0.0.0:5280:5280` - HTTP/WebSocket (public, for WebSocket clients)

**Volumes:**
- `/var/lib/prosody` → `/srv/data/prosody` (user accounts, message archives)
- `/certs` → `/srv/data/certs:ro` (SSL certificates, read-only)
- `/var/log/prosody` → `/srv/data/logs/prosody` (logs)
- `/etc/prosody/conf.d/proxy.cfg.lua` → `./generated/proxy.cfg.lua:ro` (config)

**Dependencies:**
- `prosody-modules-init` (completed)
- `prosody-config-init` (completed)

---

### 3. prosody-modules-init (Init Container)

**Purpose:** Clone prosody community modules on first start.

**Image:** `alpine:latest`

**Behavior:**
- Checks if `prosody-modules/` already exists
- If not: installs mercurial, clones `https://hg.prosody.im/prosody-modules`
- Fetches `mod_admin_web2` vendored dependencies
- Runs once, then exits

**Same as reference docker-compose.**

---

### 4. prosody-config-init (Init Container)

**Purpose:** Render prosody config template with environment variables.

**Image:** `alpine:latest`

**Behavior:**
- Reads `xmpp-proxy-stack/render-prosody-config.sh`
- Substitutes `${XMPP_DOMAIN}` from `.env`
- Outputs to `generated/proxy.cfg.lua`
- Runs on every start (cheap, no caching)

**Same as reference docker-compose.**

---

## Network Configuration

### Host Networking (xmpp-proxy-stack)

**Ports bound to host:**
- `80/tcp` - mycart HTTP (autocert challenges + redirect to HTTPS)
- `443/tcp` - mycart HTTPS (app traffic)
- `5222/tcp` - xmpp-proxy C2S
- `5223/tcp` - xmpp-proxy C2S Direct TLS
- `5269/tcp` - xmpp-proxy S2S
- `443/udp` - xmpp-proxy QUIC

**Rationale for host networking:**
1. PROXY protocol support (preserves client IPs for XMPP)
2. fail2ban-rs requires access to host's nftables (`cap_add: NET_ADMIN`)
3. Simplifies port management (no port mapping conflicts)

**Connectivity:**
- **To prosody:** Via `127.0.0.1:15222` and `127.0.0.1:15269`
- **To internet:** Direct

---

### Bridge Networking (prosody)

**Network:** `xmpp-internal` (Docker bridge network)

**Port exposure:**
- `127.0.0.1:15222:5222` - Only accessible from host (xmpp-proxy-stack can connect)
- `127.0.0.1:15269:5269` - Only accessible from host
- `0.0.0.0:5280:5280` - Publicly accessible (WebSocket clients)

**Rationale:**
- Isolates prosody from direct internet access (security)
- Only xmpp-proxy-stack can reach XMPP ports
- WebSocket port public for browser-based XMPP clients

---

## Volume Configuration

### Volume Structure

```
/srv/data/
├── certs/                    # SSL certificates (shared)
│   ├── fullchain.pem         # Written by mycart autocert
│   ├── privkey.pem           # Written by mycart autocert
│   └── lc_certs/             # autocert cache directory
├── logs/                     # Application logs
│   ├── mycart.log            # mycart application logs
│   ├── xmpp-proxy.log        # xmpp-proxy logs
│   ├── fail2ban.log          # fail2ban-rs logs
│   └── prosody/              # prosody logs subdirectory
│       ├── prosody.log
│       └── prosody.err
├── fail2ban/                 # fail2ban-rs state
│   └── (banned IPs database, nftables state)
├── mycart/                   # mycart application data
│   ├── lc_base/              # SQLite database, configuration
│   ├── lc_uploads/           # Product images, user uploads
│   └── lc_digitals/          # Downloadable products (not served statically)
└── prosody/                  # prosody XMPP data
    └── (user accounts, message archives, internal storage)
```

---

### Volume Mappings

**xmpp-proxy-stack:**
```yaml
volumes:
  - /srv/data/certs:/certs
  - /srv/data/logs:/logs
  - /srv/data/fail2ban:/var/lib/fail2ban-rs
  - /srv/data/mycart/lc_base:/app/lc_base
  - /srv/data/mycart/lc_uploads:/app/lc_uploads
  - /srv/data/mycart/lc_digitals:/app/lc_digitals
```

**prosody:**
```yaml
volumes:
  - /srv/data/prosody:/var/lib/prosody
  - /srv/data/certs:/certs:ro
  - /srv/data/logs/prosody:/var/log/prosody
  - ./generated/proxy.cfg.lua:/etc/prosody/conf.d/proxy.cfg.lua:ro
```

---

### Ownership & Permissions

**Host directory setup:**
```bash
# Create all directories
sudo mkdir -p /srv/data/{certs,logs/prosody,fail2ban,mycart/{lc_base,lc_uploads,lc_digitals},prosody}

# Set ownership
sudo chown -R root:root /srv/data/{certs,logs,fail2ban,mycart}
sudo chown -R 999:999 /srv/data/prosody  # prosody user UID
```

**Container user contexts:**
- **xmpp-proxy-stack:** Runs as `root` (requires `NET_ADMIN` for nftables)
- **prosody:** Runs as `prosody` user (UID 999)

---

## Certificate Flow

### Automatic SSL Certificate Management

1. **mycart starts** with autocert enabled for `${MYCART_DOMAIN}`
   ```go
   manager := &autocert.Manager{
       Prompt:     autocert.AcceptTOS,
       HostPolicy: autocert.HostWhitelist(hostOnly),
       Cache:      autocert.DirCache("./lc_certs"),
   }
   ```

2. **Let's Encrypt challenges** arrive at `http://example.com/.well-known/acme-challenge/...`
   - mycart's autocert handler responds on port `:80`

3. **Certificate issued** by Let's Encrypt
   - mycart writes to `/certs/lc_certs/example.com` (autocert cache)
   - Creates symlinks: `/certs/fullchain.pem` and `/certs/privkey.pem`

4. **xmpp-proxy reads** certs from `/certs/fullchain.pem` and `/certs/privkey.pem`
   - Used for XMPP TLS (ports 5223, 5269)

5. **prosody reads** certs from `/certs/fullchain.pem` and `/certs/privkey.pem`
   - Used for XMPP server-to-server TLS

6. **Automatic renewal:** autocert handles renewals ~30 days before expiration

---

### Certificate Coordination

**Potential issue:** If mycart restarts during cert renewal, temporary downtime.

**Mitigation:**
- autocert caches certs in `/certs/lc_certs/`
- On restart, mycart reads from cache (no re-acquisition needed)
- Renewals happen 30 days before expiration (plenty of buffer)

**Shared certificate strategy:**
- mycart owns cert generation (writer)
- xmpp-proxy and prosody are readers only
- All services use the same cert for `${XMPP_DOMAIN}` / `${MYCART_DOMAIN}` (must be identical)

---

## Docker Compose Structure

### File: `docker-compose.yaml`

```yaml
services:
  # Init service to set up Prosody community modules
  prosody-modules-init:
    image: alpine:latest
    container_name: prosody-modules-init
    restart: "no"
    working_dir: /workspace
    volumes:
      - ./:/workspace
    command: >
      sh -c '
        echo "=== Prosody Modules Initialization ===" &&
        if [ -f prosody-modules/mod_admin_web2/admin_web2/mod_admin_web2.lua ] && [ -f prosody-modules/mod_admin_web2/admin_web2/www_files/js/strophe.min.js ]; then
          echo "✓ prosody-modules already initialized" &&
          exit 0
        fi &&
        echo "Installing mercurial..." &&
        apk add --no-cache mercurial &&
        echo "Cloning prosody-modules repository..." &&
        rm -rf prosody-modules &&
        hg clone https://hg.prosody.im/prosody-modules prosody-modules &&
        echo "✓ Modules cloned successfully" &&
        echo "Fetching admin_web2 vendored JS/CSS dependencies..." &&
        (cd prosody-modules/mod_admin_web2/admin_web2 && sh get_deps.sh) &&
        echo "✓ admin_web2 dependencies fetched" &&
        ls -la prosody-modules/ | head -20
      '

  # Renders prosody-proxy.cfg.lua.template -> generated/proxy.cfg.lua
  prosody-config-init:
    image: alpine:latest
    container_name: prosody-config-init
    restart: "no"
    working_dir: /workspace
    env_file: .env
    volumes:
      - ./:/workspace
    command: sh xmpp-proxy-stack/render-prosody-config.sh

  # Prosody XMPP server
  prosody:
    image: prosodyim/prosody:13.0
    container_name: prosody
    restart: unless-stopped
    env_file: .env
    environment:
      PROSODY_ADMINS: ${XMPP_ADMIN:-admin@${XMPP_DOMAIN}}
      PROSODY_VIRTUAL_HOSTS: ${XMPP_DOMAIN}
      PROSODY_LOGLEVEL: ${PROSODY_LOGLEVEL:-info}
      PROSODY_STORAGE: internal
      PROSODY_ENABLE_MODULES: mam,carbons,csi_simple,ping,admin_adhoc,admin_shell,http,admin_web2,bosh,websocket
      PROSODY_CERTIFICATES: /certs
      PROSODY_RETENTION_DAYS: ${PROSODY_RETENTION_DAYS:-90}
    volumes:
      - /srv/data/prosody:/var/lib/prosody
      - /srv/data/certs:/certs:ro
      - /srv/data/logs/prosody:/var/log/prosody
      - ./generated/proxy.cfg.lua:/etc/prosody/conf.d/proxy.cfg.lua:ro
      - ./prosody-modules:/usr/lib/prosody/community:ro
    ports:
      - "127.0.0.1:15222:5222"  # C2S (localhost only)
      - "127.0.0.1:15269:5269"  # S2S (localhost only)
      - "0.0.0.0:5280:5280"     # HTTP/WebSocket (public)
    networks:
      - xmpp-internal
    healthcheck:
      test: ["CMD", "prosodyctl", "status"]
      interval: 30s
      timeout: 10s
      retries: 3
      start_period: 10s
    depends_on:
      prosody-modules-init:
        condition: service_completed_successfully
      prosody-config-init:
        condition: service_completed_successfully

  # Merged stack: mycart + xmpp-proxy + fail2ban-rs
  xmpp-proxy-stack:
    build: ./xmpp-proxy-stack
    container_name: xmpp-proxy-stack
    restart: unless-stopped
    network_mode: host
    cap_add:
      - NET_ADMIN
    env_file: .env
    volumes:
      - /srv/data/certs:/certs
      - /srv/data/logs:/logs
      - /srv/data/fail2ban:/var/lib/fail2ban-rs
      - /srv/data/mycart/lc_base:/app/lc_base
      - /srv/data/mycart/lc_uploads:/app/lc_uploads
      - /srv/data/mycart/lc_digitals:/app/lc_digitals
    depends_on:
      prosody:
        condition: service_healthy

networks:
  xmpp-internal:
    driver: bridge
```

---

### File: `.env` (Environment Variables)

```bash
# Domain configuration
XMPP_DOMAIN=example.com
MYCART_DOMAIN=example.com  # Must match XMPP_DOMAIN for shared SSL cert

# XMPP configuration
XMPP_ADMIN=admin@example.com
PROSODY_LOGLEVEL=info
PROSODY_RETENTION_DAYS=90

# mycart configuration
MYCART_HTTP_ADDR=0.0.0.0:80
MYCART_HTTPS_ADDR=0.0.0.0:443

# xmpp-proxy configuration
XMPP_PROXY_PROSODY_C2S=127.0.0.1:15222
XMPP_PROXY_PROSODY_S2S=127.0.0.1:15269

# fail2ban-rs configuration
FAIL2BAN_MAX_RETRY=5
FAIL2BAN_FIND_TIME=600
FAIL2BAN_BAN_TIME=3600
```

---

## Build Context Structure

```
/home/wj/work/dure-mycart/
├── docker-compose.yaml           # Main compose file
├── .env                          # Environment variables
├── .env.example                  # Template
├── xmpp-proxy-stack/             # Custom Dockerfile directory
│   ├── Dockerfile                # Merged: mycart + xmpp-proxy + fail2ban
│   ├── docker-entrypoint.sh      # Modified entrypoint (no nginx, no acme.sh)
│   ├── horust-services/          # Horust process configs
│   │   ├── mycart.toml           # NEW: Run mycart binary
│   │   ├── xmpp-proxy.toml       # Run xmpp-proxy binary
│   │   └── fail2ban-rs.toml      # Run fail2ban-rs binary
│   ├── templates/                # Config templates (if needed)
│   └── render-prosody-config.sh  # Copy from xmpp-proxy reference
├── generated/                     # Rendered configs
│   └── proxy.cfg.lua             # Prosody config (rendered by prosody-config-init)
├── prosody-modules/              # Cloned by prosody-modules-init
│   └── (mercurial repo)
├── scripts/                      # Helper scripts
│   └── setup-volumes.sh          # Initialize /srv/data structure
├── cmd/                          # mycart source (for build context)
│   └── main.go
├── internal/                     # mycart source
├── web/                          # mycart frontends
│   ├── admin/                    # Admin panel (Vite)
│   └── site/                     # Site frontend (Vite)
├── go.mod
├── go.sum
└── Dockerfile                    # Original standalone mycart Dockerfile (can keep for reference)
```

---

## Horust Service Configurations

### File: `xmpp-proxy-stack/horust-services/mycart.toml`

```toml
[mycart]
command = "/app/mycart"
args = ["serve", "--http", "0.0.0.0:80", "--https", "0.0.0.0:443"]
working_directory = "/app"
start_delay = "2s"
start_after = []
restart_strategy = "always"
restart_backoff = "10s"
restart_attempts = 5

[mycart.environment]
MYCART_DOMAIN = "${MYCART_DOMAIN}"
GIN_MODE = "release"

[mycart.healthiness]
http_endpoint = "http://127.0.0.1:80/health"
```

---

### File: `xmpp-proxy-stack/horust-services/xmpp-proxy.toml`

```toml
[xmpp-proxy]
command = "/usr/local/bin/xmpp-proxy"
args = ["--c2s", "127.0.0.1:15222", "--s2s", "127.0.0.1:15269", "--cert", "/certs/fullchain.pem", "--key", "/certs/privkey.pem"]
working_directory = "/etc/xmpp-proxy"
start_delay = "5s"
start_after = ["mycart"]  # Wait for mycart to generate certs
restart_strategy = "always"
restart_backoff = "10s"
restart_attempts = 5

[xmpp-proxy.environment]
XMPP_DOMAIN = "${XMPP_DOMAIN}"
RUST_LOG = "info"
```

---

### File: `xmpp-proxy-stack/horust-services/fail2ban-rs.toml`

```toml
[fail2ban-rs]
command = "/usr/local/bin/fail2ban-rs"
args = ["--config", "/etc/fail2ban-rs/config.toml"]
working_directory = "/var/lib/fail2ban-rs"
start_delay = "3s"
start_after = []
restart_strategy = "always"
restart_backoff = "10s"
restart_attempts = 5

[fail2ban-rs.environment]
FAIL2BAN_MAX_RETRY = "${FAIL2BAN_MAX_RETRY:-5}"
FAIL2BAN_FIND_TIME = "${FAIL2BAN_FIND_TIME:-600}"
FAIL2BAN_BAN_TIME = "${FAIL2BAN_BAN_TIME:-3600}"
```

---

## Dockerfile Implementation

### High-Level Structure

```dockerfile
# Stage 1: frontend-builder
FROM node:22-alpine AS frontend-builder
# Build mycart admin + site frontends with Vite
WORKDIR /app
COPY web/admin/package*.json ./web/admin/
COPY web/site/package*.json ./web/site/
RUN cd web/admin && npm ci --legacy-peer-deps
RUN cd web/site && npm ci --legacy-peer-deps
COPY web/ ./web/
RUN cd web/admin && npx vite build
RUN cd web/site && npx vite build

# Stage 2: backend-builder
FROM golang:1.26-alpine AS backend-builder
# Build mycart binary with embedded frontend
WORKDIR /go/src/app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=frontend-builder /app/web/admin/build ./web/admin/build
COPY --from=frontend-builder /app/web/site/build ./web/site/build
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o /go/bin/mycart ./cmd/main.go

# Stage 3: xmpp-binaries-builder
FROM debian:13-slim AS xmpp-binaries-builder
# Download xmpp-proxy + fail2ban-rs
# (Copy implementation from reference xmpp-proxy-stack Dockerfile)

# Stage 4: horust-builder
FROM alpine:latest AS horust-builder
# Download Horust process supervisor
# (Copy implementation from reference xmpp-proxy-stack Dockerfile)

# Stage 5: tools-builder
FROM debian:13-slim AS tools-builder
# Install busybox, openssl, curl, nft, envsubst
# Copy Horust service configs
COPY horust-services/ /etc/horust/services/
COPY docker-entrypoint.sh /docker-entrypoint.sh

# Final Stage: distroless
FROM gcr.io/distroless/base-debian13:latest
# Copy mycart binary
COPY --from=backend-builder /go/bin/mycart /app/mycart
# Copy xmpp binaries
COPY --from=xmpp-binaries-builder /usr/local/bin/xmpp-proxy /usr/local/bin/xmpp-proxy
COPY --from=xmpp-binaries-builder /usr/local/bin/fail2ban-rs /usr/local/bin/fail2ban-rs
# Copy Horust
COPY --from=horust-builder /usr/local/bin/horust /usr/local/bin/horust
# Copy tools
COPY --from=tools-builder /usr/bin/openssl /usr/bin/openssl
COPY --from=tools-builder /usr/bin/curl /usr/bin/curl
COPY --from=tools-builder /usr/sbin/nft /usr/sbin/nft
COPY --from=tools-builder /bin/busybox /bin/busybox
COPY --from=tools-builder /etc/horust/services /etc/horust/services
COPY --from=tools-builder /docker-entrypoint.sh /docker-entrypoint.sh
# ... (library copies, directory creation, etc.)

EXPOSE 80 443 5222 5223 5269
ENTRYPOINT ["/docker-entrypoint.sh"]
```

**Note:** Full Dockerfile implementation will be detailed in the implementation plan.

---

## Entrypoint Behavior

### File: `xmpp-proxy-stack/docker-entrypoint.sh`

**Modified from reference xmpp-proxy-stack:**

```bash
#!/bin/busybox sh
set -e

echo "=== xmpp-proxy-stack initialization (mycart edition) ==="

# Check required environment variables
if [ -z "${XMPP_DOMAIN:-}" ]; then
    echo "ERROR: XMPP_DOMAIN not set" >&2
    exit 1
fi

if [ -z "${MYCART_DOMAIN:-}" ]; then
    echo "ERROR: MYCART_DOMAIN not set" >&2
    exit 1
fi

# Ensure XMPP_DOMAIN and MYCART_DOMAIN match (shared SSL cert)
if [ "${XMPP_DOMAIN}" != "${MYCART_DOMAIN}" ]; then
    echo "ERROR: XMPP_DOMAIN and MYCART_DOMAIN must match for shared SSL cert" >&2
    exit 1
fi

# Check volume permissions
echo "Checking volume permissions..."
for dir in /certs /logs /var/lib/fail2ban-rs /app/lc_base /app/lc_uploads /app/lc_digitals; do
    if ! touch "${dir}/.write-test" 2>/dev/null; then
        echo "ERROR: No write permission to ${dir}" >&2
        echo "Fix: sudo chown -R root:root /srv/data/{certs,logs,fail2ban,mycart}" >&2
        exit 1
    fi
    rm -f "${dir}/.write-test"
done

echo "✓ Volume permissions OK"

# mycart's autocert will handle SSL certificates automatically
# No need to generate self-signed certs or run acme.sh

echo "Starting services via Horust..."
exec /usr/local/bin/horust --services-path /etc/horust/services
```

**Key changes from reference:**
- ✅ Removed acme.sh certificate acquisition logic
- ✅ Removed nginx start/stop logic
- ✅ Added `MYCART_DOMAIN` check
- ✅ Added validation that `XMPP_DOMAIN == MYCART_DOMAIN`
- ✅ Removed self-signed cert generation (mycart's autocert handles it)

---

## Comparison with Reference

| Aspect | Reference (xmpp-proxy) | This Design |
|--------|------------------------|-------------|
| **Stack image** | `ghcr.io/nikescar/xmpp-proxy-stack:latest` | Build locally from `./xmpp-proxy-stack/` |
| **HTTP/HTTPS handler** | nginx | mycart (Fiber framework) |
| **SSL certificates** | acme.sh | mycart's autocert |
| **Processes in stack** | nginx, xmpp-proxy, fail2ban-rs, acme.sh | mycart, xmpp-proxy, fail2ban-rs |
| **Process supervisor** | Horust | Horust (same) |
| **Application** | N/A | mycart (embedded in stack) |
| **Volume root** | `/srv/xmpp/*` | `/srv/data/*` |
| **Domain strategy** | XMPP only | Shared domain (XMPP + web) |
| **Container count** | 2 main (prosody, xmpp-proxy-stack) + 2 init | Same |
| **Build complexity** | Single image (published) | Multi-stage (local build) |

---

## Risks and Mitigations

### Risk 1: mycart crash breaks cert renewal

**Impact:** If mycart crashes during Let's Encrypt renewal window, cert expires.

**Likelihood:** Low (autocert renews 30 days before expiration, plenty of buffer)

**Mitigation:**
- Horust auto-restarts mycart on crash
- autocert caches certs in `/certs/lc_certs/` - survives restarts
- Monitor mycart uptime, set up alerts

---

### Risk 2: Tight coupling of application and infrastructure

**Impact:** mycart updates require rebuilding entire xmpp-proxy-stack image.

**Likelihood:** High (by design)

**Mitigation:**
- Keep Dockerfile multi-stage for fast rebuilds (only rebuild changed stages)
- Consider CI/CD pipeline for automated builds
- Alternative: Revert to separate containers if coupling becomes problematic

---

### Risk 3: Host networking security

**Impact:** Less isolation, exposed ports bound directly to host.

**Likelihood:** Medium (mitigated by fail2ban-rs)

**Mitigation:**
- fail2ban-rs provides rate limiting and IP banning
- Firewall rules on host (allow only 80, 443, 5222, 5223, 5269)
- Regular security updates

---

### Risk 4: Certificate coordination race condition

**Impact:** xmpp-proxy or prosody start before mycart generates certs.

**Likelihood:** Low (Horust `start_after` dependency)

**Mitigation:**
- Horust config: `xmpp-proxy.start_after = ["mycart"]`
- docker-compose: `xmpp-proxy-stack.depends_on.prosody.condition = service_healthy`
- mycart generates certs on first HTTP request (autocert lazy loading)

---

### Risk 5: Multi-stage build complexity

**Impact:** Dockerfile harder to maintain, longer build times.

**Likelihood:** Medium

**Mitigation:**
- Document each stage clearly
- Use BuildKit cache mounts for faster rebuilds
- Consider separate CI build for xmpp-proxy-stack base (update rarely)

---

## Success Criteria

1. ✅ Single `docker-compose up` starts all services
2. ✅ mycart accessible at `https://example.com/`
3. ✅ mycart admin accessible at `https://example.com/_/`
4. ✅ SSL certificate auto-generated by mycart's autocert
5. ✅ XMPP C2S working on port 5222/5223
6. ✅ XMPP S2S working on port 5269
7. ✅ fail2ban-rs blocks abusive IPs
8. ✅ All logs written to `/srv/data/logs/`
9. ✅ Data persisted to `/srv/data/mycart/` and `/srv/data/prosody/`
10. ✅ Cert renewal works automatically (test before 30-day expiration)

---

## Future Enhancements

1. **Separate mycart from xmpp-proxy-stack** if coupling becomes problematic
2. **Add monitoring** (Prometheus/Grafana for metrics)
3. **Add backup automation** for `/srv/data/`
4. **Implement health checks** for mycart in docker-compose
5. **Add nginx back as optional reverse proxy** if advanced routing needed
6. **Support multiple domains** (separate certs for XMPP and web)

---

## References

- **Reference docker-compose:** `/home/wj/work/xmpp-proxy/docker-compose.yaml`
- **Reference Dockerfile:** `/home/wj/work/xmpp-proxy/xmpp-proxy-stack/Dockerfile`
- **mycart source:** `/home/wj/work/dure-mycart/`
- **mycart Dockerfile:** `/home/wj/work/dure-mycart/Dockerfile`
- **Go autocert package:** `golang.org/x/crypto/acme/autocert`
- **Horust docs:** https://github.com/FedericoPonzi/Horust
- **xmpp-proxy:** https://github.com/nikescar/xmpp-proxy
- **fail2ban-rs:** https://github.com/aejimmi/fail2ban-rs

---

## Appendix: Environment Variables Reference

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `XMPP_DOMAIN` | Yes | - | Domain for XMPP services (must match MYCART_DOMAIN) |
| `MYCART_DOMAIN` | Yes | - | Domain for mycart web app (must match XMPP_DOMAIN) |
| `XMPP_ADMIN` | Yes | - | XMPP admin JID (e.g., admin@example.com) |
| `MYCART_HTTP_ADDR` | No | `0.0.0.0:80` | mycart HTTP listen address |
| `MYCART_HTTPS_ADDR` | No | `0.0.0.0:443` | mycart HTTPS listen address |
| `PROSODY_LOGLEVEL` | No | `info` | Prosody log level |
| `PROSODY_RETENTION_DAYS` | No | `90` | Message archive retention days |
| `XMPP_PROXY_PROSODY_C2S` | No | `127.0.0.1:15222` | Prosody C2S backend address |
| `XMPP_PROXY_PROSODY_S2S` | No | `127.0.0.1:15269` | Prosody S2S backend address |
| `FAIL2BAN_MAX_RETRY` | No | `5` | Max failed attempts before ban |
| `FAIL2BAN_FIND_TIME` | No | `600` | Time window for retry count (seconds) |
| `FAIL2BAN_BAN_TIME` | No | `3600` | Ban duration (seconds) |

---

**End of Design Document**
