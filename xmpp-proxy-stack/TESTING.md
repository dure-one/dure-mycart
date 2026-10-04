# Testing Docker Images Without Release

## Overview

Test Docker images built in GitHub CI on dure.co server without creating releases.

## How It Works

1. **Push to `test` branch** → GitHub Actions builds image
2. **Image tagged as:** `ghcr.io/dure-one/dure-mycart-prosody:test`
3. **Pull on server:** Use `pull-test-image.sh` script
4. **Test:** Runs on separate ports (no conflict with production)

## Tag Strategy

| Trigger | Tags Generated |
|---------|----------------|
| `test` branch push | `test`, `sha-abc1234` |
| Tag `v1.2.3` | `1.2.3`, `1.2`, `latest`, `sha-abc1234` |

**Production uses `latest` → test images isolated.**

## Server Setup (One-Time)

### 1. Authenticate to GHCR

```bash
# Create GitHub PAT with read:packages scope at:
# https://github.com/settings/tokens/new

# Login to GHCR
echo $GITHUB_PAT | docker login ghcr.io -u YOUR_USERNAME --password-stdin
```

### 2. Copy Test Script

```bash
# On dure.co server
scp xmpp-proxy-stack/pull-test-image.sh dure.co:/opt/dure-mycart/
chmod +x /opt/dure-mycart/pull-test-image.sh
```

## Testing Workflow

### 1. Build Test Image

```bash
# Local: Push to test branch
git checkout test
git merge main  # Sync latest changes
git push origin test
```

### 2. Pull and Run on Server

```bash
# On dure.co server
cd /opt/dure-mycart
./pull-test-image.sh
```

### 3. Verify

```bash
# Check container status
docker ps | grep dure-mycart-test

# View logs
docker logs -f dure-mycart-test

# Test HTTP endpoint
curl -v http://localhost:8080

# Test HTTPS endpoint
curl -v https://localhost:8443

# Test XMPP ALPN routing
openssl s_client -connect localhost:8443 -alpn xmpp-client -starttls xmpp
```

## Port Mapping

| Service | Production | Test | Conflict? |
|---------|-----------|------|-----------|
| HTTP | 80 | 8080 | ✅ No |
| HTTPS | 443 | 8443 | ✅ No |
| XMPP C2S | 5222 | 15222 | ✅ No |
| XMPP S2S | 5269 | 15269 | ✅ No |

Both containers can run simultaneously for A/B testing.

## Cleanup

```bash
# Stop and remove test container
docker stop dure-mycart-test
docker rm dure-mycart-test

# Remove test image
docker rmi ghcr.io/dure-one/dure-mycart-prosody:test
```

## Troubleshooting

### Image pull fails with 401 Unauthorized

**Cause:** GHCR authentication expired or missing.

**Fix:**
```bash
echo $GITHUB_PAT | docker login ghcr.io -u YOUR_USERNAME --password-stdin
```

### Port already in use

**Cause:** Another service using test ports.

**Fix:** Check what's using the port:
```bash
sudo lsof -i :8443
# or
docker ps
```

### Test branch out of sync with main

**Fix:**
```bash
git checkout test
git merge main
git push origin test
```

## Manual Build (Alternative)

If you prefer manual control instead of auto-trigger:

```bash
# Trigger workflow manually for specific commit
gh workflow run docker-publish.yml --ref test

# Or pull by specific SHA
docker pull ghcr.io/dure-one/dure-mycart-prosody:sha-abc1234
```
