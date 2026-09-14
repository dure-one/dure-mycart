package app

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shurco/mycart/internal/database"
	"github.com/shurco/mycart/internal/dbtransfer"
	"github.com/shurco/mycart/internal/testutil/pgtest"
)

// requirePostgres skips a test unless the suite is pointed at a test server.
func requirePostgres(t *testing.T) {
	t.Helper()

	if !strings.EqualFold(os.Getenv("TEST_DB_DRIVER"), database.DriverPostgres) {
		t.Skip("set TEST_DB_DRIVER=postgres and TEST_POSTGRES_DSN to run")
	}
	if os.Getenv(pgtest.AdminDSN) == "" {
		t.Skipf("%s is not set", pgtest.AdminDSN)
	}
}

func postgresConfig(dsn string) database.Config {
	return database.Config{Driver: database.DriverPostgres, DSN: dsn, Source: database.SourceFlag}
}

func sqliteConfig(path string) database.Config {
	return database.Config{Driver: database.DriverSQLite, DSN: path, Source: database.SourceFlag}
}

// seedSetting writes a setting the way an installer would, so a dump has
// something in it beyond the rows the migrations seed.
func seedSetting(t *testing.T, cfg database.Config, key, value string) {
	t.Helper()

	conn, err := database.Connect(cfg)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer func() { _ = conn.Close() }()

	if _, err := conn.ExecContext(t.Context(),
		`UPDATE setting SET value = ? WHERE key = ?`, value, key); err != nil {
		t.Fatalf("set %s: %v", key, err)
	}
}

func readSetting(t *testing.T, cfg database.Config, key string) string {
	t.Helper()

	conn, err := database.Connect(cfg)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer func() { _ = conn.Close() }()

	var value string
	if err := conn.QueryRowContext(t.Context(),
		`SELECT value FROM setting WHERE key = ?`, key).Scan(&value); err != nil {
		t.Fatalf("read %s: %v", key, err)
	}
	return value
}

// The whole point of a backup: an operator writes one file and gets a working
// shop back out of it.
func TestBackupAndRestoreDatabase(t *testing.T) {
	requirePostgres(t)

	src := postgresConfig(pgtest.MigratedDSN(t))
	seedSetting(t, src, "domain", "shop.example.com")

	path := filepath.Join(t.TempDir(), "backup.sql.gz")
	backed, err := BackupDatabase(t.Context(), src, path)
	if err != nil {
		t.Fatalf("backup: %v", err)
	}
	if backed.Rows == 0 || len(backed.Tables) == 0 {
		t.Fatal("the backup reported nothing to write")
	}

	// The file is a gzip stream, and it is whole: a backup cut short is the
	// failure that matters most here.
	file, err := os.Open(path)
	if err != nil {
		t.Fatalf("open the backup: %v", err)
	}
	defer func() { _ = file.Close() }()

	gz, err := gzip.NewReader(file)
	if err != nil {
		t.Fatalf("read the backup as gzip: %v", err)
	}
	if _, err := io.ReadAll(gz); err != nil {
		t.Fatalf("read the backup: %v", err)
	}

	// Restoring brings the data back, and the dump is what it was restored
	// from, table for table.
	dst := postgresConfig(pgtest.MigratedDSN(t))
	restored, err := RestoreDatabase(t.Context(), dst, path, false)
	if err != nil {
		t.Fatalf("restore: %v", err)
	}
	if restored.Rows != backed.Rows || len(restored.Tables) != len(backed.Tables) {
		t.Errorf("restored %d rows in %d tables, the backup holds %d in %d",
			restored.Rows, len(restored.Tables), backed.Rows, len(backed.Tables))
	}
	if got := readSetting(t, dst, "domain"); got != "shop.example.com" {
		t.Errorf("domain = %q, want the backed-up value", got)
	}
}

// A dump that is not compressed is just as valid: the file is read for what it
// holds, not for what its name says.
func TestBackupAndRestoreUncompressed(t *testing.T) {
	requirePostgres(t)

	src := postgresConfig(pgtest.MigratedDSN(t))
	seedSetting(t, src, "domain", "plain.example.com")

	path := filepath.Join(t.TempDir(), "backup.sql")
	if _, err := BackupDatabase(t.Context(), src, path); err != nil {
		t.Fatalf("backup: %v", err)
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read the backup: %v", err)
	}
	if !bytes.HasPrefix(raw, []byte("-- myCart database dump\n")) {
		t.Fatalf("the backup starts with %q, want the dump banner", firstLine(raw))
	}

	dst := postgresConfig(pgtest.MigratedDSN(t))
	if _, err := RestoreDatabase(t.Context(), dst, path, false); err != nil {
		t.Fatalf("restore: %v", err)
	}
	if got := readSetting(t, dst, "domain"); got != "plain.example.com" {
		t.Errorf("domain = %q, want the backed-up value", got)
	}
}

func firstLine(raw []byte) string {
	line, _, _ := bytes.Cut(raw, []byte("\n"))
	return string(line)
}

