# Install Detection Redesign

**Date:** 2026-09-11  
**Status:** Design Approved  
**Complexity:** Medium

## Summary

Redesign install detection to use migration version checks instead of settings table queries. Environment variables become the source of truth for database configuration, and migrations only run automatically on already-installed platforms. First-time installation is explicit via the install page.

## Requirements

1. **Environment variable loading:** `.env` files only load in Docker automatically; shell env vars determine database selection with sqlite as default
2. **Install check logic:** Check if `goose_db_version` table exists to determine if installed
3. **Install page UX:** Show hardcoded defaults (user must set env vars to match after install)
4. **Migration behavior:** Auto-run migrations only on installed platforms; show install page and accept user config on first install

## Architecture Overview

### Current State

- `db.Init()` always runs migrations
- `store.IsInstalled()` queries `settings.installed = "true"`
- `InstallCheck` middleware queries database on every request
- Install page has hardcoded defaults

### New State

**App Startup Flow:**
1. Load env vars (`DB_TYPE`, `SQLITE_PATH`, `DATABASE_URL`) with defaults
2. `db.Connect()` → establish connection, NO migrations
3. `db.IsInstalled()` → check if `goose_db_version` table exists
4. IF installed:
   - `db.Migrate()` → run `goose.Up()` for new migrations
   - Continue to normal app flow
5. IF NOT installed:
   - Skip migrations
   - Set global flag: `installRequired = true`
   - Start HTTP server (InstallCheck middleware active)

**Install Page Flow:**
1. User visits any route → InstallCheck middleware
2. IF `installRequired == true` → redirect to `/_/install`
3. Install page shows hardcoded defaults (sqlite, ./lc_base/data.db)
4. User selects database config (may match or differ from env vars)
5. POST `/api/install` with user's config
6. Install handler:
   - Connect using user's submitted config
   - Run `db.Migrate()` with user config
   - Create admin user + initial settings
   - Set `installRequired = false`
7. Redirect to signin

**Subsequent Boots:**
1. Load env vars (must match installed database)
2. `db.Connect()` → using env vars
3. `db.IsInstalled()` → returns true (`goose_db_version` exists)
4. `db.Migrate()` → apply any new migrations
5. Continue normally

### Key Architectural Changes

- `db.Init()` splits into `db.Connect()` + `db.IsInstalled()` + `db.Migrate()`
- `InstallCheck` middleware checks `installRequired` flag instead of querying settings table
- Install handler accepts user config and runs migrations with it (not env vars)
- Env vars remain source of truth for all boots after initial install

## Database Layer Changes

**File:** `internal/store/db/init.go`

### New Functions

**1. `db.Connect() error`**
- Loads config from env vars (existing `loadConfig()`)
- Connects to database with retry logic (existing `connectWithRetry()`)
- Initializes function pointers (existing `initFunctionPointers()`)
- **Does NOT run migrations**
- Sets package-level `db` and `dbType` variables
- Returns error if connection fails

