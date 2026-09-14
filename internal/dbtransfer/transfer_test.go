package dbtransfer

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"testing"

	"github.com/shurco/mycart/internal/database"
	"github.com/shurco/mycart/internal/testutil/pgtest"
	"github.com/shurco/mycart/migrations"
)

// requirePostgres skips a test unless the suite is pointed at a test server.
// The databases themselves come from pgtestdb, through
// internal/testutil/pgtest.
func requirePostgres(t *testing.T) {
	t.Helper()

	if !strings.EqualFold(os.Getenv("TEST_DB_DRIVER"), database.DriverPostgres) {
		t.Skip("set TEST_DB_DRIVER=postgres and TEST_POSTGRES_DSN to run")
	}
	if os.Getenv(pgtest.AdminDSN) == "" {
		t.Skipf("%s is not set", pgtest.AdminDSN)
	}
}

func pgConfig(dsn string) database.Config {
	return database.Config{Driver: database.DriverPostgres, DSN: dsn, Source: database.SourceFlag}
}

func sqliteConfig(path string) database.Config {
	return database.Config{Driver: database.DriverSQLite, DSN: path, Source: database.SourceFlag}
}

// exec runs statements against a database, the way an installation would.
func exec(t *testing.T, cfg database.Config, statements ...string) {
	t.Helper()

	conn, err := database.Connect(cfg)
	if err != nil {
		t.Fatalf("connect to %s: %v", cfg.Redacted(), err)
	}
	defer func() { _ = conn.Close() }()

	for _, statement := range statements {
		if _, err := conn.ExecContext(t.Context(), statement); err != nil {
			t.Fatalf("exec %q: %v", statement, err)
		}
	}
}

// queryString reads one value as text, on either engine.
func queryString(t *testing.T, cfg database.Config, query string) string {
	t.Helper()

	conn, err := database.Connect(cfg)
	if err != nil {
		t.Fatalf("connect to %s: %v", cfg.Redacted(), err)
	}
	defer func() { _ = conn.Close() }()

	var value string
	if err := conn.QueryRowContext(t.Context(), query).Scan(&value); err != nil {
		t.Fatalf("query %q: %v", query, err)
	}
	return value
}

// countRows reads a table's row count, on either engine.
func countRows(t *testing.T, cfg database.Config, table string) int64 {
	t.Helper()

	conn, err := database.Connect(cfg)
	if err != nil {
		t.Fatalf("connect to %s: %v", cfg.Redacted(), err)
	}
	defer func() { _ = conn.Close() }()

	var rows int64
	if err := conn.QueryRowContext(t.Context(),
		"SELECT COUNT(*) FROM "+quoteIdent(table)).Scan(&rows); err != nil {
		t.Fatalf("count %s: %v", table, err)
	}
	return rows
}

// seedCart adds the rows a dump has to carry unchanged: a product with a
// boolean, an amount and a timestamp that all have a different representation on
// each engine, the image that references it, and a setting holding the
// characters COPY escapes.
func seedCart(t *testing.T, cfg database.Config) {
	t.Helper()

	exec(t, cfg,
		`INSERT INTO product (id, name, "desc", slug, amount, digital, active, deleted, created, updated)
		 VALUES ('p1', 'Shoes', 'Leather', 'shoes', 1234567.89, 'api', true, false,
		         '2024-05-06 07:08:09.123456', NULL)`,
		`INSERT INTO product (id, name, "desc", slug, amount, digital, active, deleted, created, updated)
		 VALUES ('p2', 'Socks', 'Wool', 'socks', 500, 'api', false, false, '2024-05-06 07:08:09', NULL)`,
		`INSERT INTO product_image (id, product_id, name, ext, orig_name)
		 VALUES ('i1', 'p1', 'shoes.png', 'png', 'shoes.png')`,
		// A real tab, a real newline and a real backslash: the characters COPY
		// escaping exists for, and the ones that would corrupt a dump that got
		// the escaping wrong.
		"UPDATE setting SET value = 'tab\there\nand a \\ backslash' WHERE key = 'domain'",
	)
}

// dumpText writes a dump of cfg and returns it as text.
func dumpText(t *testing.T, cfg database.Config) (string, *Manifest) {
	t.Helper()

	var buf bytes.Buffer
	manifest, err := Dump(t.Context(), cfg, &buf, DumpOptions{App: "test"})
	if err != nil {
		t.Fatalf("dump %s: %v", cfg.Redacted(), err)
	}
	return buf.String(), manifest
}

