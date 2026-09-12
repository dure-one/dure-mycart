package dbtransfer

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/shurco/mycart/internal/database"
)

// CopyOptions are the extras a copy cannot infer from the databases.
type CopyOptions struct {
	// Replace allows a copy to overwrite an installation in the target.
	Replace bool
	// DryRun reports what a copy would do without writing anything.
	DryRun bool
	// App is the running myCart version, recorded in the manifest.
	App string
}

// CheckCopyTarget refuses a target a copy cannot write into. It is exported so
// a caller can refuse one before it creates or migrates anything: migrating a
// SQLite target would create the file the copy then rejects.
func CheckCopyTarget(dst database.Config) error {
	if dst.Driver != database.DriverPostgres {
		return fmt.Errorf("a copy target must be PostgreSQL, got %q", dst.Driver)
	}
	return nil
}

// Copy moves the contents of src into dst.
//
// The dump is never held in memory: the source writes into a pipe and the
// target reads out of it, so a shop of any size copies in constant memory. The
// target is PostgreSQL — SQLite is a single file that can be copied with cp,
// and it is the source people move from, not to.
func Copy(ctx context.Context, src, dst database.Config, opts CopyOptions) (*Manifest, error) {
	if err := CheckCopyTarget(dst); err != nil {
		return nil, err
	}
	if src.Driver == dst.Driver && src.DSN == dst.DSN {
		return nil, errors.New("the source and the target are the same database")
	}
	if opts.DryRun {
		return dryRunCopy(ctx, src, dst, opts)
	}

	pr, pw := io.Pipe()
	done := make(chan error, 1)
	go func() {
		_, err := Dump(ctx, src, pw, DumpOptions{App: opts.App})
		// The reader has to see the failure rather than a closed pipe, so that
		// a source that died mid-table aborts the load instead of ending it.
		_ = pw.CloseWithError(err)
		done <- err
	}()

	manifest, err := Load(ctx, dst.DSN, pr, LoadOptions{Replace: opts.Replace})
	if err != nil {
		// Unblock the dumper: nothing is reading the pipe any more.
		_ = pr.CloseWithError(err)
		<-done
		return nil, err
	}
	if err := <-done; err != nil {
		return nil, fmt.Errorf("read the source: %w", err)
	}

	return manifest, nil
}

// dryRunCopy counts the source and checks the target, without writing.
//
// It runs the same checks a copy would — the target's schema version, and
// whether it holds an installation that would be erased — so that a dry run
// fails exactly where the real thing would, rather than reporting a copy that
// could never have happened.
func dryRunCopy(ctx context.Context, src, dst database.Config, opts CopyOptions) (*Manifest, error) {
	manifest, err := Count(ctx, src, DumpOptions{App: opts.App})
	if err != nil {
		return nil, err
	}

	conn, err := connectPostgres(ctx, dst.DSN)
	if err != nil {
		return nil, err
	}
	defer func() { _ = conn.Close(context.WithoutCancel(ctx)) }()

	tx, err := conn.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin: %w", err)
	}
	defer func() { _ = tx.Rollback(context.WithoutCancel(ctx)) }()

	target, err := checkTarget(ctx, tx, manifest.Header, opts.Replace, false)
	if err != nil {
		return nil, err
	}

	carried := make(map[string]bool, len(manifest.Tables))
	for _, t := range manifest.Tables {
		carried[t.Name] = true
	}
	for _, name := range target {
		if !carried[name] {
			manifest.Absent = append(manifest.Absent, name)
		}
	}

	return manifest, nil
}
