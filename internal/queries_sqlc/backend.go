// +build sqlc

package queries_sqlc

import (
	"context"
)

// backend abstracts dialect-specific sqlc-generated code
type backend interface {
	// Auth methods
	GetPasswordByEmail(ctx context.Context, email string) (string, error)

	// Session methods
	GetSession(ctx context.Context, key string) (string, error)
	AddSession(ctx context.Context, key, value string, expires int64) error
	UpdateSession(ctx context.Context, key, value string, expires int64) error
	DeleteSession(ctx context.Context, key string) error

	// Install methods
	IsInstalled(ctx context.Context) (bool, error)
	Install(ctx context.Context, settings map[string]string) error

	// Page methods
	PageExists(ctx context.Context, slug string) (bool, error)
	GetPageBySlug(ctx context.Context, slug string) (*PageRow, error)
	GetPageByID(ctx context.Context, id string) (*PageRow, error)
	InsertPage(ctx context.Context, id, name, slug, position string, content *string, active bool) (*PageRow, error)
	UpdatePage(ctx context.Context, name, slug, position string, content *string, active bool, id string) error
	DeletePage(ctx context.Context, id string) error
	UpdatePageActive(ctx context.Context, id string) error

	// TODO: Add remaining query group methods as they are migrated
	// (setting, product, cart, customer)
}

// PageRow represents a page record
type PageRow struct {
	ID       string
	Name     string
	Slug     string
	Position string
	Content  *string
	Active   bool
	Created  int64
	Updated  *int64
}