// The file is created before the dump is read, so a failure has to leave
// nothing behind: a half-written backup is a file that looks like a shop.
func TestBackupLeavesNothingBehindWhenItFails(t *testing.T) {
	// Not parallel: it asserts on a directory it creates.
	dir := t.TempDir()
	path := filepath.Join(dir, "backup.sql.gz")

	// A PostgreSQL connection string nothing is listening on, so the dump
	// fails after the file exists.
	cfg := postgresConfig("postgres://user@127.0.0.1:1/shop?sslmode=disable")

	if _, err := BackupDatabase(t.Context(), cfg, path); err == nil {
		t.Fatal("a backup from a database that cannot be reached was reported as done")
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read the directory: %v", err)
	}
	if len(entries) != 0 {
		names := make([]string, len(entries))
		for i, entry := range entries {
			names[i] = entry.Name()
		}
		t.Errorf("the failed backup left %v behind", names)
	}
}

// A restore into a database that has never held a cart has to create the schema
// first: the dump carries data only.
func TestRestoreMigratesTheTargetFirst(t *testing.T) {
	requirePostgres(t)

	src := postgresConfig(pgtest.MigratedDSN(t))
	seedSetting(t, src, "domain", "fresh.example.com")

	path := filepath.Join(t.TempDir(), "backup.sql")
	if _, err := BackupDatabase(t.Context(), src, path); err != nil {
		t.Fatalf("backup: %v", err)
	}

	empty := postgresConfig(pgtest.EmptyDSN(t))
	if _, err := RestoreDatabase(t.Context(), empty, path, false); err != nil {
		t.Fatalf("restore into an empty database: %v", err)
	}
	if got := readSetting(t, empty, "domain"); got != "fresh.example.com" {
		t.Errorf("domain = %q, want the backed-up value", got)
	}
}

func TestRestoreRefusesAnInstallationWithoutForce(t *testing.T) {
	requirePostgres(t)

	src := postgresConfig(pgtest.MigratedDSN(t))
	path := filepath.Join(t.TempDir(), "backup.sql")
	if _, err := BackupDatabase(t.Context(), src, path); err != nil {
		t.Fatalf("backup: %v", err)
	}

	live := postgresConfig(pgtest.MigratedDSN(t))
	seedSetting(t, live, "installed", "true")
	seedSetting(t, live, "domain", "the-live-shop.example.com")

	_, err := RestoreDatabase(t.Context(), live, path, false)
	if err == nil {
		t.Fatal("a restore over a live shop was allowed without --force")
	}
	if !strings.Contains(err.Error(), "--force") {
		t.Errorf("error = %v, want it to say how to proceed", err)
	}
	if got := readSetting(t, live, "domain"); got != "the-live-shop.example.com" {
		t.Errorf("domain = %q, want the live shop untouched", got)
	}

	if _, err := RestoreDatabase(t.Context(), live, path, true); err != nil {
		t.Fatalf("restore with force: %v", err)
	}
	if got := readSetting(t, live, "domain"); got == "the-live-shop.example.com" {
		t.Error("the forced restore did not replace the live shop")
	}
}

func TestBackupAndRestoreRefuseSQLite(t *testing.T) {
	// Not parallel: Migrate drives goose, whose configuration is global.
	dir := t.TempDir()
	path := filepath.Join(dir, "data.db")
	cfg := sqliteConfig(path)

	if err := Migrate(cfg); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	_, err := BackupDatabase(context.Background(), cfg, filepath.Join(dir, "backup.sql"))
	if err == nil {
		t.Fatal("a SQLite cart was backed up as if it were PostgreSQL")
	}
	if !strings.Contains(err.Error(), "single file") {
		t.Errorf("error = %v, want it to point at copying the file", err)
	}

	if _, err := RestoreDatabase(context.Background(), cfg, filepath.Join(dir, "backup.sql"), false); err == nil {
		t.Fatal("a SQLite cart was restored into")
	}
	if _, err := os.Stat(filepath.Join(dir, "backup.sql")); !os.IsNotExist(err) {
		t.Error("the refused backup left a file behind")
	}
}

func TestBackupRejectsAnEmptyOutputPath(t *testing.T) {
	t.Parallel()

	cfg := postgresConfig("postgres://user@localhost:5432/shop")
	if _, err := BackupDatabase(context.Background(), cfg, "  "); err == nil {
		t.Fatal("a backup with no output file was written somewhere")
	}
}

