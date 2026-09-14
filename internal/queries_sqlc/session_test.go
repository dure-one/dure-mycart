// +build sqlc

package queries_sqlc

import (
	"context"
	"testing"
	"time"

	"github.com/shurco/mycart/internal/database"
	"github.com/shurco/mycart/migrations"
)

// bootstrap initialises a fresh DB and returns the *Base for sqlc backend.
func bootstrap(t *testing.T) (*Base, context.Context) {
	t.Helper()

	// Create temp directory for SQLite database
	tmpDir := t.TempDir()
	dsn := tmpDir + "/test.db"

	// Open database with migrations
	conn, err := database.Open(database.Config{
		Driver: database.DriverSQLite,
		DSN:    dsn,
	}, migrations.Embed())
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	t.Cleanup(func() { conn.Close() })

	// Create queries base
	base := NewBase(conn)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)

	return base, ctx
}

func TestSessionCRUD(t *testing.T) {
	t.Parallel()
	db, ctx := bootstrap(t)

	now := time.Now().Unix()

	// Test AddSession
	if err := db.AddSession(ctx, "sess-1", "value-1", now+3600); err != nil {
		t.Fatalf("AddSession: %v", err)
	}

	// Test GetSession
	got, err := db.GetSession(ctx, "sess-1")
	if err != nil {
		t.Fatalf("GetSession: %v", err)
	}
	if got != "value-1" {
		t.Errorf("got %q, want value-1", got)
	}

	// Idempotent re-insert is what AddSession promises
	if err := db.AddSession(ctx, "sess-1", "value-2", now+3600); err != nil {
		t.Fatalf("AddSession replace: %v", err)
	}
	got, _ = db.GetSession(ctx, "sess-1")
	if got != "value-2" {
		t.Errorf("got %q, want value-2 after replace", got)
	}

	// Test UpdateSession
	if err := db.UpdateSession(ctx, "sess-1", "value-3", now+7200); err != nil {
		t.Fatalf("UpdateSession: %v", err)
	}
	got, _ = db.GetSession(ctx, "sess-1")
	if got != "value-3" {
		t.Errorf("got %q, want value-3", got)
	}

	// Expired sessions must not be returned
	if err := db.UpdateSession(ctx, "sess-1", "value-3", now-10); err != nil {
		t.Fatalf("expire UpdateSession: %v", err)
	}
	if _, err := db.GetSession(ctx, "sess-1"); err == nil {
		t.Error("expected error for expired session")
	}

	// Test DeleteSession
	if err := db.DeleteSession(ctx, "sess-1"); err != nil {
		t.Fatalf("DeleteSession: %v", err)
	}
	if _, err := db.GetSession(ctx, "sess-1"); err == nil {
		t.Error("expected error after delete")
	}
}

func TestSessionExpiry(t *testing.T) {
	t.Parallel()
	db, ctx := bootstrap(t)

	now := time.Now().Unix()

	// Add a session that expires in the past
	if err := db.AddSession(ctx, "expired", "value", now-100); err != nil {
		t.Fatalf("AddSession with past expiry: %v", err)
	}

	// GetSession should not return expired sessions
	if _, err := db.GetSession(ctx, "expired"); err == nil {
		t.Error("expected error for expired session")
	}

	// Add a valid session
	if err := db.AddSession(ctx, "valid", "value", now+3600); err != nil {
		t.Fatalf("AddSession: %v", err)
	}

	// GetSession should return valid session
	if got, err := db.GetSession(ctx, "valid"); err != nil {
		t.Fatalf("GetSession: %v", err)
	} else if got != "value" {
		t.Errorf("got %q, want value", got)
	}
}
