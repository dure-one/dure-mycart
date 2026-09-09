package models

import (
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
		validation.Field(&v.DBType, validation.Required, validation.In("sqlite", "postgres")),
		validation.Field(&v.DatabaseURL, validation.When(
			v.DBType == "postgres",
			validation.Required,
		)),
		validation.Field(&v.SQLitePath, validation.When(
			v.DBType == "sqlite",
			validation.Required,
		)),
	)
}