// sections splits a dump into its tables' rows, sorted. Two engines that hold
// the same data write the same lines, whatever order each of them read its rows
// in — so comparing sections compares the data itself, not how it was stored.
func sections(t *testing.T, dump string) map[string][]string {
	t.Helper()

	out := map[string][]string{}
	current := ""
	for _, line := range strings.Split(strings.TrimSuffix(dump, "\n"), "\n") {
		switch {
		case strings.HasPrefix(line, "COPY "):
			header, err := parseCopyHeader(line)
			if err != nil {
				t.Fatalf("a dump this package wrote does not parse back: %v", err)
			}
			current = header.Name
			out[current] = []string{}
		case current == "":
			// The banner, the header comment and the trailer.
		case line == copyTerminal:
			current = ""
		default:
			out[current] = append(out[current], line)
		}
	}

	for name := range out {
		sort.Strings(out[name])
	}
	return out
}

// requireSameSections fails when two dumps do not hold the same rows.
func requireSameSections(t *testing.T, want, got map[string][]string) {
	t.Helper()

	for name, wantRows := range want {
		gotRows, ok := got[name]
		if !ok {
			t.Errorf("%s is missing from the second dump entirely", name)
			continue
		}
		if len(wantRows) != len(gotRows) {
			t.Errorf("%s holds %d rows, want %d", name, len(gotRows), len(wantRows))
			continue
		}
		for i := range wantRows {
			if wantRows[i] != gotRows[i] {
				t.Errorf("%s row %d = %q, want %q", name, i, gotRows[i], wantRows[i])
			}
		}
	}
	for name := range got {
		if _, ok := want[name]; !ok {
			t.Errorf("%s is in the second dump but not the first", name)
		}
	}
}

// The whole point of the format: what one database writes, another reads back
// byte for byte, boolean and timestamp representations included.
func TestDumpLoadRoundTrip(t *testing.T) {
	requirePostgres(t)

	src := pgConfig(pgtest.MigratedDSN(t))
	seedCart(t, src)

	dump, manifest := dumpText(t, src)
	sectionsOfSource := sections(t, dump)

	if !strings.HasPrefix(dump, "-- myCart database dump\n") {
		t.Errorf("dump starts with %q, want the banner", strings.SplitN(dump, "\n", 2)[0])
	}
	if !strings.Contains(dump, "\n"+copyTerminal+"\n") {
		t.Error("dump has no \\. terminator, so no tool can read it back")
	}
	if manifest.Header.Format != FormatVersion || manifest.Header.Magic != Magic {
		t.Errorf("header = %+v, want this build's format", manifest.Header)
	}
	if manifest.Header.Driver != database.DriverPostgres {
		t.Errorf("driver = %q, want %q", manifest.Header.Driver, database.DriverPostgres)
	}
	if manifest.Rows == 0 {
		t.Error("dump reports no rows, but a migrated database seeds the setting table")
	}
	// The migration bookkeeping is not data: the target has to run the
	// migrations itself, and a copied version table would claim it did.
	for _, stat := range manifest.Tables {
		if isBookkeeping(stat.Name) {
			t.Errorf("%s was dumped, but it is bookkeeping", stat.Name)
		}
	}
	if _, ok := sectionsOfSource["product"]; !ok {
		t.Error("the product table is missing from the dump")
	}

	// Into a second, freshly migrated database: the seeded rows in it have to
	// go, or the restore would merge two carts.
	dst := pgConfig(pgtest.MigratedDSN(t))
	_ = queryString(t, dst, "SELECT value FROM setting WHERE key = 'currency'")

	loaded, err := Load(t.Context(), dst.DSN, strings.NewReader(dump), LoadOptions{})
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if loaded.Rows != manifest.Rows || len(loaded.Tables) != len(manifest.Tables) {
		t.Errorf("loaded %d rows in %d tables, the dump holds %d in %d",
			loaded.Rows, len(loaded.Tables), manifest.Rows, len(manifest.Tables))
	}

	after, err := Dump(t.Context(), dst, &bytes.Buffer{}, DumpOptions{App: "test"})
	if err != nil {
		t.Fatalf("dump the target: %v", err)
	}
	if after.Rows != manifest.Rows {
		t.Errorf("the target holds %d rows, want %d", after.Rows, manifest.Rows)
	}
	requireSameSections(t, sectionsOfSource, sections(t, dumpTextOf(t, dst)))
}

func dumpTextOf(t *testing.T, cfg database.Config) string {
	t.Helper()
	dump, _ := dumpText(t, cfg)
	return dump
}

