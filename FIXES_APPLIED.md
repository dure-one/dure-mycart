# XMPP Stack Fixes Applied

## Issues Found

1. **xmpp-proxy not starting** - Service was failing because it required SSL certificates (`/certs/fullchain.pem` and `/certs/privkey.pem`) that didn't exist yet. mycart's autocert generates these certificates on first HTTPS request, creating a chicken-and-egg problem.

2. **fail2ban-rs binary missing** - The binary wasn't being extracted correctly from the downloaded tar.gz archive.

3. **fail2ban-rs config missing** - No configuration file existed for fail2ban-rs at `/etc/fail2ban-rs/config.toml`.

## Fixes Applied

### Fix 1: xmpp-proxy SSL Certificate Wait Wrapper

**File:** `xmpp-proxy-stack/xmpp-proxy-wrapper.sh` (NEW)

- Created a wrapper script that waits up to 5 minutes for SSL certificates to exist before starting xmpp-proxy
- Polls every 10 seconds for `/certs/fullchain.pem` and `/certs/privkey.pem`
- Provides clear logging of wait status
- Times out with error after 5 minutes if certificates don't appear

**Modified:** `xmpp-proxy-stack/horust-services/xmpp-proxy.toml`

- Changed command from direct xmpp-proxy invocation to wrapper script
- Service now uses: `/usr/local/bin/xmpp-proxy-wrapper.sh`

### Fix 2: fail2ban-rs Build Improvements

**Modified:** `xmpp-proxy-stack/Dockerfile` (xmpp-binaries-builder stage)

- Added error checking after tar extraction
- Shows directory contents if binary not found (for debugging)
- Verifies binary with `--version` check after extraction
- More robust `find` command to locate binary in archive

### Fix 3: fail2ban-rs Configuration

**File:** `xmpp-proxy-stack/fail2ban-rs-config.toml` (NEW)

- Created minimal configuration for fail2ban-rs
- Monitors XMPP authentication failures in xmpp-proxy logs
- Monitors HTTP authentication failures in mycart logs
- Uses nftables for IP banning
- Configurable thresholds:
  - `max_retry = 5` failed attempts
  - `find_time = 600` seconds (10 minutes)
  - `ban_time = 3600` seconds (1 hour)

**Modified:** `xmpp-proxy-stack/Dockerfile` (tools-builder stage)

- Creates `/etc/fail2ban-rs/` directory
- Copies config file to container

### Fix 4: Dockerfile Improvements

**Modified:** `xmpp-proxy-stack/Dockerfile` (tools-builder stage)

- Added copy of `xmpp-proxy-wrapper.sh` to `/usr/local/bin/`
- Made wrapper script executable
- Added fail2ban-rs config directory creation and file copy

## How Services Start Now

### Startup Sequence

1. **Horust** (PID 1) starts as init system
2. **mycart** starts immediately (2s delay)
   - Runs database migrations
   - Starts HTTP (port 80) and HTTPS (port 443) servers
   - Autocert ready to generate certificates on first HTTPS request
3. **xmpp-proxy** waits for mycart (5s delay + dependency)
   - Wrapper script polls for SSL certificates
   - Once certificates exist, starts xmpp-proxy on ports 5222/5269
4. **fail2ban-rs** waits for xmpp-proxy (3s delay + dependency)
   - Monitors logs for authentication failures
   - Bans IPs using nftables

### SSL Certificate Generation

mycart's autocert automatically generates Let's Encrypt certificates when:
- First HTTPS request arrives at `https://dure.co`
- ACME HTTP-01 challenge completes via port 80
- Certificates saved to `/certs/fullchain.pem` and `/certs/privkey.pem`

This triggers xmpp-proxy to start, as the wrapper script detects the certificate files.

## Rebuild Instructions

### Local Rebuild (Development)

```bash
# Make script executable
chmod +x scripts/rebuild-xmpp-stack.sh

# Run rebuild script
./scripts/rebuild-xmpp-stack.sh
```

Or manually:

```bash
# Stop containers
docker-compose -f docker-compose.dev.yml down

# Rebuild without cache
docker-compose -f docker-compose.dev.yml build --no-cache xmpp-proxy-stack

# Start services
docker-compose -f docker-compose.dev.yml up -d

# Watch logs
docker-compose -f docker-compose.dev.yml logs -f xmpp-proxy-stack
```

### Production Rebuild (GitHub Actions)

The CI/CD workflow will automatically rebuild and push new images when these changes are committed and pushed to GitHub:

```bash
# Commit changes
git add xmpp-proxy-stack/
git commit -m "fix(xmpp): add SSL cert wait wrapper and fail2ban-rs config"

# Push to trigger CI/CD
git push origin feature/mycart-xmpp-integration
```

GitHub Actions will:
1. Build multi-arch images (amd64, arm64)
2. Push to `ghcr.io/dure-one/xmpp-proxy-stack:latest`
3. Tag with git commit SHA

