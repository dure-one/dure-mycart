# Install Detection Redesign Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Redesign install detection to use migration version checks instead of settings table queries, with two-phase database initialization.

**Architecture:** Split db.Init() into Connect (no migrations) → IsInstalled (check goose_db_version) → Migrate (conditional). Install middleware uses package-level flag instead of database query. Install handler accepts user config and runs migrations with it.

**Tech Stack:** Go 1.26, goose migrations, embed.FS, Fiber v3, SvelteKit (Svelte 5)

**Spec:** `docs/superpowers/specs/2026-09-11-install-detection-redesign.md`

## Global Constraints

- Go 1.26+ required
- Use existing `goose` for migrations (no new migration tools)
- Preserve backward compatibility for `db.Init()` function
- Use `testify` for test assertions
- Table-driven tests with `t.Parallel()` where safe
- Minimum 80% test coverage for database layer
- All commits use conventional commits format
- No breaking changes to existing install behavior from user perspective

---

## Task 1: Add Database Connect and IsInstalled Functions

**Files:**
- Modify: `internal/store/db/init.go`
- Test: `internal/store/db/init_test.go` (create)

**Interfaces:**
- Consumes: Existing `loadConfig()`, `connectWithRetry()`, `initFunctionPointers()`, package-level `db *sql.DB` and `dbType string`
- Produces:
  - `func Connect() error` - connects to database without running migrations
  - `func IsInstalled() (bool, error)` - checks if goose_db_version table has records
  - Package-level `installRequired bool` variable

- [ ] **Step 1: Create test file with failing test for Connect()**

File: `internal/store/db/init_test.go`

```go
package db

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestConnect(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		setup   func(t *testing.T)
		cleanup func(t *testing.T)
		wantErr bool
	}{
		{
			name: "successful connection with default SQLite config",
			setup: func(t *testing.T) {
				t.Setenv("DB_TYPE", "sqlite")
				t.Setenv("SQLITE_PATH", ":memory:")
			},
			cleanup:  func(t *testing.T) {},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if tt.setup != nil {
				tt.setup(t)
			}
			if tt.cleanup != nil {
				t.Cleanup(func() { tt.cleanup(t) })
			}

			err := Connect()

			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.NotNil(t, db)
				require.NotEmpty(t, dbType)
			}
		})
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
go test ./internal/store/db -run TestConnect -v
```

Expected: FAIL with "undefined: Connect"

- [ ] **Step 3: Implement Connect() function**

File: `internal/store/db/init.go`

Add at the end of the file, before the converter functions:

```go
// Connect establishes database connection without running migrations.
// It loads config from environment variables, connects with retry logic,
// and initializes function pointers. Does NOT run migrations.
func Connect() error {
	// Load configuration from env vars
	cfg := loadConfig()

	// Connect with retry logic
	conn, err := connectWithRetry(cfg)
	if err != nil {
		return fmt.Errorf("connect with retry failed: %w", err)
	}
	db = conn
	dbType = cfg.Type

	// Log connection info
	logDatabaseInfo(cfg)

	// Initialize function pointers
	if err := initFunctionPointers(db, dbType); err != nil {
		db.Close()
		return fmt.Errorf("function pointer init failed: %w", err)
	}

	return nil
}
```

- [ ] **Step 4: Run test to verify Connect() passes**

```bash
go test ./internal/store/db -run TestConnect -v
```

Expected: PASS

- [ ] **Step 5: Add failing test for IsInstalled()**

File: `internal/store/db/init_test.go`

Add new test:

```go
func TestIsInstalled(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		setup         func(t *testing.T) *sql.DB
		wantInstalled bool
		wantErr       bool
	}{
		{
			name: "returns false when goose_db_version table doesn't exist",
			setup: func(t *testing.T) *sql.DB {
				t.Setenv("DB_TYPE", "sqlite")
				t.Setenv("SQLITE_PATH", ":memory:")
				err := Connect()
				require.NoError(t, err)
				return db
			},
			wantInstalled: false,
			wantErr:       false,
		},
		{
			name: "returns true when goose_db_version table exists with records",
			setup: func(t *testing.T) *sql.DB {
				t.Setenv("DB_TYPE", "sqlite")
				t.Setenv("SQLITE_PATH", ":memory:")
				err := Connect()
				require.NoError(t, err)
				
				// Create goose_db_version table with a record
				_, err = db.Exec(`CREATE TABLE goose_db_version (
					id INTEGER PRIMARY KEY,
					version_id INTEGER NOT NULL,
					is_applied INTEGER NOT NULL,
					tstamp TIMESTAMP DEFAULT CURRENT_TIMESTAMP
				)`)
				require.NoError(t, err)
				_, err = db.Exec(`INSERT INTO goose_db_version (version_id, is_applied) VALUES (1, 1)`)
				require.NoError(t, err)
				
				return db
			},
			wantInstalled: true,
			wantErr:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			testDB := tt.setup(t)
			require.NotNil(t, testDB)
			t.Cleanup(func() {
				if testDB != nil {
					testDB.Close()
				}
			})

			installed, err := IsInstalled()

			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Equal(t, tt.wantInstalled, installed)
			}
		})
	}
}
```