// A restored cart must keep its booleans as booleans: the SQLite side of a copy
// hands over 0 and 1, and a misread flag would flip a product's visibility.
func TestLoadKeepsBooleansAndTimestamps(t *testing.T) {
	requirePostgres(t)

	src := pgConfig(pgtest.MigratedDSN(t))
	seedCart(t, src)
	dump, _ := dumpText(t, src)

	dst := pgConfig(pgtest.MigratedDSN(t))
	if _, err := Load(t.Context(), dst.DSN, strings.NewReader(dump), LoadOptions{}); err != nil {
		t.Fatalf("load: %v", err)
	}

	conn, err := database.Connect(dst)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer func() { _ = conn.Close() }()

	var active bool
	if err := conn.QueryRowContext(t.Context(),
		`SELECT active FROM product WHERE id = 'p1'`).Scan(&active); err != nil {
		t.Fatalf("read active: %v", err)
	}
	if !active {
		t.Error("p1 came back inactive, want the boolean it was stored with")
	}

	var created string
	if err := conn.QueryRowContext(t.Context(),
		`SELECT to_char(created, 'YYYY-MM-DD HH24:MI:SS.US') FROM product WHERE id = 'p1'`).Scan(&created); err != nil {
		t.Fatalf("read created: %v", err)
	}
	if created != "2024-05-06 07:08:09.123456" {
		t.Errorf("created = %s, want 2024-05-06 07:08:09.123456", created)
	}

	// The setting that carries a tab, a newline and a backslash has to arrive
	// whole: those are the characters COPY escaping exists for.
	var domain string
	if err := conn.QueryRowContext(t.Context(),
		`SELECT value FROM setting WHERE key = 'domain'`).Scan(&domain); err != nil {
		t.Fatalf("read domain: %v", err)
	}
	if want := "tab\there\nand a \\ backslash"; domain != want {
		t.Errorf("domain = %q, want %q", domain, want)
	}
}

func TestLoadRefusesAnInstalledTarget(t *testing.T) {
	requirePostgres(t)

	src := pgConfig(pgtest.MigratedDSN(t))
	seedCart(t, src)
	dump, _ := dumpText(t, src)

	dst := pgConfig(pgtest.MigratedDSN(t))
	exec(t, dst, `UPDATE setting SET value = 'true' WHERE key = 'installed'`)

	// A restore erases everything; doing that to a shop by accident is the
	// mistake this guard exists for.
	_, err := Load(t.Context(), dst.DSN, strings.NewReader(dump), LoadOptions{})
	if err == nil {
		t.Fatal("a load over an installation was allowed")
	}
	if !strings.Contains(err.Error(), "--force") {
		t.Errorf("error = %v, want it to say how to proceed", err)
	}

	// And the refusal has to leave the target exactly as it was, not half
	// emptied.
	if got := queryString(t, dst, `SELECT value FROM setting WHERE key = 'currency'`); got == "" {
		t.Error("the refused load changed the target")
	}

	if _, err := Load(t.Context(), dst.DSN, strings.NewReader(dump), LoadOptions{Replace: true}); err != nil {
		t.Fatalf("load with Replace: %v", err)
	}
	if after, _ := dumpText(t, dst); len(sections(t, after)) != len(sections(t, dump)) {
		t.Error("the forced load did not replace the target")
	}
	if got := queryString(t, dst, `SELECT value FROM setting WHERE key = 'installed'`); got != "0" {
		t.Errorf("installed = %q, want the dump's own value", got)
	}
}

// A failure halfway through must leave the target untouched, TRUNCATE included:
// a shop emptied by a broken dump and then left to serve is worse than a
// refused restore.
func TestLoadRollsBackOnFailure(t *testing.T) {
	requirePostgres(t)

	src := pgConfig(pgtest.MigratedDSN(t))
	seedCart(t, src)
	dump, _ := dumpText(t, src)

	dst := pgConfig(pgtest.MigratedDSN(t))

	// The amount column refuses this, and the row is in a table that comes
	// after several others have already been loaded.
	broken := strings.Replace(dump, "\t1234567.89\t", "\tnot-a-number\t", 1)
	if broken == dump {
		t.Fatal("the dump did not hold the value this test breaks")
	}

	if _, err := Load(t.Context(), dst.DSN, strings.NewReader(broken), LoadOptions{}); err == nil {
		t.Fatal("a dump holding a value the target refuses was loaded")
	}

	// The seeded row is proof the whole transaction, TRUNCATE included, went
	// back.
	if got := queryString(t, dst, `SELECT value FROM setting WHERE key = 'currency'`); got != "USD" {
		t.Errorf("currency = %q, want the target's own row back", got)
	}
	if products := countRows(t, dst, "product"); products != 0 {
		t.Errorf("the target kept %d products from a failed load", products)
	}
}

func TestLoadRejectsADumpFromANewerBuild(t *testing.T) {
	requirePostgres(t)

	src := pgConfig(pgtest.MigratedDSN(t))
	dump, _ := dumpText(t, src)

	dst := pgConfig(pgtest.MigratedDSN(t))

	t.Run("unknown format", func(t *testing.T) {
		header, err := commentJSON(Header{Magic: Magic, Format: FormatVersion + 1, Migrations: 1})
		if err != nil {
			t.Fatalf("commentJSON: %v", err)
		}
		_, err = Load(t.Context(), dst.DSN, strings.NewReader(header+dump), LoadOptions{})
		if err == nil || !strings.Contains(err.Error(), "format") {
			t.Errorf("error = %v, want the format to be refused", err)
		}
	})

	t.Run("a newer schema", func(t *testing.T) {
		header, err := commentJSON(Header{Magic: Magic, Format: FormatVersion, Migrations: 9e15})
		if err != nil {
			t.Fatalf("commentJSON: %v", err)
		}
		_, err = Load(t.Context(), dst.DSN, strings.NewReader(header+dump), LoadOptions{})
		if err == nil || !strings.Contains(err.Error(), "upgrade myCart") {
			t.Errorf("error = %v, want the schema version to be refused", err)
		}
	})
}

