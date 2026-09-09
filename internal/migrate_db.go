package app

import (
	"context"
	"database/sql"
	"fmt"
	"io/fs"
	"os"

	"github.com/pressly/goose/v3"
	_ "github.com/lib/pq"
	_ "modernc.org/sqlite"

	"github.com/shurco/mycart/db/migrations"
)

// MigrateToPostgres migrates data from SQLite to PostgreSQL
func MigrateToPostgres() error {
	log.Info().Msg("Starting migration from SQLite to PostgreSQL...")

	// Connect to source SQLite database
	sqlitePath := "./lc_base/data.db"
	log.Info().Msgf("Connecting to SQLite database at %s...", sqlitePath)
	sourceDB, err := sql.Open("sqlite", sqlitePath)
	if err != nil {
		return fmt.Errorf("failed to connect to SQLite: %w", err)
	}
	defer sourceDB.Close()

	if err := sourceDB.Ping(); err != nil {
		return fmt.Errorf("failed to ping SQLite: %w", err)
	}

	// Get PostgreSQL connection from environment
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		return fmt.Errorf("DATABASE_URL environment variable not set")
	}

	// Connect to target PostgreSQL database
	log.Info().Msg("Connecting to PostgreSQL...")
	targetDB, err := sql.Open("postgres", databaseURL)
	if err != nil {
		return fmt.Errorf("failed to connect to PostgreSQL: %w", err)
	}
	defer targetDB.Close()

	if err := targetDB.Ping(); err != nil {
		return fmt.Errorf("failed to ping PostgreSQL: %w", err)
	}

	// Run migrations on target database
	log.Info().Msg("Running migrations on PostgreSQL...")
	if err := runMigrations(targetDB, "postgres"); err != nil {
		return fmt.Errorf("failed to run migrations: %w", err)
	}

	// Migrate data
	log.Info().Msg("Migrating data from SQLite to PostgreSQL...")
	if err := migrateData(sourceDB, targetDB); err != nil {
		return fmt.Errorf("failed to migrate data: %w", err)
	}

	log.Info().Msg("✓ Migration completed successfully!")
	log.Info().Msg("Next steps:")
	log.Info().Msg("  1. Verify your data in PostgreSQL")
	log.Info().Msg("  2. Update your environment variables to use PostgreSQL")
	log.Info().Msg("     Set: DATABASE_URL=<your-postgres-url>")

	return nil
}

// MigrateToSQLite migrates data from PostgreSQL to SQLite
func MigrateToSQLite() error {
	log.Info().Msg("Starting migration from PostgreSQL to SQLite...")

	// Get PostgreSQL connection from environment
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		return fmt.Errorf("DATABASE_URL environment variable not set")
	}

	// Connect to source PostgreSQL database
	log.Info().Msg("Connecting to PostgreSQL...")
	sourceDB, err := sql.Open("postgres", databaseURL)
	if err != nil {
		return fmt.Errorf("failed to connect to PostgreSQL: %w", err)
	}
	defer sourceDB.Close()

	if err := sourceDB.Ping(); err != nil {
		return fmt.Errorf("failed to ping PostgreSQL: %w", err)
	}

	// Set up target SQLite
	sqlitePath := "./lc_base/data_migrated.db"
	log.Info().Msgf("Creating SQLite database at %s...", sqlitePath)

	// Remove if exists
	_ = os.Remove(sqlitePath)

	targetDB, err := sql.Open("sqlite", sqlitePath)
	if err != nil {
		return fmt.Errorf("failed to connect to SQLite: %w", err)
	}
	defer targetDB.Close()

	// Run migrations on target database
	log.Info().Msg("Running migrations on SQLite...")
	if err := runMigrations(targetDB, "sqlite3"); err != nil {
		return fmt.Errorf("failed to run migrations: %w", err)
	}

	// Migrate data
	log.Info().Msg("Migrating data from PostgreSQL to SQLite...")
	if err := migrateData(sourceDB, targetDB); err != nil {
		return fmt.Errorf("failed to migrate data: %w", err)
	}

	log.Info().Msg("✓ Migration completed successfully!")
	log.Info().Msg("Next steps:")
	log.Info().Msg("  1. Verify your data in SQLite")
	log.Info().Msg("  2. Update your environment to use SQLite")
	log.Info().Msgf("     SQLite database: %s", sqlitePath)

	return nil
}

