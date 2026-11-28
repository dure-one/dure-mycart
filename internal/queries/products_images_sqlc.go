//go:build sqlc

package queries

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/dure-one/dure-mycart/internal/models"
	pggen "github.com/dure-one/dure-mycart/internal/queries/sqlcgen/postgres"
	sqlitegen "github.com/dure-one/dure-mycart/internal/queries/sqlcgen/sqlite"
	"github.com/dure-one/dure-mycart/pkg/security"
)

// ProductImages returns the images of a product.
func (q *ProductQueries) ProductImages(ctx context.Context, id string) (*[]models.File, error) {
	rows, err := pick(q.DB, q.DB,
		func(g *pggen.Queries) ([]pggen.ProductImageRowsRow, error) { return g.ProductImageRows(ctx, id) },
		func(g *sqlitegen.Queries) ([]pggen.ProductImageRowsRow, error) {
			r, err := g.ProductImageRows(ctx, id)
			return convertAll(r, err, func(x sqlitegen.ProductImageRowsRow) pggen.ProductImageRowsRow { return pggen.ProductImageRowsRow(x) })
		})
	if err != nil {
		return nil, fmt.Errorf("fetching product images: %w", err)
	}

	result := make([]models.File, len(rows))
	for i, r := range rows {
		result[i] = models.File{ID: r.ID, Name: r.Name, Ext: r.Ext}
	}
	return &result, nil
}

// AddImage attaches an image to a product by inserting a new record in the product_image table.
func (q *ProductQueries) AddImage(ctx context.Context, productID, fileUUID, fileExt, origName string) (*models.File, error) {
	file := &models.File{
		ID:   security.RandomString(),
		Name: fileUUID,
		Ext:  fileExt,
	}

	params := pggen.InsertProductImageParams{ID: file.ID, ProductID: productID, Name: file.Name, Ext: file.Ext, OrigName: origName}
	if err := run(q.DB, q.DB,
		func(g *pggen.Queries) error { return g.InsertProductImage(ctx, params) },
		func(g *sqlitegen.Queries) error {
			return g.InsertProductImage(ctx, sqlitegen.InsertProductImageParams(params))
		}); err != nil {
		return nil, err
	}
	return file, nil
}

// DeleteImage handles the deletion of a product image.
func (q *ProductQueries) DeleteImage(ctx context.Context, productID, imageID string) error {
	info, err := q.deleteImageRecord(ctx, productID, imageID)
	if err != nil {
		return fmt.Errorf("deleting image record: %w", err)
	}

	if err := q.deleteImageFiles(info.Name, info.Ext); err != nil {
		return fmt.Errorf("deleting image files: %w", err)
	}

	return nil
}

func (q *ProductQueries) deleteImageRecord(ctx context.Context, productID, imageID string) (pggen.ImageNameExtRow, error) {
	tx, err := q.DB.BeginTx(ctx, nil)
	if err != nil {
		return pggen.ImageNameExtRow{}, err
	}
	defer func() { _ = tx.Rollback() }()

	info, err := pick(q.DB, tx,
		func(g *pggen.Queries) (pggen.ImageNameExtRow, error) { return g.ImageNameExt(ctx, imageID) },
		func(g *sqlitegen.Queries) (pggen.ImageNameExtRow, error) {
			r, err := g.ImageNameExt(ctx, imageID)
			return pggen.ImageNameExtRow(r), err
		})
	if err != nil {
		return pggen.ImageNameExtRow{}, err
	}

	del := pggen.DeleteProductImageParams{ID: imageID, ProductID: productID}
	if err := run(q.DB, tx,
		func(g *pggen.Queries) error { return g.DeleteProductImage(ctx, del) },
		func(g *sqlitegen.Queries) error {
			return g.DeleteProductImage(ctx, sqlitegen.DeleteProductImageParams(del))
		}); err != nil {
		return pggen.ImageNameExtRow{}, err
	}

	if err := tx.Commit(); err != nil {
		return pggen.ImageNameExtRow{}, err
	}
	return info, nil
}

func (q *ProductQueries) deleteImageFiles(name, ext string) error {
	filePaths := []string{
		fmt.Sprintf("./lc_uploads/%s.%s", name, ext),
		fmt.Sprintf("./lc_uploads/%s_sm.%s", name, ext),
	}

	var removeErrors []string
	for _, filePath := range filePaths {
		if err := os.Remove(filePath); err != nil && !os.IsNotExist(err) {
			removeErrors = append(removeErrors,
				fmt.Sprintf("failed to remove %s: %v", filePath, err))
		}
	}

	if len(removeErrors) > 0 {
		return fmt.Errorf("file deletion errors: %s",
			strings.Join(removeErrors, "; "))
	}

	return nil
}
