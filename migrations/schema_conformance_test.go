package migrations_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/shurco/mycart/internal/database"
	"github.com/shurco/mycart/internal/testutil/pgtest"
	"github.com/shurco/mycart/migrations"
)

// EnvPostgresDSN names the PostgreSQL server the conformance test migrates on.
// It is the same variable the rest of the suite uses, so one PostgreSQL
// container serves both.
const EnvPostgresDSN = pgtest.AdminDSN

// column is one column of the logical schema, normalised across engines.
type column struct {
	name    string
	kind    string // bool | text | timestamp | int | numeric
	notNull bool
}

// table is the logical shape of one table: the parts of the DDL both engines
// have to agree on for one query layer to serve both.
type table struct {
	columns []column
	primary string   // primary key columns, comma-joined
	unique  []string // each UNIQUE constraint as a comma-joined column list
	indexes []string // each explicit index as "name kind(cols)"
}

// TestSchemaConformance runs the single migration set on both engines and
// compares the schemas it produces.
//
// The migrations are shared, so the schemas must be equivalent — that is the
// premise of the portability work. Without this test a migration using a
// SQLite-only type, or leaning on an implicit rowid, would pass every SQLite
// check and fail only on a PostgreSQL installation.
func TestSchemaConformance(t *testing.T) {
	if os.Getenv(EnvPostgresDSN) == "" {
		t.Skipf("set %s to compare the schemas of both engines", EnvPostgresDSN)
	}

	sqliteConn := migrateSQLite(t)
	defer sqliteConn.Close()

	pgConn := migratePostgres(t)
	defer pgConn.Close()

	want := describeSQLite(t, sqliteConn)
	got := describePostgres(t, pgConn)

	// Sanity: a schema that silently came back empty would make every
	// comparison below pass.
	if len(want) < 10 || len(got) < 10 {
		t.Fatalf("expected the whole schema, read %d sqlite and %d postgres tables", len(want), len(got))
	}
	if len(want) != len(got) {
		t.Errorf("table count: sqlite has %d, postgres has %d", len(want), len(got))
	}
	for _, name := range union(tablesOf(want), tablesOf(got)) {
		w, okW := want[name]
		g, okG := got[name]
		switch {
		case !okW:
			t.Errorf("table %s exists only on postgres", name)
		case !okG:
			t.Errorf("table %s exists only on sqlite", name)
		default:
			compareTable(t, name, w, g)
		}
	}
}

func compareTable(t *testing.T, name string, want, got table) {
	t.Helper()

	if a, b := renderColumns(want.columns), renderColumns(got.columns); a != b {
		t.Errorf("table %s columns differ:\n  sqlite:   %s\n  postgres: %s", name, a, b)
	}
	if want.primary != got.primary {
		t.Errorf("table %s primary key differs: sqlite (%s), postgres (%s)", name, want.primary, got.primary)
	}
	if a, b := strings.Join(sorted(want.unique), "; "), strings.Join(sorted(got.unique), "; "); a != b {
		t.Errorf("table %s unique constraints differ:\n  sqlite:   %s\n  postgres: %s", name, a, b)
	}
	if a, b := strings.Join(sorted(want.indexes), "; "), strings.Join(sorted(got.indexes), "; "); a != b {
		t.Errorf("table %s indexes differ:\n  sqlite:   %s\n  postgres: %s", name, a, b)
	}
}

// migrateSQLite applies the migrations to a fresh SQLite file.
func migrateSQLite(t *testing.T) *database.Conn {
	t.Helper()

	cfg := database.Config{
		Driver: database.DriverSQLite,
		DSN:    filepath.Join(t.TempDir(), "conformance.db"),
	}
	conn, err := database.Open(cfg, migrations.Embed())
	if err != nil {
		t.Fatalf("migrate sqlite: %v", err)
	}
	return conn
}

