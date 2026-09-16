# sqlc Optional Support - Completely Separate Backend Design

**Date:** 2026-09-15  
**Author:** Claude Sonnet 4.5  
**Status:** Design - Option 3 (Completely Separate Backend)

## Overview

This spec defines the implementation of **optional sqlc support** for myCart using the **completely separate backend** approach. This approach creates an independent `internal/queries_sqlc/` package with zero modifications to the existing `internal/queries/` code.

## Goals

1. **Zero Existing Code Changes**: No modifications to `internal/queries/` files
2. **Complete Separation**: Two independent implementations that can evolve separately
3. **Optional**: sqlc support via Go build tags (`-tags sqlc`)
4. **Type-Safe**: Leverage sqlc's compile-time SQL validation
5. **Incremental**: Migrate query groups one at a time
6. **4-Mode Testing**: Both backends × both databases (raw/sqlc × SQLite/PostgreSQL)

## Design Decision: Option 3 - Completely Separate Backend

### Architecture

```
internal/
├── queries/              # Existing raw SQL implementation (untouched)
│   ├── queries.go        # Base, NewBase, etc.
│   ├── auth.go           # Raw SQL auth queries
│   ├── session.go        # Raw SQL session queries
│   └── ...
├── queries_sqlc/         # NEW: sqlc implementation (separate package)
│   ├── queries.go        # Base, NewBase (matching API)
│   ├── auth.go           # sqlc-generated auth queries
│   ├── session.go        # sqlc-generated session queries
│   └── ...
└── database/             # Shared by both (no changes)
    └── conn.go

cmd/
├── main.go               # Default: imports "internal/queries"
│   // +build !sqlc
│   import queries "github.com/shurco/litecart/internal/queries"
│
└── main_sqlc.go          # NEW: imports "internal/queries_sqlc" as "queries"
    // +build sqlc
    import queries "github.com/shurco/litecart/internal/queries_sqlc"
```

### Import Aliasing Strategy

**cmd/main.go** (existing, add build tag):
```go
// +build !sqlc

package main

import (
    "github.com/shurco/litecart/internal/queries"
    // ... rest of imports
)

func main() {
    // Existing code unchanged
    base := queries.NewBase(conn)
    // ...
}
```

**cmd/main_sqlc.go** (new):
```go
// +build sqlc

package main

import (
    queries "github.com/shurco/litecart/internal/queries_sqlc"
    // ... rest of imports (identical to main.go)
)

func main() {
    // Identical code to main.go
    base := queries.NewBase(conn)
    // ...
}
```

### API Compatibility Contract

Both packages **must** expose identical public APIs:

```go
// Both internal/queries and internal/queries_sqlc export:
type Base struct {
    // May have different internal fields
}

func NewBase(conn *database.Conn) *Base
func (b *Base) AuthByEmail(ctx context.Context, email string) (*types.Auth, error)
func (b *Base) AuthCreate(ctx context.Context, auth *types.Auth) error
// ... all other methods
```

**The consuming code in `cmd/main.go` and `cmd/main_sqlc.go` is identical** because both packages provide the same interface.

## Key Architectural Decision: Native pgx Pool

**Decision Made:** 2026-09-15 during implementation

### PostgreSQL Backend: Native pgxpool.Pool

The sqlc backend uses **native pgx** (`pgxpool.Pool`) instead of `database/sql`:

**Rationale:**
1. **Build tags = Single pool at runtime**: Only ONE backend runs at a time
   - Default build: `internal/queries/` uses `*sql.DB` via pgx/v5/stdlib
   - sqlc build: `internal/queries_sqlc/` uses `pgxpool.Pool` natively
   - Never both simultaneously → no resource overhead

2. **sqlc with `sql_package: "pgx/v5"`**: Generates native pgx code
   - Better type safety
   - Access to pgx-specific features if needed later
   - Cleaner generated code

3. **Clean separation**: Each backend optimized for its approach
   - Raw SQL backend: database/sql abstraction
   - sqlc backend: native pgx performance

**Implementation:**

