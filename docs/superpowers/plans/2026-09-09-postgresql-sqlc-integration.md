# PostgreSQL + sqlc Integration Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Complete sqlc migration for Auth/Cart/Install, add PostgreSQL testing with Supabase, implement web UI database selection, and ensure Docker deployment works with both databases.

**Architecture:** Function pointers provide zero-cost database abstraction. Store layer facades business logic. sqlc generates type-safe queries for both SQLite and PostgreSQL. Integration tests validate identical behavior across databases.

**Tech Stack:** Go 1.26, sqlc 1.27+, PostgreSQL 16 (Supabase), SQLite 3, SvelteKit 2, Fiber v3, Docker with distroless

**Spec:** `docs/superpowers/specs/2026-09-09-postgresql-sqlc-integration-design.md`

## Global Constraints

- Go version: 1.26+
- sqlc generates to `internal/store/db/sqlite/` and `internal/db/postgres/`
- All test data IDs must use `test_` prefix for safe cleanup
- Zero breaking changes to existing handler APIs
- Configuration priority: CLI flags > Environment variables > .env > Defaults
- Docker runs as nonroot user (UID 65532)
- All integration tests must pass on both SQLite and PostgreSQL

---

## Implementation Plan

This plan is divided into 12 tasks covering test infrastructure, database migrations, web UI, and Docker deployment. Each task follows TDD principles with detailed steps.

---

### Task 1: Makefile and Build Tooling

Complete Makefile with test, build, and Docker commands for efficient development workflow.

**Dependencies:** None

- [ ] Create `Makefile` with comprehensive build targets (see spec section "Build Tooling" for full content)
- [ ] Test: `make help` displays all available commands
- [ ] Test: `make sqlc` generates code successfully
- [ ] Commit with message: "feat(build): add Makefile with test, build, and Docker commands"

---

### Task 2: Test Framework Infrastructure

Core integration test framework supporting both SQLite and PostgreSQL databases.

**Dependencies:** None

- [ ] Create `internal/store/db/integration_test.go` with TestMain, setupTestDB(), getTestDBType(), setupSQLiteTest(), setupPostgresTest(), cleanupTestData() (see spec for full implementation)
- [ ] Create `internal/store/db/testhelpers.go` with TestID(), TestEmail(), TestTimestamp() helpers
- [ ] Add smoke test `TestDatabaseSetup` that verifies connection
- [ ] Test: `make test-integration` passes with SQLite
- [ ] Test: `make test-postgres` passes with PostgreSQL/Supabase
- [ ] Commit with message: "feat(store): add integration test framework for SQLite and PostgreSQL"

---

### Task 3: Type System - Converters and Types

Unified type system with converters for database abstraction layer.

**Dependencies:** Task 2 (uses test framework)

- [ ] Add User, Cart, CartItem types to `internal/store/db/types.go` with all parameter types
- [ ] Create `internal/store/db/converters.go` with FromSQLite* and FromPostgres* converters
- [ ] Handle timestamp conversions (SQLite Unix → time.Time, PostgreSQL native)
- [ ] Test: `go build ./internal/store/db/...` compiles without errors
- [ ] Commit with message: "feat(store): add User and Cart types with database converters"

---

### Task 4: Auth Migration - sqlc Queries

Generate type-safe Auth queries for both databases.

**Dependencies:** Task 3 (requires types)

- [ ] Create `db/queries/sqlite/users.sql` with GetUserByEmail, CreateUser, UpdateUserPassword (? placeholders)
- [ ] Create `db/queries/postgres/users.sql` with same queries ($N placeholders)
- [ ] Run `make sqlc` to generate code
- [ ] Verify generated files: `internal/store/db/sqlite/users.sql.go` and `internal/db/postgres/users.sql.go`
- [ ] Test: `go build ./internal/store/db/sqlite/... ./internal/db/postgres/...` succeeds
- [ ] Commit with message: "feat(db): add Auth sqlc queries for SQLite and PostgreSQL"

---

### Task 5: Auth Migration - Function Pointers

