//go:build sqlc

package queries

import (
	"context"
	"database/sql"
	"errors"
	"strconv"

	"github.com/dure-one/dure-mycart/internal/database"
	"github.com/dure-one/dure-mycart/internal/models"
	pggen "github.com/dure-one/dure-mycart/internal/queries/sqlcgen/postgres"
	sqlitegen "github.com/dure-one/dure-mycart/internal/queries/sqlcgen/sqlite"
	"github.com/dure-one/dure-mycart/pkg/security"
)

// ErrAlreadyInstalled is returned by Install if the cart has already been initialized.
var ErrAlreadyInstalled = errors.New("cart already installed")

// InstallQueries is a struct that holds a dialect-aware database handle.
// This allows the struct to perform database operations directly.
type InstallQueries struct {
	DB *database.Conn
}

// IsInstalled reports whether the cart has completed first-time setup.
func (q *InstallQueries) IsInstalled(ctx context.Context) (bool, error) {
	raw, err := pick(q.DB, q.DB,
		func(g *pggen.Queries) (sql.NullString, error) { return g.GetInstalled(ctx) },
		func(g *sqlitegen.Queries) (sql.NullString, error) { return g.GetInstalled(ctx) })
	if err != nil {
		return false, err
	}
	installed, _ := strconv.ParseBool(raw.String)
	return installed, nil
}

// Install performs the installation process for the cart system.
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

	passwordHash, err := security.HashPassword(i.Password)
	if err != nil {
		return err
	}
	jwtSecret, err := security.NewToken(passwordHash)
	if err != nil {
		return err
	}

	settings := map[string]string{
		"installed":  "true",
		"domain":     i.Domain,
		"email":      i.Email,
		"password":   passwordHash,
		"jwt_secret": jwtSecret,
	}

	for key, value := range settings {
		val := sql.NullString{String: value, Valid: true}
		if err := run(q.DB, tx,
			func(g *pggen.Queries) error {
				return g.SetSettingValue(ctx, pggen.SetSettingValueParams{Value: val, Key: key})
			},
			func(g *sqlitegen.Queries) error {
				return g.SetSettingValue(ctx, sqlitegen.SetSettingValueParams{Value: val, Key: key})
			}); err != nil {
			return err
		}
	}

	return tx.Commit()
}
