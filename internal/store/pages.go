package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/shurco/mycart/internal/models"
	"github.com/shurco/mycart/internal/store/db"
	"github.com/shurco/mycart/pkg/errors"
	"github.com/shurco/mycart/pkg/security"
)

// Page retrieves a page by slug for public access
func Page(ctx context.Context, slug string) (*models.Page, error) {
	dbPage, err := db.GetPageBySlugFunc(ctx, slug)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, errors.ErrPageNotFound
		}
		return nil, err
	}

	page := convertDBPageToModel(dbPage)
	if err := loadPageSeo(ctx, page); err != nil {
		return nil, err
	}

	return page, nil
}

// ListPages retrieves a paginated list of pages
func ListPages(ctx context.Context, private bool, limit, offset int, idList ...string) ([]models.Page, int, error) {
	if limit == 0 {
		limit = 999999
	}

	// Build query based on private flag
	var query string
	var params []any

	if private {
		query = `SELECT id, name, slug, content, position, active, created, updated FROM page ORDER BY created DESC`
	} else {
		if db.Type() == "postgres" {
			query = `SELECT id, name, slug, content, position, active, created, updated FROM page WHERE active = true ORDER BY created DESC`
		} else {
			query = `SELECT id, name, slug, content, position, active, created, updated FROM page WHERE active = 1 ORDER BY created DESC`
		}
	}

	// Add pagination
	if db.Type() == "postgres" {
		query += " LIMIT $1 OFFSET $2"
		params = []any{limit, offset}
	} else {
		query += " LIMIT ? OFFSET ?"
		params = []any{limit, offset}
	}

	rows, err := db.DB().QueryContext(ctx, query, params...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	pages := []models.Page{}
	for rows.Next() {
		var p models.Page
		var content sql.NullString
		var created, updated sql.NullTime

		err := rows.Scan(&p.ID, &p.Name, &p.Slug, &content, &p.Position, &p.Active, &created, &updated)
		if err != nil {
			return nil, 0, err
		}

		if content.Valid {
			p.Content = &content.String
		}
		if created.Valid {
			p.Created = created.Time.Unix()
		}
		if updated.Valid {
			p.Updated = updated.Time.Unix()
		}

		pages = append(pages, p)
	}

	// Get total count
	var countQuery string
	if private {
		countQuery = `SELECT COUNT(*) FROM page`
	} else {
		if db.Type() == "postgres" {
			countQuery = `SELECT COUNT(*) FROM page WHERE active = true`
		} else {
			countQuery = `SELECT COUNT(*) FROM page WHERE active = 1`
		}
	}

	var total int
	if err := db.DB().QueryRowContext(ctx, countQuery).Scan(&total); err != nil {
		return nil, 0, err
	}

	return pages, total, nil
}

// PageByID retrieves a page by ID
func PageByID(ctx context.Context, id string) (*models.Page, error) {
	var query string
	if db.Type() == "postgres" {
		query = `SELECT id, name, slug, content, position, active, created, updated FROM page WHERE id = $1`
	} else {
		query = `SELECT id, name, slug, content, position, active, created, updated FROM page WHERE id = ?`
	}

	var p models.Page
	var content sql.NullString
	var created, updated sql.NullTime

	err := db.DB().QueryRowContext(ctx, query, id).Scan(&p.ID, &p.Name, &p.Slug, &content, &p.Position, &p.Active, &created, &updated)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("page not found")
		}
		return nil, err
	}

	if content.Valid {
		p.Content = &content.String
	}
	if created.Valid {
		p.Created = created.Time.Unix()
	}
	if updated.Valid {
		p.Updated = updated.Time.Unix()
	}

	if err := loadPageSeo(ctx, &p); err != nil {
		return nil, err
	}

	return &p, nil
}

// AddPage creates a new page
func AddPage(ctx context.Context, page *models.Page) (*models.Page, error) {
	if page.ID == "" {
		page.ID = security.RandomString()
	}

	seoJSON, err := json.Marshal(page.Seo)
	if err != nil {
		return nil, err
	}

	contentStr := ""
	if page.Content != nil {
		contentStr = *page.Content
	}

	var query string
	if db.Type() == "postgres" {
		query = `INSERT INTO page (id, name, slug, content, position, active, seo)
		         VALUES ($1, $2, $3, $4, $5, $6, $7)
		         RETURNING id, name, slug, content, position, active, created, updated`
	} else {
		query = `INSERT INTO page (id, name, slug, content, position, active, seo)
		         VALUES (?, ?, ?, ?, ?, ?, ?)`
	}

	if db.Type() == "postgres" {
		var created, updated sql.NullTime
		var content sql.NullString
		err = db.DB().QueryRowContext(ctx, query, page.ID, page.Name, page.Slug, contentStr, page.Position, page.Active, seoJSON).
			Scan(&page.ID, &page.Name, &page.Slug, &content, &page.Position, &page.Active, &created, &updated)
		if err != nil {
			return nil, err
		}
		if created.Valid {
			page.Created = created.Time.Unix()
		}
		if updated.Valid {
			page.Updated = updated.Time.Unix()
		}
	} else {
		_, err = db.DB().ExecContext(ctx, query, page.ID, page.Name, page.Slug, contentStr, page.Position, page.Active, seoJSON)
		if err != nil {
			return nil, err
		}
	}

	return page, nil
}

