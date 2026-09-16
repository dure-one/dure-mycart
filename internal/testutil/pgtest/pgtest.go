// Package pgtest provisions throwaway PostgreSQL databases for the test suite.
//
// Each call hands back a DSN for a database of its own. The schema (and, when
// asked for, the fixtures) is applied to a *template* database that pgtestdb
// builds once per server, and every test database is then cloned from it. The
// migrations therefore run a single time for the whole suite — not once per
// test, and not once per package — while each test still starts from a pristine
// database and never sees another test's rows.
//
// TEST_POSTGRES_DSN is an *administrative* connection: pgtestdb connects with it
// to create the role pgtdbuser, the template database testdb_tpl_*, and one
// testdb_tpl_*_inst_* database per test, and to drop the instances again. Point
// it at a dedicated test server, never at one holding data anybody wants to
// keep. The user it names needs CREATEDB, CREATEROLE and SUPERUSER.
//
// A test that passes has its database dropped; a test that fails keeps its, and
// pgtestdb logs the connection string so the state can be inspected with psql.
//
// The package also owns the fixture filesystem, which the SQLite path shares, so
// that "the fixtures" means one thing for both engines.
package pgtest

import (
	"context"
	"database/sql"
	"fmt"
	"io/fs"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"

	"github.com/peterldowns/pgtestdb"
	"github.com/peterldowns/pgtestdb/migrators/goosemigrator"

	"github.com/shurco/mycart/internal/database"
	"github.com/shurco/mycart/migrations"
)

// AdminDSN names the environment variable holding the administrative
// connection string of the PostgreSQL test server.
const AdminDSN = "TEST_POSTGRES_DSN"

// AdminMode names the environment variable controlling test database mode.
// "1" (default) = admin mode: requires CREATEDB/CREATEROLE, uses pgtestdb.
// "0" = table-level mode: only requires table permissions, truncates tables.
const AdminMode = "TEST_POSTGRES_ADMIN"

// tableLevelMutex serializes truncate+fixture operations in table-level mode
// to prevent deadlocks from concurrent TRUNCATE CASCADE on the shared database.
var tableLevelMutex sync.Mutex

// dropTemplatesOnce ensures dropTemplates runs only once per test suite, not once
// per test. When tests run in parallel, calling dropTemplates in every test creates
// a race condition where one test terminates connections that other tests are using.
var dropTemplatesOnce sync.Once

// FixturesDir is the directory under the repository root holding the fixture
// migrations.
const FixturesDir = "fixtures"

// FixturesTable is the goose bookkeeping table for the fixtures. It is separate
// from the schema's migrate_db_version on purpose: the fixtures are test data,
// not schema, and folding them into the schema's version history would make a
// fixture version look like an applied migration.
const FixturesTable = "migrate_fixtures_version"

// FixturesFS returns the fixture migrations, rooted at the directory goose
// should read.
//
// The path is anchored to this source file rather than to the working
// directory: the suite runs tests in a temporary directory of their own, so a
// relative path would find nothing.
func FixturesFS() fs.FS {
	return os.DirFS(filepath.Join(repoRoot(), FixturesDir, "migration"))
}

// EmptyDSN returns the DSN of a fresh database with nothing in it: no tables,
// no bookkeeping. It is for tests of the migration path itself, which have to
// apply the migrations to an empty database the way a real installation does.
func EmptyDSN(t *testing.T) string {
	t.Helper()
	if isTableLevelMode() {
		// Table-level mode cannot provide truly empty databases since the shared
		// database already has schema. Skip tests requiring empty databases.
		t.Skip("EmptyDSN not supported in table-level mode (TEST_POSTGRES_ADMIN=0)")
	}
	return instance(t, pgtestdb.NoopMigrator{})
}

// MigratedDSN returns the DSN of a fresh database with the embedded migrations
// applied.
func MigratedDSN(t *testing.T) string {
	t.Helper()
	if isTableLevelMode() {
		t.Logf("MigratedDSN: using table-level mode")
		return tableLevelInstance(t, false)
	}
	t.Logf("MigratedDSN: using admin mode (pgtestdb)")
	return instance(t, migrator(false))
}

// FixturesDSN returns the DSN of a fresh database with the embedded migrations
// and the fixtures applied.
func FixturesDSN(t *testing.T) string {
	t.Helper()
	if isTableLevelMode() {
		return tableLevelInstance(t, true)
	}
	return instance(t, migrator(true))
}

