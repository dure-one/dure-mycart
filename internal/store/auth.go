package store

import (
	"context"

	"github.com/shurco/mycart/internal/store/db"
	"github.com/shurco/mycart/pkg/errors"
)

// GetPasswordByEmail retrieves the password for a user by their email.
// Validates that the stored email matches the provided email.
func GetPasswordByEmail(ctx context.Context, email string) (string, error) {
	// Get email and password settings
	emailSetting, err := db.GetSettingByKeyFunc(ctx, "email")
	if err != nil {
		return "", err
	}

	passwordSetting, err := db.GetSettingByKeyFunc(ctx, "password")
	if err != nil {
		return "", err
	}

	// Validate email matches
	emailValue := ""
	if emailSetting.Value.Valid {
		emailValue = emailSetting.Value.String
	}
	if emailValue != email {
		return "", errors.ErrUserEmailNotFound
	}

	// Extract password
	passwordValue := ""
	if passwordSetting.Value.Valid {
		passwordValue = passwordSetting.Value.String
	}
	if passwordValue == "" {
		return "", errors.ErrUserPasswordNotFound
	}

	return passwordValue, nil
}
