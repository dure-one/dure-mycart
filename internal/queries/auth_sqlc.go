//go:build sqlc

package queries

import (
	"context"

	"github.com/shurco/mycart/internal/database"
	pggen "github.com/dure-one/dure-mycart/internal/queries/sqlcgen/postgres"
	sqlitegen "github.com/dure-one/dure-mycart/internal/queries/sqlcgen/sqlite"
	"github.com/shurco/mycart/pkg/errors"
)

// AuthQueries is a struct that holds a dialect-aware database handle to provide database functionality.
// This structure can be used to create methods that will execute SQL queries related to authentication.
type AuthQueries struct {
	DB *database.Conn
}

// GetPasswordByEmail retrieves the password for a user by their email.
func (q *AuthQueries) GetPasswordByEmail(ctx context.Context, email string) (string, error) {
	rows, err := pick(q.DB, q.DB,
		func(g *pggen.Queries) ([]pggen.GetAuthSettingsRow, error) { return g.GetAuthSettings(ctx) },
		func(g *sqlitegen.Queries) ([]pggen.GetAuthSettingsRow, error) {
			r, err := g.GetAuthSettings(ctx)
			return convertAll(r, err, func(x sqlitegen.GetAuthSettingsRow) pggen.GetAuthSettingsRow { return pggen.GetAuthSettingsRow(x) })
		})
	if err != nil {
		return "", err
	}

	for _, row := range rows {
		switch row.Key {
		case "email":
			if row.Value.String != email {
				return "", errors.ErrUserEmailNotFound
			}
		case "password":
			if row.Value.String == "" {
				return "", errors.ErrUserPasswordNotFound
			}
			return row.Value.String, nil
		}
	}

	return "", errors.ErrUserNotFound
}
