# dure-mycart-prosody

> Full-stack Docker image combining **mycart** e-commerce platform with **xmpp-proxy**, **fail2ban-rs**, and process management via **Horust**.

## What's Included

This all-in-one container includes:

- **mycart** - Lightweight e-commerce platform
- **xmpp-proxy** - XMPP/Jabber proxy server
- **fail2ban-rs** - Intrusion prevention system
- **Horust** - Process supervisor managing all services

## Quick Start

### Using GitHub Container Registry

Pull the image from GitHub Container Registry:

```bash
docker pull ghcr.io/dure-one/dure-mycart-prosody:latest
```

**Package URL:** https://github.com/dure-one/dure-mycart/pkgs/container/dure-mycart-prosody

### Configuration with .env File

1. Copy the example environment file:

```bash
cp .env.example .env
```

2. Edit `.env` with your configuration:

```bash
# Required: Set your domain (must match for both services)
XMPP_DOMAIN=example.com
MYCART_DOMAIN=example.com

# Configure backend XMPP server ports
XMPP_PROXY_PROSODY_C2S=127.0.0.1:5222
XMPP_PROXY_PROSODY_S2S=127.0.0.1:5269

# Mycart settings
MYCART_DEV_MODE=false
GIN_MODE=release
```

3. Run with environment file:

```bash
docker run -d \
  --env-file .env \
  -p 80:80 \
  -p 443:443 \
  -p 5222:5222 \
  -p 5269:5269 \
  -v ./certs:/certs \
  -v ./logs:/logs \
  --name mycart-prosody \
  ghcr.io/dure-one/dure-mycart-prosody:latest
```

## Environment Variables

| Variable | Description | Required | Default |
|----------|-------------|----------|---------|
| `XMPP_DOMAIN` | XMPP server domain | **Yes** | - |
| `MYCART_DOMAIN` | Mycart domain (must match XMPP_DOMAIN) | **Yes** | - |
| `XMPP_PROXY_PROSODY_C2S` | Backend Prosody c2s port | Yes | `127.0.0.1:5222` |
| `XMPP_PROXY_PROSODY_S2S` | Backend Prosody s2s port | Yes | `127.0.0.1:5269` |
| `MYCART_DEV_MODE` | Development mode | No | `false` |
| `MYCART_HTTP_ADDR` | HTTP bind address | No | `0.0.0.0:80` |
| `MYCART_HTTPS_ADDR` | HTTPS bind address | No | `0.0.0.0:443` |
| `GIN_MODE` | Gin framework mode | No | `release` |
| `REVERSE_PROXY_BINDINGS` | Reverse proxy config | No | - |

**Important:** `XMPP_DOMAIN` and `MYCART_DOMAIN` must match for shared SSL certificate functionality.

## Ports

| Port | Service | Protocol |
|------|---------|----------|
| 80 | HTTP | TCP |
| 443 | HTTPS | TCP |
| 5222 | XMPP Client | TCP |
| 5269 | XMPP Server-to-Server | TCP |

## Volumes

```bash
docker run -d \
  -v ./certs:/certs \
  -v ./logs:/logs \
  -v ./data:/app/lc_base \
  ghcr.io/dure-one/dure-mycart-prosody:latest
```

## Docker Compose Deployment

Production XMPP deployment with Prosody server and myCart integration using Docker Compose.

### Services

- **prosody-modules-init** - One-time module setup
- **prosody-config-init** - Configuration renderer
- **prosody** - XMPP server (Prosody 13.0)
- **xmpp-proxy-stack** - myCart + XMPP proxy + fail2ban

### Prerequisites

```bash
# Create data directories
sudo mkdir -p /srv/data/{prosody,certs,logs,fail2ban,mycart/{lc_base,lc_uploads,lc_digitals}}
sudo chown -R 1000:1000 /srv/data

# Create .env file in project root
cat > .env << EOF
XMPP_DOMAIN=chat.example.com
MYCART_DOMAIN=chat.example.com
XMPP_ADMIN=admin@chat.example.com
XMPP_PROXY_PROSODY_C2S=127.0.0.1:15222
XMPP_PROXY_PROSODY_S2S=127.0.0.1:15269
PROSODY_LOGLEVEL=info
PROSODY_RETENTION_DAYS=90
MYCART_DEV_MODE=false
GIN_MODE=release
EOF
```

### Usage

```bash
# Start all services
docker-compose -f xmpp-proxy-stack/docker-compose.yml up -d

# Check status
docker-compose -f xmpp-proxy-stack/docker-compose.yml ps
docker exec prosody prosodyctl status

# View logs
docker logs prosody
docker logs xmpp-proxy-stack

# Check listening ports (using host network)
ss -tnlup | grep -E '5222|5269|80|443'

# Stop services
docker-compose -f xmpp-proxy-stack/docker-compose.yml down
```

### Ports (Host Network Mode)

The xmpp-proxy-stack container uses host networking for PROXY protocol support:

- **80** - HTTP (ACME challenges)
- **443** - HTTPS
- **5222** - XMPP C2S (StartTLS)
- **5223** - XMPP C2S (Direct TLS)
- **5269** - XMPP S2S (Server-to-Server)
- **443/udp** - XMPP over QUIC

**Note:** Ports won't show in `docker ps` - use `ss -tnlup` to verify listening ports.

### Data Volumes

- `/srv/data/prosody/` - XMPP database
- `/srv/data/certs/` - TLS certificates (shared between Prosody and myCart)
- `/srv/data/logs/` - Application logs
- `/srv/data/mycart/` - myCart data (database, uploads, digital products)

### Prosody Configuration

Prosody runs on internal bridge network with ports exposed only to localhost:
- `127.0.0.1:15222` - C2S (client-to-server)
- `127.0.0.1:15269` - S2S (server-to-server)
- `127.0.0.1:15280` - HTTP/WebSocket (proxied via myCart)

The xmpp-proxy-stack container connects to these backend ports and handles:
- Public-facing XMPP ports (5222, 5269)
- TLS termination with auto-renewed certificates
- PROXY protocol for client IP preservation
- fail2ban-rs for intrusion prevention

## Architecture

Built on **distroless** base for minimal attack surface:
- Multi-stage build
- All binaries compiled from source or downloaded from official releases
- No shell access in production container
- Minimal dependencies

## Source Code

- Repository: https://github.com/dure-one/dure-mycart
- Dockerfile: `xmpp-proxy-stack/Dockerfile`

## License

MIT
