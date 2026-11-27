//go:build sqlc

package queries

import (
	"context"
	"database/sql"
	"encoding/json"

	"github.com/shurco/mycart/internal/models"
	pggen "github.com/dure-one/dure-mycart/internal/queries/sqlcgen/postgres"
	sqlitegen "github.com/dure-one/dure-mycart/internal/queries/sqlcgen/sqlite"
	"github.com/shurco/mycart/pkg/errors"
)

// purchaseProducts reads the fields of the products a list of orders shows.
//
// Deliberately neither of the two lookups the rest of the shop uses: a guide
// taken off sale is still owed to the people who paid for it. A product the
// shop has deleted since is absent from the answer, which is how a line whose
// product is gone drops out of the order.
func (q *CustomerQueries) purchaseProducts(ctx context.Context, productIDs []string) (map[string]*models.Product, error) {
	products := map[string]*models.Product{}
	if len(productIDs) == 0 {
		return products, nil
	}

	rows, err := pick(q.DB, q.DB,
		func(g *pggen.Queries) ([]pggen.PurchaseProductsRow, error) {
			ids, err := json.Marshal(productIDs)
			if err != nil {
				return nil, err
			}
			return g.PurchaseProducts(ctx, ids)
		},
		func(g *sqlitegen.Queries) ([]pggen.PurchaseProductsRow, error) {
			r, err := g.PurchaseProducts(ctx, productIDs)
			return convertAll(r, err, func(x sqlitegen.PurchaseProductsRow) pggen.PurchaseProductsRow { return pggen.PurchaseProductsRow(x) })
		})
	if err != nil {
		return nil, err
	}

	for _, r := range rows {
		product := &models.Product{Name: r.Name, Slug: r.Slug}
		product.ID = r.ID
		product.Digital.Type = r.Digital.String
		products[product.ID] = product
	}
	return products, nil
}

// purchaseFiles lists the downloadable files of the given products, keyed by
// the product they belong to, ordered by the name the buyer will see.
func (q *CustomerQueries) purchaseFiles(ctx context.Context, productIDs []string) (map[string][]models.CustomerPurchaseFile, error) {
	files := map[string][]models.CustomerPurchaseFile{}
	if len(productIDs) == 0 {
		return files, nil
	}

	rows, err := pick(q.DB, q.DB,
		func(g *pggen.Queries) ([]pggen.PurchaseFilesRow, error) {
			ids, err := json.Marshal(productIDs)
			if err != nil {
				return nil, err
			}
			return g.PurchaseFiles(ctx, ids)
		},
		func(g *sqlitegen.Queries) ([]pggen.PurchaseFilesRow, error) {
			r, err := g.PurchaseFiles(ctx, productIDs)
			return convertAll(r, err, func(x sqlitegen.PurchaseFilesRow) pggen.PurchaseFilesRow { return pggen.PurchaseFilesRow(x) })
		})
	if err != nil {
		return nil, err
	}

	for _, r := range rows {
		// A row stored without a name would reach the buyer as a blank label;
		// the stored name and extension make a name of sorts.
		origName := r.OrigName
		if origName == "" {
			origName = r.Name + "." + r.Ext
		}
		files[r.ProductID] = append(files[r.ProductID], models.CustomerPurchaseFile{ID: r.ID, OrigName: origName})
	}
	return files, nil
}

// purchaseCodes returns the licence keys the given orders claimed, keyed by the
// order and the product it claimed them for.
//
// Scoped to the orders as well as the product, and that scoping is the whole
// entitlement: a lookup by product alone would print the shop's entire
// remaining stock into the first cabinet that asked.
func (q *CustomerQueries) purchaseCodes(ctx context.Context, cartIDs []string) (map[string][]string, error) {
	codes := map[string][]string{}
	if len(cartIDs) == 0 {
		return codes, nil
	}

	rows, err := pick(q.DB, q.DB,
		func(g *pggen.Queries) ([]pggen.PurchaseCodesRow, error) {
			ids, err := json.Marshal(cartIDs)
			if err != nil {
				return nil, err
			}
			return g.PurchaseCodes(ctx, ids)
		},
		func(g *sqlitegen.Queries) ([]pggen.PurchaseCodesRow, error) {
			ids := make([]sql.NullString, len(cartIDs))
			for i, id := range cartIDs {
				ids[i] = sql.NullString{String: id, Valid: true}
			}
			r, err := g.PurchaseCodes(ctx, ids)
			return convertAll(r, err, func(x sqlitegen.PurchaseCodesRow) pggen.PurchaseCodesRow { return pggen.PurchaseCodesRow(x) })
		})
	if err != nil {
		return nil, err
	}

	for _, r := range rows {
		key := codeKey(r.CartID.String, r.ProductID)
		codes[key] = append(codes[key], r.Content)
	}
	return codes, nil
}

// digitalFile reads one digital file and the product it belongs to.
func (q *CustomerQueries) digitalFile(ctx context.Context, fileID string) (*models.File, string, error) {
	r, err := pick(q.DB, q.DB,
		func(g *pggen.Queries) (pggen.DigitalFileByIDRow, error) { return g.DigitalFileByID(ctx, fileID) },
		func(g *sqlitegen.Queries) (pggen.DigitalFileByIDRow, error) {
			r, err := g.DigitalFileByID(ctx, fileID)
			return pggen.DigitalFileByIDRow(r), err
		})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, "", errors.ErrProductNotFound
		}
		return nil, "", err
	}

	file := &models.File{ID: r.ID, Name: r.Name, Ext: r.Ext, OrigName: r.OrigName}
	if file.OrigName == "" {
		file.OrigName = file.Name + "." + file.Ext
	}
	return file, r.ProductID, nil
}