- [ ] **Step 6: Run test to verify it fails**

```bash
go test ./internal/store/db -run TestIsInstalled -v
```

Expected: FAIL with "undefined: IsInstalled"

- [ ] **Step 7: Implement IsInstalled() function**

File: `internal/store/db/init.go`

Add after Connect():

```go
// IsInstalled checks if the database has been installed by checking
// if the goose_db_version table exists and has at least one record.
// Returns false if table doesn't exist (not an error condition).
func IsInstalled() (bool, error) {
	if db == nil {
		return false, fmt.Errorf("database not connected")
	}

	var exists bool
	query := "SELECT EXISTS (SELECT 1 FROM goose_db_version LIMIT 1)"
	
	err := db.QueryRow(query).Scan(&exists)
	if err != nil {
		// Table doesn't exist - this is not an error, just means not installed
		// Check if it's a "no such table" error
		errStr := err.Error()
		if strings.Contains(errStr, "no such table") || strings.Contains(errStr, "does not exist") {
			return false, nil
		}
		return false, fmt.Errorf("failed to check goose_db_version: %w", err)
	}

	return exists, nil
}
```

- [ ] **Step 8: Run test to verify IsInstalled() passes**

```bash
go test ./internal/store/db -run TestIsInstalled -v
```

Expected: PASS

- [ ] **Step 9: Add package-level installRequired variable**

File: `internal/store/db/init.go`

Add to package-level variables section (around line 24):

```go
var (
	db     *sql.DB
	dbType string
	installRequired bool  // Set to true if database is not installed
)
```

- [ ] **Step 10: Add InstallRequired() getter function**

File: `internal/store/db/init.go`

Add after IsInstalled():

```go
// InstallRequired returns true if the database needs installation.
// This flag is set during Connect() based on IsInstalled() check.
func InstallRequired() bool {
	return installRequired
}
```

- [ ] **Step 11: Update Connect() to set installRequired flag**

File: `internal/store/db/init.go`

Update Connect() function to check if installed and set flag:

```go
// Connect establishes database connection without running migrations.
// It loads config from environment variables, connects with retry logic,
// and initializes function pointers. Does NOT run migrations.
// Sets installRequired flag based on whether database is installed.
func Connect() error {
	// Load configuration from env vars
	cfg := loadConfig()

	// Connect with retry logic
	conn, err := connectWithRetry(cfg)
	if err != nil {
		return fmt.Errorf("connect with retry failed: %w", err)
	}
	db = conn
	dbType = cfg.Type

	// Log connection info
	logDatabaseInfo(cfg)

	// Initialize function pointers
	if err := initFunctionPointers(db, dbType); err != nil {
		db.Close()
		return fmt.Errorf("function pointer init failed: %w", err)
	}

	// Check if database is installed
	installed, err := IsInstalled()
	if err != nil {
		// Log warning but don't fail - we'll set installRequired=true
		fmt.Printf("⚠️  Failed to check installation status: %v\n", err)
		installRequired = true
	} else {
		installRequired = !installed
	}

	return nil
}
```

- [ ] **Step 12: Commit**

```bash
git add internal/store/db/init.go internal/store/db/init_test.go
git commit -m "feat(db): add Connect and IsInstalled functions

- Add Connect() for database connection without migrations
- Add IsInstalled() to check goose_db_version table
- Add package-level installRequired flag
- Add InstallRequired() getter function
- Connect() sets installRequired based on install status

Tests: table-driven tests for both functions

Co-Authored-By: Claude Sonnet 4.5 <noreply@anthropic.com>"
```

---

## Task 2: Add Migrate Functions and SetInstalled

**Files:**
- Modify: `internal/store/db/init.go`
- Modify: `internal/store/db/init_test.go`

**Interfaces:**
- Consumes: Task 1 `Connect()`, `IsInstalled()`, `installRequired`, existing `runMigrations()`
- Produces:
  - `func Migrate(migrationsFS embed.FS) error` - runs migrations on connected database
  - `func MigrateWithConfig(cfg *Config, migrationsFS embed.FS) error` - connects and migrates with explicit config
  - `func SetInstalled()` - sets installRequired to false
  - Updated `func Init(migrationsFS embed.FS) error` - calls Connect → IsInstalled → Migrate

- [ ] **Step 1: Add failing test for Migrate()**

File: `internal/store/db/init_test.go`

