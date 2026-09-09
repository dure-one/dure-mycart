package queries

import (
	"context"
	"database/sql"
	"encoding/json"

	"github.com/shurco/mycart/internal/store/db/postgres"
	"github.com/shurco/mycart/internal/store/db/sqlite"
	"github.com/shurco/mycart/internal/models"
	"github.com/shurco/mycart/pkg/errors"
	"github.com/shurco/mycart/pkg/security"
)

// toModelPage converts sqlc Page rows to models.Page
func toModelPage(p interface{}) *models.Page {
	var page models.Page

	switch v := p.(type) {
	case sqlite.GetPageByIDRow:
		page.ID = v.ID
		page.Name = v.Name
		page.Slug = v.Slug
		page.Position = v.Position
		page.Active = v.Active
		if v.Content.Valid {
			page.Content = &v.Content.String
		}
		if v.Created.Valid {
			page.Created = v.Created.Time.Unix()
		}
		if v.Updated.Valid {
			page.Updated = v.Updated.Time.Unix()
		}
	case postgres.GetPageByIDRow:
		page.ID = v.ID
		page.Name = v.Name
		page.Slug = v.Slug
		page.Position = v.Position
		page.Active = v.Active
		if v.Content.Valid {
			page.Content = &v.Content.String
		}
		if v.Created.Valid {
			page.Created = v.Created.Time.Unix()
		}
		if v.Updated.Valid {
			page.Updated = v.Updated.Time.Unix()
		}
	case sqlite.GetPageBySlugRow:
		page.ID = v.ID
		page.Name = v.Name
		page.Slug = v.Slug
		page.Position = v.Position
		page.Active = v.Active
		if v.Content.Valid {
			page.Content = &v.Content.String
		}
		if v.Created.Valid {
			page.Created = v.Created.Time.Unix()
		}
		if v.Updated.Valid {
			page.Updated = v.Updated.Time.Unix()
		}
	case postgres.GetPageBySlugRow:
		page.ID = v.ID
		page.Name = v.Name
		page.Slug = v.Slug
		page.Position = v.Position
		page.Active = v.Active
		if v.Content.Valid {
			page.Content = &v.Content.String
		}
		if v.Created.Valid {
			page.Created = v.Created.Time.Unix()
		}
		if v.Updated.Valid {
			page.Updated = v.Updated.Time.Unix()
		}
	case sqlite.CreatePageRow:
		page.ID = v.ID
		page.Name = v.Name
		page.Slug = v.Slug
		page.Position = v.Position
		page.Active = v.Active
		if v.Content.Valid {
			page.Content = &v.Content.String
		}
		if v.Created.Valid {
			page.Created = v.Created.Time.Unix()
		}
		if v.Updated.Valid {
			page.Updated = v.Updated.Time.Unix()
		}
	case postgres.CreatePageRow:
		page.ID = v.ID
		page.Name = v.Name
		page.Slug = v.Slug
		page.Position = v.Position
		page.Active = v.Active
		if v.Content.Valid {
			page.Content = &v.Content.String
		}
		if v.Created.Valid {
			page.Created = v.Created.Time.Unix()
		}
		if v.Updated.Valid {
			page.Updated = v.Updated.Time.Unix()
		}
	default:
		return nil
	}

	return &page
}

// loadPageSeo loads the seo field for a page from the database
func (q *PageQueries) loadPageSeo(ctx context.Context, page *models.Page) error {
	var seo sql.NullString
	var query string
	if DBType() == "postgres" || DBType() == "postgresql" {
		query = `SELECT seo FROM page WHERE id = $1`
	} else {
		query = `SELECT seo FROM page WHERE id = ?`
	}
	if err := q.DB.QueryRowContext(ctx, query, page.ID).Scan(&seo); err != nil {
		return err
	}

	if seo.Valid && seo.String != "" {
		var seoData models.Seo
		if err := json.Unmarshal([]byte(seo.String), &seoData); err != nil {
			return err
		}
		page.Seo = &seoData
	}

	return nil
}

