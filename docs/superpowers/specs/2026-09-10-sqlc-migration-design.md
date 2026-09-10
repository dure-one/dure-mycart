# Complete Migration to sqlc-Generated Queries

**Date:** 2026-09-10  
**Author:** Claude Sonnet 4.5  
**Status:** Design Approved  
**Approach:** Big Bang Migration

## Overview

This spec defines the complete migration of all raw SQL queries from `internal/store/` to type-safe sqlc-generated queries in `db/queries/`. This eliminates all `db.Type()` conditionals and `db.DB()` direct access, completing the PostgreSQL support feature.

## Goals

1. **Type Safety:** All database queries use sqlc-generated type-safe functions
2. **Database Abstraction:** Single codebase supports both SQLite and PostgreSQL without conditionals
3. **Performance:** Zero runtime overhead from function pointer abstraction
4. **Maintainability:** All SQL in dedicated `.sql` files, not scattered in Go code
5. **Testing:** Maintain 80%+ test coverage and all existing tests pass

## Non-Goals

- Changing the store layer API (handlers remain unchanged)
- Adding new database features
- Refactoring business logic
- Optimizing existing queries (maintain current behavior)

## Background

The codebase already has a well-established sqlc + function pointer pattern:
- sqlc generates type-safe queries from `.sql` files
- Function pointers in `db/queries.go` provide database abstraction
- `initPostgres()` and `initSQLite()` wire implementations at runtime

**Current state:** ~35 `db.Type()` conditionals and ~34 `db.DB()` direct queries remain in the store layer.

**Target state:** Zero raw SQL in store layer, all queries use function pointers.

## Design Decisions

### 1. Query Complexity Strategy: Separate Queries (Option A)

**Decision:** Create explicit sqlc queries for each use case (e.g., `ListPagesPrivate`, `ListPagesPublic`).

**Rationale:**
- Simple, readable SQL
- Type-safe at compile time
- Fast (no runtime conditionals)
- Matches Go philosophy (simple > clever)
- Aligns with existing codebase patterns

**Alternative rejected:** sqlc.narg() with optional parameters (complex SQL, runtime overhead, harder to optimize)

### 2. Function Pointer Organization: Single File (Option A)

**Decision:** Keep all function pointers in `db/queries.go` (current pattern).

**Rationale:**
- Existing pattern works well
- ~150 function pointers is manageable
- Single source of truth
- Easy to review all database operations

### 3. Transaction Handling: Transaction-Aware Pointers (Option A)

**Decision:** Create transaction-scoped query access via `TxQueries` struct and `WithTx()` helper.

**Rationale:**
- Type-safe transaction operations
- Automatic rollback on error
- Reuses existing function pointer pattern
- Works with both SQLite and PostgreSQL

### 4. Migration Approach: Big Bang (Option 1)

**Decision:** Complete migration in single comprehensive changeset.

**Rationale:**
- Already on dedicated feature branch (`feature/postgresql-support`)
- Comprehensive test suite exists
- Clean cut-over, no mixed state
- User preference for complete migration

**Alternative rejected:** Incremental file-by-file (lower risk but longer timeline)

## Architecture

### Current State

```
Raw SQL Query (pages.go)
         ↓
    db.Type() check → choose postgres or sqlite syntax
         ↓
    db.DB().QueryContext() → execute raw SQL
```

### Target State

```
Store Layer (pages.go)
         ↓
Function Pointer (db.ListPagesPrivateFunc)
         ↓
Runtime-selected implementation:
    - initPostgres() wires postgres.New(sqlDB).ListPagesPrivate
    - initSQLite() wires sqlite.New(sqlDB).ListPagesPrivate
         ↓
Type-safe sqlc-generated code
```

### Migration Scope

**Files to modify:**
```
db/queries/postgres/*.sql          (add ~20 new queries)
db/queries/sqlite/*.sql            (add ~20 new queries)
internal/store/db/queries.go       (add ~50 function pointers)
internal/store/db/types.go         (add unified param/return types)
internal/store/db/init.go          (wire new function pointers, add transaction helpers)
internal/store/pages.go            (remove db.Type(), use function pointers)
internal/store/products.go         (remove db.Type(), use function pointers)
internal/store/carts.go            (remove db.Type(), use function pointers)
internal/store/sessions.go         (remove db.Type(), use function pointers)
internal/store/install.go          (transaction-aware function pointers)
internal/handlers/private/product.go (update csvimport usage)
internal/webhook/hook_test.go      (update test helpers)
```

**What stays the same:**
- ✅ `db.Init()` as single entry point
- ✅ Function pointer pattern
- ✅ Type conversion helpers in `types.go`
- ✅ Store layer business logic
- ✅ Test files (only update if needed)

**What changes:**
- ❌ Remove all `db.Type()` conditionals (35 instances)
- ❌ Remove all `db.DB()` direct access (34 instances)
- ✅ Add sqlc query definitions (~40 new queries)
- ✅ Add function pointers (~50 new pointers)
- ✅ Update store methods to use function pointers

