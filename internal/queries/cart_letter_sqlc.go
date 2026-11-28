//go:build sqlc

package queries

import (
	"context"
	"database/sql"
	"encoding/json"
	stderrors "errors"
	"fmt"
	"strings"

	"github.com/dure-one/dure-mycart/internal/database"
	"github.com/dure-one/dure-mycart/internal/models"
	pggen "github.com/dure-one/dure-mycart/internal/queries/sqlcgen/postgres"
	sqlitegen "github.com/dure-one/dure-mycart/internal/queries/sqlcgen/sqlite"
	"github.com/dure-one/dure-mycart/pkg/errors"
	"github.com/dure-one/dure-mycart/pkg/litepay"
)

// CartLetterPayment is ...
func (q *CartQueries) CartLetterPayment(ctx context.Context, email, amountPayment, paymentURL string) (*models.MessageMail, error) {
	mailLetter, err := NewBase(q.DB).GetSettingByKey(ctx, "site_name", "mail_letter_payment")
	if err != nil {
		return nil, err
	}
	letterTemplate := models.Letter{}
	if err := json.Unmarshal([]byte(mailLetter["mail_letter_payment"].Value.(string)), &letterTemplate); err != nil {
		return nil, err
	}

	mail := &models.MessageMail{
		To:     email,
		Letter: letterTemplate,
		Data: map[string]string{
			"Payment_URL":    paymentURL,
			"Site_Name":      mailLetter["site_name"].Value.(string),
			"Amount_Payment": amountPayment,
		},
	}

	return mail, nil
}

// CartLetterPurchase is ...
func (q *CartQueries) CartLetterPurchase(ctx context.Context, cartID string) (*models.MessageMail, error) {
	mail := &models.MessageMail{}

	email, products, err := q.paidCartLines(ctx, cartID)
	if err != nil {
		return nil, err
	}
	mail.To = email

	keys, files, err := q.claimPurchasedGoods(ctx, cartID, products)
	if err != nil {
		return nil, err
	}

	purchases := purchasesText(keys, files)

	// Fetch the 'mail_letter_purchase' setting value.
	mailLetter, err := NewBase(q.DB).GetSettingByKey(ctx, "email", "mail_letter_purchase")
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal([]byte(mailLetter["mail_letter_purchase"].Value.(string)), &mail.Letter); err != nil {
		return nil, err
	}

	mail.Data = map[string]string{
		"Purchases":   purchases,
		"Admin_Email": mailLetter["email"].Value.(string),
	}
	mail.Files = files

	return mail, nil
}

// paidCartLines returns the buyer's address and the lines of a paid cart, or
// errors.ErrPageNotFound when there is no such paid cart.
func (q *CartQueries) paidCartLines(ctx context.Context, cartID string) (string, []models.CartProduct, error) {
	params := pggen.PaidCartLinesParams{Paid: sql.NullString{String: string(litepay.PAID), Valid: true}, ID: cartID}
	r, err := pick(q.DB, q.DB,
		func(g *pggen.Queries) (pggen.PaidCartLinesRow, error) { return g.PaidCartLines(ctx, params) },
		func(g *sqlitegen.Queries) (pggen.PaidCartLinesRow, error) {
			r, err := g.PaidCartLines(ctx, sqlitegen.PaidCartLinesParams(params))
			return pggen.PaidCartLinesRow(r), err
		})
	if err != nil {
		if stderrors.Is(err, sql.ErrNoRows) {
			return "", nil, errors.ErrPageNotFound
		}
		return "", nil, err
	}

	products := []models.CartProduct{}
	if err := json.Unmarshal([]byte(r.Cart), &products); err != nil {
		return "", nil, err
	}

	return r.Email.String, products, nil
}

// claimPurchasedGoods hands the buyer of cartID the files and keys of every
// digital product in products, within one transaction.
func (q *CartQueries) claimPurchasedGoods(ctx context.Context, cartID string, products []models.CartProduct) ([]models.Data, []models.File, error) {
	tx, err := q.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, nil, err
	}
	defer func() { _ = tx.Rollback() }()

	keys := []models.Data{}
	files := []models.File{}
	for _, cart := range products {
		digital, err := pick(q.DB, tx,
			func(g *pggen.Queries) (sql.NullString, error) { return g.ProductDigital(ctx, cart.ProductID) },
			func(g *sqlitegen.Queries) (sql.NullString, error) { return g.ProductDigital(ctx, cart.ProductID) })
		if err != nil {
			if stderrors.Is(err, sql.ErrNoRows) {
				return nil, nil, errors.ErrPageNotFound
			}
			return nil, nil, err
		}

		switch digital.String {
		case models.DigitalFile:
			productFiles, err := scanDigitalFiles(ctx, q.DB, tx, cart.ProductID)
			if err != nil {
				return nil, nil, err
			}
			files = append(files, productFiles...)
		case models.DigitalData:
			key, err := claimDigitalData(ctx, q.DB, tx, cartID, cart.ProductID)
			if err != nil {
				return nil, nil, err
			}
			keys = append(keys, key)
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, nil, err
	}

	return keys, files, nil
}