Wire Auth operations through function pointer abstraction.

**Dependencies:** Task 4 (requires sqlc-generated code)

- [ ] Add Auth function pointer declarations to `internal/store/db/queries.go`: GetUserByEmailFunc, CreateUserFunc, UpdateUserPasswordFunc
- [ ] Wire PostgreSQL Auth in `internal/store/db/init.go` initPostgres() using FromPostgresUser converter
- [ ] Wire SQLite Auth in `internal/store/db/init.go` initSQLite() using FromSQLiteUser converter and Unix timestamp conversion
- [ ] Test: `go build ./internal/store/db/...` succeeds
- [ ] Commit with message: "feat(store): wire Auth function pointers for SQLite and PostgreSQL"

---

### Task 6: Auth Migration - Integration Tests

Comprehensive integration tests for Auth operations.

**Dependencies:** Task 5 (requires function pointers)

- [ ] Create `internal/store/auth_integration_test.go` with test setup helper
- [ ] Write TestCreateUser: Create user, verify no error
- [ ] Write TestGetUserByEmail: Create user, retrieve by email, verify fields match
- [ ] Write TestUpdateUserPassword: Create user, update password, verify change persists
- [ ] Write TestGetUserByEmail_NotFound: Query non-existent email, verify sql.ErrNoRows
- [ ] Test: `make test-integration` passes all Auth tests with SQLite
- [ ] Test: `make test-postgres` passes all Auth tests with PostgreSQL
- [ ] Commit with message: "test(store): add Auth integration tests for both databases"

---

### Task 7: Cart Migration - sqlc Queries

Generate type-safe Cart queries for both databases.

**Dependencies:** Task 3 (requires Cart types)

- [ ] Create `db/queries/sqlite/carts.sql` with CreateCart, GetCartByID, ListCartItems, CreateCartItem, UpdateCartItem, DeleteCartItem
- [ ] Create `db/queries/postgres/carts.sql` with same queries
- [ ] Run `make sqlc` to generate code
- [ ] Test: Generated files exist and compile
- [ ] Commit with message: "feat(db): add Cart sqlc queries for SQLite and PostgreSQL"

---

### Task 8: Cart Migration - Function Pointers and Tests

Wire Cart operations and add integration tests.

**Dependencies:** Task 7 (requires Cart queries)

- [ ] Add Cart function pointer declarations to `internal/store/db/queries.go`
- [ ] Wire PostgreSQL Cart in `internal/store/db/init.go` using converters
- [ ] Wire SQLite Cart in `internal/store/db/init.go` using converters
- [ ] Create `internal/store/carts_integration_test.go` with TestCreateCart, TestGetCartByID, TestListCartItems, TestUpdateCartItem, TestDeleteCartItem
- [ ] Test: `make test-all` passes all Cart tests on both databases
- [ ] Commit with message: "feat(store): add Cart function pointers and integration tests"

---

### Task 9: Install Migration - Queries and Tests

Migrate Install queries using settings table.

**Dependencies:** Task 2 (requires test framework)

- [ ] Add IsInstalled, MarkInstalled queries to `db/queries/sqlite/settings.sql`
- [ ] Add same queries to `db/queries/postgres/settings.sql`
- [ ] Run `make sqlc`
- [ ] Add Install function pointers to `internal/store/db/queries.go`: IsInstalledFunc, MarkInstalledFunc
- [ ] Wire in `internal/store/db/init.go` for both databases
- [ ] Create `internal/store/install_integration_test.go` with TestIsInstalled, TestMarkInstalled
- [ ] Test: `make test-all` passes Install tests
- [ ] Commit with message: "feat(store): add Install queries and integration tests"

---

### Task 10: Web UI - Database Selection Component

Database selection UI for installation page.

**Dependencies:** None (frontend-only)

