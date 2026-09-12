package dbtransfer

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/shurco/mycart/internal/database"
	"github.com/shurco/mycart/pkg/fsutil"
)

// The dump format is versioned so a future change can be detected instead of
// misread.
const (
	// FormatVersion is the version of the dump format this build writes and the
	// highest it can read.
	FormatVersion = 1
	// Magic tags the header comment; MagicTrailer tags the summary at the end.
	Magic        = "mycart-dump"
	MagicTrailer = "mycart-dump-trailer"
)

// Header is the structured comment a dump starts with.
type Header struct {
	Magic   string    `json:"magic"`
	Format  int       `json:"format"`
	Created time.Time `json:"created"`
	// Driver and Server describe where the data came from, so a dump can be
	// told apart from another installation's.
	Driver string `json:"driver"`
	Server string `json:"server"`
	// App is the myCart version that wrote the dump.
	App string `json:"app,omitempty"`
	// Migrations is the schema version of the source, as goose recorded it.
	Migrations int64 `json:"migrations"`
}

// TableStat is one table's contribution to a dump.
type TableStat struct {
	Name    string   `json:"name"`
	Columns []string `json:"columns"`
	Rows    int64    `json:"rows"`
}

// Trailer is the structured comment a dump ends with. It carries what can only
// be known after the data has been written, and it is what makes a truncated
// file detectable.
type Trailer struct {
	Magic  string      `json:"magic"`
	Tables []TableStat `json:"tables"`
	Rows   int64       `json:"rows"`
}

// Manifest is what a dump or a load reports: the header of the file, plus the
// tables and rows it actually carried.
type Manifest struct {
	Header
	Tables []TableStat `json:"tables"`
	Rows   int64       `json:"rows"`
	// Absent lists tables the target has that the dump does not mention. They
	// are left empty, which is what a table introduced by a newer migration
	// should be.
	Absent []string `json:"-"`
}

// DumpOptions are the extras the caller knows and this package does not.
type DumpOptions struct {
	// App is the running myCart version, recorded in the header.
	App string
}

// Dump writes the contents of the configured database to w.
//
// The result is a SQL script: psql can replay it against a migrated database
// whose tables have been cleared (the file says how), and Load reads it back
// without psql.
func Dump(ctx context.Context, cfg database.Config, w io.Writer, opts DumpOptions) (*Manifest, error) {
	src, err := openSource(ctx, cfg)
	if err != nil {
		return nil, err
	}
	defer func() { _ = src.Close(ctx) }()

	tables, err := src.Tables(ctx)
	if err != nil {
		return nil, err
	}
	version, err := src.MigrationVersion(ctx)
	if err != nil {
		return nil, err
	}

	header := Header{
		Magic:      Magic,
		Format:     FormatVersion,
		Created:    time.Now().UTC(),
		Driver:     src.Driver(),
		Server:     src.Version(),
		App:        opts.App,
		Migrations: version,
	}
	head, err := commentJSON(header)
	if err != nil {
		return nil, err
	}

	out := bufio.NewWriter(w)
	if _, err := out.WriteString("-- myCart database dump\n"); err != nil {
		return nil, err
	}
	if _, err := out.WriteString(head); err != nil {
		return nil, err
	}

	// The file carries data only, and a migrated database is not empty — the
	// migrations seed it — so replaying it by hand needs the target cleared
	// first. The line is written commented out: a dump that empties a database
	// the moment it is piped into psql is not a file to hand around, and psql
	// still fails loudly on the duplicate keys if it is left out.
	if names := tableNames(tables); len(names) > 0 {
		for _, line := range []string{
			"-- Replayed with psql, this file needs the target's tables emptied first, in the",
			"-- same transaction: `mycart db restore` does that itself, and verifies the result.",
			"--   psql -1 \"$DSN\" -c 'TRUNCATE TABLE " + quoteIdents(names) + " CASCADE;' -f <this file>",
		} {
			if _, err := out.WriteString(line + "\n"); err != nil {
				return nil, fmt.Errorf("write dump: %w", err)
			}
		}
	}

	manifest := &Manifest{Header: header}
	for _, t := range tables {
		rows, err := writeTable(ctx, out, src, t)
		if err != nil {
			return nil, err
		}
		manifest.Tables = append(manifest.Tables, TableStat{Name: t.Name, Columns: t.Columns, Rows: rows})
		manifest.Rows += rows
	}

	tail, err := commentJSON(Trailer{Magic: MagicTrailer, Tables: manifest.Tables, Rows: manifest.Rows})
	if err != nil {
		return nil, err
	}
	if _, err := out.WriteString(tail); err != nil {
		return nil, err
	}
	if err := out.Flush(); err != nil {
		return nil, fmt.Errorf("write dump: %w", err)
	}

	return manifest, nil
}

