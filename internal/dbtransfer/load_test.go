package dbtransfer

import (
	"bufio"
	"bytes"
	"context"
	"io"
	"strings"
	"testing"
)

func TestParseCopyHeader(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		line    string
		want    table
		wantErr string
	}{
		{
			name: "columns",
			line: `COPY "setting" ("id", "key", "value") FROM stdin;`,
			want: table{Name: "setting", Columns: []string{"id", "key", "value"}},
		},
		{
			name: "one column",
			line: `COPY "session" ("key") FROM stdin;`,
			want: table{Name: "session", Columns: []string{"key"}},
		},
		{
			name: "no space after the comma",
			line: `COPY "product" ("id","name") FROM stdin;`,
			want: table{Name: "product", Columns: []string{"id", "name"}},
		},
		{
			name: "keywords in lower case",
			line: `copy "product" ("id") from stdin;`,
			want: table{Name: "product", Columns: []string{"id"}},
		},
		{
			name: "an escaped quote in a name",
			line: `COPY "we""ird" ("a") FROM stdin;`,
			want: table{Name: `we"ird`, Columns: []string{"a"}},
		},
		{
			// The column is a reserved word, which is why every identifier in a
			// dump is quoted.
			name: "a reserved word",
			line: `COPY "product" ("desc") FROM stdin;`,
			want: table{Name: "product", Columns: []string{"desc"}},
		},
		{name: "not a COPY section", line: `INSERT INTO "setting" VALUES (1);`, wantErr: "not a COPY section"},
		{name: "unquoted name", line: `COPY setting (id) FROM stdin;`, wantErr: "quoted name"},
		{name: "no column list", line: `COPY "setting" FROM stdin;`, wantErr: "no column list"},
		{name: "unterminated name", line: `COPY "setting`, wantErr: "unterminated"},
		{name: "malformed column list", line: `COPY "setting" ("id" "key") FROM stdin;`, wantErr: "malformed"},
		{name: "not from stdin", line: `COPY "setting" ("id") FROM 'file.csv';`, wantErr: "FROM STDIN"},
		{name: "no source at all", line: `COPY "setting" ("id");`, wantErr: "FROM"},
		{name: "an extra clause", line: `COPY "setting" ("id") FROM stdin WITH CSV;`, wantErr: "ends with"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := parseCopyHeader(c.line)
			if c.wantErr != "" {
				if err == nil {
					t.Fatalf("parseCopyHeader(%q) = %+v, want an error", c.line, got)
				}
				if !strings.Contains(strings.ToUpper(err.Error()), strings.ToUpper(c.wantErr)) {
					t.Errorf("error = %v, want it to mention %q", err, c.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseCopyHeader(%q): %v", c.line, err)
			}
			if got.Name != c.want.Name {
				t.Errorf("table = %q, want %q", got.Name, c.want.Name)
			}
			if strings.Join(got.Columns, ",") != strings.Join(c.want.Columns, ",") {
				t.Errorf("columns = %v, want %v", got.Columns, c.want.Columns)
			}
		})
	}
}

// The header a dump is written with has to be read back: writeTable and
// parseCopyHeader are the two halves of one format, and nothing else in the
// package would notice if they drifted apart.
func TestParseCopyHeaderRoundTripThroughWriteTable(t *testing.T) {
	t.Parallel()

	tbl := table{Name: "product", Columns: []string{"id", "desc", "active"}}

	var buf bytes.Buffer
	out := bufio.NewWriter(&buf)
	if _, err := writeTable(t.Context(), out, stubSource{rows: []string{"1\tshoe\tt\n"}}, tbl); err != nil {
		t.Fatalf("writeTable: %v", err)
	}
	if err := out.Flush(); err != nil {
		t.Fatalf("flush: %v", err)
	}

	dump := buf.String()
	line, body, _ := strings.Cut(dump, "\n")
	// The file keeps the form psql itself writes; the (FORMAT text) clause only
	// belongs on the statement the server is sent.
	if strings.Contains(line, "FORMAT") {
		t.Errorf("section header = %q, want the form psql writes", line)
	}

	got, err := parseCopyHeader(line)
	if err != nil {
		t.Fatalf("parseCopyHeader(%q): %v", line, err)
	}
	if got.Name != tbl.Name || strings.Join(got.Columns, ",") != strings.Join(tbl.Columns, ",") {
		t.Errorf("round trip = %+v, want %+v", got, tbl)
	}
	if want := "1\tshoe\tt\n\\.\n"; body != want {
		t.Errorf("section body = %q, want %q", body, want)
	}
}

