//go:build sqlc

package queries

import (
	"context"
	"database/sql"
	"encoding/json"
	stderrors "errors"
	"strconv"
	"strings"

	"github.com/shurco/mycart/internal/database"
	"github.com/shurco/mycart/internal/models"
	pggen "github.com/shurco/mycart/internal/queries/sqlcgen/postgres"
	sqlitegen "github.com/shurco/mycart/internal/queries/sqlcgen/sqlite"
	"github.com/shurco/mycart/pkg/errors"
	"github.com/shurco/mycart/pkg/litepay"
)

// CartQueries is a struct that holds a dialect-aware database handle.
type CartQueries struct {
	DB *database.Conn
}

// PaymentList retrieves the status of different payment methods from the database.
func (q *CartQueries) PaymentList(ctx context.Context) (map[string]bool, error) {
	rows, err := pick(q.DB, q.DB,
		func(g *pggen.Queries) ([]pggen.PaymentActiveSettingsRow, error) { return g.PaymentActiveSettings(ctx) },
		func(g *sqlitegen.Queries) ([]pggen.PaymentActiveSettingsRow, error) {
			r, err := g.PaymentActiveSettings(ctx)
			return convertAll(r, err, func(x sqlitegen.PaymentActiveSettingsRow) pggen.PaymentActiveSettingsRow {
				return pggen.PaymentActiveSettingsRow(x)
			})
		})
	if err != nil {
		return nil, err
	}

	payments := map[string]bool{}
	for _, row := range rows {
		vBool, err := strconv.ParseBool(row.Value.String)
		if err != nil {
			return nil, err
		}
		payments[strings.ReplaceAll(row.Key, "_active", "")] = vBool
	}

	// Dummy provider is always active (no database check needed)
	payments["dummy"] = true

	return payments, nil
}

// Cart retrieves a cart from the database using the provided cartId, or
// errors.ErrCartNotFound.
func (q *CartQueries) Cart(ctx context.Context, cartId string) (*models.Cart, error) {
	r, err := pick(q.DB, q.DB,
		func(g *pggen.Queries) (pggen.CartByIDRow, error) { return g.CartByID(ctx, cartId) },
		func(g *sqlitegen.Queries) (pggen.CartByIDRow, error) {
			r, err := g.CartByID(ctx, cartId)
			return pggen.CartByIDRow(r), err
		})
	if err != nil {
		if stderrors.Is(err, sql.ErrNoRows) {
			return nil, errors.ErrCartNotFound
		}
		return nil, err
	}

	cart := &models.Cart{
		Email:         r.Email.String,
		AmountTotal:   int(r.AmountTotal),
		Currency:      r.Currency,
		PaymentID:     r.PaymentID.String,
		PaymentStatus: litepay.Status(r.PaymentStatus.String),
		PaymentSystem: litepay.PaymentSystem(r.PaymentSystem),
	}
	cart.ID, cart.Created, cart.Updated = r.ID, r.Created, r.Updated

	if err := decodeCartLines(r.Cart, cart); err != nil {
		return nil, err
	}
	return cart, nil
}

// AddCart inserts a new cart into the database.
func (q *CartQueries) AddCart(ctx context.Context, cart *models.Cart) error {
	byteCart, err := json.Marshal(cart.Cart)
	if err != nil {
		return err
	}

	params := pggen.InsertCartParams{
		ID:            cart.ID,
		Email:         sql.NullString{String: cart.Email, Valid: true},
		Cart:          string(byteCart),
		AmountTotal:   int64(cart.AmountTotal),
		Currency:      cart.Currency,
		PaymentID:     sql.NullString{String: cart.PaymentID, Valid: true},
		PaymentStatus: sql.NullString{String: string(cart.PaymentStatus), Valid: true},
		PaymentSystem: string(cart.PaymentSystem),
	}
	return run(q.DB, q.DB,
		func(g *pggen.Queries) error { return g.InsertCart(ctx, params) },
		func(g *sqlitegen.Queries) error { return g.InsertCart(ctx, sqlitegen.InsertCartParams(params)) })
}

// UpdateCart updates the cart details in the database. An empty PaymentID or
// PaymentStatus leaves that column as it is.
func (q *CartQueries) UpdateCart(ctx context.Context, cart *models.Cart) error {
	params := pggen.UpdateCartPaymentParams{
		PaymentID:     sql.NullString{String: cart.PaymentID, Valid: cart.PaymentID != ""},
		PaymentStatus: sql.NullString{String: string(cart.PaymentStatus), Valid: cart.PaymentStatus != ""},
		ID:            cart.ID,
	}
	return run(q.DB, q.DB,
		func(g *pggen.Queries) error { return g.UpdateCartPayment(ctx, params) },
		func(g *sqlitegen.Queries) error {
			return g.UpdateCartPayment(ctx, sqlitegen.UpdateCartPaymentParams(params))
		})
}

// decodeCartLines unmarshals the cart JSON column into the lines of cart.
func decodeCartLines(cartJSON string, cart *models.Cart) error {
	if cartJSON != "" {
		return json.Unmarshal([]byte(cartJSON), &cart.Cart)
	}
	return nil
}
