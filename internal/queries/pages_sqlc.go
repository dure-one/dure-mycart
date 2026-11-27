//go:build sqlc

package queries

import (
	"context"
	"database/sql"
	"encoding/json"

	"github.com/shurco/mycart/internal/database"
	"github.com/shurco/mycart/internal/models"
	pggen "github.com/shurco/mycart/internal/queries/sqlcgen/postgres"
	sqlitegen "github.com/shurco/mycart/internal/queries/sqlcgen/sqlite"
	"github.com/shurco/mycart/pkg/errors"
	"github.com/shurco/mycart/pkg/security"
)

// PageQueries is a struct that holds a dialect-aware database handle.
type PageQueries struct {
	DB *database.Conn
}

// IsPage checks if a page with the given slug exists in the database.
func (q *PageQueries) IsPage(ctx context.Context, slug string) bool {
	exists, err := pick(q.DB, q.DB,
		func(g *pggen.Queries) (bool, error) { return g.PageExists(ctx, slug) },
		func(g *sqlitegen.Queries) (bool, error) { return g.PageExists(ctx, slug) })
	return err == nil && exists
}

// ListPages retrieves a list of pages from the database.
// It filters out private pages unless `private` is set to true.
// idList is accepted for API compatibility and, as in the plain build, unused.
func (q *PageQueries) ListPages(ctx context.Context, private bool, limit, offset int, idList ...string) ([]models.Page, int, error) {
	lim, off := pageBounds(limit, offset)

	rows, err := pick(q.DB, q.DB,
		func(g *pggen.Queries) ([]pggen.ListPagesRow, error) {
			return g.ListPages(ctx, pggen.ListPagesParams{Private: private, Lim: pgLimit(lim), Off: off})
		},
		func(g *sqlitegen.Queries) ([]pggen.ListPagesRow, error) {
			r, err := g.ListPages(ctx, sqlitegen.ListPagesParams{Private: private, Lim: lim, Off: off})
			return convertAll(r, err, func(x sqlitegen.ListPagesRow) pggen.ListPagesRow { return pggen.ListPagesRow(x) })
		})
	if err != nil {
		return nil, 0, err
	}

	pages := make([]models.Page, 0, len(rows))
	for _, r := range rows {
		page := models.Page{Name: r.Name, Slug: r.Slug, Position: r.Position, Active: r.Active}
		page.ID, page.Created, page.Updated = r.ID, r.Created, r.Updated
		if err := unmarshalSeo(r.Seo, &page); err != nil {
			return nil, 0, err
		}
		pages = append(pages, page)
	}

	total, err := pick(q.DB, q.DB,
		func(g *pggen.Queries) (int64, error) { return g.CountPages(ctx, private) },
		func(g *sqlitegen.Queries) (int64, error) { return g.CountPages(ctx, private) })
	if err != nil {
		return nil, 0, err
	}

	return pages, int(total), nil
}

// Page retrieves a single published page from the database based on its slug.
//
// Publication is part of the lookup, exactly as ListPages decides it for the
// list: a draft answered here would be published at its slug from the moment
// it was created.
func (q *PageQueries) Page(ctx context.Context, slug string) (*models.Page, error) {
	r, err := pick(q.DB, q.DB,
		func(g *pggen.Queries) (pggen.PublishedPageBySlugRow, error) { return g.PublishedPageBySlug(ctx, slug) },
		func(g *sqlitegen.Queries) (pggen.PublishedPageBySlugRow, error) {
			r, err := g.PublishedPageBySlug(ctx, slug)
			return pggen.PublishedPageBySlugRow(r), err
		})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, errors.ErrPageNotFound
		}
		return nil, err
	}

	page := models.Page{Name: r.Name, Slug: slug, Content: &r.Content.String, Active: r.Active}
	page.ID = r.ID
	if err := unmarshalSeo(r.Seo, &page); err != nil {
		return nil, err
	}
	return &page, nil
}

// PageByID retrieves a single page from the database based on its ID.
func (q *PageQueries) PageByID(ctx context.Context, id string) (*models.Page, error) {
	r, err := pick(q.DB, q.DB,
		func(g *pggen.Queries) (pggen.PageByIDRow, error) { return g.PageByID(ctx, id) },
		func(g *sqlitegen.Queries) (pggen.PageByIDRow, error) {
			r, err := g.PageByID(ctx, id)
			return pggen.PageByIDRow(r), err
		})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, errors.ErrPageNotFound
		}
		return nil, err
	}

	page := models.Page{Name: r.Name, Slug: r.Slug, Position: r.Position, Active: r.Active}
	page.ID, page.Created, page.Updated = r.ID, r.Created, r.Updated
	if r.Content.Valid {
		page.Content = &r.Content.String
	}
	if err := unmarshalSeo(r.Seo, &page); err != nil {
		return nil, err
	}
	return &page, nil
}

