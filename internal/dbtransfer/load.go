package dbtransfer

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"slices"
	"strings"

	"github.com/jackc/pgx/v5"
)

// LoadOptions are the extras a load cannot infer from the dump.
type LoadOptions struct {
	// Replace allows a dump to be loaded over a database that already holds an
	// installation. Without it a load refuses to run: it empties the target
	// first, and doing that to a live shop by accident is the one mistake this
	// operation must not make quietly.
	Replace bool
}

// Load reads a dump and writes it into the PostgreSQL database named by dsn.
//
// The schema is not created here — the caller migrates the target first, and a
// dump only carries data. The target's tables are emptied before the data is
// loaded, because a migrated database is not empty: the migrations seed the
// setting table, and a load has to leave the target holding exactly what the
// dump holds.
//
// Everything happens in one transaction, so a dump that turns out to be
// truncated, or a row the target refuses, leaves the target as it was.
func Load(ctx context.Context, dsn string, r io.Reader, opts LoadOptions) (*Manifest, error) {
	in := bufio.NewReaderSize(r, 1<<16)
	header, err := readHeader(in)
	if err != nil {
		return nil, err
	}

	conn, err := connectPostgres(ctx, dsn)
	if err != nil {
		return nil, err
	}
	defer func() { _ = conn.Close(context.WithoutCancel(ctx)) }()

	tx, err := conn.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback(context.WithoutCancel(ctx))
		}
	}()

	target, err := checkTarget(ctx, tx, header, opts.Replace, true)
	if err != nil {
		return nil, err
	}

	// One statement, before any data is written: a half-emptied target left
	// behind by a failure is worse than a refusal.
	if _, err := tx.Exec(ctx, "TRUNCATE TABLE "+quoteIdents(target)+" CASCADE"); err != nil {
		return nil, fmt.Errorf("clear the target: %w", err)
	}

	manifest := &Manifest{Header: header}
	loaded := map[string]bool{}
	var trailer *Trailer

	for {
		// A read that returns nothing is the end of the file, or a file that
		// stopped arriving; there is no third case.
		line, readErr := in.ReadString('\n')
		if line == "" {
			if readErr != nil && !errors.Is(readErr, io.EOF) {
				return nil, fmt.Errorf("read the dump: %w", readErr)
			}
			break
		}

		text := strings.TrimSpace(line)
		switch {
		case text == "":
			// Blank lines between sections are harmless.
		case strings.HasPrefix(text, "--"):
			if t, ok := parseTrailer(text); ok {
				trailer = t
			}
		case hasPrefixFold(text, "COPY"):
			stat, err := loadSection(ctx, tx, in, text, target, loaded)
			if err != nil {
				return nil, err
			}
			manifest.Tables = append(manifest.Tables, stat)
			manifest.Rows += stat.Rows
		default:
			return nil, fmt.Errorf("this is not a myCart dump: unexpected line %s", quoteLine(text))
		}

		if readErr != nil && !errors.Is(readErr, io.EOF) {
			return nil, fmt.Errorf("read the dump: %w", readErr)
		}
	}

	if len(manifest.Tables) == 0 {
		return nil, errors.New("the dump carries no tables")
	}
	// The summary is written last. A file without one lost its end somewhere
	// between the machine that wrote it and this one.
	if trailer == nil {
		return nil, errors.New("the dump has no summary at the end: the file is incomplete")
	}
	if err := checkTrailer(trailer, manifest); err != nil {
		return nil, err
	}

	for _, name := range target {
		if !loaded[name] {
			manifest.Absent = append(manifest.Absent, name)
		}
	}

	if err := verifyRows(ctx, tx, manifest.Tables); err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}
	committed = true

	return manifest, nil
}