// migratePostgres applies the migrations to a database of its own on the test
// server, through the same database.Open a real installation uses.
//
// It deliberately does not take its database from pgtest.MigratedDSN: the point
// of this test is to compare what the *production* migration path produces on
// each engine, so the migrations have to be applied here rather than by
// pgtestdb's template. What pgtestdb provides is the empty database, which
// saves this test from creating and dropping one itself.
func migratePostgres(t *testing.T) *database.Conn {
	t.Helper()

	cfg := database.Config{Driver: database.DriverPostgres, DSN: pgtest.EmptyDSN(t)}
	conn, err := database.Open(cfg, migrations.Embed())
	if err != nil {
		t.Fatalf("migrate postgres: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	return conn
}

// describeSQLite reads the schema back from sqlite_master.
func describeSQLite(t *testing.T, conn *database.Conn) map[string]table {
	t.Helper()

	ctx := context.Background()
	out := map[string]table{}

	for _, name := range sqliteTables(t, conn) {
		var (
			tbl      table
			keyParts []string
		)

		rows, err := conn.Raw().QueryContext(ctx,
			`SELECT name, type, "notnull", pk FROM pragma_table_info(?) ORDER BY cid`, name)
		if err != nil {
			t.Fatalf("pragma_table_info(%s): %v", name, err)
		}
		for rows.Next() {
			var col column
			var declared string
			var notNull, pk int
			if err := rows.Scan(&col.name, &declared, &notNull, &pk); err != nil {
				t.Fatalf("scan column of %s: %v", name, err)
			}
			col.kind = kindForSQLite(declared)
			// SQLite reports a primary key column as NOT NULL only when the DDL
			// says so; PostgreSQL makes every primary key column NOT NULL. Both
			// are the same key, so fold the pk flag in.
			col.notNull = notNull == 1 || pk > 0
			tbl.columns = append(tbl.columns, col)
			if pk > 0 {
				keyParts = append(keyParts, col.name)
			}
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			t.Fatalf("read columns of %s: %v", name, err)
		}
		tbl.primary = strings.Join(keyParts, ",")

		idx, err := conn.Raw().QueryContext(ctx,
			`SELECT name, "unique", origin FROM pragma_index_list(?) ORDER BY name`, name)
		if err != nil {
			t.Fatalf("pragma_index_list(%s): %v", name, err)
		}
		for idx.Next() {
			var indexName, origin string
			var unique int
			if err := idx.Scan(&indexName, &unique, &origin); err != nil {
				t.Fatalf("scan index of %s: %v", name, err)
			}
			cols := sqliteIndexColumns(t, conn, indexName)
			switch origin {
			case "pk":
				// Already reported through pragma_table_info.
			case "u":
				tbl.unique = append(tbl.unique, strings.Join(cols, ","))
			default:
				tbl.indexes = append(tbl.indexes, indexLabel(indexName, unique == 1, cols))
			}
		}
		idx.Close()
		if err := idx.Err(); err != nil {
			t.Fatalf("read indexes of %s: %v", name, err)
		}

		out[name] = tbl
	}
	return out
}

func sqliteTables(t *testing.T, conn *database.Conn) []string {
	t.Helper()

	rows, err := conn.Raw().Query(
		`SELECT name FROM sqlite_master WHERE type = 'table' AND name NOT LIKE 'sqlite_%' AND name <> ? ORDER BY name`,
		database.MigrateTable)
	if err != nil {
		t.Fatalf("list sqlite tables: %v", err)
	}
	defer rows.Close()

	var names []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			t.Fatalf("scan table name: %v", err)
		}
		names = append(names, name)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("read table names: %v", err)
	}
	return names
}

// sqliteIndexColumns returns the columns an index covers, in index order.
func sqliteIndexColumns(t *testing.T, conn *database.Conn, index string) []string {
	t.Helper()

	// pragma_index_info is (seqno, cid, name); some builds omit the row for an
	// expression index, which this schema does not use.
	rows, err := conn.Raw().Query(`SELECT name FROM pragma_index_info(?) ORDER BY seqno`, index)
	if err != nil {
		t.Fatalf("pragma_index_info(%s): %v", index, err)
	}
	defer rows.Close()

	var cols []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			t.Fatalf("scan index column of %s: %v", index, err)
		}
		cols = append(cols, name)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("read index columns of %s: %v", index, err)
	}
	return cols
}

// describePostgres reads the same schema back from the PostgreSQL catalog.
func describePostgres(t *testing.T, conn *database.Conn) map[string]table {
	t.Helper()

	ctx := context.Background()
	out := map[string]table{}

	for _, name := range postgresTables(t, conn) {
		var tbl table

		cols, err := conn.Raw().QueryContext(ctx, `
			SELECT column_name, data_type, is_nullable
			FROM information_schema.columns
			WHERE table_schema = current_schema() AND table_name = $1
			ORDER BY ordinal_position`, name)
		if err != nil {
			t.Fatalf("list columns of %s: %v", name, err)
		}
		for cols.Next() {
			var col column
			var dataType, nullable string
			if err := cols.Scan(&col.name, &dataType, &nullable); err != nil {
				t.Fatalf("scan column of %s: %v", name, err)
			}
			col.kind = kindForPostgres(dataType)
			col.notNull = nullable == "NO"
			tbl.columns = append(tbl.columns, col)
		}
		cols.Close()
		if err := cols.Err(); err != nil {
			t.Fatalf("read columns of %s: %v", name, err)
		}

		cons, err := conn.Raw().QueryContext(ctx, `
			SELECT c.contype::text,
			       (SELECT string_agg(a.attname, ',' ORDER BY k.ord)
			        FROM unnest(c.conkey) WITH ORDINALITY AS k(attnum, ord)
			        JOIN pg_attribute a ON a.attrelid = c.conrelid AND a.attnum = k.attnum)
			FROM pg_constraint c
			JOIN pg_class t ON t.oid = c.conrelid
			WHERE t.relname = $1 AND t.relnamespace = current_schema()::regnamespace
			  AND c.contype IN ('p', 'u')
			ORDER BY c.conname`, name)
		if err != nil {
			t.Fatalf("list constraints of %s: %v", name, err)
		}
		for cons.Next() {
			var contype, columns string
			if err := cons.Scan(&contype, &columns); err != nil {
				t.Fatalf("scan constraint of %s: %v", name, err)
			}
			if contype == "p" {
				tbl.primary = columns
			} else {
				tbl.unique = append(tbl.unique, columns)
			}
		}
		cons.Close()
		if err := cons.Err(); err != nil {
			t.Fatalf("read constraints of %s: %v", name, err)
		}

		// Explicit indexes only: the ones backing a primary key or a unique
		// constraint were reported above, under their canonical form.
		idx, err := conn.Raw().QueryContext(ctx, `
			SELECT i.relname, ix.indisunique,
			       (SELECT array_agg(a.attname ORDER BY a.attname)
			        FROM unnest(ix.indkey) AS k(attnum)
			        JOIN pg_attribute a ON a.attrelid = ix.indrelid AND a.attnum = k.attnum)
			FROM pg_index ix
			JOIN pg_class i ON i.oid = ix.indexrelid
			JOIN pg_class t ON t.oid = ix.indrelid
			WHERE t.relname = $1 AND t.relnamespace = current_schema()::regnamespace
			  AND NOT ix.indisprimary
			  AND NOT EXISTS (SELECT 1 FROM pg_constraint c
			                  WHERE c.conindid = ix.indexrelid AND c.contype IN ('p', 'u'))
			ORDER BY i.relname`, name)
		if err != nil {
			t.Fatalf("list indexes of %s: %v", name, err)
		}
		for idx.Next() {
			var indexName string
			var unique bool
			var columnList []byte
			if err := idx.Scan(&indexName, &unique, &columnList); err != nil {
				t.Fatalf("scan index of %s: %v", name, err)
			}
			tbl.indexes = append(tbl.indexes, indexLabel(indexName, unique, parsePGArray(string(columnList))))
		}
		idx.Close()
		if err := idx.Err(); err != nil {
			t.Fatalf("read indexes of %s: %v", name, err)
		}

		out[name] = tbl
	}
	return out
}

