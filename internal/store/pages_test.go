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
	page, err := store.CreatePage(ctx, db.CreatePageParams{
		ID:      NewTestID(),
		Name:    "Test Page",
		Slug:    "test-page",
		Content: sql.NullString{String: "Test content", Valid: true},
		Position: "header",
		Active:  true,
	})
	require.NoError(t, err)
	require.Equal(t, NewTestID(), page.ID)

	// Retrieve page by slug
	retrieved, err := store.GetPageBySlug(ctx, "test-page")
	require.NoError(t, err)
	require.Equal(t, NewTestID(), retrieved.ID)
	require.Equal(t, "Test Page", retrieved.Name)
	require.Equal(t, "test-page", retrieved.Slug)
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
	_, err := store.CreatePage(ctx, db.CreatePageParams{
		ID:       "page-1",
		Name:     "Page 1",
		Slug:     "page-1",
		Content:  sql.NullString{String: "Content 1", Valid: true},
		Position: "header",
		Active:   true,
	})
	require.NoError(t, err)

	_, err = store.CreatePage(ctx, db.CreatePageParams{
		ID:       "page-2",
		Name:     "Page 2",
		Slug:     "page-2",
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
		if p.ID == "page-1" {
			foundPage1 = true
			require.Equal(t, "Page 1", p.Name)
		}
		if p.ID == "page-2" {
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
	page, err := store.CreatePage(ctx, db.CreatePageParams{
		ID:       "update-test",
		Name:     "Original Name",
		Slug:     "original-slug",
		Content:  sql.NullString{String: "Original content", Valid: true},
		Position: "header",
		Active:   false,
	})
	require.NoError(t, err)

	// Update the page
	err = store.UpdatePageSqlc(ctx, db.UpdatePageParams{
		Name:     "Updated Name",
		Slug:     "updated-slug",
		Content:  sql.NullString{String: "Updated content", Valid: true},
		Position: "footer",
		Active:   true,
		ID:       page.ID,
	})
	require.NoError(t, err)

	// Verify update
	updated, err := store.GetPageBySlug(ctx, "updated-slug")
	require.NoError(t, err)
	require.Equal(t, "Updated Name", updated.Name)
	require.Equal(t, "updated-slug", updated.Slug)
	require.Equal(t, "Updated content", updated.Content.String)
	require.Equal(t, "footer", updated.Position)
	require.True(t, updated.Active)
}

func TestDeletePage(t *testing.T) {
	ctx := setupTestDB(t)

	// Create a page
	page, err := store.CreatePage(ctx, db.CreatePageParams{
		ID:       "delete-test",
		Name:     "Delete Me",
		Slug:     "delete-me",
		Content:  sql.NullString{String: "Delete content", Valid: true},
		Position: "header",
		Active:   true,
	})
	require.NoError(t, err)

	// Delete the page
	err = store.DeletePageSqlc(ctx, page.ID)
	require.NoError(t, err)

	// Verify deletion
	_, err = store.GetPageBySlug(ctx, "delete-me")
	require.Error(t, err)
	require.Equal(t, sql.ErrNoRows, err)
}
