package queries

import (
	"context"
	"database/sql"
	"errors"
	"strconv"

	"github.com/shurco/mycart/internal/store/db/postgres"
	"github.com/shurco/mycart/internal/store/db/sqlite"
	"github.com/shurco/mycart/internal/models"
	"github.com/shurco/mycart/pkg/security"
)

// ErrAlreadyInstalled is returned by Install if the cart has already been initialized.
var ErrAlreadyInstalled = errors.New("cart already installed")

// IsInstalled reports whether the cart has completed first-time setup using sqlc.
func (q *InstallQueries) IsInstalled(ctx context.Context) (bool, error) {
	queries := getSQLCQueries()

	var result interface{}
	var err error

	if DBType() == "postgres" {
		pgQueries := queries.(*postgres.Queries)
		result, err = pgQueries.GetSettingByKey(ctx, "installed")
	} else {
		sqliteQueries := queries.(*sqlite.Queries)
		result, err = sqliteQueries.GetSettingByKey(ctx, "installed")
	}

	if err != nil {
		return false, err
	}

	setting := toModelSetting(result)
	valueStr, _ := setting.Value.(string)
	installed, _ := strconv.ParseBool(valueStr)
	return installed, nil
}

// InstallQueries is a struct that embeds a pointer to an sql.DB.
// This allows for the struct to have all the methods of sql.DB,
// enabling it to perform database operations directly.
type InstallQueries struct {
	*sql.DB
}

// Install performs the installation process for the cart system using sqlc.
func (q *InstallQueries) Install(ctx context.Context, i *models.Install) error {
	installed, err := q.IsInstalled(ctx)
	if err != nil {
		return err
	}
	if installed {
		return ErrAlreadyInstalled
	}

	tx, err := q.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	passwordHash := security.GeneratePassword(i.Password)
	jwt_secret, err := security.NewToken(passwordHash)
	if err != nil {
		return err
	}

	settings := map[string]string{
		"installed":  "true",
		"domain":     i.Domain,
		"email":      i.Email,
		"password":   passwordHash,
		"jwt_secret": jwt_secret,
		"db_type":    i.DBType,
	}

	// Add database-specific connection details
	if i.DBType == "postgres" {
		settings["database_url"] = i.DatabaseURL
	} else if i.DBType == "sqlite" {
		settings["sqlite_path"] = i.SQLitePath
	}

	// Create transaction-bound queries to prevent deadlock
	// Using queries from global DB while transaction is open causes blocking
	var txQueries interface{}
	if DBType() == "postgres" {
		txQueries = postgres.New(tx)
	} else {
		txQueries = sqlite.New(tx)
	}

	for key, value := range settings {
		nullValue := sql.NullString{String: value, Valid: value != ""}

		if DBType() == "postgres" {
			pgQueries := txQueries.(*postgres.Queries)
			params := postgres.UpdateSettingParams{
				Value: nullValue,
				Key:   key,
			}
			if err := pgQueries.UpdateSetting(ctx, params); err != nil {
				return err
			}
		} else {
			sqliteQueries := txQueries.(*sqlite.Queries)
			params := sqlite.UpdateSettingParams{
				Value: nullValue,
				Key:   key,
			}
			if err := sqliteQueries.UpdateSetting(ctx, params); err != nil {
				return err
			}
		}
	}

	return tx.Commit()
}