func TestLoadRejectsATruncatedDump(t *testing.T) {
	requirePostgres(t)

	src := pgConfig(pgtest.MigratedDSN(t))
	seedCart(t, src)
	dump, _ := dumpText(t, src)

	dst := pgConfig(pgtest.MigratedDSN(t))

	t.Run("the summary is missing", func(t *testing.T) {
		// A file cut off after the last table looks complete from the inside.
		cut := dump[:strings.LastIndex(dump, "\n-- {\"magic\":\"mycart-dump-trailer\"")]
		_, err := Load(t.Context(), dst.DSN, strings.NewReader(cut), LoadOptions{})
		if err == nil || !strings.Contains(err.Error(), "incomplete") {
			t.Errorf("error = %v, want the missing summary to be noticed", err)
		}
	})

	t.Run("the file stops arriving after the header", func(t *testing.T) {
		// The banner is one line, the header comment the next: a reader that
		// dies between them and the first section leaves nothing to load.
		first := strings.Index(dump, "\n")
		second := first + 1 + strings.Index(dump[first+1:], "\n") + 1

		body := &failingReader{data: []byte(dump), failAfter: second}
		_, err := Load(t.Context(), dst.DSN, body, LoadOptions{})
		if err == nil {
			t.Error("a dump that stopped arriving was loaded")
		}
	})

	t.Run("a table is cut off", func(t *testing.T) {
		cut := dump[:strings.LastIndex(dump, copyTerminal)]
		_, err := Load(t.Context(), dst.DSN, strings.NewReader(cut), LoadOptions{})
		if err == nil {
			t.Error("a dump ending inside a table was loaded")
		}
	})

	t.Run("the summary counts another file", func(t *testing.T) {
		// The trailer is the only thing that can catch a file whose middle went
		// missing in a way the row count did not notice.
		broken := strings.Replace(dump, `"rows":1`, `"rows":99`, 1)
		_, err := Load(t.Context(), dst.DSN, strings.NewReader(broken), LoadOptions{})
		if err == nil {
			t.Error("a dump that lies about its contents was loaded")
		}
	})
}

func TestLoadRejectsSomethingElseEntirely(t *testing.T) {
	requirePostgres(t)

	dst := pgConfig(pgtest.MigratedDSN(t))

	header, err := commentJSON(Header{Magic: Magic, Format: FormatVersion})
	if err != nil {
		t.Fatalf("commentJSON: %v", err)
	}
	trailer, err := commentJSON(Trailer{Magic: MagicTrailer})
	if err != nil {
		t.Fatalf("commentJSON: %v", err)
	}

	cases := []struct {
		name string
		body string
		want string
	}{
		{name: "sqlite", body: "SQLite format 3\x00", want: "not a myCart dump"},
		{name: "a pg_dump", body: "--\n-- PostgreSQL database dump\n--\nSET statement_timeout = 0;\n", want: "not a myCart dump"},
		{name: "empty", body: "", want: "not a myCart dump"},
		{
			name: "sql among the sections",
			body: "-- myCart database dump\n" + header + "INSERT INTO \"setting\" VALUES ('a');\n" + trailer,
			want: "unexpected line",
		},
		{
			name: "a dump with no data at all",
			body: "-- myCart database dump\n" + header + trailer,
			want: "carries no tables",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := Load(t.Context(), dst.DSN, strings.NewReader(c.body), LoadOptions{})
			if err == nil {
				t.Fatalf("Load(%q) succeeded", c.body)
			}
			if !strings.Contains(err.Error(), c.want) {
				t.Errorf("error = %v, want it to mention %q", err, c.want)
			}
		})
	}
}