// checkTarget reports the tables a load would replace, and refuses the cases it
// must not do silently. It is the whole of the safety, in one place, so a dry
// run applies exactly the checks a real load would.
//
// requireSchema is false for a dry run of a copy: its caller migrates the
// target before writing anything, so a missing schema is the ordinary case of
// copying into a database that has never held a cart, not something to refuse.
func checkTarget(ctx context.Context, tx pgx.Tx, header Header, replace, requireSchema bool) ([]string, error) {
	// The tables come first: a target that was never migrated has no version
	// table to compare against, and "migrate it first" is the useful thing to
	// say about it.
	target, err := listTargetTables(ctx, tx)
	if err != nil {
		return nil, err
	}
	if !slices.Contains(target, "setting") {
		if !requireSchema {
			return target, nil
		}
		return nil, errors.New("the target has no myCart schema: migrate it first")
	}

	if err := checkSchemaVersion(ctx, tx.Conn(), header); err != nil {
		return nil, err
	}

	installed, err := isInstalled(ctx, tx.Conn())
	if err != nil {
		return nil, err
	}
	if installed && !replace {
		return nil, errors.New("the target database holds an installation and would be erased — " +
			"pass --force to overwrite it")
	}
	return target, nil
}

// loadSection reads one COPY section out of the dump and hands it to the
// server, which parses it with the same code that reads a psql script.
func loadSection(ctx context.Context, tx pgx.Tx, in *bufio.Reader, header string, target []string,
	loaded map[string]bool,
) (TableStat, error) {
	t, err := parseCopyHeader(header)
	if err != nil {
		return TableStat{}, err
	}
	if loaded[t.Name] {
		return TableStat{}, fmt.Errorf("the dump carries %s twice", t.Name)
	}
	loaded[t.Name] = true

	if !slices.Contains(target, t.Name) {
		return TableStat{}, fmt.Errorf("the dump carries a table this schema does not have: %s "+
			"(the dump is from a newer myCart)", t.Name)
	}

	section := &copySection{br: in}
	tag, err := tx.Conn().PgConn().CopyFrom(ctx, section, copyFromStatement(t))
	if err != nil {
		return TableStat{}, fmt.Errorf("load %s: %w", t.Name, err)
	}

	// The server's count and the file's count have to agree. A difference means
	// the file was mangled in a way PostgreSQL tolerated, and the row count is
	// the only thing that would have noticed.
	if accepted := tag.RowsAffected(); accepted != section.rows {
		return TableStat{}, fmt.Errorf("load %s: read %d rows from the dump, the database took %d",
			t.Name, section.rows, accepted)
	}

	return TableStat{Name: t.Name, Columns: t.Columns, Rows: section.rows}, nil
}

// verifyRows confirms the target holds exactly the rows that were loaded. It is
// the check a human would make with COUNT(*), and it is what catches data a
// trigger, a default or a constraint changed on the way in.
func verifyRows(ctx context.Context, tx pgx.Tx, tables []TableStat) error {
	for _, t := range tables {
		var rows int64
		if err := tx.QueryRow(ctx, "SELECT COUNT(*) FROM "+quoteIdent(t.Name)).Scan(&rows); err != nil {
			return fmt.Errorf("verify %s: %w", t.Name, err)
		}
		if rows != t.Rows {
			return fmt.Errorf("verify %s: the dump has %d rows, the target holds %d", t.Name, t.Rows, rows)
		}
	}
	return nil
}

// checkSchemaVersion refuses a dump this build does not know how to read.
func checkSchemaVersion(ctx context.Context, conn *pgx.Conn, header Header) error {
	if header.Format > FormatVersion {
		return fmt.Errorf("the dump is in format %d and this build reads up to %d: upgrade myCart first",
			header.Format, FormatVersion)
	}

	target, err := latestMigration(ctx, conn)
	if err != nil {
		return err
	}
	if header.Migrations > target {
		return fmt.Errorf("the dump is from schema version %d and this build migrates to %d: "+
			"upgrade myCart before restoring this dump", header.Migrations, target)
	}
	return nil
}

