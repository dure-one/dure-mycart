package database

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolveOrder(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")

	if err := WriteConfigFile(Config{Driver: DriverPostgres, DSN: "postgres://file/db"}, path); err != nil {
		t.Fatalf("write config: %v", err)
	}

	t.Run("file overrides the default", func(t *testing.T) {
		cfg, err := ResolveFile(Overrides{}, path)
		if err != nil {
			t.Fatalf("resolve: %v", err)
		}
		if cfg.Driver != DriverPostgres || cfg.DSN != "postgres://file/db" || cfg.Source != SourceFile {
			t.Errorf("got %+v", cfg)
		}
	})

	t.Run("environment overrides the file", func(t *testing.T) {
		t.Setenv(EnvDriver, DriverSQLite)
		t.Setenv(EnvDSN, "./from-env.db")

		cfg, err := ResolveFile(Overrides{}, path)
		if err != nil {
			t.Fatalf("resolve: %v", err)
		}
		if cfg.Driver != DriverSQLite || cfg.DSN != "./from-env.db" || cfg.Source != SourceEnv {
			t.Errorf("got %+v", cfg)
		}
	})

	t.Run("flags override everything", func(t *testing.T) {
		t.Setenv(EnvDriver, DriverSQLite)
		t.Setenv(EnvDSN, "./from-env.db")

		cfg, err := ResolveFile(Overrides{Driver: DriverPostgres, DSN: "postgres://flag/db"}, path)
		if err != nil {
			t.Fatalf("resolve: %v", err)
		}
		if cfg.Driver != DriverPostgres || cfg.DSN != "postgres://flag/db" || cfg.Source != SourceFlag {
			t.Errorf("got %+v", cfg)
		}
	})
}

func TestResolveDefaultIsSQLite(t *testing.T) {
	cfg, err := ResolveFile(Overrides{}, filepath.Join(t.TempDir(), "missing.json"))
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if cfg.Driver != DriverSQLite || cfg.DSN != DefaultSQLiteDSN || cfg.Source != SourceDefault {
		t.Errorf("got %+v, want the built-in SQLite default", cfg)
	}
}

func TestResolveRejectsBadInput(t *testing.T) {
	tests := []struct {
		name      string
		overrides Overrides
	}{
		{"unknown driver", Overrides{Driver: "mysql"}},
		{"postgres without a DSN", Overrides{Driver: DriverPostgres}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := ResolveFile(tt.overrides, filepath.Join(t.TempDir(), "missing.json")); err == nil {
				t.Error("expected an error")
			}
		})
	}
}

func TestResolveInfersPostgresFromDSN(t *testing.T) {
	cfg, err := ResolveFile(Overrides{DSN: "postgres://user:pw@host:5432/db"}, filepath.Join(t.TempDir(), "missing.json"))
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if cfg.Driver != DriverPostgres {
		t.Errorf("driver = %q, want postgres", cfg.Driver)
	}
}

func TestConfigPinned(t *testing.T) {
	tests := map[string]bool{
		SourceFlag:    true,
		SourceEnv:     true,
		SourceFile:    false,
		SourceDefault: false,
		SourceWizard:  false,
	}

	for source, want := range tests {
		if got := (Config{Source: source}).Pinned(); got != want {
			t.Errorf("Pinned() with source %q = %v, want %v", source, got, want)
		}
	}
}

// A DSN reaches the startup banner and the unauthenticated install status
// endpoint, so every place pgx accepts a password has to be masked — not only
// the one in the URL's userinfo.
func TestConfigRedacted(t *testing.T) {
	const password = "sup3rs3cret"

	tests := []struct {
		name string
		cfg  Config
		// want is the exact rendering, when it is worth pinning.
		want string
	}{
		{
			name: "url form",
			cfg:  Config{Driver: DriverPostgres, DSN: "postgres://user:" + password + "@db:5432/mycart?sslmode=require"},
			want: "postgres://user:***@db:5432/mycart?sslmode=require",
		},
		{
			name: "password as a query parameter",
			cfg:  Config{Driver: DriverPostgres, DSN: "postgres://user@db:5432/mycart?sslmode=require&password=" + password},
		},
		{
			name: "sslpassword as a query parameter",
			cfg:  Config{Driver: DriverPostgres, DSN: "postgres://user@db:5432/mycart?sslpassword=" + password},
		},
		{
			name: "keyword form",
			cfg:  Config{Driver: DriverPostgres, DSN: "host=db user=user password=" + password + " dbname=mycart"},
			want: "host=db user=user password=*** dbname=mycart",
		},
		{
			name: "sslpassword in the keyword form",
			cfg:  Config{Driver: DriverPostgres, DSN: "host=db sslpassword=" + password},
			want: "host=db sslpassword=***",
		},
		{
			name: "a DSN without a password is left alone",
			cfg:  Config{Driver: DriverPostgres, DSN: "postgres://user@db:5432/mycart?sslmode=require"},
			want: "postgres://user@db:5432/mycart?sslmode=require",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.cfg.Redacted()
			if strings.Contains(got, password) {
				t.Errorf("Redacted() leaked the password: %s", got)
			}
			if tt.want != "" && got != tt.want {
				t.Errorf("Redacted() = %q, want %q", got, tt.want)
			}
		})
	}

	t.Run("sqlite is unchanged", func(t *testing.T) {
		cfg := Config{Driver: DriverSQLite, DSN: "./lc_base/data.db"}
		if got := cfg.Redacted(); got != cfg.DSN {
			t.Errorf("Redacted() = %q, want %q", got, cfg.DSN)
		}
	})
}

func TestWriteConfigIsPrivate(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "config.json")

	if err := WriteConfigFile(Config{Driver: DriverSQLite, DSN: "./lc_base/data.db"}, path); err != nil {
		t.Fatalf("write config: %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	// The file holds the database password.
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Errorf("config permissions = %o, want 600", perm)
	}
}

func TestNormalizePostgresDSN(t *testing.T) {
	tests := []struct {
		name    string
		dsn     string
		want    string
		wantErr bool
	}{
		{
			name: "pin the timezone",
			dsn:  "postgres://user:pw@host:5432/db?sslmode=require",
			want: "postgres://user:pw@host:5432/db?sslmode=require&timezone=UTC",
		},
		{
			name: "an explicit timezone is respected",
			dsn:  "postgres://user:pw@host:5432/db?timezone=UTC",
			want: "postgres://user:pw@host:5432/db?timezone=UTC",
		},
		{
			// A DSN that already pins the timezone through `options` is left
			// alone; re-encoding normalises its escaping, which is harmless.
			name: "options are respected",
			dsn:  "postgres://user:pw@host:5432/db?options=-c%20timezone%3DUTC",
			want: "postgres://user:pw@host:5432/db?options=-c+timezone%3DUTC",
		},
		{
			name: "keyword form gets the timezone",
			dsn:  "host=db user=user dbname=mycart",
			want: "host=db user=user dbname=mycart timezone=UTC",
		},
		{
			name:    "pgxpool options are rejected",
			dsn:     "postgres://user:pw@host:5432/db?pool_max_conns=10",
			wantErr: true,
		},
		{
			name:    "empty",
			dsn:     "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NormalizePostgresDSN(tt.dsn)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected an error, got %q", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}
