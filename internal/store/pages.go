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

	dbPages, err := db.ListPagesFunc(ctx, int32(limit), int32(offset))
	if err != nil {
		return nil, 0, err
	}

	pages := make([]models.Page, 0, len(dbPages))
	for _, dbPage := range dbPages {
		// Filter by active if not private
		if !private && !dbPage.Active {
			continue
		}

		page := convertDBPageToModel(dbPage)
		pages = append(pages, *page)
	}

	// Get total count
	total, err := db.CountPagesFunc(ctx)
	if err != nil {
		return nil, 0, err
	}

	// Adjust count for non-private (active only) if needed
	if !private {
		activeCount := int64(0)
		for _, dbPage := range dbPages {
			if dbPage.Active {
				activeCount++
			}
		}
		// Note: This is approximate. For exact count we'd need CountPagesPublic query
		return pages, int(activeCount), nil
	}

	return pages, int(total), nil
}

// PageByID retrieves a page by ID
func PageByID(ctx context.Context, id string) (*models.Page, error) {
	dbPage, err := db.GetPageByIDFunc(ctx, id)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("page not found")
		}
		return nil, err
	}

	page := convertDBPageToModel(dbPage)
	if err := loadPageSeo(ctx, page); err != nil {
		return nil, err
	}

	return page, nil
}

// AddPage creates a new page
func AddPage(ctx context.Context, page *models.Page) (*models.Page, error) {
	if page.ID == "" {
		page.ID = security.RandomString()
	}

	content := sql.NullString{}
	if page.Content != nil {
		content.String = *page.Content
		content.Valid = true
	}

	dbPage, err := db.CreatePageFunc(ctx, db.CreatePageParams{
		ID:       page.ID,
		Name:     page.Name,
		Slug:     page.Slug,
		Content:  content,
		Position: page.Position,
		Active:   page.Active,
	})
	if err != nil {
		return nil, err
	}

	result := convertDBPageToModel(dbPage)
	result.Seo = page.Seo

	// Update SEO if provided
	if page.Seo != nil {
		seoJSON, err := json.Marshal(page.Seo)
		if err != nil {
			return nil, err
		}
		// Need to update SEO separately as CreatePage doesn't handle it
		var query string
		if db.Type() == "postgres" {
			query = `UPDATE page SET seo = $1 WHERE id = $2`
		} else {
			query = `UPDATE page SET seo = ? WHERE id = ?`
		}
		if _, err := db.DB().ExecContext(ctx, query, seoJSON, page.ID); err != nil {
			return nil, err
		}
	}

	return result, nil
}

// UpdatePage updates an existing page
func UpdatePage(ctx context.Context, page *models.Page) error {
	content := sql.NullString{}
	if page.Content != nil {
		content.String = *page.Content
		content.Valid = true
	}

	err := db.UpdatePageFunc(ctx, db.UpdatePageParams{
		Name:     page.Name,
		Slug:     page.Slug,
		Content:  content,
		Position: page.Position,
		Active:   page.Active,
		ID:       page.ID,
	})
	if err != nil {
		return err
	}

	// Update SEO separately as UpdatePage doesn't handle it
	if page.Seo != nil {
		seoJSON, err := json.Marshal(page.Seo)
		if err != nil {
			return err
		}
		var query string
		if db.Type() == "postgres" {
			query = `UPDATE page SET seo = $1 WHERE id = $2`
		} else {
			query = `UPDATE page SET seo = ? WHERE id = ?`
		}
		if _, err := db.DB().ExecContext(ctx, query, seoJSON, page.ID); err != nil {
			return err
		}
	}

	return nil
}

// DeletePage deletes a page by ID
func DeletePage(ctx context.Context, id string) error {
	return db.DeletePageFunc(ctx, id)
}

// UpdatePageContent updates page content
func UpdatePageContent(ctx context.Context, page *models.Page) error {
	content := sql.NullString{}
	if page.Content != nil {
		content.String = *page.Content
		content.Valid = true
	}

	return db.UpdatePageContentFunc(ctx, page.ID, content)
}

// UpdatePageActive toggles page active status
func UpdatePageActive(ctx context.Context, id string) error {
	return db.UpdatePageActiveFunc(ctx, id)
}

// IsPage checks if a page exists by slug
func IsPage(ctx context.Context, slug string) bool {
	exists, err := db.PageExistsFunc(ctx, slug)
	if err != nil {
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
	seo, err := db.GetPageSeoFunc(ctx, page.ID)
	if err != nil {
		return err
	}

	if len(seo) > 0 {
		var seoData models.Seo
		if err := json.Unmarshal(seo, &seoData); err != nil {
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
