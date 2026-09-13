package dbtransfer

import (
	"strings"
	"testing"
)

func names(tables []table) []string {
	out := make([]string, len(tables))
	for i, t := range tables {
		out[i] = t.Name
	}
	return out
}

func TestOrderTablesPutsParentsFirst(t *testing.T) {
	t.Parallel()

	tables := []table{
		{Name: "product_image", Parents: []string{"product"}},
		{Name: "product"},
		{Name: "digital_data", Parents: []string{"product"}},
		{Name: "setting"},
	}

	got, err := orderTables(tables)
	if err != nil {
		t.Fatalf("orderTables: %v", err)
	}

	position := map[string]int{}
	for i, name := range names(got) {
		position[name] = i
	}
	for _, child := range []string{"product_image", "digital_data"} {
		if position[child] < position["product"] {
			t.Errorf("%s comes before product: %v", child, names(got))
		}
	}
	if len(got) != len(tables) {
		t.Errorf("ordered %d tables, want %d", len(got), len(tables))
	}
}

func TestOrderTablesIsDeterministic(t *testing.T) {
	t.Parallel()

	tables := func() []table {
		return []table{
			{Name: "product_image", Parents: []string{"product"}},
			{Name: "product"},
			{Name: "digital_data", Parents: []string{"product"}},
			{Name: "setting"},
			{Name: "page"},
		}
	}

	first, err := orderTables(tables())
	if err != nil {
		t.Fatalf("orderTables: %v", err)
	}
	// An unstable order would make two dumps of the same database differ, which
	// is exactly what a backup is compared against after a restore.
	for i := 0; i < 20; i++ {
		again, err := orderTables(tables())
		if err != nil {
			t.Fatalf("orderTables: %v", err)
		}
		if strings.Join(names(again), ",") != strings.Join(names(first), ",") {
			t.Fatalf("order changed: %v then %v", names(first), names(again))
		}
	}
}

func TestOrderTablesIgnoresSelfAndUnknownReferences(t *testing.T) {
	t.Parallel()

	// A self-reference needs no ordering, a reference out of the set (a
	// bookkeeping table, or one filtered out) cannot be waited for.
	tables := []table{
		{Name: "page", Parents: []string{"page", "migrate_db_version"}},
		{Name: "setting"},
	}

	got, err := orderTables(tables)
	if err != nil {
		t.Fatalf("orderTables: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("ordered %v, want both tables", names(got))
	}
}

func TestOrderTablesReportsACycle(t *testing.T) {
	t.Parallel()

	tables := []table{
		{Name: "a", Parents: []string{"b"}},
		{Name: "b", Parents: []string{"a"}},
		{Name: "c"},
	}

	_, err := orderTables(tables)
	if err == nil {
		t.Fatal("expected an error for a cycle the schema could not be written in")
	}
	for _, want := range []string{"cycle", "a", "b"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error = %v, want it to mention %q", err, want)
		}
	}
}

func TestIsBookkeeping(t *testing.T) {
	t.Parallel()

	// The goose version table must never be copied: it says which migrations
	// ran, and the target has to rebuild that by running them.
	if !isBookkeeping("migrate_db_version") {
		t.Error("migrate_db_version is not bookkeeping: a copy would claim migrations the target never ran")
	}
	if !isBookkeeping("migrate_fixtures_version") {
		t.Error("the fixture table is not bookkeeping: it belongs to a developer's database only")
	}
	if isBookkeeping("product") {
		t.Error("product is bookkeeping: it holds the shop")
	}
}

func TestQuoteIdent(t *testing.T) {
	t.Parallel()

	cases := []struct{ in, want string }{
		{"product", `"product"`},
		// A reserved word, and a column of the product table.
		{"desc", `"desc"`},
		{`we"ird`, `"we""ird"`},
	}

	for _, c := range cases {
		if got := quoteIdent(c.in); got != c.want {
			t.Errorf("quoteIdent(%q) = %s, want %s", c.in, got, c.want)
		}
	}
}

func TestCopyStatementsCarryTheColumnList(t *testing.T) {
	t.Parallel()

	tbl := table{Name: "product", Columns: []string{"id", "desc", "active"}}

	to := copyToStatement(tbl)
	for _, want := range []string{`COPY "product"`, `"id", "desc", "active"`, "TO STDOUT", "FORMAT text"} {
		if !strings.Contains(to, want) {
			t.Errorf("copyToStatement = %s, want it to contain %s", to, want)
		}
	}

	from := copyFromStatement(tbl)
	for _, want := range []string{`COPY "product"`, `"id", "desc", "active"`, "FROM STDIN", "FORMAT text"} {
		if !strings.Contains(from, want) {
			t.Errorf("copyFromStatement = %s, want it to contain %s", from, want)
		}
	}

	if got, want := selectStatement(tbl), `SELECT "id", "desc", "active" FROM "product"`; got != want {
		t.Errorf("selectStatement = %s, want %s", got, want)
	}
}