func TestLoadRejectsATableTheSchemaDoesNotHave(t *testing.T) {
	requirePostgres(t)

	src := pgConfig(pgtest.MigratedDSN(t))
	dump, _ := dumpText(t, src)
	dst := pgConfig(pgtest.MigratedDSN(t))

	section := "COPY \"extension_data\" (\"id\") FROM stdin;\n1\n\\.\n"
	// Before the trailer, so the file still ends the way a dump does.
	at := strings.LastIndex(dump, "\n-- {\"magic\":\"mycart-dump-trailer\"")
	body := dump[:at] + "\n" + section + dump[at:]

	_, err := Load(t.Context(), dst.DSN, strings.NewReader(body), LoadOptions{})
	if err == nil || !strings.Contains(err.Error(), "does not have") {
		t.Errorf("error = %v, want a table outside the schema to be refused", err)
	}

	// Twice the same table is a corrupt file rather than an extension, and the
	// second copy would silently overwrite the first.
	start := strings.Index(dump, "COPY ")
	end := strings.Index(dump[start:], copyTerminal) + start + len(copyTerminal) + 1
	doubled := dump[:end] + dump[start:end] + dump[end:]

	_, err = Load(t.Context(), dst.DSN, strings.NewReader(doubled), LoadOptions{})
	if err == nil || !strings.Contains(err.Error(), "twice") {
		t.Errorf("error = %v, want a repeated table to be refused", err)
	}
}

func TestDumpRefusesAMissingSQLiteFile(t *testing.T) {
	t.Parallel()

	missing := sqliteConfig(filepath.Join(t.TempDir(), "not-there.db"))
	if _, err := Dump(t.Context(), missing, &bytes.Buffer{}, DumpOptions{}); err == nil {
		t.Fatal("a dump of a database that does not exist was allowed")
	}
}

func TestDumpRefusesAnUnknownDriver(t *testing.T) {
	t.Parallel()

	cfg := database.Config{Driver: "mysql", DSN: "mysql://localhost/shop"}
	if _, err := Dump(t.Context(), cfg, &bytes.Buffer{}, DumpOptions{}); err == nil {
		t.Fatal("a dump of an unsupported driver was allowed")
	}
	if _, err := Count(t.Context(), cfg, DumpOptions{}); err == nil {
		t.Fatal("a count of an unsupported driver was allowed")
	}
}

// Count is what a dry run reports, so it has to agree with what a real dump
// carries.
func TestCountMatchesDump(t *testing.T) {
	requirePostgres(t)

	src := pgConfig(pgtest.MigratedDSN(t))
	seedCart(t, src)

	_, dumped := dumpText(t, src)
	counted, err := Count(t.Context(), src, DumpOptions{App: "test"})
	if err != nil {
		t.Fatalf("count: %v", err)
	}

	if counted.Rows != dumped.Rows || len(counted.Tables) != len(dumped.Tables) {
		t.Errorf("count = %d rows in %d tables, the dump holds %d in %d",
			counted.Rows, len(counted.Tables), dumped.Rows, len(dumped.Tables))
	}
	for i, stat := range counted.Tables {
		if stat.Name != dumped.Tables[i].Name || stat.Rows != dumped.Tables[i].Rows {
			t.Errorf("table %d = %+v, want %+v", i, stat, dumped.Tables[i])
		}
	}
}

// sqliteCart builds a SQLite installation on disk, migrated and holding the
// same fixture rows a PostgreSQL one would.
func sqliteCart(t *testing.T) database.Config {
	t.Helper()

	cfg := sqliteConfig(filepath.Join(t.TempDir(), "data.db"))
	if err := database.Migrate(cfg, migrations.Embed()); err != nil {
		t.Fatalf("migrate sqlite: %v", err)
	}
	seedCart(t, cfg)
	return cfg
}

// The move an existing SQLite installation makes: one command, every row, with
// the values that are stored differently on each engine intact.
func TestCopySqliteToPostgres(t *testing.T) {
	requirePostgres(t)

	src := sqliteCart(t)
	dst := pgConfig(pgtest.MigratedDSN(t))

	manifest, err := Copy(t.Context(), src, dst, CopyOptions{App: "test"})
	if err != nil {
		t.Fatalf("copy: %v", err)
	}
	if manifest.Header.Driver != database.DriverSQLite {
		t.Errorf("driver = %q, want %q: the manifest describes the source", manifest.Header.Driver, database.DriverSQLite)
	}

	// The same comparison a dump against a dump makes, so a value that changed
	// representation on the way is caught.
	requireSameSections(t, sections(t, dumpTextOf(t, src)), sections(t, dumpTextOf(t, dst)))

	// The integer SQLite stores for a BOOLEAN has to arrive as a boolean, not
	// as the text '1'.
	conn, err := database.Connect(dst)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer func() { _ = conn.Close() }()

	var (
		active bool
		amount string
	)
	if err := conn.QueryRowContext(t.Context(),
		`SELECT active, CAST(amount AS TEXT) FROM product WHERE id = 'p1'`).Scan(&active, &amount); err != nil {
		t.Fatalf("read the copied product: %v", err)
	}
	if !active {
		t.Error("p1 arrived inactive, want the flag it was stored with")
	}
	if amount != "1234567.89" {
		t.Errorf("amount = %s, want 1234567.89", amount)
	}

	// The bookkeeping table belongs to the target: a copied one would claim
	// migrations the target never ran.
	own := queryString(t, dst, `SELECT COUNT(*)::text FROM `+quoteIdent(database.MigrateTable))
	if own == "0" {
		t.Error("the target lost its own migration history")
	}
	for _, stat := range manifest.Tables {
		if stat.Name == database.MigrateTable {
			t.Errorf("%s was copied", database.MigrateTable)
		}
	}
}

