# dure-mycart-prosody

> Full-stack Docker image combining **mycart** e-commerce platform with **xmpp-proxy**, **fail2ban-rs**, and process management via **Horust**.

## What's Included

This all-in-one container includes:

- **mycart** - Lightweight e-commerce platform
- **xmpp-proxy** - XMPP/Jabber proxy server
- **fail2ban-rs** - Intrusion prevention system
- **Horust** - Process supervisor managing all services

## Quick Start

```bash
docker pull ghcr.io/dure-one/dure-mycart-prosody:latest

docker run -d \
  -p 80:80 \
  -p 443:443 \
  -p 5222:5222 \
  -p 5269:5269 \
  --name mycart-prosody \
  ghcr.io/dure-one/dure-mycart-prosody:latest
```

## Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `DOMAIN` | Your domain name | `localhost` |
| `ACME_EMAIL` | Email for Let's Encrypt | - |
| `XMPP_DOMAIN` | XMPP server domain | Same as `DOMAIN` |

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