**2. `db.IsInstalled() (bool, error)`**
- Requires prior call to `db.Connect()`
- Queries database: `SELECT EXISTS (SELECT 1 FROM goose_db_version LIMIT 1)`
- Returns `true` if table exists, `false` if not
- Returns error if query fails (table doesn't exist = not an error, returns false)
- Works for both SQLite and PostgreSQL

**3. `db.Migrate(migrationsFS embed.FS) error`**
- Requires prior call to `db.Connect()`
- Runs existing `runMigrations(db, dbType, migrationsFS)`
- Returns error if migrations fail

**4. `db.MigrateWithConfig(cfg *Config, migrationsFS embed.FS) error`**
- **New:** Accepts explicit config (from install form)
- Connects using provided config (may differ from env vars)
- Runs migrations
- Closes connection after migration
- Used only by install handler
- Returns error if connection or migration fails

### Modified Functions

**5. `db.Init(migrationsFS embed.FS) error`** *(kept for backward compatibility)*
- Now calls: `Connect()` → `IsInstalled()` → `Migrate(migrationsFS)` if installed
- Returns error if any step fails
- **Behavior change:** Only runs migrations if installed

### Package-Level State

```go
var (
    db     *sql.DB
    dbType string
    installRequired bool  // NEW: set by Connect() after IsInstalled() check
)

// NEW: Getter for install status
func InstallRequired() bool {
    return installRequired
}

// NEW: Setter called after successful install
func SetInstalled() {
    installRequired = false
}
```

## Application Startup Flow

**File:** `internal/app.go`

### Changes to `NewApp()` Function

```go
func NewApp(httpAddr, httpsAddr string, noSite, appDev bool) error {
    DevMode = appDev
    log = logging.New()

    schema, mainAddr := determineSchemaAndAddr(httpAddr, httpsAddr)

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

    // Initialize store package (unchanged)
    store.InitStoreWithType(db.DB(), db.Type())

    app, err := setupFiberApp(noSite)
    if err != nil {
        return err
    }

    // Initialize app directories (unchanged)
    if err := Init(); err != nil {
        log.Err(err).Send()
        os.Exit(1)
    }

    setupRoutes(app, noSite)
    printStartupInfo(schema, mainAddr, noSite)

    if schema == "https" {
        return startHTTPS(app, mainAddr, httpsAddr)
    }

    return startHTTP(mainAddr, app)
}
```

### Key Behaviors

1. **Connection failure** → app fails to start (current behavior preserved)
2. **Not installed** → app starts, but install page will show for all routes
3. **Installed** → migrations run automatically, app continues normally
4. **Migration failure** (when installed) → app fails to start (fail-fast on migration errors)

### Logging Changes

- Add log line: "Database not installed - install page will be shown"
- Add log line: "Database migrations completed" (when migrations run)

## Install Middleware Changes

**File:** `internal/app.go`

### New `InstallCheck` Middleware

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

### Key Changes

1. **No database query** - uses `db.InstallRequired()` flag set during startup
2. **Faster** - simple boolean check vs SQL query on every request
3. **Same redirect logic** - behavior unchanged from user perspective

### `isInstallPath()` Function (unchanged)

- Still allows: `/_/install`, `/_/assets`, `/api/install/*`, `/ping`, etc.
- Prevents: all other admin/API routes before install

## Install Handler Changes

**File:** `internal/handlers/private/install.go`

### Changes to `InstallStatus` Endpoint

```go
func InstallStatus(c fiber.Ctx) error {
    log := logging.New()
    
    // Use package-level flag instead of querying database
    installed := !db.InstallRequired()
    
    return webutil.Response(c, fiber.StatusOK, "Installation status", installStatus{Installed: installed})
}
```

### Changes to `Install` Endpoint

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
    
    return webutil.Response(c, fiber.StatusOK, "Cart installed", nil)
}

// NEW: Helper to build db.Config from install request
func buildConfigFromRequest(req *models.Install) *db.Config {
    cfg := &db.Config{
        Type: req.DBType,
    }
    
    if req.DBType == "postgres" || req.DBType == "postgresql" {
        cfg.PostgreSQL = db.PostgresConfig{
            // Parse DATABASE_URL from request
            // (use existing parseConnectionURL logic)
        }
    } else {
        cfg.SQLite = db.SQLiteConfig{
            Path: req.SQLitePath,
        }
    }
    
    return cfg
}
```

### Key Behaviors

1. User submits install form with database selection
2. Build `db.Config` from request (may differ from env vars)
3. Run migrations using user's config
4. Create admin user
5. Set `installRequired = false` so middleware allows access
6. Future boots expect env vars to match this installation

## Frontend Changes

**File:** `web/admin/src/routes/install/+page.svelte`

### Changes: MINIMAL

Current implementation already matches requirements. Only change needed:

**Update payload to match backend:**

```typescript
const payload = {
  email,
  password,
  domain,
  dbType,                                           // NEW: camelCase
  databaseUrl: dbType === 'postgres' ? databaseUrl : '',  // NEW: camelCase
  sqlitePath: dbType === 'sqlite' ? sqlitePath : ''       // NEW: camelCase
}
```

**File:** `internal/models/install.go`

**Update struct:**

```go
type Install struct {
    Email       string `json:"email"`
    Password    string `json:"password"`
    Domain      string `json:"domain"`
    DBType      string `json:"dbType"`      // NEW: camelCase
    DatabaseURL string `json:"databaseUrl"` // NEW: camelCase
    SQLitePath  string `json:"sqlitePath"`  // NEW: camelCase
}
```

**Validation logic** (add to `Install.Validate()`):
- Validate `DBType` is "sqlite" or "postgres"
- If postgres: validate `DatabaseURL` is not empty and starts with `postgres://` or `postgresql://`
- If sqlite: validate `SQLitePath` is not empty

## Error Handling

### Error Scenarios

**1. Startup - Connection Failure**
```
db.Connect() → error
↓
Log: "failed to connect to database: <details>"
↓
App fails to start
```
**User action:** Fix env vars, restart app

**2. Startup - Database Exists But Empty**
```
db.Connect() → success
db.IsInstalled() → false
↓
installRequired = true
↓
App starts, shows install page
```
**User action:** Complete install form

**3. Startup - Migration Failure (when installed)**
```
db.Connect() → success
db.IsInstalled() → true
db.Migrate() → error
↓
Log: "failed to run migrations: <details>"
↓
App fails to start
```
**User action:** Fix migration file or roll back, restart app

**4. Install - User Submits Invalid Config**
```
Install handler receives request
db.MigrateWithConfig(userConfig) → connection error
↓
Return 500 Internal Server Error
↓
Frontend shows error message
```
**User action:** Fix database config, resubmit