// UpdatePage updates an existing page
func UpdatePage(ctx context.Context, page *models.Page) error {
	seoJSON, err := json.Marshal(page.Seo)
	if err != nil {
		return err
	}

	contentStr := ""
	if page.Content != nil {
		contentStr = *page.Content
	}

	var query string
	if db.Type() == "postgres" {
		query = `UPDATE page SET name = $1, slug = $2, content = $3, position = $4, active = $5, seo = $6 WHERE id = $7`
	} else {
		query = `UPDATE page SET name = ?, slug = ?, content = ?, position = ?, active = ?, seo = ? WHERE id = ?`
	}

	_, err = db.DB().ExecContext(ctx, query, page.Name, page.Slug, contentStr, page.Position, page.Active, seoJSON, page.ID)
	return err
}

// DeletePage deletes a page by ID
func DeletePage(ctx context.Context, id string) error {
	return db.DeletePageFunc(ctx, id)
}

// UpdatePageContent updates page content
func UpdatePageContent(ctx context.Context, page *models.Page) error {
	contentStr := ""
	if page.Content != nil {
		contentStr = *page.Content
	}

	var query string
	if db.Type() == "postgres" {
		query = `UPDATE page SET content = $1 WHERE id = $2`
	} else {
		query = `UPDATE page SET content = ? WHERE id = ?`
	}

	_, err := db.DB().ExecContext(ctx, query, contentStr, page.ID)
	return err
}

// UpdatePageActive toggles page active status
func UpdatePageActive(ctx context.Context, id string) error {
	var query string
	if db.Type() == "postgres" {
		query = `UPDATE page SET active = NOT active WHERE id = $1`
	} else {
		query = `UPDATE page SET active = NOT active WHERE id = ?`
	}

	_, err := db.DB().ExecContext(ctx, query, id)
	return err
}

// IsPage checks if a page exists by slug
func IsPage(ctx context.Context, slug string) bool {
	var query string
	if db.Type() == "postgres" {
		query = `SELECT EXISTS(SELECT 1 FROM page WHERE slug = $1)`
	} else {
		query = `SELECT EXISTS(SELECT 1 FROM page WHERE slug = ?)`
	}

	var exists bool
	if err := db.DB().QueryRowContext(ctx, query, slug).Scan(&exists); err != nil {
		return false
	}
	return exists
}

// Helper functions

func convertDBPageToModel(dbPage db.Page) *models.Page {
	page := &models.Page{
		Core: models.Core{
			ID: dbPage.ID,
		},
		Name:     dbPage.Name,
		Slug:     dbPage.Slug,
		Position: dbPage.Position,
		Active:   dbPage.Active,
	}

	if dbPage.Content.Valid {
		page.Content = &dbPage.Content.String
	}
	if dbPage.Created.Valid {
		page.Created = dbPage.Created.Time.Unix()
	}
	if dbPage.Updated.Valid {
		page.Updated = dbPage.Updated.Time.Unix()
	}

	return page
}

func loadPageSeo(ctx context.Context, page *models.Page) error {
	var seo sql.NullString
	var query string
	if db.Type() == "postgres" {
		query = `SELECT seo FROM page WHERE id = $1`
	} else {
		query = `SELECT seo FROM page WHERE id = ?`
	}

	if err := db.DB().QueryRowContext(ctx, query, page.ID).Scan(&seo); err != nil {
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

// --- sqlc-based methods (new implementation) ---

// GetPageBySlug retrieves a page by slug using sqlc function pointer.
func GetPageBySlug(ctx context.Context, slug string) (db.Page, error) {
	return db.GetPageBySlugFunc(ctx, slug)
}

// ListPagesSqlc retrieves a paginated list of pages using sqlc function pointer.
func ListPagesSqlc(ctx context.Context, limit, offset int32) ([]db.Page, error) {
	return db.ListPagesFunc(ctx, limit, offset)
}

// CreatePage creates a new page using sqlc function pointer.
func CreatePage(ctx context.Context, params db.CreatePageParams) (db.Page, error) {
	return db.CreatePageFunc(ctx, params)
}

// UpdatePageSqlc updates a page using sqlc function pointer.
func UpdatePageSqlc(ctx context.Context, params db.UpdatePageParams) error {
	return db.UpdatePageFunc(ctx, params)
}

// DeletePageSqlc deletes a page using sqlc function pointer.
func DeletePageSqlc(ctx context.Context, id string) error {
	return db.DeletePageFunc(ctx, id)
}
