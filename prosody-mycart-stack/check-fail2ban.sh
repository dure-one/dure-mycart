#!/bin/bash
# Quick fail2ban-rs status check

echo "fail2ban-rs Status:"
docker compose exec dure-mycart /usr/local/bin/fail2ban-rs status 2>&1 || {
    echo
    echo "Status command failed. Checking why..."
    echo
    echo "Socket exists?"
    docker compose exec dure-mycart sh -c 'ls -la /var/run/fail2ban-rs/ 2>/dev/null || echo "Socket directory not found"'
    echo
    echo "Recent logs:"
    docker compose exec dure-mycart tail -20 /logs/fail2ban-rs-stdout.log 2>/dev/null
}
