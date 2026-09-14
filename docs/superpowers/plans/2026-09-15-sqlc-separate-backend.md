# sqlc Optional Support - Completely Separate Backend Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implement optional sqlc support using the completely separate backend approach with zero modifications to existing `internal/queries/` code.

**Architecture:** Create independent `internal/queries_sqlc/` package with matching API, use build tags for import aliasing in `cmd/`, incremental migration of 8 query groups.

**Tech Stack:** Go 1.26, sqlc v1.27.0, PostgreSQL (pgx/v5), SQLite (modernc.org/sqlite), Goose migrations

## Global Constraints

- **Zero modifications** to `internal/queries/` files (except one build tag comment in `cmd/main.go`)
- **API compatibility**: `internal/queries_sqlc` must export identical public API as `internal/queries`
- **Incremental migration**: One query group at a time (auth → session → install → setting → pages → product → cart → customer)
- **4-mode testing**: Both backends × both databases (raw/sqlc × SQLite/PostgreSQL)
- **Build tags**: `-tags sqlc` switches from raw SQL to sqlc backend

---

## Phase 1: Infrastructure Setup

### Task 1: Create Package Structure and sqlc Configuration

**Files:**
- Create: `internal/queries_sqlc/queries.go`
- Create: `internal/queries_sqlc/backend.go`
- Create: `internal/queries_sqlc/stub.go`
- Create: `sqlc.yaml`
- Create: `db/queries/postgres/` (directory)
- Create: `db/queries/sqlite/` (directory)

**Interfaces:**
- Consumes: `internal/queries/queries.go` (API reference)
- Produces: Empty package structure with matching Base API

- [ ] **Step 1: Create package directory**

```bash
mkdir -p internal/queries_sqlc
mkdir -p db/queries/postgres
mkdir -p db/queries/sqlite
mkdir -p internal/queries_sqlc/sqlc/postgres
mkdir -p internal/queries_sqlc/sqlc/sqlite
```

Expected: Directories created successfully

- [ ] **Step 2: Create sqlc.yaml configuration**

```yaml
# sqlc.yaml
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
        emit_pointers_for_null_types: true
  
  - engine: "sqlite"
    schema: "db/schema"
    queries: "db/queries/sqlite"
    gen:
      go:
        package: "sqlitegen"
        out: "internal/queries_sqlc/sqlc/sqlite"
        emit_json_tags: true
        emit_interface: true
        emit_pointers_for_null_types: true
```

Verification: `sqlc version` confirms sqlc is installed

- [ ] **Step 3: Create queries.go with Base struct**

Read `internal/queries/queries.go` to understand the API, then create matching structure:

```go
// internal/queries_sqlc/queries.go
// +build sqlc

package queries_sqlc

import (
	"github.com/shurco/litecart/internal/database"
)

// Base provides query methods matching internal/queries.Base API
type Base struct {
	conn    *database.Conn
	backend backend
}

// NewBase creates a new queries base, initializing the appropriate backend
func NewBase(conn *database.Conn) *Base {
	base := &Base{conn: conn}
	
	// Initialize backend based on dialect
	switch conn.Dialect().Name() {
	case "postgres":
		base.backend = newPostgresBackend(conn)
	case "sqlite":
		base.backend = newSQLiteBackend(conn)
	default:
		panic("unsupported dialect: " + conn.Dialect().Name())
	}
	
	return base
}
```

- [ ] **Step 4: Create backend.go interface**

```go
// internal/queries_sqlc/backend.go
// +build sqlc

package queries_sqlc

import (
	"context"
	"github.com/shurco/litecart/internal/types"
)

// backend abstracts dialect-specific sqlc-generated code
type backend interface {
	// Auth methods (to be implemented in Task 3)
	AuthByEmail(ctx context.Context, email string) (*types.Auth, error)
	AuthCreate(ctx context.Context, auth *types.Auth) error
	AuthUpdate(ctx context.Context, auth *types.Auth) error
	AuthDelete(ctx context.Context, id int64) error
	
	// Session methods (to be implemented in Task 4)
	SessionByToken(ctx context.Context, token string) (*types.Session, error)
	SessionCreate(ctx context.Context, session *types.Session) error
	SessionDelete(ctx context.Context, token string) error
	
	// TODO: Add remaining query group methods as they are migrated
	// (install, setting, pages, product, cart, customer)
}
```

Note: Start with auth and session methods only, expand as we migrate more groups

