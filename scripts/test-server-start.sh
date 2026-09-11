#!/bin/sh
# Clean test environment before starting server
# This ensures the server opens a fresh database

echo "🧹 Cleaning test environment before server start..."
rm -rf lc_base lc_digitals lc_uploads
echo "✓ Environment cleaned"

echo "📁 Creating required directories..."
mkdir -p lc_base lc_digitals lc_uploads
echo "✓ Directories created"

echo "🗄️ Initializing SQLite database..."
touch lc_base/data.db
echo "✓ Database file created"

echo "📚 Generating Swagger API documentation..."
if ! command -v swag >/dev/null 2>&1; then
    echo "  Installing swag..."
    go install github.com/swaggo/swag/cmd/swag@latest
fi
$(go env GOPATH)/bin/swag init -g cmd/main.go --output docs/swagger --parseDependency --parseInternal >/dev/null 2>&1
echo "✓ Swagger docs generated"

echo "🚀 Starting server..."
exec go run ./cmd serve
