package db

import (
	"database/sql"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestConnect(t *testing.T) {
	tests := []struct {
		name    string
		setup   func(t *testing.T)
		cleanup func(t *testing.T)
		wantErr bool
	}{
		{
			name: "successful connection with default SQLite config",
			setup: func(t *testing.T) {
				t.Setenv("DB_TYPE", "sqlite")
				t.Setenv("SQLITE_PATH", ":memory:")
			},
			cleanup: func(t *testing.T) {},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.setup != nil {
				tt.setup(t)
			}
			if tt.cleanup != nil {
				t.Cleanup(func() { tt.cleanup(t) })
			}

			err := Connect()

			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.NotNil(t, db)
				require.NotEmpty(t, dbType)
			}
		})
	}
}

func TestIsInstalled(t *testing.T) {
	tests := []struct {
		name          string
		setup         func(t *testing.T) *sql.DB
		wantInstalled bool
		wantErr       bool
	}{
		{
			name: "returns false when goose_db_version table doesn't exist",
			setup: func(t *testing.T) *sql.DB {
				t.Setenv("DB_TYPE", "sqlite")
				t.Setenv("SQLITE_PATH", ":memory:")
				err := Connect()
				require.NoError(t, err)
				return db
			},
			wantInstalled: false,
			wantErr:       false,
		},
		{
			name: "returns true when goose_db_version table exists with records",
			setup: func(t *testing.T) *sql.DB {
				t.Setenv("DB_TYPE", "sqlite")
				t.Setenv("SQLITE_PATH", ":memory:")
				err := Connect()
				require.NoError(t, err)

				// Create goose_db_version table with a record
				_, err = db.Exec(`CREATE TABLE goose_db_version (
					id INTEGER PRIMARY KEY,
					version_id INTEGER NOT NULL,
					is_applied INTEGER NOT NULL,
					tstamp TIMESTAMP DEFAULT CURRENT_TIMESTAMP
				)`)
				require.NoError(t, err)
				_, err = db.Exec(`INSERT INTO goose_db_version (version_id, is_applied) VALUES (1, 1)`)
				require.NoError(t, err)

				return db
			},
			wantInstalled: true,
			wantErr:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testDB := tt.setup(t)
			require.NotNil(t, testDB)
			t.Cleanup(func() {
				if testDB != nil {
					testDB.Close()
				}
			})

			installed, err := IsInstalled()

			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Equal(t, tt.wantInstalled, installed)
			}
		})
	}
}
