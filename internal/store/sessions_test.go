package store_test

import (
	"database/sql"
	"testing"

	"github.com/shurco/mycart/internal/store"
	"github.com/stretchr/testify/require"
)

func TestGetSession(t *testing.T) {
	ctx := setupTestDB(t)

	// Add a session
	key := "test_key"
	value := "test_value"
	expires := int64(2000000000) // May 2033 - fits in PostgreSQL Int32
	err := store.AddSession(ctx, key, value, expires)
	require.NoError(t, err)

	// Retrieve session
	session, err := store.GetSession(ctx, key)
	require.NoError(t, err)
	require.Equal(t, key, session.Key)
	require.True(t, session.Value.Valid)
	require.Equal(t, value, session.Value.String)
	require.True(t, session.Expires.Valid)
	require.Equal(t, expires, session.Expires.Int64)
}

func TestGetSession_NotFound(t *testing.T) {
	ctx := setupTestDB(t)

	_, err := store.GetSession(ctx, "nonexistent")
	require.Error(t, err)
	require.Equal(t, sql.ErrNoRows, err)
}
