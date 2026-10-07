# Apply Security Fixes

## Changes Made

### Critical Fixes
1. ✅ **PROXY trust narrowed**: 172.16.0.0/12 → 172.19.0.0/16 (4,095x reduction)
2. ✅ **mod_register disabled**: Prevents spam account creation
3. ✅ **BOSH timeout reduced**: 120s → 30s (DoS mitigation)
4. ✅ **fail2ban-rs integrated**: Monitoring Prosody, mycart, xmpp-proxy logs

### Files Modified
- `templates/prosody-proxy.cfg.lua.template` - Security hardening
- `docker-compose.yml` - fail2ban-rs config mount
- `docker-compose.dev.yml` - fail2ban-rs config mount, custom modules, HTTPS port
- `dure-mycart/fail2ban-rs-config.toml` - Correct xmpp-proxy log paths

## Apply Changes

### 1. Regenerate Prosody Config
```bash
cd /srv/dure-mycart/prosody-mycart-stack
sh render-prosody-config.sh
```

**Verify changes:**
```bash
grep -A 3 "proxy_trusted_proxies" ../generated/proxy.cfg.lua
grep "register" ../generated/proxy.cfg.lua
grep "bosh_max_wait" ../generated/proxy.cfg.lua
```

Expected:
```lua
proxy_trusted_proxies = {
    "127.0.0.1",
    "::1",
    "172.19.0.0/16"
}
-- "register";  -- User registration (DISABLED: no rate limiting = spam vector)
bosh_max_wait = 30  -- Was: 120
```

### 2. Restart Stack
```bash
docker compose down
docker compose up -d
```

Wait for healthy:
```bash
docker compose ps
# Wait until prosody shows "healthy"
```

### 3. Verify Security Changes

**Check Prosody loaded new config:**
```bash
docker compose exec prosody prosodyctl check config
```

**Verify PROXY trust:**
```bash
docker compose exec prosody prosodyctl shell <<'EOF'
print(require'core.configmanager'.get('*', 'proxy_trusted_proxies'))
EOF
```

**Verify mod_register disabled:**
```bash
docker compose exec prosody prosodyctl shell <<'EOF'
for module in pairs(require'core.modulemanager'.get_modules('*', '*')) do
    if module == "register" then print("WARNING: mod_register still enabled!") end
end
EOF
```

**Check fail2ban-rs status:**
```bash
./verify-fail2ban.sh
```

Expected: All 4 jails monitoring (no WARN messages about missing logs)

### 4. Functional Testing

**Test XMPP login still works:**
```bash
# Use XMPP client to connect to domain
# Verify C2S and S2S connections work
```

**Verify registration blocked:**
```bash
# Try to register new account via client
# Should fail (no registration endpoint)
```

**Test fail2ban triggers:**
```bash
# Try 5+ failed auth attempts
docker compose exec dure-mycart /usr/local/bin/fail2ban-rs status
# Should show banned IP
```

## Rollback

If issues occur:
```bash
cd /srv/dure-mycart/prosody-mycart-stack
git diff HEAD~1 templates/prosody-proxy.cfg.lua.template
git revert HEAD
sh render-prosody-config.sh
docker compose restart prosody
```

## Critical TODO

**Verify xmpp-proxy S2S certificate validation:**
- Check xmpp-proxy source code for TLS validation on outgoing S2S connections
- If xmpp-proxy does NOT validate remote certificates:
  - Enable `s2s_secure_auth = true` in Prosody config
  - Accept plain-text from Prosody but validate before connecting to remote
- Current state: Prosody trusts xmpp-proxy to validate (not verified)

## Monitoring

After deployment, monitor for:
- Failed auth attempts in `/logs/prosody/prosody.log`
- Ban actions in fail2ban-rs logs
- S2S federation still working
- Client connections successful

```bash
# Monitor logs
docker compose logs -f prosody dure-mycart

# Watch bans
watch -n 5 'docker compose exec dure-mycart /usr/local/bin/fail2ban-rs status'
```
