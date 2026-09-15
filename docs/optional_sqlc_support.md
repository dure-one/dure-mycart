# Optional sqlc Support

myCart has two SQL backend implementations:

1. **Raw SQL (default)**: Hand-written SQL queries with runtime parameter binding
2. **sqlc (optional)**: Compile-time type-safe queries generated from annotated SQL

Both backends support SQLite and PostgreSQL, pass the same test suite, and export identical public APIs. **You choose at build time** via Go build tags — the application binary uses one backend or the other, not both.

## Which one?

| | Raw SQL (default) | sqlc (optional) |
|---|---|---|
| Setup | none | install sqlc, run code generator |
| Type safety | runtime (via `database.Conn` wrapper) | compile-time (generated Go types) |
| Query changes | edit `.go` files | edit `.sql` files, regenerate code |
| Build speed | fast (no generation step) | slower (generate → compile) |
| Compilation errors | if query syntax wrong at runtime | if query syntax wrong at generation time |
| Fits | quick iteration, prototype mode | strict type safety, production mode |

Both run the same migrations and use the same `database.Conn` abstraction. There is no reduced-feature mode.

## Why use sqlc?

**Compile-time type safety.** sqlc parses your SQL queries and generates Go code with correct types:

```sql
-- db/queries/postgres/customer.sql
-- name: GetCustomerByEmail :one
SELECT id, email, name, created
FROM customer
WHERE email = $1;
```

sqlc generates:

```go
// internal/queries_sqlc/sqlc/postgres/customer.sql.go
type GetCustomerByEmailRow struct {
    ID      string
    Email   string
    Name    string
    Created int64
}

func (q *Queries) GetCustomerByEmail(ctx context.Context, email string) (GetCustomerByEmailRow, error) {
    // ... generated implementation
}
```

**Benefits:**
- Wrong column names fail at compile time, not runtime
- Typos in table names caught during generation
- Type mismatches (e.g., treating `int64` as `string`) rejected by compiler
- IDE autocomplete for query result fields
- No reflection, no `interface{}` casting

**Cost:**
- Must run `make sqlc-generate` after every query change
- Slower build (generation + compilation vs just compilation)
- Two sets of query files to maintain (`.sql` files + generated `.go` files)

## Installing sqlc

One command:

```bash
make install-sqlc
```

This installs the `sqlc` CLI tool via `go install`. Verify:

```bash
sqlc version
# v1.27.0 (or similar)
```

## Generating code

After modifying any `.sql` file in `db/queries/postgres/` or `db/queries/sqlite/`:

```bash
make sqlc-generate
```

This:
1. Reads `sqlc.yaml` configuration
2. Parses schema from `db/schema/` (migrations)
3. Parses queries from `db/queries/postgres/*.sql` and `db/queries/sqlite/*.sql`
4. Generates type-safe Go code:
   - `internal/queries_sqlc/sqlc/postgres/*.go` (PostgreSQL dialect)
   - `internal/queries_sqlc/sqlc/sqlite/*.go` (SQLite dialect)

**Verification mode** (recommended):

```bash
make sqlc-verify
```

This runs `sqlc generate` **and** verifies the generated code compiles.

**Quick mode** (skip verification):

```bash
make sqlc
```

## Building with sqlc

Build the sqlc-enabled backend:

```bash
make build-sqlc
```

This runs:
1. `make sqlc-generate` (ensures code is up-to-date)
2. `go build -tags sqlc -o mycart-sqlc ./cmd`

The resulting `mycart-sqlc` binary uses the generated queries instead of raw SQL.

**Build both backends** (useful for comparison):

```bash
make build-both
```

Produces:
- `mycart` (raw SQL backend)
- `mycart-sqlc` (sqlc backend)

Both binaries are functionally identical — same CLI, same API, same behavior.

## Testing with sqlc

### Test sqlc backend with SQLite

```bash
make test-queries-sqlc-sqlite
```

Runs:
```bash
go test -tags sqlc ./internal/queries_sqlc/... -count=1 -race
```

### Test sqlc backend with PostgreSQL

```bash
make test-queries-sqlc-postgres
```