```go
// backend_postgres.go
import "github.com/jackc/pgx/v5/pgxpool"

type postgresBackend struct {
    conn    *database.Conn  // Keep for dialect info
    pgxPool *pgxpool.Pool   // Native pgx for queries
    queries *pggen.Queries
}

func newPostgresBackend(conn *database.Conn) backend {
    // Extract DSN from conn or reconstruct
    cfg := extractPgxConfig(conn)
    pgxPool, _ := pgxpool.NewWithConfig(context.Background(), cfg)
    
    return &postgresBackend{
        conn:    conn,
        pgxPool: pgxPool,
        queries: pggen.New(pgxPool),
    }
}
```

### SQLite Backend: Standard database/sql

SQLite continues using `*sql.DB` since:
- No pgx equivalent for SQLite
- sqlc generates standard database/sql code for SQLite
- Works with `conn.Raw()` directly

## Implementation Strategy

### Phase 1: Infrastructure Setup

1. **Create package structure**:
   ```
   internal/queries_sqlc/
   ├── queries.go        # Base struct, NewBase function
   ├── backend.go        # Backend interface (same as queries/backend.go concept)
   └── stub.go           # Placeholder methods until migration complete
   ```

2. **Create build-tagged cmd files**:
   - Add `// +build !sqlc` to `cmd/main.go`
   - Create `cmd/main_sqlc.go` with `// +build sqlc`

3. **sqlc configuration** (`sqlc.yaml`):
   ```yaml
   version: "2"
   sql:
     - engine: "postgresql"
       schema: "db/schema"
       queries: "db/queries/postgres"
       gen:
         go:
           package: "pggen"
           out: "internal/queries_sqlc/sqlc/postgres"
           sql_package: "pgx/v5"
           emit_json_tags: true
           emit_interface: true
     
     - engine: "sqlite"
       schema: "db/schema"
       queries: "db/queries/sqlite"
       gen:
         go:
           package: "sqlitegen"
           out: "internal/queries_sqlc/sqlc/sqlite"
           emit_json_tags: true
           emit_interface: true
   ```

4. **Directory structure for SQL queries**:
   ```
   db/
   ├── queries/
   │   ├── postgres/
   │   │   ├── auth.sql
   │   │   ├── session.sql
   │   │   └── ...
   │   └── sqlite/
   │       ├── auth.sql
   │       ├── session.sql
   │       └── ...
   ```

### Phase 2: Incremental Migration (8 Query Groups)

Migrate one group at a time in this order:

1. **auth** (4 queries: ByEmail, Create, Update, Delete)
2. **session** (3 queries: ByToken, Create, Delete)
3. **install** (2 queries: CheckInstalled, MarkInstalled)
4. **setting** (3 queries: GetAll, GetByKey, Set)
5. **pages** (5 queries: List, ByID, Create, Update, Delete)
6. **product** (6 queries: List, ByID, Create, Update, Delete, Search)
7. **cart** (4 queries: ByID, AddItem, UpdateItem, DeleteItem)
8. **customer** (5 queries: List, ByID, Create, Update, Delete)

**Per-group migration steps**:

1. Write SQL queries in `db/queries/{postgres,sqlite}/group.sql`
2. Run `make sqlc-generate` → generates `internal/queries_sqlc/sqlc/{postgres,sqlite}/group.sql.go`
3. Create `internal/queries_sqlc/group.go` wrapping generated code:
   ```go
   // +build sqlc
   
   package queries_sqlc
   
   import (
       "context"
       "github.com/shurco/litecart/internal/types"
   )
   
   func (b *Base) AuthByEmail(ctx context.Context, email string) (*types.Auth, error) {
       // Delegate to backend (postgres or sqlite)
       return b.backend.AuthByEmail(ctx, email)
   }
   ```
4. Implement backend adapters in `internal/queries_sqlc/backend_postgres.go` and `backend_sqlite.go`
5. Test both engines: `make test-queries-sqlc-sqlite test-queries-sqlc-postgres`

### Phase 3: Testing Strategy

**4-Mode Test Matrix**:

| Backend | SQLite | PostgreSQL |
|---------|--------|------------|
| **Raw SQL** (`internal/queries`) | ✅ Default | ✅ `TEST_DB_DRIVER=postgres` |
| **sqlc** (`internal/queries_sqlc`) | ✅ `-tags sqlc` | ✅ `-tags sqlc TEST_DB_DRIVER=postgres` |

**Makefile Targets**:

