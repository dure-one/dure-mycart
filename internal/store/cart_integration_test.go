package store_test

import (
	"database/sql"
	"testing"
	"time"

	"github.com/shurco/mycart/internal/store/db"
	"github.com/stretchr/testify/require"
)

func TestCreateNewCart(t *testing.T) {
	ctx := setupTestDB(t)

	// Arrange
	cartID := NewTestID()
	sessionID := NewTestID()

	// Act
	err := db.CreateNewCartFunc(ctx, db.CreateCartParams{
		ID:        cartID,
		SessionID: sessionID,
		Status:    "active",
		Total:     "0",
		CreatedAt: sql.NullTime{
			Time:  time.Now().UTC(),
			Valid: true,
		},
		UpdatedAt: sql.NullTime{Valid: false},
	})

	// Assert
	require.NoError(t, err)
}

func TestGetNewCartByID(t *testing.T) {
	ctx := setupTestDB(t)

	// Arrange - Create a cart first
	cartID := NewTestID()
	sessionID := NewTestID()
	createdAt := time.Now().UTC()

	err := db.CreateNewCartFunc(ctx, db.CreateCartParams{
		ID:        cartID,
		SessionID: sessionID,
		Status:    "active",
		Total:     "100.00",
		CreatedAt: sql.NullTime{Time: createdAt, Valid: true},
		UpdatedAt: sql.NullTime{Valid: false},
	})
	require.NoError(t, err)

	// Act - Retrieve the cart
	cart, err := db.GetNewCartByIDFunc(ctx, cartID)

	// Assert
	require.NoError(t, err)
	require.Equal(t, cartID, cart.ID)
	require.Equal(t, sessionID, cart.SessionID)
	require.Equal(t, "active", cart.Status)
	require.Equal(t, "100.00", cart.Total)
	require.True(t, cart.CreatedAt.Valid)
	require.False(t, cart.UpdatedAt.Valid)
}

func TestGetNewCartBySessionID(t *testing.T) {
	ctx := setupTestDB(t)

	// Arrange - Create multiple carts with same session
	sessionID := NewTestID()
	cart1ID := NewTestID()
	cart2ID := NewTestID()

	now := time.Now().UTC()
	earlier := now.Add(-1 * time.Hour)

	// Create older cart
	err := db.CreateNewCartFunc(ctx, db.CreateCartParams{
		ID:        cart1ID,
		SessionID: sessionID,
		Status:    "completed",
		Total:     "50.00",
		CreatedAt: sql.NullTime{Time: earlier, Valid: true},
		UpdatedAt: sql.NullTime{Valid: false},
	})
	require.NoError(t, err)

	// Create newer cart
	err = db.CreateNewCartFunc(ctx, db.CreateCartParams{
		ID:        cart2ID,
		SessionID: sessionID,
		Status:    "active",
		Total:     "100.00",
		CreatedAt: sql.NullTime{Time: now, Valid: true},
		UpdatedAt: sql.NullTime{Valid: false},
	})
	require.NoError(t, err)

	// Act - Get cart by session (should return newest)
	cart, err := db.GetNewCartBySessionIDFunc(ctx, sessionID)

	// Assert
	require.NoError(t, err)
	require.Equal(t, cart2ID, cart.ID, "should return newest cart")
	require.Equal(t, "active", cart.Status)
}

func TestUpdateNewCart(t *testing.T) {
	ctx := setupTestDB(t)

	// Arrange - Create a cart
	cartID := NewTestID()
	sessionID := NewTestID()

	err := db.CreateNewCartFunc(ctx, db.CreateCartParams{
		ID:        cartID,
		SessionID: sessionID,
		Status:    "active",
		Total:     "0",
		CreatedAt: sql.NullTime{Time: time.Now().UTC(), Valid: true},
		UpdatedAt: sql.NullTime{Valid: false},
	})
	require.NoError(t, err)

	// Act - Update cart
	updatedAt := time.Now().UTC()
	err = db.UpdateNewCartFunc(ctx, db.UpdateCartParams{
		Status:    "completed",
		Total:     "150.00",
		UpdatedAt: sql.NullTime{Time: updatedAt, Valid: true},
		ID:        cartID,
	})
	require.NoError(t, err)

	// Assert - Verify update
	cart, err := db.GetNewCartByIDFunc(ctx, cartID)
	require.NoError(t, err)
	require.Equal(t, "completed", cart.Status)
	require.Equal(t, "150.00", cart.Total)
	require.True(t, cart.UpdatedAt.Valid)
}

