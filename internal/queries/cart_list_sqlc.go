//go:build sqlc

package queries

import (
	"context"
	"database/sql"

	"github.com/dure-one/dure-mycart/internal/models"
	pggen "github.com/dure-one/dure-mycart/internal/queries/sqlcgen/postgres"
	sqlitegen "github.com/dure-one/dure-mycart/internal/queries/sqlcgen/sqlite"
	"github.com/dure-one/dure-mycart/pkg/litepay"
)

// cartFromListRow maps the projection every cart listing shares. Carts and
// CartsByEmail differ only in their WHERE clause, so their rows convert into
// this one type.
func cartFromListRow(r pggen.ListCartsRow) *models.Cart {
	cart := &models.Cart{
		Email:         r.Email.String,
		AmountTotal:   int(r.AmountTotal),
		Currency:      r.Currency,
		PaymentID:     r.PaymentID.String,
		PaymentStatus: litepay.Status(r.PaymentStatus.String),
		PaymentSystem: litepay.PaymentSystem(r.PaymentSystem),
	}
	cart.ID, cart.Created, cart.Updated = r.ID, r.Created, r.Updated
	return cart
}

// Carts retrieves a list of carts from the database.
func (q *CartQueries) Carts(ctx context.Context, limit, offset int) ([]*models.Cart, int, error) {
	lim, off := pageBounds(limit, offset)

	rows, err := pick(q.DB, q.DB,
		func(g *pggen.Queries) ([]pggen.ListCartsRow, error) {
			return g.ListCarts(ctx, pggen.ListCartsParams{Lim: pgLimit(lim), Off: off})
		},
		func(g *sqlitegen.Queries) ([]pggen.ListCartsRow, error) {
			r, err := g.ListCarts(ctx, sqlitegen.ListCartsParams{Lim: lim, Off: off})
			return convertAll(r, err, func(x sqlitegen.ListCartsRow) pggen.ListCartsRow { return pggen.ListCartsRow(x) })
		})
	if err != nil {
		return nil, 0, err
	}

	carts := make([]*models.Cart, 0, len(rows))
	for _, r := range rows {
		carts = append(carts, cartFromListRow(r))
	}

	total, err := pick(q.DB, q.DB,
		func(g *pggen.Queries) (int64, error) { return g.CountCarts(ctx) },
		func(g *sqlitegen.Queries) (int64, error) { return g.CountCarts(ctx) })
	if err != nil {
		return nil, 0, err
	}

	return carts, int(total), nil
}

// CartsByEmail returns every cart created for an address, newest first, in any
// payment state.
//
// The address is matched after trimming and lower-casing, which is how the
// cabinet stores a login: checkout writes the address exactly as the buyer
// typed it, so "Anna@Example.com " and "anna@example.com" are one customer.
func (q *CartQueries) CartsByEmail(ctx context.Context, email string) ([]*models.Cart, error) {
	email = NormalizeEmail(email)
	rows, err := pick(q.DB, q.DB,
		func(g *pggen.Queries) ([]pggen.ListCartsRow, error) {
			r, err := g.CartsByEmail(ctx, email)
			return convertAll(r, err, func(x pggen.CartsByEmailRow) pggen.ListCartsRow { return pggen.ListCartsRow(x) })
		},
		func(g *sqlitegen.Queries) ([]pggen.ListCartsRow, error) {
			r, err := g.CartsByEmail(ctx, email)
			return convertAll(r, err, func(x sqlitegen.CartsByEmailRow) pggen.ListCartsRow { return pggen.ListCartsRow(x) })
		})
	if err != nil {
		return nil, err
	}

	carts := make([]*models.Cart, 0, len(rows))
	for _, r := range rows {
		carts = append(carts, cartFromListRow(r))
	}
	return carts, nil
}

// PaidCartsByEmail returns the paid carts of an address, newest first, with the
// lines of each one.
//
// It is a third listing rather than a filter over CartsByEmail because only the
// cabinet wants the cart JSON; leaving it out of the shared projection keeps an
// admin page from carrying the contents of every abandoned cart along with it.
func (q *CartQueries) PaidCartsByEmail(ctx context.Context, email string) ([]*models.Cart, error) {
	params := pggen.PaidCartsByEmailParams{
		Email: NormalizeEmail(email),
		Paid:  sql.NullString{String: string(litepay.PAID), Valid: true},
	}
	rows, err := pick(q.DB, q.DB,
		func(g *pggen.Queries) ([]pggen.PaidCartsByEmailRow, error) { return g.PaidCartsByEmail(ctx, params) },
		func(g *sqlitegen.Queries) ([]pggen.PaidCartsByEmailRow, error) {
			r, err := g.PaidCartsByEmail(ctx, sqlitegen.PaidCartsByEmailParams(params))
			return convertAll(r, err, func(x sqlitegen.PaidCartsByEmailRow) pggen.PaidCartsByEmailRow {
				return pggen.PaidCartsByEmailRow(x)
			})
		})
	if err != nil {
		return nil, err
	}

	carts := make([]*models.Cart, 0, len(rows))
	for _, r := range rows {
		cart := cartFromListRow(pggen.ListCartsRow{
			ID: r.ID, Email: r.Email, AmountTotal: r.AmountTotal, Currency: r.Currency,
			PaymentID: r.PaymentID, PaymentStatus: r.PaymentStatus, PaymentSystem: r.PaymentSystem,
			Created: r.Created, Updated: r.Updated,
		})
		if err := decodeCartLines(r.Cart, cart); err != nil {
			return nil, err
		}
		carts = append(carts, cart)
	}
	return carts, nil
}