```makefile
# Raw SQL tests (existing)
test-queries-raw-sqlite:
	go test ./internal/queries/... -count=1

test-queries-raw-postgres:
	TEST_DB_DRIVER=postgres \
	TEST_POSTGRES_DSN='postgres://postgres:password@localhost:5433/postgres?sslmode=disable' \
	go test ./internal/queries/... -count=1

# sqlc tests (new)
test-queries-sqlc-sqlite:
	go test -tags sqlc ./internal/queries_sqlc/... -count=1

test-queries-sqlc-postgres:
	TEST_DB_DRIVER=postgres \
	TEST_POSTGRES_DSN='postgres://postgres:password@localhost:5433/postgres?sslmode=disable' \
	go test -tags sqlc ./internal/queries_sqlc/... -count=1

# Run all 4 modes
test-queries-all: test-queries-raw-sqlite test-queries-raw-postgres \
                  test-queries-sqlc-sqlite test-queries-sqlc-postgres
```

## Pros and Cons

### Advantages

✅ **Zero existing code changes**: `internal/queries/` remains completely untouched  
✅ **Complete separation**: Can evolve both implementations independently  
✅ **No risk to production**: Existing backend unaffected by new code  
✅ **Easy rollback**: Simply don't use `-tags sqlc`, fall back to raw SQL  
✅ **Clean testing**: Each backend has its own test suite  
✅ **Optional feature**: Users choose at build time, not forced migration  

### Disadvantages

❌ **Code duplication**: Must maintain two implementations of the same API  
❌ **Drift risk**: Changes to one backend may not be applied to the other  
❌ **Double test burden**: Must test both backends in CI  
❌ **Build complexity**: Two separate build paths (`go build` vs `go build -tags sqlc`)  
❌ **Documentation overhead**: Must document both backends for contributors  

## Migration Path

### Stage 1: Infrastructure (No Feature Changes)

**Files created**:
- `sqlc.yaml` - sqlc configuration
- `db/queries/{postgres,sqlite}/` - directories for SQL queries
- `internal/queries_sqlc/queries.go` - Base struct, NewBase
- `internal/queries_sqlc/backend.go` - Backend interface
- `internal/queries_sqlc/stub.go` - No-op implementations
- `cmd/main_sqlc.go` - Build-tagged entrypoint

**Files modified**:
- `cmd/main.go` - Add `// +build !sqlc` tag (1 line)
- `Makefile` - Add sqlc targets

**Verification**:
```bash
# Default build (raw SQL) still works
go build ./cmd
./cmd serve

# sqlc build compiles (no-op stubs)
go build -tags sqlc ./cmd
./cmd serve
```

### Stage 2: Per-Group Migration (8 iterations)

**Example: Auth Group**

1. **Write queries** in `db/queries/postgres/auth.sql`:
   ```sql
   -- name: AuthByEmail :one
   SELECT id, email, password, role, created_at
   FROM auth
   WHERE email = $1 LIMIT 1;
   
   -- name: AuthCreate :exec
   INSERT INTO auth (email, password, role)
   VALUES ($1, $2, $3);
   ```

2. **Write queries** in `db/queries/sqlite/auth.sql`:
   ```sql
   -- name: AuthByEmail :one
   SELECT id, email, password, role, created_at
   FROM auth
   WHERE email = ? LIMIT 1;
   
   -- name: AuthCreate :exec
   INSERT INTO auth (email, password, role)
   VALUES (?, ?, ?);
   ```

3. **Generate code**:
   ```bash
   make sqlc-generate
   # Creates:
   # - internal/queries_sqlc/sqlc/postgres/auth.sql.go
   # - internal/queries_sqlc/sqlc/sqlite/auth.sql.go
   ```

4. **Implement wrappers** in `internal/queries_sqlc/auth.go`:
   ```go
   package queries_sqlc
   
   import (
       "context"
       "github.com/shurco/litecart/internal/types"
   )
   
   func (b *Base) AuthByEmail(ctx context.Context, email string) (*types.Auth, error) {
       return b.backend.AuthByEmail(ctx, email)
   }
   
   func (b *Base) AuthCreate(ctx context.Context, auth *types.Auth) error {
       return b.backend.AuthCreate(ctx, auth)
   }
   ```