Requires `TEST_POSTGRES_DSN` environment variable (see [Running tests on PostgreSQL](#running-tests-on-postgresql)).

Runs:
```bash
TEST_DB_DRIVER=postgres go test -tags sqlc ./internal/queries_sqlc/... -count=1 -race
```

### 4-mode test matrix

Test **all combinations** (raw+sqlc × SQLite+PostgreSQL):

```bash
make test-queries-all
```

This runs:
1. `test-queries-raw-sqlite` — raw SQL + SQLite
2. `test-queries-raw-postgres` — raw SQL + PostgreSQL
3. `test-queries-sqlc-sqlite` — sqlc + SQLite
4. `test-queries-sqlc-postgres` — sqlc + PostgreSQL

All four must pass before merging query changes.

## Running tests on PostgreSQL

The PostgreSQL tests need a **dedicated test server** with `CREATEDB`, `CREATEROLE` and `SUPERUSER` privileges. Each test gets its own database, which is dropped after the test passes.

**Never point this at a server with data you care about.**

### Local Docker test server

```bash
docker compose -f docker/docker-compose.yml -f docker/docker-compose_dev.yml up -d pgtestdb
```

This starts `postgres:17-alpine` on port 5433 with:
- tmpfs data directory (nothing persisted)
- durability off (fast tests)
- Credentials: `postgres:password`

Then set the environment variable:

```bash
export TEST_POSTGRES_DSN='postgres://postgres:password@localhost:5433/postgres?sslmode=disable'
make test-queries-all
```

### Remote PostgreSQL (Supabase, RDS, etc.)

**Only use a throwaway test database.** The tests will create and drop databases.

```bash
export TEST_POSTGRES_DSN='postgresql://postgres:your-password@your-host:5432/postgres?sslmode=require'
make test-queries-all
```

**URL-encode special characters** in passwords:
- Space: `%20`
- `@`: `%40`
- `#`: `%23`

Example with space in password:
```bash
# Wrong (will fail to parse):
postgresql://user:my password@host/db

# Correct:
postgresql://user:my%20password@host/db
```

See `.env.example` for more examples.

## Adding new queries

### Step 1: Write annotated SQL

Create or modify a `.sql` file in `db/queries/postgres/` and `db/queries/sqlite/`.

**PostgreSQL example** (`db/queries/postgres/customer.sql`):

```sql
-- name: GetCustomerByEmail :one
SELECT id, email, name, created
FROM customer
WHERE email = $1;

-- name: ListCustomers :many
SELECT id, email, name, created
FROM customer
ORDER BY created DESC
LIMIT $1 OFFSET $2;

-- name: CreateCustomer :one
INSERT INTO customer (id, email, name, created)
VALUES ($1, $2, $3, $4)
RETURNING id, email, name, created;
```

**SQLite example** (`db/queries/sqlite/customer.sql`):

```sql
-- name: GetCustomerByEmail :one
SELECT id, email, name, created
FROM customer
WHERE email = ?;

-- name: ListCustomers :many
SELECT id, email, name, created
FROM customer
ORDER BY created DESC
LIMIT ? OFFSET ?;

-- name: CreateCustomer :one
INSERT INTO customer (id, email, name, created)
VALUES (?, ?, ?, ?)
RETURNING id, email, name, created;
```

**Query annotations:**
- `:one` — expects exactly one row (returns single struct)
- `:many` — expects zero or more rows (returns slice)
- `:exec` — no rows returned (returns `sql.Result`)
- `:execrows` — returns rows affected count

**Parameter placeholders:**
- PostgreSQL: `$1`, `$2`, `$3`, ...
- SQLite: `?`, `?`, `?`, ...

### Step 2: Generate code

```bash
make sqlc-generate
```

sqlc produces:
- `internal/queries_sqlc/sqlc/postgres/customer.sql.go`
- `internal/queries_sqlc/sqlc/sqlite/customer.sql.go`

### Step 3: Expose via backend interface

Edit `internal/queries_sqlc/backend.go`:

```go
type backend interface {
    // Customer methods
    GetCustomerByEmail(ctx context.Context, email string) (*CustomerRow, error)
    ListCustomers(ctx context.Context, limit, offset int) ([]*CustomerRow, error)
    CreateCustomer(ctx context.Context, id, email, name string, created int64) (*CustomerRow, error)
}
```

### Step 4: Implement wrapper methods

The generated code uses sqlc-specific types. Wrap them to match the backend interface:

**PostgreSQL** (`internal/queries_sqlc/backend_postgres.go`):

```go
func (b *postgresBackend) GetCustomerByEmail(ctx context.Context, email string) (*CustomerRow, error) {
    row, err := b.queries.GetCustomerByEmail(ctx, email)
    if err != nil {
        return nil, err
    }
    return &CustomerRow{
        ID:      row.ID,
        Email:   row.Email,
        Name:    row.Name,
        Created: row.Created,
    }, nil
}
```

**SQLite** (`internal/queries_sqlc/backend_sqlite.go`):

```go
func (b *sqliteBackend) GetCustomerByEmail(ctx context.Context, email string) (*CustomerRow, error) {
    row, err := b.queries.GetCustomerByEmail(ctx, email)
    if err != nil {
        return nil, err
    }
    return &CustomerRow{
        ID:      row.ID,
        Email:   row.Email,
        Name:    row.Name,
        Created: row.Created,
    }, nil
}
```

### Step 5: Test

Write tests in `internal/queries_sqlc/*_test.go`:

```go
func TestGetCustomerByEmail(t *testing.T) {
    t.Parallel()
    ctx := context.Background()

    // Arrange
    customer := &CustomerRow{
        ID:      "c1",
        Email:   "test@example.com",
        Name:    "Test User",
        Created: time.Now().Unix(),
    }

    // Act
    created, err := CreateCustomer(ctx, customer.ID, customer.Email, customer.Name, customer.Created)
    require.NoError(t, err)

    retrieved, err := GetCustomerByEmail(ctx, customer.Email)
    require.NoError(t, err)

    // Assert
    assert.Equal(t, customer.ID, retrieved.ID)
    assert.Equal(t, customer.Email, retrieved.Email)
    assert.Equal(t, customer.Name, retrieved.Name)
}
```

Run all 4 modes:

```bash
make test-queries-all
```

## Configuration

sqlc reads `sqlc.yaml` in the repository root:

```yaml
version: "2"
sql:
  - engine: "postgresql"
    schema: "db/schema"            # migrations (CREATE TABLE statements)
    queries: "db/queries/postgres" # annotated SQL queries
    gen:
      go:
        package: "pggen"
        out: "internal/queries_sqlc/sqlc/postgres"
        sql_package: "pgx/v5"     # PostgreSQL driver
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

**Don't modify this** unless adding a new query group or changing generation options.

## Directory structure

```
mycart/
├── sqlc.yaml                          # sqlc configuration
├── db/
│   ├── schema/                        # migrations (CREATE TABLE, ...)
│   └── queries/
│       ├── postgres/*.sql             # PostgreSQL-specific queries
│       └── sqlite/*.sql               # SQLite-specific queries
└── internal/
    ├── queries/                       # raw SQL backend (default)
    │   ├── auth.go
    │   ├── customer.go
    │   └── ...
    └── queries_sqlc/                  # sqlc backend (optional)
        ├── backend.go                 # backend interface
        ├── backend_postgres.go        # PostgreSQL implementation
        ├── backend_sqlite.go          # SQLite implementation
        ├── queries.go                 # public API
        └── sqlc/                      # generated code (DO NOT EDIT)
            ├── postgres/
            │   ├── auth.sql.go
            │   ├── customer.sql.go
            │   └── ...
            └── sqlite/
                ├── auth.sql.go
                ├── customer.sql.go
                └── ...
```

**Never edit files in `internal/queries_sqlc/sqlc/`** — they are regenerated by `make sqlc-generate`.

## Build tags

The sqlc backend is gated behind the `sqlc` build tag:

```go
// +build sqlc

package queries_sqlc
```

This means:
- **Without `-tags sqlc`**: raw SQL backend is used (`internal/queries`)
- **With `-tags sqlc`**: sqlc backend is used (`internal/queries_sqlc`)

The application's `main.go` imports the appropriate backend based on build tags.

## Workflow summary

1. **First time setup:**
   ```bash
   make install-sqlc
   make sqlc-generate
   ```

2. **Daily development (raw SQL backend):**
   ```bash
   go build ./cmd              # default: raw SQL
   go test ./... -count=1 -race
   ```

3. **Using sqlc backend:**
   ```bash
   # After editing db/queries/**/*.sql:
   make sqlc-generate

   # Build and test:
   make build-sqlc
   make test-queries-sqlc-sqlite
   make test-queries-sqlc-postgres

   # Or test all 4 modes:
   make test-queries-all
   ```

4. **Before merging:**
   ```bash
   make test-queries-all  # all 4 modes must pass
   make build-both        # both backends must compile
   ```

## Troubleshooting

| Symptom | Cause | Fix |
|---------|-------|-----|
| `sqlc: command not found` | sqlc not installed | `make install-sqlc` |
| `ERROR: relation "table_name" does not exist` | Schema out of sync with queries | Ensure `db/schema/` migrations are up-to-date |
| `query parameter $1 is missing` | PostgreSQL query uses `?` instead of `$1` | Use `$1, $2, ...` in `db/queries/postgres/*.sql` |
| `query parameter ? is missing` | SQLite query uses `$1` instead of `?` | Use `?` in `db/queries/sqlite/*.sql` |
| `internal/queries_sqlc/sqlc/: directory not found` | Generated code missing | `make sqlc-generate` |
| Tests fail with `undefined: GetCustomerByEmail` | Generated code not compiled with `-tags sqlc` | `go test -tags sqlc ./internal/queries_sqlc/...` |
| `cannot use row (type pggen.GetCustomerByEmailRow) as type CustomerRow` | Missing wrapper method | Implement wrapper in `backend_postgres.go` and `backend_sqlite.go` |
| PostgreSQL test fails with `invalid userinfo` | Space or special character in password | URL-encode the password (space → `%20`, `@` → `%40`) |

## Further reading

- [sqlc documentation](https://docs.sqlc.dev/)
- [sqlc query annotations](https://docs.sqlc.dev/en/latest/reference/query-annotations.html)
- [Using PostgreSQL](./using-postgresql.md) — PostgreSQL setup and testing
- `internal/queries_sqlc/AGENTS.md` — implementation details for contributors
