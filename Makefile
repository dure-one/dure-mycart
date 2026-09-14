.PHONY: help setup reinstall deps-check dev
.PHONY: build build-sqlc build-both build-admin build-site build-all
.PHONY: test test-unit test-integration test-postgres test-all
.PHONY: test-queries-raw-sqlite test-queries-raw-postgres
.PHONY: test-queries-sqlc-sqlite test-queries-sqlc-postgres test-queries-all
.PHONY: install-sqlc sqlc-generate sqlc-verify sqlc
.PHONY: e2e-admin e2e-site e2e-all
.PHONY: migrate-up migrate-down
.PHONY: docker-build docker-up docker-down docker-logs docker-test-all

# Detect OS and conditionally enable race detector (not supported on OpenBSD)
UNAME_S != uname -s
.if ${UNAME_S} == "OpenBSD"
RACE_FLAG =
.else
RACE_FLAG = -race
.endif

# Default target
help:
	@echo "Available targets:"
	@echo ""
	@echo "Setup:"
	@echo "  setup                    - Install all prerequisites (npm, go) - RUN THIS FIRST"
	@echo "  reinstall                - Reinstall npm dependencies"
	@echo "  deps-check               - Verify all dependencies are installed"
	@echo ""
	@echo "Development:"
	@echo "  dev                      - Run development server (SQLite, hot reload)"
	@echo ""
	@echo "Backend Build:"
	@echo "  build                    - Build with raw SQL backend (default)"
	@echo "  build-sqlc               - Build with sqlc backend"
	@echo "  build-both               - Build both backends"
	@echo ""
	@echo "Frontend Build:"
	@echo "  build-admin              - Build admin panel"
	@echo "  build-site               - Build storefront"
	@echo "  build-all                - Build both frontends"
	@echo ""
	@echo "sqlc Code Generation:"
	@echo "  install-sqlc             - Install sqlc code generator"
	@echo "  sqlc-generate            - Generate sqlc code (with verification)"
	@echo "  sqlc-verify              - Verify generated code builds"
	@echo "  sqlc                     - Quick generate (no verification)"
	@echo ""
	@echo "Backend Tests:"
	@echo "  test                     - Run all Go tests (unit + integration with SQLite)"
	@echo "  test-unit                - Run only unit tests (fast)"
	@echo "  test-integration         - Run integration tests (SQLite)"
	@echo "  test-postgres            - Run integration tests (PostgreSQL)"
	@echo "  test-all                 - Run tests against both SQLite and PostgreSQL"
	@echo ""
	@echo "Backend Tests (Query Layer):"
	@echo "  test-queries-raw-sqlite  - Test raw SQL backend with SQLite"
	@echo "  test-queries-raw-postgres - Test raw SQL backend with PostgreSQL"
	@echo "  test-queries-sqlc-sqlite - Test sqlc backend with SQLite"
	@echo "  test-queries-sqlc-postgres - Test sqlc backend with PostgreSQL"
	@echo "  test-queries-all         - Run 4-mode query test matrix"
	@echo ""
	@echo "Frontend Tests:"
	@echo "  e2e-admin                - Run admin panel e2e tests (browser)"
	@echo "  e2e-site                 - Run storefront e2e tests (browser)"
	@echo "  e2e-all                  - Run all frontend e2e tests"
	@echo ""
	@echo "Database:"
	@echo "  migrate-up               - Run migrations (up)"
	@echo "  migrate-down             - Run migrations (down)"
	@echo ""
	@echo "Docker:"
	@echo "  docker-build             - Build Docker image"
	@echo "  docker-up                - Start services"
	@echo "  docker-down              - Stop services"
	@echo "  docker-logs              - Show container logs"
	@echo "  docker-test-all          - Run tests in Docker against both databases"

# Setup targets
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
	@echo "✅ Setup complete! You can now run 'make dev' or 'make e2e-all'"

reinstall:
	@echo "Reinstalling npm dependencies..."
	node ./scripts/postinstall-openbsd-natives.js
	cd web/admin && rm -rf package-lock.json node_modules/ && npm install --legacy-peer-deps
	cd web/site && rm -rf package-lock.json node_modules/ && npm install --legacy-peer-deps
	node ./scripts/postinstall-openbsd-natives.js
	git checkout web/admin/package.json web/site/package.json

deps-check:
	@echo "🔍 Checking dependencies..."
	@echo ""
	@echo -n "Node.js: "
	@command -v node >/dev/null 2>&1 && node --version || echo "❌ NOT FOUND"
	@echo -n "npm: "
	@command -v npm >/dev/null 2>&1 && npm --version || echo "❌ NOT FOUND"
	@echo -n "Go: "
	@command -v go >/dev/null 2>&1 && go version || echo "❌ NOT FOUND"
	@echo -n "patchright: "
	@[ -f "node_modules/.bin/patchright" ] && echo "✓ installed" || echo "❌ NOT FOUND (run 'make setup')"
	@echo -n "Go modules: "
	@go list -m >/dev/null 2>&1 && echo "✓ installed" || echo "❌ NOT FOUND (run 'make setup')"
	@echo -n "sqlc generated: "
	@[ -d "internal/queries_sqlc/sqlc" ] && echo "✓ generated" || echo "❌ NOT FOUND (run 'make sqlc-generate')"