- [ ] **Step 5: Create stub.go with no-op implementations**

```go
// internal/queries_sqlc/stub.go
// +build sqlc

package queries_sqlc

import (
	"context"
	"errors"
	"github.com/shurco/litecart/internal/types"
)

var ErrNotImplemented = errors.New("sqlc query not yet implemented")

// Stub methods on Base - these will be replaced group-by-group

func (b *Base) AuthByEmail(ctx context.Context, email string) (*types.Auth, error) {
	return b.backend.AuthByEmail(ctx, email)
}

func (b *Base) AuthCreate(ctx context.Context, auth *types.Auth) error {
	return b.backend.AuthCreate(ctx, auth)
}

func (b *Base) AuthUpdate(ctx context.Context, auth *types.Auth) error {
	return b.backend.AuthUpdate(ctx, auth)
}

func (b *Base) AuthDelete(ctx context.Context, id int64) error {
	return b.backend.AuthDelete(ctx, id)
}

func (b *Base) SessionByToken(ctx context.Context, token string) (*types.Session, error) {
	return b.backend.SessionByToken(ctx, token)
}

func (b *Base) SessionCreate(ctx context.Context, session *types.Session) error {
	return b.backend.SessionCreate(ctx, session)
}

func (b *Base) SessionDelete(ctx context.Context, token string) error {
	return b.backend.SessionDelete(ctx, token)
}

// TODO: Add stub methods for remaining query groups
// For now, these will return ErrNotImplemented if called
```

- [ ] **Step 6: Create backend stubs**

```go
// internal/queries_sqlc/backend_postgres.go
// +build sqlc

package queries_sqlc

import (
	"context"
	"github.com/shurco/litecart/internal/database"
	"github.com/shurco/litecart/internal/types"
)

type postgresBackend struct {
	conn *database.Conn
	// queries *pggen.Queries // will be added after sqlc generation
}

func newPostgresBackend(conn *database.Conn) backend {
	return &postgresBackend{conn: conn}
}

func (p *postgresBackend) AuthByEmail(ctx context.Context, email string) (*types.Auth, error) {
	return nil, ErrNotImplemented
}

func (p *postgresBackend) AuthCreate(ctx context.Context, auth *types.Auth) error {
	return ErrNotImplemented
}

func (p *postgresBackend) AuthUpdate(ctx context.Context, auth *types.Auth) error {
	return ErrNotImplemented
}

func (p *postgresBackend) AuthDelete(ctx context.Context, id int64) error {
	return ErrNotImplemented
}

func (p *postgresBackend) SessionByToken(ctx context.Context, token string) (*types.Session, error) {
	return nil, ErrNotImplemented
}

func (p *postgresBackend) SessionCreate(ctx context.Context, session *types.Session) error {
	return ErrNotImplemented
}

func (p *postgresBackend) SessionDelete(ctx context.Context, token string) error {
	return ErrNotImplemented
}
```

```go
// internal/queries_sqlc/backend_sqlite.go
// +build sqlc

package queries_sqlc

import (
	"context"
	"github.com/shurco/litecart/internal/database"
	"github.com/shurco/litecart/internal/types"
)

type sqliteBackend struct {
	conn *database.Conn
	// queries *sqlitegen.Queries // will be added after sqlc generation
}

func newSQLiteBackend(conn *database.Conn) backend {
	return &sqliteBackend{conn: conn}
}

func (s *sqliteBackend) AuthByEmail(ctx context.Context, email string) (*types.Auth, error) {
	return nil, ErrNotImplemented
}

func (s *sqliteBackend) AuthCreate(ctx context.Context, auth *types.Auth) error {
	return ErrNotImplemented
}

func (s *sqliteBackend) AuthUpdate(ctx context.Context, auth *types.Auth) error {
	return ErrNotImplemented
}

func (s *sqliteBackend) AuthDelete(ctx context.Context, id int64) error {
	return ErrNotImplemented
}

func (s *sqliteBackend) SessionByToken(ctx context.Context, token string) (*types.Session, error) {
	return nil, ErrNotImplemented
}

func (s *sqliteBackend) SessionCreate(ctx context.Context, session *types.Session) error {
	return ErrNotImplemented
}

func (s *sqliteBackend) SessionDelete(ctx context.Context, token string) error {
	return ErrNotImplemented
}
```

- [ ] **Step 7: Verify package builds with sqlc tag**

