# Certificate Wait Mechanism Deployment Guide

## What Was Fixed

**Commit 429f00d8:**

1. **Statistics provider error** - Fixed empty string handling (same pattern as SQL driver)
2. **Certificate wait mechanism** - xmpp-proxy now waits for certs instead of crash-looping
3. **Log diagnostic tool** - `check-logs.sh` reads actual service logs from `/srv/data/logs/`

## Critical: Service Logs Are in Files, Not Console

The dure-mycart container logs services to **files**, not stdout:
- `/srv/data/logs/mycart-stdout.log` - mycart application logs
- `/srv/data/logs/mycart-stderr.log` - mycart errors
- `/srv/data/logs/xmpp-proxy-stdout.log` - xmpp-proxy logs
- `/srv/data/logs/xmpp-proxy-stderr.log` - xmpp-proxy errors

**Use `check-logs.sh` to see what's actually happening!**

## Deployment Steps

### Step 1: Connect to Remote Server

```bash
K=$(mktemp) && cat > $K <<'EOF'
-----BEGIN OPENSSH PRIVATE KEY-----
b3BlbnNzaC1rZXktdjEAAAAABG5vbmUAAAAEbm9uZQAAAAAAAAABAAAAMwAAAAtzc2gtZW
QyNTUxOQAAACDwhcbJhObkDJJ2keo595U62clTEktNNEUPRWazCFln4AAAAJASNFZ4EjRW
eAAAAAtzc2gtZWQyNTUxOQAAACDwhcbJhObkDJJ2keo595U62clTEktNNEUPRWazCFln4A
AAAED7oqA0BIxT1SzcSY6fSa5XTPIdkrmAe6VTIubKdAq3fvCFxsmE5uQMknaR6jn3lTrZ
yVMSS000RQ9FZrMIWWfgAAAAC2R1cmUtdm0ta2V5AQI=
-----END OPENSSH PRIVATE KEY-----
EOF
chmod 600 $K && ssh -i $K root@35.212.193.170 && rm $K
```

### Step 2: Pull Latest Code and Check Logs

```bash
cd /srv/dure-mycart

# Pull latest commit (429f00d8)
git pull origin main

# Check what's preventing certificate generation
cd prosody-mycart-stack
bash check-logs.sh
```

**Expected output from `check-logs.sh`:**
- mycart startup logs showing HTTP server start or errors
- xmpp-proxy wrapper logs showing "Waiting for certificates..."
- Any ACME/certificate errors

### Step 3: Diagnose Certificate Issue

Based on `check-logs.sh` output, look for:

#### Issue A: mycart Not Starting
```
mycart-stderr.log shows: "bind: address already in use" or crash
```
**Fix**: Check if another process is using port 80/443
```bash
netstat -tlnp | grep -E ':80|:443'
docker compose down
docker compose up -d
```

#### Issue B: ACME Challenge Failing
```
mycart-stderr.log shows: "acme: authorization error" or DNS errors
```
**Fix**: Verify DNS and HTTP accessibility
```bash
# Check DNS
dig dure.co +short
# Should return: 35.212.193.170

# Check HTTP from outside
curl -I http://dure.co
# Should return HTTP response (not connection timeout)
```

#### Issue C: Missing Environment Variable
```
docker-entrypoint.sh: ERROR: MYCART_DOMAIN not set
```
**Fix**: Check .env file
```bash
grep MYCART_DOMAIN /srv/dure-mycart/prosody-mycart-stack/.env
# Should show: MYCART_DOMAIN=dure.co
```

### Step 4: Rebuild Image (Required for Wrapper)

The xmpp-proxy-wrapper.sh is built into the Docker image, so you need to rebuild:

```bash
cd /srv/dure-mycart/prosody-mycart-stack

# Stop containers
docker compose down

# Rebuild with new wrapper (this may take 5-10 minutes)
docker compose build --no-cache dure-mycart

# Start with new image
docker compose up -d
```

**Alternative: Use Pre-Built Image (if available)**
```bash
# Pull latest image from GitHub Container Registry
docker pull ghcr.io/dure-one/prosody-mycart:latest

# Start
docker compose up -d
```

### Step 5: Monitor Certificate Generation

```bash
# Watch xmpp-proxy wrapper waiting for certs
tail -f /srv/data/logs/xmpp-proxy-stdout.log

# In another terminal, watch certificate directory
watch -n 2 'ls -lah /srv/data/certs/'

# Watch mycart logs for ACME activity
tail -f /srv/data/logs/mycart-stdout.log
```

**Success indicators:**
- mycart logs show "Server started on :80"
- ACME challenge logs appear (if first run)
- `/srv/data/certs/privkey.pem` created
- xmpp-proxy wrapper logs "✓ Certificates found, starting xmpp-proxy"

### Step 6: Validate Deployment

```bash
cd /srv/dure-mycart/prosody-mycart-stack

# Check logs again
bash check-logs.sh

# Run validation script
bash validate-deployment.sh

# Check all services running
docker compose ps
```

## Troubleshooting

### Wrapper Timeout After 5 Minutes

If xmpp-proxy wrapper times out:
```
[xmpp-proxy-wrapper] ✗ Timeout waiting for certificates after 300s
[xmpp-proxy-wrapper] Check mycart logs: /logs/mycart-stderr.log
```

**Action**: Check mycart logs for the root cause:
```bash
cat /srv/data/logs/mycart-stderr.log
```

Common causes:
- Port 80 blocked → ACME HTTP-01 challenge fails
- DNS not resolving → ACME can't verify domain
- mycart crashed during startup → check stderr for panic/error

### Image Rebuild Taking Too Long

If you can't rebuild locally, wait for GitHub Actions to build and push:
1. Check GitHub Actions workflow status: https://github.com/dure-one/dure-mycart/actions
2. Wait for "Build and Push" workflow to complete
3. Pull latest image: `docker pull ghcr.io/dure-one/prosody-mycart:latest`

### Certificates Still Not Generating

If after rebuild certificates still don't generate:
```bash
# Check mycart is actually running
docker compose exec dure-mycart ps aux | grep mycart

# Check mycart can write to /certs
docker compose exec dure-mycart touch /certs/test && echo "✓ Write OK" || echo "✗ No write permission"

# Check mycart environment
docker compose exec dure-mycart env | grep MYCART
```

## Success Criteria

- [ ] `check-logs.sh` shows mycart started successfully
- [ ] xmpp-proxy wrapper logs "✓ Certificates found"
- [ ] `/srv/data/certs/privkey.pem` exists
- [ ] No statistics provider errors in Prosody logs
- [ ] `validate-deployment.sh` passes
- [ ] All services healthy in `docker compose ps`

## Next Steps After Success

1. Test XMPP connection: `prosodyctl check`
2. Create admin user: `docker compose exec prosody prosodyctl adduser admin@dure.co`
3. Access web admin: `https://dure.co/prosody/admin/`

---

**If stuck, share output of:**
```bash
bash check-logs.sh > diagnostic.log 2>&1
cat diagnostic.log
```