// AddPage inserts a new page into the database and returns the created page or an error.
func (q *PageQueries) AddPage(ctx context.Context, page *models.Page) (*models.Page, error) {
	page.ID = security.RandomString()
	page.Active = false

	created, err := pick(q.DB, q.DB,
		func(g *pggen.Queries) (int64, error) {
			return g.InsertPage(ctx, pggen.InsertPageParams{ID: page.ID, Name: page.Name, Slug: page.Slug, Position: page.Position})
		},
		func(g *sqlitegen.Queries) (int64, error) {
			return g.InsertPage(ctx, sqlitegen.InsertPageParams{ID: page.ID, Name: page.Name, Slug: page.Slug, Position: page.Position})
		})
	if err != nil {
		return nil, err
	}
	page.Created = created
	return page, nil
}

// UpdatePage updates the details of a page in the database.
// If some fields are not provided (empty strings or nil), they are loaded from the database first.
func (q *PageQueries) UpdatePage(ctx context.Context, page *models.Page) error {
	currentPage, err := q.PageByID(ctx, page.ID)
	if err != nil {
		return err
	}

	name := page.Name
	if name == "" {
		name = currentPage.Name
	}
	slug := page.Slug
	if slug == "" {
		slug = currentPage.Slug
	}
	position := page.Position
	if position == "" {
		position = currentPage.Position
	}

	var content sql.NullString
	switch {
	case page.Content != nil:
		content = sql.NullString{String: *page.Content, Valid: true}
	case currentPage.Content != nil:
		content = sql.NullString{String: *currentPage.Content, Valid: true}
	}

	seoData := page.Seo
	if seoData == nil {
		seoData = currentPage.Seo
	}
	seo, err := json.Marshal(seoData)
	if err != nil {
		return err
	}

	return run(q.DB, q.DB,
		func(g *pggen.Queries) error {
			return g.UpdatePage(ctx, pggen.UpdatePageParams{Name: name, Slug: slug, Position: position, Content: content, Seo: string(seo), ID: page.ID})
		},
		func(g *sqlitegen.Queries) error {
			return g.UpdatePage(ctx, sqlitegen.UpdatePageParams{Name: name, Slug: slug, Position: position, Content: content, Seo: string(seo), ID: page.ID})
		})
}

// DeletePage deletes a page from the database.
func (q *PageQueries) DeletePage(ctx context.Context, id string) error {
	return run(q.DB, q.DB,
		func(g *pggen.Queries) error { return g.DeletePage(ctx, id) },
		func(g *sqlitegen.Queries) error { return g.DeletePage(ctx, id) })
}

// UpdatePageContent updates the content of an existing page in the database.
func (q *PageQueries) UpdatePageContent(ctx context.Context, page *models.Page) error {
	var content sql.NullString
	if page.Content != nil {
		content = sql.NullString{String: *page.Content, Valid: true}
	}
	return run(q.DB, q.DB,
		func(g *pggen.Queries) error {
			return g.UpdatePageContent(ctx, pggen.UpdatePageContentParams{Content: content, ID: page.ID})
		},
		func(g *sqlitegen.Queries) error {
			return g.UpdatePageContent(ctx, sqlitegen.UpdatePageContentParams{Content: content, ID: page.ID})
		})
}

// UpdatePageActive toggles the active status of a page with the given ID.
func (q *PageQueries) UpdatePageActive(ctx context.Context, id string) error {
	return run(q.DB, q.DB,
		func(g *pggen.Queries) error { return g.TogglePageActive(ctx, id) },
		func(g *sqlitegen.Queries) error { return g.TogglePageActive(ctx, id) })
}

// unmarshalSeo decodes the seo column into page.Seo. The column is NOT NULL
// with a '{}' default, so an empty string only comes from a hand-edited row.
func unmarshalSeo(seo string, page *models.Page) error {
	if seo == "" {
		return nil
	}
	return json.Unmarshal([]byte(seo), &page.Seo)
}
