package app

import (
	"github.com/shurco/mycart/db/migrations"
	"github.com/shurco/mycart/internal/store/db"
	"github.com/shurco/mycart/pkg/fsutil"
)

var (
	requiredDirs = []string{"./lc_base", "./lc_uploads", "./lc_digitals"}
)

// Init initializes the directory structure
func Init() error {
	for _, dir := range requiredDirs {
		if err := fsutil.MkDirs(0o775, dir); err != nil {
			if log != nil {
				log.Err(err).Send()
			}
			return err
		}
	}

	return nil
}

// Migrate performs database migrations
// This function respects DB_TYPE and DATABASE_URL environment variables
func Migrate() error {
	return db.Init(migrations.Embed())
}