// instance provisions a database through pgtestdb and returns its DSN, ready to
// hand to database.Connect or database.Migrate.
func instance(t *testing.T, m pgtestdb.Migrator) string {
	t.Helper()

	// The returned DSN names the pgtdbuser role, not the administrative one;
	// Custom closes its own connection, so the caller owns the database.
	cfg := pgtestdb.Custom(t, config(t), m)

	// Built from the fields rather than with cfg.URL(), which interpolates the
	// password into the URL unescaped and would produce a broken DSN for one
	// containing "@" or ":".
	// Ensure timezone=UTC is in the query string for all pool connections.
	opts := cfg.Options
	if opts == "" {
		opts = "timezone=UTC"
	} else if !strings.Contains(opts, "timezone") && !strings.Contains(opts, "TimeZone") {
		opts += "&timezone=UTC"
	}

	dsn := (&url.URL{
		Scheme:   "postgres",
		User:     url.UserPassword(cfg.User, cfg.Password),
		Host:     net.JoinHostPort(cfg.Host, cfg.Port),
		Path:     "/" + cfg.Database,
		RawQuery: opts,
	}).String()

	// Then through the application's own normaliser, rather than trusting
	// pgtestdb to round-trip the query string: if this DSN is one the
	// application would refuse, the test should say so here, not somewhere
	// deep in a handler.
	dsn, err := database.NormalizePostgresDSN(dsn)
	if err != nil {
		t.Fatalf("pgtestdb produced a DSN the application refuses: %v", err)
	}
	return dsn
}

// config reads TEST_POSTGRES_DSN and parses it, failing the test if it is
// missing or unusable.
func config(t *testing.T) pgtestdb.Config {
	t.Helper()

	raw := os.Getenv(AdminDSN)
	if raw == "" {
		t.Fatalf("%s is not set", AdminDSN)
	}
	cfg, err := parseConfig(raw)
	if err != nil {
		t.Fatalf("%s: %v", AdminDSN, err)
	}

	// Drop existing template databases to force recreation with correct timezone.
	// This ensures templates built before timezone fixes are not reused.
	// Use sync.Once to run this only once per test suite, preventing race conditions
	// when tests run in parallel where one test terminates connections that other
	// tests are actively using.
	dropTemplatesOnce.Do(func() {
		dropTemplates(t, cfg)
	})

	return cfg
}

// dropTemplates drops all pgtestdb template databases to force recreation.
// This is necessary when timezone configuration changes, as templates created
// before the fix contain fixture data with incorrect timezone offsets.
func dropTemplates(t *testing.T, cfg pgtestdb.Config) {
	t.Helper()

	// Build DSN with timezone parameter to ensure our connection uses UTC
	dsn := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?timezone=UTC",
		cfg.User, cfg.Password, cfg.Host, cfg.Port, cfg.Database)

	db, err := sql.Open(cfg.DriverName, dsn)
	if err != nil {
		t.Logf("dropTemplates: failed to connect: %v", err)
		return
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		t.Logf("dropTemplates: failed to ping: %v", err)
		return
	}

	// Query for template databases
	rows, err := db.Query(`
		SELECT datname
		FROM pg_database
		WHERE datname LIKE 'testdb_tpl%'
	`)
	if err != nil {
		t.Logf("dropTemplates: failed to query templates: %v", err)
		return
	}
	defer rows.Close()

	var templates []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			continue
		}
		templates = append(templates, name)
	}

	if len(templates) > 0 {
		t.Logf("dropTemplates: found %d templates to drop: %v", len(templates), templates)
	}

	// Drop each template database
	for _, name := range templates {
		// Terminate connections first
		terminateSQL := fmt.Sprintf(`
			SELECT pg_terminate_backend(pid)
			FROM pg_stat_activity
			WHERE datname = '%s' AND pid <> pg_backend_pid()
		`, name)
		if _, err := db.Exec(terminateSQL); err != nil {
			t.Logf("dropTemplates: failed to terminate connections for %s: %v", name, err)
		}

		// Drop with proper quoting
		dropSQL := fmt.Sprintf(`DROP DATABASE IF EXISTS "%s"`, name)
		if _, err := db.Exec(dropSQL); err != nil {
			t.Logf("dropTemplates: failed to drop %s: %v", name, err)
		} else {
			t.Logf("dropTemplates: dropped %s", name)
		}
	}
}

