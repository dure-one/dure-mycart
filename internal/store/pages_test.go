package store_test

import (
	"database/sql"
	"testing"

	"github.com/shurco/mycart/internal/store"
	"github.com/shurco/mycart/internal/store/db"
	"github.com/stretchr/testify/require"
)

func TestGetPageBySlug(t *testing.T) {
	ctx := setupTestDB(t)

	// Create a page first
	pageID := NewTestID()
	slug := "test-page-" + pageID
	page, err := store.CreatePage(ctx, db.CreatePageParams{
		ID:       pageID,
		Name:     "Test Page",
		Slug:     slug,
		Content:  sql.NullString{String: "Test content", Valid: true},
		Position: "header",
		Active:   true,
	})
	require.NoError(t, err)
	require.Equal(t, pageID, page.ID)

	// Retrieve page by slug
	retrieved, err := store.GetPageBySlug(ctx, slug)
	require.NoError(t, err)
	require.Equal(t, pageID, retrieved.ID)
	require.Equal(t, "Test Page", retrieved.Name)
	require.Equal(t, slug, retrieved.Slug)
	require.True(t, retrieved.Content.Valid)
	require.Equal(t, "Test content", retrieved.Content.String)
	require.Equal(t, "header", retrieved.Position)
	require.True(t, retrieved.Active)
}

func TestGetPageBySlug_NotFound(t *testing.T) {
	ctx := setupTestDB(t)

	_, err := store.GetPageBySlug(ctx, "nonexistent")
	require.Error(t, err)
	require.Equal(t, sql.ErrNoRows, err)
}

func TestListPages(t *testing.T) {
	ctx := setupTestDB(t)

	// Create multiple pages
	page1ID := NewTestID()
	_, err := store.CreatePage(ctx, db.CreatePageParams{
		ID:       page1ID,
		Name:     "Page 1",
		Slug:     "page-1-" + page1ID,
		Content:  sql.NullString{String: "Content 1", Valid: true},
		Position: "header",
		Active:   true,
	})
	require.NoError(t, err)

	page2ID := NewTestID()
	_, err = store.CreatePage(ctx, db.CreatePageParams{
		ID:       page2ID,
		Name:     "Page 2",
		Slug:     "page-2-" + page2ID,
		Content:  sql.NullString{String: "Content 2", Valid: true},
		Position: "footer",
		Active:   false,
	})
	require.NoError(t, err)

	// List pages with pagination
	pages, err := store.ListPagesSqlc(ctx, 10, 0)
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(pages), 2, "should have at least our 2 created pages")

	// Check our created pages are in the list
	var foundPage1, foundPage2 bool
	for _, p := range pages {
		if p.ID == page1ID {
			foundPage1 = true
			require.Equal(t, "Page 1", p.Name)
		}
		if p.ID == page2ID {
			foundPage2 = true
			require.Equal(t, "Page 2", p.Name)
		}
	}
	require.True(t, foundPage1, "page-1 should be in results")
	require.True(t, foundPage2, "page-2 should be in results")
}

func TestUpdatePage(t *testing.T) {
	ctx := setupTestDB(t)

	// Create a page
	pageID := NewTestID()
	originalSlug := "original-slug-" + pageID
	page, err := store.CreatePage(ctx, db.CreatePageParams{
		ID:       pageID,
		Name:     "Original Name",
		Slug:     originalSlug,
		Content:  sql.NullString{String: "Original content", Valid: true},
		Position: "header",
		Active:   false,
	})
	require.NoError(t, err)

	// Update the page
	updatedSlug := "updated-slug-" + pageID
	err = store.UpdatePageSqlc(ctx, db.UpdatePageParams{
		Name:     "Updated Name",
		Slug:     updatedSlug,
		Content:  sql.NullString{String: "Updated content", Valid: true},
		Position: "footer",
		Active:   true,
		ID:       page.ID,
	})
	require.NoError(t, err)

	// Verify update
	updated, err := store.GetPageBySlug(ctx, updatedSlug)
	require.NoError(t, err)
	require.Equal(t, "Updated Name", updated.Name)
	require.Equal(t, updatedSlug, updated.Slug)
	require.Equal(t, "Updated content", updated.Content.String)
	require.Equal(t, "footer", updated.Position)
	require.True(t, updated.Active)
}

func TestDeletePage(t *testing.T) {
	ctx := setupTestDB(t)

	// Create a page
	pageID := NewTestID()
	slug := "delete-me-" + pageID
	page, err := store.CreatePage(ctx, db.CreatePageParams{
		ID:       pageID,
		Name:     "Delete Me",
		Slug:     slug,
		Content:  sql.NullString{String: "Delete content", Valid: true},
		Position: "header",
		Active:   true,
	})
	require.NoError(t, err)

	// Delete the page
	err = store.DeletePageSqlc(ctx, page.ID)
	require.NoError(t, err)

	// Verify deletion
	_, err = store.GetPageBySlug(ctx, slug)
	require.Error(t, err)
	require.Equal(t, sql.ErrNoRows, err)
}
