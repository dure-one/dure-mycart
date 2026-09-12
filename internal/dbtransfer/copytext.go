// Package dbtransfer moves the contents of a myCart database between engines
// and to and from a file.
//
// The interchange format is PostgreSQL's COPY text format, wrapped in a file
// that psql can replay:
//
//	-- myCart dump format 1
//	-- {"magic":"mycart-dump", ...}
//	COPY "setting" ("id", "key", "value") FROM stdin;
//	<one line per row>
//	\.
//	...
//	-- {"magic":"mycart-dump-trailer","tables":[...],"rows":123}
//
// COPY text is where the engines meet: it is line-oriented, so a row is a line
// and a table's data ends at the \. terminator, and PostgreSQL's own parser
// reads it back. Nothing here hand-writes a type conversion except where the
// engines disagree — SQLite stores BOOLEAN as 0/1 and returns TIMESTAMP as a
// time.Time, and both are rendered the way PostgreSQL renders them.
package dbtransfer

import (
	"bufio"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Identifier and value syntax of the COPY text format.
const (
	copyNull      = `\N`
	copyTrue      = "t"
	copyFalse     = "f"
	copyTerminal  = `\.`
	copySeparator = '\t'
	copyNewline   = '\n'
)

// copyEscaper escapes the bytes that would otherwise carry meaning in a text
// COPY stream. NewReplacer makes a single pass, so the backslash it introduces
// for `\n` is not escaped again.
var copyEscaper = strings.NewReplacer(
	`\`, `\\`,
	"\b", `\b`,
	"\f", `\f`,
	"\n", `\n`,
	"\r", `\r`,
	"\t", `\t`,
	"\v", `\v`,
)

// timestampLayouts are the forms a stored timestamp is rendered in. PostgreSQL
// omits the fraction when there is none, and pads to six digits otherwise.
const (
	timestampLayout = "2006-01-02 15:04:05"
	timestampMicros = "2006-01-02 15:04:05.000000"
)

// kind is how a column's value has to be rendered. It is decided by the source
// engine: PostgreSQL renders its own text output, SQLite's driver hands over Go
// values that have to be put into the same shape.
type kind int

const (
	// kindText is anything the database already holds as text.
	kindText kind = iota
	// kindBoolean is a BOOLEAN column read as 0/1 from SQLite.
	kindBoolean
)

// kindFor returns the rendering for a column declared with the given type name.
// SQLite reports the declared type verbatim (`BOOLEAN`, `TIMESTAMP`, …), which
// is the only place the SQLite side can tell a boolean from an integer.
func kindFor(declaredType string) kind {
	if strings.EqualFold(strings.TrimSpace(declaredType), "BOOLEAN") {
		return kindBoolean
	}
	return kindText
}

// writeField appends one field, followed by the separator or the end of the row.
func writeField(w *bufio.Writer, value any, k kind, last bool) error {
	if err := writeValue(w, value, k); err != nil {
		return err
	}
	if last {
		return w.WriteByte(copyNewline)
	}
	return w.WriteByte(copySeparator)
}

// writeValue renders a single value in COPY text syntax.
func writeValue(w *bufio.Writer, value any, k kind) error {
	if value == nil {
		_, err := w.WriteString(copyNull)
		return err
	}

	if k == kindBoolean {
		flag, err := asBoolean(value)
		if err != nil {
			return err
		}
		if flag {
			_, err = w.WriteString(copyTrue)
		} else {
			_, err = w.WriteString(copyFalse)
		}
		return err
	}

	switch v := value.(type) {
	case string:
		_, err := w.WriteString(copyEscaper.Replace(v))
		return err
	case []byte:
		_, err := w.WriteString(copyEscaper.Replace(string(v)))
		return err
	case int64:
		_, err := w.WriteString(strconv.FormatInt(v, 10))
		return err
	case int:
		_, err := w.WriteString(strconv.Itoa(v))
		return err
	case int32:
		_, err := w.WriteString(strconv.FormatInt(int64(v), 10))
		return err
	case float64:
		// 'f' rather than %v: a large amount must not come out as 1e+06, which
		// PostgreSQL would read as a float and store as 1000000 — right value,
		// wrong notation for a NUMERIC column.
		_, err := w.WriteString(strconv.FormatFloat(v, 'f', -1, 64))
		return err
	case bool:
		flag, err := asBoolean(v)
		if err != nil {
			return err
		}
		if flag {
			_, err = w.WriteString(copyTrue)
		} else {
			_, err = w.WriteString(copyFalse)
		}
		return err
	case time.Time:
		// Timestamps are stored without a time zone and are UTC by convention
		// (the PostgreSQL session is pinned to UTC for exactly this reason), so
		// the wall clock is rendered in UTC and without an offset.
		if v.Nanosecond() == 0 {
			_, err := w.WriteString(v.UTC().Format(timestampLayout))
			return err
		}
		_, err := w.WriteString(v.UTC().Format(timestampMicros))
		return err
	default:
		return fmt.Errorf("cannot copy a %T value", value)
	}
}

// asBoolean accepts the forms SQLite stores a BOOLEAN in. Anything else is an
// error rather than a guess: a silently misread flag would flip a product's
// visibility.
func asBoolean(value any) (bool, error) {
	switch v := value.(type) {
	case bool:
		return v, nil
	case int64:
		return booleanFromInt(v)
	case int:
		return booleanFromInt(int64(v))
	case int32:
		return booleanFromInt(int64(v))
	case float64:
		if v != 0 && v != 1 {
			return false, fmt.Errorf("cannot read %v as a boolean", v)
		}
		return v == 1, nil
	case string:
		switch strings.ToLower(v) {
		case "true", "t", "1", "yes":
			return true, nil
		case "false", "f", "0", "no":
			return false, nil
		}
		return false, fmt.Errorf("cannot read %q as a boolean", v)
	default:
		return false, fmt.Errorf("cannot read a %T as a boolean", value)
	}
}

func booleanFromInt(v int64) (bool, error) {
	switch v {
	case 0:
		return false, nil
	case 1:
		return true, nil
	default:
		return false, fmt.Errorf("cannot read %d as a boolean", v)
	}
}

// commentJSON renders a structured comment line: a JSON document behind the
// SQL comment marker, so a dump stays a valid SQL script while carrying the
// metadata a restore needs.
func commentJSON(doc any) (string, error) {
	raw, err := json.Marshal(doc)
	if err != nil {
		return "", fmt.Errorf("encode dump metadata: %w", err)
	}
	return "-- " + string(raw) + "\n", nil
}

// parseCommentJSON reads back a line written by commentJSON.
func parseCommentJSON(line string, doc any) error {
	line = strings.TrimSpace(line)
	if !strings.HasPrefix(line, "--") {
		return fmt.Errorf("%q is not a comment", line)
	}
	body := strings.TrimSpace(strings.TrimPrefix(line, "--"))
	if err := json.Unmarshal([]byte(body), doc); err != nil {
		return fmt.Errorf("parse dump metadata: %w", err)
	}
	return nil
}
