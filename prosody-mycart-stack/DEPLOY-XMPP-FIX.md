# XMPP Connection Fix Deployment Guide

## Issue
All XMPP connection test methods fail with connection errors:
- WebSocket: `connection reset by peer`
- BOSH: `EOF`
- Direct TLS/STARTTLS: `i/o timeout`

## Root Cause
Prosody's HTTP server (port 5280) bound to localhost only. mycart container cannot reach `127.0.0.1` inside prosody container at `172.19.0.2`.

## Fix
Updated `http_interfaces` in prosody template to include bridge network IP:
```lua
http_interfaces = { "127.0.0.1", "::1", "172.19.0.2" }
```

## Deployment Steps (Production: 35.212.193.170)

### 1. Pull latest code
```bash
cd /srv/dure-mycart
git fetch origin
git checkout test
git pull origin test
```

### 2. Verify template change
```bash
grep "http_interfaces" prosody-mycart-stack/templates/prosody-proxy.cfg.lua.template
# Should show: http_interfaces = { "127.0.0.1", "::1", "172.19.0.2" }
```

### 3. Run tests
```bash
# Test template
prosody-mycart-stack/tests/test_template_http_interfaces.sh
```

### 4. Re-render prosody config
```bash
cd prosody-mycart-stack
docker-compose run --rm prosody-config-init
```

### 5. Verify rendered config
```bash
grep "http_interfaces" ../generated/proxy.cfg.lua
# Should show: http_interfaces = { "127.0.0.1", "::1", "172.19.0.2" }

# Run integration test
../prosody-mycart-stack/tests/test_prosody_http_interfaces.sh
```

### 6. Restart prosody container
```bash
docker-compose restart prosody
```

### 7. Wait for prosody to be healthy
```bash
docker-compose ps prosody
# Wait until STATUS shows "healthy"

# Check logs
docker logs prosody --tail 50
```

### 8. Test XMPP connection from admin UI
1. Navigate to: https://dure.co/_/settings/responder
2. Click "연결 테스트" (Connection Test)
3. All 4 methods should succeed:
   - ✓ WebSocket (wss://dure.co/ws)
   - ✓ BOSH (https://dure.co/http-bind/)
   - ✓ Direct TLS (dure.co:5222)
   - ✓ STARTTLS (dure.co:5222)

## Verification Commands

### Check prosody is listening on correct interfaces
```bash
docker exec prosody netstat -tlnp | grep 5280
# Should show:
#   tcp  0  0 127.0.0.1:5280     0.0.0.0:*  LISTEN  1/lua5.4
#   tcp  0  0 172.19.0.2:5280    0.0.0.0:*  LISTEN  1/lua5.4
```

### Test connectivity from mycart container
```bash
docker exec dure-mycart nc -zv 172.19.0.2 5280
# Should succeed: Connection to 172.19.0.2 5280 port [tcp/*] succeeded!
```

### Check mycart logs during connection test
```bash
docker logs -f dure-mycart
# Should show successful WebSocket/BOSH proxying (no "connection refused")
```

## Rollback (if needed)

If issues occur:

```bash
cd /srv/dure-mycart/prosody-mycart-stack

# Restore old config (localhost only)
git checkout HEAD~1 templates/prosody-proxy.cfg.lua.template

# Re-render
docker-compose run --rm prosody-config-init

# Restart prosody
docker-compose restart prosody
```

## Security Notes

- Port 5280 is NOT exposed to host (no Docker port mapping)
- Bridge network 172.19.0.0/16 is isolated (only mycart + prosody containers)
- Security boundary is at Docker port mapping level, not bind interface level
- No authentication needed at proxy level (network isolation sufficient)

## Commit
- Branch: `test`
- Commit: `b63c6ce`
- Message: "fix: bind prosody HTTP interface to bridge network IP for mycart proxy"