## Query Inventory

### Summary by Domain

| Domain | Raw Queries | New sqlc Queries |
|--------|-------------|------------------|
| Pages | 10 | 9 |
| Products | 15 | 8 |
| Carts | 5 | 4 |
| Sessions | 1 | 1 |
| Install | 4 | 3 |
| **Total** | **35** | **25** |

### Naming Convention

Following existing pattern:
- **Read operations:** `Get` (single), `List` (multiple), `Count`
- **Write operations:** `Create`, `Update`, `Delete`, `Upsert`
- **Scope suffix:** `Private`, `Public`, `ByID`, `BySlug`, `WithDetails`
- **Special operations:** `Batch`, `Expired`, `ByStatus`

## Implementation Details

Complete implementation details including:
- SQL query definitions for all domains
- Unified type system with conversion helpers
- Function pointer declarations and wiring patterns
- Transaction handling with `WithTx()` and `TxQueries`

See the full design sections presented during brainstorming for:
- Concrete SQL examples (postgres + sqlite)
- Type definitions and conversions
- Function pointer wiring in `initPostgres()` and `initSQLite()`
- Transaction helper implementation

## Testing Strategy

### Coverage Requirements

- ✅ Minimum 80% test coverage
- ✅ All existing tests pass
- ✅ Both SQLite and PostgreSQL tested
- ✅ No race conditions

### Testing Phases

1. Query generation validation (`sqlc generate`)
2. Compilation validation (`go build ./...`)
3. Unit test validation (per domain)
4. Integration test validation (both databases)
5. Handler integration tests
6. Full test suite with coverage
7. Manual smoke tests

### Regression Checklist

- [ ] Install flow
- [ ] Login flow
- [ ] Product CRUD
- [ ] Page CRUD
- [ ] Cart operations
- [ ] Session management
- [ ] Payment webhooks
- [ ] Digital product delivery
- [ ] CSV import/export

## Migration Execution Plan

### Time Estimate: ~5 hours

| Step | Task | Time | Checkpoint |
|------|------|------|------------|
| 1 | Create sqlc query files | 30-45 min | Review syntax |
| 2 | Generate sqlc code | 3 min | Compilation |
| 3 | Add unified types | 15-20 min | Compilation |
| 4 | Add function pointers | 10-15 min | Compilation |
| 5 | Wire function pointers | 45-60 min | Compilation |
| 6 | Update pages.go | 15-20 min | Unit tests |
| 7 | Update products.go | 30-40 min | Unit tests |
| 8 | Update carts.go | 10-15 min | Unit tests |
| 9 | Update sessions.go | 5 min | Unit tests |
| 10 | Update install.go | 15-20 min | Unit tests |
| 11 | Update handlers | 10-15 min | Handler tests |
| 12 | Full test suite | 5-10 min | Coverage >= 80% |
| 13 | Both databases | 10 min | SQLite + Postgres |
| 14 | Smoke tests | 10-15 min | Manual verification |
| 15 | Cleanup | 5 min | gofmt, go vet |
| 16 | Commit | 5 min | Git commit |

### Rollback Plan

If critical failure:
```bash
git checkout -- .
git clean -fd
```

Or commit partial work:
```bash
git add -A
git commit -m "WIP: partial sqlc migration (step X failed)"
```

## Success Criteria

- [ ] Zero `db.Type()` calls in `internal/store/`
- [ ] Zero `db.DB()` calls in `internal/store/`
- [ ] All tests passing (SQLite + PostgreSQL)
- [ ] Test coverage >= 80%
- [ ] No race conditions detected
- [ ] Manual smoke tests pass
- [ ] Code compiles without warnings
- [ ] All sqlc queries have matching postgres + sqlite versions

## Risks & Mitigations

| Risk | Likelihood | Impact | Mitigation |
|------|-----------|---------|------------|
| Type conversion errors (int32/int64) | Medium | High | Comprehensive type wrappers, test both databases |
| Query syntax differences missed | Low | Medium | Side-by-side postgres/sqlite query files |
| Transaction handling breaks | Low | High | Thorough testing of install flow |
| Test failures during migration | Medium | Medium | Checkpoint-based approach, easy rollback |
| Performance regression | Low | Low | Function pointers have zero runtime overhead |

## Future Work

- Add more sqlc queries for currently hard-coded operations
- Add prepared statement caching for hot queries
- Add query logging/tracing for debugging
- Consider moving to database/sql generics in Go 1.26+

## References

- [sqlc documentation](https://docs.sqlc.dev/)
- [PostgreSQL vs SQLite differences](https://www.sqlite.org/lang.html)
- ECC rules: `~/.claude/rules/ecc/golang/`
- Existing migration commits on `feature/postgresql-support` branch
