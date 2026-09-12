package migrations_test

import (
	"database/sql"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/shurco/mycart/internal/database"
	"github.com/shurco/mycart/migrations"
)

// appliedVersions is the set of migrations every existing installation has
// already run, in the order goose applies them.
//
// It is frozen on purpose. goose records the *version*, not the contents, so
// editing an applied migration changes the schema of new installations only:
// an existing cart never sees the fix and never notices the drift. Whenever a
// change is needed, add a new migration — never renumber, rename or reorder
// these, and never delete one.
//
// (The PostgreSQL port did edit these files, which is safe exactly once, for
// the reason recorded there: the release that ships a database choice has not
// been built yet, and no installation can exist that has run the old text.)
var appliedVersions = []string{
	"20230714135923_init_db",
	"20230926150007_seo",
	"20231110193417_added_payment_field",
	"20231114104637_litepay",
	"20231124220911_paypal",
	"20231129131044_brief",
	"20231208185127_mail",
	"20240111145752_new_sicials",
	"20240111145753_fix_smtp_port",
	"20260127120000_coinbase",
	"20260418120000_fix_socials",
	"20260714000001_portone",
	"20260720012908_portone_enhancements",
	"20260721120000_product_variants",
	"20260722000000_fix_metadata_default",
	"20260821000000_add_missing_indexes",
}

// TestMigrationHistoryIsAppendOnly checks that the migration set on disk is the
// frozen set plus new files appended at the end.
func TestMigrationHistoryIsAppendOnly(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("read migrations directory: %v", err)
	}

	var onDisk []string
	contents := map[string]string{}
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".sql") {
			continue
		}
		onDisk = append(onDisk, strings.TrimSuffix(name, ".sql"))

		body, err := os.ReadFile(name)
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		contents[name] = string(body)
	}
	sort.Strings(onDisk)

	for i, want := range appliedVersions {
		if i >= len(onDisk) {
			t.Fatalf("migration %s is missing from the directory", want)
		}
		if onDisk[i] != want {
			t.Fatalf("migration %d is %s, want %s: applied migrations must never be renamed, reordered or removed",
				i, onDisk[i], want)
		}
	}

	// New migrations must sort after every frozen one, or goose would apply
	// them out of order on a fresh database.
	latest := appliedVersions[len(appliedVersions)-1]
	for _, name := range onDisk[len(appliedVersions):] {
		version, _, _ := strings.Cut(name, "_")
		if len(version) != 14 {
			t.Errorf("new migration %s does not start with a 14-digit timestamp", name)
		}
		if version <= latest[:14] {
			t.Errorf("new migration %s sorts before %s; migrations are append-only", name, latest)
		}
	}

	// Every migration must be reversible and carry both sections: goose needs
	// the Down half to roll back, and a file with only an Up silently breaks
	// `goose down`.
	for name, body := range contents {
		if !strings.Contains(body, "-- +goose Up") {
			t.Errorf("%s has no Up section", name)
		}
		if !strings.Contains(body, "-- +goose Down") {
			t.Errorf("%s has no Down section", name)
		}
		if upSection(body) == "" {
			t.Errorf("%s applies nothing", name)
		}
	}
}

// upSection returns the statements of a migration's Up half, with the goose
// directives and comments stripped. goose splits on semicolons unless a
// StatementBegin block says otherwise, so the exact framing is not asserted —
// only that there is something to run.
func upSection(body string) string {
	_, after, found := strings.Cut(body, "-- +goose Up")
	if !found {
		return ""
	}
	up, _, _ := strings.Cut(after, "-- +goose Down")

	var statements []string
	for _, line := range strings.Split(up, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "--") {
			continue
		}
		statements = append(statements, line)
	}
	return strings.Join(statements, "\n")
}

// TestMigrateIsIdempotent reopens an existing SQLite database, which is what
// every upgrade of an existing installation does, and checks that nothing is
// applied twice and nothing existing is touched.
func TestMigrateIsIdempotent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "existing.db")
	cfg := database.Config{Driver: database.DriverSQLite, DSN: path}

	conn, err := database.Open(cfg, migrations.Embed())
	if err != nil {
		t.Fatalf("first migrate: %v", err)
	}

	// Something an operator would already have in the database.
	if _, err := conn.ExecContext(t.Context(),
		`INSERT INTO page (id, name, slug, position, active) VALUES (?, ?, ?, 'footer', TRUE)`,
		"existingpage001", "Existing Page", "existing-page"); err != nil {
		_ = conn.Close()
		t.Fatalf("insert page: %v", err)
	}
	before := schemaVersions(t, conn.Raw())
	_ = conn.Close()

	conn, err = database.Open(cfg, migrations.Embed())
	if err != nil {
		t.Fatalf("second migrate: %v", err)
	}
	defer func() { _ = conn.Close() }()

	if after := schemaVersions(t, conn.Raw()); !equalStrings(before, after) {
		t.Errorf("re-running migrations changed the recorded versions: %v → %v", before, after)
	}

	var count int
	if err := conn.QueryRowContext(t.Context(),
		`SELECT COUNT(*) FROM page WHERE slug = ?`, "existing-page").Scan(&count); err != nil {
		t.Fatalf("read page: %v", err)
	}
	if count != 1 {
		t.Errorf("the existing row did not survive re-running migrations")
	}
}

// schemaVersions reads the versions goose has recorded, in the order applied.
func schemaVersions(t *testing.T, raw *sql.DB) []string {
	t.Helper()

	rows, err := raw.Query(`SELECT version_id FROM ` + database.MigrateTable + ` ORDER BY id`)
	if err != nil {
		t.Fatalf("read %s: %v", database.MigrateTable, err)
	}
	defer rows.Close()

	var versions []string
	for rows.Next() {
		var version int64
		if err := rows.Scan(&version); err != nil {
			t.Fatalf("scan version: %v", err)
		}
		versions = append(versions, strconv.FormatInt(version, 10))
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("read versions: %v", err)
	}
	return versions
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
