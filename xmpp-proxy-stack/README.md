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