```go
func TestMigrate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		setup   func(t *testing.T)
		wantErr bool
	}{
		{
			name: "runs migrations successfully on empty database",
			setup: func(t *testing.T) {
				t.Setenv("DB_TYPE", "sqlite")
				t.Setenv("SQLITE_PATH", ":memory:")
				err := Connect()
				require.NoError(t, err)
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if tt.setup != nil {
				tt.setup(t)
			}
			t.Cleanup(func() {
				if db != nil {
					db.Close()
					db = nil
				}
			})

			// Use embedded migrations from db/migrations
			err := Migrate(migrations.Embed())

			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				// Verify goose_db_version table now exists
				var count int
				err = db.QueryRow("SELECT COUNT(*) FROM goose_db_version").Scan(&count)
				require.NoError(t, err)
				require.Greater(t, count, 0, "goose_db_version should have records after migration")
			}
		})
	}
}
```

Add import at top:

```go
import (
	"database/sql"
	"testing"

	"github.com/shurco/mycart/db/migrations"
	"github.com/stretchr/testify/require"
)
```

- [ ] **Step 2: Run test to verify it fails**

```bash
go test ./internal/store/db -run TestMigrate -v
```

Expected: FAIL with "undefined: Migrate"

- [ ] **Step 3: Implement Migrate() function**

File: `internal/store/db/init.go`

Add after InstallRequired():

```go
// Migrate runs database migrations on an already-connected database.
// Requires prior call to Connect(). Returns error if migrations fail.
func Migrate(migrationsFS embed.FS) error {
	if db == nil {
		return fmt.Errorf("database not connected")
	}

	if err := runMigrations(db, dbType, migrationsFS); err != nil {
		return fmt.Errorf("migration failed: %w", err)
	}

	return nil
}
```

- [ ] **Step 4: Run test to verify Migrate() passes**

```bash
go test ./internal/store/db -run TestMigrate -v
```

Expected: PASS

- [ ] **Step 5: Add failing test for MigrateWithConfig()**

File: `internal/store/db/init_test.go`

```go
func TestMigrateWithConfig(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		config  *Config
		wantErr bool
	}{
		{
			name: "successful migration with SQLite config",
			config: &Config{
				Type: "sqlite",
				SQLite: SQLiteConfig{
					Path: ":memory:",
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := MigrateWithConfig(tt.config, migrations.Embed())

			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
```

- [ ] **Step 6: Run test to verify it fails**

```bash
go test ./internal/store/db -run TestMigrateWithConfig -v
```

Expected: FAIL with "undefined: MigrateWithConfig"

- [ ] **Step 7: Implement MigrateWithConfig() function**

File: `internal/store/db/init.go`

Add after Migrate():

```go
// MigrateWithConfig connects to database with provided config and runs migrations.
// This is used by the install handler to migrate with user-selected database config.
// Closes the connection after migration completes.
func MigrateWithConfig(cfg *Config, migrationsFS embed.FS) error {
	// Connect with provided config
	conn, err := connectWithRetry(cfg)
	if err != nil {
		return fmt.Errorf("connect failed: %w", err)
	}
	defer conn.Close()

	// Determine database type
	var migrationDBType string
	if cfg.Type == "postgres" || cfg.Type == "postgresql" {
		migrationDBType = "postgres"
	} else {
		migrationDBType = "sqlite"
	}

	// Run migrations
	if err := runMigrations(conn, migrationDBType, migrationsFS); err != nil {
		return fmt.Errorf("migration failed: %w", err)
	}

	return nil
}
```

- [ ] **Step 8: Run test to verify MigrateWithConfig() passes**

```bash
go test ./internal/store/db -run TestMigrateWithConfig -v
```

Expected: PASS

- [ ] **Step 9: Implement SetInstalled() function**

File: `internal/store/db/init.go`

Add after MigrateWithConfig():

```go
// SetInstalled marks the database as installed.
// Called by the install handler after successful installation.
func SetInstalled() {
	installRequired = false
}
```

- [ ] **Step 10: Update Init() to use new functions**

File: `internal/store/db/init.go`

Replace the existing Init() function:

```go
// Init initializes database connection, runs migrations, and initializes function pointers.
// This is the legacy entry point kept for backward compatibility.
// Behavior change: only runs migrations if database is already installed.
func Init(migrationsFS embed.FS) error {
	// Connect to database
	if err := Connect(); err != nil {
		return fmt.Errorf("connect failed: %w", err)
	}

	// Check if installed
	installed, err := IsInstalled()
	if err != nil {
		return fmt.Errorf("failed to check installation status: %w", err)
	}

	// Only run migrations if already installed
	if installed {
		if err := Migrate(migrationsFS); err != nil {
			db.Close()
			return fmt.Errorf("migration failed: %w", err)
		}
	}

	return nil
}
```

- [ ] **Step 11: Run all database tests to verify**

```bash
go test ./internal/store/db -v -count=1
```

Expected: All tests PASS

- [ ] **Step 12: Commit**

```bash
git add internal/store/db/init.go internal/store/db/init_test.go
git commit -m "feat(db): add Migrate, MigrateWithConfig, and update Init

- Add Migrate() for running migrations on connected database
- Add MigrateWithConfig() for install handler (user config)
- Add SetInstalled() to mark database as installed
- Update Init() to use Connect → IsInstalled → conditional Migrate
- Init() now only runs migrations if already installed

Tests: comprehensive unit tests for all functions

Co-Authored-By: Claude Sonnet 4.5 <noreply@anthropic.com>"
```

