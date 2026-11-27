//go:build sqlc

package queries

import (
	"context"
	"database/sql"
	"strings"

	"github.com/shurco/mycart/internal/database"
	"github.com/shurco/mycart/internal/models"
	pggen "github.com/shurco/mycart/internal/queries/sqlcgen/postgres"
	sqlitegen "github.com/shurco/mycart/internal/queries/sqlcgen/sqlite"
	"github.com/shurco/mycart/pkg/errors"
	"github.com/shurco/mycart/pkg/security"
)

// DefaultAccountExpireHours is how long a cabinet session lasts when the
// operator leaves account_jwt_expire_hours at zero. Thirty days: long enough
// that a buyer is not asked to sign in again on every visit, short enough that
// an abandoned session on a shared device does not live forever.
const DefaultAccountExpireHours = 720

// accountSecretKey is the setting holding the storefront signing key.
const accountSecretKey = "account_jwt_secret"

// CustomerQueries is a struct that holds a dialect-aware database handle for
// the storefront customer accounts.
type CustomerQueries struct {
	DB *database.Conn
}

// NormalizeEmail trims and lower-cases an address so the same person cannot end
// up with two accounts, and so the lookup at sign-in matches what sign-up
// stored regardless of how the address was typed either time.
func NormalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

// customerExists reports whether an address already has an account. It is the
// uniqueness check of CreateCustomer, split out so the same question can be
// asked again after a losing race on the insert.
func (q *CustomerQueries) customerExists(ctx context.Context, email string) (bool, error) {
	taken, err := pick(q.DB, q.DB,
		func(g *pggen.Queries) (int64, error) { return g.CountCustomersByEmail(ctx, email) },
		func(g *sqlitegen.Queries) (int64, error) { return g.CountCustomersByEmail(ctx, email) })
	if err != nil {
		return false, err
	}
	return taken > 0, nil
}

// CreateCustomer stores a new account with the password already hashed.
//
// The uniqueness check is a separate SELECT rather than reliance on the UNIQUE
// constraint, because the two engines report a violation differently and the
// caller needs to tell a taken address from a broken database. The constraint
// still backs it up: two simultaneous sign-ups for one address race past the
// SELECT, and the loser is told the address is taken rather than that the shop
// is broken.
func (q *CustomerQueries) CreateCustomer(ctx context.Context, email, password, name string) (*models.Customer, error) {
	email = NormalizeEmail(email)

	taken, err := q.customerExists(ctx, email)
	if err != nil {
		return nil, err
	}
	if taken {
		return nil, errors.ErrCustomerEmailTaken
	}

	hash, err := security.HashPassword(password)
	if err != nil {
		return nil, err
	}

	customer := &models.Customer{
		Core:     models.Core{ID: security.RandomString()},
		Email:    email,
		Password: hash,
		Name:     strings.TrimSpace(name),
		Active:   true,
	}

	nameVal := sql.NullString{String: customer.Name, Valid: true}
	created, err := pick(q.DB, q.DB,
		func(g *pggen.Queries) (int64, error) {
			return g.InsertCustomer(ctx, pggen.InsertCustomerParams{
				ID: customer.ID, Email: customer.Email, Password: customer.Password, Name: nameVal, Active: customer.Active,
			})
		},
		func(g *sqlitegen.Queries) (int64, error) {
			return g.InsertCustomer(ctx, sqlitegen.InsertCustomerParams{
				ID: customer.ID, Email: customer.Email, Password: customer.Password, Name: nameVal, Active: customer.Active,
			})
		})
	if err != nil {
		// The loser of a race between two sign-ups fails here, on the UNIQUE
		// constraint. The engines spell that violation differently, so the
		// address is looked up again instead: if it exists now, the caller is
		// told what a retry can act on.
		if taken, lookupErr := q.customerExists(ctx, email); lookupErr == nil && taken {
			return nil, errors.ErrCustomerEmailTaken
		}
		return nil, err
	}
	customer.Created = created

	return customer, nil
}