// stubSource is a database that hands out pre-cooked rows. writeTable takes the
// source interface, so implementing all of it is what lets this file exercise
// the writer without a server.
type stubSource struct {
	rows []string
}

func (stubSource) Driver() string { return "stub" }

func (stubSource) Version() string { return "stub" }

func (stubSource) MigrationVersion(context.Context) (int64, error) { return 0, nil }

func (stubSource) Tables(context.Context) ([]table, error) { return nil, nil }

func (stubSource) Count(context.Context, table) (int64, error) { return 0, nil }

func (stubSource) Close(context.Context) error { return nil }

func (s stubSource) CopyTo(_ context.Context, w io.Writer, _ table) (int64, error) {
	for _, row := range s.rows {
		if _, err := io.WriteString(w, row); err != nil {
			return 0, err
		}
	}
	return int64(len(s.rows)), nil
}

func TestParseTrailer(t *testing.T) {
	t.Parallel()

	line, err := commentJSON(Trailer{Magic: MagicTrailer, Rows: 7,
		Tables: []TableStat{{Name: "setting", Rows: 7}}})
	if err != nil {
		t.Fatalf("commentJSON: %v", err)
	}

	trailer, ok := parseTrailer(strings.TrimSuffix(line, "\n"))
	if !ok {
		t.Fatalf("parseTrailer(%q) did not recognise the trailer", line)
	}
	if trailer.Rows != 7 {
		t.Errorf("rows = %d, want 7", trailer.Rows)
	}

	// The banner, the header and any other comment are not the trailer.
	for _, other := range []string{"-- myCart database dump", "-- {}", "COPY \"t\" (\"c\") FROM stdin;"} {
		if _, ok := parseTrailer(other); ok {
			t.Errorf("parseTrailer(%q) claimed a line that is not the trailer", other)
		}
	}
}

func TestReadHeader(t *testing.T) {
	t.Parallel()

	line, err := commentJSON(Header{Magic: Magic, Format: FormatVersion, Migrations: 42})
	if err != nil {
		t.Fatalf("commentJSON: %v", err)
	}
	dump := "-- myCart database dump\n" + line

	header, err := readHeader(bufio.NewReader(strings.NewReader(dump)))
	if err != nil {
		t.Fatalf("readHeader: %v", err)
	}
	if header.Magic != Magic || header.Migrations != 42 {
		t.Errorf("header = %+v, want the dump's own", header)
	}
}

func TestReadHeaderRejects(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		dump string
		want string
	}{
		{name: "sql, not a dump", dump: "BEGIN;\n", want: "it starts with"},
		{name: "only comments", dump: "-- just a note\n-- and another\n", want: "no header"},
		{name: "another tool's json", dump: "-- {\"magic\":\"pgdump\"}\n", want: "no header"},
		{name: "empty file", dump: "", want: "no header"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := readHeader(bufio.NewReader(strings.NewReader(c.dump)))
			if err == nil {
				t.Fatalf("readHeader(%q) did not fail", c.dump)
			}
			if !strings.Contains(err.Error(), c.want) {
				t.Errorf("error = %v, want it to mention %q", err, c.want)
			}
		})
	}
}

func TestCopySectionReadsUpToTheTerminator(t *testing.T) {
	t.Parallel()

	body := "1\tone\n2\ttwo\n\\.\nCOPY \"next\" (\"id\") FROM stdin;\n3\n"
	in := bufio.NewReader(strings.NewReader(body))

	section := &copySection{br: in}
	got, err := io.ReadAll(section)
	if err != nil {
		t.Fatalf("read section: %v", err)
	}
	if want := "1\tone\n2\ttwo\n"; string(got) != want {
		t.Errorf("section = %q, want %q", got, want)
	}
	if section.rows != 2 {
		t.Errorf("counted %d rows, want 2", section.rows)
	}

	// The reader has to stop on the terminator line, not past it: the next
	// section starts on the very next line.
	rest, err := io.ReadAll(in)
	if err != nil {
		t.Fatalf("read the rest: %v", err)
	}
	if want := "COPY \"next\" (\"id\") FROM stdin;\n3\n"; string(rest) != want {
		t.Errorf("rest of the dump = %q, want %q", rest, want)
	}
}

func TestCopySectionEmpty(t *testing.T) {
	t.Parallel()

	section := &copySection{br: bufio.NewReader(strings.NewReader("\\.\n"))}
	got, err := io.ReadAll(section)
	if err != nil {
		t.Fatalf("read section: %v", err)
	}
	if len(got) != 0 || section.rows != 0 {
		t.Errorf("empty section read %q (%d rows), want nothing", got, section.rows)
	}
}

