//go:build sqlc

package queries

import (
	"context"

	"github.com/dure-one/dure-mycart/internal/database"
	pggen "github.com/dure-one/dure-mycart/internal/queries/sqlcgen/postgres"
	sqlitegen "github.com/dure-one/dure-mycart/internal/queries/sqlcgen/sqlite"
)

// SEOQueries holds a dialect-aware database handle for SEO-related queries.
type SEOQueries struct {
	DB *database.Conn
}

// SEOEntry represents a product or page for crawler-facing output.
type SEOEntry struct {
	Name        string
	Slug        string
	Brief       string
	Description string
	Updated     int64
}

// ActiveProductsForSEO lists active products for sitemaps and crawlers.
func (q *SEOQueries) ActiveProductsForSEO(ctx context.Context, limit int) ([]SEOEntry, error) {
	return pick(q.DB, q.DB,
		func(g *pggen.Queries) ([]SEOEntry, error) {
			rows, err := g.ActiveProductsForSEO(ctx, int32(limit))
			if err != nil {
				return nil, err
			}
			entries := make([]SEOEntry, 0, len(rows))
			for _, row := range rows {
				entries = append(entries, SEOEntry{Name: row.Name, Slug: row.Slug, Brief: row.Brief, Description: row.Desc, Updated: row.Updated})
			}
			return entries, nil
		},
		func(g *sqlitegen.Queries) ([]SEOEntry, error) {
			rows, err := g.ActiveProductsForSEO(ctx, int64(limit))
			if err != nil {
				return nil, err
			}
			entries := make([]SEOEntry, 0, len(rows))
			for _, row := range rows {
				entries = append(entries, SEOEntry{Name: row.Name, Slug: row.Slug, Brief: row.Brief, Description: row.Desc, Updated: row.Updated})
			}
			return entries, nil
		})
}

// ActivePagesForSEO lists active pages for sitemaps and crawlers.
func (q *SEOQueries) ActivePagesForSEO(ctx context.Context, limit int) ([]SEOEntry, error) {
	return pick(q.DB, q.DB,
		func(g *pggen.Queries) ([]SEOEntry, error) {
			rows, err := g.ActivePagesForSEO(ctx, int32(limit))
			if err != nil {
				return nil, err
			}
			entries := make([]SEOEntry, 0, len(rows))
			for _, row := range rows {
				entries = append(entries, SEOEntry{Name: row.Name, Slug: row.Slug, Updated: row.Updated})
			}
			return entries, nil
		},
		func(g *sqlitegen.Queries) ([]SEOEntry, error) {
			rows, err := g.ActivePagesForSEO(ctx, int64(limit))
			if err != nil {
				return nil, err
			}
			entries := make([]SEOEntry, 0, len(rows))
			for _, row := range rows {
				entries = append(entries, SEOEntry{Name: row.Name, Slug: row.Slug, Updated: row.Updated})
			}
			return entries, nil
		})
}
