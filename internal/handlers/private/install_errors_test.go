package handlers

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/shurco/mycart/internal/database"
	"github.com/shurco/mycart/internal/models"
	"github.com/shurco/mycart/internal/testutil"
	"github.com/shurco/mycart/internal/testutil/pgtest"
)

func TestInstallFailureStatus(t *testing.T) {
	cause := errors.New("connection refused")

	tests := []struct {
		name   string
		err    error
		want   int
		wantOK bool
	}{
		{"plain error has no status", cause, 0, false},
		{"typed error carries its status", &installError{status: 409, err: cause}, 409, true},
		{"wrapped typed error is found", fmt.Errorf("install: %w", &installError{status: 400, err: cause}), 400, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := installFailureStatus(tt.err)
			if ok != tt.wantOK {
				t.Errorf("installFailureStatus ok = %v, want %v", ok, tt.wantOK)
			}
			if ok && got != tt.want {
				t.Errorf("installFailureStatus = %d, want %d", got, tt.want)
			}
		})
	}

	t.Run("unwrap reaches the cause", func(t *testing.T) {
		err := error(&installError{status: 400, err: cause})
		if !errors.Is(err, cause) {
			t.Error("errors.Is did not reach the wrapped cause")
		}
		if err.Error() != "connection refused" {
			t.Errorf("Error() = %q", err.Error())
		}
	})
}

// choiceConfig turns the wizard's selection into a configuration. A selection
// that matches what the process is already on has to come back unchanged: the
// original carries the Source that decides whether the database is pinned.
func TestChoiceConfig(t *testing.T) {
	sqliteCurrent := database.Config{Driver: database.DriverSQLite, DSN: database.DefaultSQLiteDSN, Source: database.SourceEnv}
	postgresCurrent := database.Config{Driver: database.DriverPostgres, DSN: "postgres://u@h/db", Source: database.SourceFlag}

	tests := []struct {
		name    string
		choice  models.DatabaseChoice
		current database.Config
		want    database.Config
	}{
		{
			name:    "empty sqlite selection means the default file",
			choice:  models.DatabaseChoice{Driver: database.DriverSQLite},
			current: postgresCurrent,
			want:    database.Config{Driver: database.DriverSQLite, DSN: database.DefaultSQLiteDSN, Source: database.SourceWizard},
		},
		{
			name:    "sqlite with an explicit file",
			choice:  models.DatabaseChoice{Driver: database.DriverSQLite, DSN: "./other.db"},
			current: sqliteCurrent,
			want:    database.Config{Driver: database.DriverSQLite, DSN: "./other.db", Source: database.SourceWizard},
		},
		{
			name:    "postgres keeps its connection string",
			choice:  models.DatabaseChoice{Driver: database.DriverPostgres, DSN: "postgres://u@h/db"},
			current: sqliteCurrent,
			want:    database.Config{Driver: database.DriverPostgres, DSN: "postgres://u@h/db", Source: database.SourceWizard},
		},
		{
			// "SQLite (default)" on an installation already using it must not
			// quietly turn a pinned database into a wizard-chosen one, which
			// would make it changeable from the wizard afterwards.
			name:    "the current selection is returned as is",
			choice:  models.DatabaseChoice{Driver: database.DriverSQLite, DSN: database.DefaultSQLiteDSN},
			current: sqliteCurrent,
			want:    sqliteCurrent,
		},
		{
			name:    "an identical postgres selection is also returned as is",
			choice:  models.DatabaseChoice{Driver: database.DriverPostgres, DSN: "postgres://u@h/db"},
			current: postgresCurrent,
			want:    postgresCurrent,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := choiceConfig(tt.choice, tt.current)
			if got != tt.want {
				t.Errorf("choiceConfig = %+v, want %+v", got, tt.want)
			}
			if got.Pinned() != tt.want.Pinned() {
				t.Errorf("Pinned() = %v, want %v", got.Pinned(), tt.want.Pinned())
			}
		})
	}
}

