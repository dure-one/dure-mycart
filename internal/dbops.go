package app

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/shurco/mycart/internal/database"
	"github.com/shurco/mycart/internal/dbtransfer"
	"github.com/shurco/mycart/pkg/fsutil"
	"github.com/shurco/mycart/pkg/update"
)

// gzipMagic is the two bytes every gzip stream starts with. A dump is read
// through whatever the file turns out to be rather than through what its name
// claims: an uncompressed dump somebody saved as .gz would otherwise be handed
// to gzip and fail with "invalid header", which says nothing about the mistake.
var gzipMagic = []byte{0x1f, 0x8b}

// BackupDatabase writes the contents of cfg to path.
//
// The file is a SQL script: psql can replay it against a migrated, empty
// database, and RestoreDatabase reads it back without psql. A path ending in
// .gz is compressed.
//
// The dump is written to a temporary file and renamed into place, so a backup
// that fails leaves no file that looks like a backup.
func BackupDatabase(ctx context.Context, cfg database.Config, path string) (*dbtransfer.Manifest, error) {
	if cfg.Driver != database.DriverPostgres {
		return nil, fmt.Errorf("backing up %s is not supported: a SQLite cart is a single file, "+
			"copy %s while the server is stopped", cfg.Driver, cfg.DSN)
	}
	if strings.TrimSpace(path) == "" {
		return nil, errors.New("no output file given")
	}

	// The file is written before the dump is read, so a path in a directory
	// that does not exist fails now rather than after reading the database.
	part := path + ".part"
	file, err := fsutil.OpenFile(part, fsutil.FsCWTFlags, 0o600)
	if err != nil {
		return nil, fmt.Errorf("create %s: %w", path, err)
	}

	manifest, err := dumpTo(ctx, cfg, file, path)
	// The bytes have to reach the disk before the rename: without it a crash can
	// leave a renamed-but-empty file, which is the failure the .part is for.
	if err == nil {
		if syncErr := file.Sync(); syncErr != nil {
			err = fmt.Errorf("write %s: %w", path, syncErr)
		}
	}
	if closeErr := file.Close(); err == nil && closeErr != nil {
		err = fmt.Errorf("write %s: %w", path, closeErr)
	}
	if err != nil {
		// Leaving the .part behind would be a file that looks like a backup of
		// a shop but holds half of it.
		_ = os.Remove(part)
		return nil, err
	}

	if err := os.Rename(part, path); err != nil {
		_ = os.Remove(part)
		return nil, fmt.Errorf("write %s: %w", path, err)
	}

	return manifest, nil
}

// dumpTo runs the dump, compressing it when the destination is named .gz.
func dumpTo(ctx context.Context, cfg database.Config, w io.Writer, path string) (*dbtransfer.Manifest, error) {
	var (
		compressed *gzip.Writer
		out        io.Writer = w
	)
	if strings.HasSuffix(strings.ToLower(path), ".gz") {
		compressed = gzip.NewWriter(w)
		out = compressed
	}

	manifest, err := dbtransfer.Dump(ctx, cfg, out, dbtransfer.DumpOptions{App: currentAppVersion()})
	if err != nil {
		return nil, fmt.Errorf("dump the database: %w", err)
	}
	if compressed != nil {
		// Not deferred: this is the last write of the file, and gzip reports a
		// full disk here rather than in Dump.
		if err := compressed.Close(); err != nil {
			return nil, fmt.Errorf("write %s: %w", path, err)
		}
	}

	return manifest, nil
}

// RestoreDatabase replaces the contents of cfg with the dump in path.
//
// The target is migrated first: a dump carries data only, so the schema has to
// exist, and running the migrations is also what makes restoring into a
// database that has never held a cart possible. The whole load is one
// transaction, so a failure leaves the target untouched.
func RestoreDatabase(ctx context.Context, cfg database.Config, path string, force bool) (*dbtransfer.Manifest, error) {
	if cfg.Driver != database.DriverPostgres {
		return nil, fmt.Errorf("restoring into %s is not supported: a SQLite cart is a single file, "+
			"copy %s while the server is stopped", cfg.Driver, cfg.DSN)
	}

	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", path, err)
	}
	defer func() { _ = file.Close() }()

	// The schema first, so the load has tables to write into.
	if err := Migrate(cfg); err != nil {
		return nil, fmt.Errorf("migrate the target: %w", err)
	}

	body, closer, err := decompress(bufio.NewReader(file))
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	defer func() { _ = closer() }()

	manifest, err := dbtransfer.Load(ctx, cfg.DSN, body, dbtransfer.LoadOptions{Replace: force})
	if err != nil {
		return nil, fmt.Errorf("restore %s: %w", path, err)
	}
	return manifest, nil
}

// CopyDatabase moves the contents of src into dst.
func CopyDatabase(ctx context.Context, src, dst database.Config, opts dbtransfer.CopyOptions) (*dbtransfer.Manifest, error) {
	opts.App = currentAppVersion()

	manifest, err := dbtransfer.Copy(ctx, src, dst, opts)
	if err != nil {
		return nil, fmt.Errorf("copy %s to %s: %w", src.Driver, dst.Redacted(), err)
	}
	return manifest, nil
}

// decompress returns the dump inside r, gunzipping it when it is gzipped. The
// returned closer is never nil, so a caller can defer it unconditionally.
func decompress(r *bufio.Reader) (io.Reader, func() error, error) {
	head, err := r.Peek(len(gzipMagic))
	if err != nil && !errors.Is(err, io.EOF) {
		return nil, nil, err
	}
	if !bytes.Equal(head, gzipMagic) {
		return r, func() error { return nil }, nil
	}

	gz, err := gzip.NewReader(r)
	if err != nil {
		return nil, nil, err
	}
	return gz, gz.Close, nil
}

// currentAppVersion is the version a dump records. It is empty in tests, where
// SetVersion was never called.
func currentAppVersion() string {
	if info := update.VersionInfo(); info != nil {
		return info.CurrentVersion
	}
	return ""
}
