#!/bin/bash
# Verification script for fail2ban-rs log integration
# Works with busybox-based containers

echo "=== fail2ban-rs Log Integration Verification ==="
echo

# Check if containers are running
echo "1. Checking container status..."
docker compose ps prosody dure-mycart || {
    echo "❌ Containers not running. Start with: docker compose up -d"
    exit 1
}
echo "✓ Containers running"
echo

# Check if fail2ban-rs config is mounted
echo "2. Verifying fail2ban-rs config mount..."
docker compose exec -T dure-mycart busybox sh -c '[ -f /etc/fail2ban-rs/config.toml ]' && echo "✓ Config mounted" || {
    echo "❌ Config not found at /etc/fail2ban-rs/config.toml"
    exit 1
}
echo

# Check if fail2ban-rs is running
echo "3. Checking fail2ban-rs process..."
docker compose exec -T dure-mycart busybox sh -c 'ps | grep -q "[f]ail2ban-rs"' && {
    pid=$(docker compose exec -T dure-mycart busybox sh -c 'ps | grep "[f]ail2ban-rs"' | busybox awk '{print $1}' | head -1)
    echo "✓ fail2ban-rs running (PID: ${pid})"
} || {
    echo "❌ fail2ban-rs not running"
    docker compose exec -T dure-mycart busybox cat /logs/fail2ban-rs-stderr.log 2>/dev/null || echo "No error logs found"
    exit 1
}
echo

# Check if log files exist and are accessible
echo "4. Verifying log file access..."
for log in /logs/prosody/prosody.log /logs/mycart-stderr.log /logs/xmpp-proxy-stderr.log; do
    if docker compose exec -T dure-mycart busybox sh -c "[ -f '$log' ]" 2>/dev/null; then
        size=$(docker compose exec -T dure-mycart busybox sh -c "stat -c%s '$log'" 2>/dev/null || echo "?")
        echo "✓ $log exists (${size} bytes)"
    else
        echo "⚠ $log not found (will be created when service logs)"
    fi
done
echo

# Check fail2ban-rs socket
echo "5. Checking fail2ban-rs socket..."
if docker compose exec -T dure-mycart busybox sh -c '[ -S /var/run/fail2ban-rs/fail2ban-rs.sock ]' 2>/dev/null; then
    echo "✓ Control socket exists"
else
    echo "⚠ Socket not found (checking if status command works anyway...)"
fi

# Try status command
echo
echo "6. Fetching fail2ban-rs status..."
docker compose exec -T dure-mycart /usr/local/bin/fail2ban-rs status || {
    echo "⚠ Status command failed - checking logs..."
    docker compose exec -T dure-mycart busybox tail -20 /logs/fail2ban-rs-stdout.log 2>/dev/null || true
}
echo

# Check nftables rules
echo "7. Checking nftables fail2ban rules..."
if docker compose exec -T dure-mycart /usr/sbin/nft list tables 2>/dev/null | grep -q fail2ban; then
    echo "✓ fail2ban-rs nftables table exists"
    echo
    echo "Active jails and sets:"
    docker compose exec -T dure-mycart /usr/sbin/nft list table inet fail2ban-rs 2>/dev/null | grep -E "(set|chain)" | head -20
else
    echo "⚠ No fail2ban-rs nftables rules (might still be initializing)"
fi
echo

echo "=== Verification Complete ==="
echo
echo "Next steps:"
echo "  1. Check logs: docker compose exec dure-mycart busybox tail -f /logs/fail2ban-rs-stdout.log"
echo "  2. Test auth failure: Try 5+ failed XMPP logins"
echo "  3. Watch bans: docker compose exec dure-mycart /usr/sbin/nft list table inet fail2ban-rs"
echo "  4. Manual status: docker compose exec dure-mycart /usr/local/bin/fail2ban-rs status"
