package dbtransfer

import (
	"bufio"
	"bytes"
	"strings"
	"testing"
	"time"
)

// render writes a single value the way a dump would.
func render(t *testing.T, value any, k kind) string {
	t.Helper()

	var buf bytes.Buffer
	w := bufio.NewWriter(&buf)
	if err := writeValue(w, value, k); err != nil {
		t.Fatalf("writeValue(%#v): %v", value, err)
	}
	if err := w.Flush(); err != nil {
		t.Fatalf("flush: %v", err)
	}
	return buf.String()
}

func TestKindFor(t *testing.T) {
	t.Parallel()

	cases := []struct {
		declared string
		want     kind
	}{
		{"BOOLEAN", kindBoolean},
		{"boolean", kindBoolean},
		{" Boolean ", kindBoolean},
		{"TEXT", kindText},
		{"INTEGER", kindText},
		{"NUMERIC", kindText},
		{"TIMESTAMP", kindText},
		{"", kindText},
		// BOOLEAN-ish spellings SQLite would accept but this schema never
		// declares: rendered as text rather than guessed at.
		{"BOOL", kindText},
	}

	for _, c := range cases {
		if got := kindFor(c.declared); got != c.want {
			t.Errorf("kindFor(%q) = %v, want %v", c.declared, got, c.want)
		}
	}
}

func TestWriteValue(t *testing.T) {
	t.Parallel()

	// A fixed instant with a fraction, and the same one without, so both
	// timestamp layouts PostgreSQL uses are covered.
	withFraction := time.Date(2024, 5, 6, 7, 8, 9, 123456000, time.UTC)
	withoutFraction := time.Date(2024, 5, 6, 7, 8, 9, 0, time.UTC)
	// The same instant as seen from a shifted zone. A dump is text and the
	// session is pinned to UTC, so the wall clock must come out in UTC.
	shifted := withFraction.In(time.FixedZone("UTC+9", 9*3600))

	cases := []struct {
		name  string
		value any
		k     kind
		want  string
	}{
		{"null", nil, kindText, `\N`},
		{"string", "hello", kindText, "hello"},
		{"empty string", "", kindText, ""},
		{"backslash", `C:\path`, kindText, `C:\\path`},
		{"newline", "a\nb", kindText, `a\nb`},
		{"tab", "a\tb", kindText, `a\tb`},
		{"carriage return", "a\rb", kindText, `a\rb`},
		{"vertical tab", "a\vb", kindText, `a\vb`},
		{"form feed", "a\fb", kindText, `a\fb`},
		{"backspace", "a\bb", kindText, `a\bb`},
		{"bytes", []byte("raw\n"), kindText, `raw\n`},
		{"int64", int64(-42), kindText, "-42"},
		{"int", 42, kindText, "42"},
		{"int32", int32(7), kindText, "7"},
		{"float", 1234567.89, kindText, "1234567.89"},
		// %v would render this as 1e+06, which PostgreSQL would read as a float
		// and store in a NUMERIC column in the wrong notation.
		{"large float", float64(1000000), kindText, "1000000"},
		{"fractional float", 0.5, kindText, "0.5"},
		{"bool true", true, kindText, "t"},
		{"bool false", false, kindText, "f"},
		{"boolean column", int64(1), kindBoolean, "t"},
		{"boolean column false", int64(0), kindBoolean, "f"},
		{"timestamp", withoutFraction, kindText, "2024-05-06 07:08:09"},
		{"timestamp with micros", withFraction, kindText, "2024-05-06 07:08:09.123456"},
		{"timestamp in another zone", shifted, kindText, "2024-05-06 07:08:09.123456"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := render(t, c.value, c.k); got != c.want {
				t.Errorf("rendered %q, want %q", got, c.want)
			}
		})
	}
}

