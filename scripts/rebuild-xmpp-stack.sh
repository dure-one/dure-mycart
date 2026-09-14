#!/bin/bash
# Rebuild and restart xmpp-proxy-stack with fixes

set -e

echo "=== Rebuilding xmpp-proxy-stack with fixes ==="

# Stop current containers
echo "Stopping containers..."
docker-compose -f docker-compose.dev.yml down

# Rebuild with no cache to ensure fresh build
echo "Rebuilding xmpp-proxy-stack (no cache)..."
docker-compose -f docker-compose.dev.yml build --no-cache xmpp-proxy-stack

# Start all services
echo "Starting services..."
docker-compose -f docker-compose.dev.yml up -d

# Show logs
echo ""
echo "=== Services starting... ==="
echo "Watch logs with: docker-compose -f docker-compose.dev.yml logs -f xmpp-proxy-stack"
echo ""

# Wait a bit and show status
sleep 5
docker-compose -f docker-compose.dev.yml ps
