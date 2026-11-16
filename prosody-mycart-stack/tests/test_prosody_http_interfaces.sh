#!/usr/bin/env bash
# Test: Prosody HTTP interfaces should include bridge network IP for mycart proxy access
# TDD test for WebSocket/BOSH connectivity issue fix

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
CONFIG_FILE="$PROJECT_ROOT/generated/proxy.cfg.lua"

echo "=== Testing Prosody HTTP Interface Configuration ==="

# Test 1: Config file must exist
if [ ! -f "$CONFIG_FILE" ]; then
    echo "✗ FAIL: Generated config not found at $CONFIG_FILE"
    echo "  Run: cd prosody-mycart-stack && docker-compose run --rm prosody-config-init"
    exit 1
fi

# Test 2: http_interfaces must include localhost (security: allow prosodyctl access)
if ! grep -q 'http_interfaces.*127\.0\.0\.1' "$CONFIG_FILE"; then
    echo "✗ FAIL: http_interfaces missing 127.0.0.1 (required for prosodyctl)"
    exit 1
fi

# Test 3: http_interfaces must include bridge network IP (required for mycart proxy)
if ! grep -q 'http_interfaces.*172\.19\.0\.2' "$CONFIG_FILE"; then
    echo "✗ FAIL: http_interfaces missing 172.19.0.2 (required for mycart proxy)"
    echo "  mycart cannot reach prosody:5280 when bound to localhost only"
    echo ""
    echo "  Current setting:"
    grep -A1 'http_interfaces' "$CONFIG_FILE" | head -2
    echo ""
    echo "  Expected: http_interfaces = { \"127.0.0.1\", \"::1\", \"172.19.0.2\" }"
    exit 1
fi

# Test 4: Verify full setting format
if ! grep -Eq 'http_interfaces\s*=\s*\{[^}]*"127\.0\.0\.1"[^}]*"172\.19\.0\.2"[^}]*\}' "$CONFIG_FILE"; then
    echo "✗ FAIL: http_interfaces format incorrect"
    echo ""
    echo "  Current setting:"
    grep -A1 'http_interfaces' "$CONFIG_FILE" | head -2
    echo ""
    echo "  Expected: http_interfaces = { \"127.0.0.1\", \"::1\", \"172.19.0.2\" }"
    exit 1
fi

echo "✓ PASS: http_interfaces includes both localhost and bridge network IP"
echo "  Prosody HTTP server will be accessible from:"
echo "    - 127.0.0.1:5280 (prosodyctl, internal tools)"
echo "    - 172.19.0.2:5280 (mycart proxy for WebSocket/BOSH)"
exit 0