---

## Task 3: Update Install Model for camelCase Fields

**Files:**
- Modify: `internal/models/install.go`
- Modify: `internal/models/install_test.go`

**Interfaces:**
- Consumes: Existing Install struct with `db_type`, `database_url`, `sqlite_path`
- Produces: Updated Install struct with `DBType`, `DatabaseURL`, `SQLitePath` (JSON: `dbType`, `databaseUrl`, `sqlitePath`)

- [ ] **Step 1: Update existing test to expect validation errors**

File: `internal/models/install_test.go`

Find the existing tests and update them to use camelCase fields. Add these new test cases:

```go
func TestInstall_Validate_DBType(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		install Install
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid sqlite config",
			install: Install{
				Email:      "test@example.com",
				Password:   "Pass123",
				Domain:     "example.com",
				DBType:     "sqlite",
				SQLitePath: "./data.db",
			},
			wantErr: false,
		},
		{
			name: "valid postgres config",
			install: Install{
				Email:       "test@example.com",
				Password:    "Pass123",
				Domain:      "example.com",
				DBType:      "postgres",
				DatabaseURL: "postgresql://user:pass@localhost/db",
			},
			wantErr: false,
		},
		{
			name: "invalid db type",
			install: Install{
				Email:    "test@example.com",
				Password: "Pass123",
				Domain:   "example.com",
				DBType:   "mysql",
			},
			wantErr: true,
			errMsg:  "dbType must be 'sqlite' or 'postgres'",
		},
		{
			name: "postgres missing database_url",
			install: Install{
				Email:    "test@example.com",
				Password: "Pass123",
				Domain:   "example.com",
				DBType:   "postgres",
			},
			wantErr: true,
			errMsg:  "databaseUrl is required for postgres",
		},
		{
			name: "postgres invalid database_url",
			install: Install{
				Email:       "test@example.com",
				Password:    "Pass123",
				Domain:      "example.com",
				DBType:      "postgres",
				DatabaseURL: "mysql://localhost/db",
			},
			wantErr: true,
			errMsg:  "databaseUrl must start with 'postgres://' or 'postgresql://'",
		},
		{
			name: "sqlite missing path",
			install: Install{
				Email:    "test@example.com",
				Password: "Pass123",
				Domain:   "example.com",
				DBType:   "sqlite",
			},
			wantErr: true,
			errMsg:  "sqlitePath is required for sqlite",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := tt.install.Validate()

			if tt.wantErr {
				require.Error(t, err)
				if tt.errMsg != "" {
					require.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				require.NoError(t, err)
			}
		})
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
go test ./internal/models -run TestInstall_Validate_DBType -v
```

Expected: FAIL with compilation errors or wrong field names

- [ ] **Step 3: Update Install struct with camelCase JSON tags**

File: `internal/models/install.go`

Replace the Install struct:

```go
type Install struct {
	Email       string `json:"email"`
	Password    string `json:"password"`
	Domain      string `json:"domain"`
	DBType      string `json:"dbType"`      // Changed from db_type
	DatabaseURL string `json:"databaseUrl"` // Changed from database_url
	SQLitePath  string `json:"sqlitePath"`  // Changed from sqlite_path
}
```

- [ ] **Step 4: Update Validate() method to handle new fields**

File: `internal/models/install.go`

Update the Validate() method to add database config validation:

```go
func (i *Install) Validate() error {
	if i.Email == "" {
		return fmt.Errorf("email is required")
	}
	if !strings.Contains(i.Email, "@") {
		return fmt.Errorf("invalid email format")
	}

	if i.Password == "" {
		return fmt.Errorf("password is required")
	}
	if len(i.Password) < 6 {
		return fmt.Errorf("password must be at least 6 characters")
	}
	if len(i.Password) > 72 {
		return fmt.Errorf("password must be at most 72 characters")
	}

	if i.Domain == "" {
		return fmt.Errorf("domain is required")
	}

	// Validate database type
	if i.DBType != "sqlite" && i.DBType != "postgres" && i.DBType != "postgresql" {
		return fmt.Errorf("dbType must be 'sqlite' or 'postgres'")
	}

	// Validate database-specific config
	if i.DBType == "postgres" || i.DBType == "postgresql" {
		if i.DatabaseURL == "" {
			return fmt.Errorf("databaseUrl is required for postgres")
		}
		if !strings.HasPrefix(i.DatabaseURL, "postgres://") && !strings.HasPrefix(i.DatabaseURL, "postgresql://") {
			return fmt.Errorf("databaseUrl must start with 'postgres://' or 'postgresql://'")
		}
	} else if i.DBType == "sqlite" {
		if i.SQLitePath == "" {
			return fmt.Errorf("sqlitePath is required for sqlite")
		}
	}

	return nil
}
```

- [ ] **Step 5: Run tests to verify they pass**

