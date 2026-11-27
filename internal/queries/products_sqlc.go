//go:build sqlc

package queries

import (
	"context"
	"fmt"

	"github.com/gosimple/slug"

	"github.com/shurco/mycart/internal/database"
	pggen "github.com/shurco/mycart/internal/queries/sqlcgen/postgres"
	sqlitegen "github.com/shurco/mycart/internal/queries/sqlcgen/sqlite"
	"github.com/shurco/mycart/pkg/errors"
)

// ProductQueries is a struct that holds a dialect-aware database handle.
type ProductQueries struct {
	DB *database.Conn
}

// DeleteProduct removes a product from the database based on its ID.
func (q *ProductQueries) DeleteProduct(ctx context.Context, id string) error {
	// Guard: a hard delete cascades into digital_file/digital_data, which
	// would revoke downloads already purchased by customers (rows assigned
	// to a paid cart). Refuse such deletes; admins should deactivate the
	// product instead.
	sold, err := pick(q.DB, q.DB,
		func(g *pggen.Queries) (int64, error) { return g.CountSoldDigitalData(ctx, id) },
		func(g *sqlitegen.Queries) (int64, error) { return g.CountSoldDigitalData(ctx, id) })
	if err != nil {
		return err
	}
	if sold > 0 {
		return errors.ErrProductSold
	}

	return run(q.DB, q.DB,
		func(g *pggen.Queries) error { return g.DeleteProductRow(ctx, id) },
		func(g *sqlitegen.Queries) error { return g.DeleteProductRow(ctx, id) })
}

// IsProduct checks if a product with the given slug exists and is active.
func (q *ProductQueries) IsProduct(ctx context.Context, slug string) bool {
	exists, err := pick(q.DB, q.DB,
		func(g *pggen.Queries) (bool, error) { return g.ActiveProductExists(ctx, slug) },
		func(g *sqlitegen.Queries) (bool, error) { return g.ActiveProductExists(ctx, slug) })
	return err == nil && exists
}

// UpdateActive toggles the 'active' status of a product and updates its 'updated' timestamp.
func (q *ProductQueries) UpdateActive(ctx context.Context, id string) error {
	return run(q.DB, q.DB,
		func(g *pggen.Queries) error { return g.ToggleProductActive(ctx, id) },
		func(g *sqlitegen.Queries) error { return g.ToggleProductActive(ctx, id) })
}

// GenerateUniqueSlug generates a unique URL-friendly slug from product name:
// the slug of name, or the first free "<slug>-2", "<slug>-3", … among the
// products other than excludeProductID.
//
// The plain build delegates to pkg/slugify, whose uniqueness check is raw SQL;
// this is the same loop over the SlugTaken query. TestSQLCPortedFilesUnchanged
// flags pkg/slugify/slug.go changing upstream.
func (q *ProductQueries) GenerateUniqueSlug(ctx context.Context, name string, excludeProductID string) (string, error) {
	base := slug.Make(name)
	if base == "" {
		return "", fmt.Errorf("cannot generate slug from empty name")
	}

	final := base
	for counter := 2; ; counter++ {
		params := pggen.SlugTakenParams{Slug: final, ExcludeID: excludeProductID}
		taken, err := pick(q.DB, q.DB,
			func(g *pggen.Queries) (int64, error) { return g.SlugTaken(ctx, params) },
			func(g *sqlitegen.Queries) (int64, error) { return g.SlugTaken(ctx, sqlitegen.SlugTakenParams(params)) })
		if err != nil {
			return "", fmt.Errorf("checking slug existence: querying slug: %w", err)
		}
		if taken == 0 {
			return final, nil
		}

		// Safety limit, as in pkg/slugify: "<slug>-999" is the last one tried.
		if counter >= 1000 {
			return "", fmt.Errorf("failed to generate unique slug after 1000 attempts")
		}
		final = fmt.Sprintf("%s-%d", base, counter)
	}
}
