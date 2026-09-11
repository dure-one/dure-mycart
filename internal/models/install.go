package models

import (
	"strings"

	validation "github.com/go-ozzo/ozzo-validation/v4"
	"github.com/go-ozzo/ozzo-validation/v4/is"
)

// Install is ...
type Install struct {
	Email       string `json:"email"`
	Password    string `json:"password"`
	Domain      string `json:"domain"`
	DBType      string `json:"dbType"`      // "sqlite" or "postgres"
	DatabaseURL string `json:"databaseUrl"` // PostgreSQL connection string
	SQLitePath  string `json:"sqlitePath"`  // SQLite database path
}

// Validate is ...
func (v Install) Validate() error {
	return validation.ValidateStruct(&v,
		validation.Field(&v.Email, validation.Required, is.Email),
		validation.Field(&v.Password, validation.Required, validation.Length(6, 72)),
		validation.Field(&v.DBType, validation.Required, validation.In("sqlite", "postgres", "postgresql")),
		validation.Field(&v.DatabaseURL, validation.When(
			v.DBType == "postgres" || v.DBType == "postgresql",
			validation.Required,
			validation.By(func(value interface{}) error {
				s, _ := value.(string)
				if s == "" {
					return nil // Required rule will catch this
				}
				if !strings.HasPrefix(s, "postgres://") && !strings.HasPrefix(s, "postgresql://") {
					return validation.NewError("validation_postgres_url", "databaseUrl must start with 'postgres://' or 'postgresql://'")
				}
				return nil
			}),
		)),
		validation.Field(&v.SQLitePath, validation.When(
			v.DBType == "sqlite",
			validation.Required,
		)),
	)
}