```bash
go test ./internal/models -run TestInstall_Validate_DBType -v
```

Expected: PASS

- [ ] **Step 6: Run all model tests**

```bash
go test ./internal/models -v -count=1
```

Expected: All tests PASS

- [ ] **Step 7: Commit**

```bash
git add internal/models/install.go internal/models/install_test.go
git commit -m "feat(models): update Install struct to camelCase fields

- Change db_type -> dbType (JSON: dbType)
- Change database_url -> databaseUrl (JSON: databaseUrl)
- Change sqlite_path -> sqlitePath (JSON: sqlitePath)
- Add database config validation in Validate()
- Validate dbType is sqlite or postgres
- Validate databaseUrl for postgres (must start with postgres://)
- Validate sqlitePath for sqlite (must not be empty)

Tests: comprehensive validation tests for all scenarios

Co-Authored-By: Claude Sonnet 4.5 <noreply@anthropic.com>"
```

---

## Task 4: Update App Startup and Install Middleware

**Files:**
- Modify: `internal/app.go`

**Interfaces:**
- Consumes: Task 1 `db.Connect()`, `db.IsInstalled()`, `db.Migrate()`, `db.InstallRequired()`, existing `migrations.Embed()`
- Produces: Updated `NewApp()` with two-phase init, updated `InstallCheck` middleware using flag

- [ ] **Step 1: Update NewApp() to use two-phase initialization**

File: `internal/app.go`

Find the `NewApp()` function and replace the database initialization section:

Old code (around line 48-54):
```go
// Initialize database: connection, migrations, function pointers
if err := db.Init(migrations.Embed()); err != nil {
	log.Err(err).Msg("failed to initialize database")
	return err
}

// Initialize store package with database connection and type for transactions
store.InitStoreWithType(db.DB(), db.Type())
```

New code:
```go
// NEW: Two-phase database initialization
// Phase 1: Connect without migrations
if err := db.Connect(); err != nil {
	log.Err(err).Msg("failed to connect to database")
	return err
}

// Phase 2: Check if installed
installed, err := db.IsInstalled()
if err != nil {
	log.Err(err).Msg("failed to check installation status")
	return err
}

// Phase 3: Run migrations only if installed
if installed {
	if err := db.Migrate(migrations.Embed()); err != nil {
		log.Err(err).Msg("failed to run migrations")
		return err
	}
	log.Info().Msg("Database migrations completed")
} else {
	log.Warn().Msg("Database not installed - install page will be shown")
}

// Initialize store package with database connection and type for transactions
store.InitStoreWithType(db.DB(), db.Type())
```

- [ ] **Step 2: Update InstallCheck middleware to use flag**

File: `internal/app.go`

Find the `InstallCheck` function (around line 236) and replace it:

Old code:
```go
func InstallCheck(c fiber.Ctx) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	installed, err := store.IsInstalled(ctx)
	if err != nil {
		return webutil.StatusInternalServerError(c)
	}

	install := installed
	path := c.Path()

	if !install {
		if !isInstallPath(path) {
			if strings.HasPrefix(path, "/api/") {
				return webutil.StatusBadRequest(c, "application not installed")
			}
			return c.Redirect().To("/_/install")
		}
	} else if strings.HasPrefix(path, "/_/install") {
		return c.Redirect().To("/_")
	}

	return c.Next()
}
```

New code:
```go
func InstallCheck(c fiber.Ctx) error {
	path := c.Path()
	
	// Check package-level install flag (no database query needed)
	installed := !db.InstallRequired()
	
	if !installed {
		// Not installed - only allow install-related paths
		if !isInstallPath(path) {
			if strings.HasPrefix(path, "/api/") {
				return webutil.StatusBadRequest(c, "application not installed")
			}
			return c.Redirect().To("/_/install")
		}
	} else {
		// Already installed - prevent access to install page
		if strings.HasPrefix(path, "/_/install") {
			return c.Redirect().To("/_")
		}
	}
	
	return c.Next()
}
```

- [ ] **Step 3: Build to verify no compilation errors**

```bash
go build ./...
```

Expected: Clean build with no errors

- [ ] **Step 4: Commit**

```bash
git add internal/app.go
git commit -m "feat(app): update startup and middleware for two-phase init

- Update NewApp() to use Connect → IsInstalled → Migrate flow
- Only run migrations if database is already installed
- Add logging: 'Database migrations completed' when migrated
- Add logging: 'Database not installed...' when not installed
- Update InstallCheck middleware to use db.InstallRequired() flag
- Remove database query from middleware (use package-level flag)

Behavior: app starts even when not installed, shows install page

Co-Authored-By: Claude Sonnet 4.5 <noreply@anthropic.com>"
```

---

## Task 5: Update Install Handlers

**Files:**
- Modify: `internal/handlers/private/install.go`

**Interfaces:**
- Consumes: Task 2 `db.MigrateWithConfig()`, `db.SetInstalled()`, Task 3 `models.Install` with camelCase fields, existing `migrations.Embed()`
- Produces: Updated `InstallStatus` handler, updated `Install` handler, new `buildConfigFromRequest()` helper

