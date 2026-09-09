# Remove Goosemigration Layer Design

**Date:** 2026-09-10  
**Author:** Claude Sonnet 4.5  
**Status:** Design - Pending Review

## Overview

Remove the `internal/goosemigration/*` directory and consolidate all database operations into `internal/store/db/`, eliminating duplicate abstraction layers and simplifying the architecture.

## Context

The codebase currently has partial migration from the old `goosemigration` wrapper layer to the new `store/db` function pointer pattern:
- ✅ `internal/store/auth.go` - Already uses `db.*Func` pattern
- ❌ `internal/store/carts.go` - Still uses `queries.DB().CartQueries.*`
- ❌ `internal/store/products.go` - Still uses `queries.DB().ProductQueries.*`
- ❌ `internal/store/pages.go` - Still uses `queries.DB().PageQueries.*`
- ❌ `internal/store/sessions.go` - Still uses `queries.DB().SettingQueries.*`
- ❌ `internal/store/install.go` - Still uses `queries.DB().InstallQueries.*`

The `internal/goosemigration/` directory (34 files) provides:
1. Database connection and migration logic (`database/` subdirectory)
2. Query wrapper structs that delegate to sqlc-generated code (`queries/` subdirectory)

We have duplicate abstraction:
- Database adapters (SQLiteAdapter, PostgresAdapter) in `goosemigration/database/`
- Function pointers in `store/db/queries.go`

Both provide database type abstraction - we only need one.

## Goals

1. **Complete migration** - All store files use `db.*Func` pattern
2. **Consolidate database logic** - Move connection/migration to `store/db/init.go`
3. **Remove duplicate abstraction** - Drop adapter pattern, keep function pointers
4. **Preserve functionality** - No behavior changes, only structural refactoring
5. **Maintain test coverage** - Migrate tests to store layer, maintain ≥80% coverage
6. **Update documentation** - Reflect new architecture

## Architecture

### Current 3-Layer Architecture

```
Handlers → Store → Goosemigration/Queries → Store/DB (sqlc)
                        ↓
                   Database adapters
```

**Problems:**
- Three layers for database operations (store, goosemigration, db)
- Duplicate abstraction (adapters + function pointers)
- Business logic scattered (some in store, some in goosemigration/queries)
- Mixed naming conventions (old wrapper methods vs new sqlc methods)

### Target 2-Layer Architecture

```
Handlers → Store → Store/DB (sqlc function pointers)
                        ↓
                   Direct sqlc calls
```

**Benefits:**
- Two clear layers (business logic in store, database in store/db)
- Single abstraction mechanism (function pointers)
- Business logic clearly lives in `store/`
- Database operations clearly live in `store/db/`
- Simpler mental model

## Design

### 1. Database Initialization Changes

**Consolidate into `internal/store/db/init.go`:**

Expand existing `init.go` to handle all database setup:

**Components to consolidate:**

1. **Configuration** (from `goosemigration/database/config.go`)
   - Load from environment: `DB_TYPE`, `DB_PATH`, `POSTGRES_*` variables
   - Default to SQLite if not configured
   - Validate configuration

2. **Connection** (from `goosemigration/database/connect.go`)
   - Connection with retry logic (3 attempts, exponential backoff: 1s, 2s, 4s)
   - Support SQLite (`modernc.org/sqlite`) and PostgreSQL (`lib/pq`)
   - Connection pooling:
     - SQLite: max_open_conns=1 (embedded)
     - PostgreSQL: max_open_conns=25, max_idle_conns=5

3. **Migrations** (from `goosemigration/database/migrate.go`)
   - Run goose migrations using `embed.FS`
   - Support up and down migrations
   - Version tracking via goose

4. **Health Check** (from `goosemigration/database/health.go`)
   - Ping database to verify connection
   - Return database type and status

**New package-level variables:**

```go
var (
    db       *sql.DB
    dbType   string  // "sqlite" or "postgres"
    
    // Function pointers (already exist)
    GetSettingByKeyFunc func(ctx context.Context, key string) (Setting, error)
    CreateSettingFunc   func(ctx context.Context, arg CreateSettingParams) (Setting, error)
    // ... rest of function pointers
)
```

**New public API:**

```go
// Init - single entry point for database setup
// Replaces queries.New() + db.Init() + store.InitStoreWithType()
func Init(migrationsFS embed.FS) error

// Close - cleanup database connection
func Close() error

// Health - health check
func Health() error

// Type - return "sqlite" or "postgres"
func Type() string

// DB - return *sql.DB for transactions in store layer
func DB() *sql.DB
```