// purchasesText lists the claimed keys and files, numbered, for the purchase letter.
func purchasesText(keys []models.Data, files []models.File) string {
	var purchases strings.Builder
	count := 1
	if len(keys) > 0 {
		purchases.WriteString("Keys:\n")
		for _, key := range keys {
			purchases.WriteString(fmt.Sprintf("%v: %s\n", count, key.Content))
			count++
		}
	}
	if len(files) > 0 {
		purchases.WriteString("Files:\n")
		for _, file := range files {
			purchases.WriteString(fmt.Sprintf("%v: %s\n", count, file.OrigName))
			count++
		}
	}

	return purchases.String()
}

// claimDigitalData hands one unassigned digital_data row to cartID and returns
// it. Already owning a row makes the call idempotent, so a retried purchase
// mail keeps its keys.
//
// The row is claimed by an UPDATE guarded on `cart_id IS NULL`, and the claim
// only counts when it reports having changed a row: reading a candidate and
// then updating it unconditionally lets two concurrent purchases hand the same
// key to both buyers.
func claimDigitalData(ctx context.Context, conn *database.Conn, tx *database.Tx, cartID, productID string) (models.Data, error) {
	// Bounded so that a pathological amount of contention fails loudly instead
	// of spinning; each lost round is another buyer taking a key.
	const maxAttempts = 100

	cart := sql.NullString{String: cartID, Valid: true}
	for range maxAttempts {
		owned, err := pick(conn, tx,
			func(g *pggen.Queries) (pggen.ClaimedDigitalDataRow, error) {
				return g.ClaimedDigitalData(ctx, pggen.ClaimedDigitalDataParams{CartID: cart, ProductID: productID})
			},
			func(g *sqlitegen.Queries) (pggen.ClaimedDigitalDataRow, error) {
				r, err := g.ClaimedDigitalData(ctx, sqlitegen.ClaimedDigitalDataParams{CartID: cart, ProductID: productID})
				return pggen.ClaimedDigitalDataRow(r), err
			})
		if err == nil {
			return models.Data{ID: owned.ID, Content: owned.Content}, nil
		}
		if !stderrors.Is(err, sql.ErrNoRows) {
			return models.Data{}, err
		}

		candidate, err := pick(conn, tx,
			func(g *pggen.Queries) (pggen.FreeDigitalDataRow, error) { return g.FreeDigitalData(ctx, productID) },
			func(g *sqlitegen.Queries) (pggen.FreeDigitalDataRow, error) {
				r, err := g.FreeDigitalData(ctx, productID)
				return pggen.FreeDigitalDataRow(r), err
			})
		if err != nil {
			if stderrors.Is(err, sql.ErrNoRows) {
				return models.Data{}, errors.ErrPageNotFound
			}
			return models.Data{}, err
		}

		claimed, err := pick(conn, tx,
			func(g *pggen.Queries) (int64, error) {
				return g.ClaimDigitalData(ctx, pggen.ClaimDigitalDataParams{CartID: cart, ID: candidate.ID})
			},
			func(g *sqlitegen.Queries) (int64, error) {
				return g.ClaimDigitalData(ctx, sqlitegen.ClaimDigitalDataParams{CartID: cart, ID: candidate.ID})
			})
		if err != nil {
			return models.Data{}, err
		}
		if claimed == 1 {
			return models.Data{ID: candidate.ID, Content: candidate.Content}, nil
		}
	}

	return models.Data{}, fmt.Errorf("claim digital data for product %s: too much contention", productID)
}

// scanDigitalFiles loads every digital_file row for a product within the given transaction.
func scanDigitalFiles(ctx context.Context, conn *database.Conn, tx *database.Tx, productID string) ([]models.File, error) {
	rows, err := pick(conn, tx,
		func(g *pggen.Queries) ([]pggen.DigitalFilesByProductRow, error) {
			return g.DigitalFilesByProduct(ctx, productID)
		},
		func(g *sqlitegen.Queries) ([]pggen.DigitalFilesByProductRow, error) {
			r, err := g.DigitalFilesByProduct(ctx, productID)
			return convertAll(r, err, func(x sqlitegen.DigitalFilesByProductRow) pggen.DigitalFilesByProductRow {
				return pggen.DigitalFilesByProductRow(x)
			})
		})
	if err != nil {
		return nil, err
	}

	var files []models.File
	for _, r := range rows {
		files = append(files, models.File{ID: r.ID, Name: r.Name, Ext: r.Ext, OrigName: r.OrigName})
	}
	return files, nil
}