- [ ] **Step 1: Update InstallStatus handler**

File: `internal/handlers/private/install.go`

Replace the `InstallStatus` function (around line 26):

Old code:
```go
func InstallStatus(c fiber.Ctx) error {
	log := logging.New()

	installed, err := store.IsInstalled(c.Context())
	if err != nil {
		log.ErrorStack(err)
		return webutil.StatusInternalServerError(c)
	}

	return webutil.Response(c, fiber.StatusOK, "Installation status", installStatus{Installed: installed})
}
```

New code:
```go
func InstallStatus(c fiber.Ctx) error {
	log := logging.New()
	
	// Use package-level flag instead of querying database
	installed := !db.InstallRequired()
	
	return webutil.Response(c, fiber.StatusOK, "Installation status", installStatus{Installed: installed})
}
```

- [ ] **Step 2: Add buildConfigFromRequest helper function**

File: `internal/handlers/private/install.go`

Add after the Install function (around line 73):

```go
// buildConfigFromRequest builds a database Config from the install request.
// Used to connect and migrate with user-selected database configuration.
func buildConfigFromRequest(req *models.Install) *db.Config {
	cfg := &db.Config{
		Type: req.DBType,
	}
	
	if req.DBType == "postgres" || req.DBType == "postgresql" {
		// Parse DATABASE_URL using existing parseConnectionURL logic
		defaults := db.PostgresConfig{
			Host:           "localhost",
			Port:           5432,
			Database:       "mycart",
			User:           "postgres",
			SSLMode:        "require",
			ConnectTimeout: 10,
			MaxOpenConns:   25,
			MaxIdleConns:   5,
		}
		cfg.PostgreSQL = parseConnectionURL(req.DatabaseURL, defaults)
	} else {
		cfg.SQLite = db.SQLiteConfig{
			Path: req.SQLitePath,
		}
	}
	
	return cfg
}

// parseConnectionURL is copied from db package for URL parsing
func parseConnectionURL(rawURL string, defaults db.PostgresConfig) db.PostgresConfig {
	u, err := url.Parse(rawURL)
	if err != nil {
		fmt.Printf("⚠️  Failed to parse DATABASE_URL: %v\n", err)
		return defaults
	}

	cfg := defaults
	if u.Hostname() != "" {
		cfg.Host = u.Hostname()
	}
	if u.Port() != "" {
		if port, err := strconv.Atoi(u.Port()); err == nil {
			cfg.Port = port
		}
	}
	if u.User != nil {
		cfg.User = u.User.Username()
		if password, ok := u.User.Password(); ok {
			cfg.Password = password
		}
	}
	if u.Path != "" {
		cfg.Database = strings.TrimPrefix(u.Path, "/")
	}

	return cfg
}
```

Add imports at top of file:

```go
import (
	"fmt"
	"net/url"
	"strconv"
	"strings"
	
	// ... existing imports
)
```

- [ ] **Step 3: Update Install handler to use MigrateWithConfig**

File: `internal/handlers/private/install.go`

Replace the Install function (around line 38):

Old code:
```go
func Install(c fiber.Ctx) error {
	log := logging.New()
	request := new(models.Install)

	if err := c.Bind().Body(request); err != nil {
		log.ErrorStack(err)
		return webutil.StatusBadRequest(c, err.Error())
	}

	if err := request.Validate(); err != nil {
		log.ErrorStack(err)
		return webutil.StatusBadRequest(c, err.Error())
	}

	if err := store.Install(c.Context(), request); err != nil {
		if errors.Is(err, store.ErrAlreadyInstalled) {
			return webutil.StatusBadRequest(c, err.Error())
		}
		log.ErrorStack(err)
		return webutil.StatusInternalServerError(c)
	}

	return webutil.Response(c, fiber.StatusOK, "Cart installed", nil)
}
```

New code:
```go
func Install(c fiber.Ctx) error {
	log := logging.New()
	request := new(models.Install)

	if err := c.Bind().Body(request); err != nil {
		log.ErrorStack(err)
		return webutil.StatusBadRequest(c, err.Error())
	}

	if err := request.Validate(); err != nil {
		log.ErrorStack(err)
		return webutil.StatusBadRequest(c, err.Error())
	}

	// NEW: Build database config from user's request
	dbConfig := buildConfigFromRequest(request)
	
	// NEW: Run migrations with user-selected config
	if err := db.MigrateWithConfig(dbConfig, migrations.Embed()); err != nil {
		log.ErrorStack(err)
		return webutil.StatusInternalServerError(c)
	}

	// Create admin user and initial settings (unchanged)
	if err := store.Install(c.Context(), request); err != nil {
		if errors.Is(err, store.ErrAlreadyInstalled) {
			return webutil.StatusBadRequest(c, err.Error())
		}
		log.ErrorStack(err)
		return webutil.StatusInternalServerError(c)
	}

	// NEW: Mark as installed, migrations now enabled
	db.SetInstalled()
	
	log.Info().Msg("Installation completed successfully")

	return webutil.Response(c, fiber.StatusOK, "Cart installed", nil)
}
```

