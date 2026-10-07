# Prosody Security Hardening - 2026-10-07

## Changes Applied

### 1. HTTP Admin Interface Restriction ✅
**Severity**: HIGH  
**File**: `templates/prosody-proxy.cfg.lua.template`

```lua
# Before
http_interfaces = { "*", "::" }  # All interfaces

# After
http_interfaces = { "127.0.0.1", "::1" }  # Localhost only
```

**Impact**: Admin panel accessible ONLY via mycart reverse proxy (with authentication). Prevents direct access from Docker bridge network.

---

### 2. Security Modules Enabled ✅

#### mod_ipcheck - DNS Blacklist Checking
**Purpose**: Blocks connections from known malicious IPs listed in RBL/DNSBL services

**Configuration**:
```lua
ipcheck_mode = "block"  # Block immediately (not just warn)
ipcheck_lists = {
    "xbl.spamhaus.org";
    "dnsbl.sorbs.net";
    "bl.spamcop.net";
}
```

**Integration**: Works with fail2ban-rs for IP-level blocking.

---

#### mod_anti_spam - User-Based Spam Blocking
**Purpose**: Complements fail2ban-rs (IP blocking) with user-level spam detection

**Configuration**:
```lua
anti_spam_check_messages = true      # Check message spam
anti_spam_check_presence = false     # Don't check presence spam
anti_spam_check_subscriptions = true # Check subscription spam
anti_spam_check_muc = false          # MUC not used yet
```

**Architecture**:
- **fail2ban-rs**: IP/host-based blocking (network layer)
- **mod_anti_spam**: User-based blocking (application layer)

---

#### mod_limits - Connection Rate Limiting
**Purpose**: Prevent connection exhaustion and bandwidth abuse

**Configuration**:
```lua
limits = {
    c2s = {
        rate = "3kb/s";   # 3KB/sec per C2S connection
        burst = "2s";     # Allow 2-second bursts
    };
    s2s = {
        rate = "10kb/s";  # 10KB/sec per S2S connection
        burst = "3s";
    };
}
```

---

### 3. Monitoring Modules Enabled ✅

#### mod_log_slow_events
**Purpose**: Performance debugging - logs events slower than threshold

```lua
log_slow_events_threshold = 0.5  # Log events > 500ms
```

**Monitoring**: Check `/var/log/prosody/prosody.log` for slow event warnings.

---

#### mod_server_status
**Purpose**: Server health monitoring endpoint

**Access**: `http://localhost:5280/server-status`  
**Security**: Localhost-only due to `http_interfaces` restriction

---

#### mod_stanza_counter
**Purpose**: XMPP stanza metrics (messages, presence, IQ)

**Access**: Via `prosodyctl shell` or admin_web2 interface

---

### 4. Message Handling Module ⚠️

#### mod_pastebin
**Purpose**: Convert large messages to pastebin links

**Configuration**:
```lua
pastebin_threshold = 500         # Messages > 500 chars
pastebin_line_threshold = 4      # Or > 4 lines
pastebin_expire_after = 86400    # 24 hour expiry
pastebin_trigger = "!paste"      # Manual trigger
```

**⚠️ SECURITY WARNING**: Pastebin is **NOT encrypted** - messages stored in **plain text**.  
**Recommendation**: Consider disabling if handling sensitive communications, or use external encrypted pastebin service.

---

### 5. fail2ban-rs S2S Threshold Reduced ✅
**File**: `dure-mycart/fail2ban-rs-config.toml`

```toml
# Before
max_retry = 10  # Too permissive

# After
max_retry = 7   # Faster S2S abuse blocking
```

**Impact**: Faster detection and blocking of malicious S2S connection attempts.

---

## Deployment Steps

### 1. Verify Configuration
```bash
# Check generated config
grep "http_interfaces" generated/proxy.cfg.lua
grep -E "(ipcheck|anti_spam|limits)" generated/proxy.cfg.lua

# Expected: http_interfaces = { "127.0.0.1", "::1" }
```

### 2. Restart Stack
```bash
cd /srv/dure-mycart/prosody-mycart-stack  # Or your deployment path
docker compose down
docker compose up -d
```

### 3. Wait for Healthy State
```bash
# Wait for prosody to become healthy
docker compose ps

# Should show:
# prosody  ... Up (healthy)
```

### 4. Verify Modules Loaded
```bash
docker compose exec prosody prosodyctl check config

# List enabled modules
docker compose exec prosody prosodyctl shell <<'EOF'
for module in pairs(require'core.modulemanager'.get_modules('*', '*')) do
    print(module)
end
EOF
```

Expected new modules:
- ipcheck
- anti_spam
- log_slow_events
- server_status
- stanza_counter
- pastebin
- limits

### 5. Test HTTP Admin Restriction
```bash
# From host: should FAIL (connection refused)
curl http://localhost:5280/prosody/admin

# From inside container: should SUCCEED (return login page)
docker compose exec prosody curl http://localhost:5280/prosody/admin
```

