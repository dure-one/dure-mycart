#!/bin/sh
# Verification script for Prosody security hardening (2026-10-07)
set -e

echo "=== Prosody Security Hardening Verification ==="
echo ""

# Color codes
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m' # No Color

check_pass() {
    echo "${GREEN}✓${NC} $1"
}

check_warn() {
    echo "${YELLOW}⚠${NC} $1"
}

check_fail() {
    echo "${RED}✗${NC} $1"
}

echo "1. Checking generated config exists..."
if [ -f "../generated/proxy.cfg.lua" ]; then
    check_pass "Config file exists"
else
    check_fail "Config file missing - run render-prosody-config.sh"
    exit 1
fi

echo ""
echo "2. Verifying HTTP interface restriction..."
if grep -q 'http_interfaces = { "127.0.0.1", "::1" }' ../generated/proxy.cfg.lua; then
    check_pass "HTTP admin restricted to localhost"
else
    check_fail "HTTP admin NOT restricted"
fi

echo ""
echo "3. Verifying security modules enabled..."
for module in ipcheck anti_spam limits; do
    if grep -q "\"$module\"" ../generated/proxy.cfg.lua; then
        check_pass "mod_$module enabled"
    else
        check_fail "mod_$module NOT enabled"
    fi
done

echo ""
echo "4. Verifying monitoring modules enabled..."
for module in log_slow_events server_status stanza_counter; do
    if grep -q "\"$module\"" ../generated/proxy.cfg.lua; then
        check_pass "mod_$module enabled"
    else
        check_fail "mod_$module NOT enabled"
    fi
done

echo ""
echo "5. Verifying pastebin module enabled..."
if grep -q '"pastebin"' ../generated/proxy.cfg.lua; then
    check_pass "mod_pastebin enabled"
    check_warn "mod_pastebin stores messages in PLAIN TEXT (not encrypted)"
else
    check_fail "mod_pastebin NOT enabled"
fi

echo ""
echo "6. Verifying mod_ipcheck configuration..."
if grep -q "ipcheck_mode = \"block\"" ../generated/proxy.cfg.lua; then
    check_pass "ipcheck set to BLOCK mode"
else
    check_warn "ipcheck not in BLOCK mode"
fi

if grep -q "xbl.spamhaus.org" ../generated/proxy.cfg.lua; then
    check_pass "RBL lists configured (Spamhaus, SORBS, SpamCop)"
else
    check_fail "RBL lists NOT configured"
fi

echo ""
echo "7. Verifying mod_limits configuration..."
if grep -q 'rate = "3kb/s"' ../generated/proxy.cfg.lua; then
    check_pass "C2S rate limit: 3KB/s"
else
    check_fail "C2S rate limit NOT configured"
fi

if grep -q 'rate = "10kb/s"' ../generated/proxy.cfg.lua; then
    check_pass "S2S rate limit: 10KB/s"
else
    check_fail "S2S rate limit NOT configured"
fi

echo ""
echo "8. Verifying fail2ban-rs S2S threshold..."
if grep -q "max_retry = 7" dure-mycart/fail2ban-rs-config.toml; then
    check_pass "S2S ban threshold: 7 retries"
else
    check_fail "S2S ban threshold NOT updated (should be 7)"
fi

echo ""
echo "9. Checking module count..."
module_count=$(grep -c '^    "' ../generated/proxy.cfg.lua || true)
echo "   Total modules enabled: $module_count"
if [ "$module_count" -ge 35 ]; then
    check_pass "Module count looks good (≥35)"
else
    check_warn "Module count seems low (<35)"
fi

echo ""
echo "=== Summary ==="
echo "Security hardening verification complete."
echo ""
echo "Next steps:"
echo "  1. docker compose down"
echo "  2. docker compose up -d"
echo "  3. docker compose ps  # Wait for 'healthy'"
echo "  4. docker compose exec prosody prosodyctl check config"
echo "  5. ./verify-fail2ban.sh"
echo ""
echo "Documentation: SECURITY-HARDENING-2026-10-07.md"
