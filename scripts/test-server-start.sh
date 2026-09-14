#!/bin/sh
# Clean test environment before starting server
# This ensures the server opens a fresh database

echo "🧹 Cleaning test environment before server start..."
rm -rf lc_base lc_digitals lc_uploads
echo "✓ Environment cleaned"

# The suite names its port with E2E_PORT so it can run beside a development
# server that already holds the default one; without it, the default stands.
if [ -n "$E2E_PORT" ]; then
  echo "🚀 Starting server on port ${E2E_PORT}..."
  exec go run ./cmd serve --http "0.0.0.0:${E2E_PORT}"
fi

echo "🚀 Starting server..."
exec go run ./cmd serve
