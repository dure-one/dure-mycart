package db

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"io/fs"
	"math"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/pressly/goose/v3"
	_ "github.com/lib/pq"
	_ "modernc.org/sqlite"

	"github.com/shurco/mycart/internal/store/db/postgres"
	"github.com/shurco/mycart/internal/store/db/sqlite"
)

// Package-level database connection state
var (
	db     *sql.DB
	dbType string
	installRequired bool // Set to true if database is not installed
)

// Config holds database configuration
type Config struct {
	Type       string
	SQLite     SQLiteConfig
	PostgreSQL PostgresConfig
}

type SQLiteConfig struct {
	Path string
}

type PostgresConfig struct {
	Host            string
	Port            int
	Database        string
	User            string
	Password        string
	SSLMode         string
	ConnectTimeout  int
	MaxOpenConns    int
	MaxIdleConns    int
}

func (c *PostgresConfig) ConnectionString() string {
	return fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s connect_timeout=%d",
		c.Host, c.Port, c.User, c.Password, c.Database, c.SSLMode, c.ConnectTimeout,
	)
}

// safeInt64ToInt32 safely converts an int64 to int32, returning an error if the value
// is outside the valid int32 range. This prevents integer overflow vulnerabilities.
func safeInt64ToInt32(value int64, fieldName string) (int32, error) {
	if value > math.MaxInt32 || value < math.MinInt32 {
		return 0, fmt.Errorf("%s value %d exceeds int32 range [%d, %d]",
			fieldName, value, math.MinInt32, math.MaxInt32)
	}
	return int32(value), nil
}

// loadConfig loads database configuration from environment variables
func loadConfig() *Config {
	cfg := &Config{
		Type: getEnv("DB_TYPE", "sqlite"),
	}

	cfg.SQLite = SQLiteConfig{
		Path: getEnv("SQLITE_PATH", "./lc_base/data.db"),
	}

	cfg.PostgreSQL = PostgresConfig{
		Host:           getEnv("DB_HOST", "localhost"),
		Port:           getEnvInt("DB_PORT", 5432),
		Database:       getEnv("DB_NAME", "mycart"),
		User:           getEnv("DB_USER", "postgres"),
		Password:       os.Getenv("DB_PASSWORD"),
		SSLMode:        getEnv("DB_SSLMODE", "require"),
		ConnectTimeout: getEnvInt("DB_CONNECT_TIMEOUT", 10),
		MaxOpenConns:   getEnvInt("DB_MAX_OPEN_CONNS", 25),
		MaxIdleConns:   getEnvInt("DB_MAX_IDLE_CONNS", 5),
	}

	// Override with DATABASE_URL if present
	if databaseURL := os.Getenv("DATABASE_URL"); databaseURL != "" {
		cfg.PostgreSQL = parseConnectionURL(databaseURL, cfg.PostgreSQL)
	}

	return cfg
}

// getEnv returns environment variable or default value
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// getEnvInt returns environment variable as int or default value
func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intVal, err := strconv.Atoi(value); err == nil {
			return intVal
		}
	}
	return defaultValue
}