// PageQueries is a struct that embeds a pointer to an sql.DB.
// This allows for direct access to database methods on the PageQueries struct,
// effectively extending it with all the methods of *sql.DB.
type PageQueries struct {
	*sql.DB
}

// IsPage checks if a page with the given slug exists in the database using sqlc.
func (q *PageQueries) IsPage(ctx context.Context, slug string) bool {
	queries := getSQLCQueries()

	var result interface{}
	var err error

	if DBType() == "postgres" {
		pgQueries := queries.(*postgres.Queries)
		result, err = pgQueries.PageExists(ctx, slug)
	} else {
		sqliteQueries := queries.(*sqlite.Queries)
		result, err = sqliteQueries.PageExists(ctx, slug)
	}

	if err != nil {
		return false
	}

	// Both postgres and sqlite return bool for EXISTS
	exists, ok := result.(bool)
	return ok && exists
}

// ListPages retrieves a list of pages from the database using sqlc.
// It filters out private pages unless `private` is set to true.
func (q *PageQueries) ListPages(ctx context.Context, private bool, limit, offset int, idList ...string) ([]models.Page, int, error) {
	pages := []models.Page{}
	queries := getSQLCQueries()

	// For filtering by active status, we need to use direct SQL
	// because sqlc doesn't support conditional WHERE clauses
	if !private {
		// Use ListPagesByPosition query but for active pages only
		// Actually, we need a custom query here since sqlc query is too specific
		query := `SELECT id, name, slug, content, position, active, created, updated FROM page WHERE active = 1 ORDER BY created DESC`

		var params []any
		if limit > 0 {
			if DBType() == "postgres" || DBType() == "postgresql" {
				query += " LIMIT $1"
				params = append(params, limit)
				if offset > 0 {
					query += " OFFSET $2"
					params = append(params, offset)
				}
			} else {
				query += " LIMIT ?"
				params = append(params, limit)
				if offset > 0 {
					query += " OFFSET ?"
					params = append(params, offset)
				}
			}
		}

		rows, err := q.DB.QueryContext(ctx, query, params...)
		if err != nil {
			return nil, 0, err
		}
		defer func() { _ = rows.Close() }()

		for rows.Next() {
			var page models.Page
			var content sql.NullString
			var created, updated sql.NullTime

			err := rows.Scan(&page.ID, &page.Name, &page.Slug, &content, &page.Position, &page.Active, &created, &updated)
			if err != nil {
				return nil, 0, err
			}

			if content.Valid {
				page.Content = &content.String
			}
			if created.Valid {
				page.Created = created.Time.Unix()
			}
			if updated.Valid {
				page.Updated = updated.Time.Unix()
			}

			pages = append(pages, page)
		}

		if err := rows.Err(); err != nil {
			return nil, 0, err
		}
	} else {
		// Use sqlc ListPages for all pages, but handle limit=0 case
		if limit == 0 {
			limit = 999999 // Large number to get all rows
		}

		var results interface{}
		var err error

		if DBType() == "postgres" {
			pgQueries := queries.(*postgres.Queries)
			params := postgres.ListPagesParams{
				Limit:  int32(limit),
				Offset: int32(offset),
			}
			results, err = pgQueries.ListPages(ctx, params)
		} else {
			sqliteQueries := queries.(*sqlite.Queries)
			params := sqlite.ListPagesParams{
				Limit:  int64(limit),
				Offset: int64(offset),
			}
			results, err = sqliteQueries.ListPages(ctx, params)
		}

		if err != nil {
			return nil, 0, err
		}

		// Convert results to models.Page
		switch v := results.(type) {
		case []sqlite.ListPagesRow:
			for _, row := range v {
				var page models.Page
				page.ID = row.ID
				page.Name = row.Name
				page.Slug = row.Slug
				page.Position = row.Position
				page.Active = row.Active
				if row.Content.Valid {
					page.Content = &row.Content.String
				}
				if row.Created.Valid {
					page.Created = row.Created.Time.Unix()
				}
				if row.Updated.Valid {
					page.Updated = row.Updated.Time.Unix()
				}

				pages = append(pages, page)
			}
		case []postgres.ListPagesRow:
			for _, row := range v {
				var page models.Page
				page.ID = row.ID
				page.Name = row.Name
				page.Slug = row.Slug
				page.Position = row.Position
				page.Active = row.Active
				if row.Content.Valid {
					page.Content = &row.Content.String
				}
				if row.Created.Valid {
					page.Created = row.Created.Time.Unix()
				}
				if row.Updated.Valid {
					page.Updated = row.Updated.Time.Unix()
				}

				pages = append(pages, page)
			}
		}
	}

	// Count total records using sqlc
	var total int64
	var err error

	if DBType() == "postgres" {
		pgQueries := queries.(*postgres.Queries)
		total, err = pgQueries.CountPages(ctx)
	} else {
		sqliteQueries := queries.(*sqlite.Queries)
		total, err = sqliteQueries.CountPages(ctx)
	}

	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, 0, err
	}

	// If not private, count only active pages
	if !private {
		countQuery := `SELECT COUNT(*) FROM page WHERE active = 1`
		err = q.DB.QueryRowContext(ctx, countQuery).Scan(&total)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return nil, 0, err
		}
	}

	return pages, int(total), nil
}