# Development target
dev: setup
	@echo "Starting development server..."
	go run ./cmd serve --dev

# Backend build targets
build:
	@echo "Building with raw SQL backend..."
	@go build -o mycart ./cmd
	@echo "✓ Built mycart (raw SQL)"

build-sqlc: sqlc-generate
	@echo "Building with sqlc backend..."
	@go build -tags sqlc -o mycart-sqlc ./cmd
	@echo "✓ Built mycart-sqlc"

build-both: build build-sqlc
	@echo "✓ Built both backends"
	@ls -lh mycart mycart-sqlc

# Frontend build targets
build-admin:
	@echo "Building admin panel..."
	cd web/admin && bun install && bun run build

build-site:
	@echo "Building storefront..."
	cd web/site && bun install && bun run build

build-all: build-admin build-site

# sqlc code generation
install-sqlc:
	@command -v sqlc >/dev/null 2>&1 || { \
		echo "Installing sqlc..."; \
		go install github.com/sqlc-dev/sqlc/cmd/sqlc@latest; \
	}
	@echo "sqlc installed: $$(sqlc version)"

sqlc-generate: install-sqlc
	@echo "Generating sqlc code..."
	@sqlc generate
	@echo "✓ Generated internal/queries_sqlc/sqlc/postgres/*.go"
	@echo "✓ Generated internal/queries_sqlc/sqlc/sqlite/*.go"

sqlc-verify: sqlc-generate
	@echo "Verifying generated code builds..."
	@go build -tags sqlc ./internal/queries_sqlc/...
	@echo "✓ sqlc code verified"

# Quick sqlc generation without verification
sqlc:
	@echo "Generating sqlc code..."
	sqlc generate

# Backend test targets - General
test:
	@echo "Running tests with SQLite..."
	go test ./... -v -count=1 $(RACE_FLAG)

test-unit:
	@echo "Running unit tests..."
	go test ./... -short -v -count=1 $(RACE_FLAG)

test-integration:
	@echo "Running integration tests with SQLite..."
	go test ./... -v -count=1 $(RACE_FLAG)

test-postgres:
	@echo "Running integration tests with PostgreSQL..."
	@echo "Note: Requires TEST_POSTGRES_DSN environment variable (see .env.example)"
	@if [ -z "$$TEST_POSTGRES_DSN" ]; then \
		echo "ERROR: TEST_POSTGRES_DSN not set. Set it in your environment or .env file"; \
		exit 1; \
	fi
	TEST_DB_DRIVER=postgres \
	go test ./... -v -count=1 $(RACE_FLAG)

test-all:
	@echo "Running tests against SQLite..."
	go test ./... -v -count=1 $(RACE_FLAG)
	@echo ""
	@echo "Running tests against PostgreSQL..."
	@echo "Note: Requires TEST_POSTGRES_DSN environment variable (see .env.example)"
	@if [ -z "$$TEST_POSTGRES_DSN" ]; then \
		echo "ERROR: TEST_POSTGRES_DSN not set. Set it in your environment or .env file"; \
		exit 1; \
	fi
	TEST_DB_DRIVER=postgres \
	go test ./... -v -count=1 $(RACE_FLAG)

# Backend test targets - Query layer specific
test-queries-raw-sqlite:
	@echo "Testing raw SQL backend with SQLite..."
	@go test ./internal/queries/... -count=1 $(RACE_FLAG)

test-queries-raw-postgres:
	@echo "Testing raw SQL backend with PostgreSQL..."
	@if [ -z "$$TEST_POSTGRES_DSN" ]; then \
		echo "ERROR: TEST_POSTGRES_DSN not set. Set it in your environment or .env file"; \
		exit 1; \
	fi
	@TEST_DB_DRIVER=postgres \
	go test ./internal/queries/... -count=1 $(RACE_FLAG)

test-queries-sqlc-sqlite:
	@echo "Testing sqlc backend with SQLite..."
	@go test -tags sqlc ./internal/queries_sqlc/... -count=1 $(RACE_FLAG)

test-queries-sqlc-postgres:
	@echo "Testing sqlc backend with PostgreSQL..."
	@if [ -z "$$TEST_POSTGRES_DSN" ]; then \
		echo "ERROR: TEST_POSTGRES_DSN not set. Set it in your environment or .env file"; \
		exit 1; \
	fi
	@TEST_DB_DRIVER=postgres \
	go test -tags sqlc ./internal/queries_sqlc/... -count=1 $(RACE_FLAG)

# 4-mode query test matrix (raw+sqlc × SQLite+PostgreSQL)
test-queries-all: test-queries-raw-sqlite test-queries-raw-postgres \
                  test-queries-sqlc-sqlite test-queries-sqlc-postgres
	@echo "✓ All 4-mode query tests passed"

# Frontend test targets
e2e-admin:
	@echo "Running admin panel e2e tests..."
	cd web/admin && bun run test:browser

e2e-site:
	@echo "Running storefront e2e tests..."
	cd web/site && bun run test:browser

e2e-all: e2e-admin e2e-site

# Database migration targets
migrate-up:
	@echo "Running migrations..."
	go run ./cmd migrate up

migrate-down:
	@echo "Rolling back migrations..."
	go run ./cmd migrate down

# Docker targets
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
