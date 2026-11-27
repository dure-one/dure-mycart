//go:build sqlc

package queries

import (
	"context"
	"database/sql"
	stderrors "errors"
	"fmt"
	"os"

	"github.com/shurco/mycart/internal/digitalfiles"
	"github.com/shurco/mycart/internal/models"
	pggen "github.com/shurco/mycart/internal/queries/sqlcgen/postgres"
	sqlitegen "github.com/shurco/mycart/internal/queries/sqlcgen/sqlite"
	"github.com/shurco/mycart/pkg/errors"
	"github.com/shurco/mycart/pkg/security"
)

// ProductDigital retrieves the digital content associated with a given product ID.
func (q *ProductQueries) ProductDigital(ctx context.Context, productID string) (*models.Digital, error) {
	rows, err := pick(q.DB, q.DB,
		func(g *pggen.Queries) ([]pggen.ProductDigitalRowsRow, error) {
			return g.ProductDigitalRows(ctx, productID)
		},
		func(g *sqlitegen.Queries) ([]pggen.ProductDigitalRowsRow, error) {
			r, err := g.ProductDigitalRows(ctx, productID)
			return convertAll(r, err, func(x sqlitegen.ProductDigitalRowsRow) pggen.ProductDigitalRowsRow {
				return pggen.ProductDigitalRowsRow(x)
			})
		})
	if err != nil {
		return nil, err
	}

	digital := &models.Digital{}
	for _, r := range rows {
		if digital.Type == "" {
			digital.Type = r.Digital.String
		}
		if r.FileID.Valid {
			digital.Files = append(digital.Files, models.File{ID: r.FileID.String, Name: r.FileName.String, Ext: r.FileExt.String})
		}
		if r.DataID.Valid {
			digital.Data = append(digital.Data, models.Data{ID: r.DataID.String, Content: r.DataContent.String, CartID: r.DataCartID.String})
		}
	}
	return digital, nil
}

// DigitalFile returns a single digital file record for a product.
// Used by the authenticated admin download endpoint.
func (q *ProductQueries) DigitalFile(ctx context.Context, productID, fileID string) (*models.File, error) {
	params := pggen.DigitalFileOfProductParams{ID: fileID, ProductID: productID}
	r, err := pick(q.DB, q.DB,
		func(g *pggen.Queries) (pggen.DigitalFileOfProductRow, error) {
			return g.DigitalFileOfProduct(ctx, params)
		},
		func(g *sqlitegen.Queries) (pggen.DigitalFileOfProductRow, error) {
			r, err := g.DigitalFileOfProduct(ctx, sqlitegen.DigitalFileOfProductParams(params))
			return pggen.DigitalFileOfProductRow(r), err
		})
	if err != nil {
		if stderrors.Is(err, sql.ErrNoRows) {
			return nil, errors.ErrProductNotFound
		}
		return nil, err
	}
	return &models.File{ID: r.ID, Name: r.Name, Ext: r.Ext, OrigName: r.OrigName}, nil
}

// AddDigitalFile associates a digital file with a product in the database.
func (q *ProductQueries) AddDigitalFile(ctx context.Context, productID, fileUUID, fileExt, origName string) (*models.File, error) {
	file := &models.File{
		ID:   security.RandomString(),
		Name: fileUUID,
		Ext:  fileExt,
	}

	params := pggen.InsertDigitalFileParams{ID: file.ID, ProductID: productID, Name: file.Name, Ext: file.Ext, OrigName: origName}
	if err := run(q.DB, q.DB,
		func(g *pggen.Queries) error { return g.InsertDigitalFile(ctx, params) },
		func(g *sqlitegen.Queries) error {
			return g.InsertDigitalFile(ctx, sqlitegen.InsertDigitalFileParams(params))
		}); err != nil {
		return nil, err
	}
	return file, nil
}

// AddDigitalData adds a new digital data record associated with a product.
func (q *ProductQueries) AddDigitalData(ctx context.Context, productID, content string) (*models.Data, error) {
	data := &models.Data{
		ID:      security.RandomString(),
		Content: content,
	}

	params := pggen.InsertDigitalDataParams{ID: data.ID, ProductID: productID, Content: data.Content}
	if err := run(q.DB, q.DB,
		func(g *pggen.Queries) error { return g.InsertDigitalData(ctx, params) },
		func(g *sqlitegen.Queries) error {
			return g.InsertDigitalData(ctx, sqlitegen.InsertDigitalDataParams(params))
		}); err != nil {
		return nil, err
	}
	return data, nil
}

// UpdateDigital updates the content of a digital data record in the database.
func (q *ProductQueries) UpdateDigital(ctx context.Context, digital *models.Data) error {
	params := pggen.UpdateDigitalContentParams{Content: digital.Content, ID: digital.ID}
	return run(q.DB, q.DB,
		func(g *pggen.Queries) error { return g.UpdateDigitalContent(ctx, params) },
		func(g *sqlitegen.Queries) error {
			return g.UpdateDigitalContent(ctx, sqlitegen.UpdateDigitalContentParams(params))
		})
}

// DeleteDigital removes a digital file (and its stored file) or a digital data
// record of a product, according to the product's delivery type.
func (q *ProductQueries) DeleteDigital(ctx context.Context, productID, digitalID string) error {
	params := pggen.DigitalKindAndFileParams{DigitalID: digitalID, ProductID: productID}
	r, err := pick(q.DB, q.DB,
		func(g *pggen.Queries) (pggen.DigitalKindAndFileRow, error) { return g.DigitalKindAndFile(ctx, params) },
		func(g *sqlitegen.Queries) (pggen.DigitalKindAndFileRow, error) {
			r, err := g.DigitalKindAndFile(ctx, sqlitegen.DigitalKindAndFileParams(params))
			return pggen.DigitalKindAndFileRow(r), err
		})
	if err != nil {
		return err
	}
	// The plain build scans the type into a string, which fails on NULL.
	if !r.Digital.Valid {
		return fmt.Errorf("product %s has no digital type", productID)
	}

	switch r.Digital.String {
	case models.DigitalFile:
		del := pggen.DeleteDigitalFileParams{ID: digitalID, ProductID: productID}
		if err := run(q.DB, q.DB,
			func(g *pggen.Queries) error { return g.DeleteDigitalFile(ctx, del) },
			func(g *sqlitegen.Queries) error {
				return g.DeleteDigitalFile(ctx, sqlitegen.DeleteDigitalFileParams(del))
			}); err != nil {
			return fmt.Errorf("deleting from digital_file: %w", err)
		}

		filePath := digitalfiles.Path(r.FileName.String, r.FileExt.String)
		if err := os.Remove(filePath); err != nil {
			return fmt.Errorf("failed to remove file %s: %w", filePath, err)
		}

	case models.DigitalData:
		del := pggen.DeleteDigitalDataParams{ID: digitalID, ProductID: productID}
		if err := run(q.DB, q.DB,
			func(g *pggen.Queries) error { return g.DeleteDigitalData(ctx, del) },
			func(g *sqlitegen.Queries) error {
				return g.DeleteDigitalData(ctx, sqlitegen.DeleteDigitalDataParams(del))
			}); err != nil {
			return fmt.Errorf("deleting from digital_data: %w", err)
		}
	}

	return nil
}
