# mycart + XMPP Integration - Implementation Complete

**Date:** 2026-08-27  
**Branch:** `feature/mycart-xmpp-integration`  
**Status:** Implementation Complete - Ready for Testing

## Implementation Summary

All 8 tasks from the implementation plan have been completed:

1. ✅ **Directory structure and helper scripts** - Setup complete
2. ✅ **Horust service configurations** - mycart.toml, xmpp-proxy.toml, fail2ban-rs.toml created
3. ✅ **docker-entrypoint.sh** - Entrypoint with env validation created
4. ✅ **render-prosody-config.sh** - Copied from reference
5. ✅ **Multi-stage Dockerfile** - 6-stage build created
6. ✅ **docker-compose.yaml** - Full compose file with 4 services
7. ✅ **Environment template** - .env.example created
8. ✅ **Validation document** - This file

## Files Created

### Configuration Files
- `docker-compose.yaml` - Main compose orchestration
- `.env.example` - Environment variable template
- `.env` - Local environment config (git-ignored)

### xmpp-proxy-stack/
- `Dockerfile` - Multi-stage build (frontend, backend, binaries, tools, distroless)
- `docker-entrypoint.sh` - Container initialization script
- `render-prosody-config.sh` - Prosody config renderer
- `horust-services/mycart.toml` - mycart process config
- `horust-services/xmpp-proxy.toml` - xmpp-proxy process config
- `horust-services/fail2ban-rs.toml` - fail2ban-rs process config
- `templates/prosody-proxy.cfg.lua.template` - Prosody config template

### Scripts/
- `scripts/setup-volumes.sh` - Volume initialization script

## Architecture Implemented

```
Internet
   │
   ▼
┌─────────────────────────────────────┐
│ xmpp-proxy-stack (host networking)  │
│                                     │
│ Horust supervises:                  │
│ ├─ mycart :80/:443 (autocert)      │
│ ├─ xmpp-proxy :5222/:5223/:5269    │
│ └─ fail2ban-rs (rate limiting)     │
└─────────────────────────────────────┘
               │
               ▼
         ┌─────────────┐
         │ prosody     │
         │ (bridge net)│
         └─────────────┘

Volumes: /srv/data/{certs,logs,mycart,prosody,fail2ban}
```

## Next Steps - Testing & Deployment

### Step 1: Setup Volumes

```bash
cd /home/wj/work/dure-mycart
./scripts/setup-volumes.sh
```

**Expected:**
- `/srv/data/certs/` created (root:root)
- `/srv/data/logs/` created (root:root)
- `/srv/data/mycart/` created (root:root)
- `/srv/data/prosody/` created (999:999)
- `/srv/data/fail2ban/` created (root:root)

### Step 2: Configure Domain

Edit `.env` and set your actual domain:

```bash
# Replace example.com with your domain
XMPP_DOMAIN=yourdomain.com
MYCART_DOMAIN=yourdomain.com  # Must match XMPP_DOMAIN

# Update admin email
XMPP_ADMIN=admin@yourdomain.com
```

**IMPORTANT:** Ensure DNS A/AAAA record points to your server IP before starting.

### Step 3: Build xmpp-proxy-stack Image

```bash
docker-compose build xmpp-proxy-stack
```

**Expected:**
- Stage 1 (frontend-builder): Build mycart admin + site frontends
- Stage 2 (backend-builder): Build mycart Go binary
- Stage 3 (xmpp-binaries-builder): Download xmpp-proxy + fail2ban-rs
- Stage 4 (horust-builder): Download Horust
- Stage 5 (tools-builder): Prepare tools
- Final stage: Merge all into distroless image

**Build time:** ~5-10 minutes (depending on network and CPU)

### Step 4: Start Services

```bash
docker-compose up -d
```

**Expected:**
- prosody-modules-init: Completes successfully
- prosody-config-init: Completes successfully
- prosody: Starts and becomes healthy
- xmpp-proxy-stack: Starts after prosody is healthy

### Step 5: Verify Services

```bash
# Check service status
docker-compose ps

# Check xmpp-proxy-stack logs
docker-compose logs xmpp-proxy-stack | head -50

# Check prosody logs
docker-compose logs prosody | head -30
```

**Expected in xmpp-proxy-stack logs:**
```
=== xmpp-proxy-stack initialization (mycart edition) ===
Checking volume permissions...
✓ Volume permissions OK
Starting services via Horust...
```

### Step 6: Verify Processes in xmpp-proxy-stack

```bash
docker exec xmpp-proxy-stack /bin/busybox ps aux
```

**Expected processes:**
- `horust` (process supervisor)
- `mycart` (Go binary on :80/:443)
- `xmpp-proxy` (Rust binary on :5222/:5223/:5269)
- `fail2ban-rs` (Rust binary for rate limiting)

### Step 7: Test HTTP Endpoint

```bash
# Test mycart health endpoint
curl -v http://localhost/health

# Test mycart site (expect redirect to HTTPS if autocert enabled)
curl -v http://localhost/
```

**Expected:**
- HTTP 200 from `/health`
- HTTP 301 or 200 from `/` (depending on autocert state)

### Step 8: Verify XMPP Ports

```bash
ss -tnlup | grep -E ":(80|443|5222|5223|5269|5280)"
```

**Expected listening ports:**
- `:80/tcp` - mycart HTTP
- `:443/tcp` - mycart HTTPS
- `:5222/tcp` - XMPP C2S
- `:5223/tcp` - XMPP C2S Direct TLS
- `:5269/tcp` - XMPP S2S
- `:443/udp` - XMPP QUIC
- `:5280/tcp` - prosody WebSocket (public)

