package queries

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestAddRefreshToken(t *testing.T) {
	db, ctx := bootstrap(t)

	now := time.Now().Unix()
	id := uuid.NewString()
	userID := uuid.NewString()

	tests := []struct {
		name      string
		id        string
		userID    string
		role      string
		expiresAt int64
		wantErr   bool
	}{
		{
			name:      "admin token",
			id:        id,
			userID:    userID,
			role:      "admin",
			expiresAt: now + 3600,
			wantErr:   false,
		},
		{
			name:      "customer token",
			id:        uuid.NewString(),
			userID:    uuid.NewString(),
			role:      "customer",
			expiresAt: now + 7200,
			wantErr:   false,
		},
		{
			name:      "invalid role",
			id:        uuid.NewString(),
			userID:    uuid.NewString(),
			role:      "invalid",
			expiresAt: now + 3600,
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := db.AddRefreshToken(ctx, tt.id, tt.userID, tt.role, tt.expiresAt)
			if (err != nil) != tt.wantErr {
				t.Errorf("AddRefreshToken() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestGetRefreshToken(t *testing.T) {
	db, ctx := bootstrap(t)

	now := time.Now().Unix()
	id := uuid.NewString()
	userID := uuid.NewString()
	role := "admin"

	// Add token
	if err := db.AddRefreshToken(ctx, id, userID, role, now+3600); err != nil {
		t.Fatalf("setup: %v", err)
	}

	tests := []struct {
		name       string
		id         string
		wantUserID string
		wantRole   string
		wantErr    bool
	}{
		{
			name:       "valid token",
			id:         id,
			wantUserID: userID,
			wantRole:   role,
			wantErr:    false,
		},
		{
			name:    "non-existent token",
			id:      uuid.NewString(),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotUserID, gotRole, err := db.GetRefreshToken(ctx, tt.id)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetRefreshToken() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if gotUserID != tt.wantUserID {
					t.Errorf("GetRefreshToken() userID = %v, want %v", gotUserID, tt.wantUserID)
				}
				if gotRole != tt.wantRole {
					t.Errorf("GetRefreshToken() role = %v, want %v", gotRole, tt.wantRole)
				}
			}
		})
	}
}

func TestGetRefreshToken_Expired(t *testing.T) {
	db, ctx := bootstrap(t)

	now := time.Now().Unix()
	id := uuid.NewString()
	userID := uuid.NewString()

	// Add expired token
	if err := db.AddRefreshToken(ctx, id, userID, "admin", now-3600); err != nil {
		t.Fatalf("setup: %v", err)
	}

	// Should fail because token is expired
	_, _, err := db.GetRefreshToken(ctx, id)
	if err == nil {
		t.Error("GetRefreshToken() expected error for expired token, got nil")
	}
}

func TestDeleteRefreshToken(t *testing.T) {
	db, ctx := bootstrap(t)

	now := time.Now().Unix()
	id := uuid.NewString()

	// Add token
	if err := db.AddRefreshToken(ctx, id, uuid.NewString(), "admin", now+3600); err != nil {
		t.Fatalf("setup: %v", err)
	}

	// Delete it
	if err := db.DeleteRefreshToken(ctx, id); err != nil {
		t.Errorf("DeleteRefreshToken() error = %v", err)
	}

	// Verify it's gone
	_, _, err := db.GetRefreshToken(ctx, id)
	if err == nil {
		t.Error("token still exists after delete")
	}

	// Delete non-existent is ok (idempotent)
	if err := db.DeleteRefreshToken(ctx, uuid.NewString()); err != nil {
		t.Errorf("DeleteRefreshToken() on non-existent should not error, got %v", err)
	}
}

func TestDeleteRefreshTokensByUser(t *testing.T) {
	db, ctx := bootstrap(t)

	now := time.Now().Unix()
	userID := uuid.NewString()

	// Add multiple tokens for same user
	id1 := uuid.NewString()
	id2 := uuid.NewString()
	if err := db.AddRefreshToken(ctx, id1, userID, "admin", now+3600); err != nil {
		t.Fatalf("setup: %v", err)
	}
	if err := db.AddRefreshToken(ctx, id2, userID, "admin", now+3600); err != nil {
		t.Fatalf("setup: %v", err)
	}

	// Add token for different user (should not be deleted)
	otherUserID := uuid.NewString()
	otherId := uuid.NewString()
	if err := db.AddRefreshToken(ctx, otherId, otherUserID, "admin", now+3600); err != nil {
		t.Fatalf("setup: %v", err)
	}

	// Delete all tokens for userID
	if err := db.DeleteRefreshTokensByUser(ctx, userID, "admin"); err != nil {
		t.Errorf("DeleteRefreshTokensByUser() error = %v", err)
	}

	// Verify user tokens are gone
	_, _, err1 := db.GetRefreshToken(ctx, id1)
	_, _, err2 := db.GetRefreshToken(ctx, id2)
	if err1 == nil || err2 == nil {
		t.Error("user tokens still exist after DeleteRefreshTokensByUser")
	}

	// Verify other user token still exists
	_, _, err := db.GetRefreshToken(ctx, otherId)
	if err != nil {
		t.Error("other user token was deleted")
	}
}

func TestDeleteRefreshTokensByUser_RoleFilter(t *testing.T) {
	db, ctx := bootstrap(t)

	now := time.Now().Unix()
	userID := uuid.NewString()

	// Add admin and customer tokens for same user_id (edge case but possible)
	adminID := uuid.NewString()
	customerID := uuid.NewString()
	if err := db.AddRefreshToken(ctx, adminID, userID, "admin", now+3600); err != nil {
		t.Fatalf("setup: %v", err)
	}
	if err := db.AddRefreshToken(ctx, customerID, userID, "customer", now+3600); err != nil {
		t.Fatalf("setup: %v", err)
	}

	// Delete only admin tokens
	if err := db.DeleteRefreshTokensByUser(ctx, userID, "admin"); err != nil {
		t.Errorf("DeleteRefreshTokensByUser() error = %v", err)
	}

	// Verify admin token is gone
	_, _, err := db.GetRefreshToken(ctx, adminID)
	if err == nil {
		t.Error("admin token still exists")
	}

	// Verify customer token still exists
	_, _, err = db.GetRefreshToken(ctx, customerID)
	if err != nil {
		t.Error("customer token was deleted")
	}
}
