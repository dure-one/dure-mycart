#!/bin/sh
# Render xmpp-proxy.toml.template -> generated/xmpp-proxy.toml
# Substitutes ${XMPP_DOMAIN} from environment

set -e

echo "=== Rendering xmpp-proxy.toml ==="

if [ -z "$XMPP_DOMAIN" ]; then
    echo "ERROR: XMPP_DOMAIN environment variable not set" >&2
    exit 1
fi

# Create generated directory if it doesn't exist
mkdir -p generated

# Use sed to replace ${XMPP_DOMAIN} in template
sed "s/\${XMPP_DOMAIN}/$XMPP_DOMAIN/g" prosody-mycart-stack/templates/xmpp-proxy.toml.template > generated/xmpp-proxy.toml

echo "✓ xmpp-proxy.toml rendered with XMPP_DOMAIN=${XMPP_DOMAIN}"
cat generated/xmpp-proxy.toml