func TestWriteValueUnsupportedType(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	w := bufio.NewWriter(&buf)
	err := writeValue(w, struct{ A int }{1}, kindText)
	if err == nil {
		t.Fatal("expected an error for a value that has no COPY representation")
	}
	if !strings.Contains(err.Error(), "cannot copy") {
		t.Errorf("error = %v, want it to name the unsupported type", err)
	}
}

func TestWriteFieldSeparators(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	w := bufio.NewWriter(&buf)
	if err := writeField(w, "a", kindText, false); err != nil {
		t.Fatalf("writeField: %v", err)
	}
	if err := writeField(w, "b", kindText, true); err != nil {
		t.Fatalf("writeField: %v", err)
	}
	if err := w.Flush(); err != nil {
		t.Fatalf("flush: %v", err)
	}

	if got, want := buf.String(), "a\tb\n"; got != want {
		t.Errorf("row = %q, want %q", got, want)
	}
}

func TestAsBoolean(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		value   any
		want    bool
		wantErr bool
	}{
		{name: "bool true", value: true, want: true},
		{name: "bool false", value: false},
		{name: "int64 one", value: int64(1), want: true},
		{name: "int64 zero", value: int64(0)},
		{name: "int one", value: 1, want: true},
		{name: "int32 zero", value: int32(0)},
		{name: "float one", value: float64(1), want: true},
		{name: "float zero", value: float64(0)},
		{name: "string true", value: "true", want: true},
		{name: "string T", value: "T", want: true},
		{name: "string False", value: "False"},
		{name: "string 0", value: "0"},
		{name: "string yes", value: "yes", want: true},
		{name: "string no", value: "no"},
		// Anything else is an error rather than a guess: a misread flag would
		// flip a product's visibility.
		{name: "int64 two", value: int64(2), wantErr: true},
		{name: "float 0.5", value: float64(0.5), wantErr: true},
		{name: "string maybe", value: "maybe", wantErr: true},
		{name: "nil", value: nil, wantErr: true},
		{name: "bytes", value: []byte{1}, wantErr: true},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := asBoolean(c.value)
			if c.wantErr {
				if err == nil {
					t.Fatalf("asBoolean(%#v) = %v, want an error", c.value, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("asBoolean(%#v): %v", c.value, err)
			}
			if got != c.want {
				t.Errorf("asBoolean(%#v) = %v, want %v", c.value, got, c.want)
			}
		})
	}
}

func TestWriteFieldReportsAnUnrepresentableValue(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	w := bufio.NewWriter(&buf)
	if err := writeField(w, make(chan int), kindText, true); err == nil {
		t.Error("a field that has no COPY representation was written anyway")
	}
}

func TestCommentJSONRejectsAnUnencodableDocument(t *testing.T) {
	t.Parallel()

	if _, err := commentJSON(make(chan int)); err == nil {
		t.Error("a document json cannot encode was encoded anyway")
	}
}

func TestCommentJSONRoundTrip(t *testing.T) {
	t.Parallel()

	want := Header{Magic: Magic, Format: FormatVersion, Migrations: 20260721120000}
	line, err := commentJSON(want)
	if err != nil {
		t.Fatalf("commentJSON: %v", err)
	}
	if !strings.HasPrefix(line, "-- ") || !strings.HasSuffix(line, "\n") {
		t.Fatalf("comment line = %q, want it to be a SQL comment on a line of its own", line)
	}

	var got Header
	if err := parseCommentJSON(line, &got); err != nil {
		t.Fatalf("parseCommentJSON: %v", err)
	}
	if got.Magic != want.Magic || got.Format != want.Format || got.Migrations != want.Migrations {
		t.Errorf("round trip = %+v, want %+v", got, want)
	}
}

func TestParseCommentJSONRejects(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		line string
	}{
		{"not a comment", "SELECT 1;"},
		{"not json", "-- hello"},
		{"truncated json", `-- {"magic":`},
		{"empty", ""},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var header Header
			if err := parseCommentJSON(c.line, &header); err == nil {
				t.Errorf("parseCommentJSON(%q) did not fail", c.line)
			}
		})
	}
}