**5. Install - Migrations Fail During Installation**
```
db.MigrateWithConfig(userConfig) → migration error
↓
Return 500 Internal Server Error
↓
installRequired remains true
```
**User action:** Fix issue, resubmit install form

**6. Install - Database Already Installed**
```
store.Install() → ErrAlreadyInstalled
↓
Return 400 Bad Request: "cart already installed"
```
**User action:** Navigate to admin login

### Logging Strategy

- **Startup errors:** `log.Err(err).Msg()` with context
- **Install errors:** `log.ErrorStack(err)` for full stack trace
- **Install success:** `log.Info().Msg("Installation completed successfully")`

### Error Messages to User

- Generic for internal errors: "Installation failed"
- Specific for validation: "Invalid PostgreSQL connection string"
- Never expose internal paths or credentials

## Testing Strategy

### Unit Tests

**File:** `internal/store/db/init_test.go`

```go
func TestConnect(t *testing.T) {
    // Test successful connection with env vars
    // Test connection failure with invalid config
}

func TestIsInstalled(t *testing.T) {
    // Test returns true when goose_db_version exists with records
    // Test returns false when table doesn't exist
    // Test returns false when table exists but empty
}

func TestMigrate(t *testing.T) {
    // Test migrations run successfully on empty database
    // Test migrations are idempotent
}

func TestMigrateWithConfig(t *testing.T) {
    // Test migration with SQLite config
    // Test migration with PostgreSQL config
    // Test migration failure with invalid config
}
```

**File:** `internal/handlers/private/install_test.go`

```go
func TestInstallStatus(t *testing.T) {
    // Test returns installed=false before install
    // Test returns installed=true after install
}

func TestInstall(t *testing.T) {
    // Test successful installation with sqlite
    // Test successful installation with postgres
    // Test validation errors
    // Test prevents double installation
}
```

### Integration Tests

**File:** `internal/app_test.go`

```go
func TestAppStartup_NotInstalled(t *testing.T) {
    // Setup: empty database
    // Assert: installRequired = true, GET / redirects to /_/install
}

func TestAppStartup_Installed(t *testing.T) {
    // Setup: database with migrations
    // Assert: migrations run, installRequired = false
}

func TestInstallFlow_EndToEnd(t *testing.T) {
    // Full flow: POST /api/install → verify migrations + admin user
}
```

### Test Helpers

```go
// testutil package
func CreateTestDB(t *testing.T) *sql.DB
func SetupInstalledDB(t *testing.T) *sql.DB
func ResetInstallFlag()
```

### Coverage Targets

- Database layer: 80%+
- Install handlers: 80%+
- Middleware: 70%+

## Files to Change

| File | Action | Why |
|------|--------|-----|
| `internal/store/db/init.go` | UPDATE | Add Connect(), IsInstalled(), Migrate(), MigrateWithConfig() |
| `internal/app.go` | UPDATE | Update NewApp() and InstallCheck middleware |
| `internal/handlers/private/install.go` | UPDATE | Use new db functions, add buildConfigFromRequest() |
| `internal/models/install.go` | UPDATE | Change field names to camelCase |
| `web/admin/src/routes/install/+page.svelte` | UPDATE | Update payload field names to camelCase |
| `internal/store/db/init_test.go` | CREATE | Unit tests for new db functions |
| `internal/handlers/private/install_test.go` | UPDATE | Update tests for new install flow |
| `internal/app_test.go` | CREATE | Integration tests for startup and install flow |

## Validation

```bash
# Build
go build ./...

# Test
go test ./... -count=1 -race

# Frontend
cd web/admin && bun run build

# Manual validation
1. Fresh install: DELETE database file, run app, complete install page
2. Subsequent boot: Restart app, verify migrations run automatically
3. Wrong env vars: Point to non-existent DB, verify app fails to start
4. Install page: Verify defaults, submit with postgres, verify works
```

## Risks

| Risk | Likelihood | Mitigation |
|------|------------|------------|
| User selects database on install page that differs from env vars, then env vars don't match on next boot | Medium | Log warning during install if user config differs from env vars |
| Breaking existing deployments that rely on settings.installed check | Low | This is a redesign; document migration path in release notes |
| Connection pooling issues with MigrateWithConfig() creating temporary connection | Low | Explicitly close connection after migration in MigrateWithConfig() |

## Acceptance Criteria

- [ ] Fresh install flow works: empty database → install page → migrations run → admin created
- [ ] Subsequent boots work: env vars → connect → migrations run automatically
- [ ] Install page shows correct defaults and accepts user input
- [ ] Validation prevents invalid database configs
- [ ] All tests pass with 80%+ coverage
- [ ] No regression in existing install behavior from user perspective
