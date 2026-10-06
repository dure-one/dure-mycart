# dure-mycart XMPP Stack

> **2-container architecture**: dure-mycart (HTTP + XMPP + fail2ban) + Prosody (XMPP server)

## Architecture

### Services

- **dure-mycart** - Merged stack: HTTP server + xmpp-proxy + fail2ban-rs
- **prosody** - XMPP server backend (Prosody 13.0)

### Traffic Flow

| Client Connection | Path | Notes |
|---|---|---|
| HTTP (80/tcp) | Client → dure-mycart → Fiber | Web traffic |
| XMPP QUIC (443/udp) | Client → xmpp-proxy → Prosody | XEP-0467 QUIC |
| XMPP C2S (5222/tcp) | Client → xmpp-proxy → Prosody | Standard Direct TLS |
| XMPP C2S (5223/tcp) | Client → xmpp-proxy → Prosody | Legacy SSL |
| XMPP S2S (5269/tcp) | Client → xmpp-proxy → Prosody | Standard Direct TLS |

## Quick Start

### Pull Images

```bash
docker pull ghcr.io/dure-one/prosody-mycart:latest
docker pull prosodyim/prosody:13.0
```

**Package URLs:**
- prosody-mycart: https://github.com/dure-one/dure-mycart/pkgs/container/prosody-mycart

### Configuration with .env File

1. Copy the example environment file:

```bash
cd prosody-mycart-stack
cp .env.example .env
```

2. Edit `.env` with your configuration:

```bash
# Image tag (latest/test/custom)
MYCART_IMAGE_TAG=latest

# Required: Set your domain (must match for both services)
XMPP_DOMAIN=example.com
MYCART_DOMAIN=example.com

# Admin user
XMPP_ADMIN=admin@example.com
```

3. Start the stack:

```bash
# Production (pull latest image)
docker compose up -d

# Development (build from source)
docker compose -f docker-compose.yml -f docker-compose.dev.yml up -d

# Testing (pull test image)
MYCART_IMAGE_TAG=test docker compose up -d
```

## Environment Variables

| Variable | Description | Required | Default |
|----------|-------------|----------|---------|
| `MYCART_IMAGE_TAG` | prosody-mycart image tag | No | `latest` |
| `XMPP_DOMAIN` | XMPP server domain | **Yes** | - |
| `MYCART_DOMAIN` | Mycart domain (must match XMPP_DOMAIN) | **Yes** | - |
| `XMPP_ADMIN` | Admin JID | **Yes** | `admin@${XMPP_DOMAIN}` |
| `DATA_DIR` | Base directory for persistent data | No | `/srv/data` |
| `PROSODY_LOGLEVEL` | Prosody log level | No | `info` |
| `PROSODY_RETENTION_DAYS` | Message retention (MAM) | No | `90` |

**Important:**
- `XMPP_DOMAIN` and `MYCART_DOMAIN` must match for shared SSL certificate functionality.
- Image tag controls deployment mode: `latest` (production), `test` (testing), or custom tag.

## Ports

| Port | Service | Container | Protocol | Notes |
|------|---------|-----------|----------|-------|
| 80 | HTTP | dure-mycart | TCP | ACME challenges, web traffic |
| 443 | XMPP QUIC | dure-mycart | UDP | XMPP over QUIC (XEP-0467) |
| 5222 | XMPP C2S Direct TLS | dure-mycart | TCP | Standard client connections |
| 5223 | XMPP C2S Legacy SSL | dure-mycart | TCP | Legacy clients |
| 5269 | XMPP S2S Direct TLS | dure-mycart | TCP | Server-to-server federation |

**dure-mycart → Prosody Routing:**
- 443/udp (QUIC) → Prosody C2S/S2S (XEP-0467 with PROXY protocol)
- 5222/tcp (C2S) → Prosody C2S (with PROXY protocol)
- 5223/tcp (Legacy) → Prosody C2S (with PROXY protocol)
- 5269/tcp (S2S) → Prosody S2S (with PROXY protocol)

## Volumes

```bash
docker run -d \
  -v ./certs:/certs \
  -v ./logs:/logs \
  -v ./data:/app/lc_base \
  ghcr.io/dure-one/prosody-mycart:latest
```

## Docker Compose Deployment

Production XMPP deployment with Prosody server and dure-mycart integration using Docker Compose.

### Services

