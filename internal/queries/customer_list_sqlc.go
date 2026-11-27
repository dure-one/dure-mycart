//go:build sqlc

package queries

import (
	"context"
	"database/sql"
	"strings"

	"github.com/shurco/mycart/internal/models"
	pggen "github.com/dure-one/dure-mycart/internal/queries/sqlcgen/postgres"
	sqlitegen "github.com/dure-one/dure-mycart/internal/queries/sqlcgen/sqlite"
	"github.com/shurco/mycart/pkg/litepay"
)

// CustomerFilter narrows the admin customer list.
type CustomerFilter struct {
	// Search matches a substring of the address or the name, case-insensitively.
	Search string
	// RegisteredOnly drops the buyers who never created an account, leaving
	// only the rows an account-level action can be applied to.
	RegisteredOnly bool
	// Currency is the currency the spent totals are computed in. Totals cover
	// the given currency only, and the count of purchases covers everything.
	Currency string
}

// likeContains renders a substring pattern for LIKE with the wildcards escaped,
// so that searching for "100%" looks for those four characters instead of
// matching every row. The queries pair it with `LIKE … ESCAPE '\'`.
func likeContains(s string) string {
	escape := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)
	return "%" + escape.Replace(strings.ToLower(s)) + "%"
}

// Customers returns one row per address the shop knows about, newest activity
// first.
//
// A "customer" here is an address, not an account: the list is the union of
// the registered accounts and the addresses that have paid at least once.
// Addresses are compared trimmed and lower-cased throughout (see the queries
// in sqlcgen/queries/*/customer.sql).
func (q *CustomerQueries) Customers(
	ctx context.Context, filter CustomerFilter, limit, offset int,
) ([]*models.CustomerSummary, int, error) {
	paid := sql.NullString{String: string(litepay.PAID), Valid: true}
	anyName := filter.Search == ""
	pattern := ""
	if !anyName {
		pattern = likeContains(filter.Search)
	}

	total, err := pick(q.DB, q.DB,
		func(g *pggen.Queries) (int64, error) {
			return g.CountCustomerAddresses(ctx, pggen.CountCustomerAddressesParams{
				Paid: paid, AnyName: anyName, Pattern: pattern, RegisteredOnly: filter.RegisteredOnly,
			})
		},
		func(g *sqlitegen.Queries) (int64, error) {
			return g.CountCustomerAddresses(ctx, sqlitegen.CountCustomerAddressesParams{
				Paid: paid, AnyName: anyName, Pattern: pattern, RegisteredOnly: filter.RegisteredOnly,
			})
		})
	if err != nil {
		return nil, 0, err
	}

	params := pggen.CustomerSummariesParams{
		Paid:           paid,
		Currency:       filter.Currency,
		AnyName:        anyName,
		Pattern:        pattern,
		RegisteredOnly: filter.RegisteredOnly,
		Lim:            int64(limit),
		Off:            int64(offset),
	}
	rows, err := pick(q.DB, q.DB,
		func(g *pggen.Queries) ([]pggen.CustomerSummariesRow, error) { return g.CustomerSummaries(ctx, params) },
		func(g *sqlitegen.Queries) ([]pggen.CustomerSummariesRow, error) {
			r, err := g.CustomerSummaries(ctx, sqlitegen.CustomerSummariesParams(params))
			return convertAll(r, err, func(x sqlitegen.CustomerSummariesRow) pggen.CustomerSummariesRow {
				return pggen.CustomerSummariesRow(x)
			})
		})
	if err != nil {
		return nil, 0, err
	}

	customers := make([]*models.CustomerSummary, 0, len(rows))
	for _, r := range rows {
		summary := &models.CustomerSummary{
			Email:      r.Email,
			Name:       r.Name,
			Active:     r.Active.Bool,
			Registered: r.AccountID.Valid,
			Purchases:  int(r.Purchases),
			Spent:      int(r.Spent),
			LastOrder:  r.LastOrder,
			// The totals are in this currency by construction, so the row has
			// to say which one that was.
			Currency: filter.Currency,
		}
		summary.ID, summary.Created, summary.Updated = r.AccountID.String, r.Created, r.Updated
		customers = append(customers, summary)
	}

	return customers, int(total), nil
}
