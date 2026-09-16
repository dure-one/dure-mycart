# Testing Guide

This document describes how to run tests for mycart, including SQLite and PostgreSQL testing with Docker.

## Quick Start

### Run All Tests (SQLite only)
```bash
make test
```

### Run Tests with Race Detector (Local Development)
```bash
# Run with race detection for finding concurrency issues
go test -race ./...

# Or with Docker PostgreSQL
docker compose -f docker-compose.test.yml --profile test run --rm test-postgres \
  gotestsum --format short-verbose -- -race ./...
```

**Note**: Race detector adds ~10x overhead and can amplify timing-related test failures. Use it during development to catch race conditions, but disable it for CI/CD for stable results.

### Run Tests with Docker PostgreSQL

**Option 1: Fully automated (recommended for CI)**
```bash
# Run all tests in Docker (SQLite + PostgreSQL)
./scripts/test-docker.sh all

# Or using make
make docker-test-all

# Or using docker compose directly
docker compose -f docker-compose.test.yml --profile test run --rm test-all
```

**Option 2: Local development with Docker PostgreSQL**
```bash
# 1. Start PostgreSQL in Docker
./scripts/test-docker.sh up

# 2. Copy environment config
cp .env.example .env

# 3. Run tests locally
make test-postgres
# or
make test-all  # Run both SQLite and PostgreSQL
```

**Option 3: Cleanup**
```bash
./scripts/test-docker.sh down
```

## Test Targets

### All Tests
```bash
make test           # SQLite tests
make test-all       # SQLite + PostgreSQL tests
make docker-test-all # Run everything in Docker
```

### Unit Tests Only
```bash
make test-unit      # Fast unit tests (skip integration)
```

### Integration Tests
```bash
make test-sqlite    # SQLite integration tests
make test-postgres  # PostgreSQL integration tests
```

### Query Layer Tests
```bash
make test-queries-raw-sqlite      # Raw SQL + SQLite
make test-queries-raw-postgres    # Raw SQL + PostgreSQL
make test-queries-sqlc-sqlite     # sqlc + SQLite
make test-queries-sqlc-postgres   # sqlc + PostgreSQL
make test-queries-all             # All 4 combinations
```

## PostgreSQL Test Modes

### Admin Mode (Default)
- Can create/drop test databases
- Tests run in parallel (faster)
- Requires superuser or `CREATEDB` privilege
- Set `TEST_POSTGRES_ADMIN=1`

### Table-Level Mode
- Creates tables in existing database
- Tests run sequentially (slower but safer)
- Works with restricted permissions (Supabase, managed databases)
- Set `TEST_POSTGRES_ADMIN=0`

## Environment Variables

Create `.env` file from `.env.example`:

```bash
# Docker PostgreSQL (local testing)
TEST_POSTGRES_DSN=postgres://postgres:testpassword@localhost:5433/postgres?sslmode=disable
TEST_POSTGRES_ADMIN=1

# Or for Supabase (table-level mode)
TEST_POSTGRES_DSN=postgresql://postgres.xxxxx:password@aws-0-us-east-1.pooler.supabase.com:5432/postgres
TEST_POSTGRES_ADMIN=0
```

## CI/CD Integration

### GitHub Actions Example

```yaml
name: Tests

on: [push, pull_request]

jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      
      - name: Run all tests
        run: |
          docker compose -f docker-compose.test.yml --profile test run --rm test-all
```

### GitLab CI Example

```yaml
test:
  image: docker:latest
  services:
    - docker:dind
  script:
    - docker compose -f docker-compose.test.yml --profile test run --rm test-all
```

## Docker Setup Details

### PostgreSQL Test Container
- **Image**: `postgres:17-alpine`
- **Port**: `5433` (avoids conflict with local PostgreSQL on 5432)
- **Storage**: tmpfs (in-memory, faster tests, data not persisted)
- **Credentials**: 
  - User: `postgres`
  - Password: `testpassword`
  - Database: `postgres`

### Test Runner Container
- **Image**: Built from `Dockerfile.test`
- **Base**: `golang:1.23-alpine`
- **Includes**: `gotestsum`, `make`, build tools
- **Mounts**: Source code from host
- **Cache**: Go modules cached in named volume