5. **Implement backends**:
   - `internal/queries_sqlc/backend_postgres.go` - wraps pggen.Queries
   - `internal/queries_sqlc/backend_sqlite.go` - wraps sqlitegen.Queries

6. **Test**:
   ```bash
   make test-queries-sqlc-sqlite
   make test-queries-sqlc-postgres
   ```

7. **Repeat** for remaining 7 groups (session, install, setting, pages, product, cart, customer)

### Stage 3: Production Readiness

**Before recommending sqlc backend for production**:

- [ ] All 8 query groups migrated
- [ ] 4-mode test matrix passing
- [ ] Performance benchmarks (compare raw vs sqlc)
- [ ] Documentation updated (README, CONTRIBUTING, AGENTS.md)
- [ ] CI configured to test both backends
- [ ] Migration guide for users

## Build and Deployment

### Building

```bash
# Default: raw SQL backend
go build -o litecart ./cmd

# With sqlc backend
go build -tags sqlc -o litecart-sqlc ./cmd
```

### Docker

```dockerfile
# Multi-stage build supporting both backends
ARG USE_SQLC=false

FROM golang:1.26-alpine AS builder
ARG USE_SQLC
WORKDIR /app
COPY . .

RUN if [ "$USE_SQLC" = "true" ]; then \
        go build -tags sqlc -o litecart ./cmd; \
    else \
        go build -o litecart ./cmd; \
    fi

FROM alpine:latest
COPY --from=builder /app/litecart /litecart
ENTRYPOINT ["/litecart"]
```

### Release Artifacts

Provide both versions in GitHub releases:

- `litecart-linux-amd64` (raw SQL)
- `litecart-sqlc-linux-amd64` (sqlc)
- `litecart-darwin-amd64` (raw SQL)
- `litecart-sqlc-darwin-amd64` (sqlc)
- ... etc for all platforms

## Risk Assessment

### Low Risk

- **Existing functionality unaffected**: Zero changes to `internal/queries/`
- **Build tags prevent conflicts**: Cannot accidentally mix implementations
- **Easy rollback**: Remove `-tags sqlc` flag

### Medium Risk

- **Maintenance burden**: Two codebases to keep in sync
- **Test coverage gaps**: Might miss edge cases in one backend
- **Documentation drift**: One backend might become under-documented

### Mitigation Strategies

1. **Shared test cases**: Both backends run identical test suites
2. **CI enforcement**: Require both backends to pass before merge
3. **Regular audits**: Compare implementations quarterly for drift
4. **Feature flags**: Document which backend is recommended for production

## Success Criteria

1. ✅ `go build ./cmd` produces working binary (raw SQL)
2. ✅ `go build -tags sqlc ./cmd` produces working binary (sqlc)
3. ✅ Both binaries pass integration tests
4. ✅ No modifications to `internal/queries/` files
5. ✅ 4-mode test matrix passing in CI
6. ✅ Performance benchmarks show sqlc is competitive
7. ✅ Documentation covers both backends

## Open Questions

1. **Which backend should be default in 1-2 years?**
   - Keep both as options indefinitely?
   - Deprecate raw SQL after sqlc proves stable?
   - Let users choose, no "official" default?

2. **How to handle divergence?**
   - If sqlc backend gets new features first, how to backport to raw SQL?
   - Accept that features may not be available in both backends?

3. **CI complexity**:
   - Run both backends on every PR?
   - Only run sqlc backend if `db/queries/` or `internal/queries_sqlc/` changed?

## Next Steps

1. Get user approval on Option 3 design
2. Create implementation plan (breaking down into tasks)
3. Execute Phase 1: Infrastructure setup
4. Execute Phase 2: Migrate auth group (prove the pattern)
5. Execute Phase 2: Migrate remaining 7 groups
6. Execute Phase 3: Production readiness checklist

## References

- **Original wrapper delegation spec**: `docs/superpowers/specs/2026-09-14-sqlc-optional-support-design.md`
- **sqlc documentation**: https://docs.sqlc.dev/
- **Go build tags**: https://pkg.go.dev/cmd/go#hdr-Build_constraints
- **myCart queries layer**: `internal/queries/AGENTS.md`
- **myCart database layer**: `internal/database/AGENTS.md`