### Deploy on Server

After images are built and pushed:

```bash
# SSH to server
ssh root@dure-server

# Navigate to deployment directory
cd /srv/dure-mycart

# Pull latest images
docker-compose pull

# Restart services
docker-compose up -d

# Trigger certificate generation
curl -k https://dure.co

# Watch for xmpp-proxy to start
docker-compose logs -f xmpp-proxy-stack
```

## Verification

### Check Service Status

```bash
# Get container ID
CONTAINER_ID=$(docker ps -qf "name=xmpp-proxy-stack")

# Check all running processes
docker exec $CONTAINER_ID /bin/busybox ps aux

# Should show:
# PID 1:  horust
# PID 2X: mycart
# PID 3X: xmpp-proxy (after certs exist)
# PID 4X: fail2ban-rs (after xmpp-proxy starts)
```

### Check Logs

```bash
# mycart logs (should show successful start)
docker exec $CONTAINER_ID /bin/busybox cat /logs/mycart-stdout.log

# xmpp-proxy logs (should show waiting then start)
docker exec $CONTAINER_ID /bin/busybox cat /logs/xmpp-proxy-stdout.log

# fail2ban-rs logs
docker exec $CONTAINER_ID /bin/busybox cat /logs/fail2ban-rs-stdout.log
```

### Check SSL Certificates

```bash
# Verify certificates exist
docker exec $CONTAINER_ID /bin/busybox ls -la /certs/

# Should show:
# fullchain.pem
# privkey.pem
```

### Test XMPP Connection

```bash
# From server
ss -nltup | grep 443  # mycart HTTPS
ss -nltup | grep 5222 # xmpp-proxy C2S
ss -nltup | grep 5269 # xmpp-proxy S2S
```

## Architecture Summary

```
┌─────────────────────────────────────────┐
│         xmpp-proxy-stack Container       │
│                                          │
│  ┌────────────────────────────────────┐ │
│  │ Horust (PID 1) - Init System      │ │
│  └────────────────────────────────────┘ │
│           │                              │
│           ├─ mycart (PID 23)             │
│           │  ├─ HTTP :80                 │
│           │  ├─ HTTPS :443               │
│           │  └─ Autocert → /certs/       │
│           │                              │
│           ├─ xmpp-proxy-wrapper.sh       │
│           │  └─ Waits for /certs/*.pem   │
│           │     └─ xmpp-proxy            │
│           │        ├─ C2S :5222          │
│           │        └─ S2S :5269          │
│           │                              │
│           └─ fail2ban-rs                 │
│              └─ Monitors logs → nftables │
│                                          │
└─────────────────────────────────────────┘
           │
           ├─ Volumes:
           │  ├─ /certs     → /srv/data/certs
           │  ├─ /logs      → /srv/data/logs
           │  └─ /app/lc_*  → /srv/data/mycart/*
           │
           └─ Network: host mode
              (direct access to host ports)
```

## Files Changed

- `xmpp-proxy-stack/Dockerfile` - Added wrapper copy, fail2ban config, improved error handling
- `xmpp-proxy-stack/horust-services/xmpp-proxy.toml` - Use wrapper script
- `xmpp-proxy-stack/xmpp-proxy-wrapper.sh` - NEW: SSL cert wait wrapper
- `xmpp-proxy-stack/fail2ban-rs-config.toml` - NEW: fail2ban-rs configuration
- `scripts/rebuild-xmpp-stack.sh` - NEW: Rebuild automation script

## Next Steps

1. Rebuild the image locally or via CI/CD
2. Deploy to production server
3. Trigger certificate generation by accessing `https://dure.co`
4. Verify all services start in correct order
5. Test XMPP client connections

## Troubleshooting

### xmpp-proxy still not starting

```bash
# Check if wrapper is waiting for certs
docker exec $CONTAINER_ID /bin/busybox cat /logs/xmpp-proxy-stdout.log

# Manually trigger cert generation
curl -k https://dure.co

# Check if certs were created
docker exec $CONTAINER_ID /bin/busybox ls -la /certs/
```

### fail2ban-rs fails to start

```bash
# Check logs
docker exec $CONTAINER_ID /bin/busybox cat /logs/fail2ban-rs-stderr.log

# Verify binary exists
docker exec $CONTAINER_ID /bin/busybox ls -la /usr/local/bin/fail2ban-rs

# Verify config exists
docker exec $CONTAINER_ID /bin/busybox cat /etc/fail2ban-rs/config.toml
```

### Build fails

```bash
# Check fail2ban-rs download
docker-compose -f docker-compose.dev.yml build xmpp-proxy-stack 2>&1 | grep -A 10 "fail2ban-rs"

# If download fails, check GitHub release exists:
curl -I https://api.github.com/repos/aejimmi/fail2ban-rs/releases/latest
```
