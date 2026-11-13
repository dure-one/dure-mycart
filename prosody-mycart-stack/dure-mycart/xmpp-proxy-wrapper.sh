#!/bin/busybox sh
# Wait for SSL certificates before starting xmpp-proxy
# Prevents crash-restart loop when certs not yet issued

CERT_DIR="/certs"
MAX_WAIT=300  # 5 minutes
INTERVAL=5

echo "[xmpp-proxy-wrapper] Waiting for SSL certificates in $CERT_DIR..."

elapsed=0
while [ $elapsed -lt $MAX_WAIT ]; do
    if [ -f "$CERT_DIR/privkey.pem" ] && [ -f "$CERT_DIR/fullchain.pem" ]; then
        echo "[xmpp-proxy-wrapper] ✓ Certificates found, starting xmpp-proxy"
        exec /usr/local/bin/xmpp-proxy /etc/xmpp-proxy/xmpp-proxy.toml
    fi

    echo "[xmpp-proxy-wrapper] Waiting for certificates... ($elapsed/${MAX_WAIT}s)"
    sleep $INTERVAL
    elapsed=$((elapsed + INTERVAL))
done

echo "[xmpp-proxy-wrapper] ✗ Timeout waiting for certificates after ${MAX_WAIT}s" >&2
echo "[xmpp-proxy-wrapper] Check mycart logs: /logs/mycart-stderr.log" >&2
exit 1
