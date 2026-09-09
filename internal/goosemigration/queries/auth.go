package queries

import (
	"context"
	"database/sql"

	"github.com/shurco/mycart/internal/store/db/postgres"
	"github.com/shurco/mycart/internal/store/db/sqlite"
	"github.com/shurco/mycart/pkg/errors"
)

// AuthQueries is a struct that embeds *sql.DB to provide database functionality.
// This structure can be used to create methods that will execute SQL queries related to authentication.
type AuthQueries struct {
	*sql.DB
}

// GetPasswordByEmail retrieves the password for a user by their email using sqlc.
func (q *AuthQueries) GetPasswordByEmail(ctx context.Context, email string) (string, error) {
	queries := getSQLCQueries()

	// Get email setting
	var emailSetting, passwordSetting interface{}
	var err error

	if DBType() == "postgres" {
		pgQueries := queries.(*postgres.Queries)
		emailSetting, err = pgQueries.GetSettingByKey(ctx, "email")
		if err != nil {
			return "", err
		}
		passwordSetting, err = pgQueries.GetSettingByKey(ctx, "password")
		if err != nil {
			return "", err
		}
	} else {
		sqliteQueries := queries.(*sqlite.Queries)
		emailSetting, err = sqliteQueries.GetSettingByKey(ctx, "email")
		if err != nil {
			return "", err
		}
		passwordSetting, err = sqliteQueries.GetSettingByKey(ctx, "password")
		if err != nil {
			return "", err
		}
	}

	// Convert and validate email
	emailModel := toModelSetting(emailSetting)
	emailValue, _ := emailModel.Value.(string)
	if emailValue != email {
		return "", errors.ErrUserEmailNotFound
	}

	// Convert and validate password
	passwordModel := toModelSetting(passwordSetting)
	passwordValue, _ := passwordModel.Value.(string)
	if passwordValue == "" {
		return "", errors.ErrUserPasswordNotFound
	}

	return passwordValue, nil
}
