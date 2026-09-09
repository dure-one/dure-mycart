package store_test

import (
	"database/sql"
	"testing"
	"time"

	"github.com/shurco/mycart/internal/store/db"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"
)

func TestCreateUser(t *testing.T) {
	ctx := setupTestDB(t)

	// Arrange
	userID := NewTestID()
	email := NewTestEmail()
	password := "TestPassword123!"
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	require.NoError(t, err)

	// Act
	err = db.CreateUserFunc(ctx, db.CreateUserParams{
		ID:       userID,
		Email:    email,
		Password: string(hashedPassword),
		CreatedAt: sql.NullTime{
			Time:  time.Now().UTC(),
			Valid: true,
		},
		UpdatedAt: sql.NullTime{Valid: false},
	})

	// Assert
	require.NoError(t, err)
}

func TestGetUserByEmail(t *testing.T) {
	ctx := setupTestDB(t)

	// Arrange - Create a user first
	userID := NewTestID()
	email := NewTestEmail()
	password := "TestPassword123!"
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	require.NoError(t, err)
	createdAt := time.Now().UTC()

	err = db.CreateUserFunc(ctx, db.CreateUserParams{
		ID:       userID,
		Email:    email,
		Password: string(hashedPassword),
		CreatedAt: sql.NullTime{
			Time:  createdAt,
			Valid: true,
		},
		UpdatedAt: sql.NullTime{Valid: false},
	})
	require.NoError(t, err)

	// Act - Retrieve the user by email
	user, err := db.GetUserByEmailFunc(ctx, email)

	// Assert
	require.NoError(t, err)
	require.Equal(t, userID, user.ID)
	require.Equal(t, email, user.Email)
	require.NotEmpty(t, user.Password)
	require.True(t, user.CreatedAt.Valid)
	require.False(t, user.UpdatedAt.Valid)

	// Verify password hash is valid
	err = bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password))
	require.NoError(t, err, "password hash should match")
}

func TestUpdateUserPassword(t *testing.T) {
	ctx := setupTestDB(t)

	// Arrange - Create a user first
	userID := NewTestID()
	email := NewTestEmail()
	oldPassword := "OldPassword123!"
	oldHashedPassword, err := bcrypt.GenerateFromPassword([]byte(oldPassword), bcrypt.DefaultCost)
	require.NoError(t, err)

	err = db.CreateUserFunc(ctx, db.CreateUserParams{
		ID:       userID,
		Email:    email,
		Password: string(oldHashedPassword),
		CreatedAt: sql.NullTime{
			Time:  time.Now().UTC(),
			Valid: true,
		},
		UpdatedAt: sql.NullTime{Valid: false},
	})
	require.NoError(t, err)

	// Act - Update password
	newPassword := "NewPassword456!"
	newHashedPassword, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	require.NoError(t, err)
	updatedAt := time.Now().UTC()

	err = db.UpdateUserPasswordFunc(ctx, db.UpdateUserPasswordParams{
		Password: string(newHashedPassword),
		UpdatedAt: sql.NullTime{
			Time:  updatedAt,
			Valid: true,
		},
		Email: email,
	})
	require.NoError(t, err)

	// Assert - Verify password was updated
	user, err := db.GetUserByEmailFunc(ctx, email)
	require.NoError(t, err)
	require.True(t, user.UpdatedAt.Valid)

	// Old password should not work
	err = bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(oldPassword))
	require.Error(t, err, "old password should not match")

	// New password should work
	err = bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(newPassword))
	require.NoError(t, err, "new password should match")
}

func TestGetUserByEmail_NotFound(t *testing.T) {
	ctx := setupTestDB(t)

	// Act - Try to get non-existent user
	nonExistentEmail := NewTestEmail()
	_, err := db.GetUserByEmailFunc(ctx, nonExistentEmail)

	// Assert
	require.Error(t, err)
	require.Equal(t, sql.ErrNoRows, err)
}