- **prosody-modules-init** - One-time Prosody community modules setup
- **prosody-config-init** - Renders Prosody configuration template
- **xmpp-proxy-config-init** - Renders xmpp-proxy configuration template
- **prosody-permissions-init** - Fixes directory permissions (UID 1000:1000)
- **prosody** - XMPP server (Prosody 13.0)
- **dure-mycart** - Merged stack: HTTP server + xmpp-proxy + fail2ban-rs

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
docker logs dure-mycart

# Check listening ports (using host network)
ss -tnlup | grep -E '5222|5269|80|443'

# Stop services
docker compose -f prosody-mycart-stack/docker-compose.yml down
```

### Development Usage (Local Build)

Builds the dure-mycart image locally from source:

```bash
# Build and start all services
docker compose -f prosody-mycart-stack/docker-compose.dev.yml up -d --build

# Rebuild after code changes
docker compose -f prosody-mycart-stack/docker-compose.dev.yml build dure-mycart
docker compose -f prosody-mycart-stack/docker-compose.dev.yml up -d

# Stop services
docker compose -f prosody-mycart-stack/docker-compose.dev.yml down
```

### Ports (Bridge Network Mode)

The dure-mycart container uses bridge networking to connect directly to Prosody (preserving PROXY protocol headers):

**Exposed from dure-mycart:**
- **80** - HTTP (ACME challenges, redirects to HTTPS)
- **443** - HTTPS + XMPP via ALPN (multiplexed based on TLS ALPN protocol)

**Exposed from Prosody:**
- **5269** - Standard S2S (for legacy XMPP servers)

**Internal (bridge network only):**
- `172.19.0.2:5222` - Prosody C2S (receives PROXY protocol from ALPN router)
- `172.19.0.2:5269` - Prosody S2S (Direct TLS for legacy servers)  
- `172.19.0.2:5270` - Prosody S2S (receives PROXY protocol from ALPN router)

**Important:** The ALPN router terminates TLS on port 443 and routes based on negotiated protocol, then forwards to Prosody with PROXY headers preserving real client IPs.

### Data Volumes

Configurable via `DATA_DIR` environment variable (default: `/srv/data`):

- `${DATA_DIR}/prosody/` - XMPP database
- `${DATA_DIR}/certs/` - TLS certificates (shared between Prosody and dure-mycart)
- `${DATA_DIR}/logs/` - Application logs
- `${DATA_DIR}/mycart/` - dure-mycart data (database, uploads, digital products)

### Prosody Configuration

Prosody runs on internal bridge network (`xmpp-internal`) with static IP `172.19.0.2`:

**Incoming Connections (with PROXY protocol):**
- `172.19.0.2:5222` - C2S from ALPN router (port 443 → `xmpp-client` ALPN)
- `172.19.0.2:5270` - S2S from ALPN router (port 443 → `xmpp-server` ALPN, XEP-0368)

**Standard S2S (Direct TLS, no PROXY):**
- `172.19.0.2:5269` - S2S for legacy servers (exposed as host port 5269)

**HTTP Services:**
- `prosody:5280` - HTTP/WebSocket (proxied via mycart)

**IMPORTANT:** The ALPN router in dure-mycart uses static IP `172.19.0.2` to connect to Prosody.

The dure-mycart container:
- Joins the same `xmpp-internal` bridge network as Prosody
- **xmpp-proxy**: Handles XMPP protocols (QUIC, Direct TLS on 5222, 5223, 5269)
- **fail2ban-rs**: Monitors Prosody/mycart/xmpp-proxy logs, bans abusive IPs
- **Auto TLS**: Let's Encrypt certificates via autocert
- **dure-mycart**: Web application and API

### fail2ban-rs Configuration

The container includes fail2ban-rs for automatic IP banning based on authentication failures.

**Check Status:**
```bash
docker exec dure-mycart /usr/local/bin/fail2ban-rs status
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

Add MaxMind volume mount to `dure-mycart` service:
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
docker compose restart dure-mycart
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
0 3 * * 3 /usr/bin/geoipupdate && docker compose -f /srv/dure-mycart/prosody-mycart-stack/docker-compose.yml restart dure-mycart
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
docker exec dure-mycart /bin/busybox ps | grep xmpp-proxy

# Test Prosody HTTP endpoint
docker exec dure-mycart /usr/bin/curl -s -o /dev/null -w "%{http_code}\n" http://172.19.0.2:5280/http-bind
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
- Dockerfile: `prosody-mycart-stack/dure-mycart/Dockerfile`

## License

MIT