// tableNames lists the tables of a dump in the order they are written.
func tableNames(tables []table) []string {
	names := make([]string, len(tables))
	for i, t := range tables {
		names[i] = t.Name
	}
	return names
}

// writeTable writes one table's COPY section and returns the number of rows in
// it.
func writeTable(ctx context.Context, out *bufio.Writer, src source, t table) (int64, error) {
	// Written the way psql writes a data section, so the file can also be
	// replayed by psql against a migrated database.
	if _, err := fmt.Fprintf(out, "COPY %s (%s) FROM stdin;\n", quoteIdent(t.Name), quoteIdents(t.Columns)); err != nil {
		return 0, fmt.Errorf("write dump of %s: %w", t.Name, err)
	}

	rows, err := src.CopyTo(ctx, out, t)
	if err != nil {
		return 0, fmt.Errorf("dump %s: %w", t.Name, err)
	}

	// PostgreSQL terminates every row with a newline, and the SQLite source
	// writes one per row too, so the terminator starts on a line of its own.
	if _, err := out.WriteString(copyTerminal + "\n"); err != nil {
		return 0, fmt.Errorf("write dump of %s: %w", t.Name, err)
	}
	return rows, nil
}

// Count reports how many rows each table holds, without writing a dump. It is
// what a dry run shows.
func Count(ctx context.Context, cfg database.Config, opts DumpOptions) (*Manifest, error) {
	src, err := openSource(ctx, cfg)
	if err != nil {
		return nil, err
	}
	defer func() { _ = src.Close(ctx) }()

	tables, err := src.Tables(ctx)
	if err != nil {
		return nil, err
	}
	version, err := src.MigrationVersion(ctx)
	if err != nil {
		return nil, err
	}

	manifest := &Manifest{
		Header: Header{
			Magic:      Magic,
			Format:     FormatVersion,
			Created:    time.Now().UTC(),
			Driver:     src.Driver(),
			Server:     src.Version(),
			App:        opts.App,
			Migrations: version,
		},
	}
	for _, t := range tables {
		rows, err := src.Count(ctx, t)
		if err != nil {
			return nil, err
		}
		manifest.Tables = append(manifest.Tables, TableStat{Name: t.Name, Columns: t.Columns, Rows: rows})
		manifest.Rows += rows
	}
	return manifest, nil
}

// source is one database being read from: PostgreSQL or SQLite. Both render
// their rows in the same COPY text format, so nothing downstream needs to know
// which one it is talking to.
type source interface {
	Driver() string
	Version() string
	MigrationVersion(ctx context.Context) (int64, error)
	// Tables lists the tables to copy, in the order they have to be written.
	Tables(ctx context.Context) ([]table, error)
	// CopyTo writes the rows of one table as COPY text.
	CopyTo(ctx context.Context, w io.Writer, t table) (int64, error)
	Count(ctx context.Context, t table) (int64, error)
	Close(ctx context.Context) error
}

