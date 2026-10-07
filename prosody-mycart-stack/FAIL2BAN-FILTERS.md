# fail2ban-rs Filter Documentation

## Overview

fail2ban-rs monitors logs from three services to detect and ban malicious IPs:
- **Prosody**: XMPP authentication failures
- **mycart**: HTTP authentication failures
- **xmpp-proxy**: XMPP connection/protocol errors

## Log Formats

### Prosody (mod_log_auth)

**Log file**: `/logs/prosody/prosody.log`  
**Date format**: syslog (`Oct 07 01:29:28`)

**Successful authentication**:
```
Oct 07 01:29:28 c2s56211c6435a0 info    Authenticated as admin@dure.co [prosody:operator]
```

**Failed authentication** (from mod_log_auth):
```
Oct 07 14:32:15 dure.co:saslauth info    Failed authentication attempt (not-authorized) for user test@dure.co from IP: 203.0.113.42
```

**Filter regex**:
```toml
filter = [
    'Failed authentication attempt .* from IP: <HOST>',
]
```

**Test command**:
```bash
echo 'Oct 07 14:32:15 dure.co:saslauth info Failed authentication attempt (not-authorized) for user test@dure.co from IP: 203.0.113.42' | grep -E 'Failed authentication attempt .* from IP: [0-9.]+'
```

---

### mycart (zerolog JSON)

**Log file**: `/logs/mycart-stderr.log`  
**Date format**: iso8601 (Unix timestamp in `time` field)

**Successful request**:
```json
{"level":"info","ip":"141.101.86.206","latency":"438.06µs","status":200,"method":"GET","url":"/","time":1791334364,"message":"Success"}
```

**Failed authentication** (added in `internal/handlers/private/auth.go`):
```json
{"level":"warn","ip":"203.0.113.42","email":"test@example.com","time":1791334500,"message":"authentication failed: wrong password"}
```

**Filter regex**:
```toml
filter = [
    '"level":"warn".*"ip":"<HOST>".*"message":"authentication failed',
]
```

**Matches both**:
- `authentication failed: unknown email`
- `authentication failed: wrong password`

**Test command**:
```bash
echo '{"level":"warn","ip":"203.0.113.42","email":"test@example.com","time":1791334500,"message":"authentication failed: wrong password"}' | grep -E '"level":"warn".*"ip":"[0-9.]+".*"message":"authentication failed'
```

---

### xmpp-proxy (Rust log crate)

**Log file**: `/logs/xmpp-proxy-stdout.log` (NOT stderr - stderr is empty!)  
**Date format**: iso8601 (in log line prefix)

**Successful connection**:
```
[INFO  xmpp_proxy::tls::incoming] FZdF56lr5D: ([::ffff:104.28.243.28]:56119 -> (tcp-in-unk)): connected
[INFO  xmpp_proxy::context] FZdF56lr5D: ([::ffff:104.28.243.28]:56119 (admin@dure.co) -> (directtls-in-c2s) -> unk (dure.co)): stream data set
```

**Connection/TLS errors**:
```
[ERROR xmpp_proxy::tls::incoming] 0NPXhnKjpt: ([::ffff:199.45.155.29]:33660 (unk) -> (directtls-in-unk) -> unk (unk)): peer is incompatible: SignatureAlgorithmsExtensionRequired
[ERROR xmpp_proxy::tls::incoming] zwrvVeSPgk: ([::ffff:40.124.171.180]:39814 (unk) -> (starttls-in-unk) -> unk (unk)): stream ended before open
[ERROR xmpp_proxy::tls::incoming] YP3bnK5Yox: ([::ffff:40.124.171.180]:52114 -> (tcp-in-unk)): not enough bytes
```

**IP address format**: `[::ffff:IPv4]:port` (IPv4-mapped IPv6)

**C2S filter regex**:
```toml
filter = [
    '\[ERROR[^\]]*\] [^:]+: \(\[::(ffff:)?<HOST>\]:[0-9]+.*\) (peer is incompatible|not enough bytes|stream ended)',
]
```

**S2S filter regex**:
```toml
filter = [
    '\[ERROR[^\]]*\] [^:]+: \(\[::(ffff:)?<HOST>\]:[0-9]+.*(s2s|->).*\) (peer is incompatible|not enough bytes|stream ended)',
]
```

