//go:build sqlc

package queries

import (
	"context"
	"database/sql"
	stderrors "errors"
	"fmt"

	pggen "github.com/shurco/mycart/internal/queries/sqlcgen/postgres"
	sqlitegen "github.com/shurco/mycart/internal/queries/sqlcgen/sqlite"
)

// ImagePositionUpdate contains data for updating a product image position.
type ImagePositionUpdate struct {
	ImageID  string
	Position int
}

// UpdateProductImagePositions updates positions for multiple product images in a single transaction.
// All updates succeed or all fail (atomic operation).
func (q *ProductQueries) UpdateProductImagePositions(ctx context.Context, updates []ImagePositionUpdate) error {
	if len(updates) == 0 {
		return nil
	}

	tx, err := q.DB.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	for _, update := range updates {
		params := pggen.SetImagePositionParams{Position: int64(update.Position), ID: update.ImageID}
		rowsAffected, err := pick(q.DB, tx,
			func(g *pggen.Queries) (int64, error) { return g.SetImagePosition(ctx, params) },
			func(g *sqlitegen.Queries) (int64, error) {
				return g.SetImagePosition(ctx, sqlitegen.SetImagePositionParams(params))
			})
		if err != nil {
			return fmt.Errorf("update image position: %w", err)
		}
		if rowsAffected == 0 {
			return fmt.Errorf("image not found: %s", update.ImageID)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}
	return nil
}

// SetRepresentativeImage sets one image as representative and unmarks all others for the same product.
// Performs both operations atomically in a single transaction.
func (q *ProductQueries) SetRepresentativeImage(ctx context.Context, productID string, imageID string) error {
	tx, err := q.DB.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	// First, verify the image exists and belongs to this product
	existingProductID, err := pick(q.DB, tx,
		func(g *pggen.Queries) (string, error) { return g.ImageProduct(ctx, imageID) },
		func(g *sqlitegen.Queries) (string, error) { return g.ImageProduct(ctx, imageID) })
	if err != nil {
		if stderrors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("image not found: %s", imageID)
		}
		return fmt.Errorf("query image: %w", err)
	}

	if existingProductID != productID {
		return fmt.Errorf("image does not belong to product: image_id=%s, product_id=%s", imageID, productID)
	}

	if err := run(q.DB, tx,
		func(g *pggen.Queries) error { return g.ClearRepresentative(ctx, productID) },
		func(g *sqlitegen.Queries) error { return g.ClearRepresentative(ctx, productID) }); err != nil {
		return fmt.Errorf("unmark images: %w", err)
	}

	if err := run(q.DB, tx,
		func(g *pggen.Queries) error { return g.MarkRepresentative(ctx, imageID) },
		func(g *sqlitegen.Queries) error { return g.MarkRepresentative(ctx, imageID) }); err != nil {
		return fmt.Errorf("mark representative image: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}
	return nil
}