// parseConfig turns an administrative connection string into a pgtestdb
// configuration.
//
// pgtestdb opens its own connections to create and migrate the template, so the
// timezone pin has to be applied here too. Without it the template is built in
// the server's zone, and every row of data in it — the fixtures are INSERTs with
// DEFAULT CURRENT_TIMESTAMP — is stored shifted by the server's offset. Measured
// on a server running Asia/Seoul: the fixture template's timestamps came out
// +32399s, versus -1s with the pin.
func parseConfig(raw string) (pgtestdb.Config, error) {
	dsn, err := database.NormalizePostgresDSN(raw)
	if err != nil {
		return pgtestdb.Config{}, err
	}

	u, err := url.Parse(dsn)
	// The scheme matters, not just the shape: the fields below are handed back
	// to pgtestdb, which rebuilds a postgres:// URL from them. Accepting
	// `mysql://user@host/db` here would silently reconnect to it as PostgreSQL.
	if err != nil || u.Host == "" || !isPostgresScheme(u.Scheme) {
		return pgtestdb.Config{}, fmt.Errorf(
			"must be a postgres:// URL naming a database that exists, got %q", raw)
	}

	password, _ := u.User.Password()
	name := strings.TrimPrefix(u.Path, "/")
	if name == "" {
		// pgtestdb connects to this database to administer the server; the
		// maintenance database always exists.
		name = "postgres"
	}
	port := u.Port()
	if port == "" {
		port = "5432"
	}

	// Ensure timezone=UTC is in the options. NormalizePostgresDSN should have
	// added it to the query string, but we verify and add it explicitly if missing
	// to guarantee pgtestdb's connections use UTC timezone when building templates.
	opts := u.RawQuery
	if opts == "" {
		opts = "timezone=UTC"
	} else if !strings.Contains(opts, "timezone") && !strings.Contains(opts, "TimeZone") {
		opts += "&timezone=UTC"
	}

	return pgtestdb.Config{
		DriverName: database.Postgres().Driver(),
		Host:       u.Hostname(),
		Port:       port,
		User:       u.User.Username(),
		Password:   password,
		Database:   name,
		Options:    opts,
	}, nil
}

// isPostgresScheme reports whether a URL scheme names PostgreSQL. NormalizePostgresDSN
// only prefixes the two URL forms; pgx also accepts them, so nothing else is
// offered here.
func isPostgresScheme(scheme string) bool {
	return scheme == "postgres" || scheme == "postgresql"
}

// migrator builds the template migrator: the embedded migrations, optionally
// followed by the fixtures.
func migrator(withFixtures bool) pgtestdb.Migrator {
	schema := goosemigrator.New(".",
		goosemigrator.WithFS(migrations.Embed()),
		goosemigrator.WithTableName(database.MigrateTable))
	if !withFixtures {
		return schema
	}
	return withFixturesApplied{schema: schema}
}

// withFixturesApplied is the schema migrator plus the fixtures. They are applied
// to the template rather than to each test's database, so the fixture script
// runs once for the whole suite.
type withFixturesApplied struct {
	schema pgtestdb.Migrator
}

// Hash keys the template. Both halves contribute, so the fixture template is a
// different database from the plain one and editing either set of files
// rebuilds the right template.
func (m withFixturesApplied) Hash() (string, error) {
	schema, err := m.schema.Hash()
	if err != nil {
		return "", err
	}
	fixtures, err := m.fixtures().Hash()
	if err != nil {
		return "", err
	}
	return schema + "-" + fixtures, nil
}

// Migrate applies the schema first: the fixtures are INSERTs and UPDATEs
// against tables the schema creates.
func (m withFixturesApplied) Migrate(ctx context.Context, db *sql.DB, cfg pgtestdb.Config) error {
	if err := m.schema.Migrate(ctx, db, cfg); err != nil {
		return err
	}
	return m.fixtures().Migrate(ctx, db, cfg)
}

func (m withFixturesApplied) fixtures() pgtestdb.Migrator {
	return goosemigrator.New(".",
		goosemigrator.WithFS(FixturesFS()),
		goosemigrator.WithTableName(FixturesTable))
}

