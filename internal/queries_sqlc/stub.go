// +build sqlc

package queries_sqlc

import (
	"context"
	"errors"

	"github.com/shurco/mycart/internal/models"
	"github.com/shurco/mycart/pkg/security"
)

var ErrNotImplemented = errors.New("sqlc query not yet implemented")

// Stub methods on Base - these will be replaced group-by-group

func (b *Base) GetPasswordByEmail(ctx context.Context, email string) (string, error) {
	return b.backend.GetPasswordByEmail(ctx, email)
}

// Session methods
func (b *Base) GetSession(ctx context.Context, key string) (string, error) {
	return b.backend.GetSession(ctx, key)
}

func (b *Base) AddSession(ctx context.Context, key, value string, expires int64) error {
	return b.backend.AddSession(ctx, key, value, expires)
}

func (b *Base) UpdateSession(ctx context.Context, key, value string, expires int64) error {
	return b.backend.UpdateSession(ctx, key, value, expires)
}

func (b *Base) DeleteSession(ctx context.Context, key string) error {
	return b.backend.DeleteSession(ctx, key)
}

// Install methods
var ErrAlreadyInstalled = errors.New("cart already installed")

func (b *Base) IsInstalled(ctx context.Context) (bool, error) {
	return b.backend.IsInstalled(ctx)
}

func (b *Base) Install(ctx context.Context, i *models.Install) error {
	installed, err := b.IsInstalled(ctx)
	if err != nil {
		return err
	}
	if installed {
		return ErrAlreadyInstalled
	}

	// Hash password
	passwordHash, err := security.HashPassword(i.Password)
	if err != nil {
		return err
	}

	// Generate JWT secret
	jwtSecret, err := security.NewToken(passwordHash)
	if err != nil {
		return err
	}

	// Prepare settings map
	settings := map[string]string{
		"installed":  "true",
		"domain":     i.Domain,
		"email":      i.Email,
		"password":   passwordHash,
		"jwt_secret": jwtSecret,
	}

	return b.backend.Install(ctx, settings)
}

// TODO: Add stub methods for remaining query groups
// For now, these will return ErrNotImplemented if called
