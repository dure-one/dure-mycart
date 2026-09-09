# PostgreSQL + sqlc Integration Design

**Date:** 2026-09-09  
**Author:** Claude Sonnet 4.5  
**Status:** Design Approved

## Overview

Complete the migration from goose+SQLite to goose+sqlc supporting both SQLite and PostgreSQL databases. Add database selection to installation flows (web UI and CLI), test integration with Supabase, and ensure Docker deployment works seamlessly with environment variables.

## Goals

1. **Complete sqlc migration** - Migrate remaining query groups (Auth, Cart, Install) to sqlc function pointers
2. **Full integration testing** - Test all queries against both SQLite and PostgreSQL (Supabase)
3. **Installation UX** - Add database selection to web UI with connection builder and test button
4. **Docker-friendly** - CLI respects environment variables, distroless base image, simple compose files
5. **Zero breaking changes** - Maintain API compatibility, no performance regression

## Current State

**✅ Already Complete (from 2026-08-18 design):**
- sqlc configuration for both SQLite and PostgreSQL (`sqlc.yaml`)
- Generated sqlc code for both databases
- Function pointers abstraction layer (`internal/store/db/`)
- .env configuration system (`DB_TYPE`, `DATABASE_URL`, etc.)
- CLI installation with database flags
- **Partial store migrations**: Settings, Sessions, Pages, Products

**⚠️ Partial:**
- Some handlers still use old `queries.DB()` pattern
- Only 4 of 7 query groups migrated

**❌ Missing:**
- PostgreSQL testing with Supabase credentials
- Web UI database selection in installation page
- Remaining sqlc migrations (Auth, Cart, Install)

## Architecture