// repoRoot returns the absolute path to the repository root, derived from this
// source file's location at compile time. The test suite changes the working
// directory, so the path has to be anchored to the source, not to the process.
func repoRoot() string {
	_, src, _, _ := runtime.Caller(0)
	// src = <root>/internal/testutil/pgtest/pgtest.go
	return filepath.Join(filepath.Dir(src), "..", "..", "..")
}

// isTableLevelMode reports whether tests should use table-level cleanup instead
// of database-level isolation. When TEST_POSTGRES_ADMIN=0, tests use TRUNCATE
// instead of creating/dropping databases, requiring only table permissions.
func isTableLevelMode() bool {
	mode := os.Getenv(AdminMode)
	return mode == "0"
}

// redactDSN removes the password from a DSN for safe logging.
func redactDSN(dsn string) string {
	u, err := url.Parse(dsn)
	if err != nil {
		return "[unparseable DSN]"
	}
	if u.User != nil {
		u.User = url.UserPassword(u.User.Username(), "***")
	}
	return u.String()
}

// tableLevelInstance returns a DSN to the shared test database and truncates all
// tables for isolation. This mode requires only table-level permissions (SELECT,
// INSERT, UPDATE, DELETE, TRUNCATE) instead of CREATEDB/CREATEROLE.
func tableLevelInstance(t *testing.T, withFixtures bool) string {
	t.Helper()

	raw := os.Getenv(AdminDSN)
	if raw == "" {
		t.Fatalf("%s is not set (required for table-level mode)", AdminDSN)
	}

	dsn, err := database.NormalizePostgresDSN(raw)
	if err != nil {
		t.Fatalf("%s: %v", AdminDSN, err)
	}

	// Connect using database.Connect to ensure timezone pin and pool config
	cfg := database.Config{
		Driver: database.DriverPostgres,
		DSN:    dsn,
		Source: database.SourceDefault,
	}

	ctx := context.Background()
	conn, err := database.Connect(cfg)
	if err != nil {
		t.Fatalf("connect to test database: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })

	// Explicitly set session timezone to UTC. While database.NormalizePostgresDSN
	// adds timezone=UTC to the DSN, some PostgreSQL configurations may not respect
	// it. This ensures the session timezone is always UTC regardless of the server's
	// default timezone or DSN parameter handling.
	if _, err := conn.Raw().ExecContext(ctx, "SET timezone = 'UTC'"); err != nil {
		t.Fatalf("set timezone: %v", err)
	}

	// Verify timezone is actually UTC (log only on first call per test)
	var tz string
	if err := conn.Raw().QueryRowContext(ctx, "SHOW timezone").Scan(&tz); err != nil {
		t.Fatalf("check timezone: %v", err)
	}
	if tz != "UTC" {
		t.Fatalf("session timezone is %q, expected UTC", tz)
	}

	// Ensure schema is migrated
	if err := database.MigrateOn(conn, migrations.Embed()); err != nil {
		t.Fatalf("migrate test database: %v", err)
	}

	// Serialize truncate+fixture operations to prevent deadlocks from concurrent
	// TRUNCATE CASCADE on the shared database. Tests running in parallel would
	// otherwise compete for locks on foreign key relationships.
	tableLevelMutex.Lock()
	defer tableLevelMutex.Unlock()

	// Truncate all tables for isolation
	if err := truncateTables(ctx, conn.Raw()); err != nil {
		t.Fatalf("truncate tables: %v", err)
	}

	// Apply fixtures if requested
	if withFixtures {
		if err := applyFixturesTo(ctx, conn.Raw()); err != nil {
			t.Fatalf("apply fixtures: %v", err)
		}
	}

	return dsn
}

// truncateTables empties all user tables in the database, excluding bookkeeping
// tables like migration versioning and migration-seeded configuration data.
func truncateTables(ctx context.Context, db *sql.DB) error {
	rows, err := db.QueryContext(ctx, `
		SELECT table_name
		FROM information_schema.tables
		WHERE table_schema = 'public' AND table_type = 'BASE TABLE'
		ORDER BY table_name`)
	if err != nil {
		return fmt.Errorf("list tables: %w", err)
	}
	defer rows.Close()

	var tables []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return fmt.Errorf("scan table name: %w", err)
		}
		// Skip goose bookkeeping tables and migration-seeded configuration
		if name == database.MigrateTable || name == FixturesTable || name == "setting" {
			continue
		}
		tables = append(tables, name)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate tables: %w", err)
	}

	if len(tables) == 0 {
		return nil
	}

	// Quote identifiers for safety
	quoted := make([]string, len(tables))
	for i, name := range tables {
		quoted[i] = `"` + strings.ReplaceAll(name, `"`, `""`) + `"`
	}

	truncateSQL := "TRUNCATE TABLE " + strings.Join(quoted, ", ") + " CASCADE"
	if _, err := db.ExecContext(ctx, truncateSQL); err != nil {
		return fmt.Errorf("truncate tables: %w", err)
	}

	// Reset the installation state in the setting table so tests see a clean DB.
	// The setting table holds migration-seeded configuration (currency, jwt_secret,
	// etc.) that tests may depend on, so we can't truncate it. But we must reset
	// the 'installed' flag that tests set during installation flows.
	if _, err := db.ExecContext(ctx, `UPDATE setting SET value = '0' WHERE key = 'installed'`); err != nil {
		return fmt.Errorf("reset installation state: %w", err)
	}

	return nil
}

