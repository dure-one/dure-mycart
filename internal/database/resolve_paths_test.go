package database

import (
	"os"
	"strings"
	"testing"
)

// Resolve reads the fixed lc_base/config.json at the process working directory.
// This is the path a real installation uses, so it is worth one test that goes
// through WriteConfig and Resolve rather than their *File twins.
func TestResolveReadsTheInstalledConfiguration(t *testing.T) {
	t.Chdir(t.TempDir())

	want := Config{Driver: DriverPostgres, DSN: "postgres://user:pw@localhost:5432/cart", Source: SourceWizard}
	if err := WriteConfig(want); err != nil {
		t.Fatalf("WriteConfig: %v", err)
	}

	// The installer writes the file where the next start will look for it, with
	// the password unreadable to anyone else.
	if _, err := os.Stat(ConfigPath); err != nil {
		t.Fatalf("WriteConfig did not write %s: %v", ConfigPath, err)
	}
	info, err := os.Stat(ConfigPath)
	if err != nil {
		t.Fatalf("stat %s: %v", ConfigPath, err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Errorf("%s mode = %o, want 600 (it holds a password)", ConfigPath, perm)
	}

	cfg, err := Resolve(Overrides{})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if cfg.Driver != want.Driver || cfg.DSN != want.DSN {
		t.Errorf("Resolve = %+v, want the installed driver and DSN", cfg)
	}
	if cfg.Source != SourceFile {
		t.Errorf("Source = %q, want %q", cfg.Source, SourceFile)
	}
	// A file-configured database is deliberately not pinned: the install wizard
	// may still change it, unlike one named by a flag or the environment.
	if cfg.Pinned() {
		t.Error("config.json must not pin the database")
	}

	// And the environment still wins, which is how a container overrides the
	// file baked into its image.
	t.Setenv(EnvDSN, "./from-env.db")
	cfg, err = Resolve(Overrides{})
	if err != nil {
		t.Fatalf("Resolve with %s set: %v", EnvDSN, err)
	}
	if cfg.DSN != "./from-env.db" || cfg.Source != SourceEnv {
		t.Errorf("Resolve = %+v, want the environment to win", cfg)
	}
}

// A configuration file written for one working directory must not leak into
// another: without one, Resolve falls back to the built-in SQLite default.
func TestResolveWithoutAConfigFile(t *testing.T) {
	t.Chdir(t.TempDir())

	cfg, err := Resolve(Overrides{})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if cfg.Driver != DriverSQLite || cfg.DSN != DefaultSQLiteDSN || cfg.Source != SourceDefault {
		t.Errorf("Resolve = %+v, want the built-in default", cfg)
	}
	if cfg.Pinned() {
		t.Error("the built-in default must not be reported as pinned")
	}
}

// WriteConfig keeps the fields it was given and nothing else: the source of the
// configuration is a runtime notion, not part of the file.
func TestWriteConfigStoresOnlyDriverAndDSN(t *testing.T) {
	t.Chdir(t.TempDir())

	if err := WriteConfig(Config{Driver: DriverPostgres, DSN: "postgres://h/db", Source: SourceWizard}); err != nil {
		t.Fatalf("WriteConfig: %v", err)
	}

	raw, err := os.ReadFile(ConfigPath)
	if err != nil {
		t.Fatalf("read %s: %v", ConfigPath, err)
	}
	text := string(raw)
	if strings.Contains(text, SourceWizard) {
		t.Errorf("%s records the source, which it should not:\n%s", ConfigPath, text)
	}
	if !strings.Contains(text, "postgres://h/db") || !strings.Contains(text, DriverPostgres) {
		t.Errorf("%s lost the driver or the DSN:\n%s", ConfigPath, text)
	}
	if !strings.HasSuffix(text, "\n") {
		t.Errorf("%s does not end in a newline", ConfigPath)
	}
}
