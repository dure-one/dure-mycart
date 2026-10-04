# dure-mycart-prosody

> Full-stack Docker image combining **dure-mycart** e-commerce platform with **xmpp-proxy**, **fail2ban-rs**, and process management via **Horust**.

## What's Included

This all-in-one container includes:

- **dure-mycart** - Lightweight e-commerce platform
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
  --name dure-mycart-prosody \
  ghcr.io/dure-one/dure-mycart-prosody:latest
```

## Environment Variables

| Variable | Description | Required | Default |
|----------|-------------|----------|---------|
| `XMPP_DOMAIN` | XMPP server domain | **Yes** | - |
| `MYCART_DOMAIN` | Mycart domain (must match XMPP_DOMAIN) | **Yes** | - |
| `DATA_DIR` | Base directory for persistent data | No | `/srv/data` |
| `XMPP_PROXY_PROSODY_C2S` | Backend Prosody c2s port | Yes | `127.0.0.1:5222` |
| `XMPP_PROXY_PROSODY_S2S` | Backend Prosody s2s port | Yes | `127.0.0.1:5269` |
| `MYCART_DEV_MODE` | Development mode | No | `false` |
| `MYCART_HTTP_ADDR` | HTTP bind address | No | `0.0.0.0:80` |
| `MYCART_HTTPS_ADDR` | HTTPS bind address | No | `0.0.0.0:443` |
| `GIN_MODE` | Gin framework mode | No | `release` |
| `REVERSE_PROXY_BINDINGS` | Reverse proxy config | No | - |

**Important:**
- `XMPP_DOMAIN` and `MYCART_DOMAIN` must match for shared SSL certificate functionality.
- `DATA_DIR` can be absolute (`/srv/data` for production) or relative (`./srv/data` for development).

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

Production XMPP deployment with Prosody server and dure-mycart integration using Docker Compose.

### Services

- **prosody-modules-init** - One-time module setup
- **prosody-config-init** - Configuration renderer
- **prosody-permissions-init** - Fixes directory permissions (UID 1000:1000)
- **prosody** - XMPP server (Prosody 13.0)
- **prosody-mycart-stack** - dure-mycart + XMPP proxy + fail2ban

### Prerequisites

```bash
# Create data directories
export DATA_DIR=/srv/data  # or ./srv/data for development
sudo mkdir -p ${DATA_DIR}/{prosody,certs,logs,fail2ban,mycart/{lc_base,lc_uploads,lc_digitals}}
sudo chown -R 1000:1000 ${DATA_DIR}

# Create .env file in prosody-mycart-stack directory
cd prosody-mycart-stack
cp .env.example .env
# Edit .env with your domain and settings
```

### Production Usage (Pre-built Image)

Uses the latest image from GitHub Container Registry:

```bash
# Start all services
docker compose -f prosody-mycart-stack/docker-compose.yml up -d

# Check status
docker compose -f prosody-mycart-stack/docker-compose.yml ps
docker exec prosody prosodyctl status

# View logs
docker logs prosody
docker logs prosody-mycart-stack

# Check listening ports (using host network)
ss -tnlup | grep -E '5222|5269|80|443'

# Stop services
docker compose -f prosody-mycart-stack/docker-compose.yml down
```

### Development Usage (Local Build)

Builds the prosody-mycart-stack image locally from source:

```bash
# Build and start all services
docker compose -f prosody-mycart-stack/docker-compose.dev.yml up -d --build

# Rebuild after code changes
docker compose -f prosody-mycart-stack/docker-compose.dev.yml build prosody-mycart-stack
docker compose -f prosody-mycart-stack/docker-compose.dev.yml up -d

