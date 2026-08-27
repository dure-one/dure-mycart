# Testing Guide: Port Reconfiguration with fail2ban-rs

## Changes Summary

### Configuration Updates
1. **Prosody**: Now localhost-only (removed public 0.0.0.0:5280)
2. **mycart Reverse Proxy**: Handles prosody HTTP endpoints via HTTPS
   - `/xmpp-websocket` → WebSocket connections
   - `/http-bind` → BOSH (legacy clients)
   - `/http_file_upload` → File uploads
3. **fail2ban-rs**: Configured with ipset backend + DOCKER-USER chain

### Expected Port Layout

| Service | Port | Protocol | Binding | Purpose |
|---------|------|----------|---------|---------|
| mycart | 80 | TCP | 0.0.0.0 | HTTP (public) |
| mycart | 443 | TCP | 0.0.0.0 | HTTPS (public) |
| xmpp-proxy | 5222 | TCP | 0.0.0.0 | XMPP C2S StartTLS (public) |
| xmpp-proxy | 5223 | TCP | 0.0.0.0 | XMPP C2S Direct TLS (public) |
| xmpp-proxy | 5269 | TCP | 0.0.0.0 | XMPP S2S (public) |
| xmpp-proxy | 443 | UDP | 0.0.0.0 | XMPP over QUIC (public) |
| prosody | 15222 | TCP | 127.0.0.1 | C2S backend (localhost only) |
| prosody | 15269 | TCP | 127.0.0.1 | S2S backend (localhost only) |
| prosody | 15280 | TCP | 127.0.0.1 | HTTP backend (localhost only) |

## Pre-Deployment Checklist

Before deploying to production:

- [ ] Copy `.env.example` to `.env` and update with your domain
- [ ] Verify SSL certificates exist in `/srv/data/certs/`
- [ ] Backup current `.env` configuration
- [ ] Review fail2ban-rs regex patterns for your log format

## Testing Steps

### 1. Build and Deploy

```bash
cd /home/wj/work/dure-mycart

# Stop current containers
docker-compose down

# Rebuild the xmpp-proxy-stack image (if source changed)
# OR pull latest if using pre-built image
docker-compose pull xmpp-proxy-stack

# Start services
docker-compose up -d

# Watch logs
docker-compose logs -f
```

### 2. Verify Port Bindings

```bash
# Check all listening ports
ss -nltup | grep -E "80|443|5222|5269|5280|15222|15269|15280"

# Expected output:
# tcp LISTEN 0.0.0.0:80       (mycart in xmpp-proxy-stack)
# tcp LISTEN 0.0.0.0:443      (mycart in xmpp-proxy-stack)
# tcp LISTEN 0.0.0.0:5222     (xmpp-proxy in xmpp-proxy-stack)
# tcp LISTEN 0.0.0.0:5269     (xmpp-proxy in xmpp-proxy-stack)
# udp        0.0.0.0:443      (xmpp-proxy QUIC in xmpp-proxy-stack)
# tcp LISTEN 127.0.0.1:15222  (docker-proxy for prosody)
# tcp LISTEN 127.0.0.1:15269  (docker-proxy for prosody)
# tcp LISTEN 127.0.0.1:15280  (docker-proxy for prosody)

# IMPORTANT: Should NOT see 0.0.0.0:5280 or 0.0.0.0:15280
```

### 3. Test mycart HTTP/HTTPS

```bash
# Test HTTP (should redirect to HTTPS or serve content)
curl -I http://YOUR_DOMAIN/

# Test HTTPS
curl -I https://YOUR_DOMAIN/
```

### 4. Test Prosody Reverse Proxy Endpoints

**CRITICAL: WebSocket Test**

```bash
# Test WebSocket endpoint (should return 400 or upgrade headers)
curl -I https://YOUR_DOMAIN/xmpp-websocket

# Better WebSocket test with wscat:
# npm install -g wscat
wscat -c wss://YOUR_DOMAIN/xmpp-websocket
# Should connect or show XMPP-specific error, NOT 404
```

**BOSH Test**

```bash
# Test BOSH endpoint
curl -I https://YOUR_DOMAIN/http-bind
# Should return 200 or XMPP-specific headers, NOT 404
```

**File Upload Test**

```bash
# Test file upload endpoint
curl -I https://YOUR_DOMAIN/http_file_upload
# Should return prosody response, NOT 404
```

### 5. Verify fail2ban-rs

```bash
# Check fail2ban-rs is running
docker exec xmpp-proxy-stack ps aux | grep fail2ban

# Check ipset sets were created
docker exec xmpp-proxy-stack ipset list
# Expected: f2b-xmpp-auth, f2b-xmpp-auth6, f2b-mycart-auth, f2b-mycart-auth6

# Check iptables rules in DOCKER-USER chain
docker exec xmpp-proxy-stack iptables -L DOCKER-USER -n -v
# Should see fail2ban match rules

# Check fail2ban-rs logs
docker exec xmpp-proxy-stack cat /logs/fail2ban-rs-stdout.log
# Should see jail initialization messages
```

### 6. Test fail2ban-rs Banning (Optional)

**WARNING: Only test from a non-critical IP!**