// openSource connects to the configured database for reading.
func openSource(ctx context.Context, cfg database.Config) (source, error) {
	switch cfg.Driver {
	case database.DriverPostgres:
		return openPostgresSource(ctx, cfg.DSN)
	case database.DriverSQLite:
		return openSQLiteSource(ctx, cfg.DSN)
	default:
		return nil, fmt.Errorf("unknown database driver %q", cfg.Driver)
	}
}

// pgSource reads a PostgreSQL database through its own connection: COPY is a
// protocol feature, not SQL, so it needs the raw driver rather than the
// database/sql wrapper the application queries through.
type pgSource struct {
	conn    *pgx.Conn
	version string
}

func openPostgresSource(ctx context.Context, dsn string) (*pgSource, error) {
	conn, err := connectPostgres(ctx, dsn)
	if err != nil {
		return nil, err
	}

	var version string
	if err := conn.QueryRow(ctx, "SELECT version()").Scan(&version); err != nil {
		_ = conn.Close(ctx)
		return nil, fmt.Errorf("read the postgres version: %w", err)
	}

	return &pgSource{conn: conn, version: version}, nil
}

func (s *pgSource) Driver() string  { return database.DriverPostgres }
func (s *pgSource) Version() string { return s.version }

func (s *pgSource) MigrationVersion(ctx context.Context) (int64, error) {
	return latestMigration(ctx, s.conn)
}

func (s *pgSource) Tables(ctx context.Context) ([]table, error) {
	tables, err := discoverPostgres(ctx, s.conn)
	if err != nil {
		return nil, err
	}
	return orderTables(tables)
}

func (s *pgSource) CopyTo(ctx context.Context, w io.Writer, t table) (int64, error) {
	tag, err := s.conn.PgConn().CopyTo(ctx, w, copyToStatement(t))
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}

func (s *pgSource) Count(ctx context.Context, t table) (int64, error) {
	var rows int64
	err := s.conn.QueryRow(ctx, "SELECT COUNT(*) FROM "+quoteIdent(t.Name)).Scan(&rows)
	if err != nil {
		return 0, fmt.Errorf("count %s: %w", t.Name, err)
	}
	return rows, nil
}

func (s *pgSource) Close(ctx context.Context) error { return s.conn.Close(ctx) }

// sqliteSource reads a SQLite database with the application's own connection
// handling, so the file is opened with the same pragmas as always.
type sqliteSource struct {
	conn    *database.Conn
	version string
}

func openSQLiteSource(ctx context.Context, path string) (*sqliteSource, error) {
	// database.Connect creates a missing database file. For a source that is
	// the opposite of what is wanted: an empty file would dump as an empty
	// installation instead of failing.
	if !fsutil.IsFile(path) {
		return nil, fmt.Errorf("no SQLite database at %s", path)
	}

	conn, err := database.Connect(database.Config{Driver: database.DriverSQLite, DSN: path})
	if err != nil {
		return nil, err
	}

	var version string
	if err := conn.QueryRowContext(ctx, "SELECT sqlite_version()").Scan(&version); err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("read the sqlite version: %w", err)
	}

	return &sqliteSource{conn: conn, version: "SQLite " + version}, nil
}

func (s *sqliteSource) Driver() string  { return database.DriverSQLite }
func (s *sqliteSource) Version() string { return s.version }

func (s *sqliteSource) MigrationVersion(ctx context.Context) (int64, error) {
	return latestMigrationSQLite(ctx, s.conn)
}

func (s *sqliteSource) Tables(ctx context.Context) ([]table, error) {
	tables, err := discoverSQLite(ctx, s.conn)
	if err != nil {
		return nil, err
	}
	return orderTables(tables)
}