func postgresTables(t *testing.T, conn *database.Conn) []string {
	t.Helper()

	rows, err := conn.Raw().Query(`
		SELECT table_name
		FROM information_schema.tables
		WHERE table_schema = current_schema() AND table_type = 'BASE TABLE' AND table_name <> $1
		ORDER BY table_name`, database.MigrateTable)
	if err != nil {
		t.Fatalf("list postgres tables: %v", err)
	}
	defer rows.Close()

	var names []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			t.Fatalf("scan table name: %v", err)
		}
		names = append(names, name)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("read postgres tables: %v", err)
	}
	return names
}

// kindForSQLite maps a declared SQLite type to the canonical column kind.
func kindForSQLite(declared string) string {
	upper := strings.ToUpper(strings.TrimSpace(declared))
	switch {
	case strings.Contains(upper, "BOOL"):
		return "bool"
	case strings.Contains(upper, "INT"):
		return "int"
	case strings.Contains(upper, "CHAR"), strings.Contains(upper, "TEXT"), strings.Contains(upper, "CLOB"):
		return "text"
	case strings.Contains(upper, "TIME"), strings.Contains(upper, "DATE"):
		return "timestamp"
	case strings.Contains(upper, "NUM"), strings.Contains(upper, "DEC"), strings.Contains(upper, "REAL"):
		return "numeric"
	default:
		return strings.ToLower(upper)
	}
}

// kindForPostgres maps an information_schema data type to the same kinds.
func kindForPostgres(dataType string) string {
	switch dataType {
	case "boolean":
		return "bool"
	case "text", "character varying", "character":
		return "text"
	case "timestamp without time zone", "timestamp with time zone", "date":
		return "timestamp"
	case "smallint", "integer", "bigint":
		return "int"
	case "numeric", "real", "double precision":
		return "numeric"
	default:
		return dataType
	}
}

// indexLabel is the comparable form of an index: name, uniqueness and the
// sorted columns it covers. Explicit indexes carry the DDL's own name on both
// engines, so the name is part of the comparison.
func indexLabel(name string, unique bool, cols []string) string {
	kind := "index"
	if unique {
		kind = "unique"
	}
	return fmt.Sprintf("%s %s(%s)", name, kind, strings.Join(sorted(cols), ","))
}

// parsePGArray turns the text form of a PostgreSQL array into its elements.
func parsePGArray(s string) []string {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "{")
	s = strings.TrimSuffix(s, "}")
	if s == "" {
		return nil
	}
	return strings.Split(s, ",")
}

func renderColumns(cols []column) string {
	parts := make([]string, 0, len(cols))
	for _, c := range cols {
		null := "null"
		if c.notNull {
			null = "not null"
		}
		parts = append(parts, fmt.Sprintf("%s %s %s", c.name, c.kind, null))
	}
	return strings.Join(parts, ", ")
}

func tablesOf(m map[string]table) []string {
	names := make([]string, 0, len(m))
	for name := range m {
		names = append(names, name)
	}
	return names
}

func union(a, b []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, list := range [][]string{a, b} {
		for _, name := range list {
			if !seen[name] {
				seen[name] = true
				out = append(out, name)
			}
		}
	}
	sort.Strings(out)
	return out
}

func sorted(in []string) []string {
	out := append([]string(nil), in...)
	sort.Strings(out)
	return out
}