# Stop services
docker compose -f prosody-mycart-stack/docker-compose.dev.yml down
```

### Ports (Bridge Network Mode)

The prosody-mycart-stack container uses bridge networking to connect directly to Prosody (preserving PROXY protocol headers):

- **80** - HTTP (ACME challenges)
- **443** - HTTPS
- **5222** - XMPP C2S (StartTLS)
- **5223** - XMPP C2S (Direct TLS)
- **5269** - XMPP S2S (Server-to-Server)

**Important:** Direct container-to-container communication bypasses Docker port mapping, allowing PROXY protocol headers to reach Prosody with real client IPs.

### Data Volumes

Configurable via `DATA_DIR` environment variable (default: `/srv/data`):

- `${DATA_DIR}/prosody/` - XMPP database
- `${DATA_DIR}/certs/` - TLS certificates (shared between Prosody and dure-mycart)
- `${DATA_DIR}/logs/` - Application logs
- `${DATA_DIR}/mycart/` - dure-mycart data (database, uploads, digital products)

### Prosody Configuration

Prosody runs on internal bridge network (`xmpp-internal`) with static IP:
- `172.19.0.2:5222` - C2S (client-to-server) - xmpp-proxy connects here
- `172.19.0.2:5269` - S2S (server-to-server) - xmpp-proxy connects here
- `prosody:5280` - HTTP/WebSocket - proxied via mycart reverse proxy

**IMPORTANT:** xmpp-proxy requires IP:port format (hostname:port not supported). Prosody has static IP `172.19.0.2` assigned in docker-compose.yml.

The prosody-mycart-stack container:
- Joins the same `xmpp-internal` bridge network as Prosody
- Connects directly to Prosody using static IP (no Docker port mapping)
- Sends PROXY protocol v1 headers with real client IPs
- Handles public-facing XMPP ports (5222, 5223, 5269)
- Performs TLS termination with auto-renewed certificates
- PROXY protocol for client IP preservation
- fail2ban-rs for intrusion prevention
- dure-mycart web application

### fail2ban-rs Configuration

The container includes fail2ban-rs for automatic IP banning based on authentication failures.

**Check Status:**
```bash
docker exec prosody-mycart-stack /usr/local/bin/fail2ban-rs status
```

**Configuration File:** `prosody-mycart-stack/fail2ban-rs-config.toml`

Default jails monitor:
- **xmpp-auth** - Prosody authentication failures
- **mycart-auth** - dure-mycart HTTP authentication failures

**Default Settings:**
- Ban time: 1 hour
- Find time: 10 minutes (look-back window)
- Max retry: 5 failures before ban
- Backend: nftables (automatic firewall rules)

#### Optional: MaxMind GeoIP Enrichment

Add geographic information to ban logs for better attack pattern analysis.

**1. Sign Up for MaxMind GeoLite2 (Free):**

Visit https://www.maxmind.com/en/geolite2/signup and generate a license key.

**2. Download Databases:**

```bash
# On production server
sudo mkdir -p /srv/data/maxmind

# Method 1: Using geoipupdate (recommended - auto-updates)
sudo apt-get install geoipupdate

sudo tee /etc/GeoIP.conf > /dev/null <<EOF
AccountID YOUR_ACCOUNT_ID
LicenseKey YOUR_LICENSE_KEY
EditionIDs GeoLite2-ASN GeoLite2-Country GeoLite2-City
DatabaseDirectory /srv/data/maxmind
EOF

sudo geoipupdate

# Method 2: Manual download (one-time, no auto-updates)
cd /srv/data/maxmind
wget "https://download.maxmind.com/app/geoip_download?edition_id=GeoLite2-ASN&license_key=YOUR_KEY&suffix=tar.gz" -O asn.tar.gz
wget "https://download.maxmind.com/app/geoip_download?edition_id=GeoLite2-Country&license_key=YOUR_KEY&suffix=tar.gz" -O country.tar.gz
tar -xzf asn.tar.gz --strip-components=1 --wildcards '*.mmdb'
tar -xzf country.tar.gz --strip-components=1 --wildcards '*.mmdb'
```

**3. Update docker-compose.yml:**

Add MaxMind volume mount to `prosody-mycart-stack` service:
```yaml
volumes:
  - /srv/data/maxmind:/maxmind:ro