func TestCopyDryRunWritesNothing(t *testing.T) {
	requirePostgres(t)

	src := sqliteCart(t)
	dst := pgConfig(pgtest.MigratedDSN(t))

	before := queryString(t, dst, `SELECT value FROM setting WHERE key = 'currency'`)

	manifest, err := Copy(t.Context(), src, dst, CopyOptions{DryRun: true, App: "test"})
	if err != nil {
		t.Fatalf("dry run: %v", err)
	}
	if manifest.Rows == 0 || len(manifest.Tables) == 0 {
		t.Error("the dry run reported nothing to copy")
	}
	// Tables the running schema has and the source does not are named rather
	// than silently left out.
	for _, absent := range manifest.Absent {
		if isBookkeeping(absent) {
			t.Errorf("%s is reported as absent: it is never copied either way", absent)
		}
	}

	if got := queryString(t, dst, `SELECT value FROM setting WHERE key = 'currency'`); got != before {
		t.Errorf("the dry run changed the target: currency = %q, want %q", got, before)
	}
}

func TestCopyDryRunStillRefusesAnInstalledTarget(t *testing.T) {
	requirePostgres(t)

	src := sqliteCart(t)
	dst := pgConfig(pgtest.MigratedDSN(t))
	exec(t, dst, `UPDATE setting SET value = 'true' WHERE key = 'installed'`)

	// A dry run is what an operator checks before committing; it has to fail
	// where the real copy would, or it would endorse something impossible.
	if _, err := Copy(t.Context(), src, dst, CopyOptions{DryRun: true}); err == nil {
		t.Fatal("a dry run into an installed target was allowed")
	}
	if _, err := Copy(t.Context(), src, dst, CopyOptions{DryRun: true, Replace: true}); err != nil {
		t.Fatalf("dry run with Replace: %v", err)
	}
}

// A copy into a database that has never held a cart is the case the command
// exists for, so a dry run of it has to pass: the caller migrates the target
// before writing, and the schema it will create is not a reason to refuse.
func TestCopyDryRunIntoAnUnmigratedTarget(t *testing.T) {
	requirePostgres(t)

	src := sqliteCart(t)
	dst := pgConfig(pgtest.EmptyDSN(t))

	manifest, err := Copy(t.Context(), src, dst, CopyOptions{DryRun: true, App: "test"})
	if err != nil {
		t.Fatalf("dry run into an unmigrated target: %v", err)
	}
	if manifest.Rows == 0 {
		t.Error("the dry run reported nothing to copy")
	}

	// And it left the target exactly as it found it, schema included.
	conn, err := database.Connect(dst)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer func() { _ = conn.Close() }()

	var tables int64
	if err := conn.QueryRowContext(t.Context(),
		`SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = 'public'`).Scan(&tables); err != nil {
		t.Fatalf("count tables: %v", err)
	}
	if tables != 0 {
		t.Errorf("the dry run created %d tables in the target", tables)
	}
}

func TestCopyRefusesWhatItCannotDo(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	sqlite := sqliteConfig("data.db")
	pg := pgConfig("postgres://postgres@localhost:5432/shop?sslmode=disable")

	t.Run("into sqlite", func(t *testing.T) {
		// SQLite is a file: copying it with cp is the whole operation.
		if _, err := Copy(ctx, pg, sqlite, CopyOptions{}); err == nil {
			t.Fatal("a copy into SQLite was allowed")
		}
	})

	t.Run("onto itself", func(t *testing.T) {
		if _, err := Copy(ctx, sqlite, sqliteConfig("data.db"), CopyOptions{}); err == nil {
			t.Fatal("a copy of a database onto itself was allowed")
		}
	})
}

// A target that was never migrated has to be told so, not handed a PostgreSQL
// error about a table it does not have.
func TestLoadIntoAnUnmigratedTarget(t *testing.T) {
	requirePostgres(t)

	src := pgConfig(pgtest.MigratedDSN(t))
	dump, _ := dumpText(t, src)

	empty := pgConfig(pgtest.EmptyDSN(t))
	_, err := Load(t.Context(), empty.DSN, strings.NewReader(dump), LoadOptions{})
	if err == nil {
		t.Fatal("a load into a database with no schema was allowed")
	}
	if !strings.Contains(err.Error(), "migrate it first") {
		t.Errorf("error = %v, want it to say the target has to be migrated", err)
	}
}

