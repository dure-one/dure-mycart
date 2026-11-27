//go:build sqlc

package queries

import (
	"context"
	"encoding/json"

	"github.com/shurco/mycart/internal/models"
	pggen "github.com/shurco/mycart/internal/queries/sqlcgen/postgres"
	sqlitegen "github.com/shurco/mycart/internal/queries/sqlcgen/sqlite"
)

// ListProducts retrieves a list of products from the database.
// If cartID is provided, it will also include digital products that were purchased in that cart.
func (q *ProductQueries) ListProducts(ctx context.Context, private bool, limit, offset int, cartID string, idList ...models.CartProduct) (*models.Products, error) {
	currency, err := NewBase(q.DB).GetSettingByKey(ctx, "currency")
	if err != nil {
		return nil, err
	}

	products := &models.Products{
		Currency: currency["currency"].Value.(string),
	}

	ids := make([]string, 0, len(idList))
	for _, item := range idList {
		ids = append(ids, item.ProductID)
	}
	allIDs := len(ids) == 0
	idsJSON, err := json.Marshal(ids)
	if err != nil {
		return nil, err
	}

	lim, off := pageBounds(limit, offset)
	params := pggen.ListProductsParams{
		Private: private, CartID: cartID, AllIds: allIDs, Ids: idsJSON, Lim: pgLimit(lim), Off: off,
	}
	rows, err := pick(q.DB, q.DB,
		func(g *pggen.Queries) ([]pggen.ListProductsRow, error) { return g.ListProducts(ctx, params) },
		func(g *sqlitegen.Queries) ([]pggen.ListProductsRow, error) {
			r, err := g.ListProducts(ctx, sqlitegen.ListProductsParams{
				Private: private, CartID: cartID, AllIds: allIDs, Ids: string(idsJSON), Lim: lim, Off: off,
			})
			return convertAll(r, err, func(x sqlitegen.ListProductsRow) pggen.ListProductsRow { return pggen.ListProductsRow(x) })
		})
	if err != nil {
		return nil, err
	}

	for _, r := range rows {
		product, err := productFromListRow(r, private)
		if err != nil {
			return nil, err
		}
		products.Products = append(products.Products, product)
	}

	total, err := pick(q.DB, q.DB,
		func(g *pggen.Queries) (int64, error) {
			return g.CountListedProducts(ctx, pggen.CountListedProductsParams{
				Private: private, CartID: cartID, AllIds: allIDs, Ids: idsJSON,
			})
		},
		func(g *sqlitegen.Queries) (int64, error) {
			return g.CountListedProducts(ctx, sqlitegen.CountListedProductsParams{
				Private: private, CartID: cartID, AllIds: allIDs, Ids: string(idsJSON),
			})
		})
	if err != nil {
		return nil, err
	}
	products.Total = int(total)

	return products, nil
}

// productFromListRow maps one row of the product listing. An empty image or
// variants column is the aggregate's NULL (see product.sql).
func productFromListRow(r pggen.ListProductsRow, private bool) (models.Product, error) {
	product := models.Product{
		Name:        r.Name,
		Brief:       r.Brief,
		Slug:        r.Slug,
		Amount:      int(r.Amount),
		Quantity:    int(r.Quantity),
		HasVariants: r.HasVariants,
		Active:      r.Active,
	}
	product.ID, product.Created = r.ID, r.Created

	if r.Image != "" {
		if err := decodeFiles(r.Image, &product.Images); err != nil {
			return models.Product{}, err
		}
	}
	if r.Variants != "" {
		if err := decodeVariants(r.Variants, &product.Variants); err != nil {
			return models.Product{}, err
		}
	}

	product.Digital.Type = r.Digital.String
	if private && r.Digital.Valid {
		product.Digital.Filled = r.DigitalFilled
	}

	return product, nil
}
