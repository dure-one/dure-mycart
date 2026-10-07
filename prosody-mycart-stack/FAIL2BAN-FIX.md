# fail2ban-rs Log Integration Fix

## Issue Found
xmpp-proxy logs to `/logs/xmpp-proxy-stderr.log` but fail2ban-rs was configured to watch `/var/log/xmpp-proxy/xmpp-proxy.log`.

## Fix Applied

**Updated `dure-mycart/fail2ban-rs-config.toml`:**
```toml
[jail.xmpp-proxy-c2s]
log_path = "/logs/xmpp-proxy-stderr.log"  # Was: /var/log/xmpp-proxy/xmpp-proxy.log

[jail.xmpp-proxy-s2s]
log_path = "/logs/xmpp-proxy-stderr.log"  # Was: /var/log/xmpp-proxy/xmpp-proxy.log
```

## Apply the Fix

**Restart dure-mycart container to reload config:**
```bash
cd /srv/dure-mycart/prosody-mycart-stack
docker compose restart dure-mycart
```

**Verify all jails now monitoring correctly:**
```bash
./fail2ban-debug.sh
```

Expected output for step 8:
```
8. Monitored log files:
  /logs/prosody/prosody.log:
    ✓ Exists (56089 bytes)
  /logs/mycart-stderr.log:
    ✓ Exists (38363 bytes)
  /logs/xmpp-proxy-stderr.log:
    ✓ Exists (XXXX bytes)
```

**Check stderr logs - warnings should be gone:**
```bash
docker compose exec dure-mycart busybox tail -20 /logs/fail2ban-rs-stderr.log
```

Should see:
```
INFO watcher started phase=startup jail=xmpp-proxy-c2s path=/logs/xmpp-proxy-stderr.log
INFO watcher started phase=startup jail=xmpp-proxy-s2s path=/logs/xmpp-proxy-stderr.log
```

NO more `WARN log open failed` messages.

## Final Verification

Run full verification:
```bash
./verify-fail2ban.sh
```

All should pass:
- ✅ Config mounted
- ✅ Process running
- ✅ Socket exists
- ✅ Status responds
- ✅ All 3 log files exist
- ✅ nftables rules active
- ✅ 4 jails monitoring

## Test Ban Functionality

**Trigger auth failures** (5+ attempts):
```bash
# Try failed XMPP login 5 times, then check:
docker compose exec dure-mycart /usr/local/bin/fail2ban-rs status
docker compose exec dure-mycart /usr/sbin/nft list set inet fail2ban-rs f2b-xmpp-auth
```

Should see banned IP in the set.
