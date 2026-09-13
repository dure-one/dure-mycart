package migrations

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/shurco/mycart/internal/base"
	_ "modernc.org/sqlite"
)

// TestProductImageOrderingMigration verifies that the product_image table
// has position and is_representative columns with correct defaults after migration.
func TestProductImageOrderingMigration(t *testing.T) {
	// Not parallel: goose mutates package-level state
	tmp := t.TempDir()
	prev, _ := os.Getwd()
	_ = os.Chdir(tmp)
	t.Cleanup(func() { _ = os.Chdir(prev) })

	dbPath := filepath.Join(tmp, "lc_base", "test.db")
	db, err := base.New(dbPath, Embed())
	if err != nil {
		t.Fatalf("base.New: %v", err)
	}
	defer func() { _ = db.Close() }()

	t.Run("is_representative column exists with default false", func(t *testing.T) {
		var colType string
		var defaultVal interface{}
		err := db.QueryRow(`
			SELECT type, "default" FROM pragma_table_info('product_image')
			WHERE name = 'is_representative'
		`).Scan(&colType, &defaultVal)
		if err != nil {
			t.Fatalf("is_representative column not found: %v", err)
		}
		if colType != "BOOLEAN" {
			t.Errorf("is_representative should be BOOLEAN, got %s", colType)
		}
	})

	t.Run("position column exists", func(t *testing.T) {
		var colType string
		err := db.QueryRow(`
			SELECT type FROM pragma_table_info('product_image')
			WHERE name = 'position'
		`).Scan(&colType)
		if err != nil {
			t.Fatalf("position column not found: %v", err)
		}
		if colType != "INTEGER" {
			t.Errorf("position should be INTEGER, got %s", colType)
		}
	})

	t.Run("can insert and query product_image with default is_representative", func(t *testing.T) {
		// Insert a test product and image
		_, err := db.Exec(`
			INSERT INTO product (id, name, desc, slug, amount, active)
			VALUES ('test-prod', 'Test Product', 'Test', 'test-slug', 99.99, true)
		`)
		if err != nil {
			t.Fatalf("failed to insert product: %v", err)
		}

		_, err = db.Exec(`
			INSERT INTO product_image (id, product_id, name, ext, orig_name)
			VALUES ('test-img-1', 'test-prod', 'image.jpg', 'jpg', 'image.jpg')
		`)
		if err != nil {
			t.Fatalf("failed to insert product_image: %v", err)
		}

		var isRep bool
		var position int
		err = db.QueryRow(`
			SELECT is_representative, COALESCE(position, 0) FROM product_image WHERE id = 'test-img-1'
		`).Scan(&isRep, &position)
		if err != nil {
			t.Fatalf("failed to query product_image: %v", err)
		}
		if isRep {
			t.Errorf("is_representative should default to false, got %v", isRep)
		}
	})

	t.Run("can update is_representative", func(t *testing.T) {
		_, err := db.Exec(`
			UPDATE product_image SET is_representative = 1 WHERE id = 'test-img-1'
		`)
		if err != nil {
			t.Fatalf("failed to update is_representative: %v", err)
		}

		var isRep bool
		err = db.QueryRow(`
			SELECT is_representative FROM product_image WHERE id = 'test-img-1'
		`).Scan(&isRep)
		if err != nil {
			t.Fatalf("failed to query after update: %v", err)
		}
		if !isRep {
			t.Errorf("is_representative should be true after update, got %v", isRep)
		}
	})
}
