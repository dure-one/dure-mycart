package queries

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/shurco/mycart/internal/database"
	"github.com/shurco/mycart/internal/models"
	"github.com/shurco/mycart/migrations"
)

func withTempBase(t *testing.T) func() {
	t.Helper()
	dir := t.TempDir()
	oldwd, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir to temp: %v", err)
	}
	_ = os.MkdirAll("lc_base", 0o775)
	_ = os.MkdirAll("lc_uploads", 0o775)
	_ = os.MkdirAll("lc_digitals", 0o775)
	return func() { _ = os.Chdir(oldwd) }
}

func Test_queries_init_and_settings(t *testing.T) {
	cleanup := withTempBase(t)
	defer cleanup()

	if err := New(database.Config{Driver: database.DriverSQLite, DSN: database.DefaultSQLiteDSN}, migrations.Embed()); err != nil {
		t.Fatalf("init queries: %v", err)
	}

	db := DB()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	setting, err := db.GetSettingByKey(ctx, "installed")
	if err != nil {
		t.Fatalf("get setting: %v", err)
	}
	if _, ok := setting["installed"]; !ok {
		t.Fatalf("installed key not found")
	}
}

func Test_queries_page_crud(t *testing.T) {
	cleanup := withTempBase(t)
	defer cleanup()
	if err := New(database.Config{Driver: database.DriverSQLite, DSN: database.DefaultSQLiteDSN}, migrations.Embed()); err != nil {
		t.Fatalf("init queries: %v", err)
	}
	db := DB()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	page, err := db.AddPage(ctx, &models.Page{Name: "Test", Slug: "test", Position: "footer"})
	if err != nil {
		t.Fatalf("add page: %v", err)
	}
	if page.Created == 0 {
		t.Fatalf("expected created timestamp")
	}

	list, _, err := db.ListPages(ctx, true, 0, 0)
	if err != nil {
		t.Fatalf("list pages: %v", err)
	}
	if len(list) == 0 {
		t.Fatalf("expected pages > 0")
	}

	if err := db.UpdatePageContent(ctx, &models.Page{Core: models.Core{ID: page.ID}, Content: ptr("content")}); err != nil {
		t.Fatalf("update content: %v", err)
	}
}

func ptr[T any](v T) *T { return &v }

// Swap is how a running process moves to another database — the install wizard
// switching an installation from SQLite to PostgreSQL uses it. It has to hand
// back the connection it replaced, because only the caller knows whether anyone
// is still holding a statement on it.
func TestSwapInstallsTheNewHandleAndReturnsTheOldOne(t *testing.T) {
	bootstrap(t) // installs a process-wide handle and gives us a temp workdir

	first := Conn()
	if first == nil {
		t.Fatal("bootstrap did not install a connection")
	}

	second, err := database.Connect(database.Config{
		Driver: database.DriverSQLite,
		DSN:    filepath.Join(t.TempDir(), "second.db"),
	})
	if err != nil {
		t.Fatalf("connect to a second database: %v", err)
	}
	t.Cleanup(func() { _ = second.Close() })

	old := Swap(second)
	if old != first {
		t.Errorf("Swap returned %p, want the handle it replaced (%p)", old, first)
	}
	if Conn() != second {
		t.Error("Swap did not install the new connection")
	}
	base := DB()
	if base == nil {
		t.Fatal("DB() is nil after Swap")
	}
	if base.conn != second {
		t.Error("the query groups still point at the old connection")
	}

	// Putting it back works the same way, and is what the tests around this one
	// rely on.
	if restored := Swap(first); restored != second {
		t.Errorf("restoring returned %p, want %p", restored, second)
	}
	if Conn() != first {
		t.Error("the original connection was not restored")
	}
}