// CustomerByEmail returns the account with the given address, or
// errors.ErrCustomerNotFound.
func (q *CustomerQueries) CustomerByEmail(ctx context.Context, email string) (*models.Customer, error) {
	email = NormalizeEmail(email)
	row, err := pick(q.DB, q.DB,
		func(g *pggen.Queries) (pggen.CustomerByIDRow, error) {
			r, err := g.CustomerByEmail(ctx, email)
			return pggen.CustomerByIDRow(r), err
		},
		func(g *sqlitegen.Queries) (pggen.CustomerByIDRow, error) {
			r, err := g.CustomerByEmail(ctx, email)
			return pggen.CustomerByIDRow(r), err
		})
	return customerFromRow(row, err)
}

// CustomerByID returns the account with the given id, or
// errors.ErrCustomerNotFound.
func (q *CustomerQueries) CustomerByID(ctx context.Context, id string) (*models.Customer, error) {
	row, err := pick(q.DB, q.DB,
		func(g *pggen.Queries) (pggen.CustomerByIDRow, error) { return g.CustomerByID(ctx, id) },
		func(g *sqlitegen.Queries) (pggen.CustomerByIDRow, error) {
			r, err := g.CustomerByID(ctx, id)
			return pggen.CustomerByIDRow(r), err
		})
	return customerFromRow(row, err)
}

// customerFromRow turns a single-row lookup into a customer, and "no rows" into
// the package's sentinel, so handlers branch with errors.Is instead of the
// driver's error.
func customerFromRow(row pggen.CustomerByIDRow, err error) (*models.Customer, error) {
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, errors.ErrCustomerNotFound
		}
		return nil, err
	}

	customer := &models.Customer{
		Email:    row.Email,
		Password: row.Password,
		Name:     row.Name.String,
		Active:   row.Active,
	}
	customer.ID, customer.Created, customer.Updated = row.ID, row.Created, row.Updated
	return customer, nil
}

// SetCustomerActive blocks or unblocks an account.
func (q *CustomerQueries) SetCustomerActive(ctx context.Context, id string, active bool) error {
	return run(q.DB, q.DB,
		func(g *pggen.Queries) error {
			return g.SetCustomerActive(ctx, pggen.SetCustomerActiveParams{Active: active, ID: id})
		},
		func(g *sqlitegen.Queries) error {
			return g.SetCustomerActive(ctx, sqlitegen.SetCustomerActiveParams{Active: active, ID: id})
		})
}

// SetCustomerPassword replaces the stored password hash.
//
// The hash is stored already hashed: this function is deliberately unable to
// take a plaintext password, so no caller can leave one lying in a column.
func (q *CustomerQueries) SetCustomerPassword(ctx context.Context, id, passwordHash string) error {
	return run(q.DB, q.DB,
		func(g *pggen.Queries) error {
			return g.SetCustomerPassword(ctx, pggen.SetCustomerPasswordParams{Password: passwordHash, ID: id})
		},
		func(g *sqlitegen.Queries) error {
			return g.SetCustomerPassword(ctx, sqlitegen.SetCustomerPasswordParams{Password: passwordHash, ID: id})
		})
}

// DeleteCustomer removes an account.
//
// The carts are left in place. They are the record of a payment that happened,
// and an operator deleting an account must not thereby erase the shop's takings.
func (q *CustomerQueries) DeleteCustomer(ctx context.Context, id string) error {
	return run(q.DB, q.DB,
		func(g *pggen.Queries) error { return g.DeleteCustomer(ctx, id) },
		func(g *sqlitegen.Queries) error { return g.DeleteCustomer(ctx, id) })
}

// RevokeCustomerSessions drops every cabinet session belonging to a customer.
//
// The session table is the revocation list for the storefront: a token is
// honoured only while its row exists, and the row's value carries the customer
// id, so deleting by value ends the session on every device the buyer signed in
// from.
func (q *CustomerQueries) RevokeCustomerSessions(ctx context.Context, customerID string) error {
	value := sql.NullString{String: SessionValue(SessionRoleCustomer, customerID), Valid: true}
	return run(q.DB, q.DB,
		func(g *pggen.Queries) error { return g.DeleteSessionsByValue(ctx, value) },
		func(g *sqlitegen.Queries) error { return g.DeleteSessionsByValue(ctx, value) })
}