**Access Method**: Via mycart reverse proxy with authentication ONLY.

### 6. Verify fail2ban-rs
```bash
# Check jail status
./verify-fail2ban.sh

# Or manually:
docker compose exec dure-mycart /usr/local/bin/fail2ban-rs status
```

Expected: 4 jails active (xmpp-auth, mycart-auth, xmpp-proxy-c2s, xmpp-proxy-s2s)

---

## Testing

### Test 1: DNS Blacklist Blocking
1. Configure test IP in RBL (or use known-bad IP)
2. Attempt connection
3. Verify blocked in Prosody logs

### Test 2: Rate Limiting
```bash
# Simulate high-rate connection
# Should be throttled after burst window
```

### Test 3: Slow Event Logging
```bash
# Monitor logs for slow events
docker compose logs -f prosody | grep "Slow event"
```

### Test 4: fail2ban-rs S2S
```bash
# Trigger 7 S2S connection failures
# Verify IP banned after 7th attempt (not 10th)
```

---

## Monitoring

### Key Metrics to Watch

1. **Authentication Failures**:
   ```bash
   docker compose logs prosody | grep "Failed authentication"
   ```

2. **DNS Blacklist Blocks**:
   ```bash
   docker compose logs prosody | grep ipcheck
   ```

3. **Rate Limit Actions**:
   ```bash
   docker compose logs prosody | grep limits
   ```

4. **Slow Events**:
   ```bash
   docker compose logs prosody | grep "Slow event"
   ```

5. **fail2ban-rs Bans**:
   ```bash
   docker compose exec dure-mycart /usr/local/bin/fail2ban-rs status
   ```

### Server Status Dashboard
```bash
# Access server status (localhost only)
docker compose exec prosody curl http://localhost:5280/server-status
```

---

## Rollback Procedure

If issues occur:

```bash
cd /srv/dure-mycart/prosody-mycart-stack

# Revert template changes
git diff HEAD templates/prosody-proxy.cfg.lua.template
git checkout templates/prosody-proxy.cfg.lua.template

# Revert fail2ban config
git checkout dure-mycart/fail2ban-rs-config.toml

# Regenerate config
sh render-prosody-config.sh

# Restart
docker compose down
docker compose up -d
```

---

## Security Posture Summary

### Before Hardening
- HTTP admin accessible from bridge network
- No DNS blacklist checking
- No user-based spam blocking
- No connection rate limiting
- S2S ban threshold: 10 retries

### After Hardening ✅
- ✅ HTTP admin: localhost-only (reverse proxy required)
- ✅ DNS blacklist: 3 RBL services (Spamhaus, SORBS, SpamCop)
- ✅ User spam blocking: mod_anti_spam enabled
- ✅ Rate limiting: 3KB/s C2S, 10KB/s S2S with burst support
- ✅ S2S ban threshold: 7 retries (30% faster blocking)
- ✅ Performance monitoring: slow event logging
- ✅ Health monitoring: server_status endpoint
- ✅ Metrics: stanza counter enabled

### Defense Layers
1. **Network Layer**: fail2ban-rs (IP-based blocking)
2. **DNS Layer**: mod_ipcheck (RBL/DNSBL)
3. **Transport Layer**: TLS validation (xmpp-proxy)
4. **Application Layer**: mod_anti_spam (user-based)
5. **Rate Limiting**: mod_limits (bandwidth/connection)

---

## Known Limitations

1. **mod_pastebin**: NOT encrypted - plain text storage
   - **Mitigation**: Disable if privacy critical, or use external encrypted service

2. **RBL Latency**: DNS queries add latency to connection handshake
   - **Mitigation**: Monitor connection times, adjust RBL list if needed

3. **Rate Limiting**: May affect legitimate high-bandwidth users
   - **Mitigation**: Adjust `limits.c2s.rate` if false positives occur

---

## References

- [mod_ipcheck](https://modules.prosody.im/mod_ipcheck.html)
- [mod_anti_spam](https://modules.prosody.im/mod_anti_spam)
- [mod_log_slow_events](https://modules.prosody.im/mod_log_slow_events.html)
- [mod_server_status](https://modules.prosody.im/mod_server_status.html)
- [mod_stanza_counter](https://modules.prosody.im/mod_stanza_counter.html)
- [mod_pastebin](https://modules.prosody.im/mod_pastebin.html)
- [mod_limits](https://prosody.im/doc/modules/mod_limits)
- [Prosody Security Guide](https://prosody.im/doc/security)

---

## Next Steps

1. ✅ Deploy to production
2. Monitor logs for 24-48 hours
3. Review RBL block rate (adjust lists if needed)
4. Tune rate limits based on actual usage
5. Consider external encrypted pastebin if mod_pastebin used