// Page retrieves a single page from the database based on its slug using sqlc.
func (q *PageQueries) Page(ctx context.Context, slug string) (*models.Page, error) {
	queries := getSQLCQueries()

	var result interface{}
	var err error

	if DBType() == "postgres" {
		pgQueries := queries.(*postgres.Queries)
		result, err = pgQueries.GetPageBySlug(ctx, slug)
	} else {
		sqliteQueries := queries.(*sqlite.Queries)
		result, err = sqliteQueries.GetPageBySlug(ctx, slug)
	}

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, errors.ErrPageNotFound
		}
		return nil, err
	}

	page := toModelPage(result)
	if page == nil {
		return nil, errors.ErrPageNotFound
	}

	// Load seo field separately
	if err := q.loadPageSeo(ctx, page); err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}

	return page, nil
}

// PageByID retrieves a single page from the database based on its ID using sqlc.
func (q *PageQueries) PageByID(ctx context.Context, id string) (*models.Page, error) {
	queries := getSQLCQueries()

	var result interface{}
	var err error

	if DBType() == "postgres" {
		pgQueries := queries.(*postgres.Queries)
		result, err = pgQueries.GetPageByID(ctx, id)
	} else {
		sqliteQueries := queries.(*sqlite.Queries)
		result, err = sqliteQueries.GetPageByID(ctx, id)
	}

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, errors.ErrPageNotFound
		}
		return nil, err
	}

	page := toModelPage(result)
	if page == nil {
		return nil, errors.ErrPageNotFound
	}

	// Load seo field separately
	if err := q.loadPageSeo(ctx, page); err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}

	return page, nil
}

// AddPage inserts a new page into the database and returns the created page using sqlc.
func (q *PageQueries) AddPage(ctx context.Context, page *models.Page) (*models.Page, error) {
	page.ID = security.RandomString()
	page.Active = false

	queries := getSQLCQueries()

	var contentVal sql.NullString
	if page.Content != nil {
		contentVal = sql.NullString{String: *page.Content, Valid: true}
	}

	var result interface{}
	var err error

	if DBType() == "postgres" {
		pgQueries := queries.(*postgres.Queries)
		params := postgres.CreatePageParams{
			ID:       page.ID,
			Name:     page.Name,
			Slug:     page.Slug,
			Content:  contentVal,
			Position: page.Position,
			Active:   page.Active,
		}
		result, err = pgQueries.CreatePage(ctx, params)
	} else {
		sqliteQueries := queries.(*sqlite.Queries)
		params := sqlite.CreatePageParams{
			ID:       page.ID,
			Name:     page.Name,
			Slug:     page.Slug,
			Content:  contentVal,
			Position: page.Position,
			Active:   page.Active,
		}
		result, err = sqliteQueries.CreatePage(ctx, params)
	}

	if err != nil {
		return nil, err
	}

	created := toModelPage(result)
	if created == nil {
		return nil, errors.ErrPageNotFound
	}

	return created, nil
}

