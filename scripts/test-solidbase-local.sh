#!/usr/bin/env bash
# Test Solidbase theme building locally

set -e

echo "=== Testing Solidbase Theme Build Locally ==="

# Read theme upstream from _config.yml
theme_upstream=$(grep -m1 'theme_upstream:' _config.yml | awk '{print $2}' | sed "s/'//g")/archive/refs/heads/main.tar.gz

echo "Theme upstream: $theme_upstream"

# Clean previous test
rm -rf .solidbase main.tar.gz

# Download theme
echo "Downloading theme..."
wget -q "$theme_upstream" -O main.tar.gz

if [ $? -ne 0 ]; then
    echo "Failed to download theme from $theme_upstream"
    exit 1
fi

# Extract theme
echo "Extracting theme..."
mkdir -p .solidbase
tar -xzf main.tar.gz --strip-components=1 -C .solidbase

# Build with theme
echo "Building documentation..."
cd .solidbase

# Run the build script
NPM_CONFIG_LEGACY_PEER_DEPS=true bash ghworkflow.sh --no-deploy --src-path ../

cd ..

echo ""
echo "=== Build Complete ==="
echo "Output should be in .solidbase/.output/public/"
ls -la .solidbase/.output/public/ 2>/dev/null || echo "No output directory created"