// A table the dump does not carry is emptied rather than left holding rows from
// whatever was in the database before: a restore has to leave the target
// holding exactly what the dump holds.
func TestLoadEmptiesAndReportsTablesTheDumpDoesNotHave(t *testing.T) {
	requirePostgres(t)

	src := pgConfig(pgtest.MigratedDSN(t))
	dump, _ := dumpText(t, src)

	dst := pgConfig(pgtest.MigratedDSN(t))
	exec(t, dst, `CREATE TABLE extension (id TEXT PRIMARY KEY NOT NULL)`,
		`INSERT INTO extension (id) VALUES ('left-over')`)

	loaded, err := Load(t.Context(), dst.DSN, strings.NewReader(dump), LoadOptions{})
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	if !slices.Contains(loaded.Absent, "extension") {
		t.Errorf("Absent = %v, want it to name the table the dump does not carry", loaded.Absent)
	}
	if slices.Contains(loaded.Absent, "setting") {
		t.Errorf("Absent = %v, want every table the dump carries to be left out of it", loaded.Absent)
	}
	if rows := countRows(t, dst, "extension"); rows != 0 {
		t.Errorf("the table the dump does not carry still holds %d rows", rows)
	}
}

// A database that never had the setting row is a database nobody installed, so
// a restore into it is not one that erases a shop.
func TestLoadWithoutAnInstalledRow(t *testing.T) {
	requirePostgres(t)

	src := pgConfig(pgtest.MigratedDSN(t))
	seedCart(t, src)
	dump, _ := dumpText(t, src)

	dst := pgConfig(pgtest.MigratedDSN(t))
	exec(t, dst, `DELETE FROM setting WHERE key = 'installed'`)

	if _, err := Load(t.Context(), dst.DSN, strings.NewReader(dump), LoadOptions{}); err != nil {
		t.Fatalf("load into a database with no installation: %v", err)
	}
	if got := queryString(t, dst, `SELECT COUNT(*)::text FROM product`); got != "2" {
		t.Errorf("the target holds %s products, want the dump's 2", got)
	}
}

// The row counts are checked after the load, which is the only thing that would
// notice a trigger or a rule quietly dropping rows on the way in.
func TestLoadVerifiesTheRowCounts(t *testing.T) {
	requirePostgres(t)

	src := pgConfig(pgtest.MigratedDSN(t))
	seedCart(t, src)
	dump, _ := dumpText(t, src)

	dst := pgConfig(pgtest.MigratedDSN(t))
	exec(t, dst,
		`CREATE FUNCTION swallow() RETURNS trigger AS $$ BEGIN RETURN NULL; END; $$ LANGUAGE plpgsql`,
		`CREATE TRIGGER swallow_images BEFORE INSERT ON product_image
		 FOR EACH ROW EXECUTE FUNCTION swallow()`)

	_, err := Load(t.Context(), dst.DSN, strings.NewReader(dump), LoadOptions{})
	if err == nil {
		t.Fatal("a load that lost rows to a trigger was reported as successful")
	}
	if !strings.Contains(err.Error(), "product_image") {
		t.Errorf("error = %v, want it to name the table that lost rows", err)
	}
}

func TestLoadRejectsAMalformedSection(t *testing.T) {
	requirePostgres(t)

	src := pgConfig(pgtest.MigratedDSN(t))
	dump, _ := dumpText(t, src)
	dst := pgConfig(pgtest.MigratedDSN(t))

	at := strings.LastIndex(dump, "\n-- {\"magic\":\"mycart-dump-trailer\"")
	body := dump[:at] + "\nCOPY nonsense\n" + dump[at:]

	_, err := Load(t.Context(), dst.DSN, strings.NewReader(body), LoadOptions{})
	if err == nil {
		t.Fatal("a section this package could not have written was loaded")
	}
	if !strings.Contains(err.Error(), "quoted name") {
		t.Errorf("error = %v, want it to name the malformed section", err)
	}
}

// A dump that cannot be written has to fail rather than produce a file that
// looks like a backup and holds half a shop. A full disk is what this stands
// for.
func TestDumpFailsWhenTheDestinationDoes(t *testing.T) {
	requirePostgres(t)

	src := pgConfig(pgtest.MigratedDSN(t))
	seedCart(t, src)

	if _, err := Dump(t.Context(), src, failWriter{}, DumpOptions{}); err == nil {
		t.Fatal("a dump into a writer that refuses everything was reported as written")
	}

	sections := []table{{Name: "product", Columns: []string{"id"}}}

	t.Run("the section header", func(t *testing.T) {
		// A buffer this small overflows on the header, so the failure comes
		// back from writeTable itself rather than from a later flush.
		out := bufio.NewWriterSize(failWriter{}, 8)
		if _, err := writeTable(t.Context(), out, stubSource{rows: []string{"1\n"}}, sections[0]); err == nil {
			t.Error("writeTable reported no error for a writer that refuses everything")
		}
	})

	t.Run("the rows", func(t *testing.T) {
		out := bufio.NewWriterSize(&bytes.Buffer{}, 8)
		_, err := writeTable(t.Context(), out, failingSource{}, sections[0])
		if err == nil || !strings.Contains(err.Error(), "product") {
			t.Errorf("error = %v, want the table whose rows could not be read", err)
		}
	})
}

