package models

import (
	"fmt"

	validation "github.com/go-ozzo/ozzo-validation/v4"
	"github.com/go-ozzo/ozzo-validation/v4/is"
)

// Install is ...
type Install struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	Domain   string `json:"domain"`
	// Database is the optional database selection from the install wizard.
	// Absent means "keep the database the process is already configured for",
	// which is what every installation did before PostgreSQL support existed.
	Database *DatabaseChoice `json:"database"`
}

// Validate is ...
func (v Install) Validate() error {
	return validation.ValidateStruct(&v,
		validation.Field(&v.Email, validation.Required, is.Email),
		validation.Field(&v.Password, validation.Required, validation.Length(6, 72)),
		validation.Field(&v.Database),
	)
}

// DatabaseChoice is the database picked in the install wizard.
type DatabaseChoice struct {
	Driver string `json:"driver"`
	DSN    string `json:"dsn"`
}

// Validate is ...
func (v DatabaseChoice) Validate() error {
	return validation.ValidateStruct(&v,
		validation.Field(&v.Driver, validation.Required, validation.In("sqlite", "postgres")),
		validation.Field(&v.DSN, validation.By(func(value any) error {
			dsn, _ := value.(string)
			if v.Driver == "postgres" && dsn == "" {
				return fmt.Errorf("a PostgreSQL connection string is required")
			}
			return nil
		})),
	)
}
