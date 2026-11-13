#!/bin/bash
# Check actual service logs (not Docker console logs)
# Logs are in /srv/data/logs/ on host, /logs/ in container

set -e

LOG_DIR="${DATA_DIR:-/srv/data}/logs"

echo "=== Checking service logs in $LOG_DIR ==="
echo ""

# Check mycart logs
if [ -f "$LOG_DIR/mycart-stdout.log" ]; then
    echo "=== mycart STDOUT (last 50 lines) ==="
    tail -50 "$LOG_DIR/mycart-stdout.log"
    echo ""
else
    echo "⚠️  mycart-stdout.log not found (service may not have started)"
    echo ""
fi

if [ -f "$LOG_DIR/mycart-stderr.log" ]; then
    echo "=== mycart STDERR (last 50 lines) ==="
    tail -50 "$LOG_DIR/mycart-stderr.log"
    echo ""
else
    echo "⚠️  mycart-stderr.log not found"
    echo ""
fi

# Check xmpp-proxy logs
if [ -f "$LOG_DIR/xmpp-proxy-stdout.log" ]; then
    echo "=== xmpp-proxy STDOUT (last 50 lines) ==="
    tail -50 "$LOG_DIR/xmpp-proxy-stdout.log"
    echo ""
else
    echo "⚠️  xmpp-proxy-stdout.log not found (may be waiting for certs)"
    echo ""
fi

if [ -f "$LOG_DIR/xmpp-proxy-stderr.log" ]; then
    echo "=== xmpp-proxy STDERR (last 50 lines) ==="
    tail -50 "$LOG_DIR/xmpp-proxy-stderr.log"
    echo ""
else
    echo "⚠️  xmpp-proxy-stderr.log not found"
    echo ""
fi

# Check fail2ban-rs logs
if [ -f "$LOG_DIR/fail2ban-rs-stdout.log" ]; then
    echo "=== fail2ban-rs STDOUT (last 20 lines) ==="
    tail -20 "$LOG_DIR/fail2ban-rs-stdout.log"
    echo ""
fi

# Check for certificate files
echo "=== Certificate Status ==="
if [ -d "$LOG_DIR/../certs/" ]; then
    ls -lah "$LOG_DIR/../certs/" || echo "Cannot list certs directory"
else
    echo "⚠️  Certs directory not found at $LOG_DIR/../certs/"
fi
echo ""

# Check for ACME/certificate errors
echo "=== ACME/Certificate Errors (last 20 lines) ==="
if ls "$LOG_DIR"/mycart-*.log >/dev/null 2>&1; then
    grep -i "acme\|certificate\|tls\|error\|fatal" "$LOG_DIR"/mycart-*.log 2>/dev/null | tail -20 || echo "No ACME errors found"
else
    echo "No mycart log files to search"
fi
echo ""

# Summary
echo "=== Summary ==="
echo "Log directory: $LOG_DIR"
echo "Files:"
ls -lh "$LOG_DIR"/*.log 2>/dev/null || echo "No log files found"