// parseConnectionURL parses DATABASE_URL and merges with defaults
func parseConnectionURL(rawURL string, defaults PostgresConfig) PostgresConfig {
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

// connectWithRetry connects to database with retry logic (3 attempts, exponential backoff)
func connectWithRetry(cfg *Config) (*sql.DB, error) {
	var conn *sql.DB
	var err error

	// Determine driver and DSN
	var driver, dsn string
	switch cfg.Type {
	case "postgres", "postgresql":
		driver = "postgres"
		dsn = cfg.PostgreSQL.ConnectionString()
	default:
		driver = "sqlite"
		dsn = cfg.SQLite.Path
	}

	// Retry logic: 3 attempts with exponential backoff
	maxAttempts := 3
	backoff := []time.Duration{1 * time.Second, 2 * time.Second, 4 * time.Second}

	for attempt := 0; attempt < maxAttempts; attempt++ {
		conn, err = sql.Open(driver, dsn)
		if err != nil {
			if attempt < maxAttempts-1 {
				fmt.Printf("⚠️  Database connection attempt %d/%d failed: %v\n", attempt+1, maxAttempts, err)
				time.Sleep(backoff[attempt])
				continue
			}
			return nil, fmt.Errorf("failed to open database after %d attempts: %w", maxAttempts, err)
		}

		// Test connection
		if err = conn.Ping(); err != nil {
			conn.Close()
			if attempt < maxAttempts-1 {
				fmt.Printf("⚠️  Database ping attempt %d/%d failed: %v\n", attempt+1, maxAttempts, err)
				time.Sleep(backoff[attempt])
				continue
			}
			return nil, fmt.Errorf("failed to ping database after %d attempts: %w", maxAttempts, err)
		}

		// Configure SQLite-specific settings
		if driver == "sqlite" {
			// Enable WAL mode for better concurrency and performance
			if _, err := conn.Exec("PRAGMA journal_mode=WAL"); err != nil {
				conn.Close()
				return nil, fmt.Errorf("failed to enable WAL mode: %w", err)
			}

			// Enable foreign key constraints
			if _, err := conn.Exec("PRAGMA foreign_keys=ON"); err != nil {
				conn.Close()
				return nil, fmt.Errorf("failed to enable foreign keys: %w", err)
			}

			// Set synchronous mode to NORMAL for better performance with WAL
			// NORMAL is safe with WAL and much faster than FULL
			if _, err := conn.Exec("PRAGMA synchronous=NORMAL"); err != nil {
				conn.Close()
				return nil, fmt.Errorf("failed to set synchronous mode: %w", err)
			}

			// Set busy timeout to 5 seconds to handle concurrent access
			if _, err := conn.Exec("PRAGMA busy_timeout=5000"); err != nil {
				conn.Close()
				return nil, fmt.Errorf("failed to set busy timeout: %w", err)
			}
		}

		// Configure connection pool
		if cfg.Type == "postgres" || cfg.Type == "postgresql" {
			conn.SetMaxOpenConns(cfg.PostgreSQL.MaxOpenConns)
			conn.SetMaxIdleConns(cfg.PostgreSQL.MaxIdleConns)
		} else {
			conn.SetMaxOpenConns(1) // SQLite: single connection
		}

		return conn, nil
	}

	return nil, fmt.Errorf("unreachable: retry logic failed")
}

// runMigrations runs goose migrations
func runMigrations(db *sql.DB, dbType string, migrationsFS embed.FS) error {
	// Determine the migration path based on database type
	var migrationPath string
	switch dbType {
	case "sqlite":
		migrationPath = "sqlite"
	case "postgresql", "postgres":
		migrationPath = "postgres"
	default:
		return fmt.Errorf("unsupported database type for migrations: %s", dbType)
	}

	// Get the subdirectory from the embedded filesystem
	migrationsSubFS, err := fs.Sub(migrationsFS, migrationPath)
	if err != nil {
		return fmt.Errorf("failed to access migrations directory %s: %w", migrationPath, err)
	}

	// Set the appropriate goose dialect
	var dialect string
	if dbType == "sqlite" {
		dialect = "sqlite3"
	} else {
		dialect = "postgres"
	}

	// Set goose to use the correct dialect
	if err := goose.SetDialect(dialect); err != nil {
		return fmt.Errorf("failed to set goose dialect to %s: %w", dialect, err)
	}

	// Run migrations
	goose.SetBaseFS(migrationsSubFS)
	if err := goose.Up(db, "."); err != nil {
		return fmt.Errorf("failed to run migrations: %w", err)
	}

	return nil
}

// logDatabaseInfo logs connection information
func logDatabaseInfo(cfg *Config) {
	switch cfg.Type {
	case "postgres", "postgresql":
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

// initFunctionPointers delegates to postgres or sqlite initialization
func initFunctionPointers(sqlDB *sql.DB, dbTypeName string) error {
	switch dbTypeName {
	case "postgres", "postgresql":
		initPostgres(sqlDB)
	case "sqlite", "sqlite3":
		initSQLite(sqlDB)
	default:
		return fmt.Errorf("unsupported database type: %s", dbTypeName)
	}
	return nil
}

// Connect establishes database connection without running migrations.
// It loads config from environment variables, connects with retry logic,
// initializes function pointers, and sets installRequired flag based on
// whether database is installed. Does NOT run migrations.
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

// IsInstalled checks if the database is installed by verifying whether
// the goose_db_version table exists and has at least one record.
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

// InstallRequired returns true if the database needs installation.
// This flag is set during Connect() based on IsInstalled() check.
func InstallRequired() bool {
	return installRequired
}

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

// MigrateWithConfig connects to database with provided config and runs migrations.
// This is used by the install handler to migrate with user-selected database config.
// Updates the global db connection to use the new config.
func MigrateWithConfig(cfg *Config, migrationsFS embed.FS) error {
	// Close existing connection if any
	if db != nil {
		db.Close()
	}

	// Connect with provided config
	conn, err := connectWithRetry(cfg)
	if err != nil {
		return fmt.Errorf("connect failed: %w", err)
	}

	// Update global connection
	db = conn
	dbType = cfg.Type

	// Initialize function pointers for new connection
	if err := initFunctionPointers(db, dbType); err != nil {
		db.Close()
		db = nil
		return fmt.Errorf("function pointer init failed: %w", err)
	}

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

// SetInstalled marks the database as installed.
// Called by the install handler after successful installation.
func SetInstalled() {
	installRequired = false
}

// Close closes the database connection
func Close() error {
	if db != nil {
		return db.Close()
	}
	return nil
}

// Health checks database health
func Health() error {
	if db == nil {
		return fmt.Errorf("database not initialized")
	}
	return db.Ping()
}

// Type returns database type ("sqlite" or "postgres")
func Type() string {
	return dbType
}

// DB returns *sql.DB for transactions in store layer
func DB() *sql.DB {
	return db
}

// InitFromDB initializes function pointers from an existing database connection
// Used for testing with in-memory databases
func InitFromDB(sqlDB *sql.DB, dbTypeName string) error {
	if sqlDB == nil {
		return fmt.Errorf("database connection is nil")
	}

	db = sqlDB
	dbType = dbTypeName

	return initFunctionPointers(sqlDB, dbTypeName)
}

// initPostgresSettings assigns Settings-related PostgreSQL implementations
func initPostgresSettings(q *postgres.Queries) {
	GetSettingByKeyFunc = func(ctx context.Context, key string) (Setting, error) {
		pgSetting, err := q.GetSettingByKey(ctx, key)
		if err != nil {
			return Setting{}, err
		}
		return FromPostgresSetting(pgSetting), nil
	}

	DeleteSettingFunc = q.DeleteSetting

	ListSettingsFunc = func(ctx context.Context) ([]Setting, error) {
		pgSettings, err := q.ListSettings(ctx)
		if err != nil {
			return nil, err
		}
		result := make([]Setting, len(pgSettings))
		for i, s := range pgSettings {
			result[i] = FromPostgresSetting(s)
		}
		return result, nil
	}

	CreateSettingFunc = func(ctx context.Context, arg CreateSettingParams) (Setting, error) {
		pgParams := postgres.CreateSettingParams{
			ID:    arg.ID,
			Key:   arg.Key,
			Value: arg.Value,
		}
		pgSetting, err := q.CreateSetting(ctx, pgParams)
		if err != nil {
			return Setting{}, err
		}
		return Setting{
			ID:    pgSetting.ID,
			Key:   pgSetting.Key,
			Value: pgSetting.Value,
		}, nil
	}

	UpdateSettingFunc = func(ctx context.Context, arg UpdateSettingParams) error {
		pgParams := postgres.UpdateSettingParams{
			Value: arg.Value,
			Key:   arg.Key,
		}
		return q.UpdateSetting(ctx, pgParams)
	}
}

// initPostgresSessions assigns Sessions-related PostgreSQL implementations
func initPostgresSessions(q *postgres.Queries) {
	GetSessionFunc = func(ctx context.Context, key string) (Session, error) {
		pgSession, err := q.GetSession(ctx, key)
		if err != nil {
			return Session{}, err
		}
		return FromPostgresSession(pgSession), nil
	}

	DeleteSessionFunc = q.DeleteSession

	UpsertSessionFunc = func(ctx context.Context, arg UpsertSessionParams) error {
		return q.UpsertSession(ctx, postgres.UpsertSessionParams{
			Key:     arg.Key,
			Value:   sql.NullString{String: arg.Value, Valid: arg.Value != ""},
			Expires: sql.NullInt32{Int32: int32(arg.Expires), Valid: true},
		})
	}

	CreateSessionFunc = func(ctx context.Context, arg CreateSessionParams) error {
		var expires sql.NullInt32
		if arg.Expires.Valid {
			expiresInt32, err := safeInt64ToInt32(arg.Expires.Int64, "session expires")
			if err != nil {
				return fmt.Errorf("failed to create session: %w", err)
			}
			expires = sql.NullInt32{Int32: expiresInt32, Valid: true}
		}

		pgParams := postgres.CreateSessionParams{
			Key:     arg.Key,
			Value:   arg.Value,
			Expires: expires,
		}
		return q.CreateSession(ctx, pgParams)
	}

	UpdateSessionFunc = func(ctx context.Context, arg UpdateSessionParams) error {
		var expires sql.NullInt32
		if arg.Expires.Valid {
			expiresInt32, err := safeInt64ToInt32(arg.Expires.Int64, "session expires")
			if err != nil {
				return fmt.Errorf("failed to update session: %w", err)
			}
			expires = sql.NullInt32{Int32: expiresInt32, Valid: true}
		}

		pgParams := postgres.UpdateSessionParams{
			Value:   arg.Value,
			Expires: expires,
			Key:     arg.Key,
		}
		return q.UpdateSession(ctx, pgParams)
	}
}

// initPostgresPages assigns Pages-related PostgreSQL implementations
func initPostgresPages(q *postgres.Queries) {
	GetPageBySlugFunc = func(ctx context.Context, slug string) (Page, error) {
		pgPage, err := q.GetPageBySlug(ctx, slug)
		if err != nil {
			return Page{}, err
		}
		return FromPostgresPageRow(pgPage), nil
	}

	ListPagesFunc = func(ctx context.Context, limit, offset int32) ([]Page, error) {
		pgPages, err := q.ListPages(ctx, postgres.ListPagesParams{
			Limit:  limit,
			Offset: offset,
		})
		if err != nil {
			return nil, err
		}
		pages := make([]Page, len(pgPages))
		for i, p := range pgPages {
			pages[i] = FromPostgresPageRow(p)
		}
		return pages, nil
	}

	CreatePageFunc = func(ctx context.Context, params CreatePageParams) (Page, error) {
		pgPage, err := q.CreatePage(ctx, postgres.CreatePageParams{
			ID:       params.ID,
			Name:     params.Name,
			Slug:     params.Slug,
			Content:  params.Content,
			Position: params.Position,
			Active:   params.Active,
		})
		if err != nil {
			return Page{}, err
		}
		return FromPostgresPageRow(pgPage), nil
	}

	UpdatePageFunc = func(ctx context.Context, params UpdatePageParams) error {
		return q.UpdatePage(ctx, postgres.UpdatePageParams{
			Name:     params.Name,
			Slug:     params.Slug,
			Content:  params.Content,
			Position: params.Position,
			Active:   params.Active,
			ID:       params.ID,
		})
	}

	DeletePageFunc = func(ctx context.Context, id string) error {
		return q.DeletePage(ctx, id)
	}

	GetPageByIDFunc = func(ctx context.Context, id string) (Page, error) {
		pgPage, err := q.GetPageByID(ctx, id)
		if err != nil {
			return Page{}, err
		}
		return FromPostgresPageRow(pgPage), nil
	}

	CountPagesFunc = func(ctx context.Context) (int64, error) {
		return q.CountPages(ctx)
	}

	UpdatePageContentFunc = func(ctx context.Context, id string, content sql.NullString) error {
		return q.UpdatePageContent(ctx, postgres.UpdatePageContentParams{
			Content: content,
			ID:      id,
		})
	}

	UpdatePageActiveFunc = func(ctx context.Context, id string) error {
		return q.UpdatePageActive(ctx, id)
	}

	PageExistsFunc = func(ctx context.Context, slug string) (bool, error) {
		return q.PageExists(ctx, slug)
	}

	GetPageSeoFunc = func(ctx context.Context, id string) ([]byte, error) {
		return q.GetPageSeo(ctx, id)
	}

	UpdatePageSeoFunc = func(ctx context.Context, seo []byte, id string) error {
		return q.UpdatePageSeo(ctx, postgres.UpdatePageSeoParams{
			Seo: seo,
			ID:  id,
		})
	}
}

// initPostgresProducts assigns Products-related PostgreSQL implementations
func initPostgresProducts(q *postgres.Queries) {
	GetProductByIDFunc = func(ctx context.Context, id string) (Product, error) {
		pgProduct, err := q.GetProductByID(ctx, id)
		if err != nil {
			return Product{}, err
		}
		return FromPostgresProductRow(pgProduct), nil
	}

	GetProductBySlugFunc = func(ctx context.Context, slug string) (Product, error) {
		pgProduct, err := q.GetProductBySlug(ctx, slug)
		if err != nil {
			return Product{}, err
		}
		return FromPostgresProductRow(pgProduct), nil
	}

	CreateProductFunc = func(ctx context.Context, params CreateProductParams) (Product, error) {
		// Convert Int64 to Int32 for postgres with bounds checking
		var quantity sql.NullInt32
		if params.Quantity.Valid {
			quantityInt32, err := safeInt64ToInt32(params.Quantity.Int64, "product quantity")
			if err != nil {
				return Product{}, fmt.Errorf("failed to create product: %w", err)
			}
			quantity = sql.NullInt32{Int32: quantityInt32, Valid: true}
		}

		// Convert bool to NullBool
		hasVariants := sql.NullBool{Bool: params.HasVariants, Valid: true}

		pgProduct, err := q.CreateProduct(ctx, postgres.CreateProductParams{
			ID:          params.ID,
			Name:        params.Name,
			Brief:       params.Brief,
			Desc:        params.Desc,
			Slug:        params.Slug,
			Amount:      params.Amount,
			Metadata:    params.Metadata,
			Attribute:   params.Attribute,
			Digital:     params.Digital,
			Active:      params.Active,
			HasVariants: hasVariants,
			Quantity:    quantity,
			Sku:         params.SKU,
			Seo:         params.Seo,
		})
		if err != nil {
			return Product{}, err
		}
		return FromPostgresProductRow(pgProduct), nil
	}

	UpdateProductFunc = func(ctx context.Context, params UpdateProductParams) error {
		return q.UpdateProduct(ctx, postgres.UpdateProductParams{
			Name:      params.Name,
			Desc:      params.Desc,
			Slug:      params.Slug,
			Amount:    params.Amount,
			Metadata:  params.Metadata,
			Attribute: params.Attribute,
			Digital:   params.Digital,
			Active:    params.Active,
			ID:        params.ID,
		})
	}

	DeleteProductFunc = func(ctx context.Context, id string) error {
		return q.DeleteProduct(ctx, id)
	}

	UpdateProductFullFunc = func(ctx context.Context, params UpdateProductFullParams) error {
		var quantity sql.NullInt32
		if params.Quantity.Valid {
			quantityInt32, err := safeInt64ToInt32(params.Quantity.Int64, "product quantity")
			if err != nil {
				return fmt.Errorf("failed to update product: %w", err)
			}
			quantity = sql.NullInt32{Int32: quantityInt32, Valid: true}
		}

		return q.UpdateProductFull(ctx, postgres.UpdateProductFullParams{
			Name:        params.Name,
			Brief:       params.Brief,
			Desc:        params.Desc,
			Slug:        params.Slug,
			Amount:      params.Amount,
			Quantity:    quantity,
			Sku:         params.Sku,
			HasVariants: params.HasVariants, // sql.NullBool matches
			Metadata:    params.Metadata,
			Attribute:   params.Attribute,
			Seo:         params.Seo,
			ID:          params.ID,
		})
	}

	UpdateProductActiveFunc = func(ctx context.Context, id string) error {
		return q.UpdateProductActive(ctx, id)
	}

	ProductHasSoldDigitalDataFunc = func(ctx context.Context, productID string) (bool, error) {
		return q.ProductHasSoldDigitalData(ctx, productID)
	}

	SoftDeleteProductFunc = func(ctx context.Context, id string) error {
		return q.SoftDeleteProduct(ctx, id)
	}

	CheckSlugExistsFunc = func(ctx context.Context, slug string, excludeID string) (int64, error) {
		return q.CheckSlugExists(ctx, postgres.CheckSlugExistsParams{
			Slug: slug,
			ID:   excludeID,
		})
	}

	ListProductsPrivateFunc = func(ctx context.Context, params ListProductsPrivateParams) ([]ProductListRow, error) {
		pgProducts, err := q.ListProductsPrivate(ctx, postgres.ListProductsPrivateParams{
			Limit:  params.Limit,
			Offset: params.Offset,
		})
		if err != nil {
			return nil, err
		}
		products := make([]ProductListRow, len(pgProducts))
		for i, pg := range pgProducts {
			products[i] = ProductListRow{
				ID:            pg.ID,
				Name:          pg.Name,
				Brief:         pg.Brief,
				Slug:          pg.Slug,
				Amount:        pg.Amount,
				Quantity:      sql.NullInt64{Int64: int64(pg.Quantity.Int32), Valid: pg.Quantity.Valid},
				HasVariants:   pg.HasVariants,
				Digital:       pg.Digital,
				DigitalFilled: pg.DigitalFilled,
				Active:        pg.Active,
			}
			// Convert interface{} to []byte for Image and Variants
			if imageData, ok := pg.Image.([]byte); ok {
				products[i].Image = imageData
			} else if imageStr, ok := pg.Image.(string); ok {
				products[i].Image = []byte(imageStr)
			}
			if variantsData, ok := pg.Variants.([]byte); ok {
				products[i].Variants = variantsData
			} else if variantsStr, ok := pg.Variants.(string); ok {
				products[i].Variants = []byte(variantsStr)
			}
			if pg.Created != 0 {
				products[i].Created = sql.NullTime{Time: time.Unix(pg.Created, 0), Valid: true}
			}
		}
		return products, nil
	}

	ListProductsPublicFunc = func(ctx context.Context, params ListProductsPublicParams) ([]ProductListRow, error) {
		pgProducts, err := q.ListProductsPublic(ctx, postgres.ListProductsPublicParams{
			Limit:  params.Limit,
			Offset: params.Offset,
		})
		if err != nil {
			return nil, err
		}
		products := make([]ProductListRow, len(pgProducts))
		for i, pg := range pgProducts {
			products[i] = ProductListRow{
				ID:            pg.ID,
				Name:          pg.Name,
				Brief:         pg.Brief,
				Slug:          pg.Slug,
				Amount:        pg.Amount,
				Quantity:      sql.NullInt64{Int64: int64(pg.Quantity.Int32), Valid: pg.Quantity.Valid},
				HasVariants:   pg.HasVariants,
				Digital:       pg.Digital,
				DigitalFilled: pg.DigitalFilled,
				Active:        pg.Active,
			}
			// Convert interface{} to []byte for Image and Variants
			if imageData, ok := pg.Image.([]byte); ok {
				products[i].Image = imageData
			} else if imageStr, ok := pg.Image.(string); ok {
				products[i].Image = []byte(imageStr)
			}
			if variantsData, ok := pg.Variants.([]byte); ok {
				products[i].Variants = variantsData
			} else if variantsStr, ok := pg.Variants.(string); ok {
				products[i].Variants = []byte(variantsStr)
			}
			if pg.Created != 0 {
				products[i].Created = sql.NullTime{Time: time.Unix(pg.Created, 0), Valid: true}
			}
		}
		return products, nil
	}

	CountProductsFunc = func(ctx context.Context) (int64, error) {
		return q.CountProducts(ctx)
	}

	GetProductDetailByIDFunc = func(ctx context.Context, id string) (ProductDetail, error) {
		pgDetail, err := q.GetProductDetailByID(ctx, id)
		if err != nil {
			return ProductDetail{}, err
		}
		detail := ProductDetail{
			ID:          pgDetail.ID,
			Name:        pgDetail.Name,
			Brief:       pgDetail.Brief,
			Desc:        pgDetail.Desc,
			Slug:        pgDetail.Slug,
			Amount:      pgDetail.Amount,
			Quantity:    sql.NullInt64{Int64: int64(pgDetail.Quantity.Int32), Valid: pgDetail.Quantity.Valid},
			Sku:         pgDetail.Sku,
			HasVariants: pgDetail.HasVariants.Bool,
			Metadata:    pgDetail.Metadata,
			Attribute:   pgDetail.Attribute,
			Seo:         pgDetail.Seo,
			Digital:     pgDetail.Digital,
			Active:      pgDetail.Active,
		}
		// Handle interface{} types from PostgreSQL
		if createdInt, ok := pgDetail.Created.(int64); ok && createdInt != 0 {
			detail.Created = sql.NullTime{Time: time.Unix(createdInt, 0), Valid: true}
		}
		if updatedInt, ok := pgDetail.Updated.(int64); ok && updatedInt != 0 {
			detail.Updated = sql.NullTime{Time: time.Unix(updatedInt, 0), Valid: true}
		}
		return detail, nil
	}

	GetProductDetailBySlugFunc = func(ctx context.Context, slug string) (ProductDetail, error) {
		pgDetail, err := q.GetProductDetailBySlug(ctx, slug)
		if err != nil {
			return ProductDetail{}, err
		}
		detail := ProductDetail{
			ID:          pgDetail.ID,
			Name:        pgDetail.Name,
			Brief:       pgDetail.Brief,
			Desc:        pgDetail.Desc,
			Slug:        pgDetail.Slug,
			Amount:      pgDetail.Amount,
			Quantity:    sql.NullInt64{Int64: int64(pgDetail.Quantity.Int32), Valid: pgDetail.Quantity.Valid},
			Sku:         pgDetail.Sku,
			HasVariants: pgDetail.HasVariants.Bool,
			Metadata:    pgDetail.Metadata,
			Attribute:   pgDetail.Attribute,
			Seo:         pgDetail.Seo,
			Digital:     pgDetail.Digital,
			Active:      pgDetail.Active,
		}
		// Handle interface{} types from PostgreSQL
		if createdInt, ok := pgDetail.Created.(int64); ok && createdInt != 0 {
			detail.Created = sql.NullTime{Time: time.Unix(createdInt, 0), Valid: true}
		}
		if updatedInt, ok := pgDetail.Updated.(int64); ok && updatedInt != 0 {
			detail.Updated = sql.NullTime{Time: time.Unix(updatedInt, 0), Valid: true}
		}
		return detail, nil
	}

	// Product image operations
	GetProductImageFunc = func(ctx context.Context, id string) (ProductImage, error) {
		pgImg, err := q.GetProductImage(ctx, id)
		if err != nil {
			return ProductImage{}, err
		}
		return ProductImage{
			ID:        pgImg.ID,
			ProductID: pgImg.ProductID,
			Name:      pgImg.Name,
			Ext:       pgImg.Ext,
			OrigName:  pgImg.OrigName,
		}, nil
	}

	ListProductImagesFunc = func(ctx context.Context, productID string) ([]ProductImage, error) {
		pgImgs, err := q.ListProductImages(ctx, productID)
		if err != nil {
			return nil, err
		}
		images := make([]ProductImage, len(pgImgs))
		for i, img := range pgImgs {
			images[i] = ProductImage{
				ID:        img.ID,
				ProductID: img.ProductID,
				Name:      img.Name,
				Ext:       img.Ext,
				OrigName:  img.OrigName,
			}
		}
		return images, nil
	}

	CreateProductImageFunc = func(ctx context.Context, params CreateProductImageParams) (ProductImage, error) {
		pgImg, err := q.CreateProductImage(ctx, postgres.CreateProductImageParams{
			ID:        params.ID,
			ProductID: params.ProductID,
			Name:      params.Name,
			Ext:       params.Ext,
			OrigName:  params.OrigName,
		})
		if err != nil {
			return ProductImage{}, err
		}
		return ProductImage{
			ID:        pgImg.ID,
			ProductID: pgImg.ProductID,
			Name:      pgImg.Name,
			Ext:       pgImg.Ext,
			OrigName:  pgImg.OrigName,
		}, nil
	}

	DeleteProductImageFunc = func(ctx context.Context, id string) error {
		return q.DeleteProductImage(ctx, id)
	}

	DeleteProductImagesFunc = func(ctx context.Context, productID string) error {
		return q.DeleteProductImages(ctx, productID)
	}

	UpdateProductImagePositionFunc = func(ctx context.Context, params UpdateProductImagePositionParams) error {
		return q.UpdateProductImagePosition(ctx, postgres.UpdateProductImagePositionParams{
			Position: sql.NullInt32{
				Int32: int32(params.Position.Int64),
				Valid: params.Position.Valid,
			},
			ID:        params.ID,
			ProductID: params.ProductID,
		})
	}

	GetProductRepImageBySlugFunc = func(ctx context.Context, slug string) (GetProductRepImageBySlugRow, error) {
		pgRow, err := q.GetProductRepImageBySlug(ctx, slug)
		if err != nil {
			return GetProductRepImageBySlugRow{}, err
		}
		return GetProductRepImageBySlugRow{
			ID:       pgRow.ID,
			Name:     pgRow.Name,
			Ext:      pgRow.Ext,
			OrigName: pgRow.OrigName,
		}, nil
	}

	// Product variant operations
	ListProductVariantsByProductFunc = func(ctx context.Context, productID string) ([]ProductVariant, error) {
		pgVariants, err := q.ListProductVariantsByProduct(ctx, productID)
		if err != nil {
			return nil, err
		}
		variants := make([]ProductVariant, len(pgVariants))
		for i, v := range pgVariants {
			variants[i] = ProductVariant{
				ID:             v.ID,
				ProductID:      v.ProductID,
				Sku:            v.Sku,
				PriceSurcharge: v.PriceSurcharge,
				Quantity:       sql.NullInt64{Int64: int64(v.Quantity.Int32), Valid: v.Quantity.Valid},
				OptionValues:   v.OptionValues,
				Active:         v.Active, // sql.NullBool
				Deleted:        v.Deleted, // sql.NullBool
				Created:        v.Created,
				Updated:        v.Updated,
			}
		}
		return variants, nil
	}

	CreateProductVariantFunc = func(ctx context.Context, params CreateProductVariantParams) (ProductVariant, error) {
		var quantity sql.NullInt32
		if params.Quantity.Valid {
			quantityInt32, err := safeInt64ToInt32(params.Quantity.Int64, "variant quantity")
			if err != nil {
				return ProductVariant{}, fmt.Errorf("failed to create product variant: %w", err)
			}
			quantity = sql.NullInt32{Int32: quantityInt32, Valid: true}
		}

		pgVariant, err := q.CreateProductVariant(ctx, postgres.CreateProductVariantParams{
			ID:             params.ID,
			ProductID:      params.ProductID,
			Sku:            params.Sku,
			PriceSurcharge: params.PriceSurcharge,
			Quantity:       quantity,
			OptionValues:   params.OptionValues,
		})
		if err != nil {
			return ProductVariant{}, err
		}
		return ProductVariant{
			ID:             pgVariant.ID,
			ProductID:      pgVariant.ProductID,
			Sku:            pgVariant.Sku,
			PriceSurcharge: pgVariant.PriceSurcharge,
			Quantity:       sql.NullInt64{Int64: int64(pgVariant.Quantity.Int32), Valid: pgVariant.Quantity.Valid},
			OptionValues:   pgVariant.OptionValues,
			Active:         pgVariant.Active, // sql.NullBool
			Deleted:        pgVariant.Deleted, // sql.NullBool
			Created:        pgVariant.Created,
			Updated:        pgVariant.Updated,
		}, nil
	}

	UpdateProductVariantFunc = func(ctx context.Context, params UpdateProductVariantParams) error {
		var quantity sql.NullInt32
		if params.Quantity.Valid {
			quantityInt32, err := safeInt64ToInt32(params.Quantity.Int64, "variant quantity")
			if err != nil {
				return fmt.Errorf("failed to update product variant: %w", err)
			}
			quantity = sql.NullInt32{Int32: quantityInt32, Valid: true}
		}

		return q.UpdateProductVariant(ctx, postgres.UpdateProductVariantParams{
			Sku:            params.Sku,
			PriceSurcharge: params.PriceSurcharge,
			Quantity:       quantity,
			OptionValues:   params.OptionValues,
			ID:             params.ID,
			// Note: Active field not updated in PostgreSQL query
		})
	}

	DeleteProductVariantFunc = func(ctx context.Context, id string) error {
		return q.DeleteProductVariant(ctx, id)
	}

	// Product option operations
	ListProductOptionsByProductFunc = func(ctx context.Context, productID string) ([]ProductOption, error) {
		pgOptions, err := q.ListProductOptionsByProduct(ctx, productID)
		if err != nil {
			return nil, err
		}
		options := make([]ProductOption, len(pgOptions))
		for i, pg := range pgOptions {
			options[i] = ProductOption{
				ID:        pg.ID,
				ProductID: pg.ProductID,
				Name:      pg.Name,
				Position:  sql.NullInt64{Int64: int64(pg.Position.Int32), Valid: pg.Position.Valid},
				Created:   pg.Created,
			}
		}
		return options, nil
	}

	CreateProductOptionFunc = func(ctx context.Context, params CreateProductOptionParams) (ProductOption, error) {
		var position sql.NullInt32
		if params.Position.Valid {
			positionInt32, err := safeInt64ToInt32(params.Position.Int64, "option position")
			if err != nil {
				return ProductOption{}, fmt.Errorf("failed to create product option: %w", err)
			}
			position = sql.NullInt32{Int32: positionInt32, Valid: true}
		}

		pgOption, err := q.CreateProductOption(ctx, postgres.CreateProductOptionParams{
			ID:        params.ID,
			Name:      params.Name,
			ProductID: params.ProductID,
			Position:  position,
		})
		if err != nil {
			return ProductOption{}, err
		}
		return ProductOption{
			ID:        pgOption.ID,
			ProductID: pgOption.ProductID,
			Name:      pgOption.Name,
			Position:  sql.NullInt64{Int64: int64(pgOption.Position.Int32), Valid: pgOption.Position.Valid},
			Created:   pgOption.Created,
		}, nil
	}

	DeleteProductOptionFunc = func(ctx context.Context, id string) error {
		return q.DeleteProductOption(ctx, id)
	}

	CreateProductOptionValueFunc = func(ctx context.Context, params CreateProductOptionValueParams) (ProductOptionValue, error) {
		var position sql.NullInt32
		if params.Position.Valid {
			positionInt32, err := safeInt64ToInt32(params.Position.Int64, "option value position")
			if err != nil {
				return ProductOptionValue{}, fmt.Errorf("failed to create product option value: %w", err)
			}
			position = sql.NullInt32{Int32: positionInt32, Valid: true}
		}

		pgValue, err := q.CreateProductOptionValue(ctx, postgres.CreateProductOptionValueParams{
			ID:       params.ID,
			OptionID: params.OptionID,
			Value:    params.Value,
			Position: position,
		})
		if err != nil {
			return ProductOptionValue{}, err
		}
		return ProductOptionValue{
			ID:       pgValue.ID,
			OptionID: pgValue.OptionID,
			Value:    pgValue.Value,
			Position: sql.NullInt64{Int64: int64(pgValue.Position.Int32), Valid: pgValue.Position.Valid},
		}, nil
	}

	ListProductOptionValuesByOptionFunc = func(ctx context.Context, optionID string) ([]ProductOptionValue, error) {
		pgValues, err := q.ListProductOptionValuesByOption(ctx, optionID)
		if err != nil {
			return nil, err
		}
		values := make([]ProductOptionValue, len(pgValues))
		for i, pg := range pgValues {
			values[i] = ProductOptionValue{
				ID:       pg.ID,
				OptionID: pg.OptionID,
				Value:    pg.Value,
				Position: sql.NullInt64{Int64: int64(pg.Position.Int32), Valid: pg.Position.Valid},
			}
		}
		return values, nil
	}

	DeleteProductOptionValueFunc = func(ctx context.Context, id string) error {
		return q.DeleteProductOptionValue(ctx, id)
	}

	DeleteProductOptionsByProductFunc = func(ctx context.Context, productID string) error {
		return q.DeleteProductOptionsByProduct(ctx, productID)
	}

	DeleteProductVariantsByProductFunc = func(ctx context.Context, productID string) error {
		return q.DeleteProductVariantsByProduct(ctx, productID)
	}
}

// initPostgresDigital assigns Digital-related PostgreSQL implementations
func initPostgresDigital(q *postgres.Queries) {
	GetDigitalFileFunc = func(ctx context.Context, id string) (DigitalFile, error) {
		pgFile, err := q.GetDigitalFile(ctx, id)
		if err != nil {
			return DigitalFile{}, err
		}
		return DigitalFile{
			ID:        pgFile.ID,
			ProductID: pgFile.ProductID,
			Name:      pgFile.Name,
			Ext:       pgFile.Ext,
			OrigName:  pgFile.OrigName,
		}, nil
	}

	ListDigitalFilesFunc = func(ctx context.Context, productID string) ([]DigitalFile, error) {
		pgFiles, err := q.ListDigitalFiles(ctx, productID)
		if err != nil {
			return nil, err
		}
		files := make([]DigitalFile, len(pgFiles))
		for i, f := range pgFiles {
			files[i] = DigitalFile{
				ID:        f.ID,
				ProductID: f.ProductID,
				Name:      f.Name,
				Ext:       f.Ext,
				OrigName:  f.OrigName,
			}
		}
		return files, nil
	}

	CreateDigitalFileFunc = func(ctx context.Context, params CreateDigitalFileParams) (DigitalFile, error) {
		pgFile, err := q.CreateDigitalFile(ctx, postgres.CreateDigitalFileParams{
			ID:        params.ID,
			ProductID: params.ProductID,
			Name:      params.Name,
			Ext:       params.Ext,
			OrigName:  params.OrigName,
		})
		if err != nil {
			return DigitalFile{}, err
		}
		return DigitalFile{
			ID:        pgFile.ID,
			ProductID: pgFile.ProductID,
			Name:      pgFile.Name,
			Ext:       pgFile.Ext,
			OrigName:  pgFile.OrigName,
		}, nil
	}

	DeleteDigitalFileFunc = func(ctx context.Context, id string) error {
		return q.DeleteDigitalFile(ctx, id)
	}

	DeleteDigitalFilesFunc = func(ctx context.Context, productID string) error {
		return q.DeleteDigitalFiles(ctx, productID)
	}

	// Digital data operations
	GetDigitalDataFunc = func(ctx context.Context, id string) (DigitalData, error) {
		pgData, err := q.GetDigitalData(ctx, id)
		if err != nil {
			return DigitalData{}, err
		}
		return DigitalData{
			ID:        pgData.ID,
			ProductID: pgData.ProductID,
			Content:   pgData.Content,
			CartID:    pgData.CartID,
		}, nil
	}

	GetDigitalDataByProductFunc = func(ctx context.Context, productID string) (DigitalData, error) {
		pgData, err := q.GetDigitalDataByProduct(ctx, productID)
		if err != nil {
			return DigitalData{}, err
		}
		return DigitalData{
			ID:        pgData.ID,
			ProductID: pgData.ProductID,
			Content:   pgData.Content,
			CartID:    pgData.CartID,
		}, nil
	}

	ListDigitalDataByCartFunc = func(ctx context.Context, cartID string) ([]DigitalData, error) {
		pgData, err := q.ListDigitalDataByCart(ctx, sql.NullString{String: cartID, Valid: true})
		if err != nil {
			return nil, err
		}
		data := make([]DigitalData, len(pgData))
		for i, d := range pgData {
			data[i] = DigitalData{
				ID:        d.ID,
				ProductID: d.ProductID,
				Content:   d.Content,
				CartID:    d.CartID,
			}
		}
		return data, nil
	}

	ListUnassignedDigitalDataByProductFunc = func(ctx context.Context, productID string) ([]DigitalData, error) {
		pgData, err := q.ListUnassignedDigitalDataByProduct(ctx, productID)
		if err != nil {
			return nil, err
		}
		data := make([]DigitalData, len(pgData))
		for i, d := range pgData {
			data[i] = DigitalData{
				ID:        d.ID,
				ProductID: d.ProductID,
				Content:   d.Content,
				CartID:    d.CartID,
			}
		}
		return data, nil
	}

	CreateDigitalDataFunc = func(ctx context.Context, params CreateDigitalDataParams) (DigitalData, error) {
		pgData, err := q.CreateDigitalData(ctx, postgres.CreateDigitalDataParams{
			ID:        params.ID,
			ProductID: params.ProductID,
			Content:   params.Content,
			CartID:    params.CartID,
		})
		if err != nil {
			return DigitalData{}, err
		}
		return DigitalData{
			ID:        pgData.ID,
			ProductID: pgData.ProductID,
			Content:   pgData.Content,
			CartID:    pgData.CartID,
		}, nil
	}

	UpdateDigitalDataFunc = func(ctx context.Context, params UpdateDigitalDataParams) error {
		return q.UpdateDigitalData(ctx, postgres.UpdateDigitalDataParams{
			Content: params.Content,
			ID:      params.ID,
		})
	}

	DeleteDigitalDataFunc = func(ctx context.Context, id string) error {
		return q.DeleteDigitalData(ctx, id)
	}

	DeleteDigitalDataByProductFunc = func(ctx context.Context, productID string) error {
		return q.DeleteDigitalDataByProduct(ctx, productID)
	}
}

// initPostgresAuth assigns Auth-related PostgreSQL implementations (users, carts, payments, orders)
func initPostgresAuth(q *postgres.Queries) {
	GetUserByEmailFunc = func(ctx context.Context, email string) (User, error) {
		pgUser, err := q.GetUserByEmail(ctx, email)
		if err != nil {
			return User{}, err
		}
		return FromPostgresUser(pgUser), nil
	}

	CreateUserFunc = func(ctx context.Context, arg CreateUserParams) error {
		return q.CreateUser(ctx, postgres.CreateUserParams{
			ID:        arg.ID,
			Email:     arg.Email,
			Password:  arg.Password,
			CreatedAt: arg.CreatedAt,
			UpdatedAt: arg.UpdatedAt,
		})
	}

	UpdateUserPasswordFunc = func(ctx context.Context, arg UpdateUserPasswordParams) error {
		return q.UpdateUserPassword(ctx, postgres.UpdateUserPasswordParams{
			Password:  arg.Password,
			UpdatedAt: arg.UpdatedAt,
			Email:     arg.Email,
		})
	}

	// Cart operations
	CreateNewCartFunc = func(ctx context.Context, arg CreateCartParams) error {
		return q.CreateNewCart(ctx, postgres.CreateNewCartParams{
			ID:        arg.ID,
			SessionID: arg.SessionID,
			Status:    arg.Status,
			Total:     arg.Total,
			CreatedAt: arg.CreatedAt,
			UpdatedAt: arg.UpdatedAt,
		})
	}

	GetNewCartByIDFunc = func(ctx context.Context, id string) (Cart, error) {
		pgCart, err := q.GetNewCartByID(ctx, id)
		if err != nil {
			return Cart{}, err
		}
		return FromPostgresNewCart(pgCart), nil
	}

	GetNewCartBySessionIDFunc = func(ctx context.Context, sessionID string) (Cart, error) {
		pgCart, err := q.GetNewCartBySessionID(ctx, sessionID)
		if err != nil {
			return Cart{}, err
		}
		return FromPostgresNewCart(pgCart), nil
	}

	UpdateNewCartFunc = func(ctx context.Context, arg UpdateCartParams) error {
		return q.UpdateNewCart(ctx, postgres.UpdateNewCartParams{
			Status:    arg.Status,
			Total:     arg.Total,
			UpdatedAt: arg.UpdatedAt,
			ID:        arg.ID,
		})
	}

	DeleteNewCartFunc = func(ctx context.Context, id string) error {
		return q.DeleteNewCart(ctx, id)
	}

	CreateCartItemFunc = func(ctx context.Context, arg CreateCartItemParams) error {
		return q.CreateCartItem(ctx, postgres.CreateCartItemParams{
			ID:        arg.ID,
			CartID:    arg.CartID,
			ProductID: arg.ProductID,
			Quantity:  int32(arg.Quantity), // Convert int64 to int32
			Price:     arg.Price,
			CreatedAt: arg.CreatedAt,
		})
	}

	GetCartItemFunc = func(ctx context.Context, id string) (CartItem, error) {
		pgItem, err := q.GetCartItem(ctx, id)
		if err != nil {
			return CartItem{}, err
		}
		return FromPostgresCartItem(pgItem), nil
	}

	ListCartItemsFunc = func(ctx context.Context, cartID string) ([]CartItem, error) {
		pgItems, err := q.ListCartItems(ctx, cartID)
		if err != nil {
			return nil, err
		}
		items := make([]CartItem, len(pgItems))
		for i, item := range pgItems {
			items[i] = FromPostgresCartItem(item)
		}
		return items, nil
	}

	UpdateCartItemFunc = func(ctx context.Context, arg UpdateCartItemParams) error {
		return q.UpdateCartItem(ctx, postgres.UpdateCartItemParams{
			Quantity: int32(arg.Quantity), // Convert int64 to int32
			Price:    arg.Price,
			ID:       arg.ID,
		})
	}

	DeleteCartItemFunc = func(ctx context.Context, id string) error {
		return q.DeleteCartItem(ctx, id)
	}

	DeleteCartItemsByCartIDFunc = func(ctx context.Context, cartID string) error {
		return q.DeleteCartItemsByCartID(ctx, cartID)
	}

	// Legacy cart operations (old JSON schema)
	GetOldCartFunc = func(ctx context.Context, id string) (OldCartRow, error) {
		pgCart, err := q.GetCart(ctx, id)
		if err != nil {
			return OldCartRow{}, err
		}
		return FromPostgresGetCartRow(pgCart), nil
	}

	ListOldCartsFunc = func(ctx context.Context, limit, offset int32) ([]OldCartRow, error) {
		pgCarts, err := q.ListCarts(ctx, postgres.ListCartsParams{
			Limit:  limit,
			Offset: offset,
		})
		if err != nil {
			return nil, err
		}
		carts := make([]OldCartRow, len(pgCarts))
		for i, c := range pgCarts {
			carts[i] = FromPostgresListCartsRow(c)
		}
		return carts, nil
	}

	CountOldCartsFunc = func(ctx context.Context) (int64, error) {
		return q.CountCarts(ctx)
	}

	CreateOldCartFunc = func(ctx context.Context, arg CreateOldCartParams) (OldCartRow, error) {
		pgCart, err := q.CreateCart(ctx, postgres.CreateCartParams{
			ID:            arg.ID,
			Email:         arg.Email,
			AmountTotal:   arg.AmountTotal,
			Currency:      arg.Currency,
			PaymentID:     arg.PaymentID,
			PaymentStatus: arg.PaymentStatus,
			Cart:          arg.Cart,
			PaymentSystem: arg.PaymentSystem,
		})
		if err != nil {
			return OldCartRow{}, err
		}
		return FromPostgresCreateCartRow(pgCart), nil
	}

	UpdateOldCartFunc = func(ctx context.Context, arg UpdateOldCartParams) error {
		return q.UpdateCart(ctx, postgres.UpdateCartParams{
			Email:         arg.Email,
			AmountTotal:   arg.AmountTotal,
			Currency:      arg.Currency,
			PaymentID:     arg.PaymentID,
			PaymentStatus: arg.PaymentStatus,
			Cart:          arg.Cart,
			PaymentSystem: arg.PaymentSystem,
			ID:            arg.ID,
		})
	}

	DeleteOldCartFunc = func(ctx context.Context, id string) error {
		return q.DeleteCart(ctx, id)
	}

	UpdateCartPaymentIDFunc = func(ctx context.Context, paymentID sql.NullString, id string) error {
		return q.UpdateCartPaymentID(ctx, postgres.UpdateCartPaymentIDParams{
			PaymentID: paymentID,
			ID:        id,
		})
	}

	UpdateCartPaymentStatusFunc = func(ctx context.Context, paymentStatus sql.NullString, id string) error {
		return q.UpdateCartPaymentStatus(ctx, postgres.UpdateCartPaymentStatusParams{
			PaymentStatus: paymentStatus,
			ID:            id,
		})
	}

	UpdateCartPaymentFieldsFunc = func(ctx context.Context, paymentID, paymentStatus sql.NullString, id string) error {
		return q.UpdateCartPaymentFields(ctx, postgres.UpdateCartPaymentFieldsParams{
			PaymentID:     paymentID,
			PaymentStatus: paymentStatus,
			ID:            id,
		})
	}

	GetCartByStatusAndIDFunc = func(ctx context.Context, paymentStatus sql.NullString, id string) (string, string, error) {
		row, err := q.GetCartByStatusAndID(ctx, postgres.GetCartByStatusAndIDParams{
			PaymentStatus: paymentStatus,
			ID:            id,
		})
		if err != nil {
			return "", "", err
		}
		return row.Email.String, row.Cart, nil
	}

	GetPaymentSettingsFunc = func(ctx context.Context) ([]PaymentSettingRow, error) {
		pgSettings, err := q.GetPaymentSettings(ctx)
		if err != nil {
			return nil, err
		}
		settings := make([]PaymentSettingRow, len(pgSettings))
		for i, s := range pgSettings {
			settings[i] = PaymentSettingRow{
				Key:   s.Key,
				Value: s.Value,
			}
		}
		return settings, nil
	}
}

// initPostgres assigns PostgreSQL sqlc implementations to function pointers
func initPostgres(sqlDB *sql.DB) {
	q := postgres.New(sqlDB)

	// Initialize operations by group
	initPostgresSettings(q)
	initPostgresSessions(q)
	initPostgresPages(q)
	initPostgresProducts(q)
	initPostgresDigital(q)
	initPostgresAuth(q)
}

// initSQLiteSettings assigns Settings-related SQLite implementations
func initSQLiteSettings(q *sqlite.Queries) {
	GetSettingByKeyFunc = func(ctx context.Context, key string) (Setting, error) {
		sqliteSetting, err := q.GetSettingByKey(ctx, key)
		if err != nil {
			return Setting{}, err
		}
		return FromSQLiteSetting(sqliteSetting), nil
	}

	DeleteSettingFunc = q.DeleteSetting

	ListSettingsFunc = func(ctx context.Context) ([]Setting, error) {
		sqliteSettings, err := q.ListSettings(ctx)
		if err != nil {
			return nil, err
		}
		result := make([]Setting, len(sqliteSettings))
		for i, s := range sqliteSettings {
			result[i] = FromSQLiteSetting(s)
		}
		return result, nil
	}

	GetSessionFunc = func(ctx context.Context, key string) (Session, error) {
		sqliteSession, err := q.GetSession(ctx, key)
		if err != nil {
			return Session{}, err
		}
		return FromSQLiteSession(sqliteSession), nil
	}

	DeleteSessionFunc = q.DeleteSession

	UpsertSessionFunc = func(ctx context.Context, arg UpsertSessionParams) error {
		return q.UpsertSession(ctx, sqlite.UpsertSessionParams{
			Key:     arg.Key,
			Value:   sql.NullString{String: arg.Value, Valid: arg.Value != ""},
			Expires: sql.NullInt64{Int64: arg.Expires, Valid: true},
		})
	}

	// Wrap methods that need parameter type conversion
	CreateSettingFunc = func(ctx context.Context, arg CreateSettingParams) (Setting, error) {
		sqliteParams := sqlite.CreateSettingParams{
			ID:    arg.ID,
			Key:   arg.Key,
			Value: arg.Value,
		}
		sqliteSetting, err := q.CreateSetting(ctx, sqliteParams)
		if err != nil {
			return Setting{}, err
		}
		return Setting{
			ID:    sqliteSetting.ID,
			Key:   sqliteSetting.Key,
			Value: sqliteSetting.Value,
		}, nil
	}

	UpdateSettingFunc = func(ctx context.Context, arg UpdateSettingParams) error {
		sqliteParams := sqlite.UpdateSettingParams{
			Value: arg.Value,
			Key:   arg.Key,
		}
		return q.UpdateSetting(ctx, sqliteParams)
	}
}

// initSQLiteSessions assigns Sessions-related SQLite implementations
func initSQLiteSessions(q *sqlite.Queries) {
	CreateSessionFunc = func(ctx context.Context, arg CreateSessionParams) error {
		sqliteParams := sqlite.CreateSessionParams{
			Key:     arg.Key,
			Value:   arg.Value,
			Expires: arg.Expires, // sqlite uses Int64, matches our unified type
		}
		return q.CreateSession(ctx, sqliteParams)
	}

	UpdateSessionFunc = func(ctx context.Context, arg UpdateSessionParams) error {
		sqliteParams := sqlite.UpdateSessionParams{
			Value:   arg.Value,
			Expires: arg.Expires, // sqlite uses Int64, matches our unified type
			Key:     arg.Key,
		}
		return q.UpdateSession(ctx, sqliteParams)
	}
}

// initSQLitePages assigns Pages-related SQLite implementations
func initSQLitePages(q *sqlite.Queries) {
	GetPageBySlugFunc = func(ctx context.Context, slug string) (Page, error) {
		sqlitePage, err := q.GetPageBySlug(ctx, slug)
		if err != nil {
			return Page{}, err
		}
		return FromSQLitePageRow(sqlitePage), nil
	}

	ListPagesFunc = func(ctx context.Context, limit, offset int32) ([]Page, error) {
		sqlitePages, err := q.ListPages(ctx, sqlite.ListPagesParams{
			Limit:  int64(limit),
			Offset: int64(offset),
		})
		if err != nil {
			return nil, err
		}
		pages := make([]Page, len(sqlitePages))
		for i, p := range sqlitePages {
			pages[i] = FromSQLitePageRow(p)
		}
		return pages, nil
	}

	CreatePageFunc = func(ctx context.Context, params CreatePageParams) (Page, error) {
		sqlitePage, err := q.CreatePage(ctx, sqlite.CreatePageParams{
			ID:       params.ID,
			Name:     params.Name,
			Slug:     params.Slug,
			Content:  params.Content,
			Position: params.Position,
			Active:   params.Active,
		})
		if err != nil {
			return Page{}, err
		}
		return FromSQLitePageRow(sqlitePage), nil
	}

	UpdatePageFunc = func(ctx context.Context, params UpdatePageParams) error {
		return q.UpdatePage(ctx, sqlite.UpdatePageParams{
			Name:     params.Name,
			Slug:     params.Slug,
			Content:  params.Content,
			Position: params.Position,
			Active:   params.Active,
			ID:       params.ID,
		})
	}

	DeletePageFunc = func(ctx context.Context, id string) error {
		return q.DeletePage(ctx, id)
	}

	GetPageByIDFunc = func(ctx context.Context, id string) (Page, error) {
		sqlitePage, err := q.GetPageByID(ctx, id)
		if err != nil {
			return Page{}, err
		}
		return FromSQLitePageRow(sqlitePage), nil
	}

	CountPagesFunc = func(ctx context.Context) (int64, error) {
		return q.CountPages(ctx)
	}

	UpdatePageContentFunc = func(ctx context.Context, id string, content sql.NullString) error {
		return q.UpdatePageContent(ctx, sqlite.UpdatePageContentParams{
			Content: content,
			ID:      id,
		})
	}

	UpdatePageActiveFunc = func(ctx context.Context, id string) error {
		return q.UpdatePageActive(ctx, id)
	}

	PageExistsFunc = func(ctx context.Context, slug string) (bool, error) {
		return q.PageExists(ctx, slug)
	}

	GetPageSeoFunc = func(ctx context.Context, id string) ([]byte, error) {
		seoStr, err := q.GetPageSeo(ctx, id)
		if err != nil {
			return nil, err
		}
		return []byte(seoStr), nil
	}

	UpdatePageSeoFunc = func(ctx context.Context, seo []byte, id string) error {
		return q.UpdatePageSeo(ctx, sqlite.UpdatePageSeoParams{
			Seo: string(seo),
			ID:  id,
		})
	}
}

// initSQLiteProducts assigns Products-related SQLite implementations
func initSQLiteProducts(q *sqlite.Queries) {
	GetProductByIDFunc = func(ctx context.Context, id string) (Product, error) {
		sqliteProduct, err := q.GetProductByID(ctx, id)
		if err != nil {
			return Product{}, err
		}
		return FromSQLiteProductRow(sqliteProduct), nil
	}

	GetProductBySlugFunc = func(ctx context.Context, slug string) (Product, error) {
		sqliteProduct, err := q.GetProductBySlug(ctx, slug)
		if err != nil {
			return Product{}, err
		}
		return FromSQLiteProductRow(sqliteProduct), nil
	}

	CreateProductFunc = func(ctx context.Context, params CreateProductParams) (Product, error) {
		// Convert string Amount to interface{} for sqlite
		var amount interface{} = params.Amount

		// Convert bool to NullBool
		hasVariants := sql.NullBool{Bool: params.HasVariants, Valid: true}

		sqliteProduct, err := q.CreateProduct(ctx, sqlite.CreateProductParams{
			ID:          params.ID,
			Name:        params.Name,
			Brief:       params.Brief,
			Desc:        params.Desc,
			Slug:        params.Slug,
			Amount:      amount,
			Metadata:    string(params.Metadata),
			Attribute:   string(params.Attribute),
			Digital:     params.Digital,
			Active:      params.Active,
			HasVariants: hasVariants,
			Quantity:    params.Quantity,
			Sku:         params.SKU,
			Seo:         string(params.Seo),
		})
		if err != nil {
			return Product{}, err
		}
		return FromSQLiteProductRow(sqliteProduct), nil
	}

	UpdateProductFunc = func(ctx context.Context, params UpdateProductParams) error {
		return q.UpdateProduct(ctx, sqlite.UpdateProductParams{
			Name:      params.Name,
			Desc:      params.Desc,
			Slug:      params.Slug,
			Amount:    params.Amount,
			Metadata:  string(params.Metadata),
			Attribute: string(params.Attribute),
			Digital:   params.Digital,
			Active:    params.Active,
			ID:        params.ID,
		})
	}

	DeleteProductFunc = func(ctx context.Context, id string) error {
		return q.DeleteProduct(ctx, id)
	}

	UpdateProductFullFunc = func(ctx context.Context, params UpdateProductFullParams) error {
		return q.UpdateProductFull(ctx, sqlite.UpdateProductFullParams{
			Name:        params.Name,
			Brief:       params.Brief,
			Desc:        params.Desc,
			Slug:        params.Slug,
			Amount:      params.Amount,
			Quantity:    params.Quantity, // sqlite uses Int64
			Sku:         params.Sku,
			HasVariants: params.HasVariants,
			Metadata:    string(params.Metadata),
			Attribute:   string(params.Attribute),
			Seo:         string(params.Seo),
			ID:          params.ID,
		})
	}

	UpdateProductActiveFunc = func(ctx context.Context, id string) error {
		return q.UpdateProductActive(ctx, id)
	}

	ProductHasSoldDigitalDataFunc = func(ctx context.Context, productID string) (bool, error) {
		return q.ProductHasSoldDigitalData(ctx, productID)
	}

	SoftDeleteProductFunc = func(ctx context.Context, id string) error {
		return q.SoftDeleteProduct(ctx, id)
	}

	CheckSlugExistsFunc = func(ctx context.Context, slug string, excludeID string) (int64, error) {
		return q.CheckSlugExists(ctx, sqlite.CheckSlugExistsParams{
			Slug: slug,
			ID:   excludeID,
		})
	}

	ListProductsPrivateFunc = func(ctx context.Context, params ListProductsPrivateParams) ([]ProductListRow, error) {
		sqliteProducts, err := q.ListProductsPrivate(ctx, sqlite.ListProductsPrivateParams{
			Limit:  int64(params.Limit),
			Offset: int64(params.Offset),
		})
		if err != nil {
			return nil, err
		}
		products := make([]ProductListRow, len(sqliteProducts))
		for i, sq := range sqliteProducts {
			products[i] = ProductListRow{
				ID:            sq.ID,
				Name:          sq.Name,
				Brief:         sq.Brief,
				Slug:          sq.Slug,
				Quantity:      sql.NullInt64{Int64: sq.Quantity.Int64, Valid: sq.Quantity.Valid},
				HasVariants:   sq.HasVariants,
				Digital:       sq.Digital,
				DigitalFilled: sq.DigitalFilled,
				Active:        sq.Active,
			}
			// SQLite returns interface{} for some fields - need type assertions
			// Amount can be string, int64, float64, etc.
			if amountStr, ok := sq.Amount.(string); ok {
				products[i].Amount = amountStr
			} else if amountInt, ok := sq.Amount.(int64); ok {
				products[i].Amount = fmt.Sprintf("%d", amountInt)
			} else if amountFloat, ok := sq.Amount.(float64); ok {
				products[i].Amount = fmt.Sprintf("%.0f", amountFloat)
			}
			// Convert interface{} to []byte for Image and Variants
			if imageData, ok := sq.Image.([]byte); ok {
				products[i].Image = imageData
			} else if imageStr, ok := sq.Image.(string); ok {
				products[i].Image = []byte(imageStr)
			}
			if variantsData, ok := sq.Variants.([]byte); ok {
				products[i].Variants = variantsData
			} else if variantsStr, ok := sq.Variants.(string); ok {
				products[i].Variants = []byte(variantsStr)
			}
			if createdInt, ok := sq.Created.(int64); ok && createdInt != 0 {
				products[i].Created = sql.NullTime{Time: time.Unix(createdInt, 0), Valid: true}
			}
		}
		return products, nil
	}

	ListProductsPublicFunc = func(ctx context.Context, params ListProductsPublicParams) ([]ProductListRow, error) {
		sqliteProducts, err := q.ListProductsPublic(ctx, sqlite.ListProductsPublicParams{
			Limit:  int64(params.Limit),
			Offset: int64(params.Offset),
		})
		if err != nil {
			return nil, err
		}
		products := make([]ProductListRow, len(sqliteProducts))
		for i, sq := range sqliteProducts {
			products[i] = ProductListRow{
				ID:            sq.ID,
				Name:          sq.Name,
				Brief:         sq.Brief,
				Slug:          sq.Slug,
				Quantity:      sql.NullInt64{Int64: sq.Quantity.Int64, Valid: sq.Quantity.Valid},
				HasVariants:   sq.HasVariants,
				Digital:       sq.Digital,
				DigitalFilled: sq.DigitalFilled,
				Active:        sq.Active,
			}
			// SQLite returns interface{} for some fields - need type assertions
			// Amount can be string, int64, float64, etc.
			if amountStr, ok := sq.Amount.(string); ok {
				products[i].Amount = amountStr
			} else if amountInt, ok := sq.Amount.(int64); ok {
				products[i].Amount = fmt.Sprintf("%d", amountInt)
			} else if amountFloat, ok := sq.Amount.(float64); ok {
				products[i].Amount = fmt.Sprintf("%.0f", amountFloat)
			}
			// Convert interface{} to []byte for Image and Variants
			if imageData, ok := sq.Image.([]byte); ok {
				products[i].Image = imageData
			} else if imageStr, ok := sq.Image.(string); ok {
				products[i].Image = []byte(imageStr)
			}
			if variantsData, ok := sq.Variants.([]byte); ok {
				products[i].Variants = variantsData
			} else if variantsStr, ok := sq.Variants.(string); ok {
				products[i].Variants = []byte(variantsStr)
			}
			if createdInt, ok := sq.Created.(int64); ok && createdInt != 0 {
				products[i].Created = sql.NullTime{Time: time.Unix(createdInt, 0), Valid: true}
			}
		}
		return products, nil
	}

	CountProductsFunc = func(ctx context.Context) (int64, error) {
		return q.CountProducts(ctx)
	}

	GetProductDetailByIDFunc = func(ctx context.Context, id string) (ProductDetail, error) {
		sqliteDetail, err := q.GetProductDetailByID(ctx, id)
		if err != nil {
			return ProductDetail{}, err
		}
		detail := ProductDetail{
			ID:          sqliteDetail.ID,
			Name:        sqliteDetail.Name,
			Brief:       sqliteDetail.Brief,
			Desc:        sqliteDetail.Desc,
			Slug:        sqliteDetail.Slug,
			Quantity:    sqliteDetail.Quantity,
			Sku:         sqliteDetail.Sku,
			HasVariants: sqliteDetail.HasVariants.Bool,
			Metadata:    []byte(sqliteDetail.Metadata),
			Attribute:   []byte(sqliteDetail.Attribute),
			Seo:         []byte(sqliteDetail.Seo),
			Digital:     sqliteDetail.Digital,
			Active:      sqliteDetail.Active,
		}
		// Handle interface{} types from SQLite
		detail.Amount = convertAmount(sqliteDetail.Amount)
		if createdInt, ok := sqliteDetail.Created.(int64); ok && createdInt != 0 {
			detail.Created = sql.NullTime{Time: time.Unix(createdInt, 0), Valid: true}
		}
		if updatedInt, ok := sqliteDetail.Updated.(int64); ok && updatedInt != 0 {
			detail.Updated = sql.NullTime{Time: time.Unix(updatedInt, 0), Valid: true}
		}
		return detail, nil
	}

	GetProductDetailBySlugFunc = func(ctx context.Context, slug string) (ProductDetail, error) {
		sqliteDetail, err := q.GetProductDetailBySlug(ctx, slug)
		if err != nil {
			return ProductDetail{}, err
		}
		detail := ProductDetail{
			ID:          sqliteDetail.ID,
			Name:        sqliteDetail.Name,
			Brief:       sqliteDetail.Brief,
			Desc:        sqliteDetail.Desc,
			Slug:        sqliteDetail.Slug,
			Quantity:    sqliteDetail.Quantity,
			Sku:         sqliteDetail.Sku,
			HasVariants: sqliteDetail.HasVariants.Bool,
			Metadata:    []byte(sqliteDetail.Metadata),
			Attribute:   []byte(sqliteDetail.Attribute),
			Seo:         []byte(sqliteDetail.Seo),
			Digital:     sqliteDetail.Digital,
			Active:      sqliteDetail.Active,
		}
		// Handle interface{} types from SQLite
		detail.Amount = convertAmount(sqliteDetail.Amount)
		if createdInt, ok := sqliteDetail.Created.(int64); ok && createdInt != 0 {
			detail.Created = sql.NullTime{Time: time.Unix(createdInt, 0), Valid: true}
		}
		if updatedInt, ok := sqliteDetail.Updated.(int64); ok && updatedInt != 0 {
			detail.Updated = sql.NullTime{Time: time.Unix(updatedInt, 0), Valid: true}
		}
		return detail, nil
	}

	// Product image operations
	GetProductImageFunc = func(ctx context.Context, id string) (ProductImage, error) {
		sqliteImg, err := q.GetProductImage(ctx, id)
		if err != nil {
			return ProductImage{}, err
		}
		return ProductImage{
			ID:        sqliteImg.ID,
			ProductID: sqliteImg.ProductID,
			Name:      sqliteImg.Name,
			Ext:       sqliteImg.Ext,
			OrigName:  sqliteImg.OrigName,
		}, nil
	}

	ListProductImagesFunc = func(ctx context.Context, productID string) ([]ProductImage, error) {
		sqliteImgs, err := q.ListProductImages(ctx, productID)
		if err != nil {
			return nil, err
		}
		images := make([]ProductImage, len(sqliteImgs))
		for i, img := range sqliteImgs {
			images[i] = ProductImage{
				ID:        img.ID,
				ProductID: img.ProductID,
				Name:      img.Name,
				Ext:       img.Ext,
				OrigName:  img.OrigName,
			}
		}
		return images, nil
	}

	CreateProductImageFunc = func(ctx context.Context, params CreateProductImageParams) (ProductImage, error) {
		sqliteImg, err := q.CreateProductImage(ctx, sqlite.CreateProductImageParams{
			ID:        params.ID,
			ProductID: params.ProductID,
			Name:      params.Name,
			Ext:       params.Ext,
			OrigName:  params.OrigName,
		})
		if err != nil {
			return ProductImage{}, err
		}
		return ProductImage{
			ID:        sqliteImg.ID,
			ProductID: sqliteImg.ProductID,
			Name:      sqliteImg.Name,
			Ext:       sqliteImg.Ext,
			OrigName:  sqliteImg.OrigName,
		}, nil
	}

	DeleteProductImageFunc = func(ctx context.Context, id string) error {
		return q.DeleteProductImage(ctx, id)
	}

	DeleteProductImagesFunc = func(ctx context.Context, productID string) error {
		return q.DeleteProductImages(ctx, productID)
	}

	UpdateProductImagePositionFunc = func(ctx context.Context, params UpdateProductImagePositionParams) error {
		return q.UpdateProductImagePosition(ctx, sqlite.UpdateProductImagePositionParams{
			Position:  params.Position,
			ID:        params.ID,
			ProductID: params.ProductID,
		})
	}

	GetProductRepImageBySlugFunc = func(ctx context.Context, slug string) (GetProductRepImageBySlugRow, error) {
		sqliteRow, err := q.GetProductRepImageBySlug(ctx, slug)
		if err != nil {
			return GetProductRepImageBySlugRow{}, err
		}
		return GetProductRepImageBySlugRow{
			ID:       sqliteRow.ID,
			Name:     sqliteRow.Name,
			Ext:      sqliteRow.Ext,
			OrigName: sqliteRow.OrigName,
		}, nil
	}

	// Product variant operations
	ListProductVariantsByProductFunc = func(ctx context.Context, productID string) ([]ProductVariant, error) {
		sqliteVariants, err := q.ListProductVariantsByProduct(ctx, productID)
		if err != nil {
			return nil, err
		}
		variants := make([]ProductVariant, len(sqliteVariants))
		for i, v := range sqliteVariants {
			variants[i] = ProductVariant{
				ID:             v.ID,
				ProductID:      v.ProductID,
				Sku:            v.Sku,
				PriceSurcharge: convertFloatToPrice(v.PriceSurcharge),
				Quantity:       v.Quantity, // sqlite already uses Int64
				OptionValues:   v.OptionValues,
				Active:         v.Active,
				Deleted:        v.Deleted,
				Created:        v.Created,
				Updated:        v.Updated,
			}
		}
		return variants, nil
	}

	CreateProductVariantFunc = func(ctx context.Context, params CreateProductVariantParams) (ProductVariant, error) {
		sqliteVariant, err := q.CreateProductVariant(ctx, sqlite.CreateProductVariantParams{
			ID:             params.ID,
			ProductID:      params.ProductID,
			Sku:            params.Sku,
			PriceSurcharge: convertPriceToFloat(params.PriceSurcharge),
			Quantity:       params.Quantity, // sqlite uses Int64
			OptionValues:   params.OptionValues,
		})
		if err != nil {
			return ProductVariant{}, err
		}
		return ProductVariant{
			ID:             sqliteVariant.ID,
			ProductID:      sqliteVariant.ProductID,
			Sku:            sqliteVariant.Sku,
			PriceSurcharge: convertFloatToPrice(sqliteVariant.PriceSurcharge),
			Quantity:       sqliteVariant.Quantity,
			OptionValues:   sqliteVariant.OptionValues,
			Active:         sqliteVariant.Active,
			Deleted:        sqliteVariant.Deleted,
			Created:        sqliteVariant.Created,
			Updated:        sqliteVariant.Updated,
		}, nil
	}

	UpdateProductVariantFunc = func(ctx context.Context, params UpdateProductVariantParams) error {
		return q.UpdateProductVariant(ctx, sqlite.UpdateProductVariantParams{
			Sku:            params.Sku,
			PriceSurcharge: convertPriceToFloat(params.PriceSurcharge),
			Quantity:       params.Quantity, // sqlite uses Int64
			OptionValues:   params.OptionValues,
			ID:             params.ID,
			// Note: Active field not updated in SQLite query
		})
	}

	DeleteProductVariantFunc = func(ctx context.Context, id string) error {
		return q.DeleteProductVariant(ctx, id)
	}

	// Product option operations
	ListProductOptionsByProductFunc = func(ctx context.Context, productID string) ([]ProductOption, error) {
		sqliteOptions, err := q.ListProductOptionsByProduct(ctx, productID)
		if err != nil {
			return nil, err
		}
		options := make([]ProductOption, len(sqliteOptions))
		for i, sq := range sqliteOptions {
			options[i] = ProductOption{
				ID:        sq.ID,
				ProductID: sq.ProductID,
				Name:      sq.Name,
				Position:  sq.Position,
				Created:   sq.Created,
			}
		}
		return options, nil
	}

	CreateProductOptionFunc = func(ctx context.Context, params CreateProductOptionParams) (ProductOption, error) {
		sqliteOption, err := q.CreateProductOption(ctx, sqlite.CreateProductOptionParams{
			ID:        params.ID,
			Name:      params.Name,
			ProductID: params.ProductID,
			Position:  params.Position, // sqlite uses Int64
		})
		if err != nil {
			return ProductOption{}, err
		}
		return ProductOption{
			ID:        sqliteOption.ID,
			ProductID: sqliteOption.ProductID,
			Name:      sqliteOption.Name,
			Position:  sqliteOption.Position,
			Created:   sqliteOption.Created,
		}, nil
	}

	DeleteProductOptionFunc = func(ctx context.Context, id string) error {
		return q.DeleteProductOption(ctx, id)
	}

	CreateProductOptionValueFunc = func(ctx context.Context, params CreateProductOptionValueParams) (ProductOptionValue, error) {
		sqliteValue, err := q.CreateProductOptionValue(ctx, sqlite.CreateProductOptionValueParams{
			ID:       params.ID,
			OptionID: params.OptionID,
			Value:    params.Value,
			Position: params.Position, // sqlite uses Int64
		})
		if err != nil {
			return ProductOptionValue{}, err
		}
		return ProductOptionValue{
			ID:       sqliteValue.ID,
			OptionID: sqliteValue.OptionID,
			Value:    sqliteValue.Value,
			Position: sqliteValue.Position,
		}, nil
	}

	ListProductOptionValuesByOptionFunc = func(ctx context.Context, optionID string) ([]ProductOptionValue, error) {
		sqliteValues, err := q.ListProductOptionValuesByOption(ctx, optionID)
		if err != nil {
			return nil, err
		}
		values := make([]ProductOptionValue, len(sqliteValues))
		for i, sv := range sqliteValues {
			values[i] = ProductOptionValue{
				ID:       sv.ID,
				OptionID: sv.OptionID,
				Value:    sv.Value,
				Position: sv.Position,
			}
		}
		return values, nil
	}

	DeleteProductOptionValueFunc = func(ctx context.Context, id string) error {
		return q.DeleteProductOptionValue(ctx, id)
	}

	DeleteProductOptionsByProductFunc = func(ctx context.Context, productID string) error {
		return q.DeleteProductOptionsByProduct(ctx, productID)
	}

	DeleteProductVariantsByProductFunc = func(ctx context.Context, productID string) error {
		return q.DeleteProductVariantsByProduct(ctx, productID)
	}
}

// initSQLiteDigital assigns Digital-related SQLite implementations
func initSQLiteDigital(q *sqlite.Queries) {
	GetDigitalFileFunc = func(ctx context.Context, id string) (DigitalFile, error) {
		sqliteFile, err := q.GetDigitalFile(ctx, id)
		if err != nil {
			return DigitalFile{}, err
		}
		return DigitalFile{
			ID:        sqliteFile.ID,
			ProductID: sqliteFile.ProductID,
			Name:      sqliteFile.Name,
			Ext:       sqliteFile.Ext,
			OrigName:  sqliteFile.OrigName,
		}, nil
	}

	ListDigitalFilesFunc = func(ctx context.Context, productID string) ([]DigitalFile, error) {
		sqliteFiles, err := q.ListDigitalFiles(ctx, productID)
		if err != nil {
			return nil, err
		}
		files := make([]DigitalFile, len(sqliteFiles))
		for i, f := range sqliteFiles {
			files[i] = DigitalFile{
				ID:        f.ID,
				ProductID: f.ProductID,
				Name:      f.Name,
				Ext:       f.Ext,
				OrigName:  f.OrigName,
			}
		}
		return files, nil
	}

	CreateDigitalFileFunc = func(ctx context.Context, params CreateDigitalFileParams) (DigitalFile, error) {
		sqliteFile, err := q.CreateDigitalFile(ctx, sqlite.CreateDigitalFileParams{
			ID:        params.ID,
			ProductID: params.ProductID,
			Name:      params.Name,
			Ext:       params.Ext,
			OrigName:  params.OrigName,
		})
		if err != nil {
			return DigitalFile{}, err
		}
		return DigitalFile{
			ID:        sqliteFile.ID,
			ProductID: sqliteFile.ProductID,
			Name:      sqliteFile.Name,
			Ext:       sqliteFile.Ext,
			OrigName:  sqliteFile.OrigName,
		}, nil
	}

	DeleteDigitalFileFunc = func(ctx context.Context, id string) error {
		return q.DeleteDigitalFile(ctx, id)
	}

	DeleteDigitalFilesFunc = func(ctx context.Context, productID string) error {
		return q.DeleteDigitalFiles(ctx, productID)
	}

	// Digital data operations
	GetDigitalDataFunc = func(ctx context.Context, id string) (DigitalData, error) {
		sqliteData, err := q.GetDigitalData(ctx, id)
		if err != nil {
			return DigitalData{}, err
		}
		return DigitalData{
			ID:        sqliteData.ID,
			ProductID: sqliteData.ProductID,
			Content:   sqliteData.Content,
			CartID:    sqliteData.CartID,
		}, nil
	}

	GetDigitalDataByProductFunc = func(ctx context.Context, productID string) (DigitalData, error) {
		sqliteData, err := q.GetDigitalDataByProduct(ctx, productID)
		if err != nil {
			return DigitalData{}, err
		}
		return DigitalData{
			ID:        sqliteData.ID,
			ProductID: sqliteData.ProductID,
			Content:   sqliteData.Content,
			CartID:    sqliteData.CartID,
		}, nil
	}

	ListDigitalDataByCartFunc = func(ctx context.Context, cartID string) ([]DigitalData, error) {
		sqliteData, err := q.ListDigitalDataByCart(ctx, sql.NullString{String: cartID, Valid: true})
		if err != nil {
			return nil, err
		}
		data := make([]DigitalData, len(sqliteData))
		for i, d := range sqliteData {
			data[i] = DigitalData{
				ID:        d.ID,
				ProductID: d.ProductID,
				Content:   d.Content,
				CartID:    d.CartID,
			}
		}
		return data, nil
	}

	ListUnassignedDigitalDataByProductFunc = func(ctx context.Context, productID string) ([]DigitalData, error) {
		sqliteData, err := q.ListUnassignedDigitalDataByProduct(ctx, productID)
		if err != nil {
			return nil, err
		}
		data := make([]DigitalData, len(sqliteData))
		for i, d := range sqliteData {
			data[i] = DigitalData{
				ID:        d.ID,
				ProductID: d.ProductID,
				Content:   d.Content,
				CartID:    d.CartID,
			}
		}
		return data, nil
	}

	CreateDigitalDataFunc = func(ctx context.Context, params CreateDigitalDataParams) (DigitalData, error) {
		sqliteData, err := q.CreateDigitalData(ctx, sqlite.CreateDigitalDataParams{
			ID:        params.ID,
			ProductID: params.ProductID,
			Content:   params.Content,
			CartID:    params.CartID,
		})
		if err != nil {
			return DigitalData{}, err
		}
		return DigitalData{
			ID:        sqliteData.ID,
			ProductID: sqliteData.ProductID,
			Content:   sqliteData.Content,
			CartID:    sqliteData.CartID,
		}, nil
	}

	UpdateDigitalDataFunc = func(ctx context.Context, params UpdateDigitalDataParams) error {
		return q.UpdateDigitalData(ctx, sqlite.UpdateDigitalDataParams{
			Content: params.Content,
			ID:      params.ID,
		})
	}

	DeleteDigitalDataFunc = func(ctx context.Context, id string) error {
		return q.DeleteDigitalData(ctx, id)
	}

	DeleteDigitalDataByProductFunc = func(ctx context.Context, productID string) error {
		return q.DeleteDigitalDataByProduct(ctx, productID)
	}
}

// initSQLiteAuth assigns Auth-related SQLite implementations (users, carts, payments, orders)
func initSQLiteAuth(q *sqlite.Queries) {
	GetUserByEmailFunc = func(ctx context.Context, email string) (User, error) {
		sqliteUser, err := q.GetUserByEmail(ctx, email)
		if err != nil {
			return User{}, err
		}
		return FromSQLiteUser(sqliteUser), nil
	}

	CreateUserFunc = func(ctx context.Context, arg CreateUserParams) error {
		return q.CreateUser(ctx, sqlite.CreateUserParams{
			ID:        arg.ID,
			Email:     arg.Email,
			Password:  arg.Password,
			CreatedAt: arg.CreatedAt,
			UpdatedAt: arg.UpdatedAt,
		})
	}

	UpdateUserPasswordFunc = func(ctx context.Context, arg UpdateUserPasswordParams) error {
		return q.UpdateUserPassword(ctx, sqlite.UpdateUserPasswordParams{
			Password:  arg.Password,
			UpdatedAt: arg.UpdatedAt,
			Email:     arg.Email,
		})
	}

	// Cart operations
	CreateNewCartFunc = func(ctx context.Context, arg CreateCartParams) error {
		return q.CreateNewCart(ctx, sqlite.CreateNewCartParams{
			ID:        arg.ID,
			SessionID: arg.SessionID,
			Status:    arg.Status,
			Total:     arg.Total,
			CreatedAt: arg.CreatedAt,
			UpdatedAt: arg.UpdatedAt,
		})
	}

	GetNewCartByIDFunc = func(ctx context.Context, id string) (Cart, error) {
		sqliteCart, err := q.GetNewCartByID(ctx, id)
		if err != nil {
			return Cart{}, err
		}
		return FromSQLiteNewCart(sqliteCart), nil
	}

	GetNewCartBySessionIDFunc = func(ctx context.Context, sessionID string) (Cart, error) {
		sqliteCart, err := q.GetNewCartBySessionID(ctx, sessionID)
		if err != nil {
			return Cart{}, err
		}
		return FromSQLiteNewCart(sqliteCart), nil
	}

	UpdateNewCartFunc = func(ctx context.Context, arg UpdateCartParams) error {
		return q.UpdateNewCart(ctx, sqlite.UpdateNewCartParams{
			Status:    arg.Status,
			Total:     arg.Total,
			UpdatedAt: arg.UpdatedAt,
			ID:        arg.ID,
		})
	}

	DeleteNewCartFunc = func(ctx context.Context, id string) error {
		return q.DeleteNewCart(ctx, id)
	}

	CreateCartItemFunc = func(ctx context.Context, arg CreateCartItemParams) error {
		return q.CreateCartItem(ctx, sqlite.CreateCartItemParams{
			ID:        arg.ID,
			CartID:    arg.CartID,
			ProductID: arg.ProductID,
			Quantity:  arg.Quantity, // int64 matches SQLite
			Price:     arg.Price,
			CreatedAt: arg.CreatedAt,
		})
	}

	GetCartItemFunc = func(ctx context.Context, id string) (CartItem, error) {
		sqliteItem, err := q.GetCartItem(ctx, id)
		if err != nil {
			return CartItem{}, err
		}
		return FromSQLiteCartItem(sqliteItem), nil
	}

	ListCartItemsFunc = func(ctx context.Context, cartID string) ([]CartItem, error) {
		sqliteItems, err := q.ListCartItems(ctx, cartID)
		if err != nil {
			return nil, err
		}
		items := make([]CartItem, len(sqliteItems))
		for i, item := range sqliteItems {
			items[i] = FromSQLiteCartItem(item)
		}
		return items, nil
	}

	UpdateCartItemFunc = func(ctx context.Context, arg UpdateCartItemParams) error {
		return q.UpdateCartItem(ctx, sqlite.UpdateCartItemParams{
			Quantity: arg.Quantity, // int64 matches SQLite
			Price:    arg.Price,
			ID:       arg.ID,
		})
	}

	DeleteCartItemFunc = func(ctx context.Context, id string) error {
		return q.DeleteCartItem(ctx, id)
	}

	DeleteCartItemsByCartIDFunc = func(ctx context.Context, cartID string) error {
		return q.DeleteCartItemsByCartID(ctx, cartID)
	}

	// Legacy cart operations (old JSON schema)
	GetOldCartFunc = func(ctx context.Context, id string) (OldCartRow, error) {
		sqliteCart, err := q.GetCart(ctx, id)
		if err != nil {
			return OldCartRow{}, err
		}
		return FromSQLiteGetCartRow(sqliteCart), nil
	}

	ListOldCartsFunc = func(ctx context.Context, limit, offset int32) ([]OldCartRow, error) {
		sqliteCarts, err := q.ListCarts(ctx, sqlite.ListCartsParams{
			Limit:  int64(limit),
			Offset: int64(offset),
		})
		if err != nil {
			return nil, err
		}
		carts := make([]OldCartRow, len(sqliteCarts))
		for i, c := range sqliteCarts {
			carts[i] = FromSQLiteListCartsRow(c)
		}
		return carts, nil
	}

	CountOldCartsFunc = func(ctx context.Context) (int64, error) {
		return q.CountCarts(ctx)
	}

	CreateOldCartFunc = func(ctx context.Context, arg CreateOldCartParams) (OldCartRow, error) {
		// Convert string amount to interface{} (int64) for SQLite
		var amountTotal interface{}
		if amt, err := strconv.ParseInt(arg.AmountTotal, 10, 64); err == nil {
			amountTotal = amt
		} else {
			amountTotal = arg.AmountTotal // Fallback to string if parse fails
		}

		sqliteCart, err := q.CreateCart(ctx, sqlite.CreateCartParams{
			ID:            arg.ID,
			Email:         arg.Email,
			AmountTotal:   amountTotal,
			Currency:      arg.Currency,
			PaymentID:     arg.PaymentID,
			PaymentStatus: arg.PaymentStatus,
			Cart:          arg.Cart,
			PaymentSystem: arg.PaymentSystem,
		})
		if err != nil {
			return OldCartRow{}, err
		}
		return FromSQLiteCreateCartRow(sqliteCart), nil
	}

	UpdateOldCartFunc = func(ctx context.Context, arg UpdateOldCartParams) error {
		// Convert string amount to interface{} (int64) for SQLite
		var amountTotal interface{}
		if amt, err := strconv.ParseInt(arg.AmountTotal, 10, 64); err == nil {
			amountTotal = amt
		} else {
			amountTotal = arg.AmountTotal // Fallback to string if parse fails
		}

		return q.UpdateCart(ctx, sqlite.UpdateCartParams{
			Email:         arg.Email,
			AmountTotal:   amountTotal,
			Currency:      arg.Currency,
			PaymentID:     arg.PaymentID,
			PaymentStatus: arg.PaymentStatus,
			Cart:          arg.Cart,
			PaymentSystem: arg.PaymentSystem,
			ID:            arg.ID,
		})
	}

	DeleteOldCartFunc = func(ctx context.Context, id string) error {
		return q.DeleteCart(ctx, id)
	}

	UpdateCartPaymentIDFunc = func(ctx context.Context, paymentID sql.NullString, id string) error {
		return q.UpdateCartPaymentID(ctx, sqlite.UpdateCartPaymentIDParams{
			PaymentID: paymentID,
			ID:        id,
		})
	}

	UpdateCartPaymentStatusFunc = func(ctx context.Context, paymentStatus sql.NullString, id string) error {
		return q.UpdateCartPaymentStatus(ctx, sqlite.UpdateCartPaymentStatusParams{
			PaymentStatus: paymentStatus,
			ID:            id,
		})
	}

	UpdateCartPaymentFieldsFunc = func(ctx context.Context, paymentID, paymentStatus sql.NullString, id string) error {
		return q.UpdateCartPaymentFields(ctx, sqlite.UpdateCartPaymentFieldsParams{
			PaymentID:     paymentID,
			PaymentStatus: paymentStatus,
			ID:            id,
		})
	}

	GetCartByStatusAndIDFunc = func(ctx context.Context, paymentStatus sql.NullString, id string) (string, string, error) {
		row, err := q.GetCartByStatusAndID(ctx, sqlite.GetCartByStatusAndIDParams{
			PaymentStatus: paymentStatus,
			ID:            id,
		})
		if err != nil {
			return "", "", err
		}
		return row.Email.String, row.Cart, nil
	}

	GetPaymentSettingsFunc = func(ctx context.Context) ([]PaymentSettingRow, error) {
		sqliteSettings, err := q.GetPaymentSettings(ctx)
		if err != nil {
			return nil, err
		}
		settings := make([]PaymentSettingRow, len(sqliteSettings))
		for i, s := range sqliteSettings {
			settings[i] = PaymentSettingRow{
				Key:   s.Key,
				Value: s.Value,
			}
		}
		return settings, nil
	}
}

// initSQLite assigns SQLite sqlc implementations to function pointers
func initSQLite(sqlDB *sql.DB) {
	q := sqlite.New(sqlDB)

	// Initialize operations by group
	initSQLiteSettings(q)
	initSQLiteSessions(q)
	initSQLitePages(q)
	initSQLiteProducts(q)
	initSQLiteDigital(q)
	initSQLiteAuth(q)
}
