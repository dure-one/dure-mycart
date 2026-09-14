.PHONY: help build build-sqlc build-both test test-queries-all
.PHONY: install-sqlc sqlc-generate sqlc-verify
.PHONY: test-queries-raw-sqlite test-queries-raw-postgres
.PHONY: test-queries-sqlc-sqlite test-queries-sqlc-postgres

# Default target
help:
	@echo "Available targets:"
	@echo "  build                    - Build with raw SQL backend (default)"
	@echo "  build-sqlc               - Build with sqlc backend"
	@echo "  build-both               - Build both backends"
	@echo "  test                     - Run all tests"
	@echo "  test-queries-all         - Run 4-mode query test matrix"
	@echo "  install-sqlc             - Install sqlc code generator"
	@echo "  sqlc-generate            - Generate sqlc code"
	@echo "  sqlc-verify              - Verify generated code builds"

# Build targets
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

# sqlc installation
install-sqlc:
	@command -v sqlc >/dev/null 2>&1 || { \
		echo "Installing sqlc..."; \
		go install github.com/sqlc-dev/sqlc/cmd/sqlc@latest; \
	}
	@echo "sqlc installed: $$(sqlc version)"

# sqlc generation
sqlc-generate: install-sqlc
	@echo "Generating sqlc code..."
	@sqlc generate
	@echo "✓ Generated internal/queries_sqlc/sqlc/postgres/*.go"
	@echo "✓ Generated internal/queries_sqlc/sqlc/sqlite/*.go"

sqlc-verify: sqlc-generate
	@echo "Verifying generated code builds..."
	@go build -tags sqlc ./internal/queries_sqlc/...
	@echo "✓ sqlc code verified"

# Test targets - Raw SQL backend
test-queries-raw-sqlite:
	@echo "Testing raw SQL backend with SQLite..."
	@go test ./internal/queries/... -count=1 -race

test-queries-raw-postgres:
	@echo "Testing raw SQL backend with PostgreSQL..."
	@TEST_DB_DRIVER=postgres \
	TEST_POSTGRES_DSN='postgres://postgres:password@localhost:5433/postgres?sslmode=disable' \
	go test ./internal/queries/... -count=1 -race

# Test targets - sqlc backend
test-queries-sqlc-sqlite:
	@echo "Testing sqlc backend with SQLite..."
	@go test -tags sqlc ./internal/queries_sqlc/... -count=1 -race

test-queries-sqlc-postgres:
	@echo "Testing sqlc backend with PostgreSQL..."
	@TEST_DB_DRIVER=postgres \
	TEST_POSTGRES_DSN='postgres://postgres:password@localhost:5433/postgres?sslmode=disable' \
	go test -tags sqlc ./internal/queries_sqlc/... -count=1 -race

# 4-mode test matrix
test-queries-all: test-queries-raw-sqlite test-queries-raw-postgres \
                  test-queries-sqlc-sqlite test-queries-sqlc-postgres
	@echo "✓ All 4-mode query tests passed"

# General test target
test:
	@echo "Running all tests..."
	@go test ./... -count=1 -race
	@echo "✓ All tests passed"
