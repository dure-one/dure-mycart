package app

import (
	"github.com/shurco/mycart/internal/database"
	"github.com/shurco/mycart/internal/digitalfiles"
	"github.com/shurco/mycart/internal/queries"
	"github.com/shurco/mycart/migrations"
	"github.com/shurco/mycart/pkg/fsutil"
)

var (
	requiredDirs = []string{"./lc_uploads", digitalfiles.Dir}
)

// Init creates the directory layout and connects to the configured database,
// bringing the schema up to date. The handle is installed process-wide, so
// every query group talks to the same database in the same dialect.
func Init(cfg database.Config) error {
	if err := fsutil.MkDirs(0o775, requiredDirs...); err != nil {
		if lg := logger(); lg != nil {
			lg.Err(err).Send()
		}
		return err
	}

	if err := queries.New(cfg, migrations.Embed()); err != nil {
		if lg := logger(); lg != nil {
			lg.Err(err).Send()
		}
		return err
	}

	return nil
}

// Migrate brings the schema up to date without connecting the application.
func Migrate(cfg database.Config) error {
	return database.Migrate(cfg, migrations.Embed())
}