```

**4. Enable in fail2ban-rs-config.toml:**

Uncomment the MaxMind paths in `[global]` section:
```toml
[global]
maxmind_asn = "/maxmind/GeoLite2-ASN.mmdb"
maxmind_country = "/maxmind/GeoLite2-Country.mmdb"
# maxmind_city = "/maxmind/GeoLite2-City.mmdb"  # Optional
```

Enable per-jail enrichment:
```toml
[jail.xmpp-auth]
maxmind = ["asn", "country"]

[jail.mycart-auth]
maxmind = ["asn", "country"]
```

**5. Restart Container:**
```bash
docker compose restart prosody-mycart-stack
```

**Example Ban Log with GeoIP:**
```
banned ip=1.2.3.4 jail=xmpp-auth asn="AS15169 Google LLC" country="United States"
```

**Automatic Updates:**

Add to crontab for weekly updates:
```bash
sudo crontab -e
# Add: Weekly MaxMind database update (Wednesdays at 3 AM)
0 3 * * 3 /usr/bin/geoipupdate && docker compose -f /srv/dure-mycart/prosody-mycart-stack/docker-compose.yml restart prosody-mycart-stack
```

## Troubleshooting

### xmpp-proxy fails to start ("invalid config file")

**Symptom:** Container logs show "invalid config file" and xmpp-proxy process not running.

**Cause:** xmpp-proxy only supports IP:port format, not hostname:port.

**Solution:** Verify `.env` file uses IP addresses:
```bash
XMPP_PROXY_PROSODY_C2S=172.19.0.2:5222
XMPP_PROXY_PROSODY_S2S=172.19.0.2:5269
```

**Verify static IP assignment:**
```bash
docker network inspect prosody-mycart-stack_xmpp-internal --format "{{range .Containers}}{{.Name}}: {{.IPv4Address}} {{end}}"
# Should show: prosody: 172.19.0.2/16
```

### Prosody admin page redirect errors

**Symptom:** BOSH connection fails, admin page doesn't load.

**Cause:** xmpp-proxy not running (see above) or reverse proxy misconfigured.

**Check:**
```bash
# Verify xmpp-proxy is running
docker exec prosody-mycart-stack /bin/busybox ps | grep xmpp-proxy

# Test Prosody HTTP endpoint
docker exec prosody-mycart-stack /usr/bin/curl -s -o /dev/null -w "%{http_code}\n" http://172.19.0.2:5280/http-bind
# Should return: 200
```

### Prosody logs show internal IP (172.18.0.1) instead of real client IP

**Symptom:** Authentication failure logs show Docker gateway IP.

**Cause:** PROXY protocol not configured or xmpp-proxy not running.

**Solution:**
1. Verify xmpp-proxy is running and connected to Prosody
2. Check Prosody has mod_net_proxy enabled and proxy_trusted_proxies configured
3. Verify xmpp-proxy config has `proxy = true`

**Test PROXY protocol:**
```bash
# Trigger authentication failure and check logs
docker logs prosody 2>&1 | grep "Failed authentication" | tail -5
# Should show real client IP, not 172.x.x.x
```

### Container IP changed after restart

**Symptom:** xmpp-proxy can't connect to Prosody after container recreation.

**Cause:** Static IP not properly configured in docker-compose.yml.

**Solution:** Ensure docker-compose.yml has:
```yaml
networks:
  xmpp-internal:
    driver: bridge
    ipam:
      config:
        - subnet: 172.19.0.0/16
          gateway: 172.19.0.1

services:
  prosody:
    networks:
      xmpp-internal:
        ipv4_address: 172.19.0.2
```

## Architecture

Built on **distroless** base for minimal attack surface:
- Multi-stage build
- All binaries compiled from source or downloaded from official releases
- No shell access in production container
- Minimal dependencies

## Source Code

- Repository: https://github.com/dure-one/dure-mycart
- Dockerfile: `prosody-mycart-stack/Dockerfile`

## License

MIT