func TestCopySectionWithoutATerminator(t *testing.T) {
	t.Parallel()

	// A dump cut short mid-table. Passing the truncated rows off as the whole
	// table is the failure this has to prevent.
	section := &copySection{br: bufio.NewReader(strings.NewReader("1\tone\n2\ttwo\n"))}
	_, err := io.ReadAll(section)
	if err == nil {
		t.Fatal("expected an error for a section that never ends")
	}
	if !strings.Contains(err.Error(), "terminator") {
		t.Errorf("error = %v, want it to mention the missing terminator", err)
	}
}

func TestCopySectionReadsInSmallChunks(t *testing.T) {
	t.Parallel()

	// pgconn reads the section through a buffer of its own choosing; a row
	// longer than the buffer it hands over must still come out whole.
	long := strings.Repeat("x", 5000)
	body := long + "\nshort\n\\.\n"

	section := &copySection{br: bufio.NewReader(strings.NewReader(body))}
	buf := make([]byte, 16)
	var got strings.Builder
	for {
		n, err := section.Read(buf)
		got.Write(buf[:n])
		if err != nil {
			if err != io.EOF {
				t.Fatalf("read: %v", err)
			}
			break
		}
	}

	if want := long + "\nshort\n"; got.String() != want {
		t.Errorf("section = %d bytes, want %d", got.Len(), len(want))
	}
	if section.rows != 2 {
		t.Errorf("counted %d rows, want 2", section.rows)
	}
}

func TestIsTerminator(t *testing.T) {
	t.Parallel()

	if !isTerminator([]byte("\\.\n")) {
		t.Error(`\. was not recognised as the terminator`)
	}
	if !isTerminator([]byte("\\.\r\n")) {
		t.Error(`\. with CRLF was not recognised as the terminator`)
	}
	// A row whose single field is a backslash is `\\`, which is a different
	// line entirely.
	for _, line := range []string{"\\\\\n", "1\tone\n", "\n", "", "\\.x\n"} {
		if isTerminator([]byte(line)) {
			t.Errorf("%q was taken for the terminator", line)
		}
	}
}

func TestCheckTrailer(t *testing.T) {
	t.Parallel()

	manifest := &Manifest{
		Tables: []TableStat{{Name: "setting", Rows: 2}, {Name: "product", Rows: 1}},
		Rows:   3,
	}
	good := &Trailer{
		Magic:  MagicTrailer,
		Tables: []TableStat{{Name: "setting", Rows: 2}, {Name: "product", Rows: 1}},
		Rows:   3,
	}
	if err := checkTrailer(good, manifest); err != nil {
		t.Fatalf("checkTrailer: %v", err)
	}

	cases := []struct {
		name    string
		trailer *Trailer
		want    string
	}{
		{
			name:    "fewer rows than claimed",
			trailer: &Trailer{Magic: MagicTrailer, Tables: good.Tables, Rows: 9},
			want:    "incomplete",
		},
		{
			name: "fewer tables than claimed",
			trailer: &Trailer{Magic: MagicTrailer, Rows: 3,
				Tables: []TableStat{{Name: "setting", Rows: 2}}},
			want: "incomplete",
		},
		{
			name: "a table with the wrong count",
			trailer: &Trailer{Magic: MagicTrailer, Rows: 3,
				Tables: []TableStat{{Name: "setting", Rows: 2}, {Name: "product", Rows: 5}}},
			want: "product",
		},
		{
			name: "tables in another order",
			trailer: &Trailer{Magic: MagicTrailer, Rows: 3,
				Tables: []TableStat{{Name: "product", Rows: 1}, {Name: "setting", Rows: 2}}},
			want: "claims",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := checkTrailer(c.trailer, manifest)
			if err == nil {
				t.Fatal("expected a truncated dump to be refused")
			}
			if !strings.Contains(err.Error(), c.want) {
				t.Errorf("error = %v, want it to mention %q", err, c.want)
			}
		})
	}
}

func TestQuoteLineTruncates(t *testing.T) {
	t.Parallel()

	short := quoteLine("hello")
	if short != `"hello"` {
		t.Errorf("quoteLine(%q) = %s", "hello", short)
	}

	long := quoteLine(strings.Repeat("x", 200))
	if len(long) > 70 {
		t.Errorf("quoteLine dumped %d bytes of a bad line, want it cut short", len(long))
	}
}
