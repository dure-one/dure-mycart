# Prosody Security Audit - 2026-10-07

## Executive Summary

Security audit of Prosody 13.0 XMPP server in prosody-mycart-stack identified **3 CRITICAL** and **5 HIGH** severity findings requiring immediate remediation.

## Critical Findings

### 1. PROXY Protocol Trust Boundary Too Broad
- **Severity**: HIGH
- **Location**: `templates/prosody-proxy.cfg.lua.template:45`
- **Issue**: Trusts entire Docker bridge range `172.16.0.0/12` (268M IPs)
- **Risk**: Any container on bridge network can spoof client IPs via PROXY headers
- **Fix**: Narrow to xmpp-proxy container only:
  ```lua
  proxy_trusted_proxies = {
      "127.0.0.1",
      "::1",
      "172.19.0.3"  -- xmpp-proxy container static IP
  }
  ```

### 2. Open User Registration Without Rate Limiting
- **Severity**: CRITICAL
- **Location**: `templates/prosody-proxy.cfg.lua.template:128`
- **Issue**: `mod_register` enabled with no rate limiting
- **Risk**: Unlimited spam account creation, resource exhaustion
- **Fix**: Disable or add rate limiting:
  ```lua
  -- Option 1: Disable (recommended)
  modules_disabled = { "register" };
  
  -- Option 2: Add rate limiting
  registration_throttle_max = 5
  registration_throttle_period = 3600
  registration_blacklist = { "..." }
  ```

### 3. S2S Certificate Validation Disabled
- **Severity**: CRITICAL
- **Location**: `templates/prosody-proxy.cfg.lua.template:75`
- **Issue**: `s2s_secure_auth = false` allows MITM on federation
- **Risk**: Attacker can impersonate federated servers
- **Fix**: Verify xmpp-proxy validates certs, or enable:
  ```lua
  s2s_secure_auth = true
  s2s_require_encryption = true
  ```

## High Severity Findings

### 4. No Federation Domain Filtering
- **Severity**: HIGH
- **Issue**: Accepts S2S connections from any domain
- **Risk**: Spam relay, malicious federation
- **Fix**: Add whitelist or content filtering module

### 5. Admin Web Interface Exposed
- **Severity**: HIGH
- **Location**: Port 5280 accessible on bridge network
- **Issue**: No IP whitelist, relies only on auth
- **Risk**: Brute force, zero-day exploits
- **Fix**: Add `http_interfaces = { "127.0.0.1" }` or IP whitelist

### 6. BOSH Timeout DoS Potential
- **Severity**: MEDIUM
- **Location**: `templates/prosody-proxy.cfg.lua.template:143`
- **Issue**: `bosh_max_wait = 120s` allows connection holding
- **Fix**: Reduce to 30s:
  ```lua
  bosh_max_wait = 30
  ```

### 7. fail2ban-rs Config Not Mounted (FIXED)
- **Severity**: HIGH
- **Issue**: fail2ban-rs config not mounted to container
- **Status**: ✅ FIXED
- **Fix Applied**: Added mount in docker-compose.yml

### 8. Missing HTTPS Port in Dev Compose
- **Severity**: MEDIUM
- **Issue**: docker-compose.dev.yml missing 443/tcp port
- **Status**: ✅ FIXED

## Medium Severity Findings

### 9. Log Flooding Risk
- **Issue**: No rate limit on log writes
- **Risk**: Disk fill DoS
- **Fix**: Add log rotation in docker-compose

### 10. S2S fail2ban Threshold Too High
- **Location**: `dure-mycart/fail2ban-rs-config.toml:79`
- **Issue**: `max_retry = 10` for S2S jail
- **Fix**: Reduce to 5-7 for faster blocking

### 11. Legacy SSL Port Enabled
- **Issue**: Port 5223 supports TLS 1.0/1.1
- **Risk**: Protocol downgrade attacks
- **Fix**: Disable if not needed

## Fixes Applied (2026-10-07)

### fail2ban-rs Log Integration
**Problem**: fail2ban-rs running but not reading Prosody/mycart logs

**Root Cause**: Config file not mounted to container

**Solution**:
1. Added fail2ban-rs config mount to both compose files:
   ```yaml
   - ./dure-mycart/fail2ban-rs-config.toml:/etc/fail2ban-rs/config.toml:ro
   ```

2. Added missing custom modules mount to dev compose:
   ```yaml
   - ../prosody-mycart-stack/custom-modules:/usr/lib/prosody/custom:ro
   ```

3. Added missing HTTPS port to dev compose:
   ```yaml
   - "443:443/tcp"  # HTTPS (mycart web + ACME TLS-ALPN)
   ```

**Verification**:
```bash
cd prosody-mycart-stack
./verify-fail2ban.sh
```

## Log Path Configuration (Verified Correct)

| Service | Container Path | Host Path | fail2ban Path |
|---------|---------------|-----------|---------------|
| Prosody | `/var/log/prosody/prosody.log` | `${DATA_DIR}/logs/prosody/` | `/logs/prosody/prosody.log` ✓ |
| mycart | `/logs/mycart-stderr.log` | `${DATA_DIR}/logs/` | `/logs/mycart-stderr.log` ✓ |
| xmpp-proxy | `/var/log/xmpp-proxy/xmpp-proxy.log` | `${DATA_DIR}/logs/xmpp-proxy/` | `/var/log/xmpp-proxy/xmpp-proxy.log` ✓ |

## Recommended Immediate Actions

1. **CRITICAL**: Narrow PROXY trust to xmpp-proxy IP only
2. **CRITICAL**: Disable `mod_register` or add strict rate limiting
3. **CRITICAL**: Enable S2S certificate validation or verify xmpp-proxy does it
4. **HIGH**: Add federation domain whitelist
5. **HIGH**: Restrict admin interface to localhost
6. **MEDIUM**: Test fail2ban integration with `./verify-fail2ban.sh`

## Next Steps

1. Apply critical security fixes
2. Test with `prosodyctl check config`
3. Restart stack: `docker compose down && docker compose up -d`
4. Run verification: `./verify-fail2ban.sh`
5. Monitor logs: `docker logs -f prosody dure-mycart`
6. Test auth failures trigger bans

## References

- Prosody Security Guide: https://prosody.im/doc/security
- XMPP Compliance: https://compliance.conversations.im/
- fail2ban-rs: reference/fail2ban-rs/README.md
- PROXY Protocol Spec: https://www.haproxy.org/download/1.8/doc/proxy-protocol.txt