### Step 9: Test SSL Certificate (with real domain)

If using a real domain with DNS configured:

```bash
# Check if autocert acquired certificate
ls -la /srv/data/certs/

# Test HTTPS
curl -v https://yourdomain.com/
```

**Expected:**
- `/srv/data/certs/lc_certs/yourdomain.com/` contains autocert cache
- HTTPS works with valid Let's Encrypt certificate
- Certificate auto-renews ~30 days before expiration

### Step 10: Test XMPP Connectivity

Using an XMPP client (e.g., Gajim, Conversations):

1. Configure client to connect to `yourdomain.com:5222`
2. Create test account or connect with existing credentials
3. Send test message
4. Verify message delivery and logging

### Step 11: Test fail2ban-rs

```bash
# Check fail2ban logs
docker exec xmpp-proxy-stack /bin/busybox cat /logs/fail2ban.log

# Verify nftables rules (if fail2ban active)
docker exec xmpp-proxy-stack /usr/sbin/nft list tables
```

## Validation Checklist

- [ ] Volumes created at `/srv/data/`
- [ ] `.env` configured with real domain
- [ ] DNS A/AAAA record points to server
- [ ] Image built successfully (all 6 stages)
- [ ] All services started
- [ ] Horust running with 3 processes (mycart, xmpp-proxy, fail2ban-rs)
- [ ] mycart HTTP endpoint responds
- [ ] All XMPP ports listening
- [ ] SSL certificate acquired (if real domain)
- [ ] XMPP client can connect
- [ ] fail2ban-rs logs show activity

## Troubleshooting

### Issue: Volume permission errors

```bash
# Fix ownership
sudo chown -R root:root /srv/data/{certs,logs,fail2ban,mycart}
sudo chown -R 999:999 /srv/data/prosody
```

### Issue: Build fails at frontend stage

```bash
# Check npm version
docker run --rm node:22-alpine npm --version

# Rebuild with no cache
docker-compose build --no-cache xmpp-proxy-stack
```

### Issue: mycart fails to start

```bash
# Check mycart logs
docker-compose logs xmpp-proxy-stack | grep mycart

# Verify environment variables
docker exec xmpp-proxy-stack /bin/busybox env | grep MYCART
```

### Issue: SSL certificate not acquired

**Requirements for autocert:**
1. Domain must have DNS A/AAAA record pointing to server
2. Port 80 must be accessible from internet (for ACME HTTP-01 challenge)
3. `MYCART_DOMAIN` must match `XMPP_DOMAIN` in `.env`

**Debug:**
```bash
# Check mycart logs for autocert errors
docker-compose logs xmpp-proxy-stack | grep -i "acme\|cert"

# Verify domain resolves
dig yourdomain.com A
```

### Issue: XMPP clients can't connect

```bash
# Check xmpp-proxy logs
docker-compose logs xmpp-proxy-stack | grep xmpp-proxy

# Verify prosody is reachable
docker exec xmpp-proxy-stack /bin/busybox nc -zv 127.0.0.1 15222
docker exec xmpp-proxy-stack /bin/busybox nc -zv 127.0.0.1 15269
```

## Success Criteria (from Spec)

All 10 success criteria from the design spec:

1. ✅ Single `docker-compose up` starts all services
2. ⏳ mycart accessible at `https://example.com/` (requires DNS setup)
3. ⏳ mycart admin accessible at `https://example.com/_/` (requires DNS setup)
4. ⏳ SSL certificate auto-generated by mycart's autocert (requires DNS setup)
5. ⏳ XMPP C2S working on port 5222/5223 (requires testing)
6. ⏳ XMPP S2S working on port 5269 (requires testing)
7. ⏳ fail2ban-rs blocks abusive IPs (requires monitoring)
8. ✅ All logs written to `/srv/data/logs/`
9. ✅ Data persisted to `/srv/data/mycart/` and `/srv/data/prosody/`
10. ⏳ Cert renewal works automatically (requires 30-day test)

**Legend:**
- ✅ Implemented and verified in code
- ⏳ Requires deployment testing with real domain

## Production Readiness

Before deploying to production:

1. **Firewall Configuration**
   - Allow ports: 80, 443, 5222, 5223, 5269, 443/udp, 5280
   - Block all other ports
   - Consider geoblocking if applicable

2. **Monitoring**
   - Set up uptime monitoring for mycart (`:80/health`)
   - Monitor XMPP service availability
   - Set up alerts for certificate expiration (30-day window)
   - Monitor fail2ban logs for attack patterns

3. **Backup Strategy**
   - Regular backups of `/srv/data/mycart/lc_base/` (SQLite database)
   - Backup `/srv/data/prosody/` (user accounts, messages)
   - Backup `/srv/data/certs/` (SSL certificates)

4. **Security Hardening**
   - Review fail2ban-rs configuration for your threat model
   - Consider adding intrusion detection (e.g., OSSEC)
   - Regular security updates: `docker-compose pull && docker-compose up -d`

5. **Performance Tuning**
   - Monitor resource usage (CPU, memory, disk I/O)
   - Adjust Horust restart attempts if services unstable
   - Consider rate limits in mycart application layer

## Rollback Plan

If issues occur in production:

```bash
# Stop services
docker-compose down

# Revert to previous deployment (if applicable)
git checkout main
docker-compose up -d

# Data is preserved in /srv/data/ (not affected by rollback)
```

## Integration Complete

All implementation work is complete and committed to the `feature/mycart-xmpp-integration` branch. The system is ready for deployment testing with a real domain.

**Next action:** Follow the testing steps above to validate the deployment.
