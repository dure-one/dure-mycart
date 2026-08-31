#!/bin/sh
#
# Fix Vite on OpenBSD by installing WebAssembly fallback for Rolldown
#
# Problem: Vite 8.x uses Rolldown bundler which requires platform-specific
#          native bindings. OpenBSD is not supported.
# Solution: Install @rolldown/binding-wasm32-wasi to use WebAssembly fallback
#
# Usage: ./scripts/fix-vite-openbsd.sh [admin|site]

set -e

WASI_PACKAGE="@rolldown/binding-wasm32-wasi"
WASI_VERSION="1.2.6"
TARGET="${1:-admin}"

if [ "$TARGET" != "admin" ] && [ "$TARGET" != "site" ]; then
    echo "Usage: $0 [admin|site]"
    exit 1
fi

WEB_DIR="web/$TARGET"

if [ ! -d "$WEB_DIR" ]; then
    echo "Error: Directory $WEB_DIR not found"
    exit 1
fi

echo "=== Vite OpenBSD Fix for $TARGET ==="
echo "Installing $WASI_PACKAGE@$WASI_VERSION..."
echo ""

cd "$WEB_DIR"

if npm ls "$WASI_PACKAGE" 2>/dev/null | grep -q "$WASI_PACKAGE@$WASI_VERSION"; then
    echo "✓ Already installed"
    exit 0
fi

npm install --save-optional --legacy-peer-deps "$WASI_PACKAGE@$WASI_VERSION"

echo ""
echo "✓ Fix applied successfully"
echo "Test with: cd $WEB_DIR && npx vite --version"
