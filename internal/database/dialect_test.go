package database

import "testing"

// The dialect fragments are the only engine-specific SQL in the project, so
// they are frozen here: a change to any of them should be a deliberate one.
func TestDialectFragments(t *testing.T) {
	tests := []struct {
		name    string
		dialect Dialect
		call    func(Dialect) string
		want    string
	}{
		{"sqlite driver", SQLite(), func(d Dialect) string { return d.Driver() }, "sqlite"},
		{"postgres driver", Postgres(), func(d Dialect) string { return d.Driver() }, "pgx"},
		{"sqlite goose dialect", SQLite(), func(d Dialect) string { return d.GooseDialect() }, "sqlite3"},
		{"postgres goose dialect", Postgres(), func(d Dialect) string { return d.GooseDialect() }, "postgres"},

		{"sqlite epoch", SQLite(), func(d Dialect) string { return d.Epoch("created") }, "strftime('%s', created)"},
		{"postgres epoch", Postgres(), func(d Dialect) string { return d.Epoch("created") }, "EXTRACT(EPOCH FROM created)::bigint"},

		{"sqlite json object", SQLite(), func(d Dialect) string { return d.JSONObject("'a', b") }, "json_object('a', b)"},
		{"postgres json object", Postgres(), func(d Dialect) string { return d.JSONObject("'a', b") }, "json_build_object('a', b)"},

		{"sqlite json agg", SQLite(), func(d Dialect) string { return d.JSONAgg("x") }, "json_group_array(x)"},
		{"postgres json agg", Postgres(), func(d Dialect) string { return d.JSONAgg("x") }, "json_agg(x)"},

		{"sqlite json value", SQLite(), func(d Dialect) string { return d.JSONValue("t.c") }, "json(t.c)"},
		{"postgres json value", Postgres(), func(d Dialect) string { return d.JSONValue("t.c") }, "(t.c)::json"},

		{
			"sqlite json bool",
			SQLite(),
			func(d Dialect) string { return d.JSONBool("t.active") },
			"CASE WHEN t.active = 1 THEN json('true') ELSE json('false') END",
		},
		{"postgres json bool", Postgres(), func(d Dialect) string { return d.JSONBool("t.active") }, "t.active"},

		{"sqlite rebind", SQLite(), func(d Dialect) string { return d.Rebind("SELECT * FROM t WHERE a = ?") }, "SELECT * FROM t WHERE a = ?"},
		{"postgres rebind", Postgres(), func(d Dialect) string { return d.Rebind("SELECT * FROM t WHERE a = ? AND b = ?") }, "SELECT * FROM t WHERE a = $1 AND b = $2"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.call(tt.dialect); got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestDialectFor(t *testing.T) {
	for _, driver := range []string{DriverSQLite, DriverPostgres} {
		d, err := DialectFor(driver)
		if err != nil {
			t.Fatalf("DialectFor(%q): %v", driver, err)
		}
		if d.Name() != driver {
			t.Errorf("DialectFor(%q).Name() = %q", driver, d.Name())
		}
	}

	if _, err := DialectFor("mysql"); err == nil {
		t.Error("DialectFor(\"mysql\") should fail")
	}
}