// UpdatePage updates the details of a page in the database using sqlc.
// If some fields are not provided (empty strings or nil), they are loaded from the database first.
func (q *PageQueries) UpdatePage(ctx context.Context, page *models.Page) error {
	// Load current page data for partial updates
	currentPage, err := q.PageByID(ctx, page.ID)
	if err != nil {
		return err
	}

	// Use provided values or current ones from database
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

	var contentVal sql.NullString
	switch {
	case page.Content != nil:
		contentVal = sql.NullString{String: *page.Content, Valid: true}
	case currentPage.Content != nil:
		contentVal = sql.NullString{String: *currentPage.Content, Valid: true}
	}

	queries := getSQLCQueries()

	// Update main fields with sqlc
	if DBType() == "postgres" {
		pgQueries := queries.(*postgres.Queries)
		params := postgres.UpdatePageParams{
			Name:     name,
			Slug:     slug,
			Content:  contentVal,
			Position: position,
			Active:   page.Active,
			ID:       page.ID,
		}
		if err := pgQueries.UpdatePage(ctx, params); err != nil {
			return err
		}
	} else {
		sqliteQueries := queries.(*sqlite.Queries)
		params := sqlite.UpdatePageParams{
			Name:     name,
			Slug:     slug,
			Content:  contentVal,
			Position: position,
			Active:   page.Active,
			ID:       page.ID,
		}
		if err := sqliteQueries.UpdatePage(ctx, params); err != nil {
			return err
		}
	}

	// Update seo field separately (direct SQL since seo is not in sqlc queries)
	var seoData *models.Seo
	if page.Seo != nil {
		seoData = page.Seo
	} else {
		seoData = currentPage.Seo
	}

	seo, err := json.Marshal(seoData)
	if err != nil {
		return err
	}

	var query string
	if DBType() == "postgres" || DBType() == "postgresql" {
		query = `UPDATE page SET seo = $1 WHERE id = $2`
	} else {
		query = `UPDATE page SET seo = ? WHERE id = ?`
	}
	_, err = q.DB.ExecContext(ctx, query, seo, page.ID)
	return err
}

// DeletePage method belongs to the PageQueries struct. This method is responsible for deleting a page from the database using sqlc.
func (q *PageQueries) DeletePage(ctx context.Context, id string) error {
	queries := getSQLCQueries()

	if DBType() == "postgres" {
		pgQueries := queries.(*postgres.Queries)
		return pgQueries.DeletePage(ctx, id)
	}

	sqliteQueries := queries.(*sqlite.Queries)
	return sqliteQueries.DeletePage(ctx, id)
}

// UpdatePageContent updates the content of an existing page in the database using sqlc.
func (q *PageQueries) UpdatePageContent(ctx context.Context, page *models.Page) error {
	queries := getSQLCQueries()

	var contentVal sql.NullString
	if page.Content != nil {
		contentVal = sql.NullString{String: *page.Content, Valid: true}
	}

	if DBType() == "postgres" {
		pgQueries := queries.(*postgres.Queries)
		params := postgres.UpdatePageContentParams{
			Content: contentVal,
			ID:      page.ID,
		}
		return pgQueries.UpdatePageContent(ctx, params)
	}

	sqliteQueries := queries.(*sqlite.Queries)
	params := sqlite.UpdatePageContentParams{
		Content: contentVal,
		ID:      page.ID,
	}
	return sqliteQueries.UpdatePageContent(ctx, params)
}

// UpdatePageActive toggles the active status of a page with the given ID using sqlc.
func (q *PageQueries) UpdatePageActive(ctx context.Context, id string) error {
	queries := getSQLCQueries()

	if DBType() == "postgres" {
		pgQueries := queries.(*postgres.Queries)
		return pgQueries.UpdatePageActive(ctx, id)
	}

	sqliteQueries := queries.(*sqlite.Queries)
	return sqliteQueries.UpdatePageActive(ctx, id)
}
