#!/bin/bash
set -e

echo "=== Prosody Stack Validation ==="

# Check 1: No SQL driver errors
echo "Checking for SQL driver errors..."
if docker compose logs prosody 2>&1 | grep -q 'Driver.*not found'; then
    echo "❌ FAIL: SQL driver error still present"
    exit 1
fi
echo "✓ No SQL driver errors"

# Check 2: No Lua API deprecation warnings
echo "Checking for Lua API warnings..."
WARN_COUNT=$(docker compose logs prosody 2>&1 | grep -c 'deprecated.*Lua API' || true)
if [ "$WARN_COUNT" -gt 0 ]; then
    echo "❌ FAIL: $WARN_COUNT Lua API warnings found"
    exit 1
fi
echo "✓ No Lua API warnings"

# Check 3: Prosody is healthy
echo "Checking Prosody health..."
if ! docker inspect prosody 2>/dev/null | grep -q '"Status".*"running"'; then
    echo "❌ FAIL: Prosody not running"
    exit 1
fi
echo "✓ Prosody is running"

# Check 4: Rendered config uses sandboxed API
echo "Checking rendered config..."
if [ ! -f ../generated/proxy.cfg.lua ]; then
    echo "❌ FAIL: Rendered config not found"
    exit 1
fi
LUA_COUNT=$(grep -c 'Lua.os.getenv' ../generated/proxy.cfg.lua || true)
if [ "$LUA_COUNT" -lt 40 ]; then
    echo "❌ FAIL: Config not using sandboxed Lua API (found $LUA_COUNT, expected >40)"
    exit 1
fi
echo "✓ Config uses sandboxed Lua API ($LUA_COUNT instances)"

# Check 5: Storage is file-based (not SQL with empty driver)
echo "Checking storage backend..."
if docker compose logs prosody 2>&1 | tail -100 | grep -q 'storage.*sql'; then
    echo "⚠️  WARNING: SQL storage may be configured"
else
    echo "✓ Using file-based storage"
fi

echo ""
echo "=== All checks passed! ==="
