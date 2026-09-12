package main

import (
	"testing"

	"github.com/shurco/mycart/internal/database"
)

// dbConfigFrom decides what `db copy` is talking to. A connection string
// mistaken for a file path would silently look for a database on disk, so the
// two forms are pinned here.
func TestDbConfigFrom(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		spec    string
		driver  string
		dsn     string
		wantErr bool
	}{
		{
			name:   "a postgres url",
			spec:   "postgres://mycart:secret@db.example.com:5432/mycart?sslmode=disable",
			driver: database.DriverPostgres,
			dsn:    "postgres://mycart:secret@db.example.com:5432/mycart?sslmode=disable",
		},
		{
			name:   "the postgresql spelling",
			spec:   "postgresql://db.example.com/mycart",
			driver: database.DriverPostgres,
			dsn:    "postgresql://db.example.com/mycart",
		},
		{
			name:   "key=value pairs",
			spec:   "host=db.example.com dbname=mycart",
			driver: database.DriverSQLite,
			dsn:    "host=db.example.com dbname=mycart",
		},
		{
			name:   "the default sqlite file",
			spec:   "./lc_base/data.db",
			driver: database.DriverSQLite,
			dsn:    "./lc_base/data.db",
		},
		{
			name:   "a path said out loud",
			spec:   "sqlite:./lc_base/data.db",
			driver: database.DriverSQLite,
			dsn:    "./lc_base/data.db",
		},
		{
			name:   "surrounded by spaces",
			spec:   "  ./lc_base/data.db  ",
			driver: database.DriverSQLite,
			dsn:    "./lc_base/data.db",
		},
		{name: "nothing at all", spec: "", wantErr: true},
		{name: "whitespace", spec: "   ", wantErr: true},
		{name: "a prefix with no path", spec: "sqlite:", wantErr: true},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			cfg, err := dbConfigFrom(c.spec)
			if c.wantErr {
				if err == nil {
					t.Fatalf("dbConfigFrom(%q) = %+v, want an error", c.spec, cfg)
				}
				return
			}
			if err != nil {
				t.Fatalf("dbConfigFrom(%q): %v", c.spec, err)
			}
			if cfg.Driver != c.driver {
				t.Errorf("driver = %q, want %q", cfg.Driver, c.driver)
			}
			if cfg.DSN != c.dsn {
				t.Errorf("dsn = %q, want %q", cfg.DSN, c.dsn)
			}
		})
	}
}
