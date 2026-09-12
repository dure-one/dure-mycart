package slugify

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/gosimple/slug"
)

// Querier is the database surface SlugService needs. It is declared here, on
// the consumer side, so this package does not depend on the storage layer and
// stays usable with any handle that rebinds placeholders for its dialect.
type Querier interface {
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

// SlugService handles slug generation and uniqueness checking
type SlugService struct {
	db Querier
}

// NewSlugService creates a new slug service
func NewSlugService(db Querier) *SlugService {
	return &SlugService{db: db}
}

// Generate creates a URL-friendly slug from name, ensures uniqueness
func (s *SlugService) Generate(ctx context.Context, name string, excludeID string) (string, error) {
	base := slug.Make(name)
	if base == "" {
		return "", fmt.Errorf("cannot generate slug from empty name")
	}

	// Check if base slug is available
	final := base
	counter := 2

	for {
		exists, err := s.exists(ctx, final, excludeID)
		if err != nil {
			return "", fmt.Errorf("checking slug existence: %w", err)
		}
		if !exists {
			return final, nil
		}

		// Try next number
		final = fmt.Sprintf("%s-%d", base, counter)
		counter++

		// Safety limit
		if counter > 1000 {
			return "", fmt.Errorf("failed to generate unique slug after 1000 attempts")
		}
	}
}

// exists checks if a slug exists in the database (excluding a specific product ID)
func (s *SlugService) exists(ctx context.Context, slug string, excludeID string) (bool, error) {
	query := "SELECT COUNT(*) FROM product WHERE slug = ? AND id != ?"
	if excludeID == "" {
		excludeID = "" // Ensures no match
	}

	var count int
	err := s.db.QueryRowContext(ctx, query, slug, excludeID).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("querying slug: %w", err)
	}

	return count > 0, nil
}