// runMigrations runs goose migrations on the given database
func runMigrations(db *sql.DB, dialect string) error {
	if err := goose.SetDialect(dialect); err != nil {
		return fmt.Errorf("set dialect: %w", err)
	}

	// Extract subdirectory from embedded filesystem
	var migrationsSubFS fs.FS
	var err error
	if dialect == "postgres" {
		migrationsSubFS, err = fs.Sub(migrations.Embed(), "postgres")
	} else {
		migrationsSubFS, err = fs.Sub(migrations.Embed(), "sqlite")
	}
	if err != nil {
		return fmt.Errorf("access migrations: %w", err)
	}

	goose.SetBaseFS(migrationsSubFS)
	goose.SetTableName("migrate_db_version")

	if err := goose.Up(db, "."); err != nil {
		return fmt.Errorf("run migrations: %w", err)
	}

	return nil
}

// migrateData copies all data from source to target database
func migrateData(source, target *sql.DB) error {
	ctx := context.Background()

	// Get list of tables to migrate (excluding migration tracking tables)
	tables := []string{
		"setting",
		"session",
		"page",
		"product",
		"product_option",
		"product_variant",
		"product_image",
		"digital_file",
		"digital_data",
		"cart",
		"cart_product",
	}

	// Begin transaction on target
	tx, err := target.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback()

	for _, table := range tables {
		log.Info().Msgf("Migrating table: %s", table)

		// Get column names from source table
		columns, err := getTableColumns(source, table)
		if err != nil {
			log.Warn().Msgf("Skipping table %s: %v", table, err)
			continue
		}

		if len(columns) == 0 {
			log.Warn().Msgf("Skipping table %s: no columns found", table)
			continue
		}

		// Copy data
		if err := copyTableData(ctx, source, tx, table, columns); err != nil {
			return fmt.Errorf("copy table %s: %w", table, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}

	log.Info().Msg("All tables migrated successfully")
	return nil
}

// getTableColumns returns the column names for a table
func getTableColumns(db *sql.DB, table string) ([]string, error) {
	query := fmt.Sprintf("SELECT * FROM %s LIMIT 0", table)
	rows, err := db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return rows.Columns()
}

// copyTableData copies all rows from source table to target table
func copyTableData(ctx context.Context, source *sql.DB, targetTx *sql.Tx, table string, columns []string) error {
	// Build column list
	columnList := ""
	placeholders := ""
	for i, col := range columns {
		if i > 0 {
			columnList += ", "
			placeholders += ", "
		}
		columnList += col
		placeholders += fmt.Sprintf("$%d", i+1)
	}

	// Select all data from source
	selectQuery := fmt.Sprintf("SELECT %s FROM %s", columnList, table)
	rows, err := source.QueryContext(ctx, selectQuery)
	if err != nil {
		return fmt.Errorf("select from source: %w", err)
	}
	defer rows.Close()

	// Prepare insert statement for target
	insertQuery := fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s)", table, columnList, placeholders)
	stmt, err := targetTx.PrepareContext(ctx, insertQuery)
	if err != nil {
		return fmt.Errorf("prepare insert: %w", err)
	}
	defer stmt.Close()

	// Copy each row
	count := 0
	for rows.Next() {
		// Create slice of interface{} to hold row values
		values := make([]interface{}, len(columns))
		valuePtrs := make([]interface{}, len(columns))
		for i := range values {
			valuePtrs[i] = &values[i]
		}

		if err := rows.Scan(valuePtrs...); err != nil {
			return fmt.Errorf("scan row: %w", err)
		}

		if _, err := stmt.ExecContext(ctx, values...); err != nil {
			return fmt.Errorf("insert row: %w", err)
		}

		count++
		if count%100 == 0 {
			log.Info().Msgf("  Copied %d rows from %s...", count, table)
		}
	}

	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate rows: %w", err)
	}

	log.Info().Msgf("  ✓ Copied %d rows from %s", count, table)
	return nil
}
