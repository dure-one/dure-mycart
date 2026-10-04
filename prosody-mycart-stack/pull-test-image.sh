#!/bin/bash
# Test Docker image pull and run script for dure.co server
# Usage: ./pull-test-image.sh

set -e

IMAGE="ghcr.io/dure-one/prosody-mycart:test"
CONTAINER_NAME="dure-mycart-test"

# Colors
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

echo -e "${GREEN}==> Pulling test image: $IMAGE${NC}"
docker pull "$IMAGE"

echo -e "${GREEN}==> Stopping existing test container (if running)${NC}"
docker stop "$CONTAINER_NAME" 2>/dev/null || true
docker rm "$CONTAINER_NAME" 2>/dev/null || true

echo -e "${GREEN}==> Starting test container${NC}"
docker run -d \
  --name "$CONTAINER_NAME" \
  -p 8443:443 \
  -p 8080:80 \
  -p 15222:5222 \
  -p 15269:5269 \
  -e XMPP_C2S_TARGET=127.0.0.1:5222 \
  -e XMPP_S2S_TARGET=127.0.0.1:5269 \
  --restart unless-stopped \
  "$IMAGE"

echo -e "${GREEN}==> Test container started successfully${NC}"
echo ""
echo "Container ports mapping:"
echo "  HTTP:  8080 -> 80   (test: http://localhost:8080)"
echo "  HTTPS: 8443 -> 443  (test: https://localhost:8443)"
echo "  XMPP C2S: 15222 -> 5222"
echo "  XMPP S2S: 15269 -> 5269"
echo ""
echo "Commands:"
echo "  View logs:    docker logs -f $CONTAINER_NAME"
echo "  Stop:         docker stop $CONTAINER_NAME"
echo "  Remove:       docker rm $CONTAINER_NAME"
echo "  Test HTTP:    curl -v http://localhost:8080"
echo "  Test HTTPS:   curl -v https://localhost:8443"
echo "  Test ALPN:    openssl s_client -connect localhost:8443 -alpn xmpp-client"
