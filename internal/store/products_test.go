package store_test

import (
	"database/sql"
	"testing"

	"github.com/shurco/mycart/internal/store"
	"github.com/shurco/mycart/internal/store/db"
	"github.com/stretchr/testify/require"
)

func TestGetProductByID(t *testing.T) {
	ctx := setupTestDB(t)

	// Create a product first
	product, err := store.CreateProduct(ctx, db.CreateProductParams{
		ID:        NewTestID(),
		Name:      "Test Product",
		Desc:      "Test description",
		Slug:      "test-product",
		Amount:    "1000",
		Metadata:  []byte(`{}`),
		Attribute: []byte(`{}`),
		Digital:   sql.NullString{Valid: false},
		Active:    true,
	})
	require.NoError(t, err)
	require.Equal(t, NewTestID(), product.ID)

	// Retrieve product by ID
	retrieved, err := store.GetProductByID(ctx, NewTestID())
	require.NoError(t, err)
	require.Equal(t, NewTestID(), retrieved.ID)
	require.Equal(t, "Test Product", retrieved.Name)
	require.Equal(t, "Test description", retrieved.Desc)
	require.Equal(t, "test-product", retrieved.Slug)
	require.Equal(t, "1000", retrieved.Amount)
	require.True(t, retrieved.Active)
	require.False(t, retrieved.Deleted)
}

func TestGetProductBySlug(t *testing.T) {
	ctx := setupTestDB(t)

	// Create a product
	productID := NewTestID()
	slug := "slug-test-product-" + productID
	_, err := store.CreateProduct(ctx, db.CreateProductParams{
		ID:        productID,
		Name:      "Slug Test",
		Desc:      "Description",
		Slug:      slug,
		Amount:    "2000",
		Metadata:  []byte(`{}`),
		Attribute: []byte(`{}`),
		Digital:   sql.NullString{Valid: false},
		Active:    true,
	})
	require.NoError(t, err)

	// Retrieve by slug
	product, err := store.GetProductBySlug(ctx, slug)
	require.NoError(t, err)
	require.Equal(t, productID, product.ID)
	require.Equal(t, "Slug Test", product.Name)
}

func TestUpdateProduct(t *testing.T) {
	ctx := setupTestDB(t)

	// Create a product
	productID := NewTestID()
	slug := "original-slug-" + productID
	product, err := store.CreateProduct(ctx, db.CreateProductParams{
		ID:        productID,
		Name:      "Original Name",
		Desc:      "Original description",
		Slug:      slug,
		Amount:    "1500",
		Metadata:  []byte(`{}`),
		Attribute: []byte(`{}`),
		Digital:   sql.NullString{Valid: false},
		Active:    false,
	})
	require.NoError(t, err)

	// Update the product
	err = store.UpdateProductSqlc(ctx, db.UpdateProductParams{
		Name:      "Updated Name",
		Desc:      "Updated description",
		Slug:      "updated-slug",
		Amount:    "2500",
		Metadata:  []byte(`{"key":"value"}`),
		Attribute: []byte(`{"attr":"val"}`),
		Digital:   sql.NullString{String: "file", Valid: true},
		Active:    true,
		ID:        product.ID,
	})
	require.NoError(t, err)

	// Verify update
	updated, err := store.GetProductByID(ctx, product.ID)
	require.NoError(t, err)
	require.Equal(t, "Updated Name", updated.Name)
	require.Equal(t, "Updated description", updated.Desc)
	require.Equal(t, "updated-slug", updated.Slug)
	require.Equal(t, "2500", updated.Amount)
	require.True(t, updated.Active)
}

func TestDeleteProduct(t *testing.T) {
	ctx := setupTestDB(t)

	// Create a product
	product, err := store.CreateProduct(ctx, db.CreateProductParams{
		ID:        "delete-test",
		Name:      "Delete Me",
		Desc:      "To be deleted",
		Slug:      "delete-me",
		Amount:    "999",
		Metadata:  []byte(`{}`),
		Attribute: []byte(`{}`),
		Digital:   sql.NullString{Valid: false},
		Active:    true,
	})
	require.NoError(t, err)

	// Delete the product
	err = store.DeleteProductSqlc(ctx, product.ID)
	require.NoError(t, err)

	// Verify deletion (should return error)
	_, err = store.GetProductByID(ctx, product.ID)
	require.Error(t, err)
	require.Equal(t, sql.ErrNoRows, err)
}