**Init() implementation flow:**

```go
func Init(migrationsFS embed.FS) error {
    // 1. Load configuration from environment
    cfg := loadConfig()
    
    // 2. Connect to database with retry logic
    conn, err := connectWithRetry(cfg)
    if err != nil {
        return fmt.Errorf("database connection failed: %w", err)
    }
    db = conn
    dbType = cfg.Type
    
    // 3. Run migrations
    if err := runMigrations(db, dbType, migrationsFS); err != nil {
        db.Close()
        return fmt.Errorf("migration failed: %w", err)
    }
    
    // 4. Initialize function pointers
    if err := initFunctionPointers(db, dbType); err != nil {
        db.Close()
        return fmt.Errorf("function pointer init failed: %w", err)
    }
    
    return nil
}
```

**Drop adapter abstraction:**

We don't need the `Database` interface and separate `SQLiteAdapter`/`PostgresAdapter` classes since function pointers already provide database type abstraction.

---

### 2. Function Pointer Additions

**Add to `internal/store/db/queries.go`:**

**Cart operations:**

```go
// List operations
ListCartsFunc          func(ctx context.Context, limit, offset int32) ([]Cart, error)
CountCartsFunc         func(ctx context.Context) (int64, error)

// Payment settings
GetPaymentSettingsFunc func(ctx context.Context) ([]Setting, error)

// Cart item operations with products
ListCartItemsWithProductsFunc func(ctx context.Context, cartID string) ([]CartItemWithProduct, error)
```

**Product operations:**

```go
// List with pagination
ListProductsFunc       func(ctx context.Context, limit, offset int32) ([]Product, error)
CountProductsFunc      func(ctx context.Context) (int64, error)

// Bulk operations
ListProductsByIDsFunc  func(ctx context.Context, ids []string) ([]Product, error)
```

**Page operations:**

```go
CountPagesFunc         func(ctx context.Context) (int64, error)
```

**Installation check:**

```go
IsInstalledFunc        func(ctx context.Context) (bool, error)
```

**Pattern for adding:**

1. Declare `var` in `queries.go`
2. Assign in `initPostgres()` and `initSQLite()` helper functions called by `Init()`
3. Both postgres and sqlite implementations must exist in sqlc-generated code

**Example:**

```go
// In queries.go
var ListCartsFunc func(ctx context.Context, limit, offset int32) ([]Cart, error)

// In init.go - initPostgres()
ListCartsFunc = func(ctx context.Context, limit, offset int32) ([]Cart, error) {
    pgQueries := postgres.New(db)
    rows, err := pgQueries.ListCarts(ctx, postgres.ListCartsParams{
        Limit: limit,
        Offset: offset,
    })
    // Convert postgres.Cart to db.Cart
    return convertPostgresCarts(rows), err
}

// In init.go - initSQLite()
ListCartsFunc = func(ctx context.Context, limit, offset int32) ([]Cart, error) {
    sqliteQueries := sqlite.New(db)
    rows, err := sqliteQueries.ListCarts(ctx, sqlite.ListCartsParams{
        Limit: limit,
        Offset: offset,
    })
    // Convert sqlite.Cart to db.Cart
    return convertSQLiteCarts(rows), err
}
```

---

### 3. Store Layer Migration

**Migration pattern for each file:**

**Before (current `carts.go`):**

```go
package store

import (
    "context"
    "github.com/shurco/mycart/internal/goosemigration/queries"
    "github.com/shurco/mycart/internal/models"
)

func Carts(ctx context.Context, limit, offset int) ([]*models.Cart, int, error) {
    return queries.DB().CartQueries.Carts(ctx, limit, offset)
}
```

**After (new pattern):**

```go
package store

import (
    "context"
    "fmt"
    "github.com/shurco/mycart/internal/store/db"
    "github.com/shurco/mycart/internal/models"
)

func Carts(ctx context.Context, limit, offset int) ([]*models.Cart, int, error) {
    // Call sqlc function via function pointer
    carts, err := db.ListCartsFunc(ctx, int32(limit), int32(offset))
    if err != nil {
        return nil, 0, fmt.Errorf("list carts: %w", err)
    }
    
    // Get total count for pagination
    total, err := db.CountCartsFunc(ctx)
    if err != nil {
        return nil, 0, fmt.Errorf("count carts: %w", err)
    }
    
    // Convert db.Cart to models.Cart
    result := make([]*models.Cart, len(carts))
    for i, c := range carts {
        result[i] = convertCart(&c)
    }
    
    return result, int(total), nil
}

// convertCart converts db.Cart to models.Cart
func convertCart(c *db.Cart) *models.Cart {
    return &models.Cart{
        ID: c.ID,
        SessionID: c.SessionID.String,
        // ... rest of field mapping
    }
}
```

