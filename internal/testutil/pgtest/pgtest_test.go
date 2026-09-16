package pgtest

import (
	"io/fs"
	"os"
	"strings"
	"testing"
)

func TestParseConfig(t *testing.T) {
	driver := os.Getenv("TEST_DB_DRIVER")
	if driver != "" && !strings.EqualFold(driver, "postgres") {
		t.Skip("PostgreSQL-specific test, skipping in SQLite-only mode")
	}

	tests := []struct {
		name     string
		dsn      string
		want     map[string]string // pgtestdb.Config fields, as strings
		wantOpts string
	}{
		{
			name: "url form",
			dsn:  "postgres://user:pw@db.example:5433/mycart_test?sslmode=disable",
			want: map[string]string{
				"driver": "pgx",
				"host":   "db.example",
				"port":   "5433",
				"user":   "user",
				"pass":   "pw",
				"db":     "mycart_test",
			},
			// url.Values.Encode sorts, so the pinned timezone lands after the DSN's own params.
			wantOpts: "sslmode=disable&timezone=UTC",
		},
		{
			name:     "missing port defaults to 5432",
			dsn:      "postgres://user@localhost/db",
			want:     map[string]string{"port": "5432", "db": "db", "pass": ""},
			wantOpts: "timezone=UTC",
		},
		{
			// pgtestdb administers the server through this connection, so when
			// the DSN names no database the maintenance one is the only usable
			// choice.
			name:     "missing database falls back to postgres",
			dsn:      "postgres://user@localhost:5432",
			want:     map[string]string{"db": "postgres"},
			wantOpts: "timezone=UTC",
		},
		{
			name:     "postgresql scheme is accepted",
			dsn:      "postgresql://user@localhost/db",
			want:     map[string]string{"db": "db"},
			wantOpts: "timezone=UTC",
		},
		{
			// A password with a URL delimiter is exactly what breaks when the
			// DSN is rebuilt by string interpolation instead of escaping.
			name:     "password with url delimiters",
			dsn:      "postgres://user:p%40ss%3Aword@localhost/db",
			want:     map[string]string{"pass": "p@ss:word"},
			wantOpts: "timezone=UTC",
		},
		{
			// An explicit timezone survives: the pin only fills a gap, and
			// database.Connect is what refuses a non-UTC session later.
			name:     "explicit timezone is left alone",
			dsn:      "postgres://user@localhost/db?timezone=Asia/Seoul",
			want:     map[string]string{"db": "db"},
			wantOpts: "timezone=Asia%2FSeoul",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, err := parseConfig(tt.dsn)
			if err != nil {
				t.Fatalf("parseConfig: %v", err)
			}

			got := map[string]string{
				"driver": cfg.DriverName,
				"host":   cfg.Host,
				"port":   cfg.Port,
				"user":   cfg.User,
				"pass":   cfg.Password,
				"db":     cfg.Database,
			}
			for field, want := range tt.want {
				if got[field] != want {
					t.Errorf("%s = %q, want %q", field, got[field], want)
				}
			}
			if cfg.Options != tt.wantOpts {
				t.Errorf("options = %q, want %q", cfg.Options, tt.wantOpts)
			}
		})
	}
}

func TestParseConfigRejectsUnusableDSNs(t *testing.T) {
	tests := []struct {
		name, dsn, want string
	}{
		{"empty", "", "empty"},
		{"keyword form", "host=localhost user=postgres", "postgres://"},
		{"no host", "postgres:///db", "postgres://"},
		{"wrong scheme", "mysql://user@localhost/db", "postgres://"},
		{"pgxpool option", "postgres://user@localhost/db?pool_max_conns=5", "pool_max_conns"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parseConfig(tt.dsn)
			if err == nil {
				t.Fatal("expected an error")
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Errorf("error = %q, want it to mention %q", err, tt.want)
			}
		})
	}
}

// The fixture template must be a different database from the plain one, and both
// keys must change when their files change. Only the first half is observable
// without editing files: the hashes have to differ.
func TestFixtureTemplateDoesNotShareTheSchemaTemplate(t *testing.T) {
	plain, err := migrator(false).Hash()
	if err != nil {
		t.Fatalf("plain hash: %v", err)
	}
	withFixtures, err := migrator(true).Hash()
	if err != nil {
		t.Fatalf("fixture hash: %v", err)
	}

	if plain == "" || withFixtures == "" {
		t.Fatal("a template hash must not be empty")
	}
	if plain == withFixtures {
		t.Error("the fixture template and the plain template hash to the same key")
	}
	if !strings.Contains(withFixtures, plain) {
		t.Errorf("the fixture hash %q does not build on the schema hash %q", withFixtures, plain)
	}
}

// FixturesFS is anchored to the source tree rather than the working directory,
// and it is what the SQLite path loads the fixtures from as well.
func TestFixturesFSFindsTheFixtureMigration(t *testing.T) {
	dir, err := fs.ReadDir(FixturesFS(), ".")
	if err != nil {
		t.Fatalf("read fixtures: %v", err)
	}

	var sql int
	for _, entry := range dir {
		if strings.HasSuffix(entry.Name(), ".sql") {
			sql++
		}
	}
	if sql == 0 {
		t.Fatal("no fixture migrations found")
	}
}