func TestCreateCartItem(t *testing.T) {
	ctx := setupTestDB(t)

	// Arrange - Create product first (cart_items has FK to product)
	productID := NewTestID()
	_, err := db.CreateProductFunc(ctx, db.CreateProductParams{
		ID:        productID,
		Name:      "Test Product",
		Desc:      "Test Description",
		Slug:      "test-product-" + productID,
		Amount:    "50.00",
		Metadata:  []byte("{}"),
		Attribute: []byte("{}"),
		Digital:   sql.NullString{Valid: false},
		Active:    true,
	})
	require.NoError(t, err)

	// Create cart
	cartID := NewTestID()
	sessionID := NewTestID()
	err = db.CreateNewCartFunc(ctx, db.CreateCartParams{
		ID:        cartID,
		SessionID: sessionID,
		Status:    "active",
		Total:     "0",
		CreatedAt: sql.NullTime{Time: time.Now().UTC(), Valid: true},
		UpdatedAt: sql.NullTime{Valid: false},
	})
	require.NoError(t, err)

	// Act - Create cart item
	itemID := NewTestID()
	err = db.CreateCartItemFunc(ctx, db.CreateCartItemParams{
		ID:        itemID,
		CartID:    cartID,
		ProductID: productID,
		Quantity:  2,
		Price:     "50.00",
		CreatedAt: sql.NullTime{Time: time.Now().UTC(), Valid: true},
	})

	// Assert
	require.NoError(t, err)
}

func TestListCartItems(t *testing.T) {
	ctx := setupTestDB(t)

	// Arrange - Create products
	product1ID := NewTestID()
	_, err := db.CreateProductFunc(ctx, db.CreateProductParams{
		ID:        product1ID,
		Name:      "Product 1",
		Desc:      "Description 1",
		Slug:      "product-1-" + product1ID,
		Amount:    "25.00",
		Metadata:  []byte("{}"),
		Attribute: []byte("{}"),
		Digital:   sql.NullString{Valid: false},
		Active:    true,
	})
	require.NoError(t, err)

	product2ID := NewTestID()
	_, err = db.CreateProductFunc(ctx, db.CreateProductParams{
		ID:        product2ID,
		Name:      "Product 2",
		Desc:      "Description 2",
		Slug:      "product-2-" + product2ID,
		Amount:    "15.00",
		Metadata:  []byte("{}"),
		Attribute: []byte("{}"),
		Digital:   sql.NullString{Valid: false},
		Active:    true,
	})
	require.NoError(t, err)

	// Create cart
	cartID := NewTestID()
	sessionID := NewTestID()
	err = db.CreateNewCartFunc(ctx, db.CreateCartParams{
		ID:        cartID,
		SessionID: sessionID,
		Status:    "active",
		Total:     "0",
		CreatedAt: sql.NullTime{Time: time.Now().UTC(), Valid: true},
		UpdatedAt: sql.NullTime{Valid: false},
	})
	require.NoError(t, err)

	// Create multiple items
	item1ID := NewTestID()
	err = db.CreateCartItemFunc(ctx, db.CreateCartItemParams{
		ID:        item1ID,
		CartID:    cartID,
		ProductID: product1ID,
		Quantity:  1,
		Price:     "25.00",
		CreatedAt: sql.NullTime{Time: time.Now().UTC(), Valid: true},
	})
	require.NoError(t, err)

	item2ID := NewTestID()
	err = db.CreateCartItemFunc(ctx, db.CreateCartItemParams{
		ID:        item2ID,
		CartID:    cartID,
		ProductID: product2ID,
		Quantity:  3,
		Price:     "15.00",
		CreatedAt: sql.NullTime{Time: time.Now().UTC(), Valid: true},
	})
	require.NoError(t, err)

	// Act - List all items
	items, err := db.ListCartItemsFunc(ctx, cartID)

	// Assert
	require.NoError(t, err)
	require.Len(t, items, 2)
	require.Equal(t, item1ID, items[0].ID)
	require.Equal(t, item2ID, items[1].ID)
}

