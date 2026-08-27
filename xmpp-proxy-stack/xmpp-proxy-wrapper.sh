#!/bin/busybox sh
# Wait for SSL certificates before starting xmpp-proxy

echo "Waiting for SSL certificates..."

# Wait up to 5 minutes for certificates to be generated
TIMEOUT=300
ELAPSED=0

while [ $ELAPSED -lt $TIMEOUT ]; do
    if [ -f /certs/fullchain.pem ] && [ -f /certs/privkey.pem ]; then
        echo "✓ SSL certificates found"
        echo "Starting xmpp-proxy..."
        exec /usr/local/bin/xmpp-proxy --c2s 127.0.0.1:15222 --s2s 127.0.0.1:15269 --cert /certs/fullchain.pem --key /certs/privkey.pem
    fi

    echo "Waiting for certificates... (${ELAPSED}s/${TIMEOUT}s)"
    sleep 10
    ELAPSED=$((ELAPSED + 10))
done

echo "ERROR: Timeout waiting for SSL certificates" >&2
exit 1
