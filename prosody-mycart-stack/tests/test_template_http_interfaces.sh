#!/usr/bin/env bash
# Test: Prosody template must include bridge network IP in http_interfaces
# Tests the template itself (not generated config)

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
TEMPLATE_FILE="$SCRIPT_DIR/../templates/prosody-proxy.cfg.lua.template"

echo "=== Testing Prosody Template HTTP Interfaces ==="

# Test 1: Template must exist
if [ ! -f "$TEMPLATE_FILE" ]; then
    echo "✗ FAIL: Template not found at $TEMPLATE_FILE"
    exit 1
fi

# Test 2: Template must include bridge network IP
if ! grep -q 'http_interfaces.*172\.19\.0\.2' "$TEMPLATE_FILE"; then
    echo "✗ FAIL: Template missing 172.19.0.2 in http_interfaces"
    echo ""
    echo "  Current setting:"
    grep -A1 'http_interfaces' "$TEMPLATE_FILE" | head -2
    echo ""
    echo "  Expected: http_interfaces = { \"127.0.0.1\", \"::1\", \"172.19.0.2\" }"
    exit 1
fi

# Test 3: Verify complete setting
if ! grep -Eq 'http_interfaces\s*=\s*\{[^}]*"127\.0\.0\.1"[^}]*"172\.19\.0\.2"[^}]*\}' "$TEMPLATE_FILE"; then
    echo "✗ FAIL: http_interfaces format incorrect in template"
    exit 1
fi

echo "✓ PASS: Template includes bridge network IP 172.19.0.2"
echo "  After render + container restart, prosody will accept connections from mycart proxy"
exit 0