**Test commands**:
```bash
# C2S test
echo '[ERROR xmpp_proxy::tls::incoming] ABC: ([::ffff:203.0.113.42]:12345 (unk) -> (directtls-in-unk) -> unk (unk)): peer is incompatible: SignatureAlgorithmsExtensionRequired' | grep -E '\[ERROR[^\]]*\] [^:]+: \(\[::ffff:[0-9.]+\]:[0-9]+.*\) (peer is incompatible|not enough bytes|stream ended)'

# S2S test
echo '[ERROR xmpp_proxy::tls::incoming] XYZ: ([::ffff:203.0.113.42]:5269 -> s2s): stream ended before open' | grep -E '\[ERROR[^\]]*\] [^:]+: \(\[::ffff:[0-9.]+\]:[0-9]+.*(s2s|->).*\) (peer is incompatible|not enough bytes|stream ended)'
```

---

## fail2ban-rs Configuration

**File**: `prosody-mycart-stack/dure-mycart/fail2ban-rs-config.toml`

### Jail Parameters

| Parameter | Prosody | mycart | xmpp-proxy C2S | xmpp-proxy S2S |
|-----------|---------|--------|----------------|----------------|
| `max_retry` | 5 | 5 | 5 | 10 |
| `find_time` | 10m | 10m | 10m | 10m |
| `ban_time` | 1h | 1h | 1h | 1h |
| `backend` | nftables | nftables | nftables | nftables |

### Why different max_retry for S2S?

S2S connections can legitimately fail due to:
- DNS resolution issues
- Certificate mismatches
- Protocol version incompatibilities

Setting `max_retry = 10` reduces false positives from legitimate federation attempts.

---

## Testing Filters

### 1. Trigger Prosody auth failure

**Method 1**: XMPP client with wrong password

**Method 2**: Manual SASL (requires netcat + base64)

**Expected log**:
```
Failed authentication attempt (not-authorized) for user test@dure.co from IP: <client-ip>
```

### 2. Trigger mycart auth failure

```bash
curl -X POST https://dure.co/api/sign/in \
  -H "Content-Type: application/json" \
  -d '{"email":"test@example.com","password":"wrong"}'
```

**Expected log**:
```json
{"level":"warn","ip":"<client-ip>","email":"test@example.com","time":1234567890,"message":"authentication failed: wrong password"}
```

### 3. Trigger xmpp-proxy errors

Difficult to trigger intentionally (requires TLS handshake failures or malformed XMPP).

Check existing errors in logs:
```bash
docker exec dure-mycart grep ERROR /logs/xmpp-proxy-stdout.log | head -5
```

---

## Verification Commands

### Check fail2ban-rs status
```bash
docker exec dure-mycart fail2ban-rs status
```

### List banned IPs
```bash
docker exec dure-mycart nft list ruleset | grep -A5 fail2ban
```

### Monitor logs in real-time
```bash
# Prosody
docker exec prosody tail -f /var/log/prosody/prosody.log

# mycart
docker exec dure-mycart tail -f /logs/mycart-stderr.log | grep warn

# xmpp-proxy
docker exec dure-mycart tail -f /logs/xmpp-proxy-stdout.log | grep ERROR
```

### Manually ban/unban IP (for testing)
```bash
# Ban
docker exec dure-mycart fail2ban-rs ban <jail-name> <ip-address>

# Unban
docker exec dure-mycart fail2ban-rs unban <jail-name> <ip-address>
```

---

## Troubleshooting

### Logs not being monitored

Check fail2ban-rs stderr for errors:
```bash
docker exec dure-mycart tail -50 /logs/fail2ban-rs-stderr.log
```

Look for:
- `WARN log open failed, retrying` - file doesn't exist
- `ERROR failed to parse date` - wrong date_format setting

### Filter not matching

Test regex against actual log lines:
```bash
# Extract a sample log line
docker exec dure-mycart tail -1 /logs/mycart-stderr.log > /tmp/sample.log

# Test regex
cat /tmp/sample.log | grep -E 'your-regex-here'
```

### False positives

If legitimate traffic is being banned:
1. Check banned IPs: `nft list ruleset | grep fail2ban`
2. Review logs to confirm pattern
3. Adjust `max_retry` or refine filter regex
4. Add IPs to `ignoreip` list in config

---

## Log Rotation

fail2ban-rs tracks log file position. After log rotation:
1. fail2ban-rs detects file change (inode change)
2. Automatically reopens new file
3. Continues monitoring from beginning

**No restart required** for log rotation.

---

## References

- fail2ban-rs: reference/fail2ban-rs/README.md
- Prosody mod_log_auth: prosody-modules/mod_log_auth/
- mycart auth handler: internal/handlers/private/auth.go
- xmpp-proxy source: reference/xmpp-proxy/src/