```bash
go build -tags sqlc ./internal/queries_sqlc/...
```

Expected: Package compiles successfully (with ErrNotImplemented stubs)

---

### Task 2: Add Build Tags to cmd/

**Files:**
- Modify: `cmd/main.go` (add 1-line build tag)
- Create: `cmd/main_sqlc.go` (copy of main.go with different import)

**Interfaces:**
- Consumes: `cmd/main.go` structure
- Produces: Two build-tagged entrypoints with import aliasing

- [ ] **Step 1: Add build tag to existing cmd/main.go**

Read `cmd/main.go` and add build tag at the very top:

```go
// +build !sqlc

package main

// ... rest of file unchanged
```

This is the ONLY modification to existing code outside `internal/queries_sqlc/`

- [ ] **Step 2: Create cmd/main_sqlc.go**

Copy `cmd/main.go` to `cmd/main_sqlc.go` and modify:

```go
// +build sqlc

package main

import (
	// ... all existing imports unchanged ...
	
	// Import queries_sqlc package aliased as "queries"
	queries "github.com/shurco/litecart/internal/queries_sqlc"
	
	// ... rest of imports ...
)

// ... rest of file is IDENTICAL to main.go ...
```

The key difference: `import queries "github.com/shurco/litecart/internal/queries_sqlc"`

- [ ] **Step 3: Verify both builds compile**

```bash
# Default build (raw SQL)
go build -o /tmp/litecart-raw ./cmd
echo "Binary size (raw):" $(du -h /tmp/litecart-raw | cut -f1)

# sqlc build
go build -tags sqlc -o /tmp/litecart-sqlc ./cmd
echo "Binary size (sqlc):" $(du -h /tmp/litecart-sqlc | cut -f1)
```

Expected: Both binaries compile successfully

- [ ] **Step 4: Verify default build still uses raw SQL**

```bash
# Ensure no sqlc code is compiled in default build
go build -o /tmp/litecart-test ./cmd
! nm /tmp/litecart-test | grep -q queries_sqlc
echo "Default build correctly excludes sqlc code: $?"
```

Expected: Exit code 0 (grep finds nothing, ! inverts to success)

---

### Task 3: Update Makefile

**Files:**
- Modify: `Makefile` (add sqlc targets)

**Interfaces:**
- Consumes: Existing Makefile structure
- Produces: New targets for sqlc generation and testing

- [ ] **Step 1: Add sqlc installation target**

```makefile
# Add to Makefile

.PHONY: install-sqlc
install-sqlc:
	@command -v sqlc >/dev/null 2>&1 || { \
		echo "Installing sqlc..."; \
		go install github.com/sqlc-dev/sqlc/cmd/sqlc@latest; \
	}
	@echo "sqlc installed: $$(sqlc version)"
```

- [ ] **Step 2: Add sqlc generation targets**

```makefile
.PHONY: sqlc-generate
sqlc-generate: install-sqlc
	@echo "Generating sqlc code..."
	@sqlc generate
	@echo "✓ Generated internal/queries_sqlc/sqlc/postgres/*.go"
	@echo "✓ Generated internal/queries_sqlc/sqlc/sqlite/*.go"

.PHONY: sqlc-verify
sqlc-verify: sqlc-generate
	@echo "Verifying generated code builds..."
	@go build -tags sqlc ./internal/queries_sqlc/...
	@echo "✓ sqlc code verified"
```

- [ ] **Step 3: Add test targets for 4-mode matrix**

```makefile
# Test targets for 4-mode matrix

.PHONY: test-queries-raw-sqlite
test-queries-raw-sqlite:
	@echo "Testing raw SQL backend with SQLite..."
	@go test ./internal/queries/... -count=1 $(if $(RACE_FLAG),$(RACE_FLAG),)

.PHONY: test-queries-raw-postgres
test-queries-raw-postgres:
	@echo "Testing raw SQL backend with PostgreSQL..."
	@TEST_DB_DRIVER=postgres \
	TEST_POSTGRES_DSN='postgres://postgres:password@localhost:5433/postgres?sslmode=disable' \
	go test ./internal/queries/... -count=1 $(if $(RACE_FLAG),$(RACE_FLAG),)

.PHONY: test-queries-sqlc-sqlite
test-queries-sqlc-sqlite:
	@echo "Testing sqlc backend with SQLite..."
	@go test -tags sqlc ./internal/queries_sqlc/... -count=1 $(if $(RACE_FLAG),$(RACE_FLAG),)

.PHONY: test-queries-sqlc-postgres
test-queries-sqlc-postgres:
	@echo "Testing sqlc backend with PostgreSQL..."
	@TEST_DB_DRIVER=postgres \
	TEST_POSTGRES_DSN='postgres://postgres:password@localhost:5433/postgres?sslmode=disable' \
	go test -tags sqlc ./internal/queries_sqlc/... -count=1 $(if $(RACE_FLAG),$(RACE_FLAG),)

.PHONY: test-queries-all
test-queries-all: test-queries-raw-sqlite test-queries-raw-postgres \
                  test-queries-sqlc-sqlite test-queries-sqlc-postgres
	@echo "✓ All 4-mode query tests passed"
```