**Files to migrate (5 files):**

1. **`internal/store/carts.go`**
   - Methods: `Carts()`, `Cart()`, `AddCart()`, `UpdateCart()`, `PaymentList()`, `ValidateCartItems()`, `BuildCartItems()`
   - Business logic to inline: `BuildCartItems()`, `ValidateCartItems()`

2. **`internal/store/products.go`**
   - Methods: `Products()`, `Product()`, `AddProduct()`, `UpdateProduct()`, `DeleteProduct()`, etc.
   - May need bulk operations for cart validation

3. **`internal/store/pages.go`**
   - Methods: `Pages()`, `Page()`, `AddPage()`, `UpdatePage()`, `DeletePage()`
   - Simpler - mostly CRUD

4. **`internal/store/sessions.go`**
   - Methods: `GetSession()`, `AddSession()`, `UpdateSession()`, `DeleteSession()`
   - May already be migrated (uses SettingQueries in old code)

5. **`internal/store/install.go`**
   - Methods: `IsInstalled()`, installation check logic

**Business logic to inline:**

- `BuildCartItems()` - Pure Go function, stays in `store/carts.go`
- `ValidateCartItems()` - Validation logic, stays in `store/carts.go`, calls `db.*Func` as needed
- Type conversions between `db.*` types and `models.*` types

**Import changes:**

- Remove: `"github.com/shurco/mycart/internal/goosemigration/queries"`
- Add: `"github.com/shurco/mycart/internal/store/db"`

**Error handling:**

- Wrap errors with context: `fmt.Errorf("operation context: %w", err)`
- Preserve sentinel errors (e.g., `sql.ErrNoRows` → `errors.ErrNotFound`)

---

### 4. Application Initialization Changes

**Current initialization in `internal/app.go`:**

```go
// Line 49-61
if err := queries.New(migrations.Embed()); err != nil {
    log.Err(err).Send()
    return err
}

// Initialize function pointers for zero-overhead database abstraction
if err := db.Init(queries.Adapter().DB(), queries.DBType()); err != nil {
    log.Err(err).Msg("failed to initialize database function pointers")
    return err
}

// Initialize store package with database connection and type for transactions
store.InitStoreWithType(queries.Adapter().DB(), queries.DBType())
```

**New initialization (simplified):**

```go
// Single Init call handles everything
if err := db.Init(migrations.Embed()); err != nil {
    log.Err(err).Msg("failed to initialize database")
    return err
}

// Initialize store with database for transactions
store.InitStoreWithType(db.DB(), db.Type())
```

**Import changes in `app.go`:**

- Remove: `"github.com/shurco/mycart/internal/goosemigration/queries"`
- Keep: `"github.com/shurco/mycart/internal/store/db"`

**Cleanup in `app.go`:**

- Change `queries.Close()` → `db.Close()` in shutdown logic

---

### 5. Test Migration Strategy

**Test files to migrate:**

| Goosemigration Test | Target Store Test | Status |
|---------------------|-------------------|--------|
| `queries/auth_test.go` | `store/auth_integration_test.go` | Exists - merge |
| `queries/cart_test.go` | `store/cart_integration_test.go` | Exists - merge |
| `queries/install_test.go` | `store/install_test.go` | Create new |
| `queries/pages_test.go` | `store/pages_test.go` | Exists - merge |
| `queries/products_test.go` | `store/products_test.go` | Exists - merge |
| `queries/session_test.go` | `store/sessions_test.go` | Exists - merge |
| `database/*_test.go` | `store/db/init_test.go` | Create new |

**Test migration pattern:**

**Before (goosemigration test):**

```go
package queries

func TestCartQueries_Carts(t *testing.T) {
    db := setupTestDB(t)
    NewFromDB(db)
    
    carts, total, err := DB().CartQueries.Carts(context.Background(), 10, 0)
    require.NoError(t, err)
    assert.Equal(t, 0, total)
    assert.Empty(t, carts)
}
```