### High-Level Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                    HTTP Handlers                             │
│  (internal/handlers/private/*, internal/handlers/public/*)   │
└────────────────────┬────────────────────────────────────────┘
                     │ calls
                     ▼
┌─────────────────────────────────────────────────────────────┐
│                   Store Layer (Facade)                       │
│  internal/store/{auth,sessions,pages,products,carts,install} │
│  - Business logic                                            │
│  - Input validation                                          │
│  - Error wrapping                                            │
└────────────────────┬────────────────────────────────────────┘
                     │ calls
                     ▼
┌─────────────────────────────────────────────────────────────┐
│              Function Pointers (Abstraction)                 │
│         internal/store/db/{queries.go, init.go}              │
│  - Database-agnostic interface                               │
│  - Initialized once at startup                               │
└─────────────┬──────────────────────────┬────────────────────┘
              │                          │
    ┌─────────▼──────────┐    ┌─────────▼──────────┐
    │  SQLite sqlc       │    │ PostgreSQL sqlc    │
    │  (internal/store/  │    │ (internal/db/      │
    │   db/sqlite/)      │    │  postgres/)        │
    │  - Generated code  │    │ - Generated code   │
    └─────────┬──────────┘    └─────────┬──────────┘
              │                          │
              ▼                          ▼
         [SQLite DB]              [PostgreSQL/Supabase]
```

### Key Design Principles

1. **Function Pointers for Zero-Cost Abstraction**
   - No interface overhead at runtime
   - Initialized once at startup based on `DB_TYPE`
   - All handlers call same `store.*` methods regardless of database

2. **Unified Types**
   - `internal/store/db/types.go` defines database-agnostic types
   - Converters: `FromSQLite*()` and `FromPostgres*()` for type mapping
   - Handles differences (e.g., SQLite INT64 vs PostgreSQL INT32 for timestamps)

3. **Test Framework**
   - Switch databases via `TEST_DB_TYPE` environment variable
   - Same test code runs against both SQLite and PostgreSQL
   - Validates sqlc queries work identically on both databases

4. **Configuration Priority** (Docker-aware)
   ```
   CLI flags > Environment variables > .env file > Defaults
   ```

## Test Framework Design

### Test File Structure

```
internal/store/
├── db/
│   ├── integration_test.go         # Main test framework
│   ├── testhelpers.go              # Shared test utilities
│   └── testdata/
│       ├── fixtures.go             # Test data factories
│       └── cleanup.sql             # Cleanup queries
├── sessions_test.go                # Existing tests (keep)
├── pages_test.go                   # Existing tests (keep)
├── products_test.go                # Existing tests (keep)
├── auth_integration_test.go        # NEW - Auth layer tests
├── carts_integration_test.go       # NEW - Cart layer tests
└── install_integration_test.go     # NEW - Install layer tests
```

### Test Framework Core

**File:** `internal/store/db/integration_test.go`

```go
package db_test

import (
    "context"
    "database/sql"
    "os"
    "testing"
    
    "github.com/shurco/mycart/internal/store/db"
    "github.com/shurco/mycart/internal/goosemigration/database"
    "github.com/shurco/mycart/db/migrations"
)

// TestMain sets up and tears down test database
func TestMain(m *testing.M) {
    if err := setupTestEnvironment(); err != nil {
        panic(err)
    }
    code := m.Run()
    cleanupTestEnvironment()
    os.Exit(code)
}

// setupTestDB creates and migrates a test database
func setupTestDB(t *testing.T) (*sql.DB, string, func()) {
    dbType := getTestDBType()
    
    var sqlDB *sql.DB
    var cleanup func()
    var err error
    
    switch dbType {
    case "sqlite":
        sqlDB, cleanup = setupSQLiteTest(t)
    case "postgres":
        sqlDB, cleanup = setupPostgresTest(t)
    default:
        t.Fatalf("unsupported TEST_DB_TYPE: %s", dbType)
    }
    
    // Run migrations
    if err := database.RunMigrations(sqlDB, dbType, migrations.Embed()); err != nil {
        cleanup()
        t.Fatalf("failed to run migrations: %v", err)
    }
    
    // Initialize function pointers
    if err := db.Init(sqlDB, dbType); err != nil {
        cleanup()
        t.Fatalf("failed to initialize db: %v", err)
    }
    
    return sqlDB, dbType, cleanup
}

// setupSQLiteTest creates in-memory SQLite for tests
func setupSQLiteTest(t *testing.T) (*sql.DB, func()) {
    sqlDB, err := sql.Open("sqlite", ":memory:")
    if err != nil {
        t.Fatalf("failed to open sqlite: %v", err)
    }
    
    cleanup := func() {
        sqlDB.Close()
    }
    
    return sqlDB, cleanup
}

// setupPostgresTest connects to Supabase for tests
func setupPostgresTest(t *testing.T) (*sql.DB, func()) {
    connStr := os.Getenv("TEST_DATABASE_URL")
    if connStr == "" {
        connStr = "postgresql://postgres:enfpakdlzkxm!23@db.tybjgfktpgkvrjmzamhx.supabase.co:5432/postgres"
    }
    
    sqlDB, err := sql.Open("postgres", connStr)
    if err != nil {
        t.Fatalf("failed to connect to postgres: %v", err)
    }
    
    if err := sqlDB.Ping(); err != nil {
        sqlDB.Close()
        t.Fatalf("failed to ping postgres: %v", err)
    }
    
    cleanup := func() {
        cleanupTestData(t, sqlDB)
        sqlDB.Close()
    }
    
    return sqlDB, cleanup
}

// cleanupTestData removes test data from Supabase
func cleanupTestData(t *testing.T, sqlDB *sql.DB) {
    tables := []string{
        "carts", "cart_items", "digital_files", "digital_data",
        "product_images", "products", "pages", "sessions", 
        "settings", "subdomains",
    }
    
    ctx := context.Background()
    for _, table := range tables {
        _, err := sqlDB.ExecContext(ctx, 
            "DELETE FROM "+table+" WHERE id LIKE 'test_%'")
        if err != nil {
            t.Logf("cleanup warning: %s: %v", table, err)
        }
    }
}

// getTestDBType returns database type for tests
func getTestDBType() string {
    if dbType := os.Getenv("TEST_DB_TYPE"); dbType != "" {
        return dbType
    }
    return "sqlite" // default to SQLite for fast tests
}
```

### Running Tests

```bash
# Fast: SQLite in-memory (default)
make test

# Full: PostgreSQL (Supabase)
make test-postgres

# CI: Both databases
make test-all

# Docker tests
make docker-test-all
```

## Migration Strategy

### Migration Pattern

Each migration follows this pattern:

1. Add sqlc queries → `db/queries/{sqlite,postgres}/*.sql`
2. Generate code → `sqlc generate`
3. Add function pointers → `internal/store/db/queries.go`
4. Wire in init.go → `internal/store/db/init.go`
5. Update store layer → `internal/store/*.go`
6. Update handlers → `internal/handlers/*/*.go`
7. Write integration tests

### Auth Migration

**sqlc queries:** (`db/queries/sqlite/auth.sql` & `db/queries/postgres/auth.sql`)

```sql
-- name: GetUserByEmail :one
SELECT id, email, password, created_at, updated_at
FROM users
WHERE email = ? LIMIT 1;  -- SQLite uses ?

-- name: CreateUser :exec
INSERT INTO users (id, email, password, created_at, updated_at)
VALUES (?, ?, ?, ?, ?);

-- name: UpdateUserPassword :exec
UPDATE users SET password = ?, updated_at = ?
WHERE email = ?;
```

**Function pointers:** (`internal/store/db/queries.go`)

```go
// Auth operations
var (
    GetUserByEmailFunc     func(ctx context.Context, email string) (User, error)
    CreateUserFunc         func(ctx context.Context, arg CreateUserParams) error
    UpdateUserPasswordFunc func(ctx context.Context, arg UpdateUserPasswordParams) error
)
```

### Cart Migration

**sqlc queries:** (`db/queries/sqlite/carts.sql`)

```sql
-- name: CreateCart :exec
INSERT INTO carts (id, session_id, status, total, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?);

-- name: GetCartByID :one
SELECT id, session_id, status, total, created_at, updated_at
FROM carts WHERE id = ? LIMIT 1;

-- name: ListCartItems :many
SELECT id, cart_id, product_id, quantity, price, created_at
FROM cart_items WHERE cart_id = ?;

-- name: UpdateCartItem :exec
UPDATE cart_items SET quantity = ?, price = ?
WHERE id = ?;

-- name: DeleteCartItem :exec
DELETE FROM cart_items WHERE id = ?;
```

### Install Migration

**sqlc queries:** (`db/queries/sqlite/settings.sql` - reuse settings table)

```sql
-- name: IsInstalled :one
SELECT EXISTS(
    SELECT 1 FROM settings WHERE key = 'installed'
) AS installed;

-- name: MarkInstalled :exec
INSERT INTO settings (id, key, value)
VALUES (?, 'installed', 'true');
```

## Web UI Database Selection

### Installation Page Flow

```
Step 1: Admin Account
├─ Email
├─ Password
└─ Domain

Step 2: Database Configuration (NEW)
├─ Database Type: [SQLite] [PostgreSQL]
│
├─ IF SQLite:
│   └─ Path: ./lc_base/data.db (default)
│
└─ IF PostgreSQL:
    ├─ Connection String (OR)
    │   └─ postgresql://user:pass@host/db
    │
    └─ Manual Configuration
        ├─ Host, Port, Database, User, Password
        ├─ SSL Mode: [require] [disable]
        └─ [Test Connection] button

[Install] button
```

### Frontend Implementation

**File:** `web/admin/src/routes/install/+page.svelte`

Key features:
- Radio buttons for SQLite vs PostgreSQL
- Toggle between connection string and manual config
- Test connection button (calls `/api/install/test-connection`)
- Real-time validation
- Connection status indicators

### Backend API

**New endpoint:** `POST /api/install/test-connection`

**File:** `internal/handlers/private/install.go`

```go
func TestDatabaseConnection(c fiber.Ctx) error {
	request := new(models.DatabaseConfig)
	if err := c.Bind().Body(request); err != nil {
		return webutil.StatusBadRequest(c, err.Error())
	}

	cfg := &database.Config{Type: request.DBType}
	// Build PostgreSQL or SQLite config

	adapter, err := database.Connect(cfg)
	if err != nil {
		return webutil.StatusInternalServerError(c, "connection failed")
	}
	defer adapter.Close()

	if err := adapter.DB().Ping(); err != nil {
		return webutil.StatusInternalServerError(c, "connection failed")
	}

	return webutil.Response(c, fiber.StatusOK, "Connection successful", nil)
}
```

**Update Install endpoint** to save database config to .env file.

## Docker Integration

### Dockerfile (Distroless-based)

**File:** `Dockerfile`

Three-stage build:
1. **Frontend builder** (node:22-alpine) - Build admin + site with bun
2. **Backend builder** (golang:1.26-alpine) - Generate sqlc + build Go binary
3. **Runtime** (gcr.io/distroless/static-debian13:nonroot) - Ultra-secure deployment

Key features:
- CGO_ENABLED=0 for static binary
- ldflags="-w -s" for smaller binary
- Runs as nonroot user (UID 65532)
- Minimal attack surface

### docker-compose.yml (Production)

**File:** `docker-compose.yml`

```yaml
version: '3.8'

services:
  mycart:
    build: .
    ports:
      - "8080:8080"
    environment:
      # Option 1: SQLite (default)
      - DB_TYPE=sqlite
      - SQLITE_PATH=/data/mycart.db
      
      # Option 2: PostgreSQL (Supabase) - uncomment to use
      # - DB_TYPE=postgres
      # - DATABASE_URL=postgresql://postgres:password@db.supabase.co:5432/postgres
    volumes:
      - mycart_data:/data
    restart: unless-stopped

volumes:
  mycart_data:
```

**Simple, single-service configuration.** Users switch databases by changing environment variables.

### docker-compose.test.yml (Testing)

**File:** `docker-compose.test.yml`

Multiple test services:
- `test-sqlite` - Run tests with SQLite
- `test-postgres-supabase` - Run tests with Supabase
- `test-postgres-local` - Run tests with local PostgreSQL
- `test-all` - Run tests against both databases
- `postgres` - Local PostgreSQL for testing

Uses `profiles: [test]` to keep these services separate from production.

### Configuration Priority

```
1. CLI Flags (highest priority)
   ↓ if not set
2. Environment Variables (Docker, .env)
   ↓ if not set
3. Defaults (lowest priority)
```

**Docker-friendly:** CLI install command checks environment variables first, only uses flags if provided.

## Error Handling & Type Safety

### Database Type Differences

| Concern | SQLite | PostgreSQL | Solution |
|---------|--------|------------|----------|
| **Integer types** | INT64 | INT32/INT64 | Unified `int64` in Go, convert in adapters |
| **Boolean** | INTEGER (0/1) | BOOLEAN | sqlc handles automatically |
| **Timestamps** | INTEGER (Unix) | TIMESTAMP | Unified `time.Time`, sqlc converts |
| **NULL handling** | sql.NullInt64 | sql.NullInt32 | Unified types in `db/types.go` |
| **JSON** | TEXT | JSONB | Both use `string` in Go |

### Unified Type System

**File:** `internal/store/db/types.go`

Defines database-agnostic types:
- `Session` with `sql.NullInt64` for Expires (works for both)
- `Product` with `int64` for Amount (works for both)
- `User`, `Page`, `Setting` with consistent types

**File:** `internal/store/db/converters.go`

Type converters:
- `FromSQLite*()` - SQLite types → Unified types
- `FromPostgres*()` - PostgreSQL types → Unified types

Example:
```go
func FromPostgresSession(s postgres.Session) Session {
    return Session{
        Key:   s.Key,
        Value: s.Value,
        Expires: sql.NullInt64{
            Int64: int64(s.Expires.Int32), // INT32 → INT64
            Valid: s.Expires.Valid,
        },
    }
}
```

### Error Handling Strategy

1. **Connection errors** - Retry logic with exponential backoff
2. **Migration errors** - Clear error messages, rollback support
3. **Query errors** - Wrap with context, distinguish `sql.ErrNoRows`
4. **Transaction errors** - Automatic rollback on failure

## Build Tooling

### Makefile

**File:** `Makefile`

Commands:
- **Development:** `make dev` - Run dev server
- **Backend tests:** `make test`, `make test-postgres`, `make test-all`
- **Frontend tests:** `make e2e-admin`, `make e2e-site`, `make e2e-all`
- **Build:** `make build-all`, `make sqlc`
- **Database:** `make migrate-up`, `make migrate-down`
- **Docker:** `make docker-build`, `make docker-up`, `make docker-test-all`

## Implementation Timeline

### Week 1: Test Infrastructure & Migrations

**Days 1-2: Test Infrastructure**
- Create integration test framework
- Support SQLite/PostgreSQL switching
- Test helpers and cleanup utilities

**Days 3-4: Auth & Cart Migrations**
- Migrate Auth queries to sqlc
- Migrate Cart queries to sqlc
- Write integration tests

**Day 5: Install Migration**
- Migrate Install queries to sqlc
- Update all handlers
- Full test suite validation

### Week 2: Web UI & Docker

**Days 1-2: Web UI**
- Database selection form
- Connection builder
- Test connection endpoint
- Save to .env

**Days 3-4: Docker**
- Distroless Dockerfile
- docker-compose.yml (production)
- docker-compose.test.yml (testing)
- Docker documentation

**Day 5: Validation**
- Full test suite (both databases)
- Supabase connection validation
- Docker deployment testing
- Documentation updates

## Success Criteria

✅ All 7 store layers migrated to sqlc  
✅ Integration tests pass on both SQLite and PostgreSQL  
✅ Web UI allows database selection during installation  
✅ CLI respects environment variables (Docker-friendly)  
✅ Docker deployment works with both databases  
✅ No performance regression vs current implementation  
✅ Zero breaking changes to existing APIs  

## Risk Mitigation

| Risk | Mitigation |
|------|------------|
| **Type mismatches** | Unified type system + converters + integration tests |
| **Supabase connection issues** | Retry logic + clear errors + local PostgreSQL fallback |
| **Breaking handlers** | Incremental migration + comprehensive test coverage |
| **Docker permissions** | Distroless nonroot user + proper file ownership |
| **Migration failures** | Transaction-based migrations + rollback support |

## File Changes Summary

**New files:**
- `Makefile`
- `Dockerfile` (distroless)
- `docker-compose.yml`
- `docker-compose.test.yml`
- `.env.supabase`
- `internal/store/db/integration_test.go`
- `internal/store/db/testhelpers.go`
- `internal/store/db/converters.go`
- `internal/store/auth_integration_test.go`
- `internal/store/carts_integration_test.go`
- `internal/store/install_integration_test.go`

**Modified files:**
- `internal/store/db/init.go`
- `internal/store/db/queries.go`
- `internal/store/db/types.go`
- `internal/store/auth.go`
- `internal/store/carts.go`
- `internal/store/install.go`
- `internal/handlers/private/install.go`
- `internal/models/install.go`
- `web/admin/src/routes/install/+page.svelte`
- `db/queries/sqlite/*.sql`
- `db/queries/postgres/*.sql`

**Total estimated changes:** ~3,000-4,000 lines of code

## Supabase Connection Details

**Direct PostgreSQL connection:**
```
postgresql://postgres:enfpakdlzkxm!23@db.tybjgfktpgkvrjmzamhx.supabase.co:5432/postgres
```

**Environment variable:**
```bash
DATABASE_URL=postgresql://postgres:enfpakdlzkxm!23@db.tybjgfktpgkvrjmzamhx.supabase.co:5432/postgres
```

## References

- sqlc documentation: https://docs.sqlc.dev/
- Goose migrations: https://github.com/pressly/goose
- Distroless images: https://github.com/GoogleContainerTools/distroless
- Supabase PostgreSQL: https://supabase.com/docs/guides/database
- Prior design (2026-08-18): `2026-08-18-goose-sqlc-migration-design.md`