// The probe is reachable before the cart exists, so its answer must name the
// problem and nothing else: no user, database, host or path from what was typed.
func TestDescribeConnectFailure(t *testing.T) {
	tests := []struct {
		name, detail, want string
	}{
		{
			"password rejected",
			`connect to postgres: failed to connect to "user=cart password=x database=cart": password authentication failed for user "cart"`,
			"authentication failed: check the user and password",
		},
		{
			"role does not exist",
			`connect to postgres: FATAL: role "someone" does not exist (SQLSTATE 28000)`,
			"authentication failed: check the user and password",
		},
		{
			"no password supplied",
			`connect to postgres: failed to connect to "user=cart": no password supplied`,
			"authentication failed: check the user and password",
		},
		{
			"database does not exist",
			`connect to postgres: FATAL: database "cart" does not exist (SQLSTATE 3D000)`,
			"the database does not exist on that server",
		},
		{
			"refused",
			`connect to postgres: dial tcp 10.0.0.5:5432: connect: connection refused`,
			"the server is not reachable at that address",
		},
		{
			"unknown host",
			`connect to postgres: dial tcp: lookup db.internal: no such host`,
			"the server is not reachable at that address",
		},
		{
			"timeout",
			`connect to postgres: dial tcp 10.0.0.5:5432: i/o timeout`,
			"the server is not reachable at that address",
		},
		{
			"context deadline",
			`connect to postgres: context deadline exceeded`,
			"the server is not reachable at that address",
		},
		{
			"network unreachable",
			`connect to postgres: dial tcp 10.0.0.5:5432: connect: network is unreachable`,
			"the server is not reachable at that address",
		},
		{
			"sqlite path missing",
			`open sqlite ./lc_base/data.db: no such file or directory`,
			"the database file cannot be opened at that path",
		},
		{
			"sqlite path unopenable",
			`open sqlite /root/data.db: unable to open database file`,
			"the database file cannot be opened at that path",
		},
		{
			"sqlite permission denied",
			`open sqlite /root/data.db: permission denied`,
			"the database file cannot be opened at that path",
		},
		{
			// Our own message: it explains the fix and names nothing private.
			"non-UTC session",
			`check postgres timezone: postgres session timezone is "Asia/Seoul", must be "UTC"`,
			`check postgres timezone: postgres session timezone is "Asia/Seoul", must be "UTC"`,
		},
		{
			"pgxpool option",
			`connection string parameter "pool_max_conns" is a pgxpool option and is not supported here`,
			`connection string parameter "pool_max_conns" is a pgxpool option and is not supported here`,
		},
		{
			"anything else",
			`connect to postgres: some future driver wording`,
			"could not connect to the selected database: check the connection settings",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := describeConnectFailure(errors.New(tt.detail))
			if got != tt.want {
				t.Errorf("describeConnectFailure = %q, want %q", got, tt.want)
			}

			// Whatever the reason is, it must not echo the address, the user
			// or the database back — except for the two messages this project
			// writes itself, which contain none of them.
			for _, secret := range []string{"cart", "someone", "10.0.0.5", "db.internal", "/root", "./lc_base"} {
				if strings.Contains(got, secret) {
					t.Errorf("reason %q leaks %q", got, secret)
				}
			}
		})
	}
}

// The same mapping, but driven by errors a real PostgreSQL server produces, so a
// change in the driver's wording is caught here rather than in production.
func TestDescribeConnectFailureFromPostgres(t *testing.T) {
	if !testutil.TestingPostgres() {
		t.Skip("set TEST_DB_DRIVER=postgres and TEST_POSTGRES_DSN to run")
	}

	good := pgtest.MigratedDSN(t)

	t.Run("wrong password", func(t *testing.T) {
		if os.Getenv(pgtest.AdminMode) == "0" {
			t.Skip("connection error tests require admin mode (TEST_POSTGRES_ADMIN=1)")
		}
		dsn := strings.Replace(good, "pgtdbpass", "wrong-password", 1)
		_, err := database.Connect(database.Config{Driver: database.DriverPostgres, DSN: dsn})
		if err == nil {
			t.Fatal("expected the connection to be refused")
		}

		got := describeConnectFailure(err)
		if got != "authentication failed: check the user and password" {
			t.Errorf("describeConnectFailure = %q", got)
		}
		if strings.Contains(got, "pgtdbuser") || strings.Contains(got, "wrong-password") {
			t.Errorf("reason %q leaks the credentials", got)
		}
	})

	t.Run("database does not exist", func(t *testing.T) {
		if os.Getenv(pgtest.AdminMode) == "0" {
			t.Skip("connection error tests require admin mode (TEST_POSTGRES_ADMIN=1)")
		}
		dsn := strings.Replace(good, "/testdb_tpl_", "/no_such_database_at_all_", 1)
		_, err := database.Connect(database.Config{Driver: database.DriverPostgres, DSN: dsn})
		if err == nil {
			t.Fatal("expected the connection to be refused")
		}

		got := describeConnectFailure(err)
		if got != "the database does not exist on that server" {
			t.Errorf("describeConnectFailure = %q", got)
		}
		if strings.Contains(got, "no_such_database_at_all") {
			t.Errorf("reason %q leaks the database name", got)
		}
	})
}
