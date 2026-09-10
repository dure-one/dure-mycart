package store_test

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/joho/godotenv"
	"github.com/shurco/mycart/db/migrations"
	"github.com/shurco/mycart/internal/store/db"
	"github.com/stretchr/testify/require"
)

// getTestDBType returns the database type for tests from environment variable
func getTestDBType() string {
	if dbType := os.Getenv("TEST_DB_TYPE"); dbType != "" {
		return dbType
	}
	return "sqlite" // default to SQLite for fast tests
}

// setupTestDB initializes a test database (SQLite or PostgreSQL) based on TEST_DB_TYPE
func setupTestDB(t *testing.T) context.Context {
	t.Helper()

	dbType := getTestDBType()

	switch dbType {
	case "sqlite":
		return setupSQLiteTest(t)
	case "postgres", "postgresql":
		return setupPostgresTest(t)
	default:
		t.Fatalf("unsupported TEST_DB_TYPE: %s", dbType)
		return nil
	}
}

// setupSQLiteTest initializes an in-memory SQLite database for testing
func setupSQLiteTest(t *testing.T) context.Context {
	t.Helper()

	// Set up environment for SQLite
	os.Setenv("DB_TYPE", "sqlite")
	os.Setenv("SQLITE_PATH", ":memory:")

	// Initialize database (runs migrations and sets up function pointers)
	err := db.Init(migrations.Embed())
	require.NoError(t, err)

	t.Cleanup(func() {
		db.Close()
	})

	return context.Background()
}

// setupPostgresTest initializes a PostgreSQL database connection for testing
func setupPostgresTest(t *testing.T) context.Context {
	t.Helper()

	// Load .env file if it exists (contains DATABASE_URL)
	_ = godotenv.Load("../../.env") // Ignore error if .env doesn't exist

	// Get connection string from environment (TEST_DATABASE_URL or DATABASE_URL from .env)
	connStr := os.Getenv("TEST_DATABASE_URL")
	if connStr == "" {
		connStr = os.Getenv("DATABASE_URL")
	}
	if connStr == "" {
		t.Fatal("DATABASE_URL not set in .env or TEST_DATABASE_URL environment variable")
	}

	// Set up environment for PostgreSQL
	os.Setenv("DB_TYPE", "postgres")
	os.Setenv("DATABASE_URL", connStr)

	// Initialize database (runs migrations and sets up function pointers)
	err := db.Init(migrations.Embed())
	require.NoError(t, err)

	// Verify connection
	err = db.Health()
	require.NoError(t, err, "failed to ping PostgreSQL database")

	t.Cleanup(func() {
		cleanupTestData(t, db.DB())
		db.Close()
	})

	return context.Background()
}

// cleanupTestData removes test data from PostgreSQL database
func cleanupTestData(t *testing.T, sqlDB *sql.DB) {
	t.Helper()
	ctx := context.Background()

	// Check which tables exist first to avoid PostgreSQL errors in logs
	existingTables := make(map[string]bool)
	rows, err := sqlDB.QueryContext(ctx, `
		SELECT tablename
		FROM pg_tables
		WHERE schemaname = 'public'
	`)
	if err != nil {
		t.Logf("failed to query existing tables: %v", err)
		return
	}
	defer rows.Close()

	for rows.Next() {
		var tableName string
		if err := rows.Scan(&tableName); err != nil {
			continue
		}
		existingTables[tableName] = true
	}

	// Tables with 'id' column (singular names from init_db.sql)
	tablesWithID := []string{
		"cart_items", "new_carts", "cart", "digital_file", "digital_data",
		"product_image", "product", "page", "setting", "subdomain", "users",
	}

	for _, table := range tablesWithID {
		if !existingTables[table] {
			continue // Skip non-existent tables
		}
		_, err := sqlDB.ExecContext(ctx,
			fmt.Sprintf("DELETE FROM %s WHERE id LIKE 'test_%%'", table))
		if err != nil {
			t.Logf("cleanup failed for %s: %v", table, err)
		}
	}

	// session table uses 'key' instead of 'id'
	if existingTables["session"] {
		_, err := sqlDB.ExecContext(ctx, "DELETE FROM session WHERE key LIKE 'test_%'")
		if err != nil {
			t.Logf("cleanup failed for session: %v", err)
		}
	}
}

// Test data helpers

// NewTestID generates a test-prefixed UUID for safe cleanup
func NewTestID() string {
	return "test_" + uuid.New().String()
}

// NewTestEmail generates a unique test email
func NewTestEmail() string {
	return fmt.Sprintf("test_%s@example.com", uuid.New().String()[:8])
}

// NewTestTimestamp returns current time for test data
func NewTestTimestamp() time.Time {
	return time.Now().UTC()
}