func TestUpdateCartItem(t *testing.T) {
	ctx := setupTestDB(t)

	// Arrange - Create product
	productID := NewTestID()
	_, err := db.CreateProductFunc(ctx, db.CreateProductParams{
		ID:        productID,
		Name:      "Test Product",
		Desc:      "Test Description",
		Slug:      "test-product-" + productID,
		Amount:    "50.00",
		Metadata:  []byte("{}"),
		Attribute: []byte("{}"),
		Digital:   sql.NullString{Valid: false},
		Active:    true,
	})
	require.NoError(t, err)

	// Create cart
	cartID := NewTestID()
	sessionID := NewTestID()
	err = db.CreateNewCartFunc(ctx, db.CreateCartParams{
		ID:        cartID,
		SessionID: sessionID,
		Status:    "active",
		Total:     "0",
		CreatedAt: sql.NullTime{Time: time.Now().UTC(), Valid: true},
		UpdatedAt: sql.NullTime{Valid: false},
	})
	require.NoError(t, err)

	// Create cart item
	itemID := NewTestID()
	err = db.CreateCartItemFunc(ctx, db.CreateCartItemParams{
		ID:        itemID,
		CartID:    cartID,
		ProductID: productID,
		Quantity:  2,
		Price:     "50.00",
		CreatedAt: sql.NullTime{Time: time.Now().UTC(), Valid: true},
	})
	require.NoError(t, err)

	// Act - Update quantity and price
	err = db.UpdateCartItemFunc(ctx, db.UpdateCartItemParams{
		Quantity: 5,
		Price:    "45.00",
		ID:       itemID,
	})
	require.NoError(t, err)

	// Assert - Verify update
	item, err := db.GetCartItemFunc(ctx, itemID)
	require.NoError(t, err)
	require.Equal(t, int64(5), item.Quantity)
	require.Equal(t, "45.00", item.Price)
}

func TestDeleteCartItem(t *testing.T) {
	ctx := setupTestDB(t)

	// Arrange - Create product
	productID := NewTestID()
	_, err := db.CreateProductFunc(ctx, db.CreateProductParams{
		ID:        productID,
		Name:      "Test Product",
		Desc:      "Test Description",
		Slug:      "test-product-" + productID,
		Amount:    "50.00",
		Metadata:  []byte("{}"),
		Attribute: []byte("{}"),
		Digital:   sql.NullString{Valid: false},
		Active:    true,
	})
	require.NoError(t, err)

	// Create cart
	cartID := NewTestID()
	sessionID := NewTestID()
	err = db.CreateNewCartFunc(ctx, db.CreateCartParams{
		ID:        cartID,
		SessionID: sessionID,
		Status:    "active",
		Total:     "0",
		CreatedAt: sql.NullTime{Time: time.Now().UTC(), Valid: true},
		UpdatedAt: sql.NullTime{Valid: false},
	})
	require.NoError(t, err)

	// Create cart item
	itemID := NewTestID()
	err = db.CreateCartItemFunc(ctx, db.CreateCartItemParams{
		ID:        itemID,
		CartID:    cartID,
		ProductID: productID,
		Quantity:  2,
		Price:     "50.00",
		CreatedAt: sql.NullTime{Time: time.Now().UTC(), Valid: true},
	})
	require.NoError(t, err)

	// Act - Delete item
	err = db.DeleteCartItemFunc(ctx, itemID)
	require.NoError(t, err)

	// Assert - Verify deletion
	_, err = db.GetCartItemFunc(ctx, itemID)
	require.Error(t, err)
	require.Equal(t, sql.ErrNoRows, err)
}

func TestDeleteCartByID(t *testing.T) {
	ctx := setupTestDB(t)

	// Arrange - Create cart
	cartID := NewTestID()
	sessionID := NewTestID()
	err := db.CreateNewCartFunc(ctx, db.CreateCartParams{
		ID:        cartID,
		SessionID: sessionID,
		Status:    "active",
		Total:     "0",
		CreatedAt: sql.NullTime{Time: time.Now().UTC(), Valid: true},
		UpdatedAt: sql.NullTime{Valid: false},
	})
	require.NoError(t, err)

	// Act - Delete cart
	err = db.DeleteNewCartFunc(ctx, cartID)
	require.NoError(t, err)

	// Assert - Verify cart deleted
	_, err = db.GetNewCartByIDFunc(ctx, cartID)
	require.Error(t, err)
	require.Equal(t, sql.ErrNoRows, err)
}

func TestGetNewCartByID_NotFound(t *testing.T) {
	ctx := setupTestDB(t)

	// Act - Try to get non-existent cart
	nonExistentID := NewTestID()
	_, err := db.GetNewCartByIDFunc(ctx, nonExistentID)

	// Assert
	require.Error(t, err)
	require.Equal(t, sql.ErrNoRows, err)
}