func TestRestoreRejectsWhatIsNotADump(t *testing.T) {
	requirePostgres(t)

	dir := t.TempDir()
	dst := postgresConfig(pgtest.MigratedDSN(t))
	seedSetting(t, dst, "domain", "still-here.example.com")

	t.Run("a file that does not exist", func(t *testing.T) {
		if _, err := RestoreDatabase(t.Context(), dst, filepath.Join(dir, "nope.sql"), false); err == nil {
			t.Error("a restore of a file that is not there was attempted")
		}
	})

	t.Run("a file that is not a dump", func(t *testing.T) {
		path := filepath.Join(dir, "notes.txt")
		if err := os.WriteFile(path, []byte("just some notes\n"), 0o600); err != nil {
			t.Fatalf("write the file: %v", err)
		}

		_, err := RestoreDatabase(t.Context(), dst, path, false)
		if err == nil || !strings.Contains(err.Error(), "not a myCart dump") {
			t.Errorf("error = %v, want the file to be refused", err)
		}
	})

	t.Run("a truncated gzip stream", func(t *testing.T) {
		src := postgresConfig(pgtest.MigratedDSN(t))
		full := filepath.Join(dir, "backup.sql.gz")
		if _, err := BackupDatabase(t.Context(), src, full); err != nil {
			t.Fatalf("backup: %v", err)
		}

		raw, err := os.ReadFile(full)
		if err != nil {
			t.Fatalf("read the backup: %v", err)
		}
		cut := filepath.Join(dir, "cut.sql.gz")
		if err := os.WriteFile(cut, raw[:len(raw)/2], 0o600); err != nil {
			t.Fatalf("write the cut backup: %v", err)
		}

		_, err = RestoreDatabase(t.Context(), dst, cut, false)
		if err == nil {
			t.Error("a backup cut in half was restored")
		}
	})

	// Every refusal left the target as it was.
	if got := readSetting(t, dst, "domain"); got != "still-here.example.com" {
		t.Errorf("domain = %q, want the target untouched by the refused restores", got)
	}
}

func TestCopyDatabaseMovesASQLiteCart(t *testing.T) {
	requirePostgres(t)

	dir := t.TempDir()
	src := sqliteConfig(filepath.Join(dir, "data.db"))
	if err := Migrate(src); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	seedSetting(t, src, "domain", "moved.example.com")

	dst := postgresConfig(pgtest.MigratedDSN(t))
	manifest, err := CopyDatabase(t.Context(), src, dst, dbtransfer.CopyOptions{})
	if err != nil {
		t.Fatalf("copy: %v", err)
	}
	if manifest.Rows == 0 {
		t.Fatal("the copy reported nothing moved")
	}
	if got := readSetting(t, dst, "domain"); got != "moved.example.com" {
		t.Errorf("domain = %q, want the copied value", got)
	}
}

func TestCopyDatabaseReportsWhatItCannotDo(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	src := sqliteConfig(filepath.Join(dir, "data.db"))
	dst := sqliteConfig(filepath.Join(dir, "target.db"))

	_, err := CopyDatabase(context.Background(), src, dst, dbtransfer.CopyOptions{})
	if err == nil {
		t.Fatal("a copy into SQLite was allowed")
	}
	if !strings.Contains(err.Error(), "PostgreSQL") {
		t.Errorf("error = %v, want it to say what a target has to be", err)
	}
}

// decompress has to pass a plain file through and gunzip a compressed one,
// which is how a dump written years ago by another myCart still restores.
func TestDecompress(t *testing.T) {
	t.Parallel()

	var plain bytes.Buffer
	plain.WriteString("-- myCart database dump\n")
	reader, closer, err := decompress(bufio.NewReader(&plain))
	if err != nil {
		t.Fatalf("decompress plain: %v", err)
	}
	if err := closer(); err != nil {
		t.Errorf("closing a plain dump: %v", err)
	}
	got, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("read plain: %v", err)
	}
	if string(got) != "-- myCart database dump\n" {
		t.Errorf("plain dump read back as %q", got)
	}

	var compressed bytes.Buffer
	gz := gzip.NewWriter(&compressed)
	if _, err := gz.Write([]byte("-- myCart database dump\n")); err != nil {
		t.Fatalf("compress: %v", err)
	}
	if err := gz.Close(); err != nil {
		t.Fatalf("compress: %v", err)
	}

	reader, closer, err = decompress(bufio.NewReader(&compressed))
	if err != nil {
		t.Fatalf("decompress gzip: %v", err)
	}
	defer func() { _ = closer() }()
	got, err = io.ReadAll(reader)
	if err != nil {
		t.Fatalf("read gzip: %v", err)
	}
	if string(got) != "-- myCart database dump\n" {
		t.Errorf("compressed dump read back as %q", got)
	}
}

func TestDecompressRejectsABrokenGzipStream(t *testing.T) {
	t.Parallel()

	broken := bufio.NewReader(bytes.NewReader([]byte{0x1f, 0x8b, 0x00, 0x00}))
	if _, _, err := decompress(broken); err == nil {
		t.Error("a file that only starts like gzip was accepted as one")
	}
}

func TestDecompressReportsAReadFailure(t *testing.T) {
	t.Parallel()

	if _, _, err := decompress(bufio.NewReader(&failingReader{})); err == nil {
		t.Error("a reader that fails was reported as an empty file")
	}
}

// failingReader refuses to hand over anything at all.
type failingReader struct{}

func (failingReader) Read([]byte) (int, error) {
	return 0, errors.New("the file went away")
}
