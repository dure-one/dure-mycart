package dbtransfer

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/shurco/mycart/internal/database"
)

// bookkeepingTables are never copied. The goose version table is the migration
// state, which the target rebuilds by running the migrations — copying it would
// claim migrations are applied on a database that never ran them. The fixtures
// table belongs to the development fixture set (internal/testutil/pgtest) and
// only exists in a developer's database.
var bookkeepingTables = map[string]bool{
	database.MigrateTable:      true,
	"migrate_fixtures_version": true,
}

// table is one table to move: its columns, in the order they are written and
// read, and the tables it references.
type table struct {
	Name    string
	Columns []string
	// Kinds tells the writer how to render a value. It is filled in by the
	// SQLite source, whose driver hands over Go values (0/1 for a boolean);
	// PostgreSQL renders its own text output and needs no help.
	Kinds []kind
	// Parents are the tables referenced by a foreign key from this one.
	Parents []string
}

// isBookkeeping reports whether a table is one the transfer never touches.
func isBookkeeping(name string) bool {
	return bookkeepingTables[name]
}

// orderTables sorts tables so that a table is written after every table it
// references through a foreign key: the order a restore has to insert in.
//
// A cycle is reported rather than resolved: with the foreign keys in place
// there is no order that satisfies both ends, and a dump written anyway would
// fail halfway through a restore.
func orderTables(tables []table) ([]table, error) {
	byName := make(map[string]table, len(tables))
	children := make(map[string][]string, len(tables))
	parentsLeft := make(map[string]int, len(tables))

	for _, t := range tables {
		byName[t.Name] = t
		parentsLeft[t.Name] = 0
	}
	for _, t := range tables {
		for _, parent := range t.Parents {
			// A table referencing itself needs no ordering, and a reference to
			// a table outside the set (a bookkeeping table, or one filtered
			// out) cannot be waited for.
			if parent == t.Name || byName[parent].Name == "" {
				continue
			}
			parentsLeft[t.Name]++
			children[parent] = append(children[parent], t.Name)
		}
	}

	ready := make([]string, 0, len(tables))
	for name, left := range parentsLeft {
		if left == 0 {
			ready = append(ready, name)
		}
	}
	sort.Strings(ready)

	ordered := make([]table, 0, len(tables))
	for len(ready) > 0 {
		name := ready[0]
		ready = ready[1:]
		ordered = append(ordered, byName[name])

		next := make([]string, 0, len(children[name]))
		for _, child := range children[name] {
			parentsLeft[child]--
			if parentsLeft[child] == 0 {
				next = append(next, child)
			}
		}
		sort.Strings(next)
		ready = append(ready, next...)
	}

	if len(ordered) != len(tables) {
		stuck := make([]string, 0, len(tables)-len(ordered))
		for name, left := range parentsLeft {
			if left > 0 {
				stuck = append(stuck, name)
			}
		}
		sort.Strings(stuck)
		return nil, fmt.Errorf("cannot order tables for copying: foreign keys form a cycle through %s",
			strings.Join(stuck, ", "))
	}

	return ordered, nil
}

// discoverPostgres lists the tables of a PostgreSQL database with their columns.
func discoverPostgres(ctx context.Context, q pgQuerier) ([]table, error) {
	rows, err := q.Query(ctx, `
		SELECT table_name, column_name
		FROM information_schema.columns
		WHERE table_schema = 'public' AND is_generated <> 'ALWAYS'
		ORDER BY table_name, ordinal_position`)
	if err != nil {
		return nil, fmt.Errorf("list columns: %w", err)
	}

	columns := map[string][]string{}
	var order []string
	for rows.Next() {
		var tableName, columnName string
		if err := rows.Scan(&tableName, &columnName); err != nil {
			rows.Close()
			return nil, fmt.Errorf("scan column: %w", err)
		}
		if isBookkeeping(tableName) {
			continue
		}
		if _, seen := columns[tableName]; !seen {
			order = append(order, tableName)
		}
		columns[tableName] = append(columns[tableName], columnName)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, fmt.Errorf("list columns: %w", err)
	}
	rows.Close()

	tables := make([]table, 0, len(order))
	for _, name := range order {
		tables = append(tables, table{Name: name, Columns: columns[name]})
	}

	refs, err := foreignKeysPostgres(ctx, q)
	if err != nil {
		return nil, err
	}
	for i := range tables {
		tables[i].Parents = refs[tables[i].Name]
	}

	return tables, nil
}