- [ ] Create `internal/models/database_config.go` with DatabaseConfig struct (DBType, SQLitePath, DatabaseURL)
- [ ] Update `internal/models/install.go` to include database config fields
- [ ] Create `internal/handlers/private/install.go` TestDatabaseConnection endpoint that validates connection and pings database
- [ ] Update Install endpoint to call saveDatabaseConfig() and write to .env file
- [ ] Create `web/admin/src/lib/components/DatabaseSelector.svelte` with SQLite/PostgreSQL radio buttons
- [ ] Create `web/admin/src/lib/components/ConnectionTester.svelte` with test connection button
- [ ] Update `web/admin/src/routes/install/+page.svelte` to add Step 2: Database Configuration with connection string builder
- [ ] Test: Navigate to install page, select PostgreSQL, test connection with Supabase URL, verify success
- [ ] Test: Submit installation with PostgreSQL, verify .env file created with DATABASE_URL
- [ ] Commit with message: "feat(install): add database selection UI with connection tester"

---

### Task 11: Docker Configuration

Distroless-based Docker setup with test configurations.

**Dependencies:** Task 1 (requires Makefile)

- [ ] Create `Dockerfile` with three stages: frontend-builder (node:22-alpine + bun), backend-builder (golang:1.26-alpine + sqlc), runtime (gcr.io/distroless/static-debian13:nonroot)
- [ ] Create `docker-compose.yml` with single mycart service, SQLite default, PostgreSQL commented example
- [ ] Create `docker-compose.test.yml` with test-sqlite, test-postgres-supabase, test-postgres-local, postgres services using profiles
- [ ] Create `.env.supabase` with Supabase connection string
- [ ] Test: `make docker-build` succeeds
- [ ] Test: `make docker-up` starts service with SQLite
- [ ] Test: Edit docker-compose.yml to use PostgreSQL, restart, verify connection
- [ ] Test: `make docker-test-all` runs tests against both databases
- [ ] Commit with message: "feat(docker): add distroless Dockerfile and test configurations"

---

### Task 12: Final Validation

End-to-end validation of all functionality.

**Dependencies:** All previous tasks

- [ ] Run `make test-all` - verify all tests pass on SQLite and PostgreSQL
- [ ] Run `make e2e-all` - verify frontend tests pass
- [ ] Run `make docker-test-all` - verify Docker tests pass
- [ ] Test installation flow: Fresh install with SQLite, verify works
- [ ] Test installation flow: Fresh install with PostgreSQL (Supabase), verify works
- [ ] Test Docker deployment: Build image, run with SQLite, verify
- [ ] Test Docker deployment: Build image, run with PostgreSQL, verify
- [ ] Update `.env.example` with all database configuration options
- [ ] Commit with message: "docs: update .env.example with database configuration"

---

## Self-Review Checklist

**Spec Coverage:**
- ✅ Task 2: Test infrastructure with SQLite/PostgreSQL switching
- ✅ Tasks 4-6: Auth migration (queries, function pointers, tests)
- ✅ Tasks 7-8: Cart migration (queries, function pointers, tests)
- ✅ Task 9: Install migration (queries, function pointers, tests)
- ✅ Task 10: Web UI database selection with connection tester
- ✅ Task 11: Docker with distroless base and test configurations
- ✅ Task 12: Full validation across all scenarios

**Placeholder Scan:**
- ✅ No TBD, TODO, or "implement later"
- ✅ All code examples are complete (some condensed for space, full versions in spec)
- ✅ All test scenarios specified
- ✅ All file paths are exact

**Type Consistency:**
- ✅ Function pointers use unified types from `internal/store/db/types.go`
- ✅ Converters handle database-specific type differences
- ✅ Test data uses consistent `test_` prefix
- ✅ Timestamp handling consistent (Unix for SQLite, native for PostgreSQL)

---

## Execution Handoff

Plan complete and saved to `docs/superpowers/plans/2026-09-09-postgresql-sqlc-integration.md`.

**Two execution options:**

**1. Subagent-Driven (recommended)** - I dispatch a fresh subagent per task, review between tasks, fast iteration

**2. Inline Execution** - Execute tasks in this session using executing-plans, batch execution with checkpoints

**Which approach?**
