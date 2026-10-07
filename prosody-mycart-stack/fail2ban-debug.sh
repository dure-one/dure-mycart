#!/bin/bash
# Debug script for fail2ban-rs status (busybox container)

echo "=== fail2ban-rs Debug Info ==="
echo

echo "1. Process status:"
docker compose exec -T dure-mycart busybox sh -c 'ps | grep fail2ban' || echo "Not running"
echo

echo "2. Config file:"
docker compose exec -T dure-mycart busybox sh -c '[ -f /etc/fail2ban-rs/config.toml ] && echo "✓ Mounted" || echo "✗ Missing"'
echo

echo "3. Socket:"
docker compose exec -T dure-mycart busybox sh -c '[ -S /var/run/fail2ban-rs/fail2ban-rs.sock ] && ls -la /var/run/fail2ban-rs/ || echo "Socket not found"'
echo

echo "4. Recent stdout logs:"
docker compose exec -T dure-mycart busybox tail -30 /logs/fail2ban-rs-stdout.log 2>/dev/null || echo "No stdout logs"
echo

echo "5. Recent stderr logs:"
docker compose exec -T dure-mycart busybox tail -30 /logs/fail2ban-rs-stderr.log 2>/dev/null || echo "No stderr logs"
echo

echo "6. fail2ban-rs status:"
docker compose exec -T dure-mycart /usr/local/bin/fail2ban-rs status 2>&1
echo

echo "7. nftables rules (first 40 lines):"
docker compose exec -T dure-mycart /usr/sbin/nft list table inet fail2ban-rs 2>/dev/null | busybox head -40
echo

echo "8. Monitored log files:"
for log in /logs/prosody/prosody.log /logs/mycart-stderr.log /logs/xmpp-proxy-stderr.log; do
    echo "  $log:"
    docker compose exec -T dure-mycart busybox sh -c "[ -f '$log' ] && echo \"    ✓ Exists (\$(stat -c%s '$log') bytes)\" || echo '    ✗ Missing'"
done