**After (store test):**

```go
package store

func TestCarts(t *testing.T) {
    ctx := context.Background()
    db := testutil.NewTestDB(t)
    InitStoreWithType(db, "sqlite")
    
    carts, total, err := Carts(ctx, 10, 0)
    require.NoError(t, err)
    assert.Equal(t, 0, total)
    assert.Empty(t, carts)
}
```

**Database init tests:**

Create `internal/store/db/init_test.go` to test:
- Database connection (SQLite and PostgreSQL)
- Connection retry logic
- Migration execution
- Function pointer initialization
- Health check

**Testing checklist:**

- [ ] All test cases from goosemigration preserved
- [ ] Edge cases covered (empty results, errors, pagination, nil values)
- [ ] Run with `-race` flag
- [ ] Verify coverage stays ≥80%
- [ ] Test both SQLite and PostgreSQL paths (where applicable)

---

### 6. Documentation Updates

**Files requiring updates:**

**1. `internal/AGENTS.md`**

```diff
@@ -14,7 +14,7 @@
 | `middleware/` | Fiber middlewares (JWT auth, CORS, logging). |
 | `models/` | Plain data structs shared across layers. |
 | `store/` | Business logic facade - handlers call this layer. |
-| `goosemigration/queries/` | Database query layer (goose migration infrastructure + query wrappers). |
+| `store/db/` | Database layer: sqlc-generated queries, function pointer abstraction, migrations. |
 | `routes/` | Router wiring + SPA fallback. |
 | `mailer/` | SMTP + templated email rendering. |
 | `webhook/` | Outbound webhook dispatch. |
@@ -23,10 +23,10 @@
 ## Rules
 
 1. **Handlers are thin.** They parse input, call `store` methods, format the
-   response. Business logic belongs in `store/` which delegates to `goosemigration/queries/`.
+   response. Business logic belongs in `store/` which calls `store/db/` function pointers.
 2. **No direct database access in handlers.** Always use `internal/store` facade,
-   never call `goosemigration/queries` or database directly from handlers.
+   never call `store/db` directly from handlers - always use the `store/` facade.
 3. **No `http.DefaultClient`.** Webhooks use `webhook.sharedClient` which
    is built from `pkg/httpclient`.
 4. **Pagination** uses `webutil.ParsePagination`. Clamp is `[1, 100]`.
@@ -34,7 +34,7 @@
    setting group is a one-line change there — do not bring back the
    per-switch branches.
-6. **Error taxonomy.** Return sentinel errors from `goosemigration/queries/` (e.g.
-   `queries.ErrAlreadyInstalled`), translate to HTTP codes in handlers
+6. **Error taxonomy.** Return sentinel errors from `store/` methods (e.g.
+   `store.ErrAlreadyInstalled`), translate to HTTP codes in handlers
    with `errors.Is`.
 7. **Sessions table** is idempotent (`INSERT OR REPLACE`) — callers may
    refresh the same key without a prior delete.
```

**2. `AGENTS.md` (root level)**

Update repo layout table:

```diff
@@ -11,7 +11,7 @@
 | `internal/` | Private application code (HTTP handlers, store layer, DB queries, middleware, mailer, webhooks). |
 | `internal/store/` | Business logic facade layer - handlers call this, delegates to goosemigration queries. |
-| `internal/goosemigration/` | Database migration infrastructure and query wrappers (goose + internal queries). |
+| `internal/store/db/` | Database layer: connection, migrations, sqlc-generated type-safe queries. |
 | `pkg/` | Reusable packages that could in theory live in their own repo (`litepay`, `jwtutil`, `httpclient`, `webutil`, …). |
```

**3. `docs/database-development.md`** (if exists)

Search for any references to `goosemigration` and update to reference `store/db` or new architecture.

**4. Code comments**

- Remove comments referencing `goosemigration`
- Update `internal/app.go` initialization comments
- Update store method comments to mention they call `db.*Func` pointers

**5. Optional: Migration guide**

Create `docs/architecture/goosemigration-removal.md`:

