//go:build sqlc

package queries

import (
	"context"
	"database/sql"

	"github.com/dure-one/dure-mycart/internal/models"
	pggen "github.com/dure-one/dure-mycart/internal/queries/sqlcgen/postgres"
	sqlitegen "github.com/dure-one/dure-mycart/internal/queries/sqlcgen/sqlite"
	"github.com/dure-one/dure-mycart/pkg/errors"
	"github.com/dure-one/dure-mycart/pkg/security"
)

// Account returns the storefront account settings.
func (q *CustomerQueries) Account(ctx context.Context) (*models.Account, error) {
	setting, err := NewBase(q.DB).GetSettingByGroup(ctx, new(models.Account))
	if err != nil {
		return nil, err
	}
	return setting.(*models.Account), nil
}

// AccountEnabled reports whether the customer cabinet is available. It is FALSE
// on a fresh installation.
func (q *CustomerQueries) AccountEnabled(ctx context.Context) (bool, error) {
	account, err := q.Account(ctx)
	if err != nil {
		return false, err
	}
	return account.Enabled, nil
}

// accountSecretValue reads the stored storefront signing key.
func (q *CustomerQueries) accountSecretValue(ctx context.Context) (string, error) {
	value, err := pick(q.DB, q.DB,
		func(g *pggen.Queries) (sql.NullString, error) { return g.GetSettingValue(ctx, accountSecretKey) },
		func(g *sqlitegen.Queries) (sql.NullString, error) { return g.GetSettingValue(ctx, accountSecretKey) })
	return value.String, err
}

// AccountSecret returns the key that signs storefront session tokens, creating
// it on first use.
//
// It must not be the admin jwt_secret: both keys sign tokens whose only claim
// is a session id, and both middlewares accept a token on the strength of a
// row in the shared session table. The secret is generated here rather than
// seeded by a migration because a key written into a public SQL file is a
// published key.
func (q *CustomerQueries) AccountSecret(ctx context.Context) (string, error) {
	value, err := q.accountSecretValue(ctx)
	if err != nil {
		return "", err
	}
	if value != "" {
		return value, nil
	}

	token, err := security.NewToken(security.RandomString())
	if err != nil {
		return "", err
	}

	// Fill the empty row, then re-read it. Two requests that find the secret
	// missing at the same moment must end up agreeing on one key: the loser of
	// this race returns what was stored, not what it generated.
	val := sql.NullString{String: token, Valid: true}
	if err := run(q.DB, q.DB,
		func(g *pggen.Queries) error {
			return g.FillEmptySetting(ctx, pggen.FillEmptySettingParams{Value: val, Key: accountSecretKey})
		},
		func(g *sqlitegen.Queries) error {
			return g.FillEmptySetting(ctx, sqlitegen.FillEmptySettingParams{Value: val, Key: accountSecretKey})
		}); err != nil {
		return "", err
	}

	value, err = q.accountSecretValue(ctx)
	if err != nil {
		return "", err
	}
	if value == "" {
		return "", errors.ErrSettingNotFound
	}

	return value, nil
}