// checkTrailer compares the summary a dump ends with against what was read. The
// trailer is written last, so a file that was cut short fails here rather than
// restoring a shop without its orders.
func checkTrailer(trailer *Trailer, manifest *Manifest) error {
	if trailer.Rows != manifest.Rows {
		return fmt.Errorf("the dump claims %d rows but carries %d: the file is incomplete",
			trailer.Rows, manifest.Rows)
	}
	if len(trailer.Tables) != len(manifest.Tables) {
		return fmt.Errorf("the dump claims %d tables but carries %d: the file is incomplete",
			len(trailer.Tables), len(manifest.Tables))
	}
	for i, want := range trailer.Tables {
		got := manifest.Tables[i]
		if want.Name != got.Name || want.Rows != got.Rows {
			return fmt.Errorf("the dump claims %s holds %d rows but carries %d",
				want.Name, want.Rows, got.Rows)
		}
	}
	return nil
}

// readHeader reads the leading comments of a dump and returns the myCart header
// among them. Anything else at the top of the file is not a dump.
func readHeader(in *bufio.Reader) (Header, error) {
	for range 20 {
		line, err := in.ReadString('\n')
		text := strings.TrimSpace(line)
		if text == "" {
			if errors.Is(err, io.EOF) {
				break
			}
			if err != nil {
				return Header{}, fmt.Errorf("read the dump: %w", err)
			}
			continue
		}
		if !strings.HasPrefix(text, "--") {
			return Header{}, fmt.Errorf("this is not a myCart dump: it starts with %s", quoteLine(text))
		}

		var header Header
		if parseCommentJSON(text, &header) == nil && header.Magic == Magic {
			return header, nil
		}
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return Header{}, fmt.Errorf("read the dump: %w", err)
		}
	}
	return Header{}, errors.New("this is not a myCart dump: it has no header")
}

// listTargetTables returns the tables a target database has, except the
// bookkeeping ones the transfer never touches.
func listTargetTables(ctx context.Context, q pgQuerier) ([]string, error) {
	rows, err := q.Query(ctx, `
		SELECT table_name
		FROM information_schema.tables
		WHERE table_schema = 'public' AND table_type = 'BASE TABLE'
		ORDER BY table_name`)
	if err != nil {
		return nil, fmt.Errorf("list the target's tables: %w", err)
	}
	defer rows.Close()

	var tables []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, fmt.Errorf("list the target's tables: %w", err)
		}
		if isBookkeeping(name) {
			continue
		}
		tables = append(tables, name)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list the target's tables: %w", err)
	}
	return tables, nil
}

// isInstalled reports whether the target holds an installation. A migrated but
// never installed database has the setting row set to false, or no row at all.
func isInstalled(ctx context.Context, conn *pgx.Conn) (bool, error) {
	var value string
	err := conn.QueryRow(ctx, "SELECT value FROM setting WHERE key = 'installed'").Scan(&value)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("read the installation state: %w", err)
	}
	return strings.EqualFold(strings.TrimSpace(value), "true"), nil
}

// copySection streams one table's rows out of a dump, stopping before the \.
// terminator, and counts them.
//
// A dump holds one row per line — COPY text escapes the newlines inside a value
// — so the terminator is a line of its own that no row can produce, and the row
// count is simply the number of lines handed on.
type copySection struct {
	br      *bufio.Reader
	pending []byte
	rows    int64
	done    bool
}

func (s *copySection) Read(p []byte) (int, error) {
	for len(s.pending) == 0 {
		if s.done {
			return 0, io.EOF
		}

		line, err := s.br.ReadBytes('\n')
		if len(line) == 0 {
			if errors.Is(err, io.EOF) {
				return 0, errors.New("the dump ends inside a table: the terminator is missing")
			}
			if err != nil {
				return 0, err
			}
			continue
		}

		if isTerminator(line) {
			s.done = true
			return 0, io.EOF
		}

		s.rows++
		s.pending = line
	}

	n := copy(p, s.pending)
	s.pending = s.pending[n:]
	return n, nil
}

