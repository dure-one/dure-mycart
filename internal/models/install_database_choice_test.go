package models

import (
	"strings"
	"testing"
)

func TestDatabaseChoice_Validate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		choice  DatabaseChoice
		wantErr string
	}{
		{"sqlite needs no connection string", DatabaseChoice{Driver: "sqlite"}, ""},
		{"sqlite with a path", DatabaseChoice{Driver: "sqlite", DSN: "./lc_base/data.db"}, ""},
		{"postgres with a connection string", DatabaseChoice{Driver: "postgres", DSN: "postgres://u@h/db"}, ""},
		{"postgres without a connection string", DatabaseChoice{Driver: "postgres"}, "a PostgreSQL connection string is required"},
		{"no driver", DatabaseChoice{DSN: "postgres://u@h/db"}, "driver"},
		{"an unsupported driver", DatabaseChoice{Driver: "mysql", DSN: "mysql://u@h/db"}, "driver"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := tt.choice.Validate()
			switch {
			case tt.wantErr == "" && err != nil:
				t.Errorf("Validate() = %v, want nil", err)
			case tt.wantErr != "" && err == nil:
				t.Errorf("Validate() = nil, want %q", tt.wantErr)
			case tt.wantErr != "" && !strings.Contains(err.Error(), tt.wantErr):
				t.Errorf("Validate() = %v, want it to contain %q", err, tt.wantErr)
			}
		})
	}
}

// The wizard sends the database choice inside the install request, so the
// choice has to be validated when the whole request is validated.
func TestInstall_ValidateDatabaseChoice(t *testing.T) {
	t.Parallel()

	base := Install{Email: "admin@example.com", Password: "Pass123", Domain: "site.com"}

	if err := base.Validate(); err != nil {
		t.Errorf("no database choice must stay valid: %v", err)
	}

	withChoice := base
	withChoice.Database = &DatabaseChoice{Driver: "sqlite"}
	if err := withChoice.Validate(); err != nil {
		t.Errorf("a sqlite choice must be valid: %v", err)
	}

	withBadChoice := base
	withBadChoice.Database = &DatabaseChoice{Driver: "postgres"}
	err := withBadChoice.Validate()
	if err == nil {
		t.Fatal("a postgres choice without a connection string must be rejected")
	}
	if !strings.Contains(err.Error(), "a PostgreSQL connection string is required") {
		t.Errorf("Validate() = %v, want it to explain the missing connection string", err)
	}
}
