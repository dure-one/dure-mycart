package app

import (
	"context"
	"fmt"

	"github.com/shurco/mycart/internal/database"
	"github.com/shurco/mycart/internal/models"
	"github.com/shurco/mycart/internal/queries"
)

// InstallAdmin performs first-time setup with the given admin credentials.
// It initializes storage directories, runs migrations when needed, and
// creates the admin account. Safe to run from a one-shot container job.
func InstallAdmin(ctx context.Context, dbCfg database.Config, install *models.Install) error {
	if err := install.Validate(); err != nil {
		return fmt.Errorf("validate install: %w", err)
	}

	if err := Init(dbCfg); err != nil {
		return fmt.Errorf("init: %w", err)
	}

	if err := queries.DB().Install(ctx, install); err != nil {
		return fmt.Errorf("install: %w", err)
	}

	// Record a database that was chosen explicitly, so a later `serve` starts
	// on the same one. The data now lives there: falling back to the built-in
	// default would serve an empty shop. The default itself is not recorded —
	// an installation nobody configured keeps reading the default.
	if dbCfg.Source != database.SourceDefault {
		if err := database.WriteConfig(dbCfg); err != nil {
			return fmt.Errorf("write database config: %w", err)
		}
	}

	return nil
}