// applyFixturesTo loads fixtures into an already-migrated and truncated database.
// In table-level mode, we execute the fixture SQL directly instead of using goose's
// versioning system, since the tables are already empty from truncation.
func applyFixturesTo(ctx context.Context, db *sql.DB) error {
	// Ensure the fixtures version table exists
	if _, err := db.ExecContext(ctx, fmt.Sprintf(
		"CREATE TABLE IF NOT EXISTS %s (id SERIAL PRIMARY KEY, version_id BIGINT, is_applied BOOLEAN, tstamp TIMESTAMP DEFAULT CURRENT_TIMESTAMP)",
		FixturesTable)); err != nil {
		return fmt.Errorf("create fixtures version table: %w", err)
	}

	// Check if fixtures have already been applied by checking for any version records
	var count int
	if err := db.QueryRowContext(ctx, fmt.Sprintf("SELECT COUNT(*) FROM %s WHERE is_applied = TRUE", FixturesTable)).Scan(&count); err != nil {
		return fmt.Errorf("check fixtures version: %w", err)
	}

	// If fixtures are already applied, skip re-applying them
	if count > 0 {
		return nil
	}

	// Read the fixture SQL file directly
	entries, err := fs.ReadDir(FixturesFS(), ".")
	if err != nil {
		return fmt.Errorf("read fixtures directory: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql") {
			continue
		}

		content, err := fs.ReadFile(FixturesFS(), entry.Name())
		if err != nil {
			return fmt.Errorf("read fixture file %s: %w", entry.Name(), err)
		}

		// Extract SQL from goose migration file (between StatementBegin/End markers)
		sqlContent := extractGooseSQL(string(content))

		// Execute the fixture SQL
		if _, err := db.ExecContext(ctx, sqlContent); err != nil {
			return fmt.Errorf("execute fixture %s: %w", entry.Name(), err)
		}

		// Mark this fixture as applied
		if _, err := db.ExecContext(ctx, fmt.Sprintf(
			"INSERT INTO %s (version_id, is_applied) VALUES (1, TRUE)", FixturesTable)); err != nil {
			return fmt.Errorf("mark fixture %s as applied: %w", entry.Name(), err)
		}
	}

	return nil
}

// extractGooseSQL extracts SQL statements from the "Up" section of a goose
// migration file (content between first StatementBegin and StatementEnd).
func extractGooseSQL(content string) string {
	var result strings.Builder
	inUpStatement := false
	foundFirstStatement := false

	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)

		// Check for goose directives
		if strings.HasPrefix(trimmed, "-- +goose") {
			if strings.Contains(trimmed, "Up") {
				// Start of Up section
				continue
			} else if strings.Contains(trimmed, "Down") {
				// Start of Down section - stop extracting
				break
			} else if strings.Contains(trimmed, "StatementBegin") {
				inUpStatement = true
				foundFirstStatement = true
				continue
			} else if strings.Contains(trimmed, "StatementEnd") {
				// If we found the first statement, this ends it
				if foundFirstStatement {
					break
				}
				inUpStatement = false
				continue
			}
			continue
		}

		// Include content when in the Up statement block
		if inUpStatement {
			result.WriteString(line)
			result.WriteString("\n")
		}
	}

	return result.String()
}