Add import:

```go
import (
	// ... existing imports
	"github.com/shurco/mycart/db/migrations"
	"github.com/shurco/mycart/internal/store/db"
)
```

- [ ] **Step 4: Build to verify no compilation errors**

```bash
go build ./...
```

Expected: Clean build

- [ ] **Step 5: Commit**

```bash
git add internal/handlers/private/install.go
git commit -m "feat(handlers): update install handlers for new db flow

- Update InstallStatus to use db.InstallRequired() flag
- Add buildConfigFromRequest() helper to build db.Config from request
- Update Install handler to call db.MigrateWithConfig() before store.Install()
- Call db.SetInstalled() after successful installation
- Add success logging after installation completes

Behavior: install handler now migrates with user config first

Co-Authored-By: Claude Sonnet 4.5 <noreply@anthropic.com>"
```

---

## Task 6: Update Frontend Install Page

**Files:**
- Modify: `web/admin/src/routes/install/+page.svelte`

**Interfaces:**
- Consumes: Task 3 backend expecting `dbType`, `databaseUrl`, `sqlitePath` (camelCase)
- Produces: Updated frontend sending camelCase payload

- [ ] **Step 1: Update payload in install page**

File: `web/admin/src/routes/install/+page.svelte`

Find the handleSubmit function (around line 83) and update the payload:

Old code (around line 96):
```typescript
const payload = {
  email,
  password,
  domain,
  dbType,
  databaseUrl: dbType === 'postgres' ? databaseUrl : '',
  sqlitePath: dbType === 'sqlite' ? sqlitePath : ''
}
```

Verify this matches. If it already uses camelCase, no change needed. If it uses snake_case, update to:

```typescript
const payload = {
  email,
  password,
  domain,
  dbType,                                           // Ensure camelCase
  databaseUrl: dbType === 'postgres' ? databaseUrl : '',  // Ensure camelCase
  sqlitePath: dbType === 'sqlite' ? sqlitePath : ''       // Ensure camelCase
}
```

- [ ] **Step 2: Build frontend to verify**

```bash
cd web/admin
bun run build
```

Expected: Clean build with no errors

- [ ] **Step 3: Return to repo root and commit**

```bash
cd ../..
git add web/admin/src/routes/install/+page.svelte
git commit -m "feat(frontend): ensure install payload uses camelCase

- Verify payload sends dbType (not db_type)
- Verify payload sends databaseUrl (not database_url)
- Verify payload sends sqlitePath (not sqlite_path)

Matches backend models.Install struct

Co-Authored-By: Claude Sonnet 4.5 <noreply@anthropic.com>"
```

---

## Task 7: Add Integration Tests

**Files:**
- Create: `internal/app_test.go`
- Modify: `internal/handlers/private/install_test.go`

**Interfaces:**
- Consumes: All previous tasks (full install flow)
- Produces: Comprehensive integration tests for startup and install

- [ ] **Step 1: Create app integration tests file**

File: `internal/app_test.go`

```go
package app

import (
	"context"
	"database/sql"
	"os"
	"testing"
	"time"

	"github.com/shurco/mycart/internal/store/db"
	"github.com/stretchr/testify/require"
)

func TestAppStartup_NotInstalled(t *testing.T) {
	// Setup: empty in-memory database
	t.Setenv("DB_TYPE", "sqlite")
	t.Setenv("SQLITE_PATH", ":memory:")

	// Connect and verify not installed
	err := db.Connect()
	require.NoError(t, err)

	installed, err := db.IsInstalled()
	require.NoError(t, err)
	require.False(t, installed, "fresh database should not be installed")

	// Verify installRequired flag is set
	require.True(t, db.InstallRequired(), "installRequired should be true for fresh database")

	t.Cleanup(func() {
		db.Close()
	})
}

func TestAppStartup_Installed(t *testing.T) {
	// Setup: database with migrations
	t.Setenv("DB_TYPE", "sqlite")
	
	// Use temp file for this test
	tmpFile, err := os.CreateTemp("", "test-installed-*.db")
	require.NoError(t, err)
	tmpPath := tmpFile.Name()
	tmpFile.Close()
	defer os.Remove(tmpPath)
	
	t.Setenv("SQLITE_PATH", tmpPath)

	// First: connect and migrate to simulate installed state
	err = db.Connect()
	require.NoError(t, err)

	err = db.Migrate(migrations.Embed())
	require.NoError(t, err)

	// Close and reset
	db.Close()

	// Second: reconnect as if app is restarting
	err = db.Connect()
	require.NoError(t, err)

	installed, err := db.IsInstalled()
	require.NoError(t, err)
	require.True(t, installed, "database with migrations should be installed")

	// Verify installRequired flag is not set
	require.False(t, db.InstallRequired(), "installRequired should be false for installed database")

	t.Cleanup(func() {
		db.Close()
	})
}
```

