# Prosody Stack Fix Deployment Guide

## What Was Fixed

1. **Docker Compose variable warnings** - Escaped `$username` and `$host` to prevent Docker from treating them as env vars
2. **SQL driver error** - Already fixed in commits c262b4d0 + 1392f1a6 (needs deployment)
3. **Lua API warnings** - Already fixed in commit 1392f1a6 (needs deployment)

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

### Step 2: Check Current State

```bash
cd /srv/dure-mycart/prosody-mycart-stack

# Check if .env exists
ls -la .env

# If .env doesn't exist, create it
if [ ! -f .env ]; then
    cp .env.example .env
    echo "⚠️  Edit .env to set XMPP_DOMAIN and MYCART_DOMAIN before proceeding!"
    exit 1
fi

# Check SQL driver setting
echo "Current SQL driver setting:"
grep PROSODY_SQL .env | grep -v '^#'
```

### Step 3: Fix .env File (if needed)

If you see `PROSODY_SQL_DRIVER=` (empty string), fix it:

```bash
# Remove or comment out the empty PROSODY_SQL_DRIVER line
sed -i.bak '/^PROSODY_SQL_DRIVER=$/d' /srv/dure-mycart/prosody-mycart-stack/.env

# Verify it's gone
grep PROSODY_SQL_DRIVER /srv/dure-mycart/prosody-mycart-stack/.env
# Should show only commented examples or nothing
```

### Step 4: Pull Latest Code

```bash
cd /srv/dure-mycart

# Check for local changes
git status

# Stash if needed
git stash

# Pull latest fixes (commits c262b4d0, 1392f1a6, feb8d64f)
git pull origin main

# Show what changed
git log --oneline -3
```

Expected commits:
- `feb8d64f` - fix: escape Lua variables in Docker Compose
- `1392f1a6` - fix: use sandboxed Lua API
- `c262b4d0` - fix: handle empty PROSODY_SQL_DRIVER

### Step 5: Deploy Updated Stack

```bash
cd /srv/dure-mycart/prosody-mycart-stack

# Stop containers
docker compose down

# Re-render config and start
docker compose up -d

# Watch logs for errors
docker compose logs -f --tail=50
```

Press `Ctrl+C` to stop watching logs when Prosody is healthy.

### Step 6: Validate Deployment

```bash
cd /srv/dure-mycart/prosody-mycart-stack

# Run validation script
bash validate-deployment.sh
```

Expected output:
```
=== Prosody Stack Validation ===
Checking for SQL driver errors...
✓ No SQL driver errors
Checking for Lua API warnings...
✓ No Lua API warnings
Checking Prosody health...
✓ Prosody is running
Checking rendered config...
✓ Config uses sandboxed Lua API (48 instances)
Checking storage backend...
✓ Using file-based storage

=== All checks passed! ===
```

### Step 7: Final Verification

```bash
# Check no warnings on startup
docker compose logs prosody 2>&1 | grep -i warn | grep -v 'WARN\[0000\]' | head -20

# Should see NO warnings like:
# - "Driver '' not found"
# - "deprecated.*Lua API"
# - "username variable is not set"
# - "host variable is not set"

# Check containers are healthy
docker compose ps
```

All containers should show `running` or `healthy`.

## Troubleshooting

### If SQL driver error persists:

```bash
# Check rendered config
grep -n 'sql_driver\|storage.*=' /srv/dure-mycart/generated/proxy.cfg.lua | head -20

# Should show:
# local sql_driver = Lua.os.getenv("PROSODY_SQL_DRIVER") or ""
# if sql_driver ~= "" then
#     storage = "sql"
# else
#     storage = Lua.os.getenv("PROSODY_STORAGE") or "internal"
```

### If Lua API warnings persist:

```bash
# Count sandboxed API usage
grep -c 'Lua\.os\.getenv\|Lua\.table\|Lua\.tonumber' /srv/dure-mycart/generated/proxy.cfg.lua

# Should be > 40
```

### If config re-render fails:

```bash
# Check template exists
ls -la /srv/dure-mycart/prosody-mycart-stack/templates/prosody-proxy.cfg.lua.template

# Check .env has required vars
grep -E '^XMPP_DOMAIN|^MYCART_DOMAIN' /srv/dure-mycart/prosody-mycart-stack/.env
```

## Success Criteria

- ✅ No "username/host variable not set" warnings
- ✅ No "Driver '' not found" errors
- ✅ No "deprecated Lua API" warnings
- ✅ Prosody container healthy
- ✅ `validate-deployment.sh` passes

## Next Steps After Success

1. Test XMPP connection: `prosodyctl check`
2. Create admin user: `docker compose exec prosody prosodyctl adduser admin@dure.co`
3. Access web admin: `https://dure.co/prosody/admin/`

---

**Need help?** Run `docker compose logs prosody` and share the output.