// foreignKeysPostgres maps each table to the tables it references.
func foreignKeysPostgres(ctx context.Context, q pgQuerier) (map[string][]string, error) {
	rows, err := q.Query(ctx, `
		SELECT tc.table_name, ccu.table_name
		FROM information_schema.table_constraints tc
		JOIN information_schema.constraint_column_usage ccu
		  ON ccu.constraint_name = tc.constraint_name
		 AND ccu.constraint_schema = tc.constraint_schema
		WHERE tc.constraint_type = 'FOREIGN KEY' AND tc.table_schema = 'public'`)
	if err != nil {
		return nil, fmt.Errorf("list foreign keys: %w", err)
	}
	defer rows.Close()

	refs := map[string][]string{}
	for rows.Next() {
		var child, parent string
		if err := rows.Scan(&child, &parent); err != nil {
			return nil, fmt.Errorf("scan foreign key: %w", err)
		}
		if isBookkeeping(child) || isBookkeeping(parent) {
			continue
		}
		refs[child] = append(refs[child], parent)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list foreign keys: %w", err)
	}
	return refs, nil
}

// discoverSQLite lists the tables of a SQLite database with their columns,
// their declared types and the tables they reference.
func discoverSQLite(ctx context.Context, conn *database.Conn) ([]table, error) {
	rows, err := conn.QueryContext(ctx,
		`SELECT name FROM sqlite_master WHERE type = 'table' ORDER BY name`)
	if err != nil {
		return nil, fmt.Errorf("list tables: %w", err)
	}
	var names []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			rows.Close()
			return nil, fmt.Errorf("scan table: %w", err)
		}
		// sqlite_* is SQLite's own bookkeeping (sqlite_sequence and friends).
		if strings.HasPrefix(name, "sqlite_") || isBookkeeping(name) {
			continue
		}
		names = append(names, name)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, fmt.Errorf("list tables: %w", err)
	}
	rows.Close()

	tables := make([]table, 0, len(names))
	for _, name := range names {
		columns, kinds, err := sqliteColumns(ctx, conn, name)
		if err != nil {
			return nil, err
		}
		parents, err := sqliteParents(ctx, conn, name)
		if err != nil {
			return nil, err
		}
		tables = append(tables, table{Name: name, Columns: columns, Kinds: kinds, Parents: parents})
	}

	return tables, nil
}

// sqliteColumns returns a table's columns in declaration order, together with
// the rendering each one needs. The declared type is what tells a BOOLEAN from
// an INTEGER: SQLite has no column types of its own, it stores what the schema
// said.
func sqliteColumns(ctx context.Context, conn *database.Conn, name string) ([]string, []kind, error) {
	// pragma_table_info is the table-valued form of PRAGMA table_info: unlike
	// the statement form it accepts the table name as a parameter.
	rows, err := conn.QueryContext(ctx, `SELECT name, type FROM pragma_table_info(?)`, name)
	if err != nil {
		return nil, nil, fmt.Errorf("columns of %s: %w", name, err)
	}
	defer rows.Close()

	var columns []string
	var kinds []kind
	for rows.Next() {
		var column, declared string
		if err := rows.Scan(&column, &declared); err != nil {
			return nil, nil, fmt.Errorf("columns of %s: %w", name, err)
		}
		columns = append(columns, column)
		kinds = append(kinds, kindFor(declared))
	}
	if err := rows.Err(); err != nil {
		return nil, nil, fmt.Errorf("columns of %s: %w", name, err)
	}
	return columns, kinds, nil
}

// sqliteParents returns the tables a table references.
func sqliteParents(ctx context.Context, conn *database.Conn, name string) ([]string, error) {
	rows, err := conn.QueryContext(ctx,
		`SELECT DISTINCT "table" FROM pragma_foreign_key_list(?) WHERE "table" IS NOT NULL`, name)
	if err != nil {
		return nil, fmt.Errorf("foreign keys of %s: %w", name, err)
	}
	defer rows.Close()

	var parents []string
	for rows.Next() {
		var parent string
		if err := rows.Scan(&parent); err != nil {
			return nil, fmt.Errorf("foreign keys of %s: %w", name, err)
		}
		if parent != name && !isBookkeeping(parent) {
			parents = append(parents, parent)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("foreign keys of %s: %w", name, err)
	}
	return parents, nil
}
