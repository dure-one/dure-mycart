#!/bin/bash
set -e

echo "=== Setting up /srv/data volume structure ==="

# Create all directories
sudo mkdir -p /srv/data/{certs,logs/prosody,fail2ban,mycart/{lc_base,lc_uploads,lc_digitals},prosody}

# Set ownership
echo "Setting ownership..."
sudo chown -R root:root /srv/data/{certs,logs,fail2ban,mycart}
sudo chown -R 999:999 /srv/data/prosody  # prosody user UID

echo "✓ Volume structure created:"
tree -L 3 /srv/data/ || ls -la /srv/data/

echo ""
echo "Volume setup complete. You can now run docker-compose up."
