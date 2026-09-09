package queries

import (
	"database/sql"
	"embed"
	"fmt"
	"strings"

	"github.com/shurco/mycart/db/migrations"
	"github.com/shurco/mycart/internal/goosemigration/database"
	_ "github.com/lib/pq"
	_ "modernc.org/sqlite"
)

var db *Base
var dbAdapter database.Database

// Define the structure 'Base' that aggregates various queries related to different modules like
// settings, authentication, installation, pages, products, and cart management.
type Base struct {
	SettingQueries
	AuthQueries
	InstallQueries
	PageQueries
	ProductQueries
	CartQueries
}

// New initializes the application's database and returns an error if any occurs during the process.
// It takes an 'embed.FS' which represents the file system intended to be used with embedded files.
// This now supports both SQLite and PostgreSQL based on configuration.
func New(migrationsFS embed.FS) (err error) {
	// Load database configuration from environment
	cfg, err := database.LoadConfig()
	if err != nil {
		return err
	}

	// For backward compatibility, if no config is set, default to SQLite
	if cfg.Type == "" {
		cfg.Type = "sqlite"
		cfg.SQLite.Path = "./lc_base/data.db"
	}

	// Connect to database with retry logic
	dbAdapter, err = database.ConnectWithRetry(cfg)
	if err != nil {
		return err
	}

	// Log database information
	logDatabaseInfo(cfg)

	// Run migrations
	err = database.RunMigrations(dbAdapter.DB(), cfg.Type, migrations.Embed())
	if err != nil {
		dbAdapter.Close()
		return err
	}

	// Initialize the Base struct with the underlying *sql.DB
	sqlDB := dbAdapter.DB()
	db = &Base{
		AuthQueries:    AuthQueries{DB: sqlDB},
		InstallQueries: InstallQueries{DB: sqlDB},
		SettingQueries: SettingQueries{DB: sqlDB},
		PageQueries:    PageQueries{DB: sqlDB},
		ProductQueries: ProductQueries{DB: sqlDB},
		CartQueries:    CartQueries{DB: sqlDB},
	}
	return nil
}

// logDatabaseInfo logs the database connection information
func logDatabaseInfo(cfg *database.Config) {
	switch cfg.Type {
	case "postgres", "postgresql":
		// Mask password in connection string for security
		connInfo := fmt.Sprintf("%s@%s:%d/%s",
			cfg.PostgreSQL.User,
			cfg.PostgreSQL.Host,
			cfg.PostgreSQL.Port,
			cfg.PostgreSQL.Database)
		fmt.Printf("🔌 Database: PostgreSQL (%s)\n", connInfo)
	default:
		fmt.Printf("🔌 Database: SQLite (%s)\n", cfg.SQLite.Path)
	}
}

// NewFromDB initializes Base from an existing *sql.DB (e.g. in-memory SQLite for tests).
func NewFromDB(sqlite *sql.DB) {
	// Create SQLite adapter for test database
	dbAdapter = database.NewSQLiteAdapter(sqlite)

	db = &Base{
		AuthQueries:    AuthQueries{DB: sqlite},
		InstallQueries: InstallQueries{DB: sqlite},
		SettingQueries: SettingQueries{DB: sqlite},
		PageQueries:    PageQueries{DB: sqlite},
		ProductQueries: ProductQueries{DB: sqlite},
		CartQueries:    CartQueries{DB: sqlite},
	}
}

// DB returns the Base instance. If the database is not initialized, returns nil.
// Use New() to initialize the database before calling DB().
func DB() *Base {
	return db
}

// Adapter returns the underlying database adapter for advanced operations.
func Adapter() database.Database {
	return dbAdapter
}

// DBType returns the database type ("sqlite" or "postgres").
func DBType() string {
	if dbAdapter != nil {
		return dbAdapter.Type()
	}
	return "sqlite" // default
}

// Close closes the database connection.
func Close() error {
	if dbAdapter != nil {
		return dbAdapter.Close()
	}
	return nil
}

// BuildPlaceholder returns database-specific parameter placeholder for a single parameter
func BuildPlaceholder(index int) string {
	if DBType() == "postgres" || DBType() == "postgresql" {
		return fmt.Sprintf("$%d", index)
	}
	return "?"
}

// BuildPlaceholders returns comma-separated placeholders for IN clauses
func BuildPlaceholders(count int) string {
	if count == 0 {
		return ""
	}

	if DBType() == "postgres" || DBType() == "postgresql" {
		parts := make([]string, count)
		for i := 0; i < count; i++ {
			parts[i] = fmt.Sprintf("$%d", i+1)
		}
		return strings.Join(parts, ", ")
	}

	return strings.Repeat("?, ", count-1) + "?"
}