```bash
# Simulate failed auth attempts to trigger ban
# (Adjust based on your auth endpoint)
for i in {1..10}; do
  curl -X POST https://YOUR_DOMAIN/api/login \
    -H "Content-Type: application/json" \
    -d '{"email":"fake@test.com","password":"wrong"}'
done

# Check if IP was banned
docker exec xmpp-proxy-stack ipset list f2b-mycart-auth
# Should see your IP if authentication failures were logged correctly

# Unban yourself
docker exec xmpp-proxy-stack fail2ban-rs unban YOUR_IP
```

### 7. Test XMPP Connections

```bash
# Test C2S port is listening
telnet YOUR_DOMAIN 5222
# Should connect, show XMPP stream start

# Test S2S port is listening
telnet YOUR_DOMAIN 5269
# Should connect, show XMPP stream start

# Test QUIC (requires QUIC client)
# Example with xmpp-proxy client or another QUIC tool
```

## Troubleshooting

### Prosody 5280 Still Public

**Symptom**: `ss -nltup | grep 5280` shows `0.0.0.0:5280`

**Fix**:
```bash
# Verify docker-compose.yaml changes were applied
grep -A 5 "ports:" docker-compose.yaml | grep 5280
# Should only show 127.0.0.1:15280:5280

# Restart containers
docker-compose down && docker-compose up -d
```

### WebSocket 404 Errors

**Symptom**: `curl https://YOUR_DOMAIN/xmpp-websocket` returns 404

**Diagnosis**:
```bash
# Check mycart logs for proxy registration
docker logs xmpp-proxy-stack 2>&1 | grep "Registering reverse proxy"
# Should see: path="/xmpp-websocket/*" target="http://127.0.0.1:15280"

# Check if REVERSE_PROXY_BINDINGS was loaded
docker exec xmpp-proxy-stack env | grep REVERSE_PROXY
# Should show the three bindings

# Verify prosody is listening on 15280
docker exec prosody ss -nltup | grep 5280
# Should show 0.0.0.0:5280 or 127.0.0.1:5280
```

**Fix**:
```bash
# Ensure .env has the correct bindings
cat .env | grep REVERSE_PROXY_BINDINGS

# Restart mycart service
docker-compose restart xmpp-proxy-stack
```

### fail2ban-rs Not Banning

**Symptom**: Failed auth attempts don't trigger bans

**Diagnosis**:
```bash
# Check if logs are being monitored
docker exec xmpp-proxy-stack cat /logs/mycart-stdout.log | grep -i "auth"
docker exec xmpp-proxy-stack cat /logs/xmpp-proxy-stdout.log | grep -i "auth"

# Check fail2ban-rs is reading logs
docker exec xmpp-proxy-stack cat /logs/fail2ban-rs-stdout.log | tail -50
```

**Fix**:
```bash
# Update regex patterns in fail2ban-rs-config.toml to match your log format
# Example mycart log: "authentication failed for user@example.com from 1.2.3.4"
# Example xmpp-proxy log: "authentication failed from 1.2.3.4:12345"

# Restart fail2ban-rs
docker-compose restart xmpp-proxy-stack
```

### DOCKER-USER Chain Not Working

**Symptom**: Banned IPs can still connect

**Diagnosis**:
```bash
# Verify DOCKER-USER chain exists
docker exec xmpp-proxy-stack iptables -L DOCKER-USER
# Should show fail2ban match rules

# Check if ipset is enabled in kernel
docker exec xmpp-proxy-stack lsmod | grep ip_set
```

**Fix**:
```bash
# Install ipset tools in Dockerfile if missing
# Ensure xt_set kernel module is loaded on host
modprobe xt_set
```

## Rollback Procedure

If issues occur, rollback with:

```bash
# Stop containers
docker-compose down

# Restore .env.example to previous version
git checkout HEAD -- .env.example

# Restore docker-compose.yaml
git checkout HEAD -- docker-compose.yaml

# Restore fail2ban-rs config
git checkout HEAD -- xmpp-proxy-stack/fail2ban-rs-config.toml

# Restart with old config
docker-compose up -d
```

## Success Criteria

- ✅ Prosody 5280 NOT accessible from 0.0.0.0
- ✅ WebSocket connects at `wss://YOUR_DOMAIN/xmpp-websocket`
- ✅ BOSH accessible at `https://YOUR_DOMAIN/http-bind`
- ✅ File upload accessible at `https://YOUR_DOMAIN/http_file_upload`
- ✅ fail2ban-rs creates ipset sets
- ✅ fail2ban-rs rules in DOCKER-USER chain
- ✅ XMPP clients can connect to 5222, 5269
- ✅ mycart serves HTTP/HTTPS correctly

## Production Deployment Notes

1. **Backup first**: Backup `/srv/data/prosody` and `/srv/data/mycart`
2. **Test in staging**: If possible, test on a staging server first
3. **Schedule maintenance**: Notify users of brief downtime
4. **Monitor logs**: Watch fail2ban-rs and service logs for the first hour
5. **Update client configs**: Update XMPP client WebSocket URLs to use HTTPS paths

## Contact

If issues persist after troubleshooting, check:
- mycart logs: `docker logs xmpp-proxy-stack 2>&1 | grep mycart`
- xmpp-proxy logs: `docker logs xmpp-proxy-stack 2>&1 | grep xmpp-proxy`
- prosody logs: `docker logs prosody`
- fail2ban-rs logs: `docker exec xmpp-proxy-stack cat /logs/fail2ban-rs-stdout.log`