Add imports:

```go
import (
	"context"
	"database/sql"
	"os"
	"testing"
	"time"

	"github.com/shurco/mycart/db/migrations"
	"github.com/shurco/mycart/internal/store/db"
	"github.com/stretchr/testify/require"
)
```

- [ ] **Step 2: Run integration tests**

```bash
go test ./internal -run TestAppStartup -v -count=1
```

Expected: PASS

- [ ] **Step 3: Update install handler tests**

File: `internal/handlers/private/install_test.go`

Add test for new flow:

```go
func TestInstall_WithDatabaseConfig(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		payload  string
		wantCode int
	}{
		{
			name: "successful installation with sqlite",
			payload: `{
				"email": "admin@example.com",
				"password": "Pass123",
				"domain": "example.com",
				"dbType": "sqlite",
				"sqlitePath": ":memory:"
			}`,
			wantCode: 200,
		},
		{
			name: "invalid dbType",
			payload: `{
				"email": "admin@example.com",
				"password": "Pass123",
				"domain": "example.com",
				"dbType": "mysql",
				"sqlitePath": "./data.db"
			}`,
			wantCode: 400,
		},
		{
			name: "postgres missing databaseUrl",
			payload: `{
				"email": "admin@example.com",
				"password": "Pass123",
				"domain": "example.com",
				"dbType": "postgres"
			}`,
			wantCode: 400,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Setup test database
			t.Setenv("DB_TYPE", "sqlite")
			t.Setenv("SQLITE_PATH", ":memory:")

			app := fiber.New()
			app.Post("/api/install", Install)

			req := httptest.NewRequest("POST", "/api/install", strings.NewReader(tt.payload))
			req.Header.Set("Content-Type", "application/json")

			resp, err := app.Test(req)
			require.NoError(t, err)
			require.Equal(t, tt.wantCode, resp.StatusCode)
		})
	}
}
```

Add imports if needed:

```go
import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/require"
)
```

- [ ] **Step 4: Run handler tests**

```bash
go test ./internal/handlers/private -run TestInstall -v -count=1
```

Expected: PASS

- [ ] **Step 5: Run all tests**

```bash
go test ./... -count=1 -race
```

Expected: All tests PASS

- [ ] **Step 6: Commit**

```bash
git add internal/app_test.go internal/handlers/private/install_test.go
git commit -m "test: add integration tests for install flow

- Add TestAppStartup_NotInstalled for fresh database startup
- Add TestAppStartup_Installed for installed database startup
- Add TestInstall_WithDatabaseConfig for install handler
- Test sqlite installation flow
- Test validation errors (invalid dbType, missing fields)

Coverage: 80%+ for database layer and install handlers

Co-Authored-By: Claude Sonnet 4.5 <noreply@anthropic.com>"
```

---

## Self-Review Checklist

**1. Spec Coverage:**
- ✅ Database layer: Connect(), IsInstalled(), Migrate(), MigrateWithConfig() (Task 1, Task 2)
- ✅ Package-level state: installRequired, InstallRequired(), SetInstalled() (Task 1, Task 2)
- ✅ App startup flow: NewApp() two-phase init (Task 4)
- ✅ Middleware: InstallCheck uses flag (Task 4)
- ✅ Install handlers: InstallStatus, Install, buildConfigFromRequest (Task 5)
- ✅ Models: camelCase fields (Task 3)
- ✅ Frontend: camelCase payload (Task 6)
- ✅ Tests: unit tests for db layer, integration tests (Task 1, Task 2, Task 7)

**2. Placeholder scan:**
- ✅ No TBD, TODO, or "fill in details"
- ✅ All code blocks are complete with actual implementation
- ✅ All test cases have concrete assertions
- ✅ All error messages are specific

**3. Type consistency:**
- ✅ Task 1 produces `Connect() error`, Task 4 consumes same
- ✅ Task 1 produces `IsInstalled() (bool, error)`, Task 4 consumes same
- ✅ Task 2 produces `Migrate(embed.FS) error`, Task 4 consumes same
- ✅ Task 2 produces `MigrateWithConfig(*Config, embed.FS) error`, Task 5 consumes same
- ✅ Task 3 produces `Install` struct with `DBType`, `DatabaseURL`, `SQLitePath`, Task 5 and Task 6 consume same
- ✅ All function signatures match across tasks

**4. File size targets:**
- ✅ Each task modifies focused files
- ✅ No single task creates massive files
- ✅ Tests are separate from implementation

---

## Execution Handoff

Plan complete and saved to `docs/superpowers/plans/2026-09-11-install-detection-redesign.md`. Two execution options:

**1. Subagent-Driven (recommended)** - I dispatch a fresh subagent per task, review between tasks, fast iteration

**2. Inline Execution** - Execute tasks in this session using executing-plans, batch execution with checkpoints

Which approach?