- [ ] **Step 4: Add build targets**

```makefile
.PHONY: build-sqlc
build-sqlc: sqlc-generate
	@echo "Building with sqlc backend..."
	@go build -tags sqlc -o litecart-sqlc ./cmd
	@echo "✓ Built litecart-sqlc"

.PHONY: build-both
build-both: build build-sqlc
	@echo "✓ Built both backends"
	@ls -lh litecart litecart-sqlc
```

- [ ] **Step 5: Verify Makefile targets**

```bash
make sqlc-verify
make build-both
```

Expected: All targets run successfully

---

## Phase 2: Incremental Migration (8 Query Groups)

### Task 4: Migrate Auth Group (Proof of Concept)

**Files:**
- Create: `db/queries/postgres/auth.sql`
- Create: `db/queries/sqlite/auth.sql`
- Generate: `internal/queries_sqlc/sqlc/postgres/auth.sql.go` (via sqlc)
- Generate: `internal/queries_sqlc/sqlc/sqlite/auth.sql.go` (via sqlc)
- Create: `internal/queries_sqlc/auth.go` (wrapper)
- Modify: `internal/queries_sqlc/backend_postgres.go` (implement auth methods)
- Modify: `internal/queries_sqlc/backend_sqlite.go` (implement auth methods)
- Create: `internal/queries_sqlc/auth_test.go` (tests)

**Interfaces:**
- Consumes: `internal/queries/auth.go` (API reference)
- Produces: Fully functional auth queries in sqlc backend

- [ ] **Step 1: Read existing auth.go to understand API**

```bash
# Review the interface we need to match
cat internal/queries/auth.go | grep '^func (b \*Base)'
```

Expected: See methods like `AuthByEmail`, `AuthCreate`, `AuthUpdate`, `AuthDelete`

- [ ] **Step 2: Write PostgreSQL queries**

Create `db/queries/postgres/auth.sql`:

```sql
-- name: AuthByEmail :one
SELECT id, email, password, role, created_at, updated_at
FROM auth
WHERE email = $1
LIMIT 1;

-- name: AuthCreate :one
INSERT INTO auth (email, password, role)
VALUES ($1, $2, $3)
RETURNING id, email, password, role, created_at, updated_at;

-- name: AuthUpdate :exec
UPDATE auth
SET email = $1, password = $2, role = $3, updated_at = CURRENT_TIMESTAMP
WHERE id = $4;

-- name: AuthDelete :exec
DELETE FROM auth WHERE id = $1;
```

- [ ] **Step 3: Write SQLite queries**

Create `db/queries/sqlite/auth.sql`:

```sql
-- name: AuthByEmail :one
SELECT id, email, password, role, created_at, updated_at
FROM auth
WHERE email = ?
LIMIT 1;

-- name: AuthCreate :one
INSERT INTO auth (email, password, role)
VALUES (?, ?, ?)
RETURNING id, email, password, role, created_at, updated_at;

-- name: AuthUpdate :exec
UPDATE auth
SET email = ?, password = ?, role = ?, updated_at = CURRENT_TIMESTAMP
WHERE id = ?;

-- name: AuthDelete :exec
DELETE FROM auth WHERE id = ?;
```

- [ ] **Step 4: Generate sqlc code**

```bash
make sqlc-generate
```

Expected: Generated files appear:
- `internal/queries_sqlc/sqlc/postgres/auth.sql.go`
- `internal/queries_sqlc/sqlc/sqlite/auth.sql.go`

- [ ] **Step 5: Implement PostgreSQL backend adapter**

Update `internal/queries_sqlc/backend_postgres.go` to use native pgxpool:

```go
// Add imports
import (
	"context"
	"fmt"
	
	"github.com/jackc/pgx/v5/pgxpool"
	pggen "github.com/shurco/litecart/internal/queries_sqlc/sqlc/postgres"
	"github.com/shurco/mycart/internal/database"
	"github.com/shurco/mycart/pkg/errors"
)

type postgresBackend struct {
	conn    *database.Conn
	pgxPool *pgxpool.Pool
	queries *pggen.Queries
}

func newPostgresBackend(conn *database.Conn) backend {
	// Get DSN - need to extract from conn or use active config
	cfg := database.Active()
	
	// Create native pgx pool
	pgxCfg, err := pgxpool.ParseConfig(cfg.DSN)
	if err != nil {
		panic(fmt.Sprintf("parse pgx config: %v", err))
	}
	
	pgxPool, err := pgxpool.NewWithConfig(context.Background(), pgxCfg)
	if err != nil {
		panic(fmt.Sprintf("create pgx pool: %v", err))
	}
	
	return &postgresBackend{
		conn:    conn,
		pgxPool: pgxPool,
		queries: pggen.New(pgxPool),
	}
}

func (p *postgresBackend) AuthByEmail(ctx context.Context, email string) (*types.Auth, error) {
	row, err := p.queries.AuthByEmail(ctx, email)
	if err != nil {
		return nil, fmt.Errorf("AuthByEmail: %w", err)
	}
	
	return &types.Auth{
		ID:        row.ID,
		Email:     row.Email,
		Password:  row.Password,
		Role:      row.Role,
		CreatedAt: row.CreatedAt.Unix(),
		UpdatedAt: row.UpdatedAt.Unix(),
	}, nil
}

func (p *postgresBackend) AuthCreate(ctx context.Context, auth *types.Auth) error {
	row, err := p.queries.AuthCreate(ctx, pggen.AuthCreateParams{
		Email:    auth.Email,
		Password: auth.Password,
		Role:     auth.Role,
	})
	if err != nil {
		return fmt.Errorf("AuthCreate: %w", err)
	}
	
	auth.ID = row.ID
	auth.CreatedAt = row.CreatedAt.Unix()
	auth.UpdatedAt = row.UpdatedAt.Unix()
	return nil
}

func (p *postgresBackend) AuthUpdate(ctx context.Context, auth *types.Auth) error {
	err := p.queries.AuthUpdate(ctx, pggen.AuthUpdateParams{
		Email:    auth.Email,
		Password: auth.Password,
		Role:     auth.Role,
		ID:       auth.ID,
	})
	if err != nil {
		return fmt.Errorf("AuthUpdate: %w", err)
	}
	return nil
}

func (p *postgresBackend) AuthDelete(ctx context.Context, id int64) error {
	err := p.queries.AuthDelete(ctx, id)
	if err != nil {
		return fmt.Errorf("AuthDelete: %w", err)
	}
	return nil
}

// Remove ErrNotImplemented returns
```

- [ ] **Step 6: Implement SQLite backend adapter**

Update `internal/queries_sqlc/backend_sqlite.go` (similar pattern, using `sqlitegen` package)

- [ ] **Step 7: Remove stub.go auth methods**

Remove the auth methods from `stub.go` since they're now implemented in `backend_*.go`

- [ ] **Step 8: Create auth_test.go**