// isTerminator reports whether a line is the \. that ends a COPY section.
func isTerminator(line []byte) bool {
	return bytes.Equal(bytes.TrimRight(line, "\r\n"), []byte(copyTerminal))
}

// parseTrailer reads a dump's closing summary, if the comment is one.
func parseTrailer(line string) (*Trailer, bool) {
	var trailer Trailer
	if err := parseCommentJSON(line, &trailer); err != nil || trailer.Magic != MagicTrailer {
		return nil, false
	}
	return &trailer, true
}

// parseCopyHeader reads the `COPY "table" ("c1", "c2") FROM stdin;` line that
// starts a section.
func parseCopyHeader(line string) (table, error) {
	rest, ok := cutPrefixFold(strings.TrimSpace(line), "COPY")
	if !ok {
		return table{}, fmt.Errorf("this is not a COPY section: %s", quoteLine(line))
	}

	name, rest, err := readQuotedIdent(strings.TrimSpace(rest))
	if err != nil {
		return table{}, err
	}

	rest, ok = strings.CutPrefix(strings.TrimSpace(rest), "(")
	if !ok {
		return table{}, fmt.Errorf("the COPY section of %s has no column list", name)
	}

	var columns []string
	closed := false
	for !closed {
		var column string
		column, rest, err = readQuotedIdent(strings.TrimSpace(rest))
		if err != nil {
			return table{}, err
		}
		columns = append(columns, column)

		rest = strings.TrimSpace(rest)
		switch {
		case strings.HasPrefix(rest, ","):
			rest = rest[1:]
		case strings.HasPrefix(rest, ")"):
			rest = rest[1:]
			closed = true
		default:
			return table{}, fmt.Errorf("the COPY section of %s has a malformed column list", name)
		}
	}

	rest = strings.TrimSpace(rest)
	rest, ok = cutPrefixFold(rest, "FROM")
	if !ok {
		return table{}, fmt.Errorf("the COPY section of %s does not say FROM stdin", name)
	}
	rest, ok = cutPrefixFold(strings.TrimSpace(rest), "STDIN")
	if !ok {
		return table{}, fmt.Errorf("the COPY section of %s does not say FROM stdin", name)
	}
	if rest = strings.TrimSpace(rest); rest != ";" {
		return table{}, fmt.Errorf("the COPY section of %s ends with %s", name, quoteLine(rest))
	}

	return table{Name: name, Columns: columns}, nil
}

// readQuotedIdent reads a double-quoted identifier and returns it with the rest
// of the line. The dump quotes every identifier, so anything else is a file
// this package did not write.
func readQuotedIdent(s string) (string, string, error) {
	if !strings.HasPrefix(s, `"`) {
		return "", s, fmt.Errorf("expected a quoted name, got %s", quoteLine(s))
	}

	var name strings.Builder
	for i := 1; i < len(s); i++ {
		if s[i] != '"' {
			name.WriteByte(s[i])
			continue
		}
		if i+1 < len(s) && s[i+1] == '"' {
			name.WriteByte('"')
			i++
			continue
		}
		return name.String(), s[i+1:], nil
	}
	return "", s, fmt.Errorf("unterminated name in %s", quoteLine(s))
}

// cutPrefixFold is strings.CutPrefix without the case sensitivity: SQL keywords
// are written in any case by anything that is not this package.
func cutPrefixFold(s, prefix string) (string, bool) {
	if hasPrefixFold(s, prefix) {
		return s[len(prefix):], true
	}
	return s, false
}

func hasPrefixFold(s, prefix string) bool {
	return len(s) >= len(prefix) && strings.EqualFold(s[:len(prefix)], prefix)
}

// quoteLine renders a line for an error message without dumping a whole file
// into the terminal.
func quoteLine(line string) string {
	const limit = 60
	if len(line) > limit {
		return fmt.Sprintf("%q...", line[:limit])
	}
	return fmt.Sprintf("%q", line)
}