// failWriter is a destination that never accepts anything, standing in for a
// full disk.
type failWriter struct{}

func (failWriter) Write(p []byte) (int, error) {
	return 0, errors.New("no space left on device")
}

// failingSource is a database that dies in the middle of a table.
type failingSource struct{ stubSource }

func (failingSource) CopyTo(context.Context, io.Writer, table) (int64, error) {
	return 0, errors.New("the connection went away")
}

func TestConnectPostgresRejectsAnUnusableDSN(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		dsn  string
		want string
	}{
		{name: "empty", dsn: "", want: "empty"},
		{name: "not a url", dsn: "postgres://user@host:notaport/shop", want: "connection string"},
		// Nothing is listening there, and the failure has to come back as one
		// rather than as a hang.
		{name: "nothing listening", dsn: "postgres://user@127.0.0.1:1/shop?sslmode=disable", want: "connect"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := connectPostgres(t.Context(), c.dsn)
			if err == nil {
				t.Fatalf("connectPostgres(%q) succeeded", c.dsn)
			}
			if !strings.Contains(strings.ToLower(err.Error()), c.want) {
				t.Errorf("error = %v, want it to mention %q", err, c.want)
			}
		})
	}
}

func TestLoadFromAnAlreadyClosedDump(t *testing.T) {
	requirePostgres(t)

	src := pgConfig(pgtest.MigratedDSN(t))
	dump, _ := dumpText(t, src)
	dst := pgConfig(pgtest.MigratedDSN(t))

	// A reader that fails partway, the way a network file or a failing disk
	// does. The target has to be left alone.
	body := &failingReader{data: []byte(dump), failAfter: len(dump) / 2}

	if _, err := Load(t.Context(), dst.DSN, body, LoadOptions{}); err == nil {
		t.Fatal("a dump that stopped arriving was loaded")
	}
	if rows := countRows(t, dst, "product"); rows != 0 {
		t.Errorf("the target holds %d products from a load that failed", rows)
	}
	if got := queryString(t, dst, `SELECT value FROM setting WHERE key = 'currency'`); got != "USD" {
		t.Errorf("currency = %q, want the target's own row back", got)
	}
}

// failingReader hands over data and then dies, the way a half-read file does.
type failingReader struct {
	data      []byte
	offset    int
	failAfter int
}

func (r *failingReader) Read(p []byte) (int, error) {
	if r.offset >= r.failAfter {
		return 0, errors.New("input/output error")
	}
	if r.offset >= len(r.data) {
		return 0, io.EOF
	}

	n := copy(p, r.data[r.offset:])
	if r.offset+n > r.failAfter {
		n = r.failAfter - r.offset
	}
	r.offset += n
	return n, nil
}

// A copy into an installation without --force is refused, and refused by the
// writer that is streaming the dump into the target: the source goroutine has
// to stop rather than block on a pipe nobody reads.
func TestCopyRefusesAnInstalledTarget(t *testing.T) {
	requirePostgres(t)

	src := sqliteCart(t)
	dst := pgConfig(pgtest.MigratedDSN(t))
	exec(t, dst, `UPDATE setting SET value = 'true' WHERE key = 'installed'`)

	_, err := Copy(t.Context(), src, dst, CopyOptions{App: "test"})
	if err == nil {
		t.Fatal("a copy over an installation was allowed")
	}
	if !strings.Contains(err.Error(), "--force") {
		t.Errorf("error = %v, want it to say how to proceed", err)
	}

	// And the same copy with --force goes through, so the refusal is the only
	// thing that was in the way.
	if _, err := Copy(t.Context(), src, dst, CopyOptions{App: "test", Replace: true}); err != nil {
		t.Fatalf("copy with Replace: %v", err)
	}
}

// A source that cannot be read has to fail the copy, not hang it: the reader
// end of the pipe has to see the failure.
func TestCopyReportsASourceThatFails(t *testing.T) {
	requirePostgres(t)

	path := filepath.Join(t.TempDir(), "not-a-database.db")
	if err := os.WriteFile(path, []byte("this is not a SQLite database"), 0o600); err != nil {
		t.Fatalf("write the file: %v", err)
	}

	dst := pgConfig(pgtest.MigratedDSN(t))
	if _, err := Copy(t.Context(), sqliteConfig(path), dst, CopyOptions{App: "test"}); err == nil {
		t.Fatal("a copy from a file that is not a database was reported as done")
	}
}