```go
// internal/queries_sqlc/auth_test.go
// +build sqlc

package queries_sqlc

import (
	"context"
	"testing"
	
	"github.com/shurco/litecart/internal/testutil"
	"github.com/shurco/litecart/internal/types"
)

func TestAuthByEmail(t *testing.T) {
	t.Parallel()
	
	db := testutil.SetupTestDB(t)
	base := NewBase(db)
	
	ctx := context.Background()
	
	// Create test user
	auth := &types.Auth{
		Email:    "test@example.com",
		Password: "hashed_password",
		Role:     "user",
	}
	
	err := base.AuthCreate(ctx, auth)
	if err != nil {
		t.Fatalf("AuthCreate failed: %v", err)
	}
	
	// Retrieve by email
	found, err := base.AuthByEmail(ctx, "test@example.com")
	if err != nil {
		t.Fatalf("AuthByEmail failed: %v", err)
	}
	
	if found.Email != auth.Email {
		t.Errorf("Email mismatch: got %s, want %s", found.Email, auth.Email)
	}
	
	if found.Role != auth.Role {
		t.Errorf("Role mismatch: got %s, want %s", found.Role, auth.Role)
	}
}

func TestAuthCRUD(t *testing.T) {
	t.Parallel()
	
	db := testutil.SetupTestDB(t)
	base := NewBase(db)
	
	ctx := context.Background()
	
	// Create
	auth := &types.Auth{
		Email:    "crud@example.com",
		Password: "password123",
		Role:     "admin",
	}
	
	err := base.AuthCreate(ctx, auth)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	
	if auth.ID == 0 {
		t.Fatal("Expected ID to be set after create")
	}
	
	// Update
	auth.Role = "user"
	err = base.AuthUpdate(ctx, auth)
	if err != nil {
		t.Fatalf("Update failed: %v", err)
	}
	
	// Read and verify update
	updated, err := base.AuthByEmail(ctx, "crud@example.com")
	if err != nil {
		t.Fatalf("Read after update failed: %v", err)
	}
	
	if updated.Role != "user" {
		t.Errorf("Role not updated: got %s, want user", updated.Role)
	}
	
	// Delete
	err = base.AuthDelete(ctx, auth.ID)
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}
	
	// Verify deletion
	_, err = base.AuthByEmail(ctx, "crud@example.com")
	if err == nil {
		t.Error("Expected error after delete, got nil")
	}
}
```

- [ ] **Step 9: Run auth tests (4-mode matrix)**

```bash
make test-queries-sqlc-sqlite
make test-queries-sqlc-postgres
```

Expected: All auth tests pass on both databases

- [ ] **Step 10: Compare with raw SQL tests**

```bash
# Ensure sqlc implementation matches raw SQL behavior
make test-queries-raw-sqlite
make test-queries-sqlc-sqlite

# Both should pass with same coverage
```

---

### Task 5-11: Migrate Remaining Query Groups

**Repeat Task 4 pattern for each group:**

- Task 5: Session (3 queries)
- Task 6: Install (2 queries)
- Task 7: Setting (3 queries)
- Task 8: Pages (5 queries)
- Task 9: Product (6 queries)
- Task 10: Cart (4 queries)
- Task 11: Customer (5 queries)

Each task follows the same steps:
1. Write PostgreSQL queries in `db/queries/postgres/<group>.sql`
2. Write SQLite queries in `db/queries/sqlite/<group>.sql`
3. Run `make sqlc-generate`
4. Implement `backend_postgres.go` methods
5. Implement `backend_sqlite.go` methods
6. Create `<group>_test.go` tests
7. Run 4-mode test matrix

---

## Phase 3: Documentation and CI

### Task 12: Update Documentation

**Files:**
- Modify: `README.md` (add sqlc build instructions)
- Modify: `AGENTS.md` (document both backends)
- Create: `docs/sqlc-backend.md` (detailed sqlc guide)

- [ ] **Step 1: Update README.md**

Add section:

```markdown
## Building

### Default Build (Raw SQL)

```bash
go build -o litecart ./cmd
```

### Build with sqlc Backend (Optional)

For type-safe SQL queries with compile-time validation:

```bash
# Install sqlc (one-time)
make install-sqlc

# Generate sqlc code
make sqlc-generate

# Build with sqlc backend
make build-sqlc
```

Both backends support SQLite and PostgreSQL.
```

- [ ] **Step 2: Update AGENTS.md**

Add section after database overview:

```markdown
### Query Backend Options

Two query backends are available:

1. **Raw SQL** (`internal/queries/`) - Default, hand-written SQL
2. **sqlc** (`internal/queries_sqlc/`) - Generated type-safe queries

Both backends:
- Support SQLite and PostgreSQL
- Export identical public APIs
- Pass the same test suite

Use `-tags sqlc` to switch backends at build time.

See `internal/queries_sqlc/AGENTS.md` for sqlc-specific guidance.
```

- [ ] **Step 3: Create internal/queries_sqlc/AGENTS.md**

Document sqlc-specific patterns and maintenance

- [ ] **Step 4: Create docs/sqlc-backend.md**

Comprehensive guide for users choosing sqlc backend

---

### Task 13: CI Configuration

**Files:**
- Modify: `.github/workflows/test.yml` (add 4-mode matrix)

- [ ] **Step 1: Add sqlc test job to CI**

