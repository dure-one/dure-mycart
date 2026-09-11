package slugify

import (
	"context"
	"database/sql"
	"fmt"
	"testing"

	"github.com/shurco/mycart/internal/store/db"
	_ "modernc.org/sqlite"
)

func setupTestDB(t *testing.T) *sql.DB {
	sqlDB, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("Failed to open test database: %v", err)
	}

	// Create product table with deleted column for sqlc queries
	_, err = sqlDB.Exec(`
		CREATE TABLE product (
			id TEXT PRIMARY KEY,
			slug TEXT UNIQUE NOT NULL,
			deleted BOOLEAN DEFAULT 0
		)
	`)
	if err != nil {
		t.Fatalf("Failed to create test table: %v", err)
	}

	// Initialize db function pointers with this test database
	if err := db.InitFromDB(sqlDB, "sqlite"); err != nil {
		t.Fatalf("Failed to initialize db function pointers: %v", err)
	}

	return sqlDB
}

func TestSlugGeneration(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	service := NewSlugService(db)
	ctx := context.Background()

	tests := []struct {
		name      string
		input     string
		existing  []string
		want      string
		excludeID string
	}{
		{
			name:     "basic slug",
			input:    "Yoga Strap",
			existing: []string{},
			want:     "yoga-strap",
		},
		{
			name:     "special characters",
			input:    "T-Shirt!!! @#$ (Medium)",
			existing: []string{},
			want:     "t-shirt-at-medium",
		},
		{
			name:     "duplicate handling",
			input:    "Yoga Strap",
			existing: []string{"yoga-strap"},
			want:     "yoga-strap-2",
		},
		{
			name:     "multiple duplicates",
			input:    "Yoga Strap",
			existing: []string{"yoga-strap", "yoga-strap-2"},
			want:     "yoga-strap-3",
		},
		{
			name:      "exclude own ID",
			input:     "Yoga Strap",
			existing:  []string{"yoga-strap"},
			want:      "yoga-strap",
			excludeID: "existing-id",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clear table
			_, _ = db.Exec("DELETE FROM product")

			// Insert existing slugs
			for i, slug := range tt.existing {
				id := fmt.Sprintf("test-id-%d", i)
				if tt.excludeID != "" && i == 0 {
					id = tt.excludeID
				}
				_, err := db.Exec("INSERT INTO product (id, slug) VALUES (?, ?)", id, slug)
				if err != nil {
					t.Fatalf("Failed to insert test data: %v", err)
				}
			}

			got, err := service.Generate(ctx, tt.input, tt.excludeID)
			if err != nil {
				t.Errorf("Generate() error = %v", err)
				return
			}
			if got != tt.want {
				t.Errorf("Generate() = %v, want %v", got, tt.want)
			}
		})
	}
}
