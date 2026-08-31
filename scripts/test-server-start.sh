#!/bin/sh
# Clean test environment before starting server
# This ensures the server opens a fresh database

echo "🧹 Cleaning test environment before server start..."
rm -rf lc_base lc_digitals lc_uploads
echo "✓ Environment cleaned"

echo "📚 Generating Swagger documentation..."
SWAG_BIN="$(go env GOPATH)/bin/swag"

if [ ! -f "$SWAG_BIN" ]; then
  echo "⚠️  swag not found, installing..."
  go install github.com/swaggo/swag/cmd/swag@latest
fi

"$SWAG_BIN" init -g cmd/main.go --output docs/swagger --parseDependency --parseInternal
if [ $? -eq 0 ]; then
  echo "✓ Swagger docs generated"
else
  echo "❌ Failed to generate Swagger docs"
  exit 1
fi

echo "🚀 Starting server..."
exec go run ./cmd serve
