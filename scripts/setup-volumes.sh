#!/bin/bash
set -e

echo "=== Setting up /srv/data volume structure ==="

# Prosody UID:GID from prosodyim/prosody:13.0 image
# Run: docker run --rm --entrypoint="" prosodyim/prosody:13.0 id prosody
PROSODY_UID=100
PROSODY_GID=102

# Helper function to run commands with privilege
run_privileged() {
    if command -v sudo &> /dev/null; then
        sudo "$@"
    elif command -v doas &> /dev/null; then
        doas "$@"
    else
        echo "⚠️  Neither sudo nor doas found, using Docker to run as root..."
        docker run --rm -v /srv/data:/srv/data alpine:latest "$@"
    fi
}

# Create all directories
echo "Creating directories..."
run_privileged mkdir -p /srv/data/{certs,logs/prosody,fail2ban,mycart/{lc_base,lc_uploads,lc_digitals},prosody}

# Set ownership
echo "Setting ownership..."
run_privileged chown -R root:root /srv/data/certs
run_privileged chown -R root:root /srv/data/fail2ban
run_privileged chown -R root:root /srv/data/mycart
run_privileged chown -R root:root /srv/data/logs
run_privileged chown -R ${PROSODY_UID}:${PROSODY_GID} /srv/data/logs/prosody
run_privileged chown -R ${PROSODY_UID}:${PROSODY_GID} /srv/data/prosody

echo "✓ Volume structure created:"
tree -L 3 /srv/data/ || ls -la /srv/data/

echo ""
echo "Volume setup complete. You can now run docker-compose up."