```markdown
# Goosemigration Layer Removal

**Date:** 2026-09-10  
**Status:** Completed

## What Changed

Removed the `internal/goosemigration/*` directory and consolidated database operations into `internal/store/db/`.

## Old Architecture

Handlers → Store → Goosemigration/Queries → Store/DB (sqlc)

## New Architecture

Handlers → Store → Store/DB (sqlc function pointers)

## Migration Guide

If you're adding new database operations:

1. Add sqlc query to `db/queries/*.sql`
2. Run `sqlc generate`
3. Add function pointer to `internal/store/db/queries.go`
4. Initialize in `internal/store/db/init.go` (both postgres and sqlite)
5. Call from `internal/store/*.go` via `db.*Func()`

## Why We Did This

- Eliminated duplicate abstraction (adapters + function pointers)
- Simplified architecture (2 layers instead of 3)
- Clearer separation (business logic in store, database in store/db)
- Easier to understand and maintain
```

---

## Implementation Strategy

**Approach: Big Bang Migration** (all changes in one coordinated PR)

**Steps:**

1. **Add missing function pointers**
   - Add all cart, product, page, install function pointers to `queries.go`
   - Implement in `init.go` for both postgres and sqlite

2. **Consolidate database init**
   - Expand `store/db/init.go` with connection, migration, health logic
   - Add `Init()`, `Close()`, `Health()`, `Type()`, `DB()` functions

3. **Migrate store files**
   - Update all 5 store files to use `db.*Func` pattern
   - Inline business logic (BuildCartItems, ValidateCartItems)
   - Add type conversion helpers

4. **Update app.go**
   - Simplify initialization to single `db.Init()` call
   - Update imports and cleanup calls

5. **Migrate tests**
   - Move test cases from goosemigration to store
   - Create `store/db/init_test.go` for database tests
   - Verify coverage ≥80%

6. **Remove goosemigration**
   - Delete `internal/goosemigration/` directory (34 files)
   - Verify no remaining imports

7. **Update documentation**
   - Update AGENTS.md files
   - Update code comments
   - Optionally create migration guide

**Single PR benefits:**
- Clean, complete change
- No intermediate state with mixed patterns
- Easier to review as one cohesive change

**Risk mitigation:**
- Comprehensive test coverage before/after
- Run full test suite with `-race` flag
- Manual testing of critical paths (cart, checkout, admin)

---

## Risks & Mitigation

| Risk | Likelihood | Impact | Mitigation |
|------|------------|--------|------------|
| Missing function pointer | Medium | High | Comprehensive audit of all store method calls before migration |
| Type conversion errors | Medium | Medium | Add explicit conversion functions with tests |
| Test coverage drops | Low | High | Migrate ALL test cases, verify coverage ≥80% |
| Database init fails | Low | Critical | Preserve existing retry logic, test both SQLite and PostgreSQL |
| Broken imports | High | Medium | Use IDE refactoring, verify with `go build ./...` |
| Business logic lost | Low | High | Inline all helper functions, don't delete anything until verified |

---

## Acceptance Criteria

- [ ] All 5 store files use `db.*Func` pattern
- [ ] `internal/goosemigration/*` directory deleted (0 files remaining)
- [ ] No imports of `internal/goosemigration` in codebase
- [ ] All tests pass with `-race` flag
- [ ] Test coverage ≥80%
- [ ] Application starts successfully with both SQLite and PostgreSQL
- [ ] Database migrations run successfully
- [ ] Health check endpoint works
- [ ] Documentation updated (AGENTS.md, code comments)
- [ ] Manual testing of cart, checkout, admin operations passes

---

## Files Changed

**Added/Modified:**
- `internal/store/db/init.go` - Expanded with connection, migration, health
- `internal/store/db/queries.go` - Added missing function pointers
- `internal/store/carts.go` - Migrated to db.*Func
- `internal/store/products.go` - Migrated to db.*Func
- `internal/store/pages.go` - Migrated to db.*Func
- `internal/store/sessions.go` - Migrated to db.*Func (if needed)
- `internal/store/install.go` - Migrated to db.*Func
- `internal/store/*_test.go` - Merged goosemigration tests
- `internal/store/db/init_test.go` - New database init tests
- `internal/app.go` - Simplified initialization
- `internal/AGENTS.md` - Updated architecture docs
- `AGENTS.md` - Updated repo layout
- `docs/architecture/goosemigration-removal.md` - Optional migration guide

**Deleted:**
- `internal/goosemigration/` - Entire directory (34 files)

**Total impact:** ~50+ files changed, ~34 files deleted

---

## Open Questions

None - all design decisions confirmed with user.

---

## Next Steps

After design approval:
1. Run spec self-review
2. User reviews written spec
3. Invoke `writing-plans` skill to create implementation plan
4. Execute implementation following plan