## Troubleshooting

### Port 5433 already in use
```bash
# Check what's using the port
lsof -i :5433

# Stop the test container
./scripts/test-docker.sh down
```

### PostgreSQL not ready
```bash
# Check container status
docker compose -f docker-compose.test.yml --profile test ps

# Check logs
docker compose -f docker-compose.test.yml --profile test logs postgres-test
```

### Tests fail with "connection refused"
```bash
# Ensure PostgreSQL is healthy
docker compose -f docker-compose.test.yml --profile test exec postgres-test pg_isready -U postgres

# Or restart
./scripts/test-docker.sh down
./scripts/test-docker.sh up
```

### Cleanup everything
```bash
# Remove containers and volumes
./scripts/test-docker.sh down

# Remove build cache
docker system prune -f
```

## Coverage

Generate coverage report:
```bash
go test -cover ./...

# HTML coverage report
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out -o coverage.html
```

## Frontend Tests

```bash
make e2e-admin    # Admin panel E2E tests
make e2e-site     # Storefront E2E tests
make e2e-all      # All frontend tests
```

## PostgreSQL Test Database Issues

### Fixed: Connection Termination During Parallel Tests

**Symptoms:**
- `FATAL: terminating connection due to administrator command (SQLSTATE 57P01)`
- `FATAL: database "testdb_tpl_*_inst_*" does not exist (SQLSTATE 3D000)`
- Tests fail intermittently when running with `-race` flag

**Root Cause:**
The `dropTemplates` function was being called by every test through `config(t)`. When tests run in parallel (default with `-race` flag), this created a race condition where:

1. Test A calls `dropTemplates` → executes `pg_terminate_backend` on template databases
2. Test B is actively using a template connection → gets terminated  
3. Test C tries to reconnect → database no longer exists

**Fix Applied (2026-09-17):**

1. **Use `sync.Once` for template cleanup** (`internal/testutil/pgtest/pgtest.go`):
   - Ensures `dropTemplates` runs only once per test suite, not once per test
   - Prevents race condition where parallel tests terminate each other's connections

2. **Increased PostgreSQL resource limits** (`docker-compose.test.yml`):
   - `max_connections=200` (up from default 100)
   - `shared_buffers=256MB`
   - `statement_timeout=30000` (30 seconds)
   - `idle_in_transaction_session_timeout=30000`
   - `shm_size=256mb`

### Debugging Failed Tests

When a test fails in admin mode, the database instance is preserved for inspection:

```bash
# The test output shows the connection string:
# testdbconf: postgres://pgtdbuser:pgtdbpass@postgres-test:5432/testdb_tpl_*_inst_*?...

# Connect to inspect the state
docker exec -it mycart-postgres-test psql -U pgtdbuser -d <database_name>
```

### Common Test Warnings (Safe to Ignore)

1. **"cannot drop a template database"**: Normal cleanup noise when dropTemplates tries to remove active templates
2. **"dropTemplates: failed to drop"**: Expected when templates are in use, they'll be cleaned up later

### Race Detector Usage

**Default: Disabled** (for stable CI/CD)
```bash
# Standard test run (fast, stable)
./scripts/test-docker.sh
```

**Manual: Enabled** (for development)
```bash
# Run with race detection to find concurrency bugs
docker compose -f docker-compose.test.yml --profile test run --rm test-postgres \
  gotestsum --format short-verbose -- -race ./...
```

**Trade-offs**:
- ✅ **Without `-race`**: Fast, stable, good for CI/CD
- ⚠️ **With `-race`**: Slower (~10x overhead), may expose timing-related test flakiness, but catches real race conditions

**When to use `-race`**:
- Adding new concurrent code
- Debugging suspected race conditions  
- Before major releases
- When investigating data races

**When NOT to use `-race`**:
- Regular CI/CD runs
- Quick local testing
- When tests are timing-sensitive

### Test Coverage Requirements

**Minimum: 80%**

```bash
# Check coverage
go test -cover ./...

# Generate HTML report
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out -o coverage.html
open coverage.html
```
