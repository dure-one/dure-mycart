.PHONY: help setup deps-check dev test test-unit test-integration test-postgres test-all
.PHONY: e2e-admin e2e-site e2e-all sqlc migrate-up migrate-down
.PHONY: build-admin build-site build-all docker-build docker-up docker-down docker-logs docker-test-all

help:
	@echo "Available targets:"
	@echo ""
	@echo "Setup:"
	@echo "  setup             - Install all prerequisites (npm, go, sqlc) - RUN THIS FIRST on new platforms"
	@echo "  deps-check        - Verify all dependencies are installed"
	@echo ""
	@echo "Development:"
	@echo "  dev               - Run development server (SQLite, hot reload)"
	@echo ""
	@echo "Backend Tests:"
	@echo "  test              - Run all Go tests (unit + integration with SQLite)"
	@echo "  test-unit         - Run only unit tests (fast)"
	@echo "  test-integration  - Run integration tests (SQLite)"
	@echo "  test-postgres     - Run integration tests (PostgreSQL/Supabase)"
	@echo "  test-all          - Run tests against both SQLite and PostgreSQL"
	@echo ""
	@echo "Frontend Tests:"
	@echo "  e2e-all           - Run all frontend e2e tests"
	@echo ""
	@echo "Build:"
	@echo "  build-admin       - Build admin panel"
	@echo "  build-site        - Build storefront"
	@echo "  build-all         - Build both frontends"
	@echo "  sqlc              - Generate sqlc code for both databases"
	@echo ""
	@echo "Database:"
	@echo "  migrate-up        - Run migrations (up)"
	@echo "  migrate-down      - Run migrations (down)"
	@echo ""
	@echo "Docker:"
	@echo "  docker-build      - Build Docker image"
	@echo "  docker-up         - Start services"
	@echo "  docker-down       - Stop services"
	@echo "  docker-logs       - Show container logs"
	@echo "  docker-test-all   - Run tests in Docker against both databases"

setup:
	@echo "🔧 Installing prerequisites..."
	@echo ""
	@echo "📦 Installing npm dependencies..."
	@if [ ! -d "node_modules" ] || [ ! -f "node_modules/.bin/patchright" ]; then \
		npm install --legacy-peer-deps; \
	else \
		echo "✓ npm dependencies already installed"; \
	fi
	@echo ""
	@echo "🐹 Installing Go dependencies..."
	@go mod download
	@go mod tidy
	@echo ""
	@echo "🗄️  Generating sqlc code..."
	@sqlc generate
	@echo ""
	@echo "✅ Setup complete! You can now run 'make dev' or 'make e2e-all'"

deps-check:
	@echo "🔍 Checking dependencies..."
	@echo ""
	@echo -n "Node.js: "
	@command -v node >/dev/null 2>&1 && node --version || echo "❌ NOT FOUND"
	@echo -n "npm: "
	@command -v npm >/dev/null 2>&1 && npm --version || echo "❌ NOT FOUND"
	@echo -n "Go: "
	@command -v go >/dev/null 2>&1 && go version || echo "❌ NOT FOUND"
	@echo -n "sqlc: "
	@command -v sqlc >/dev/null 2>&1 && sqlc version || echo "❌ NOT FOUND"
	@echo -n "patchright: "
	@[ -f "node_modules/.bin/patchright" ] && echo "✓ installed" || echo "❌ NOT FOUND (run 'make setup')"
	@echo -n "Go modules: "
	@go list -m >/dev/null 2>&1 && echo "✓ installed" || echo "❌ NOT FOUND (run 'make setup')"
	@echo -n "sqlc generated: "
	@[ -d "internal/store/db" ] && echo "✓ generated" || echo "❌ NOT FOUND (run 'make setup')"

dev: setup
	@echo "Starting development server..."
	@if [ -f .env ]; then \
		echo "Loading .env file..."; \
		export $$(grep -v '^#' .env | xargs) && go run ./cmd serve --dev; \
	else \
		echo "Warning: .env file not found, using default configuration"; \
		go run ./cmd serve --dev; \
	fi

test:
	@echo "Running tests with SQLite..."
	go test ./... -v -count=1

test-unit:
	@echo "Running unit tests..."
	go test ./... -short -v -count=1

test-integration:
	@echo "Running integration tests with SQLite..."
	TEST_DB_TYPE=sqlite go test ./internal/store/... -v -count=1

test-postgres:
	@echo "Running integration tests with PostgreSQL..."
	@if [ -f .env ]; then \
		export $$(grep -v '^#' .env | xargs) && TEST_DB_TYPE=postgres go test ./internal/store/... -v -count=1; \
	else \
		TEST_DB_TYPE=postgres go test ./internal/store/... -v -count=1; \
	fi

test-all:
	@echo "Running tests against SQLite..."
	TEST_DB_TYPE=sqlite go test ./internal/store/... -v -count=1
	@echo ""
	@echo "Running tests against PostgreSQL..."
	@if [ -f .env ]; then \
		export $$(grep -v '^#' .env | xargs) && TEST_DB_TYPE=postgres go test ./internal/store/... -v -count=1; \
	else \
		TEST_DB_TYPE=postgres go test ./internal/store/... -v -count=1; \
	fi

e2e-all: setup
	@echo "Running admin panel e2e tests..."
	npm run test:e2e

build-admin:
	@echo "Building admin panel..."
	cd web/admin && bun install && bun run build

build-site:
	@echo "Building storefront..."
	cd web/site && bun install && bun run build

build-all: build-admin build-site

sqlc:
	@echo "Generating sqlc code..."
	sqlc generate

migrate-up:
	@echo "Running migrations..."
	go run ./cmd migrate up

migrate-down:
	@echo "Rolling back migrations..."
	go run ./cmd migrate down

docker-build:
	@echo "Building Docker image..."
	docker-compose build

docker-up:
	@echo "Starting mycart..."
	docker-compose up -d

docker-down:
	@echo "Stopping mycart..."
	docker-compose down

docker-logs:
	@echo "Showing logs..."
	docker-compose logs -f

docker-test-all:
	@echo "Running tests with both databases in Docker..."
	docker-compose -f docker-compose.test.yml --profile test run test-all
