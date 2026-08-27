#!/bin/busybox sh
set -e

echo "=== xmpp-proxy-stack initialization (mycart edition) ==="

# Check required environment variables
if [ -z "${XMPP_DOMAIN:-}" ]; then
    echo "ERROR: XMPP_DOMAIN not set" >&2
    exit 1
fi

if [ -z "${MYCART_DOMAIN:-}" ]; then
    echo "ERROR: MYCART_DOMAIN not set" >&2
    exit 1
fi

# Ensure XMPP_DOMAIN and MYCART_DOMAIN match (shared SSL cert)
if [ "${XMPP_DOMAIN}" != "${MYCART_DOMAIN}" ]; then
    echo "ERROR: XMPP_DOMAIN and MYCART_DOMAIN must match for shared SSL cert" >&2
    exit 1
fi

# Check volume permissions
echo "Checking volume permissions..."
for dir in /certs /logs /var/lib/fail2ban-rs /app/lc_base /app/lc_uploads /app/lc_digitals; do
    if ! touch "${dir}/.write-test" 2>/dev/null; then
        echo "ERROR: No write permission to ${dir}" >&2
        echo "Fix: sudo chown -R root:root /srv/data/{certs,logs,fail2ban,mycart}" >&2
        exit 1
    fi
    rm -f "${dir}/.write-test"
done

echo "✓ Volume permissions OK"

# Create symlink for mycart's autocert cache
# mycart writes to ./lc_certs (relative to /app), but we want it in /certs
# Remove any incorrect symlink first
if [ -L /app/certs ]; then
    rm /app/certs
fi

if [ ! -e /app/lc_certs ]; then
    ln -s /certs /app/lc_certs
    echo "✓ Created symlink: /app/lc_certs -> /certs"
fi

# mycart's autocert will handle SSL certificates automatically
# No need to generate self-signed certs or run acme.sh

# Process horust service configs and wrapper scripts with envsubst
echo "Processing service configs and wrappers..."
for file in /etc/horust/services/*.toml /usr/local/bin/mycart-wrapper.sh; do
    if [ -f "$file" ]; then
        envsubst < "$file" > "$file.tmp" && mv "$file.tmp" "$file"
        chmod +x "$file" 2>/dev/null || true
    fi
done
echo "✓ Service configs and wrappers processed"

echo "Starting services via Horust..."
exec /usr/local/bin/horust --services-path /etc/horust/services
