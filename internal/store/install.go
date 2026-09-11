package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"

	"github.com/shurco/mycart/internal/models"
	"github.com/shurco/mycart/internal/store/db"
	"github.com/shurco/mycart/internal/store/db/postgres"
	"github.com/shurco/mycart/internal/store/db/sqlite"
	"github.com/shurco/mycart/pkg/security"
)

var ErrAlreadyInstalled = errors.New("cart already installed")

// IsInstalled checks if the application has been installed.
func IsInstalled(ctx context.Context) (bool, error) {
	setting, err := db.GetSettingByKeyFunc(ctx, "installed")
	if err != nil {
		return false, err
	}
	installed, _ := strconv.ParseBool(setting.Value.String)
	return installed, nil
}

// Install performs the initial application installation.
func Install(ctx context.Context, request *models.Install) error {
	installed, err := IsInstalled(ctx)
	if err != nil {
		return err
	}
	if installed {
		return ErrAlreadyInstalled
	}

	tx, err := db.DB().BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	passwordHash := security.GeneratePassword(request.Password)
	jwtSecret, err := security.NewToken(passwordHash)
	if err != nil {
		return err
	}

	settings := map[string]string{
		"installed":  "true",
		"domain":     request.Domain,
		"email":      request.Email,
		"password":   passwordHash,
		"jwt_secret": jwtSecret,
		"db_type":    request.DBType,
	}

	// Add database-specific connection details
	if request.DBType == "postgres" {
		settings["database_url"] = request.DatabaseURL
	} else if request.DBType == "sqlite" {
		settings["sqlite_path"] = request.SQLitePath
	}

	// Create transaction-bound queries
	dbType := db.Type()
	for key, value := range settings {
		nullValue := sql.NullString{String: value, Valid: value != ""}

		if dbType == "postgres" {
			pgQueries := postgres.New(tx)
			params := postgres.UpdateSettingParams{
				Value: nullValue,
				Key:   key,
			}
			if err := pgQueries.UpdateSetting(ctx, params); err != nil {
				return fmt.Errorf("failed to update setting %s: %w", key, err)
			}
		} else {
			sqliteQueries := sqlite.New(tx)
			params := sqlite.UpdateSettingParams{
				Value: nullValue,
				Key:   key,
			}
			if err := sqliteQueries.UpdateSetting(ctx, params); err != nil {
				return fmt.Errorf("failed to update setting %s: %w", key, err)
			}
		}
	}

	return tx.Commit()
}
