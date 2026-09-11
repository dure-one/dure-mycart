package app

import (
	"context"
	"fmt"

	"github.com/shurco/mycart/db/migrations"
	"github.com/shurco/mycart/internal/models"
	"github.com/shurco/mycart/internal/store"
	"github.com/shurco/mycart/internal/store/db"
)

// InstallAdmin performs first-time setup with the given admin credentials.
// It initializes storage directories, runs migrations, and creates the admin
// account. Safe to run from a one-shot container job or CLI command.
func InstallAdmin(ctx context.Context, install *models.Install) error {
	if err := install.Validate(); err != nil {
		return fmt.Errorf("validate install: %w", err)
	}

	if err := Init(); err != nil {
		return fmt.Errorf("init: %w", err)
	}

	// Connect to database (reads config from environment)
	if err := db.Connect(); err != nil {
		return fmt.Errorf("connect database: %w", err)
	}

	// Run migrations (required for fresh installations)
	if err := db.Migrate(migrations.Embed()); err != nil {
		return fmt.Errorf("migrate database: %w", err)
	}

	// Mark as installed so future db.Init() calls will run migrations
	db.SetInstalled()

	if err := store.Install(ctx, install); err != nil {
		return fmt.Errorf("install: %w", err)
	}

	return nil
}