```yaml
# .github/workflows/test.yml

name: Test

on: [push, pull_request]

jobs:
  test-raw-sql:
    name: Test Raw SQL Backend
    runs-on: ubuntu-latest
    strategy:
      matrix:
        db: [sqlite, postgres]
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: '1.26'
      
      - name: Start PostgreSQL (if needed)
        if: matrix.db == 'postgres'
        run: |
          docker run -d -p 5432:5432 \
            -e POSTGRES_PASSWORD=password \
            --tmpfs /var/lib/postgresql/data:rw \
            postgres:17-alpine
      
      - name: Test
        run: |
          if [ "${{ matrix.db }}" = "postgres" ]; then
            export TEST_DB_DRIVER=postgres
            export TEST_POSTGRES_DSN='postgres://postgres:password@localhost:5432/postgres?sslmode=disable'
          fi
          make test-queries-raw-${{ matrix.db }}
  
  test-sqlc:
    name: Test sqlc Backend
    runs-on: ubuntu-latest
    strategy:
      matrix:
        db: [sqlite, postgres]
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: '1.26'
      
      - name: Install sqlc
        run: make install-sqlc
      
      - name: Generate sqlc code
        run: make sqlc-generate
      
      - name: Start PostgreSQL (if needed)
        if: matrix.db == 'postgres'
        run: |
          docker run -d -p 5432:5432 \
            -e POSTGRES_PASSWORD=password \
            --tmpfs /var/lib/postgresql/data:rw \
            postgres:17-alpine
      
      - name: Test
        run: |
          if [ "${{ matrix.db }}" = "postgres" ]; then
            export TEST_DB_DRIVER=postgres
            export TEST_POSTGRES_DSN='postgres://postgres:password@localhost:5432/postgres?sslmode=disable'
          fi
          make test-queries-sqlc-${{ matrix.db }}
```

- [ ] **Step 2: Verify CI passes**

Push branch and ensure all 4 test jobs pass (raw-sqlite, raw-postgres, sqlc-sqlite, sqlc-postgres)

---

## Verification Checklist

After completing all tasks:

**Infrastructure:**
- [ ] `internal/queries_sqlc/` package structure created
- [ ] `sqlc.yaml` configuration correct
- [ ] `db/queries/{postgres,sqlite}/` directories exist
- [ ] Build tags added to `cmd/main.go` and `cmd/main_sqlc.go`
- [ ] Makefile targets for sqlc generation and testing

**Implementation:**
- [ ] All 8 query groups migrated (auth, session, install, setting, pages, product, cart, customer)
- [ ] PostgreSQL backend fully implemented
- [ ] SQLite backend fully implemented
- [ ] No ErrNotImplemented errors remain

**Testing:**
- [ ] 4-mode test matrix passing:
  - [ ] Raw SQL + SQLite
  - [ ] Raw SQL + PostgreSQL
  - [ ] sqlc + SQLite
  - [ ] sqlc + PostgreSQL
- [ ] Test coverage matches or exceeds raw SQL backend
- [ ] Integration tests pass with both backends

**Documentation:**
- [ ] README.md updated with build instructions
- [ ] AGENTS.md documents both backends
- [ ] internal/queries_sqlc/AGENTS.md created
- [ ] docs/sqlc-backend.md created

**CI:**
- [ ] CI runs all 4 test modes
- [ ] All CI jobs passing

**Zero Changes Verification:**
- [ ] `internal/queries/` files unchanged (except build tag comment in `cmd/main.go`)
- [ ] `git diff internal/queries/` shows zero changes
- [ ] Existing code still compiles and passes tests without `-tags sqlc`

---

## Success Criteria

1. ✅ Zero modifications to `internal/queries/` (except one build tag)
2. ✅ Both backends compile: `go build ./cmd` and `go build -tags sqlc ./cmd`
3. ✅ All 4 test modes passing in CI
4. ✅ sqlc backend API matches raw SQL backend API
5. ✅ Documentation covers both backends
6. ✅ Users can choose backend at build time

---

## Execution Handoff

Plan complete and saved to `docs/superpowers/plans/2026-09-15-sqlc-separate-backend.md`.

**Recommended execution approach:**
1. Execute Task 1-3 (Infrastructure) first - proves the foundation
2. Execute Task 4 (Auth migration) - proves the pattern works
3. Execute Task 5-11 (Remaining groups) - can be parallelized
4. Execute Task 12-13 (Documentation and CI) - finalize

**Next step:** Get user approval, then begin with Task 1 (Package Structure).