// CopyTo reads a table row by row and renders each row as COPY text.
func (s *sqliteSource) CopyTo(ctx context.Context, w io.Writer, t table) (int64, error) {
	rows, err := s.conn.QueryContext(ctx, selectStatement(t))
	if err != nil {
		return 0, fmt.Errorf("read %s: %w", t.Name, err)
	}
	defer func() { _ = rows.Close() }()

	out := bufio.NewWriter(w)
	var count int64
	for rows.Next() {
		values := make([]any, len(t.Columns))
		pointers := make([]any, len(t.Columns))
		for i := range values {
			pointers[i] = &values[i]
		}
		if err := rows.Scan(pointers...); err != nil {
			return 0, fmt.Errorf("read %s: %w", t.Name, err)
		}

		for i, value := range values {
			k := kindText
			if i < len(t.Kinds) {
				k = t.Kinds[i]
			}
			if err := writeField(out, value, k, i == len(values)-1); err != nil {
				return 0, fmt.Errorf("render %s column %s: %w", t.Name, t.Columns[i], err)
			}
		}
		count++
	}
	if err := rows.Err(); err != nil {
		return 0, fmt.Errorf("read %s: %w", t.Name, err)
	}
	if err := out.Flush(); err != nil {
		return 0, fmt.Errorf("dump %s: %w", t.Name, err)
	}

	return count, nil
}

func (s *sqliteSource) Count(ctx context.Context, t table) (int64, error) {
	var rows int64
	err := s.conn.QueryRowContext(ctx, "SELECT COUNT(*) FROM "+quoteIdent(t.Name)).Scan(&rows)
	if err != nil {
		return 0, fmt.Errorf("count %s: %w", t.Name, err)
	}
	return rows, nil
}

func (s *sqliteSource) Close(ctx context.Context) error { return s.conn.Close() }

// copyToStatement reads a table out of PostgreSQL: `COPY "t" ("c") TO STDOUT
// (FORMAT text)`.
func copyToStatement(t table) string {
	return fmt.Sprintf("COPY %s (%s) TO STDOUT (FORMAT text)",
		quoteIdent(t.Name), quoteIdents(t.Columns))
}

// copyFromStatement writes a table into PostgreSQL, the mirror of
// copyToStatement.
func copyFromStatement(t table) string {
	return fmt.Sprintf("COPY %s (%s) FROM STDIN (FORMAT text)",
		quoteIdent(t.Name), quoteIdents(t.Columns))
}

// selectStatement reads a table's columns in the order the dump writes them.
func selectStatement(t table) string {
	return "SELECT " + quoteIdents(t.Columns) + " FROM " + quoteIdent(t.Name)
}

// quoteIdent quotes an identifier the way PostgreSQL does when it has to: always.
// The schema has a column named `desc`, which is a reserved word, and a dump
// that failed on it would be a dump nobody could restore.
func quoteIdent(name string) string {
	return pgx.Identifier{name}.Sanitize()
}

// quoteIdents quotes a list of identifiers.
func quoteIdents(names []string) string {
	quoted := make([]string, len(names))
	for i, name := range names {
		quoted[i] = quoteIdent(name)
	}
	return strings.Join(quoted, ", ")
}

// latestMigrationSQL is the goose version query both drivers run. It is a const
// and so quotes the table by hand: quoteIdent is not a constant expression, and
// MigrateTable is a fixed name with nothing to escape.
const latestMigrationSQL = `SELECT COALESCE(MAX(version_id), 0) FROM "` +
	database.MigrateTable + `" WHERE is_applied`

// latestMigration returns the goose version a PostgreSQL database is on.
func latestMigration(ctx context.Context, conn *pgx.Conn) (int64, error) {
	var version int64
	if err := conn.QueryRow(ctx, latestMigrationSQL).Scan(&version); err != nil {
		return 0, fmt.Errorf("read the migration version: %w", err)
	}
	return version, nil
}

// latestMigrationSQLite returns the goose version a SQLite database is on.
func latestMigrationSQLite(ctx context.Context, conn *database.Conn) (int64, error) {
	var version int64
	if err := conn.QueryRowContext(ctx, latestMigrationSQL).Scan(&version); err != nil {
		return 0, fmt.Errorf("read the migration version: %w", err)
	}
	return version, nil
}
